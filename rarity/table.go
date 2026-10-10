// Package rarity scores how uncommon a word is in a large general corpus.
// The extractor uses that score so a word that is common everywhere ranks
// below a word that is specific to the article.
//
// A table is built by cmd/build-rarity from Leipzig sentence files. Each
// language pack embeds its table as lang/<code>/rarity.bin.gz. Sources and
// the licence are in lang/THIRD_PARTY.md.
package rarity

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
	"time"
)

const (
	magic   = "RIDF"
	version = 1
	// MinCount is the smallest number of sentences a word must appear in
	// to be stored. Rarer words are left out and score as unknown.
	MinCount = 3
)

// Table maps a normalized word to a rarity score in [0, 1]. 0 is a word
// that appears in every sentence of the corpus; 1 is a word that appears
// in only MinCount sentences. Lookup misses are not stored.
type Table struct {
	sentences   int
	packedWords string   // every stored word, sorted and concatenated
	offsets     []uint32 // start of each word, plus a final end offset
	scores      []uint8
}

// Build quantises inverse sentence frequency. sentenceCounts maps a
// normalized word to the number of sentences it appears in; a word is
// counted once per sentence. Words seen fewer than minCount times are
// dropped. The stored score is log(sentences/count) /
// log(sentences/minCount), clamped to [0, 1].
func Build(sentences int, sentenceCounts map[string]int, minCount int) (*Table, error) {
	if sentences < 1 || minCount < 1 || minCount > sentences {
		return nil, fmt.Errorf("rarity: sentences %d, min count %d", sentences, minCount)
	}
	words := make([]string, 0, len(sentenceCounts))
	for word, count := range sentenceCounts {
		if word == "" || count < minCount || count > sentences || len(word) > math.MaxUint16 {
			continue
		}
		words = append(words, word)
	}
	slices.Sort(words)
	logFloor := math.Log(float64(sentences) / float64(minCount))
	var packed strings.Builder
	offsets := make([]uint32, len(words)+1)
	scores := make([]uint8, len(words))
	for i, word := range words {
		offsets[i] = uint32(packed.Len())
		packed.WriteString(word)
		scores[i] = quantize(float64(sentences)/float64(sentenceCounts[word]), logFloor)
	}
	offsets[len(words)] = uint32(packed.Len())

	return &Table{sentences: sentences, packedWords: packed.String(), offsets: offsets, scores: scores}, nil
}

// quantize maps log(ratio) / logFloor onto a byte. logFloor is 0 when every
// kept word is exactly at the rarity floor, so those words score 1.
func quantize(ratio, logFloor float64) uint8 {
	if logFloor <= 0 || ratio <= 1 {
		if logFloor <= 0 {
			return 255
		}

		return 0
	}
	normalized := math.Log(ratio) / logFloor
	if normalized > 1 {
		normalized = 1
	}

	return uint8(math.Round(normalized * 255))
}

// Words is the number of stored words.
func (t *Table) Words() int {
	if t == nil {
		return 0
	}
	return len(t.scores)
}

// Sentences is the corpus size the scores were computed from.
func (t *Table) Sentences() int {
	if t == nil {
		return 0
	}
	return t.sentences
}

// Score reports the stored rarity of a normalized word.
func (t *Table) Score(norm string) (float64, bool) {
	if t == nil || len(t.scores) == 0 {
		return 0, false
	}
	return t.search(norm)
}

func (t *Table) search(norm string) (float64, bool) {
	lo, hi := 0, len(t.scores)
	for lo < hi {
		mid := (lo + hi) / 2
		if t.word(mid) < norm {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo < len(t.scores) && t.word(lo) == norm {
		return float64(t.scores[lo]) / 255, true
	}
	return 0, false
}

func (t *Table) word(i int) string {
	return t.packedWords[t.offsets[i]:t.offsets[i+1]]
}

// Write writes the table as gzip. The output is deterministic.
func (t *Table) Write(writer io.Writer) error {
	gzipWriter, err := gzip.NewWriterLevel(writer, gzip.BestCompression)
	if err != nil {
		return err
	}
	gzipWriter.Name = ""
	gzipWriter.ModTime = time.Time{}
	gzipWriter.OS = 255
	if err := writeRaw(gzipWriter, t); err != nil {
		_ = gzipWriter.Close()
		return err
	}
	return gzipWriter.Close()
}

func writeRaw(writer io.Writer, table *Table) error {
	buffered := bufio.NewWriter(writer)
	if _, err := buffered.WriteString(magic); err != nil {
		return err
	}
	var header [9]byte
	header[0] = version
	binary.BigEndian.PutUint32(header[1:5], uint32(table.sentences))
	binary.BigEndian.PutUint32(header[5:9], uint32(len(table.scores)))
	if _, err := buffered.Write(header[:]); err != nil {
		return err
	}
	var wordLen [2]byte
	for i := range table.scores {
		word := table.word(i)
		binary.BigEndian.PutUint16(wordLen[:], uint16(len(word)))
		if _, err := buffered.Write(wordLen[:]); err != nil {
			return err
		}
		if _, err := buffered.WriteString(word); err != nil {
			return err
		}
		if err := buffered.WriteByte(table.scores[i]); err != nil {
			return err
		}
	}
	return buffered.Flush()
}

// Read reads a table written by Write.
func Read(reader io.Reader) (*Table, error) {
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("rarity: %w", err)
	}
	defer func() { _ = gzipReader.Close() }()
	return readRaw(gzipReader)
}

func readRaw(reader io.Reader) (*Table, error) {
	buffered := bufio.NewReader(reader)
	magicBytes := make([]byte, len(magic))
	if _, err := io.ReadFull(buffered, magicBytes); err != nil {
		return nil, fmt.Errorf("rarity: %w", err)
	}
	if string(magicBytes) != magic {
		return nil, fmt.Errorf("rarity: bad magic %q", magicBytes)
	}
	var header [9]byte
	if _, err := io.ReadFull(buffered, header[:]); err != nil {
		return nil, fmt.Errorf("rarity: %w", err)
	}
	if header[0] != version {
		return nil, fmt.Errorf("rarity: version %d", header[0])
	}
	sentences := int(binary.BigEndian.Uint32(header[1:5]))
	wordCount := int(binary.BigEndian.Uint32(header[5:9]))
	if sentences < 1 {
		return nil, fmt.Errorf("rarity: sentences %d", sentences)
	}
	var packed strings.Builder
	offsets := make([]uint32, wordCount+1)
	scores := make([]uint8, wordCount)
	var wordLen [2]byte
	var prev string
	for i := range wordCount {
		if _, err := io.ReadFull(buffered, wordLen[:]); err != nil {
			return nil, fmt.Errorf("rarity: word %d: %w", i, err)
		}
		size := int(binary.BigEndian.Uint16(wordLen[:]))
		if size == 0 {
			return nil, fmt.Errorf("rarity: word %d is empty", i)
		}
		buf := make([]byte, size)
		if _, err := io.ReadFull(buffered, buf); err != nil {
			return nil, fmt.Errorf("rarity: word %d: %w", i, err)
		}
		word := string(buf)
		if i > 0 && word <= prev {
			return nil, fmt.Errorf("rarity: words out of order at %d", i)
		}
		prev = word
		score, err := buffered.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("rarity: word %d: %w", i, err)
		}
		offsets[i] = uint32(packed.Len())
		packed.WriteString(word)
		scores[i] = score
	}
	if _, err := buffered.ReadByte(); err != io.EOF {
		return nil, fmt.Errorf("rarity: trailing bytes")
	}
	offsets[wordCount] = uint32(packed.Len())
	return &Table{sentences: sentences, packedWords: packed.String(), offsets: offsets, scores: scores}, nil
}

// Bytes is the gzip encoding, for tests.
func (t *Table) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := t.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

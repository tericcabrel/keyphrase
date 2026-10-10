package postag

import (
	"bufio"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"slices"
)

const modelMagic = "APOS1"

// Tagger labels each word of a sentence with a part of speech. It is a
// greedy left-to-right averaged perceptron: every word gets a score per tag
// from weighted features (the word, its suffixes, its neighbours, the two
// previous predicted tags), and the best-scoring tag wins. Frequent words
// that almost always carry one tag are looked up in a tag dictionary
// instead. A Tagger is read-only and safe for concurrent use.
type Tagger struct {
	tagDict map[string]Tag
	weights map[uint64]weightRef
	tags    []Tag
	values  []float32
}

// weightRef locates one feature's non-zero weights in Tagger.tags and
// Tagger.values.
type weightRef struct {
	offset uint32
	count  uint8
}

// Tag returns one tag per word. words are the surface forms of one
// sentence's non-space tokens, in order, including punctuation.
func (t *Tagger) Tag(words []string) []Tag {
	tags, _ := t.TagWithMargins(words)
	return tags
}

// TagWithMargins is Tag plus a confidence value per word: how far the chosen
// tag's score is ahead of the runner-up tag. A small margin on an adverb
// means the tagger nearly chose something else, such as an adjective. Words
// found in the tag dictionary get +Inf.
func (t *Tagger) TagWithMargins(words []string) ([]Tag, []float32) {
	tags := make([]Tag, len(words))
	margins := make([]float32, len(words))
	if len(words) == 0 {
		return tags, margins
	}
	sentenceContext := newContext(words)
	featureHashes := make([]uint64, 0, numFeatures)
	prev, prev2 := tagStart, tagStart2
	for i := range words {
		tag, ok := t.tagDict[sentenceContext.norms[i+2]]
		margin := float32(math.Inf(1))
		if !ok {
			featureHashes = sentenceContext.features(featureHashes[:0], i, prev, prev2)
			tag, margin = t.predict(featureHashes)
		}
		tags[i], margins[i] = tag, margin
		prev2, prev = prev, byte(tag)
	}
	return tags, margins
}

func (t *Tagger) predict(featureHashes []uint64) (Tag, float32) {
	var scores [NumTags]float32
	for _, featureHash := range featureHashes {
		ref, ok := t.weights[featureHash]
		if !ok {
			continue
		}
		end := ref.offset + uint32(ref.count)
		for k := ref.offset; k < end; k++ {
			scores[t.tags[k]] += t.values[k]
		}
	}
	best := argmax(scores[:])
	runnerUp := float32(math.Inf(-1))
	for tagIndex, score := range scores {
		if Tag(tagIndex) != best && score > runnerUp {
			runnerUp = score
		}
	}
	return best, scores[best] - runnerUp
}

// argmax returns the highest-scoring tag; ties go to the lower tag number so
// that results are deterministic.
func argmax[S ~float32 | ~float64](scores []S) Tag {
	best := 0
	for i := 1; i < len(scores); i++ {
		if scores[i] > scores[best] {
			best = i
		}
	}
	return Tag(best)
}

// Features reports the number of features with at least one weight, and the
// number of tag-dictionary words.
func (t *Tagger) Features() (features, dictWords int) {
	return len(t.weights), len(t.tagDict)
}

// WriteTo writes the model as gzip-compressed binary data. The output is
// deterministic: features are sorted by hash and dictionary words by text.
func (t *Tagger) WriteTo(w io.Writer) (int64, error) {
	counter := &countingWriter{writer: w}
	gzipWriter, err := gzip.NewWriterLevel(counter, gzip.BestCompression)
	if err != nil {
		return 0, err
	}
	buffered := bufio.NewWriter(gzipWriter)
	var buf [binary.MaxVarintLen64]byte
	uvarint := func(value uint64) { _, _ = buffered.Write(buf[:binary.PutUvarint(buf[:], value)]) }

	_, _ = buffered.WriteString(modelMagic)
	uvarint(NumTags)
	uvarint(uint64(len(t.tagDict)))
	for _, word := range slices.Sorted(maps.Keys(t.tagDict)) {
		uvarint(uint64(len(word)))
		_, _ = buffered.WriteString(word)
		_ = buffered.WriteByte(byte(t.tagDict[word]))
	}
	uvarint(uint64(len(t.weights)))
	for _, featureHash := range slices.Sorted(maps.Keys(t.weights)) {
		ref := t.weights[featureHash]
		binary.LittleEndian.PutUint64(buf[:8], featureHash)
		_, _ = buffered.Write(buf[:8])
		_ = buffered.WriteByte(ref.count)
		for k := ref.offset; k < ref.offset+uint32(ref.count); k++ {
			_ = buffered.WriteByte(byte(t.tags[k]))
			binary.LittleEndian.PutUint32(buf[:4], math.Float32bits(t.values[k]))
			_, _ = buffered.Write(buf[:4])
		}
	}
	if err := buffered.Flush(); err != nil {
		return counter.written, err
	}
	if err := gzipWriter.Close(); err != nil {
		return counter.written, err
	}
	return counter.written, counter.err
}

// ReadTagger reads a model written by WriteTo.
func ReadTagger(r io.Reader) (*Tagger, error) {
	gzipReader, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gzipReader.Close() }()
	buffered := bufio.NewReader(gzipReader)
	fail := func(what string, err error) (*Tagger, error) {
		if errors.Is(err, io.EOF) {
			err = io.ErrUnexpectedEOF
		}
		return nil, fmt.Errorf("postag: reading %s: %w", what, err)
	}

	magic := make([]byte, len(modelMagic))
	if _, readErr := io.ReadFull(buffered, magic); readErr != nil {
		return fail("header", readErr)
	}
	if string(magic) != modelMagic {
		return nil, errors.New("postag: not a model file")
	}
	tagCount, err := binary.ReadUvarint(buffered)
	if err != nil {
		return fail("header", err)
	}
	if tagCount != NumTags {
		return nil, fmt.Errorf("postag: model has %d tags, want %d", tagCount, NumTags)
	}
	readTag := func() (Tag, error) {
		b, readErr := buffered.ReadByte()
		if readErr == nil && int(b) >= NumTags {
			readErr = fmt.Errorf("invalid tag %d", b)
		}
		return Tag(b), readErr
	}

	dictWordCount, err := binary.ReadUvarint(buffered)
	if err != nil {
		return fail("dictionary", err)
	}
	tagger := &Tagger{tagDict: make(map[string]Tag, dictWordCount)}
	for range dictWordCount {
		wordLen, readErr := binary.ReadUvarint(buffered)
		if readErr != nil {
			return fail("dictionary", readErr)
		}
		word := make([]byte, wordLen)
		if _, readErr = io.ReadFull(buffered, word); readErr != nil {
			return fail("dictionary", readErr)
		}
		tag, tagErr := readTag()
		if tagErr != nil {
			return fail("dictionary", tagErr)
		}
		tagger.tagDict[string(word)] = tag
	}

	featureCount, readErr := binary.ReadUvarint(buffered)
	if readErr != nil {
		return fail("weights", readErr)
	}
	tagger.weights = make(map[uint64]weightRef, featureCount)
	var buf [8]byte
	for range featureCount {
		if _, readErr = io.ReadFull(buffered, buf[:8]); readErr != nil {
			return fail("weights", readErr)
		}
		featureHash := binary.LittleEndian.Uint64(buf[:8])
		weightCount, byteErr := buffered.ReadByte()
		if byteErr != nil {
			return fail("weights", byteErr)
		}
		tagger.weights[featureHash] = weightRef{offset: uint32(len(tagger.tags)), count: weightCount}
		for range weightCount {
			tag, tagErr := readTag()
			if tagErr != nil {
				return fail("weights", tagErr)
			}
			if _, readErr = io.ReadFull(buffered, buf[:4]); readErr != nil {
				return fail("weights", readErr)
			}
			tagger.tags = append(tagger.tags, tag)
			tagger.values = append(tagger.values, math.Float32frombits(binary.LittleEndian.Uint32(buf[:4])))
		}
	}
	if _, readErr = buffered.ReadByte(); readErr != io.EOF {
		return nil, errors.New("postag: trailing data after model")
	}
	return tagger, nil
}

type countingWriter struct {
	writer  io.Writer
	written int64
	err     error
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.writer.Write(p)
	c.written += int64(n)
	if err != nil && c.err == nil {
		c.err = err
	}
	return n, err
}

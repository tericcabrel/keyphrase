package postag

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Feature templates. Each feature is a 64-bit FNV-1a hash of its template id
// and its values, so model files store no feature strings. Ids are stored
// implicitly in trained weights and must not be renumbered.
const (
	featBias byte = iota
	featSuffix1
	featSuffix2
	featSuffix3
	featSuffix4
	featPrefix1
	featPrevTag
	featPrev2Tag
	featPrevTags
	featWord
	featPrevTagWord
	featPrevWord
	featPrevSuffix3
	featPrev2Word
	featNextWord
	featNextSuffix3
	featNext2Word
	featShape
	featShapeInitial
	featPrevWordWord
	featWordNextWord
	numFeatures
)

// Sentinel values for positions outside the sentence.
const (
	tagStart  = byte(NumTags)
	tagStart2 = byte(NumTags + 1)
)

var (
	padBefore = [2]string{"-START2-", "-START-"}
	padAfter  = [2]string{"-END-", "-END2-"}
)

// context is one sentence prepared for feature extraction: normalized words
// with two padding entries on each side, and word shapes.
type context struct {
	norms  []string
	shapes []string
}

func newContext(words []string) context {
	sentenceContext := context{
		norms:  make([]string, 0, len(words)+4),
		shapes: make([]string, len(words)),
	}
	sentenceContext.norms = append(sentenceContext.norms, padBefore[:]...)
	for i, word := range words {
		sentenceContext.norms = append(sentenceContext.norms, normalize(word))
		sentenceContext.shapes[i] = shape(word)
	}
	sentenceContext.norms = append(sentenceContext.norms, padAfter[:]...)
	return sentenceContext
}

// features appends the feature hashes for word i, given the two previously
// predicted tags.
func (c context) features(dst []uint64, i int, prev, prev2 byte) []uint64 {
	j := i + 2
	word := c.norms[j]
	initial := byte(0)
	if i == 0 {
		initial = 1
	}
	return append(dst,
		newHash(featBias).sum(),
		newHash(featSuffix1).str(suffix(word, 1)).sum(),
		newHash(featSuffix2).str(suffix(word, 2)).sum(),
		newHash(featSuffix3).str(suffix(word, 3)).sum(),
		newHash(featSuffix4).str(suffix(word, 4)).sum(),
		newHash(featPrefix1).str(prefix1(word)).sum(),
		newHash(featPrevTag).byte(prev).sum(),
		newHash(featPrev2Tag).byte(prev2).sum(),
		newHash(featPrevTags).byte(prev).byte(prev2).sum(),
		newHash(featWord).str(word).sum(),
		newHash(featPrevTagWord).byte(prev).str(word).sum(),
		newHash(featPrevWord).str(c.norms[j-1]).sum(),
		newHash(featPrevSuffix3).str(suffix(c.norms[j-1], 3)).sum(),
		newHash(featPrev2Word).str(c.norms[j-2]).sum(),
		newHash(featNextWord).str(c.norms[j+1]).sum(),
		newHash(featNextSuffix3).str(suffix(c.norms[j+1], 3)).sum(),
		newHash(featNext2Word).str(c.norms[j+2]).sum(),
		newHash(featShape).str(c.shapes[i]).sum(),
		newHash(featShapeInitial).str(c.shapes[i]).byte(initial).sum(),
		newHash(featPrevWordWord).str(c.norms[j-1]).str(word).sum(),
		newHash(featWordNextWord).str(word).str(c.norms[j+1]).sum(),
	)
}

// normalize lowercases a word, folds typographic apostrophes, and maps
// numbers to a few classes so that unseen numbers share features.
func normalize(word string) string {
	if r, _ := utf8.DecodeRuneInString(word); unicode.IsDigit(r) {
		if len(word) == 4 && strings.IndexFunc(word, func(r rune) bool { return !unicode.IsDigit(r) }) < 0 {
			return "!YEAR"
		}
		return "!DIGITS"
	}
	return strings.ReplaceAll(strings.ToLower(word), "\u2019", "'")
}

// shape maps a word to a compact character-class pattern: X upper, x lower,
// d digit, and the character itself otherwise, with repeats collapsed
// ("Paris" -> "Xx", "COVID-19" -> "X-d").
func shape(word string) string {
	var builder strings.Builder
	var last rune
	for _, r := range word {
		var class rune
		switch {
		case unicode.IsUpper(r):
			class = 'X'
		case unicode.IsLetter(r):
			class = 'x'
		case unicode.IsDigit(r):
			class = 'd'
		default:
			class = r
		}
		if class != last {
			builder.WriteRune(class)
			last = class
		}
	}
	return builder.String()
}

// suffix returns the last runeCount runes of word, or all of word if it is
// shorter.
func suffix(word string, runeCount int) string {
	i := len(word)
	for ; runeCount > 0 && i > 0; runeCount-- {
		_, size := utf8.DecodeLastRuneInString(word[:i])
		i -= size
	}
	return word[i:]
}

func prefix1(word string) string {
	_, size := utf8.DecodeRuneInString(word)
	return word[:size]
}

const (
	fnvOffset = 14695981039346656037
	fnvPrime  = 1099511628211
)

type hash uint64

func newHash(template byte) hash { return hash(fnvOffset).byte(template) }

func (h hash) byte(b byte) hash { return (h ^ hash(b)) * fnvPrime }

// str hashes text followed by a separator, so that ("ab","c") and ("a","bc")
// differ.
func (h hash) str(text string) hash {
	for i := range len(text) {
		h = h.byte(text[i])
	}

	return h.byte(0xff)
}

func (h hash) sum() uint64 { return uint64(h) }

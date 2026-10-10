// Package extractor finds a few short topical phrases that occur verbatim
// in plain text.
//
// Import a language package before calling Extract. A language that was not
// imported returns ErrUnsupportedLanguage.
//
// The pipeline is: UAX #29 sentence and word segmentation with original byte
// offsets, part-of-speech tagging to mark verbs, adverbs and non-nouns,
// language-pack-driven candidate generation (at most 3 words within a
// sentence, no verbs or adverbs, single words only if they are names,
// capitalized like a name, or rare common nouns), aggregation by normalized
// phrase, a deterministic
// weighted feature score, and greedy non-overlapping selection. Returned text
// is always sliced from the input, never rebuilt from tokens.
package extractor

import (
	"errors"
	"strings"
	"unicode"

	"github.com/tericcabrel/keyphrase/lang"
	"github.com/tericcabrel/keyphrase/rarity"
)

const (
	DefaultLimit = 5
)

var (
	ErrUnsupportedLanguage = errors.New("unsupported language")
	ErrInvalidLimit        = errors.New("limit must be at least 1")
)

// Keyword is one selected span. StartByte and EndByte are zero-based,
// half-open UTF-8 byte offsets, and Text == input[StartByte:EndByte].
// Score is a document-relative ranking signal in [0, 1]; it is not a
// probability and is not comparable across documents.
type Keyword struct {
	Text      string  `json:"text"`
	StartByte int     `json:"start_byte"`
	EndByte   int     `json:"end_byte"`
	Score     float64 `json:"score"`
}

// Counts is the count summary of one extraction. Words is the number of words
// in the submitted text. Extracted is every candidate phrase. Ranked is how
// many of those were returned after ranking and selection. The
// more-than-one-word fields count phrases of two or more words, including
// connectors ("wind power" is two). MoreThanOneWord lists every extracted
// phrase that meets that test, best-ranked first, so its length equals
// ExtractedMoreThanOneWord.
type Counts struct {
	Words                    int      `json:"words"`
	Extracted                int      `json:"extracted"`
	Ranked                   int      `json:"ranked"`
	ExtractedMoreThanOneWord int      `json:"extracted_more_than_one_word"`
	RankedMoreThanOneWord    int      `json:"ranked_more_than_one_word"`
	MoreThanOneWord          []string `json:"more_than_one_word"`
}

// Extract returns up to limit non-overlapping keyphrases for text.
// language is the code of a language package the program imports (lang/en,
// lang/fr, lang/de, lang/es, or lang/it). A code whose package was not
// imported returns ErrUnsupportedLanguage. limit must be at least 1. There
// is no maximum: a limit greater than the number of phrases returns all of
// them. The call uses DefaultWeights.
func Extract(text, language string, limit int) ([]Keyword, error) {
	keywords, _, err := extract(text, language, limit, DefaultWeights)
	return keywords, err
}

// ExtractWithCounts is Extract plus candidate and selection totals.
func ExtractWithCounts(text, language string, limit int) ([]Keyword, Counts, error) {
	return extract(text, language, limit, DefaultWeights)
}

// ExtractWithWeights is Extract with explicit ranking weights, for
// experiments and tests.
func ExtractWithWeights(text, language string, limit int, weights Weights) ([]Keyword, error) {
	keywords, _, err := extract(text, language, limit, weights)
	return keywords, err
}

func extract(text, language string, limit int, weights Weights) ([]Keyword, Counts, error) {
	pack, ok := lang.Get(language)
	if !ok {
		return nil, Counts{}, ErrUnsupportedLanguage
	}
	if limit < 1 {
		return nil, Counts{}, ErrInvalidLimit
	}
	if strings.TrimFunc(text, unicode.IsSpace) == "" {
		return []Keyword{}, Counts{MoreThanOneWord: []string{}}, nil
	}
	tokens, _ := tokenize(text, pack)
	markTags(text, tokens, pack.Tagger())
	// A positive LoneNounMinRarity keeps rare common nouns, so those candidates
	// have to survive until ranking, where the table is available.
	hardDrop := weights.DropLoneNouns && weights.LoneNounMinRarity == 0 && pack.Tagger() != nil
	phrases := generateCandidates(text, tokens, pack, hardDrop)
	var table *rarity.Table
	if weights.Rarity != 0 || weights.MinSingleRarity > 0 || (weights.DropLoneNouns && weights.LoneNounMinRarity > 0) {
		table = pack.Rarity()
	}
	phrases = rank(phrases, tokens, len(text), weights, pack, table)
	keywords, counts := selectPhrases(text, phrases, limit)
	counts.Words = countWords(tokens)
	return keywords, counts, nil
}

// countWords is the number of word tokens in the submitted text. Punctuation
// and whitespace are not words. A hyphenated compound such as "e-mail" is one.
func countWords(tokens []token) int {
	words := 0
	for i := range tokens {
		if tokens[i].kind == kindWord {
			words++
		}
	}
	return words
}

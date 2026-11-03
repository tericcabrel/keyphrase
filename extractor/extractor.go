// Package extractor finds a few short topical phrases that occur verbatim in plain text.
package extractor

import (
	"errors"
	"strings"
	"unicode"

	"github.com/tericcabrel/keyphrase/lang"
	"github.com/tericcabrel/keyphrase/rarity"
)

const (
	DefaultLimit = 10
)

var (
	ErrUnsupportedLanguage   = errors.New("unsupported language")
	ErrInvalidLimit          = errors.New("limit must be at least 1")
	ErrInvalidExtractOptions = errors.New("invalid extract options")
	ErrTooManyExtractOptions = errors.New("at most one extract options value")
)

type ExtractOptions struct {
	DropLoneNouns     bool
	LoneNounMinRarity float64
	DropOneOffPhrases bool
	OneOffMinOthers   int
}

var DefaultExtractOptions = ExtractOptions{
	DropLoneNouns:     true,
	LoneNounMinRarity: 0.6,
	DropOneOffPhrases: true,
	OneOffMinOthers:   10,
}

type ranking struct {
	Rarity          float64
	PhraseRarityMax bool
	MinSingleRarity float64
}

var defaultRanking = ranking{
	Rarity:          1,
	PhraseRarityMax: true,
}

// A NaN rarity fails because every comparison with NaN is false.
func (o ExtractOptions) valid() bool {
	return o.LoneNounMinRarity >= 0 && o.LoneNounMinRarity <= 1 && o.OneOffMinOthers >= 0
}

type Keyword struct {
	Text      string  `json:"text"`
	StartByte int     `json:"start_byte"`
	EndByte   int     `json:"end_byte"`
	Score     float64 `json:"score"`
}

type Counts struct {
	Words                    int      `json:"words"`
	Extracted                int      `json:"extracted"`
	Returned                 int      `json:"returned"`
	ExtractedMoreThanOneWord int      `json:"extracted_more_than_one_word"`
	ReturnedMoreThanOneWord  int      `json:"returned_more_than_one_word"`
	MoreThanOneWord          []string `json:"more_than_one_word"`
}

func Extract(text, language string, limit int, options ...ExtractOptions) ([]Keyword, Counts, error) {
	if err := validateExtract(language, limit); err != nil {
		return nil, Counts{}, err
	}
	if len(options) > 1 {
		return nil, Counts{}, ErrTooManyExtractOptions
	}
	chosen := DefaultExtractOptions
	if len(options) == 1 {
		chosen = options[0]
	}
	return extract(text, language, limit, chosen, defaultRanking)
}

func validateExtract(language string, limit int) error {
	if _, ok := lang.Get(language); !ok {
		return ErrUnsupportedLanguage
	}
	if limit < 1 {
		return ErrInvalidLimit
	}
	return nil
}

func extract(text, language string, limit int, options ExtractOptions, ranking ranking) ([]Keyword, Counts, error) {
	if !options.valid() {
		return nil, Counts{}, ErrInvalidExtractOptions
	}
	pack, ok := lang.Get(language)
	if !ok {
		return nil, Counts{}, ErrUnsupportedLanguage
	}
	if strings.TrimFunc(text, unicode.IsSpace) == "" {
		return []Keyword{}, Counts{MoreThanOneWord: []string{}}, nil
	}

	formula := weightsFor(options, ranking)
	tokens, _ := tokenize(text, pack)
	markTags(text, tokens, pack.Tagger())
	hardDrop := formula.dropLoneNouns && formula.loneNounMinRarity == 0 && pack.Tagger() != nil
	phrases := generateCandidates(text, tokens, pack, hardDrop)
	var table *rarity.Table

	if formula.rarity != 0 || formula.minSingleRarity > 0 || (formula.dropLoneNouns && formula.loneNounMinRarity > 0) {
		table = pack.Rarity()
	}

	phrases = rank(phrases, tokens, len(text), formula, pack, table)
	keywords, counts := selectPhrases(text, phrases, limit)
	counts.Words = countWords(tokens)

	return keywords, counts, nil
}

func countWords(tokens []token) int {
	words := 0
	for i := range tokens {
		if tokens[i].kind == kindWord {
			words++
		}
	}
	return words
}

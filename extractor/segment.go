package extractor

import (
	"strings"

	"github.com/tericcabrel/keyphrase/lang"
	"github.com/tericcabrel/keyphrase/postag"
)

type Span struct{ Start, End int }

func Segment(text, language string) ([][]Span, error) {
	pack, ok := lang.Get(language)
	if !ok {
		return nil, ErrUnsupportedLanguage
	}
	tokens, _ := tokenize(text, pack)
	var sentences [][]Span
	for _, sentence := range taggedSentences(tokens) {
		spans := make([]Span, len(sentence))
		for j, tokenIndex := range sentence {
			spans[j] = Span{tokens[tokenIndex].start, tokens[tokenIndex].end}
		}
		sentences = append(sentences, spans)
	}
	return sentences, nil
}

func Norms(text, language string) ([]string, error) {
	pack, ok := lang.Get(language)
	if !ok {
		return nil, ErrUnsupportedLanguage
	}
	tokens, _ := tokenize(text, pack)
	var norms []string
	for i := range tokens {
		if tokens[i].kind == kindWord {
			norms = append(norms, tokens[i].norm)
		}
	}
	return norms, nil
}

const minAdverbMargin = 15

func isHyphenated(word string) bool {
	return strings.ContainsAny(word, "-\u2010\u2011")
}

func markTags(text string, tokens []token, tagger *postag.Tagger) {
	if tagger == nil {
		return
	}
	type tally struct{ verb, adverb, nominal, name, total int }
	tallies := map[string]*tally{}
	var words []string
	for _, sentence := range taggedSentences(tokens) {
		words = words[:0]
		for _, tokenIndex := range sentence {
			words = append(words, text[tokens[tokenIndex].start:tokens[tokenIndex].end])
		}
		tags, margins := tagger.TagWithMargins(words)
		for j, tag := range tags {
			token := &tokens[sentence[j]]
			if token.kind != kindWord {
				continue
			}
			wordTally := tallies[token.norm]
			if wordTally == nil {
				wordTally = &tally{}
				tallies[token.norm] = wordTally
			}
			wordTally.total++
			switch {
			case tag.IsVerb():
				token.verb = true
				wordTally.verb++
			case tag == postag.ADV && margins[j] >= minAdverbMargin && !isHyphenated(words[j]):
				token.adverb = true
				wordTally.adverb++
			case tag.IsNominal():
				wordTally.nominal++
				if tag == postag.PROPN || tag == postag.X {
					wordTally.name++
				}
			}
		}
	}
	for i := range tokens {
		token := &tokens[i]
		if token.kind != kindWord {
			continue
		}
		wordTally := tallies[token.norm]
		token.verb = token.verb && 2*wordTally.verb >= wordTally.total
		token.adverb = token.adverb && 2*wordTally.adverb >= wordTally.total
		token.needsHead = 2*wordTally.nominal < wordTally.total
		token.name = 2*wordTally.name >= wordTally.total
	}
}

func taggedSentences(tokens []token) [][]int {
	var sentences [][]int
	var current []int
	for i := range tokens {
		if tokens[i].kind == kindSpace {
			continue
		}
		if len(current) > 0 && tokens[current[0]].sentence != tokens[i].sentence {
			sentences = append(sentences, current)
			current = nil
		}
		current = append(current, i)
	}
	if len(current) > 0 {
		sentences = append(sentences, current)
	}
	return sentences
}

package extractor

import (
	"strings"

	"github.com/tericcabrel/keyphrase/lang"
	"github.com/tericcabrel/keyphrase/postag"
)

// Span is a half-open byte range of the input.
type Span struct{ Start, End int }

// Segment splits text into sentences of non-space tokens (words and
// punctuation), exactly as the extractor does before part-of-speech tagging.
// The tagger trainer uses it so that it learns from the same kind of tokens
// the tagger sees in production.
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

// Norms returns the normalized word forms of text, in order, using the same
// tokenization as extraction. The rarity builder uses it so table keys match
// the forms the ranker looks up.
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

// minAdverbMargin is how far an ADV tag must lead the runner-up tag before
// the word is kept out of phrases. Unambiguous adverbs ("probably",
// "instead") come from the tag dictionary with an infinite margin, and most
// predicted ones ("intentionally", "increasingly", "rapidamente") lead by
// 15-50, while words the models mistake for adverbs lead by 1-13: "Offshore"
// in "Offshore wind", Italian "acque" and "ambiente". An adverb below the
// margin still can't stand alone as a keyword, because it is not a noun.
// Hyphenated compounds joined by joinHyphens are never taken as adverbs:
// the models rarely saw them in training, and the Italian one tags
// "multi-agente" in "sistema multi-agente" as an adverb with margins of
// 14-17.
const minAdverbMargin = 15

func isHyphenated(word string) bool {
	return strings.ContainsAny(word, "-\u2010\u2011")
}

// markTags tags each sentence with the pack's part-of-speech tagger and
// flags verbs, adverbs, and words that need a head. Punctuation and
// stopwords are tagged too, since they are context for their neighbours.
// Offsets are not touched.
//
// An occurrence keeps its verb or adverb flag only if the tagger gives the
// word that tag in at least half of its occurrences in the document. A word
// that is mostly tagged as something else is taken to be mis-tagged where it
// looks like a verb: the Italian model tags "alta" in "acqua alta" as a verb
// in 2 of 7 occurrences, which would otherwise split the phrase's count.
// Only confident adverb tags count (see minAdverbMargin). needsHead and
// name are decided for the word as a whole. needsHead is set on every
// occurrence when the tagger calls the word a noun or name in fewer than
// half of them. name is set when at least half the tags are a proper noun
// or unclassified (X); a common noun does not count.
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

// taggedSentences groups the indices of non-space tokens by sentence. These
// are the sequences the part-of-speech tagger labels.
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

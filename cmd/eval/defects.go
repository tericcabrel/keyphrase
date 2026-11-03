package main

import (
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tericcabrel/keyphrase/extractor"
	"github.com/tericcabrel/keyphrase/lang"
	"github.com/tericcabrel/keyphrase/postag"
)

const (
	checkFunctionWordEdge = "function-word-edge"
	checkVerb             = "verb"
	checkTruncated        = "truncated"
	checkFragment         = "fragment"
	checkHeadingWord      = "heading-word"
)

var defectChecks = [...]string{checkFunctionWordEdge, checkVerb, checkTruncated, checkFragment, checkHeadingWord}

const (
	// A selected phrase has at most 3 words, so the check looks that far on each side.
	fragmentReach   = 3
	maxHeadingWords = 8
)

type textToken struct {
	start, end int
	sentence   int
	word       bool // has a letter or digit, unlike punctuation
	form       string
	norm       string
}

func segmentTokens(text, language string) ([]textToken, error) {
	sentences, err := extractor.Segment(text, language)
	if err != nil {
		return nil, err
	}
	var tokens []textToken
	for sentenceIndex, spans := range sentences {
		for _, span := range spans {
			form := text[span.Start:span.End]
			tokens = append(tokens, textToken{
				start:    span.Start,
				end:      span.End,
				sentence: sentenceIndex,
				word:     strings.IndexFunc(form, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0,
				form:     form,
				norm:     lang.Normalize(form),
			})
		}
	}
	return tokens, nil
}

func findDefects(text string, tokens []textToken, keywords []extractor.Keyword, words lexicon) map[string][]int {
	defects := map[string][]int{}
	phraseWords := make([][]int, len(keywords))
	firstRank := map[string]int{}
	for rank, keyword := range keywords {
		phraseWords[rank] = wordTokensIn(tokens, keyword.StartByte, keyword.EndByte)
		key := normKey(tokens, phraseWords[rank])
		if _, seen := firstRank[key]; !seen {
			firstRank[key] = rank
		}
	}
	for rank, indices := range phraseWords {
		if len(indices) == 0 {
			continue
		}
		var flagged []string
		if isFragment(text, tokens, indices, rank, firstRank) {
			flagged = append(flagged, checkFragment)
		}
		if words.exact != nil {
			flagged = append(flagged, lexiconDefects(text, tokens, indices, words)...)
		}
		for _, check := range flagged {
			defects[check] = append(defects[check], rank)
		}
	}
	return defects
}

func lexiconDefects(text string, tokens []textToken, indices []int, words lexicon) []string {
	var flagged []string
	first, last := tokens[indices[0]], tokens[indices[len(indices)-1]]
	isFunction := func(token textToken) bool {
		return !isAcronym(token.form) && words.mostly(token.form, isFunctionTag)
	}
	if isFunction(first) || isFunction(last) {
		flagged = append(flagged, checkFunctionWordEdge)
	}
	if slices.ContainsFunc(indices, func(i int) bool {
		return !isAcronym(tokens[i].form) && words.mostly(tokens[i].form, postag.Tag.IsVerb)
	}) {
		flagged = append(flagged, checkVerb)
	}
	next := indices[len(indices)-1] + 1
	if next < len(tokens) && tokens[next].word && joined(text, tokens, next-1, next) &&
		words.mostly(last.form, isAdjectiveTag) && words.mostly(tokens[next].form, isNounTag) {
		flagged = append(flagged, checkTruncated)
	}
	if len(indices) == 1 && words.mostly(first.form, isCommonTag) && onlyInHeadings(text, tokens, first.norm) {
		flagged = append(flagged, checkHeadingWord)
	}
	return flagged
}

func isFragment(text string, tokens []textToken, indices []int, rank int, firstRank map[string]int) bool {
	from, to := indices[0], indices[len(indices)-1]+1
	for reach := 0; reach < fragmentReach && from > 0 && tokens[from-1].word && joined(text, tokens, from-1, from); reach++ {
		from--
	}
	for reach := 0; reach < fragmentReach && to < len(tokens) && tokens[to].word && joined(text, tokens, to-1, to); reach++ {
		to++
	}
	phraseStart, phraseEnd := indices[0], indices[len(indices)-1]+1
	window := make([]int, 0, to-from)
	for i := from; i < to; i++ {
		window = append(window, i)
	}
	for a := range window {
		for b := a + 1; b <= min(a+fragmentReach, len(window)); b++ {
			start, end := window[a], window[b-1]+1
			overlaps := start < phraseEnd && end > phraseStart
			contains := start <= phraseStart && end >= phraseEnd
			inside := start >= phraseStart && end <= phraseEnd
			touches := (end == phraseStart && !twoNames(tokens[end-1], tokens[phraseStart])) ||
				(start == phraseEnd && !twoNames(tokens[phraseEnd-1], tokens[start]))
			partial := overlaps && !contains && !inside
			if !partial && !touches {
				continue
			}
			if other, ok := firstRank[normKey(tokens, window[a:b])]; ok && other < rank {
				return true
			}
		}
	}
	return false
}

// A capitalized word on each side is a list of names, so the touch is not a fragment.
func twoNames(left, right textToken) bool {
	return startsUpper(left.form) && startsUpper(right.form)
}

func startsUpper(word string) bool {
	first, _ := utf8.DecodeRuneInString(word)
	return unicode.IsUpper(first)
}

func joined(text string, tokens []textToken, a, b int) bool {
	if tokens[a].sentence != tokens[b].sentence {
		return false
	}
	return strings.Trim(text[tokens[a].end:tokens[b].start], " \t") == ""
}

func onlyInHeadings(text string, tokens []textToken, norm string) bool {
	for _, token := range tokens {
		if token.norm != norm {
			continue
		}
		lineStart := strings.LastIndexByte(text[:token.start], '\n') + 1
		lineEnd := strings.IndexByte(text[token.end:], '\n')
		if lineEnd < 0 {
			lineEnd = len(text)
		} else {
			lineEnd += token.end
		}
		line := strings.TrimSpace(text[lineStart:lineEnd])
		lastRune, _ := utf8.DecodeLastRuneInString(line)
		if len(strings.Fields(line)) > maxHeadingWords || strings.ContainsRune(".!?;:…", lastRune) {
			return false
		}
	}
	return true
}

func isCommonTag(tag postag.Tag) bool { return tag == postag.NOUN || tag == postag.ADJ }

// "IT" is not the pronoun "it".
func isAcronym(word string) bool {
	return len(word) >= 2 && strings.IndexFunc(word, unicode.IsLower) < 0
}

func wordTokensIn(tokens []textToken, start, end int) []int {
	first := sort.Search(len(tokens), func(i int) bool { return tokens[i].start >= start })
	var indices []int
	for i := first; i < len(tokens) && tokens[i].end <= end; i++ {
		if tokens[i].word {
			indices = append(indices, i)
		}
	}
	return indices
}

func normKey(tokens []textToken, indices []int) string {
	norms := make([]string, len(indices))
	for i, index := range indices {
		norms[i] = tokens[index].norm
	}
	return strings.Join(norms, " ")
}

package extractor

import (
	"cmp"
	"math"
	"slices"
	"strings"
)

const genericShare = 0.25

type chosen struct {
	phrase     *phrase
	occurrence occurrence
}

func selectPhrases(text string, phrases []*phrase, limit int) ([]Keyword, Counts) {
	ranked := slices.Clone(phrases)
	slices.SortFunc(ranked, func(a, b *phrase) int {
		return cmp.Or(
			cmp.Compare(b.rawScore, a.rawScore),
			cmp.Compare(a.occurrences[0].start, b.occurrences[0].start),
			strings.Compare(a.key, b.key),
		)
	})
	counts := Counts{Extracted: len(phrases), MoreThanOneWord: []string{}}
	for _, phrase := range ranked {
		if phraseWords(phrase) <= 1 {
			continue
		}
		counts.ExtractedMoreThanOneWord++
		firstOccurrence := phrase.occurrences[0]
		counts.MoreThanOneWord = append(counts.MoreThanOneWord, text[firstOccurrence.start:firstOccurrence.end])
	}
	if len(ranked) == 0 {
		return []Keyword{}, counts
	}

	mostSpecific := map[string]int{}
	for _, phrase := range ranked {
		if !phrase.multiword() || len(phrase.occurrences) < 2 {
			continue
		}
		for _, variantKey := range phrase.variantKeys {
			mostSpecific[variantKey] = max(mostSpecific[variantKey], len(phrase.occurrences))
		}
	}

	var picked []chosen
	usedVariants := map[string]struct{}{}
	for _, phrase := range ranked {
		if len(picked) == limit {
			break
		}
		if phrase.subsumed() {
			continue
		}
		if _, dup := usedVariants[phrase.variantKey]; dup {
			continue
		}
		if !phrase.multiword() && float64(mostSpecific[phrase.variantKeys[0]]) >= genericShare*float64(len(phrase.occurrences)) {
			continue
		}
		if slices.ContainsFunc(picked, func(choice chosen) bool {
			return containsSeq(choice.phrase.variantKeys, phrase.variantKeys) || containsSeq(phrase.variantKeys, choice.phrase.variantKeys)
		}) {
			continue
		}
		free, ok := firstFree(phrase, picked)
		if !ok {
			continue
		}
		picked = append(picked, chosen{phrase, free})
		usedVariants[phrase.variantKey] = struct{}{}
	}

	if len(picked) == 0 {
		return []Keyword{}, counts
	}
	topScore := picked[0].phrase.rawScore
	keywords := make([]Keyword, 0, len(picked))
	for _, choice := range picked {
		keywords = append(keywords, Keyword{
			Text:      text[choice.occurrence.start:choice.occurrence.end],
			StartByte: choice.occurrence.start,
			EndByte:   choice.occurrence.end,
			Score:     normalizeScore(choice.phrase.rawScore, topScore),
		})
		if phraseWords(choice.phrase) > 1 {
			counts.ReturnedMoreThanOneWord++
		}
	}
	counts.Returned = len(keywords)
	slices.SortFunc(keywords, compareKeywords)
	return keywords, counts
}

func phraseWords(phrase *phrase) int {
	if phrase.key == "" {
		return 0
	}
	return strings.Count(phrase.key, " ") + 1
}

func compareKeywords(a, b Keyword) int {
	return cmp.Or(
		cmp.Compare(b.Score, a.Score),
		cmp.Compare(a.StartByte, b.StartByte),
		strings.Compare(a.Text, b.Text),
	)
}

func normalizeScore(rawScore, topScore float64) float64 {
	if topScore <= 0 {
		return 0
	}
	return math.Round(rawScore/topScore*1e4) / 1e4
}

func firstFree(phrase *phrase, picked []chosen) (occurrence, bool) {
	for _, match := range phrase.occurrences {
		if !slices.ContainsFunc(picked, func(choice chosen) bool {
			return match.start < choice.occurrence.end && choice.occurrence.start < match.end
		}) {
			return match, true
		}
	}
	return occurrence{}, false
}

func containsSeq(haystack, needle []string) bool {
	if len(needle) == 0 || len(needle) > len(haystack) {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if slices.Equal(haystack[i:i+len(needle)], needle) {
			return true
		}
	}
	return false
}

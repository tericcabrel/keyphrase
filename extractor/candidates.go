package extractor

import (
	"strings"
	"unicode/utf8"

	"github.com/tericcabrel/keyphrase/lang"
)

const (
	maxWords                 = 3
	maxConsecutiveConnectors = 2
)

type occurrence struct {
	start, end  int
	first, last int
	sentence    int
	capitalized bool
}

type phrase struct {
	key          string
	contentNorms []string
	variantKeys  []string
	variantKey   string
	occurrences  []occurrence
	sentences    int
	subsumers    []*phrase
	rawScore     float64
}

func (p *phrase) subsumed() bool {
	for _, subsumer := range p.subsumers {
		if subsumer.rawScore >= p.rawScore {
			return true
		}
	}
	return false
}

func (p *phrase) multiword() bool { return len(p.contentNorms) > 1 }

type classifier struct {
	text          string
	pack          lang.Pack
	dropLoneNouns bool
}

func (c classifier) isContent(token *token) bool {
	return token.kind == kindWord && !token.verb && !token.adverb && !c.pack.IsStopword(token.norm) &&
		!isMachineToken(c.text[token.start:token.end])
}

func (c classifier) canStart(token *token) bool {
	if !c.isContent(token) || token.number || token.letters == 0 || strings.ContainsRune(c.text[token.start:token.end], utf8.RuneError) {
		return false
	}
	return token.letters >= 3 || (token.letters == 2 && token.allCaps)
}

func (c classifier) canEnd(token *token) bool {
	return c.canStart(token) || (c.isContent(token) && token.number)
}

func generateCandidates(text string, tokens []token, pack lang.Pack, dropLoneNouns bool) []*phrase {
	boundary := classifier{text: text, pack: pack, dropLoneNouns: dropLoneNouns}
	byKey := map[string]*phrase{}
	var ordered []*phrase

	var run []int
	flush := func() {
		collectRun(boundary, tokens, run, byKey, &ordered)
		run = run[:0]
	}
	for i := range tokens {
		token := &tokens[i]
		switch token.kind {
		case kindWord:
			if len(run) > 0 && tokens[run[len(run)-1]].sentence != token.sentence {
				flush()
			}
			run = append(run, i)
		case kindSpace:
			if token.newline {
				flush()
			}
		case kindPunct:
			flush()
		}
	}
	flush()

	for _, phrase := range ordered {
		phrase.contentNorms, phrase.variantKeys = contentOf(boundary, tokens, phrase.occurrences[0])
		phrase.variantKey = strings.Join(phrase.variantKeys, " ")
		seen := map[int]struct{}{}
		for _, occurrence := range phrase.occurrences {
			seen[occurrence.sentence] = struct{}{}
		}
		phrase.sentences = len(seen)
	}
	markSubsumed(tokens, ordered, byKey)
	return ordered
}

func markSubsumed(tokens []token, phrases []*phrase, byKey map[string]*phrase) {
	var words []string
	for _, phrase := range phrases {
		firstOccurrence := phrase.occurrences[0]
		if len(phrase.occurrences) < 2 && !firstOccurrence.capitalized {
			continue
		}
		words = words[:0]
		for i := firstOccurrence.first; i <= firstOccurrence.last; i++ {
			if tokens[i].kind == kindWord {
				words = append(words, tokens[i].norm)
			}
		}
		if len(words) < 2 {
			continue
		}
		for start := range words {
			for end := start + 1; end <= len(words); end++ {
				if end-start == len(words) {
					continue
				}
				subphrase, ok := byKey[strings.Join(words[start:end], " ")]
				if ok && len(subphrase.occurrences) == len(phrase.occurrences) {
					subphrase.subsumers = append(subphrase.subsumers, phrase)
				}
			}
		}
	}
}

func collectRun(boundary classifier, tokens []token, run []int, byKey map[string]*phrase, ordered *[]*phrase) {
	var key strings.Builder
	for start := range run {
		first := &tokens[run[start]]
		if !boundary.canStart(first) && !boundary.opensName(tokens, run, start) {
			continue
		}
		connectors := 0
		key.Reset()
		for end := start; end < len(run); end++ {
			if end-start+1 > maxWords {
				break
			}
			token := &tokens[run[end]]
			if boundary.isContent(token) {
				connectors = 0
			} else if boundary.pack.IsConnector(token.norm) {
				connectors++
				if connectors > maxConsecutiveConnectors {
					break
				}
			} else {
				break
			}
			if end > start {
				key.WriteByte(' ')
			}
			key.WriteString(token.norm)
			if !boundary.canEnd(token) || boundary.truncatesName(tokens, run, start, end) {
				continue
			}
			match := occurrence{
				start: first.start, end: token.end,
				first: run[start], last: run[end],
				sentence:    first.sentence,
				capitalized: isCapitalized(boundary, tokens, run[start], run[end]),
			}
			if start == end && !match.capitalized && (token.needsHead || (boundary.dropLoneNouns && !token.name)) {
				continue
			}
			phraseKey := key.String()
			candidate, ok := byKey[phraseKey]
			if !ok {
				candidate = &phrase{key: phraseKey}
				byKey[phraseKey] = candidate
				*ordered = append(*ordered, candidate)
			}
			candidate.occurrences = append(candidate.occurrences, match)
		}
	}
}

func (c classifier) opensName(tokens []token, run []int, start int) bool {
	token := &tokens[run[start]]
	if !token.upper || token.sentenceInitial || !c.pack.IsConnector(token.norm) || start+1 >= len(run) {
		return false
	}
	next := &tokens[run[start+1]]
	return next.upper && c.canStart(next)
}

func (c classifier) truncatesName(tokens []token, run []int, start, end int) bool {
	if c.pack.CapitalizesNouns() {
		return false
	}
	capitalContent := func(t *token) bool { return t.upper && c.isContent(t) && !t.number }
	if start > 0 && tokens[run[start]].upper && capitalContent(&tokens[run[start-1]]) {
		return true
	}
	return end+1 < len(run) && tokens[run[end]].upper && capitalContent(&tokens[run[end+1]])
}

func isCapitalized(boundary classifier, tokens []token, first, last int) bool {
	if first == last && boundary.pack.CapitalizesNouns() {
		return tokens[first].allCaps
	}
	contentWords := 0
	for i := first; i <= last; i++ {
		token := &tokens[i]
		if token.kind != kindWord || !boundary.isContent(token) || token.number {
			continue
		}
		if !token.upper {
			return false
		}
		contentWords++
	}
	if contentWords == 0 {
		return false
	}
	return contentWords > 1 || !tokens[first].sentenceInitial
}

func contentOf(boundary classifier, tokens []token, occurrence occurrence) (norms, variants []string) {
	for i := occurrence.first; i <= occurrence.last; i++ {
		token := &tokens[i]
		if token.kind == kindWord && boundary.isContent(token) {
			norms = append(norms, token.norm)
			variants = append(variants, boundary.pack.VariantKey(token.norm))
		}
	}
	return norms, variants
}

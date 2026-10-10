package extractor

import (
	"strings"
	"unicode/utf8"

	"github.com/tericcabrel/keyphrase/lang"
)

const (
	// maxWords is the phrase length limit, counting connectors. "tassi di
	// interesse" is three words; "Dogger Bank Wind Farm" is four and is not
	// a candidate.
	maxWords = 3
	// maxConsecutiveConnectors stops runs such as "de la de". A phrase must
	// also start and end on a content word, so two connectors do not fit
	// inside the three-word limit.
	maxConsecutiveConnectors = 2
)

// occurrence is one source span of a candidate phrase. first and last are
// indices into the token slice.
type occurrence struct {
	start, end  int
	first, last int
	sentence    int
	capitalized bool
}

// phrase aggregates all occurrences of one normalized candidate.
type phrase struct {
	key          string   // token norms joined by single spaces
	contentNorms []string // non-connector words, in order
	variantKeys  []string // pack.VariantKey of contentNorms
	variantKey   string
	occurrences  []occurrence
	sentences    int // distinct sentences containing the phrase
	// subsumers are longer candidates that contain this phrase and occur
	// equally often, so this phrase never appears on its own ("Central Bank"
	// inside "European Central Bank").
	subsumers []*phrase
	rawScore  float64
}

// subsumed reports whether a subsumer ranks at least as high, in which case
// the longer phrase is the one to keep. A lower-ranked subsumer such as
// "help meet the Paris agreement" does not suppress "Paris agreement".
func (p *phrase) subsumed() bool {
	for _, subsumer := range p.subsumers {
		if subsumer.rawScore >= p.rawScore {
			return true
		}
	}
	return false
}

func (p *phrase) multiword() bool { return len(p.contentNorms) > 1 }

// classifier makes per-token boundary decisions for one document.
type classifier struct {
	text          string
	pack          lang.Pack
	dropLoneNouns bool
}

// isContent reports whether a word token is neither a stopword, a verb, an
// adverb, nor a machine-shaped token (an underscore identifier or a source
// filename). Those tokens end a phrase like any other non-connector stopword.
func (c classifier) isContent(token *token) bool {
	return token.kind == kindWord && !token.verb && !token.adverb && !c.pack.IsStopword(token.norm) &&
		!isMachineToken(c.text[token.start:token.end])
}

// canStart reports whether a token may open a phrase: a content word with at
// least one letter and a reasonable length. Two-letter words qualify only as
// acronyms ("EU", "UE").
func (c classifier) canStart(token *token) bool {
	if !c.isContent(token) || token.number || token.letters == 0 || strings.ContainsRune(c.text[token.start:token.end], utf8.RuneError) {
		return false
	}
	return token.letters >= 3 || (token.letters == 2 && token.allCaps)
}

// canEnd additionally allows numbers ("Windows 11", "COVID-19" is a word).
func (c classifier) canEnd(token *token) bool {
	return c.canStart(token) || (c.isContent(token) && token.number)
}

// generateCandidates enumerates contiguous phrases within a sentence that
// start and end on valid boundary words, contain only content words and
// connectors, and are separated only by same-line whitespace.
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

// markSubsumed links every candidate that is a contiguous sub-phrase of a
// longer candidate with the same occurrence count, provided the longer phrase
// is repeated or is a capitalized name. A longer phrase seen once is not
// otherwise enough evidence: it would let one-off phrases such as
// "Parlamento Europeo aprobó" absorb the name inside them, while "Santa Cruz
// de Tenerife" should absorb "Santa Cruz" and "Tenerife".
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

// collectRun adds every valid candidate inside a run of word tokens that are
// separated only by whitespace.
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

// opensName reports whether a capitalized connector in mid-sentence starts a
// name, as the article does in "La Rochelle", "Los Angeles" or "Il Sole".
func (c classifier) opensName(tokens []token, run []int, start int) bool {
	token := &tokens[run[start]]
	if !token.upper || token.sentenceInitial || !c.pack.IsConnector(token.norm) || start+1 >= len(run) {
		return false
	}
	next := &tokens[run[start+1]]
	return next.upper && c.canStart(next)
}

// truncatesName reports whether run[start..end] cuts through a capitalized name:
// it starts or ends on a capitalized word that is directly adjacent to
// another capitalized content word ("Security Agency" inside "UK Health
// Security Agency"). Languages that capitalize all nouns skip this rule,
// since adjacent capitalized words there are usually separate nouns.
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

// isCapitalized reports whether an occurrence looks like a proper name:
// every content word starts uppercase, and the capital is not explained by
// sentence position alone (a lone sentence-initial word does not count).
// In languages that capitalize all nouns, a lone word counts only if it is
// an acronym.
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

package extractor

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rivo/uniseg"

	"github.com/tericcabrel/keyphrase/lang"
)

type tokenKind uint8

const (
	kindWord tokenKind = iota
	kindSpace
	kindPunct
)

type token struct {
	start, end      int
	kind            tokenKind
	norm            string
	sentence        int
	upper, allCaps  bool
	sentenceInitial bool
	number          bool
	letters         int
	newline         bool
	verb            bool
	adverb          bool
	needsHead       bool
	name            bool
}

type span struct{ start, end int }

func tokenize(text string, pack lang.Pack) (tokens []token, sentences int) {
	for i, sentenceSpan := range segmentSentences(text, pack) {
		sentenceTokens := segmentWords(text, sentenceSpan, i)
		sentenceTokens = joinHyphens(text, sentenceTokens)
		sentenceTokens = extendDottedAbbreviations(text, sentenceTokens)
		sentenceTokens = splitClitics(text, sentenceTokens, pack)
		for j := range sentenceTokens {
			if sentenceTokens[j].kind == kindWord {
				sentenceTokens[j].sentenceInitial = true

				break
			}
		}
		tokens = append(tokens, sentenceTokens...)
		sentences = i + 1
	}
	return tokens, sentences
}

func segmentSentences(text string, pack lang.Pack) []span {
	var rawSpans []span
	state := -1
	offset := 0
	for rest := text; len(rest) > 0; {
		var sentence string
		sentence, rest, state = uniseg.FirstSentenceInString(rest, state)
		rawSpans = append(rawSpans, span{offset, offset + len(sentence)})
		offset += len(sentence)
	}
	var sentences []span
	for _, sentenceSpan := range rawSpans {
		if n := len(sentences); n > 0 && mergeWithNext(text[sentences[n-1].start:sentences[n-1].end], pack) {
			sentences[n-1].end = sentenceSpan.end
			continue
		}
		sentences = append(sentences, sentenceSpan)
	}
	return sentences
}

func mergeWithNext(sentence string, pack lang.Pack) bool {
	trimmed := strings.TrimRightFunc(sentence, unicode.IsSpace)
	body, ok := strings.CutSuffix(trimmed, ".")
	if !ok {
		return false
	}
	start := len(body)
	for start > 0 {
		r, size := utf8.DecodeLastRuneInString(body[:start])
		if !unicode.IsLetter(r) && r != '.' {
			break
		}
		start -= size
	}
	word := body[start:]
	if word == "" {
		return false
	}
	if utf8.RuneCountInString(word) == 1 || strings.Contains(word, ".") {
		return true
	}
	return pack.IsAbbreviation(lang.Normalize(word))
}

func segmentWords(text string, sentenceSpan span, sentence int) []token {
	var tokens []token
	state := -1
	offset := sentenceSpan.start
	for rest := text[sentenceSpan.start:sentenceSpan.end]; len(rest) > 0; {
		var segment string
		segment, rest, state = uniseg.FirstWordInString(rest, state)
		tokens = append(tokens, newToken(text, offset, offset+len(segment), sentence))
		offset += len(segment)
	}
	return tokens
}

func newToken(text string, start, end, sentence int) token {
	result := token{start: start, end: end, sentence: sentence, kind: kindPunct}
	segment := text[start:end]
	var hasLetter, hasDigit, allSpace = false, false, true
	upperLetters := 0
	for i, r := range segment {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
			result.letters++
			if unicode.IsUpper(r) {
				upperLetters++
				if i == 0 {
					result.upper = true
				}
			}
		case unicode.IsDigit(r):
			hasDigit = true
		}
		if !unicode.IsSpace(r) {
			allSpace = false
		} else if r == '\n' || r == '\r' || r == '\u2028' || r == '\u2029' {
			result.newline = true
		}
	}
	switch {
	case allSpace:
		result.kind = kindSpace
	case hasLetter || hasDigit:
		result.kind = kindWord
		result.norm = lang.Normalize(segment)
		result.number = hasDigit && !hasLetter
		result.allCaps = result.letters >= 2 && upperLetters == result.letters
	}
	return result
}

func isMachineToken(segment string) bool {
	if strings.Contains(segment, "_") {
		return true
	}
	trimmed := strings.TrimRight(segment, ".")
	dot := strings.LastIndex(trimmed, ".")
	if dot <= 0 || dot == len(trimmed)-1 {
		return false
	}
	extension := trimmed[dot+1:]
	if !isSourceExt(extension) {
		return false
	}
	stem := trimmed[:dot]
	if strings.EqualFold(extension, "js") && containsUpper(stem) {
		return false
	}
	for _, r := range stem {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.' && r != '-' {
			return false
		}
	}
	return true
}

func containsUpper(text string) bool {
	for _, r := range text {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

func isSourceExt(extension string) bool {
	switch strings.ToLower(extension) {
	case "ts", "tsx", "js", "jsx", "mjs", "cjs",
		"py", "go", "rs", "java", "kt", "rb",
		"php", "css", "scss", "html", "json",
		"yaml", "yml", "toml", "md", "sh", "sql",
		"vue", "svelte":
		return true
	default:
		return false
	}
}

func isHyphen(text string) bool {
	return text == "-" || text == "\u2010" || text == "\u2011"
}

func joinHyphens(text string, tokens []token) []token {
	joined := tokens[:0]
	for i := 0; i < len(tokens); i++ {
		current := tokens[i]
		for current.kind == kindWord && i+2 < len(tokens) &&
			tokens[i+1].kind == kindPunct && isHyphen(text[tokens[i+1].start:tokens[i+1].end]) &&
			tokens[i+2].kind == kindWord {
			current = mergeTokens(text, current, tokens[i+2])
			i += 2
		}
		joined = append(joined, current)
	}
	return joined
}

func extendDottedAbbreviations(text string, tokens []token) []token {
	extended := tokens[:0]
	for i := 0; i < len(tokens); i++ {
		current := tokens[i]
		if current.kind == kindWord && strings.Contains(text[current.start:current.end], ".") &&
			i+1 < len(tokens) && text[tokens[i+1].start:tokens[i+1].end] == "." {
			current = mergeTokens(text, current, tokens[i+1])
			i++
		}
		extended = append(extended, current)
	}
	return extended
}

func mergeTokens(text string, left, right token) token {
	merged := newToken(text, left.start, right.end, left.sentence)
	merged.upper = left.upper
	return merged
}

func splitClitics(text string, tokens []token, pack lang.Pack) []token {
	elisions, clitics := pack.ElisionPrefixes(), pack.CliticSuffixes()
	if len(elisions) == 0 && len(clitics) == 0 {
		return tokens
	}
	split := make([]token, 0, len(tokens))
	for _, current := range tokens {
		if current.kind != kindWord {
			split = append(split, current)
			continue
		}
		word := text[current.start:current.end]
		if cut := elisionCut(word, elisions); cut > 0 {
			split = append(split, newToken(text, current.start, current.start+cut, current.sentence))
			current = newToken(text, current.start+cut, current.end, current.sentence)
			word = text[current.start:current.end]
		}
		if cut := cliticCut(word, clitics); cut > 0 {
			split = append(split,
				newToken(text, current.start, current.start+cut, current.sentence),
				newToken(text, current.start+cut, current.end, current.sentence))
			continue
		}
		split = append(split, current)
	}
	return split
}

func apostropheAt(text string, i int) int {
	switch {
	case strings.HasPrefix(text[i:], "'"):
		return 1
	case strings.HasPrefix(text[i:], "\u2019"):
		return len("\u2019")
	}
	return 0
}

func elisionCut(word string, prefixes []string) int {
	for i := range len(word) {
		width := apostropheAt(word, i)
		if width == 0 {
			continue
		}
		cut := i + width
		if cut >= len(word) {
			return 0
		}
		if r, _ := utf8.DecodeRuneInString(word[cut:]); !unicode.IsLetter(r) {
			return 0
		}
		norm := lang.Normalize(word[:cut])
		if slices.Contains(prefixes, norm) {
			return cut
		}
		return 0
	}
	return 0
}

func cliticCut(word string, suffixes []string) int {
	for i := len(word) - 1; i > 0; i-- {
		if apostropheAt(word, i) == 0 {
			continue
		}
		norm := lang.Normalize(word[i:])
		if slices.Contains(suffixes, norm) {
			return i
		}
		return 0
	}
	return 0
}

// Package lang is the registry of language packs that the extractor looks up.
//
// Built-in languages are separate packages: lang/en, lang/fr, lang/de,
// lang/es, lang/it, and lang/pt. A program imports the ones it needs, and
// only those embeds are linked. lang/all imports every built-in language.
//
// Another language implements Pack and calls Register from its init function.
// New builds the usual pack from stopword lists.
package lang

import (
	"bytes"
	"cmp"
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/tericcabrel/keyphrase/postag"
	"github.com/tericcabrel/keyphrase/rarity"
)

// Pack is the language-specific behaviour the extraction core relies on. All
// string arguments are normalized forms: lowercase, with typographic
// apostrophes (U+2019) folded to ASCII '.
type Pack interface {
	// Code is the lowercase ISO 639-1 code, e.g. "en".
	Code() string
	// IsStopword reports whether a word may not start or end a phrase.
	IsStopword(norm string) bool
	// IsConnector reports whether a stopword may appear inside a phrase,
	// e.g. French "de" in "ministère de la Santé".
	IsConnector(norm string) bool
	// ElisionPrefixes lists prefixes such as "l'" that are split off the
	// front of a word so that the phrase can start at the content word.
	// Longest prefixes come first.
	ElisionPrefixes() []string
	// CliticSuffixes lists suffixes such as English "'s" that are split off
	// the end of a word. Longest suffixes come first.
	CliticSuffixes() []string
	// IsAbbreviation reports whether a word followed by "." should not end
	// a sentence, e.g. "dr" in "Dr. Smith". The word is given without its
	// trailing period.
	IsAbbreviation(norm string) bool
	// CapitalizesNouns reports whether ordinary nouns are capitalized (as in
	// German), which makes capitalization a weak signal for proper names.
	CapitalizesNouns() bool
	// VariantKey folds simple inflectional variants (mostly plurals) of a
	// content word to a shared key. It is used only to detect near
	// duplicates, never to produce output text.
	VariantKey(norm string) string
	// Tagger returns the part-of-speech tagger used to keep verbs out of
	// phrases, or nil if the language has no model.
	Tagger() *postag.Tagger
	// Rarity returns the corpus rarity table used as a ranking feature,
	// or nil if the language has no table.
	Rarity() *rarity.Table
}

// Spec is the data New turns into a Pack.
type Spec struct {
	Code string
	// Lists are stopword files: whitespace-separated words, and lines
	// starting with # are comments.
	Lists         [][]byte
	Connectors    string
	Abbreviations string
	Elisions      string
	Clitics       string
	NounCaps      bool
	Variant       func(string) string
	// Tagger and Rarity are called every time those methods run. LazyTagger
	// and LazyTable decode an embedded file on the first call.
	Tagger func() *postag.Tagger
	Rarity func() *rarity.Table
}

var (
	registryMu sync.RWMutex
	registry   = map[string]Pack{}
)

// Register adds a pack. It panics on duplicate codes because registration
// happens at init time and a duplicate is a programming error.
func Register(pack Pack) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := registry[pack.Code()]; dup {
		panic(fmt.Sprintf("lang: duplicate pack %q", pack.Code()))
	}
	registry[pack.Code()] = pack
}

// Get returns the pack for an exact lowercase code.
func Get(code string) (Pack, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	pack, ok := registry[code]
	return pack, ok
}

// Codes returns the registered language codes in sorted order.
func Codes() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return slices.Sorted(maps.Keys(registry))
}

// New builds a pack and reads its stopword lists immediately.
func New(spec Spec) Pack {
	pack := &dataPack{
		code:          spec.Code,
		stopwords:     map[string]struct{}{},
		connectors:    wordSet(spec.Connectors),
		abbreviations: wordSet(spec.Abbreviations),
		elisions:      longestFirst(spec.Elisions),
		clitics:       longestFirst(spec.Clitics),
		nounCaps:      spec.NounCaps,
		variant:       spec.Variant,
		tagger:        spec.Tagger,
		rarity:        spec.Rarity,
	}
	for _, raw := range spec.Lists {
		for line := range strings.Lines(string(raw)) {
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				continue
			}
			for word := range strings.FieldsSeq(line) {
				pack.stopwords[Normalize(word)] = struct{}{}
			}
		}
	}
	for word := range pack.connectors {
		pack.stopwords[word] = struct{}{}
	}
	for word := range pack.abbreviations {
		pack.stopwords[word] = struct{}{}
	}
	for _, prefix := range pack.elisions {
		pack.stopwords[prefix] = struct{}{}
	}
	for _, suffix := range pack.clitics {
		pack.stopwords[suffix] = struct{}{}
	}
	return pack
}

// MustRead reads name from fsys. A missing file is a build error, so it panics.
func MustRead(fsys fs.FS, name string) []byte {
	raw, err := fs.ReadFile(fsys, name)
	if err != nil {
		panic(fmt.Sprintf("lang: %s: %v", name, err))
	}
	return raw
}

// LazyTagger returns a function that decodes name on its first call.
func LazyTagger(fsys fs.FS, name string) func() *postag.Tagger {
	return sync.OnceValue(func() *postag.Tagger {
		tagger, err := postag.ReadTagger(bytes.NewReader(MustRead(fsys, name)))
		if err != nil {
			panic(fmt.Sprintf("lang: %s: %v", name, err))
		}
		return tagger
	})
}

// LazyTable returns a function that decodes name on its first call.
func LazyTable(fsys fs.FS, name string) func() *rarity.Table {
	return sync.OnceValue(func() *rarity.Table {
		table, err := rarity.Read(bytes.NewReader(MustRead(fsys, name)))
		if err != nil {
			panic(fmt.Sprintf("lang: %s: %v", name, err))
		}
		return table
	})
}

// dataPack is the table-driven Pack implementation shared by the built-in
// languages.
type dataPack struct {
	code          string
	stopwords     map[string]struct{}
	connectors    map[string]struct{}
	abbreviations map[string]struct{}
	elisions      []string
	clitics       []string
	nounCaps      bool
	variant       func(string) string
	tagger        func() *postag.Tagger
	rarity        func() *rarity.Table
}

func (p *dataPack) Code() string              { return p.code }
func (p *dataPack) ElisionPrefixes() []string { return p.elisions }
func (p *dataPack) CliticSuffixes() []string  { return p.clitics }
func (p *dataPack) CapitalizesNouns() bool    { return p.nounCaps }

func (p *dataPack) Tagger() *postag.Tagger {
	if p.tagger == nil {
		return nil
	}
	return p.tagger()
}

func (p *dataPack) Rarity() *rarity.Table {
	if p.rarity == nil {
		return nil
	}
	return p.rarity()
}

func (p *dataPack) IsStopword(norm string) bool {
	_, ok := p.stopwords[norm]
	return ok
}

func (p *dataPack) IsConnector(norm string) bool {
	_, ok := p.connectors[norm]
	return ok
}

func (p *dataPack) IsAbbreviation(norm string) bool {
	_, ok := p.abbreviations[norm]
	return ok
}

func (p *dataPack) VariantKey(norm string) string {
	if p.variant == nil {
		return norm
	}
	return p.variant(norm)
}

// Normalize lowercases a word and folds typographic apostrophes to ASCII.
func Normalize(word string) string {
	return strings.ReplaceAll(strings.ToLower(word), "\u2019", "'")
}

// TrimSuffixMin removes the first matching suffix if at least minStem bytes
// remain. Language packs use it to fold plurals for near-duplicate detection.
func TrimSuffixMin(word string, minStem int, suffixes ...string) string {
	for _, suffix := range suffixes {
		if strings.HasSuffix(word, suffix) && len(word)-len(suffix) >= minStem {
			return word[:len(word)-len(suffix)]
		}
	}
	return word
}

func wordSet(text string) map[string]struct{} {
	set := map[string]struct{}{}
	for word := range strings.FieldsSeq(text) {
		set[Normalize(word)] = struct{}{}
	}
	return set
}

func longestFirst(text string) []string {
	words := slices.Collect(strings.FieldsSeq(Normalize(text)))
	slices.SortStableFunc(words, func(a, b string) int {
		return cmp.Or(cmp.Compare(len(b), len(a)), strings.Compare(a, b))
	})
	return words
}

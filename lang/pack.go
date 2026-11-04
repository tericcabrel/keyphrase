// Package lang is the registry of language packs the extractor looks up.
// Import a built-in package for its side effect, or call Register from init.
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

type Pack interface {
	Code() string
	IsStopword(norm string) bool
	IsConnector(norm string) bool
	ElisionPrefixes() []string
	CliticSuffixes() []string
	IsAbbreviation(norm string) bool
	CapitalizesNouns() bool
	VariantKey(norm string) string
	Tagger() *postag.Tagger
	Rarity() *rarity.Table
}

type Spec struct {
	Code          string
	Lists         [][]byte
	Connectors    string
	Abbreviations string
	Elisions      string
	Clitics       string
	NounCaps      bool
	Variant       func(string) string
	Tagger        func() *postag.Tagger
	Rarity        func() *rarity.Table
}

var (
	registryMu sync.RWMutex
	registry   = map[string]Pack{}
)

func Register(pack Pack) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := registry[pack.Code()]; dup {
		panic(fmt.Sprintf("lang: duplicate pack %q", pack.Code()))
	}
	registry[pack.Code()] = pack
}

func Get(code string) (Pack, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	pack, ok := registry[code]
	return pack, ok
}

func Codes() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return slices.Sorted(maps.Keys(registry))
}

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

func MustRead(fsys fs.FS, name string) []byte {
	raw, err := fs.ReadFile(fsys, name)
	if err != nil {
		panic(fmt.Sprintf("lang: %s: %v", name, err))
	}
	return raw
}

func LazyTagger(fsys fs.FS, name string) func() *postag.Tagger {
	return sync.OnceValue(func() *postag.Tagger {
		tagger, err := postag.ReadTagger(bytes.NewReader(MustRead(fsys, name)))
		if err != nil {
			panic(fmt.Sprintf("lang: %s: %v", name, err))
		}
		return tagger
	})
}

func LazyTable(fsys fs.FS, name string) func() *rarity.Table {
	return sync.OnceValue(func() *rarity.Table {
		table, err := rarity.Read(bytes.NewReader(MustRead(fsys, name)))
		if err != nil {
			panic(fmt.Sprintf("lang: %s: %v", name, err))
		}
		return table
	})
}

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

func Normalize(word string) string {
	return strings.ReplaceAll(strings.ToLower(word), "\u2019", "'")
}

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

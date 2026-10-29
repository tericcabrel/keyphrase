// Package pt registers the Portuguese language pack.
//
// Import it for its side effect:
//
//	import _ "github.com/tericcabrel/keyphrase/lang/pt"
package pt

import (
	"embed"
	"strings"

	"github.com/tericcabrel/keyphrase/lang"
)

//go:embed snowball_pt.txt extra_pt.txt model.bin.gz rarity.bin.gz
var files embed.FS

func init() {
	lang.Register(lang.New(lang.Spec{
		Code:          "pt",
		Lists:         [][]byte{lang.MustRead(files, "snowball_pt.txt"), lang.MustRead(files, "extra_pt.txt")},
		Connectors:    "de do da dos das d' a o os as para",
		Abbreviations: "sr sra srta dr dra prof eng av pág p.ex ex",
		Elisions:      "d'",
		Variant:       portugueseVariant,
		Tagger:        lang.LazyTagger(files, "model.bin.gz"),
		Rarity:        lang.LazyTable(files, "rarity.bin.gz"),
	}))
}

// portugueseVariant folds plural -ões/-ães to -ão, then strips a plural -s
// and a final -e so that "eleições" and "eleição", or "cidades" and
// "cidade", share a key.
func portugueseVariant(word string) string {
	for _, suffix := range []string{"ões", "ães"} {
		if len(word)-len(suffix) >= 3 && strings.HasSuffix(word, suffix) {
			return word[:len(word)-len(suffix)] + "ão"
		}
	}
	return lang.TrimSuffixMin(lang.TrimSuffixMin(word, 3, "s"), 3, "e")
}

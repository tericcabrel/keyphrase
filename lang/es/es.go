// Package es registers the Spanish language pack.
//
// Import it for its side effect:
//
//	import _ "github.com/tericcabrel/keyphrase/lang/es"
package es

import (
	"embed"

	"github.com/tericcabrel/keyphrase/lang"
)

//go:embed snowball_es.txt extra_es.txt model.bin.gz rarity.bin.gz
var files embed.FS

func init() {
	lang.Register(lang.New(lang.Spec{
		Code:          "es",
		Lists:         [][]byte{lang.MustRead(files, "snowball_es.txt"), lang.MustRead(files, "extra_es.txt")},
		Connectors:    "de del la el los las para",
		Abbreviations: "sr sra srta dr dra ud uds núm pág p.ej av avda",
		// Strip a plural -s, then a final -e, so that "presidente" and
		// "presidentes", or "ciudad" and "ciudades", share a key.
		Variant: func(word string) string {
			return lang.TrimSuffixMin(lang.TrimSuffixMin(word, 3, "s"), 3, "e")
		},
		Tagger: lang.LazyTagger(files, "model.bin.gz"),
		Rarity: lang.LazyTable(files, "rarity.bin.gz"),
	}))
}

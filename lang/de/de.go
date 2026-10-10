// Package de registers the German language pack.
//
// Import it for its side effect:
//
//	import _ "github.com/tericcabrel/keyphrase/lang/de"
package de

import (
	"embed"

	"github.com/tericcabrel/keyphrase/lang"
)

//go:embed snowball_de.txt extra_de.txt model.bin.gz rarity.bin.gz
var files embed.FS

func init() {
	lang.Register(lang.New(lang.Spec{
		Code:          "de",
		Lists:         [][]byte{lang.MustRead(files, "snowball_de.txt"), lang.MustRead(files, "extra_de.txt")},
		Connectors:    "von der des die dem den für zur zum vom",
		Abbreviations: "z b d h u a s o bzw ca dr prof nr mio mrd str vgl ggf evtl inkl z.b d.h u.a s.o hr fr",
		NounCaps:      true,
		// German plurals mostly add -en, -er, -e, -n or -s. Umlaut plurals are
		// not folded.
		Variant: func(word string) string {
			return lang.TrimSuffixMin(word, 4, "en", "er", "e", "n", "s")
		},
		Tagger: lang.LazyTagger(files, "model.bin.gz"),
		Rarity: lang.LazyTable(files, "rarity.bin.gz"),
	}))
}

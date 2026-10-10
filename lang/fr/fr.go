// Package fr registers the French language pack.
//
// Import it for its side effect:
//
//	import _ "github.com/tericcabrel/keyphrase/lang/fr"
package fr

import (
	"embed"

	"github.com/tericcabrel/keyphrase/lang"
)

//go:embed snowball_fr.txt extra_fr.txt model.bin.gz rarity.bin.gz
var files embed.FS

func init() {
	lang.Register(lang.New(lang.Spec{
		Code:          "fr",
		Lists:         [][]byte{lang.MustRead(files, "snowball_fr.txt"), lang.MustRead(files, "extra_fr.txt")},
		Connectors:    "de du des d' la le l' les pour",
		Abbreviations: "m mm mme mlle dr pr st ste p ex cf env av bd n°",
		Elisions:      "l' d' j' m' n' s' t' c' qu' jusqu' lorsqu' puisqu' quoiqu'",
		Variant: func(word string) string {
			return lang.TrimSuffixMin(word, 3, "s", "x")
		},
		Tagger: lang.LazyTagger(files, "model.bin.gz"),
		Rarity: lang.LazyTable(files, "rarity.bin.gz"),
	}))
}

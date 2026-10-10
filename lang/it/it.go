// Package it registers the Italian language pack.
//
// Import it for its side effect:
//
//	import _ "github.com/tericcabrel/keyphrase/lang/it"
package it

import (
	"embed"

	"github.com/tericcabrel/keyphrase/lang"
)

//go:embed snowball_it.txt extra_it.txt model.bin.gz rarity.bin.gz
var files embed.FS

func init() {
	lang.Register(lang.New(lang.Spec{
		Code:          "it",
		Lists:         [][]byte{lang.MustRead(files, "snowball_it.txt"), lang.MustRead(files, "extra_it.txt")},
		Connectors:    "di del della dello dei degli delle dell' d' per la il lo le gli l'",
		Abbreviations: "sig sigg dott prof avv ing on sen pag es n",
		Elisions:      "l' d' dell' dall' nell' sull' all' coll' un' quest' quell' c' s' v' m' t'",
		// Italian number and gender inflect the final vowel
		// (economia/economie, mercato/mercati).
		Variant: func(word string) string {
			return lang.TrimSuffixMin(word, 3, "a", "e", "i", "o")
		},
		Tagger: lang.LazyTagger(files, "model.bin.gz"),
		Rarity: lang.LazyTable(files, "rarity.bin.gz"),
	}))
}

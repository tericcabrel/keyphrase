// Package en registers the English language pack.
//
// Import it for its side effect:
//
//	import _ "github.com/tericcabrel/keyphrase/lang/en"
package en

import (
	"embed"
	"strings"

	"github.com/tericcabrel/keyphrase/lang"
)

//go:embed snowball_en.txt extra_en.txt model.bin.gz rarity.bin.gz
var files embed.FS

func init() {
	lang.Register(lang.New(lang.Spec{
		Code:          "en",
		Lists:         [][]byte{lang.MustRead(files, "snowball_en.txt"), lang.MustRead(files, "extra_en.txt")},
		Connectors:    "of the for",
		Abbreviations: "mr mrs ms dr prof st jr sr vs e.g i.e no fig approx dept gov sen rep gen lt col sgt rev",
		Clitics:       "'s",
		Variant:       englishVariant,
		Tagger:        lang.LazyTagger(files, "model.bin.gz"),
		Rarity:        lang.LazyTable(files, "rarity.bin.gz"),
	}))
}

func englishVariant(word string) string {
	switch {
	case len(word) > 4 && strings.HasSuffix(word, "ies"):
		return word[:len(word)-3] + "y"
	case len(word) > 3 && strings.HasSuffix(word, "s") &&
		!strings.HasSuffix(word, "ss") && !strings.HasSuffix(word, "us") && !strings.HasSuffix(word, "is"):
		return word[:len(word)-1]
	}
	return word
}

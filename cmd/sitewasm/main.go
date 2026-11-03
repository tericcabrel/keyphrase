//go:build js && wasm

// Command sitewasm is the browser build of the extractor, loaded by the Try page.
package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/tericcabrel/keyphrase/extractor"
	_ "github.com/tericcabrel/keyphrase/lang/all"
)

// A field missing from the page's JSON keeps its DefaultExtractOptions value.
type settings struct {
	DropLoneNouns     bool    `json:"drop_lone_nouns"`
	LoneNounMinRarity float64 `json:"lone_noun_min_rarity"`
	DropOneOffPhrases bool    `json:"drop_one_off_phrases"`
	OneOffMinOthers   int     `json:"one_off_min_others"`
}

func defaultSettings() settings {
	return settings{
		DropLoneNouns:     extractor.DefaultExtractOptions.DropLoneNouns,
		LoneNounMinRarity: extractor.DefaultExtractOptions.LoneNounMinRarity,
		DropOneOffPhrases: extractor.DefaultExtractOptions.DropOneOffPhrases,
		OneOffMinOthers:   extractor.DefaultExtractOptions.OneOffMinOthers,
	}
}

func (s settings) options() extractor.ExtractOptions {
	options := extractor.DefaultExtractOptions
	options.DropLoneNouns = s.DropLoneNouns
	options.LoneNounMinRarity = s.LoneNounMinRarity
	options.DropOneOffPhrases = s.DropOneOffPhrases
	options.OneOffMinOthers = s.OneOffMinOthers
	return options
}

func main() {
	js.Global().Set("keyphraseExtract", js.FuncOf(extractJS))
	js.Global().Set("keyphraseDefaults", js.FuncOf(defaultsJS))
	js.Global().Call("dispatchEvent", js.Global().Get("CustomEvent").New("keyphrase-ready"))
	<-make(chan struct{})
}

func defaultsJS(_ js.Value, _ []js.Value) any {
	body, err := json.Marshal(defaultSettings())
	if err != nil {
		return "{}"
	}
	return string(body)
}

func extractJS(_ js.Value, args []js.Value) any {
	if len(args) < 3 {
		return jsonString(nil, extractor.Counts{}, "missing arguments")
	}
	chosen := defaultSettings()
	if len(args) > 3 && args[3].Type() == js.TypeString {
		if err := json.Unmarshal([]byte(args[3].String()), &chosen); err != nil {
			return jsonString(nil, extractor.Counts{}, "invalid settings")
		}
	}
	kws, counts, err := extractor.Extract(args[0].String(), args[1].String(), args[2].Int(), chosen.options())
	if err != nil {
		return jsonString(kws, counts, err.Error())
	}
	return jsonString(kws, counts, "")
}

func jsonString(kws []extractor.Keyword, counts extractor.Counts, errMsg string) string {
	if kws == nil {
		kws = []extractor.Keyword{}
	}
	if counts.MoreThanOneWord == nil {
		counts.MoreThanOneWord = []string{}
	}
	body, err := json.Marshal(struct {
		Keywords []extractor.Keyword `json:"keywords"`
		Counts   extractor.Counts    `json:"counts"`
		Error    string              `json:"error,omitempty"`
	}{Keywords: kws, Counts: counts, Error: errMsg})
	if err != nil {
		return `{"keywords":[],"counts":{"words":0,"extracted":0,"returned":0,"extracted_more_than_one_word":0,"returned_more_than_one_word":0,"more_than_one_word":[]},"error":"encode failed"}`
	}
	return string(body)
}

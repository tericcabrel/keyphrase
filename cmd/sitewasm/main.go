//go:build js && wasm

// Command sitewasm is the browser build of the extractor. It is compiled
// with GOOS=js GOARCH=wasm and loaded only by the Try page.
package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/tericcabrel/keyphrase/extractor"
	_ "github.com/tericcabrel/keyphrase/lang/all"
)

func main() {
	js.Global().Set("keyphraseExtract", js.FuncOf(extractJS))
	js.Global().Call("dispatchEvent", js.Global().Get("CustomEvent").New("keyphrase-ready"))
	<-make(chan struct{})
}

func extractJS(_ js.Value, args []js.Value) any {
	if len(args) < 3 {
		return jsonString(nil, extractor.Counts{}, "missing arguments")
	}
	kws, counts, err := extractor.ExtractWithCounts(args[0].String(), args[1].String(), args[2].Int())
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
		return `{"keywords":[],"counts":{"words":0,"extracted":0,"ranked":0,"extracted_more_than_one_word":0,"ranked_more_than_one_word":0,"more_than_one_word":[]},"error":"encode failed"}`
	}
	return string(body)
}

package extractor_test

import (
	"fmt"

	"github.com/tericcabrel/keyphrase/extractor"
	_ "github.com/tericcabrel/keyphrase/lang/en"
)

func ExampleExtract() {
	text := "Offshore wind farms are growing along the Yorkshire coast."
	kws, err := extractor.Extract(text, "en", 2)
	if err != nil {
		panic(err)
	}
	for _, kw := range kws {
		fmt.Printf("%s [%d:%d]\n", kw.Text, kw.StartByte, kw.EndByte)
	}
	// Output:
	// Yorkshire [42:51]
	// Offshore wind [0:13]
}

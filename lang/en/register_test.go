package en_test

import (
	"testing"

	"github.com/tericcabrel/keyphrase/lang"
	_ "github.com/tericcabrel/keyphrase/lang/en"
)

func TestImportRegistersOnlyEnglish(t *testing.T) {
	if _, ok := lang.Get("en"); !ok {
		t.Fatal("en not registered")
	}
	if _, ok := lang.Get("fr"); ok {
		t.Fatal("importing lang/en registered French")
	}
}

package all

import (
	"strings"
	"testing"

	"github.com/tericcabrel/keyphrase/lang"
)

func TestEveryLanguageHasAModelAndTable(t *testing.T) {
	for _, code := range []string{"de", "en", "es", "fr", "it", "pt"} {
		p, ok := lang.Get(code)
		if !ok {
			t.Fatalf("%s: not registered", code)
		}
		tagger := p.Tagger()
		if tagger == nil {
			t.Fatalf("%s: no model", code)
		}
		if features, dict := tagger.Features(); features < 10000 || dict < 100 {
			t.Errorf("%s: suspiciously small model: %d features, %d dictionary words", code, features, dict)
		}
		if p.Tagger() != tagger {
			t.Errorf("%s: model loaded twice", code)
		}
		table := p.Rarity()
		if table == nil {
			t.Fatalf("%s: no table", code)
		}
		if table.Sentences() != 2_000_000 || table.Words() < 100_000 {
			t.Errorf("%s: %d sentences, %d words", code, table.Sentences(), table.Words())
		}
		if p.Rarity() != table {
			t.Errorf("%s: table loaded twice", code)
		}
	}
	en, _ := lang.Get("en")
	table := en.Rarity()
	people, ok := table.Score("people")
	if !ok || people > 0.35 {
		t.Errorf("people = %v ok=%v, want a common word", people, ok)
	}
	rare, ok := table.Score("chloroplast")
	if ok && rare <= people {
		t.Errorf("chloroplast %v is not rarer than people %v", rare, people)
	}
}

// TestSpotChecks pins a few verb and noun decisions per language. Sentences
// are given pre-split, the way the extractor splits them.
func TestSpotChecks(t *testing.T) {
	tests := []struct {
		language, sentence string
		verbs, nonVerbs    []string
	}{
		{"it", "La Banca d' Italia ha rivisto le stime .", []string{"ha", "rivisto"}, []string{"Banca", "stime"}},
		{"it", "Un aumento dei prezzi preoccupa le famiglie .", []string{"preoccupa"}, []string{"aumento", "prezzi"}},
		{"es", "Los sistemas intentan replicar el cerebro humano .", []string{"intentan", "replicar"}, []string{"sistemas", "cerebro"}},
		{"es", "El Parlamento Europeo aprobó la ley .", []string{"aprobó"}, []string{"Parlamento", "ley"}},
		{"es", "La toma de decisiones es compleja .", []string{"es"}, []string{"toma", "decisiones"}},
		{"pt", "O Banco de Portugal reviu as previsões .", []string{"reviu"}, []string{"Banco", "previsões"}},
		{"fr", "La sécheresse frappe durement le sud .", []string{"frappe"}, []string{"sécheresse", "sud"}},
		{"de", "Werder Bremen sicherte sich den Titel .", []string{"sicherte"}, []string{"Bremen", "Titel"}},
		{"en", "Measles cases have risen sharply .", []string{"have", "risen"}, []string{"Measles", "cases"}},
		{"en", "The use of energy has increased .", []string{"has", "increased"}, []string{"use", "energy"}},
		{"en", "We use energy .", []string{"use"}, []string{"energy"}},
	}
	for _, tc := range tests {
		words := strings.Fields(tc.sentence)
		p, ok := lang.Get(tc.language)
		if !ok || p.Tagger() == nil {
			t.Fatalf("%s: no tagger", tc.language)
		}
		tags := p.Tagger().Tag(words)
		isVerb := map[string]bool{}
		for i, w := range words {
			isVerb[w] = tags[i].IsVerb()
		}
		for _, w := range tc.verbs {
			if !isVerb[w] {
				t.Errorf("%s %q: %q not tagged as a verb (tags %v)", tc.language, tc.sentence, w, tags)
			}
		}
		for _, w := range tc.nonVerbs {
			if isVerb[w] {
				t.Errorf("%s %q: %q tagged as a verb (tags %v)", tc.language, tc.sentence, w, tags)
			}
		}
	}
}

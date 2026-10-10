package lang_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/tericcabrel/keyphrase/lang"
	_ "github.com/tericcabrel/keyphrase/lang/all"
)

func TestRegisteredCodes(t *testing.T) {
	if got, want := lang.Codes(), []string{"de", "en", "es", "fr", "it"}; !slices.Equal(got, want) {
		t.Fatalf("Codes() = %q, want %q", got, want)
	}
	for _, code := range []string{"EN", "pt", "", "en-US"} {
		if _, ok := lang.Get(code); ok {
			t.Errorf("Get(%q) should fail", code)
		}
	}
}

func TestPackData(t *testing.T) {
	for _, code := range lang.Codes() {
		p, _ := lang.Get(code)
		stopwords, connectors := lang.TestingPack(p)
		t.Run(code, func(t *testing.T) {
			if p.Code() != code {
				t.Fatalf("Code() = %q", p.Code())
			}
			if len(stopwords) < 150 {
				t.Errorf("only %d stopwords", len(stopwords))
			}
			if len(connectors) == 0 {
				t.Error("no connectors")
			}
			for w := range connectors {
				if !p.IsStopword(w) {
					t.Errorf("connector %q is not a stopword", w)
				}
			}
			for w := range stopwords {
				if w != lang.Normalize(w) || strings.ContainsAny(w, " \t|") {
					t.Errorf("stopword %q is not normalized", w)
				}
			}
		})
	}
}

func TestLanguageSpecifics(t *testing.T) {
	tests := []struct {
		code                string
		stop, notStop, conn []string
		elisions            []string
		abbreviation        string
	}{
		{"en", []string{"the", "said", "however", "percent"}, []string{"wind", "vaccine", "bank"}, []string{"of"}, nil, "dr"},
		{"fr", []string{"le", "de", "selon", "mardi"}, []string{"sécheresse", "économie"}, []string{"de", "d'", "la"}, []string{"l'", "d'", "qu'"}, "mme"},
		{"de", []string{"der", "und", "sagte", "prozent"}, []string{"wärmepumpen", "bundesliga"}, []string{"für", "von", "des"}, nil, "z"},
		{"es", []string{"el", "según", "millones"}, []string{"vivienda", "inteligencia"}, []string{"de", "del"}, nil, "sra"},
		{"it", []string{"il", "secondo", "euro"}, []string{"inflazione", "turismo"}, []string{"di", "della", "dell'"}, []string{"l'", "dell'", "nell'"}, "prof"},
	}
	for _, tc := range tests {
		p, _ := lang.Get(tc.code)
		for _, w := range tc.stop {
			if !p.IsStopword(w) {
				t.Errorf("%s: %q should be a stopword", tc.code, w)
			}
		}
		for _, w := range tc.notStop {
			if p.IsStopword(w) {
				t.Errorf("%s: %q should not be a stopword", tc.code, w)
			}
		}
		for _, w := range tc.conn {
			if !p.IsConnector(w) {
				t.Errorf("%s: %q should be a connector", tc.code, w)
			}
		}
		for _, conj := range []string{"and", "et", "und", "y", "e"} {
			if p.IsConnector(conj) {
				t.Errorf("%s: conjunction %q must not be a connector", tc.code, conj)
			}
		}
		for _, e := range tc.elisions {
			if !slices.Contains(p.ElisionPrefixes(), e) {
				t.Errorf("%s: missing elision %q", tc.code, e)
			}
		}
		if !p.IsAbbreviation(tc.abbreviation) {
			t.Errorf("%s: %q should be an abbreviation", tc.code, tc.abbreviation)
		}
	}
	if de, _ := lang.Get("de"); !de.CapitalizesNouns() {
		t.Error("de should capitalize nouns")
	}
	if en, _ := lang.Get("en"); en.CapitalizesNouns() {
		t.Error("en should not capitalize nouns")
	}
}

func TestElisionsLongestFirst(t *testing.T) {
	it, _ := lang.Get("it")
	e := it.ElisionPrefixes()
	if slices.Index(e, "dell'") > slices.Index(e, "l'") {
		t.Fatalf("dell' must precede l': %q", e)
	}
}

func TestVariantKeys(t *testing.T) {
	tests := []struct{ code, a, b string }{
		{"en", "turbine", "turbines"},
		{"en", "policy", "policies"},
		{"fr", "exploitation", "exploitations"},
		{"fr", "prix", "prix"},
		{"de", "regierung", "regierungen"},
		{"de", "bank", "banken"},
		{"es", "ciudad", "ciudades"},
		{"es", "presidente", "presidentes"},
		{"it", "economia", "economie"},
		{"it", "mercato", "mercati"},
	}
	for _, tc := range tests {
		p, _ := lang.Get(tc.code)
		if ka, kb := p.VariantKey(tc.a), p.VariantKey(tc.b); ka != kb {
			t.Errorf("%s: VariantKey(%q)=%q != VariantKey(%q)=%q", tc.code, tc.a, ka, tc.b, kb)
		}
	}
	en, _ := lang.Get("en")
	if en.VariantKey("bus") == en.VariantKey("bu") || en.VariantKey("glass") != "glass" {
		t.Error("en variant key over-strips -us/-ss")
	}
}

func TestNormalize(t *testing.T) {
	if got := lang.Normalize("L’Économie"); got != "l'économie" {
		t.Fatalf("Normalize = %q", got)
	}
}

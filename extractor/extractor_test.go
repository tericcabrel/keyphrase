package extractor

import (
	"errors"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/tericcabrel/keyphrase/internal/eval"
	"github.com/tericcabrel/keyphrase/lang"
)

func texts(keywords []Keyword) []string {
	out := make([]string, len(keywords))
	for i, k := range keywords {
		out[i] = k.Text
	}
	return out
}

// candidateTexts returns the source text of the first occurrence of every
// generated candidate, before ranking and selection.
func candidateTexts(t *testing.T, text, language string) []string {
	t.Helper()
	pack, ok := lang.Get(language)
	if !ok {
		t.Fatalf("no pack %q", language)
	}
	tokens, _ := tokenize(text, pack)
	phrases := generateCandidates(text, tokens, pack, false)
	out := make([]string, 0, len(phrases))
	for _, p := range phrases {
		o := p.occurrences[0]
		out = append(out, text[o.start:o.end])
	}
	return out
}

func TestCandidateBoundaries(t *testing.T) {
	tests := []struct {
		name, language, text string
		want, wantNot        []string
		// absentInside: wantNot must not occur inside a longer candidate either.
		absentInside bool
	}{
		{
			name: "en connectors inside, stopwords never at edges", language: "en",
			text:    "The Department of Health published the report.",
			want:    []string{"Department of Health", "Department", "Health", "report"},
			wantNot: []string{"The Department", "of Health", "Health published the", "the report", "report."},
		},
		{
			name: "en conjunction is not a connector", language: "en",
			text:    "Tenants and landlords disagree.",
			want:    []string{"Tenants", "landlords"},
			wantNot: []string{"Tenants and landlords"},
		},
		{
			name: "en possessive clitic splits off", language: "en",
			text:    "Britain's energy strategy changed.",
			want:    []string{"Britain", "energy strategy"},
			wantNot: []string{"Britain's", "Britain's energy strategy"},
		},
		{
			name: "fr connectors de la", language: "fr",
			text:    "Le ministère de la Santé publie un rapport.",
			want:    []string{"Santé"},
			wantNot: []string{"ministère de la Santé", "de la Santé", "Le ministère", "ministère de la"},
		},
		{
			name: "fr elision straight and curly apostrophes", language: "fr",
			text:    "L'économie ralentit. Il parle de l’économie.",
			want:    []string{"économie"},
			wantNot: []string{"L'économie", "l’économie"},
		},
		{
			name: "de compounds stay whole and hyphen compounds join", language: "de",
			text:    "Das Bundesverfassungsgericht prüft die Corona-Pandemie.",
			want:    []string{"Bundesverfassungsgericht", "Corona-Pandemie"},
			wantNot: []string{"Corona", "Pandemie", "Bundes"},
		},
		{
			name: "de connector für", language: "de",
			text:    "Die Kreditanstalt für Wiederaufbau zahlt.",
			want:    []string{"Kreditanstalt für Wiederaufbau"},
			wantNot: []string{"für Wiederaufbau", "Die Kreditanstalt"},
		},
		{
			name: "es connector de", language: "es",
			text:    "El Banco de España prevé una bajada.",
			want:    []string{"Banco de España"},
			wantNot: []string{"El Banco", "de España"},
		},
		{
			name: "it elided connector inside a name", language: "it",
			text:    "La Banca d'Italia e il bilancio dell'Unione europea.",
			want:    []string{"Banca d'Italia", "Unione europea"},
			wantNot: []string{"dell'Unione europea", "d'Italia", "La Banca"},
		},
		{
			name: "pt connector de", language: "pt",
			text:    "O Banco de Portugal prevê uma descida.",
			want:    []string{"Banco de Portugal"},
			wantNot: []string{"O Banco", "de Portugal"},
		},
		{
			name: "punctuation breaks phrases", language: "en",
			text:    "Paris, Lyon (Marseille) and Nice.",
			want:    []string{"Paris", "Lyon", "Marseille", "Nice"},
			wantNot: []string{"Paris, Lyon", "Paris Lyon", "Lyon (Marseille"},
		},
		{
			name: "numbers cannot start or stand alone", language: "en",
			text:    "In 2024 the Windows 11 rollout reached 300 offices.",
			want:    []string{"Windows 11", "Windows 11 rollout"},
			wantNot: []string{"2024", "300", "300 offices", "11"},
		},
		{
			name: "line breaks split phrases", language: "en",
			text:    "solar panels\nwind turbines",
			want:    []string{"solar panels", "wind turbines"},
			wantNot: []string{"panels\nwind"},
		},
		{
			name: "capitalized mid-sentence article opens a name", language: "fr",
			text:    "Le train est construit à La Rochelle par Alstom.",
			want:    []string{"La Rochelle"},
			wantNot: []string{"Le train", "construit à La Rochelle"},
		},
		{
			name: "truncated capitalized names are rejected", language: "en",
			text:    "Officials at the World Health Organization agreed.",
			want:    []string{"World Health Organization"},
			wantNot: []string{"Health Organization", "World Health"},
		},
		{
			name: "two-letter words only as acronyms", language: "en",
			text:    "The EU and ox carts.",
			want:    []string{"EU"},
			wantNot: []string{"ox", "ox carts"},
		},
		{
			name: "code identifiers and source filenames are not content", language: "en",
			text: "Set SENTRY_PROJECT_DSN and APP_VERSION. Edit index.ts, route.ts, instrument.ts and index.js. The Node.js application publishes source maps.",
			want: []string{"Node.js", "source maps"},
			wantNot: []string{
				"SENTRY_PROJECT_DSN", "APP_VERSION",
				"index.ts", "index.js", "route.ts", "instrument.ts",
			},
			absentInside: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := candidateTexts(t, tc.text, tc.language)
			for _, w := range tc.want {
				if !slices.Contains(got, w) {
					t.Errorf("missing candidate %q in %q", w, got)
				}
			}
			for _, w := range tc.wantNot {
				if slices.Contains(got, w) {
					t.Errorf("unexpected candidate %q in %q", w, got)
				}
			}
			if tc.absentInside {
				for _, w := range tc.wantNot {
					for _, c := range got {
						if strings.Contains(c, w) {
							t.Errorf("candidate %q contains %q", c, w)
						}
					}
				}
			}
		})
	}
}

func TestElisionOffsets(t *testing.T) {
	text := "Selon l’économie et l'Europe."
	kws, _, err := Extract(text, "fr", 5)
	if err != nil {
		t.Fatal(err)
	}
	checkInvariants(t, text, 5, kws)
	got := texts(kws)
	slices.Sort(got)
	// "économie" is a common noun and is not returned on its own. "Europe"
	// is a name, and the elided article must not be part of its span.
	if !slices.Equal(got, []string{"Europe"}) {
		t.Fatalf("got %q", got)
	}
	for _, k := range kws {
		if k.Text == "Europe" && k.StartByte != strings.Index(text, "Europe") {
			t.Errorf("Europe starts at %d", k.StartByte)
		}
	}
}

func TestSentenceAbbreviations(t *testing.T) {
	tests := []struct {
		language, text string
		want           int
	}{
		{"en", "Dr. Smith met Mr. Jones in the U.S. Senate. They talked.", 2},
		{"de", "Die Regierung plant z. B. neue Regeln für 3,5 Mio. Euro. Das ist neu.", 2},
		{"fr", "M. Dupont est arrivé. Mme Durand aussi.", 2},
		{"es", "La Sra. García habló. Luego se fue.", 2},
		{"it", "Il prof. Rossi ha parlato. Poi è uscito.", 2},
		{"pt", "A Sra. Silva falou. Depois saiu.", 2},
		{"en", "J. K. Rowling wrote books. Many sold.", 2},
	}
	for _, tc := range tests {
		pack, _ := lang.Get(tc.language)
		if _, got := tokenize(tc.text, pack); got != tc.want {
			t.Errorf("%s %q: %d sentences, want %d", tc.language, tc.text, got, tc.want)
		}
	}
}

func TestDottedAbbreviationKeepsFinalPeriod(t *testing.T) {
	got := candidateTexts(t, "Growth in the U.S. economy slowed.", "en")
	if !slices.Contains(got, "U.S. economy") {
		t.Fatalf("candidates %q lack %q", got, "U.S. economy")
	}
}

func TestRepeatedPhraseUsesEarliestOccurrenceAndAggregates(t *testing.T) {
	text := "Heat pumps are popular. Installers fit heat pumps quickly. " +
		"Grants cover HEAT PUMPS for households. The weather was mild."
	kws, _, err := Extract(text, "en", 3)
	if err != nil {
		t.Fatal(err)
	}
	checkInvariants(t, text, 3, kws)
	if kws[0].Text != "Heat pumps" || kws[0].StartByte != 0 {
		t.Fatalf("top keyword = %+v, want earliest %q at 0", kws[0], "Heat pumps")
	}
	for _, k := range kws[1:] {
		if strings.EqualFold(k.Text, "heat pumps") || strings.EqualFold(k.Text, "heat") || strings.EqualFold(k.Text, "pumps") {
			t.Errorf("case variant or contained word %q selected alongside %q", k.Text, kws[0].Text)
		}
	}
}

func TestContainedAndOverlappingPhrasesRemoved(t *testing.T) {
	text := "The European Central Bank raised rates. The European Central Bank's " +
		"decision surprised markets. Markets expected the European Central Bank to wait."
	kws, _, err := Extract(text, "en", 10)
	if err != nil {
		t.Fatal(err)
	}
	checkInvariants(t, text, 10, kws)
	got := texts(kws)
	if got[0] != "European Central Bank" {
		t.Fatalf("top = %q, want European Central Bank (all: %q)", got[0], got)
	}
	for _, sub := range []string{"European Central", "Central Bank", "European", "Central", "Bank"} {
		if slices.Contains(got, sub) {
			t.Errorf("sub-phrase %q selected alongside the full name: %q", sub, got)
		}
	}
}

func TestNearDuplicatePluralRemoved(t *testing.T) {
	text := "Wind turbine output rose. Each wind turbine is inspected. " +
		"Wind turbines need maintenance. Engineers repair wind turbines."
	kws, _, err := Extract(text, "en", 10)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, k := range kws {
		if strings.HasPrefix(strings.ToLower(k.Text), "wind turbine") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("want exactly one wind turbine(s) variant, got %q", texts(kws))
	}
}

func TestGenericWordYieldsToSpecificPhrase(t *testing.T) {
	text := "Interest rates rose again. Banks raised interest rates on loans. " +
		"Higher interest rates hurt borrowers. Rates matter."
	kws, _, err := Extract(text, "en", 3)
	if err != nil {
		t.Fatal(err)
	}
	got := texts(kws)
	if got[0] != "Interest rates" {
		t.Fatalf("top = %q, want %q (all %q)", got[0], "Interest rates", got)
	}
	if slices.Contains(got, "Rates") || slices.Contains(got, "rates") || slices.Contains(got, "interest") {
		t.Fatalf("generic single word selected: %q", got)
	}
}

func TestDeterministic(t *testing.T) {
	for _, f := range exampleFiles(t) {
		raw := mustRead(t, f)
		language := langOf(f)
		first, _, err := Extract(raw, language, 1000)
		if err != nil {
			t.Fatal(err)
		}
		for range 20 {
			again, _, _ := Extract(raw, language, 1000)
			if !slices.Equal(first, again) {
				t.Fatalf("%s: nondeterministic output\n%v\n%v", f, first, again)
			}
		}
	}
}

func TestTieBreakOrder(t *testing.T) {
	kws := []Keyword{
		{Text: "b", StartByte: 5, Score: 0.5},
		{Text: "a", StartByte: 5, Score: 0.5},
		{Text: "c", StartByte: 1, Score: 0.5},
		{Text: "d", StartByte: 9, Score: 1},
	}
	slices.SortFunc(kws, compareKeywords)
	if got := texts(kws); !slices.Equal(got, []string{"d", "c", "a", "b"}) {
		t.Fatalf("order = %q", got)
	}
}

func TestEmptyAndWhitespaceInput(t *testing.T) {
	for _, l := range languages {
		for _, text := range []string{"", " ", "\n\t \u00a0\u2003\n"} {
			kws, _, err := Extract(text, l, 5)
			if err != nil {
				t.Fatalf("%s %q: %v", l, text, err)
			}
			if kws == nil || len(kws) != 0 {
				t.Fatalf("%s %q: got %#v, want empty non-nil slice", l, text, kws)
			}
		}
	}
}

func TestOnlyStopwordsOrPunctuation(t *testing.T) {
	for _, tc := range []struct{ l, text string }{
		{"en", "the of and, it is. 2024!"},
		{"fr", "de la, et le... 12 %"},
		{"de", "und der die – das."},
		{"es", "de la y el; los."},
		{"it", "di e il, la."},
		{"pt", "de a e o, os."},
	} {
		kws, _, err := Extract(tc.text, tc.l, 5)
		if err != nil {
			t.Fatal(err)
		}
		if len(kws) != 0 {
			t.Errorf("%s %q: got %q, want none", tc.l, tc.text, texts(kws))
		}
	}
}

func TestCounts(t *testing.T) {
	text := "The Bank of England raised rates. The Bank of England raised rates again."
	kws, counts, err := Extract(text, "en", 5)
	if err != nil {
		t.Fatal(err)
	}
	if counts.Returned != len(kws) {
		t.Fatalf("returned %d, keywords %d", counts.Returned, len(kws))
	}
	if counts.Words != 13 {
		t.Fatalf("words = %d, want 13", counts.Words)
	}
	if counts.Extracted < counts.Returned || counts.ExtractedMoreThanOneWord < 1 {
		t.Fatalf("counts = %+v", counts)
	}
	var returned []string
	for _, k := range kws {
		if len(strings.Fields(k.Text)) > 1 {
			returned = append(returned, k.Text)
		}
	}
	if counts.ReturnedMoreThanOneWord != len(returned) {
		t.Fatalf("returned multiword %d, keyword texts %q", counts.ReturnedMoreThanOneWord, returned)
	}
	if len(counts.MoreThanOneWord) != counts.ExtractedMoreThanOneWord {
		t.Fatalf("list %d, extracted multiword %d", len(counts.MoreThanOneWord), counts.ExtractedMoreThanOneWord)
	}
	for _, text := range returned {
		if !slices.Contains(counts.MoreThanOneWord, text) {
			t.Fatalf("returned %q missing from %q", text, counts.MoreThanOneWord)
		}
	}
	if !slices.Contains(counts.MoreThanOneWord, "Bank of England") {
		t.Fatalf("more than one word = %q", counts.MoreThanOneWord)
	}
}

func TestLanguageAndLimitErrors(t *testing.T) {
	for _, l := range []string{"", "EN", "En", "nl", "en-US", "english"} {
		if _, _, err := Extract("text", l, 5); !errors.Is(err, ErrUnsupportedLanguage) {
			t.Errorf("language %q: err = %v", l, err)
		}
	}
	for _, n := range []int{-1, 0} {
		if _, _, err := Extract("text", "en", n); !errors.Is(err, ErrInvalidLimit) {
			t.Errorf("limit %d: err = %v", n, err)
		}
	}
	for _, n := range []int{1, 1000} {
		if _, _, err := Extract("text", "en", n); err != nil {
			t.Errorf("limit %d: %v", n, err)
		}
	}
}

const offshoreWind = `Offshore wind farms are becoming the backbone of Britain's energy
strategy. The government wants offshore wind capacity to reach 50
gigawatts by 2030, enough to power every home in the country.

The largest project, Dogger Bank Wind Farm, sits more than 130
kilometres off the Yorkshire coast. When complete, Dogger Bank will
use 277 turbines, each taller than the Eiffel Tower. Engineers say
the turbines can generate electricity even in moderate winds.

Grid connections remain the biggest obstacle. National Grid has
warned that some offshore wind farms may wait years before they can
connect to the transmission network. Developers also face rising
steel prices and higher interest rates, which have pushed up the
cost of building turbines at sea.`

func TestExtractOptionsAdjustWhichPhrasesAreReturned(t *testing.T) {
	tests := []struct {
		name    string
		adjust  func(*ExtractOptions)
		want    []string
		wantNot []string
	}{
		{
			name:    "defaults drop common lone nouns and one-off phrases",
			adjust:  func(*ExtractOptions) {},
			want:    []string{"Dogger Bank", "backbone", "obstacle"},
			wantNot: []string{"government", "electricity", "transmission network", "steel prices"},
		},
		{
			name:    "keeping lone nouns returns common single words",
			adjust:  func(weights *ExtractOptions) { weights.DropLoneNouns = false },
			want:    []string{"government", "electricity", "Engineers", "Developers"},
			wantNot: []string{"steel prices"},
		},
		{
			name:    "a lower rarity threshold keeps less rare nouns",
			adjust:  func(weights *ExtractOptions) { weights.LoneNounMinRarity = 0.5 },
			want:    []string{"electricity", "Engineers"},
			wantNot: []string{"government"},
		},
		{
			name:    "a zero rarity threshold drops every lone common noun",
			adjust:  func(weights *ExtractOptions) { weights.LoneNounMinRarity = 0 },
			want:    []string{"Dogger Bank"},
			wantNot: []string{"backbone", "obstacle"},
		},
		{
			name:    "keeping one-off phrases returns them",
			adjust:  func(weights *ExtractOptions) { weights.DropOneOffPhrases = false },
			want:    []string{"transmission network", "steel prices"},
			wantNot: []string{"government"},
		},
		{
			name:   "a higher minimum keeps one-off phrases in a short text",
			adjust: func(weights *ExtractOptions) { weights.OneOffMinOthers = 30 },
			want:   []string{"transmission network", "steel prices"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			weights := DefaultExtractOptions
			tc.adjust(&weights)
			kws, counts, err := Extract(offshoreWind, "en", 100, weights)
			if err != nil {
				t.Fatal(err)
			}
			if counts.Returned != len(kws) {
				t.Fatalf("returned %d, keywords %d", counts.Returned, len(kws))
			}
			got := texts(kws)
			for _, phrase := range tc.want {
				if !slices.Contains(got, phrase) {
					t.Errorf("missing %q in %q", phrase, got)
				}
			}
			for _, phrase := range tc.wantNot {
				if slices.Contains(got, phrase) {
					t.Errorf("unexpected %q in %q", phrase, got)
				}
			}
		})
	}
}

func TestInvalidExtractOptions(t *testing.T) {
	tests := []struct {
		name   string
		adjust func(*ExtractOptions)
	}{
		{"negative rarity", func(weights *ExtractOptions) { weights.LoneNounMinRarity = -0.1 }},
		{"rarity above 1", func(weights *ExtractOptions) { weights.LoneNounMinRarity = 1.5 }},
		{"rarity NaN", func(weights *ExtractOptions) { weights.LoneNounMinRarity = math.NaN() }},
		{"negative minimum of other candidates", func(weights *ExtractOptions) { weights.OneOffMinOthers = -1 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			weights := DefaultExtractOptions
			tc.adjust(&weights)
			if _, _, err := Extract(offshoreWind, "en", 10, weights); !errors.Is(err, ErrInvalidExtractOptions) {
				t.Errorf("err = %v, want ErrInvalidExtractOptions", err)
			}
			if _, _, err := Extract("", "en", 10, weights); !errors.Is(err, ErrInvalidExtractOptions) {
				t.Errorf("empty text err = %v, want ErrInvalidExtractOptions", err)
			}
		})
	}
}

func TestExtractOptionsAreOptional(t *testing.T) {
	text := mustRead(t, "../examples/en/offshore-wind.txt")
	omitted, _, err := Extract(text, "en", 5)
	if err != nil {
		t.Fatal(err)
	}
	passed, _, err := Extract(text, "en", 5, DefaultExtractOptions)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(omitted, passed) {
		t.Fatalf("omitted options = %+v, DefaultExtractOptions = %+v", omitted, passed)
	}
	_, _, err = Extract(text, "en", 5, DefaultExtractOptions, DefaultExtractOptions)
	if !errors.Is(err, ErrTooManyExtractOptions) {
		t.Fatalf("err = %v, want ErrTooManyExtractOptions", err)
	}
	ranked, err := eval.Extract(text, "en", 5, eval.Options{
		DropLoneNouns:     DefaultExtractOptions.DropLoneNouns,
		LoneNounMinRarity: DefaultExtractOptions.LoneNounMinRarity,
		DropOneOffPhrases: DefaultExtractOptions.DropOneOffPhrases,
		OneOffMinOthers:   DefaultExtractOptions.OneOffMinOthers,
	}, eval.DefaultRanking)
	if err != nil {
		t.Fatal(err)
	}
	if len(ranked) != len(omitted) {
		t.Fatalf("Extract = %+v, eval hook = %+v", omitted, ranked)
	}
	for i := range omitted {
		if ranked[i].Text != omitted[i].Text || ranked[i].StartByte != omitted[i].StartByte || ranked[i].EndByte != omitted[i].EndByte || ranked[i].Score != omitted[i].Score {
			t.Fatalf("Extract = %+v, eval hook = %+v", omitted, ranked)
		}
	}
}

func TestLargeLimitReturnsEveryKeyword(t *testing.T) {
	text := mustRead(t, "../examples/en/offshore-wind.txt")
	all, _, err := Extract(text, "en", 100000)
	if err != nil {
		t.Fatal(err)
	}
	again, _, err := Extract(text, "en", 1000000)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) == 0 || len(all) != len(again) {
		t.Fatalf("limits above the keyword count returned %d and %d", len(all), len(again))
	}
	exact, _, err := Extract(text, "en", len(all))
	if err != nil {
		t.Fatal(err)
	}
	if len(exact) != len(all) {
		t.Fatalf("limit %d returned %d keywords", len(all), len(exact))
	}
}

// TestPerLanguageExpectations pins a few clearly topical phrases from the
// example set so ranking regressions are visible. These are spot checks, not
// an accuracy measure.
func TestPerLanguageExpectations(t *testing.T) {
	tests := []struct {
		file string
		want []string
	}{
		{"en/offshore-wind.txt", []string{"Offshore wind", "Dogger Bank"}},
		{"en/measles-vaccination.txt", []string{"MMR vaccine", "World Health Organization"}},
		{"fr/tgv-sncf.txt", []string{"SNCF", "TGV"}},
		{"fr/secheresse-agriculture.txt", []string{"sécheresse", "agriculture"}},
		{"de/waermepumpen.txt", []string{"Wärmepumpen", "Kreditanstalt für Wiederaufbau"}},
		{"de/bundesliga.txt", []string{"Xabi Alonso", "Bayer Leverkusen"}},
		{"es/turismo-canarias.txt", []string{"Islas Canarias", "alquiler vacacional"}},
		{"es/inteligencia-artificial.txt", []string{"inteligencia artificial", "Parlamento Europeo"}},
		{"it/banca-italia-inflazione.txt", []string{"Banca d'Italia", "tassi di interesse", "prodotto interno lordo"}},
		{"it/venezia-turismo.txt", []string{"centro storico", "ticket d'accesso", "turismo di massa"}},
		{"pt/banco-portugal-inflacao.txt", []string{"Banco de Portugal", "taxas de juro", "produto interno bruto"}},
		{"pt/habitacao-lisboa.txt", []string{"alojamento local", "mercado de arrendamento", "centro histórico"}},
	}
	for _, tc := range tests {
		path := "../examples/" + tc.file
		kws, _, err := Extract(mustRead(t, path), langOf(path), 8)
		if err != nil {
			t.Fatal(err)
		}
		got := texts(kws)
		for _, w := range tc.want {
			if !slices.Contains(got, w) {
				t.Errorf("%s: %q not in top 8 %q", tc.file, w, got)
			}
		}
	}
}

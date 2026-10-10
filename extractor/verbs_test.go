package extractor

import (
	"errors"
	"slices"
	"testing"

	"github.com/tericcabrel/keyphrase/lang"
)

// taggedCandidateTexts is candidateTexts with part-of-speech tagging, as in
// Extract. Lone common nouns are still candidates; see
// taggedCandidateTextsDroppingNouns.
func taggedCandidateTexts(t *testing.T, text, language string) []string {
	t.Helper()
	return taggedCandidates(t, text, language, false)
}

// taggedCandidateTextsDroppingNouns is taggedCandidateTexts with lone common
// nouns removed. A lone word remains when it is a name or capitalized like one.
func taggedCandidateTextsDroppingNouns(t *testing.T, text, language string) []string {
	t.Helper()
	return taggedCandidates(t, text, language, true)
}

func taggedCandidates(t *testing.T, text, language string, dropLoneNouns bool) []string {
	t.Helper()
	pack, ok := lang.Get(language)
	if !ok {
		t.Fatalf("no pack %q", language)
	}
	tokens, _ := tokenize(text, pack)
	markTags(text, tokens, pack.Tagger())
	phrases := generateCandidates(text, tokens, pack, dropLoneNouns)
	out := make([]string, 0, len(phrases))
	for _, p := range phrases {
		o := p.occurrences[0]
		out = append(out, text[o.start:o.end])
	}
	return out
}

func TestVerbsBreakPhrases(t *testing.T) {
	tests := []struct {
		name, language, text string
		want, wantNot        []string
	}{
		{
			name: "it past participle", language: "it",
			text:    "La Banca d'Italia ha rivisto le stime.",
			want:    []string{"Banca d'Italia", "stime"},
			wantNot: []string{"rivisto"},
		},
		{
			name: "es verb pair", language: "es",
			text:    "Los sistemas intentan replicar el cerebro humano.",
			want:    []string{"sistemas", "cerebro humano"},
			wantNot: []string{"intentan replicar", "intentan", "replicar", "sistemas intentan"},
		},
		{
			name: "en participle", language: "en",
			text:    "Measles cases have risen sharply.",
			want:    []string{"Measles cases"},
			wantNot: []string{"risen", "risen sharply"},
		},
		{
			name: "de verb after a name", language: "de",
			text:    "Werder Bremen sicherte sich den Titel.",
			want:    []string{"Werder Bremen", "Titel"},
			wantNot: []string{"Werder Bremen sicherte", "sicherte"},
		},
		{
			name: "fr present tense", language: "fr",
			text:    "La sécheresse frappe durement le sud.",
			want:    []string{"sécheresse"},
			wantNot: []string{"frappe", "frappe durement", "sécheresse frappe"},
		},
		{
			name: "en rise as a noun is kept", language: "en",
			text:    "The rise of energy prices has worried investors.",
			want:    []string{"rise of energy", "energy prices"},
			wantNot: []string{"rise of energy prices", "worried", "worried investors"},
		},
		{
			name: "en rise as a verb is removed", language: "en",
			text:    "Energy prices rise every winter.",
			want:    []string{"Energy prices"},
			wantNot: []string{"rise", "Energy prices rise"},
		},
		{
			name: "es noun toma is kept", language: "es",
			text: "La toma de decisiones es compleja.",
			want: []string{"toma de decisiones"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := taggedCandidateTexts(t, tc.text, tc.language)
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
		})
	}
}

func TestModifiersNeedAHead(t *testing.T) {
	tests := []struct {
		name, language, text string
		want, wantNot        []string
	}{
		{
			name: "en adjectives, numerals and adverbs", language: "en",
			text:    "We will probably need a big effort. Thirty volunteers said it was actually worth the small budget.",
			want:    []string{"big effort", "small budget", "volunteers", "budget"},
			wantNot: []string{"probably", "big", "Thirty", "Thirty volunteers", "actually", "worth", "small", "actually worth"},
		},
		{
			name: "en adjective mis-tagged as adverb", language: "en",
			text: "Offshore wind farms are growing. Offshore wind capacity will reach 50 gigawatts.",
			want: []string{"Offshore wind", "Offshore wind farms"},
		},
		{
			name: "en capitalized adjective is kept", language: "en",
			text: "The ferry sails to the Scottish islands. It is a Scottish company.",
			want: []string{"Scottish islands", "Scottish"},
		},
		{
			name: "fr adjective after its noun", language: "fr",
			text:    "La politique agricole commune est souvent critiquée. Elle reste agricole.",
			want:    []string{"politique agricole commune"},
			wantNot: []string{"agricole", "souvent"},
		},
		{
			name: "de adjective before its noun", language: "de",
			text:    "Die neue Wärmepumpe spart Strom. Neue Geräte sind leiser.",
			want:    []string{"neue Wärmepumpe", "Wärmepumpe"},
			wantNot: []string{"neue"},
		},
		{
			name: "es adjective after its noun", language: "es",
			text:    "La inteligencia artificial avanza rápidamente en sistemas nuevos.",
			want:    []string{"inteligencia artificial", "sistemas nuevos"},
			wantNot: []string{"artificial", "nuevos", "rápidamente"},
		},
		{
			name: "it hyphenated compound is not an adverb", language: "it",
			text: "Un sistema multi-agente per l'ottimizzazione dei contenuti.",
			want: []string{"sistema multi-agente"},
		},
		{
			name: "de question word", language: "de",
			text:    "Warum Bankberatung teuer ist.",
			want:    []string{"Bankberatung"},
			wantNot: []string{"Warum", "Warum Bankberatung"},
		},
		{
			name: "it adverb and adjective", language: "it",
			text:    "La banca centrale ha invece alzato i tassi. Il tasso resta alto.",
			want:    []string{"banca centrale"},
			wantNot: []string{"centrale", "invece", "alto"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := taggedCandidateTexts(t, tc.text, tc.language)
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
		})
	}
}

func TestDropLoneNouns(t *testing.T) {
	tests := []struct {
		name, language, text string
		want, wantNot        []string
	}{
		{
			name: "en common nouns need a phrase", language: "en",
			text:    "We will probably need a big effort. Thirty volunteers said it was actually worth the small budget.",
			want:    []string{"big effort", "small budget"},
			wantNot: []string{"volunteers", "budget", "probably", "big", "Thirty"},
		},
		{
			name: "en capitalized adjective is kept", language: "en",
			text: "The ferry sails to the Scottish islands. It is a Scottish company.",
			want: []string{"Scottish islands", "Scottish"},
		},
		{
			name: "en proper name stands alone", language: "en",
			text:    "The committee met Merkel yesterday.",
			want:    []string{"Merkel"},
			wantNot: []string{"committee", "yesterday"},
		},
		{
			name: "de common noun is not alone", language: "de",
			text:    "Die neue Wärmepumpe spart Strom. Neue Geräte sind leiser.",
			want:    []string{"neue Wärmepumpe"},
			wantNot: []string{"Wärmepumpe", "neue"},
		},
		{
			name: "de common noun after a question word", language: "de",
			text:    "Warum Strom teuer ist.",
			wantNot: []string{"Warum", "Warum Strom", "Strom"},
		},
		{
			// The model calls "Bankberatung" a name, so the filter keeps it.
			// "Wärmepumpe" and "Strom" above are the common nouns it drops.
			name: "de name after a question word is kept", language: "de",
			text:    "Warum Bankberatung teuer ist.",
			want:    []string{"Bankberatung"},
			wantNot: []string{"Warum", "Warum Bankberatung"},
		},
		{
			// One capitalised occurrence out of three is not a name. German
			// still keeps the acronym, and drops the lowercase copies.
			name: "de acronym stands alone", language: "de",
			text:    "Die ABC bleibt. Eine abc kommt. Noch eine abc folgt.",
			want:    []string{"ABC"},
			wantNot: []string{"abc"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := taggedCandidateTextsDroppingNouns(t, tc.text, tc.language)
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
		})
	}
}

func TestRareCommonNounStays(t *testing.T) {
	w := DefaultWeights
	w.DropLoneNouns = true
	w.LoneNounMinRarity = 0.6
	text := "See the chloroplast. The budget is small. Volunteers arrived yesterday. It is a Scottish company. The committee met Merkel yesterday."
	kws, err := ExtractWithWeights(text, "en", 1000, w)
	if err != nil {
		t.Fatal(err)
	}
	got := texts(kws)
	for _, w := range []string{"chloroplast", "Scottish", "Merkel"} {
		if !slices.Contains(got, w) {
			t.Errorf("missing %q in %q", w, got)
		}
	}
	for _, w := range []string{"budget", "Volunteers", "committee"} {
		if slices.Contains(got, w) {
			t.Errorf("unexpected %q in %q", w, got)
		}
	}
}

// TestNoKeywordContainsVerb checks every returned keyword of every example
// and benchmark article against the tokens flagged as verbs.
func TestNoKeywordContainsVerb(t *testing.T) {
	for _, f := range exampleFiles(t) {
		text := mustRead(t, f)
		language := langOf(f)
		pack, _ := lang.Get(language)
		if pack.Tagger() == nil {
			t.Fatalf("%s: no tagger for %s", f, language)
		}
		tokens, _ := tokenize(text, pack)
		markTags(text, tokens, pack.Tagger())
		kws, err := Extract(text, language, 1000)
		if err != nil {
			t.Fatal(err)
		}
		for _, k := range kws {
			for _, tok := range tokens {
				if tok.verb && tok.start < k.EndByte && k.StartByte < tok.end {
					t.Errorf("%s: keyword %q contains verb %q", f, k.Text, text[tok.start:tok.end])
				}
			}
		}
	}
}

func TestVerbRemovalOnKnownOutputs(t *testing.T) {
	readme := "La Banca d'Italia ha rivisto le stime. Il governatore della Banca d'Italia parla dei tassi di interesse e dei tassi di interesse reali."
	kws, err := Extract(readme, "it", 3)
	if err != nil {
		t.Fatal(err)
	}
	if got := texts(kws); slices.Contains(got, "rivisto") {
		t.Errorf("README example still returns the verb: %q", got)
	}

	kws, err = Extract(mustRead(t, "../examples/es/wiki-inteligencia-artificial.txt"), "es", 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, verbPhrase := range []string{"intentan replicar", "resolver problemas"} {
		if got := texts(kws); slices.Contains(got, verbPhrase) {
			t.Errorf("es article still returns %q: %q", verbPhrase, got)
		}
	}
}

// TestMostlyNonVerbWordKeepsPhrase covers the document-level vote: the
// Italian model tags "alta" as a verb in a minority of "acqua alta"
// occurrences, and the phrase must still be returned. Rarity can rank
// specific names above it, so the check is the full limit, not the top 10.
func TestMostlyNonVerbWordKeepsPhrase(t *testing.T) {
	kws, err := Extract(mustRead(t, "../examples/it/wiki-venezia.txt"), "it", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if got := texts(kws); !slices.Contains(got, "acqua alta") {
		t.Errorf("\"acqua alta\" not in %q", got)
	}
}

func TestSegment(t *testing.T) {
	text := "Dr. Smith's report, published today. It is short."
	sents, err := Segment(text, "en")
	if err != nil {
		t.Fatal(err)
	}
	got := make([][]string, 0, len(sents))
	for _, s := range sents {
		var words []string
		for _, sp := range s {
			words = append(words, text[sp.Start:sp.End])
		}
		got = append(got, words)
	}
	want := [][]string{
		{"Dr", ".", "Smith", "'s", "report", ",", "published", "today", "."},
		{"It", "is", "short", "."},
	}
	if !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("Segment = %q, want %q", got, want)
	}
	if _, err := Segment(text, "xx"); !errors.Is(err, ErrUnsupportedLanguage) {
		t.Errorf("unknown language: err = %v", err)
	}
}

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tericcabrel/keyphrase/extractor"
	"github.com/tericcabrel/keyphrase/internal/eval"
)

func mustParse(t *testing.T, s string) labels {
	t.Helper()
	l, err := parseLabels(strings.NewReader(s))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestParseLabels(t *testing.T) {
	l := mustParse(t, "# comment\n+\tWind Power\nA\twind\n\nI\tthe  Eiffel\u00a0Tower\n?\tl\u2019économie\r\n")
	for text, mark := range map[string]byte{"wind power": '+', "WIND": 'A', "the eiffel tower": 'I', "L'Économie": '?'} {
		if got := l[matchKey(text)].mark; got != mark {
			t.Errorf("%q: mark %q, want %q", text, got, mark)
		}
	}
	for _, bad := range []string{"x\tword\n", "+word\n", "++\tword\n", "+\t  \n", "+\tword\nA\tWord\n"} {
		if _, err := parseLabels(strings.NewReader(bad)); err == nil {
			t.Errorf("%q: want an error", bad)
		}
	}
	if _, err := parseLabels(strings.NewReader("+\tword\n+\tWord\n")); err != nil {
		t.Errorf("repeating a label with the same mark: %v", err)
	}
}

func TestScore(t *testing.T) {
	l := mustParse(t, "+\ta\n+\tb\nA\tc\nI\td\n?\te\n+\tnever returned\n")
	kws := make([]string, 0, 16)
	kws = append(kws, "a", "c", "x", "d", "e")
	for i := range 10 {
		kws = append(kws, "good"+string(rune('0'+i)))
		l[matchKey("good"+string(rune('0'+i)))] = label{mark: '+'}
	}
	kws = append(kws, "b")
	m := score("en/doc", kws, l)
	// Top 10: a + c A x ? d I e ? good0-4 +  -> 6 good of 8 judged.
	if m.Good[0] != 6 || m.Judged[0] != 8 {
		t.Errorf("top 10: %d good of %d judged, want 6 of 8", m.Good[0], m.Judged[0])
	}
	// Top 50: everything: 12 good of 14 judged.
	if m.Good[1] != 12 || m.Judged[1] != 14 {
		t.Errorf("top 50: %d good of %d judged, want 12 of 14", m.Good[1], m.Judged[1])
	}
	if m.Awkward != 1 || m.Irrelevant != 1 || m.Unjudged != 2 || m.Missed != 1 {
		t.Errorf("counts = %+v", m)
	}
	if p := new(metrics).precision(0); p != -1 {
		t.Errorf("precision with nothing judged = %v, want -1", p)
	}
}

func TestScoreCountsRankBands(t *testing.T) {
	keywords := make([]string, 130)
	for i := range keywords {
		keywords[i] = fmt.Sprintf("phrase %d", i)
	}
	// Ranks 1, 11, 51 and 101 are good; ranks 2 and 114 are awkward; rank
	// 107 is good. Past rank 100, phrases 106, 113, 114, 122, 127 and 128 of
	// "en/doc" are in the tail sample, so 101 does not count.
	l := mustParse(t, "+\tphrase 0\nA\tphrase 1\n+\tphrase 10\n+\tphrase 50\n+\tphrase 100\n+\tphrase 106\nA\tphrase 113\n")
	m := score("en/doc", keywords, l)
	tests := []struct {
		band                            string
		returned, sampled, judged, good int
	}{
		{"1-10", 10, 10, 2, 1},
		{"11-50", 40, 40, 1, 1},
		{"51-100", 50, 50, 1, 1},
		{"101+", 30, 6, 2, 1},
	}
	for band, tc := range tests {
		if name := bandName(band); name != tc.band {
			t.Errorf("band %d is named %q, want %q", band, name, tc.band)
		}
		if m.BandReturned[band] != tc.returned || m.BandSampled[band] != tc.sampled || m.BandJudged[band] != tc.judged || m.BandGood[band] != tc.good {
			t.Errorf("band %s: returned %d sampled %d judged %d good %d, want %d %d %d %d", tc.band,
				m.BandReturned[band], m.BandSampled[band], m.BandJudged[band], m.BandGood[band], tc.returned, tc.sampled, tc.judged, tc.good)
		}
	}
	// P@100 counts ranks 1-100 only; A and ? stop at labelDepth too.
	if m.Good[2] != 3 || m.Judged[2] != 4 {
		t.Errorf("top 100: %d good of %d judged, want 3 of 4", m.Good[2], m.Judged[2])
	}
	if m.Awkward != 1 || m.Unjudged != 96 {
		t.Errorf("awkward %d, unjudged %d in the first %d, want 1 and 96", m.Awkward, m.Unjudged, labelDepth)
	}
}

func TestAppendDraftTakesTheTopAndTheTailSample(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.labels.tsv")
	keywords := make([]string, 130)
	for i := range keywords {
		keywords[i] = fmt.Sprintf("phrase %d", i)
	}
	n, err := appendDraft(path, doc{Name: "en/doc", Keywords: keywords}, labels{})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	tail := lines[1+labelDepth:]
	want := []string{"?\tphrase 106", "?\tphrase 113", "?\tphrase 114", "?\tphrase 122", "?\tphrase 127", "?\tphrase 128"}
	if n != labelDepth+len(want) || strings.Join(tail, "|") != strings.Join(want, "|") {
		t.Errorf("appended %d, tail %q, want %d and %q", n, tail, labelDepth+len(want), want)
	}
}

func TestTailSampleIgnoresRank(t *testing.T) {
	top := make([]string, labelDepth)
	for i := range top {
		top[i] = fmt.Sprintf("top %d", i)
	}
	// Of "en/doc" phrases 100, 101 and 106, only 106 is in the tail sample.
	for _, tc := range []struct {
		name string
		tail []string
	}{
		{"phrase 106 at rank 101", []string{"phrase 106", "phrase 101"}},
		{"phrase 106 at rank 103", []string{"phrase 101", "phrase 100", "phrase 106"}},
	} {
		m := score("en/doc", append(slices.Clone(top), tc.tail...), labels{})
		if sampled := m.BandSampled[numBands-1]; sampled != 1 {
			t.Errorf("%s: %d tail keywords sampled, want 1", tc.name, sampled)
		}
	}
	inSample := 0
	for i := range 1000 {
		if inTailSample("en/doc", fmt.Sprintf("key %d", i)) {
			inSample++
		}
	}
	if inSample < 150 || inSample > 250 {
		t.Errorf("%d of 1000 keys sampled, want about %d", inSample, 1000/tailSampleEvery)
	}
}

func TestCompareCutsRunsToTheSmallerLimit(t *testing.T) {
	all := map[string]labels{"en/a": mustParse(t, "+\tgood\nA\ttail\n")}
	base := run{Time: "then", Limit: 1, Docs: []doc{{Name: "en/a", Language: "en", Keywords: []string{"good"}}}}
	cur := run{Limit: 0, Docs: []doc{{Name: "en/a", Language: "en", Keywords: []string{"good", "tail"}}}}
	var out strings.Builder
	compare(&out, base, cur, all, nil, false)
	got := out.String()
	if !strings.Contains(got, "both runs cut to the first 1 keywords") {
		t.Errorf("output lacks the cut:\n%s", got)
	}
	if strings.Contains(got, "tail") {
		t.Errorf("a keyword past the baseline's limit is listed as a change:\n%s", got)
	}
	for _, tc := range []struct{ a, b, want int }{{0, 0, 0}, {50, 0, 50}, {0, 100, 100}, {50, 100, 50}} {
		if got := commonLimit(tc.a, tc.b); got != tc.want {
			t.Errorf("commonLimit(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestGoodPerDocIgnoresHowManyKeywordsAreReturned(t *testing.T) {
	l := mustParse(t, "+\tgood one\n+\tgood two\nA\tbad one\nA\tbad two\n")
	tests := []struct {
		name          string
		keywords      []string
		wantPrecision float64
		wantGood      float64
	}{
		{"all four", []string{"good one", "bad one", "good two", "bad two"}, 0.5, 2},
		{"awkward ones left out", []string{"good one", "good two"}, 1, 2},
		{"one good one left out", []string{"good one", "bad one"}, 0.5, 1},
	}
	for _, tc := range tests {
		m := score("en/doc", tc.keywords, l)
		if got := m.precision(0); got != tc.wantPrecision {
			t.Errorf("%s: P@10 = %v, want %v", tc.name, got, tc.wantPrecision)
		}
		if got := m.goodPerDoc(0); got != tc.wantGood {
			t.Errorf("%s: G@10 = %v, want %v", tc.name, got, tc.wantGood)
		}
	}
}

func TestEvalWeightsDefaultsToTheShippedWeights(t *testing.T) {
	shipped := extractor.DefaultExtractOptions
	shippedRanking := eval.DefaultRanking
	keepAll := shipped
	keepAll.DropLoneNouns = false
	dropAll := shipped
	dropAll.LoneNounMinRarity = 0
	meanRarity := shippedRanking
	meanRarity.PhraseRarityMax = false
	tests := []struct {
		name          string
		dropLoneNouns bool
		loneMin       float64
		phrase        string
		want          extractor.ExtractOptions
		wantRanking   eval.Ranking
	}{
		{"flag defaults", shipped.DropLoneNouns, -1, "", shipped, shippedRanking},
		{"keep lone nouns", false, -1, "", keepAll, shippedRanking},
		{"drop every lone noun", shipped.DropLoneNouns, 0, "", dropAll, shippedRanking},
		{"mean rarity", shipped.DropLoneNouns, -1, "mean", shipped, meanRarity},
	}
	for _, tc := range tests {
		got, gotRanking, err := evalWeights(-1, tc.phrase, -1, tc.dropLoneNouns, tc.loneMin, shipped.DropOneOffPhrases, -1)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got != tc.want || gotRanking != tc.wantRanking {
			t.Errorf("%s: got %+v %+v, want %+v %+v", tc.name, got, gotRanking, tc.want, tc.wantRanking)
		}
	}
}

func TestShortTextEndsOnTheLineThatReachesTheCount(t *testing.T) {
	line := func(words int) string { return strings.TrimSpace(strings.Repeat("word ", words)) + "\n" }
	tests := []struct {
		name string
		text string
		want string
	}{
		{"cut after the line that reaches the count", line(100) + line(60) + line(10), line(100) + line(60)},
		{"a line exactly at the count", line(150) + line(10), line(150)},
		{"a text shorter than the count", line(20) + "last words", line(20) + "last words"},
		{"empty", "", ""},
	}
	for _, tc := range tests {
		if got := shortText(tc.text); got != tc.want {
			t.Errorf("%s: got %d words in %d lines, want %d words in %d lines", tc.name,
				len(strings.Fields(got)), strings.Count(got, "\n"), len(strings.Fields(tc.want)), strings.Count(tc.want, "\n"))
		}
	}
}

func TestDraftLabelsDoesNotRepeatAPhraseOfTheShortView(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "en"), 0o755); err != nil {
		t.Fatal(err)
	}
	docs := []doc{
		{Name: "en/page", Keywords: []string{"wind power", "grid"}},
		{Name: "en/page", Keywords: []string{"Wind Power", "offshore wind"}}, // the short view
		{Name: "en/held", Keywords: []string{"secret"}},
	}
	all := map[string]labels{"en/page": {}, "en/held": {}}
	var out strings.Builder
	if err := draftLabels(&out, dir, docs, all, func(name string) bool { return name != "en/held" }); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "en", "page.labels.tsv"))
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")[1:]
	if want := []string{"?\twind power", "?\tgrid", "?\toffshore wind"}; !slices.Equal(lines, want) {
		t.Errorf("labels = %q, want %q", lines, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "en", "held.labels.tsv")); err == nil {
		t.Error("a document include rejects was drafted")
	}
	if len(all["en/page"]) != 3 {
		t.Errorf("labels were not reloaded: %d", len(all["en/page"]))
	}
}

func TestDecodeText(t *testing.T) {
	if got, ok := decodeText([]byte(`Title\n\"quoted\" caf\u00e9` + "\n")); !ok || got != "Title\n\"quoted\" café" {
		t.Errorf("valid JSON string: %q, %v", got, ok)
	}
	got, ok := decodeText([]byte(`A "raw" quote\nnext\tline`))
	if ok || got != "A \"raw\" quote\nnext\tline" {
		t.Errorf("fallback: %q, %v", got, ok)
	}
}

func TestAppendDraft(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.labels.tsv")
	d := doc{Name: "en/doc", Keywords: []string{"wind power", "Wind  Power", "grid"}}
	n, err := appendDraft(path, d, labels{})
	if err != nil || n != 2 {
		t.Fatalf("first draft: n=%d err=%v, want 2", n, err)
	}
	// A manual correction must survive a second draft run.
	raw, _ := os.ReadFile(path)
	edited := strings.Replace(string(raw), "?\tgrid", "A\tgrid", 1)
	if writeErr := os.WriteFile(path, []byte(edited), 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}
	l, loadErr := loadLabels(path)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	d.Keywords = append(d.Keywords, "offshore wind")
	if n, err = appendDraft(path, d, l); err != nil || n != 1 {
		t.Fatalf("second draft: n=%d err=%v, want 1", n, err)
	}
	raw, _ = os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	want := []string{"?\twind power", "A\tgrid", "?\toffshore wind"}
	if len(lines) != 4 || !strings.HasPrefix(lines[0], "# ") || strings.Join(lines[1:], "|") != strings.Join(want, "|") {
		t.Errorf("file = %q", raw)
	}
	if n, _ := appendDraft(path, d, mustParse(t, strings.Join(lines, "\n"))); n != 0 {
		t.Errorf("nothing new: appended %d", n)
	}
}

func TestCompareUsesSharedDocuments(t *testing.T) {
	all := map[string]labels{
		"en/a": mustParse(t, "+\tgood\nA\tbad\n"),
		"en/b": mustParse(t, "A\tbad\n"),
	}
	base := run{Time: "then", Docs: []doc{{Name: "en/a", Language: "en", Keywords: []string{"good", "bad"}}}}
	cur := run{Docs: []doc{
		{Name: "en/a", Language: "en", Keywords: []string{"good"}},
		{Name: "en/b", Language: "en", Keywords: []string{"bad"}},
	}}
	var out strings.Builder
	compare(&out, base, cur, all, nil, false)
	got := out.String()
	// On the shared document "en/a" precision goes from 50% to 100%; the new
	// all-bad document "en/b" must not pull the delta down.
	for _, want := range []string{"1 shared documents", "50% -> 100% (+50)", "- A bad", "== new documents (not in baseline)", "en/b"} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "removed documents") {
		t.Errorf("no document was removed:\n%s", got)
	}
}

// Held-out documents are scored apart and their keywords never printed.
func TestHoldout(t *testing.T) {
	all := map[string]labels{
		"en/tune": mustParse(t, "+\tgood tune\n?\tunjudged tune\n"),
		"en/held": mustParse(t, "+\tsecret good\nA\tsecret bad\n?\tsecret unjudged\n"),
		"fr/held": mustParse(t, "A\tsecret français\n"),
	}
	holdout := map[string]bool{"en/held": true, "fr/held": true}
	base := run{Time: "then", Docs: []doc{
		{Name: "en/tune", Language: "en", Keywords: []string{"good tune"}},
		{Name: "en/held", Language: "en", Keywords: []string{"secret good", "secret bad"}},
	}}
	cur := run{Docs: []doc{
		{Name: "en/tune", Language: "en", Keywords: []string{"good tune", "unjudged tune"}},
		{Name: "en/held", Language: "en", Keywords: []string{"secret good", "secret unjudged"}},
		{Name: "fr/held", Language: "fr", Keywords: []string{"secret français"}},
	}}

	var out strings.Builder
	report(&out, cur, all, true, holdout, false)
	compare(&out, base, cur, all, holdout, false)
	got := out.String()
	for _, want := range []string{
		"en/tune", "? unjudged tune",
		"== held-out documents (2)", "en (1 docs)", "fr (1 docs)",
		"== held-out: 1 shared documents", "50% -> 100% (+50)",
		"1 held-out documents are not in the baseline.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
	for _, leak := range []string{"secret", "en/held", "fr/held"} {
		if strings.Contains(got, leak) {
			t.Errorf("output shows held-out %q:\n%s", leak, got)
		}
	}
}

func TestEvaluateAndSaveRun(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "en"), 0o755); err != nil {
		t.Fatal(err)
	}
	text := `Offshore wind farms are growing.\nOffshore wind capacity will reach 50 gigawatts. Offshore wind lowers energy bills.`
	if err := os.WriteFile(filepath.Join(dir, "en", "wind.txt"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "en", "wind.labels.tsv"), []byte("+\tOffshore wind\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn strings.Builder
	cur, all, err := evaluate(dir, 5, extractor.DefaultExtractOptions, eval.DefaultRanking, nil, &warn)
	if err != nil {
		t.Fatal(err)
	}
	if len(cur.Docs) != 1 || cur.Docs[0].Name != "en/wind" || len(cur.Docs[0].Keywords) == 0 {
		t.Fatalf("docs = %+v", cur.Docs)
	}
	if m := score("en/wind", cur.Docs[0].Keywords, all["en/wind"]); m.Good[0] != 1 {
		t.Errorf("Offshore wind not counted as good: %+v, keywords %q", m, cur.Docs[0].Keywords)
	}
	path, err := saveRun(dir, cur)
	if err != nil {
		t.Fatal(err)
	}
	back, err := loadRun(path)
	if err != nil || len(back.Docs) != 1 || back.Docs[0].Name != "en/wind" {
		t.Errorf("reloaded run = %+v, %v", back, err)
	}
}

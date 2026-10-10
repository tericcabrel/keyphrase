package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tericcabrel/keyphrase/extractor"
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
	m := score(kws, l)
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
	cur, all, err := evaluate(dir, 5, extractor.DefaultWeights, &warn)
	if err != nil {
		t.Fatal(err)
	}
	if len(cur.Docs) != 1 || cur.Docs[0].Name != "en/wind" || len(cur.Docs[0].Keywords) == 0 {
		t.Fatalf("docs = %+v", cur.Docs)
	}
	if m := score(cur.Docs[0].Keywords, all["en/wind"]); m.Good[0] != 1 {
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

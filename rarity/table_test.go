package rarity

import (
	"bytes"
	"compress/gzip"
	"math"
	"testing"
)

func TestBuildLookupAndRoundTrip(t *testing.T) {
	df := map[string]int{
		"people":      500,
		"chloroplast": 3,
		"once":        1,
		"":            100,
	}
	tab, err := Build(1000, df, MinCount)
	if err != nil {
		t.Fatal(err)
	}
	if tab.Words() != 2 || tab.Sentences() != 1000 {
		t.Fatalf("words %d sentences %d", tab.Words(), tab.Sentences())
	}
	if _, ok := tab.Score("once"); ok {
		t.Fatal("hapax was stored")
	}
	if _, ok := tab.Score("missing"); ok {
		t.Fatal("missing word was stored")
	}
	got, ok := tab.Score("chloroplast")
	if !ok || got != 1 {
		t.Fatalf("chloroplast = %v, %v", got, ok)
	}
	want := math.Log(1000.0/500) / math.Log(1000.0/3)
	got, ok = tab.Score("people")
	quant := math.Round(want*255) / 255
	if !ok || math.Abs(got-quant) > 1e-12 {
		t.Fatalf("people = %v, want %v", got, quant)
	}

	raw, err := tab.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	again, err := tab.Bytes()
	if err != nil || !bytes.Equal(raw, again) {
		t.Fatal("encoding is not deterministic")
	}
	back, err := Read(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if back.Words() != tab.Words() || back.Sentences() != tab.Sentences() {
		t.Fatalf("round trip %+v", back)
	}
	for _, w := range []string{"people", "chloroplast"} {
		a, _ := tab.Score(w)
		b, ok := back.Score(w)
		if !ok || a != b {
			t.Errorf("%s: %v vs %v ok=%v", w, a, b, ok)
		}
	}
}

func TestReadRejectsCorrupt(t *testing.T) {
	var buf bytes.Buffer
	z := gzip.NewWriter(&buf)
	if _, err := z.Write([]byte("nope")); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(&buf); err == nil {
		t.Fatal("corrupt table was accepted")
	}
}

func TestBuildRejectsBadCounts(t *testing.T) {
	if _, err := Build(0, nil, 3); err == nil {
		t.Fatal("zero sentences accepted")
	}
	if _, err := Build(2, nil, 3); err == nil {
		t.Fatal("min count above sentences accepted")
	}
}

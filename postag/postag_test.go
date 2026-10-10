package postag

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

// toyCorpus is a tiny training set in which "walk" is a noun after "a" and
// a verb after "they", so the tagger has to use context.
func toyCorpus() []Sentence {
	parse := func(s string) Sentence {
		var out Sentence
		for f := range strings.FieldsSeq(s) {
			word, tag, _ := strings.Cut(f, "/")
			t, ok := ParseTag(tag)
			if !ok {
				panic(tag)
			}
			out.Words = append(out.Words, word)
			out.Tags = append(out.Tags, t)
		}
		return out
	}
	sents := make([]Sentence, 0, 100)
	for range 20 {
		sents = append(sents,
			parse("They/PRON walk/VERB home/NOUN ./PUNCT"),
			parse("A/DET walk/NOUN helps/VERB ./PUNCT"),
			parse("They/PRON run/VERB fast/ADV ./PUNCT"),
			parse("A/DET run/NOUN helps/VERB ./PUNCT"),
			parse("Paris/PROPN has/AUX grown/VERB ./PUNCT"),
		)
	}
	return sents
}

func TestTrainLearnsContext(t *testing.T) {
	tagger := Train(toyCorpus(), TrainOptions{Iterations: 5, DictMin: 1000}, nil)
	got := tagger.Tag([]string{"A", "walk", "helps", "."})
	if want := []Tag{DET, NOUN, VERB, PUNCT}; !slices.Equal(got, want) {
		t.Errorf("noun context: got %v, want %v", got, want)
	}
	got = tagger.Tag([]string{"They", "walk", "home", "."})
	if want := []Tag{PRON, VERB, NOUN, PUNCT}; !slices.Equal(got, want) {
		t.Errorf("verb context: got %v, want %v", got, want)
	}
	if m := Evaluate(tagger, toyCorpus()); m.Accuracy() != 1 || m.VerbPrecision() != 1 || m.VerbRecall() != 1 {
		t.Errorf("training-set metrics %+v", m)
	}
}

func TestTagEmptyAndUnknownWords(t *testing.T) {
	tagger := Train(toyCorpus(), TrainOptions{Iterations: 2}, nil)
	if got := tagger.Tag(nil); len(got) != 0 {
		t.Errorf("empty sentence: %v", got)
	}
	got, margins := tagger.TagWithMargins([]string{"日本語", "🙂", "", "x"})
	if len(got) != 4 || len(margins) != 4 {
		t.Fatalf("got %d tags, %d margins", len(got), len(margins))
	}
	for _, tag := range got {
		if int(tag) >= NumTags {
			t.Errorf("invalid tag %d", tag)
		}
	}
}

func TestModelRoundTripIsDeterministic(t *testing.T) {
	var a, b bytes.Buffer
	ta := Train(toyCorpus(), TrainOptions{Iterations: 3, Seed: 7, DictMin: 30}, nil)
	if _, err := ta.WriteTo(&a); err != nil {
		t.Fatal(err)
	}
	tb := Train(toyCorpus(), TrainOptions{Iterations: 3, Seed: 7, DictMin: 30}, nil)
	if _, err := tb.WriteTo(&b); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("training twice with the same seed gave different model files")
	}

	read, err := ReadTagger(bytes.NewReader(a.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	f1, d1 := ta.Features()
	f2, d2 := read.Features()
	if f1 != f2 || d1 != d2 || d1 == 0 {
		t.Errorf("features/dictionary: wrote %d/%d, read %d/%d", f1, d1, f2, d2)
	}
	for _, s := range toyCorpus()[:5] {
		if got, want := read.Tag(s.Words), ta.Tag(s.Words); !slices.Equal(got, want) {
			t.Errorf("%q: reloaded model tags %v, original %v", s.Words, got, want)
		}
	}
}

func TestReadTaggerRejectsBadInput(t *testing.T) {
	var good bytes.Buffer
	if _, err := Train(toyCorpus(), TrainOptions{Iterations: 1}, nil).WriteTo(&good); err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"empty":     nil,
		"not gzip":  []byte("APOS1 plain"),
		"truncated": good.Bytes()[:good.Len()/2],
	}
	for name, data := range cases {
		if _, err := ReadTagger(bytes.NewReader(data)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestFeatureHelpers(t *testing.T) {
	if got := suffix("économie", 3); got != "mie" {
		t.Errorf("suffix = %q", got)
	}
	if got := suffix("été", 4); got != "été" {
		t.Errorf("short suffix = %q", got)
	}
	for in, want := range map[string]string{"Paris": "Xx", "COVID-19": "X-d", "e-mail": "x-x", "2024": "d"} {
		if got := shape(in); got != want {
			t.Errorf("shape(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"L’Europe": "l'europe", "2024": "!YEAR", "3,5": "!DIGITS", "Straße": "straße"} {
		if got := normalize(in); got != want {
			t.Errorf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseTag(t *testing.T) {
	for i := range NumTags {
		tag := Tag(i)
		got, ok := ParseTag(tag.String())
		if !ok || got != tag {
			t.Errorf("ParseTag(%q) = %v, %v", tag.String(), got, ok)
		}
	}
	if _, ok := ParseTag("NOPE"); ok {
		t.Error("ParseTag accepted an unknown name")
	}
	if !VERB.IsVerb() || !AUX.IsVerb() || NOUN.IsVerb() {
		t.Error("IsVerb")
	}
}

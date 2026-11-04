package treebank

import (
	"strings"
	"testing"

	"github.com/tericcabrel/keyphrase/postag"
)

func row(id, form, upos, misc string) string {
	return strings.Join([]string{id, form, "_", upos, "_", "_", "_", "_", "_", misc}, "\t") + "\n"
}

func TestReadRebuildsTextAndTags(t *testing.T) {
	input := "# text = Vino del mar.\n" +
		row("1", "Vino", "VERB", "_") +
		row("2-3", "del", "_", "_") +
		row("2", "de", "ADP", "_") +
		row("3", "el", "DET", "_") +
		row("3.1", "ghost", "NOUN", "_") +
		row("4", "mar", "NOUN", "SpaceAfter=No") +
		row("5", ".", "PUNCT", "_") +
		"\n" +
		row("1", "Hola", "INTJ", "_")

	sentences, err := Read(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(sentences) != 2 {
		t.Fatalf("got %d sentences, want 2", len(sentences))
	}
	tests := []struct {
		sentence int
		text     string
		words    []string
		tags     []postag.Tag
	}{
		{0, "Vino del mar. ", []string{"Vino", "del", "mar", "."}, []postag.Tag{postag.VERB, postag.ADP, postag.NOUN, postag.PUNCT}},
		{1, "Hola ", []string{"Hola"}, []postag.Tag{postag.INTJ}},
	}
	for _, tc := range tests {
		sentence := sentences[tc.sentence]
		if sentence.Text != tc.text {
			t.Errorf("sentence %d text = %q, want %q", tc.sentence, sentence.Text, tc.text)
		}
		if len(sentence.Words) != len(tc.words) {
			t.Fatalf("sentence %d: %d words, want %d", tc.sentence, len(sentence.Words), len(tc.words))
		}
		for i, word := range sentence.Words {
			if form := sentence.Text[word.Start:word.End]; form != tc.words[i] || word.Tag != tc.tags[i] {
				t.Errorf("sentence %d word %d = %q/%s, want %q/%s", tc.sentence, i, form, word.Tag, tc.words[i], tc.tags[i])
			}
		}
	}
}

func TestReadRejectsMalformedLines(t *testing.T) {
	tests := map[string]string{
		"too few columns": "1\tword\tNOUN\n",
		"unknown tag":     row("1", "word", "NOUNISH", "_"),
		"bad id":          row("x", "word", "NOUN", "_"),
		"bad range":       row("1-x", "word", "_", "_"),
	}
	for name, input := range tests {
		if _, err := Read(strings.NewReader(input)); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

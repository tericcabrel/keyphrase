package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/tericcabrel/keyphrase/extractor"
	"github.com/tericcabrel/keyphrase/postag"
)

func testLexicon(tags map[string]postag.Tag) lexicon {
	words := newLexicon()
	for form, tag := range tags {
		for range minLexiconCount {
			words.add(form, tag)
		}
	}
	return words
}

func keywordsAt(t *testing.T, text string, phrases ...string) []extractor.Keyword {
	t.Helper()
	keywords := make([]extractor.Keyword, len(phrases))
	for i, phrase := range phrases {
		start := strings.Index(text, phrase)
		if start < 0 {
			t.Fatalf("%q is not in the text", phrase)
		}
		keywords[i] = extractor.Keyword{Text: phrase, StartByte: start, EndByte: start + len(phrase)}
	}
	return keywords
}

func TestFindDefectsFlagsEachCheck(t *testing.T) {
	text := "Processo Seletivo Rápido\n" +
		"Os talentos de alta qualidade buscam empresas.\n" +
		"Criatividade\n" +
		"O código os torna ativos valiosos.\n" +
		"No final desse artigo há dicas.\n" +
		"Moustapha Thiam Neoklis Avdalas jogam.\n"
	words := testLexicon(map[string]postag.Tag{
		"alta": postag.ADJ, "qualidade": postag.NOUN, "criatividade": postag.NOUN,
		"torna": postag.VERB, "desse": postag.ADP, "empresas": postag.NOUN,
	})
	keywords := keywordsAt(t, text,
		"Processo Seletivo",     // 0: no defect
		"Seletivo Rápido",       // 1: fragment of rank 0
		"talentos de alta",      // 2: truncated before "qualidade"
		"Criatividade",          // 3: heading word
		"torna ativos valiosos", // 4: verb
		"desse artigo",          // 5: function word at the edge
		"empresas",              // 6: no defect
		"Moustapha Thiam",       // 7: no defect
		"Neoklis Avdalas",       // 8: touches rank 7, but both are names
	)
	tokens, err := segmentTokens(text, "pt")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		words lexicon
		want  map[string][]int
	}{
		{"with a lexicon", words, map[string][]int{
			checkFragment:         {1},
			checkTruncated:        {2},
			checkHeadingWord:      {3},
			checkVerb:             {4},
			checkFunctionWordEdge: {5},
		}},
		{"without a lexicon", lexicon{}, map[string][]int{checkFragment: {1}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := findDefects(text, tokens, keywords, tc.words)
			if len(got) != len(tc.want) {
				t.Errorf("checks = %v, want %v", got, tc.want)
			}
			for check, ranks := range tc.want {
				if !slices.Equal(got[check], ranks) {
					t.Errorf("%s flags ranks %v, want %v", check, got[check], ranks)
				}
			}
		})
	}
}

func TestLexiconPrefersTheFormAsWritten(t *testing.T) {
	words := testLexicon(map[string]postag.Tag{"Lernen": postag.NOUN, "lernen": postag.VERB})
	tests := []struct {
		form string
		verb bool
	}{
		{"Lernen", false},
		{"lernen", true},
		{"LERNEN", false}, // only the normalized form, half noun and half verb
		{"unbekannt", false},
	}
	for _, tc := range tests {
		if got := words.mostly(tc.form, postag.Tag.IsVerb); got != tc.verb {
			t.Errorf("mostly(%q, verb) = %v, want %v", tc.form, got, tc.verb)
		}
	}
}

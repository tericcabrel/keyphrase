package extractor

import (
	"math"
	"slices"
	"testing"

	"github.com/tericcabrel/keyphrase/lang"
	"github.com/tericcabrel/keyphrase/rarity"
)

func phraseAt(norms []string, start, n int) *phrase {
	occ := make([]occurrence, n)
	for i := range occ {
		occ[i] = occurrence{start: start}
	}
	return &phrase{contentNorms: norms, occurrences: occ, sentences: 1}
}

func TestRareWordOutranksCommon(t *testing.T) {
	pack, ok := lang.Get("en")
	if !ok {
		t.Fatal("no english pack")
	}
	table, err := rarity.Build(1000, map[string]int{"people": 500, "chloroplast": 3}, rarity.MinCount)
	if err != nil {
		t.Fatal(err)
	}
	phrases := []*phrase{
		phraseAt([]string{"people"}, 20, 1),
		phraseAt([]string{"chloroplast"}, 20, 1),
	}
	rank(phrases, nil, 100, Weights{Rarity: 1}, pack, table)
	if phrases[1].rawScore <= phrases[0].rawScore {
		t.Fatalf("rare %v, common %v", phrases[1].rawScore, phrases[0].rawScore)
	}
}

func TestPhraseRarityMeanAndMax(t *testing.T) {
	pack, _ := lang.Get("en")
	table, err := rarity.Build(1000, map[string]int{"people": 500, "chloroplast": 3}, rarity.MinCount)
	if err != nil {
		t.Fatal(err)
	}
	mean := []*phrase{phraseAt([]string{"people", "chloroplast"}, 10, 1)}
	maxP := []*phrase{phraseAt([]string{"people", "chloroplast"}, 10, 1)}
	rank(mean, nil, 100, Weights{Rarity: 1}, pack, table)
	rank(maxP, nil, 100, Weights{Rarity: 1, PhraseRarityMax: true}, pack, table)
	if maxP[0].rawScore <= mean[0].rawScore {
		t.Fatalf("max %v, mean %v", maxP[0].rawScore, mean[0].rawScore)
	}
	if maxP[0].rawScore != 1 {
		t.Fatalf("max rarity = %v, want 1", maxP[0].rawScore)
	}
}

func TestPluralUsesSingularRarity(t *testing.T) {
	pack, _ := lang.Get("en")
	table, err := rarity.Build(1000, map[string]int{"chloroplast": 100}, rarity.MinCount)
	if err != nil {
		t.Fatal(err)
	}
	want, ok := table.Score("chloroplast")
	if !ok || want == 1 {
		t.Fatalf("stem score %v ok=%v", want, ok)
	}
	phrases := []*phrase{phraseAt([]string{"chloroplasts"}, 10, 1)}
	rank(phrases, nil, 100, Weights{Rarity: 1}, pack, table)
	if math.Abs(phrases[0].rawScore-want) > 1e-12 {
		t.Fatalf("plural rarity %v, want stem %v", phrases[0].rawScore, want)
	}
}

func TestMissingTableDropsRarityWeight(t *testing.T) {
	pack, _ := lang.Get("en")
	without := []*phrase{phraseAt([]string{"people"}, 10, 2), phraseAt([]string{"weather"}, 80, 1)}
	with := []*phrase{phraseAt([]string{"people"}, 10, 2), phraseAt([]string{"weather"}, 80, 1)}
	base := Weights{Frequency: 1, Early: 1}
	rank(without, nil, 100, base, pack, nil)
	base.Rarity = 5
	rank(with, nil, 100, base, pack, nil)
	for i := range without {
		if math.Abs(without[i].rawScore-with[i].rawScore) > 1e-12 {
			t.Fatalf("phrase %d: without table %v, with unused rarity weight %v", i, without[i].rawScore, with[i].rawScore)
		}
	}
}

func TestMinSingleRarityDropsCommonWords(t *testing.T) {
	pack, _ := lang.Get("en")
	table, err := rarity.Build(1000, map[string]int{"people": 500, "chloroplast": 3}, rarity.MinCount)
	if err != nil {
		t.Fatal(err)
	}
	phrases := []*phrase{
		phraseAt([]string{"people"}, 10, 1),
		phraseAt([]string{"chloroplast"}, 10, 1),
		phraseAt([]string{"people", "chloroplast"}, 10, 2),
	}
	got := rank(phrases, nil, 100, Weights{Rarity: 1, MinSingleRarity: 0.5}, pack, table)
	for _, p := range got {
		if len(p.contentNorms) == 1 && p.contentNorms[0] == "people" {
			t.Fatal("common single word kept")
		}
	}
	if len(got) != 2 {
		t.Fatalf("kept %d phrases, want the rare word and the phrase", len(got))
	}
}

func TestNormsMatchesTokens(t *testing.T) {
	const text = "L'économie d'aujourd'hui."
	pack, ok := lang.Get("fr")
	if !ok {
		t.Fatal("no french pack")
	}
	tokens, _ := tokenize(text, pack)
	var want []string
	for i := range tokens {
		if tokens[i].kind == kindWord {
			want = append(want, tokens[i].norm)
		}
	}
	got, err := Norms(text, "fr")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Norms %q, tokens %q", got, want)
	}
	if _, err := Norms(text, "xx"); err != ErrUnsupportedLanguage {
		t.Fatalf("bad language: %v", err)
	}
}

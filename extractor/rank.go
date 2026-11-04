package extractor

import (
	"math"
	"slices"

	"github.com/tericcabrel/keyphrase/lang"
	"github.com/tericcabrel/keyphrase/rarity"
)

type weights struct {
	frequency         float64
	early             float64
	spread            float64
	capitalization    float64
	cohesion          float64
	rarity            float64
	phraseRarityMax   bool
	minSingleRarity   float64
	dropLoneNouns     bool
	loneNounMinRarity float64
	dropOneOffPhrases bool
	oneOffMinOthers   int
}

// The six feature weights stay at 1. Changing one is a quality decision
// recorded in docs/quality-history.md.
func weightsFor(options ExtractOptions, ranking ranking) weights {
	return weights{
		frequency:         1,
		early:             1,
		spread:            1,
		capitalization:    1,
		cohesion:          1,
		rarity:            ranking.Rarity,
		phraseRarityMax:   ranking.PhraseRarityMax,
		minSingleRarity:   ranking.MinSingleRarity,
		dropLoneNouns:     options.DropLoneNouns,
		loneNounMinRarity: options.LoneNounMinRarity,
		dropOneOffPhrases: options.DropOneOffPhrases,
		oneOffMinOthers:   options.OneOffMinOthers,
	}
}

type features struct {
	frequency, early, spread, capitalization, cohesion, rarity float64
}

func computeFeatures(phrase *phrase, maxCount, maxSpread, textLen int, wordCount map[string]int, rarityScore float64) features {
	occurrences := len(phrase.occurrences)
	signals := features{
		frequency: math.Log1p(float64(occurrences)) / math.Log1p(float64(maxCount)),
		early:     1 - float64(phrase.occurrences[0].start)/float64(textLen),
		spread:    math.Log1p(float64(phrase.sentences)) / math.Log1p(float64(maxSpread)),
		rarity:    rarityScore,
	}
	capitalized := 0
	for _, occurrence := range phrase.occurrences {
		if occurrence.capitalized {
			capitalized++
		}
	}
	signals.capitalization = float64(capitalized) / float64(occurrences)
	if phrase.multiword() && occurrences >= 2 {
		maxWordCount := 0
		for _, norm := range phrase.contentNorms {
			maxWordCount = max(maxWordCount, wordCount[norm])
		}
		signals.cohesion = min(1, float64(occurrences)/float64(max(maxWordCount, 1)))
	}
	return signals
}

func (w weights) score(signals features, useRarity bool) float64 {
	total := w.frequency + w.early + w.spread + w.capitalization + w.cohesion
	weightedSum := w.frequency*signals.frequency + w.early*signals.early + w.spread*signals.spread +
		w.capitalization*signals.capitalization + w.cohesion*signals.cohesion
	if useRarity {
		total += w.rarity
		weightedSum += w.rarity * signals.rarity
	}
	if total == 0 {
		return 0
	}
	return weightedSum / total
}

func wordRarity(table *rarity.Table, pack lang.Pack, norm string) float64 {
	if score, ok := table.Score(norm); ok {
		return score
	}
	if variantKey := pack.VariantKey(norm); variantKey != norm {
		if score, ok := table.Score(variantKey); ok {
			return score
		}
	}
	return 1
}

func rarityOf(table *rarity.Table, pack lang.Pack, norms []string, takeMax bool) float64 {
	if len(norms) == 0 {
		return 0
	}
	sum, maxScore := 0.0, 0.0
	for _, norm := range norms {
		score := wordRarity(table, pack, norm)
		sum += score
		maxScore = max(maxScore, score)
	}
	if takeMax || len(norms) == 1 {
		return maxScore
	}
	return sum / float64(len(norms))
}

func dropByRarity(phrases []*phrase, tokens []token, weights weights, pack lang.Pack, table *rarity.Table) []*phrase {
	if table == nil || (weights.minSingleRarity == 0 && !(weights.dropLoneNouns && weights.loneNounMinRarity > 0)) {
		return phrases
	}
	kept := phrases[:0]
	for _, phrase := range phrases {
		if phrase.multiword() {
			kept = append(kept, phrase)

			continue
		}
		score := rarityOf(table, pack, phrase.contentNorms, false)
		if weights.minSingleRarity > 0 && score < weights.minSingleRarity {
			continue
		}
		if weights.dropLoneNouns && weights.loneNounMinRarity > 0 && score < weights.loneNounMinRarity && loneCommonNoun(phrase, tokens) {
			continue
		}
		kept = append(kept, phrase)
	}

	return kept
}

func rank(phrases []*phrase, tokens []token, textLen int, weights weights, pack lang.Pack, table *rarity.Table) []*phrase {
	phrases = dropByRarity(phrases, tokens, weights, pack, table)
	if weights.dropOneOffPhrases {
		phrases = dropOneOffPhrases(phrases, tokens, weights.oneOffMinOthers)
	}
	wordCount := map[string]int{}
	for i := range tokens {
		if tokens[i].kind == kindWord {
			wordCount[tokens[i].norm]++
		}
	}
	maxCount, maxSpread := 1, 1
	for _, phrase := range phrases {
		maxCount = max(maxCount, len(phrase.occurrences))
		maxSpread = max(maxSpread, phrase.sentences)
	}
	useRarity := table != nil && weights.rarity != 0
	for _, phrase := range phrases {
		rarityScore := 0.0
		if useRarity {
			rarityScore = rarityOf(table, pack, phrase.contentNorms, weights.phraseRarityMax)
		}
		phrase.rawScore = weights.score(computeFeatures(phrase, maxCount, maxSpread, textLen, wordCount, rarityScore), useRarity)
	}
	return phrases
}

func dropOneOffPhrases(phrases []*phrase, tokens []token, minOthers int) []*phrase {
	others := 0
	for _, phrase := range phrases {
		if !oneOffCommonPhrase(phrase, tokens) {
			others++
		}
	}
	if others < minOthers {
		return phrases
	}
	return slices.DeleteFunc(phrases, func(phrase *phrase) bool { return oneOffCommonPhrase(phrase, tokens) })
}

func oneOffCommonPhrase(phrase *phrase, tokens []token) bool {
	if !phrase.multiword() || len(phrase.occurrences) > 1 {
		return false
	}
	only := phrase.occurrences[0]
	if only.capitalized {
		return false
	}
	for i := only.first; i <= only.last; i++ {
		if tokens[i].kind == kindWord && tokens[i].name {
			return false
		}
	}
	return true
}

func loneCommonNoun(phrase *phrase, tokens []token) bool {
	for _, occurrence := range phrase.occurrences {
		if tokens[occurrence.first].name || occurrence.capitalized {
			return false
		}
	}
	return true
}

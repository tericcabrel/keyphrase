package extractor

import (
	"math"

	"github.com/tericcabrel/keyphrase/lang"
	"github.com/tericcabrel/keyphrase/rarity"
)

// Weights are the feature weights of the ranking formula. Each feature is
// normalized to [0, 1] and the raw score is the weighted mean, so raw scores
// are also in [0, 1].
type Weights struct {
	Frequency      float64
	Early          float64
	Spread         float64
	Capitalization float64
	Cohesion       float64
	Rarity         float64
	// PhraseRarityMax reports whether a multi-word phrase takes the highest
	// rarity among its content words. Otherwise it takes the mean.
	PhraseRarityMax bool
	// MinSingleRarity drops a one-word candidate whose rarity is below this.
	// Zero disables the rule. It needs a rarity table.
	MinSingleRarity float64
	// DropLoneNouns drops a one-word candidate that the tagger does not call
	// a name, unless the word is capitalized like a name. Zero leaves lone
	// common nouns in place.
	DropLoneNouns bool
	// LoneNounMinRarity, when positive, keeps a lone common noun whose rarity
	// is at least this. It applies only together with DropLoneNouns, and it
	// needs a rarity table. Zero drops every lone common noun.
	LoneNounMinRarity float64
}

// DefaultWeights starts from equal weights, as the design requires. Changes
// must be justified by the quality review and recorded in docs/quality-history.md.
var DefaultWeights = Weights{
	Frequency:         1,
	Early:             1,
	Spread:            1,
	Capitalization:    1,
	Cohesion:          1,
	Rarity:            1,
	PhraseRarityMax:   true,
	DropLoneNouns:     true,
	LoneNounMinRarity: 0.6,
}

// features are the normalized ranking signals of one phrase.
type features struct {
	frequency, early, spread, capitalization, cohesion, rarity float64
}

// computeFeatures derives the ranking signals:
//
//	frequency      log(1+occurrences) / log(1+occurrences of the most
//	               frequent candidate)
//	early          1 - first occurrence byte offset / text length
//	spread         log(1+distinct sentences containing it) / log(1+the
//	               largest such count among candidates)
//	capitalization capitalized occurrences / occurrences (see isCapitalized)
//	cohesion       single words and phrases seen once: 0; repeated phrases:
//	               occurrences / occurrences of its most frequent content
//	               word anywhere in the text
//	rarity         how uncommon the phrase's content words are in a general
//	               corpus (see package rarity). A single word uses its own
//	               score. A phrase uses the mean or the maximum of its
//	               content words (Weights.PhraseRarityMax). Unknown words
//	               score 1. When the language has no table the feature is
//	               left out of the mean.
//
// A phrase seen once is not evidence that its words belong together; without
// the repetition requirement every one-off bigram scores 1 on cohesion.
// Frequency and spread are log-scaled because counts are heavy-tailed: with
// linear scaling one very frequent word compresses every other candidate
// towards 0, and position then dominates in long documents.
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

func (w Weights) score(signals features, useRarity bool) float64 {
	total := w.Frequency + w.Early + w.Spread + w.Capitalization + w.Cohesion
	weightedSum := w.Frequency*signals.frequency + w.Early*signals.early + w.Spread*signals.spread +
		w.Capitalization*signals.capitalization + w.Cohesion*signals.cohesion
	if useRarity {
		total += w.Rarity
		weightedSum += w.Rarity * signals.rarity
	}
	if total == 0 {
		return 0
	}
	return weightedSum / total
}

// wordRarity is the table score of a normalized word. A missing word is
// looked up again under the pack's variant key (so a plural can use the
// singular's count) and scores 1 when it is still unknown.
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

// rarityOf scores one phrase. A single word uses its own score. A longer
// phrase uses the mean of its content words, or the maximum when takeMax is set.
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

// dropByRarity removes one-word candidates the rarity table rejects.
func dropByRarity(phrases []*phrase, tokens []token, weights Weights, pack lang.Pack, table *rarity.Table) []*phrase {
	if table == nil || (weights.MinSingleRarity == 0 && !(weights.DropLoneNouns && weights.LoneNounMinRarity > 0)) {
		return phrases
	}
	kept := phrases[:0]
	for _, phrase := range phrases {
		if phrase.multiword() {
			kept = append(kept, phrase)

			continue
		}
		score := rarityOf(table, pack, phrase.contentNorms, false)
		if weights.MinSingleRarity > 0 && score < weights.MinSingleRarity {
			continue
		}
		if weights.DropLoneNouns && weights.LoneNounMinRarity > 0 && score < weights.LoneNounMinRarity && loneCommonNoun(phrase, tokens) {
			continue
		}
		kept = append(kept, phrase)
	}

	return kept
}

// rank assigns rawScore for every phrase. A one-word candidate below
// Weights.MinSingleRarity is dropped when a table is available. With
// DropLoneNouns and a positive LoneNounMinRarity, a lone common noun is
// dropped unless it is rare enough. The rarity weight is left out of the
// mean when table is nil.
func rank(phrases []*phrase, tokens []token, textLen int, weights Weights, pack lang.Pack, table *rarity.Table) []*phrase {
	phrases = dropByRarity(phrases, tokens, weights, pack, table)
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
	useRarity := table != nil && weights.Rarity != 0
	for _, phrase := range phrases {
		rarityScore := 0.0
		if useRarity {
			rarityScore = rarityOf(table, pack, phrase.contentNorms, weights.PhraseRarityMax)
		}
		phrase.rawScore = weights.score(computeFeatures(phrase, maxCount, maxSpread, textLen, wordCount, rarityScore), useRarity)
	}
	return phrases
}

// loneCommonNoun reports whether a one-word phrase is neither a name nor
// capitalized like one. The name flag is the same on every occurrence of a
// word; capitalization can differ, and one capitalized occurrence is enough.
func loneCommonNoun(phrase *phrase, tokens []token) bool {
	for _, occurrence := range phrase.occurrences {
		if tokens[occurrence.first].name || occurrence.capitalized {
			return false
		}
	}
	return true
}

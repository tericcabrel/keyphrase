package postag

import (
	"maps"
	"math"
	"math/rand/v2"
	"slices"
)

// Sentence is one training or evaluation sentence: surface words and their
// correct tags.
type Sentence struct {
	Words []string
	Tags  []Tag
}

// TrainOptions controls Train. Zero fields take the defaults noted below.
type TrainOptions struct {
	Iterations int     // passes over the data; default 8
	Seed       uint64  // shuffle seed, so training is reproducible
	DictMin    int     // minimum count for a tag-dictionary word; default 20
	DictShare  float64 // minimum share of the majority tag; default 0.97
	MinWeight  float64 // averaged weights below this magnitude are dropped
}

// param is one feature-tag weight during training. total and lastStep
// implement averaging: total accumulates the weight over every update step
// since training began.
type param struct {
	tag      Tag
	weight   float64
	total    float64
	lastStep int
}

// Train learns a Tagger from tagged sentences with the averaged perceptron
// algorithm: tag each word with the current weights and, when the guess is
// wrong, move weight from the guessed tag's features to the correct tag's.
// The final weights are the average over all steps, which generalizes
// better than the last ones. progress, if non-nil, is called after each pass
// with the share of training words tagged correctly during that pass.
func Train(sentences []Sentence, options TrainOptions, progress func(iter int, accuracy float64)) *Tagger {
	if options.Iterations == 0 {
		options.Iterations = 8
	}
	if options.DictMin == 0 {
		options.DictMin = 20
	}
	if options.DictShare == 0 {
		options.DictShare = 0.97
	}
	tagDict := buildTagDict(sentences, options.DictMin, options.DictShare)
	weights := map[uint64][]param{}
	step := 0

	update := func(truth, guess Tag, featureHashes []uint64) {
		step++
		if truth == guess {
			return
		}
		for _, featureHash := range featureHashes {
			params := weights[featureHash]
			params = bump(params, truth, 1, step)
			params = bump(params, guess, -1, step)
			weights[featureHash] = params
		}
	}

	order := make([]int, len(sentences))
	for i := range order {
		order[i] = i
	}
	rng := rand.New(rand.NewPCG(options.Seed, options.Seed^0x9e3779b97f4a7c15))
	featureHashes := make([]uint64, 0, numFeatures)
	for iter := 1; iter <= options.Iterations; iter++ {
		correct, total := 0, 0
		for _, sentenceIndex := range order {
			sentence := sentences[sentenceIndex]
			sentenceContext := newContext(sentence.Words)
			prev, prev2 := tagStart, tagStart2
			for i := range sentence.Words {
				guess, ok := tagDict[sentenceContext.norms[i+2]]
				if !ok {
					featureHashes = sentenceContext.features(featureHashes[:0], i, prev, prev2)
					guess = predictTraining(weights, featureHashes)
					update(sentence.Tags[i], guess, featureHashes)
				}
				if guess == sentence.Tags[i] {
					correct++
				}
				total++
				prev2, prev = prev, byte(guess)
			}
		}
		if progress != nil && total > 0 {
			progress(iter, float64(correct)/float64(total))
		}
		rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	}
	return finalize(weights, tagDict, step, options.MinWeight)
}

func bump(params []param, tag Tag, delta float64, step int) []param {
	for i := range params {
		if params[i].tag == tag {
			entry := &params[i]
			entry.total += float64(step-entry.lastStep) * entry.weight
			entry.lastStep = step
			entry.weight += delta
			return params
		}
	}
	return append(params, param{tag: tag, weight: delta, lastStep: step})
}

func predictTraining(weights map[uint64][]param, featureHashes []uint64) Tag {
	var scores [NumTags]float64
	for _, featureHash := range featureHashes {
		for _, entry := range weights[featureHash] {
			scores[entry.tag] += entry.weight
		}
	}
	return argmax(scores[:])
}

// finalize averages the weights, drops small ones, and packs the rest.
func finalize(weights map[uint64][]param, tagDict map[string]Tag, steps int, minWeight float64) *Tagger {
	tagger := &Tagger{tagDict: tagDict, weights: make(map[uint64]weightRef, len(weights))}
	for _, featureHash := range slices.Sorted(maps.Keys(weights)) {
		params := slices.Clone(weights[featureHash])
		slices.SortFunc(params, func(a, b param) int { return int(a.tag) - int(b.tag) })
		offset := uint32(len(tagger.tags))
		for _, entry := range params {
			average := (entry.total + float64(steps-entry.lastStep)*entry.weight) / float64(steps)
			if math.Abs(average) < minWeight || average == 0 {
				continue
			}
			tagger.tags = append(tagger.tags, entry.tag)
			tagger.values = append(tagger.values, float32(average))
		}
		if count := uint32(len(tagger.tags)) - offset; count > 0 {
			tagger.weights[featureHash] = weightRef{offset: offset, count: uint8(count)}
		}
	}
	return tagger
}

// buildTagDict returns the words that occur at least minCount times and
// carry one tag in at least share of their occurrences.
func buildTagDict(sentences []Sentence, minCount int, share float64) map[string]Tag {
	counts := map[string]*[NumTags]int{}
	for _, sentence := range sentences {
		for i, word := range sentence.Words {
			norm := normalize(word)
			tagCounts := counts[norm]
			if tagCounts == nil {
				tagCounts = new([NumTags]int)
				counts[norm] = tagCounts
			}
			tagCounts[sentence.Tags[i]]++
		}
	}
	dict := map[string]Tag{}
	for word, tagCounts := range counts {
		total, best := 0, 0
		for tagIndex, count := range tagCounts {
			total += count
			if count > tagCounts[best] {
				best = tagIndex
			}
		}
		if total >= minCount && float64(tagCounts[best]) >= share*float64(total) {
			dict[word] = Tag(best)
		}
	}
	return dict
}

// Metrics summarizes tagging accuracy on labelled sentences. Verb counts
// treat VERB and AUX as one class, since that is the distinction the
// extractor uses.
type Metrics struct {
	Words, Correct int
	// VerbTP: verbs tagged as verbs. VerbFP: non-verbs tagged as verbs.
	// VerbFN: verbs tagged as something else.
	VerbTP, VerbFP, VerbFN int
}

func (m Metrics) Accuracy() float64      { return ratio(m.Correct, m.Words) }
func (m Metrics) VerbPrecision() float64 { return ratio(m.VerbTP, m.VerbTP+m.VerbFP) }
func (m Metrics) VerbRecall() float64    { return ratio(m.VerbTP, m.VerbTP+m.VerbFN) }

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

// Evaluate tags every sentence and compares the result with its tags.
func Evaluate(tagger *Tagger, sentences []Sentence) Metrics {
	var metrics Metrics
	for _, sentence := range sentences {
		got := tagger.Tag(sentence.Words)
		for i, want := range sentence.Tags {
			metrics.Words++
			if got[i] == want {
				metrics.Correct++
			}
			switch {
			case want.IsVerb() && got[i].IsVerb():
				metrics.VerbTP++
			case got[i].IsVerb():
				metrics.VerbFP++
			case want.IsVerb():
				metrics.VerbFN++
			}
		}
	}
	return metrics
}

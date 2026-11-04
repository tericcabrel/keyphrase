package postag

import (
	"maps"
	"math"
	"math/rand/v2"
	"slices"
)

type Sentence struct {
	Words []string
	Tags  []Tag
}

type TrainOptions struct {
	Iterations int
	Seed       uint64
	DictMin    int
	DictShare  float64
	MinWeight  float64
}

type param struct {
	tag      Tag
	weight   float64
	total    float64
	lastStep int
}

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

type Metrics struct {
	Words, Correct         int
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

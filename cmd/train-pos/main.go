// Command train-pos trains a part-of-speech model from CoNLL-U treebanks.
//
// Each sentence is split with the extractor's tokenizer, and each token gets
// the tag of the first non-punctuation treebank word it covers.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/tericcabrel/keyphrase/extractor"
	"github.com/tericcabrel/keyphrase/internal/treebank"
	"github.com/tericcabrel/keyphrase/postag"

	_ "github.com/tericcabrel/keyphrase/lang/all"
)

func main() {
	language := flag.String("lang", "", "language code (en, fr, de, es, it, pt)")
	train := flag.String("train", "", "comma-separated CoNLL-U training files")
	test := flag.String("test", "", "comma-separated CoNLL-U evaluation files")
	out := flag.String("out", "", "model output path (.bin.gz)")
	iters := flag.Int("iters", 8, "training passes")
	minWeight := flag.Float64("min-weight", 0, "drop averaged weights below this magnitude")
	seed := flag.Uint64("seed", 1, "shuffle seed")
	show := flag.String("show", "", "after training, print the tags of this text")
	flag.Parse()
	if *language == "" || *train == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	log.SetFlags(0)

	trainSents, stats, err := load(*language, *train)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("train: %s", stats)

	tagger := postag.Train(trainSents, postag.TrainOptions{
		Iterations: *iters,
		Seed:       *seed,
		MinWeight:  *minWeight,
	}, func(iter int, acc float64) {
		log.Printf("pass %d: training accuracy %.2f%%", iter, 100*acc)
	})
	features, dictWords := tagger.Features()
	log.Printf("model: %d features, %d dictionary words", features, dictWords)

	if *test != "" {
		testSents, stats, testErr := load(*language, *test)
		if testErr != nil {
			log.Fatal(testErr)
		}
		log.Printf("test: %s", stats)
		m := postag.Evaluate(tagger, testSents)
		log.Printf("test: accuracy %.2f%%, verb precision %.2f%%, verb recall %.2f%% (%d words)",
			100*m.Accuracy(), 100*m.VerbPrecision(), 100*m.VerbRecall(), m.Words)
	}

	f, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	n, err := tagger.WriteTo(f)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s (%d bytes)", *out, n)

	if *show != "" {
		sents, err := extractor.Segment(*show, *language)
		if err != nil {
			log.Fatal(err)
		}
		for _, spans := range sents {
			words := make([]string, len(spans))
			for i, s := range spans {
				words[i] = (*show)[s.Start:s.End]
			}
			var b strings.Builder
			for i, tag := range tagger.Tag(words) {
				_, _ = fmt.Fprintf(&b, "%s/%s ", words[i], tag)
			}
			fmt.Println(strings.TrimSpace(b.String()))
		}
	}
}

type alignStats struct {
	files, treebankSentences, sentences, words int
	merged, split                              int // one token over several treebank words, or one word cut into several tokens
}

func (s alignStats) String() string {
	return fmt.Sprintf("%d files, %d treebank sentences -> %d sentences, %d tokens (%d merged, %d split)",
		s.files, s.treebankSentences, s.sentences, s.words, s.merged, s.split)
}

func load(language, paths string) ([]postag.Sentence, alignStats, error) {
	var out []postag.Sentence
	var stats alignStats
	for path := range strings.SplitSeq(paths, ",") {
		f, err := os.Open(path)
		if err != nil {
			return nil, stats, err
		}
		tbSents, readErr := treebank.Read(f)
		_ = f.Close()
		if readErr != nil {
			return nil, stats, fmt.Errorf("%s: %w", path, readErr)
		}
		stats.files++
		for _, tb := range tbSents {
			stats.treebankSentences++
			sents, err := align(language, tb, &stats)
			if err != nil {
				return nil, stats, err
			}
			out = append(out, sents...)
		}
	}
	return out, stats, nil
}

func align(language string, tb treebank.Sentence, stats *alignStats) ([]postag.Sentence, error) {
	spans, err := extractor.Segment(tb.Text, language)
	if err != nil {
		return nil, err
	}
	pieces := make([]int, len(tb.Words))
	var out []postag.Sentence
	w := 0
	for _, sent := range spans {
		s := postag.Sentence{}
		for _, sp := range sent {
			for w < len(tb.Words) && tb.Words[w].End <= sp.Start {
				w++
			}
			tag, found, overlaps := postag.PUNCT, false, 0
			for k := w; k < len(tb.Words) && tb.Words[k].Start < sp.End; k++ {
				overlaps++
				pieces[k]++
				if !found && tb.Words[k].Tag != postag.PUNCT {
					tag, found = tb.Words[k].Tag, true
				}
			}
			if overlaps == 0 {
				continue
			}
			if overlaps > 1 {
				stats.merged++
			}
			s.Words = append(s.Words, tb.Text[sp.Start:sp.End])
			s.Tags = append(s.Tags, tag)
		}
		if len(s.Words) > 0 {
			out = append(out, s)
			stats.sentences++
			stats.words += len(s.Words)
		}
	}
	for _, p := range pieces {
		if p > 1 {
			stats.split++
		}
	}
	return out, nil
}

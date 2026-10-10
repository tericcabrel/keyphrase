// Command train-pos trains a part-of-speech model for package postag
// from Universal Dependencies treebanks in CoNLL-U format.
//
// It rebuilds each treebank sentence's text, splits it with the extractor's
// own tokenizer, and gives every resulting token the tag of the treebank
// word it covers. The model therefore learns on exactly the tokens it will
// see in production, whatever the treebank's own word splitting.
//
//	go run ./cmd/train-pos -lang en \
//	    -train postag/models/ud/UD_English-EWT/en_ewt-ud-train.conllu \
//	    -test postag/models/ud/UD_English-EWT/en_ewt-ud-test.conllu \
//	    -out lang/en/model.bin.gz
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/tericcabrel/keyphrase/extractor"
	"github.com/tericcabrel/keyphrase/postag"

	_ "github.com/tericcabrel/keyphrase/lang/all"
)

func main() {
	language := flag.String("lang", "", "language code (en, fr, de, es, it)")
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

// alignStats counts how treebank words and extractor tokens lined up.
type alignStats struct {
	files, treebankSentences, sentences, words int
	// merged: extractor tokens covering several treebank words ("don't"
	// for "do" + "n't"). split: treebank words cut into several extractor
	// tokens.
	merged, split int
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
		tbSents, readErr := readCoNLLU(f)
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

// surfaceWord is one treebank token as written in the text. A multiword
// token such as Spanish "del" (de + el) is one surface word carrying the
// tag of its first part.
type surfaceWord struct {
	start, end int
	tag        postag.Tag
}

// unsetTag marks a multiword token whose first part has not been read yet.
const unsetTag = postag.Tag(255)

type treebankSentence struct {
	text  string
	words []surfaceWord
}

// readCoNLLU parses CoNLL-U sentences and rebuilds each sentence's text from
// its surface words and SpaceAfter=No marks.
func readCoNLLU(r io.Reader) ([]treebankSentence, error) {
	var out []treebankSentence
	var text strings.Builder
	var words []surfaceWord
	rangeEnd := 0 // last word id covered by the current multiword token
	flush := func() {
		for i := range words {
			if words[i].tag == unsetTag {
				words[i].tag = postag.X
			}
		}
		if len(words) > 0 {
			out = append(out, treebankSentence{text: text.String(), words: words})
		}
		text.Reset()
		words = nil
		rangeEnd = 0
	}
	addSurface := func(form, misc string) {
		start := text.Len()
		text.WriteString(form)
		words = append(words, surfaceWord{start: start, end: text.Len()})
		if !strings.Contains(misc, "SpaceAfter=No") {
			text.WriteByte(' ')
		}
	}

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	line := 0
	for sc.Scan() {
		line++
		l := sc.Text()
		if l == "" {
			flush()

			continue
		}
		if strings.HasPrefix(l, "#") {
			continue
		}
		cols := strings.Split(l, "\t")
		if len(cols) != 10 {
			return nil, fmt.Errorf("line %d: %d columns, want 10", line, len(cols))
		}
		id, form, upos, misc := cols[0], cols[1], cols[3], cols[9]
		if strings.Contains(id, ".") {
			continue // empty node: not in the text
		}
		if from, to, ok := strings.Cut(id, "-"); ok {
			var a, b int
			if _, err := fmt.Sscanf(from+" "+to, "%d %d", &a, &b); err != nil {
				return nil, fmt.Errorf("line %d: bad range %q", line, id)
			}
			addSurface(form, misc)
			words[len(words)-1].tag = unsetTag
			rangeEnd = b

			continue
		}
		var n int
		if _, err := fmt.Sscanf(id, "%d", &n); err != nil {
			return nil, fmt.Errorf("line %d: bad id %q", line, id)
		}
		tag, ok := postag.ParseTag(upos)
		if !ok {
			return nil, fmt.Errorf("line %d: unknown UPOS %q", line, upos)
		}
		if n <= rangeEnd {
			if last := &words[len(words)-1]; last.tag == unsetTag {
				last.tag = tag
			}

			continue
		}
		addSurface(form, misc)
		words[len(words)-1].tag = tag
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	flush()
	return out, nil
}

// align tokenizes the sentence text with the extractor and tags each token
// with the first non-punctuation treebank word it overlaps, or PUNCT if it
// overlaps only punctuation.
func align(language string, tb treebankSentence, stats *alignStats) ([]postag.Sentence, error) {
	spans, err := extractor.Segment(tb.text, language)
	if err != nil {
		return nil, err
	}
	pieces := make([]int, len(tb.words))
	var out []postag.Sentence
	w := 0
	for _, sent := range spans {
		s := postag.Sentence{}
		for _, sp := range sent {
			for w < len(tb.words) && tb.words[w].end <= sp.Start {
				w++
			}
			tag, found, overlaps := postag.PUNCT, false, 0
			for k := w; k < len(tb.words) && tb.words[k].start < sp.End; k++ {
				overlaps++
				pieces[k]++
				if !found && tb.words[k].tag != postag.PUNCT {
					tag, found = tb.words[k].tag, true
				}
			}
			if overlaps == 0 {
				continue
			}
			if overlaps > 1 {
				stats.merged++
			}
			s.Words = append(s.Words, tb.text[sp.Start:sp.End])
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

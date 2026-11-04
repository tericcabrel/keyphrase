package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tericcabrel/keyphrase/internal/treebank"
	"github.com/tericcabrel/keyphrase/lang"
	"github.com/tericcabrel/keyphrase/postag"
)

// The treebanks postag/models/train.sh trains on, every split included.
var treebankDirs = map[string][]string{
	"de": {"UD_German-GSD"},
	"en": {"UD_English-EWT"},
	"es": {"UD_Spanish-GSD"},
	"fr": {"UD_French-GSD"},
	"it": {"UD_Italian-ParlaMint", "UD_Italian-PUD", "UD_Italian-TWITTIRO", "UD_Italian-MarkIT"},
	"pt": {"UD_Portuguese-GSD"},
}

const (
	minLexiconCount = 3
	// A form that is often two classes, such as "work" or "estudo", stays unclassified.
	dominantShare = 0.8
)

type tagCounts [postag.NumTags]int

// Tag counts per form, independent of the tagger: the tagger guesses in
// context, and this says what a word usually is. Forms are kept as written
// and normalized, because case matters in German.
type lexicon struct {
	exact, normalized map[string]*tagCounts
}

func newLexicon() lexicon {
	return lexicon{exact: map[string]*tagCounts{}, normalized: map[string]*tagCounts{}}
}

func (l lexicon) add(form string, tag postag.Tag) {
	countTag(l.exact, form, tag)
	countTag(l.normalized, lang.Normalize(form), tag)
}

func countTag(counts map[string]*tagCounts, key string, tag postag.Tag) {
	if counts[key] == nil {
		counts[key] = &tagCounts{}
	}
	counts[key][tag]++
}

func isFunctionTag(tag postag.Tag) bool {
	switch tag {
	case postag.ADP, postag.ADV, postag.CCONJ, postag.DET, postag.PART, postag.PRON, postag.SCONJ:
		return true
	default:
		return false
	}
}

func isAdjectiveTag(tag postag.Tag) bool { return tag == postag.ADJ }

func isNounTag(tag postag.Tag) bool { return tag == postag.NOUN || tag == postag.PROPN }

func (l lexicon) mostly(form string, class func(postag.Tag) bool) bool {
	if l.exact == nil {
		return false
	}
	total, inClass := classCount(l.exact[form], class)
	if total < minLexiconCount {
		total, inClass = classCount(l.normalized[lang.Normalize(form)], class)
	}
	return total >= minLexiconCount && float64(inClass) >= dominantShare*float64(total)
}

func classCount(counts *tagCounts, class func(postag.Tag) bool) (total, inClass int) {
	if counts == nil {
		return 0, 0
	}
	for tag, count := range counts {
		total += count
		if class(postag.Tag(tag)) {
			inClass += count
		}
	}
	return total, inClass
}

func loadLexicons(dir string) (map[string]lexicon, []string, error) {
	lexicons := map[string]lexicon{}
	var missing []string
	for _, code := range sortedKeys(treebankDirs) {
		var files []string
		for _, name := range treebankDirs[code] {
			matches, err := filepath.Glob(filepath.Join(dir, name, "*.conllu"))
			if err != nil {
				return nil, nil, err
			}
			files = append(files, matches...)
		}
		if len(files) == 0 {
			missing = append(missing, code)

			continue
		}
		words := newLexicon()
		for _, path := range files {
			if err := addTreebank(words, path); err != nil {
				return nil, nil, err
			}
		}
		lexicons[code] = words
	}
	return lexicons, missing, nil
}

func addTreebank(words lexicon, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	sentences, err := treebank.Read(file)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	for _, sentence := range sentences {
		for _, word := range sentence.Words {
			words.add(sentence.Text[word.Start:word.End], word.Tag)
		}
	}
	return nil
}

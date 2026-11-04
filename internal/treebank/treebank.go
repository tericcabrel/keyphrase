// Package treebank reads Universal Dependencies treebanks in CoNLL-U format.
// The tagger trainer learns from them, and the evaluation uses them as a
// word-class reference that does not depend on the tagger.
package treebank

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/tericcabrel/keyphrase/postag"
)

// Word is one treebank token as written in the text, a byte range of its
// sentence's Text. A multiword token such as Spanish "del" (de + el) is one
// surface word carrying the tag of its first part.
type Word struct {
	Start, End int
	Tag        postag.Tag
}

// Sentence is a treebank sentence's rebuilt text and its surface words.
type Sentence struct {
	Text  string
	Words []Word
}

// unsetTag marks a multiword token whose first part has not been read yet.
const unsetTag = postag.Tag(255)

// Read parses CoNLL-U sentences and rebuilds each sentence's text from its
// surface words and SpaceAfter=No marks.
func Read(r io.Reader) ([]Sentence, error) {
	var out []Sentence
	var text strings.Builder
	var words []Word
	rangeEnd := 0 // last word id covered by the current multiword token
	flush := func() {
		for i := range words {
			if words[i].Tag == unsetTag {
				words[i].Tag = postag.X
			}
		}
		if len(words) > 0 {
			out = append(out, Sentence{Text: text.String(), Words: words})
		}
		text.Reset()
		words = nil
		rangeEnd = 0
	}
	addSurface := func(form, misc string) {
		start := text.Len()
		text.WriteString(form)
		words = append(words, Word{Start: start, End: text.Len()})
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
			words[len(words)-1].Tag = unsetTag
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
			if last := &words[len(words)-1]; last.Tag == unsetTag {
				last.Tag = tag
			}

			continue
		}
		addSurface(form, misc)
		words[len(words)-1].Tag = tag
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	flush()
	return out, nil
}

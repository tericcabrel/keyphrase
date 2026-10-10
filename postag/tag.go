package postag

import "fmt"

// Tag is a Universal Dependencies part-of-speech label (UPOS). The numeric
// values are stored in model files, so the order must not change.
type Tag uint8

const (
	ADJ     Tag = iota // adjective
	ADP                // adposition: of, de, für
	ADV                // adverb
	AUX                // auxiliary (helper) verb: has, ha, wird
	CCONJ              // coordinating conjunction: and, et
	DET                // determiner: the, la
	INTJ               // interjection
	NOUN               // noun
	NUM                // numeral
	PART               // particle: not, 's
	PRON               // pronoun
	PROPN              // proper noun (name)
	PUNCT              // punctuation
	SCONJ              // subordinating conjunction: that, que
	SYM                // symbol
	VERB               // verb
	X                  // other
	NumTags = iota
)

var tagNames = [NumTags]string{
	"ADJ", "ADP", "ADV", "AUX", "CCONJ", "DET", "INTJ", "NOUN", "NUM",
	"PART", "PRON", "PROPN", "PUNCT", "SCONJ", "SYM", "VERB", "X",
}

func (t Tag) String() string {
	if int(t) < NumTags {
		return tagNames[t]
	}
	return fmt.Sprintf("Tag(%d)", uint8(t))
}

// IsVerb reports whether the tag marks a verb or an auxiliary verb.
func (t Tag) IsVerb() bool { return t == VERB || t == AUX }

// IsNominal reports whether the tag marks a word that can name a topic on its
// own: a noun, a proper noun, or a foreign or unclassified word.
func (t Tag) IsNominal() bool { return t == NOUN || t == PROPN || t == X }

// ParseTag converts a UPOS name such as "NOUN" to a Tag.
func ParseTag(name string) (Tag, bool) {
	for i, tagName := range tagNames {
		if tagName == name {
			return Tag(i), true
		}
	}
	return 0, false
}

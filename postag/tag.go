package postag

import "fmt"

// Tag numbers are stored in model files, so this order must not change.
type Tag uint8

const (
	ADJ Tag = iota
	ADP
	ADV
	AUX
	CCONJ
	DET
	INTJ
	NOUN
	NUM
	PART
	PRON
	PROPN
	PUNCT
	SCONJ
	SYM
	VERB
	X
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

func (t Tag) IsVerb() bool { return t == VERB || t == AUX }

func (t Tag) IsNominal() bool { return t == NOUN || t == PROPN || t == X }

func ParseTag(name string) (Tag, bool) {
	for i, tagName := range tagNames {
		if tagName == name {
			return Tag(i), true
		}
	}
	return 0, false
}

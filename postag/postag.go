// Package postag labels words with their part of speech (noun, verb,
// adjective, ...) so that the extractor can keep verbs out of keywords.
//
// Models are trained offline by cmd/train-pos on Universal
// Dependencies treebanks. Each language package embeds its own model.
package postag

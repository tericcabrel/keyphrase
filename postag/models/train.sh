#!/bin/sh
# Retrain every part-of-speech model. Run from the module root after
# postag/models/fetch.sh. Training is deterministic: the same treebanks give
# byte-identical model files.
set -eu

U=postag/models/ud
mkdir -p tmp
go build -o tmp/train-pos ./cmd/train-pos
train() {
	code=$1 train=$2 test=$3
	echo "== $code"
	./tmp/train-pos -lang "$code" -iters 12 \
		-train "$train" -test "$test" -out "lang/$code/model.bin.gz"
}

train en "$U/UD_English-EWT/en_ewt-ud-train.conllu" "$U/UD_English-EWT/en_ewt-ud-test.conllu"
train fr "$U/UD_French-GSD/fr_gsd-ud-train.conllu" "$U/UD_French-GSD/fr_gsd-ud-test.conllu"
train de "$U/UD_German-GSD/de_gsd-ud-train.conllu" "$U/UD_German-GSD/de_gsd-ud-test.conllu"
train es "$U/UD_Spanish-GSD/es_gsd-ud-train.conllu" "$U/UD_Spanish-GSD/es_gsd-ud-test.conllu"
# Italian: only openly licensed treebanks (the larger ISDT, VIT, ParTUT and
# PoSTWITA are non-commercial). PUD has no training split, so all of it is
# used for training; the test set is the other three test splits.
train it \
	"$U/UD_Italian-ParlaMint/it_parlamint-ud-train.conllu,$U/UD_Italian-PUD/it_pud-ud-test.conllu,$U/UD_Italian-TWITTIRO/it_twittiro-ud-train.conllu,$U/UD_Italian-TWITTIRO/it_twittiro-ud-dev.conllu,$U/UD_Italian-MarkIT/it_markit-ud-train.conllu,$U/UD_Italian-MarkIT/it_markit-ud-dev.conllu" \
	"$U/UD_Italian-ParlaMint/it_parlamint-ud-test.conllu,$U/UD_Italian-TWITTIRO/it_twittiro-ud-test.conllu,$U/UD_Italian-MarkIT/it_markit-ud-test.conllu"

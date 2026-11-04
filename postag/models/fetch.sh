#!/bin/sh
# Download the Universal Dependencies treebanks used to train the
# part-of-speech models, pinned to one UD release. Run from the module root.
# The downloads go to postag/models/ud/ and are not committed; only the
# trained lang/<code>/model.bin.gz files are.
set -eu

RELEASE=r2.18
DEST=postag/models/ud

TREEBANKS="
UD_English-EWT
UD_French-GSD
UD_German-GSD
UD_Spanish-GSD
UD_Portuguese-GSD
UD_Italian-ParlaMint
UD_Italian-PUD
UD_Italian-TWITTIRO
UD_Italian-MarkIT
"

mkdir -p "$DEST"
for tb in $TREEBANKS; do
	if [ -d "$DEST/$tb" ]; then
		echo "$tb: already present"
		continue
	fi
	echo "$tb: cloning $RELEASE"
	git -c advice.detachedHead=false clone --quiet --depth 1 --branch "$RELEASE" \
		"https://github.com/UniversalDependencies/$tb.git" "$DEST/$tb"
done

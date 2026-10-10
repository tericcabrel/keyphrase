#!/bin/sh
# Download the Leipzig Corpora Collection samples used to build the rarity
# tables, and check them against rarity/SHA256SUMS. Run from the module root.
# The downloads go to rarity/leipzig/ and are not committed; only the built
# lang/<code>/rarity.bin.gz tables are.
#
# Licence and citation: lang/THIRD_PARTY.md.
set -eu

BASE=https://downloads.wortschatz-leipzig.de/corpora
DEST=rarity/leipzig
SUMS=rarity/SHA256SUMS

mkdir -p "$DEST"
while read -r hash file; do
	case "$hash" in
	""|\#*) continue ;;
	esac
	name=${file%.tar.gz}
	archive="$DEST/$file"
	if [ ! -f "$archive" ]; then
		echo "$name: downloading"
		curl -fL --retry 3 --retry-delay 2 -o "$archive" "$BASE/$file"
	else
		echo "$name: already present"
	fi
done < "$SUMS"

if command -v sha256sum >/dev/null 2>&1; then
	(cd "$DEST" && sha256sum -c "../SHA256SUMS")
else
	(cd "$DEST" && shasum -a 256 -c "../SHA256SUMS")
fi

while read -r hash file; do
	case "$hash" in
	""|\#*) continue ;;
	esac
	name=${file%.tar.gz}
	sentences="$DEST/$name/$name-sentences.txt"
	if [ -f "$sentences" ]; then
		continue
	fi
	echo "$name: extracting sentences"
	tar -xf "$DEST/$name.tar.gz" -C "$DEST" "$name/$name-sentences.txt"
done < "$SUMS"

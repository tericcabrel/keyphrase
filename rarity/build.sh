#!/bin/sh
# Build lang/<code>/rarity.bin.gz from the sentence files fetch.sh extracted.
# Run from the module root, after rarity/fetch.sh.
set -eu

build() {
	lang=$1
	shift
	echo "building $lang"
	go run ./cmd/build-rarity -lang "$lang" -out "lang/$lang/rarity.bin.gz" "$@"
}

build en \
	rarity/leipzig/eng_news_2024_1M/eng_news_2024_1M-sentences.txt \
	rarity/leipzig/eng_wikipedia_2016_1M/eng_wikipedia_2016_1M-sentences.txt
build de \
	rarity/leipzig/deu_news_2024_1M/deu_news_2024_1M-sentences.txt \
	rarity/leipzig/deu_wikipedia_2021_1M/deu_wikipedia_2021_1M-sentences.txt
build fr \
	rarity/leipzig/fra_news_2023_1M/fra_news_2023_1M-sentences.txt \
	rarity/leipzig/fra_wikipedia_2021_1M/fra_wikipedia_2021_1M-sentences.txt
build es \
	rarity/leipzig/spa_news_2024_1M/spa_news_2024_1M-sentences.txt \
	rarity/leipzig/spa_wikipedia_2021_1M/spa_wikipedia_2021_1M-sentences.txt
build it \
	rarity/leipzig/ita_news_2023_1M/ita_news_2023_1M-sentences.txt \
	rarity/leipzig/ita_wikipedia_2021_1M/ita_wikipedia_2021_1M-sentences.txt

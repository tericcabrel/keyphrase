# Language data sources and licences

| file | source | licence |
|---|---|---|
| `en/snowball_en.txt` | Snowball English stop word list, [snowball-website `algorithms/english/stop.txt`](https://github.com/snowballstem/snowball-website/blob/master/algorithms/english/stop.txt) | BSD-3-Clause |
| `fr/snowball_fr.txt` | Snowball French stop word list, `algorithms/french/stop.txt` | BSD-3-Clause |
| `de/snowball_de.txt` | Snowball German stop word list, `algorithms/german/stop.txt` | BSD-3-Clause |
| `es/snowball_es.txt` | Snowball Spanish stop word list, `algorithms/spanish/stop.txt` | BSD-3-Clause |
| `it/snowball_it.txt` | Snowball Italian stop word list, `algorithms/italian/stop.txt` | BSD-3-Clause |
| `pt/snowball_pt.txt` | Snowball Portuguese stop word list, `algorithms/portuguese/stop.txt` | BSD-3-Clause |
| `<code>/extra_<code>.txt` | Original work for this project: reporting verbs, auxiliaries, dates, units, possessives, generic time nouns | MIT |
| connectors, elisions, clitics and abbreviations in `<code>/<code>.go` | Original work for this project | MIT |

## Part-of-speech models

The files `lang/<code>/model.bin.gz` are statistical models trained by
`cmd/train-pos` (see `postag/models/train.sh`) on the Universal
Dependencies treebanks below, all at release tag `r2.18` (downloaded by
`postag/models/fetch.sh`). The models contain feature weights and a list of
frequent words with their usual part of speech, not the treebank sentences.
Because they are derived from ShareAlike-licensed data, treat the model files
as covered by the same licences: redistribute them with attribution to the
treebanks and under CC BY-SA terms.

| model | treebank | licence |
|---|---|---|
| `en/model.bin.gz` | [UD_English-EWT](https://github.com/UniversalDependencies/UD_English-EWT) | CC BY-SA 4.0 |
| `fr/model.bin.gz` | [UD_French-GSD](https://github.com/UniversalDependencies/UD_French-GSD) | CC BY-SA 4.0 |
| `de/model.bin.gz` | [UD_German-GSD](https://github.com/UniversalDependencies/UD_German-GSD) | CC BY-SA 4.0 |
| `es/model.bin.gz` | [UD_Spanish-GSD](https://github.com/UniversalDependencies/UD_Spanish-GSD) | CC BY-SA 4.0 |
| `pt/model.bin.gz` | [UD_Portuguese-GSD](https://github.com/UniversalDependencies/UD_Portuguese-GSD) | CC BY-SA 4.0 |
| `it/model.bin.gz` | [UD_Italian-ParlaMint](https://github.com/UniversalDependencies/UD_Italian-ParlaMint) | CC BY-SA 4.0 |
| | [UD_Italian-PUD](https://github.com/UniversalDependencies/UD_Italian-PUD) | CC BY-SA 3.0 |
| | [UD_Italian-TWITTIRO](https://github.com/UniversalDependencies/UD_Italian-TWITTIRO) | CC BY-SA 4.0 |
| | [UD_Italian-MarkIT](https://github.com/UniversalDependencies/UD_Italian-MarkIT) | CC BY 4.0 |

The largest Italian treebanks were deliberately not used, because their
licences forbid commercial use: ISDT and VIT (CC BY-NC-SA 3.0), ParTUT,
PoSTWITA and KIParla (CC BY-NC-SA 4.0).

## Rarity tables

The files `lang/<code>/rarity.bin.gz` are inverse-sentence-frequency tables
built by `cmd/build-rarity` (see `rarity/build.sh`) from the Leipzig
Corpora Collection 1-million-sentence samples that `rarity/fetch.sh`
downloads. Each table stores normalized words and a one-byte score, not the
sentences. Because the tables are derived from the corpora, treat them as
covered by the same licence: redistribute them with attribution and under
CC BY.

The Leipzig terms of usage (<https://wortschatz-leipzig.de/en/usage>) license
the downloadable text corpora under CC BY. The Wortschatz applications and
other downloads are CC BY-NC and are not used here. Cite:

D. Goldhahn, T. Eckart & U. Quasthoff: Building Large Monolingual Dictionaries
at the Leipzig Corpora Collection: From 100 to 200 Languages. In: Proceedings
of the 8th International Conference on Language Resources and Evaluation
(LREC 2012).

| table | corpora |
|---|---|
| `en/rarity.bin.gz` | eng_news_2024_1M, eng_wikipedia_2016_1M |
| `de/rarity.bin.gz` | deu_news_2024_1M, deu_wikipedia_2021_1M |
| `fr/rarity.bin.gz` | fra_news_2023_1M, fra_wikipedia_2021_1M |
| `es/rarity.bin.gz` | spa_news_2024_1M, spa_wikipedia_2021_1M |
| `it/rarity.bin.gz` | ita_news_2023_1M, ita_wikipedia_2021_1M |
| `pt/rarity.bin.gz` | por_news_2024_1M, por_wikipedia_2021_1M |

English has no Wikipedia sample newer than 2016 at the 1-million-sentence
size. Counts are per sentence, because Leipzig publishes shuffled sentences
rather than whole documents. A word is counted once per sentence and kept
when it appears in at least 3 sentences.

## Conversion notes

The Snowball lists were converted mechanically: comments (after `|`) were
removed, one word per line, duplicates dropped. The words themselves were not
changed. The full Snowball licence text, which BSD-3-Clause requires to be
redistributed, is in [`data/SNOWBALL_COPYING`](data/SNOWBALL_COPYING).

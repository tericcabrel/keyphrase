# How extraction works

1. **Segmentation.** [uniseg](https://github.com/rivo/uniseg) (Unicode UAX #29) finds sentence and word boundaries. Every token keeps its original byte offsets.
   - **Abbreviation correction.** UAX #29 rule SB8 breaks sentences after `Dr.` or `z.` when a capital follows. A sentence that ends in a known pack abbreviation, a single letter (initials, `z. B.`) or a dotted form (`U.S.`) is merged with the next one. Only sentence indices change, never offsets.
2. **Token fixes.** These keep offsets exact:
   - word-hyphen-word joins into one token (`e-mail`, `Corona-Pandemie`);
   - dotted forms keep their final period (`U.S.`);
   - elided articles split off (`l'`, `d'`, `dell'`, with straight or curly apostrophes), so `l'économie` yields the phrase `économie`;
   - English `'s` splits off.

   German compounds stay single tokens.
3. **Part-of-speech tagging.** A small tagger per language (package `postag`) labels every token of each sentence as noun, verb, adjective, name, and so on, using the word, its last letters, its neighbours and the previous labels. Tokens labelled VERB or AUX (helper verbs such as "has", "ha") are treated like stopwords, so they never start, end or appear inside a phrase. A word keeps the verb label only if the tagger gives it in at least half of that word's occurrences in the document, which overrules occasional mistakes ("alta" in "acqua alta"). Adverbs ("probably", "instead", "intentionally") are treated the same way, but only confident adverb labels count (at least 15 points ahead of the runner-up tag), because the models also call some adjectives and nouns adverbs ("Offshore wind", Italian "acque"). A word that the tagger calls a noun or name in fewer than half of its occurrences is marked as needing a head (see Candidates). The tagger only adds these flags; offsets are untouched. See [Part-of-speech tagger](#part-of-speech-tagger).
4. **Candidates.**
   - Contiguous runs of at most 3 words within one sentence, counting connectors (`of`, `de`, `für`). Punctuation, line breaks, verbs and adverbs end a run.
   - A single word must be a name, or capitalized like a name ("Scottish"), or a common noun that is rare in general text (rarity at least 0.6, so "chloroplast" stays and "budget" does not). Adjectives, numerals and pronouns ("big", "sure", "worth", "nobody") only appear inside a longer phrase ("big data", "politique agricole commune").
   - Spelled-out numbers up to ninety and question words ("warum", "pourquoi", "cómo") are stopwords; like digits, numbers can't start a phrase. Hyphenated compounds are never treated as adverbs ("sistema multi-agente").
   - Pack connectors may appear inside a phrase (`of`, `für`, `della`, `d'`), but never at either end and never more than two in a row. Two connectors (`de la`, `of the`) do not fit, because the phrase would be at least four words.
   - Coordinating conjunctions are not connectors.
   - A phrase can't start with a stopword, a number, a one-letter word, or a lowercase two-letter word. A number may end a phrase (`Windows 11`).
   - Underscore identifiers (`SENTRY_PROJECT_DSN`) and source filenames (`index.ts`, `index.js`, `route.ts`) are not content, so they end a run. A `.js` name whose stem contains an uppercase letter stays content (`Node.js`).
   - Exception: a capitalized mid-sentence connector may open a name (`La Rochelle`).
   - Except in German, a span that starts or ends next to another capitalized content word is rejected as a truncated name.
5. **Aggregation.** Candidates are grouped by normalized form (lowercase, apostrophes folded). Normalization is used only for counting; returned text is always sliced from the input.
6. **Scoring.** The score is the equal-weight mean of six features in `[0, 1]`, defined in `extractor/rank.go`:
   - `frequency`: log(1+n) / log(1+max n);
   - `early`: 1 minus the first offset divided by the text length;
   - `spread`: log(1+sentences) / log(1+max sentences);
   - `capitalization`: the share of occurrences that look like a name, ignoring a lone sentence-initial capital (in German, lone words count only as acronyms);
   - `cohesion`: 0 for single words and one-off phrases, otherwise n / count of the phrase's most frequent content word;
   - `rarity`: how uncommon the phrase's content words are in a general corpus (package `rarity`). A single word uses its own score. A phrase uses the highest score among its content words, so one specific word carries a generic companion ("prodotto interno lordo"). A word missing from the table, including after a plural fold, scores 1. A language with no table leaves this feature out of the mean.
7. **Selection**, in rank order:
   - skip a phrase if a longer candidate with the same count contains it and ranks at least as high;
   - skip plural/variant near duplicates;
   - skip phrases contained in, or containing, an already selected phrase;
   - skip a single word when one repeated phrase covers at least 25% of its occurrences (`wind` versus `wind power`);
   - otherwise pick the phrase's earliest occurrence that doesn't overlap a selected span.

   Scores are divided by the best selected score and rounded to 4 decimals.

The process is deterministic: identical input gives identical output, and a test checks this.

## Part-of-speech tagger

The tagger is a greedy left-to-right averaged perceptron in pure Go (`postag/`), with no new dependencies. For each word it scores every label by adding up learned weights for about 20 clues: the lowercased word, its last 1-4 letters, its first letter, its shape (capitalized, all caps, digits), the two words on each side, and the two labels it has just given. The highest score wins. Frequent words that almost always have one label (for example "the", "de", "und") are looked up in a small dictionary instead.

The weights are learned offline from Universal Dependencies treebanks: sentences in which people have labelled every word. The training program rebuilds each treebank sentence's text and splits it with the extractor's own tokenizer (`extractor.Segment`). It then gives each token the label of the treebank word it covers, so the tagger learns on exactly the tokens it sees in production.

```sh
just pos-fetch   # download the pinned treebanks to postag/models/ud/ (not committed)
just pos-train   # retrain all six models; the output is byte-identical on rerun
```

`just pos-fetch` runs `postag/models/fetch.sh`. `just pos-train` runs `postag/models/train.sh`, which writes `lang/<code>/model.bin.gz`. The downloads are a local cache. The committed files are the model archives.

Accuracy on each treebank's held-out test sentences, as printed by the training program. "Verb precision" is the share of words labelled as verbs that really are verbs. "Verb recall" is the share of real verbs that get the verb label.

| language | training data | words labelled correctly | verb precision | verb recall | model file | memory once loaded |
|---|---|---|---|---|---|---|
| en | EWT, 209k tokens | 94.0% | 96.6% | 97.4% | 2.6 MB | 6.2 MiB |
| fr | GSD, 346k tokens | 96.8% | 96.7% | 98.1% | 3.0 MB | 6.7 MiB |
| de | GSD, 260k tokens | 93.9% | 96.7% | 97.4% | 3.7 MB | 7.2 MiB |
| es | GSD, 376k tokens | 95.6% | 97.9% | 96.9% | 3.8 MB | 7.2 MiB |
| pt | GSD, 238k tokens | 95.9% | 96.7% | 96.8% | 2.5 MB | 5.8 MiB |
| it | ParlaMint, PUD, TWITTIRO, MarkIT, 90k tokens | 94.4% | 95.5% | 96.4% | 1.6 MB | 3.3 MiB |

Italian uses only openly licensed treebanks. The larger standard Italian treebanks (ISDT, VIT, ParTUT, PoSTWITA) are licensed for non-commercial use only, so the Italian model is trained on about a quarter as much data as the others. It is the weakest of the six. A model is decoded the first time its language is used. Sources and licences are in [lang/THIRD_PARTY.md](../lang/THIRD_PARTY.md).

## Adding a language

A language is a package whose `init` calls `lang.Register`. `lang.New` builds the usual pack from:

- stopword lists, one word per line, with `#` comments;
- connectors, elision prefixes, clitic suffixes, and abbreviations;
- a `Variant` function, often `lang.TrimSuffixMin`;
- `NounCaps` when the language capitalizes ordinary nouns;
- optional model and rarity bytes, decoded with `lang.LazyTagger` and `lang.LazyTable`.

The built-in packages (`lang/en` and the others) each embed only that language's lists, `model.bin.gz`, and `rarity.bin.gz`. Importing one of them does not embed the others. `lang/all` imports all six.

Add examples under `examples/<code>/` and cases in `lang/pack_test.go`. Record any imported list in `lang/THIRD_PARTY.md`.

For verb removal, add a Universal Dependencies treebank for the language to `postag/models/fetch.sh` and a `train` line to `postag/models/train.sh` that writes `lang/<code>/model.bin.gz`, then run `just pos-fetch` and `just pos-train`. A language without a model file still works; it keeps verbs. The same pattern builds a rarity table: `just rarity-fetch` and `just rarity-build` write `lang/<code>/rarity.bin.gz`.

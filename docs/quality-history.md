# Quality history

This is a qualitative review of extractor outputs on the versioned examples.
It is not a measured precision score: one reviewer (the implementer) judged
the outputs, and the judgements have not been reviewed independently. The
labelled-set procedure and results are in [Quality and evaluation](quality.md).

## Method

`examples/<lang>/*.txt` holds two short news-style texts per language
(150 to 205 words). `examples/<lang>/wiki-*.txt` holds one Wikipedia article
per language, about 2,000 words (sources in
[`examples/SOURCES.md`](../examples/SOURCES.md)). `quality/final-examples.md` is
limit 8 on the short texts. `quality/final-wiki-articles.md` is limit 10 on
the Wikipedia articles. The tool posts each file to the HTTP handler and
fails if a returned span differs from `text[start_byte:end_byte]`.

Every intermediate output is kept in [`quality/`](../quality/). **A** means
awkward: a fragment, truncated, too generic, or grammatically odd. **I**
means irrelevant to the topic. The marked tables are in those two files.
Across the 80 example keywords, 18 are marked A and 2 are marked I. German
is the weakest language on long text, because capitalization cannot separate
names from ordinary nouns.

## Changes made in response to observed outputs

The original five feature weights are still equal (1 each). A sixth feature,
rarity, was added at the same weight; see the note after the table. The
other changes were to candidate filtering, feature normalization, or
selection. Each change was kept only if it improved outputs in several
languages without visible regressions elsewhere.

| # | observed defect (before) | change | after | snapshot |
|---|---|---|---|---|
| 0 | - | Equal-weight baseline as specified | Many one-off verb phrases: "frappe durement", "risen sharply", "far pagare", "aprobó el miércoles" | [iteration-0](../quality/iteration-0-equal-weights.md) |
| 1 | One-off bigrams scored cohesion = 1.0 because each of their words also appeared only once | Cohesion requires the phrase to occur at least twice | Verb-phrase noise largely gone in all five languages | [iteration-1](../quality/iteration-1.md) |
| 1 | Name pieces were chosen instead of full names: "Security Agency", "prodotto interno" plus "interno lordo", "Santa Cruz" | Subsumption: a sub-phrase that never occurs outside a longer candidate yields to it | "prodotto interno lordo", "Zentralverband des Deutschen Handwerks" | iteration-1 |
| 1 | Coordinations: "Mieter und Eigentümer", "Paris-Lyon et Paris-Marseille", "Unidos y China" | Coordinating conjunctions (and, und, et, y, e) removed from connectors | Coordinations no longer produced | iteration-1 |
| 1 | "Mio", "Thursday", "millions d'euros", "Prozent der Kosten" | Abbreviations become stopwords; weekdays, months and number/currency words added to every pack | Gone | iteration-1 |
| 2 | Subsumption let one-off junk absorb names ("Parlamento Europeo aprobó") | Subsumption only by repeated longer phrases | Fixed; see the next row for the regression it caused | [iteration-2](../quality/iteration-2.md) |
| 2 | Truncated one-off names ("Wind Farm", "Security Agency") | Reject spans that start or end next to another capitalized content word (not applied to German) | "UK Health Security Agency", "Dogger Bank Wind Farm", "Climate Change Committee" | iteration-2 |
| 2 | Top score was below 1 when the best-ranked candidate was skipped | Normalize scores to the best *selected* keyword | Top score is always 1 | iteration-2 |
| 3 | Regression from row 2: "Santa Cruz" + "Tenerife" and "Bundesministerium" + "Wirtschaft" split apart | Fully capitalized one-off names may also subsume | "Santa Cruz de Tenerife" restored | [iteration-3](../quality/iteration-3.md) |
| 4 | "Alstom à La Rochelle"; German generic nouns ("Trainer", "Kosten", "Titel") got name credit | French à/au/aux/en and Italian al/alla removed from connectors; in German a lone word gets capitalization credit only if it is an acronym | German institution names rise ("Kreditanstalt für Wiederaufbau") | [iteration-4](../quality/iteration-4.md) |
| 5 | Article-initial names cut to "Rochelle" | A capitalized mid-sentence connector may open a name ("La Rochelle", "Los Angeles", "Il Sole") | "La Rochelle" | [iteration-5](../quality/iteration-5.md) |
| 6 | Long articles: "Paris Agreement" absorbed by the lower-ranked "help meet the Paris agreement" | A subsumer must rank at least as high as the sub-phrase | "Paris Agreement" ranks first | final |
| 6 | Long articles: "Wind" (96 occurrences) chosen over "Wind power" (28) | The generic single-word rule became a usage share: skip a word if one repeated phrase covers at least 25% of its occurrences (was: phrase score at least 0.75 times the word's score) | "Wind power" selected; bare "wind" and "power" skipped | final |
| 6 | "GW", "MJ", "TWh" and French "est" (a verb) selected | Unit symbols added to every pack; "est" added to French | Gone | final |
| 6 | Long articles: linear frequency (count / max) squashed key terms, so position dominated ("Wärmepumpe" with 13 occurrences was missing) | Frequency and spread are now log-scaled against the document maximum | en adds "Wind power", "electricity", "capacity"; fr adds "TGV", "train"; it adds "acqua alta", "Veneto"; de adds "Wärmepumpe", "Kältemittel". Examples mostly reorder slightly | final |
| 7 | Verbs selected as keywords or inside them: "rivisto" in the README example, "intentan replicar" and "resolver problemas" in the es article, and in the top 30 of the examples "sicherte", "risen", "aprobó", "frappe", "accesso costa" | Part-of-speech tagger per language; tokens labelled VERB or AUX break phrases. A word keeps the verb label only if it gets it in at least half its occurrences in the document | In the top 30 of all 15 texts, 62 phrases dropped out, all verbs or verb phrases; "limit climate change" became "climate change". Top 8 of the examples unchanged; es article adds "Anthropic" and "Midjourney" | [iteration-7-pos](../quality/iteration-7-pos.md) |
| 7 | Before the document-level vote was added, the change above cut "acqua alta" out of the it article's top 10: the Italian model labels "alta" a verb in 2 of 7 occurrences | Added the document-level vote | "acqua alta" back; no other change | iteration-7-pos |
| 8 | Generic nouns rank with names because the score only sees the current document ("people", "trabajo", "città") | Rarity from Leipzig news and Wikipedia sentence counts, weight 1. A phrase takes the highest content-word score | Tuning P@50 30% → 34% and held-out 33% → 37%, every language up on both (run `20261001-222553`) | final |
| 9 | Lone common nouns still fill the top 50 ("budget", "trabajo", "Inhalte") | Drop a one-word candidate unless it is a name, capitalized like a name, or rarer than 0.6 | Tuning P@50 up about one point in every language; held-out 37% → 38%, no language down | final |

Rarity was accepted only after labelling. On the 54 tuning pages, weight 0
matched the baseline. The highest-word reduction at weight 1 scored tuning
P@50 34% (was 30%) and held-out P@50 37% (was 33%), with every language up.
Weight 0.5 scored tuning P@50 32%. Weight 2 was not taken: hundreds of its
keywords were still unjudged, and treating them as awkward put it below
weight 1. The mean reduction dropped more keywords already marked good.
Dropping every lone common noun cut tuning P@50 from 34% to 32%. The rule
that shipped keeps a lone common noun when its rarity is at least 0.6, so
"chloroplast" stays and "people" does not. After those new keywords were
labelled, tuning P@50 rose about one point in every language and held-out
P@50 rose from 37% to 38%. Counts are per Leipzig sentence, not per document.
A word seen fewer than 3 times scores as unknown (1). Weight 0 reproduces
the previous ranking exactly.

Two German capitalization rules were tried and reverted.

- **Adjacent capitals only.** Name credit only for directly adjacent
  capitalized words outside sentence-initial position. This lost connector
  names such as "Kreditanstalt für Wiederaufbau" and still rewarded one-off
  pairs like "Beispiel Adsorption".
- **Acronyms only.** This removed all name credit, lost "Bayer Leverkusen"
  and "Bundesministerium für Wirtschaft", and left the long-article noise in
  place.

## Baselines

This comparison was run once, in September 2026, with scripts that are not in
this repository. Neither YAKE 0.7.3 nor RAKE (the rake.go package, with the
same stopword lists) returns byte offsets. YAKE kept 79 of 80 phrases as substrings, but often left verbs and
function words at the edges. RAKE lowercases and often returns clause-length
phrases; 31 of 80 are not substrings once case is lost. Median YAKE time per
2,000-word article was 50 to 60 ms in-process in CPython. This extractor's
phrases were shorter and more name-like, and it is the only one of the three
that guarantees exact offsets. On these samples YAKE handled German better,
because it does not rely on capitalization the same way. The comparison was
a one-time look. `cmd/eval` is what tracks later changes.

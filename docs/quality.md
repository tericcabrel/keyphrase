# Quality and evaluation

Accuracy is not a validated figure. The labelled Ghost posts used for the numbers below are not in this repository, so those numbers cannot be reproduced from a clone. `data-text/` stays gitignored: the pages are other people's articles.

[Quality history](quality-history.md) is the changelog of ranking changes judged on the short examples and the Wikipedia texts. Marked outputs are in [`quality/`](../quality/). In a one-time comparison during development, the phrases were shorter and more name-like than YAKE and RAKE on those ten examples, and this extractor was the only one that guarantees exact offsets. YAKE handled German better on those samples, because it does not depend on capitalization to find names. The baseline scripts are not in this repository, and the comparison does not show whether a later change helped or hurt. `cmd/eval` is what tracks that, against the hand labels.

## Evaluation

`just eval` scores the extractor against hand-written labels. It reads `data-text/<lang>/<name>.txt` (not versioned; each file holds the text as a JSON string body, with `\n` and `\"` escapes) and the labels next to it in `<name>.labels.tsv`, one `<mark><TAB><anchor>` line per anchor. Each text is extracted by `POST /v1/text`'s code from a saved snapshot of the page, `<name>.html`, whose address is in `<name>.url`.

| mark | meaning |
|---|---|
| `+` | good anchor |
| `A` | awkward: a fragment, truncated, too generic, or grammatically odd |
| `I` | irrelevant to the page topic, including navigation and page chrome |
| `?` | not judged yet |

The question for each phrase is whether this exact phrase would make a good link anchor from this article to a page about it:

- `+`: a full name or a specific concept, product or place the article mentions, even in passing (Pinta, Global Workspace Theory). The publisher's own brand, products and people count, as anchors for links to the publisher's own pages.
- `A`: about the article but not usable alone: a single generic word or bare abbreviation (entreprise, recipe, IS), a lone surname or first name, a truncated name or title, or a fragment of a clause.
- `I`: not part of the article's subject: page chrome, the byline author, photo and licence credits, and off-topic mentions. A generic or broken phrase about the subject is `A`, not `I`.

Phrases match labels after lowercasing, apostrophe folding and whitespace collapsing.

```sh
just eval                                         # report, and save the run under data-text/.runs/
just eval-draft                                   # also append unlabelled phrases as "?"
just eval-diff data-text/.runs/<run>.json         # compare with a saved run
```

`just eval` runs `go run ./cmd/eval`. These commands read `data-text/`, which is not in the repository.

- The evaluation keeps every phrase the extractor returns, not only the labelled top, because a caller with a higher limit sees the rest. `-limit N` cuts the output to N instead.
- `P@10`, `P@50` and `P@100` are good phrases divided by judged phrases among the first 10, 50 and 100, in score order. `G@10`, `G@50` and `G@100` are the mean number of good phrases per page in the same ranks. A change that returns fewer phrases can raise P@k by leaving out awkward ones without adding a good one; G@k shows that, because it can't rise that way. `?` phrases are excluded and counted separately. `A`, `I` and `?` count the first 100. `missed` counts phrases labelled good that were not returned.
- The rank-band table gives, for ranks 1–10, 11–50, 51–100 and 101 onward, the precision among judged phrases and the share of the band that is judged. A band that is mostly unjudged has an unreliable precision: the labels there are often left over from earlier runs, when those phrases ranked higher.
- Past rank 100, only a fixed sample of about one phrase in five is labelled and scored. A phrase is in the sample when a hash of its page and normalized text says so, not its rank, so a ranking change does not reshuffle the sample, and only phrases that newly appear need labels. Labels on phrases outside the sample are ignored there, so the 101+ precision is an unbiased estimate of the whole tail.
- The defect checks flag phrases at every rank without needing labels. Each one is a heuristic, so the report also shows how many of a check's labelled flags are `A` or `I`:
  - `function-word-edge`: the first or last word is mostly a determiner, pronoun, preposition, conjunction, particle or adverb ("desse artigo", "internacionais geralmente");
  - `verb`: a word is mostly a verb ("torna ativos valiosos");
  - `truncated`: the phrase ends on a word that is mostly an adjective, and the next word on the same line is mostly a noun ("talentos de alta" before "qualidade");
  - `fragment`: a higher-ranked phrase overlaps the phrase or touches it in the text ("remotas internacionais" after "vagas"). Two neighbouring capitalized words, as in a list of names, don't count as touching;
  - `heading-word`: a single common noun or adjective whose every occurrence is on a heading line ("Criatividade").
- "Mostly" means at least 80% of at least 3 occurrences in the language's Universal Dependencies treebanks. The treebanks are read from `postag/models/ud/` (`just pos-fetch`; another directory with `-treebanks`). The form as written is used when the treebank has it often enough, so German "Lernen" is a noun and "lernen" a verb. Without the treebanks, only the `fragment` check runs.
- Each page is also extracted from its first lines alone, up to the line that reaches 150 words: the size of a passage pasted into the Try page. That short text has its own table, with phrases returned per text, `P@10`, the precision over every returned phrase (`P@all`) and the share flagged by a defect check. A long page repeats its key terms, and the ranking counts repetition, so the full-page numbers can overstate what a pasted paragraph gets. The short text shares the page's labels file; the held-out pages' short texts are held out too.
- `-draft` only appends; existing lines are never rewritten. It drafts the unlabelled phrases of the tuning pages in the first 100, in the tail sample and in the short text. `-draft-holdout` does the same for the held-out pages and prints only counts, so that someone who doesn't tune can label them. To add a text, drop it into `data-text/<lang>/` and run `just eval-draft`; to relabel, edit the marks or delete the labels file and draft again.
- `just eval-diff` compares only the documents present in both runs and scores both with the current labels, so a corrected label or an added text can't move the deltas. When the runs used different limits, both are cut to the smaller one. Documents present in only one run are listed separately. The short texts get their own delta tables. For each shared document it lists the phrases added and dropped with their labels, and for each check both runs ran, the share of returned phrases it flags.
- Held-out pages, listed in `data-text/holdout.txt`, check whether a change generalises to pages the rules were not tuned on. They are labelled like the others, but the report and the diff show them only as per-language totals in their own table, and the verbose listing and the phrase diffs skip them. Don't read their phrases while tuning: if the tuning pages improve and the held-out pages don't, the change fits the tuning pages rather than the language.

`go run ./cmd/eval -v -save=false` lists unjudged and missed phrases per document.

## Clean check

`just cleancheck` checks that the extracted texts hold only the title and the article. For each snapshot it compares the new text with the texts and labels archived in `data-text/.archive/2026-09-29/`, which were extracted before main-content extraction existed. It reads `data-text/`, which is not in the repository.

- leftover chrome: a phrase labelled `I` in the archive that is still on a line that isn't a sentence, or a line of 1 to 3 words that looks like a menu entry and isn't a heading, list item, table cell or bold run;
- lost body: an archived sentence of 15 or more words, without an `I` phrase, that was inside the chosen container but is missing. Missing sentences that were pruned as boilerplate, were outside the container, or are no longer on the page are listed with the element that removed them, but don't fail the page;
- the text must start with the page's `h1` or `og:title`.

Lines listed in `<name>.accept` were read and judged to be article text. They are reported as accepted instead of failing.

```sh
just cleancheck                                          # report: rule, container, size, first and last lines, findings
go run ./cmd/cleancheck -text -page green-got     # print one page's full text
go run ./cmd/cleancheck -where "phrase"           # print the elements holding a phrase
go run ./cmd/cleancheck -write                    # write <name>.txt for every passing page
```

## Adding pages

List the pages in a file, one `<lang> <url>` line each, with `holdout` as a third word for a held-out page (`#` starts a comment):

```text
fr https://example-ghost-site.fr/some-post/
fr https://another-ghost-site.fr/other-post/ holdout
```

then run:

```sh
just pages urls.txt
```

`just pages` runs `scripts/add-pages.sh`. It reads and writes `data-text/`, which is not in the repository. Pass `-any` to also accept pages that are not Ghost posts: `just pages urls.txt -any`.

For each URL not saved yet, it fetches the page with `/v1/text`'s client and limits, and keeps it only if it is a Ghost post (`-any` lifts that). It saves the page as `data-text/<lang>/<site>.html` with its address in `<site>.url` (`<site>-2` and so on when the name is taken). It then runs the clean check on the new pages, writes the `.txt` of those that pass, and drafts their labels with `eval -draft`. A page also fails when its text has fewer than 300 words or is over 64 KiB. Re-running the script with the same file skips the pages already saved.

Pages marked `holdout` are added to `data-text/holdout.txt`, one `<lang>/<name>` per line; marking a page that is already saved works too, and the list can also be edited by hand.

What remains is by hand: read the start and end of each new text, since new pages have no archived labels to check chrome against; label the `?` lines; then run `just eval` to save a run. A page that fails keeps its `.html` and gets no `.txt` or labels, so it isn't evaluated.

All 69 pages pass, 11 of them with accepted lines. For the first 20 pages, which have archived texts, the new texts are 49% to 92% of the old size; the difference is menus, headers, bylines, footers, related posts, cookie banners and author boxes.

## Results

The evaluation was restarted from scratch on the cleanly extracted texts. The earlier run labelled texts full of page chrome, so its numbers are not comparable and it is kept only in the archive.

The set has 84 Ghost posts: 66 tuning pages and 18 held-out pages, 3 per language. Each page has its first 100 phrases labelled, the 1-in-5 sample past rank 100, and every phrase of its short text. The project owner labelled the English and French held-out pages of the first top-50 round and reviewed the implementer's labels on the first 8 English and French tuning pages. The implementer labelled the other tuning pages, and separate agents, which report only counts, labelled the held-out phrases added since. Run `20261030-003417`. G@k is the mean number of good phrases per page in the first k:

| split | pages | P@10 | P@50 | P@100 | G@10 | G@50 | G@100 | 101+ sample | short P@10 | short, all phrases |
|---|---|---|---|---|---|---|---|---|---|---|
| tuning | 66 | 64% | 38% | 28% | 6.4 | 18.4 | 23.7 | 14% | 43% | 33% |
| held-out | 18 | 67% | 42% | 32% | 6.7 | 20.4 | 27.6 | 13% | 47% | 33% |

| language | P@10 | P@100 | short P@10 | held-out P@10 | held-out P@100 | held-out short P@10 |
|---|---|---|---|---|---|---|
| de | 54% | 26% | 39% | 53% | 22% | 40% |
| en | 66% | 33% | 44% | 70% | 39% | 37% |
| es | 56% | 20% | 37% | 80% | 31% | 60% |
| fr | 75% | 27% | 43% | 60% | 32% | 57% |
| it | 60% | 24% | 35% | 63% | 22% | 41% |
| pt | 72% | 38% | 58% | 73% | 43% | 44% |

Until run `20261030-001512`, `cmd/eval` turned off the lone-noun rule unless a flag turned it back on, so earlier runs scored a configuration `Extract` does not use, and their numbers are lower for that reason. A run now scores `DefaultExtractOptions` unless a flag overrides it.

A page returns 90 to 250 phrases on average, depending on the language, and precision falls with rank: 64% in the first 10, 31% at ranks 11–50, 15% at 51–100 and about 14% after. A caller who raises the limit past about 50 gets mostly awkward phrases. A short text of about 150 words returns about 17 phrases, and its first 10 are about 20 points worse than a full page's. Almost none are irrelevant; most are awkward: bare nouns that pass the rarity threshold but are cut from a longer phrase ("machinery", "subordination"), verbs the tagger misses ("Mantén", "Posso"), and pieces of longer names ("sapiens" from "Homo sapiens"). The defect checks flag 3% of the first 10 and 7% of the tail; 93–100% of their labelled flags are not good.

Rarity (see [Quality history](quality-history.md)) moved tuning P@50 from 30% to 34% and held-out P@50 from 33% to 37%, with every language up on both splits.

All labels follow the rules above. They were settled after the project owner relabelled 4 of the implementer's English and French tuning pages blind (200 phrases): the two agreed on good versus not good for 81% of phrases (Cohen's kappa 0.58), and most disagreements were about passing mentions of specific names, on-topic single common words, and the publisher's own products. The implementer's labels were then updated, and a separate agent reviewed the owner's held-out labels against the same rules. The English and French held-out pages still score above their tuning pages; with 3 pages each, that is within what the choice of pages can explain.

These percentages cannot be reproduced from a clone. The labelled pages are not published.

## What works

- The HTTP contract, validation and size limits in [development](development.md).
- The exact-span invariant, including accents, straight and curly apostrophes, elision, emoji, invalid UTF-8 input and input at the HTTP server's 64 KiB limit. The library itself has no size limit. The invariant is enforced by tests, fuzzing and the samples tool.
- Six language packs behind one interface; the core is language-agnostic.
- A rarity table per language, so words that are common in general text rank below words that are specific to the page.
- Verbs are kept out of phrases in all six languages by an embedded, reproducibly trained part-of-speech tagger.
- Deterministic ranking and stable tie-breaks.
- The development server ran within an enforced 1 CPU / 512 MiB cgroup during development. `cmd/bench -cgroup` repeats that measurement on Linux; it is not run in CI.

## Limitations

- **Quality is barely measured.** The only precision numbers (see [Results](#results)) come from 84 Ghost posts, 14 per language (13 in English, 15 in Portuguese), with the first 100 phrases of each labelled and a sample after. Most labels are the implementer's; the project owner labelled the English and French held-out pages, with no measure of agreement between the two. Each language has only 3 held-out pages, so one unusual page moves its held-out numbers by several points. The adverb, lone-adjective, number-word and question-word rules were tuned on the first 20 pages. There are no recall numbers beyond the `missed` count.
- **Verb removal depends on the tagger, which is wrong on about 2-4% of verbs.** It sometimes misses a verb, for example "grew" in "The use of energy grew" is labelled a noun, so the verb can still appear. It sometimes labels a non-verb as a verb, which then can't be part of a phrase; the document-level vote catches this only for words that occur several times. Italian, trained on the least data, is the least accurate.
- **Lone adjectives still get through when the tagger calls them nouns.** The English model often labels an adjective before a noun as a noun ("complex software", "wider reach", "faster"), and its confidence on those labels overlaps with real nouns, so no threshold separates them. Nouns the tagger mostly calls verbs ("work") are no longer returned alone.
- **No named-entity information.** A lone common noun is kept only when it is rare in the Leipzig table. A generic noun inside a longer phrase, or a rare one, can still rank high.
- **Short texts and high limits are weak.** On a pasted paragraph of about 150 words, 43% of the first 10 phrases are good; past rank 50 of a full page, about 1 in 7 is.
- **German long text is weak**, because capitalization doesn't mark names.
- **Names get truncated or split** when they contain lowercase words or more than three words.
- **Plural folding is heuristic**, and there is no cross-language deduplication ("IA" versus "AI").
- **Language support.** No automatic language detection, and no support for scripts without spaces (for example Chinese, Japanese, Thai).
- **Plain text only, and clean text is the caller's job.** Menus, footers or other page chrome left in the input can surface as phrases, and so can headings and list items. The evaluation texts keep headings and list items as their own lines; if production callers send only paragraphs, measured precision may not match production.
- **`/v1/text` is a development helper tuned on 69 Ghost posts.** Other CMSs are handled by the `article`, `main` and body fallbacks, which keep whatever chrome the pruning rules don't recognise; only the test fixtures cover them. Featured-image captions are dropped along with the header that holds them.

## Next steps toward human-reviewed precision testing

Build a licensed, multi-domain corpus per language; write annotation guidelines; double-annotate and measure agreement; report precision@k per language with confidence intervals against YAKE and RAKE on a held-out split; tune weights only after that; then evaluate further uses of the part-of-speech labels or a named-entity step, starting with German.

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

- `P@10` and `P@50` are good phrases divided by judged phrases among the first 10 and 50, in score order. `?` phrases are excluded and counted separately. `missed` counts phrases labelled good that were not returned.
- `-draft` only appends; existing lines are never rewritten. To add a text, drop it into `data-text/<lang>/` and run `just eval-draft`; to relabel, edit the marks or delete the labels file and draft again.
- `just eval-diff` compares only the documents present in both runs and scores both with the current labels, so a corrected label or an added text can't move the deltas. Documents present in only one run are listed separately. For each shared document it lists the phrases added and dropped with their labels.
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

The set has 69 Ghost posts with the top 50 phrases of each labelled: 54 tuning pages and 15 held-out pages, 3 per language. The project owner labelled the English and French held-out pages, and reviewed the implementer's labels on the first 8 English and French tuning pages. The implementer labelled the rest. Run `20261001-222553`, after the rarity feature (see [Quality history](quality-history.md)):

| language | tuning pages | P@10 | P@50 | held-out pages | P@10 | P@50 |
|---|---|---|---|---|---|---|
| de | 11 | 54% | 32% | 3 | 50% | 22% |
| en | 10 | 65% | 45% | 3 | 67% | 51% |
| es | 11 | 51% | 25% | 3 | 77% | 31% |
| fr | 11 | 67% | 36% | 3 | 60% | 49% |
| it | 11 | 56% | 31% | 3 | 53% | 33% |
| all | 54 | 59% | 34% | 15 | 61% | 37% |

Against the previous run (`20261001-213729`), tuning P@50 moved from 30% to 34% and held-out P@50 from 33% to 37%. Every language rose on both splits. The new phrases that rarity brought into the top 50 were labelled before those numbers were kept: the implementer labelled the tuning pages, and a separate agent labelled the held-out pages.

The first 20 pages scored 35% P@50 on their own before rarity; the 34 pages added later are harder. Of the 2,700 tuning phrases in this run, 1,724 are awkward and 71 irrelevant, and 30 good phrases from earlier labels are no longer returned. Most awkward ones are single generic nouns ("trabajo", "Inhalte", "season"), fragments of longer names or titles ("Autodromo Enzo", "Normandie Rouen"), and, in German, noun phrases cut around a genitive or preposition ("Beitrag zum Architekt", "Arbeitstag pro"). Personal essays and stories with few names score lowest.

All labels follow the rules above. They were settled after the project owner relabelled 4 of the implementer's English and French tuning pages blind (200 phrases): the two agreed on good versus not good for 81% of phrases (Cohen's kappa 0.58), and most disagreements were about passing mentions of specific names, on-topic single common words, and the publisher's own products. The implementer's labels were then updated, and a separate agent reviewed the owner's held-out labels against the same rules. The English and French held-out pages still score above their tuning pages; with 3 pages each, that is within what the choice of pages can explain.

These percentages cannot be reproduced from a clone. The labelled pages are not published.

## What works

- The HTTP contract, validation and size limits in [development](development.md).
- The exact-span invariant, including accents, straight and curly apostrophes, elision, emoji, invalid UTF-8 input and input at the HTTP server's 64 KiB limit. The library itself has no size limit. The invariant is enforced by tests, fuzzing and the samples tool.
- Five language packs behind one interface; the core is language-agnostic.
- A rarity table per language, so words that are common in general text rank below words that are specific to the page.
- Verbs are kept out of phrases in all five languages by an embedded, reproducibly trained part-of-speech tagger.
- Deterministic ranking and stable tie-breaks.
- The development server ran within an enforced 1 CPU / 512 MiB cgroup during development. `cmd/bench -cgroup` repeats that measurement on Linux; it is not run in CI.

## Limitations

- **Quality is barely measured.** The only precision numbers (see [Results](#results)) come from 69 Ghost posts, 14 per language (13 in English), with the top 50 phrases of each labelled. Most labels are the implementer's; the project owner labelled the English and French held-out pages, with no measure of agreement between the two. Each language has only 3 held-out pages, so one unusual page moves its held-out numbers by several points. The adverb, lone-adjective, number-word and question-word rules were tuned on the first 20 pages. There are no recall numbers beyond the `missed` count.
- **Verb removal depends on the tagger, which is wrong on about 2-4% of verbs.** It sometimes misses a verb, for example "grew" in "The use of energy grew" is labelled a noun, so the verb can still appear. It sometimes labels a non-verb as a verb, which then can't be part of a phrase; the document-level vote catches this only for words that occur several times. Italian, trained on the least data, is the least accurate.
- **Lone adjectives still get through when the tagger calls them nouns.** The English model often labels an adjective before a noun as a noun ("complex software", "wider reach", "faster"), and its confidence on those labels overlaps with real nouns, so no threshold separates them. Nouns the tagger mostly calls verbs ("work") are no longer returned alone.
- **No named-entity information.** A lone common noun is kept only when it is rare in the Leipzig table. A generic noun inside a longer phrase, or a rare one, can still rank high.
- **German long text is weak**, because capitalization doesn't mark names.
- **Names get truncated or split** when they contain lowercase words or more than three words.
- **Plural folding is heuristic**, and there is no cross-language deduplication ("IA" versus "AI").
- **Language support.** No automatic language detection, and no support for scripts without spaces (for example Chinese, Japanese, Thai).
- **Plain text only, and clean text is the caller's job.** Menus, footers or other page chrome left in the input can surface as phrases, and so can headings and list items. The evaluation texts keep headings and list items as their own lines; if production callers send only paragraphs, measured precision may not match production.
- **`/v1/text` is a development helper tuned on 69 Ghost posts.** Other CMSs are handled by the `article`, `main` and body fallbacks, which keep whatever chrome the pruning rules don't recognise; only the test fixtures cover them. Featured-image captions are dropped along with the header that holds them.

## Next steps toward human-reviewed precision testing

Build a licensed, multi-domain corpus per language; write annotation guidelines; double-annotate and measure agreement; report precision@k per language with confidence intervals against YAKE and RAKE on a held-out split; tune weights only after that; then evaluate further uses of the part-of-speech labels or a named-entity step, starting with German.

# Development

`server` is a development tool used to build the evaluation set. It is not part of the published package, not a released binary, and not a supported API. Library users call `extractor.Extract`.

`POST /v1/text` fetches the URL it is given, including private addresses. The server allows only `http` and `https`, waits at most 10 seconds, follows at most 5 redirects, and reads at most 2 MiB of the response.

## Requirements and versions

| component | version | licence |
|---|---|---|
| Go minimum | `go 1.26.0` (`go` line in `go.mod`). `errors.AsType` and `golang.org/x/net` v0.59.0 both require 1.26.0 | BSD-3-Clause |
| Go toolchain | go1.27.3 (`toolchain` line in `go.mod`). Used when working in this module | BSD-3-Clause |
| [github.com/rivo/uniseg](https://github.com/rivo/uniseg) | v0.4.7 (pinned in `go.mod`/`go.sum`) | MIT |
| [golang.org/x/net](https://pkg.go.dev/golang.org/x/net) | v0.59.0 (HTML parse and charset; pulls in `golang.org/x/text` v0.42.0). Used by the development server, not by `extractor` | BSD-3-Clause |
| Snowball stop word lists (embedded data) | snowball-website master, fetched 2026-09-28 | BSD-3-Clause, see [lang/THIRD_PARTY.md](../lang/THIRD_PARTY.md) |
| Part-of-speech models (embedded data), trained on [Universal Dependencies](https://universaldependencies.org/) treebanks | UD release r2.18: EWT (en), GSD (fr, de, es, pt), ParlaMint + PUD + TWITTIRO + MarkIT (it) | CC BY-SA 4.0 (it: also CC BY-SA 3.0 and CC BY 4.0), see [lang/THIRD_PARTY.md](../lang/THIRD_PARTY.md) |
| Rarity tables (embedded data), counted from [Leipzig Corpora Collection](https://wortschatz-leipzig.de/en/download/) 1M-sentence news and Wikipedia samples | eng news 2024 + Wikipedia 2016; deu news 2024 + Wikipedia 2021; fra news 2023 + Wikipedia 2021; spa news 2024 + Wikipedia 2021; ita news 2023 + Wikipedia 2021; por news 2024 + Wikipedia 2021 | CC BY, see [lang/THIRD_PARTY.md](../lang/THIRD_PARTY.md) |
| Wikipedia texts (test data only) | revisions in [examples/SOURCES.md](../examples/SOURCES.md) | CC BY-SA 4.0 |
| This project's code | | MIT |

The code uses recent Go features: range-over-func iterators (`strings.Lines`, `strings.FieldsSeq`, `maps.Keys`, `slices.Sorted`), `errors.AsType`, `sync.WaitGroup.Go`, `testing.B.Loop`, range-over-int, and method-pattern `http.ServeMux` routes. The newest of those is `errors.AsType` (Go 1.26). Together with the `golang.org/x/net` requirement, that sets the minimum at 1.26.0.

Dependabot opens a weekly pull request for Go modules, GitHub Actions refs, and the site's Yarn packages. Patch and minor updates share one pull request per ecosystem. A major update is its own pull request. Those pulls do not change `CHANGELOG.md`, so merging one does not publish a module version. The toolchain is go1.27.3, lint runs on the 1.27 line with golangci-lint v2.15.0, GoReleaser is v2.19.0, just is 1.59.0, Yarn is 4.18.1, Wrangler is 4.153.0, Node is 24, and gotestsum is v1.13.0. Later bumps of the toolchain, golangci-lint, GoReleaser, gotestsum, just, Yarn, Wrangler's workflow pin, and Node stay manual.

## Commands

`just test` and `just lint` run the same commands as CI.

| recipe | what it does |
|---|---|
| `just test` | `gotestsum -- ./...` |
| `just lint` | `golangci-lint run ./...` |
| `just lint-fix` | `golangci-lint run ./... --fix` |
| `just fuzz` | fuzz `FuzzExtract` for 60 seconds |
| `just dev` | rebuild the local development server with `air`. Not part of the published package |
| `just eval` | score the labelled pages and save a run. Reads `data-text/`, which is not in the repository |
| `just eval-draft` | append unlabelled phrases in `data-text/` as `?` |
| `just eval-diff <run.json>` | compare the current run with a saved JSON file |
| `just cleancheck` | report chrome left in the extracted pages |
| `just pages <urls.txt>` | fetch, check, and draft labels for a URL list |
| `just pos-fetch` | download the treebanks into `postag/models/ud/`. The download is a local cache |
| `just pos-train` | retrain the models. The committed files are `lang/<code>/model.bin.gz` |
| `just rarity-fetch` | download the Leipzig sentences into `rarity/leipzig/`. The download is a local cache |
| `just rarity-build` | rebuild the rarity tables. The committed files are `lang/<code>/rarity.bin.gz` |
| `just site-build` | build the browser extractor into `site/public`. The site workflow runs this before it publishes |

A user-facing change is a section at the top of `CHANGELOG.md`. The heading is `## v0.1.1` or `## v0.1.1 - YYYY-MM-DD`, and the text says what a caller of the library can do now. Commit that and push `main`.

A pull request into `main` runs test and lint. Every push to `main` runs them again. The tag and the GitHub release happen only after both succeed and that push changed `CHANGELOG.md`. The release notes are the commits since the previous tag, newest first, and each line names its author. The changelog is the product summary. A push to another branch does not publish.

A change that would alter the published site also deploys it to the Cloudflare Pages project `keyphrase`. That is a file under `site/` that ships, or Go that `just site-build` compiles into the Try page. A pull request deploys a preview on the pull request branch. The merge to `main` deploys production. A docs-only change, or a test-only change, leaves the site unpublished.

The tests cover:

- the exact-slice, non-overlap, ordering and score-range invariants on every example file, on 300 random multilingual strings, and under fuzzing;
- candidate boundaries in all six languages: connectors, conjunctions, punctuation, line breaks, numbers, acronyms, truncated names and article-initial names;
- straight and curly apostrophes, elision offsets, German compounds and hyphen compounds;
- sentence-abbreviation handling in all six languages;
- repeated-phrase aggregation with earliest-occurrence choice, containment and overlap removal, plural near duplicates, and generic-word suppression;
- determinism and the tie-break order;
- empty, whitespace-only and stopword-only input;
- text exactly at 64 KiB;
- language and limit errors;
- pack data, and per-language spot checks of expected phrases;
- verbs: phrases break at verbs in all six languages, nouns that can also be verbs are kept ("rise of energy prices", "toma de decisiones"), no phrase of any example contains a verb-labelled token, and the document-level vote keeps "acqua alta";
- modifiers: in all six languages, adjectives, numerals and adverbs are not returned on their own but stay inside phrases ("big effort", "inteligencia artificial"), and "Offshore wind" survives the tagger calling "Offshore" an adverb;
- the tagger: per-language verb and noun spot checks, learning from context on a toy corpus, reproducible training, model save and reload, and rejection of corrupt model files;
- HTTP: normal responses, field names, default and null limit, limit bounds, empty text for every language, 20 bad-request cases, 413 for text and body, the worst-case escaped body, 405/404, `/healthz`, exact spans after a JSON round trip, and `POST /v1/text` (visible text, plain text, truncation, 400, 413, 502, and 405);
- main-content extraction: fixture pages for Ghost's Source and Casper themes, a non-Ghost news page and a div-only page, the more-than-half rule, exact content classes, whole-part class matching and date lines. The clean-check tool's own checks have unit tests too.

Fuzzing found no failures during development: a 60 s run (about 700,000 inputs), a 30 s run after the final ranking changes, and a 60 s run (about 530,000 inputs) after adding part-of-speech tagging.

## Layout

```
extractor/          core: tokenize.go, segment.go, candidates.go, rank.go, select.go, extractor.go
lang/               registry, Pack, Normalize; THIRD_PARTY.md; data/SNOWBALL_COPYING
lang/en fr de es it pt  one package per language: word lists, model.bin.gz, rarity.bin.gz
lang/all            blank-imports the six languages, for tests and local commands
postag/             part-of-speech tagger: features, model format, training, evaluation
postag/models/      fetch.sh (treebank download) and train.sh
rarity/             corpus rarity tables, fetch.sh, build.sh
cmd/train-pos/      trains a model from CoNLL-U with the extractor's tokenizer
cmd/build-rarity/   counts Leipzig sentences into a rarity table
internal/webtext/   fetch a URL and return its title and main content
internal/httpapi/   HTTP handler and tests
cmd/server/         local development server
cmd/samples/        runs example files through the HTTP handler, prints Markdown
cmd/eval/           scores output against labels in data-text/
cmd/cleancheck/     checks extracted texts and adds pages from URL lists
scripts/add-pages.sh  fetch, check, write, draft labels, hold out
internal/evalset/   the held-out page list shared by eval and cleancheck
cmd/bench/          local load generator for the development server
examples/           short texts and Wikipedia articles (wiki-*.txt)
quality/            output snapshots per review iteration
docs/               pipeline, quality, and these development notes
```

## Build, run, request

`just dev` rebuilds the server when files change. To run it once:

```sh
go build -o server ./cmd/server
./server -addr :8080
```

```sh
curl -s localhost:8080/healthz
# {"status":"ok"}

curl -s localhost:8080/v1/keywords \
  -d '{"text":"La Banca d'"'"'Italia ha rivisto le stime. Il governatore della Banca d'"'"'Italia parla dei tassi di interesse e dei tassi di interesse reali.","language":"it","limit":3}'
# {"language":"it","keywords":[{"text":"Banca d'Italia","start_byte":3,"end_byte":17,"score":1},{"text":"tassi di interesse","start_byte":85,"end_byte":103,"score":0.6029},{"text":"stime","start_byte":32,"end_byte":37,"score":0.4068}]}

curl -s localhost:8080/v1/text \
  -d '{"url":"https://example.com"}'
# {"text":"...","truncated":false}
```

"ha rivisto" ("has revised") is not returned: verbs are never part of a phrase (see [How extraction works](algorithm.md)).

## HTTP contract

`POST /v1/keywords` with body `{"text": string, "language": "en|fr|de|es|it|pt", "limit": integer, "counts": bool}`.

- `limit` is optional and defaults to 5; `null` is treated as omitted. It must be at least 1. At most `limit` phrases are returned, so a limit greater than the number of phrases returns all of them.
- `counts` is optional. `null` and `false` are treated as omitted. When `true`, the response adds `counts: {"words", "extracted", "ranked", "extracted_more_than_one_word", "ranked_more_than_one_word", "more_than_one_word"}`. `words` is the number of words in the submitted text. Punctuation is not counted, and a hyphenated compound such as "e-mail" is one word. `extracted` is every candidate phrase. `ranked` is how many of those were returned. The other two counts are phrases of two or more words, including connectors. `more_than_one_word` lists every extracted phrase of two or more words, best-ranked first. Its length equals `extracted_more_than_one_word`.
- The response is `{"language": ..., "keywords": [{"text", "start_byte", "end_byte", "score"}]}`. Offsets are zero-based, half-open UTF-8 byte indices into the decoded `text`.
- Results are ordered by score descending, then by earlier `start_byte`, then by `text`. Spans never overlap.
- `score` is in `[0, 1]`, and the best phrase always scores 1. It is a ranking signal relative to this document, not a probability, and it is not comparable across documents.
- Empty or whitespace-only text returns `{"language":"xx","keywords":[]}`.
- **400** with `{"error": "..."}` for: malformed JSON, a non-object body, trailing data, a missing, null, unsupported or non-lowercase language, a missing or null text, a limit below 1, or wrong field types (for example `"limit":"5"` or `2.5`).
- **413** when the decoded `text` exceeds 64 KiB (65,536 bytes; exactly 65,536 is accepted), or when the raw body exceeds 400 KiB. That is six times 64 KiB plus 16 KiB, which fits the worst case where every byte of text is JSON-escaped as `\u00XX`.
- **405** for other methods on `/v1/keywords`; `GET /healthz` returns 200 `{"status":"ok"}`.

`POST /v1/text` with body `{"url": "http...|https..."}`.

This endpoint is a development helper. Its main-content rules were tuned on Ghost blogs only; on other sites it falls back to the `article`, `main` or body element and may keep page chrome.

The response is `{"text": string, "truncated": bool}`. For an HTML page, `text` is the article's title on the first line, the post's excerpt on the second when the page has one, then the article body. Block elements become line breaks, and whitespace inside each line is collapsed. A `text/plain` response is returned as the body, without HTML parsing.

The body is found in three steps (`internal/webtext/content.go`):

1. **Prune boilerplate.** An element is dropped with everything inside it when it is `nav`, `aside`, `form`, `button`, `dialog`, `header` or `footer`; has a `navigation`, `banner`, `contentinfo`, `complementary`, `search` or `dialog` role; is `hidden` or `aria-hidden`; or has a class or id containing a boilerplate word as whole `-`/`_` parts (`nav`, `menu`, `header`, `footer`, `sidebar`, `share`, `subscribe`, `newsletter`, `related`, `comments`, `cookie`, `author`, `byline`, `meta`, `tag`, `cta`, and more). Ghost's own blocks are included: `gh-head`, `gh-foot`, signup, bookmark and CTA cards. `subheader` or `canvas` don't match, and Ghost's per-post `tag-*` and `author-*` classes are ignored. Two exceptions: an element holding more than half of the page's text is never pruned by its class, and a CTA card without a button is kept as a callout, minus its "SPONSORED" label. `script`, `style`, `noscript`, `template`, `svg`, `iframe` and `pre` are always dropped.
2. **Choose the container.** The first match wins: an element whose class is exactly `gh-content`, `post-content`, `article-content`, `entry-content`, `post-body` or `article-body`; else the `article` with the most paragraph, list and heading text, if it has at least 250 characters of it; else the same for `main` or `role="main"`; else the pruned `body`.
3. **Write the text.** The title is the first `h1` of the post (read from its header even though the header is pruned), else `og:title`, else `<title>`. The lede is the post's excerpt or standfirst. The body then drops blocks that are only a short link ("Become a member"), photo-credit captions, date, time and reading-time lines, repeats of the title or lede, and headings left at the end with nothing under them ("Comments").

`truncated` is true when the page text was longer than 64 KiB. `text` is then cut to 64 KiB on a UTF-8 boundary, so it is always a valid `text` value for `POST /v1/keywords`.

- **400** with `{"error": "..."}` for malformed JSON, a non-object body, trailing data, a missing or null url, a wrong field type, an invalid url, a scheme other than `http` or `https`, or a response that is not HTML or plain text.
- **413** when the request body exceeds 16 KiB.
- **502** when the fetch fails: DNS, connection, timeout, more than 5 redirects, or a non-2xx status.
- **405** for other methods on `/v1/text`.

The server fetches the URL it is given, including private addresses.

# Development

`server` is a development tool used to build the evaluation set. It is not part of the published package, not a released binary, and not a supported API. Library users call `extractor.Extract` with the text, a language, a limit, and an optional `ExtractOptions` value. The call returns the phrases, the count summary, and an error.

## Requirements and versions

| component | version | licence |
|---|---|---|
| Go minimum | `go 1.26.0` (`go` line in `go.mod`). `errors.AsType` and `golang.org/x/net` v0.59.0 both require 1.26.0 | BSD-3-Clause |
| Go toolchain | go1.27.3 (`toolchain` line in `go.mod`). Used when working in this module | BSD-3-Clause |
| [github.com/rivo/uniseg](https://github.com/rivo/uniseg) | v0.4.7 (pinned in `go.mod`/`go.sum`) | MIT |
| [golang.org/x/net](https://pkg.go.dev/golang.org/x/net) | v0.59.0 (HTML parse and charset; pulls in `golang.org/x/text` v0.42.0). Used by `internal/webtext` for `cleancheck` and `just pages`, not by `extractor` or the HTTP server | BSD-3-Clause |
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
| `just site` | serve the documentation site and Pages Functions locally with Wrangler |
| `just site-test` | run the site Function unit tests (no Firecrawl, no Go server) |

A user-facing change is a section at the top of `CHANGELOG.md`. The heading is `## v0.1.1` or `## v0.1.1 - YYYY-MM-DD`, and the text says what a caller of the library can do now. Commit that and push `main`.

A pull request into `main` runs test and lint. Every push to `main` runs them again. The tag and the GitHub release happen only after both succeed and that push changed `CHANGELOG.md`. The release notes are the commits since the previous tag, newest first, and each line names its author. The changelog is the product summary. A push to another branch does not publish.

A change under `site/` that ships on the published site also deploys it to the Cloudflare Pages project `keyphrase`. A pull request deploys a preview on the pull request branch. The merge to `main` deploys production. A docs-only change, or a test-only change, leaves the site unpublished.

The Try page calls `POST /extract` on the same host. That Pages Function proxies to the Go server on a VPS when `EXTRACT_ORIGIN` and `EXTRACT_TOKEN` are set. The server image is `ghcr.io/tericcabrel/keyphrase/server` (built from [`site/Dockerfile`](../site/Dockerfile) on pushes to `main` that touch the server). Deploy on the VPS with [`site/deploy-keyphrase-server.example.sh`](../site/deploy-keyphrase-server.example.sh) (copy to a local script with your token; the real script is gitignored). Set the GHCR package to public after the first push. Point Cloudflare Tunnel or your reverse proxy at `127.0.0.1:8080` and set `EXTRACT_ORIGIN` to that URL. `EXTRACT_TOKEN` must match `KEYPHRASE_TOKEN` on the container.

## Folder structure

```
extractor/          core: tokenize.go, segment.go, candidates.go, rank.go, select.go, extractor.go
lang/               registry, Pack, Normalize; THIRD_PARTY.md; data/SNOWBALL_COPYING
lang/en fr de es it pt  one package per language: word lists, model.bin.gz, rarity.bin.gz
lang/all            blank-imports the six languages, for tests and local commands
postag/             part-of-speech tagger: features, model format, training, evaluation
postag/models/      fetch.sh (treebank download) and train.sh
rarity/             corpus rarity tables, fetch.sh, build.sh
cmd/train-pos/      trains a model from CoNLL-U with the extractor's tokenizer
internal/treebank/  CoNLL-U reader shared by train-pos and eval
cmd/build-rarity/   counts Leipzig sentences into a rarity table
internal/webtext/   fetch a URL and return its title and main content
cmd/server/         development HTTP server; httpapi.go and httpapi_test.go
site/Dockerfile     multi-stage image for ghcr.io/tericcabrel/keyphrase/server
cmd/samples/        runs example files through the HTTP handler, prints Markdown
cmd/eval/           scores output against labels in data-text/, plus label-free defect checks
cmd/cleancheck/     checks extracted texts and adds pages from URL lists
scripts/add-pages.sh  fetch, check, write, draft labels, hold out
internal/eval/      rarity override for the eval command, and the held-out page list
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
```

## Local site

The site is static files under `site/public` plus Pages Functions (`/page`, `/extract`). Wrangler serves both.

1. Copy [`site/.dev.vars.example`](../site/.dev.vars.example) to `site/.dev.vars`.
2. Set `EXTRACT_ORIGIN` (for example `http://127.0.0.1:8080`) and `EXTRACT_TOKEN` (for example `dev`). Both are required for the Try page to enable extraction. When the Go server runs with `KEYPHRASE_TOKEN`, use the same value for `EXTRACT_TOKEN`.
3. Optionally set `FIRECRAWL_API_KEY` for URL loading on the Try page.
4. In one terminal, start the Go server: `just dev` or `go run ./cmd/server -addr :8080`.
5. In another terminal, from the repository root: `just site` (or `cd site && yarn dev`).

Wrangler prints a local URL (often `http://localhost:8788`). `localhost` is allowed for `/page` and `/extract`.

Production uses the same variable names as Cloudflare Pages secrets: `EXTRACT_ORIGIN`, `EXTRACT_TOKEN`, and `FIRECRAWL_API_KEY`.
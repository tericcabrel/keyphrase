# Project guidelines

keyphrase is a library. A caller passes text, a language, and a limit, and receives phrases. The HTTP server and the local commands are development tools.

How extraction works, the layout, and how quality is judged already live in these docs. This file states the coding rules.

- [How extraction works](docs/algorithm.md)
- [Development](docs/development.md) — layout, commands, HTTP contract
- [Quality](docs/quality.md) and [Quality history](docs/quality-history.md)

## Functional core and shell

The extraction pipeline, the tagger, the rarity tables, and the language packs are the functional core. They take values and return values. They do not open the network, touch the filesystem, read the clock, log, or take a `context.Context`.

The HTTP adapter decodes the request, calls extraction, maps known errors to status codes, and writes JSON. Ranking stays in the core.

Fetching a page and the local commands own I/O.

Orchestration is one function in the core. Keep it a function. A decision helper receives the data it needs, such as tokens, a language pack, and ranking weights, and returns a value. It does not fetch.

The library has no domain, use case, or repository packages. Keep new behavior in the package that already owns it.

## Immutability

A returned phrase is a slice of the caller's text, for example `text[start:end]`. Leave the caller's string unchanged.

Public results are values. They have no setters. A selected phrase, a count summary, and a set of ranking weights are examples.

Invalid input returns a sentinel error. At the fetch boundary, return a typed error that carries the status to use. Empty or whitespace-only text is success and returns an empty result.

The pipeline may fill its own scratch in place, for example by writing tag flags onto a token slice or by reusing a buffer with `words[:0]`. Leave that in place.

Phrase order is part of the contract: score descending, then earlier position, then the phrase text. Ranging a map must not decide that order.

## Dependency injection

Pass policy as a value. Ranking weights on the extraction call are an example. Prefer a value parameter over an interface.

Language packs register themselves when their package is imported. That registry is how a program adds a language. Leave it as the extension point.

Add an interface only beside the code that calls it, and only when a second implementation or a test fake exists. Do not generate mocks.

Wire the process at the program entrypoint. Building the HTTP handler stays a function.

The page fetcher may keep its HTTP client beside the fetch code. A test that needs a fake client passes it at that boundary. Leave the client out of extraction.

## Errors

Library failures that callers branch on are sentinel errors. An unsupported language and a limit below 1 are examples.

The fetch shell uses a typed error when the HTTP status depends on the failure.

Compare with `errors.Is` and `errors.AsType`. The handler maps those known errors to status codes.

## Tests and commands

Write table-driven tests with explicit fixtures. Name the test for the behavior. Assert spans, scores, and error values.

Fuzzing is the random suite. Other tests use fixed inputs. Tests do not open the network.

Run checks from the repository root with `just test` and `just lint`. When one of those fails, fix the failure it reports.

## Style

Format with `gofmt` and `goimports`. Imports stay at the top of the file.

A comment explains an invariant or a non-obvious decision. The core package comment is the public contract. Keep it.

Add an abstraction when the same behavior already appears in more than one place.

Change only what was asked. Stop and say so before a change that would break a documented invariant: verbatim spans, non-overlapping results, deterministic order, no verbs inside phrases, or the documented text size limit.

Early returns stay on the next line. The linter is configured that way, so a blank line before every return is not the style.

## Naming

A name says what the value holds or what it is for. Prefer a whole word to an abbreviation: `weights`, `sentenceCounts`, `keywords`, not `w`, `df`, `kws`.

Short names are fine in a small scope: a loop index (`i`, `j`), a rune in a rune loop (`r`), a comparator's `a, b`, `err`, `ok`, `ctx`, and the `w http.ResponseWriter, r *http.Request` pair. Once a variable lives past a few lines or is a function parameter, give it a full name.

When ranging over a map, name both the key and the value: `for word, count := range sentenceCounts`. Do the same for a slice whose elements are not obvious: `for _, phrase := range phrases`.

Method receivers keep the usual one- or two-letter Go form (`t *Table`).

Constants use Go MixedCaps (`maxWords`, `MinCount`). An exported name is public API. Do not rename it without saying so.

If the natural name matches a type in the same scope (`phrase`, `token`), use it, unless the body also needs the type. In that case pick a role name such as `candidate`.

## Commits

Write commit messages as [Conventional Commits](https://www.conventionalcommits.org/): `type: subject`.

The subject is one imperative line, lowercase, with no trailing period. A body is optional and says why.

| type | use |
|---|---|
| `feat` | a new capability for a caller |
| `fix` | a bug |
| `docs` | documentation only |
| `test` | tests only |
| `refactor` | a behavior-preserving change |
| `ci` | workflows and release tooling |

A breaking change adds `!` after the type (`feat!:`) and a `BREAKING CHANGE:` footer. `feat` and `fix` add a section at the top of `CHANGELOG.md`. `docs` leaves the changelog unchanged.

# Security policy

## Supported versions

Only the latest release receives fixes.

## Reporting a vulnerability

Report a vulnerability privately through [GitHub's private vulnerability reporting](https://github.com/tericcabrel/keyphrase/security/advisories/new), or by email to [tericcabrel@yahoo.com](mailto:tericcabrel@yahoo.com). Do not open a public issue.

Include the version, the language, and an input that reproduces the problem. You should get a reply within a week.

## Scope

The library is `extractor`, `lang`, `postag`, and `rarity`. The HTTP server in `cmd/server` is a local development tool, and `POST /v1/text` fetches any URL it is given, including private addresses. Do not expose it to a network. Reports about that behaviour are out of scope.

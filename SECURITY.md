# Security policy

## Supported versions

Only the latest release receives fixes.

## Reporting a vulnerability

Report a vulnerability privately through [GitHub's private vulnerability reporting](https://github.com/tericcabrel/keyphrase/security/advisories/new), or by email to [tericcabrel@yahoo.com](mailto:tericcabrel@yahoo.com). Do not open a public issue.

Include the version, the language, and an input that reproduces the problem. You should get a reply within a week.

## Scope

The library is `extractor`, `lang`, `postag`, and `rarity`. The HTTP server in `cmd/server` exposes only `POST /v1/keywords` and `GET /healthz`. It is also built for the Try page proxy; treat network exposure like any other API that accepts user text.

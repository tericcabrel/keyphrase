# Changelog

## v0.2.0

Add Portuguese (`pt`) language support (`github.com/tericcabrel/keyphrase/lang/pt`), with its own part-of-speech model and corpus rarity table.

BREAKING CHANGE: One `Extract` call replaces the older variants. Optional `ExtractOptions` tune phrase filters; custom ranking weights are no longer exposed.

BREAKING CHANGE: Count fields are renamed to `Returned` and `ReturnedMoreThanOneWord` (JSON: `returned`, `returned_more_than_one_word`).

Phrase filters are configurable in code. Defaults drop more lone common nouns and one-off generic phrases on longer text, with fewer awkward results; short inputs are largely unchanged.

## v0.1.0

First release! Extract short topical phrases that occur verbatim in
plain text, for English, French, German, Spanish, and Italian. Import only
the language packages you need.


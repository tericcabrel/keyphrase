# keyphrase

[![Release](https://github.com/tericcabrel/keyphrase/actions/workflows/release.yml/badge.svg?branch=main)](https://github.com/tericcabrel/keyphrase/actions/workflows/release.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/tericcabrel/keyphrase.svg)](https://pkg.go.dev/github.com/tericcabrel/keyphrase)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

keyphrase extracts a few short topical phrases that occur verbatim in plain text. The caller names the language and sends clean article text. A phrase is at most three words, verbs are left out, and a single word is the exception.

Supported languages are English (`en`), French (`fr`), German (`de`), Spanish (`es`), Italian (`it`), and Portuguese (`pt`). There is no language detection. Requires Go 1.26.

The website is [keyphrase.tericcabrel.com](https://keyphrase.tericcabrel.com). Its [Try page](https://keyphrase.tericcabrel.com/try/) runs the extractor in the browser on any text.

Accuracy is measured on texts that are not versioned. The tools are in this repository and run on any text. The [limitations](#limitations) say what the extractor leaves out.

## Install

```sh
go get github.com/tericcabrel/keyphrase@v0.2.0
```

```go
import (
    "github.com/tericcabrel/keyphrase/extractor"
    _ "github.com/tericcabrel/keyphrase/lang/fr"
)

kws, err := extractor.Extract(text, "fr", 10)
// kws[i].Text == text[kws[i].StartByte:kws[i].EndByte]
```

## Results

English (`en`). The passage is the start of [examples/en/offshore-wind.txt](examples/en/offshore-wind.txt).

```go
import (
    "github.com/tericcabrel/keyphrase/extractor"
    _ "github.com/tericcabrel/keyphrase/lang/en"
)

text := `Offshore wind farms are becoming the backbone of Britain's energy
strategy. The government wants offshore wind capacity to reach 50
gigawatts by 2030, enough to power every home in the country.

The largest project, Dogger Bank Wind Farm, sits more than 130
kilometres off the Yorkshire coast. When complete, Dogger Bank will
use 277 turbines, each taller than the Eiffel Tower. Engineers say
the turbines can generate electricity even in moderate winds.

Grid connections remain the biggest obstacle. National Grid has
warned that some offshore wind farms may wait years before they can
connect to the transmission network. Developers also face rising
steel prices and higher interest rates, which have pushed up the
cost of building turbines at sea.`

kws, err := extractor.Extract(text, "en", 10)
```

```text
┌─────────────────┬────────┬─────────┐
│ phrase          │  score │   bytes │
├─────────────────┼────────┼─────────┤
│ Offshore wind   │ 1.0000 │    0-13 │
│ Dogger Bank     │ 0.8017 │ 309-320 │
│ Britain         │ 0.7768 │   49-56 │
│ Yorkshire       │ 0.7406 │ 277-286 │
│ turbines        │ 0.7376 │ 334-342 │
│ Eiffel Tower    │ 0.7371 │ 365-377 │
│ National Grid   │ 0.6649 │ 502-515 │
│ backbone        │ 0.5958 │   37-45 │
│ wind capacity   │ 0.5389 │ 106-119 │
│ largest project │ 0.4944 │ 199-214 │
└─────────────────┴────────┴─────────┘
```

French (`fr`). The passage is the start of [examples/fr/tgv-sncf.txt](examples/fr/tgv-sncf.txt).

```go
import (
    "github.com/tericcabrel/keyphrase/extractor"
    _ "github.com/tericcabrel/keyphrase/lang/fr"
)

text := `La SNCF a présenté mardi son nouveau TGV M, un train à grande
vitesse qui doit entrer en service commercial l'année prochaine.
Construit par Alstom à La Rochelle, le TGV M pourra transporter
jusqu'à 740 voyageurs.

Selon la SNCF, ce train consommera 20 % d'énergie en moins que les
rames actuelles. Sa conception modulaire permettra de transformer
une voiture de première classe en voiture de seconde classe en
quelques heures.

Les premières lignes concernées seront Paris-Lyon et
Paris-Marseille, les axes les plus fréquentés du réseau à grande
vitesse. La SNCF espère ainsi répondre à la hausse de la demande,
alors que les trains sont souvent complets pendant les vacances
scolaires.`

kws, err := extractor.Extract(text, "fr", 10)
```

```text
┌──────────────────────┬────────┬─────────┐
│ phrase               │  score │   bytes │
├──────────────────────┼────────┼─────────┤
│ SNCF                 │ 1.0000 │     3-7 │
│ TGV                  │ 0.9124 │   39-42 │
│ Alstom               │ 0.7690 │ 145-151 │
│ La Rochelle          │ 0.7548 │ 155-166 │
│ Paris-Lyon           │ 0.7072 │ 478-488 │
│ service commercial   │ 0.5198 │  92-110 │
│ conception modulaire │ 0.5031 │ 309-329 │
│ Paris-Marseille      │ 0.4994 │ 492-507 │
│ rames actuelles      │ 0.4846 │ 289-304 │
│ seconde classe       │ 0.4109 │ 401-415 │
└──────────────────────┴────────┴─────────┘
```

Spanish (`es`). The passage is the start of [examples/es/inteligencia-artificial.txt](examples/es/inteligencia-artificial.txt).

```go
import (
    "github.com/tericcabrel/keyphrase/extractor"
    _ "github.com/tericcabrel/keyphrase/lang/es"
)

text := `El Parlamento Europeo aprobó el miércoles la primera ley integral
del mundo sobre inteligencia artificial. La norma, conocida como Ley
de Inteligencia Artificial, clasifica los sistemas según su nivel de
riesgo y prohíbe algunos usos considerados inaceptables.

Entre las prácticas prohibidas figuran los sistemas de puntuación
social y el reconocimiento de emociones en el lugar de trabajo. El
reconocimiento facial en espacios públicos solo se permitirá en
casos excepcionales, como la búsqueda de víctimas de secuestro.

Las empresas que desarrollen modelos de inteligencia artificial de
uso general deberán publicar resúmenes de los datos utilizados para
entrenarlos. Las multas por incumplimiento podrán alcanzar el 7 % de
la facturación mundial.

España ha sido pionera en la supervisión de la inteligencia
artificial.`

kws, err := extractor.Extract(text, "es", 10)
```

```text
┌─────────────────────────────┬────────┬─────────┐
│ phrase                      │  score │   bytes │
├─────────────────────────────┼────────┼─────────┤
│ inteligencia artificial     │ 1.0000 │  84-107 │
│ Parlamento Europeo          │ 0.7819 │    3-21 │
│ ley                         │ 0.7565 │   55-58 │
│ prácticas prohibidas        │ 0.5287 │ 276-297 │
│ sistemas de puntuación      │ 0.4978 │ 310-333 │
│ reconocimiento facial       │ 0.4877 │ 401-422 │
│ reconocimiento de emociones │ 0.4752 │ 346-373 │
│ casos excepcionales         │ 0.4684 │ 467-486 │
│ resúmenes                   │ 0.4442 │ 631-641 │
│ víctimas de secuestro       │ 0.4424 │ 509-531 │
└─────────────────────────────┴────────┴─────────┘
```

## Languages

| language | package | code |
|---|---|---|
| English | `lang/en` | `en` |
| French | `lang/fr` | `fr` |
| German | `lang/de` | `de` |
| Spanish | `lang/es` | `es` |
| Italian | `lang/it` | `it` |
| Portuguese | `lang/pt` | `pt` |

The blank import registers that language. Import only the languages you need. Each one embeds its own part-of-speech model and rarity table: French is about 3.8 MB gzipped (a 3.0 MB model and an 820 KB table). Import more than one package to bundle a subset. `lang/all` embeds and registers every built-in language.

`lang`, `postag`, and `rarity` are public. `lang.Pack` is how another language is added. `extractor.Extract` and `ExtractWithCounts`, together with the language imports, are the stable call.

## Limitations

- Accuracy is measured on texts that are not versioned. The tools are in this repository and run on any text. See [Quality and evaluation](docs/quality.md).
- The caller passes the language. Nothing detects it.
- The tagger misses some verbs, and sometimes drops a word that is not a verb.
- Names longer than three words are split.
- The input must already be article text. Menus, footers, and other page chrome can come back as phrases.

The full review is in [docs/quality.md](docs/quality.md).

## Adding a language

Implement `lang.Pack` and call `lang.Register` from `init`, then import that package. The built-in packages are the pattern: stopword lists, a variant key, and optional model and rarity files. The steps are in [docs/algorithm.md](docs/algorithm.md).

## Contributing

Issues and pull requests are welcome. Run `just test` and `just lint` before opening a pull request; CI runs the same commands. Add a user-facing change to `CHANGELOG.md`. The coding rules are in [AGENTS.md](AGENTS.md), and the layout and commands are in [docs/development.md](docs/development.md). Report a vulnerability as described in [SECURITY.md](SECURITY.md).

## License

The code is [MIT](LICENSE). The embedded models, rarity tables, and Snowball word lists keep their own terms. [NOTICE](NOTICE) points at [lang/THIRD_PARTY.md](lang/THIRD_PARTY.md).

## Further reading

- [How extraction works](docs/algorithm.md)
- [Quality and evaluation](docs/quality.md)
- [Development](docs/development.md)

package main

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/tericcabrel/keyphrase/extractor"
	_ "github.com/tericcabrel/keyphrase/lang/en"
	_ "github.com/tericcabrel/keyphrase/lang/es"
	_ "github.com/tericcabrel/keyphrase/lang/fr"
)

func main() {
	extract("en", enText)
	extract("fr", frText)
	extract("es", esText)
}

const enText = `Offshore wind farms are becoming the backbone of Britain's energy
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

const frText = `La SNCF a présenté mardi son nouveau TGV M, un train à grande
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

const esText = `El Parlamento Europeo aprobó el miércoles la primera ley integral
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

func extract(lang, text string) {
	kws, err := extractor.Extract(text, lang, 10)
	if err != nil {
		panic(err)
	}
	type row struct{ phrase, score, bytes string }
	rows := make([]row, 1, 1+len(kws))
	rows[0] = row{"phrase", "score", "bytes"}
	for _, kw := range kws {
		if text[kw.StartByte:kw.EndByte] != kw.Text {
			panic("span mismatch " + kw.Text)
		}
		rows = append(rows, row{kw.Text, fmt.Sprintf("%.4f", kw.Score), fmt.Sprintf("%d-%d", kw.StartByte, kw.EndByte)})
	}
	w := [3]int{}
	for _, r := range rows {
		for i, cell := range []string{r.phrase, r.score, r.bytes} {
			n := utf8.RuneCountInString(cell)
			if n > w[i] {
				w[i] = n
			}
		}
	}
	pad := func(s string, n int, right bool) string {
		sp := strings.Repeat(" ", n-utf8.RuneCountInString(s))
		if right {
			return sp + s
		}
		return s + sp
	}
	bar := func(left, mid, right string) string {
		return left + strings.Repeat("─", w[0]+2) + mid + strings.Repeat("─", w[1]+2) + mid + strings.Repeat("─", w[2]+2) + right
	}
	fmt.Printf("## %s\n", lang)
	fmt.Println(bar("┌", "┬", "┐"))
	fmt.Printf("│ %s │ %s │ %s │\n", pad(rows[0].phrase, w[0], false), pad(rows[0].score, w[1], true), pad(rows[0].bytes, w[2], true))
	fmt.Println(bar("├", "┼", "┤"))
	for _, r := range rows[1:] {
		fmt.Printf("│ %s │ %s │ %s │\n", pad(r.phrase, w[0], false), pad(r.score, w[1], true), pad(r.bytes, w[2], true))
	}
	fmt.Println(bar("└", "┴", "┘"))
	fmt.Println()
}

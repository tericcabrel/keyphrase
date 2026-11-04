// Command eval scores extractor output against the labels in data-text.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"log"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/tericcabrel/keyphrase/extractor"
	"github.com/tericcabrel/keyphrase/internal/eval"
	"github.com/tericcabrel/keyphrase/lang"

	_ "github.com/tericcabrel/keyphrase/lang/all"
)

const (
	markGood       = '+'
	markAwkward    = 'A'
	markIrrelevant = 'I'
	markUnjudged   = '?'
)

var topK = [...]int{10, 50, 100}

const numBands = len(topK) + 1

var labelDepth = topK[len(topK)-1]

const tailSampleEvery = 5

// The sample hashes the document and the keyword, not its rank, so a keyword
// stays in or out when a ranking change moves it.
func inTailSample(docName, key string) bool {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(docName + "\x00" + key))
	return hash.Sum32()%tailSampleEvery == 0
}

func wantsLabel(docName string, rank int, key string) bool {
	return rank < labelDepth || inTailSample(docName, key)
}

func bandOf(rank int) int {
	for band, k := range topK {
		if rank < k {
			return band
		}
	}
	return len(topK)
}

func bandName(band int) string {
	switch {
	case band == 0:
		return fmt.Sprintf("1-%d", topK[0])
	case band < len(topK):
		return fmt.Sprintf("%d-%d", topK[band-1]+1, topK[band])
	default:
		return fmt.Sprintf("%d+", topK[len(topK)-1]+1)
	}
}

type label struct {
	mark byte
	text string
}

type labels map[string]label

type doc struct {
	Name     string           `json:"name"` // "<lang>/<name>"
	Language string           `json:"language"`
	Keywords []string         `json:"keywords"`
	Defects  map[string][]int `json:"defects,omitempty"` // check name to zero-based ranks
}

// Limit 0 kept every phrase.
type run struct {
	Time         string   `json:"time"`
	Limit        int      `json:"limit"`
	DefectChecks []string `json:"defect_checks,omitempty"`
	Docs         []doc    `json:"docs"`
	Short        []doc    `json:"short,omitempty"` // each page cut to shortTextWords and uses the full page's labels.
}

const shortTextWords = 150

type metrics struct {
	Docs                          int
	Good, Judged                  [len(topK)]int
	Awkward, Irrelevant, Unjudged int // in the first labelDepth
	Missed                        int // labelled good, not returned
	BandReturned                  [numBands]int
	// Keywords the band scores: every keyword up to labelDepth, then the tail sample.
	BandSampled          [numBands]int
	BandJudged, BandGood [numBands]int
}

func (m *metrics) add(o metrics) {
	m.Docs += o.Docs
	for i := range topK {
		m.Good[i] += o.Good[i]
		m.Judged[i] += o.Judged[i]
	}
	for band := range numBands {
		m.BandReturned[band] += o.BandReturned[band]
		m.BandSampled[band] += o.BandSampled[band]
		m.BandJudged[band] += o.BandJudged[band]
		m.BandGood[band] += o.BandGood[band]
	}
	m.Awkward += o.Awkward
	m.Irrelevant += o.Irrelevant
	m.Unjudged += o.Unjudged
	m.Missed += o.Missed
}

func (m *metrics) precision(i int) float64 {
	if m.Judged[i] == 0 {
		return -1
	}
	return float64(m.Good[i]) / float64(m.Judged[i])
}

// Returning fewer keywords cannot raise it.
func (m *metrics) goodPerDoc(i int) float64 {
	if m.Docs == 0 {
		return 0
	}
	return float64(m.Good[i]) / float64(m.Docs)
}

func main() {
	dir := flag.String("dir", "data-text", "directory of <lang>/<name>.txt files")
	limit := flag.Int("limit", 0, "keywords per document; 0 keeps every phrase")
	treebanks := flag.String("treebanks", "postag/models/ud", "cached treebanks for the word-class defect checks (just pos-fetch)")
	draft := flag.Bool("draft", false, "append unlabelled keywords of the tuning pages to the labels files as ?")
	draftHoldout := flag.Bool("draft-holdout", false, "append unlabelled keywords of the held-out pages to the labels files as ?, printing only counts")
	baseline := flag.String("baseline", "", "saved run to compare against")
	save := flag.Bool("save", true, "save this run under <dir>/.runs/")
	verbose := flag.Bool("v", false, "list unjudged and missed anchors per document")
	rarityWeight := flag.Float64("rarity", -1, "override the rarity weight; negative keeps the built-in weight")
	rarityPhrase := flag.String("rarity-phrase", "", "mean or max over a phrase's content words; empty keeps the built-in rule")
	singleMin := flag.Float64("single-rarity-min", -1, "drop one-word keywords below this rarity; negative keeps the built-in rule")
	dropLoneNouns := flag.Bool("drop-lone-nouns", extractor.DefaultExtractOptions.DropLoneNouns, "drop a one-word keyword unless the tagger calls it a name, it is capitalized like one, or it is rare enough")
	loneMin := flag.Float64("lone-noun-min", -1, "with -drop-lone-nouns, keep a one-word common noun whose rarity is at least this; 0 drops every lone common noun; negative keeps the built-in threshold")
	dropOneOff := flag.Bool("drop-one-off", extractor.DefaultExtractOptions.DropOneOffPhrases, "drop a multi-word keyword seen once unless it contains a name or is capitalized like one")
	oneOffMinOthers := flag.Int("one-off-min-others", -1, "with -drop-one-off, drop only when at least this many other candidates remain; negative keeps the built-in value")
	tuningOnly := flag.Bool("tuning-only", false, "do not print held-out scores")
	flag.Parse()

	options, ranking, err := evalWeights(*rarityWeight, *rarityPhrase, *singleMin, *dropLoneNouns, *loneMin, *dropOneOff, *oneOffMinOthers)
	if err != nil {
		log.Fatal(err)
	}
	if *limit < 0 {
		log.Fatal("-limit must be 0 or more")
	}
	lexicons, missing, err := loadLexicons(*treebanks)
	if err != nil {
		log.Fatal(err)
	}
	if len(missing) > 0 {
		_, _ = fmt.Fprintf(os.Stderr, "no treebanks under %s for %s: only the %s check runs (just pos-fetch)\n", *treebanks, strings.Join(missing, ", "), checkFragment)
		lexicons = nil
	}
	cur, all, err := evaluate(*dir, *limit, options, ranking, lexicons, os.Stderr)
	if err != nil {
		log.Fatal(err)
	}
	holdout, err := eval.ReadHoldout(*dir)
	if err != nil {
		log.Fatal(err)
	}
	if *draft || *draftHoldout {
		include := func(name string) bool { return holdout[name] && *draftHoldout || !holdout[name] && *draft }
		if err := draftLabels(os.Stdout, *dir, slices.Concat(cur.Docs, cur.Short), all, include); err != nil {
			log.Fatal(err)
		}
	}
	report(os.Stdout, cur, all, *verbose, holdout, *tuningOnly)
	if *save {
		path, err := saveRun(*dir, cur)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("\nsaved %s\n", path)
	}
	if *baseline != "" {
		base, err := loadRun(*baseline)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println()
		compare(os.Stdout, base, cur, all, holdout, *tuningOnly)
	}
}

// Negative and empty flag values leave the built-in options and ranking unchanged.
func evalWeights(rarityWeight float64, phrase string, singleMin float64, dropLoneNouns bool, loneMin float64, dropOneOff bool, oneOffMinOthers int) (extractor.ExtractOptions, eval.Ranking, error) {
	options := extractor.DefaultExtractOptions
	options.DropLoneNouns = dropLoneNouns
	if loneMin >= 0 {
		options.LoneNounMinRarity = loneMin
	}
	options.DropOneOffPhrases = dropOneOff
	if oneOffMinOthers >= 0 {
		options.OneOffMinOthers = oneOffMinOthers
	}
	ranking := eval.DefaultRanking
	if rarityWeight >= 0 {
		ranking.Rarity = rarityWeight
	}
	switch phrase {
	case "":
	case "mean":
		ranking.PhraseRarityMax = false
	case "max":
		ranking.PhraseRarityMax = true
	default:
		return options, ranking, fmt.Errorf("-rarity-phrase must be mean or max, not %q", phrase)
	}
	if singleMin >= 0 {
		ranking.MinSingleRarity = singleMin
	}
	return options, ranking, nil
}

func evaluate(dir string, limit int, options extractor.ExtractOptions, ranking eval.Ranking, lexicons map[string]lexicon, warn io.Writer) (run, map[string]labels, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*", "*.txt"))
	if err != nil {
		return run{}, nil, err
	}
	slices.Sort(files)
	cur := run{Time: time.Now().UTC().Format(time.RFC3339), Limit: limit, DefectChecks: []string{checkFragment}}
	if lexicons != nil {
		cur.DefectChecks = defectChecks[:]
	}
	extractLimit := limit
	if extractLimit == 0 {
		extractLimit = math.MaxInt
	}
	for _, f := range files {
		language := filepath.Base(filepath.Dir(f))
		if _, ok := lang.Get(language); !ok {
			_, _ = fmt.Fprintf(warn, "%s: skipped, unsupported language %q\n", f, language)

			continue
		}
		raw, readErr := os.ReadFile(f)
		if readErr != nil {
			return run{}, nil, readErr
		}
		text, ok := decodeText(raw)
		if !ok {
			_, _ = fmt.Fprintf(warn, "%s: not a valid JSON string, decoded \\n \\\" \\t only\n", f)
		}
		name := language + "/" + strings.TrimSuffix(filepath.Base(f), ".txt")
		for _, view := range []struct {
			text string
			docs *[]doc
		}{{text, &cur.Docs}, {shortText(text), &cur.Short}} {
			d, extractErr := extractDoc(name, language, view.text, extractLimit, options, ranking, lexicons[language])
			if extractErr != nil {
				return run{}, nil, fmt.Errorf("%s: %w", f, extractErr)
			}
			*view.docs = append(*view.docs, d)
		}
	}
	if len(cur.Docs) == 0 {
		return run{}, nil, fmt.Errorf("no <lang>/*.txt files under %s", dir)
	}
	all, err := loadAllLabels(dir, cur.Docs)
	return cur, all, err
}

func extractDoc(name, language, text string, limit int, options extractor.ExtractOptions, ranking eval.Ranking, words lexicon) (doc, error) {
	got, err := eval.Extract(text, language, limit, eval.Options{
		DropLoneNouns:     options.DropLoneNouns,
		LoneNounMinRarity: options.LoneNounMinRarity,
		DropOneOffPhrases: options.DropOneOffPhrases,
		OneOffMinOthers:   options.OneOffMinOthers,
	}, ranking)
	if err != nil {
		return doc{}, err
	}
	kws := make([]extractor.Keyword, len(got))
	for i, keyword := range got {
		kws[i] = extractor.Keyword{
			Text:      keyword.Text,
			StartByte: keyword.StartByte,
			EndByte:   keyword.EndByte,
			Score:     keyword.Score,
		}
	}
	tokens, err := segmentTokens(text, language)
	if err != nil {
		return doc{}, err
	}
	d := doc{Name: name, Language: language, Defects: findDefects(text, tokens, kws, words)}
	for _, k := range kws {
		d.Keywords = append(d.Keywords, k.Text)
	}
	return d, nil
}

func shortText(text string) string {
	words, end := 0, 0
	for end < len(text) && words < shortTextWords {
		next := strings.IndexByte(text[end:], '\n')
		if next < 0 {
			next = len(text) - end
		} else {
			next++
		}
		words += len(strings.Fields(text[end : end+next]))
		end += next
	}
	return text[:end]
}

// Texts are stored as a JSON string body. When that fails to parse, only
// \n, \" and \t are replaced and ok is false.
func decodeText(raw []byte) (text string, ok bool) {
	body := strings.TrimRight(string(raw), "\r\n")
	if err := json.Unmarshal([]byte(`"`+body+`"`), &text); err == nil {
		return text, true
	}
	return strings.NewReplacer(`\n`, "\n", `\"`, `"`, `\t`, "\t").Replace(body), false
}

func labelsPath(dir, name string) string {
	return filepath.Join(dir, filepath.FromSlash(name)+".labels.tsv")
}

func loadAllLabels(dir string, docs []doc) (map[string]labels, error) {
	all := map[string]labels{}
	for _, d := range docs {
		l, err := loadLabels(labelsPath(dir, d.Name))
		if err != nil {
			return nil, err
		}
		all[d.Name] = l
	}
	return all, nil
}

func loadLabels(path string) (labels, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return labels{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	l, err := parseLabels(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return l, nil
}

func parseLabels(r io.Reader) (labels, error) {
	l := labels{}
	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		mark, text, ok := strings.Cut(line, "\t")
		if !ok || len(mark) != 1 || !strings.ContainsRune("+AI?", rune(mark[0])) {
			return nil, fmt.Errorf("line %d: want \"<+|A|I|?>\\t<anchor>\", got %q", n, line)
		}
		key := matchKey(text)
		if key == "" {
			return nil, fmt.Errorf("line %d: empty anchor", n)
		}
		if prev, dup := l[key]; dup && prev.mark != mark[0] {
			return nil, fmt.Errorf("line %d: %q is labelled both %c and %s", n, text, prev.mark, mark)
		}
		l[key] = label{mark: mark[0], text: strings.TrimSpace(text)}
	}
	return l, sc.Err()
}

func matchKey(s string) string {
	return strings.Join(strings.Fields(lang.Normalize(s)), " ")
}

func score(docName string, keywords []string, l labels) metrics {
	m := metrics{Docs: 1}
	returned := map[string]bool{}
	for i, k := range keywords {
		key := matchKey(k)
		returned[key] = true
		lb, ok := l[key]
		if !ok {
			lb.mark = markUnjudged
		}
		if i < labelDepth {
			switch lb.mark {
			case markAwkward:
				m.Awkward++
			case markIrrelevant:
				m.Irrelevant++
			case markUnjudged:
				m.Unjudged++
			}
		}
		band := bandOf(i)
		m.BandReturned[band]++
		if !wantsLabel(docName, i, key) {
			continue
		}
		m.BandSampled[band]++
		if lb.mark == markUnjudged {
			continue
		}
		m.BandJudged[band]++
		if lb.mark == markGood {
			m.BandGood[band]++
		}
		for j, k := range topK {
			if i < k {
				m.Judged[j]++
				if lb.mark == markGood {
					m.Good[j]++
				}
			}
		}
	}
	for key, lb := range l {
		if lb.mark == markGood && !returned[key] {
			m.Missed++
		}
	}
	return m
}

// A page's full and short views share one labels file, so the labels are reloaded after each append.
func draftLabels(w io.Writer, dir string, docs []doc, all map[string]labels, include func(name string) bool) error {
	for _, d := range docs {
		if !include(d.Name) {
			continue
		}
		path := labelsPath(dir, d.Name)
		n, err := appendDraft(path, d, all[d.Name])
		if err != nil {
			return err
		}
		if n > 0 {
			_, _ = fmt.Fprintf(w, "%s: appended %d unjudged anchors\n", path, n)
		}
		if all[d.Name], err = loadLabels(path); err != nil {
			return err
		}
	}
	return nil
}

func appendDraft(path string, d doc, l labels) (int, error) {
	var lines []string
	seen := map[string]bool{}
	for rank, k := range d.Keywords {
		key := matchKey(k)
		if _, ok := l[key]; ok || seen[key] || !wantsLabel(d.Name, rank, key) {
			continue
		}
		seen[key] = true
		lines = append(lines, fmt.Sprintf("%c\t%s", markUnjudged, strings.Join(strings.Fields(k), " ")))
	}
	if len(lines) == 0 {
		return 0, nil
	}
	_, err := os.Stat(path)
	isNew := errors.Is(err, fs.ErrNotExist)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	var b strings.Builder
	if isNew {
		_, _ = fmt.Fprintf(&b, "# Labels for %s.txt. Marks: + good, A awkward, I irrelevant, ? unjudged.\n", d.Name)
	}
	for _, line := range lines {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if _, writeErr := f.WriteString(b.String()); writeErr != nil {
		_ = f.Close()

		return 0, writeErr
	}
	return len(lines), f.Close()
}

func saveRun(dir string, r run) (string, error) {
	runs := filepath.Join(dir, ".runs")
	if err := os.MkdirAll(runs, 0o755); err != nil {
		return "", err
	}
	t, err := time.Parse(time.RFC3339, r.Time)
	if err != nil {
		return "", err
	}
	path := filepath.Join(runs, t.Format("20060102-150405")+".json")
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, append(raw, '\n'), 0o644)
}

func loadRun(path string) (run, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return run{}, err
	}
	var r run
	if err := json.Unmarshal(raw, &r); err != nil {
		return run{}, fmt.Errorf("%s: %w", path, err)
	}
	return r, nil
}

func totals(docs []doc, all map[string]labels) (perDoc map[string]metrics, perLang map[string]metrics) {
	perDoc, perLang = map[string]metrics{}, map[string]metrics{}
	for _, d := range docs {
		m := score(d.Name, d.Keywords, all[d.Name])
		perDoc[d.Name] = m
		for _, key := range []string{d.Language, "all"} {
			t := perLang[key]
			t.add(m)
			perLang[key] = t
		}
	}
	return perDoc, perLang
}

func pct(p float64) string {
	if p < 0 {
		return "   -"
	}
	return fmt.Sprintf("%3.0f%%", 100*p)
}

func header(w io.Writer, first string) {
	_, _ = fmt.Fprintf(w, "%-34s", first)
	for _, k := range topK {
		_, _ = fmt.Fprintf(w, " %6s", fmt.Sprintf("P@%d", k))
	}
	for _, k := range topK {
		_, _ = fmt.Fprintf(w, " %6s", fmt.Sprintf("G@%d", k))
	}
	_, _ = fmt.Fprintf(w, " %5s %5s %5s %6s\n", "A", "I", "?", "missed")
}

func row(w io.Writer, name string, m metrics) {
	_, _ = fmt.Fprintf(w, "%-34s", name)
	for i := range topK {
		_, _ = fmt.Fprintf(w, " %6s", pct(m.precision(i)))
	}
	for i := range topK {
		_, _ = fmt.Fprintf(w, " %6.1f", m.goodPerDoc(i))
	}
	_, _ = fmt.Fprintf(w, " %5d %5d %5d %6d\n", m.Awkward, m.Irrelevant, m.Unjudged, m.Missed)
}

func report(w io.Writer, r run, all map[string]labels, verbose bool, holdout map[string]bool, tuningOnly bool) {
	tuning, held := split(r.Docs, holdout)
	perDoc, perLang := totals(tuning, all)
	header(w, "document")
	for _, d := range tuning {
		row(w, d.Name, perDoc[d.Name])
		if verbose {
			listDetails(w, d, all[d.Name])
		}
	}
	_, _ = fmt.Fprintln(w)
	languageTable(w, perLang)
	_, _ = fmt.Fprintln(w)
	bandTable(w, perLang)
	_, _ = fmt.Fprintln(w)
	defectReport(w, r.DefectChecks, tuning, all)
	if len(held) > 0 && !tuningOnly {
		_, heldLang := totals(held, all)
		_, _ = fmt.Fprintf(w, "\n== held-out documents (%d): per-document scores and keywords are not shown\n", len(held))
		languageTable(w, heldLang)
		_, _ = fmt.Fprintln(w)
		bandTable(w, heldLang)
		_, _ = fmt.Fprintln(w)
		defectLanguageTable(w, held, all)
	}
	if len(r.Short) > 0 {
		shortTuning, shortHeld := split(r.Short, holdout)
		_, _ = fmt.Fprintf(w, "\n== short texts: the first ~%d words of each page\n", shortTextWords)
		shortTable(w, shortTuning, all)
		if len(shortHeld) > 0 && !tuningOnly {
			_, _ = fmt.Fprintf(w, "\n== short texts of the held-out documents (%d)\n", len(shortHeld))
			shortTable(w, shortHeld, all)
		}
	}
	_, _ = fmt.Fprintf(w, "\nP@k = good / judged among the first k keywords; ? = unjudged, excluded from P@k. A, I and ? count the first %d.\n", labelDepth)
}

func shortTable(w io.Writer, docs []doc, all map[string]labels) {
	_, perLang := totals(docs, all)
	defects := defectTotals(docs, all)
	_, _ = fmt.Fprintf(w, "%-20s %9s %6s %6s %5s %5s %5s %7s\n", "language", "per text", "P@10", "P@all", "A", "I", "?", "defect")
	langs := slices.DeleteFunc(sortedKeys(perLang), func(l string) bool { return l == "all" })
	for _, l := range append(langs, "all") {
		m, stats := perLang[l], defects[l]
		returned, judged, good, flagged := 0, 0, 0, 0
		for band := range numBands {
			returned += m.BandReturned[band]
			judged += m.BandJudged[band]
			good += m.BandGood[band]
			flagged += stats.Any[band]
		}
		_, _ = fmt.Fprintf(w, "%-20s %9.1f %6s %6s %5d %5d %5d %7s\n", fmt.Sprintf("%s (%d docs)", l, m.Docs),
			float64(returned)/float64(m.Docs), pct(m.precision(0)), pct(ratio(good, judged)),
			m.Awkward, m.Irrelevant, m.Unjudged, rate(flagged, returned))
	}
}

func bandTable(w io.Writer, perLang map[string]metrics) {
	_, _ = fmt.Fprintf(w, "%-20s", "rank band")
	for band := range numBands {
		_, _ = fmt.Fprintf(w, " %14s", bandName(band))
	}
	_, _ = fmt.Fprintf(w, " %9s\n", "returned")
	langs := slices.DeleteFunc(sortedKeys(perLang), func(l string) bool { return l == "all" })
	for _, l := range append(langs, "all") {
		m := perLang[l]
		_, _ = fmt.Fprintf(w, "%-20s", fmt.Sprintf("%s (%d docs)", l, m.Docs))
		total := 0
		for band := range numBands {
			total += m.BandReturned[band]
			cell := "-"
			if m.BandReturned[band] > 0 {
				cell = fmt.Sprintf("%s (%s)", strings.TrimSpace(pct(ratio(m.BandGood[band], m.BandJudged[band]))),
					strings.TrimSpace(pct(ratio(m.BandJudged[band], m.BandSampled[band]))))
			}
			_, _ = fmt.Fprintf(w, " %14s", cell)
		}
		_, _ = fmt.Fprintf(w, " %9d\n", total)
	}
	_, _ = fmt.Fprintf(w, "Each band: precision among judged keywords (share judged). Past %d, only the 1-in-%d sample counts.\n", labelDepth, tailSampleEvery)
}

func ratio(part, whole int) float64 {
	if whole == 0 {
		return -1
	}
	return float64(part) / float64(whole)
}

func languageTable(w io.Writer, perLang map[string]metrics) {
	header(w, "language")
	for _, l := range sortedKeys(perLang) {
		if l != "all" {
			row(w, fmt.Sprintf("%s (%d docs)", l, perLang[l].Docs), perLang[l])
		}
	}
	row(w, fmt.Sprintf("all (%d docs)", perLang["all"].Docs), perLang["all"])
}

func split(docs []doc, holdout map[string]bool) (tuning, held []doc) {
	for _, d := range docs {
		if holdout[d.Name] {
			held = append(held, d)
		} else {
			tuning = append(tuning, d)
		}
	}
	return tuning, held
}

func listDetails(w io.Writer, d doc, l labels) {
	returned := map[string]bool{}
	for i, k := range d.Keywords {
		key := matchKey(k)
		returned[key] = true
		if lb, ok := l[key]; wantsLabel(d.Name, i, key) && (!ok || lb.mark == markUnjudged) {
			_, _ = fmt.Fprintf(w, "    ? %s\n", k)
		}
	}
	var missed []string
	for key, lb := range l {
		if lb.mark == markGood && !returned[key] {
			missed = append(missed, lb.text)
		}
	}
	slices.Sort(missed)
	for _, m := range missed {
		_, _ = fmt.Fprintf(w, "    missed + %s\n", m)
	}
}

func compare(w io.Writer, base, cur run, all map[string]labels, holdout map[string]bool, tuningOnly bool) {
	if cut := commonLimit(base.Limit, cur.Limit); cut > 0 {
		base, cur = truncateRun(base, cut), truncateRun(cur, cut)
		_, _ = fmt.Fprintf(w, "== both runs cut to the first %d keywords\n", cut)
	}
	baseDocs := map[string]doc{}
	for _, d := range base.Docs {
		baseDocs[d.Name] = d
	}
	curNames := map[string]bool{}
	var sharedBase, sharedCur, heldBase, heldCur, added []doc
	heldApart := 0
	for _, d := range cur.Docs {
		curNames[d.Name] = true
		b, ok := baseDocs[d.Name]
		switch {
		case ok && holdout[d.Name]:
			heldBase, heldCur = append(heldBase, b), append(heldCur, d)
		case ok:
			sharedBase, sharedCur = append(sharedBase, b), append(sharedCur, d)
		case holdout[d.Name]:
			heldApart++
		default:
			added = append(added, d)
		}
	}
	var removed []doc
	for _, d := range base.Docs {
		if !curNames[d.Name] && !holdout[d.Name] {
			removed = append(removed, d)
		}
	}

	_, _ = fmt.Fprintf(w, "== baseline %s vs now, %d shared documents, both scored with current labels\n", base.Time, len(sharedCur))
	deltaTable(w, sharedBase, sharedCur, all)
	sharedChecks := slices.DeleteFunc(slices.Clone(cur.DefectChecks), func(check string) bool {
		return !slices.Contains(base.DefectChecks, check)
	})
	defectDeltaTable(w, sharedChecks, sharedBase, sharedCur, all)
	if !tuningOnly && len(heldCur) > 0 {
		_, _ = fmt.Fprintf(w, "\n== held-out: %d shared documents, keyword changes not shown\n", len(heldCur))
		deltaTable(w, heldBase, heldCur, all)
		defectDeltaTable(w, sharedChecks, heldBase, heldCur, all)
	}
	if !tuningOnly && heldApart > 0 {
		_, _ = fmt.Fprintf(w, "\n%d held-out documents are not in the baseline.\n", heldApart)
	}
	compareShort(w, base.Short, cur.Short, all, holdout, tuningOnly)
	for i, d := range sharedCur {
		gone, fresh := diffKeywords(sharedBase[i].Keywords, d.Keywords)
		if len(gone) == 0 && len(fresh) == 0 {
			continue
		}
		_, _ = fmt.Fprintf(w, "\n%s\n", d.Name)
		for _, k := range gone {
			_, _ = fmt.Fprintf(w, "  - %c %s\n", markOf(all[d.Name], k), k)
		}
		for _, k := range fresh {
			_, _ = fmt.Fprintf(w, "  + %c %s\n", markOf(all[d.Name], k), k)
		}
	}
	for _, part := range []struct {
		title string
		docs  []doc
	}{{"new documents (not in baseline)", added}, {"removed documents (only in baseline)", removed}} {
		if len(part.docs) == 0 {
			continue
		}
		_, _ = fmt.Fprintf(w, "\n== %s\n", part.title)
		header(w, "document")
		perDoc, _ := totals(part.docs, all)
		for _, d := range part.docs {
			row(w, d.Name, perDoc[d.Name])
		}
	}
}

func deltaTable(w io.Writer, base, cur []doc, all map[string]labels) {
	_, bl := totals(base, all)
	_, cl := totals(cur, all)
	_, _ = fmt.Fprintf(w, "%-10s", "language")
	for _, k := range topK {
		_, _ = fmt.Fprintf(w, " %-20s", fmt.Sprintf("P@%d", k))
	}
	for _, k := range topK {
		_, _ = fmt.Fprintf(w, " %-18s", fmt.Sprintf("G@%d", k))
	}
	_, _ = fmt.Fprintln(w)
	langs := slices.DeleteFunc(sortedKeys(cl), func(l string) bool { return l == "all" })
	for _, l := range append(langs, "all") {
		before, after := bl[l], cl[l]
		_, _ = fmt.Fprintf(w, "%-10s", l)
		for i := range topK {
			_, _ = fmt.Fprintf(w, " %-20s", delta(before.precision(i), after.precision(i)))
		}
		for i := range topK {
			b, c := before.goodPerDoc(i), after.goodPerDoc(i)
			_, _ = fmt.Fprintf(w, " %-18s", fmt.Sprintf("%.1f -> %.1f (%+.1f)", b, c, c-b))
		}
		_, _ = fmt.Fprintln(w)
	}
}

func defectDeltaTable(w io.Writer, checks []string, base, cur []doc, all map[string]labels) {
	if len(checks) == 0 {
		return
	}
	before, after := defectTotals(base, all)["all"], defectTotals(cur, all)["all"]
	flaggedShare := func(stats defectStats, check int) float64 {
		flagged, returned := 0, 0
		for band := range numBands {
			flagged += stats.Flagged[check][band]
			returned += stats.Returned[band]
		}
		return ratio(flagged, returned)
	}
	_, _ = fmt.Fprintf(w, "%-20s %s\n", "defect check", "share of all returned keywords flagged")
	for check, name := range defectChecks {
		if slices.Contains(checks, name) {
			_, _ = fmt.Fprintf(w, "%-20s %s\n", name, delta(flaggedShare(before, check), flaggedShare(after, check)))
		}
	}
}

func delta(b, c float64) string {
	if b < 0 || c < 0 {
		return fmt.Sprintf("%s -> %s", strings.TrimSpace(pct(b)), strings.TrimSpace(pct(c)))
	}
	return fmt.Sprintf("%s -> %s (%+.0f)", strings.TrimSpace(pct(b)), strings.TrimSpace(pct(c)), 100*(c-b))
}

func commonLimit(a, b int) int {
	switch {
	case a == 0:
		return b
	case b == 0:
		return a
	default:
		return min(a, b)
	}
}

func compareShort(w io.Writer, base, cur []doc, all map[string]labels, holdout map[string]bool, tuningOnly bool) {
	if len(base) == 0 || len(cur) == 0 {
		return
	}
	baseDocs := map[string]doc{}
	for _, d := range base {
		baseDocs[d.Name] = d
	}
	var tuningBase, tuningCur, heldBase, heldCur []doc
	for _, d := range cur {
		b, ok := baseDocs[d.Name]
		switch {
		case !ok:
		case holdout[d.Name]:
			heldBase, heldCur = append(heldBase, b), append(heldCur, d)
		default:
			tuningBase, tuningCur = append(tuningBase, b), append(tuningCur, d)
		}
	}
	_, _ = fmt.Fprintf(w, "\n== short texts: %d shared documents\n", len(tuningCur))
	deltaTable(w, tuningBase, tuningCur, all)
	if !tuningOnly && len(heldCur) > 0 {
		_, _ = fmt.Fprintf(w, "\n== short texts of the held-out documents: %d shared\n", len(heldCur))
		deltaTable(w, heldBase, heldCur, all)
	}
}

func truncateRun(r run, limit int) run {
	truncate := func(in []doc) []doc {
		out := make([]doc, len(in))
		for i, d := range in {
			d.Keywords = d.Keywords[:min(len(d.Keywords), limit)]
			out[i] = d
		}
		return out
	}
	r.Docs, r.Short = truncate(r.Docs), truncate(r.Short)
	return r
}

func markOf(l labels, k string) byte {
	if lb, ok := l[matchKey(k)]; ok {
		return lb.mark
	}
	return markUnjudged
}

func diffKeywords(before, after []string) (gone, fresh []string) {
	in := func(list []string) map[string]bool {
		m := map[string]bool{}
		for _, k := range list {
			m[matchKey(k)] = true
		}
		return m
	}
	b, a := in(before), in(after)
	for _, k := range before {
		if !a[matchKey(k)] {
			gone = append(gone, k)
		}
	}
	for _, k := range after {
		if !b[matchKey(k)] {
			fresh = append(fresh, k)
		}
	}
	return gone, fresh
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

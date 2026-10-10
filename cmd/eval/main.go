// Command eval scores extractor output against hand-written labels.
//
// It reads every <dir>/<lang>/<name>.txt, extracts keywords in-process, and
// looks each keyword up in <dir>/<lang>/<name>.labels.tsv. A labels file has
// one "<mark>\t<anchor text>" line per anchor, where the mark is
//
//   - good anchor
//     A  awkward: a fragment, truncated, too generic, or grammatically odd
//     I  irrelevant to the topic
//     ?  not judged yet
//
// Lines starting with # are comments. Keywords and labels match after
// lowercasing, apostrophe folding and whitespace collapsing.
//
// -draft appends every returned but unlabelled keyword as "?". Each run is
// saved under <dir>/.runs/, and -baseline compares the current run with a
// saved one on the documents both runs contain, scoring both with the
// current labels.
//
// Pages listed in <dir>/holdout.txt are held out: they are scored apart,
// per language only, and their keywords and keyword changes are never
// printed, so that rules can't be tuned on them.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/tericcabrel/keyphrase/extractor"
	"github.com/tericcabrel/keyphrase/internal/evalset"
	"github.com/tericcabrel/keyphrase/lang"

	_ "github.com/tericcabrel/keyphrase/lang/all"
)

const (
	markGood       = '+'
	markAwkward    = 'A'
	markIrrelevant = 'I'
	markUnjudged   = '?'
)

// topK are the cut-offs precision is reported at.
var topK = []int{10, 50}

// label is one judged anchor.
type label struct {
	mark byte
	text string
}

// labels maps matchKey(anchor text) to its label.
type labels map[string]label

// doc is one evaluated text. Keywords are in output order, best first.
type doc struct {
	Name     string   `json:"name"` // "<lang>/<file name without .txt>"
	Language string   `json:"language"`
	Keywords []string `json:"keywords"`
}

// run is what -baseline reads back.
type run struct {
	Time  string `json:"time"`
	Limit int    `json:"limit"`
	Docs  []doc  `json:"docs"`
}

// metrics are counts, so they add up across documents.
type metrics struct {
	Docs                          int
	Good, Judged                  [2]int // per topK entry
	Awkward, Irrelevant, Unjudged int
	Missed                        int // labelled good, not returned
}

func (m *metrics) add(o metrics) {
	m.Docs += o.Docs
	for i := range topK {
		m.Good[i] += o.Good[i]
		m.Judged[i] += o.Judged[i]
	}
	m.Awkward += o.Awkward
	m.Irrelevant += o.Irrelevant
	m.Unjudged += o.Unjudged
	m.Missed += o.Missed
}

// precision is good / judged among the first topK[i] keywords, or -1 when
// nothing there is judged.
func (m *metrics) precision(i int) float64 {
	if m.Judged[i] == 0 {
		return -1
	}
	return float64(m.Good[i]) / float64(m.Judged[i])
}

func main() {
	dir := flag.String("dir", "data-text", "directory of <lang>/<name>.txt files")
	limit := flag.Int("limit", 50, "keywords per document")
	draft := flag.Bool("draft", false, "append unlabelled keywords to the labels files as ?")
	baseline := flag.String("baseline", "", "saved run to compare against")
	save := flag.Bool("save", true, "save this run under <dir>/.runs/")
	verbose := flag.Bool("v", false, "list unjudged and missed anchors per document")
	rarityWeight := flag.Float64("rarity", -1, "override the rarity weight; negative keeps the built-in weight")
	rarityPhrase := flag.String("rarity-phrase", "", "mean or max over a phrase's content words; empty keeps the built-in rule")
	singleMin := flag.Float64("single-rarity-min", -1, "drop one-word keywords below this rarity; negative keeps the built-in rule")
	dropLoneNouns := flag.Bool("drop-lone-nouns", false, "drop a one-word keyword unless the tagger calls it a name or it is capitalized like one")
	loneMin := flag.Float64("lone-noun-min", -1, "with -drop-lone-nouns, keep a one-word common noun whose rarity is at least this; negative drops every lone common noun")
	tuningOnly := flag.Bool("tuning-only", false, "do not print held-out scores")
	flag.Parse()

	w, err := evalWeights(*rarityWeight, *rarityPhrase, *singleMin, *dropLoneNouns, *loneMin)
	if err != nil {
		log.Fatal(err)
	}
	cur, all, err := evaluate(*dir, *limit, w, os.Stderr)
	if err != nil {
		log.Fatal(err)
	}
	holdout, err := evalset.ReadHoldout(*dir)
	if err != nil {
		log.Fatal(err)
	}
	if *draft {
		for _, d := range cur.Docs {
			if holdout[d.Name] {
				continue
			}
			path := labelsPath(*dir, d.Name)
			n, draftErr := appendDraft(path, d, all[d.Name])
			if draftErr != nil {
				log.Fatal(draftErr)
			}
			if n > 0 {
				fmt.Printf("%s: appended %d unjudged anchors\n", path, n)
			}
		}
		if all, err = loadAllLabels(*dir, cur.Docs); err != nil {
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

// evalWeights copies the built-in weights and applies command-line overrides.
func evalWeights(rarityWeight float64, phrase string, singleMin float64, dropLoneNouns bool, loneMin float64) (extractor.Weights, error) {
	w := extractor.DefaultWeights
	w.DropLoneNouns = dropLoneNouns
	if loneMin >= 0 {
		w.DropLoneNouns = true
		w.LoneNounMinRarity = loneMin
	}
	if rarityWeight >= 0 {
		w.Rarity = rarityWeight
	}
	switch phrase {
	case "":
	case "mean":
		w.PhraseRarityMax = false
	case "max":
		w.PhraseRarityMax = true
	default:
		return w, fmt.Errorf("-rarity-phrase must be mean or max, not %q", phrase)
	}
	if singleMin >= 0 {
		w.MinSingleRarity = singleMin
	}
	return w, nil
}

// evaluate extracts keywords for every text under dir and loads its labels.
func evaluate(dir string, limit int, w extractor.Weights, warn io.Writer) (run, map[string]labels, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*", "*.txt"))
	if err != nil {
		return run{}, nil, err
	}
	slices.Sort(files)
	cur := run{Time: time.Now().UTC().Format(time.RFC3339), Limit: limit}
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
		kws, extractErr := extractor.ExtractWithWeights(text, language, limit, w)
		if extractErr != nil {
			return run{}, nil, fmt.Errorf("%s: %w", f, extractErr)
		}
		d := doc{Name: language + "/" + strings.TrimSuffix(filepath.Base(f), ".txt"), Language: language}
		for _, k := range kws {
			d.Keywords = append(d.Keywords, k.Text)
		}
		cur.Docs = append(cur.Docs, d)
	}
	if len(cur.Docs) == 0 {
		return run{}, nil, fmt.Errorf("no <lang>/*.txt files under %s", dir)
	}
	all, err := loadAllLabels(dir, cur.Docs)
	return cur, all, err
}

// decodeText undoes the JSON string escaping the texts are stored with. When
// the file is not a valid JSON string body (an unescaped quote, say), only
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

// loadLabels reads a labels file; a missing file means no labels.
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

// matchKey is the form keywords and labels are compared in.
func matchKey(s string) string {
	return strings.Join(strings.Fields(lang.Normalize(s)), " ")
}

// score counts one document's keywords against its labels.
func score(keywords []string, l labels) metrics {
	m := metrics{Docs: 1}
	returned := map[string]bool{}
	for i, k := range keywords {
		key := matchKey(k)
		returned[key] = true
		lb, ok := l[key]
		if !ok {
			lb.mark = markUnjudged
		}
		switch lb.mark {
		case markAwkward:
			m.Awkward++
		case markIrrelevant:
			m.Irrelevant++
		case markUnjudged:
			m.Unjudged++
		}
		if lb.mark == markUnjudged {
			continue
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

// appendDraft adds every keyword of d that has no label to the labels file
// as "?", creating the file with a header if needed. Existing lines are never
// rewritten. It returns the number of lines added.
func appendDraft(path string, d doc, l labels) (int, error) {
	var lines []string
	seen := map[string]bool{}
	for _, k := range d.Keywords {
		key := matchKey(k)
		if _, ok := l[key]; ok || seen[key] {
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

// totals sums metrics per language and overall ("all") for the given docs.
func totals(docs []doc, all map[string]labels) (perDoc map[string]metrics, perLang map[string]metrics) {
	perDoc, perLang = map[string]metrics{}, map[string]metrics{}
	for _, d := range docs {
		m := score(d.Keywords, all[d.Name])
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
	_, _ = fmt.Fprintf(w, "%-34s %6s %6s %5s %5s %5s %6s\n", first, "P@10", "P@50", "A", "I", "?", "missed")
}

func row(w io.Writer, name string, m metrics) {
	_, _ = fmt.Fprintf(w, "%-34s %6s %6s %5d %5d %5d %6d\n", name, pct(m.precision(0)), pct(m.precision(1)), m.Awkward, m.Irrelevant, m.Unjudged, m.Missed)
}

// report prints per-document and per-language metrics for the tuning
// documents, then only per-language metrics for the held-out ones.
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
	if len(held) > 0 && !tuningOnly {
		_, heldLang := totals(held, all)
		_, _ = fmt.Fprintf(w, "\n== held-out documents (%d): per-document scores and keywords are not shown\n", len(held))
		languageTable(w, heldLang)
	}
	_, _ = fmt.Fprintln(w, "\nP@k = good / judged among the first k keywords; ? = unjudged, excluded from P@k.")
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

// split separates the tuning documents from the held-out ones.
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
	for _, k := range d.Keywords {
		key := matchKey(k)
		returned[key] = true
		if lb, ok := l[key]; !ok || lb.mark == markUnjudged {
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

// compare prints base versus cur on the documents both contain, scoring both
// with the current labels, then lists documents present in only one run.
// Held-out documents only count towards their own per-language deltas; their
// keyword changes and per-document scores are not shown.
func compare(w io.Writer, base, cur run, all map[string]labels, holdout map[string]bool, tuningOnly bool) {
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
	if !tuningOnly && len(heldCur) > 0 {
		_, _ = fmt.Fprintf(w, "\n== held-out: %d shared documents, keyword changes not shown\n", len(heldCur))
		deltaTable(w, heldBase, heldCur, all)
	}
	if !tuningOnly && heldApart > 0 {
		_, _ = fmt.Fprintf(w, "\n%d held-out documents are not in the baseline.\n", heldApart)
	}
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
	_, _ = fmt.Fprintf(w, "%-10s %-20s %-20s\n", "language", "P@10", "P@50")
	langs := slices.DeleteFunc(sortedKeys(cl), func(l string) bool { return l == "all" })
	for _, l := range append(langs, "all") {
		_, _ = fmt.Fprintf(w, "%-10s %-20s %-20s\n", l, delta(bl[l], cl[l], 0), delta(bl[l], cl[l], 1))
	}
}

func delta(base, cur metrics, i int) string {
	b, c := base.precision(i), cur.precision(i)
	if b < 0 || c < 0 {
		return fmt.Sprintf("%s -> %s", strings.TrimSpace(pct(b)), strings.TrimSpace(pct(c)))
	}
	return fmt.Sprintf("%s -> %s (%+.0f)", strings.TrimSpace(pct(b)), strings.TrimSpace(pct(c)), 100*(c-b))
}

func markOf(l labels, k string) byte {
	if lb, ok := l[matchKey(k)]; ok {
		return lb.mark
	}
	return markUnjudged
}

// diffKeywords returns keywords only in before and only in after.
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

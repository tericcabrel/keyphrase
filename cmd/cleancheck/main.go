// Command cleancheck checks extracted page text against the archived snapshots in data-text.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/net/html"

	"github.com/tericcabrel/keyphrase/internal/webtext"
	"github.com/tericcabrel/keyphrase/lang"

	_ "github.com/tericcabrel/keyphrase/lang/all"
)

const (
	minLostWords       = 15
	maxChromeLineWords = 9
	maxShortWords      = 3
	minPageWords       = 300
)

func main() {
	dir := flag.String("dir", "data-text", "folder with one sub-folder per language")
	archive := flag.String("archive", filepath.Join("data-text", ".archive", "2026-09-29"), "folder with the old texts and labels")
	page := flag.String("page", "", "only check pages whose name contains this")
	showText := flag.Bool("text", false, "print the full extracted text")
	where := flag.String("where", "", "print the elements holding this phrase instead of checking")
	write := flag.Bool("write", false, "write <name>.txt next to each page that passes, for eval")
	add := flag.String("add", "", `fetch the "<lang> <url> [holdout]" lines of this file, then check and -write only those pages`)
	anyCMS := flag.Bool("any", false, "with -add, also accept pages that are not Ghost posts")
	flag.Parse()

	var only map[string]bool
	if *add != "" {
		entries, err := readURLList(*add)
		if err != nil {
			fatal(err)
		}
		only = map[string]bool{}
		added, saved := addPages(*dir, entries, *anyCMS)
		for _, name := range added {
			only[name] = true
		}
		markHoldout(*dir, entries, saved)
		if len(only) == 0 {
			fmt.Println("no new pages")
			return
		}
		*write = true
		fmt.Println()
	}

	pages, err := filepath.Glob(filepath.Join(*dir, "*", "*.html"))
	if err != nil {
		fatal(err)
	}
	sort.Strings(pages)
	passed, total := 0, 0
	for _, path := range pages {
		name := strings.TrimSuffix(filepath.ToSlash(must(filepath.Rel(*dir, path))), ".html")
		if !strings.Contains(name, *page) || (only != nil && !only[name]) {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			fatal(err)
		}
		if *where != "" {
			printChain(name, raw, *where)

			continue
		}
		old, err := readOld(filepath.Join(*archive, filepath.FromSlash(name)+".txt"))
		if err != nil {
			fatal(err)
		}
		phrases, err := readIrrelevant(filepath.Join(*archive, filepath.FromSlash(name)+".labels.tsv"))
		if err != nil {
			fatal(err)
		}
		accept, err := readLines(filepath.Join(*dir, filepath.FromSlash(name)+".accept"))
		if err != nil {
			fatal(err)
		}
		r, err := check(raw, old, phrases)
		if err != nil {
			fatal(fmt.Errorf("%s: %w", name, err))
		}
		r.Name = name
		r.accept(accept)
		total++
		if r.pass() {
			passed++
			if *write {
				if err := writeText(filepath.Join(*dir, filepath.FromSlash(name)+".txt"), strings.Join(r.Lines, "\n")); err != nil {
					fatal(err)
				}
			}
		}
		r.print(os.Stdout, *showText)
	}
	if *where == "" {
		fmt.Printf("\n%d of %d pages pass\n", passed, total)
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		fatal(err)
	}
	return v
}

func fatal(err error) {
	_, _ = fmt.Fprintln(os.Stderr, "cleancheck:", err)
	os.Exit(1)
}

type report struct {
	Name      string
	Rule      webtext.Rule
	Container string
	Lines     []string
	OldBytes  int
	NewBytes  int
	TitleOK   bool
	Titles    []string
	Chrome    []string
	Short     []string
	Lost      []string
	Pruned    []string
	Outside   []string
	Gone      []string // not in the snapshot: the page changed since the old text
	Accepted  []string // lines listed in <name>.accept
	Words     int
	Size      string
}

func (r *report) pass() bool {
	return r.TitleOK && r.Size == "" && len(r.Chrome) == 0 && len(r.Short) == 0 && len(r.Lost) == 0
}

func (r *report) accept(lines []string) {
	ok := map[string]bool{}
	for _, l := range lines {
		ok[key(l)] = true
	}
	keep := func(findings []string, line func(string) string) []string {
		var out []string
		for _, f := range findings {
			if ok[key(line(f))] {
				r.Accepted = append(r.Accepted, f)

				continue
			}
			out = append(out, f)
		}
		return out
	}
	r.Chrome = keep(r.Chrome, chromeLine)
	r.Short = keep(r.Short, func(s string) string { return s })
}

func chromeLine(finding string) string {
	_, rest, _ := strings.Cut(finding, ` on line "`)
	return strings.TrimSuffix(rest, `"`)
}

func (r *report) print(w *os.File, showText bool) {
	status := "PASS"
	if !r.pass() {
		status = "FAIL"
	}
	size := "new page"
	if r.OldBytes > 0 {
		size = fmt.Sprintf("old %d, %d%%", r.OldBytes, 100*r.NewBytes/r.OldBytes)
	}
	_, _ = fmt.Fprintf(w, "\n== %s  %s  rule=%s (%s)  %d words, %d bytes (%s)\n", r.Name, status, r.Rule, r.Container, r.Words, r.NewBytes, size)
	if !r.TitleOK {
		_, _ = fmt.Fprintf(w, "   title: first line is not one of %q\n", r.Titles)
	}
	if r.Size != "" {
		_, _ = fmt.Fprintf(w, "   size:   %s\n", r.Size)
	}
	for i, line := range r.Lines {
		if showText || i < 3 || i >= len(r.Lines)-3 {
			_, _ = fmt.Fprintf(w, "   %3d| %s\n", i+1, clipLine(line))
		} else if i == 3 {
			_, _ = fmt.Fprintln(w, "   ...")
		}
	}
	for _, c := range r.Chrome {
		_, _ = fmt.Fprintf(w, "   chrome: %s\n", c)
	}
	for _, s := range r.Short {
		_, _ = fmt.Fprintf(w, "   short:  %s\n", s)
	}
	for _, l := range r.Lost {
		_, _ = fmt.Fprintf(w, "   lost:   %s\n", clipLine(l))
	}
	for _, l := range r.Pruned {
		_, _ = fmt.Fprintf(w, "   pruned: %s\n", clipLine(l))
	}
	for _, l := range r.Outside {
		_, _ = fmt.Fprintf(w, "   outside: %s\n", clipLine(l))
	}
	for _, l := range r.Gone {
		_, _ = fmt.Fprintf(w, "   not in snapshot: %s\n", clipLine(l))
	}
	for _, l := range r.Accepted {
		_, _ = fmt.Fprintf(w, "   accepted: %s\n", clipLine(l))
	}
}

func clipLine(s string) string {
	const max = 110
	if r := []rune(s); len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}

func check(page []byte, old string, irrelevant []string) (report, error) {
	c, err := webtext.Extract(bytes.NewReader(page))
	if err != nil {
		return report{}, err
	}
	facts, err := pageFacts(page)
	if err != nil {
		return report{}, err
	}
	lines := strings.Split(c.Text, "\n")
	r := report{
		Rule:      c.Rule,
		Container: c.Container,
		Lines:     lines,
		OldBytes:  len(old),
		NewBytes:  len(c.Text),
		Titles:    facts.titles,
		Chrome:    leftoverChrome(lines, irrelevant, facts.headings),
		Short:     shortLines(lines[1:], facts.allowed),
		Words:     len(strings.Fields(c.Text)),
	}
	switch {
	case r.Words < minPageWords:
		r.Size = fmt.Sprintf("only %d words, want at least %d", r.Words, minPageWords)
	case r.NewBytes > webtext.MaxTextBytes:
		r.Size = fmt.Sprintf("%d bytes, over the %d-byte limit of POST /v1/keywords", r.NewBytes, webtext.MaxTextBytes)
	}
	for _, s := range missing(old, c.Text, irrelevant) {
		where := locate(facts.doc, key(s), c.Container)
		switch {
		case !where.found:
			r.Gone = append(r.Gone, s)
		case where.pruned != "":
			r.Pruned = append(r.Pruned, fmt.Sprintf("[%s] %s", where.pruned, s))
		case where.inside:
			r.Lost = append(r.Lost, s)
		default:
			r.Outside = append(r.Outside, s)
		}
	}
	for _, t := range facts.titles {
		if key(lines[0]) == key(t) {
			r.TitleOK = true
		}
	}
	return r, nil
}

type facts struct {
	doc      *html.Node
	titles   []string // og:title and every h1
	headings map[string]bool
	allowed  map[string]bool // heading, list, table and bold text, which may be short
}

func pageFacts(page []byte) (facts, error) {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return facts{}, err
	}
	f := facts{doc: doc, headings: map[string]bool{}, allowed: map[string]bool{}}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			k := key(flatText(n))
			switch n.Data {
			case "meta":
				if attr(n, "property") == "og:title" {
					f.titles = append(f.titles, attr(n, "content"))
				}
			case "h1", "h2", "h3", "h4", "h5", "h6":
				if n.Data == "h1" {
					f.titles = append(f.titles, flatText(n))
				}
				f.headings[k] = true
				f.allowed[k] = true
			case "li", "td", "th", "dt", "dd", "strong", "b", "em", "figcaption":
				f.allowed[k] = true
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return f, nil
}

func printChain(name string, page []byte, phrase string) {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		fatal(err)
	}
	holder := deepest(doc, key(phrase))
	if holder == doc {
		return
	}
	var chain []string
	for n := holder; n != nil; n = n.Parent {
		if n.Type == html.ElementNode {
			chain = append([]string{webtext.Describe(n)}, chain...)
		}
	}
	fmt.Printf("%s: %s\n", name, strings.Join(chain, " > "))
}

func deepest(doc *html.Node, sk string) *html.Node {
	var holder *html.Node
	for n := doc; n != nil; {
		holder = n
		var next *html.Node
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && strings.Contains(key(flatText(c)), sk) {
				next = c

				break
			}
		}
		n = next
	}
	return holder
}

type place struct {
	found  bool
	inside bool
	pruned string // the boilerplate element that holds it, when one does
}

func locate(doc *html.Node, sk, container string) place {
	holder := deepest(doc, sk)
	if holder == doc {
		return place{}
	}
	p := place{found: true}
	for n := holder; n != nil; n = n.Parent {
		if n.Type != html.ElementNode {
			continue
		}
		if p.pruned == "" && webtext.Boilerplate(n) {
			p.pruned = webtext.Describe(n)
		}
		if webtext.Describe(n) == container {
			p.inside = true
		}
	}
	return p
}

// A space at each block boundary, so the text lines up with what the extractor joins.
func flatText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			b.WriteString(n.Data)
		case html.ElementNode:
			switch n.Data {
			case "script", "style", "noscript", "template", "svg", "iframe", "pre":
				return
			case "a", "b", "strong", "em", "i", "span", "code", "small", "sup", "sub", "mark", "u", "s", "abbr", "time", "cite", "q":
			default:
				b.WriteByte(' ')
				defer b.WriteByte(' ')
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func attr(n *html.Node, k string) string {
	for _, a := range n.Attr {
		if a.Key == k {
			return a.Val
		}
	}
	return ""
}

func key(s string) string {
	return strings.Join(strings.Fields(lang.Normalize(s)), " ")
}

func containsWords(s, phrase string) bool {
	for i := 0; ; {
		j := strings.Index(s[i:], phrase)
		if j < 0 {
			return false
		}
		start, end := i+j, i+j+len(phrase)
		if (start == 0 || !isWordByte(s[start-1])) && (end == len(s) || !isWordByte(s[end])) {
			return true
		}
		i = start + 1
	}
}

// A non-ASCII byte is part of a word: the letters of these languages are.
func isWordByte(b byte) bool {
	return b >= 0x80 || b == '\'' || b == '-' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z')
}

func leftoverChrome(lines, irrelevant []string, headings map[string]bool) []string {
	var out []string
	for _, p := range irrelevant {
		pk := key(p)
		for _, line := range lines {
			if headings[key(line)] {
				continue
			}
			n := len(strings.Fields(line))
			if (n <= maxShortWords || (n <= maxChromeLineWords && !endsSentence(line))) && containsWords(key(line), pk) {
				out = append(out, fmt.Sprintf("%q on line %q", p, line))

				break
			}
		}
	}
	return out
}

func shortLines(lines []string, allowed map[string]bool) []string {
	var out []string
	for _, line := range lines {
		n := len(strings.Fields(line))
		if n < 1 || n > maxShortWords || !menuLike(line) || endsSentence(line) || allowed[key(line)] {
			continue
		}
		out = append(out, line)
	}
	return out
}

func menuLike(s string) bool {
	letters := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			letters++
		}
	}
	return unicode.IsLetter([]rune(s)[0]) && letters >= 2 && strings.IndexFunc(s, unicode.IsDigit) < 0
}

func endsSentence(line string) bool {
	line = strings.TrimRight(line, ` "'»”’)`)
	return strings.HasSuffix(line, ".") || strings.HasSuffix(line, ":") || strings.HasSuffix(line, "?") ||
		strings.HasSuffix(line, "!") || strings.HasSuffix(line, "…")
}

var sentenceEnd = regexp.MustCompile(`[.!?…]["»”)]*\s+`)

func missing(old, new string, irrelevant []string) []string {
	have := key(new)
	keys := make([]string, 0, len(irrelevant))
	for _, p := range irrelevant {
		keys = append(keys, key(p))
	}
	var out []string
	seen := map[string]bool{}
	for line := range strings.SplitSeq(old, "\n") {
		for _, s := range splitSentences(line) {
			sk := key(s)
			if len(strings.Fields(s)) < minLostWords || seen[sk] || strings.Contains(have, sk) || containsAny(sk, keys) {
				continue
			}
			seen[sk] = true
			out = append(out, s)
		}
	}
	return out
}

func splitSentences(line string) []string {
	var out []string
	prev := 0
	for _, loc := range sentenceEnd.FindAllStringIndex(line, -1) {
		out = append(out, strings.TrimSpace(line[prev:loc[1]]))
		prev = loc[1]
	}
	if rest := strings.TrimSpace(line[prev:]); rest != "" {
		out = append(out, rest)
	}
	return out
}

func containsAny(s string, phrases []string) bool {
	for _, p := range phrases {
		if containsWords(s, p) {
			return true
		}
	}
	return false
}

func readOld(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return decodeText(raw), nil
}

// A JSON string body without the quotes, or plain text with \n escapes when that fails.
func decodeText(raw []byte) string {
	body := strings.TrimRight(string(raw), "\r\n")
	var text string
	if err := json.Unmarshal([]byte(`"`+body+`"`), &text); err == nil {
		return text
	}
	return strings.NewReplacer(`\n`, "\n", `\"`, `"`, `\t`, "\t").Replace(body)
}

// The data-text format is a JSON string without its quotes, on one line.
func writeText(path, text string) error {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(text); err != nil {
		return err
	}
	line := strings.TrimSuffix(b.String(), "\n")
	return os.WriteFile(path, []byte(line[1:len(line)-1]+"\n"), 0o644)
}

func readLines(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for l := range strings.SplitSeq(string(raw), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			out = append(out, l)
		}
	}
	return out, nil
}

func readIrrelevant(path string) ([]string, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		mark, phrase, ok := strings.Cut(sc.Text(), "\t")
		if ok && mark == "I" && strings.TrimSpace(phrase) != "" {
			out = append(out, strings.TrimSpace(phrase))
		}
	}
	return out, sc.Err()
}

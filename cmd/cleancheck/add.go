package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tericcabrel/keyphrase/internal/eval"
	"github.com/tericcabrel/keyphrase/internal/webtext"
	"github.com/tericcabrel/keyphrase/lang"
)

var ghostGenerator = regexp.MustCompile(`(?i)<meta\s+name="generator"\s+content="Ghost\b`)

type entry struct {
	lang, url string
	holdout   bool
}

func readURLList(path string) ([]entry, error) {
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}
	if lines == nil {
		if _, err := os.Stat(path); err != nil {
			return nil, err
		}
	}
	var out []entry
	for i, l := range lines {
		fields := strings.Fields(l)
		if len(fields) < 2 || len(fields) > 3 || (len(fields) == 3 && fields[2] != "holdout") {
			return nil, fmt.Errorf("%s: line %d: want \"<lang> <url> [holdout]\", got %q", path, i+1, l)
		}
		if _, ok := lang.Get(fields[0]); !ok {
			return nil, fmt.Errorf("%s: line %d: unsupported language %q", path, i+1, fields[0])
		}
		out = append(out, entry{lang: fields[0], url: fields[1], holdout: len(fields) == 3})
	}
	return out, nil
}

func addPages(dir string, entries []entry, anyCMS bool) (added []string, saved map[string]string) {
	saved = savedURLs(dir)
	for _, e := range entries {
		if name, ok := saved[e.url]; ok {
			fmt.Printf("skip %s: already saved as %s\n", e.url, name)

			continue
		}
		p, err := webtext.Fetch(context.Background(), e.url)
		if err != nil {
			fmt.Printf("skip %s: %v\n", e.url, err)

			continue
		}
		if !p.HTML {
			fmt.Printf("skip %s: not an HTML page\n", e.url)

			continue
		}
		if !anyCMS && !ghostGenerator.MatchString(p.Body) {
			fmt.Printf("skip %s: not a Ghost site (no Ghost generator tag; use -any to add it anyway)\n", e.url)

			continue
		}
		name := e.lang + "/" + freeName(dir, e.lang, siteName(e.url))
		base := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(base), 0o755); err != nil {
			fatal(err)
		}
		if err := os.WriteFile(base+".html", []byte(p.Body), 0o644); err != nil {
			fatal(err)
		}
		if err := os.WriteFile(base+".url", []byte(e.url+"\n"), 0o644); err != nil {
			fatal(err)
		}
		saved[e.url] = name
		added = append(added, name)
		fmt.Printf("saved %s as %s\n", e.url, name)
	}
	return added, saved
}

func markHoldout(dir string, entries []entry, saved map[string]string) {
	var names []string
	for _, e := range entries {
		if name, ok := saved[e.url]; ok && e.holdout {
			names = append(names, name)
		}
	}
	added, err := eval.AddHoldout(dir, names)
	if err != nil {
		fatal(err)
	}
	for _, n := range added {
		fmt.Printf("held out %s\n", n)
	}
}

func savedURLs(dir string) map[string]string {
	out := map[string]string{}
	files, _ := filepath.Glob(filepath.Join(dir, "*", "*.url"))
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			fatal(err)
		}
		name := strings.TrimSuffix(filepath.ToSlash(must(filepath.Rel(dir, f))), ".url")
		out[strings.TrimSpace(string(raw))] = name
	}
	return out
}

// The registrable label: "blog.teco.com" is "teco", An IP address is "page".
func siteName(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" || net.ParseIP(u.Hostname()) != nil {
		return "page"
	}
	labels := strings.Split(strings.ToLower(u.Hostname()), ".")
	if len(labels) < 2 {
		return labels[0]
	}
	i := len(labels) - 2
	if i > 0 && secondLevel[labels[i]] {
		i--
	}
	return labels[i]
}

// Labels that sit under a country code, as in "example.co.uk".
var secondLevel = map[string]bool{"co": true, "com": true, "org": true, "net": true, "gov": true, "ac": true, "edu": true}

func freeName(dir, language, base string) string {
	name := base
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(dir, language, name+".html")); os.IsNotExist(err) {
			return name
		}
		name = fmt.Sprintf("%s-%d", base, i)
	}
}

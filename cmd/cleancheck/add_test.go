package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tericcabrel/keyphrase/internal/evalset"
)

func TestSiteName(t *testing.T) {
	for _, tc := range []struct{ url, want string }{
		{"https://blog.elloha.com/2026/08/29/bilan/", "elloha"},
		{"https://mag.ticketsms.it/trend/halloween/", "ticketsms"},
		{"https://www.herr-meier.de/ghost/", "herr-meier"},
		{"https://blog-candidatos.occ.com.mx/sin-trafico/", "occ"},
		{"https://www.example.co.uk/post/", "example"},
		{"http://localhost:8080/post/", "localhost"},
		{"http://127.0.0.1:8080/post/", "page"},
	} {
		if got := siteName(tc.url); got != tc.want {
			t.Errorf("siteName(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}

func TestReadURLList(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "urls.txt")
	writeFile(t, good, "# French pages\nfr https://a.example/one/\n\nen  https://b.example/two/  holdout\n")
	got, err := readURLList(good)
	if err != nil {
		t.Fatal(err)
	}
	want := []entry{{lang: "fr", url: "https://a.example/one/"}, {lang: "en", url: "https://b.example/two/", holdout: true}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	for _, bad := range []string{"https://a.example/ no-lang-first\n", "pt https://a.example/\n", "fr\n", "fr https://a.example/ held\n", "fr https://a.example/ holdout extra\n"} {
		path := filepath.Join(dir, "bad.txt")
		writeFile(t, path, bad)
		if _, err := readURLList(path); err == nil {
			t.Errorf("readURLList(%q) = nil error", bad)
		}
	}
	if _, err := readURLList(filepath.Join(dir, "missing.txt")); err == nil {
		t.Error("missing file: nil error")
	}
}

// addPages saves Ghost posts under their site name, skips pages already
// saved and pages that aren't Ghost posts.
func TestAddPages(t *testing.T) {
	const ghost = `<html><head><meta name="generator" content="Ghost 6.65"></head><body><h1>T</h1><p>x</p></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.URL.Path == "/wordpress/" {
			_, _ = w.Write([]byte(`<html><head><meta name="generator" content="WordPress 6.8"></head><body><p>x</p></body></html>`))
			return
		}
		_, _ = w.Write([]byte(ghost))
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "fr", "page.html"), ghost)
	writeFile(t, filepath.Join(dir, "fr", "page.url"), srv.URL+"/old/\n")

	entries := []entry{
		{lang: "fr", url: srv.URL + "/old/", holdout: true},
		{lang: "fr", url: srv.URL + "/new/"},
		{lang: "fr", url: srv.URL + "/new/"},
		{lang: "fr", url: srv.URL + "/wordpress/", holdout: true},
		{lang: "de", url: srv.URL + "/other/", holdout: true},
	}
	added, saved := addPages(dir, entries, false)
	if want := []string{"fr/page-2", "de/page"}; !slices.Equal(added, want) {
		t.Fatalf("added %v, want %v", added, want)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "fr", "page-2.url"))
	if err != nil || strings.TrimSpace(string(raw)) != srv.URL+"/new/" {
		t.Fatalf("url file %q, %v", raw, err)
	}
	if html, readErr := os.ReadFile(filepath.Join(dir, "de", "page.html")); readErr != nil || string(html) != ghost {
		t.Fatalf("html file %q, %v", html, readErr)
	}

	// The page saved by an earlier run is held out too; the skipped
	// WordPress page isn't.
	markHoldout(dir, entries, saved)
	held, err := evalset.ReadHoldout(dir)
	if err != nil || len(held) != 2 || !held["fr/page"] || !held["de/page"] {
		t.Fatalf("held out %v, %v", held, err)
	}

	if added, _ := addPages(dir, []entry{{lang: "fr", url: srv.URL + "/wordpress/"}}, true); !slices.Equal(added, []string{"fr/page-3"}) {
		t.Fatalf("with anyCMS: added %v", added)
	}
}

func TestSizeCheck(t *testing.T) {
	page := []byte(`<html><head><meta property="og:title" content="Short"></head><body><h1>Short</h1><p>Only a few words here.</p></body></html>`)
	r, err := check(page, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.pass() || !strings.HasPrefix(r.Size, "only 6 words") {
		t.Fatalf("pass %v size %q", r.pass(), r.Size)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

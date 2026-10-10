package main

import (
	"slices"
	"strings"
	"testing"
)

func TestLeftoverChrome(t *testing.T) {
	lines := []string{
		"Title of the post",
		"Share on Facebook",
		"By Jordan Snobelen",
		"Readers who share their stories help the paper, the editor said on Monday.",
		"Related posts",
	}
	irrelevant := []string{"Facebook", "Jordan Snobelen", "share", "Related posts", "Newsletter"}
	got := leftoverChrome(lines, irrelevant, map[string]bool{key("Related posts"): true})
	want := []string{
		`"Facebook" on line "Share on Facebook"`,
		`"Jordan Snobelen" on line "By Jordan Snobelen"`,
		`"share" on line "Share on Facebook"`,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestShortLines(t *testing.T) {
	lines := []string{
		"Home",
		"About us",
		"Sign in now",
		"What you need",
		"4 200 milliards",
		"ℹ️",
		"💡",
		"Durchaus.",
		"Four words are here",
	}
	allowed := map[string]bool{key("What you need"): true}
	want := []string{"Home", "About us", "Sign in now"}
	if got := shortLines(lines, allowed); !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestMissing(t *testing.T) {
	const long = "The council voted on Monday to look at other ways to slow traffic near the schools and parks of the county."
	const kept = "Since the cameras stopped ticketing in November, more than eighty per cent of drivers speed in the safety zones."
	const chrome = "Join our community of readers and get weekly updates on what matters most in the county from our newsroom."
	old := strings.Join([]string{"Short line.", long + " " + kept, chrome, long}, "\n")
	got := missing(old, "Title\n"+kept, []string{"Join our community"})
	if !slices.Equal(got, []string{long}) {
		t.Fatalf("got %q", got)
	}
}

// check sorts missing sentences by where they were in the page.
func TestCheckPlaces(t *testing.T) {
	const page = `<html><head><meta property="og:title" content="Speed cameras"></head><body>
<header><p>Subscribe today to get every story from the county newsroom delivered to your inbox each week.</p></header>
<main><div class="topper"><p>A standfirst that sits above the article in the hero and repeats what the story says.</p></div>
<article><h1>Speed cameras</h1>
<p>Since the cameras stopped ticketing in November, more than eighty per cent of drivers speed in the safety zones.</p>
<p>The numbers come from the cameras themselves, which kept recording traffic after the contract with the vendor ended.</p>
<p>Council will vote later this month on whether staff should look at other ways to slow traffic near schools.</p>
</article></main></body></html>`
	old := strings.Join([]string{
		"Subscribe today to get every story from the county newsroom delivered to your inbox each week.",
		"A standfirst that sits above the article in the hero and repeats what the story says.",
		"This sentence was on the page when the old text was fetched but the page has changed since then.",
		"Since the cameras stopped ticketing in November, more than eighty per cent of drivers speed in the safety zones.",
	}, "\n")
	r, err := check([]byte(page), old, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.TitleOK || len(r.Chrome) != 0 || len(r.Short) != 0 {
		t.Fatalf("title ok %v chrome %q short %q", r.TitleOK, r.Chrome, r.Short)
	}
	if len(r.Pruned) != 1 || !strings.HasPrefix(r.Pruned[0], "[header] Subscribe today") {
		t.Errorf("pruned %q", r.Pruned)
	}
	if len(r.Outside) != 1 || !strings.HasPrefix(r.Outside[0], "A standfirst") {
		t.Errorf("outside %q", r.Outside)
	}
	if len(r.Gone) != 1 || !strings.HasPrefix(r.Gone[0], "This sentence") {
		t.Errorf("gone %q", r.Gone)
	}
	if len(r.Lost) != 0 {
		t.Errorf("lost %q", r.Lost)
	}
}

func TestAccept(t *testing.T) {
	r := report{
		Chrome: []string{`"Fernando Maciá" on line "Sección firmada por Fernando Maciá 🖊️"`, `"Share" on line "Share this"`},
		Short:  []string{"El radar", "Home"},
	}
	r.accept([]string{"Sección firmada por Fernando Maciá 🖊️", "el radar"})
	if !slices.Equal(r.Chrome, []string{`"Share" on line "Share this"`}) || !slices.Equal(r.Short, []string{"Home"}) || len(r.Accepted) != 2 {
		t.Fatalf("%+v", r)
	}
}

package webtext

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// A Ghost post in the Source theme: the content is section.gh-content, the
// title and excerpt are in the article's header next to the byline.
const ghostSource = `<!DOCTYPE html>
<html><head>
<title>Paid memberships | Herr Meier</title>
<meta property="og:title" content="Paid memberships with Ghost">
</head><body class="post-template">
<div class="gh-viewport">
<header id="gh-navigation" class="gh-navigation">
  <a class="gh-navigation-logo" href="/">Herr Meier</a>
  <ul class="nav"><li><a href="/">Home</a></li><li><a href="/about/">About</a></li></ul>
  <button class="gh-search">Search</button>
</header>
<main class="gh-main">
<article class="gh-article post tag-ghost author-herr">
  <header class="gh-article-header gh-canvas">
    <a class="gh-article-tag" href="/tag/ghost/">Ghost</a>
    <h1 class="gh-article-title">Set up paid memberships with Ghost and Stripe</h1>
    <p class="gh-article-excerpt">Paid memberships let your readers support your work directly.</p>
    <div class="gh-article-meta">
      <span class="gh-article-author-name"><a href="/author/herr/">Herr Meier</a></span>
      <time datetime="2026-09-12">12 Sep 2026</time>
      <span class="gh-article-meta-length">— 6 min read</span>
    </div>
  </header>
  <section class="gh-content gh-canvas is-body">
    <p>Free memberships are a good start when you want to build a loyal readership and send newsletters.</p>
    <h2>What you need</h2>
    <ul><li>A Ghost site with members turned on</li><li>A Stripe account</li></ul>
    <figure class="kg-card kg-image-card"><img src="a.png"><figcaption>Photo by Jane Doe / Unsplash</figcaption></figure>
    <p>Connect Stripe from the membership settings and confirm the connection.</p>
    <figure class="kg-card kg-bookmark-card"><a class="kg-bookmark-container" href="/other/">
      <div class="kg-bookmark-title">Another post</div>
      <div class="kg-bookmark-description">A different post about newsletters that is only linked here.</div>
    </a></figure>
    <div class="kg-card kg-cta-card kg-cta-bg-grey">
      <div class="kg-cta-sponsor-label-wrapper"><div class="kg-cta-sponsor-label">SPONSORED</div></div>
      <div class="kg-cta-content"><div class="kg-cta-text"><p>Yearly plans usually lower the cancellation rate.</p></div></div>
    </div>
    <div class="kg-card kg-cta-card kg-cta-minimal">
      <div class="kg-cta-text"><p>Want the checklist? Get it free by email.</p></div>
      <a class="kg-cta-button" href="#/portal/signup">Get the checklist</a>
    </div>
    <p><a href="/membership/">Become a member</a></p>
    <h2>Comments</h2>
  </section>
</article>
<aside class="gh-readmore gh-canvas"><h3>Read more</h3>
  <article class="gh-card post"><a class="gh-card-link" href="/x/"><h3 class="gh-card-title">Newsletter tips</h3>
  <p class="gh-card-excerpt">How to write newsletters that readers open every week.</p></a></article>
</aside>
</main>
<footer class="gh-footer"><p>© 2026 Herr Meier. Powered by Ghost</p></footer>
</div>
</body></html>`

const ghostSourceText = `Set up paid memberships with Ghost and Stripe
Paid memberships let your readers support your work directly.
Free memberships are a good start when you want to build a loyal readership and send newsletters.
What you need
A Ghost site with members turned on
A Stripe account
Connect Stripe from the membership settings and confirm the connection.
Yearly plans usually lower the cancellation rate.`

// A Ghost post in the Casper theme: the content is section.post-content, the
// author box is in the post's footer and read-next cards follow it.
const ghostCasper = `<!DOCTYPE html>
<html><head><title>Burnout is not brat</title></head><body>
<div class="site-wrapper">
<header id="site-head" class="site-header outer"><nav class="site-nav">
  <ul class="nav"><li class="nav-news"><a href="/news/">News</a></li><li class="nav-features"><a href="/features/">Features</a></li></ul>
</nav></header>
<main id="site-main" class="site-main outer">
<article class="article post tag-features">
  <header class="article-header gh-canvas">
    <section class="post-card-tags"><span class="post-card-primary-tag"><a href="/tag/features/">Features</a></span></section>
    <h1 class="article-title">Burnout is not brat</h1>
    <p class="article-excerpt">Emma asks what touring is like for everyone else in music.</p>
    <div class="article-byline"><div class="article-byline-meta">
      <h4 class="author-name"><a href="/author/emma/">Emma Wilkes</a></h4>
      <div class="byline-meta-content"><time datetime="2026-09-01">Sep 1, 2026</time> <span class="byline-reading-time">5 min read</span></div>
    </div></div>
  </header>
  <section class="post-content narrow-container">
    <p>Being on tour all year is not the way she wants to live her life.</p>
    <p>Published 1 September 2026 at 11:02 AM</p>
    <blockquote><p>“It is the love of the game that keeps us going.”</p></blockquote>
    <div class="share-buttons"><a href="https://x.com/share">Share</a><a href="https://facebook.com/share">Facebook</a></div>
    <p>Some of this is part of the job, but there has to be a better way.</p>
  </section>
  <footer class="post-full-footer">
    <section class="author-card"><h4>Emma Wilkes</h4><p>Emma writes about live music and the people who make it happen.</p></section>
  </footer>
</article>
</main>
<aside class="read-more-wrap outer"><div class="read-more inner">
  <article class="post-card post"><h2 class="post-card-title">Another feature</h2><p>An excerpt of another feature on the site.</p></article>
</div></aside>
<footer class="site-footer outer"><a href="/privacy/">Privacy</a></footer>
</div>
</body></html>`

const ghostCasperText = `Burnout is not brat
Emma asks what touring is like for everyone else in music.
Being on tour all year is not the way she wants to live her life.
“It is the love of the game that keeps us going.”
Some of this is part of the job, but there has to be a better way.`

// A news page with no content class: the article element holds the story,
// the standfirst sits in a hero before it and a sidebar lists other stories.
const newsPage = `<!DOCTYPE html>
<html><head><title>Speed cameras | County News</title></head><body>
<div id="cookie-consent" class="cookie-banner"><p>We use cookies to improve your experience on our website.</p><button>Accept</button></div>
<header class="masthead"><a href="/">County News</a><nav><a href="/local/">Local</a><a href="/sports/">Sports</a></nav></header>
<main>
<div class="topper"><p class="topper__standfirst">Data from the former cameras shows most drivers ignore the limit.</p></div>
<div class="layout">
<article>
  <h1>Drivers speed up after camera enforcement stopped</h1>
  <p class="dateline">Posted on 3 March 2026</p>
  <p>Since the county's speed cameras stopped ticketing in November, more than 80 per cent of drivers speed in community safety zones.</p>
  <p>The numbers come from the cameras themselves, which kept recording traffic after the county ended its contract with the vendor.</p>
  <p>Council will vote later this month on whether staff should look at other ways to slow traffic near schools and parks.</p>
  <ul class="tags"><li><a href="/tag/traffic/">traffic</a></li><li><a href="/tag/council/">council</a></li></ul>
</article>
<aside class="sidebar"><h3>Most read</h3><ul>
  <li><a href="/a/">Council approves new budget for roads and bridges across the county</a></li>
  <li><a href="/b/">Local team wins the championship after a long season</a></li>
</ul></aside>
</div>
</main>
<footer><p>Copyright 2026 County News. All rights reserved.</p></footer>
</body></html>`

const newsPageText = `Drivers speed up after camera enforcement stopped
Data from the former cameras shows most drivers ignore the limit.
Since the county's speed cameras stopped ticketing in November, more than 80 per cent of drivers speed in community safety zones.
The numbers come from the cameras themselves, which kept recording traffic after the county ended its contract with the vendor.
Council will vote later this month on whether staff should look at other ways to slow traffic near schools and parks.`

// A page made only of divs: the menu and footer are found by their ids and
// classes, and the title comes from og:title.
const divPage = `<!DOCTYPE html>
<html><head><title>Old site</title><meta property="og:title" content="Tips for small offices"></head><body>
<div id="top-menu"><div><a href="/">Home</a></div><div><a href="/blog/">Blog</a></div></div>
<div class="wrapper"><div class="col">
  <div>Small offices rarely have an IT department, so a few habits go a long way.</div>
  <div>Back up every laptop every night and test a restore once a month.</div>
</div></div>
<div id="copyright-footer">© 2026 Old site</div>
</body></html>`

const divPageText = `Tips for small offices
Small offices rarely have an IT department, so a few habits go a long way.
Back up every laptop every night and test a restore once a month.`

func TestExtract(t *testing.T) {
	for _, tc := range []struct {
		name, page, want string
		rule             Rule
		container        string
	}{
		{"ghost source", ghostSource, ghostSourceText, RuleContentClass, "section.gh-content.gh-canvas.is-body"},
		{"ghost casper", ghostCasper, ghostCasperText, RuleContentClass, "section.post-content.narrow-container"},
		{"news page", newsPage, newsPageText, RuleArticle, "article"},
		{"div page", divPage, divPageText, RuleBody, "body"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := Extract(strings.NewReader(tc.page))
			if err != nil {
				t.Fatal(err)
			}
			if c.Text != tc.want {
				t.Errorf("text =\n%s\nwant\n%s", c.Text, tc.want)
			}
			if c.Rule != tc.rule || c.Container != tc.container {
				t.Errorf("rule %q container %q, want %q %q", c.Rule, c.Container, tc.rule, tc.container)
			}
		})
	}
}

// An article whose class holds a boilerplate word stays when it holds most
// of the page's text.
func TestMajorityGuard(t *testing.T) {
	const page = `<html><body>
<nav><a href="/">Home</a></nav>
<article class="sticky-sidebar-blog-content">
  <h1>Party denied from running</h1>
  <p>The court threw the party off the ballot for next month's election, days before voting starts.</p>
  <p>The ruling leaves no party on the ballot that openly opposes the conflict, analysts said on Monday.</p>
  <p>The party said it would appeal, although few expect the higher court to overturn the decision in time.</p>
  <p>All remaining parties in the race either support the conflict or decline to criticise it in public.</p>
</article>
<aside class="sidebar"><p><a href="/x/">Other story</a></p></aside>
</body></html>`
	c, err := Extract(strings.NewReader(page))
	if err != nil {
		t.Fatal(err)
	}
	if c.Rule != RuleArticle || !strings.HasPrefix(c.Text, "Party denied from running\nThe court threw") {
		t.Fatalf("rule %q text\n%s", c.Rule, c.Text)
	}
}

// A content class must be a whole class name: a card inside the content
// does not replace it.
func TestContentClassExact(t *testing.T) {
	const page = `<html><body><main>
<div class="gh-featured-post__content"><p>A featured post teaser.</p></div>
<div class="post-content"><h1>Real post</h1><p>The real post body sits here and is written in full sentences.</p></div>
</main></body></html>`
	c, err := Extract(strings.NewReader(page))
	if err != nil {
		t.Fatal(err)
	}
	if c.Container != "div.post-content" || c.Text != "Real post\nThe real post body sits here and is written in full sentences." {
		t.Fatalf("container %q text\n%s", c.Container, c.Text)
	}
}

// The heading above Ghost's members' comments is pruned even when the theme
// puts it inside the content container.
func TestCommentHeading(t *testing.T) {
	const page = `<html><body>
<div class="gh-content">
  <h1>Public money</h1>
  <p>A company with ten thousand euros of capital earned millions in a few months.</p>
  <p>The contracts were signed without any call for tenders during the crisis.</p>
  <div class="section-heading"><span>Discussion des membres</span><span><script data-ghost-comment-count="1"></script></span></div>
</div>
</body></html>`
	c, err := Extract(strings.NewReader(page))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(c.Text, "Discussion") {
		t.Fatalf("comment heading kept\n%s", c.Text)
	}
}

func TestBoilerplate(t *testing.T) {
	for _, tc := range []struct {
		element string
		want    bool
	}{
		{`<div class="shadow">`, false},
		{`<div class="canvas">`, false},
		{`<div class="gh-canvas">`, false},
		{`<div class="subheader">`, false},
		{`<div class="loaded">`, false},
		{`<div class="tag-news author-jane">`, false},
		{`<div class="gh-content">`, false},
		{`<main class="site-main">`, false},
		{`<div class="header-image">`, true},
		{`<div class="post-header">`, true},
		{`<div class="share-buttons">`, true},
		{`<div class="site_footer">`, true},
		{`<div id="top-menu">`, true},
		{`<div class="kg-card kg-bookmark-card">`, true},
		{`<div role="navigation">`, true},
		{`<div aria-hidden="true">`, true},
		{`<nav>`, true},
		{`<aside>`, true},
		{`<header>`, true},
	} {
		n := parseElement(t, tc.element+"x")
		if got := Boilerplate(n); got != tc.want {
			t.Errorf("Boilerplate(%s) = %v, want %v", tc.element, got, tc.want)
		}
	}
}

func TestMetaLine(t *testing.T) {
	for _, tc := range []struct {
		line string
		want bool
	}{
		{"12 Sep 2026", true},
		{"— 6 min read", true},
		{"Published 1 September 2026 at 11:02 AM", true},
		{"Veröffentlicht am 3. März 2026 um 10:15 Uhr", true},
		{"Publié le 29 août 2026", true},
		{"5 minutos de lectura", true},
		{"The council met on 3 March 2026 to vote.", false},
		{"2026", false},
		{"Sage : 1.85°C", false},
	} {
		if got := metaLine(tc.line); got != tc.want {
			t.Errorf("metaLine(%q) = %v, want %v", tc.line, got, tc.want)
		}
	}
}

func parseElement(t *testing.T, fragment string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader("<html><body>" + fragment + "</body></html>"))
	if err != nil {
		t.Fatal(err)
	}
	body := findElement(doc, "body")
	for c := body.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			return c
		}
	}
	t.Fatalf("no element in %q", fragment)
	return nil
}

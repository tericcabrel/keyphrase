package webtext

import (
	"io"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
)

// Rule names the step that chose the content container.
type Rule string

const (
	RuleContentClass Rule = "content class"
	RuleArticle      Rule = "article"
	RuleMain         Rule = "main"
	RuleBody         Rule = "body"
)

const (
	// minContainerText is the scored text an article or main element needs
	// to be chosen over the pruned body.
	minContainerText = 250
	// maxLinkBlockWords is the longest block made only of a link that is
	// dropped as a button or menu entry.
	maxLinkBlockWords = 4
	// minLedeWords is the shortest excerpt written as the lede.
	minLedeWords = 5
)

// Content is the main content of a page.
type Content struct {
	Text string
	Rule Rule
	// Container is the tag and classes of the chosen element, such as
	// "section.gh-content.gh-canvas".
	Container string
}

// FromHTML returns the title and main content of an HTML page, one block per
// line.
func FromHTML(reader io.Reader) (string, error) {
	content, err := Extract(reader)
	return content.Text, err
}

// Extract is FromHTML that also reports which rule chose the container.
//
// Boilerplate (navigation, headers, footers, sidebars, share and subscribe
// blocks, buttons, related posts, cookie banners) is pruned first. The
// container is then the element with a known content class, else the best
// scored article or main element, else the pruned body. The output starts
// with the title and the post's excerpt, each written once.
func Extract(reader io.Reader) (Content, error) {
	doc, err := html.Parse(reader)
	if err != nil {
		return Content{}, err
	}
	body := findElement(doc, "body")
	if body == nil {
		body = doc
	}
	state := &extraction{dropped: map[*html.Node]bool{}, headings: map[string]bool{}}
	state.pageText = state.score(body)
	state.prune(body)
	container, rule := state.chooseContainer(body)
	scope := articleScope(container)
	title, titleNode := state.pageTitle(doc, container, scope)
	lede, ledeNode := state.lede(body, scope)

	var builder strings.Builder
	builder.WriteString(title)
	builder.WriteByte('\n')
	builder.WriteString(lede)
	builder.WriteByte('\n')
	state.render(&builder, container, titleNode, ledeNode)
	return Content{
		Text:      state.dropLines(normalize(builder.String()), title, lede),
		Rule:      rule,
		Container: Describe(container),
	}, nil
}

// Describe returns an element's tag followed by its classes, dot-separated.
func Describe(node *html.Node) string {
	class, _ := attr(node, "class")
	return strings.Join(append([]string{node.Data}, strings.Fields(class)...), ".")
}

// extraction holds the pruned nodes of one page. Nodes stay in the tree so
// that the title and excerpt can still be read from the post's own header.
type extraction struct {
	dropped map[*html.Node]bool
	// pageText is the scored text of the whole body. An element holding
	// more than half of it is the article, so no rule but hidden drops it:
	// a "sticky-sidebar-blog-content" wrapper or a page-wide form stay.
	pageText int
	// headings holds the rendered h1 to h6 texts.
	headings map[string]bool
}

var (
	contentClasses = map[string]bool{
		"gh-content": true, "post-content": true, "article-content": true,
		"entry-content": true, "post-body": true, "article-body": true,
	}
	boilerplateWords = splitWords(
		"nav", "menu", "header", "footer", "sidebar", "share", "social",
		"subscribe", "newsletter", "signup", "related", "recommend", "comments",
		"cookie", "consent", "breadcrumb", "pagination", "promo", "advert", "ad",
		"toc", "btn", "button", "cta", "author", "authors", "byline", "meta",
		"tag", "tags", "gh-head", "gh-foot", "kg-cta-card",
		"kg-signup-card", "kg-bookmark-card", "pswp",
	)
	boilerplateRoles = map[string]bool{
		"navigation": true, "banner": true, "contentinfo": true,
		"complementary": true, "search": true, "dialog": true,
	}
)

func splitWords(words ...string) [][]string {
	out := make([][]string, len(words))
	for i, word := range words {
		out[i] = classParts(word)
	}
	return out
}

// classParts splits a class or id on hyphens and underscores.
func classParts(name string) []string {
	return strings.FieldsFunc(strings.ToLower(name), func(r rune) bool { return r == '-' || r == '_' })
}

// hasParts reports whether word appears as consecutive whole parts of parts.
func hasParts(parts, word []string) bool {
	for i := 0; i+len(word) <= len(parts); i++ {
		match := true
		for j, part := range word {
			if parts[i+j] != part {
				match = false

				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// names returns the class and id tokens of node. Ghost adds tag-* and author-*
// classes to the body and to post cards; they name topics, not layout.
func names(node *html.Node) [][]string {
	var out [][]string
	for _, attribute := range node.Attr {
		switch attribute.Key {
		case "class", "id":
			for token := range strings.FieldsSeq(attribute.Val) {
				lower := strings.ToLower(token)
				if strings.HasPrefix(lower, "tag-") || strings.HasPrefix(lower, "author-") {
					continue
				}
				out = append(out, classParts(token))
			}
		}
	}
	return out
}

func attr(node *html.Node, key string) (string, bool) {
	for _, attribute := range node.Attr {
		if attribute.Key == key {
			return attribute.Val, true
		}
	}
	return "", false
}

func hasContentClass(node *html.Node) bool {
	if node.Type != html.ElementNode {
		return false
	}
	class, _ := attr(node, "class")
	for token := range strings.FieldsSeq(class) {
		if contentClasses[strings.ToLower(token)] {
			return true
		}
	}
	return false
}

func hidden(node *html.Node) bool {
	if value, ok := attr(node, "aria-hidden"); ok && strings.EqualFold(value, "true") {
		return true
	}
	_, ok := attr(node, "hidden")
	return ok
}

// Boilerplate reports whether an element is page chrome by its tag, role
// or class and id words, without looking at its text.
func Boilerplate(node *html.Node) bool {
	return boilerplateReason(node) != ""
}

// boilerplateReason returns the tag, role or word that makes node page chrome,
// or "".
func boilerplateReason(node *html.Node) string {
	if node.Type != html.ElementNode {
		return ""
	}
	switch node.Data {
	case "nav", "aside", "form", "button", "dialog", "header", "footer":
		return node.Data
	case "html", "body", "main":
		return ""
	}
	if role, _ := attr(node, "role"); boilerplateRoles[strings.ToLower(role)] {
		return "role=" + role
	}
	if hidden(node) {
		return "hidden"
	}
	if hasContentClass(node) {
		return ""
	}
	for _, parts := range names(node) {
		for _, word := range boilerplateWords {
			if hasParts(parts, word) {
				return strings.Join(word, "-")
			}
		}
	}
	return ""
}

func (x *extraction) prune(node *html.Node) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		if hidden(child) {
			x.dropped[child] = true
			continue
		}
		if callout(child) {
			if label := findNode(child, func(descendant *html.Node) bool { return hasName(descendant, sponsorLabel) }); label != nil {
				x.dropped[label] = true
			}
			continue
		}
		if boilerplateReason(child) != "" && 2*x.score(child) <= x.pageText || commentHeading(child) {
			x.dropped[child] = true
			continue
		}
		x.prune(child)
	}
}

const maxCommentHeadingWords = 6

// commentHeading reports a short element holding Ghost's comment counter,
// the heading themes put above the members' comments.
func commentHeading(node *html.Node) bool {
	counter := findNode(node, func(descendant *html.Node) bool {
		_, ok := attr(descendant, "data-ghost-comment-count")
		return ok
	})
	return counter != nil && len(strings.Fields(nodeText(node))) <= maxCommentHeadingWords
}

var sponsorLabel = []string{"kg", "cta", "sponsor", "label"}

// callout reports a Ghost CTA card without a button. Authors use those as a
// highlight box inside the article; a card asking the reader to act has one.
// Only its "SPONSORED" label is pruned.
func callout(node *html.Node) bool {
	card, button := []string{"kg", "cta", "card"}, []string{"kg", "cta", "button"}
	if !hasName(node, card) {
		return false
	}
	return findNode(node, func(descendant *html.Node) bool { return hasName(descendant, button) }) == nil
}

func hasName(node *html.Node, word []string) bool {
	for _, parts := range names(node) {
		if hasParts(parts, word) {
			return true
		}
	}
	return false
}

func findNode(node *html.Node, match func(*html.Node) bool) *html.Node {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		if match(child) {
			return child
		}
		if found := findNode(child, match); found != nil {
			return found
		}
	}
	return nil
}

func (x *extraction) chooseContainer(body *html.Node) (*html.Node, Rule) {
	var classed, articles, mains []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if x.dropped[node] {
			return
		}
		if node.Type == html.ElementNode {
			if hasContentClass(node) {
				classed = append(classed, node)
			}
			role, _ := attr(node, "role")
			switch {
			case node.Data == "article":
				articles = append(articles, node)
			case node.Data == "main" || strings.EqualFold(role, "main"):
				mains = append(mains, node)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(body)

	if node, score := x.best(classed); score > 0 {
		return node, RuleContentClass
	}
	if node, score := x.best(articles); score >= minContainerText {
		return node, RuleArticle
	}
	if node, score := x.best(mains); score >= minContainerText {
		return node, RuleMain
	}
	return body, RuleBody
}

func (x *extraction) best(nodes []*html.Node) (*html.Node, int) {
	var top *html.Node
	topScore := -1
	for _, node := range nodes {
		if score := x.score(node); score > topScore {
			top, topScore = node, score
		}
	}
	return top, topScore
}

// score is the text inside p, li and h2 to h6, minus the text of the links
// inside them, in runes.
func (x *extraction) score(node *html.Node) int {
	total := 0
	var walk func(*html.Node)
	walk = func(descendant *html.Node) {
		if x.dropped[descendant] {
			return
		}
		if descendant.Type == html.ElementNode {
			if skipped(descendant.Data) {
				return
			}
			switch descendant.Data {
			case "p", "li", "h2", "h3", "h4", "h5", "h6":
				total += x.textLen(descendant, false) - x.textLen(descendant, true)
				return
			}
		}
		for child := descendant.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return total
}

// textLen counts the non-space runes of node's text, or only of the text inside
// links when linksOnly is set.
func (x *extraction) textLen(node *html.Node, linksOnly bool) int {
	count := 0
	var walk func(*html.Node, bool)
	walk = func(descendant *html.Node, inLink bool) {
		if x.dropped[descendant] {
			return
		}
		switch descendant.Type {
		case html.ElementNode:
			if skipped(descendant.Data) {
				return
			}
			inLink = inLink || descendant.Data == "a"
		case html.TextNode:
			if inLink || !linksOnly {
				for _, r := range descendant.Data {
					if !unicode.IsSpace(r) {
						count++
					}
				}
			}
		}
		for child := descendant.FirstChild; child != nil; child = child.NextSibling {
			walk(child, inLink)
		}
	}
	walk(node, false)
	return count
}

// articleScope is the article holding the container, or the container.
func articleScope(container *html.Node) *html.Node {
	for ancestor := container; ancestor != nil; ancestor = ancestor.Parent {
		if ancestor.Type == html.ElementNode && ancestor.Data == "article" {
			return ancestor
		}
	}
	return container
}

// pageTitle returns the first h1 of the post, else og:title, else the title
// element. Inside an article the h1 is read from its pruned header too. The
// h1 node is returned so it is not written twice.
func (x *extraction) pageTitle(doc, container, scope *html.Node) (string, *html.Node) {
	withDropped := scope.Data == "article"
	if scope != container && !withDropped {
		scope = container
	}
	if h1 := x.find(scope, withDropped, func(candidate *html.Node) bool { return candidate.Data == "h1" && nodeText(candidate) != "" }); h1 != nil {
		return nodeText(h1), h1
	}
	var ogTitle, title string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			switch node.Data {
			case "meta":
				if property, _ := attr(node, "property"); property == "og:title" && ogTitle == "" {
					content, _ := attr(node, "content")
					ogTitle = collapse(content)
				}
			case "title":
				if title == "" {
					title = nodeText(node)
				}
			case "body":
				return
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if ogTitle != "" {
		return ogTitle, nil
	}
	return title, nil
}

// lede returns the post's excerpt, an element whose class has an "excerpt"
// or "standfirst" part. It is read from the article's pruned header too, else
// from before the container, where some themes put the post's hero. Post
// cards have excerpts of other posts and are skipped.
func (x *extraction) lede(body, scope *html.Node) (string, *html.Node) {
	card := []string{"card"}
	match := func(candidate *html.Node) bool {
		if hasName(candidate, card) {
			return false
		}
		return (hasName(candidate, []string{"excerpt"}) || hasName(candidate, []string{"standfirst"})) &&
			len(strings.Fields(nodeText(candidate))) >= minLedeWords
	}
	withDropped := scope.Data == "article"
	excerpt := x.find(scope, withDropped, match)
	if excerpt == nil {
		excerpt = x.findBefore(body, scope, match)
	}
	if excerpt == nil {
		return "", nil
	}
	// An "excerpt-and-metadata" wrapper also holds the byline.
	for inner := x.find(excerpt, withDropped, match); inner != nil; inner = x.find(excerpt, withDropped, match) {
		excerpt = inner
	}
	return nodeText(excerpt), excerpt
}

// findBefore returns the first element that matches, skipping pruned
// subtrees and post cards, among the elements before stop in document order.
func (x *extraction) findBefore(node, stop *html.Node, match func(*html.Node) bool) *html.Node {
	card := []string{"card"}
	var found *html.Node
	var walk func(*html.Node) bool
	walk = func(descendant *html.Node) bool {
		for child := descendant.FirstChild; child != nil; child = child.NextSibling {
			if child == stop {
				return true
			}
			if child.Type != html.ElementNode || skipped(child.Data) || x.dropped[child] || hasName(child, card) {
				continue
			}
			if match(child) {
				found = child
				return true
			}
			if walk(child) {
				return true
			}
		}
		return false
	}
	walk(node)
	return found
}

// find returns the first element below node that matches, skipping pruned
// subtrees unless withDropped is set, and never descending into post cards.
func (x *extraction) find(node *html.Node, withDropped bool, match func(*html.Node) bool) *html.Node {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode || skipped(child.Data) || (x.dropped[child] && !withDropped) {
			continue
		}
		if match(child) {
			return child
		}
		if found := x.find(child, withDropped, match); found != nil {
			return found
		}
	}
	return nil
}

func findElement(node *html.Node, tag string) *html.Node {
	if node.Type == html.ElementNode && node.Data == tag {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := findElement(child, tag); found != nil {
			return found
		}
	}
	return nil
}

func nodeText(node *html.Node) string {
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(descendant *html.Node) {
		if descendant.Type == html.ElementNode && skipped(descendant.Data) {
			return
		}
		if descendant.Type == html.TextNode {
			writeText(&builder, descendant.Data)
			builder.WriteByte(' ')
		}
		for child := descendant.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return collapse(builder.String())
}

func collapse(text string) string { return strings.Join(strings.Fields(text), " ") }

func (x *extraction) render(builder *strings.Builder, node *html.Node, skip ...*html.Node) {
	if x.dropped[node] {
		return
	}
	if slices.Contains(skip, node) {
		return
	}
	block := node.Type == html.ElementNode && blockTag(node.Data)
	if node.Type == html.ElementNode {
		if skipped(node.Data) || (node.Data == "figcaption" && isCredit(nodeText(node))) {
			return
		}
		if block && x.linkBlock(node) {
			return
		}
		switch node.Data {
		case "h1", "h2", "h3", "h4", "h5", "h6":
			x.headings[nodeText(node)] = true
		}
	}
	if block {
		builder.WriteByte('\n')
	}
	if node.Type == html.TextNode {
		writeText(builder, node.Data)
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		x.render(builder, child, skip...)
	}
	if block {
		builder.WriteByte('\n')
	}
}

// linkBlock reports a short block whose text is all link text, such as a
// button or a menu entry.
func (x *extraction) linkBlock(node *html.Node) bool {
	textRunes := x.textLen(node, false)
	return textRunes > 0 && textRunes == x.textLen(node, true) && len(strings.Fields(nodeText(node))) <= maxLinkBlockWords
}

var (
	creditRe = regexp.MustCompile(`(?i)^(photo|photos|foto|fotos|image|images|bild|bilder|crédit|crédits|credit|credits|source|quelle|fuente|fonte|illustration)\b|©|\b(unsplash|getty|shutterstock|pexels|pixabay|adobe stock|istock)\b`)

	month  = `(?:jan|janv|january|januar|janvier|enero|ene|gennaio|gen|feb|febr|february|februar|févr|fevr|février|fevrier|febrero|febbraio|mar|march|märz|mrz|mars|marzo|apr|april|avr|avril|abril|abr|aprile|may|mai|mayo|maggio|mag|jun|june|juni|juin|junio|giugno|giu|jul|july|juli|juil|juillet|julio|luglio|lug|aug|august|août|aout|agosto|ago|sep|sept|september|septembre|septiembre|settembre|set|oct|october|okt|oktober|octobre|octubre|ottobre|ott|nov|november|novembre|noviembre|dec|december|dez|dezember|déc|décembre|decembre|diciembre|dicembre|dic)\.?`
	dateRe = regexp.MustCompile(`(?i)\b\d{1,2}\.?\s+(?:de\s+)?` + month + `,?\s+(?:de\s+)?\d{4}\b|\b` + month + `\s+\d{1,2},?\s+\d{4}\b|\b\d{4}-\d{2}-\d{2}\b|\b\d{1,2}[./]\d{1,2}[./]\d{2,4}\b`)
	timeRe = regexp.MustCompile(`(?i)\b\d{1,2}[:.h]\d{2}\s*(?:[ap]\.?m\.?|uhr)?`)
	readRe = regexp.MustCompile(`(?i)\b\d+\s*(?:min|mins|minute|minutes|minuten|minuto|minutos|minuti)\b\.?(?:\s+(?:read|reading|de\s+lecture|lesezeit|lesedauer|de\s+lectura|di\s+lettura|lettura|lectura))?|\b(?:lettura|lectura|lesezeit|lesedauer)\s+\d+\s*min\b\.?`)
	// stampWords may surround a date: "Published: September 23, 2026 at 11:02 AM".
	stampWords = splitSet("published updated posted last on at veröffentlicht aktualisiert am um publié mis à jour le publicado actualizado el pubblicato aggiornato il")
)

func splitSet(words string) map[string]bool {
	set := map[string]bool{}
	for word := range strings.FieldsSeq(words) {
		set[word] = true
	}
	return set
}

// isCredit reports a figcaption that is only a short photo credit.
func isCredit(caption string) bool {
	return caption != "" && len(strings.Fields(caption)) <= 12 && creditRe.MatchString(caption)
}

// metaLine reports a line that is only dates, times, reading times and the
// words that introduce them.
func metaLine(line string) bool {
	rest := readRe.ReplaceAllString(dateRe.ReplaceAllString(line, ""), "")
	if utf8.RuneCountInString(rest) == utf8.RuneCountInString(line) {
		return false
	}
	rest = timeRe.ReplaceAllString(rest, "")
	words := strings.FieldsFunc(strings.ToLower(rest), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	for _, word := range words {
		if !stampWords[word] {
			return false
		}
	}
	return true
}

// dropLines removes date and reading-time lines, later copies of the title
// and lede, and headings left at the end with nothing under them, such as
// the heading of a pruned comments section.
func (x *extraction) dropLines(text, title, lede string) string {
	lines := strings.Split(text, "\n")
	out := lines[:0]
	seen := map[string]bool{}
	for _, line := range lines {
		if metaLine(line) {
			continue
		}
		if strings.EqualFold(line, title) || strings.EqualFold(line, lede) {
			if seen[strings.ToLower(line)] {
				continue
			}
			seen[strings.ToLower(line)] = true
		}
		out = append(out, line)
	}
	for len(out) > 1 && x.headings[out[len(out)-1]] {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n")
}

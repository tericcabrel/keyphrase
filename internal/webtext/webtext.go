// Package webtext fetches a page and returns its title and main content.
package webtext

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html/charset"
)

const (
	// MaxTextBytes is the maximum returned text. It matches the keyword API
	// limit so the result can be sent as text to POST /v1/keywords.
	MaxTextBytes = 64 << 10

	maxDownloadBytes = 2 << 20
	maxRedirects     = 5
	fetchTimeout     = 10 * time.Second
	userAgent        = "keyphrase"
)

// Error is a fetch failure. StatusCode is the HTTP status the API should return.
type Error struct {
	StatusCode int
	Message    string
}

func (e *Error) Error() string { return e.Message }

func badRequest(message string) *Error {
	return &Error{StatusCode: http.StatusBadRequest, Message: message}
}

func upstream(message string) *Error {
	return &Error{StatusCode: http.StatusBadGateway, Message: message}
}

// Text fetches rawURL and returns its title and main content, see FromHTML
// (plain text is returned as is). The text is at most
// MaxTextBytes. truncated is true when the page text was longer than that.
func Text(ctx context.Context, rawURL string) (text string, truncated bool, err error) {
	page, err := Fetch(ctx, rawURL)
	if err != nil {
		return "", false, err
	}
	text = page.Body
	if page.HTML && text != "" {
		text, err = FromHTML(strings.NewReader(page.Body))
		if err != nil {
			return "", false, badRequest("response is not HTML or plain text")
		}
	}
	text, truncated = clip(text)
	return text, truncated, nil
}

// Page is a fetched response body, decoded to UTF-8.
type Page struct {
	Body string
	// HTML is false for text/plain.
	HTML bool
}

// Fetch downloads rawURL with the limits of Text: http and https only, at
// most 5 redirects, 10 seconds and 2 MiB, and only HTML or plain text.
func Fetch(ctx context.Context, rawURL string) (Page, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	parsedURL, err := parseURL(rawURL)
	if err != nil {
		return Page{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return Page{}, badRequest("invalid url")
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain;q=0.9,*/*;q=0.8")

	response, err := client.Do(request)
	if err != nil {
		if fetchErr, ok := errors.AsType[*Error](err); ok {
			return Page{}, fetchErr
		}
		return Page{}, upstream("fetch failed")
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return Page{}, upstream(fmt.Sprintf("upstream returned %d", response.StatusCode))
	}
	kind, err := classify(response.Header.Get("Content-Type"))
	if err != nil {
		return Page{}, err
	}
	raw, err := readLimited(response.Body)
	if err != nil {
		return Page{}, err
	}
	page := Page{HTML: kind == "html"}
	if len(raw) == 0 {
		return page, nil
	}
	page.Body, err = decode(raw, response.Header.Get("Content-Type"))
	if err != nil {
		return Page{}, err
	}
	return page, nil
}

// via includes the original request, so len(via) > maxRedirects means that
// many redirects were already followed.
func checkRedirect(request *http.Request, via []*http.Request) error {
	if len(via) > maxRedirects {
		return upstream("too many redirects")
	}
	return validateURL(request.URL)
}

var client = &http.Client{
	Timeout:       fetchTimeout,
	CheckRedirect: checkRedirect,
}

func parseURL(rawURL string) (*url.URL, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, badRequest("invalid url")
	}
	if err := validateURL(parsedURL); err != nil {
		return nil, err
	}
	return parsedURL, nil
}

func validateURL(parsedURL *url.URL) error {
	if parsedURL == nil {
		return badRequest("invalid url")
	}
	switch parsedURL.Scheme {
	case "http", "https":
		if parsedURL.Host == "" {
			return badRequest("invalid url")
		}
		return nil
	case "":
		return badRequest("invalid url")
	default:
		return badRequest("url scheme must be http or https")
	}
}

func classify(contentType string) (string, error) {
	if strings.TrimSpace(contentType) == "" {
		return "", badRequest("response is not HTML or plain text")
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", badRequest("response is not HTML or plain text")
	}
	switch mediaType {
	case "text/html", "application/xhtml+xml":
		return "html", nil
	case "text/plain":
		return "plain", nil
	default:
		return "", badRequest("response is not HTML or plain text")
	}
}

func readLimited(reader io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, maxDownloadBytes+1))
	if err != nil {
		return nil, upstream("fetch failed")
	}
	if len(raw) > maxDownloadBytes {
		raw = raw[:maxDownloadBytes]
	}
	return raw, nil
}

func decode(raw []byte, contentType string) (string, error) {
	reader, err := charset.NewReader(bytes.NewReader(raw), contentType)
	if err != nil {
		return "", upstream("fetch failed")
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return "", badRequest("cannot decode page")
	}
	return string(decoded), nil
}

func skipped(tag string) bool {
	switch tag {
	case "script", "style", "noscript", "template", "svg", "iframe", "pre":
		return true
	default:
		return false
	}
}

func blockTag(tag string) bool {
	switch tag {
	case "address", "article", "aside", "blockquote", "br", "dd", "div", "dl", "dt",
		"figcaption", "figure", "footer", "h1", "h2", "h3", "h4", "h5", "h6",
		"header", "hr", "li", "main", "nav", "ol", "p", "section",
		"table", "tbody", "td", "tfoot", "th", "thead", "title", "tr", "ul":
		return true
	default:
		return false
	}
}

func writeText(builder *strings.Builder, text string) {
	for _, r := range text {
		if unicode.IsSpace(r) {
			builder.WriteByte(' ')
			continue
		}
		builder.WriteRune(r)
	}
}

func normalize(text string) string {
	var lines []string
	for line := range strings.Lines(text) {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func clip(text string) (string, bool) {
	if len(text) <= MaxTextBytes {
		return text, false
	}
	end := MaxTextBytes
	for end > 0 && !utf8.RuneStart(text[end]) {
		end--
	}
	return text[:end], true
}

package webtext

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestHTMLVisibleText(t *testing.T) {
	const page = `<!DOCTYPE html>
<html>
<head>
<title>  Wind   farms </title>
<style>body{color:red}</style>
<script>var secret = "do-not-extract";</script>
</head>
<body>
<nav>Home</nav>
<h1>Offshore wind</h1>
<p>First paragraph
continues.</p>
<p>Second <b>paragraph</b>.</p>
<p>Line one<br>Line two</p>
<script>alert("hidden")</script>
<noscript>enable js</noscript>
<template>hidden-template</template>
<svg><text>hidden-svg</text></svg>
<iframe>hidden-iframe</iframe>
<pre>hidden-pre</pre>
</body>
</html>`
	const want = "Offshore wind\nFirst paragraph continues.\nSecond paragraph.\nLine one\nLine two"

	srv := serve(t, http.StatusOK, "text/html; charset=utf-8", page)
	text, truncated, err := Text(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if truncated {
		t.Fatal("truncated")
	}
	if text != want {
		t.Fatalf("text =\n%s\nwant\n%s", text, want)
	}
	for _, hidden := range []string{"do-not-extract", "hidden", "enable js", "hidden-template", "hidden-svg", "hidden-iframe", "hidden-pre", "color"} {
		if strings.Contains(text, hidden) {
			t.Errorf("text contains %q", hidden)
		}
	}
}

func TestPlainText(t *testing.T) {
	const body = "hello\n  world"
	srv := serve(t, http.StatusOK, "text/plain; charset=utf-8", body)
	text, truncated, err := Text(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if truncated || text != body {
		t.Fatalf("text %q truncated %v", text, truncated)
	}
}

func TestCharset(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=iso-8859-1")
		_, _ = w.Write([]byte("<title>Caf\xe9</title><p>cr\xe8me</p>"))
	}))
	t.Cleanup(srv.Close)

	text, truncated, err := Text(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if truncated || text != "Café\ncrème" {
		t.Fatalf("text %q truncated %v", text, truncated)
	}
}

func TestTruncateUTF8(t *testing.T) {
	euro := "\u20ac"
	body := strings.Repeat(euro, MaxTextBytes/len(euro)+2)
	if len(body) <= MaxTextBytes {
		t.Fatalf("body length %d", len(body))
	}
	srv := serve(t, http.StatusOK, "text/plain", body)
	text, truncated, err := Text(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !truncated {
		t.Fatal("want truncated")
	}
	if len(text) > MaxTextBytes || len(text) == 0 || len(text)%len(euro) != 0 || !utf8.ValidString(text) || !strings.HasPrefix(body, text) {
		t.Fatalf("len %d valid %v", len(text), utf8.ValidString(text))
	}
}

func TestExactTextLimit(t *testing.T) {
	body := strings.Repeat("a", MaxTextBytes)
	srv := serve(t, http.StatusOK, "text/plain", body)
	text, truncated, err := Text(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if truncated || text != body {
		t.Fatalf("len %d truncated %v", len(text), truncated)
	}
}

func TestBadURL(t *testing.T) {
	tests := []struct {
		name, raw, msg string
	}{
		{"empty", "", "invalid url"},
		{"space", "   ", "invalid url"},
		{"no scheme", "example.com", "invalid url"},
		{"missing host", "http://", "invalid url"},
		{"ftp", "ftp://example.com/a", "url scheme must be http or https"},
		{"file", "file:///etc/passwd", "url scheme must be http or https"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := Text(context.Background(), tc.raw)
			requireStatus(t, err, http.StatusBadRequest, tc.msg)
		})
	}
}

func TestNotHTML(t *testing.T) {
	for _, tc := range []struct{ name, ct string }{
		{"json", "application/json"},
		{"bytes", "application/octet-stream"},
		{"missing", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := serve(t, http.StatusOK, tc.ct, `{"a":1}`)
			_, _, err := Text(context.Background(), srv.URL)
			requireStatus(t, err, http.StatusBadRequest, "response is not HTML or plain text")
		})
	}
}

func TestUpstreamStatus(t *testing.T) {
	srv := serve(t, http.StatusNotFound, "text/html", "missing")
	_, _, err := Text(context.Background(), srv.URL)
	requireStatus(t, err, http.StatusBadGateway, "upstream returned 404")
}

func TestRedirectLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, _ := strconv.Atoi(r.URL.Query().Get("n"))
		if n < maxRedirects {
			http.Redirect(w, r, fmt.Sprintf("/?n=%d", n+1), http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "done")
	}))
	t.Cleanup(srv.Close)

	text, _, err := Text(context.Background(), srv.URL+"/?n=0")
	if err != nil {
		t.Fatal(err)
	}
	if text != "done" {
		t.Fatalf("text %q", text)
	}

	loop := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, _ := strconv.Atoi(r.URL.Query().Get("n"))
		http.Redirect(w, r, fmt.Sprintf("/?n=%d", n+1), http.StatusFound)
	}))
	t.Cleanup(loop.Close)
	_, _, err = Text(context.Background(), loop.URL+"/?n=0")
	requireStatus(t, err, http.StatusBadGateway, "too many redirects")
}

func TestRedirectBadScheme(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "ftp://example.com/file", http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	_, _, err := Text(context.Background(), srv.URL)
	requireStatus(t, err, http.StatusBadRequest, "url scheme must be http or https")
}

func serve(t *testing.T, status int, contentType, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != userAgent {
			t.Errorf("User-Agent = %q", r.Header.Get("User-Agent"))
		}
		// Setting the key, even to an empty value, stops the server from sniffing.
		w.Header()["Content-Type"] = []string{contentType}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func requireStatus(t *testing.T, err error, status int, msg string) {
	t.Helper()
	we, ok := errors.AsType[*Error](err)
	if !ok {
		t.Fatalf("error %v, want status %d %q", err, status, msg)
	}
	if we.StatusCode != status || we.Message != msg {
		t.Fatalf("status %d message %q, want %d %q", we.StatusCode, we.Message, status, msg)
	}
}

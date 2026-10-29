package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/tericcabrel/keyphrase/extractor"
	"github.com/tericcabrel/keyphrase/internal/webtext"

	_ "github.com/tericcabrel/keyphrase/lang/all"
)

const article = "Offshore wind farms are growing. Offshore wind capacity will reach 50 gigawatts. " +
	"The Dogger Bank project uses large turbines. Offshore wind lowers energy bills."

func do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	NewHandler().ServeHTTP(rec, req)
	return rec
}

func jsonBody(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("invalid JSON response %q: %v", rec.Body.String(), err)
	}
	return v
}

func TestKeywordsOK(t *testing.T) {
	rec := do(t, http.MethodPost, "/v1/keywords", jsonBody(t, map[string]any{"text": article, "language": "en", "limit": 3}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	res := decode[response](t, rec)
	if res.Language != "en" || len(res.Keywords) == 0 || len(res.Keywords) > 3 {
		t.Fatalf("unexpected response %+v", res)
	}
	if res.Keywords[0].Text != "Offshore wind" || res.Keywords[0].Score != 1 {
		t.Errorf("top keyword = %+v", res.Keywords[0])
	}
	for _, k := range res.Keywords {
		if article[k.StartByte:k.EndByte] != k.Text {
			t.Errorf("span mismatch for %+v", k)
		}
	}
}

func TestResponseFieldNames(t *testing.T) {
	rec := do(t, http.MethodPost, "/v1/keywords", jsonBody(t, map[string]any{"text": article, "language": "en", "limit": 1}))
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	kw := raw["keywords"].([]any)[0].(map[string]any)
	for _, f := range []string{"text", "start_byte", "end_byte", "score"} {
		if _, ok := kw[f]; !ok {
			t.Errorf("missing field %q in %v", f, kw)
		}
	}
	if len(kw) != 4 || len(raw) != 2 {
		t.Errorf("unexpected extra fields: %v", raw)
	}
}

func TestDefaultLimitAndNullLimit(t *testing.T) {
	long := strings.Repeat(article+" Grid connections delay National Grid projects. Steel prices rise. ", 3)
	for _, body := range []string{
		jsonBody(t, map[string]any{"text": long, "language": "en"}),
		`{"text":` + jsonBody(t, long) + `,"language":"en","limit":null}`,
	} {
		rec := do(t, http.MethodPost, "/v1/keywords", body)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		if n := len(decode[response](t, rec).Keywords); n != extractor.DefaultLimit {
			t.Errorf("got %d keywords, want default %d", n, extractor.DefaultLimit)
		}
	}
}

func TestLimitBounds(t *testing.T) {
	for _, limit := range []int{1, 51, 1000} {
		rec := do(t, http.MethodPost, "/v1/keywords", jsonBody(t, map[string]any{"text": article, "language": "en", "limit": limit}))
		if rec.Code != http.StatusOK {
			t.Errorf("limit %d: status %d", limit, rec.Code)
		}
		if n := len(decode[response](t, rec).Keywords); n > limit {
			t.Errorf("limit %d: %d keywords", limit, n)
		}
	}
}

func TestCounts(t *testing.T) {
	for _, body := range []string{
		jsonBody(t, map[string]any{"text": article, "language": "en", "limit": 5}),
		jsonBody(t, map[string]any{"text": article, "language": "en", "limit": 5, "counts": false}),
		`{"text":` + jsonBody(t, article) + `,"language":"en","counts":null}`,
	} {
		rec := do(t, http.MethodPost, "/v1/keywords", body)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		var raw map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
			t.Fatal(err)
		}
		if _, ok := raw["counts"]; ok {
			t.Fatalf("counts present without counts=true: %s", rec.Body)
		}
	}

	rec := do(t, http.MethodPost, "/v1/keywords", jsonBody(t, map[string]any{"text": article, "language": "en", "limit": 5, "counts": true}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	res := decode[response](t, rec)
	if res.Counts == nil {
		t.Fatal("missing counts")
	}
	var returned []string
	for _, k := range res.Keywords {
		if len(strings.Fields(k.Text)) > 1 {
			returned = append(returned, k.Text)
		}
	}
	c := res.Counts
	if c.Ranked != len(res.Keywords) || c.RankedMoreThanOneWord != len(returned) || len(c.MoreThanOneWord) != c.ExtractedMoreThanOneWord {
		t.Errorf("counts = %+v, keywords %d, returned multiword %q", c, len(res.Keywords), returned)
	}
	for _, text := range returned {
		if !slices.Contains(c.MoreThanOneWord, text) {
			t.Errorf("returned %q missing from %q", text, c.MoreThanOneWord)
		}
	}
	if c.Extracted < c.Ranked || c.ExtractedMoreThanOneWord < c.RankedMoreThanOneWord || c.ExtractedMoreThanOneWord > c.Extracted {
		t.Errorf("extracted counts = %+v", c)
	}
	if c.Extracted == 0 || c.Words == 0 {
		t.Errorf("expected candidates and words, got %+v", c)
	}

	rec = do(t, http.MethodPost, "/v1/keywords", `{"text":"","language":"en","counts":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("empty: status %d: %s", rec.Code, rec.Body)
	}
	want := `{"language":"en","keywords":[],"counts":{"words":0,"extracted":0,"ranked":0,"extracted_more_than_one_word":0,"ranked_more_than_one_word":0,"more_than_one_word":[]}}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("empty counts body %s", got)
	}
}

func TestEmptyTextEveryLanguage(t *testing.T) {
	for _, l := range []string{"en", "fr", "de", "es", "it", "pt"} {
		for _, text := range []string{"", "   \n\t"} {
			rec := do(t, http.MethodPost, "/v1/keywords", jsonBody(t, map[string]any{"text": text, "language": l}))
			if rec.Code != http.StatusOK {
				t.Fatalf("%s: status %d", l, rec.Code)
			}
			want := `{"language":"` + l + `","keywords":[]}`
			if got := strings.TrimSpace(rec.Body.String()); got != want {
				t.Errorf("%s: body %s, want %s", l, got, want)
			}
		}
	}
}

func TestBadRequests(t *testing.T) {
	tests := []struct{ name, body string }{
		{"malformed JSON", `{"text":"x","language":"en"`},
		{"not an object", `["text"]`},
		{"empty body", ``},
		{"trailing data", `{"text":"x","language":"en"} {"x":1}`},
		{"missing language", `{"text":"x"}`},
		{"null language", `{"text":"x","language":null}`},
		{"unsupported language", `{"text":"x","language":"nl"}`},
		{"uppercase language", `{"text":"x","language":"EN"}`},
		{"empty language", `{"text":"x","language":""}`},
		{"missing text", `{"language":"en"}`},
		{"null text", `{"text":null,"language":"en"}`},
		{"limit zero", `{"text":"x","language":"en","limit":0}`},
		{"counts string", `{"text":"x","language":"en","counts":"yes"}`},
		{"limit negative", `{"text":"x","language":"en","limit":-3}`},
		{"limit float", `{"text":"x","language":"en","limit":2.5}`},
		{"limit string", `{"text":"x","language":"en","limit":"5"}`},
		{"limit overflow", `{"text":"x","language":"en","limit":1e40}`},
		{"text number", `{"text":5,"language":"en"}`},
		{"text array", `{"text":["a"],"language":"en"}`},
		{"language number", `{"text":"x","language":1}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, http.MethodPost, "/v1/keywords", tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status %d, want 400: %s", rec.Code, rec.Body)
			}
			if rec.Header().Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %q", rec.Header().Get("Content-Type"))
			}
			if res := decode[errorResponse](t, rec); res.Error == "" {
				t.Errorf("empty error message")
			}
		})
	}
}

func TestTextSizeLimit(t *testing.T) {
	atLimit := strings.Repeat("wind ", MaxTextBytes/5) + strings.Repeat("x", MaxTextBytes%5)
	if len(atLimit) != MaxTextBytes {
		t.Fatalf("built %d bytes", len(atLimit))
	}
	rec := do(t, http.MethodPost, "/v1/keywords", jsonBody(t, map[string]any{"text": atLimit, "language": "en"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("text at limit: status %d: %s", rec.Code, rec.Body)
	}
	rec = do(t, http.MethodPost, "/v1/keywords", jsonBody(t, map[string]any{"text": atLimit + "y", "language": "en"}))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("text over limit: status %d, want 413", rec.Code)
	}
	decode[errorResponse](t, rec)
}

func TestEscapedTextAtLimitFitsBody(t *testing.T) {
	// Every byte escapes to six ("\u0001"), the worst case for body size.
	text := strings.Repeat("\x01", MaxTextBytes)
	body := jsonBody(t, map[string]any{"text": text, "language": "en"})
	if len(body) <= 5*MaxTextBytes {
		t.Fatalf("body only %d bytes; escaping assumption wrong", len(body))
	}
	if rec := do(t, http.MethodPost, "/v1/keywords", body); rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
}

func TestBodySizeLimit(t *testing.T) {
	for _, body := range []string{
		`{"text":"x","language":"en","pad":"` + strings.Repeat("a", MaxBodyBytes) + `"}`,
		`{"text":"x","language":"en"}` + strings.Repeat(" ", MaxBodyBytes),
	} {
		rec := do(t, http.MethodPost, "/v1/keywords", body)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status %d, want 413: %s", rec.Code, rec.Body)
		}
	}
}

func TestMethodAndRoutes(t *testing.T) {
	if rec := do(t, http.MethodGet, "/v1/keywords", ""); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /v1/keywords: %d", rec.Code)
	}
	if rec := do(t, http.MethodGet, "/v1/text", ""); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /v1/text: %d", rec.Code)
	}
	if rec := do(t, http.MethodPost, "/nope", "{}"); rec.Code != http.StatusNotFound {
		t.Errorf("POST /nope: %d", rec.Code)
	}
	rec := do(t, http.MethodGet, "/healthz", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz: %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` {
		t.Errorf("healthz body %q", rec.Body)
	}
}

func TestExactSpansThroughJSON(t *testing.T) {
	text := "L’économie de l'Europe: «Banque centrale européenne», Banque centrale européenne. 🙂 Zürich"
	rec := do(t, http.MethodPost, "/v1/keywords", jsonBody(t, map[string]any{"text": text, "language": "fr", "limit": 20}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	res := decode[response](t, rec)
	if len(res.Keywords) == 0 {
		t.Fatal("no keywords")
	}
	for _, k := range res.Keywords {
		if text[k.StartByte:k.EndByte] != k.Text {
			t.Errorf("span mismatch %+v", k)
		}
	}
}

func TestTextOK(t *testing.T) {
	const page = `<title>Wind</title><p>Offshore wind farms grow.</p>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page))
	}))
	defer srv.Close()

	rec := do(t, http.MethodPost, "/v1/text", jsonBody(t, map[string]any{"url": srv.URL}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	res := decode[textResponse](t, rec)
	if res.Truncated || res.Text != "Wind\nOffshore wind farms grow." {
		t.Fatalf("response %+v", res)
	}
	kw := do(t, http.MethodPost, "/v1/keywords", jsonBody(t, map[string]any{"text": res.Text, "language": "en"}))
	if kw.Code != http.StatusOK {
		t.Fatalf("keywords status %d: %s", kw.Code, kw.Body)
	}
}

func TestTextFieldNames(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()

	rec := do(t, http.MethodPost, "/v1/text", jsonBody(t, map[string]any{"url": srv.URL}))
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if raw["text"] != "hello" || raw["truncated"] != false || len(raw) != 2 {
		t.Fatalf("body %s", rec.Body)
	}
}

func TestTextBadRequests(t *testing.T) {
	tests := []struct{ name, body string }{
		{"malformed JSON", `{"url":"http://example.com"`},
		{"not an object", `["http://example.com"]`},
		{"empty body", ``},
		{"trailing data", `{"url":"http://example.com"} {"x":1}`},
		{"missing url", `{}`},
		{"null url", `{"url":null}`},
		{"url number", `{"url":5}`},
		{"empty url", `{"url":""}`},
		{"ftp", `{"url":"ftp://example.com/a"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, http.MethodPost, "/v1/text", tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status %d, want 400: %s", rec.Code, rec.Body)
			}
			if res := decode[errorResponse](t, rec); res.Error == "" {
				t.Errorf("empty error message")
			}
		})
	}
}

func TestTextUpstream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	rec := do(t, http.MethodPost, "/v1/text", jsonBody(t, map[string]any{"url": srv.URL}))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if res := decode[errorResponse](t, rec); res.Error != "upstream returned 503" {
		t.Fatalf("error %q", res.Error)
	}
}

func TestTextBodyLimit(t *testing.T) {
	body := `{"url":"http://example.com","pad":"` + strings.Repeat("a", maxURLBodyBytes) + `"}`
	rec := do(t, http.MethodPost, "/v1/text", body)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d, want 413: %s", rec.Code, rec.Body)
	}
}

func TestTextCapMatchesKeywordLimit(t *testing.T) {
	if webtext.MaxTextBytes != MaxTextBytes {
		t.Fatalf("webtext cap %d, keyword cap %d", webtext.MaxTextBytes, MaxTextBytes)
	}
}

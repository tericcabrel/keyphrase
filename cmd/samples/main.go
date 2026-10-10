// Command samples sends every <dir>/<lang>/*.txt or <dir>/<lang>.txt
// file to the development HTTP server and prints the responses as Markdown
// for the snapshots in quality/.
//
// By default it starts the server in-process on a loopback port, so the
// output is what the HTTP API returns. Use -url to target a running server.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tericcabrel/keyphrase/extractor"
	"github.com/tericcabrel/keyphrase/internal/httpapi"

	_ "github.com/tericcabrel/keyphrase/lang/all"
)

type result struct {
	Language string              `json:"language"`
	Keywords []extractor.Keyword `json:"keywords"`
}

func main() {
	dir := flag.String("dir", "examples", "examples directory")
	url := flag.String("url", "", "base URL of a running server (default: in-process)")
	limit := flag.Int("limit", 5, "keywords per document")
	flag.Parse()

	base := *url
	if base == "" {
		srv := httptest.NewServer(httpapi.NewHandler())
		defer srv.Close()
		base = srv.URL
	}

	nested, _ := filepath.Glob(filepath.Join(*dir, "*", "*.txt"))
	flat, _ := filepath.Glob(filepath.Join(*dir, "*.txt"))
	files := slices.Sorted(slices.Values(append(nested, flat...)))
	if len(files) == 0 {
		log.Fatalf("no .txt files under %s", *dir)
	}
	for _, f := range files {
		language := filepath.Base(filepath.Dir(f))
		if filepath.Dir(f) == filepath.Clean(*dir) {
			language = strings.TrimSuffix(filepath.Base(f), ".txt")
		}
		text, err := os.ReadFile(f)
		if err != nil {
			log.Fatal(err)
		}
		res, err := post(base, string(text), language, *limit)
		if err != nil {
			log.Fatalf("%s: %v", f, err)
		}
		fmt.Printf("#### `%s`\n\n", filepath.ToSlash(f))
		fmt.Println("| # | score | text | bytes |")
		fmt.Println("|---|---|---|---|")
		for i, k := range res.Keywords {
			if string(text[k.StartByte:k.EndByte]) != k.Text {
				log.Fatalf("%s: span mismatch for %q", f, k.Text)
			}
			fmt.Printf("| %d | %.4f | %s | %d-%d |\n", i+1, k.Score, strings.ReplaceAll(k.Text, "|", `\|`), k.StartByte, k.EndByte)
		}
		fmt.Println()
	}
}

func post(base, text, language string, limit int) (result, error) {
	body, _ := json.Marshal(map[string]any{"text": text, "language": language, "limit": limit})
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, base+"/v1/keywords", bytes.NewReader(body))
	if err != nil {
		return result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return result{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return result{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return result{}, fmt.Errorf("status %d: %s", resp.StatusCode, raw)
	}
	var r result
	return r, json.Unmarshal(raw, &r)
}

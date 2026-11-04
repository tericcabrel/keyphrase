// Command samples prints extractor responses for the files under a directory as Markdown.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/tericcabrel/keyphrase/extractor"

	_ "github.com/tericcabrel/keyphrase/lang/all"
)

type result struct {
	Language string              `json:"language"`
	Keywords []extractor.Keyword `json:"keywords"`
}

func main() {
	examplesDirectory := flag.String("dir", "examples", "examples directory")
	url := flag.String("url", "", "base URL of a running server (default: start a temporary server)")
	limit := flag.Int("limit", extractor.DefaultLimit, "keywords per document")
	flag.Parse()

	base, stop, err := startServer(*url)
	if err != nil {
		log.Fatal(err)
	}
	defer stop()

	nested, _ := filepath.Glob(filepath.Join(*examplesDirectory, "*", "*.txt"))
	flat, _ := filepath.Glob(filepath.Join(*examplesDirectory, "*.txt"))
	files := slices.Sorted(slices.Values(append(nested, flat...)))
	if len(files) == 0 {
		log.Fatalf("no .txt files under %s", *examplesDirectory)
	}
	for _, f := range files {
		language := filepath.Base(filepath.Dir(f))
		if filepath.Dir(f) == filepath.Clean(*examplesDirectory) {
			language = strings.TrimSuffix(filepath.Base(f), ".txt")
		}
		text, err := os.ReadFile(f)
		if err != nil {
			log.Fatal(err)
		}
		response, err := post(base, string(text), language, *limit)
		if err != nil {
			log.Fatalf("%s: %v", f, err)
		}
		fmt.Printf("#### `%s`\n\n", filepath.ToSlash(f))
		fmt.Println("| # | score | text | bytes |")
		fmt.Println("|---|---|---|---|")
		for i, k := range response.Keywords {
			if string(text[k.StartByte:k.EndByte]) != k.Text {
				log.Fatalf("%s: span mismatch for %q", f, k.Text)
			}
			fmt.Printf("| %d | %.4f | %s | %d-%d |\n", i+1, k.Score, strings.ReplaceAll(k.Text, "|", `\|`), k.StartByte, k.EndByte)
		}
		fmt.Println()
	}
}

func startServer(base string) (string, func(), error) {
	if base != "" {
		return base, func() {}, nil
	}
	addr, err := freeAddr()
	if err != nil {
		return "", nil, err
	}
	binary, err := os.CreateTemp("", "keyphrase-server-*")
	if err != nil {
		return "", nil, err
	}
	path := binary.Name()
	_ = binary.Close()
	build := exec.CommandContext(context.Background(), "go", "build", "-o", path, "./cmd/server")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		_ = os.Remove(path)
		return "", nil, err
	}
	command := exec.CommandContext(context.Background(), path, "-addr", addr)
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		_ = os.Remove(path)
		return "", nil, err
	}
	client := &http.Client{Timeout: 2 * time.Second}
	waitURL := "http://" + addr
	if err := waitHealthy(client, waitURL); err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		_ = os.Remove(path)
		return "", nil, err
	}
	return waitURL, func() {
		_ = command.Process.Kill()
		_ = command.Wait()
		_ = os.Remove(path)
	}, nil
}

func freeAddr() (string, error) {
	var listenConfig net.ListenConfig
	instance, err := listenConfig.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer func() { _ = instance.Close() }()
	return instance.Addr().String(), nil
}

func waitHealthy(client *http.Client, base string) error {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, base+"/healthz", nil)
		if err != nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		response, err := client.Do(request)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("server at %s did not become healthy", base)
}

func post(base, text, language string, limit int) (result, error) {
	body, _ := json.Marshal(map[string]any{"text": text, "language": language, "limit": limit, "counts": false})
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, base+"/v1/keywords", bytes.NewReader(body))
	if err != nil {
		return result{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return result{}, err
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return result{}, err
	}
	if response.StatusCode != http.StatusOK {
		return result{}, fmt.Errorf("status %d: %s", response.StatusCode, raw)
	}
	var r result
	return r, json.Unmarshal(raw, &r)
}

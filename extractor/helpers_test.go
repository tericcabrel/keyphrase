package extractor

import (
	"os"
	"path/filepath"
	"testing"
)

func mustRead(tb testing.TB, path string) string {
	tb.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		tb.Fatal(err)
	}
	return string(raw)
}

func langOf(path string) string {
	return filepath.Base(filepath.Dir(path))
}

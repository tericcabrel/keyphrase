package extractor

import (
	"path/filepath"
	"strings"
	"testing"
)

// BenchmarkExtract measures the core on the ~2,000-word benchmark articles.
// Run with: GOMAXPROCS=1 go test ./extractor -run '^$' -bench Extract -benchmem
func BenchmarkExtract(b *testing.B) {
	files, err := filepath.Glob(filepath.Join("..", "examples", "*", "wiki-*.txt"))
	if err != nil || len(files) == 0 {
		b.Fatalf("no benchmark articles: %v", err)
	}
	for _, f := range files {
		text := mustRead(b, f)
		language := filepath.Base(filepath.Dir(f))
		b.Run(language, func(b *testing.B) { benchExtract(b, text, language) })
	}
}

// BenchmarkExtractMaxInput measures the worst accepted input size: 64 KiB
// (about 9,500 English words), built by repeating the English article.
func BenchmarkExtractMaxInput(b *testing.B) {
	article := mustRead(b, filepath.Join("..", "examples", "en", "wiki-wind-power.txt"))
	text := strings.Repeat(article, (64<<10)/len(article)+1)
	text = strings.ToValidUTF8(text[:64<<10], "")
	benchExtract(b, text, "en")
}

func benchExtract(b *testing.B, text, language string) {
	b.Helper()
	b.ReportAllocs()
	b.SetBytes(int64(len(text)))
	for b.Loop() {
		if _, err := Extract(text, language, DefaultLimit); err != nil {
			b.Fatal(err)
		}
	}
}

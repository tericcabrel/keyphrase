package extractor

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

var languages = []string{"de", "en", "es", "fr", "it"}

// checkInvariants verifies the output contract for one Extract call.
func checkInvariants(t *testing.T, text string, limit int, kws []Keyword) {
	t.Helper()
	if kws == nil {
		t.Fatal("keywords must be a non-nil slice")
	}
	if len(kws) > limit {
		t.Fatalf("got %d keywords, limit %d", len(kws), limit)
	}
	for i, k := range kws {
		if k.StartByte < 0 || k.EndByte > len(text) || k.StartByte >= k.EndByte {
			t.Fatalf("keyword %d: bad offsets %d-%d for text of %d bytes", i, k.StartByte, k.EndByte, len(text))
		}
		if got := text[k.StartByte:k.EndByte]; got != k.Text {
			t.Fatalf("keyword %d: text[%d:%d] = %q, want %q", i, k.StartByte, k.EndByte, got, k.Text)
		}
		if !utf8.ValidString(k.Text) && utf8.ValidString(text) {
			t.Fatalf("keyword %d: %q splits a UTF-8 sequence", i, k.Text)
		}
		if strings.TrimSpace(k.Text) != k.Text {
			t.Fatalf("keyword %d: %q has surrounding whitespace", i, k.Text)
		}
		if n := len(strings.Fields(k.Text)); n > maxWords {
			t.Fatalf("keyword %d: %q has %d words, max %d", i, k.Text, n, maxWords)
		}
		if k.Score < 0 || k.Score > 1 {
			t.Fatalf("keyword %d: score %v outside [0, 1]", i, k.Score)
		}
		for j := range i {
			o := kws[j]
			if k.StartByte < o.EndByte && o.StartByte < k.EndByte {
				t.Fatalf("keywords %q and %q overlap", o.Text, k.Text)
			}
		}
	}
	if !slices.IsSortedFunc(kws, compareKeywords) {
		t.Fatalf("keywords not in score/offset/text order: %+v", kws)
	}
	if len(kws) > 0 && kws[0].Score != 1 {
		t.Fatalf("top score = %v, want 1", kws[0].Score)
	}
}

func exampleFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "examples", "*", "*.txt"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no example files found: %v", err)
	}
	return files
}

func TestExamplesSatisfyContract(t *testing.T) {
	perLang := map[string]int{}
	for _, f := range exampleFiles(t) {
		language := filepath.Base(filepath.Dir(f))
		perLang[language]++
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(filepath.Base(f), func(t *testing.T) {
			for _, limit := range []int{1, 5, 1000} {
				kws, err := Extract(string(raw), language, limit)
				if err != nil {
					t.Fatal(err)
				}
				checkInvariants(t, string(raw), limit, kws)
				// Lone common nouns are not keywords, so some articles no
				// longer reach 25. Small limits must still be filled, and a
				// limit of 1000 returns every keyword the article has.
				if want := min(limit, 19); len(kws) < want {
					t.Errorf("limit %d: got only %d keywords from a full article, want at least %d", limit, len(kws), want)
				}
			}
		})
	}
	for _, l := range languages {
		if perLang[l] < 2 {
			t.Errorf("language %s has %d example files, want at least 2", l, perLang[l])
		}
	}
}

// TestRandomTextSatisfiesContract feeds deterministic pseudo-random text
// mixing accents, combining marks, curly apostrophes, emoji, CJK, digits,
// hyphens and stray punctuation.
func TestRandomTextSatisfiesContract(t *testing.T) {
	pool := []string{
		"économie", "l’Europe", "d'Italia", "Straße", "Bundesverfassungsgericht", "e-mail",
		"Ärzte", "niño", "perché", "company's", "U.S.", "Dr.", "z. B.", "3,5", "2024",
		"日本語", "🙂", "e\u0301te\u0301", "ﬁnance", "İstanbul", "—", "«", "»", "(", ")",
		",", ".", "!", "?", ";", "\n", "\n\n", "\t", "  ", "de", "la", "of", "the", "und",
		"Santa", "Cruz", "Banca", "Central", "Bank", "\u00a0", "'", "’", "-", "l'", "x",
	}
	r := rand.New(rand.NewPCG(1, 2))
	for i := range 300 {
		var b strings.Builder
		for range r.IntN(200) {
			b.WriteString(pool[r.IntN(len(pool))])
			if r.IntN(3) > 0 {
				b.WriteByte(' ')
			}
		}
		text := b.String()
		language := languages[i%len(languages)]
		limit := 1 + r.IntN(50)
		kws, err := Extract(text, language, limit)
		if err != nil {
			t.Fatal(err)
		}
		checkInvariants(t, text, limit, kws)
	}
}

func FuzzExtract(f *testing.F) {
	f.Add("L'économie de l’Europe. Dr. Smith said the company's CEO arrived.", "fr", 5)
	f.Add("Die Bundesregierung plant z. B. neue Regeln.", "de", 3)
	f.Add("\xff\xfe invalid UTF-8 Banca d'Italia", "it", 20)
	f.Fuzz(func(t *testing.T, text, language string, limit int) {
		kws, err := Extract(text, language, limit)
		if err != nil {
			return
		}
		checkInvariants(t, text, limit, kws)
	})
}

func TestTextAtInputLimit(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "examples", "fr", "tgv-sncf.txt"))
	if err != nil {
		t.Fatal(err)
	}
	const size = 64 << 10
	var b strings.Builder
	for b.Len() < size {
		b.Write(raw)
	}
	text := b.String()[:size]
	for !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}
	text += strings.Repeat(" ", size-len(text))
	if len(text) != size {
		t.Fatalf("built %d bytes, want %d", len(text), size)
	}
	kws, err := Extract(text, "fr", 1000)
	if err != nil {
		t.Fatal(err)
	}
	checkInvariants(t, text, 1000, kws)
	if len(kws) == 0 {
		t.Fatal("no keywords from 64 KiB of text")
	}
}

package evalset

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestHoldoutList(t *testing.T) {
	dir := t.TempDir()
	got, err := ReadHoldout(dir)
	if err != nil || len(got) != 0 {
		t.Fatalf("missing list: %v, %v", got, err)
	}

	added, err := AddHoldout(dir, []string{"fr/a", "de/b", "fr/a"})
	if err != nil || !slices.Equal(added, []string{"fr/a", "de/b"}) {
		t.Fatalf("first add: %v, %v", added, err)
	}
	added, err = AddHoldout(dir, []string{"de/b", "it/c"})
	if err != nil || !slices.Equal(added, []string{"it/c"}) {
		t.Fatalf("second add: %v, %v", added, err)
	}
	if added, addErr := AddHoldout(dir, []string{"fr/a"}); addErr != nil || added != nil {
		t.Fatalf("nothing new: %v, %v", added, err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, HoldoutFile))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 4 || !strings.HasPrefix(lines[0], "# ") || strings.Join(lines[1:], " ") != "fr/a de/b it/c" {
		t.Fatalf("list = %q", raw)
	}
	got, err = ReadHoldout(dir)
	if err != nil || len(got) != 3 || !got["fr/a"] || !got["de/b"] || !got["it/c"] {
		t.Fatalf("read back: %v, %v", got, err)
	}
}

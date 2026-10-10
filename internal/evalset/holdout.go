// Package evalset reads and writes the list of held-out evaluation pages.
//
// Held-out pages are labelled like the others but are never looked at while
// tuning: eval scores them apart and never prints their keywords. The
// list is <dir>/holdout.txt, one "<lang>/<name>" page per line; blank lines
// and lines starting with # are ignored.
package evalset

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// HoldoutFile is the list's name inside the data directory.
const HoldoutFile = "holdout.txt"

// ReadHoldout returns the held-out page names; a missing list has none.
func ReadHoldout(dir string) (map[string]bool, error) {
	file, err := os.Open(filepath.Join(dir, HoldoutFile))
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	out := map[string]bool{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			out[line] = true
		}
	}
	return out, scanner.Err()
}

// AddHoldout appends the names that aren't listed yet, creating the list
// with a header if needed. It returns the names it added.
func AddHoldout(dir string, names []string) ([]string, error) {
	existing, err := ReadHoldout(dir)
	if err != nil {
		return nil, err
	}
	var builder strings.Builder
	var added []string
	for _, name := range names {
		if existing[name] {
			continue
		}
		existing[name] = true
		added = append(added, name)
		builder.WriteString(name)
		builder.WriteString("\n")
	}
	if len(added) == 0 {
		return nil, nil
	}
	path := filepath.Join(dir, HoldoutFile)
	if _, statErr := os.Stat(path); errors.Is(statErr, fs.ErrNotExist) {
		header := "# Held-out pages: scored apart by eval, keywords never printed. Don't tune on them.\n"
		if writeErr := os.WriteFile(path, []byte(header), 0o644); writeErr != nil {
			return nil, writeErr
		}
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	if _, writeErr := file.WriteString(builder.String()); writeErr != nil {
		_ = file.Close()

		return nil, writeErr
	}
	return added, file.Close()
}

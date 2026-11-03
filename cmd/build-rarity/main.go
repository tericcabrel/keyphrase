// Command build-rarity counts sentences per word and writes a rarity table.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/tericcabrel/keyphrase/extractor"
	"github.com/tericcabrel/keyphrase/rarity"

	_ "github.com/tericcabrel/keyphrase/lang/all"
)

func main() {
	language := flag.String("lang", "", "language code (en, fr, de, es, it, pt)")
	out := flag.String("out", "", "table output path (.bin.gz)")
	minCount := flag.Int("min", rarity.MinCount, "minimum sentences a word must appear in")
	flag.Parse()
	if *language == "" || *out == "" || flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	log.SetFlags(0)

	df := map[string]int{}
	seen := map[string]struct{}{}
	sentences := 0
	for _, path := range flag.Args() {
		n, err := countFile(path, *language, df, seen)
		if err != nil {
			log.Fatal(err)
		}
		sentences += n
		log.Printf("%s: %d sentences, %d distinct words so far", path, n, len(df))
	}
	table, err := rarity.Build(sentences, df, *minCount)
	if err != nil {
		log.Fatal(err)
	}
	f, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	err = table.Write(f)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		log.Fatal(err)
	}
	info, err := os.Stat(*out)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%s: %d sentences, %d words, %d bytes", *out, table.Sentences(), table.Words(), info.Size())
}

func countFile(path, language string, df map[string]int, seen map[string]struct{}) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	n := 0
	for sc.Scan() {
		line := sc.Text()
		_, sent, ok := strings.Cut(line, "\t")
		if !ok {
			sent = line
		}
		norms, err := extractor.Norms(sent, language)
		if err != nil {
			return n, fmt.Errorf("%s: %w", path, err)
		}
		for _, w := range norms {
			if _, ok := seen[w]; ok {
				continue
			}
			seen[w] = struct{}{}
			df[w]++
		}
		clear(seen)
		n++
		if n%200000 == 0 {
			log.Printf("%s: %d sentences", path, n)
		}
	}
	if err := sc.Err(); err != nil {
		return n, fmt.Errorf("%s: %w", path, err)
	}
	return n, nil
}

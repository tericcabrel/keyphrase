package main

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

const numChecks = len(defectChecks)

type defectStats struct {
	Docs              int
	Returned          [numBands]int
	Any               [numBands]int
	Flagged           [numChecks][numBands]int
	Labelled, NotGood [numChecks]int // judged flags, and how many of those are A or I
}

func (s *defectStats) add(o defectStats) {
	s.Docs += o.Docs
	for band := range numBands {
		s.Returned[band] += o.Returned[band]
		s.Any[band] += o.Any[band]
	}
	for check := range numChecks {
		for band := range numBands {
			s.Flagged[check][band] += o.Flagged[check][band]
		}
		s.Labelled[check] += o.Labelled[check]
		s.NotGood[check] += o.NotGood[check]
	}
}

func docDefects(d doc, l labels) defectStats {
	stats := defectStats{Docs: 1}
	flagged := make([][]int, len(d.Keywords))
	for check, name := range defectChecks {
		for _, rank := range d.Defects[name] {
			if rank < len(flagged) {
				flagged[rank] = append(flagged[rank], check)
			}
		}
	}
	for rank, keyword := range d.Keywords {
		band := bandOf(rank)
		stats.Returned[band]++
		if len(flagged[rank]) > 0 {
			stats.Any[band]++
		}
		mark := markOf(l, keyword)
		for _, check := range flagged[rank] {
			stats.Flagged[check][band]++
			if mark != markUnjudged {
				stats.Labelled[check]++
				if mark != markGood {
					stats.NotGood[check]++
				}
			}
		}
	}
	return stats
}

func defectTotals(docs []doc, all map[string]labels) map[string]defectStats {
	perLang := map[string]defectStats{}
	for _, d := range docs {
		stats := docDefects(d, all[d.Name])
		for _, key := range []string{d.Language, "all"} {
			total := perLang[key]
			total.add(stats)
			perLang[key] = total
		}
	}
	return perLang
}

func defectReport(w io.Writer, checks []string, docs []doc, all map[string]labels) {
	if len(docs) == 0 {
		return
	}
	total := defectTotals(docs, all)["all"]
	_, _ = fmt.Fprintf(w, "%-20s", "defect check")
	for band := range numBands {
		_, _ = fmt.Fprintf(w, " %8s", bandName(band))
	}
	_, _ = fmt.Fprintf(w, "  %s\n", "labelled flags not good")
	for check, name := range defectChecks {
		if !slices.Contains(checks, name) {
			continue
		}
		_, _ = fmt.Fprintf(w, "%-20s", name)
		for band := range numBands {
			_, _ = fmt.Fprintf(w, " %8s", rate(total.Flagged[check][band], total.Returned[band]))
		}
		_, _ = fmt.Fprintf(w, "  %d/%d %s\n", total.NotGood[check], total.Labelled[check],
			strings.TrimSpace(pct(ratio(total.NotGood[check], total.Labelled[check]))))
	}
	_, _ = fmt.Fprintf(w, "%-20s", "any")
	for band := range numBands {
		_, _ = fmt.Fprintf(w, " %8s", rate(total.Any[band], total.Returned[band]))
	}
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Each band: share of the band's keywords the check flags.")
	_, _ = fmt.Fprintln(w)
	defectLanguageTable(w, docs, all)
}

func defectLanguageTable(w io.Writer, docs []doc, all map[string]labels) {
	perLang := defectTotals(docs, all)
	_, _ = fmt.Fprintf(w, "%-20s", "any defect")
	for band := range numBands {
		_, _ = fmt.Fprintf(w, " %8s", bandName(band))
	}
	_, _ = fmt.Fprintln(w)
	langs := slices.DeleteFunc(sortedKeys(perLang), func(l string) bool { return l == "all" })
	for _, l := range append(langs, "all") {
		stats := perLang[l]
		_, _ = fmt.Fprintf(w, "%-20s", fmt.Sprintf("%s (%d docs)", l, stats.Docs))
		for band := range numBands {
			_, _ = fmt.Fprintf(w, " %8s", rate(stats.Any[band], stats.Returned[band]))
		}
		_, _ = fmt.Fprintln(w)
	}
}

func rate(part, whole int) string {
	if whole == 0 {
		return "-"
	}
	return strings.TrimSpace(pct(ratio(part, whole)))
}

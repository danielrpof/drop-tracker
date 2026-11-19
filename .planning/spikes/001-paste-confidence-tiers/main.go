// Spike 001 (G-27-3): measure Phase 27 D-02 High rate on a realistic paste,
// and compare candidate score-based revisions. Run from the repo root:
//
//	go run ./.planning/spikes/001-paste-confidence-tiers
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danielrpof/drop-tracker/internal/musicbrainz"
	"golang.org/x/time/rate"
)

const dir = ".planning/spikes/001-paste-confidence-tiers"

// Copied from internal/artistart/match.go (unexported there).
var folds = map[rune]rune{
	'à': 'a', 'á': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a', 'å': 'a', 'À': 'A', 'Á': 'A', 'Â': 'A', 'Ã': 'A', 'Ä': 'A', 'Å': 'A',
	'è': 'e', 'é': 'e', 'ê': 'e', 'ë': 'e', 'È': 'E', 'É': 'E', 'Ê': 'E', 'Ë': 'E',
	'ì': 'i', 'í': 'i', 'î': 'i', 'ï': 'i', 'Ì': 'I', 'Í': 'I', 'Î': 'I', 'Ï': 'I',
	'ò': 'o', 'ó': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o', 'Ò': 'O', 'Ó': 'O', 'Ô': 'O', 'Õ': 'O', 'Ö': 'O',
	'ù': 'u', 'ú': 'u', 'û': 'u', 'ü': 'u', 'Ù': 'U', 'Ú': 'U', 'Û': 'U', 'Ü': 'U',
	'ñ': 'n', 'Ñ': 'N', 'ç': 'c', 'Ç': 'C', 'ý': 'y', 'ÿ': 'y', 'Ý': 'Y',
}

func normalize(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "’", "'")
	var b strings.Builder
	for _, r := range s {
		if f, ok := folds[r]; ok {
			r = f
		}
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(strings.ToLower(b.String())), " ")
}

type Exact struct {
	Rank           int    `json:"rank"`
	Score          int    `json:"score"`
	MBID           string `json:"mbid"`
	Disambiguation string `json:"disambiguation"`
	Country        string `json:"country"`
	Type           string `json:"type"`
}

type Row struct {
	Line    string               `json:"line"`
	Error   string               `json:"error,omitempty"`
	Results []musicbrainz.Artist `json:"results"`
	Exacts  []Exact              `json:"exacts"`
	Tiers   map[string]string    `json:"tiers"`
}

// Each rule returns High/Medium/NotFound for one row.
var rules = []struct {
	key, desc string
	fn        func(r Row) string
}{
	{"D02", "unique exact name among the 10 results (current D-02)", func(r Row) string {
		return base(r, len(r.Exacts) == 1)
	}},
	{"S100", "unique exact name among results scoring 100", func(r Row) string {
		n := 0
		for _, e := range r.Exacts {
			if e.Score == 100 {
				n++
			}
		}
		return base(r, n == 1)
	}},
	{"TOP", "top result is exact; any other exact scores ≥10 lower", func(r Row) string {
		if len(r.Exacts) == 0 || r.Exacts[0].Rank != 1 {
			return base(r, false)
		}
		for _, e := range r.Exacts[1:] {
			if e.Score > r.Exacts[0].Score-10 {
				return base(r, false)
			}
		}
		return base(r, true)
	}},
	{"TOPANY", "top result is exact (ignores other exacts)", func(r Row) string {
		return base(r, len(r.Exacts) > 0 && r.Exacts[0].Rank == 1)
	}},
}

func base(r Row, high bool) string {
	switch {
	case r.Error != "":
		return "Error"
	case len(r.Results) == 0:
		return "NotFound"
	case high:
		return "High"
	default:
		return "Medium"
	}
}

func main() {
	rps := flag.Float64("rps", 1, "MusicBrainz requests per second (production default is 1)")
	tag := flag.String("out", "results", "output file stem")
	ua := flag.String("ua", "paste-confidence-spike/0.1 ( local one-off measurement )", "User-Agent sent to MusicBrainz")
	flag.Parse()
	f, err := os.Open(filepath.Join(dir, "names.txt"))
	must(err)
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); l != "" {
			lines = append(lines, l)
		}
	}
	_ = f.Close()

	c := musicbrainz.NewClient(*ua, rate.NewLimiter(rate.Limit(*rps), 1), nil)
	ctx := context.Background()

	var rows []Row
	for i, l := range lines {
		r := Row{Line: l, Tiers: map[string]string{}}
		res, err := c.SearchArtists(ctx, l, 10)
		if err != nil {
			r.Error = err.Error()
		}
		r.Results = res
		want := normalize(l)
		for k, a := range res {
			if normalize(a.Name) == want {
				r.Exacts = append(r.Exacts, Exact{k + 1, a.Score, a.MBID, a.Disambiguation, a.Country, a.Type})
			}
		}
		for _, ru := range rules {
			r.Tiers[ru.key] = ru.fn(r)
		}
		fmt.Fprintf(os.Stderr, "%2d/%d %-22s exacts=%d D02=%s\n", i+1, len(lines), l, len(r.Exacts), r.Tiers["D02"])
		rows = append(rows, r)
	}

	out, _ := json.MarshalIndent(rows, "", "  ")
	must(os.WriteFile(filepath.Join(dir, *tag+".json"), out, 0o644))

	var md strings.Builder
	md.WriteString("| # | Line | Top result (score · disambiguation) | Exact matches (rank:score) |")
	for _, ru := range rules {
		md.WriteString(" " + ru.key + " |")
	}
	md.WriteString("\n|---|---|---|---|" + strings.Repeat("---|", len(rules)) + "\n")
	for i, r := range rows {
		top := "—"
		if len(r.Results) > 0 {
			t := r.Results[0]
			top = fmt.Sprintf("%s (%d · %s)", t.Name, t.Score, t.Disambiguation)
		}
		var ex []string
		for _, e := range r.Exacts {
			ex = append(ex, fmt.Sprintf("%d:%d %s", e.Rank, e.Score, e.Disambiguation))
		}
		fmt.Fprintf(&md, "| %d | %s | %s | %s |", i+1, r.Line, esc(top), esc(strings.Join(ex, "; ")))
		for _, ru := range rules {
			md.WriteString(" " + r.Tiers[ru.key] + " |")
		}
		md.WriteString("\n")
	}
	md.WriteString("\n| Rule | Meaning | High | Medium | Not found | Error |\n|---|---|---|---|---|---|\n")
	for _, ru := range rules {
		cnt := map[string]int{}
		for _, r := range rows {
			cnt[r.Tiers[ru.key]]++
		}
		fmt.Fprintf(&md, "| %s | %s | %d (%d%%) | %d | %d | %d |\n", ru.key, ru.desc,
			cnt["High"], 100*cnt["High"]/len(rows), cnt["Medium"], cnt["NotFound"], cnt["Error"])
	}
	must(os.WriteFile(filepath.Join(dir, *tag+".md"), []byte(md.String()), 0o644))
	fmt.Print(md.String())
}

func esc(s string) string { return strings.ReplaceAll(s, "|", "\\|") }

func must(err error) {
	if err != nil {
		panic(err)
	}
}

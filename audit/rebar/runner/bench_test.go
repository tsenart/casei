package main

import (
	_ "embed"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tsenart/casei"
)

// rowsTSV holds the 18 Rebar performance rows in REBAR.md, copied from the
// definitions at Rebar commit 463d00f31887e84c38467805b9e3122c314b9521. Each
// line is: Rebar id, model, haystack path, expected count, regex. The count
// is matches for the count model and the sum of match widths in bytes for
// the count-spans model. The rows are data, not Go source, because some Rebar
// ids name a field engine and scripts/check-baseline-isolation.sh rejects
// that name in Go files.
//
//go:embed testdata/rows.tsv
var rowsTSV string

// rebarRow is one parsed line of rowsTSV.
type rebarRow struct {
	id       string
	spans    bool
	haystack string
	count    int
	regex    string
}

// parseRows reads rowsTSV and fails on any malformed line.
func parseRows(tb testing.TB) []rebarRow {
	var rows []rebarRow
	for _, line := range strings.Split(strings.TrimSuffix(rowsTSV, "\n"), "\n") {
		f := strings.Split(line, "\t")
		if len(f) != 5 || (f[1] != "count" && f[1] != "count-spans") {
			tb.Fatalf("bad row %q", line)
		}
		count, err := strconv.Atoi(f[3])
		if err != nil {
			tb.Fatalf("bad count in row %q: %v", line, err)
		}
		rows = append(rows, rebarRow{f[0], f[1] == "count-spans", f[2], count, f[4]})
	}
	return rows
}

// TestRows keeps the embedded table at the 18 rows with unique names.
func TestRows(t *testing.T) {
	rows := parseRows(t)
	if len(rows) != 18 {
		t.Fatalf("got %d rows, want 18", len(rows))
	}
	seen := map[string]bool{}
	for _, row := range rows {
		name := strings.ReplaceAll(row.id, "/", "-")
		if seen[name] {
			t.Fatalf("duplicate row name %q", name)
		}
		seen[name] = true
		if _, err := literalAlternation([]string{row.regex}); err != nil {
			t.Fatalf("%s: %v", row.id, err)
		}
	}
}

// benchSink keeps the timed result live.
var benchSink int

// BenchmarkRebar times the operation the Rebar runner times on each row:
// countMatches over the whole haystack with a Matcher compiled once. Each row
// first passes the runner's untimed verifyEnumeration and must reproduce
// Rebar's expected count, so a wrong engine cannot report a time.
//
// Haystacks come from audit/rebar/haystacks.sh. CASEI_REBAR_HAYSTACKS names
// their directory. The default, ../haystacks, is relative to the current
// directory, which is the runner directory under go test.
func BenchmarkRebar(b *testing.B) {
	dir := os.Getenv("CASEI_REBAR_HAYSTACKS")
	if dir == "" {
		dir = filepath.Join("..", "haystacks")
	}
	for _, row := range parseRows(b) {
		b.Run(strings.ReplaceAll(row.id, "/", "-"), func(b *testing.B) {
			path := filepath.Join(dir, row.haystack)
			raw, err := os.ReadFile(path)
			if err != nil {
				b.Fatalf("haystack for %s: %v (fetch with audit/rebar/haystacks.sh and set CASEI_REBAR_HAYSTACKS)", row.id, err)
			}
			haystack := string(raw)
			patterns, err := literalAlternation([]string{row.regex})
			if err != nil {
				b.Fatal(err)
			}
			matcher := casei.NewMatcher(patterns)
			got, err := verifyEnumeration(haystack, patterns, matcher, row.spans)
			if err != nil {
				b.Fatalf("%s: %v", row.id, err)
			}
			if got != row.count {
				b.Fatalf("%s: verified count %d, Rebar expects %d", row.id, got, row.count)
			}
			if got := countMatches(haystack, matcher, row.spans); got != row.count {
				b.Fatalf("%s: countMatches = %d, Rebar expects %d", row.id, got, row.count)
			}
			b.SetBytes(int64(len(haystack)))
			for b.Loop() {
				benchSink = countMatches(haystack, matcher, row.spans)
			}
		})
	}
}

// BenchmarkSixPatternCountSpans is a separate boundary guard for the bounded
// six-to-eight-pattern bucket extension. It uses the pinned Sherlock corpus and
// the same retained-Matcher count-spans operation, but is not a Rebar row or a
// field-ratio result.
func BenchmarkSixPatternCountSpans(b *testing.B) {
	dir := os.Getenv("CASEI_REBAR_HAYSTACKS")
	if dir == "" {
		dir = filepath.Join("..", "haystacks")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "sherlock.txt"))
	if err != nil {
		b.Fatalf("six-pattern guard haystack: %v (fetch with audit/rebar/haystacks.sh and set CASEI_REBAR_HAYSTACKS)", err)
	}
	haystack := string(raw)
	patterns := []string{"Sherlock", "Sailor", "Kelvin", "Kettle", "Irene", "India"}
	matcher := casei.NewMatcher(patterns)
	want, err := verifyEnumeration(haystack, patterns, matcher, true)
	if err != nil {
		b.Fatalf("six-pattern guard preflight: %v", err)
	}
	if want != 968 {
		b.Fatalf("six-pattern guard preflight spans=%d, want 968", want)
	}
	if got := countMatches(haystack, matcher, true); got != want {
		b.Fatalf("six-pattern guard count-spans = %d, preflight expects %d", got, want)
	}
	b.SetBytes(int64(len(haystack)))
	for b.Loop() {
		benchSink = countMatches(haystack, matcher, true)
	}
}

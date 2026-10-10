package casei

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
)

type exactEachResult struct {
	match Match
	width int
}

func exactRefFind(haystack string, patterns []string) (Match, bool) {
	best := Match{}
	found := false
	for pattern, needle := range patterns {
		start := strings.Index(haystack, needle)
		if start < 0 {
			continue
		}
		if !found || start < best.Start || start == best.Start && pattern < best.Pattern {
			best = Match{Pattern: pattern, Start: start}
			found = true
		}
	}
	return best, found
}

func exactRefEach(haystack string, patterns []string) []exactEachResult {
	var results []exactEachResult
	for at := 0; at <= len(haystack); {
		match, ok := exactRefFind(haystack[at:], patterns)
		if !ok {
			return results
		}
		match.Start += at
		width := len(patterns[match.Pattern])
		results = append(results, exactEachResult{match: match, width: width})
		if width != 0 {
			at = match.Start + width
			continue
		}
		if match.Start == len(haystack) {
			return results
		}
		at = match.Start + 1
	}
	return results
}

func checkExactMatcher(t *testing.T, haystack string, patterns []string) []exactEachResult {
	t.Helper()
	matcher := NewExactMatcher(patterns)
	want, wantOK := exactRefFind(haystack, patterns)
	got, gotOK := matcher.Find(haystack)
	if gotOK != wantOK || gotOK && got != want {
		t.Fatalf("Find(%x, %q) = %+v,%t, want %+v,%t", haystack, patterns, got, gotOK, want, wantOK)
	}

	wantEach := exactRefEach(haystack, patterns)
	var gotEach []exactEachResult
	if !matcher.Each(haystack, func(match Match, width int) bool {
		gotEach = append(gotEach, exactEachResult{match: match, width: width})
		return true
	}) {
		t.Fatal("Each stopped during full enumeration")
	}
	if !reflect.DeepEqual(gotEach, wantEach) {
		t.Fatalf("Each(%x, %q) = %+v, want %+v", haystack, patterns, gotEach, wantEach)
	}
	return gotEach
}

func TestNewExactMatcherOrderingWidthsAndByteFragments(t *testing.T) {
	cases := []struct {
		name     string
		haystack string
		patterns []string
		want     Match
		ok       bool
	}{
		{"earliest start beats lower ID", "early then late", []string{"late", "early"}, Match{Pattern: 1, Start: 0}, true},
		{"long prefix tie", "abc", []string{"abc", "a"}, Match{Pattern: 0, Start: 0}, true},
		{"short prefix tie", "abc", []string{"a", "abc"}, Match{Pattern: 0, Start: 0}, true},
		{"duplicate tie", "xABC", []string{"ABC", "ABC"}, Match{Pattern: 0, Start: 1}, true},
		{"case differs", "fatal PANIC", []string{"fatal panic"}, Match{}, false},
		{"continuation-byte fragment", "K", []string{"\x84"}, Match{Pattern: 0, Start: 1}, true},
		{"rune interior fragment", "K", []string{"\x84\xaa"}, Match{Pattern: 0, Start: 1}, true},
		{"malformed bytes", "\xffa\x80", []string{"\xffa"}, Match{Pattern: 0, Start: 0}, true},
		{"no patterns", "abc", nil, Match{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			matcher := NewExactMatcher(tc.patterns)
			got, ok := matcher.Find(tc.haystack)
			if ok != tc.ok || ok && got != tc.want {
				t.Fatalf("Find(%x) = %+v,%t, want %+v,%t", tc.haystack, got, ok, tc.want, tc.ok)
			}
			checkExactMatcher(t, tc.haystack, tc.patterns)
		})
	}

	got := checkExactMatcher(t, "ababa", []string{"aba", "ba", "a"})
	want := []exactEachResult{{Match{Pattern: 0, Start: 0}, 3}, {Match{Pattern: 1, Start: 3}, 2}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("non-overlapping Each = %+v, want %+v", got, want)
	}
}

func TestNewExactMatcherEmptyProgressAndStop(t *testing.T) {
	for _, tc := range []struct {
		name     string
		haystack string
		patterns []string
		want     []exactEachResult
	}{
		{"empty wins ties and advances by byte", "AK", []string{"", "A"}, []exactEachResult{
			{Match{Pattern: 0, Start: 0}, 0},
			{Match{Pattern: 0, Start: 1}, 0},
			{Match{Pattern: 0, Start: 2}, 0},
			{Match{Pattern: 0, Start: 3}, 0},
			{Match{Pattern: 0, Start: 4}, 0},
		}},
		{"nonempty lower ID consumes its bytes", "a", []string{"a", ""}, []exactEachResult{
			{Match{Pattern: 0, Start: 0}, 1},
			{Match{Pattern: 1, Start: 1}, 0},
		}},
		{"empty haystack", "", []string{"", ""}, []exactEachResult{
			{Match{Pattern: 0, Start: 0}, 0},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := checkExactMatcher(t, tc.haystack, tc.patterns)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Each = %+v, want %+v", got, tc.want)
			}
		})
	}

	matcher := NewExactMatcher([]string{"ab", "b"})
	calls := 0
	complete := matcher.Each("abb", func(match Match, width int) bool {
		calls++
		if match != (Match{Pattern: 0, Start: 0}) || width != 2 {
			t.Fatalf("first result = %+v width %d, want {0 0} width 2", match, width)
		}
		return false
	})
	if complete || calls != 1 {
		t.Fatalf("stopped Each = (%t, %d callbacks), want (false, 1)", complete, calls)
	}
}

func TestNewExactMatcherPopulationAgainstByteOracle(t *testing.T) {
	corpora := []string{
		"the search engine scans English prose and returns a literal match among ordinary words; ",
		"func handleReq(ctx context.Context, in []byte) error { return search(in, key) }\n",
		"2026-08-04T12:30:45.123Z INFO service=search region=eu-west-1 msg=cache miss latency_ms=31\n",
		"Доктор Ватсон заметил улицу Бейкер и туман над Лондоном; письмо лежало у окна. ",
	}
	for corpusID, corpus := range corpora {
		for size := 1; size <= 64; size++ {
			for _, count := range []int{size, 64} {
				patterns := make([]string, count)
				for pattern := range patterns {
					start := (pattern*67 + size*13) % (len(corpus) - size + 1)
					patterns[pattern] = corpus[start : start+size]
				}
				t.Run(fmt.Sprintf("corpus-%d/count-%d/bytes-%d", corpusID, count, size), func(t *testing.T) {
					checkExactMatcher(t, corpus+corpus, patterns)
				})
			}
		}
	}
}

func TestNewExactMatcherCopiesPatternsAndAllowsNestedConcurrentReads(t *testing.T) {
	patterns := []string{"alpha", "βeta", "\x84"}
	matcher := NewExactMatcher(patterns)
	patterns[0] = "changed"
	returned := matcher.Patterns()
	returned[1] = "changed"
	if got, ok := matcher.Find("xalphaβeta"); !ok || got != (Match{Pattern: 0, Start: 1}) {
		t.Fatalf("Find after caller mutation = %+v,%t", got, ok)
	}
	if got := matcher.Patterns(); !reflect.DeepEqual(got, []string{"alpha", "βeta", "\x84"}) {
		t.Fatalf("Patterns after caller mutation = %q", got)
	}

	var callbacks int
	if !matcher.Each("alphaβeta", func(match Match, width int) bool {
		callbacks++
		if nested, ok := matcher.Find("βeta"); !ok || nested != (Match{Pattern: 1, Start: 0}) {
			t.Fatalf("nested Find = %+v,%t", nested, ok)
		}
		var nestedEach []exactEachResult
		if !matcher.Each("\x84", func(match Match, width int) bool {
			nestedEach = append(nestedEach, exactEachResult{match: match, width: width})
			return true
		}) {
			t.Fatal("nested Each stopped")
		}
		if want := []exactEachResult{{Match{Pattern: 2, Start: 0}, 1}}; !reflect.DeepEqual(nestedEach, want) {
			t.Fatalf("nested Each = %+v, want %+v", nestedEach, want)
		}
		return true
	}) {
		t.Fatal("outer Each stopped")
	}
	if callbacks != 2 {
		t.Fatalf("outer Each callbacks = %d, want 2", callbacks)
	}

	const callers = 16
	start := make(chan struct{})
	errs := make(chan string, callers)
	var group sync.WaitGroup
	for caller := 0; caller < callers; caller++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			for range 100 {
				if got, ok := matcher.Find("xalphaβeta"); !ok || got != (Match{Pattern: 0, Start: 1}) {
					errs <- "concurrent Find"
					return
				}
				var gotEach []exactEachResult
				if !matcher.Each("alphaβeta", func(match Match, width int) bool {
					gotEach = append(gotEach, exactEachResult{match: match, width: width})
					return true
				}) {
					errs <- "concurrent Each stopped"
					return
				}
				if want := []exactEachResult{{Match{Pattern: 0, Start: 0}, 5}, {Match{Pattern: 1, Start: 5}, 5}}; !reflect.DeepEqual(gotEach, want) {
					errs <- "concurrent Each result"
					return
				}
			}
		}()
	}
	close(start)
	group.Wait()
	close(errs)
	if err, ok := <-errs; ok {
		t.Fatal(err)
	}
}

func TestNewExactMatcherNilInputsAndFoldedAPIsRemainFolded(t *testing.T) {
	var nilMatcher *Matcher
	if got, ok := nilMatcher.Find("x"); ok || got != (Match{}) {
		t.Fatalf("nil Matcher Find = %+v,%t", got, ok)
	}
	if !nilMatcher.Each("x", func(Match, int) bool { t.Fatal("nil Matcher called yield"); return true }) {
		t.Fatal("nil Matcher Each returned false")
	}
	if !NewExactMatcher(nil).Each("x", nil) {
		t.Fatal("nil yield returned false")
	}
	if _, ok := NewExactMatcher(nil).Find("x"); ok {
		t.Fatal("empty set matched")
	}

	for _, tc := range []struct {
		haystack string
		needle   string
	}{
		{"KELVIN", "kelvin"},
		{"ſecret", "SECRET"},
		{"Σςσ", "σσσ"},
		{"\xffK\x80", "\xffk\x80"},
	} {
		want := reference(tc.haystack, tc.needle)
		matcher := NewMatcher([]string{tc.needle})
		if got, ok := matcher.Find(tc.haystack); !ok || got != (Match{Pattern: 0, Start: want}) {
			t.Fatalf("NewMatcher.Find(%x,%x) = %+v,%t, reference offset %d", tc.haystack, tc.needle, got, ok, want)
		}
		if got := IndexFold(tc.haystack, tc.needle); got != want {
			t.Fatalf("IndexFold(%x,%x) = %d, want %d", tc.haystack, tc.needle, got, want)
		}
		var gotEach []refEachResult
		matcher.Each(tc.haystack, func(match Match, width int) bool {
			gotEach = append(gotEach, refEachResult{match: match, width: width})
			return true
		})
		if wantEach := refEach(tc.haystack, []string{tc.needle}); !reflect.DeepEqual(gotEach, wantEach) {
			t.Fatalf("NewMatcher.Each(%x,%x) = %+v, want %+v", tc.haystack, tc.needle, gotEach, wantEach)
		}
	}

	if _, ok := NewMatcher([]string{"\x84"}).Find("K"); ok {
		t.Fatal("folded matcher accepted a continuation byte inside a valid rune")
	}
	if got, ok := NewExactMatcher([]string{"\x84"}).Find("K"); !ok || got != (Match{Pattern: 0, Start: 1}) {
		t.Fatalf("exact matcher did not find the raw continuation byte: %+v,%t", got, ok)
	}
}

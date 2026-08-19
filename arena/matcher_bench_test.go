package arena_test

// The multi-needle arena. Pre-registered before any multi-needle claim.
// Baselines:
//
//   candidate  - casei.Matcher (naive reference form, under optimization)
//   per-pattern - warmed N=1 Matcher loop: the repeated-traversal control
//   regexpAlt  - precompiled (?i)(?:p0|p1|...): the stdlib answer and the
//                semantic anchor for leftmost-start
//   pcre2-jit  - precompiled PCRE2 caseless UTF-8 alternation, with JIT
//                required before it enters either valid-UTF-8 tier
//   rure       - precompiled rust-regex C API (?i) alternation; capture branches
//                expose its leftmost-first selection as a pattern ID, and only
//                an observed memchr AVX2 query can enter the competitive bar
//   vectorscan - precompiled Vectorscan caseless literal set; its scan adapter
//                reduces all reports to leftmost/lowest-pattern order
//   stringzilla - precompiled full-fold StringZilla literals; the timed adapter
//                verifies simple folding and reduces leftmost/lowest-pattern
//   rustac     - direct pinned Rust aho-corasick DFA, ASCII case-insensitive,
//                LeftmostFirst, with its prefilter enabled and query-audited
//                before it enters the ASCII field
//   ac         - github.com/petar-dambovaliev/aho-corasick, DFA,
//                supplemental and non-winning (ASCII tier only:
//                the reference multi-pattern library renounces Unicode
//                folding in its own docs)
//   ceiling    - exact-match AC (DFA) over pre-folded haystack+patterns:
//                what multi-needle costs when folding is free

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	ac "github.com/petar-dambovaliev/aho-corasick"

	"github.com/tsenart/casei"
)

type multiScenario struct {
	name     string
	haystack string
	patterns []string
	utf8     bool
}

func genNeedles(n int, format string) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf(format, i)
	}
	return out
}

var multiScenarios = func() []multiScenario {
	logs1m := buildLogCorpus(1 << 20)
	prose1m := buildProseCorpus(1 << 20)
	cyr1m := buildWordCorpus(cyrillicWords, 1<<20)

	hit8 := []string{
		"fatal panic", "segfault detected", "oom killed", "disk full",
		"Payment Declined", "quota exceeded", "handshake failed", "watchdog fired",
	}
	hazard8 := []string{
		"щупальце", "kelvin", "zygomorphic", "ſecret",
		"Zq9xW", "grofse", "ΤΈΛΟΣ", "watchdog",
	}
	return []multiScenario{
		{"multi_N2_miss_log_1mb", logs1m, []string{"fatal panic", "segfault detected"}, false},
		{"multi_N8_miss_log_1mb", logs1m, genNeedles(8, "Zq%03dxW vK"), false},
		{"multi_N64_miss_log_64kb", logs1m[:64<<10], genNeedles(64, "Zq%03dxW"), false},
		{"multi_N512_miss_log_64kb", logs1m[:64<<10], genNeedles(512, "Zq%03dxW"), false},
		// The ASCII N=512 row above exercises pattern count and nothing else: its
		// orbit union is 14 orbits over 18 members, all single-byte, so a plan
		// compiled from it never meets a width-changing fold. This row is the same
		// pattern count over the hazards -- Kelvin sign, long s, sigma, and Cyrillic
		// -- where a shared multi-pattern plan has to agree about unit boundaries
		// across 512 needles at once. Without it, a multi-needle claim says nothing
		// about the tier this repository exists for.
		{"multi_N512_miss_hazard_64kb", cyr1m[:64<<10], genHazardNeedles(512), true},
		{"multi_N8_hit_log_1mb", plant(logs1m, "Payment Declined", 4), hit8, false},
		{"multi_N8_miss_ru_1mb", cyr1m, genNeedles(8, "щупальце%d"), true},
		{"multi_N64_miss_ru_64kb", cyr1m[:64<<10], genNeedles(64, "щупальце%d"), true},
		// This is the all-ASCII half of the mixed-fold hazard set. It keeps the
		// Cyrillic, Kelvin, long-s, and Greek alternatives compiled while making
		// every candidate a miss, so a shared plan cannot hide repeated root
		// survivors behind the planted-hit early exit below.
		{"multi_N8_miss_hazard_1mb", prose1m, hazard8, true},
		{"multi_N8_hazard_hit_1mb", plant(prose1m, "Kelvin", 8), hazard8, true},
	}
}()

// singleMatchers makes the warmed one-pattern control for Matcher. Each
// element uses the same N=1 plan that powers IndexFold, but keeping it here
// avoids cache-slot collisions during a repeated per-pattern measurement.
func singleMatchers(patterns []string) []*casei.Matcher {
	matchers := make([]*casei.Matcher, len(patterns))
	for i, pattern := range patterns {
		matchers[i] = casei.NewMatcher([]string{pattern})
	}
	return matchers
}

// perPatternFind is the repeated single-needle control that Matcher replaces.
// Its plans are built outside the timed operation while it retains one
// haystack traversal per pattern.
func perPatternFind(haystack string, matchers []*casei.Matcher) (casei.Match, bool) {
	best := casei.Match{Pattern: -1, Start: -1}
	for pattern, matcher := range matchers {
		if match, ok := matcher.Find(haystack); ok &&
			(best.Pattern < 0 || match.Start < best.Start) {
			best = casei.Match{Pattern: pattern, Start: match.Start}
		}
	}
	if best.Pattern < 0 {
		return casei.Match{}, false
	}
	return best, true
}

func regexpAltFor(patterns []string) *regexp.Regexp {
	quoted := make([]string, len(patterns))
	for i, p := range patterns {
		quoted[i] = regexp.QuoteMeta(p)
	}
	return regexp.MustCompile(`(?i)(?:` + strings.Join(quoted, "|") + `)`)
}

func acBuild(patterns []string, caseInsensitive bool) ac.AhoCorasick {
	b := ac.NewAhoCorasickBuilder(ac.Opts{
		AsciiCaseInsensitive: caseInsensitive,
		MatchKind:            ac.LeftMostFirstMatch,
		DFA:                  true,
	})
	return b.Build(patterns)
}

func acFirst(a *ac.AhoCorasick, h string) (casei.Match, bool) {
	it := a.Iter(h)
	m := it.Next()
	if m == nil {
		return casei.Match{}, false
	}
	return casei.Match{Pattern: m.Pattern(), Start: m.Start()}, true
}

func foldAll(patterns []string, utf8Tier bool) []string {
	out := make([]string, len(patterns))
	for i, p := range patterns {
		if utf8Tier {
			out[i] = canonFoldString(p)
		} else {
			out[i] = asciiLower(p)
		}
	}
	return out
}

// TestMultiBaselinesAgree pins the multi-needle contract: candidate, each
// native adapter, and the ASCII-tier AC baseline against refFind; regexpAlt is
// checked on match start.
func TestMultiBaselinesAgree(t *testing.T) {
	for scenarioIndex, s := range multiScenarios {
		want, wantOK := refFind(s.haystack, s.patterns)
		got, gotOK := casei.NewMatcher(s.patterns).Find(s.haystack)
		if gotOK != wantOK || (gotOK && got != want) {
			t.Errorf("%s/candidate: %+v,%v want %+v,%v", s.name, got, gotOK, want, wantOK)
		}
		got, gotOK = perPatternFind(s.haystack, singleMatchers(s.patterns))
		if gotOK != wantOK || (gotOK && got != want) {
			t.Errorf("%s/per-pattern: %+v,%v want %+v,%v", s.name, got, gotOK, want, wantOK)
		}
		re := regexpAltFor(s.patterns)
		reStart := -1
		if loc := re.FindStringIndex(s.haystack); loc != nil {
			reStart = loc[0]
		}
		wantStart := -1
		if wantOK {
			wantStart = want.Start
		}
		if reStart != wantStart {
			t.Errorf("%s/regexpAlt: start %d want %d", s.name, reStart, wantStart)
		}
		pcre := pcre2Alts[scenarioIndex]
		start, pattern, pcreOK := pcre.Find(s.haystack)
		if pcreOK != wantOK || (pcreOK && (start != want.Start || pattern != want.Pattern)) {
			t.Errorf("%s/pcre2-jit: {%d %d},%v want %+v,%v", s.name, pattern, start, pcreOK, want, wantOK)
		}
		rure := rureAlts[scenarioIndex]
		start, pattern, rureOK := rure.Find(s.haystack)
		if rureOK != wantOK || (rureOK && (start != want.Start || pattern != want.Pattern)) {
			t.Errorf("%s/rure: {%d %d},%v want %+v,%v", s.name, pattern, start, rureOK, want, wantOK)
		}
		vectorscan := vectorscanAlts[scenarioIndex]
		start, pattern, vectorscanOK := vectorscan.Find(s.haystack)
		if vectorscanOK != wantOK || (vectorscanOK && (start != want.Start || pattern != want.Pattern)) {
			t.Errorf("%s/vectorscan: {%d %d},%v want %+v,%v", s.name, pattern, start, vectorscanOK, want, wantOK)
		}
		if stringZillaAvailable {
			stringzilla := stringZillaAlts[scenarioIndex]
			start, pattern, stringzillaOK := stringzilla.Find(s.haystack)
			if stringzillaOK != wantOK || (stringzillaOK && (start != want.Start || pattern != want.Pattern)) {
				t.Errorf("%s/stringzilla: {%d %d},%v want %+v,%v", s.name, pattern, start, stringzillaOK, want, wantOK)
			}
		}
		if !s.utf8 {
			rust := rustACAlts[scenarioIndex]
			start, pattern, rustOK := rust.Find(s.haystack)
			if rustOK != wantOK || (rustOK && (start != want.Start || pattern != want.Pattern)) {
				t.Errorf("%s/rustac: {%d %d},%v want %+v,%v", s.name, pattern, start, rustOK, want, wantOK)
			}
			a := acBuild(s.patterns, true)
			acGot, acOK := acFirst(&a, s.haystack)
			if acOK != wantOK || (acOK && acGot != want) {
				t.Errorf("%s/ac: %+v,%v want %+v,%v", s.name, acGot, acOK, want, wantOK)
			}
		}
	}
}

func BenchmarkMatcher(b *testing.B) {
	for scenarioIndex, s := range multiScenarios {
		m := casei.NewMatcher(s.patterns)
		b.Run(s.name+"/candidate", func(b *testing.B) {
			b.SetBytes(int64(len(s.haystack)))
			b.ReportAllocs()
			for b.Loop() {
				_, matcherFound = m.Find(s.haystack)
			}
		})
		singles := singleMatchers(s.patterns)
		b.Run(s.name+"/per-pattern", func(b *testing.B) {
			b.SetBytes(int64(len(s.haystack)))
			b.ReportAllocs()
			for b.Loop() {
				_, matcherFound = perPatternFind(s.haystack, singles)
			}
		})
		re := regexpAltFor(s.patterns)
		b.Run(s.name+"/regexpAlt", func(b *testing.B) {
			b.SetBytes(int64(len(s.haystack)))
			for b.Loop() {
				matcherSink = len(re.FindStringIndex(s.haystack))
			}
		})
		pcre := pcre2Alts[scenarioIndex]
		b.Run(s.name+"/pcre2-jit", func(b *testing.B) {
			b.SetBytes(int64(len(s.haystack)))
			for b.Loop() {
				_, _, matcherFound = pcre.Find(s.haystack)
			}
		})
		// This diagnostic lane remains useful for the adapter, but BenchmarkBar
		// admits it only after the query-level Rust dispatch audit sees AVX2.
		rure := rureAlts[scenarioIndex]
		b.Run(s.name+"/rure", func(b *testing.B) {
			b.SetBytes(int64(len(s.haystack)))
			for b.Loop() {
				_, _, matcherFound = rure.Find(s.haystack)
			}
		})
		vectorscan := vectorscanAlts[scenarioIndex]
		b.Run(s.name+"/vectorscan", func(b *testing.B) {
			b.SetBytes(int64(len(s.haystack)))
			for b.Loop() {
				_, _, matcherFound = vectorscan.Find(s.haystack)
			}
		})
		if stringZillaAvailable {
			stringzilla := stringZillaAlts[scenarioIndex]
			b.Run(s.name+"/stringzilla", func(b *testing.B) {
				b.SetBytes(int64(len(s.haystack)))
				for b.Loop() {
					_, _, matcherFound = stringzilla.Find(s.haystack)
				}
			})
		}
		if !s.utf8 {
			rust := rustACAlts[scenarioIndex]
			b.Run(s.name+"/rustac", func(b *testing.B) {
				b.SetBytes(int64(len(s.haystack)))
				for b.Loop() {
					_, _, matcherFound = rust.Find(s.haystack)
				}
			})
			a := acBuild(s.patterns, true)
			b.Run(s.name+"/ac", func(b *testing.B) {
				b.SetBytes(int64(len(s.haystack)))
				for b.Loop() {
					_, matcherFound = acFirst(&a, s.haystack)
				}
			})
		}
		lh := s.haystack
		if s.utf8 {
			lh = canonFoldString(s.haystack)
		} else {
			lh = asciiLower(s.haystack)
		}
		exact := acBuild(foldAll(s.patterns, s.utf8), false)
		b.Run(s.name+"/ceiling", func(b *testing.B) {
			b.SetBytes(int64(len(lh)))
			for b.Loop() {
				_, matcherFound = acFirst(&exact, lh)
			}
		})
	}
}

var (
	matcherSink  int
	matcherFound bool
)

// genHazardNeedles builds n distinct needles whose fold orbits change UTF-8
// width, so a compiled multi-pattern plan cannot assume a fixed byte stride.
// The cycle covers the three hazard classes the repository pins -- KELVIN SIGN
// (3 bytes folding to 1), LONG S (2 to 1), and the sigma trio (2 to 2 with three
// members) -- plus a Cyrillic base so the row is non-ASCII throughout.
func genHazardNeedles(n int) []string {
	seeds := []string{"\u212Aелвин%d", "\u017Fекрет%d", "\u03A3игма%d", "\u03C2игма%d", "щупальце%d"}
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf(seeds[i%len(seeds)], i)
	}
	return out
}

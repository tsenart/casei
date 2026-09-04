//go:build go1.24

package casei

import (
	"fmt"
	"strings"
	"testing"
)

// BenchmarkPairShuftiMatcher covers the dense two-group root projection on a
// non-ASCII miss stream. The short case keeps the length guard scalar; longer
// cases enter the AVX-512 transition when it is runtime-enabled.
func BenchmarkPairShuftiMatcher(b *testing.B) {
	matcher := NewMatcher([]string{"Kелвин0", "ſекрет1", "Σигма2", "ςигма3", "щупальце4"})
	if !matcher.plan.filter.shufti.usable() {
		b.Fatal("pair Shufti filter was not compiled")
	}
	for _, tc := range []struct {
		name     string
		haystack string
	}{
		{"64B", strings.Repeat("ж", 32)},
		{"1KiB", strings.Repeat("ж", 512)},
		{"64KiB", strings.Repeat("ж", 32<<10)},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(tc.haystack)))
			if _, ok := matcher.Find(tc.haystack); ok {
				b.Fatal("unexpected match")
			}
			for b.Loop() {
				_, _ = matcher.Find(tc.haystack)
			}
		})
	}
}

var (
	rawByteBenchmarkMatch Match
	rawByteBenchmarkOK    bool
)

// BenchmarkRawByteConstruction keeps plan construction and its first ordinary
// Find in one operation. It catches a cache-sized raw transition allocation on
// one-shot Matchers and separately measures raw publication plus its first use,
// all through the same public route as the density sweep.
func BenchmarkRawByteConstruction(b *testing.B) {
	for _, tc := range []struct {
		name                  string
		patterns              []string
		haystack              string
		rawInstallAndFirstUse bool
	}{
		{"two_patterns_512B", rawByteCyrillicPatterns[:2], rawByteFalseCandidates(4, 128), false},
		{"two_patterns_4KiB", rawByteCyrillicPatterns[:2], rawByteFalseCandidates(4, 1024), false},
		{"five_patterns_512B", rawByteCyrillicPatterns, rawByteFalseCandidates(4, 128), false},
		{"five_patterns_513B_zero_admission", rawByteCyrillicPatterns, strings.Repeat("x", 513), false},
		{"five_patterns_5MiB_raw_install_and_first_use", rawByteCyrillicPatterns, rawByteFalseCandidatesAtLeast(256, rawBytePublicationCorpusBytes), true},
		{"two_shared_100_units_1KiB_zero_admission", rawByteLongPrefixPatterns(), strings.Repeat("x", 1<<10), false},
		{"two_mixed_folded_ascii_root_1KiB", []string{"Д", "xД"}, strings.Repeat("x", 1<<10), false},
	} {
		b.Run(tc.name, func(b *testing.B) {
			bytes := len(tc.haystack)
			if tc.rawInstallAndFirstUse {
				bytes *= 3
			}
			b.SetBytes(int64(bytes))
			b.ReportAllocs()
			for b.Loop() {
				matcher := NewMatcher(tc.patterns)
				if got, ok := matcher.Find(tc.haystack); ok || got != (Match{}) {
					b.Fatalf("false-candidate stream matched: %+v,%t", got, ok)
				}
				if tc.rawInstallAndFirstUse {
					// Keep decoded admission, the next reuse, and one following reuse in
					// one operation. An implementation may publish an eligible acceleration
					// between those calls, but both source arms execute the same public work.
					for range 2 {
						if got, ok := matcher.Find(tc.haystack); ok || got != (Match{}) {
							b.Fatalf("raw install/first use stream matched: %+v,%t", got, ok)
						}
					}
				}
			}
		})
	}
}

// BenchmarkRawByteDensity measures false-root streams through the public
// Matcher.Find entry point. The baseline advances decoded units; an eligible
// compiled plan may replace that transition with its raw row without changing
// this workload.
func BenchmarkRawByteDensity(b *testing.B) {
	for _, tc := range []struct {
		name     string
		patterns []string
		haystack string
	}{
		{"two_4KiB_one_in_32", rawByteCyrillicPatterns[:2], rawByteFalseCandidates(32, 4096/32)},
		{"two_4KiB_zero_admission", rawByteCyrillicPatterns[:2], strings.Repeat("x", 4<<10)},
		{"five_one_in_64", rawByteCyrillicPatterns, rawByteFalseCandidatesAtLeast(64, rawByteBenchmarkCorpusBytes)},
		{"five_one_in_4", rawByteCyrillicPatterns, rawByteFalseCandidatesAtLeast(4, rawByteBenchmarkCorpusBytes/16)},
	} {
		matcher := NewMatcher(tc.patterns)
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(tc.haystack)))
			b.ReportAllocs()
			if got, ok := matcher.Find(tc.haystack); ok || got != (Match{}) {
				b.Fatalf("false-candidate stream matched: %+v,%t", got, ok)
			}
			// The construction benchmark owns the first-call crossover. This
			// benchmark measures a reused public Matcher, so complete its second
			// ordinary Find before the timed steady-state loop.
			if got, ok := matcher.Find(tc.haystack); ok || got != (Match{}) {
				b.Fatalf("false-candidate stream matched: %+v,%t", got, ok)
			}
			b.ResetTimer()
			for b.Loop() {
				rawByteBenchmarkMatch, rawByteBenchmarkOK = matcher.Find(tc.haystack)
			}
		})
	}
}

// BenchmarkRawByteFreshFind measures one ordinary Find on a new Matcher. It
// includes every setup cost that this call pays. The long miss, near miss,
// sparse miss, clustered miss, and early answer keep the decision boundary
// visible to both sides of a comparison.
func BenchmarkRawByteFreshFind(b *testing.B) {
	longMiss := rawByteFalseCandidatesAtLeast(256, 10<<20)
	longNearMiss := rawByteNearMissCandidatesAtLeast(256, rawBytePublicationCorpusBytes)
	longSparse := rawByteFalseCandidatesAtLeast(8192, rawBytePublicationCorpusBytes)
	longClustered := rawByteFalseCandidates(64, rawByteFreshSampleBytes/64) +
		strings.Repeat("x", rawBytePublicationCorpusBytes-rawByteFreshSampleBytes)
	longLateMatch, longLateMatchStart := rawByteLateMatchCandidatesAtLeast(256, 10<<20)
	longEarlyMatch, longEarlyMatchStart := rawByteEarlyMatchCandidatesAtLeast(256, rawBytePublicationCorpusBytes, 8<<10)
	for _, tc := range []struct {
		name     string
		haystack string
		want     Match
		wantOK   bool
	}{
		{"five_long_false_one_in_256", longMiss, Match{}, false},
		{"five_long_near_miss_one_in_256", longNearMiss, Match{}, false},
		{"five_long_sparse_one_in_8192", longSparse, Match{}, false},
		{"five_long_clustered", longClustered, Match{}, false},
		{"five_long_early_match_one_in_256", longEarlyMatch, Match{Pattern: 0, Start: longEarlyMatchStart}, true},
		{"five_long_late_match_one_in_256", longLateMatch, Match{Pattern: 0, Start: longLateMatchStart}, true},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(tc.haystack)))
			b.ReportAllocs()
			for b.Loop() {
				matcher := NewMatcher(rawByteCyrillicPatterns)
				got, gotOK := matcher.Find(tc.haystack)
				if gotOK != tc.wantOK || got != tc.want {
					b.Fatalf("fresh Find = %+v,%t; want %+v,%t", got, gotOK, tc.want, tc.wantOK)
				}
			}
		})
	}
}

// BenchmarkRawByteFindAfterFreshFind measures a second Find after the same
// Matcher has completed a long fresh no-match search. The setup is deliberately
// outside the timer: the fresh benchmark owns its cost, while this benchmark
// keeps the published-view reuse state comparable across both source arms.
func BenchmarkRawByteFindAfterFreshFind(b *testing.B) {
	publication := rawByteFalseCandidatesAtLeast(256, 10<<20)
	lateMatch, lateMatchStart := rawByteLateMatchCandidatesAtLeast(256, rawBytePublicationCorpusBytes)
	for _, tc := range []struct {
		name     string
		haystack string
		want     Match
		wantOK   bool
	}{
		{"five_long_reused_near_miss_one_in_256", rawByteNearMissCandidatesAtLeast(256, rawBytePublicationCorpusBytes), Match{}, false},
		{"five_long_reused_late_match_one_in_256", lateMatch, Match{Pattern: 0, Start: lateMatchStart}, true},
	} {
		b.Run(tc.name, func(b *testing.B) {
			matcher := NewMatcher(rawByteCyrillicPatterns)
			if got, ok := matcher.Find(publication); ok || got != (Match{}) {
				b.Fatalf("publication Find = %+v,%t", got, ok)
			}
			b.SetBytes(int64(len(tc.haystack)))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				got, gotOK := matcher.Find(tc.haystack)
				if gotOK != tc.wantOK || got != tc.want {
					b.Fatalf("reused Find = %+v,%t; want %+v,%t", got, gotOK, tc.want, tc.wantOK)
				}
			}
		})
	}
}

func BenchmarkTripleSkipBytes(b *testing.B) {
	plan := newSearchPlan([]string{"fatal panic", "segfault detected"})
	haystack := strings.Repeat("x", 1<<20)
	b.SetBytes(int64(len(haystack)))
	for b.Loop() {
		_ = tripleSkipBytes(haystack, 0, &plan.triples)
	}
}

func BenchmarkTriplePlan(b *testing.B) {
	plan := newSearchPlan([]string{"fatal panic", "segfault detected"})
	haystack := strings.Repeat("x", 1<<20)
	b.SetBytes(int64(len(haystack)))
	for b.Loop() {
		_, _ = plan.find(haystack)
	}
}

var asciiOnlyPartitionSink int

// asciiOnlyPartitionCorpus is mostly ASCII source text with sparse valid UTF-8
// exception clusters. The final 64 bytes stay clean so a partitioned scan can
// finish its last ASCII region instead of taking the dense-tail fallback.
func asciiOnlyPartitionCorpus(size int) string {
	data := []byte(strings.Repeat("x", size))
	for at := 4096; at+len("ſK") < size-64; at += 16384 {
		copy(data[at:], "ſK")
	}
	return string(data)
}

// BenchmarkASCIIOnlyPartition measures one long ASCII literal on a sparse
// valid-UTF-8 stream. The literal's simple-fold orbit includes long s and
// Kelvin spellings, so an ASCII candidate transition must hand only bounded
// source-width halos around the exception clusters to the decoded plan.
func BenchmarkASCIIOnlyPartition(b *testing.B) {
	const size = 1 << 20
	haystack := asciiOnlyPartitionCorpus(size)
	const needle = "Sherlock Holmes"
	if got := IndexFold(haystack, needle); got != -1 {
		b.Fatalf("partition corpus unexpectedly matched at %d", got)
	}
	b.SetBytes(int64(len(haystack)))
	b.ReportAllocs()
	for b.Loop() {
		asciiOnlyPartitionSink = IndexFold(haystack, needle)
	}
}

func BenchmarkTripleShuftiEach(b *testing.B) {
	base := tripleShuftiEachPatterns
	haystack := tripleShuftiEachCorpus()
	routeOrders := [3][]string{
		{"generic", "normalized", "explicit"},
		{"normalized", "explicit", "generic"},
		{"explicit", "generic", "normalized"},
	}
	for rotation := range routeOrders {
		patterns := make([]string, len(base))
		for i := range patterns {
			patterns[i] = base[(i+rotation)%len(base)]
		}
		wantPattern := (len(base) - rotation) % len(base)
		for _, mode := range routeOrders[rotation] {
			mode := mode
			matcher := tripleShuftiProofMatcher(patterns, mode)
			if !matcher.Each(haystack, tripleShuftiEachYield) ||
				tripleShuftiEachSink.Start != len(haystack)-3 ||
				tripleShuftiEachSink.Pattern != wantPattern ||
				tripleShuftiEachSinkWidth != 3 {
				b.Fatalf("%s rotation %d: Each = %+v width %d", mode, rotation, tripleShuftiEachSink, tripleShuftiEachSinkWidth)
			}
			b.Run(fmt.Sprintf("rotation%d/%s", rotation, mode), func(b *testing.B) {
				b.SetBytes(int64(len(haystack)))
				b.ReportAllocs()
				for b.Loop() {
					if !matcher.Each(haystack, tripleShuftiEachYield) {
						b.Fatal("Each stopped unexpectedly")
					}
				}
			})
		}
	}
}

package arena_test

// The competitive bar reports the candidate's time divided by the fastest
// eligible field entrant on each scenario. `x_vs_best` is lower-is-better:
// one is parity and values below one are a win. The accompanying dispatch
// metrics make the row's ISA contract explicit instead of treating unlike
// native paths as one unnamed field.

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"testing"
	"time"

	"golang.org/x/sys/cpu"

	"github.com/tsenart/casei"
	pcre2jit "github.com/tsenart/casei/arena/pcre2"
	rustac "github.com/tsenart/casei/arena/rustac"
	stringzilla "github.com/tsenart/casei/arena/stringzilla"
	vectorscan "github.com/tsenart/casei/arena/vectorscan"
)

// windowBudget is the minimum length of one timing window.
const windowBudget = 25 * time.Millisecond

// timeWindow returns ns/op for one manually timed window of at least budget.
// It runs op before it reads the clock, so a window always counts at least one
// operation, and it ends only after the monotonic clock has advanced by budget
// and by more than zero, so a window never divides by no time. A preempted
// first check or a coarse clock therefore cannot produce an empty window.
// testing.Benchmark cannot be nested inside a running benchmark.
func timeWindow(op func(), budget time.Duration) float64 {
	start := time.Now()
	for n := 1; ; n++ {
		op()
		if elapsed := time.Since(start); elapsed >= budget && elapsed > 0 {
			return float64(elapsed.Nanoseconds()) / float64(n)
		}
	}
}

// pairedRatio measures the candidate beside one competitor six times, with
// each operation going first in three pairs. The median paired ratio reduces
// drift from CPU migration, frequency changes, and host load between timing
// windows.
func pairedRatio(candidate, competitor func()) float64 {
	candidate()
	competitor()
	var ratios [6]float64
	for round := range ratios {
		var candidateNS, competitorNS float64
		if round%2 == 0 {
			candidateNS = timeWindow(candidate, windowBudget)
			competitorNS = timeWindow(competitor, windowBudget)
		} else {
			competitorNS = timeWindow(competitor, windowBudget)
			candidateNS = timeWindow(candidate, windowBudget)
		}
		ratios[round] = candidateNS / competitorNS
	}
	sort.Float64s(ratios[:])
	return (ratios[2] + ratios[3]) / 2
}

// velozVectorBits is deliberately strict: Veloz has an SSE/scalar fallback,
// but the field's x86 entrant is its source-audited AVX2 path. Do not race a
// weaker fallback under the same name.
func velozVectorBits() int {
	if cpu.X86.HasAVX2 {
		return 256
	}
	return 0
}

func boolMetric(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

// singleField returns the entrants that count toward x_vs_best on a
// single-needle scenario: every exact implementation other than the candidate
// whose tier covers the scenario. Every pinned entrant that supports a row is
// timed; a dispatch width is a diagnostic and never removes an entrant.
func singleField(s scenario) []impl {
	var field []impl
	for _, im := range impls {
		if im.name != "candidate" && im.foldExact && im.supports(s) {
			field = append(field, im)
		}
	}
	return field
}

// benchSingle reports x_vs_best for one single-needle scenario: the candidate
// paired with each entrant of singleField.
func benchSingle(b *testing.B, s scenario) {
	candidate := func() { sink = runSingleScenario(casei.IndexFold, s) }
	field := singleField(s)
	best := 0.0
	for _, im := range field {
		best = max(best, pairedRatio(candidate, func() { sink = runSingleScenario(im.index, s) }))
	}
	for b.Loop() {
		candidate()
	}
	b.ReportMetric(best, "x_vs_best")
	b.ReportMetric(float64(len(field)), "competitors")
	b.ReportMetric(float64(len(field)+1), "entrants")
	reportSingleDispatch(b, s, field)
}

func reportSingleDispatch(b *testing.B, s scenario, field []impl) {
	active := func(name string) float64 {
		return boolMetric(slices.ContainsFunc(field, func(im impl) bool { return im.name == name }))
	}
	vscan := vectorscanSingles[s.needle]
	if vscan == nil {
		panic(fmt.Sprintf("Vectorscan baseline was not compiled for %q", s.needle))
	}
	velozBits := 0
	if !s.utf8 {
		velozBits = velozVectorBits()
	}
	stringZillaBits := 0
	if stringZillaAvailable {
		stringZillaBits = stringzilla.VectorBits()
	}
	rureBits := 0
	if re := rureSingles[s.needle]; re != nil {
		rureBits = re.VectorBits()
	}
	b.ReportMetric(1, "candidate_active")
	b.ReportMetric(float64(casei.NewMatcher([]string{s.needle}).VectorBits()), "candidate_vector_bits")
	b.ReportMetric(active("regexp"), "regexp_active")
	b.ReportMetric(0, "regexp_vector_bits")
	b.ReportMetric(active("pcre2-jit"), "pcre2_active")
	b.ReportMetric(float64(pcre2jit.VectorBits()), "pcre2_vector_bits")
	b.ReportMetric(active("rure"), "rure_active")
	b.ReportMetric(float64(rureBits), "rure_vector_bits")
	b.ReportMetric(active("vectorscan"), "vectorscan_active")
	b.ReportMetric(float64(vscan.VectorBits()), "vectorscan_vector_bits")
	b.ReportMetric(boolMetric(vscan.HasVBMI()), "vectorscan_vbmi")
	b.ReportMetric(active("stringzilla"), "stringzilla_active")
	b.ReportMetric(float64(stringZillaBits), "stringzilla_vector_bits")
	b.ReportMetric(active("veloz"), "veloz_active")
	b.ReportMetric(float64(velozBits), "veloz_vector_bits")
	b.ReportMetric(active("rustac"), "rustac_active")
	b.ReportMetric(float64(rustACSingles[s.needle].VectorBits()), "rustac_vector_bits")
}

func reportMultiDispatch(b *testing.B, s multiScenario, candidateBits int, rure *rureRegex, rust *rustac.Matcher, vscan *vectorscan.Matcher) {
	velozBits := 0 // Veloz has no multi-pattern API.
	stringZillaBits := 0
	if stringZillaAvailable {
		stringZillaBits = stringzilla.VectorBits()
	}
	rureBits := rure.VectorBits()
	rustBits := rust.VectorBits()
	b.ReportMetric(1, "candidate_active")
	b.ReportMetric(float64(candidateBits), "candidate_vector_bits")
	b.ReportMetric(1, "regexp_active")
	b.ReportMetric(0, "regexp_vector_bits")
	b.ReportMetric(1, "pcre2_active")
	b.ReportMetric(float64(pcre2jit.VectorBits()), "pcre2_vector_bits")
	b.ReportMetric(1, "rure_active")
	b.ReportMetric(float64(rureBits), "rure_vector_bits")
	b.ReportMetric(1, "vectorscan_active")
	b.ReportMetric(float64(vscan.VectorBits()), "vectorscan_vector_bits")
	b.ReportMetric(boolMetric(vscan.HasVBMI()), "vectorscan_vbmi")
	b.ReportMetric(boolMetric(stringZillaAvailable), "stringzilla_active")
	b.ReportMetric(float64(stringZillaBits), "stringzilla_vector_bits")
	b.ReportMetric(0, "veloz_active")
	b.ReportMetric(float64(velozBits), "veloz_vector_bits")
	b.ReportMetric(boolMetric(!s.utf8), "rustac_active")
	b.ReportMetric(float64(rustBits), "rustac_vector_bits")
	b.ReportMetric(boolMetric(!s.utf8), "go_ac_active")
	b.ReportMetric(0, "go_ac_vector_bits")
}

// BenchmarkASCIIOnlyPartitionField pairs the sparse exception-cluster
// workload with the field entrants that can answer the same single-query
// contract. It is a focused mechanism probe and is intentionally outside the
// acceptance rows.
func BenchmarkASCIIOnlyPartitionField(b *testing.B) {
	benchSingle(b, asciiPartitionScenario)
}

// BenchmarkBar reports x_vs_best per scenario: candidate time relative to the
// fastest other implementation that can answer the same query correctly. The
// Go Aho-Corasick baseline remains visible as a supplemental scalar entrant,
// but it is intentionally not eligible to establish the winning bar; the
// direct Rust DFA is the native multi-pattern control.
func BenchmarkBar(b *testing.B) {
	for _, s := range scenarios {
		s := s
		b.Run("single/"+s.name, func(b *testing.B) { benchSingle(b, s) })
	}

	for scenarioIndex, s := range multiScenarios {
		s := s
		scenarioIndex := scenarioIndex
		b.Run("multi/"+s.name, func(b *testing.B) {
			m := casei.NewMatcher(s.patterns)
			candidate := func() { _, matcherFound = m.Find(s.haystack) }

			re := regexpAltFor(s.patterns)
			best := pairedRatio(candidate, func() { matcherSink = len(re.FindStringIndex(s.haystack)) })
			competitors := 1
			pcre := pcre2Alts[scenarioIndex]
			if ratio := pairedRatio(candidate, func() { _, _, matcherFound = pcre.Find(s.haystack) }); ratio > best {
				best = ratio
			}
			competitors++
			rure := rureAlts[scenarioIndex]
			if ratio := pairedRatio(candidate, func() { _, _, matcherFound = rure.Find(s.haystack) }); ratio > best {
				best = ratio
			}
			competitors++
			vscan := vectorscanAlts[scenarioIndex]
			if ratio := pairedRatio(candidate, func() { _, _, matcherFound = vscan.Find(s.haystack) }); ratio > best {
				best = ratio
			}
			competitors++
			if stringZillaAvailable {
				stringzilla := stringZillaAlts[scenarioIndex]
				if ratio := pairedRatio(candidate, func() { _, _, matcherFound = stringzilla.Find(s.haystack) }); ratio > best {
					best = ratio
				}
				competitors++
			}
			supplemental := 0
			rust := rustACAlts[scenarioIndex]
			if !s.utf8 {
				// The memchr audit cannot see the packed Teddy path, so the
				// reported width is a diagnostic and never removes the entrant.
				if ratio := pairedRatio(candidate, func() { _, _, matcherFound = rust.Find(s.haystack) }); ratio > best {
					best = ratio
				}
				competitors++

				goAC := acBuild(s.patterns, true)
				_ = pairedRatio(candidate, func() { _, matcherFound = acFirst(&goAC, s.haystack) })
				supplemental++
			}
			for b.Loop() {
				candidate()
			}
			b.ReportMetric(best, "x_vs_best")
			b.ReportMetric(float64(competitors), "competitors")
			b.ReportMetric(float64(competitors+1+supplemental), "entrants")
			reportMultiDispatch(b, s, m.VectorBits(), rure, rust, vscan)
		})
	}
}

// TestSingleFieldCountsRustAC holds single rows to the field rule for the Rust
// aho-corasick entrant: it is exact on the ASCII tier, so every ASCII single
// scenario times it and no UTF-8 one does.
func TestSingleFieldCountsRustAC(t *testing.T) {
	for _, s := range singleScenarios {
		timed := slices.ContainsFunc(singleField(s), func(im impl) bool { return im.name == "rustac" })
		if timed != !s.utf8 {
			t.Errorf("%s (utf8=%v): rustac timed=%v", s.name, s.utf8, timed)
		}
	}
}

// TestTimeWindowCountsWork holds every timing window to at least one operation
// and a positive, finite time per operation. A zero budget is the case where
// the budget is already spent when the window first checks the clock, as
// after a preemption.
func TestTimeWindowCountsWork(t *testing.T) {
	for _, budget := range []time.Duration{-time.Millisecond, 0, time.Millisecond} {
		calls := 0
		ns := timeWindow(func() { calls++ }, budget)
		if calls < 1 || !(ns > 0) || math.IsInf(ns, 0) {
			t.Errorf("budget %v: %d calls, %v ns/op; want at least one call and a positive finite time", budget, calls, ns)
		}
	}
}

package arena_test

// The competitive bar reports the candidate's time divided by the fastest
// eligible field entrant on each scenario. `x_vs_best` is lower-is-better:
// one is parity and values below one are a win. The accompanying dispatch
// metrics make the row's ISA contract explicit instead of treating unlike
// native paths as one unnamed field.

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	veloz "github.com/mhr3/veloz/ascii"
	"golang.org/x/sys/cpu"

	"github.com/tsenart/casei"
	pcre2jit "github.com/tsenart/casei/arena/pcre2"
	rustac "github.com/tsenart/casei/arena/rustac"
	stringzilla "github.com/tsenart/casei/arena/stringzilla"
	vectorscan "github.com/tsenart/casei/arena/vectorscan"
)

// timeSample returns ns/op for one operation. It times manually rather
// than through testing.Benchmark, which cannot be nested inside a running
// benchmark.
func timeSample(op func()) float64 {
	const budget = 25 * time.Millisecond
	n := 0
	start := time.Now()
	for time.Since(start) < budget {
		op()
		n++
	}
	return float64(time.Since(start).Nanoseconds()) / float64(n)
}

// timeOps measures every entrant three times. Each round shuffles the entrants
// before sampling them so neither the candidate nor a field implementation
// systematically inherits a warmer cache, branch history, or frequency state.
func timeOps(ops ...func()) []float64 {
	best := make([]float64, len(ops))
	order := make([]int, len(ops))
	for i := range order {
		order[i] = i
	}
	seed := uint64(time.Now().UnixNano())
	rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	for sample := 0; sample < 3; sample++ {
		rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
		for _, i := range order {
			if ns := timeSample(ops[i]); best[i] == 0 || ns < best[i] {
				best[i] = ns
			}
		}
	}
	return best
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

func reportSingleDispatch(b *testing.B, s scenario) {
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
	b.ReportMetric(1, "regexp_active")
	b.ReportMetric(0, "regexp_vector_bits")
	b.ReportMetric(1, "pcre2_active")
	b.ReportMetric(float64(pcre2jit.VectorBits()), "pcre2_vector_bits")
	b.ReportMetric(boolMetric(rureBits == 256), "rure_active")
	b.ReportMetric(float64(rureBits), "rure_vector_bits")
	b.ReportMetric(1, "vectorscan_active")
	b.ReportMetric(float64(vscan.VectorBits()), "vectorscan_vector_bits")
	b.ReportMetric(boolMetric(vscan.HasVBMI()), "vectorscan_vbmi")
	b.ReportMetric(boolMetric(stringZillaAvailable), "stringzilla_active")
	b.ReportMetric(float64(stringZillaBits), "stringzilla_vector_bits")
	b.ReportMetric(boolMetric(!s.utf8 && velozBits == 256), "veloz_active")
	b.ReportMetric(float64(velozBits), "veloz_vector_bits")
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
	b.ReportMetric(boolMetric(rureBits == 256), "rure_active")
	b.ReportMetric(float64(rureBits), "rure_vector_bits")
	b.ReportMetric(1, "vectorscan_active")
	b.ReportMetric(float64(vscan.VectorBits()), "vectorscan_vector_bits")
	b.ReportMetric(boolMetric(vscan.HasVBMI()), "vectorscan_vbmi")
	b.ReportMetric(boolMetric(stringZillaAvailable), "stringzilla_active")
	b.ReportMetric(float64(stringZillaBits), "stringzilla_vector_bits")
	b.ReportMetric(0, "veloz_active")
	b.ReportMetric(float64(velozBits), "veloz_vector_bits")
	b.ReportMetric(boolMetric(!s.utf8 && rustBits == 256), "rustac_active")
	b.ReportMetric(float64(rustBits), "rustac_vector_bits")
	b.ReportMetric(boolMetric(!s.utf8), "go_ac_active")
	b.ReportMetric(0, "go_ac_vector_bits")
}

// BenchmarkBar reports x_vs_best per scenario: candidate time relative to the
// fastest other implementation that can answer the same query correctly. The
// Go Aho-Corasick baseline remains visible as a supplemental scalar entrant,
// but it is intentionally not eligible to establish the winning bar; the
// direct Rust DFA is the native multi-pattern control.
func BenchmarkBar(b *testing.B) {
	for _, s := range scenarios {
		s := s
		b.Run("single/"+s.name, func(b *testing.B) {
			rure := rureSingles[s.needle]
			ops := []func(){
				func() { sink = runSingleScenario(casei.IndexFold, s) },
				func() { sink = runSingleScenario(indexRegexp, s) },
				func() { sink = runSingleScenario(indexPCRE2, s) },
				func() { sink = runSingleScenario(indexRure, s) },
				func() { sink = runSingleScenario(indexVectorscan, s) },
			}
			// A scalar/SSE Rust adapter remains diagnostic only; it cannot establish
			// the target-width field ceiling merely because this host has AVX2.
			eligible := []bool{false, true, true, rure.VectorBits() == 256, true}
			if stringZillaAvailable {
				ops = append(ops, func() { sink = runSingleScenario(indexStringZilla, s) })
				eligible = append(eligible, true)
			}
			if !s.utf8 && velozVectorBits() == 256 {
				ops = append(ops, func() { sink = runSingleScenario(veloz.IndexFold, s) })
				eligible = append(eligible, true)
			}
			times := timeOps(ops...)
			cand := times[0]
			best := 0.0
			competitors := 0
			for i := 1; i < len(times); i++ {
				if !eligible[i] {
					continue
				}
				if best == 0 || times[i] < best {
					best = times[i]
				}
				competitors++
			}
			for b.Loop() {
				sink = runSingleScenario(casei.IndexFold, s)
			}
			b.ReportMetric(cand/best, "x_vs_best")
			b.ReportMetric(float64(competitors), "competitors")
			b.ReportMetric(float64(competitors+1), "entrants")
			reportSingleDispatch(b, s)
		})
	}

	for scenarioIndex, s := range multiScenarios {
		s := s
		scenarioIndex := scenarioIndex
		b.Run("multi/"+s.name, func(b *testing.B) {
			m := casei.NewMatcher(s.patterns)
			re := regexpAltFor(s.patterns)
			pcre := pcre2Alts[scenarioIndex]
			rure := rureAlts[scenarioIndex]
			vscan := vectorscanAlts[scenarioIndex]
			ops := []func(){
				func() { _, matcherFound = m.Find(s.haystack) },
				func() { matcherSink = len(re.FindStringIndex(s.haystack)) },
				func() { _, _, matcherFound = pcre.Find(s.haystack) },
				func() { _, _, matcherFound = rure.Find(s.haystack) },
				func() { _, _, matcherFound = vscan.Find(s.haystack) },
			}
			eligible := []bool{false, true, true, rure.VectorBits() == 256, true}
			if stringZillaAvailable {
				stringzilla := stringZillaAlts[scenarioIndex]
				ops = append(ops, func() { _, _, matcherFound = stringzilla.Find(s.haystack) })
				eligible = append(eligible, true)
			}
			supplemental := 0
			rust := rustACAlts[scenarioIndex]
			if !s.utf8 {
				ops = append(ops, func() { _, _, matcherFound = rust.Find(s.haystack) })
				// The direct Rust DFA is eligible only when this query reaches its
				// observed AVX2 prefilter path.
				eligible = append(eligible, rust.VectorBits() == 256)
				goAC := acBuild(s.patterns, true)
				ops = append(ops, func() { _, matcherFound = acFirst(&goAC, s.haystack) })
				eligible = append(eligible, false)
				supplemental++
			}
			times := timeOps(ops...)
			cand := times[0]
			best := 0.0
			competitors := 0
			for i := 1; i < len(times); i++ {
				if !eligible[i] {
					continue
				}
				if best == 0 || times[i] < best {
					best = times[i]
				}
				competitors++
			}
			for b.Loop() {
				_, matcherFound = m.Find(s.haystack)
			}
			b.ReportMetric(cand/best, "x_vs_best")
			b.ReportMetric(float64(competitors), "competitors")
			b.ReportMetric(float64(competitors+1+supplemental), "entrants")
			reportMultiDispatch(b, s, m.VectorBits(), rure, rust, vscan)
		})
	}
}

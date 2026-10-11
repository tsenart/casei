package arena_test

// The arena. Every implementation races on the same scenario matrix:
// realistic corpora (logs, prose, code, Cyrillic text), miss-heavy
// throughput scans, dense-hit counting, short-haystack latency, a
// needle-length sweep, fold-hazard UTF-8 scenarios, and the adversarial
// family (periodic, samechar, torture) that punishes quadratic verification.
//
// Two tiers share one semantics (Unicode simple folding):
//
//   ASCII tier - pure-ASCII corpora and needles. All baselines compete,
//                including veloz (an ASCII-only engine that is nonetheless
//                fold-correct on pure-ASCII input).
//   UTF-8 tier - corpora or needles leave ASCII. Baselines that cannot
//                speak the semantics drop out: veloz is skipped, and the
//                tolower idiom runs for perf reference but is EXCLUDED from
//                the agreement test because strings.ToLower is not case
//                folding (it separates σ/ς, misses K→k on some classes'
//                inverses, and re-encodes).
//
// Baselines:
//
//   candidate  - casei.IndexFold, the function under optimization
//   tolower    - strings.Index(strings.ToLower(h), strings.ToLower(n)):
//                the idiom everyone actually writes, allocations included
//   regexp     - precompiled (?i) literal via regexp: the stdlib answer and
//                the semantic anchor (exact simple folding)
//   pcre2-jit  - precompiled PCRE2 caseless UTF-8 literal, with JIT required
//                before it enters either valid-UTF-8 tier (including ASCII)
//   rure       - precompiled rust-regex C API (?i) literal; its audited
//                memchr width is a diagnostic and never removes it
//   vectorscan - precompiled Vectorscan caseless UTF-8 literal set, with a
//                leftmost/lowest-ID adapter timed with each scan
//   stringzilla - precompiled StringZilla full-fold literal, with timed
//                simple-fold verification and multi-pattern reduction
//   veloz      - github.com/mhr3/veloz/ascii.IndexFold (ASCII tier only)
//   rustac     - Rust aho-corasick DFA with ASCII case insensitivity, as an
//                N=1 pattern set (ASCII tier only)
//   ceiling    - strings.Index on pre-folded input: exact-match physics,
//                what caseless search costs if folding were free

import (
	"math/rand/v2"
	"regexp"
	"strings"
	"testing"

	veloz "github.com/mhr3/veloz/ascii"

	"github.com/tsenart/casei"
	"github.com/tsenart/casei/arena/board"
)

// ---- corpora (deterministic; no testdata files) ----------------------------

func corpusRNG() *rand.Rand { return rand.New(rand.NewPCG(0xCA5E1, 0xA12E9A)) }

// The corpus builders live in package board, which draws them per cell; the
// fixed scenarios below pass corpusRNG so their bytes never change.
func buildLogCorpus(size int) string   { return board.Logs(corpusRNG(), size) }
func buildProseCorpus(size int) string { return board.Prose(corpusRNG(), size) }
func buildCodeCorpus(size int) string  { return board.Code(corpusRNG(), size) }
func buildCyrillicCorpus(size int) string {
	return board.Russian(corpusRNG(), size)
}

func buildASCIIOnlyPartitionCorpus(size int) string {
	data := []byte(strings.Repeat("x", size))
	for at := 4096; at+len("ſK") < size-64; at += 16384 {
		copy(data[at:], "ſK")
	}
	return string(data)
}

// plant returns corpus with occ case-flipped copies of needle spliced in at
// even spacing, so hit scenarios have known, realistic density.
func plant(corpus, needle string, occ int) string {
	rng := rand.New(rand.NewPCG(7, 7))
	if occ <= 0 {
		return corpus
	}
	step := len(corpus) / (occ + 1)
	var b strings.Builder
	b.Grow(len(corpus) + occ*len(needle))
	prev := 0
	for i := 1; i <= occ; i++ {
		pos := i * step
		for pos > prev && corpus[pos]&0xC0 == 0x80 { // rune boundary
			pos--
		}
		b.WriteString(corpus[prev:pos])
		b.WriteString(flipCases(rng, needle))
		prev = pos
	}
	b.WriteString(corpus[prev:])
	return b.String()
}

// ---- scenario matrix --------------------------------------------------------

type scenario struct {
	name     string
	haystack string
	needle   string
	count    bool // count all (overlap-allowed) occurrences instead of first
	utf8     bool // UTF-8 tier: veloz skipped, tolower excluded from agreement
}

var scenarios = func() []scenario {
	logs1m := buildLogCorpus(1 << 20)
	prose1m := buildProseCorpus(1 << 20)
	code256k := buildCodeCorpus(256 << 10)
	cyr1m := buildCyrillicCorpus(1 << 20)

	return []scenario{
		// ASCII tier: miss-heavy scans, full-haystack throughput.
		{"log_miss_1kb", logs1m[:1024], "fatal panic", false, false},
		{"log_miss_64kb", logs1m[:64<<10], "fatal panic", false, false},
		{"log_miss_1mb", logs1m, "fatal panic", false, false},
		{"prose_miss_1mb", prose1m, "zygomorphic", false, false},
		{"code_miss_256kb", code256k, "goto retryLabel", false, false},

		// Needle-length sweep, all misses, letters included so folding is live.
		{"log_needle3_64kb", logs1m[:64<<10], "vQx", false, false},
		{"log_needle8_64kb", logs1m[:64<<10], "vQxKz9Jw", false, false},
		{"log_needle16_64kb", logs1m[:64<<10], "vQxKz9JwPl2Rt7Ym", false, false},
		{"log_needle32_64kb", logs1m[:64<<10], "vQxKz9JwPl2Rt7YmNb4Cd8Fg1Hk5Ls0Z", false, false},

		// Hits: sparse (first-match latency over distance) and dense (count).
		{"log_hit_sparse_1mb", plant(logs1m, "payment declined by issuer", 16), "payment declined by issuer", true, false},
		{"prose_hit_dense_1mb", prose1m, "The ", true, false},
		{"code_hit_brackets_256kb", code256k, "[keys[i%", true, false},

		// Short-haystack latency (the per-row / per-line call shape).
		{"latency_match_start_1kb", "Needle in front " + prose1m[:1008], "needle in front", false, false},
		{"latency_match_mid_1kb", prose1m[:500] + "NeEdLe MiDwAy" + prose1m[500:1011], "needle midway", false, false},
		{"latency_match_end_1kb", prose1m[:1009] + "NEEDLE AT END", "needle at end", false, false},
		{"latency_miss_1kb", prose1m[:1024], "absent needle", false, false},

		// UTF-8 tier: Cyrillic scans and fold-hazard scenarios.
		{"ru_miss_1mb", cyr1m, "яростный дракон", false, true},
		{"ru_hit_sparse_1mb", plant(cyr1m, "Шерлок Холмс", 16), "шерлок холмс", true, true},
		{"kelvin_hazard_1mb", plant(prose1m, "\u212Aelvin", 16), "kelvin", true, true},
		{"ru_latency_miss_1kb", cyr1m[:1024], "яростный дракон", false, true},

		// Adversarial: repetitive structure, near-matches, quadratic traps.
		{"periodic_miss_64kb", strings.Repeat("ab", 32<<10), "abababababababac", false, false},
		{"samechar_miss_64kb", strings.Repeat("a", 64<<10), "aaaaaaaaaaaaaaab", false, false},
		{"torture_miss_64kb", strings.Repeat(strings.Repeat("a", 31)+"b", 2048), strings.Repeat("a", 32), false, false},
	}
}()

// asciiPartitionScenario is a focused field comparison for the partitioned
// executor on sparse valid-UTF-8 exception clusters. It stays outside the
// acceptance matrix because the row targets this mechanism's field gap.
var asciiPartitionScenario = scenario{
	name:     "ascii_partition_sparse_utf8_1mb",
	haystack: buildASCIIOnlyPartitionCorpus(1 << 20),
	needle:   "Sherlock Holmes",
	utf8:     true,
}

var singleScenarios = append(append([]scenario(nil), scenarios...), asciiPartitionScenario)

// ---- implementations under test ----------------------------------------------

func indexToLower(h, n string) int {
	return strings.Index(strings.ToLower(h), strings.ToLower(n))
}

var regexpCache = func() map[string]*regexp.Regexp {
	m := make(map[string]*regexp.Regexp, len(singleScenarios))
	for _, s := range singleScenarios {
		if _, ok := m[s.needle]; !ok {
			m[s.needle] = regexp.MustCompile(`(?i)` + regexp.QuoteMeta(s.needle))
		}
	}
	return m
}()

func indexRegexp(h, n string) int {
	loc := regexpCache[n].FindStringIndex(h)
	if loc == nil {
		return -1
	}
	return loc[0]
}

// impl is one single-needle implementation on the scenario matrix.
type impl struct {
	name      string
	index     func(h, n string) int
	asciiOnly bool // skip entirely on UTF-8 tier scenarios
	foldExact bool // exact on its declared tier: counts toward x_vs_best there
}

// supports reports whether the implementation runs on the scenario's tier.
func (im impl) supports(s scenario) bool { return !s.utf8 || !im.asciiOnly }

var impls = func() []impl {
	out := []impl{
		{"candidate", casei.IndexFold, false, true},
		{"tolower", indexToLower, false, false}, // agreement on ASCII tier only: ToLower is not folding
		{"regexp", indexRegexp, false, true},
		{"pcre2-jit", indexPCRE2, false, true},
		{"rure", indexRure, false, true},
		{"vectorscan", indexVectorscan, false, true},
	}
	if stringZillaAvailable {
		out = append(out, impl{"stringzilla", indexStringZilla, false, true})
	}
	// Veloz's source uses AVX2 when available and falls back below it. The
	// weaker implementation is excluded instead of being raced under the same
	// entrant name.
	if velozVectorBits() == 256 {
		out = append(out, impl{"veloz", veloz.IndexFold, true, true})
	}
	return append(out, impl{"rustac", indexRustAC, true, true})
}()

// countAll counts overlap-allowed occurrences by repeated first-match calls,
// so every implementation is measured through the same access pattern.
func countAll(index func(h, n string) int, h, n string) int {
	c, off := 0, 0
	for off <= len(h) {
		i := index(h[off:], n)
		if i < 0 {
			return c
		}
		c++
		off += i + 1
	}
	return c
}

// runSingleScenario is the shared single-needle operation for both benchmark
// surfaces. Count rows must measure all overlap-allowed occurrences; every
// other row measures the first match.
func runSingleScenario(index func(h, n string) int, s scenario) int {
	if s.count {
		return countAll(index, s.haystack, s.needle)
	}
	return index(s.haystack, s.needle)
}

func TestRunSingleScenarioHonorsCount(t *testing.T) {
	s := scenario{haystack: "aaaa", needle: "aa", count: true}
	if got := runSingleScenario(strings.Index, s); got != 3 {
		t.Fatalf("count scenario = %d, want 3", got)
	}
	s.count = false
	if got := runSingleScenario(strings.Index, s); got != 0 {
		t.Fatalf("first-match scenario = %d, want 0", got)
	}
}

// TestBaselinesAgree pins every implementation to the reference on the whole
// scenario matrix, so a benchmark win can never come from semantic drift.
// On the UTF-8 tier only fold-exact implementations are held to it.
func TestBaselinesAgree(t *testing.T) {
	for _, s := range singleScenarios {
		want := reference(s.haystack, s.needle)
		wantCount := 0
		if s.count {
			wantCount = countAll(reference, s.haystack, s.needle)
		}
		for _, im := range impls {
			if !im.supports(s) || s.utf8 && !im.foldExact {
				continue
			}
			if got := im.index(s.haystack, s.needle); got != want {
				t.Errorf("%s/%s: first = %d, want %d", s.name, im.name, got, want)
			}
			if s.count {
				if got := countAll(im.index, s.haystack, s.needle); got != wantCount {
					t.Errorf("%s/%s: count = %d, want %d", s.name, im.name, got, wantCount)
				}
			}
		}
	}
}

func BenchmarkIndexFold(b *testing.B) {
	for _, s := range scenarios {
		for _, im := range impls {
			if !im.supports(s) {
				continue
			}
			b.Run(s.name+"/"+im.name, func(b *testing.B) {
				b.SetBytes(int64(len(s.haystack)))
				b.ReportAllocs()
				for b.Loop() {
					sink = runSingleScenario(im.index, s)
				}
			})
		}
		// The exact-match ceiling: same scenario, folding pre-paid outside
		// the timed region (ASCII fold on the ASCII tier, canonical simple
		// fold on the UTF-8 tier). This is the physics target the winning
		// implementation is judged against (see CONTEXT.md).
		ceiling := s
		if s.utf8 {
			ceiling.haystack = board.FoldString(s.haystack)
			ceiling.needle = board.FoldString(s.needle)
		} else {
			ceiling.haystack = asciiLower(s.haystack)
			ceiling.needle = asciiLower(s.needle)
		}
		b.Run(s.name+"/ceiling", func(b *testing.B) {
			b.SetBytes(int64(len(ceiling.haystack)))
			for b.Loop() {
				sink = runSingleScenario(strings.Index, ceiling)
			}
		})
	}
}

var sink int

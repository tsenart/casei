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
//   rure       - precompiled rust-regex C API (?i) literal, kept for Unicode
//                simple-fold agreement and admitted to the field only after its
//                exact query observes the audited memchr AVX2 backend
//   vectorscan - precompiled Vectorscan caseless UTF-8 literal set, with a
//                leftmost/lowest-ID adapter timed with each scan
//   stringzilla - precompiled StringZilla full-fold literal, with timed
//                simple-fold verification and multi-pattern reduction
//   veloz      - github.com/mhr3/veloz/ascii.IndexFold (ASCII tier only)
//   ceiling    - strings.Index on pre-folded input: exact-match physics,
//                what caseless search costs if folding were free

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	veloz "github.com/mhr3/veloz/ascii"

	"github.com/tsenart/casei"
)

// ---- corpora (deterministic; no testdata files) ----------------------------

func corpusRNG() *rand.Rand { return rand.New(rand.NewPCG(0xCA5E1, 0xA12E9A)) }

var logLevels = []string{"DEBUG", "INFO", "INFO", "INFO", "WARN", "ERROR"}
var logServices = []string{"checkout", "search", "ingest", "billing", "Gateway", "AuthZ", "replicator"}
var logMessages = []string{
	"request completed", "cache miss", "retry scheduled", "connection reset by peer",
	"slow query detected", "Payment Authorized", "token refreshed", "queue depth high",
	"compaction finished", "TLS handshake complete", "rate limit applied",
}

func buildLogCorpus(size int) string {
	rng := corpusRNG()
	var b strings.Builder
	b.Grow(size + 256)
	for b.Len() < size {
		fmt.Fprintf(&b, "2026-08-04T%02d:%02d:%02d.%03dZ %s service=%s region=eu-west-%d trace=%08x%08x msg=%q latency_ms=%d\n",
			rng.IntN(24), rng.IntN(60), rng.IntN(60), rng.IntN(1000),
			logLevels[rng.IntN(len(logLevels))],
			logServices[rng.IntN(len(logServices))],
			1+rng.IntN(3), rng.Uint32(), rng.Uint32(),
			logMessages[rng.IntN(len(logMessages))],
			rng.IntN(2000))
	}
	return b.String()[:size]
}

var proseWords = strings.Fields(`the of and to in a is that it was for on are with as his they at be
this have from or one had by word but not what all were we when your can said there use an each which
she do how their if will up other about out many then them these so some her would make like him into
time has look two more write go see number no way could people my than first water been call who oil
its now find long down day did get come made may part over new sound take only little work know place
year live me back give most very after thing our just name good sentence man think say great where
help through much before line right too mean old any same tell boy follow came want show also around
form three small set put end does another well large must big even such because turn here why ask went
men read need land different home us move try kind hand picture again change off play spell air away
animal house point page letter mother answer found study still learn should America world`)

var cyrillicWords = strings.Fields(`доктор ватсон улица бейкер лондон туман дело улика письмо газета
вечер утро дверь окно комната огонь свеча тень шаг голос вопрос ответ время город река мост камень
дождь ветер ночь свет тайна встреча друг враг правда история конец начало Инспектор Лестрейд`)

func buildWordCorpus(words []string, size int) string {
	rng := corpusRNG()
	var b strings.Builder
	b.Grow(size + 128)
	for b.Len() < size {
		n := 6 + rng.IntN(9)
		for i := 0; i < n; i++ {
			w := words[rng.IntN(len(words))]
			if i == 0 {
				r, sz := utf8.DecodeRuneInString(w)
				w = string(unicode.ToUpper(r)) + w[sz:]
			}
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(w)
		}
		b.WriteString(". ")
	}
	s := b.String()
	// Trim to size at a rune boundary so corpora stay valid UTF-8.
	for size > 0 && size < len(s) && s[size]&0xC0 == 0x80 {
		size--
	}
	return s[:size]
}

func buildProseCorpus(size int) string { return buildWordCorpus(proseWords, size) }

func buildCodeCorpus(size int) string {
	rng := corpusRNG()
	var b strings.Builder
	b.Grow(size + 256)
	for b.Len() < size {
		id := rng.IntN(10000)
		fmt.Fprintf(&b, "func handleReq%d(ctx context.Context, in []byte) (map[string]int, error) {\n", id)
		fmt.Fprintf(&b, "\tout := make(map[string]int, %d)\n\tfor i, v := range in {\n", rng.IntN(64))
		fmt.Fprintf(&b, "\t\tif v&0x%02x != 0 {\n\t\t\tout[keys[i%%%d]] += int(v)\n\t\t}\n\t}\n", rng.IntN(256), 1+rng.IntN(16))
		fmt.Fprintf(&b, "\tif len(out) == 0 {\n\t\treturn nil, fmt.Errorf(\"empty%d: %%w\", errSentinel)\n\t}\n\treturn out, nil\n}\n\n", id)
	}
	return b.String()[:size]
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
	cyr1m := buildWordCorpus(cyrillicWords, 1<<20)

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

// ---- implementations under test ----------------------------------------------

func indexToLower(h, n string) int {
	return strings.Index(strings.ToLower(h), strings.ToLower(n))
}

var regexpCache = func() map[string]*regexp.Regexp {
	m := make(map[string]*regexp.Regexp, len(scenarios))
	for _, s := range scenarios {
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

var impls = func() []struct {
	name      string
	index     func(h, n string) int
	asciiOnly bool // skip entirely on UTF-8 tier scenarios
	utf8Only  bool // skip entirely on ASCII tier scenarios
	foldExact bool // implements simple folding on its declared tier
} {
	out := []struct {
		name      string
		index     func(h, n string) int
		asciiOnly bool
		utf8Only  bool
		foldExact bool
	}{
		{"candidate", casei.IndexFold, false, false, true},
		{"tolower", indexToLower, false, false, false}, // agreement on ASCII tier only: ToLower is not folding
		{"regexp", indexRegexp, false, false, true},
		{"pcre2-jit", indexPCRE2, false, false, true},
		{"rure", indexRure, false, false, true},
		{"vectorscan", indexVectorscan, false, false, true},
	}
	if stringZillaAvailable {
		out = append(out, struct {
			name      string
			index     func(h, n string) int
			asciiOnly bool
			utf8Only  bool
			foldExact bool
		}{"stringzilla", indexStringZilla, false, false, true})
	}
	// Veloz's source uses AVX2 when available and falls back below it. The
	// weaker implementation is excluded instead of being raced under the same
	// entrant name.
	if velozVectorBits() == 256 {
		out = append(out, struct {
			name      string
			index     func(h, n string) int
			asciiOnly bool
			utf8Only  bool
			foldExact bool
		}{"veloz", veloz.IndexFold, true, false, false})
	}
	return out
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
	for _, s := range scenarios {
		want := reference(s.haystack, s.needle)
		wantCount := 0
		if s.count {
			wantCount = countAll(reference, s.haystack, s.needle)
		}
		for _, im := range impls {
			if (s.utf8 && (im.asciiOnly || !im.foldExact)) || (!s.utf8 && im.utf8Only) {
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
			if (s.utf8 && im.asciiOnly) || (!s.utf8 && im.utf8Only) {
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
			ceiling.haystack = canonFoldString(s.haystack)
			ceiling.needle = canonFoldString(s.needle)
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

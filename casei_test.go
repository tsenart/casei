package casei

import (
	"math/rand/v2"
	"regexp"
	"strings"
	"sync"
	"testing"
	"unicode"
	"unicode/utf8"
)

// ---- independent reference implementation -----------------------------------

// orbitMin maps a rune to the smallest member of its simple-fold orbit, so
// fold-equality becomes plain equality of canonical forms.
func orbitMin(r rune) rune {
	m := r
	for x := unicode.SimpleFold(r); x != r; x = unicode.SimpleFold(x) {
		if x < m {
			m = x
		}
	}
	return m
}

// canonFold decodes s into canonical fold form. Opaque (invalid-encoding)
// bytes become distinct negative sentinels so they compare byte-exactly and
// can never collide with a real rune. offs[i] is the byte offset in s of
// canonical element i; a final entry holds len(s).
func canonFold(s string) (canon []rune, offs []int) {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			canon = append(canon, -rune(s[i])-1)
		} else {
			canon = append(canon, orbitMin(r))
		}
		offs = append(offs, i)
		i += size
	}
	offs = append(offs, len(s))
	return canon, offs
}

// reference is a second, structurally different implementation: canonical
// fold both strings, then exact slice search. Every implementation in this
// repository must agree with it on every input.
func reference(haystack, needle string) int {
	if len(needle) == 0 {
		return 0
	}
	ch, offs := canonFold(haystack)
	cn, _ := canonFold(needle)
	for i := 0; i+len(cn) <= len(ch); i++ {
		match := true
		for j := range cn {
			if ch[i+j] != cn[j] {
				match = false
				break
			}
		}
		if match {
			return offs[i]
		}
	}
	return -1
}

// ---- trap table --------------------------------------------------------------

var trapCases = []struct {
	name     string
	haystack string
	needle   string
	want     int
}{
	{"empty needle", "abc", "", 0},
	{"empty both", "", "", 0},
	{"needle longer", "ab", "abc", -1},
	{"exact", "hello world", "world", 6},
	{"fold simple", "Hello World", "world", 6},
	{"fold needle upper", "hello world", "WORLD", 6},
	{"whole string case diff", "HELLO", "hello", 0},
	{"match at start", "Foobar", "foo", 0},
	{"match at end", "barFOO", "foo", 3},

	// 0x20-adjacent ASCII punctuation pairs must NOT fold.
	{"bracket brace", "[hello]", "{hello}", -1},
	{"brace bracket upper", "{hello}", "[HELLO]", -1},
	{"at backtick", "@X@x", "`x", -1},
	{"backtick literal", "@X`x", "`x", 2},
	{"backslash pipe", `a\b`, "a|b", -1},
	{"caret tilde", "a^b", "a~b", -1},
	{"bracket exact inside needle", "fn[T any](x)", "FN[t ANY](X)", 0},
	{"brace does not match bracket needle", "fn{T any}(x)", "fn[T any](x)", -1},

	// Unicode simple-fold orbits.
	{"latin1 pair folds", "naïve", "Ï", 2}, // ï matches Ï
	{"cyrillic", "Шерлок", "шерлок", 0},    // Шерлок / шерлок
	{"kelvin sign in haystack", "xxKelvin", "kelvin", 2},
	{"kelvin sign in needle", "5 kelvin", "Kelvin", 2},
	{"kelvin window longer than needle", "abKelvincd", "kelvin", 2},
	{"long s", "ſecret", "SECRET", 0},
	{"angstrom sign", "1Å", "1å", 0},
	{"micro sign", "5µs", "5μS", 0},
	{"sigma trio", "τέλος", "ΤΈΛΟΣ", 0}, // τέλος / ΤΈΛΟΣ
	{"sharp s capital", "große", "GROẞE", 0},
	{"sharp s is not ss", "große", "GROSSE", -1},
	{"dotted capital I folds only to itself", "İstanbul", "istanbul", -1},
	{"dotless i folds only to itself", "ı", "I", -1},

	// Invalid UTF-8 bytes are opaque: exact match only, never folded.
	{"opaque byte exact", "\x80abc", "\x80a", 0},
	{"opaque pair not folded", "\x80abc", "\xa0A", -1},
	{"opaque byte with letter fold", "\x80ABC", "\x80abc", 0},
	{"lone continuation vs kelvin bytes", "K", "\x84", -1}, // no mid-rune starts

	// Long needles with the case difference in the tail (the class of bug a
	// scalar-tail verify path gets wrong: see CONTEXT.md, "known traps").
	{"tail case diff 17B match", strings.Repeat("x", 100) + "abcdefghijklmnopQ", "abcdefghijklmnopq", 100},
	{"tail mismatch 17B", strings.Repeat("x", 100) + "abcdefghijklmnopQ", "abcdefghijklmnopr", -1},

	// Candidate abutting the very end of the haystack.
	{"match flush at end", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzHELLO", "hello", 32},
	{"near miss at end", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzHELL", "hello", -1},

	// Periodic / repetitive inputs (adaptive-filter and budget territory).
	{"periodic hit", strings.Repeat("ab", 1000) + "abc", "ABC", 2000},
	{"periodic miss", strings.Repeat("ab", 1000), "abc", -1},
	{"samechar hit", strings.Repeat("a", 500) + "b", strings.Repeat("A", 4) + "B", 496},
	{"overlapping prefix", "aaab", "AAB", 1},
}

func TestIndexFoldTraps(t *testing.T) {
	for _, tc := range trapCases {
		if got := IndexFold(tc.haystack, tc.needle); got != tc.want {
			t.Errorf("%s: IndexFold(%q, %q) = %d, want %d", tc.name, tc.haystack, tc.needle, got, tc.want)
		}
		// The trap table itself must agree with the independent reference.
		if ref := reference(tc.haystack, tc.needle); ref != tc.want {
			t.Fatalf("%s: trap table wrong: reference = %d, table says %d", tc.name, ref, tc.want)
		}
	}
}

// ---- differential against Go regexp (?i): the semantic anchor ----------------

// regexp with (?i) implements exactly Unicode simple case folding, so on
// valid UTF-8 the arena's semantics are pinned to the standard library.
var diffRunes = []rune{
	'a', 'z', 'A', 'Z', 'k', 'K', 's', 'S', '0', ' ', '[', '{', '@', '`',
	'K', 'ſ', 'ß', 'ẞ', 'µ', 'μ', 'Μ',
	'σ', 'ς', 'Σ', 'İ', 'ı', 'é', 'É',
	'ш', 'Ш', 'Å', 'å', '汉',
}

func randomRuneString(rng *rand.Rand, maxLen int) string {
	n := rng.IntN(maxLen + 1)
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteRune(diffRunes[rng.IntN(len(diffRunes))])
	}
	return b.String()
}

func TestIndexFoldCachesCompiledPlan(t *testing.T) {
	needle := "\x00compiled-plan-cache"
	haystack := strings.Repeat("x", 80)
	slot := &singlePlanCache[singlePlanCacheIndex(needle)]
	savedSlot, savedRecent := slot.Load(), recentSinglePlan.Load()
	slot.Store(nil)
	recentSinglePlan.Store(nil)
	t.Cleanup(func() {
		slot.Store(savedSlot)
		recentSinglePlan.Store(savedRecent)
	})

	want := reference(haystack, needle)
	if got := IndexFold(haystack, needle); got != want {
		t.Fatalf("IndexFold = %d, want %d", got, want)
	}
	entry := slot.Load()
	if entry == nil || entry.needle != needle || entry.plan == nil {
		t.Fatalf("IndexFold did not cache a compiled plan: %+v", entry)
	}
	if recent := recentSinglePlan.Load(); recent != entry {
		t.Fatalf("recent cache = %+v, want %+v", recent, entry)
	}
}

func TestIndexFoldConcurrentCache(t *testing.T) {
	cases := []struct {
		haystack string
		needle   string
		want     int
	}{
		{"xxAlpha", "alpha", 2},
		{"xxBRAVO", "bravo", 2},
		{"xxKelvin", "kelvin", 2},
		{"xx\x80opaque", "\x80opaque", 2},
		{"no match", "needle", -1},
	}
	const callers = 16
	start := make(chan struct{})
	errs := make(chan string, callers)
	var group sync.WaitGroup
	for caller := 0; caller < callers; caller++ {
		group.Add(1)
		go func(offset int) {
			defer group.Done()
			<-start
			for i := 0; i < 100; i++ {
				tc := cases[(offset+i)%len(cases)]
				if got := IndexFold(tc.haystack, tc.needle); got != tc.want {
					errs <- tc.needle
					return
				}
			}
		}(caller)
	}
	close(start)
	group.Wait()
	close(errs)
	if needle, ok := <-errs; ok {
		t.Fatalf("concurrent IndexFold returned a wrong result for %q", needle)
	}
}

func TestIndexFoldMatchesRegexp(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260804, 7))
	for i := 0; i < 8000; i++ {
		h := randomRuneString(rng, 24)
		n := randomRuneString(rng, 6)
		re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(n))
		want := -1
		if loc := re.FindStringIndex(h); loc != nil {
			want = loc[0]
		}
		if got := IndexFold(h, n); got != want {
			t.Fatalf("iter %d: IndexFold(%q, %q) = %d, regexp says %d", i, h, n, got, want)
		}
	}
}

// ---- randomized differential against the reference (arbitrary bytes) ---------

var byteAlphabet = []byte("aAzZ mM09[{]}@`\\|^~\x80\xc3\xaf\x8f\xe2\x84\xaa")

func randomBytes(rng *rand.Rand, maxLen int) string {
	n := rng.IntN(maxLen + 1)
	b := make([]byte, n)
	for i := range b {
		b[i] = byteAlphabet[rng.IntN(len(byteAlphabet))]
	}
	return string(b)
}

func TestIndexFoldMatchesReferenceRandom(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260804, 42))
	for i := 0; i < 50000; i++ {
		h := randomBytes(rng, 48)
		n := randomBytes(rng, 8)
		got, want := IndexFold(h, n), reference(h, n)
		if got != want {
			t.Fatalf("iter %d: IndexFold(%q, %q) = %d, want %d", i, h, n, got, want)
		}
	}
	// A second pass planting a case-mangled needle so hits are common.
	for i := 0; i < 20000; i++ {
		n := randomRuneString(rng, 8)
		if n == "" {
			continue
		}
		h := randomBytes(rng, 24) + flipCases(rng, n) + randomBytes(rng, 24)
		got, want := IndexFold(h, n), reference(h, n)
		if got != want {
			t.Fatalf("planted iter %d: IndexFold(%q, %q) = %d, want %d", i, h, n, got, want)
		}
	}
}

// flipCases randomly flips ASCII letter case (planting helper; fold-neutral).
func flipCases(rng *rand.Rand, s string) string {
	b := []byte(s)
	for i, c := range b {
		if rng.IntN(2) == 0 {
			continue
		}
		switch {
		case 'a' <= c && c <= 'z':
			b[i] = c - 0x20
		case 'A' <= c && c <= 'Z':
			b[i] = c + 0x20
		}
	}
	return string(b)
}

func FuzzIndexFold(f *testing.F) {
	f.Add("Hello World", "world")
	f.Add("[hello]", "{hello}")
	f.Add("5 Kelvin", "kelvin")
	f.Add("ſecret ςΣ", "secret σσ")
	f.Add("große", "GROSSE")
	f.Add(strings.Repeat("ab", 64), "abc")
	f.Add("na\xc3\xafve", "\xc3\x8f")
	f.Add(strings.Repeat("x", 64)+"ПРИКЛЮЧЕНИЯ ЛИЛИЙ"+strings.Repeat("x", 4096), "приключения лилий")
	f.Add(strings.Repeat("x", 64)+"приключения лилия"+"x"+"ПРИКЛЮЧЕНИЯ ЛИЛИЙ"+strings.Repeat("x", 4096), "приключения лилий")
	f.Add(strings.Repeat("x", 64)+"\x80ПРИКЛЮЧЕНИЯ ЛИЛИЙ"+strings.Repeat("x", 4096), "\x80приключения лилий")
	f.Fuzz(func(t *testing.T, haystack, needle string) {
		got, want := IndexFold(haystack, needle), reference(haystack, needle)
		if got != want {
			t.Fatalf("IndexFold(%q, %q) = %d, want %d", haystack, needle, got, want)
		}
		if utf8.ValidString(haystack) && utf8.ValidString(needle) {
			re, err := regexp.Compile(`(?i)` + regexp.QuoteMeta(needle))
			if err == nil {
				reWant := -1
				if loc := re.FindStringIndex(haystack); loc != nil {
					reWant = loc[0]
				}
				if got != reWant {
					t.Fatalf("IndexFold(%q, %q) = %d, regexp says %d", haystack, needle, got, reWant)
				}
			}
		}
	})
}

func TestContainsFold(t *testing.T) {
	for _, tc := range []struct {
		name             string
		haystack, needle string
		want             bool
	}{
		{"ascii", "Payment Declined by issuer", "payment declined", true},
		{"kelvin sign folds to k", "temperature 273\u212A today", "273k", true},
		{"long s folds to s", "\u017Fecret", "secret", true},
		{"final sigma orbit", "\u039F\u0394\u039F\u03A3", "odo\u03C2", false},
		{"greek folds", "\u03A3\u03BF\u03C6\u03AF\u03B1", "\u03C3\u03BF\u03C6\u03AF\u03B1", true},
		{"no full folding", "stra\u00DFe", "strasse", false},
		{"sharp s orbit", "stra\u00DFe", "STRA\u1E9EE", true},
		{"absent", "nothing here", "absent", false},
		{"empty needle", "anything", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ContainsFold(tc.haystack, tc.needle)
			if got != tc.want {
				t.Errorf("ContainsFold(%q, %q) = %v, want %v", tc.haystack, tc.needle, got, tc.want)
			}
			if want := IndexFold(tc.haystack, tc.needle) >= 0; got != want {
				t.Errorf("ContainsFold(%q, %q) = %v, disagrees with IndexFold >= 0 (%v)", tc.haystack, tc.needle, got, want)
			}
		})
	}
}

func TestBoundedSearchesAllocateNothing(t *testing.T) {
	haystack := strings.Repeat("payment accepted ", 64)
	needle := "payment declined"

	// Warm the direct-mapped single-pattern cache before measuring its hot path.
	if got := IndexFold(haystack, needle); got != -1 {
		t.Fatalf("IndexFold warmup = %d, want -1", got)
	}
	if got := testing.AllocsPerRun(100, func() {
		if at := IndexFold(haystack, needle); at != -1 {
			t.Fatalf("IndexFold = %d, want -1", at)
		}
	}); got != 0 {
		t.Fatalf("cache-hit IndexFold allocations = %g, want 0", got)
	}

	matcher := NewMatcher([]string{"payment declined", "fatal panic", "oom killed"})
	if got := testing.AllocsPerRun(100, func() {
		if match, ok := matcher.Find(haystack); ok {
			t.Fatalf("Find = %+v,true, want no match", match)
		}
	}); got != 0 {
		t.Fatalf("bounded Matcher.Find allocations = %g, want 0", got)
	}
}

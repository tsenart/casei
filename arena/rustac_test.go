package arena_test

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	"golang.org/x/sys/cpu"

	rustac "github.com/tsenart/casei/arena/rustac"
)

type rustACMatcher = rustac.Matcher

func newRustACAlternation(patterns []string) (*rustACMatcher, error) {
	return rustac.CompileAlternation(patterns)
}

var rustACAlts = func() []*rustACMatcher {
	out := make([]*rustACMatcher, len(multiScenarios))
	for i, s := range multiScenarios {
		if s.utf8 {
			continue
		}
		m, err := newRustACAlternation(s.patterns)
		if err != nil {
			panic(fmt.Sprintf("Rust Aho-Corasick baseline unavailable for %s: %v", s.name, err))
		}
		out[i] = m
	}
	return out
}()

// rustACSingles holds one compiled matcher per needle of the ASCII-tier single
// scenarios. The entrant is exact for ASCII only (field.yaml), so UTF-8
// scenarios never reach it.
var rustACSingles = func() map[string]*rustACMatcher {
	out := make(map[string]*rustACMatcher)
	for _, s := range singleScenarios {
		if _, ok := out[s.needle]; !ok && !s.utf8 {
			m, err := rustac.Compile([]string{s.needle})
			if err != nil {
				panic(fmt.Sprintf("Rust Aho-Corasick baseline unavailable for %q: %v", s.needle, err))
			}
			out[s.needle] = m
		}
	}
	return out
}()

// indexRustAC answers a single-needle scenario with the Rust DFA's leftmost
// match.
func indexRustAC(haystack, needle string) int {
	m := rustACSingles[needle]
	if m == nil {
		panic(fmt.Sprintf("Rust Aho-Corasick baseline was not compiled for %q", needle))
	}
	if start, _, ok := m.Find(haystack); ok {
		return start
	}
	return -1
}

func TestRustACAlternationAgree(t *testing.T) {
	cases := []struct {
		haystack string
		patterns []string
	}{
		{"xxABCxx", []string{"abc", "ABC"}},
		{"xxabc xx abc", []string{"abc", "xxabc"}},
		{"x", []string{"later", "", "x"}},
		{"x", []string{"x", "", "later"}},
		{"", []string{"", "later"}},
		{"anything", nil},
	}
	for _, tc := range cases {
		m, err := newRustACAlternation(tc.patterns)
		if err != nil {
			t.Fatalf("CompileAlternation(%q): %v", tc.patterns, err)
		}
		start, pattern, ok := m.Find(tc.haystack)
		want, wantOK := refFind(tc.haystack, tc.patterns)
		if ok != wantOK || (ok && (start != want.Start || pattern != want.Pattern)) {
			t.Errorf("Find(%q, %q) = {%d %d},%v want %+v,%v", tc.haystack, tc.patterns, pattern, start, ok, want, wantOK)
		}
	}

	rng := rand.New(rand.NewPCG(20260912, 31))
	alphabet := []byte("aAzZqQwW0123 []{}@`")
	for iteration := 0; iteration < 1000; iteration++ {
		haystack := make([]byte, rng.IntN(128))
		for i := range haystack {
			haystack[i] = alphabet[rng.IntN(len(alphabet))]
		}
		patterns := make([]string, rng.IntN(8))
		for p := range patterns {
			needle := make([]byte, rng.IntN(16))
			for i := range needle {
				needle[i] = alphabet[rng.IntN(len(alphabet))]
			}
			patterns[p] = string(needle)
		}
		m, err := newRustACAlternation(patterns)
		if err != nil {
			t.Fatalf("iteration %d CompileAlternation: %v", iteration, err)
		}
		start, pattern, ok := m.Find(string(haystack))
		want, wantOK := refFind(string(haystack), patterns)
		if ok != wantOK || (ok && (start != want.Start || pattern != want.Pattern)) {
			t.Fatalf("iteration %d Find(%q, %q) = {%d %d},%v want %+v,%v", iteration, haystack, patterns, pattern, start, ok, want, wantOK)
		}
	}
}

func TestRustACDispatchWidth(t *testing.T) {
	if _, err := newRustACAlternation([]string{"Kelvin"}); err == nil {
		t.Fatal("Rust Aho-Corasick accepted a non-ASCII pattern")
	}
	m, err := newRustACAlternation([]string{"casei-prefilter-needle"})
	if err != nil {
		t.Fatal(err)
	}
	_, _, _ = m.Find(strings.Repeat("x", 64<<10))
	got := m.VectorBits()
	if got != 0 && got != 128 && got != 256 {
		t.Fatalf("Rust Aho-Corasick audited vector width = %d, want 0, 128, or 256", got)
	}
	if cpu.X86.HasAVX2 && got != 256 {
		t.Fatalf("Rust Aho-Corasick prefilter width = %d, want AVX2 256", got)
	}
}

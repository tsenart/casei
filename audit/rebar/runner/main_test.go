package main

import (
	"testing"

	"github.com/tsenart/casei"
)

func TestLiteralAlternation(t *testing.T) {
	got, err := literalAlternation([]string{"Sherlock|Holmes|Шерлок"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Sherlock", "Holmes", "Шерлок"}
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %q, want %q", got, want)
		}
	}

	for _, regex := range []string{"Sher[a-z]+", "a||b", "a\\|b"} {
		if _, err := literalAlternation([]string{regex}); err == nil {
			t.Fatalf("accepted non-literal alternation %q", regex)
		}
	}
}

func TestFoldPrefixWidth(t *testing.T) {
	for _, tt := range []struct {
		haystack string
		pattern  string
		width    int
		ok       bool
	}{
		{"Kelvin", "k", 3, true},
		{"ſuffix", "S", 2, true},
		{"ς", "Σ", 2, true},
		{"x", "s", 0, false},
		{string([]byte{0xff}), string([]byte{0xff}), 1, true},
		{string([]byte{0xfe}), string([]byte{0xff}), 0, false},
	} {
		width, ok := foldPrefixWidth(tt.haystack, tt.pattern)
		if width != tt.width || ok != tt.ok {
			t.Errorf("foldPrefixWidth(%q, %q) = (%d, %v), want (%d, %v)",
				tt.haystack, tt.pattern, width, ok, tt.width, tt.ok)
		}
	}
}

func TestCountMatches(t *testing.T) {
	patterns := []string{"ss", "s"}
	matcher := casei.NewMatcher(patterns)
	for _, tt := range []struct {
		spans bool
		want  int
	}{
		{false, 2},
		{true, 5},
	} {
		got, err := countMatches("SSſs", patterns, matcher, tt.spans)
		if err != nil {
			t.Fatal(err)
		}
		if got != tt.want {
			t.Fatalf("countMatches(spans=%v) = %d, want %d", tt.spans, got, tt.want)
		}
	}
}

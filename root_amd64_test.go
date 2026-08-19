//go:build amd64

package casei

import (
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/cpu"
)

func TestASCIIPairShortSkip64TailPosition(t *testing.T) {
	if !cpu.X86.HasAVX512F || !cpu.X86.HasAVX512BW {
		t.Skip("AVX-512 BW pair path is disabled")
	}

	plan := newSearchPlan([]string{"fatal panic"})
	probe := &plan.asciiPair
	if !probe.usable() {
		t.Fatal("fixed long ASCII literal did not compile an aligned pair probe")
	}

	for _, prefix := range []int{0, 128} {
		candidates := prefix + 64
		for lane := 0; lane < 64; lane++ {
			want := prefix + lane
			input := []byte(strings.Repeat("x", candidates+64))
			input[want] = probe.first
			input[want+probe.secondAt] = probe.second
			haystack := string(input)

			if got := asciiPairShortSkip64(unsafe.StringData(haystack), candidates, probe); got != want {
				t.Fatalf("prefix %d lane %d: direct skip = %d, want %d", prefix, lane, got, want)
			}
			if got := asciiPairSkipBytes(haystack, 0, candidates, probe); got != want {
				t.Fatalf("prefix %d lane %d: wrapped skip = %d, want %d", prefix, lane, got, want)
			}
		}
	}
}

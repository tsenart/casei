//go:build amd64

package casei

import (
	"bytes"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/cpu"
)

func TestTripleShuftiExactRouteAndStops(t *testing.T) {
	if runtimeVectorBits() != 512 {
		t.Skip("AVX-512 triple Shufti route is disabled")
	}
	haystack := tripleShuftiEachCorpus()
	base := tripleShuftiEachPatterns
	type result struct {
		match Match
		width int
	}
	var explicitRoutes, normalizedStops, explicitStops uint64
	for rotation := range 3 {
		patterns := make([]string, len(base))
		for i := range patterns {
			patterns[i] = base[(i+rotation)%len(base)]
		}
		for _, mode := range []string{"generic", "normalized", "explicit"} {
			matcher := tripleShuftiProofMatcher(patterns, mode)
			var got []result
			if !matcher.Each(haystack, func(match Match, width int) bool {
				got = append(got, result{match: match, width: width})
				return true
			}) {
				t.Fatalf("rotation %d %s: Each stopped", rotation, mode)
			}
			wantPattern := (len(base) - rotation) % len(base)
			if len(got) != 1 || got[0].match.Start != len(haystack)-3 ||
				got[0].match.Pattern != wantPattern || got[0].width != 3 {
				t.Fatalf("rotation %d %s: got %+v", rotation, mode, got)
			}
			if mode == "normalized" {
				counted := tripleShuftiProofCount(haystack, &matcher.plan.asciiTriples.shufti)
				normalizedStops += counted.stops
			}
			if mode == "explicit" {
				counted := tripleShuftiProofCount(haystack, &matcher.plan.asciiTriples.shufti)
				if counted.routes == 0 {
					t.Fatalf("rotation %d explicit route was not entered", rotation)
				}
				explicitRoutes += counted.routes
				explicitStops += counted.stops
			}
		}
	}
	if explicitStops >= normalizedStops {
		t.Fatalf("exact stops=%d, normalized stops=%d; exact route did not remove aliases", explicitStops, normalizedStops)
	}
	t.Logf("explicit route/stop counters: routes=%d stops=%d; normalized stops=%d", explicitRoutes, explicitStops, normalizedStops)
}

func TestLiteralSkipExact64MatchesModel(t *testing.T) {
	if !cpu.X86.HasAVX512F || !cpu.X86.HasAVX512BW {
		t.Skip("AVX-512 BW exact-byte path is disabled")
	}
	target := uint64(' ') * byteOnes
	check := func(input []byte, n int) {
		t.Helper()
		full := n &^ 63
		want := bytes.IndexByte(input[:full], ' ')
		if want < 0 {
			want = full
		}
		if got := literalSkipExact64(unsafe.SliceData(input), n, target); got != want {
			t.Fatalf("n=%d: skip=%d want=%d", n, got, want)
		}
	}
	lengths := []int{0, 1, 63, 64, 65, 127, 128, 129, 255, 256, 257, 511, 512, 513, 1023, 1024, 1025, 4095}
	positions := []int{0, 1, 63, 64, 127, 128, 191, 192, 255, 256, 319, 320, 383, 384, 447, 448, 511, 512, 1023, 4094}
	for _, n := range lengths {
		for _, alignment := range []int{0, 1, 31, 63} {
			backing := []byte(strings.Repeat("x", alignment+n+64))
			input := backing[alignment : alignment+n]
			check(input, n)
			for _, at := range positions {
				if at >= n {
					continue
				}
				input[at] = ' '
				check(input, n)
				input[at] = 'x'
			}
		}
	}
}

func TestPairSetSkip(t *testing.T) {
	filter := rootFilter{
		pairs: [16]rootPair{
			{first: 0xce, second: 0xa3},
			{first: 0xd0, second: 0xaf},
		},
		pairN: 2,
	}
	for _, offset := range []int{0, 1, 31, 32, 63, 64, 65, 127, 128, 191} {
		stream := strings.Repeat("x", offset) + "Σ" + strings.Repeat("x", 193)
		got := pairSetSkipBytes(stream, 0, &filter)
		want := filterSkipScalar(stream, 0, &filter)
		if got != want || got != offset {
			t.Fatalf("offset %d: pair-set skip=%d want %d", offset, got, want)
		}
	}
	miss := strings.Repeat("x", 257)
	if got, want := pairSetSkipBytes(miss, 0, &filter), filterSkipScalar(miss, 0, &filter); got != want {
		t.Fatalf("pair-set miss=%d want %d", got, want)
	}
}

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

func TestASCIIPairSkipBytesNonEightDisplacement(t *testing.T) {
	if !cpu.X86.HasAVX512F || !cpu.X86.HasAVX512BW {
		t.Skip("AVX-512 BW pair path is disabled")
	}

	probe := asciiPairProbe{
		first:      's',
		second:     'k',
		firstFold:  0x20,
		secondFold: 0x20,
		secondAt:   7,
	}
	makeASCIIPairVBMIProbe(&probe, "sherlock")
	if probe.vbmi.valid == 0 || probe.vbmi.secondAt != uint8(probe.secondAt) {
		t.Fatal("non-eight pair did not compile a matching VBMI probe")
	}

	check := func(t *testing.T) {
		const candidates = 512
		for _, want := range []int{0, 63, 64, 255, 400, 511} {
			input := []byte(strings.Repeat("x", candidates+64))
			input[want] = 'S'
			input[want+probe.secondAt] = 'K'
			haystack := string(input)

			if got := asciiPairSkipBytes(haystack, 0, candidates, &probe); got != want {
				t.Fatalf("candidate %d: pair skip = %d, want %d", want, got, want)
			}
		}
	}

	if cpu.X86.HasAVX512VBMI {
		t.Run("VBMI", check)
	}
	t.Run("without compiled VBMI", func(t *testing.T) {
		probe.vbmi.valid = 0
		check(t)
	})
}

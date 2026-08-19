//go:build amd64

package casei

import (
	"crypto/sha256"
	"encoding/binary"
	"math/rand/v2"
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/cpu"
)

const backendKernelBytes = 1 << 20

type backendKernelCase struct {
	name  string
	bytes int64
	want  int
	call  func() int
	input []byte
}

// backendKernelCases keeps every vector body reachable through a direct,
// full-block miss. The public-plan tests remain the semantic authority; these
// cases pin the individual ABI boundary and provide a throughput benchmark for
// the complete backend without choosing a different implementation per kernel.
func backendKernelCases() []backendKernelCase {
	input := make([]byte, backendKernelBytes+256)
	for i := range input {
		input[i] = 'x'
	}
	ptr := unsafe.SliceData(input)
	const target = uint64('z') * byteOnes
	const runTarget = uint64('x') * byteOnes

	probe := asciiProbe{first: 'a', second: 'b', third: 'c', fold: 7, secondAt: 1, thirdAt: 2}
	makeASCIIVBMIProbe(&probe)
	pairProbe := asciiPairProbe{first: 'a', second: 'b', firstFold: 0x20, secondFold: 0x20, secondAt: 8}
	makeASCIIPairVBMIProbe(&pairProbe, "abcdefghijk")

	filter := rootFilter{
		ones:  [16]byte{'a', 'b'},
		pairs: [16]rootPair{{first: 'c', second: 'd', fold: rootPairFoldFirst}, {first: 'e', second: 'f', fold: rootPairFoldSecond}},
		oneN:  2,
		pairN: 2,
	}
	pairSet := rootFilter{pairs: [16]rootPair{{first: 'a', second: 'b'}, {first: 'c', second: 'd'}}, pairN: 2}

	shuftiRoots := rootFilter{pairN: 5}
	for i := range shuftiRoots.pairN {
		shuftiRoots.pairs[i] = rootPair{first: byte('a' + i), second: byte('m' + i), fold: rootPairFoldFirst}
	}
	shufti := makePairShuftiFilter(shuftiRoots)

	shuftiWithOnesRoots := rootFilter{ones: [16]byte{'a', 'b'}, oneN: 2, pairN: 9}
	for i := range shuftiWithOnesRoots.pairN {
		shuftiWithOnesRoots.pairs[i] = rootPair{first: byte('c' + i), second: byte('n' + i)}
	}
	shuftiWithOnes := makePairShuftiFilter(shuftiWithOnesRoots)

	triple := tripleFilter{values: [16]rootTriple{{'a', 'b', 'c', 7}, {'d', 'e', 'f', 7}, {'g', 'h', 'i', 7}, {'j', 'k', 'l', 7}}, n: 4}
	tripleShufti := makeTripleShuftiFilter(triple)
	triplePair := tripleFilter{values: [16]rootTriple{{'a', 'b', 'c', 7}, {'d', 'e', 'f', 7}}, n: 2}
	tripleMixed := tripleFilter{values: [16]rootTriple{{'a', 'b', 'c', 7}, {'d', 'e', 'f', 7}, {0xc3, 0xa9, 'g', 4}}, n: 3}
	tripleASCIIUTF8 := tripleFilter{values: [16]rootTriple{{'a', 'b', 'c', 7}, {0xc3, 0xa9, 0x80, 0}}, n: 2}
	tripleSharedPrefix := tripleFilter{values: [16]rootTriple{{'a', 'b', 'c', 7}, {'a', 'b', 0xc3, 3}}, n: 2}

	anchors := asciiPairAnchors{anchors: [asciiPairAnchorSlots]asciiPairAnchor{{first: 'a', second: 'b'}, {first: 'c', second: 'd'}}, n: 2}
	anchors.filter = makeASCIIPairAnchorFilter(&anchors)

	pairPair := pairPairFilter{
		first0: 'a', second0: 'b', first1: 'c', second1: 'd',
		confirmFirst0: 'e', confirmSecond0: 'f', confirmFirst1: 'g', confirmSecond1: 'h',
		offset: 8, valid: 1,
	}
	makePairPairVBMIFilter(&pairPair)

	const n32 = backendKernelBytes
	const n64 = backendKernelBytes
	const pairN = backendKernelBytes + 1
	const tripleN = backendKernelBytes + 2
	cases := make([]backendKernelCase, 0, 38)
	add := func(name string, bytes int64, want int, call func() int) {
		cases = append(cases, backendKernelCase{name: name, bytes: bytes, want: want, call: call, input: input})
	}

	if cpu.X86.HasAVX2 {
		add("rootSkip32", n32, n32, func() int { return rootSkip32(ptr, n32, target, byteCaseBit) })
		add("literalSkip32", n32, n32, func() int { return literalSkip32(ptr, n32, target, byteCaseBit) })
		add("runSkip32", n32, n32, func() int { return runSkip32(ptr, n32, runTarget, byteCaseBit) })
		add("runMask32", 32, int(^uint32(0)), func() int { return int(runMask32(ptr, runTarget, byteCaseBit)) })
		add("probeSkip32", n32, n32, func() int { return probeSkip32(ptr, n32, &probe) })
		add("pairSetSkip32", pairN, n32, func() int { return pairSetSkip32(ptr, pairN, &pairSet) })
		add("pairSecondSkip32", n32, n32, func() int { return pairSecondSkip32(ptr, n32, &filter) })
		add("pairSkip32", pairN, n32, func() int { return pairSkip32(ptr, pairN, target, byteCaseBit, target, byteCaseBit) })
		add("filterSkip32", pairN, n32, func() int { return filterSkip32(ptr, pairN, &filter) })
		add("tripleSkip32", tripleN, n32, func() int { return tripleSkip32(ptr, tripleN, &triple) })
		add("triplePairSkip32", tripleN, n32, func() int { return triplePairSkip32(ptr, tripleN, &triplePair) })
	}

	if cpu.X86.HasAVX512F && cpu.X86.HasAVX512BW {
		add("rootSkip64", n64, n64, func() int { return rootSkip64(ptr, n64, target, byteCaseBit) })
		add("literalSkip64", n64, n64, func() int { return literalSkip64(ptr, n64, target, byteCaseBit) })
		add("runSkip64", n64, n64, func() int { return runSkip64(ptr, n64, runTarget, byteCaseBit) })
		add("runMask64", 64, -1, func() int { return int(runMask64(ptr, runTarget, byteCaseBit)) })
		add("probeSkip64", n64, n64, func() int { return probeSkip64(ptr, n64, &probe) })
		if cpu.X86.HasAVX512VBMI {
			add("probeVBMISkip64", n64, n64, func() int { return probeVBMISkip64(ptr, n64, &probe.vbmi) })
		}
		add("asciiOnlyProbeSkip64", n64, n64, func() int { return asciiOnlyProbeSkip64(ptr, n64, &probe) })
		add("asciiPairDirectSkip64", n64, n64, func() int { return asciiPairDirectSkip64(ptr, n64, &pairProbe) })
		if cpu.X86.HasAVX512VBMI {
			add("asciiPairDirectVBMISkip64", n64, n64, func() int { return asciiPairDirectVBMISkip64(ptr, n64, &pairProbe.vbmi) })
		}
		add("asciiPairShortSkip64", n64, n64, func() int { return asciiPairShortSkip64(ptr, n64, &pairProbe) })
		add("pairSetSkip64", pairN, n64, func() int { return pairSetSkip64(ptr, pairN, &pairSet) })
		add("pairShuftiSkip64", pairN, n64, func() int { return pairShuftiSkip64(ptr, pairN, &shufti) })
		add("pairShuftiWithOnesSkip64", pairN, n64, func() int { return pairShuftiWithOnesSkip64(ptr, pairN, &shuftiWithOnes) })
		add("pairPairSkip64", n64, n64, func() int { return pairPairSkip64(ptr, n64, &pairPair) })
		if cpu.X86.HasAVX512VBMI {
			add("pairPairVBMISkip64", n64, n64, func() int { return pairPairVBMISkip64(ptr, n64, &pairPair.vbmi) })
		}
		add("pairPairWordSkip64", n64, n64, func() int { return pairPairWordSkip64(ptr, n64, &pairPair) })
		add("pairSecondSkip64", n64, n64, func() int { return pairSecondSkip64(ptr, n64, &filter) })
		add("pairSkip64", pairN, n64, func() int { return pairSkip64(ptr, pairN, target, byteCaseBit, target, byteCaseBit) })
		add("filterSkip64", pairN, n64, func() int { return filterSkip64(ptr, pairN, &filter) })
		add("tripleSkip64", tripleN, n64, func() int { return tripleSkip64(ptr, tripleN, &triple) })
		add("tripleShuftiSkip64", tripleN, n64, func() int { return tripleShuftiSkip64(ptr, tripleN, &tripleShufti) })
		add("asciiPairAnchorSkip64", pairN, n64, func() int { return asciiPairAnchorSkip64(ptr, pairN, &anchors.filter) })
		if cpu.X86.HasAVX512VBMI {
			add("asciiPairAnchorVBMISkip64", pairN, n64, func() int { return asciiPairAnchorVBMISkip64(ptr, pairN, &anchors.filter.vbmi) })
		}
		add("tripleSharedPrefixSkip64", tripleN, n64, func() int { return tripleSharedPrefixSkip64(ptr, tripleN, &tripleSharedPrefix) })
		add("tripleASCIIUTF8Skip64", tripleN, n64, func() int { return tripleASCIIUTF8Skip64(ptr, tripleN, &tripleASCIIUTF8) })
		add("tripleMixedSkip64", tripleN, n64, func() int { return tripleMixedSkip64(ptr, tripleN, &tripleMixed) })
		add("triplePairSkip64", tripleN, n64, func() int { return triplePairSkip64(ptr, tripleN, &triplePair) })
	}
	return cases
}

func TestAMD64BackendKernels(t *testing.T) {
	cases := backendKernelCases()
	if len(cases) == 0 {
		t.Skip("the host has neither AVX2 nor AVX-512 BW")
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.call()
			if got != tc.want {
				t.Fatalf("%s = %d, want %d", tc.name, got, tc.want)
			}
			runtime.KeepAlive(tc.input)
		})
	}
}

// BenchmarkAMD64BackendKernels exposes each vector body independently. These
// are secondary throughput probes; the public IndexFold and Matcher.Find
// benchmarks below are the A/B outcome workloads.
func BenchmarkAMD64BackendKernels(b *testing.B) {
	cases := backendKernelCases()
	if len(cases) == 0 {
		b.Skip("the host has neither AVX2 nor AVX-512 BW")
	}
	for _, tc := range cases {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(tc.bytes)
			result := 0
			for b.Loop() {
				result = tc.call()
			}
			if result != tc.want {
				b.Fatalf("%s = %d, want %d", tc.name, result, tc.want)
			}
			runtime.KeepAlive(tc.input)
		})
	}
}

type backendIndexCase struct {
	name     string
	haystack string
	needle   string
	want     int
}

// backendIndexCases keeps the one-needle public API on the dispatch path under
// test. The latency case is intentionally separate from the streaming case so
// it cannot be hidden by a byte-weighted aggregate.
func backendIndexCases() []backendIndexCase {
	shortPrefix := strings.Repeat("x", 1009)
	return []backendIndexCase{
		{
			name:     "stream_ascii_miss_64kb",
			haystack: strings.Repeat("x", 64<<10),
			needle:   "fatal panic",
			want:     -1,
		},
		{
			name:     "latency_ascii_end_1kb",
			haystack: shortPrefix + "NeEdLe At EnD",
			needle:   "needle at end",
			want:     len(shortPrefix),
		},
		{
			name:     "utf8_miss_64kb",
			haystack: strings.Repeat("привет мир ", 4096),
			needle:   "яростный дракон",
			want:     -1,
		},
	}
}

func checkBackendIndexCase(t testing.TB, tc backendIndexCase) {
	t.Helper()
	if got := IndexFold(tc.haystack, tc.needle); got != tc.want {
		t.Fatalf("%s: IndexFold = %d, want %d", tc.name, got, tc.want)
	}
}

func TestAMD64BackendIndexCases(t *testing.T) {
	for _, tc := range backendIndexCases() {
		t.Run(tc.name, func(t *testing.T) { checkBackendIndexCase(t, tc) })
	}
}

func BenchmarkAMD64BackendIndexFold(b *testing.B) {
	for _, tc := range backendIndexCases() {
		tc := tc
		checkBackendIndexCase(b, tc)
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(tc.haystack)))
			got := 0
			for b.Loop() {
				got = IndexFold(tc.haystack, tc.needle)
			}
			if got != tc.want {
				b.Fatalf("IndexFold = %d, want %d", got, tc.want)
			}
			runtime.KeepAlive(tc)
		})
	}
}

type backendMatcherCase struct {
	name     string
	haystack string
	matcher  *Matcher
	want     Match
	wantOK   bool
}

// backendMatcherCases exercises the multi-needle public API with long-stream,
// short-latency, and width-changing Unicode searches.
func backendMatcherCases() []backendMatcherCase {
	shortPrefix := strings.Repeat("x", 1009)
	unicodePrefix := strings.Repeat("ordinary ASCII prose ", 4096)
	return []backendMatcherCase{
		{
			name:     "stream_ascii_miss_64kb",
			haystack: strings.Repeat("x", 64<<10),
			matcher:  NewMatcher([]string{"fatal panic", "segfault detected", "oom killed", "disk full", "payment declined", "quota exceeded", "handshake failed", "watchdog fired"}),
		},
		{
			name:     "latency_ascii_end_1kb",
			haystack: shortPrefix + "NeEdLe At EnD",
			matcher:  NewMatcher([]string{"needle at end", "fatal panic", "segfault detected", "watchdog fired"}),
			want:     Match{Start: len(shortPrefix)},
			wantOK:   true,
		},
		{
			name:     "unicode_kelvin_match",
			haystack: unicodePrefix + "KELVIN",
			matcher:  NewMatcher([]string{"щупальце", "kelvin", "zygomorphic", "ſecret", "Zq9xW", "grofse", "ΤΈΛΟΣ", "watchdog"}),
			want:     Match{Pattern: 1, Start: len(unicodePrefix)},
			wantOK:   true,
		},
	}
}

func checkBackendMatcherCase(t testing.TB, tc backendMatcherCase) {
	t.Helper()
	got, ok := tc.matcher.Find(tc.haystack)
	if ok != tc.wantOK || ok && got != tc.want {
		t.Fatalf("%s: Find = %+v,%t, want %+v,%t", tc.name, got, ok, tc.want, tc.wantOK)
	}
}

func TestAMD64BackendMatcherCases(t *testing.T) {
	for _, tc := range backendMatcherCases() {
		t.Run(tc.name, func(t *testing.T) { checkBackendMatcherCase(t, tc) })
	}
}

func BenchmarkAMD64BackendMatcherFind(b *testing.B) {
	for _, tc := range backendMatcherCases() {
		tc := tc
		checkBackendMatcherCase(b, tc)
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(tc.haystack)))
			var got Match
			var ok bool
			for b.Loop() {
				got, ok = tc.matcher.Find(tc.haystack)
			}
			if ok != tc.wantOK || ok && got != tc.want {
				b.Fatalf("Find = %+v,%t, want %+v,%t", got, ok, tc.want, tc.wantOK)
			}
			runtime.KeepAlive(tc)
		})
	}
}

// TestAMD64BackendDigest checks reference semantics while emitting a stable
// cross-product digest. The differential command runs this test in the
// assembly and simd builds under each supported feature configuration.
func TestAMD64BackendDigest(t *testing.T) {
	digest := sha256.New()
	writeInt := func(value int) {
		var encoded [8]byte
		binary.LittleEndian.PutUint64(encoded[:], uint64(int64(value)))
		_, _ = digest.Write(encoded[:])
	}
	writeString := func(value string) {
		_, _ = digest.Write([]byte(value))
		_, _ = digest.Write([]byte{0})
	}

	for _, tc := range backendKernelCases() {
		got := tc.call()
		if got != tc.want {
			t.Fatalf("%s = %d, want %d", tc.name, got, tc.want)
		}
		writeString(tc.name)
		writeInt(got)
		runtime.KeepAlive(tc.input)
	}
	for _, tc := range backendIndexCases() {
		checkBackendIndexCase(t, tc)
		writeString(tc.name)
		writeInt(IndexFold(tc.haystack, tc.needle))
	}
	for _, tc := range backendMatcherCases() {
		checkBackendMatcherCase(t, tc)
		match, ok := tc.matcher.Find(tc.haystack)
		writeString(tc.name)
		writeInt(match.Pattern)
		writeInt(match.Start)
		if ok {
			writeInt(1)
		} else {
			writeInt(0)
		}
	}

	rng := rand.New(rand.NewPCG(20260818, 37))
	for i := 0; i < 4096; i++ {
		haystack := randomBytes(rng, 96)
		needle := randomBytes(rng, 16)
		got, want := IndexFold(haystack, needle), reference(haystack, needle)
		if got != want {
			t.Fatalf("IndexFold case %d = %d, want %d", i, got, want)
		}
		writeInt(got)
	}
	for i := 0; i < 2048; i++ {
		haystack := randomBytes(rng, 96)
		patterns := make([]string, 1+rng.IntN(8))
		for j := range patterns {
			patterns[j] = randomBytes(rng, 16)
		}
		got, gotOK := NewMatcher(patterns).Find(haystack)
		want, wantOK := refFind(haystack, patterns)
		if gotOK != wantOK || gotOK && got != want {
			t.Fatalf("Matcher case %d = %+v,%t, want %+v,%t", i, got, gotOK, want, wantOK)
		}
		writeInt(got.Pattern)
		writeInt(got.Start)
		if gotOK {
			writeInt(1)
		} else {
			writeInt(0)
		}
	}
	t.Logf("amd64-backend-digest=%x", digest.Sum(nil))
}

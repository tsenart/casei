package casei

import (
	"strings"
	"testing"

	"golang.org/x/sys/cpu"
)

func TestRootSkipASCII(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		kind   uint8
		needle byte
		want   int
	}{
		{"all rootless", strings.Repeat("x", 257), rootASCIIFold, 'z', 257},
		{"case-fold root", strings.Repeat("x", 130) + "Z" + strings.Repeat("x", 130), rootASCIIFold, 'z', 130},
		{"exact root", strings.Repeat("x", 130) + "!" + strings.Repeat("x", 130), rootExact, '!', 130},
		{"non-ascii stop", strings.Repeat("x", 130) + "K" + strings.Repeat("x", 130), rootASCIIFold, 'z', 130},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rootSkipASCII(tc.input, 0, tc.kind, tc.needle); got != tc.want {
				t.Fatalf("rootSkipASCII(%q) = %d, want %d", tc.name, got, tc.want)
			}
		})
	}
}

func TestLiteralSkipASCII(t *testing.T) {
	input := strings.Repeat("x", 63) + "K" + strings.Repeat("x", 63) + "Z"
	if got, want := literalSkipASCII(input, 0, rootASCIIFold, 'z'), len(input)-1; got != want {
		t.Fatalf("literalSkipASCII = %d, want %d", got, want)
	}
	if got, want := literalSkipASCII("\xff\x80abc", 0, rootASCIIFold, 'z'), 5; got != want {
		t.Fatalf("literalSkipASCII malformed = %d, want %d", got, want)
	}
}

func TestTripleFilterKeepsOtherRoots(t *testing.T) {
	patterns := []string{"~", "aZm]"}
	plan := newSearchPlan(patterns)
	if plan.triples.usable() || !plan.filter.usable() {
		t.Fatalf("partial triples were not retained by the complete filter: triples=%+v filter=%+v", plan.triples, plan.filter)
	}
	if got, ok := plan.find(strings.Repeat("x", 10) + "~" + strings.Repeat("x", 10)); !ok || got != (Match{Pattern: 0, Start: 10}) {
		t.Fatalf("Find = %+v,%v; triples=%+v filter=%+v", got, ok, plan.triples, plan.filter)
	}
}

func TestTripleFilterSkip(t *testing.T) {
	plan := newSearchPlan([]string{"fatal panic", "segfault detected"})
	if !plan.triples.usable() {
		t.Fatalf("no triples: filter=%+v", plan.filter)
	}
	t.Logf("triples=%+v roots=%v fallback=%+v", plan.triples, plan.tripleRoots, plan.filter)
	if got := tripleSkipBytes(strings.Repeat("x", 257), 0, &plan.triples); got != 255 {
		t.Fatalf("triple skip = %d, want 255; triples=%+v", got, plan.triples)
	}
	if got := filterSkipBytes(strings.Repeat("x", 257), 0, &plan.filter); got != 257 {
		t.Fatalf("fallback skip = %d, want 257; filter=%+v", got, plan.filter)
	}
	if got := tripleSkipBytes(strings.Repeat("x", 130)+"FAT", 0, &plan.triples); got != 130 {
		t.Fatalf("triple match skip = %d, want 130; triples=%+v", got, plan.triples)
	}
}

func TestTripleMixedFilter(t *testing.T) {
	patterns := []string{"fatal panic", "segfault detected"}
	plan := newSearchPlan(patterns)
	if plan.triples.n != 3 || plan.triples.values[0].fold != 7 || plan.triples.values[1].fold != 7 ||
		plan.triples.values[2].fold != 4 || plan.triples.values[2].first < 0x80 {
		t.Fatalf("mixed triple plan = %+v", plan.triples)
	}
	prefix := strings.Repeat("x", 130)
	for _, tc := range []struct {
		haystack string
		want     Match
	}{
		{prefix + "FATAL PANIC", Match{Pattern: 0, Start: 130}},
		{prefix + "ſEGFAULT DETECTED", Match{Pattern: 1, Start: 130}},
	} {
		if got, ok := plan.find(tc.haystack); !ok || got != tc.want {
			t.Fatalf("Find(%q) = %+v,%v, want %+v,true", tc.haystack[130:], got, ok, tc.want)
		}
	}
	for _, haystack := range []string{
		prefix + "\xc5\xffegfault detected",
		prefix + "\xbfegfault detected",
	} {
		if got, ok := plan.find(haystack); ok {
			t.Fatalf("Find malformed %q = %+v,%v", haystack[130:], got, ok)
		}
	}
}

func TestOpaqueRootFilter(t *testing.T) {
	plan := newSearchPlan([]string{"\xe2"})
	if !plan.filter.usable() {
		t.Fatalf("opaque root did not get a filter: %+v", plan.filter)
	}
	if got := filterSkipBytes("xxxxxxxxxxx\xe2", 0, &plan.filter); got != 11 {
		t.Fatalf("opaque filter skip = %d, want 11; filter=%+v", got, plan.filter)
	}
	if got := IndexFold("xxxxxxxxxxx\xe2", "\xe2"); got != 11 {
		t.Fatalf("IndexFold opaque root = %d, want 11; filter=%+v", got, plan.filter)
	}
}

func TestFilterSkipBytes(t *testing.T) {
	var filter rootFilter
	if !filter.addOne('z') || !filter.addPair(0xce, 0xa3) ||
		!filter.addFoldPair('f', 'a', rootPairFoldFirst|rootPairFoldSecond) {
		t.Fatal("could not build root filter")
	}
	for _, tc := range []struct {
		input string
		want  int
	}{
		{strings.Repeat("x", 130) + "z" + strings.Repeat("x", 130), 130},
		{strings.Repeat("x", 130) + "Σ" + strings.Repeat("x", 130), 130},
		{strings.Repeat("x", 63) + "Σ" + strings.Repeat("x", 130), 63},
		{strings.Repeat("x", 130) + "FA" + strings.Repeat("x", 130), 130},
		{strings.Repeat("x", 257), 257},
	} {
		if got := filterSkipBytes(tc.input, 0, &filter); got != tc.want {
			t.Fatalf("filterSkipBytes(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestASCIIPairSkip(t *testing.T) {
	plan := newSearchPlan([]string{"fatal panic"})
	probe := &plan.asciiPair
	if !probe.usable() {
		t.Fatal("fixed long ASCII literal did not compile an aligned pair probe")
	}

	for _, offset := range []int{0, 1, 55, 56, 62, 63, 64, 65, 119, 120, 127, 128, 191, 255, 256, 257, 319, 320, 383, 1023, 1024, 1025, 1087} {
		haystack := strings.Repeat("x", offset) + "FaTaL pAnIc" + strings.Repeat("x", 193)
		candidates := len(haystack) - len(plan.asciiNeedle) + 1
		want := 0
		for want < candidates && !asciiPairAt(haystack, want, probe) {
			want++
		}
		if got := asciiPairSkipBytes(haystack, 0, candidates, probe); got != want {
			t.Fatalf("offset %d: pair skip=%d want %d", offset, got, want)
		}
		match, ok := plan.find(haystack)
		if !ok || match.Start != offset || match.Pattern != 0 {
			t.Fatalf("offset %d: find=%+v ok=%t", offset, match, ok)
		}
	}

	// An all-miss stream covers the steady-state four-block loop rather than
	// returning from an early candidate.
	miss := strings.Repeat("x", 2048)
	missCandidates := len(miss) - len(plan.asciiNeedle) + 1
	if got := asciiPairSkipBytes(miss, 0, missCandidates, probe); got != missCandidates {
		t.Fatalf("all-miss pair skip=%d want %d", got, missCandidates)
	}

	// The light pair transition admits this non-match, then must continue to
	// the later full literal rather than changing leftmost semantics.
	haystack := strings.Repeat("x", 63) + "fxxxxxxxn" + strings.Repeat("x", 64) + "fatal panic"
	match, ok := plan.find(haystack)
	want := 63 + len("fxxxxxxxn") + 64
	if !ok || match.Start != want || match.Pattern != 0 {
		t.Fatalf("false pair candidate: find=%+v ok=%t want start %d", match, ok, want)
	}
}

func TestASCIIVBMIProjections(t *testing.T) {
	plan := newSearchPlan([]string{"goto retryLabel"})
	probe := &plan.asciiProbe
	if probe.vbmi.valid == 0 {
		t.Fatalf("letter-only probe did not compile a VBMI projection: %+v", probe)
	}
	values := [3]byte{probe.first, probe.second, probe.third}
	tables := [3]*[64]byte{&probe.vbmi.first, &probe.vbmi.second, &probe.vbmi.third}
	for i, value := range values {
		if tables[i][value&0x3f] == 0 {
			t.Fatalf("probe byte %d (%q) missing from VBMI table", i, value)
		}
		if probe.fold&(1<<i) != 0 && tables[i][(value^0x20)&0x3f] == 0 {
			t.Fatalf("folded probe byte %d (%q) missing from VBMI table", i, value^0x20)
		}
	}

	// The direct compare route has lower survivor latency for non-letter
	// anchors, including this dense-whitespace spelling.
	if blocked := newSearchPlan([]string{"The "}); blocked.asciiProbe.vbmi.valid != 0 {
		t.Fatalf("whitespace probe unexpectedly compiled a VBMI projection: %+v", blocked.asciiProbe.vbmi)
	}

	pairPlan := newSearchPlan([]string{"fatal panic"})
	pair := &pairPlan.asciiPair
	if pair.vbmi.valid == 0 || pair.vbmi.first[pair.first&0x3f] == 0 || pair.vbmi.second[pair.second&0x3f] == 0 {
		t.Fatalf("letter pair did not compile VBMI tables: %+v", pair.vbmi)
	}
	if pair.vbmi.secondAt != uint8(pair.secondAt) {
		t.Fatalf("rare leading pair moved VBMI anchor from %d to %d", pair.secondAt, pair.vbmi.secondAt)
	}

	displaced := newSearchPlan([]string{"goto retryLabel"}).asciiPair
	if displaced.secondAt != 8 || displaced.vbmi.valid == 0 || displaced.vbmi.secondAt != 9 {
		t.Fatalf("VBMI displacement = %+v", displaced)
	}
	if displaced.vbmi.second['y'&0x3f] == 0 || displaced.vbmi.second['Y'&0x3f] == 0 {
		t.Fatalf("displaced VBMI table lost y/Y: %+v", displaced.vbmi.second)
	}
	if punctuated := newSearchPlan([]string{"!abcdefg?"}); punctuated.asciiPair.vbmi.valid != 0 {
		t.Fatalf("punctuated pair unexpectedly compiled a VBMI projection: %+v", punctuated.asciiPair.vbmi)
	}
}

func TestASCIIVBMIProbeBoundaries(t *testing.T) {
	needle := "goto retryLabel"
	plan := newSearchPlan([]string{needle})
	probe := &plan.asciiProbe
	for alignment := 0; alignment < 128; alignment++ {
		haystack := strings.Repeat("x", alignment) + "GoTo ReTrYlAbEl" + strings.Repeat("x", 193)
		candidates := len(haystack) - len(needle) + 1
		got := probeSkipBytes(haystack, 0, candidates, probe)
		if got > alignment {
			t.Fatalf("alignment %d: VBMI probe skip=%d skips match at %d", alignment, got, alignment)
		}
		match, ok := plan.find(haystack)
		if !ok || match != (Match{Pattern: 0, Start: alignment}) {
			t.Fatalf("alignment %d: Find=%+v,%t", alignment, match, ok)
		}
	}

	// Low-six-bit aliases are allowed to reach replay, but they cannot hide the
	// later exact spelling. High-byte aliases exercise the same boundary without
	// admitting an invalid byte as a match.
	alias := []byte(strings.Repeat("x", len(needle)))
	values := [3]byte{probe.first, probe.second, probe.third}
	offsets := [3]int{probe.firstAt, probe.secondAt, probe.thirdAt}
	for i := range values {
		alias[offsets[i]] = values[i] ^ 0x40
	}
	haystack := string(alias) + strings.Repeat("x", 64) + needle
	want := len(alias) + 64
	if cpu.X86.HasAVX512F && cpu.X86.HasAVX512BW && cpu.X86.HasAVX512VBMI {
		if got := probeSkipBytes(haystack, 0, len(haystack)-len(needle)+1, probe); got != 0 {
			t.Fatalf("VBMI alias did not reach replay: skip=%d", got)
		}
	}
	match, ok := plan.find(haystack)
	if !ok || match != (Match{Pattern: 0, Start: want}) {
		t.Fatalf("VBMI alias hid exact match: Find=%+v,%t want start %d", match, ok, want)
	}
}

func TestASCIIPairVBMIDisplacement(t *testing.T) {
	needle := "goto retryLabel"
	plan := newSearchPlan([]string{needle})
	if !plan.asciiPairVBMIDisplaced() {
		t.Fatalf("plan did not compile a displaced VBMI pair: %+v", plan.asciiPair.vbmi)
	}

	for _, offset := range []int{0, 1, 63, 64, 127, 128, 255, 256, 4095} {
		haystack := strings.Repeat("x", offset) + "GoTo ReTrYlAbEl" + strings.Repeat("x", 4608)
		got, ok := plan.find(haystack)
		want, wantOK := refFind(haystack, []string{needle})
		if ok != wantOK || ok && got != want {
			t.Fatalf("offset %d: Find = %+v,%t want %+v,%t", offset, got, ok, want, wantOK)
		}
	}

	// A low-six-bit alias may stop the VBMI filter, but the shared plan must
	// reject it and continue to the later real match.
	alias := []byte(strings.Repeat("x", 64))
	alias[0] = plan.asciiPair.first ^ 0x40
	alias[plan.asciiPair.vbmi.secondAt] = 'y' ^ 0x40
	haystack := string(alias) + strings.Repeat("x", 64) + needle + strings.Repeat("x", 4096)
	want := len(alias) + 64
	if got, ok := plan.find(haystack); !ok || got != (Match{Pattern: 0, Start: want}) {
		t.Fatalf("VBMI alias hid later match: Find = %+v,%t want start %d", got, ok, want)
	}

	if asciiPairVBMIEnabled() {
		hasVBMI := cpu.X86.HasAVX512VBMI
		cpu.X86.HasAVX512VBMI = false
		defer func() { cpu.X86.HasAVX512VBMI = hasVBMI }()
		if got, ok := plan.find(haystack); !ok || got != (Match{Pattern: 0, Start: want}) {
			t.Fatalf("VBMI-disabled fallback: Find = %+v,%t want start %d", got, ok, want)
		}
	}
}

func TestASCIIOnlyStructuredProbe(t *testing.T) {
	needle := "[keys[i%"
	plan := newSearchPlan([]string{needle})
	if !plan.asciiOnly || !plan.asciiOnlyLong ||
		plan.asciiOnlyProbe.secondAt != plan.asciiOnlyProbe.thirdAt {
		t.Fatalf("structured ASCII probe = %+v long=%t", plan.asciiOnlyProbe, plan.asciiOnlyLong)
	}

	for _, offset := range []int{0, 1, 55, 56, 62, 63, 64, 65, 119, 120, 127, 128, 191, 255} {
		haystack := strings.Repeat("x", offset) + "[KeYs[i%" + strings.Repeat("x", 193)
		match, ok := plan.find(haystack)
		if !ok || match != (Match{Pattern: 0, Start: offset}) {
			t.Fatalf("ASCII offset %d: find=%+v ok=%t", offset, match, ok)
		}
	}

	// A light pair survivor must be fully confirmed before it can hide a later
	// match, and width-changing fold forms must take the Unicode fallback.
	falseAt := 63
	haystack := strings.Repeat("x", falseAt) + "[xxxxxx%" + strings.Repeat("x", 64) + needle
	match, ok := plan.find(haystack)
	want := falseAt + len("[xxxxxx%") + 64
	if !ok || match != (Match{Pattern: 0, Start: want}) {
		t.Fatalf("false structured pair: find=%+v ok=%t want=%d", match, ok, want)
	}
	for _, spelling := range []string{"[\u212Aeys[i%", "[key\u017f[i%"} {
		haystack := strings.Repeat("x", 130) + spelling + strings.Repeat("x", 193)
		match, ok := plan.find(haystack)
		if !ok || match != (Match{Pattern: 0, Start: 130}) {
			t.Fatalf("Unicode spelling %q: find=%+v ok=%t", spelling, match, ok)
		}
	}
}

func TestPairShuftiSkip(t *testing.T) {
	plan := newSearchPlan([]string{"Kелвин0", "ſекрет1", "Σигма2", "ςигма3", "щупальце4"})
	filter := &plan.filter
	if !filter.shufti.usable() {
		t.Fatalf("hazard roots did not compile the two-group pair filter: %+v", filter)
	}

	// The table projection is exact for its expanded raw-pair union. Exhaust
	// every two-byte input so case-expanded ASCII roots and UTF-8 prefixes
	// cannot silently lose a lane before the normal plan transition sees it.
	for first := 0; first < 256; first++ {
		for second := 0; second < 256; second++ {
			input := string([]byte{byte(first), byte(second)})
			want := filterSkipScalar(input, 0, filter) == 0
			if got := pairShuftiAt(byte(first), byte(second), &filter.shufti); got != want {
				t.Fatalf("pair %02x %02x: shufti=%t want %t", first, second, got, want)
			}
		}
	}

	// Exercise vector block boundaries and the scalar tail against the original
	// complete filter. The raw spellings cover both ASCII-folded and UTF-8 roots.
	pairs := []string{
		"k\xd0", "K\xd0", "s\xd0", "S\xd0", "\xe2\x84", "\xc5\xbf",
		"\xce\xa3", "\xcf\x82", "\xcf\x83", "\xd1\x89", "\xd0\xa9",
	}
	for _, offset := range []int{0, 1, 62, 63, 64, 65, 126, 127, 128, 191} {
		for _, pair := range pairs {
			haystack := strings.Repeat("x", offset) + pair + strings.Repeat("x", 193)
			got := pairShuftiSkipBytes(haystack, 0, filter)
			want := filterSkipScalar(haystack, 0, filter)
			if got != want {
				t.Fatalf("offset %d pair % x: shufti skip=%d want %d", offset, pair, got, want)
			}
		}
	}

	// A nontrivial byte stream checks that every stop returned by the vector
	// loop agrees with the scalar complete filter, including starts after a
	// failed candidate.
	bytes := make([]byte, 513)
	state := uint32(1)
	for i := range bytes {
		state = state*1664525 + 1013904223
		bytes[i] = byte(state >> 24)
	}
	haystack := string(bytes)
	for at := range haystack {
		got := pairShuftiSkipBytes(haystack, at, filter)
		want := filterSkipScalar(haystack, at, filter)
		if got != want {
			t.Fatalf("random at %d: shufti skip=%d want %d", at, got, want)
		}
	}
}

func TestPairShuftiWithOneRoots(t *testing.T) {
	patterns := []string{"щупальце", "kelvin", "zygomorphic", "ſecret", "Zq9xW", "grofse", "ΤΈΛΟΣ", "watchdog"}
	plan := newSearchPlan(patterns)
	filter := &plan.filter
	if filter.shufti.oneN != 2 || !filter.shufti.usable() {
		t.Fatalf("mixed shufti filter = %+v", filter.shufti)
	}

	// This normalized projection may stop early, but it must never skip any
	// byte where the complete root filter would stop. Exercise the AVX-512 loop
	// at every alignment with both one-byte and pair-root spellings.
	stream := strings.Repeat("x", 63) + "z" + strings.Repeat("x", 63) + "Z" +
		strings.Repeat("x", 63) + "\xd0\xa9" + strings.Repeat("x", 63) + "KE" +
		strings.Repeat("x", 63) + "\xe2\x84" + strings.Repeat("x", 193)
	for at := range stream {
		got := pairShuftiSkipBytes(stream, at, filter)
		want := filterSkipScalar(stream, at, filter)
		if got > want {
			t.Fatalf("at %d: shufti skip=%d skips complete-filter stop %d", at, got, want)
		}
	}

	for first := 0; first < 256; first++ {
		for second := 0; second < 256; second++ {
			input := string([]byte{byte(first), byte(second)})
			if filterSkipScalar(input, 0, filter) == 0 &&
				!pairShuftiAt(byte(first), byte(second), &filter.shufti) {
				t.Fatalf("pair %02x %02x lost from normalized projection", first, second)
			}
		}
	}
}

func TestTripleShuftiSkip(t *testing.T) {
	plan := newSearchPlan([]string{"щупальце", "kelvin", "zygomorphic", "ſecret", "Zq9xW", "grofse", "ΤΈΛΟΣ", "watchdog"})
	filter := &plan.asciiTriples
	if !plan.asciiTriplesComplete || !filter.shufti.usable() {
		t.Fatalf("mixed triple table was not compiled: complete=%t filter=%+v", plan.asciiTriplesComplete, filter)
	}

	// Every literal triple rendering must remain a Shufti survivor. Toggle each
	// folded ASCII byte too, then compare the table scan with the precise
	// transition over randomized and vector-boundary inputs. The table may stop
	// early because bit-five normalization is deliberately conservative; it may
	// never pass a complete triple match.
	for i := range filter.n {
		triple := filter.values[i]
		for form := 0; form < 8; form++ {
			bytes := [3]byte{triple.first, triple.second, triple.third}
			for at := range bytes {
				if triple.fold&(1<<at) != 0 && form&(1<<at) != 0 {
					bytes[at] ^= 0x20
				}
			}
			if !tripleShuftiAt(bytes[0], bytes[1], bytes[2], &filter.shufti) {
				t.Fatalf("triple %d form %x was lost", i, bytes)
			}
		}
	}

	// Place every folded form at every lane alignment. This drives the amd64
	// block transition (rather than just its scalar table helper) across its
	// overlapping byte loads.
	for i := range filter.n {
		triple := filter.values[i]
		for form := 0; form < 8; form++ {
			for alignment := 0; alignment < 64; alignment++ {
				streamBytes := []byte(strings.Repeat("!", 256))
				at := alignment
				streamBytes[at], streamBytes[at+1], streamBytes[at+2] = triple.first, triple.second, triple.third
				for byteAt := 0; byteAt < 3; byteAt++ {
					if triple.fold&(1<<byteAt) != 0 && form&(1<<byteAt) != 0 {
						streamBytes[at+byteAt] ^= 0x20
					}
				}
				stream := string(streamBytes)
				got := tripleShuftiSkipBytes(stream, 0, &filter.shufti)
				want := tripleSkipScalar(stream, 0, filter)
				if got > want {
					t.Fatalf("triple %d form %d alignment %d: shufti skip=%d skips precise triple at %d", i, form, alignment, got, want)
				}
			}
		}
	}

	stream := strings.Repeat("x", 63) + "KeLvIn" + strings.Repeat("x", 63) +
		"SeCrEt" + strings.Repeat("x", 63) + "WATCHDOG" + strings.Repeat("x", 193)
	for at := range stream {
		got := tripleShuftiSkipBytes(stream, at, &filter.shufti)
		want := tripleSkipScalar(stream, at, filter)
		if got > want {
			t.Fatalf("at %d: shufti skip=%d skips precise triple at %d", at, got, want)
		}
	}

	bytes := make([]byte, 513)
	state := uint32(1)
	for i := range bytes {
		state = state*1664525 + 1013904223
		bytes[i] = byte(state >> 24)
	}
	stream = string(bytes)
	for at := range stream {
		got := tripleShuftiSkipBytes(stream, at, &filter.shufti)
		want := tripleSkipScalar(stream, at, filter)
		if got > want {
			t.Fatalf("random at %d: shufti skip=%d skips precise triple at %d", at, got, want)
		}
	}
}

func TestPairShuftiTrailingOne(t *testing.T) {
	plan := newSearchPlan([]string{"Ab", "Cd", "Ef", "Gh", "Ij", "Kl", "Mn", "Op", "Qr", "x"})
	if !plan.filter.shufti.usable() || plan.filter.shufti.oneN == 0 {
		t.Fatalf("mixed filter did not compile Shufti one roots: %+v", plan.filter)
	}
	for _, prefix := range []int{0, 1, 63, 64, 65, 127, 128} {
		stream := strings.Repeat("!", prefix) + "x"
		got := pairShuftiSkipBytes(stream, 0, &plan.filter)
		want := filterSkipScalar(stream, 0, &plan.filter)
		if got != want || got != prefix {
			t.Fatalf("prefix %d: pair Shufti skip=%d want %d", prefix, got, want)
		}
	}
}

func TestASCIIPairAnchorSkip(t *testing.T) {
	patterns := []string{"щупальце", "kelvin", "zygomorphic", "ſecret", "Zq9xW", "grofse", "ΤΈΛΟΣ", "watchdog"}
	plan := newSearchPlan(patterns)
	anchors := &plan.asciiPairAnchors
	if !anchors.usable() {
		t.Fatalf("mixed plan did not compile the all-ASCII pair anchors: %+v", anchors)
	}
	if anchors.filter.vbmi.valid == 0 {
		t.Fatalf("mixed plan did not compile the exact VBMI pair projection: %+v", anchors.filter.vbmi)
	}

	// The VPERMT2B tables split the exact ASCII domain at bit six. Check every
	// possible ASCII source pair against the pre-existing scalar projection.
	vbmiByte := func(lo, hi *[64]byte, value byte) byte {
		if value&0x40 == 0 {
			return lo[value&0x3f]
		}
		return hi[value&0x3f]
	}
	for first := 0; first < 128; first++ {
		for second := 0; second < 128; second++ {
			got := vbmiByte(&anchors.filter.vbmi.firstLo, &anchors.filter.vbmi.firstHi, byte(first)) &
				vbmiByte(&anchors.filter.vbmi.secondLo, &anchors.filter.vbmi.secondHi, byte(second))
			want := asciiPairAnchorFilterAt(byte(first), byte(second), &anchors.filter)
			if (got != 0) != want {
				t.Fatalf("ASCII pair %02x %02x: VBMI=%t want=%t", first, second, got != 0, want)
			}
		}
	}

	// Every selected spelling and case rendering must reach the scalar table at
	// every vector alignment. The amd64 block transition is exact for this
	// normalized pair projection; all actual match confirmation remains in the
	// shared plan test below.
	for i := range anchors.n {
		anchor := anchors.anchors[i]
		for form := 0; form < 4; form++ {
			first, second := anchor.first, anchor.second
			if isASCIILetter(first) && form&1 != 0 {
				first ^= 0x20
			}
			if isASCIILetter(second) && form&2 != 0 {
				second ^= 0x20
			}
			for alignment := 0; alignment < 64; alignment++ {
				stream := strings.Repeat("!", alignment) + string([]byte{first, second}) + strings.Repeat("!", 193)
				got := asciiPairAnchorSkipBytes(stream, 0, &anchors.filter)
				want := asciiPairAnchorSkipScalar(stream, 0, &anchors.filter)
				if got != want {
					t.Fatalf("anchor %d form %d alignment %d: pair skip=%d want %d", i, form, alignment, got, want)
				}
			}
		}
	}

	bytes := make([]byte, 513)
	state := uint32(1)
	for i := range bytes {
		state = state*1664525 + 1013904223
		bytes[i] = byte(state >> 24)
	}
	stream := string(bytes)
	for at := range stream {
		got := asciiPairAnchorSkipBytes(stream, at, &anchors.filter)
		want := asciiPairAnchorSkipScalar(stream, at, &anchors.filter)
		if got != want {
			t.Fatalf("random at %d: pair skip=%d want %d", at, got, want)
		}
	}
}

func TestPairPairWordSkip(t *testing.T) {
	// Exercise the AVX-512 BW/BMI2 fallback even on a VBMI-capable host.
	hasVBMI := cpu.X86.HasAVX512VBMI
	cpu.X86.HasAVX512VBMI = false
	defer func() { cpu.X86.HasAVX512VBMI = hasVBMI }()

	filter := pairPairFilter{
		first0: 0xd1, second0: 0x8f, first1: 0xd0, second1: 0xaf,
		confirmFirst0: 0xd1, confirmSecond0: 0x80, confirmFirst1: 0xd0, confirmSecond1: 0xb0,
		offset: 5, valid: 1,
	}
	skipScalar := func(s string, at int) int {
		candidates := len(s) - at - int(filter.offset) - 1
		if candidates <= 0 {
			return 0
		}
		for skipped := 0; skipped < candidates; skipped++ {
			if pairPairAt(s, at+skipped, &filter) {
				return skipped
			}
		}
		return candidates
	}

	for alignment := 0; alignment < 128; alignment++ {
		for primary := 0; primary < 2; primary++ {
			for confirmation := 0; confirmation < 2; confirmation++ {
				stream := []byte(strings.Repeat("x", 320))
				if primary == 0 {
					stream[alignment], stream[alignment+1] = filter.first0, filter.second0
				} else {
					stream[alignment], stream[alignment+1] = filter.first1, filter.second1
				}
				confirmAt := alignment + int(filter.offset)
				if confirmation == 0 {
					stream[confirmAt], stream[confirmAt+1] = filter.confirmFirst0, filter.confirmSecond0
				} else {
					stream[confirmAt], stream[confirmAt+1] = filter.confirmFirst1, filter.confirmSecond1
				}
				haystack := string(stream)
				if got, want := pairPairSkipBytes(haystack, 0, &filter), skipScalar(haystack, 0); got != want {
					t.Fatalf("alignment %d primary %d confirmation %d: word skip=%d want %d", alignment, primary, confirmation, got, want)
				}
			}
		}
	}

	bytes := make([]byte, 513)
	state := uint32(1)
	for i := range bytes {
		state = state*1664525 + 1013904223
		bytes[i] = byte(state >> 24)
	}
	haystack := string(bytes)
	for at := range haystack {
		if got, want := pairPairSkipBytes(haystack, at, &filter), skipScalar(haystack, at); got != want {
			t.Fatalf("random at %d: word skip=%d want %d", at, got, want)
		}
	}
}

func TestPairPairVBMIProjection(t *testing.T) {
	plan := newSearchPlan([]string{"яр"})
	if plan.unicodePairN < 2 {
		t.Fatalf("no Unicode pair anchors: %+v", plan.unicodePairs)
	}
	filter := &plan.unicodePairs[0].pairPair
	if filter.valid == 0 || filter.vbmi.valid == 0 {
		t.Fatalf("no VBMI pair-pair projection: %+v", filter)
	}
	pairSlot := func(first, second byte, firstTable, secondTable *[64]byte) byte {
		return firstTable[first&0x3f] & secondTable[second&0x3f]
	}
	for _, pair := range [][2]byte{{filter.first0, filter.second0}, {filter.first1, filter.second1}} {
		if pairSlot(pair[0], pair[1], &filter.vbmi.first, &filter.vbmi.second) == 0 {
			t.Fatalf("primary pair % x was lost from VBMI tables", pair)
		}
	}
	for _, pair := range [][2]byte{{filter.confirmFirst0, filter.confirmSecond0}, {filter.confirmFirst1, filter.confirmSecond1}} {
		if pairSlot(pair[0], pair[1], &filter.vbmi.confirmFirst, &filter.vbmi.confirmSecond) == 0 {
			t.Fatalf("confirmation pair % x was lost from VBMI tables", pair)
		}
	}

	// The byte projection may reach an alias, but the ordinary Unicode matcher
	// must reject it and continue through the later exact rendering.
	alias := []byte(strings.Repeat("x", int(filter.offset)+2))
	alias[0], alias[1] = filter.first0^0x40, filter.second0^0x40
	alias[filter.offset], alias[filter.offset+1] = filter.confirmFirst0^0x40, filter.confirmSecond0^0x40
	const gap = 64
	haystack := string(alias) + strings.Repeat("x", gap) + "ЯР"
	want := len(alias) + gap
	if cpu.X86.HasAVX512F && cpu.X86.HasAVX512BW && cpu.X86.HasAVX512VBMI {
		if got := pairPairSkipBytes(haystack, 0, filter); got != 0 {
			t.Fatalf("VBMI pair alias did not reach replay: skip=%d", got)
		}
	}
	match, ok := plan.find(haystack)
	if !ok || match != (Match{Pattern: 0, Start: want}) {
		t.Fatalf("VBMI pair alias hid exact match: Find=%+v,%t want start %d", match, ok, want)
	}
}

func TestStaticASCIIByteAnchor(t *testing.T) {
	needle := strings.Repeat("a", 15) + "b"
	plan := newSearchPlan([]string{needle})
	if !plan.asciiStaticAnchor || plan.asciiStaticAt != len(needle)-1 ||
		plan.asciiStaticKind != rootASCIIFold || plan.asciiStaticByte != 'b' {
		t.Fatalf("static repeated-byte anchor = at %d kind %d byte %q enabled %t", plan.asciiStaticAt, plan.asciiStaticKind, plan.asciiStaticByte, plan.asciiStaticAnchor)
	}

	miss := strings.Repeat("a", 64<<10)
	if got, ok := plan.find(miss); ok || got != (Match{}) {
		t.Fatalf("same-byte miss = %+v,%t", got, ok)
	}
	for _, offset := range []int{0, 1, 63, 64, 127, 128, 4093} {
		haystack := strings.Repeat("a", offset) + "AAAAAAAAAAAAAAAB" + strings.Repeat("a", 193)
		got, gotOK := plan.find(haystack)
		want, wantOK := refFind(haystack, []string{needle})
		if gotOK != wantOK || gotOK && got != want {
			t.Fatalf("offset %d: Find = %+v,%t want %+v,%t", offset, got, gotOK, want, wantOK)
		}
	}

	// A lone anchor-byte false positive must not hide the following literal.
	haystack := strings.Repeat("a", 63) + "xxxxxxxxxxxxxxb" + strings.Repeat("a", 64) + needle
	got, gotOK := plan.find(haystack)
	want, wantOK := refFind(haystack, []string{needle})
	if gotOK != wantOK || gotOK && got != want {
		t.Fatalf("false static anchor: Find = %+v,%t want %+v,%t", got, gotOK, want, wantOK)
	}
}

func TestHitTripleFilter(t *testing.T) {
	plan := newSearchPlan([]string{
		"fatal panic", "segfault detected", "oom killed", "disk full",
		"Payment Declined", "quota exceeded", "handshake failed", "watchdog fired",
	})
	t.Logf("triples=%+v roots=%v fallback=%+v", plan.triples, plan.tripleRoots, plan.filter)
}

// BenchmarkPairShuftiMatcher covers the dense two-group root projection on a
// non-ASCII miss stream. The short case keeps the length guard scalar; longer
// cases enter the AVX-512 transition when it is runtime-enabled.
func BenchmarkPairShuftiMatcher(b *testing.B) {
	matcher := NewMatcher([]string{"Kелвин0", "ſекрет1", "Σигма2", "ςигма3", "щупальце4"})
	if !matcher.plan.filter.shufti.usable() {
		b.Fatal("pair Shufti filter was not compiled")
	}
	for _, tc := range []struct {
		name     string
		haystack string
	}{
		{"64B", strings.Repeat("ж", 32)},
		{"1KiB", strings.Repeat("ж", 512)},
		{"64KiB", strings.Repeat("ж", 32<<10)},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(tc.haystack)))
			if _, ok := matcher.Find(tc.haystack); ok {
				b.Fatal("unexpected match")
			}
			for i := 0; i < b.N; i++ {
				_, _ = matcher.Find(tc.haystack)
			}
		})
	}
}

func BenchmarkTripleSkipBytes(b *testing.B) {
	plan := newSearchPlan([]string{"fatal panic", "segfault detected"})
	haystack := strings.Repeat("x", 1<<20)
	b.SetBytes(int64(len(haystack)))
	for i := 0; i < b.N; i++ {
		_ = tripleSkipBytes(haystack, 0, &plan.triples)
	}
}

func BenchmarkTriplePlan(b *testing.B) {
	plan := newSearchPlan([]string{"fatal panic", "segfault detected"})
	haystack := strings.Repeat("x", 1<<20)
	b.SetBytes(int64(len(haystack)))
	for i := 0; i < b.N; i++ {
		_, _ = plan.find(haystack)
	}
}

func TestPairSkipASCII(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  int
	}{
		{"all pairless", strings.Repeat("x", 257), 256},
		{"case-fold pair", strings.Repeat("x", 130) + "ZQ" + strings.Repeat("x", 130), 130},
		{"root without continuation", strings.Repeat("x", 130) + "Z!" + strings.Repeat("x", 130), 261},
		{"non-ascii stop", strings.Repeat("x", 130) + "K" + strings.Repeat("x", 130), 129},
		{"pair across vector boundary", strings.Repeat("x", 63) + "zq" + strings.Repeat("x", 130), 63},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pairSkipASCII(tc.input, 0, rootASCIIFold, 'z', rootASCIIFold, 'q'); got != tc.want {
				t.Fatalf("pairSkipASCII(%q) = %d, want %d", tc.name, got, tc.want)
			}
		})
	}
}

func TestASCIIShortLiteral(t *testing.T) {
	plan := newSearchPlan([]string{"The "})
	literal := plan.asciiProbe.short
	if literal.valid == 0 {
		t.Fatal("four-byte literal was not compiled")
	}
	for form := 0; form < 8; form++ {
		value := []byte("The ")
		for i := 0; i < 3; i++ {
			if form&(1<<i) != 0 {
				value[i] ^= 0x20
			}
		}
		if !asciiShortLiteralAt(string(value), 0, literal) {
			t.Fatalf("case form %q was rejected", value)
		}
	}
	for _, input := range []string{"The!", "t-e ", "The"} {
		if plan.asciiAnchorMatches(input, 0) {
			t.Fatalf("non-literal %q was accepted", input)
		}
	}

	punctuation := newSearchPlan([]string{"A-1!"})
	if !punctuation.asciiAnchorMatches("a-1!", 0) || punctuation.asciiAnchorMatches("a_1!", 0) {
		t.Fatal("short literal changed punctuation matching")
	}
	for _, haystack := range []string{"xx tHE yy", "xx tHE! yy", "\xfftHE "} {
		got, gotOK := plan.find(haystack)
		want, wantOK := refFind(haystack, []string{"The "})
		if gotOK != wantOK || gotOK && got != want {
			t.Fatalf("Find(%q) = %+v,%t want %+v,%t", haystack, got, gotOK, want, wantOK)
		}
	}
}

func TestPairSkipASCIIDoubleBlockBoundaries(t *testing.T) {
	ref := func(s string) int {
		at := 0
		for at+1 < len(s) {
			left, right := s[at], s[at+1]
			if left >= 0x80 || right >= 0x80 {
				break
			}
			if left|0x20 == 'z' && right|0x20 == 'q' {
				break
			}
			at++
		}
		return at
	}
	for _, offset := range []int{0, 1, 62, 63, 64, 65, 126, 127, 128, 129, 190, 255} {
		for _, suffix := range []string{"ZQ", "Z!", "\xc2\xa0"} {
			haystack := strings.Repeat("x", offset) + suffix + strings.Repeat("x", 193)
			want := ref(haystack)
			if got := pairSkipASCII(haystack, 0, rootASCIIFold, 'z', rootASCIIFold, 'q'); got != want {
				t.Fatalf("offset %d suffix %q: pairSkipASCII = %d, want %d", offset, suffix, got, want)
			}
		}
	}
	multiple := []byte(strings.Repeat("x", 320))
	multiple[65], multiple[66] = 'Z', 'Q'
	multiple[128], multiple[129] = 'z', 'q'
	if got := pairSkipASCII(string(multiple), 0, rootASCIIFold, 'z', rootASCIIFold, 'q'); got != 65 {
		t.Fatalf("two-block earliest pair = %d, want 65", got)
	}
}

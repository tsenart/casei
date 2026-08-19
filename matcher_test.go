package casei

import (
	"math/rand/v2"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// refFind is the independent multi-needle reference: per-pattern canonical
// fold reference, leftmost start, ties to the lowest pattern index.
func refFind(haystack string, patterns []string) (Match, bool) {
	best := Match{Pattern: -1, Start: -1}
	for i, p := range patterns {
		pos := reference(haystack, p)
		if pos < 0 {
			continue
		}
		if best.Pattern < 0 || pos < best.Start {
			best = Match{Pattern: i, Start: pos}
		}
	}
	if best.Pattern < 0 {
		return Match{}, false
	}
	return best, true
}

var matcherTraps = []struct {
	name     string
	haystack string
	patterns []string
	want     Match
	ok       bool
}{
	{"single hit", "Hello World", []string{"world"}, Match{0, 6}, true},
	{"no hit", "Hello World", []string{"mars", "venus"}, Match{}, false},
	{"leftmost wins", "aa bravo alpha", []string{"ALPHA", "BRAVO"}, Match{1, 3}, true},
	{"tie goes to lower index", "xxABCxx", []string{"abc", "ABC"}, Match{0, 2}, true},
	{"empty pattern matches at zero", "abc", []string{"zzz", ""}, Match{1, 0}, true},
	{"empty pattern matches empty haystack", "", []string{""}, Match{0, 0}, true},
	{"non-empty lower ID beats empty tie", "ABC", []string{"abc", ""}, Match{0, 0}, true},
	{"longer terminal beats suffix terminal", "abc", []string{"BC", "ABC"}, Match{1, 0}, true},
	{"empty set", "abc", []string{}, Match{}, false},
	{"fold pair across patterns", "the Kelvin scale", []string{"KELVIN", "scale"}, Match{0, 4}, true},
	{"kelvin sign haystack multi", "xxKelvin scale", []string{"scale", "kelvin"}, Match{1, 2}, true},
	{"cyrillic multi", "доктор Ватсон", []string{"ШЕРЛОК", "ватсон"}, Match{1, 13}, true},
	{"opaque byte cannot match continuation", "K", []string{"\x84"}, Match{}, false},
	{"opaque byte matches opaque byte", "x\x84Y", []string{"\x84y"}, Match{0, 1}, true},
	{"bracket not brace multi", "fn{T}(x)", []string{"fn[T]", "(X)"}, Match{1, 5}, true},
	{"overlapping patterns leftmost", "abcd", []string{"BCD", "ABC"}, Match{1, 0}, true},
}

func TestMatcherTraps(t *testing.T) {
	for _, tc := range matcherTraps {
		m := NewMatcher(tc.patterns)
		got, ok := m.Find(tc.haystack)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("%s: Find = %+v,%v want %+v,%v", tc.name, got, ok, tc.want, tc.ok)
		}
		ref, refOK := refFind(tc.haystack, tc.patterns)
		if refOK != tc.ok || (refOK && ref != tc.want) {
			t.Fatalf("%s: trap table wrong: reference = %+v,%v", tc.name, ref, refOK)
		}
	}
}

func TestMatcherCompilesSharedPlan(t *testing.T) {
	patterns := []string{"first needle", "second needle"}
	haystack := strings.Repeat("x", 80)
	m := NewMatcher(patterns)
	if m.plan == nil {
		t.Fatal("NewMatcher did not compile a shared plan")
	}
	plan := m.plan
	want, wantOK := refFind(haystack, patterns)

	for call := 0; call < 2; call++ {
		got, gotOK := m.Find(haystack)
		if gotOK != wantOK || gotOK && got != want {
			t.Fatalf("Find call %d = %+v,%v want %+v,%v", call, got, gotOK, want, wantOK)
		}
		if m.plan != plan {
			t.Fatalf("Find call %d replaced the compiled plan", call)
		}
	}
}

func TestMatcherPatternsAreIsolated(t *testing.T) {
	patterns := []string{"needle", "other"}
	m := NewMatcher(patterns)

	patterns[0] = "constructor input changed"
	if got := m.Patterns()[0]; got != "needle" {
		t.Fatalf("Patterns()[0] after constructor input mutation = %q, want %q", got, "needle")
	}

	exposed := m.Patterns()
	exposed[0] = "returned slice changed"
	if got := m.Patterns()[0]; got != "needle" {
		t.Fatalf("Patterns()[0] after returned slice mutation = %q, want %q", got, "needle")
	}
	if got, ok := m.Find("needle"); !ok || got != (Match{Pattern: 0, Start: 0}) {
		t.Fatalf("Find after returned slice mutation = %+v,%v, want {Pattern:0 Start:0},true", got, ok)
	}
}

func TestMatcherConcurrentFirstFind(t *testing.T) {
	patterns := []string{"first needle", "second needle"}
	haystack := strings.Repeat("x", 80)
	want, wantOK := refFind(haystack, patterns)
	m := NewMatcher(patterns)

	const callers = 16
	start := make(chan struct{})
	results := make(chan struct {
		match Match
		ok    bool
	}, callers)
	var group sync.WaitGroup
	for range callers {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			match, ok := m.Find(haystack)
			results <- struct {
				match Match
				ok    bool
			}{match, ok}
		}()
	}
	close(start)
	group.Wait()
	close(results)
	for got := range results {
		if got.ok != wantOK || got.ok && got.match != want {
			t.Fatalf("concurrent Find = %+v,%v want %+v,%v", got.match, got.ok, want, wantOK)
		}
	}
	if m.plan == nil {
		t.Fatal("concurrent Find lost the compiled plan")
	}
}

func TestMatcherPairPrefixBoundaries(t *testing.T) {
	patterns := []string{"ZqA", "zQa", "ZqB"}
	for offset := 0; offset < 192; offset++ {
		haystack := strings.Repeat("x", offset) + "zQa" + strings.Repeat("x", 192-offset)
		got, gotOK := NewMatcher(patterns).Find(haystack)
		want, wantOK := refFind(haystack, patterns)
		if gotOK != wantOK || (gotOK && got != want) {
			t.Fatalf("offset %d: Find = %+v,%v want %+v,%v", offset, got, gotOK, want, wantOK)
		}
	}

	// The pair probe may skip an ASCII root byte only when its required next
	// token is absent. High bytes remain scalar so width-changing folds and
	// opaque bytes preserve the regular transition semantics.
	for _, haystack := range []string{
		strings.Repeat("Z!", 128) + "zQa",
		strings.Repeat("x", 63) + "Kqa" + strings.Repeat("x", 128),
		strings.Repeat("x", 63) + "z\x80a" + strings.Repeat("x", 128),
	} {
		patterns := []string{"ZqA", "kqa", "z\x80a"}
		got, gotOK := NewMatcher(patterns).Find(haystack)
		want, wantOK := refFind(haystack, patterns)
		if gotOK != wantOK || (gotOK && got != want) {
			t.Fatalf("Find(%q) = %+v,%v want %+v,%v", haystack, got, gotOK, want, wantOK)
		}
	}
}

func TestMatcherUnicodePairPairBoundaries(t *testing.T) {
	patterns := []string{"яр"}
	plan := newSearchPlan(patterns)
	if plan.unicodePairN < 2 || plan.unicodePairs[0].pairPair.valid == 0 {
		t.Fatalf("no dispersed pair transition: %+v", plan.unicodePairs[0])
	}
	for _, offset := range []int{0, 63, 64, 127, 128, 4093} {
		for _, rendering := range []string{"яр", "ЯР", "яР", "Яр"} {
			haystack := strings.Repeat("x", offset) + rendering + strings.Repeat("x", 4300-offset)
			got, gotOK := plan.find(haystack)
			want, wantOK := refFind(haystack, patterns)
			if gotOK != wantOK || (gotOK && got != want) {
				t.Fatalf("offset %d, %q: Find = %+v,%v want %+v,%v", offset, rendering, got, gotOK, want, wantOK)
			}
		}
	}

	// A primary pair without its dispersed partner must not enter the token
	// machine, including when an invalid byte replaces the partner.
	for _, haystack := range []string{
		strings.Repeat("яx", 2048),
		strings.Repeat("x", 63) + "я\xd1\xff" + strings.Repeat("x", 4096),
	} {
		got, gotOK := plan.find(haystack)
		want, wantOK := refFind(haystack, patterns)
		if gotOK != wantOK || (gotOK && got != want) {
			t.Fatalf("false-primary Find = %+v,%v want %+v,%v", got, gotOK, want, wantOK)
		}
	}
}

func TestMatcherMatchesReferenceRandom(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260804, 99))
	for i := 0; i < 20000; i++ {
		h := randomBytes(rng, 64)
		n := 1 + rng.IntN(5)
		pats := make([]string, n)
		for j := range pats {
			pats[j] = randomBytes(rng, 6)
		}
		got, gotOK := NewMatcher(pats).Find(h)
		want, wantOK := refFind(h, pats)
		if gotOK != wantOK || (gotOK && got != want) {
			t.Fatalf("iter %d: Find(%q, %q) = %+v,%v want %+v,%v", i, h, pats, got, gotOK, want, wantOK)
		}
	}
	// Planted pass over rune strings so hits are common.
	for i := 0; i < 10000; i++ {
		n := 1 + rng.IntN(4)
		pats := make([]string, n)
		for j := range pats {
			pats[j] = randomRuneString(rng, 5)
		}
		pick := pats[rng.IntN(n)]
		h := randomRuneString(rng, 20) + flipCases(rng, pick) + randomRuneString(rng, 20)
		got, gotOK := NewMatcher(pats).Find(h)
		want, wantOK := refFind(h, pats)
		if gotOK != wantOK || (gotOK && got != want) {
			t.Fatalf("planted iter %d: Find(%q, %q) = %+v,%v want %+v,%v", i, h, pats, got, gotOK, want, wantOK)
		}
	}
}

// TestMatcherMatchesRegexpAlternation pins the leftmost-start contract to
// the stdlib: (?i)(?:p0|p1|...) must agree on the match START (regexp
// cannot report which alternative won, so pattern identity is pinned by
// refFind above instead).
func TestMatcherMatchesRegexpAlternation(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260804, 123))
	for i := 0; i < 4000; i++ {
		n := 1 + rng.IntN(4)
		pats := make([]string, n)
		quoted := make([]string, n)
		nonEmpty := false
		for j := range pats {
			pats[j] = randomRuneString(rng, 5)
			quoted[j] = regexp.QuoteMeta(pats[j])
			if pats[j] != "" {
				nonEmpty = true
			}
		}
		if !nonEmpty {
			continue // regexp empty alternation semantics diverge trivially
		}
		h := randomRuneString(rng, 30)
		re := regexp.MustCompile(`(?i)(?:` + strings.Join(quoted, "|") + `)`)
		wantStart := -1
		if loc := re.FindStringIndex(h); loc != nil {
			wantStart = loc[0]
		}
		got, ok := NewMatcher(pats).Find(h)
		gotStart := -1
		if ok {
			gotStart = got.Start
		}
		if gotStart != wantStart {
			t.Fatalf("iter %d: Find(%q, %q) start = %d, regexp says %d", i, h, pats, gotStart, wantStart)
		}
	}
}

func TestMatcherASCIIAnchored(t *testing.T) {
	patterns := []string{"щупальце", "kelvin", "zygomorphic", "ſecret", "Zq9xW", "grofse", "ΤΈΛΟΣ", "watchdog"}
	plan := newSearchPlan(patterns)
	if !plan.asciiPairAnchors.usable() {
		t.Fatalf("mixed plan did not compile the all-ASCII pair transition: %+v", plan.asciiPairAnchors)
	}
	m := NewMatcher(patterns)

	prefix := strings.Repeat("the ordinary prose ", 512)
	for _, tc := range []struct {
		name     string
		haystack string
	}{
		{"kelvin", prefix + "KeLvIn" + strings.Repeat(" x", 64)},
		{"long s", prefix + "SeCrEt" + strings.Repeat(" x", 64)},
		{"watchdog", prefix + "WATCHDOG" + strings.Repeat(" x", 64)},
		// NUL, an invalid byte, and a width-changing fold leave the all-ASCII
		// route. All must still enter the same shared plan and preserve its
		// selected result.
		{"nul fallback", prefix + "\x00kElViN" + strings.Repeat(" x", 64)},
		{"invalid-byte fallback", prefix + "\xffkElViN" + strings.Repeat(" x", 64)},
		{"kelvin-sign fallback", prefix + "Kelvin" + strings.Repeat(" x", 64)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, gotOK := m.Find(tc.haystack)
			want, wantOK := refFind(tc.haystack, patterns)
			if gotOK != wantOK || gotOK && got != want {
				t.Fatalf("Find = %+v,%v want %+v,%v", got, gotOK, want, wantOK)
			}
		})
	}

	// The output propagated through the shared plan remains responsible for a
	// same-start tie, even when a triple survivor enters through either branch.
	tied := append(append([]string(nil), patterns...), "KELVIN")
	got, gotOK := NewMatcher(tied).Find(prefix + "Kelvin")
	want, wantOK := refFind(prefix+"Kelvin", tied)
	if gotOK != wantOK || gotOK && got != want {
		t.Fatalf("tied Find = %+v,%v want %+v,%v", got, gotOK, want, wantOK)
	}

	// An early all-ASCII hit still preserves the shared plan's selected result.
	early := "KeLvIn" + strings.Repeat(" ordinary prose", 1024)
	got, gotOK = m.Find(early)
	want, wantOK = refFind(early, patterns)
	if gotOK != wantOK || gotOK && got != want {
		t.Fatalf("early Find = %+v,%v want %+v,%v", got, gotOK, want, wantOK)
	}

	// Do not settle a short match near the probe boundary while a lower-index,
	// longer pattern with the same start can finish in the following block.
	boundaryPatterns := []string{"abcdef", "abc", "zebra"}
	boundary := strings.Repeat("x", 4093) + "aBcDeF" + strings.Repeat("x", 64)
	got, gotOK = NewMatcher(boundaryPatterns).Find(boundary)
	want, wantOK = refFind(boundary, boundaryPatterns)
	if gotOK != wantOK || gotOK && got != want {
		t.Fatalf("boundary Find = %+v,%v want %+v,%v", got, gotOK, want, wantOK)
	}

	// A non-ASCII-only root beyond the prefix must leave the all-ASCII
	// specialization before a later ASCII candidate is considered.
	mixedRoots := []string{"Ωmega", "kelvin", "zebra"}
	mixedHaystack := strings.Repeat("x", 4096) + "ΩMEGA" + strings.Repeat("x", 64) + "KeLvIn"
	got, gotOK = NewMatcher(mixedRoots).Find(mixedHaystack)
	want, wantOK = refFind(mixedHaystack, mixedRoots)
	if gotOK != wantOK || gotOK && got != want {
		t.Fatalf("mixed-root Find = %+v,%v want %+v,%v", got, gotOK, want, wantOK)
	}

	// Exercise the vector-sized route against the independent per-pattern
	// reference across all-ASCII misses and planted fold renderings.
	rng := rand.New(rand.NewPCG(20260810, 7))
	asciiPatterns := []string{"KeLvIn", "zYgOmOrPhIc", "SeCrEt", "zQ9Xw", "GrOfSe", "wAtChDoG"}
	const alphabet = "abcdefghijklmnopqrstuvwxyz ABCDEFGHIJKLMNOPQRSTUVWXYZ-_."
	for i := 0; i < 128; i++ {
		bytes := make([]byte, 4096+rng.IntN(1024))
		for j := range bytes {
			bytes[j] = alphabet[rng.IntN(len(alphabet))]
		}
		if i%2 == 0 {
			needle := asciiPatterns[rng.IntN(len(asciiPatterns))]
			at := rng.IntN(len(bytes) - len(needle) + 1)
			copy(bytes[at:], needle)
		}
		haystack := string(bytes)
		got, gotOK := m.Find(haystack)
		want, wantOK := refFind(haystack, patterns)
		if gotOK != wantOK || gotOK && got != want {
			t.Fatalf("random %d: Find = %+v,%v want %+v,%v", i, got, gotOK, want, wantOK)
		}
	}
}

func TestMatcherASCIIPairAnchorOrderAndTies(t *testing.T) {
	// The shorter zqX occurrence starts one byte after the longer first
	// pattern, but its selected pair is at offset zero while the longer
	// pattern's pair is later. This makes pair encounter order disagree with
	// source-start order and pins the max-offset delay plus plan replay.
	patterns := []string{"Ωmega", "azqXzqYab", "zqX", "kelvin", "watchdog", "grofse"}
	plan := newSearchPlan(patterns)
	if !plan.asciiPairAnchors.usable() {
		t.Fatalf("order plan did not compile pair anchors: %+v", plan.asciiPairAnchors)
	}
	var long, short asciiPairAnchor
	for i := range plan.asciiPairAnchors.n {
		anchor := plan.asciiPairAnchors.anchors[i]
		switch anchor.pattern {
		case 1:
			long = anchor
		case 2:
			short = anchor
		}
	}
	if long.at <= short.at {
		t.Fatalf("anchor offsets do not invert encounter order: long=%+v short=%+v", long, short)
	}

	start := 127
	haystack := strings.Repeat("!", start) + "azqXzqYab" + strings.Repeat("!", 256)
	got, gotOK := plan.find(haystack)
	want, wantOK := refFind(haystack, patterns)
	if gotOK != wantOK || gotOK && got != want || !gotOK || got != (Match{Pattern: 1, Start: start}) {
		t.Fatalf("ordered pair anchors: Find = %+v,%t want %+v,%t", got, gotOK, want, wantOK)
	}

	// A same-start duplicate must still resolve through the propagated plan
	// output, not whichever pair record reached the replay first.
	tied := append(append([]string(nil), patterns...), "AZQXZQYAB")
	got, gotOK = NewMatcher(tied).Find(haystack)
	want, wantOK = refFind(haystack, tied)
	if gotOK != wantOK || gotOK && got != want {
		t.Fatalf("duplicate tie: Find = %+v,%t want %+v,%t", got, gotOK, want, wantOK)
	}

	// An empty pattern remains a start-zero candidate until the bounded pair
	// lookahead has ruled out a lower-ID non-empty pattern at the same start.
	emptyFirst := append([]string{""}, patterns...)
	got, gotOK = NewMatcher(emptyFirst).Find(haystack)
	want, wantOK = refFind(haystack, emptyFirst)
	if gotOK != wantOK || gotOK && got != want {
		t.Fatalf("empty early: Find = %+v,%t want %+v,%t", got, gotOK, want, wantOK)
	}
	emptyLast := append(append([]string(nil), patterns...), "")
	got, gotOK = NewMatcher(emptyLast).Find("azqXzqYab" + strings.Repeat("!", 256))
	want, wantOK = refFind("azqXzqYab"+strings.Repeat("!", 256), emptyLast)
	if gotOK != wantOK || gotOK && got != want {
		t.Fatalf("empty tie: Find = %+v,%t want %+v,%t", got, gotOK, want, wantOK)
	}

	// The Shufti pair filter normalizes bit five for every byte. A bracket is
	// therefore represented as '{' in the table, which can admit a brace but
	// must still retain the actual bracket occurrence for plan confirmation.
	punctuated := append(append([]string(nil), patterns...), "ab[cd")
	punctuatedPlan := newSearchPlan(punctuated)
	var punct asciiPairAnchor
	for i := range punctuatedPlan.asciiPairAnchors.n {
		anchor := punctuatedPlan.asciiPairAnchors.anchors[i]
		if anchor.pattern == len(punctuated)-1 {
			punct = anchor
		}
	}
	if punct != (asciiPairAnchor{first: 'b', second: '{', at: 1, pattern: len(punctuated) - 1}) {
		t.Fatalf("punctuation anchor = %+v", punct)
	}
	punctuationStart := 96 + len("AB{CD") + 47
	punctuationHaystack := strings.Repeat("!", 96) + "AB{CD" + strings.Repeat("!", 47) + "AB[CD" + strings.Repeat("!", 256)
	got, gotOK = punctuatedPlan.find(punctuationHaystack)
	want, wantOK = refFind(punctuationHaystack, punctuated)
	if gotOK != wantOK || gotOK && got != want || !gotOK || got != (Match{Pattern: len(punctuated) - 1, Start: punctuationStart}) {
		t.Fatalf("punctuation pair anchor: Find = %+v,%t want %+v,%t", got, gotOK, want, wantOK)
	}
}

func TestMatcherFilteredUTF8Boundaries(t *testing.T) {
	for _, tc := range []struct {
		name     string
		haystack string
		patterns []string
	}{
		{
			name:     "opaque continuation cannot start inside rune",
			haystack: "\xe2\x80\x80\xcf\xc2",
			patterns: []string{"\x80\x80\xcf"},
		},
		{
			name:     "opaque bytes still match opaque input",
			haystack: "\x80\x80\xcf",
			patterns: []string{"\x80\x80\xcf"},
		},
		{
			name:     "one-byte root at ascii tail",
			haystack: "щx",
			patterns: []string{"Ab", "Cd", "Ef", "Gh", "Ij", "Kl", "Mn", "Op", "Qr", "x"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := newSearchPlan(tc.patterns)
			got, gotOK := plan.find(tc.haystack)
			want, wantOK := refFind(tc.haystack, tc.patterns)
			if gotOK != wantOK || gotOK && got != want {
				t.Fatalf("Find = %+v,%t want %+v,%t", got, gotOK, want, wantOK)
			}
		})
	}

	// Every suffix beginning at a continuation byte is opaque pattern data. It
	// must not acquire a false source boundary inside one of these valid runes.
	for _, haystack := range []string{"¢", "щ", "€", "𐐀"} {
		for at := 1; at < len(haystack); at++ {
			if haystack[at]&0xc0 != 0x80 {
				continue
			}
			patterns := []string{haystack[at:]}
			plan := newSearchPlan(patterns)
			if !plan.opaqueContinuation {
				t.Fatalf("continuation suffix %x did not retain its boundary guard", patterns[0])
			}
			got, gotOK := plan.find(haystack)
			want, wantOK := refFind(haystack, patterns)
			if gotOK != wantOK || gotOK && got != want {
				t.Fatalf("continuation suffix %x in %x: Find = %+v,%t want %+v,%t", patterns[0], haystack, got, gotOK, want, wantOK)
			}
		}
	}
}

func TestMatcherOpaqueContinuationDifferential(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260811, 31))
	for i := 0; i < 4000; i++ {
		raw := []byte{byte(0x80 + rng.IntN(0x40))}
		for j := 0; j < 1+rng.IntN(4); j++ {
			raw = append(raw, byteAlphabet[rng.IntN(len(byteAlphabet))])
		}
		patterns := []string{randomRuneString(rng, 4), string(raw), randomRuneString(rng, 4)}
		haystack := randomRuneString(rng, 12)
		if i&1 == 0 {
			// A planted opaque sequence is a real byte-boundary match, unlike a
			// continuation suffix of the valid rune prefix.
			haystack += string(raw)
		}
		haystack += randomRuneString(rng, 12)
		got, gotOK := NewMatcher(patterns).Find(haystack)
		want, wantOK := refFind(haystack, patterns)
		if gotOK != wantOK || gotOK && got != want {
			t.Fatalf("iter %d: Find(%x, %x) = %+v,%t want %+v,%t", i, haystack, patterns, got, gotOK, want, wantOK)
		}
	}
}

func FuzzMatcher(f *testing.F) {
	f.Add("Hello World", "world", "mars", "")
	f.Add("xxKelvin scale", "scale", "kelvin", "ſecret")
	f.Add("доктор Ватсон", "ватсон", "ШЕРЛОК", "z")
	f.Fuzz(func(t *testing.T, haystack, p0, p1, p2 string) {
		pats := []string{p0, p1, p2}
		got, gotOK := NewMatcher(pats).Find(haystack)
		want, wantOK := refFind(haystack, pats)
		if gotOK != wantOK || (gotOK && got != want) {
			t.Fatalf("Find(%q, %q) = %+v,%v want %+v,%v", haystack, pats, got, gotOK, want, wantOK)
		}
	})
}

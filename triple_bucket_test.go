//go:build amd64

package casei

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
	"unsafe"

	"golang.org/x/sys/cpu"
)

var tripleBucketTestPatterns = []string{
	"Sherlock Holmes",
	"Sherlock Holmes",
	"Sherlock",
	"John Watson",
	"Irene Adler",
	"Inspector Lestrade",
	"Professor Moriarty",
}

var tripleBucketSevenNamePatterns = []string{
	"Sherlock", "Holmes", "Watson", "Irene", "Adler", "John", "Baker",
}

var tripleBucketSixNamePatterns = []string{
	"Sherlock", "Sailor", "Kelvin", "Kettle", "Irene", "India",
}

var tripleBucketEightHitPatterns = []string{
	"fatal panic", "segfault detected", "oom killed", "disk full",
	"Payment Declined", "quota exceeded", "handshake failed", "watchdog fired",
}

var tripleBucketEightLogPatterns = []string{
	"Zq000xW vK", "Zq001xW vK", "Zq002xW vK", "Zq003xW vK",
	"Zq004xW vK", "Zq005xW vK", "Zq006xW vK", "Zq007xW vK",
}

var tripleBucketEightRussianPatterns = []string{
	"щупальце0", "щупальце1", "щупальце2", "щупальце3",
	"щупальце4", "щупальце5", "щупальце6", "щупальце7",
}

var tripleBucketEightHazardPatterns = []string{
	"щупальце", "kelvin", "zygomorphic", "ſecret",
	"Zq9xW", "grofse", "ΤΈΛΟΣ", "watchdog",
}

var tripleBucketSixCompletePatterns = []string{
	"kelvin", "zygomorphic", "ſecret", "Zq9xW", "grofse", "watchdog",
}

var tripleBucketTiePatterns = []string{
	"Sherlock Holmes", "Sherlock", "Holmes", "Watson", "Irene", "Adler", "John", "Baker",
}

func tripleBucketTestMatcher(patterns []string, enabled bool) *Matcher {
	base := NewMatcher(patterns)
	if base.plan.rawByteMulti.usable() {
		panic("bucket test pattern set selected the independent tagged-anchor route")
	}
	plan := *base.plan
	if !enabled {
		plan.tripleRoots = nil
		plan.triples.shufti = tripleShuftiFilter{}
	}
	return &Matcher{patterns: base.patterns, plan: &plan}
}

func tripleBucketAt(s []byte, at int, filter tripleBucketFilter) bool {
	if at+3 >= len(s) || s[at] >= 128 || s[at+1] >= 128 || s[at+2] >= 128 || s[at+3] >= 128 {
		return false
	}
	mask := filter[int(s[at])] & filter[128+int(s[at+1])] &
		filter[256+int(s[at+2])] & filter[384+int(s[at+3])]
	return mask != 0
}

func tripleBucketPrefixModel(p *searchPlan, s []byte, at int) bool {
	state := 0
	for depth := 1; depth <= tripleBucketPrefixBytes; depth++ {
		token := p.ascii[s[at+depth-1]]
		child, ok := p.nodes[state].edges[token]
		if token == 0 || !ok {
			return false
		}
		state = child
		node := &p.nodes[state]
		if depth == 3 && node.output.pattern >= 0 && node.output.units == 3 {
			return true
		}
		if depth == tripleBucketPrefixBytes {
			return (node.output.pattern >= 0 && node.output.units == depth) || len(node.edges) != 0
		}
	}
	return false
}

func TestTripleBucketCompilerMatchesTrieModel(t *testing.T) {
	plans := []*searchPlan{
		NewMatcher([]string{"Sherlock Holmes", "John Watson", "Irene Adler", "Inspector Lestrade", "Professor Moriarty"}).plan,
		NewMatcher([]string{"Tom", "Sawyer", "Huckleberry", "Finn"}).plan,
		NewMatcher(tripleBucketSevenNamePatterns).plan,
		NewMatcher(tripleBucketSixNamePatterns).plan,
		NewMatcher(tripleBucketEightHitPatterns).plan,
	}
	if !asciiPairVBMIEnabled() {
		t.Skip("AVX-512 VBMI bucket path is disabled")
	}
	rng := rand.New(rand.NewSource(0x4b17))
	for planIndex, p := range plans {
		filter := p.tripleBucketFilter()
		if !filter.usable() {
			t.Fatalf("plan %d has no bucket", planIndex)
		}
		for sample := 0; sample < 250_000; sample++ {
			var bytes [tripleBucketPrefixBytes]byte
			for i := range bytes {
				bytes[i] = byte(rng.Intn(utf8.RuneSelf))
			}
			want := tripleBucketPrefixModel(p, bytes[:], 0)
			if got := tripleBucketAt(bytes[:], 0, filter); got != want {
				t.Fatalf("plan %d sample %d bytes=%q: bucket=%v trie=%v", planIndex, sample, bytes, got, want)
			}
		}
	}
}

// tripleBucketSkip64Model is the lane-for-lane specification of the assembly
// block scan, including its Shufti handoff for blocks containing high bytes.
func tripleBucketSkip64Model(s []byte, filter tripleBucketFilter, shufti *tripleShuftiFilter) int {
	offset := 0
	for len(s)-offset >= 67 {
		ascii := true
		for i := offset; i < offset+67; i++ {
			if s[i] >= 128 {
				ascii = false
				break
			}
		}
		for lane := 0; lane < 64; lane++ {
			at := offset + lane
			matched := tripleBucketAt(s, at, filter)
			if !ascii {
				matched = tripleShuftiAt(s[at], s[at+1], s[at+2], shufti)
			}
			if matched {
				return at
			}
		}
		offset += 64
	}
	return offset
}

func TestTripleBucketPlanEligibility(t *testing.T) {
	english := NewMatcher([]string{
		"Sherlock Holmes", "John Watson", "Irene Adler", "Inspector Lestrade", "Professor Moriarty",
	})
	leipzig := NewMatcher([]string{"Tom", "Sawyer", "Huckleberry", "Finn"})
	russian := NewMatcher([]string{"Шерлок Холмс", "Джон Уотсон", "Ирен Адлер", "инспектор Лестрейд", "профессор Мориарти"})
	russianEight := NewMatcher(tripleBucketEightRussianPatterns)
	hazardEight := NewMatcher(tripleBucketEightHazardPatterns)
	logEight := NewMatcher(tripleBucketEightLogPatterns)
	single := NewMatcher([]string{"Sherlock Holmes"})
	seven := NewMatcher(tripleBucketSevenNamePatterns)
	six := NewMatcher(tripleBucketSixNamePatterns)
	eightHit := NewMatcher(tripleBucketEightHitPatterns)
	ninePatterns := append(append([]string(nil), tripleBucketSevenNamePatterns...), "Sherlock Holmes", "Moriarty")
	nine := NewMatcher(ninePatterns)
	eightComplete := NewMatcher(tripleBucketSixCompletePatterns)

	if !english.plan.triplesComplete || !english.plan.triples.shufti.usable() ||
		!leipzig.plan.triplesComplete || !leipzig.plan.triples.shufti.usable() {
		t.Fatal("existing 4/5-name plans no longer use the complete triple-Shufti owner")
	}
	for _, plan := range []*searchPlan{english.plan, leipzig.plan} {
		if plan.rawByteMulti.usable() || plan.asciiPairAnchors.usable() || plan.unicodePairN != 0 ||
			plan.unicodeAnchor.n != 0 || plan.asciiProbe.usable() || plan.asciiOnly || plan.asciiRun {
			t.Fatal("existing 4/5-name consumer no longer reaches the complete multi-triple plan route")
		}
	}
	if !russian.plan.rawByteMulti.usable() {
		t.Fatal("Russian Rebar counterpart no longer uses its retained raw multi-anchor owner")
	}
	if !seven.plan.triplesComplete || seven.plan.triples.n != 9 || seven.plan.rawByteMulti.usable() ||
		seven.plan.rootKind != rootGeneric || !six.plan.triplesComplete || six.plan.triples.n != 9 ||
		six.plan.rootKind != rootGeneric || !eightHit.plan.triplesComplete || eightHit.plan.triples.n != 10 ||
		eightHit.plan.rootKind != rootGeneric {
		t.Fatalf("six-to-eight-pattern eligibility drifted: seven=%d/%v/%v six=%d/%v eight=%d/%v",
			seven.plan.triples.n, seven.plan.triplesComplete, seven.plan.rawByteMulti.usable(),
			six.plan.triples.n, six.plan.triplesComplete, eightHit.plan.triples.n, eightHit.plan.triplesComplete)
	}
	if eightComplete.plan.triples.n != tripleShuftiSlots || !eightComplete.plan.triples.shufti.usable() {
		t.Fatal("six-pattern eight-form control no longer uses its existing exact Shufti table")
	}
	if nine.plan.patternCount != 9 || !nine.plan.triplesComplete || nine.plan.triples.n <= tripleShuftiSlots ||
		nine.plan.rawByteMulti.usable() || nine.plan.triples.shufti.usable() || nine.plan.tripleBucketFilter().usable() {
		t.Fatal("shared-slot coverage escaped the six-to-eight-pattern limit")
	}
	if russianEight.plan.tripleBucketFilter() != nil || russianEight.plan.triples.shufti.usable() ||
		hazardEight.plan.tripleBucketFilter() != nil || hazardEight.plan.triplesComplete ||
		hazardEight.plan.triples.shufti.usable() || logEight.plan.tripleBucketFilter() != nil ||
		logEight.plan.rootKind == rootGeneric {
		t.Fatal("unaffected eight-pattern benchmark plan installed the shared bucket projection")
	}
	if _, ok := seven.plan.rootBucketEachFilter(strings.Repeat("x", rootBucketEachMinBytes)); ok {
		t.Fatal("seven-name Matcher.Each crossed the existing four/five-pattern root eligibility gate")
	}

	if !asciiPairVBMIEnabled() {
		for _, plan := range []*searchPlan{english.plan, leipzig.plan, seven.plan, six.plan, eightHit.plan} {
			if plan.tripleBucketFilter() != nil {
				t.Fatal("bucket compiled without runtime AVX-512 VBMI")
			}
		}
		if seven.plan.triples.shufti.usable() || six.plan.triples.shufti.usable() || eightHit.plan.triples.shufti.usable() {
			t.Fatal("shared-slot Shufti projection compiled without runtime AVX-512 VBMI")
		}
		t.Skip("plan-owned bucket is runtime-gated to AVX-512 VBMI")
	}

	for _, tc := range []struct {
		name     string
		plan     *searchPlan
		forms    int
		prefixes int
	}{
		{"English five-name", english.plan, 0, 5},
		{"Leipzig four-name", leipzig.plan, 0, 4},
		{"Sherlock seven-name", seven.plan, 9, 7},
		{"six-name boundary", six.plan, 9, 6},
		{"BenchmarkBar eight-name hit", eightHit.plan, 10, 8},
	} {
		bucket := tc.plan.tripleBucketFilter()
		if !bucket.usable() || bucket.prefixCount() != tc.prefixes || !tc.plan.triples.shufti.usable() {
			t.Fatalf("%s bucket=%v prefixes=%d shufti=%v; want %d prefixes and a Shufti fallback",
				tc.name, bucket.usable(), bucket.prefixCount(), tc.plan.triples.shufti.usable(), tc.prefixes)
		}
		if tc.forms != 0 && int(tc.plan.triples.n) != tc.forms {
			t.Fatalf("%s raw forms=%d, want %d", tc.name, tc.plan.triples.n, tc.forms)
		}
	}
	if russian.plan.tripleBucketFilter() != nil || russianEight.plan.tripleBucketFilter() != nil ||
		single.plan.tripleBucketFilter() != nil {
		t.Fatal("bucket escaped the complete multi-literal plan boundary")
	}
}

func TestTripleSharedShuftiContainsStoredForms(t *testing.T) {
	plan := NewMatcher(tripleBucketSevenNamePatterns).plan
	if plan.triples.n <= tripleShuftiSlots {
		t.Fatalf("target raw forms=%d, need more than %d to exercise shared slots", plan.triples.n, tripleShuftiSlots)
	}
	shared := makeTripleSharedShuftiFilter(plan.triples)
	if !shared.usable() {
		t.Fatal("shared-slot projection was not built")
	}
	for i := 0; i < int(plan.triples.n); i++ {
		triple := plan.triples.values[i]
		for variant := 0; variant < 1<<3; variant++ {
			if variant&^int(triple.fold) != 0 {
				continue
			}
			values := [3]byte{triple.first, triple.second, triple.third}
			for position := range values {
				if variant&(1<<position) != 0 {
					values[position] ^= 0x20
				}
			}
			if !tripleShuftiAt(values[0], values[1], values[2], &shared) {
				t.Fatalf("shared projection omitted form %d variant %03b: % x", i, variant, values)
			}
		}
	}

	reversed := plan.triples
	for left, right := 0, int(reversed.n)-1; left < right; left, right = left+1, right-1 {
		reversed.values[left], reversed.values[right] = reversed.values[right], reversed.values[left]
	}
	if got := makeTripleSharedShuftiFilter(reversed); got != shared {
		t.Fatal("shared-slot projection depends on nondeterministic raw-form order")
	}
}

func TestTripleBucketRejectsPrefixOverflowWithoutInstalling(t *testing.T) {
	if !asciiPairVBMIEnabled() {
		t.Skip("prefix-overflow installation test requires the AVX-512 VBMI compiler gate")
	}
	plan := &searchPlan{
		patternCount:    7,
		rootKind:        rootGeneric,
		triplesComplete: true,
		triples:         tripleFilter{n: tripleShuftiSlots + 1},
		tripleRoots:     []byte{1, 0, 1},
		nodes:           []planNode{{output: planOutput{pattern: -1}}},
	}
	const second, third, fourth uint32 = 10, 11, 12
	plan.ascii['!'], plan.ascii['#'], plan.ascii['$'] = second, third, fourth
	for i := 0; i < tripleShuftiSlots+1; i++ {
		first := uint32(i + 1)
		plan.ascii['A'+i] = first
		state1 := len(plan.nodes)
		plan.nodes = append(plan.nodes, planNode{output: planOutput{pattern: -1}})
		state2 := len(plan.nodes)
		plan.nodes = append(plan.nodes, planNode{output: planOutput{pattern: -1}})
		state3 := len(plan.nodes)
		plan.nodes = append(plan.nodes, planNode{output: planOutput{pattern: -1}})
		state4 := len(plan.nodes)
		plan.nodes = append(plan.nodes, planNode{output: planOutput{pattern: i, units: 4}})
		if plan.nodes[0].edges == nil {
			plan.nodes[0].edges = make(map[uint32]int)
		}
		plan.nodes[0].edges[first] = state1
		plan.nodes[state1].edges = map[uint32]int{second: state2}
		plan.nodes[state2].edges = map[uint32]int{third: state3}
		plan.nodes[state3].edges = map[uint32]int{fourth: state4}
	}
	originalRoots := append([]byte(nil), plan.tripleRoots...)
	_, prefixes, ok := plan.collectTripleBucketPrefixes()
	if ok || prefixes != tripleShuftiSlots {
		t.Fatalf("prefix collector returned ok=%v count=%d, want overflow after %d slots", ok, prefixes, tripleShuftiSlots)
	}
	if plan.makeTripleBucketFilter() {
		t.Fatal("installed bucket when the bounded prefix set overflowed")
	}
	if plan.tripleBucketFilter() != nil || plan.triples.shufti.usable() || !reflect.DeepEqual(plan.tripleRoots, originalRoots) {
		t.Fatal("prefix overflow partially installed a bucket or shared Shufti projection")
	}
}

func TestTripleBucketSkip64MatchesModel(t *testing.T) {
	if !asciiPairVBMIEnabled() {
		t.Skip("AVX-512 VBMI bucket path is disabled")
	}
	plan := NewMatcher([]string{
		"Sherlock Holmes", "John Watson", "Irene Adler", "Inspector Lestrade", "Professor Moriarty",
	}).plan
	filter := plan.tripleBucketFilter()
	if !filter.usable() {
		t.Fatal("eligible plan has no bucket")
	}

	rng := rand.New(rand.NewSource(0x5f7a))
	lengths := []int{67, 68, 127, 128, 129, 130, 131, 191, 192, 193, 255, 256, 257, 513, 1025}
	for _, n := range lengths {
		for _, alignment := range []int{0, 1, 31, 63} {
			for _, ascii := range []bool{true, false} {
				backing := make([]byte, alignment+n)
				input := backing[alignment:]
				for i := range input {
					if ascii {
						input[i] = byte('!' + rng.Intn('~'-'!'+1))
					} else {
						input[i] = byte(rng.Intn(256))
					}
				}
				for _, at := range []int{0, 1, 62, 63, 64, 65, 126, 127, 128, 190, 191, 192, 255} {
					if at+4 <= len(input) {
						copy(input[at:], "Sher")
					}
				}
				want := tripleBucketSkip64Model(input, filter, &plan.triples.shufti)
				got := tripleBucketSkip64(unsafe.SliceData(input), len(input), unsafe.SliceData(filter[:tripleBucketTableBytes]), &plan.triples.shufti)
				if got != want {
					t.Fatalf("n=%d align=%d ascii=%v: skip=%d want %d", n, alignment, ascii, got, want)
				}
			}
		}
	}

	// Every location in a three-block horizon must hand its containing block to
	// Shufti when any of its four overlapping source windows sees a high byte.
	for high := 0; high < 3*64+67; high++ {
		input := []byte(strings.Repeat("x", 3*64+67))
		input[high] = 0x80
		want := tripleBucketSkip64Model(input, filter, &plan.triples.shufti)
		if got := tripleBucketSkip64(unsafe.SliceData(input), len(input), unsafe.SliceData(filter[:tripleBucketTableBytes]), &plan.triples.shufti); got != want {
			t.Fatalf("high byte at %d: skip=%d want %d", high, got, want)
		}
	}

	sharedPlan := NewMatcher(tripleBucketSevenNamePatterns).plan
	shared := sharedPlan.tripleBucketFilter()
	if !shared.usable() || !sharedPlan.triples.shufti.usable() {
		t.Fatal("seven-name plan has no shared-slot high-byte fallback")
	}
	for high := 0; high < 3*64+67; high++ {
		input := []byte(strings.Repeat("x", 3*64+67))
		input[high] = 0x80
		want := tripleBucketSkip64Model(input, shared, &sharedPlan.triples.shufti)
		if got := tripleBucketSkip64(unsafe.SliceData(input), len(input), unsafe.SliceData(shared[:tripleBucketTableBytes]), &sharedPlan.triples.shufti); got != want {
			t.Fatalf("shared-Shufti high byte at %d: skip=%d want %d", high, got, want)
		}
	}
}

func collectTripleBucketEach(m *Matcher, haystack string) ([]Match, []int, bool) {
	var matches []Match
	var widths []int
	complete := m.Each(haystack, func(match Match, width int) bool {
		matches = append(matches, match)
		widths = append(widths, width)
		return true
	})
	return matches, widths, complete
}

func TestTripleBucketPreservesPlanResultsAndWidths(t *testing.T) {
	fast := tripleBucketTestMatcher(tripleBucketSevenNamePatterns, true)
	generic := tripleBucketTestMatcher(tripleBucketSevenNamePatterns, false)
	if !fast.plan.tripleBucketFilter().usable() {
		t.Skip("AVX-512 VBMI bucket path is disabled")
	}
	if fast.plan.rawByteMulti.usable() || fast.plan.triples.n <= tripleShuftiSlots ||
		!fast.plan.triples.shufti.usable() {
		t.Fatal("seven-name plan did not install shared Shufti behind its bucket")
	}

	malformed := append([]byte(strings.Repeat("x", 63)), 0xff)
	malformed = append(malformed, strings.Repeat("x", 96)...)
	malformed = append(malformed, []byte("Sherlock Holmes and John Watson")...)
	haystacks := []string{
		strings.Repeat("x", 63) + "Sherlock Holmes and John Watson",
		strings.Repeat("x", 61) + "ſherlock Holmes",
		strings.Repeat("x", 59) + "SherlocK Holmes",
		string(malformed),
		strings.Repeat("x", 64) + "Sherlock Sherlock Holmes John Watson",
		strings.Repeat("z", 257),
		"\x00" + strings.Repeat("x", 63) + "Sherlock Holmes",
	}
	for _, haystack := range haystacks {
		gotFind, gotOK := fast.Find(haystack)
		wantFind, wantOK := generic.Find(haystack)
		if gotFind != wantFind || gotOK != wantOK {
			t.Errorf("Find(%q): bucket=%+v,%v generic=%+v,%v", haystack, gotFind, gotOK, wantFind, wantOK)
		}
		gotMatches, gotWidths, gotComplete := collectTripleBucketEach(fast, haystack)
		wantMatches, wantWidths, wantComplete := collectTripleBucketEach(generic, haystack)
		if gotComplete != wantComplete || !reflect.DeepEqual(gotMatches, wantMatches) || !reflect.DeepEqual(gotWidths, wantWidths) {
			t.Errorf("Each(%q): bucket=%+v/%v complete=%v generic=%+v/%v complete=%v", haystack, gotMatches, gotWidths, gotComplete, wantMatches, wantWidths, wantComplete)
		}
	}

	match, ok := fast.Find(strings.Repeat("x", 64) + "sHerlock Holmes")
	if !ok || match != (Match{Pattern: 0, Start: 64}) {
		t.Fatalf("leftmost/lowest-ID tie = %+v,%v, want pattern 0 at 64", match, ok)
	}
	var first Match
	var firstWidth int
	fast.Each(strings.Repeat("x", 63)+"ſherlock Holmes", func(match Match, width int) bool {
		first, firstWidth = match, width
		return false
	})
	if first != (Match{Pattern: 0, Start: 63}) || firstWidth != len("ſherlock") {
		t.Fatalf("width-changing simple fold = %+v width %d, want byte width %d", first, firstWidth, len("ſherlock"))
	}
	kelvin := strings.Repeat("x", 59) + "SherlocK Holmes"
	fast.Each(kelvin, func(match Match, width int) bool {
		if match != (Match{Pattern: 0, Start: 59}) || width != len("SherlocK") {
			t.Fatalf("Kelvin source width = %+v width %d, want byte width %d", match, width, len("SherlocK"))
		}
		return false
	})
}

func TestTripleBucketKeepsLeftmostTiesAndEarlyStop(t *testing.T) {
	fast := tripleBucketTestMatcher(tripleBucketTiePatterns, true)
	generic := tripleBucketTestMatcher(tripleBucketTiePatterns, false)
	if !fast.plan.tripleBucketFilter().usable() || fast.plan.triples.n <= tripleShuftiSlots {
		t.Skip("the tie fixture does not select the shared-slot bucket")
	}
	for _, tc := range []struct {
		haystack string
		want     Match
	}{
		{strings.Repeat("x", 63) + "Sherlock Holmes", Match{Pattern: 0, Start: 63}},
		{"Holmes xx Sherlock Holmes", Match{Pattern: 2, Start: 0}},
	} {
		got, gotOK := fast.Find(tc.haystack)
		want, wantOK := generic.Find(tc.haystack)
		if !gotOK || !wantOK || got != tc.want || want != tc.want {
			t.Fatalf("Find(%q) = %+v,%v; generic=%+v,%v; want %+v",
				tc.haystack, got, gotOK, want, wantOK, tc.want)
		}
		fastMatches, fastWidths, fastComplete := collectTripleBucketEach(fast, tc.haystack)
		genericMatches, genericWidths, genericComplete := collectTripleBucketEach(generic, tc.haystack)
		if fastComplete != genericComplete || !reflect.DeepEqual(fastMatches, genericMatches) ||
			!reflect.DeepEqual(fastWidths, genericWidths) {
			t.Fatalf("Each(%q) changed order or width: bucket=%+v/%v generic=%+v/%v",
				tc.haystack, fastMatches, fastWidths, genericMatches, genericWidths)
		}
		var fastFirst, genericFirst Match
		var fastWidth, genericWidth int
		fastFinished := fast.Each(tc.haystack, func(match Match, width int) bool {
			fastFirst, fastWidth = match, width
			return false
		})
		genericFinished := generic.Each(tc.haystack, func(match Match, width int) bool {
			genericFirst, genericWidth = match, width
			return false
		})
		if fastFinished || genericFinished || fastFirst != genericFirst || fastWidth != genericWidth ||
			fastFirst != tc.want {
			t.Fatalf("early-stop Each(%q) = %+v/%d/%v, generic=%+v/%d/%v",
				tc.haystack, fastFirst, fastWidth, fastFinished, genericFirst, genericWidth, genericFinished)
		}
	}
}

func TestTripleBucketNonASCIIRootByteSeams(t *testing.T) {
	if !asciiPairVBMIEnabled() {
		t.Skip("AVX-512 VBMI bucket path is disabled")
	}
	patterns := []string{"AbcdЖ", "John Watson", "Irene Adler", "Inspector Lestrade", "Professor Moriarty"}
	for _, needle := range []string{
		"AbcdЖ", string([]byte{'A', 'b', 'c', 'd', 0xff}),
		"AbcЖ", string([]byte{'A', 'b', 'c', 0xff}),
	} {
		patterns[0] = needle
		fast := tripleBucketTestMatcher(patterns, true)
		shufti := tripleBucketTestMatcher(patterns, false)
		if !fast.plan.tripleBucketFilter().usable() || !fast.plan.triples.shufti.usable() || fast.plan.rawByteMulti.usable() {
			t.Fatalf("mixed non-ASCII root-byte plan does not reach bucket+Shufti: bucket=%v shufti=%v rawMulti=%v", fast.plan.tripleBucketFilter().usable(), fast.plan.triples.shufti.usable(), fast.plan.rawByteMulti.usable())
		}
		for _, start := range []int{62, 63, 64, 126, 127, 128} {
			haystack := strings.Repeat("x", start) + needle + strings.Repeat("x", 80)
			want := Match{Pattern: 0, Start: start}
			if got, ok := fast.Find(haystack); !ok || got != want {
				t.Fatalf("needle %q at %d: bucket Find=%+v,%v want %+v", needle, start, got, ok, want)
			}
			if got, ok := shufti.Find(haystack); !ok || got != want {
				t.Fatalf("needle %q at %d: Shufti control Find=%+v,%v want %+v", needle, start, got, ok, want)
			}
			var match Match
			var width int
			fast.Each(haystack, func(m Match, w int) bool { match, width = m, w; return false })
			if match != want || width != len(needle) {
				t.Fatalf("needle %q at %d: bucket Each=%+v width %d, want width %d", needle, start, match, width, len(needle))
			}
		}
	}
}

func TestTripleBucketFallsBackWithoutVBMI(t *testing.T) {
	if !asciiPairVBMIEnabled() {
		t.Skip("the runtime has no AVX-512 VBMI to disable")
	}
	fast := tripleBucketTestMatcher(tripleBucketSevenNamePatterns, true)
	if !fast.plan.tripleBucketFilter().usable() {
		t.Fatal("eligible plan did not compile its bucket")
	}
	want := tripleBucketTestMatcher(tripleBucketSevenNamePatterns, false)
	hadVBMI := cpu.X86.HasAVX512VBMI
	cpu.X86.HasAVX512VBMI = false
	defer func() { cpu.X86.HasAVX512VBMI = hadVBMI }()
	for _, haystack := range []string{
		strings.Repeat("x", 1<<12) + "Sherlock Holmes",
		strings.Repeat("x", 63) + "ſherlock Holmes",
		strings.Repeat("x", 67) + "SherlocK Holmes",
		string([]byte{0xff, 'x', 'x'}) + strings.Repeat("x", 96) + "John Watson",
	} {
		got, gotOK := fast.Find(haystack)
		wantMatch, wantOK := want.Find(haystack)
		if got != wantMatch || gotOK != wantOK {
			t.Fatalf("VBMI-disabled Find(%q) = %+v,%v, want %+v,%v", haystack, got, gotOK, wantMatch, wantOK)
		}
		gotMatches, gotWidths, gotComplete := collectTripleBucketEach(fast, haystack)
		wantMatches, wantWidths, wantComplete := collectTripleBucketEach(want, haystack)
		if gotComplete != wantComplete || !reflect.DeepEqual(gotMatches, wantMatches) || !reflect.DeepEqual(gotWidths, wantWidths) {
			t.Fatalf("VBMI-disabled Each(%q) differs: bucket=%+v/%v Shufti=%+v/%v", haystack, gotMatches, gotWidths, wantMatches, wantWidths)
		}
	}
}

func TestTripleBucketDoesNotCompileWithoutVBMI(t *testing.T) {
	if !asciiPairVBMIEnabled() {
		t.Skip("the runtime has no AVX-512 VBMI to disable")
	}
	hadVBMI := cpu.X86.HasAVX512VBMI
	cpu.X86.HasAVX512VBMI = false
	defer func() { cpu.X86.HasAVX512VBMI = hadVBMI }()

	matcher := NewMatcher(tripleBucketSevenNamePatterns)
	if matcher.plan.tripleBucketFilter().usable() || matcher.plan.triples.shufti.usable() {
		t.Fatal("shared bucket state compiled while AVX-512 VBMI was disabled")
	}
	var got Match
	var width int
	complete := matcher.Each(strings.Repeat("x", 63)+"ſherlock Holmes", func(match Match, sourceWidth int) bool {
		got, width = match, sourceWidth
		return false
	})
	if complete || got != (Match{Pattern: 0, Start: 63}) || width != len("ſherlock") {
		t.Fatalf("feature-off Each = (%+v, %d, complete=%v), want first long-s match at byte 63 with width %d",
			got, width, complete, len("ſherlock"))
	}
}

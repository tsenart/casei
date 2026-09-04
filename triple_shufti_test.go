package casei

import "strings"

var tripleShuftiEachPatterns = []string{
	"a@b", "c\\d", "e`f", "g|h", "i!j", "l?m", "n#m", "o%p", "a@bX", "щупальце",
}

// makeNormalizedTripleShuftiForProof recreates the old representation for the
// comparison arm. The old scan ORed bit five into every input byte, so each
// normalized table value has both raw preimages here. The helper is retained in
// the proof fixture so the exact route can be compared with its predecessor in
// the same Matcher.Each workload.
func makeNormalizedTripleShuftiForProof(filter tripleFilter) tripleShuftiFilter {
	var out tripleShuftiFilter
	for i := range filter.n {
		triple := filter.values[i]
		values := [3]byte{triple.first, triple.second, triple.third}
		los := [3]*[16]byte{&out.firstLo, &out.secondLo, &out.thirdLo}
		his := [3]*[16]byte{&out.firstHi, &out.secondHi, &out.thirdHi}
		bit := byte(1 << uint(i))
		for position, value := range values {
			value |= 0x20
			for _, raw := range [2]byte{value, value ^ 0x20} {
				los[position][raw&0x0f] |= bit
				his[position][raw>>4] |= bit
			}
		}
	}
	out.valid = 1
	return out
}

// tripleShuftiProofCounters are test-only counters. The counter scan below
// calls the same route directly after the public Matcher.Each comparison; the
// production plan has no counter field or hot-path accounting branch.
type tripleShuftiProofCounters struct {
	routes uint64
	stops  uint64
}

func tripleShuftiProofMatcher(patterns []string, mode string) *Matcher {
	base := NewMatcher(patterns)
	plan := *base.plan
	if plan.asciiPairAnchors.usable() || plan.rawByteMulti.usable() || !plan.asciiTriplesComplete || !plan.asciiTriples.shufti.usable() {
		panic("ordinary triple-Shufti fixture selected another route")
	}
	switch mode {
	case "normalized":
		plan.asciiTriples.shufti = makeNormalizedTripleShuftiForProof(plan.asciiTriples)
	case "explicit":
	case "generic":
		plan.triples = plan.asciiTriples
		plan.triples.shufti = tripleShuftiFilter{}
		plan.triplesComplete = true
		plan.asciiTriplesComplete = false
		plan.filter = rootFilter{}
	default:
		panic("unknown triple Shufti proof mode: " + mode)
	}
	return &Matcher{patterns: base.patterns, plan: &plan}
}

// tripleShuftiProofCount drives the actual bounded route once and counts the
// candidate stops it returns. It is deliberately outside Matcher.Each so the
// proof instrumentation cannot become product work or contaminate timing.
func tripleShuftiProofCount(haystack string, filter *tripleShuftiFilter) tripleShuftiProofCounters {
	var counters tripleShuftiProofCounters
	if !filter.usable() {
		return counters
	}
	if runtimeVectorBits() == 512 && len(haystack) >= 66 {
		counters.routes = 1
	}
	for at := 0; at+2 < len(haystack); {
		skipped := tripleShuftiSkipBytes(haystack, at, filter)
		if skipped == 0 {
			counters.stops++
			at++
			continue
		}
		at += skipped
	}
	return counters
}

func tripleShuftiEachCorpus() string {
	data := []byte(strings.Repeat("x", 1<<20))
	aliases := [][3]byte{
		{'A', '`', 'B'},
		{'C', '|', 'D'},
		{'E', '@', 'F'},
		{'G', '\\', 'H'},
		{'I', 1, 'J'},
		{'L', 31, 'M'},
	}
	for at := 0; at+3 <= len(data)-3; at += 64 {
		copy(data[at:], aliases[(at/64)%len(aliases)][:])
	}
	copy(data[len(data)-3:], "A@B")
	return string(data)
}

var (
	tripleShuftiEachSink      Match
	tripleShuftiEachSinkWidth int
)

func tripleShuftiEachYield(match Match, width int) bool {
	tripleShuftiEachSink = match
	tripleShuftiEachSinkWidth = width
	return true
}

package casei

import (
	"sync/atomic"
	"unicode"
	"unicode/utf8"
)

// searchPlan is the package-owned compiled representation used by both public
// search entry points. It maps every fold orbit needed by the pattern set to a
// compact token, then advances one deterministic state machine over decoded
// haystack units. Valid UTF-8 runes and invalid bytes intentionally take
// separate paths: an invalid byte is an opaque singleton and can never match a
// continuation byte of a valid rune.
//
// The implementation is original to this package. It does not import, link,
// or delegate lookup to an arena entrant.
type searchPlan struct {
	// ascii and opaque map respectively valid one-byte runes and invalid UTF-8
	// bytes to plan tokens. Token zero means that no pattern can consume the
	// input unit, which always returns the state machine to its root.
	ascii                [utf8.RuneSelf]uint32
	opaque               [256]uint32
	rootByte             [256]uint8
	rootKind             uint8
	rootNeedle           byte
	pairKind             uint8
	pairNeedle           byte
	filter               rootFilter
	pairSecond           bool
	triples              tripleFilter
	tripleRoots          []bool
	triplesComplete      bool
	asciiTriples         tripleFilter
	asciiTriplesComplete bool
	// asciiPairAnchors is the all-ASCII multi-pattern transition for the
	// partial-triple shape. Its bounded pair table only nominates starts; the
	// decoded plan replays each survivor and remains the match authority.
	asciiPairAnchors asciiPairAnchors
	// asciiProbe is a single-pattern, byte-aligned block transition. It
	// intersects three dispersed literal positions, then confirms the same
	// compiled pattern at the surviving start.
	asciiProbe     asciiProbe
	asciiOnlyProbe asciiProbe
	// singlePayload is the literal for the all-ASCII route. That route is
	// disabled for Unicode patterns, where this otherwise unused string instead
	// holds the packed raw terminal confirmation for the N=1 VBMI transition.
	singlePayload     string
	asciiOnlyWord     uint64
	asciiOnlyFold     uint64
	asciiOnly         bool
	asciiOnlyLong     bool
	asciiPair         asciiPairProbe
	asciiNeedle       string
	asciiFirstWord    uint64
	asciiFirstFold    uint64
	asciiTailWord     uint64
	asciiTailFold     uint64
	asciiTailMask     uint64
	asciiVerifyTokens bool
	asciiFixedPrefix  int
	asciiByteAnchor   bool
	asciiStaticAnchor bool
	asciiStaticAt     int
	asciiStaticKind   uint8
	asciiStaticByte   byte
	asciiRun          bool
	asciiRunKind      uint8
	asciiRunByte      byte
	unicodeAnchor     tripleFilter
	unicodeAt         int
	unicodePairs      [8]unicodePairAnchor
	unicodePairN      uint8
	singleTokens      []uint32
	// rawByteMulti is the fixed, tagged interior-pair screen used only by
	// eligible multi-pattern enumeration. It is compiled into the plan with the
	// raw transition map; it never depends on a caller's haystack or history.
	rawByteMulti       rawByteMultiAnchorFilter
	rawByteOrigin      rawByteOriginGate
	runes              map[rune]uint32
	opaqueContinuation bool

	nodes []planNode

	// dense contains complete token transitions when the product of states and
	// tokens is small enough to be useful. Larger, sparse plans retain their
	// trie edges and follow failure links instead of allocating a quadratic
	// table. Both forms implement the same state transition.
	dense  []uint32
	stride int

	empty    int
	maxUnits int
	// maxBytes bounds the source bytes consumed by any simple-fold spelling
	// of a compiled pattern. Boundary windows use it to preserve cross-run matches.
	maxBytes     int
	patternCount int
}

// rootFilter is a compact byte-prefix description of every root token. It is
// only used when all root encodings fit in the bounded sets below. One-byte
// candidates include ASCII and non-continuation opaque bytes; pairs identify
// the first two bytes of valid UTF-8 root forms. A pair test is conservative:
// decoding and the regular transition still decide whether it is a root.
type rootFilter struct {
	ones  [16]byte
	pairs [16]rootPair
	oneN  uint8
	pairN uint8

	// shufti is a bounded exact projection of a dense raw-pair set. It is
	// populated only for the shape where the AVX-512 nibble transition is less
	// work than comparing every pair separately; the regular filter remains
	// the semantic authority after every survivor.
	shufti pairShuftiFilter
}

const (
	pairShuftiGroups    = 2
	pairShuftiSlots     = 8
	asciiTripleMinBytes = 64

	// A rolling exceptional-byte budget keeps a sparse prefix from admitting a
	// later dense region. The partitioner checks the same bounded neighborhood
	// as its entry guard while it discovers subsequent spans.
	asciiPartitionSampleBytes    = 1024
	asciiPartitionMaxHigh        = 16
	asciiPartitionLookaheadBytes = asciiPartitionSampleBytes * asciiPartitionMaxHigh
)

// pairShuftiGroup stores four nibble-to-slot tables. The slot bits identify
// pair renderings within this group. Intersecting all four lookups is exact for
// raw tables and conservative for byte-5-normalized tables; either form only
// filters before the decoded plan transition.
//
// The 16-byte table layout is consumed by pairShuftiSkip64 through
// VBROADCASTI32X4, which repeats each table in the four 128-bit VPSHUFB lanes.
type pairShuftiGroup struct {
	firstLo  [16]byte
	firstHi  [16]byte
	secondLo [16]byte
	secondHi [16]byte
}

type pairShuftiFilter struct {
	groups [pairShuftiGroups]pairShuftiGroup

	// The normalized dense route also admits exactly two single-byte roots.
	// Keeping them after the tables preserves the assembly table offsets.
	ones  [2]byte
	oneN  uint8
	valid uint8
}

func (f *pairShuftiFilter) usable() bool { return f.valid != 0 }

const (
	rootPairFoldFirst = 1 << iota
	rootPairFoldSecond
)

// rootPair is four bytes so the amd64 block probes can index it directly.
// fold marks ASCII letters that are normalized with OR 0x20 before comparison.
type rootPair struct {
	first, second byte
	fold          uint8
	_             uint8
}

// makePairShuftiFilter assigns compact pair renderings to two nibble-table
// groups. A byte gets a slot bit only when both its nibbles agree with that
// rendering. Pure-pair filters expand their ASCII folds exactly; the mixed
// two-one-root form normalizes byte five instead, which is conservative but
// keeps its dense orbit set bounded. The compiler deliberately keeps this
// narrow: a smaller set is cheaper in the regular broadcast-and-compare
// transition, while more than two groups would erase the block transition's
// advantage.
func makePairShuftiFilter(filter rootFilter) pairShuftiFilter {
	if filter.pairN < 5 || filter.oneN > 2 || filter.oneN == 1 {
		return pairShuftiFilter{}
	}

	// A mixed one/pair root set cannot use the exact expanded-pair tables: its
	// ten-orbit hazard shape expands beyond two eight-slot groups. Folding bit
	// five into every pair component preserves every true root while allowing
	// harmless extra survivors, and the two raw one-byte roots are admitted by
	// the companion transition below.
	normalize := filter.oneN == 2

	type rawPair struct{ first, second byte }
	var raw [pairShuftiGroups * pairShuftiSlots]rawPair
	rawN := 0
	addRaw := func(first, second byte) bool {
		for i := range rawN {
			if raw[i] == (rawPair{first: first, second: second}) {
				return true
			}
		}
		if rawN == len(raw) {
			return false
		}
		raw[rawN] = rawPair{first: first, second: second}
		rawN++
		return true
	}

	for i := range filter.pairN {
		pair := filter.pairs[i]
		if normalize {
			if !addRaw(pair.first|0x20, pair.second|0x20) {
				return pairShuftiFilter{}
			}
			continue
		}
		first, second := [2]byte{pair.first}, [2]byte{pair.second}
		firstN, secondN := 1, 1
		if pair.fold&rootPairFoldFirst != 0 && isASCIILetter(pair.first) {
			first[1] = pair.first ^ 0x20
			firstN = 2
		}
		if pair.fold&rootPairFoldSecond != 0 && isASCIILetter(pair.second) {
			second[1] = pair.second ^ 0x20
			secondN = 2
		}
		for _, left := range first[:firstN] {
			for _, right := range second[:secondN] {
				if !addRaw(left, right) {
					return pairShuftiFilter{}
				}
			}
		}
	}
	// One group would add four vector shuffles without removing enough of the
	// regular filter's pair work. The dense multi-hazard root set expands to
	// two groups and is the shape this transition is for.
	if rawN <= pairShuftiSlots {
		return pairShuftiFilter{}
	}

	var out pairShuftiFilter
	if normalize {
		out.ones, out.oneN = [2]byte{filter.ones[0], filter.ones[1]}, 2
	}
	for i := range rawN {
		pair := raw[i]
		group := &out.groups[i/pairShuftiSlots]
		bit := byte(1 << uint(i%pairShuftiSlots))
		group.firstLo[pair.first&0x0f] |= bit
		group.firstHi[pair.first>>4] |= bit
		group.secondLo[pair.second&0x0f] |= bit
		group.secondHi[pair.second>>4] |= bit
	}
	out.valid = 1
	return out
}

func pairShuftiAt(first, second byte, filter *pairShuftiFilter) bool {
	for i := range filter.oneN {
		if first == filter.ones[i] {
			return true
		}
	}
	if filter.oneN != 0 {
		first |= 0x20
		second |= 0x20
	}
	for i := range filter.groups {
		group := &filter.groups[i]
		firstMask := group.firstLo[first&0x0f] & group.firstHi[first>>4]
		secondMask := group.secondLo[second&0x0f] & group.secondHi[second>>4]
		if firstMask&secondMask != 0 {
			return true
		}
	}
	return false
}

func pairShuftiSkipScalar(s string, at int, filter *pairShuftiFilter) int {
	start := at
	for at+1 < len(s) {
		if pairShuftiAt(s[at], s[at+1], filter) {
			return at - start
		}
		at++
	}
	// The pair projection needs a following byte, but mixed filters can also
	// admit a one-byte root. Check that final byte before reporting the whole
	// tail skippable.
	if at < len(s) {
		for i := range filter.oneN {
			if s[at] == filter.ones[i] {
				return at - start
			}
		}
	}
	return len(s) - start
}

// tripleFilter represents deterministic ASCII-letter roots through three
// folded bytes. Non-ASCII renderings of the same root remain in rootFilter.
type tripleFilter struct {
	values [16]rootTriple
	n      uint8

	// shufti packs up to eight three-byte forms into six nibble-to-slot
	// tables. Fold-marked ASCII positions add both exact cases; raw positions
	// add only their exact byte. The regular decoded plan still decides every
	// survivor.
	shufti tripleShuftiFilter
}

type rootTriple struct{ first, second, third, fold byte }

const tripleShuftiSlots = 8

// tripleShuftiFilter is a single-group three-byte Shufti projection. Each
// table maps one nibble to the set of triple slots that allow it. Intersecting
// the six table results leaves a slot bit for a possible triple. Fold-marked
// ASCII bytes contribute their two cases; unmarked bytes contribute one raw
// value, so the projection has the same predicate as the generic triple loop.
//
// The six consecutive 16-byte tables are consumed by tripleShuftiSkip64 via
// VBROADCASTI32X4, which repeats each table in the four 128-bit VPSHUFB lanes.
type tripleShuftiFilter struct {
	firstLo  [16]byte
	firstHi  [16]byte
	secondLo [16]byte
	secondHi [16]byte
	thirdLo  [16]byte
	thirdHi  [16]byte
	valid    uint8
}

func (f *tripleShuftiFilter) usable() bool { return f.valid != 0 }

func makeTripleShuftiFilter(filter tripleFilter) tripleShuftiFilter {
	// The existing two- and three-form transitions use fewer instructions.
	// A table has one bit per form, so broader unions need a second group and
	// lose the benefit over the regular compare loop.
	if filter.n < 4 || filter.n > tripleShuftiSlots {
		return tripleShuftiFilter{}
	}

	var out tripleShuftiFilter
	add := func(lo, hi *[16]byte, value, bit byte) {
		(*lo)[value&0x0f] |= bit
		(*hi)[value>>4] |= bit
	}
	for i := range filter.n {
		triple := filter.values[i]
		values := [3]byte{triple.first, triple.second, triple.third}
		los := [3]*[16]byte{&out.firstLo, &out.secondLo, &out.thirdLo}
		his := [3]*[16]byte{&out.firstHi, &out.secondHi, &out.thirdHi}
		bit := byte(1 << uint(i))
		for position, value := range values {
			add(los[position], his[position], value, bit)
			if triple.fold&(1<<uint(position)) != 0 {
				// makeTripleFilter marks only one-byte ASCII letters. Their
				// generic predicate accepts exactly these two values.
				add(los[position], his[position], value^0x20, bit)
			}
		}
	}
	out.valid = 1
	return out
}

func tripleShuftiAt(first, second, third byte, filter *tripleShuftiFilter) bool {
	matches := filter.firstLo[first&0x0f] & filter.firstHi[first>>4]
	matches &= filter.secondLo[second&0x0f] & filter.secondHi[second>>4]
	matches &= filter.thirdLo[third&0x0f] & filter.thirdHi[third>>4]
	return matches != 0
}

func tripleShuftiSkipScalar(s string, at int, filter *tripleShuftiFilter) int {
	start := at
	for at+2 < len(s) {
		if tripleShuftiAt(s[at], s[at+1], s[at+2], filter) {
			return at - start
		}
		at++
	}
	return at - start
}

const asciiPairAnchorSlots = 8

// asciiPairVBMIAnchorFilter is the exact 128-entry VPERMT2B projection of
// one bounded ASCII pair group. Bit six chooses one of each pair of tables, so
// the all-ASCII route need not introduce the low-six-bit aliases of VPERMB.
type asciiPairVBMIAnchorFilter struct {
	firstLo, firstHi   [64]byte
	secondLo, secondHi [64]byte
	valid              uint8
}

// asciiPairAnchorFilter is a one-group Shufti projection for the selected
// adjacent ASCII pairs. All table bytes are normalized with bit five because
// the table is only a conservative filter; the shared decoded plan validates
// every resulting start. The four tables are consecutive for the AVX-512 BW
// transition, while vbmi holds its optional AVX-512 VBMI projection.
type asciiPairAnchorFilter struct {
	firstLo  [16]byte
	firstHi  [16]byte
	secondLo [16]byte
	secondHi [16]byte
	valid    uint8
	vbmi     asciiPairVBMIAnchorFilter
}

// asciiPairAnchor identifies one fixed-width pair inside a pattern's ASCII
// fold spelling. pattern is descriptive only: the plan, not this record,
// selects the actual terminal and tie after the candidate replay.
type asciiPairAnchor struct {
	first, second, at byte
	pattern           int
}

// asciiPairAnchors is intentionally bounded to one table group. More pairs
// cost another four shuffles and leave the existing triple transition faster.
type asciiPairAnchors struct {
	anchors [asciiPairAnchorSlots]asciiPairAnchor
	n       uint8
	maxAt   uint8
	filter  asciiPairAnchorFilter
}

func (a *asciiPairAnchors) usable() bool { return a.n != 0 && a.filter.valid != 0 }

func makeASCIIPairAnchorFilter(anchors *asciiPairAnchors) asciiPairAnchorFilter {
	var out asciiPairAnchorFilter
	for i := range anchors.n {
		anchor := anchors.anchors[i]
		bit := byte(1 << uint(i))
		out.firstLo[anchor.first&0x0f] |= bit
		out.firstHi[anchor.first>>4] |= bit
		out.secondLo[anchor.second&0x0f] |= bit
		out.secondHi[anchor.second>>4] |= bit
		// asciiPairAnchorMatches folds bit five unconditionally, including
		// punctuation aliases, so preserve that exact conservative admission.
		addASCIIVBMI128Slot(&out.vbmi.firstLo, &out.vbmi.firstHi, anchor.first, 0x20, bit)
		addASCIIVBMI128Slot(&out.vbmi.secondLo, &out.vbmi.secondHi, anchor.second, 0x20, bit)
	}
	if anchors.n != 0 {
		out.valid = 1
		out.vbmi.valid = 1
	}
	return out
}

func asciiPairAnchorFilterAt(first, second byte, filter *asciiPairAnchorFilter) bool {
	first |= 0x20
	second |= 0x20
	matches := filter.firstLo[first&0x0f] & filter.firstHi[first>>4]
	matches &= filter.secondLo[second&0x0f] & filter.secondHi[second>>4]
	return matches != 0
}

func asciiPairAnchorSkipScalar(s string, at int, filter *asciiPairAnchorFilter) int {
	start := at
	for at+1 < len(s) {
		if asciiPairAnchorFilterAt(s[at], s[at+1], filter) {
			return at - start
		}
		at++
	}
	return len(s) - start
}

func asciiPairAnchorMatches(s string, at int, anchor asciiPairAnchor) bool {
	return s[at]|0x20 == anchor.first && s[at+1]|0x20 == anchor.second
}

// pairPairVBMIFilter maps each raw byte to an alternative slot for the two
// dispersed pairs. VPERMB's low-six-bit index can over-admit a byte alias, but
// never removes a real raw pair; the shared plan confirms every survivor.
type pairPairVBMIFilter struct {
	first, second, confirmFirst, confirmSecond [64]byte
	offset                                     byte
	valid                                      uint8
}

type pairPairFilter struct {
	first0, second0, first1, second1             byte
	confirmFirst0, confirmSecond0, confirmFirst1 byte
	confirmSecond1, offset, valid                byte
	vbmi                                         pairPairVBMIFilter
}

type unicodePairAnchor struct {
	filter    rootFilter
	at        int
	confirm   rootFilter
	confirmAt int
	pairPair  pairPairFilter
}

// asciiVBMIProbe is the AVX-512 VBMI projection for one sparse three-byte
// ASCII-letter probe. VPERMB indexes only the low six input bits. Each table
// therefore admits the bit-six alias too; it is a conservative filter and the
// ordinary plan replay remains responsible for exact matching.
type asciiVBMIProbe struct {
	firstAt, secondAt, thirdAt int
	first, second, third       [64]byte
	valid                      uint8
}

// asciiPairVBMIProbe is the corresponding two-byte projection for long fixed
// ASCII literals. The tables contain a non-zero lane value only for a possible
// spelling of their respective anchor byte. secondAt is independent from the
// baseline pair probe so the VBMI kernel can classify a rarer later letter
// without changing the portable or non-VBMI transition.
type asciiPairVBMIProbe struct {
	first, second [64]byte
	secondAt      uint8
	valid         uint8
}

// asciiShortLiteral records the fully compiled four-byte confirmation used
// after a dense ASCII probe survivor. The fold mask has bit five only where the
// source spelling has an ASCII letter, so punctuation remains exact.
type asciiShortLiteral struct {
	word, fold uint32
	valid      uint8
}

// asciiProbe keeps the three byte tests and their source offsets together so
// the vector block transition can load each test directly from a candidate
// start. Its leading 32 bytes are intentionally stable for the existing amd64
// assembly path; the optional VBMI projection follows them.
type asciiProbe struct {
	first, second, third, fold byte
	firstAt, secondAt, thirdAt int
	vbmi                       asciiVBMIProbe
	short                      asciiShortLiteral
}

// asciiPairProbe is the fixed-offset two-byte form used by the long-literal
// AVX-512 path. secondAt is deliberately eight: short inputs can carry the
// next 64-byte block with VALIGNQ, while large scans use independent direct
// loads to expose more block-level parallelism. This path is restricted to a
// width-invariant ASCII literal, so direct byte confirmation is the same
// plan's token transition. Its leading 16 bytes remain the layout consumed by
// the AVX-512 BW kernels.
type asciiPairProbe struct {
	first, second         byte
	firstFold, secondFold byte
	secondAt              int
	vbmi                  asciiPairVBMIProbe
}

// addASCIIVBMISlot records every source spelling admitted by the existing
// byte comparison. The low-six-bit projection intentionally adds bit-six
// aliases, never removes a true candidate.
func addASCIIVBMISlot(table *[64]byte, value, fold, slot byte) {
	table[value&0x3f] |= slot
	if fold != 0 {
		table[(value^fold)&0x3f] |= slot
	}
}

func addPairPairVBMISlot(table *[64]byte, value, slot byte) {
	table[value&0x3f] |= slot
}

func makePairPairVBMIFilter(filter *pairPairFilter) {
	var out pairPairVBMIFilter
	addPairPairVBMISlot(&out.first, filter.first0, 1)
	addPairPairVBMISlot(&out.second, filter.second0, 1)
	addPairPairVBMISlot(&out.first, filter.first1, 2)
	addPairPairVBMISlot(&out.second, filter.second1, 2)
	addPairPairVBMISlot(&out.confirmFirst, filter.confirmFirst0, 1)
	addPairPairVBMISlot(&out.confirmSecond, filter.confirmSecond0, 1)
	addPairPairVBMISlot(&out.confirmFirst, filter.confirmFirst1, 2)
	addPairPairVBMISlot(&out.confirmSecond, filter.confirmSecond1, 2)
	out.offset, out.valid = filter.offset, 1
	filter.vbmi = out
}

func addASCIIVBMI128Slot(lo, hi *[64]byte, value, fold, slot byte) {
	add := func(value byte) {
		if value&0x40 == 0 {
			lo[value&0x3f] |= slot
		} else {
			hi[value&0x3f] |= slot
		}
	}
	add(value)
	if fold != 0 {
		add(value ^ fold)
	}
}

func makeASCIIVBMIProbe(probe *asciiProbe) {
	var out asciiVBMIProbe
	out.firstAt, out.secondAt, out.thirdAt = probe.firstAt, probe.secondAt, probe.thirdAt
	tables := [3]*[64]byte{&out.first, &out.second, &out.third}
	values := [3]byte{probe.first, probe.second, probe.third}
	// Table lookup reduces steady-state miss work, while direct comparisons
	// retain lower candidate latency for punctuation and whitespace anchors.
	for _, value := range values {
		if !isASCIILetter(value) {
			probe.vbmi = out
			return
		}
	}
	for i, value := range values {
		fold := byte(0)
		if probe.fold&(1<<i) != 0 {
			fold = 0x20
		}
		addASCIIVBMISlot(tables[i], value, fold, 1)
	}
	out.valid = 1
	probe.vbmi = out
}

func makeASCIIShortLiteral(pattern string) asciiShortLiteral {
	if len(pattern) != 4 {
		return asciiShortLiteral{}
	}
	var out asciiShortLiteral
	for i := range pattern {
		value := pattern[i]
		if isASCIILetter(value) {
			value |= 0x20
			out.fold |= 0x20 << (8 * i)
		}
		out.word |= uint32(value) << (8 * i)
	}
	out.valid = 1
	return out
}

func asciiShortLiteralAt(s string, at int, literal asciiShortLiteral) bool {
	got := uint32(s[at]) | uint32(s[at+1])<<8 | uint32(s[at+2])<<16 | uint32(s[at+3])<<24
	return got|literal.fold == literal.word
}

func makeASCIIPairVBMIProbe(probe *asciiPairProbe, pattern string) {
	var out asciiPairVBMIProbe
	out.secondAt = uint8(probe.secondAt)
	if !isASCIILetter(probe.first) || !isASCIILetter(probe.second) {
		probe.vbmi = out
		return
	}

	second, secondFold := probe.second, probe.secondFold
	// The byte-zero and byte-eight anchors are the established portable pair.
	// Only when both are common can the VBMI-only long-input route exchange the
	// latter for a rarer later ASCII letter. The same plan confirms every table
	// survivor, so changing this filter displacement cannot affect semantics.
	if asciiRarity(probe.first) == 2 && asciiRarity(probe.second) == 2 {
		bestRank := 2
		for at := probe.secondAt + 1; at < len(pattern); at++ {
			value := pattern[at]
			if !isASCIILetter(value) {
				continue
			}
			value |= 0x20
			rank := asciiRarity(value)
			if rank >= bestRank {
				continue
			}
			out.secondAt, second, secondFold = uint8(at), value, 0x20
			bestRank = rank
			if rank == 0 {
				break
			}
		}
	}

	addASCIIVBMISlot(&out.first, probe.first, probe.firstFold, 1)
	addASCIIVBMISlot(&out.second, second, secondFold, 1)
	out.valid = 1
	probe.vbmi = out
}

func (p *asciiProbe) usable() bool { return p.thirdAt != 0 || p.secondAt != 0 || p.firstAt != 0 }

func (p *asciiPairProbe) usable() bool { return p.secondAt != 0 }

func asciiPairAt(s string, at int, probe *asciiPairProbe) bool {
	return s[at]|probe.firstFold == probe.first &&
		s[at+probe.secondAt]|probe.secondFold == probe.second
}

func (f *tripleFilter) usable() bool { return f.n != 0 }

// arrangeTripleMixed puts the common two ASCII-folded plus one raw UTF-8
// rendering shape in the order consumed by the dedicated AVX-512 transition.
// Reordering an OR-set does not change its filter semantics.
func arrangeTripleASCIIUTF8(f *tripleFilter) bool {
	if f.n != 2 {
		return false
	}
	ascii, raw := -1, -1
	for i := range f.n {
		triple := f.values[i]
		switch {
		case triple.fold == 7:
			ascii = int(i)
		case triple.fold == 0 && triple.first >= utf8.RuneSelf:
			raw = int(i)
		}
	}
	if ascii < 0 || raw < 0 {
		return false
	}
	if ascii != 0 {
		f.values[0], f.values[ascii] = f.values[ascii], f.values[0]
		if raw == 0 {
			raw = ascii
		}
	}
	if raw != 1 {
		f.values[1], f.values[raw] = f.values[raw], f.values[1]
	}
	return true
}

func arrangeTripleSharedPrefix(f *tripleFilter) bool {
	if f.n != 2 {
		return false
	}
	for ascii := range f.n {
		raw := 1 - ascii
		left, right := f.values[ascii], f.values[raw]
		if left.fold == 7 && right.fold == 3 && right.third >= utf8.RuneSelf &&
			left.first == right.first && left.second == right.second {
			if ascii != 0 {
				f.values[0], f.values[1] = f.values[1], f.values[0]
			}
			return true
		}
	}
	return false
}

func arrangeTripleMixed(f *tripleFilter) bool {
	if f.n != 3 {
		return false
	}
	var ascii [2]rootTriple
	asciiN := 0
	var raw rootTriple
	for i := range f.n {
		triple := f.values[i]
		switch {
		case triple.fold == 7 && asciiN < len(ascii):
			ascii[asciiN] = triple
			asciiN++
		case triple.fold == 4 && triple.first >= utf8.RuneSelf:
			raw = triple
		default:
			return false
		}
	}
	if asciiN != len(ascii) || raw.first == 0 {
		return false
	}
	f.values[0], f.values[1], f.values[2] = ascii[0], ascii[1], raw
	return true
}

func (f *tripleFilter) add(first, second, third, fold byte) bool {
	triple := rootTriple{first: first, second: second, third: third, fold: fold}
	for i := range f.n {
		if f.values[i] == triple {
			return true
		}
	}
	if int(f.n) == len(f.values) {
		return false
	}
	f.values[f.n] = triple
	f.n++
	return true
}

func (f *rootFilter) usable() bool { return f.oneN != 0 || f.pairN != 0 }

func (f *rootFilter) addOne(value byte) bool {
	for i := range f.oneN {
		if f.ones[i] == value {
			return true
		}
	}
	if int(f.oneN) == len(f.ones) {
		return false
	}
	f.ones[f.oneN] = value
	f.oneN++
	return true
}

func (f *rootFilter) addPair(first, second byte) bool {
	return f.addFoldPair(first, second, 0)
}

func makePairSecond(filter *rootFilter) bool {
	if filter.oneN != 0 || filter.pairN != 2 {
		return false
	}
	for i := range filter.pairN {
		if filter.pairs[i].fold != 0 {
			return false
		}
	}
	return true
}

func (f *rootFilter) addFoldPair(first, second, fold uint8) bool {
	pair := rootPair{first: first, second: second, fold: fold}
	for i := range f.pairN {
		if f.pairs[i] == pair {
			return true
		}
	}
	if int(f.pairN) == len(f.pairs) {
		return false
	}
	f.pairs[f.pairN] = pair
	f.pairN++
	return true
}

type planNode struct {
	edges   map[uint32]int
	failure int
	output  planOutput
}

// planOutput is the only terminal that matters at one input end position.
// Among patterns ending there, the longest starts furthest left; equal-length
// duplicates tie to the lowest pattern index. Shorter terminals are observed
// at their own end positions before they can be relevant to a leftmost result.
type planOutput struct {
	pattern int
	units   int
}

const (
	maxDensePlanTransitions = 1 << 20
	singlePlanCacheSlots    = 64
)

const (
	rootGeneric uint8 = iota
	rootExact
	rootASCIIFold
)

type singlePlanCacheEntry struct {
	needle string
	plan   *searchPlan
}

// IndexFold has no caller-owned Matcher on which to retain its N=1 plan. Keep
// a small direct-mapped cache of immutable plans instead. A collision only
// replaces a cached compilation; it never changes matching state or results.
var (
	singlePlanCache  [singlePlanCacheSlots]atomic.Pointer[singlePlanCacheEntry]
	recentSinglePlan atomic.Pointer[singlePlanCacheEntry]
)

func cachedSinglePlan(needle string) *searchPlan {
	if entry := recentSinglePlan.Load(); entry != nil && entry.needle == needle {
		return entry.plan
	}

	slot := &singlePlanCache[singlePlanCacheIndex(needle)]
	if entry := slot.Load(); entry != nil && entry.needle == needle {
		recentSinglePlan.Store(entry)
		return entry.plan
	}

	entry := &singlePlanCacheEntry{needle: needle, plan: newSearchPlan([]string{needle})}
	slot.Store(entry)
	recentSinglePlan.Store(entry)
	return entry.plan
}

func singlePlanCacheIndex(needle string) int {
	// Sample three positions, so lookup remains a fixed-cost operation on the
	// short inputs where compilation would otherwise dominate IndexFold.
	h := uint(len(needle)) * 0x9e3779b1
	if len(needle) != 0 {
		h ^= uint(needle[0]) * 0x85ebca6b
		h ^= uint(needle[len(needle)/2]) * 0xc2b2ae35
		h ^= uint(needle[len(needle)-1]) * 0x27d4eb2f
	}
	return int(h & (singlePlanCacheSlots - 1))
}

func newSearchPlan(patterns []string) *searchPlan {
	p := &searchPlan{
		patternCount: len(patterns),
		runes:        make(map[rune]uint32),
		empty:        -1,
		nodes:        []planNode{{output: planOutput{pattern: -1}}},
	}

	var nextToken uint32 = 1
	for patternID, pattern := range patterns {
		if pattern == "" {
			if p.empty < 0 || patternID < p.empty {
				p.empty = patternID
			}
			continue
		}

		state, units, maxBytes := 0, 0, 0
		for at := 0; at < len(pattern); {
			token, size := p.patternToken(pattern, at, &nextToken)
			r, _ := utf8.DecodeRuneInString(pattern[at:])
			unitBytes := size
			if r != utf8.RuneError || size != 1 {
				unitBytes = maxFoldRuneWidth(r)
			}
			maxBytes += unitBytes
			next, ok := p.nodes[state].edges[token]
			if !ok {
				next = len(p.nodes)
				p.nodes = append(p.nodes, planNode{output: planOutput{pattern: -1}})
				if p.nodes[state].edges == nil {
					p.nodes[state].edges = make(map[uint32]int)
				}
				p.nodes[state].edges[token] = next
			}
			state = next
			if len(patterns) == 1 {
				p.singleTokens = append(p.singleTokens, token)
			}
			at += size
			units++
		}

		terminal := planOutput{pattern: patternID, units: units}
		if preferOutput(terminal, p.nodes[state].output) {
			p.nodes[state].output = terminal
		}
		if units > p.maxUnits {
			p.maxUnits = units
		}
		if maxBytes > p.maxBytes {
			p.maxBytes = maxBytes
		}
	}

	p.finish(nextToken)
	if len(patterns) > 1 {
		p.makeASCIIPairAnchors(patterns)
	}
	if len(patterns) == 1 {
		p.makeASCIIAnchor(patterns[0])
		p.makeUnicodeAnchor(patterns[0])
	}
	p.makeRawByteTokenPlan(patterns)
	return p
}

// maxFoldRuneWidth returns the widest UTF-8 encoding in r's simple-fold orbit.
func maxFoldRuneWidth(r rune) int {
	maxWidth := 1
	for member := r; ; member = unicode.SimpleFold(member) {
		if width := utf8.RuneLen(member); width > maxWidth {
			maxWidth = width
		}
		if unicode.SimpleFold(member) == r {
			break
		}
	}
	return maxWidth
}

// patternToken emits one token for a pattern unit and advances at by its
// source width. A malformed pattern byte is an opaque unit, just as it is in a
// haystack scan.
func (p *searchPlan) patternToken(s string, at int, nextToken *uint32) (uint32, int) {
	r, size := utf8.DecodeRuneInString(s[at:])
	if r == utf8.RuneError && size == 1 {
		byteValue := s[at]
		token := p.opaque[byteValue]
		if token == 0 {
			token = *nextToken
			*nextToken++
			p.opaque[byteValue] = token
		}
		return token, size
	}
	return p.runeToken(r, nextToken), size
}

func (p *searchPlan) runeToken(r rune, nextToken *uint32) uint32 {
	if token := p.runes[r]; token != 0 {
		return token
	}

	token := *nextToken
	*nextToken++
	for member := r; ; member = unicode.SimpleFold(member) {
		p.runes[member] = token
		next := unicode.SimpleFold(member)
		if next == r {
			break
		}
	}
	return token
}

func preferOutput(candidate, current planOutput) bool {
	return candidate.pattern >= 0 && (current.pattern < 0 ||
		candidate.units > current.units ||
		candidate.units == current.units && candidate.pattern < current.pattern)
}

func (p *searchPlan) makeASCIIAnchor(pattern string) {
	p.makeASCIIOnlyProbe(pattern)
	if len(pattern) < 3 {
		return
	}
	fixedPrefix := 0
	for fixedPrefix < len(pattern) {
		if pattern[fixedPrefix] >= utf8.RuneSelf {
			break
		}
		fixed := true
		for member := rune(pattern[fixedPrefix]); ; member = unicode.SimpleFold(member) {
			if member >= utf8.RuneSelf {
				fixed = false
				break
			}
			if unicode.SimpleFold(member) == rune(pattern[fixedPrefix]) {
				break
			}
		}
		if !fixed {
			break
		}
		fixedPrefix++
	}
	if fixedPrefix < 3 {
		return
	}

	first := pattern[0]
	if isASCIILetter(first) {
		first |= 0x20
	}
	same := fixedPrefix == len(pattern)
	for i := 1; same && i < len(pattern); i++ {
		value := pattern[i]
		if isASCIILetter(value) {
			value |= 0x20
		}
		if value != first {
			same = false
		}
	}
	if same {
		p.asciiRun, p.asciiRunByte = true, first
		if isASCIILetter(first) {
			p.asciiRunKind = rootASCIIFold
		} else {
			p.asciiRunKind = rootExact
		}
		p.asciiNeedle = pattern
		return
	}

	// Retain the unique literal positions as optional one-byte block probes.
	// Find samples the actual haystack before choosing one; absent candidates
	// are much cheaper than the robust three-position transition.
	var counts [utf8.RuneSelf]uint8
	for i := 0; i < fixedPrefix; i++ {
		value := pattern[i]
		if isASCIILetter(value) {
			value |= 0x20
		}
		counts[value]++
	}
	p.asciiNeedle = pattern
	p.asciiVerifyTokens = fixedPrefix != len(pattern)
	p.asciiFixedPrefix = fixedPrefix
	p.makeStaticASCIIByteAnchor(pattern, fixedPrefix)
	if fixedPrefix == len(pattern) && len(pattern) >= 8 {
		for at := 0; at < 8; at++ {
			value := pattern[at]
			if isASCIILetter(value) {
				value |= 0x20
				p.asciiFirstFold |= uint64(0x20) << (8 * at)
			}
			p.asciiFirstWord |= uint64(value) << (8 * at)
		}
		for at := 8; at < len(pattern) && at < 16; at++ {
			value := pattern[at]
			shift := 8 * (at - 8)
			if isASCIILetter(value) {
				value |= 0x20
				p.asciiTailFold |= uint64(0x20) << shift
			}
			p.asciiTailWord |= uint64(value) << shift
			p.asciiTailMask |= uint64(0xff) << shift
		}
	}
	for i := 0; i < fixedPrefix; i++ {
		value := pattern[i]
		if isASCIILetter(value) {
			value |= 0x20
		}
		// A dynamic one-byte scan pays a fixed sampling pass. Restrict it to
		// rank-zero bytes, which are rare enough to plausibly eliminate a full
		// block; rank-one prose letters stay on the constant-cost three-probe
		// transition, especially on hits and moderate-size misses.
		if counts[value] == 1 && asciiRarity(value) == 0 {
			p.asciiByteAnchor = true
			break
		}
	}

	probe := &p.asciiProbe
	probe.firstAt, probe.secondAt, probe.thirdAt = 0, fixedPrefix/2, fixedPrefix-1
	values := [3]*byte{&probe.first, &probe.second, &probe.third}
	offsets := [3]int{probe.firstAt, probe.secondAt, probe.thirdAt}
	for i, at := range offsets {
		value := pattern[at]
		if isASCIILetter(value) {
			value |= 0x20
			probe.fold |= 1 << i
		}
		*values[i] = value
	}
	makeASCIIVBMIProbe(probe)
	if fixedPrefix == len(pattern) {
		probe.short = makeASCIIShortLiteral(pattern)
	}
	p.asciiNeedle = pattern
	p.asciiVerifyTokens = fixedPrefix != len(pattern)

	// Long fixed-width literals can use a lighter two-position transition. A
	// probe at byte eight lets the AVX-512 implementation reuse the following
	// block through VALIGNQ; the full literal remains the authority after a
	// survivor. Keep shorter literals on the three-probe path, where setup and
	// candidate confirmation dominate.
	if fixedPrefix == len(pattern) && 9 <= fixedPrefix && fixedPrefix <= 15 {
		pair := &p.asciiPair
		pair.secondAt = 8
		pair.first, pair.second = pattern[0], pattern[pair.secondAt]
		if isASCIILetter(pair.first) {
			pair.first |= 0x20
			pair.firstFold = 0x20
		}
		if isASCIILetter(pair.second) {
			pair.second |= 0x20
			pair.secondFold = 0x20
		}
		makeASCIIPairVBMIProbe(pair, pattern)
	}
}

// makeStaticASCIIByteAnchor avoids sampling the haystack for the narrow
// repeated-byte shape. The lone normalized byte is a fixed rare anchor, while
// asciiAnchorMatches still confirms the complete literal through this plan.
func (p *searchPlan) makeStaticASCIIByteAnchor(pattern string, fixedPrefix int) {
	const staticASCIIAnchorMin = 16
	if fixedPrefix != len(pattern) || fixedPrefix < staticASCIIAnchorMin {
		return
	}

	var counts [utf8.RuneSelf]int
	for i := 0; i < fixedPrefix; i++ {
		value := pattern[i]
		if isASCIILetter(value) {
			value |= 0x20
		}
		counts[value]++
	}

	values, singleton, repeated := 0, byte(0), byte(0)
	for value, count := range counts {
		if count == 0 {
			continue
		}
		values++
		switch count {
		case 1:
			singleton = byte(value)
		case fixedPrefix - 1:
			repeated = byte(value)
		}
	}
	if values != 2 || counts[singleton] != 1 || counts[repeated] != fixedPrefix-1 || asciiRarity(singleton) >= 2 {
		return
	}
	for at := 0; at < fixedPrefix; at++ {
		value := pattern[at]
		if isASCIILetter(value) {
			value |= 0x20
		}
		if value != singleton {
			continue
		}
		p.asciiStaticAnchor, p.asciiStaticAt, p.asciiStaticByte = true, at, singleton
		if isASCIILetter(singleton) {
			p.asciiStaticKind = rootASCIIFold
		} else {
			p.asciiStaticKind = rootExact
		}
		return
	}
}

func (p *searchPlan) makeASCIIOnlyProbe(pattern string) {
	if len(pattern) < 3 {
		return
	}
	for i := range pattern {
		if pattern[i] >= utf8.RuneSelf {
			return
		}
		// A non-space punctuation byte makes an ASCII-only long path useful
		// for structured input (for example source syntax). Letter-only text
		// needles stay short-only: a sparse Unicode fold rendering would make
		// an optimistic ASCII pass duplicate a long Unicode scan.
		if !isASCIILetter(pattern[i]) && pattern[i] != ' ' {
			p.asciiOnlyLong = true
		}
	}
	probe := &p.asciiOnlyProbe
	probe.firstAt, probe.secondAt, probe.thirdAt = 0, len(pattern)/2, len(pattern)-1
	if p.asciiOnlyLong {
		// Structured literals usually carry decisive punctuation at their
		// boundaries. Reusing the final position engages the two-position
		// ASCII-only vector transition; full confirmation remains unchanged.
		probe.secondAt = probe.thirdAt
	}
	values := [3]*byte{&probe.first, &probe.second, &probe.third}
	offsets := [3]int{probe.firstAt, probe.secondAt, probe.thirdAt}
	for i, at := range offsets {
		value := pattern[at]
		if isASCIILetter(value) {
			value |= 0x20
			probe.fold |= 1 << i
		}
		*values[i] = value
	}
	if len(pattern) >= 8 {
		for at := 0; at < 8; at++ {
			value := pattern[at]
			if isASCIILetter(value) {
				value |= 0x20
				p.asciiOnlyFold |= uint64(0x20) << (8 * at)
			}
			p.asciiOnlyWord |= uint64(value) << (8 * at)
		}
	}
	p.singlePayload = pattern
	p.asciiOnly = true
}

func asciiRarity(value byte) int {
	switch value {
	case 'q', 'x', 'z', 'j':
		return 0
	case 'b', 'f', 'k', 'v', 'w', 'y':
		return 1
	default:
		return 2
	}
}

// asciiFoldSpelling returns the byte-5-normalized spelling used by the
// conservative all-ASCII pair filter. A pattern unit without an ASCII orbit
// member cannot occur on the specialized stream and deliberately contributes no
// anchor. Normalizing every byte, rather than just letters, permits the Shufti
// projection to over-admit punctuation aliases; the shared plan rejects them.
func asciiFoldSpelling(pattern string) (string, bool) {
	spelling := make([]byte, 0, len(pattern))
	for at := 0; at < len(pattern); {
		r, size := utf8.DecodeRuneInString(pattern[at:])
		if r == utf8.RuneError && size == 1 {
			return "", false
		}
		var value byte
		found := false
		for member := r; ; member = unicode.SimpleFold(member) {
			if 0 <= member && member < utf8.RuneSelf {
				value = byte(member) | 0x20
				found = true
				break
			}
			if unicode.SimpleFold(member) == r {
				break
			}
		}
		if !found {
			return "", false
		}
		spelling = append(spelling, value)
		at += size
	}
	return string(spelling), true
}

// asciiPairRarity adds a compact common-digraph penalty to the byte ranks.
// It is a compile-time selection policy, so a Matcher never samples its
// haystack before choosing an interior anchor. The penalty only picks a pair;
// the conservative table and shared plan retain the actual semantics.
func asciiPairRarity(first, second byte) int {
	score := asciiRarity(first) + asciiRarity(second)
	switch uint16(first)<<8 | uint16(second) {
	case 't'<<8 | 'h', 'h'<<8 | 'e', 'i'<<8 | 'n', 'e'<<8 | 'r', 'a'<<8 | 'n',
		'r'<<8 | 'e', 'o'<<8 | 'n', 'a'<<8 | 't', 'e'<<8 | 'n', 'n'<<8 | 'd',
		't'<<8 | 'i', 'e'<<8 | 's', 'o'<<8 | 'r', 't'<<8 | 'e', 'o'<<8 | 'f',
		'e'<<8 | 'd', 'i'<<8 | 's', 'i'<<8 | 't', 'a'<<8 | 'l', 'a'<<8 | 'r',
		's'<<8 | 't', 't'<<8 | 'o', 'n'<<8 | 't', 'n'<<8 | 'g', 's'<<8 | 'e',
		'h'<<8 | 'a', 'a'<<8 | 's', 'o'<<8 | 'u', 'i'<<8 | 'o', 'l'<<8 | 'e',
		'v'<<8 | 'e', 'c'<<8 | 'o', 'm'<<8 | 'e', 'd'<<8 | 'e', 'h'<<8 | 'i',
		'r'<<8 | 'i', 'r'<<8 | 'o', 'i'<<8 | 'c', 'n'<<8 | 'e', 'e'<<8 | 'a',
		'l'<<8 | 'i', 'c'<<8 | 'h', 'l'<<8 | 'l', 'b'<<8 | 'e', 'm'<<8 | 'a',
		's'<<8 | 'i', 'o'<<8 | 'm', 'u'<<8 | 'r', 'w'<<8 | 'a', 'd'<<8 | 'o',
		'k'<<8 | 'e':
		return score + 4
	}
	return score
}

// chooseASCIIPairAnchor uses static byte and digraph ranks. Ties prefer the
// most interior pair, so roots that share a prefix do not force every
// candidate through the same early byte position.
func chooseASCIIPairAnchor(spelling string) int {
	bestAt, bestScore, bestInterior := 0, int(^uint(0)>>1), -1
	for at := 0; at+1 < len(spelling); at++ {
		score := asciiPairRarity(spelling[at], spelling[at+1])
		interior := at
		if tail := len(spelling) - 2 - at; tail < interior {
			interior = tail
		}
		if score < bestScore || score == bestScore && interior > bestInterior {
			bestAt, bestScore, bestInterior = at, score, interior
		}
	}
	return bestAt
}

// makeASCIIPairAnchors replaces the expensive three-byte table only for its
// partial-root, all-ASCII shape. Every pattern that can match on that stream
// contributes one pair; a missing or one-byte spelling leaves the existing
// triple path in charge rather than risking a skipped match.
func (p *searchPlan) makeASCIIPairAnchors(patterns []string) {
	if p.patternCount < 2 || p.rootKind != rootGeneric || p.triplesComplete ||
		!p.asciiTriplesComplete || !p.asciiTriples.shufti.usable() {
		return
	}

	var anchors asciiPairAnchors
	for patternID, pattern := range patterns {
		if pattern == "" {
			continue
		}
		spelling, ok := asciiFoldSpelling(pattern)
		if !ok {
			continue
		}
		if len(spelling) < 2 || anchors.n == asciiPairAnchorSlots {
			return
		}
		at := chooseASCIIPairAnchor(spelling)
		if at > 255 {
			return
		}
		anchor := asciiPairAnchor{
			first: spelling[at], second: spelling[at+1], at: byte(at), pattern: patternID,
		}
		anchors.anchors[anchors.n] = anchor
		anchors.n++
		if anchor.at > anchors.maxAt {
			anchors.maxAt = anchor.at
		}
	}
	if anchors.n == 0 {
		return
	}
	anchors.filter = makeASCIIPairAnchorFilter(&anchors)
	p.asciiPairAnchors = anchors
}

func patternRawForms(pattern string) (forms [][]string, widths []int) {
	for at := 0; at < len(pattern); {
		r, size := utf8.DecodeRuneInString(pattern[at:])
		if r == utf8.RuneError && size == 1 {
			forms = append(forms, []string{pattern[at : at+1]})
			widths = append(widths, 1)
			at++
			continue
		}
		unit := make([]string, 0, 4)
		for member := r; ; member = unicode.SimpleFold(member) {
			var encoded [utf8.UTFMax]byte
			n := utf8.EncodeRune(encoded[:], member)
			form := string(encoded[:n])
			duplicate := false
			for _, prior := range unit {
				if prior == form {
					duplicate = true
					break
				}
			}
			if !duplicate {
				unit = append(unit, form)
			}
			if unicode.SimpleFold(member) == r {
				break
			}
		}
		forms = append(forms, unit)
		widths = append(widths, size)
		at += size
	}
	return forms, widths
}

func tripleFromForms(forms [][]string, start int) (tripleFilter, bool) {
	var filter tripleFilter
	var expand func(int, [3]byte, int) bool
	expand = func(unit int, prefix [3]byte, length int) bool {
		if unit == len(forms) {
			return false
		}
		for _, form := range forms[unit] {
			bytes, nextLength := prefix, length
			for i := range len(form) {
				if nextLength == len(bytes) {
					break
				}
				bytes[nextLength] = form[i]
				nextLength++
			}
			if nextLength == len(bytes) {
				if !filter.add(bytes[0], bytes[1], bytes[2], 0) {
					return false
				}
				continue
			}
			if !expand(unit+1, bytes, nextLength) {
				return false
			}
		}
		return true
	}
	return filter, expand(start, [3]byte{}, 0)
}

// makeUnicodeAnchor records a complete, raw-byte rendering set for an interior
// three-byte window. It is only used when every rendering before that window
// has the same width, so a filtered byte position still maps to one source
// start. Exact confirmation below consumes this plan's tokens, not a second
// matcher.
func (p *searchPlan) makeUnicodePairAnchor(pattern string) {
	forms, widths := patternRawForms(pattern)
	offset, fixedPrefix := 0, true
	for unit := range forms {
		if fixedPrefix && int(p.unicodePairN) < len(p.unicodePairs) {
			var filter rootFilter
			for _, form := range forms[unit] {
				if len(form) != 2 || !filter.addPair(form[0], form[1]) {
					filter = rootFilter{}
					break
				}
			}
			if filter.pairN != 0 && filter.pairN <= 2 {
				p.unicodePairs[p.unicodePairN] = unicodePairAnchor{filter: filter, at: offset}
				p.unicodePairN++
			}
		}
		width := len(forms[unit][0])
		for _, form := range forms[unit][1:] {
			if len(form) != width {
				fixedPrefix = false
				break
			}
		}
		offset += widths[unit]
	}
	if p.unicodePairN >= 2 {
		for i := range p.unicodePairN {
			anchor := &p.unicodePairs[i]
			partner := (int(i) + 1) % int(p.unicodePairN)
			anchor.confirm = p.unicodePairs[partner].filter
			anchor.confirmAt = p.unicodePairs[partner].at
			delta := anchor.confirmAt - anchor.at
			if delta <= 0 || delta > 255 || anchor.filter.pairN == 0 || anchor.confirm.pairN == 0 {
				continue
			}
			first := anchor.filter.pairs[0]
			second := anchor.filter.pairs[0]
			if anchor.filter.pairN > 1 {
				second = anchor.filter.pairs[1]
			}
			confirmFirst := anchor.confirm.pairs[0]
			confirmSecond := anchor.confirm.pairs[0]
			if anchor.confirm.pairN > 1 {
				confirmSecond = anchor.confirm.pairs[1]
			}
			anchor.pairPair = pairPairFilter{
				first0: first.first, second0: first.second, first1: second.first, second1: second.second,
				confirmFirst0: confirmFirst.first, confirmSecond0: confirmFirst.second,
				confirmFirst1: confirmSecond.first, confirmSecond1: confirmSecond.second,
				offset: byte(delta), valid: 1,
			}
			makePairPairVBMIFilter(&anchor.pairPair)
		}
	}
}

func (p *searchPlan) makeUnicodeAnchor(pattern string) {
	p.makeUnicodePairAnchor(pattern)
	if !p.asciiOnly && p.unicodePairN != 0 && p.unicodePairs[0].pairPair.valid != 0 {
		anchor := p.unicodePairs[0]
		if confirm := makeUnicodePairConfirm(pattern, anchor.at, anchor.confirmAt); confirm.valid() {
			p.singlePayload = string(confirm)
		} else if confirm := makeUnicodePairVariableConfirm(pattern, anchor.at); confirm.valid() {
			p.singlePayload = string(confirm)
		}
	}
	forms, widths := patternRawForms(pattern)
	offset, fixedPrefix := 0, true
	var best tripleFilter
	bestAt, bestN := 0, len(tripleFilter{}.values)+1
	for unit := range forms {
		if fixedPrefix {
			if candidate, ok := tripleFromForms(forms, unit); ok && candidate.n != 0 &&
				(int(candidate.n) < bestN || int(candidate.n) == bestN && offset > bestAt) {
				best, bestAt, bestN = candidate, offset, int(candidate.n)
			}
		}
		width := len(forms[unit][0])
		for _, form := range forms[unit][1:] {
			if len(form) != width {
				fixedPrefix = false
				break
			}
		}
		offset += widths[unit]
	}
	if best.n != 0 {
		p.unicodeAnchor, p.unicodeAt = best, bestAt
	}
}

// finish computes failure links, propagates the leftmost-relevant terminal
// through them, and optionally materializes complete transitions for the
// compact token alphabet.
func (p *searchPlan) finish(nextToken uint32) {
	for r, token := range p.runes {
		if r >= 0 && r < utf8.RuneSelf {
			p.ascii[byte(r)] = token
		}
	}
	p.stride = int(nextToken)
	// A root transition on a byte outside this set is a no-op. Mark non-ASCII
	// bytes as stop markers too, so the ASCII block path hands their first byte
	// to the UTF-8 decoder rather than stepping over it.
	for byteValue := utf8.RuneSelf; byteValue < len(p.rootByte); byteValue++ {
		p.rootByte[byteValue] = 1
		if byteValue < 0xc0 && p.opaque[byteValue] != 0 {
			// A raw byte prefilter cannot distinguish this opaque continuation
			// from the interior of a valid UTF-8 rune. Keep such plans on the
			// decoded transition below.
			p.opaqueContinuation = true
		}
	}
	var rootValues [2]byte
	rootCount := 0
	for byteValue, token := range p.ascii {
		if token != 0 {
			if _, ok := p.nodes[0].edges[token]; ok {
				p.rootByte[byteValue] = 1
				if rootCount < len(rootValues) {
					rootValues[rootCount] = byte(byteValue)
				}
				rootCount++
			}
		}
	}
	if rootCount == 1 {
		p.rootKind, p.rootNeedle = rootExact, rootValues[0]
	} else if rootCount == 2 && rootValues[0]|0x20 == rootValues[1]|0x20 &&
		isASCIILetter(rootValues[0]) && isASCIILetter(rootValues[1]) {
		p.rootKind, p.rootNeedle = rootASCIIFold, rootValues[0]|0x20
	}
	if p.rootKind != rootGeneric {
		firstToken := p.ascii[rootValues[0]]
		firstState := p.nodes[0].edges[firstToken]
		if p.nodes[firstState].output.pattern < 0 && len(p.nodes[firstState].edges) == 1 {
			for token := range p.nodes[firstState].edges {
				p.pairKind, p.pairNeedle = p.asciiTokenKind(token)
			}
		}
	}
	p.triples, p.tripleRoots, p.triplesComplete = p.makeTripleFilter()
	if p.triplesComplete {
		arrangeTripleMixed(&p.triples)
		arrangeTripleASCIIUTF8(&p.triples)
		arrangeTripleSharedPrefix(&p.triples)
		p.triples.shufti = makeTripleShuftiFilter(p.triples)
		p.filter = p.makeRootFilter(p.tripleRoots)
	} else {
		// A partial triple set cannot screen a root that it does not cover on a
		// general UTF-8 stream. It can still screen an all-ASCII stream if it
		// covers every root which has an ASCII rendering. Retain that bounded
		// transition separately; the regular filter remains the authority for
		// mixed input and every unsupported plan shape.
		p.asciiTriples = p.triples
		p.asciiTriplesComplete = p.asciiTripleRootsComplete(p.tripleRoots)
		if p.asciiTriplesComplete {
			p.asciiTriples.shufti = makeTripleShuftiFilter(p.asciiTriples)
		}
		p.triples = tripleFilter{}
		p.tripleRoots = nil
		p.filter = p.makeRootFilter(nil)
	}
	p.pairSecond = makePairSecond(&p.filter)
	p.filter.shufti = makePairShuftiFilter(p.filter)

	queue := make([]int, 0, len(p.nodes))
	for _, child := range p.nodes[0].edges {
		queue = append(queue, child)
	}
	for head := 0; head < len(queue); head++ {
		state := queue[head]
		for token, child := range p.nodes[state].edges {
			failure := p.nodes[state].failure
			for failure != 0 {
				if next, ok := p.nodes[failure].edges[token]; ok {
					failure = next
					goto linked
				}
				failure = p.nodes[failure].failure
			}
			if next, ok := p.nodes[0].edges[token]; ok && next != child {
				failure = next
			}
		linked:
			p.nodes[child].failure = failure
			if preferOutput(p.nodes[failure].output, p.nodes[child].output) {
				p.nodes[child].output = p.nodes[failure].output
			}
			queue = append(queue, child)
		}
	}

	if p.stride <= 1 || len(p.nodes) > maxDensePlanTransitions/p.stride {
		return
	}
	p.dense = make([]uint32, len(p.nodes)*p.stride)
	root := p.dense[:p.stride]
	for token, child := range p.nodes[0].edges {
		root[token] = uint32(child)
	}
	for _, state := range queue {
		row := p.dense[state*p.stride : (state+1)*p.stride]
		failure := p.nodes[state].failure
		copy(row, p.dense[failure*p.stride:(failure+1)*p.stride])
		for token, child := range p.nodes[state].edges {
			row[token] = uint32(child)
		}
	}
}

// asciiTripleRootsComplete reports whether triples cover every root which an
// all-ASCII haystack can reach. Roots that have only multi-byte renderings are
// impossible on that restricted stream and do not need a byte-prefix stop.
func (p *searchPlan) asciiTripleRootsComplete(roots []bool) bool {
	if !p.asciiTriples.usable() {
		return false
	}
	for _, token := range p.ascii {
		if token == 0 {
			continue
		}
		if _, isRoot := p.nodes[0].edges[token]; isRoot &&
			(int(token) >= len(roots) || !roots[token]) {
			return false
		}
	}
	return true
}

func isASCIILetter(b byte) bool {
	return 'A' <= b && b <= 'Z' || 'a' <= b && b <= 'z'
}

func (p *searchPlan) makeTripleFilter() (tripleFilter, []bool, bool) {
	type tripleForm struct {
		bytes string
		fold  bool
	}
	rawForms := make([][]string, p.stride)
	var encoded [utf8.UTFMax]byte
	for r, token := range p.runes {
		size := utf8.EncodeRune(encoded[:], r)
		rawForms[token] = append(rawForms[token], string(encoded[:size]))
	}
	for value, token := range p.opaque {
		if token != 0 {
			rawForms[token] = append(rawForms[token], string([]byte{byte(value)}))
		}
	}
	forms := make([][]tripleForm, p.stride)
	for token, raw := range rawForms {
		var folded [utf8.RuneSelf]bool
		for _, form := range raw {
			if len(form) == 1 && isASCIILetter(form[0]) {
				lower := form[0] | 0x20
				if !folded[lower] {
					forms[token] = append(forms[token], tripleForm{bytes: string([]byte{lower}), fold: true})
					folded[lower] = true
				}
				continue
			}
			duplicate := false
			for _, prior := range forms[token] {
				if !prior.fold && prior.bytes == form {
					duplicate = true
					break
				}
			}
			if !duplicate {
				forms[token] = append(forms[token], tripleForm{bytes: form})
			}
		}
	}

	isRoot := func(token uint32) bool {
		_, ok := p.nodes[0].edges[token]
		return token != 0 && ok
	}
	unsafeRoot := make([]bool, p.stride)
	for value, token := range p.opaque {
		if isRoot(token) && 0x80 <= value && value < 0xc0 {
			unsafeRoot[token] = true
		}
	}

	var filter tripleFilter
	roots := make([]bool, p.stride)
	for token := uint32(1); int(token) < p.stride; token++ {
		state, ok := p.nodes[0].edges[token]
		if !ok || unsafeRoot[token] {
			continue
		}
		hasASCIIRoot := false
		for _, form := range forms[token] {
			if len(form.bytes) == 1 && isASCIILetter(form.bytes[0]) {
				hasASCIIRoot = true
				break
			}
		}
		if !hasASCIIRoot {
			continue
		}
		candidate := filter
		var expand func(int, uint32, [3]byte, int, uint8) bool
		expand = func(state int, current uint32, prefix [3]byte, length int, folds uint8) bool {
			if len(forms[current]) == 0 {
				return false
			}
			for _, form := range forms[current] {
				bytes, nextLength, nextFolds := prefix, length, folds
				for index := range len(form.bytes) {
					if nextLength == len(bytes) {
						break
					}
					bytes[nextLength] = form.bytes[index]
					if form.fold && index == 0 {
						nextFolds |= 1 << nextLength
					}
					nextLength++
				}
				if nextLength == len(bytes) {
					if !candidate.add(bytes[0], bytes[1], bytes[2], nextFolds) {
						return false
					}
					continue
				}
				if p.nodes[state].output.pattern >= 0 || len(p.nodes[state].edges) == 0 {
					return false
				}
				for next, child := range p.nodes[state].edges {
					if !expand(child, next, bytes, nextLength, nextFolds) {
						return false
					}
				}
			}
			return true
		}
		if expand(state, token, [3]byte{}, 0, 0) {
			filter = candidate
			roots[token] = true
		}
	}
	complete := true
	for token := range p.nodes[0].edges {
		if !roots[token] {
			complete = false
			break
		}
	}
	return filter, roots, complete
}

func (p *searchPlan) makeRootFilter(excludeRoots []bool) rootFilter {
	forms := make([][]string, p.stride)
	var encoded [utf8.UTFMax]byte
	for r, token := range p.runes {
		size := utf8.EncodeRune(encoded[:], r)
		forms[token] = append(forms[token], string(encoded[:size]))
	}
	for value, token := range p.opaque {
		if token != 0 {
			forms[token] = append(forms[token], string([]byte{byte(value)}))
		}
	}

	isRoot := func(token uint32) bool {
		_, ok := p.nodes[0].edges[token]
		return token != 0 && ok
	}
	// A continuation byte can occur inside a valid UTF-8 rune. The byte
	// prefilter cannot distinguish that position from an opaque byte, so retain
	// the regular decoder for plans that need one at their root.
	for value, token := range p.opaque {
		if isRoot(token) && 0x80 <= value && value < 0xc0 {
			return rootFilter{}
		}
	}

	addFirstPrefix := func(filter *rootFilter, form string) bool {
		if len(form) == 1 {
			return filter.addOne(form[0])
		}
		return filter.addPair(form[0], form[1])
	}
	type pairForm struct {
		bytes string
		fold  bool
	}
	compactPairForms := func(forms []string) []pairForm {
		out := make([]pairForm, 0, len(forms))
		var folded [utf8.RuneSelf]bool
		for _, form := range forms {
			if len(form) == 1 && isASCIILetter(form[0]) {
				lower := form[0] | 0x20
				if !folded[lower] {
					out = append(out, pairForm{bytes: string([]byte{lower}), fold: true})
					folded[lower] = true
				}
				continue
			}
			duplicate := false
			for _, prior := range out {
				if !prior.fold && prior.bytes == form {
					duplicate = true
					break
				}
			}
			if !duplicate {
				out = append(out, pairForm{bytes: form})
			}
		}
		return out
	}
	addRequiredNext := func(filter *rootFilter, first, second []string) bool {
		candidate := *filter
		for _, firstForm := range compactPairForms(first) {
			if len(firstForm.bytes) > 1 {
				if !candidate.addPair(firstForm.bytes[0], firstForm.bytes[1]) {
					return false
				}
				continue
			}
			for _, secondForm := range compactPairForms(second) {
				if len(secondForm.bytes) == 0 {
					return false
				}
				fold := uint8(0)
				if firstForm.fold {
					fold |= rootPairFoldFirst
				}
				if secondForm.fold {
					fold |= rootPairFoldSecond
				}
				if !candidate.addFoldPair(firstForm.bytes[0], secondForm.bytes[0], fold) {
					return false
				}
			}
		}
		*filter = candidate
		return true
	}

	var filter rootFilter
	for token := uint32(1); int(token) < len(forms); token++ {
		if !isRoot(token) {
			continue
		}
		rootForms := forms[token]
		if int(token) < len(excludeRoots) && excludeRoots[token] {
			continue
		}
		if len(rootForms) == 0 {
			continue
		}
		child := p.nodes[0].edges[token]
		if p.nodes[child].output.pattern < 0 && len(p.nodes[child].edges) == 1 {
			for next := range p.nodes[child].edges {
				if addRequiredNext(&filter, rootForms, forms[next]) {
					goto added
				}
			}
		}
		for _, form := range rootForms {
			if !addFirstPrefix(&filter, form) {
				return rootFilter{}
			}
		}
	added:
	}
	return filter
}

func (p *searchPlan) asciiTokenKind(token uint32) (uint8, byte) {
	var values [2]byte
	count := 0
	for byteValue, candidate := range p.ascii {
		if candidate == token {
			if count < len(values) {
				values[count] = byte(byteValue)
			}
			count++
		}
	}
	if count == 1 {
		return rootExact, values[0]
	}
	if count == 2 && values[0]|0x20 == values[1]|0x20 &&
		isASCIILetter(values[0]) && isASCIILetter(values[1]) {
		return rootASCIIFold, values[0] | 0x20
	}
	return rootGeneric, 0
}

func (p *searchPlan) haystackToken(s string, at int) (uint32, int) {
	byteValue := s[at]
	if byteValue < utf8.RuneSelf {
		return p.ascii[byteValue], 1
	}
	r, size := utf8.DecodeRuneInString(s[at:])
	if r == utf8.RuneError && size == 1 {
		// An eligible raw-byte plan has no opaque pattern bytes. Its opaque
		// storage holds direct two-byte tokens instead, so malformed input must
		// remain the ordinary zero-token reset.
		if p.hasRawByteTokenPlan() {
			return 0, 1
		}
		return p.opaque[byteValue], 1
	}
	return p.runes[r], size
}

func (p *searchPlan) advance(state int, token uint32) int {
	if token == 0 {
		return 0
	}
	if p.dense != nil {
		return int(p.dense[state*p.stride+int(token)])
	}
	for {
		if next, ok := p.nodes[state].edges[token]; ok {
			return next
		}
		if state == 0 {
			return 0
		}
		state = p.nodes[state].failure
	}
}

func (p *searchPlan) findASCIIRun(haystack string) (Match, bool) {
	if start := findASCIIRunBytes(haystack, p.asciiRunKind, p.asciiRunByte, len(p.asciiNeedle)); start >= 0 {
		return Match{Pattern: 0, Start: start}, true
	}
	return Match{}, false
}

func (p *searchPlan) asciiAnchorMatches(haystack string, at int) bool {
	if p.asciiVerifyTokens {
		return p.matchesSingleAt(haystack, at)
	}
	if literal := p.asciiProbe.short; literal.valid != 0 {
		return at+4 <= len(haystack) && asciiShortLiteralAt(haystack, at, literal)
	}
	return at+len(p.asciiNeedle) <= len(haystack) && asciiPatternAt(haystack, at, p.asciiNeedle)
}

func (p *searchPlan) chooseASCIIByteAnchor(haystack string) (int, uint8, byte, bool) {
	// A lone-byte scan wins only when an actual haystack sample has no copies
	// of one unique literal. Otherwise the three-position transition avoids
	// paying full confirmation at every common byte.
	if len(haystack) < 4096 || p.asciiFixedPrefix > 64 {
		return 0, 0, 0, false
	}
	var raw, folded [utf8.RuneSelf]uint16
	const sampleWidth = 64
	sampleBlocks := 4
	if len(haystack) >= 256<<10 {
		sampleBlocks = 16
	}
	for block := 0; block < sampleBlocks; block++ {
		start := block * (len(haystack) - sampleWidth) / (sampleBlocks - 1)
		for i := start; i < start+sampleWidth; i++ {
			value := haystack[i]
			if value < utf8.RuneSelf {
				raw[value]++
				folded[value|0x20]++
			}
		}
	}
	var patternCounts [utf8.RuneSelf]uint8
	for i := 0; i < p.asciiFixedPrefix; i++ {
		value := p.asciiNeedle[i]
		if isASCIILetter(value) {
			value |= 0x20
		}
		patternCounts[value]++
	}
	bestAt, bestCount := -1, uint16(sampleWidth*sampleBlocks+1)
	for i := 0; i < p.asciiFixedPrefix; i++ {
		value := p.asciiNeedle[i]
		count := raw[value]
		if isASCIILetter(value) {
			value |= 0x20
			count = folded[value]
		}
		if patternCounts[value] == 1 && count < bestCount {
			bestAt, bestCount = i, count
		}
	}
	if bestAt < 0 || bestCount != 0 {
		return 0, 0, 0, false
	}
	needle := p.asciiNeedle[bestAt]
	if isASCIILetter(needle) {
		needle |= 0x20
		return bestAt, rootASCIIFold, needle, true
	}
	return bestAt, rootExact, needle, true
}

func (p *searchPlan) findASCIIByteAnchor(haystack string, anchorAt int, kind uint8, needle byte) (Match, bool) {
	for at := 0; at < len(haystack); {
		at += literalSkipASCII(haystack, at, kind, needle)
		if at == len(haystack) {
			break
		}
		start := at - anchorAt
		if start >= 0 {
			// The vector root is intentionally a single byte. Before a full
			// confirmation, cheaply reject its false hits with the compiled
			// dispersed literal probe. Width-changing fold tokens keep their
			// token-level confirmation unchanged.
			if !p.asciiVerifyTokens && start+len(p.asciiNeedle) <= len(haystack) && !asciiProbeAt(haystack, start, &p.asciiProbe) {
				at++
				continue
			}
			if p.asciiAnchorMatches(haystack, start) {
				return Match{Pattern: 0, Start: start}, true
			}
		}
		at++
	}
	return Match{}, false
}

func (p *searchPlan) asciiPairPromising() bool {
	return asciiRarity(p.asciiPair.first) < 2 || asciiRarity(p.asciiPair.second) < 2
}

func (p *searchPlan) asciiPairVBMIDisplaced() bool {
	return p.asciiPair.vbmi.valid != 0 && int(p.asciiPair.vbmi.secondAt) != p.asciiPair.secondAt
}

func (p *searchPlan) asciiPairSparse(haystack string) bool {
	limit := len(haystack) - len(p.asciiNeedle) + 1
	if limit <= 0 {
		return true
	}
	if limit > 256 {
		limit = 256
	}
	for at := 0; at < limit; at++ {
		if asciiPairAt(haystack, at, &p.asciiPair) {
			return false
		}
	}
	return true
}

func (p *searchPlan) findASCIIPairAnchor(haystack string) (Match, bool) {
	if len(haystack) >= len(p.asciiNeedle) && p.asciiFixedAt(haystack, 0) {
		return Match{Pattern: 0}, true
	}
	limit := len(haystack) - len(p.asciiNeedle) + 1
	for at := 0; at < limit; {
		at += asciiPairSkipBytes(haystack, at, limit-at, &p.asciiPair)
		if at == limit {
			break
		}
		if asciiPatternAt(haystack, at, p.asciiNeedle) {
			return Match{Pattern: 0, Start: at}, true
		}
		at++
	}
	return Match{}, false
}

func (p *searchPlan) findASCIIAnchor(haystack string) (Match, bool) {
	needle := p.asciiNeedle
	limit := len(haystack) - len(needle) + 1
	if p.asciiVerifyTokens {
		// Width-changing simple folds may make a matching haystack shorter or
		// longer than the original spelling. The probe itself is fixed-width;
		// its final byte is the only bound needed before token confirmation.
		limit = len(haystack) - p.asciiProbe.thirdAt
	}
	for at := 0; at < limit; {
		at += probeSkipBytes(haystack, at, limit-at, &p.asciiProbe)
		if at == limit {
			break
		}
		matched := p.asciiAnchorMatches(haystack, at)
		if matched {
			return Match{Pattern: 0, Start: at}, true
		}
		at++
	}
	return Match{}, false
}

// findASCIIOnlyAnchorWithHigh runs the existing ASCII candidate transition
// and reports the first high byte when no ASCII match was found. Keeping the
// stop position lets the partitioned executor hand only the suffix around that
// byte to the decoded owner instead of rescanning the whole input to discover
// the same boundary.
func (p *searchPlan) findASCIIOnlyAnchorWithHigh(haystack, needle string) (Match, bool, int) {
	limit := len(haystack) - len(needle) + 1
	if limit < 0 {
		return Match{}, false, -1
	}
	for at := 0; at < limit; {
		skipped, ascii := asciiOnlyProbeSkipBytes(haystack, at, limit-at, &p.asciiOnlyProbe)
		at += skipped
		if !ascii {
			return Match{}, false, at
		}
		if at == limit {
			break
		}
		if p.asciiOnlyPatternAt(haystack, at, needle) {
			return Match{Pattern: 0, Start: at}, true, -1
		}
		at++
	}
	for offset, value := range haystack[limit:] {
		if value >= utf8.RuneSelf {
			return Match{}, false, limit + offset
		}
	}
	return Match{}, false, -1
}

func (p *searchPlan) findASCIIOnlyAnchor(haystack, needle string) (Match, bool, bool) {
	match, ok, firstHigh := p.findASCIIOnlyAnchorWithHigh(haystack, needle)
	return match, ok, firstHigh < 0
}

func (p *searchPlan) findASCIIOnlyRegion(haystack string, start, end int) (Match, bool) {
	if start >= end {
		return Match{}, false
	}
	match, ok, handled := p.findASCIIOnlyAnchor(haystack[start:end], p.singlePayload)
	if !handled {
		// The coordinator only supplies a known-ASCII region. Keep the decoded
		// owner as a defensive fallback if a future boundary change violates
		// that precondition.
		match, ok = p.findUnfilteredWithStarts(haystack[start:end], nil)
	}
	if ok {
		match.Start += start
	}
	return match, ok
}

func (p *searchPlan) asciiOnlyPatternAt(haystack string, at int, needle string) bool {
	if len(needle) >= 8 && asciiFixedPrefix8(haystack, at, p.asciiOnlyWord, p.asciiOnlyFold) {
		return asciiPatternAt(haystack, at+8, needle[8:])
	}
	return asciiPatternAt(haystack, at, needle)
}

func (p *searchPlan) asciiFixedAt(haystack string, at int) bool {
	needle := p.asciiNeedle
	start := 0
	if len(needle) >= 8 {
		if !asciiFixedPrefix8(haystack, at, p.asciiFirstWord, p.asciiFirstFold) {
			return false
		}
		start = 8
	}
	if p.asciiTailMask != 0 && at+16 <= len(haystack) {
		if !asciiFixedPrefix8Masked(haystack, at+8, p.asciiTailWord, p.asciiTailFold, p.asciiTailMask) {
			return false
		}
		start = 16
	}
	for i := start; i < len(needle); i++ {
		got, want := haystack[at+i], needle[i]
		if isASCIILetter(want) {
			if got|0x20 != want|0x20 {
				return false
			}
		} else if got != want {
			return false
		}
	}
	return true
}

func asciiPatternAt(haystack string, at int, pattern string) bool {
	for i := range pattern {
		got, want := haystack[at+i], pattern[i]
		if isASCIILetter(want) {
			if got|0x20 != want|0x20 {
				return false
			}
		} else if got != want {
			return false
		}
	}
	return true
}

func (p *searchPlan) matchesSingleAt(haystack string, at int) bool {
	for _, want := range p.singleTokens {
		if at == len(haystack) {
			return false
		}
		got, size := p.haystackToken(haystack, at)
		if got != want {
			return false
		}
		at += size
	}
	return true
}

func (p *searchPlan) chooseUnicodePairAnchor(haystack string) *unicodePairAnchor {
	// Two fixed-width pair positions give an exact primary transition and a
	// dispersed confirmation without sampling or per-call setup.
	if len(haystack) < 4096 || p.unicodePairN < 2 {
		return nil
	}
	return &p.unicodePairs[0]
}

func pairFilterAt(haystack string, at int, filter *rootFilter) bool {
	if at < 0 || at+1 >= len(haystack) {
		return false
	}
	for i := range filter.pairN {
		pair := filter.pairs[i]
		if haystack[at] == pair.first && haystack[at+1] == pair.second {
			return true
		}
	}
	return false
}

// findUnicodePairConfirm keeps the exact N=1 raw confirmation in the VBMI
// transition for full vector blocks. The scalar tail remains bounded and uses
// the same compiled raw forms, while unavailable vector hosts retain the
// decoded executor below.
func (p *searchPlan) unicodePairConfirm() unicodePairConfirm {
	if p.asciiOnly {
		return ""
	}
	return unicodePairConfirm(p.singlePayload)
}

func (p *searchPlan) findUnicodePairConfirm(haystack string, anchor *unicodePairAnchor) (Match, int, bool) {
	confirm := p.unicodePairConfirm()
	if len(haystack) < confirm.minLength() {
		return Match{}, 0, false
	}

	at := anchor.at
	if lastStart := len(haystack) - confirm.maxLength(); lastStart >= 0 {
		full := (lastStart + 1) &^ 63
		if full != 0 {
			skipped, width := pairPairConfirmBytes(haystack, at, full, &anchor.pairPair, confirm)
			if skipped < full {
				return Match{Pattern: 0, Start: at + skipped - anchor.at}, width, true
			}
			at += full
		}
	}

	lastAnchor := len(haystack) - confirm.minLength() + anchor.at
	for at <= lastAnchor {
		at += pairPairSkipBytes(haystack, at, &anchor.pairPair)
		if at > lastAnchor {
			break
		}
		start := at - anchor.at
		if width, ok := confirm.matchWidthAt(haystack, start); ok {
			return Match{Pattern: 0, Start: start}, width, true
		}
		at++
	}
	return Match{}, 0, false
}

func (p *searchPlan) findUnicodePairAnchor(haystack string, anchor *unicodePairAnchor) (Match, int, bool) {
	if anchor.pairPair.valid != 0 {
		if p.unicodePairConfirm().valid() && unicodePairConfirmVectorEnabled() {
			return p.findUnicodePairConfirm(haystack, anchor)
		}
		for at := 0; at+int(anchor.pairPair.offset)+1 < len(haystack); {
			at += pairPairSkipBytes(haystack, at, &anchor.pairPair)
			if at+int(anchor.pairPair.offset)+1 >= len(haystack) {
				break
			}
			start := at - anchor.at
			if start >= 0 && p.matchesSingleAt(haystack, start) {
				return Match{Pattern: 0, Start: start}, 0, true
			}
			at++
		}
		return Match{}, 0, false
	}
	for at := 0; at+1 < len(haystack); {
		at += filterSkipBytes(haystack, at, &anchor.filter)
		if at+1 >= len(haystack) {
			break
		}
		start := at - anchor.at
		if start >= 0 && pairFilterAt(haystack, start+anchor.confirmAt, &anchor.confirm) && p.matchesSingleAt(haystack, start) {
			return Match{Pattern: 0, Start: start}, 0, true
		}
		at++
	}
	return Match{}, 0, false
}

func (p *searchPlan) findUnicodeAnchor(haystack string) (Match, bool) {
	for at := 0; at+2 < len(haystack); {
		at += tripleSkipBytes(haystack, at, &p.unicodeAnchor)
		if at+2 >= len(haystack) {
			break
		}
		start := at - p.unicodeAt
		if start >= 0 && p.matchesSingleAt(haystack, start) {
			return Match{Pattern: 0, Start: start}, true
		}
		at++
	}
	return Match{}, false
}

func withZeroWidth(match Match, ok bool) (Match, int, bool) {
	return match, 0, ok
}

func (p *searchPlan) find(haystack string) (Match, bool) {
	match, _, ok := p.findWithWidth(haystack)
	return match, ok
}

// findWithWidth is the package's one search decision tree. Most routes return
// zero width; an exact raw confirmation returns the source width it has already
// proved so Matcher.Each does not decode the same match again.
func (p *searchPlan) findWithWidth(haystack string) (Match, int, bool) {
	if p.maxUnits == 0 {
		if p.empty >= 0 {
			return Match{Pattern: p.empty}, 0, true
		}
		return Match{}, 0, false
	}
	if p.opaqueContinuation {
		return withZeroWidth(p.findUnfiltered(haystack))
	}
	if p.asciiRun {
		return withZeroWidth(p.findASCIIRun(haystack))
	}
	if p.asciiPair.usable() && len(haystack) >= len(p.asciiNeedle) && p.asciiFixedAt(haystack, 0) {
		return Match{Pattern: 0}, 0, true
	}
	if p.asciiPairVBMIDisplaced() && len(haystack) >= 4096 && asciiPairVBMIEnabled() {
		return withZeroWidth(p.findASCIIPairAnchor(haystack))
	}
	if !p.asciiPairVBMIDisplaced() && p.asciiPair.usable() && p.asciiPairPromising() {
		return withZeroWidth(p.findASCIIPairAnchor(haystack))
	}
	if p.asciiStaticAnchor && len(haystack) >= 4096 {
		return withZeroWidth(p.findASCIIByteAnchor(haystack, p.asciiStaticAt, p.asciiStaticKind, p.asciiStaticByte))
	}
	if p.asciiByteAnchor {
		if anchorAt, kind, needle, ok := p.chooseASCIIByteAnchor(haystack); ok {
			return withZeroWidth(p.findASCIIByteAnchor(haystack, anchorAt, kind, needle))
		}
	}
	if !p.asciiPairVBMIDisplaced() && p.asciiPair.usable() && len(haystack) >= 4096 && p.asciiPairSparse(haystack) {
		return withZeroWidth(p.findASCIIPairAnchor(haystack))
	}
	if p.asciiProbe.usable() {
		return withZeroWidth(p.findASCIIAnchor(haystack))
	}
	// k and s have width-changing Unicode orbit members, so their patterns do
	// not enter the fixed ASCII probe above. Long N=1 ASCII literals use the
	// same probe on clean gaps and decode only source-width halos around sparse
	// exceptional spans; dense or unproven input remains on the full plan.
	partitionRejected := false
	if p.asciiOnlyPartitionUsable() && len(haystack) > 4096 {
		if match, ok, handled := p.findASCIIOnlyPartitioned(haystack); handled {
			return match, 0, ok
		}
		partitionRejected = true
	}
	// The integrated high-byte check lets a short or feature-limited structured
	// ASCII haystack use the same vector transition in one pass; any high byte
	// falls through to the full Unicode plan unchanged.
	if !partitionRejected && p.asciiOnly && (len(haystack) <= 4096 || p.asciiOnlyLong) {
		if match, ok, handled := p.findASCIIOnlyAnchor(haystack, p.singlePayload); handled {
			return match, 0, ok
		}
	}
	// The tagged multi-anchor filter is one plan-owned raw transition scan for
	// both Find and Each. Its exact replay decides the result, so Find can stop
	// at the first completed leftmost candidate without selecting a second engine.
	if p.rawByteMulti.usable() {
		if len(haystack) >= 4096 && p.rawByteOrigin.usable() {
			return withZeroWidth(p.findRawByteOrigin(haystack))
		}
		return withZeroWidth(p.findRawByteFixedAnchored(haystack))
	}
	if anchor := p.chooseUnicodePairAnchor(haystack); anchor != nil {
		return p.findUnicodePairAnchor(haystack, anchor)
	}
	if p.unicodeAnchor.n == 1 {
		return withZeroWidth(p.findUnicodeAnchor(haystack))
	}
	// A partial root triple set cannot skip a general UTF-8 stream because a
	// non-ASCII-only root may occur later. On AVX-512 BW, the high-byte scan
	// first proves that the complete input is ASCII and NUL-free, leaving only
	// the separately covered ASCII roots for the bounded Shufti transition. It
	// still advances this one compiled plan at every survivor; it is a block
	// transition, not a second matcher.
	if p.asciiPairAnchors.usable() && runtimeVectorBits() == 512 && len(haystack) >= asciiTripleMinBytes &&
		rootSkipASCII(haystack, 0, rootExact, 0) == len(haystack) {
		return withZeroWidth(p.findASCIIPairAnchored(haystack))
	}
	if p.patternCount > 1 && p.rootKind == rootGeneric && p.asciiTriplesComplete && p.asciiTriples.shufti.usable() &&
		runtimeVectorBits() == 512 && len(haystack) >= asciiTripleMinBytes &&
		rootSkipASCII(haystack, 0, rootExact, 0) == len(haystack) {
		return withZeroWidth(p.findASCIITripleFiltered(haystack))
	}
	if p.triplesComplete && p.triples.usable() && (p.patternCount == 1 || p.rootKind == rootGeneric) {
		return withZeroWidth(p.findFiltered(haystack))
	}
	if p.rootKind == rootGeneric && p.filter.usable() {
		return withZeroWidth(p.findFiltered(haystack))
	}

	return withZeroWidth(p.findUnfiltered(haystack))
}

// findWithWidthAfterASCIIOnly enters the post-ASCII decision tree when a
// rejected or malformed partition resumes from a safe cursor. It mirrors the
// tail of findWithWidth so ordinary searches keep their current inlined
// dispatch while the rare fallback retains every Unicode candidate route.
func (p *searchPlan) findWithWidthAfterASCIIOnly(haystack string) (Match, int, bool) {
	// The tagged multi-anchor filter is one plan-owned raw transition scan for
	// both Find and Each. Its exact replay decides the result, so Find can stop
	// at the first completed leftmost candidate without selecting a second engine.
	if p.rawByteMulti.usable() {
		if len(haystack) >= 4096 && p.rawByteOrigin.usable() {
			return withZeroWidth(p.findRawByteOrigin(haystack))
		}
		return withZeroWidth(p.findRawByteFixedAnchored(haystack))
	}
	if anchor := p.chooseUnicodePairAnchor(haystack); anchor != nil {
		return p.findUnicodePairAnchor(haystack, anchor)
	}
	if p.unicodeAnchor.n == 1 {
		return withZeroWidth(p.findUnicodeAnchor(haystack))
	}
	// A partial root triple set cannot skip a general UTF-8 stream because a
	// non-ASCII-only root may occur later. On AVX-512 BW, the high-byte scan
	// first proves that the complete input is ASCII and NUL-free, leaving only
	// the separately covered ASCII roots for the bounded Shufti transition. It
	// still advances this one compiled plan at every survivor; it is a block
	// transition, not a second matcher.
	if p.asciiPairAnchors.usable() && runtimeVectorBits() == 512 && len(haystack) >= asciiTripleMinBytes &&
		rootSkipASCII(haystack, 0, rootExact, 0) == len(haystack) {
		return withZeroWidth(p.findASCIIPairAnchored(haystack))
	}
	if p.patternCount > 1 && p.rootKind == rootGeneric && p.asciiTriplesComplete && p.asciiTriples.shufti.usable() &&
		runtimeVectorBits() == 512 && len(haystack) >= asciiTripleMinBytes &&
		rootSkipASCII(haystack, 0, rootExact, 0) == len(haystack) {
		return withZeroWidth(p.findASCIITripleFiltered(haystack))
	}
	if p.triplesComplete && p.triples.usable() && (p.patternCount == 1 || p.rootKind == rootGeneric) {
		return withZeroWidth(p.findFiltered(haystack))
	}
	if p.rootKind == rootGeneric && p.filter.usable() {
		return withZeroWidth(p.findFiltered(haystack))
	}

	return withZeroWidth(p.findUnfiltered(haystack))
}

// asciiPartitionWindowWorthwhile keeps a widened window from dominating short
// haystacks with unusually wide compiled patterns.
func asciiPartitionWindowWorthwhile(length, maxBytes int) bool {
	// A widened window spends up to maxBytes on either side of an exceptional
	// span. Keep at least three quarters of a sufficiently long haystack
	// available for ASCII-run work; short inputs and very wide compiled patterns
	// otherwise turn the partition into extra work.
	return length >= 2*asciiPartitionSampleBytes && maxBytes > 0 && maxBytes <= length/8
}

// findUnfiltered advances the decoded plan without raw byte filters. It is the
// boundary-safe path for plans containing opaque UTF-8 continuation bytes.
func (p *searchPlan) findUnfiltered(haystack string) (Match, bool) {
	if p.asciiPartitionUsable() && asciiPartitionWindowWorthwhile(len(haystack), p.maxBytes) && !asciiPartitionTailBoundary(haystack) {
		firstHigh := rootSkipASCII(haystack, 0, rootExact, 0)
		if firstHigh < len(haystack) && asciiPartitionSparseEnough(haystack, firstHigh) {
			return p.findPartitionedASCII(haystack, firstHigh)
		}
	}
	return p.findUnfilteredDecodedLegacy(haystack)
}

// asciiPartitionStats is an optional diagnostic view of the partition route.
// It is populated only by focused package benchmarks; normal searches pass nil
// and retain the same hot path without counters.
type asciiPartitionStats struct {
	highBytes           int
	firstExceptional    int
	asciiCandidateBytes int
	decodedWindowBytes  int
	decodedWindows      int
	fallbackEntries     int
}

func (p *searchPlan) findUnfilteredWithStats(haystack string, stats *asciiPartitionStats) (Match, bool) {
	if stats != nil {
		stats.firstExceptional = -1
	}
	if p.asciiPartitionUsable() && asciiPartitionWindowWorthwhile(len(haystack), p.maxBytes) && !asciiPartitionTailBoundary(haystack) {
		firstHigh := rootSkipASCII(haystack, 0, rootExact, 0)
		if firstHigh < len(haystack) && asciiPartitionSparseEnough(haystack, firstHigh) {
			if stats != nil {
				stats.firstExceptional = firstHigh
				return p.findPartitionedASCIIWithStats(haystack, firstHigh, stats)
			}
			return p.findPartitionedASCII(haystack, firstHigh)
		}
	}
	if stats != nil {
		stats.fallbackEntries++
		for at := 0; at < len(haystack); at++ {
			if haystack[at] >= utf8.RuneSelf {
				stats.highBytes++
				if stats.firstExceptional < 0 {
					stats.firstExceptional = at
				}
			}
		}
	}
	return p.findUnfilteredDecodedLegacy(haystack)
}

// asciiPartitionEarlyBoundary rejects an exceptional byte in the first
// sample before the all-or-nothing candidate probe pays for the rest of the
// haystack. An early exception leaves too little clean prefix for partitioning.
func asciiPartitionEarlyBoundary(haystack string) bool {
	end := len(haystack)
	if end > asciiPartitionSampleBytes {
		end = asciiPartitionSampleBytes
	}
	return rootSkipASCII(haystack[:end], 0, rootExact, 0) < end
}

// asciiPartitionTailBoundary keeps the old decoded executor for dense input
// and for a final boundary with no block-sized run after it. Earlier runs are
// still eligible when the tail is clean, while the check itself is bounded by
// the existing ASCII block threshold.
func asciiPartitionTailBoundary(haystack string) bool {
	if len(haystack) < asciiTripleMinBytes {
		return true
	}
	start := len(haystack) - asciiTripleMinBytes
	for at := start; at < len(haystack); at++ {
		if haystack[at] >= utf8.RuneSelf || haystack[at] == 0 {
			return true
		}
	}
	return false
}

// asciiPartitionSparseEnough rejects exceptional-byte densities for which the
// boundary work and repeated decoded transitions cost more than the legacy
// executor. The samples are deliberately capped and spread through the input:
// sparse inputs retain the vector ASCII gaps, while a later dense region is
// rejected before the first partitioned window is spent on it.
func asciiPartitionSparseEnough(haystack string, firstExceptional int) bool {
	sparseSample := func(start, end int) bool {
		high := 0
		for at := start; at < end; {
			at += rootSkipASCII(haystack[:end], at, rootExact, 0)
			if at >= end {
				break
			}
			if haystack[at] == 0 {
				return false
			}
			high++
			if high >= asciiPartitionMaxHigh {
				return false
			}
			at++
		}
		return true
	}

	end := firstExceptional + asciiPartitionSampleBytes
	if end > len(haystack) {
		end = len(haystack)
	}
	if !sparseSample(firstExceptional, end) {
		return false
	}
	if len(haystack)-firstExceptional <= asciiPartitionSampleBytes {
		return true
	}

	// A bounded set of fixed-fraction samples catches a dense suffix or middle
	// without turning admission into a second full haystack scan. Keep samples
	// disjoint from the first window and clamp them at the input boundary.
	for _, start := range [...]int{len(haystack) / 4, len(haystack) / 2, len(haystack) - asciiPartitionSampleBytes} {
		if start < end {
			continue
		}
		if !sparseSample(start, start+asciiPartitionSampleBytes) {
			return false
		}
	}
	return true
}

// findUnfilteredDecodedLegacy retains the original decoded prefix and
// fallback shape for plans that cannot profit from ASCII-run re-entry. Keeping
// this owner separate prevents the partition state and its boundary checks
// from changing ordinary fallback work.
func (p *searchPlan) findUnfilteredDecodedLegacy(haystack string) (Match, bool) {
	state, unit := 0, 0
	bestUnitStart := -1
	best := Match{Pattern: -1, Start: -1}
	if p.empty >= 0 {
		// An empty pattern is a candidate at start zero, not an immediate
		// answer: a lower-index non-empty pattern may also start at zero.
		bestUnitStart = 0
		best = Match{Pattern: p.empty, Start: 0}
	}

	// Common text stays in this no-allocation path. Because each preceding
	// unit is one byte, a terminal's source start is direct arithmetic rather
	// than a lookup in the variable-width offset ring used after the first
	// non-ASCII byte.
	at := 0
	for at < len(haystack) && haystack[at] < utf8.RuneSelf {
		// A root-to-root block cannot emit a non-empty terminal. Advance over
		// sixteen such bytes at once; every other block remains on the exact
		// same transition path below.
		if state == 0 {
			if p.rootKind != rootGeneric {
				for at < len(haystack) {
					skipped := rootSkipASCII(haystack, at, p.rootKind, p.rootNeedle)
					if p.pairKind != rootGeneric {
						skipped = pairSkipASCII(haystack, at, p.rootKind, p.rootNeedle, p.pairKind, p.pairNeedle)
					}
					if skipped == 0 {
						break
					}
					if bestUnitStart >= 0 && unit+skipped-1 >= bestUnitStart+p.maxUnits-1 {
						return best, true
					}
					at += skipped
					unit += skipped
				}
			} else {
				for at+16 <= len(haystack) {
					block := haystack[at : at+16]
					if p.rootByte[block[0]]|p.rootByte[block[1]]|p.rootByte[block[2]]|p.rootByte[block[3]]|
						p.rootByte[block[4]]|p.rootByte[block[5]]|p.rootByte[block[6]]|p.rootByte[block[7]]|
						p.rootByte[block[8]]|p.rootByte[block[9]]|p.rootByte[block[10]]|p.rootByte[block[11]]|
						p.rootByte[block[12]]|p.rootByte[block[13]]|p.rootByte[block[14]]|p.rootByte[block[15]] != 0 {
						break
					}
					if bestUnitStart >= 0 && unit+15 >= bestUnitStart+p.maxUnits-1 {
						return best, true
					}
					at += len(block)
					unit += len(block)
				}
			}
			if at == len(haystack) || haystack[at] >= utf8.RuneSelf {
				break
			}
		}
		state = p.advance(state, p.ascii[haystack[at]])
		if output := p.nodes[state].output; output.pattern >= 0 {
			startUnit := unit - output.units + 1
			if bestUnitStart < 0 || startUnit < bestUnitStart ||
				startUnit == bestUnitStart && output.pattern < best.Pattern {
				bestUnitStart = startUnit
				best = Match{Pattern: output.pattern, Start: startUnit}
			}
		}
		if bestUnitStart >= 0 && unit >= bestUnitStart+p.maxUnits-1 {
			return best, true
		}
		at++
		unit++
	}
	if at == len(haystack) {
		if bestUnitStart < 0 {
			return Match{}, false
		}
		return best, true
	}

	var inlineStarts [256]int
	starts := inlineStarts[:]
	if p.maxUnits > len(starts) {
		starts = make([]int, p.maxUnits)
	}
	return p.findNonASCIIDecoded(haystack, at, unit, state, bestUnitStart, best, starts)
}

// findUnfilteredWithStarts is the decoded executor used for a bounded window.
// A caller can provide its offset ring so each window reuses the same stack
// storage rather than allocating a new ring.
func (p *searchPlan) findUnfilteredWithStarts(haystack string, starts []int) (Match, bool) {
	state, unit := 0, 0
	bestUnitStart := -1
	best := Match{Pattern: -1, Start: -1}
	if p.empty >= 0 {
		// An empty pattern is a candidate at start zero, not an immediate
		// answer: a lower-index non-empty pattern may also start at zero.
		bestUnitStart = 0
		best = Match{Pattern: p.empty, Start: 0}
	}

	// Common text stays in this no-allocation path. Because each preceding
	// unit is one byte, a terminal's source start is direct arithmetic rather
	// than a lookup in the variable-width offset ring used after the first
	// non-ASCII byte.
	at := 0
	for at < len(haystack) && haystack[at] < utf8.RuneSelf {
		// A root-to-root block cannot emit a non-empty terminal. Advance over
		// sixteen such bytes at once; every other block remains on the exact
		// same transition path below.
		if state == 0 {
			if p.rootKind != rootGeneric {
				for at < len(haystack) {
					skipped := rootSkipASCII(haystack, at, p.rootKind, p.rootNeedle)
					if p.pairKind != rootGeneric {
						skipped = pairSkipASCII(haystack, at, p.rootKind, p.rootNeedle, p.pairKind, p.pairNeedle)
					}
					if skipped == 0 {
						break
					}
					if bestUnitStart >= 0 && unit+skipped-1 >= bestUnitStart+p.maxUnits-1 {
						return best, true
					}
					at += skipped
					unit += skipped
				}
			} else {
				for at+16 <= len(haystack) {
					block := haystack[at : at+16]
					if p.rootByte[block[0]]|p.rootByte[block[1]]|p.rootByte[block[2]]|p.rootByte[block[3]]|
						p.rootByte[block[4]]|p.rootByte[block[5]]|p.rootByte[block[6]]|p.rootByte[block[7]]|
						p.rootByte[block[8]]|p.rootByte[block[9]]|p.rootByte[block[10]]|p.rootByte[block[11]]|
						p.rootByte[block[12]]|p.rootByte[block[13]]|p.rootByte[block[14]]|p.rootByte[block[15]] != 0 {
						break
					}
					if bestUnitStart >= 0 && unit+15 >= bestUnitStart+p.maxUnits-1 {
						return best, true
					}
					at += len(block)
					unit += len(block)
				}
			}
			if at == len(haystack) || haystack[at] >= utf8.RuneSelf {
				break
			}
		}
		state = p.advance(state, p.ascii[haystack[at]])
		if output := p.nodes[state].output; output.pattern >= 0 {
			startUnit := unit - output.units + 1
			if bestUnitStart < 0 || startUnit < bestUnitStart ||
				startUnit == bestUnitStart && output.pattern < best.Pattern {
				bestUnitStart = startUnit
				best = Match{Pattern: output.pattern, Start: startUnit}
			}
		}
		if bestUnitStart >= 0 && unit >= bestUnitStart+p.maxUnits-1 {
			return best, true
		}
		at++
		unit++
	}
	if at == len(haystack) {
		if bestUnitStart < 0 {
			return Match{}, false
		}
		return best, true
	}
	if starts == nil {
		var inlineStarts [256]int
		starts = inlineStarts[:]
		if p.maxUnits > len(starts) {
			starts = make([]int, p.maxUnits)
		}
	}
	return p.findNonASCII(haystack, at, unit, state, bestUnitStart, best, starts)
}

// findASCIITripleFiltered is the all-ASCII specialization of the shared plan.
// Its caller proves that every byte is a non-NUL ASCII unit and that
// asciiTriples covers every root that can occur on such input. A triple stop is
// only a conservative candidate; the regular decoded transition decides every
// match, leftmost start, and pattern-ID tie.
func (p *searchPlan) findASCIITripleFiltered(haystack string) (Match, bool) {
	var inlineStarts [256]int
	starts := inlineStarts[:]
	if p.maxUnits > len(starts) {
		starts = make([]int, p.maxUnits)
	}

	state, history := 0, 0
	best := Match{Pattern: -1, Start: -1}
	if p.empty >= 0 {
		best = Match{Pattern: p.empty, Start: 0}
	}
	for at := 0; at < len(haystack); {
		if state == 0 {
			if best.Pattern >= 0 && at > best.Start {
				return best, true
			}
			history = 0
			skipped := tripleSkipBytes(haystack, at, &p.asciiTriples)
			at += skipped
			if at == len(haystack) {
				break
			}
		}

		starts[history%len(starts)] = at
		token, size := p.haystackToken(haystack, at)
		state = p.advance(state, token)
		history++
		if output := p.nodes[state].output; output.pattern >= 0 {
			start := starts[(history-output.units)%len(starts)]
			if best.Pattern < 0 || start < best.Start ||
				start == best.Start && output.pattern < best.Pattern {
				best = Match{Pattern: output.pattern, Start: start}
			}
		}
		at += size
	}
	if best.Pattern < 0 {
		return Match{}, false
	}
	return best, true
}

// tripleSkipASCIIRegion scans candidate starts in [start,end). The byte scout
// may inspect the two following bytes needed by overlapping triple loads, but
// the returned skip is clamped to the known-ASCII region. The decoded boundary
// window owns starts whose source crosses the following exceptional span.
func tripleSkipASCIIRegion(s string, start, end int, filter *tripleFilter) int {
	if start >= end {
		return 0
	}
	scanEnd := end + 2
	if scanEnd > len(s) {
		scanEnd = len(s)
	}
	skipped := tripleSkipBytes(s[start:scanEnd], 0, filter)
	if limit := end - start; skipped > limit {
		return limit
	}
	return skipped
}

// findASCIITripleFilteredRange is the bounded form used for a maximal ASCII
// run. Its vector scout may read two bytes beyond end from the original
// haystack, but it never advances a candidate beyond end; a decoded window
// handles matches whose source crosses the following exceptional span.
func (p *searchPlan) findASCIITripleFilteredRange(haystack string, start, end int) (Match, bool) {
	var inlineStarts [256]int
	starts := inlineStarts[:]
	if p.maxUnits > len(starts) {
		starts = make([]int, p.maxUnits)
	}

	state, history := 0, 0
	best := Match{Pattern: -1, Start: -1}
	for at := start; at < end; {
		if state == 0 {
			if best.Pattern >= 0 && at > best.Start {
				return best, true
			}
			history = 0
			skipped := tripleSkipASCIIRegion(haystack, at, end, &p.asciiTriples)
			at += skipped
			if at == end {
				break
			}
		}

		starts[history%len(starts)] = at
		token, size := p.haystackToken(haystack, at)
		state = p.advance(state, token)
		history++
		if output := p.nodes[state].output; output.pattern >= 0 {
			startAt := starts[(history-output.units)%len(starts)]
			if best.Pattern < 0 || startAt < best.Start ||
				startAt == best.Start && output.pattern < best.Pattern {
				best = Match{Pattern: output.pattern, Start: startAt}
			}
		}
		at += size
	}
	if best.Pattern < 0 {
		return Match{}, false
	}
	return best, true
}

// replayASCIIAnchorStart feeds a bounded candidate window back through the
// shared compiled plan. Filtering never confirms a pattern directly: the
// plan's propagated outputs select both the terminal and its lowest-ID tie.
func (p *searchPlan) replayASCIIAnchorStart(haystack string, start int) (Match, bool) {
	state := 0
	best := Match{Pattern: -1, Start: -1}
	limit := start + p.maxUnits
	if limit > len(haystack) {
		limit = len(haystack)
	}
	for at := start; at < limit; at++ {
		state = p.advance(state, p.ascii[haystack[at]])
		if output := p.nodes[state].output; output.pattern >= 0 {
			outputStart := at - output.units + 1
			if outputStart == start && (best.Pattern < 0 || output.pattern < best.Pattern) {
				best = Match{Pattern: output.pattern, Start: start}
			}
		}
	}
	if best.Pattern < 0 {
		return Match{}, false
	}
	return best, true
}

// findASCIIPairAnchored is the lower-work all-ASCII representation of the
// partial mixed-root plan. Its pair table is only a union prefilter. A matching
// pair may belong to any spelling, so every possible mapped start is replayed
// through the common decoded plan before it can affect the selected result.
func (p *searchPlan) findASCIIPairAnchored(haystack string) (Match, bool) {
	anchors := &p.asciiPairAnchors
	best := Match{Pattern: -1, Start: -1}
	if p.empty >= 0 {
		best = Match{Pattern: p.empty, Start: 0}
	}

	for at := 0; at+1 < len(haystack); {
		// A later-start match can have an earlier pair only when that pair's
		// source offset is larger. Wait through the maximum compiled offset
		// before finalizing best, rather than treating filter encounter order as
		// leftmost order.
		if best.Pattern >= 0 && at > best.Start+int(anchors.maxAt) {
			return best, true
		}
		at += asciiPairAnchorSkipBytes(haystack, at, &anchors.filter)
		if at+1 >= len(haystack) {
			break
		}
		for i := range anchors.n {
			anchor := anchors.anchors[i]
			if !asciiPairAnchorMatches(haystack, at, anchor) {
				continue
			}
			start := at - int(anchor.at)
			if start < 0 || best.Pattern >= 0 && start > best.Start {
				continue
			}
			if match, ok := p.replayASCIIAnchorStart(haystack, start); ok &&
				(best.Pattern < 0 || match.Start < best.Start ||
					match.Start == best.Start && match.Pattern < best.Pattern) {
				best = match
			}
		}
		at++
	}
	if best.Pattern < 0 {
		return Match{}, false
	}
	return best, true
}

// findFiltered is the generic-root path. It scans byte prefixes until a root
// token is possible, then lets the normal token transition decide it. Skipped
// spans contain no root and therefore cannot contribute a live state; the
// local offset ring can start anew after each such span instead of decoding
// every unrelated UTF-8 rune.
func (p *searchPlan) findFiltered(haystack string) (Match, bool) {
	var inlineStarts [256]int
	starts := inlineStarts[:]
	if p.maxUnits > len(starts) {
		starts = make([]int, p.maxUnits)
	}

	state, history := 0, 0
	best := Match{Pattern: -1, Start: -1}
	if p.empty >= 0 {
		best = Match{Pattern: p.empty, Start: 0}
	}
	raw := p.hasRawByteTokenPlan()
	for at := 0; at < len(haystack); {
		if state == 0 {
			// Once no prefix is live, a previously found start cannot be
			// displaced by future input. This also preserves the empty-pattern
			// tie rule after its first byte has been checked.
			if best.Pattern >= 0 && at > best.Start {
				return best, true
			}
			history = 0
			skipped := len(haystack) - at
			if p.pairSecond {
				skipped = pairSecondSkipBytes(haystack, at, &p.filter)
			} else if p.filter.usable() {
				skipped = filterSkipBytes(haystack, at, &p.filter)
			}
			if p.triples.usable() {
				if tripleSkipped := tripleSkipBytes(haystack, at, &p.triples); tripleSkipped < skipped {
					skipped = tripleSkipped
				}
			}
			if skipped != 0 {
				at += skipped
				if at == len(haystack) {
					break
				}
			}
		}

		starts[history%len(starts)] = at
		size := 0
		if raw {
			// Keep the raw classification in this loop so the source-byte loads
			// and dense transition stay in one generated body. Wider and malformed
			// input falls through to the complete decoded transition below.
			value := haystack[at]
			if value < utf8.RuneSelf {
				token := p.ascii[value]
				if token == 0 {
					state = 0
				} else {
					state = int(p.dense[state*p.stride+int(token)])
				}
				size = 1
			} else if value >= 0xc2 && value <= 0xdf && at+1 < len(haystack) {
				trail := haystack[at+1]
				if trail >= 0x80 && trail < 0xc0 {
					row := p.opaque[value]
					if row == 0 {
						state = 0
					} else if token := p.opaque[(int(row)-1)*rawByteTrailClasses+int(trail-0x80)]; token == 0 {
						state = 0
					} else {
						state = int(p.dense[state*p.stride+int(token)])
					}
					size = 2
				}
			}
		}
		if size == 0 {
			token, decodedSize := p.haystackToken(haystack, at)
			state, size = p.advance(state, token), decodedSize
		}
		history++
		if output := p.nodes[state].output; output.pattern >= 0 {
			start := starts[(history-output.units)%len(starts)]
			if best.Pattern < 0 || start < best.Start ||
				start == best.Start && output.pattern < best.Pattern {
				best = Match{Pattern: output.pattern, Start: start}
			}
		}
		at += size
	}
	if best.Pattern < 0 {
		return Match{}, false
	}
	return best, true
}

func (p *searchPlan) asciiPartitionUsable() bool {
	if p.empty >= 0 || p.opaqueContinuation || p.rootKind != rootGeneric || p.patternCount <= 1 ||
		!p.asciiTriplesComplete || !p.asciiTriples.shufti.usable() {
		return false
	}
	return runtimeVectorBits() == 512
}

// findASCIIRegion runs the strongest existing byte candidate transition that
// is sound on a known-ASCII region. The caller has already separated all bytes
// at or above UTF-8's RuneSelf, so the all-ASCII routes do not need to prove
// that property again.
func (p *searchPlan) findASCIIRegion(haystack string, start, end int, starts []int) (Match, bool) {
	if p.asciiOnly {
		return p.findASCIIOnlyRegion(haystack, start, end)
	}
	if end-start >= asciiTripleMinBytes {
		return p.findASCIITripleFilteredRange(haystack, start, end)
	}
	match, ok := p.findUnfilteredWithStarts(haystack[start:end], starts)
	if ok {
		match.Start += start
	}
	return match, ok
}

// asciiOnlyPartitionUsable is the long-input N=1 letter-only gate. Structured
// ASCII literals already use their established all-or-nothing probe, so adding
// partition admission there would repeat its bounded checks on every call.
// AVX-512 is required here to retain the probe's 64-byte block transition while
// the decoded owner handles exceptional windows.
func (p *searchPlan) asciiOnlyPartitionUsable() bool {
	return p.patternCount == 1 && p.asciiOnly && !p.asciiOnlyLong && p.asciiOnlyProbe.usable() && runtimeVectorBits() == 512
}

// findASCIIOnlyPartitioned admits only a long input with a clean prefix and
// tail before running the existing all-ASCII candidate probe. If that probe
// encounters a high byte, the same plan resumes with the shared partition
// coordinator, which uses the ASCII-only region owner and decoded source-width
// halos around exceptional spans. Dense or unproven input stays on the
// established decoded owner.
func (p *searchPlan) findASCIIOnlyPartitioned(haystack string) (Match, bool, bool) {
	if asciiPartitionTailBoundary(haystack) || asciiPartitionEarlyBoundary(haystack) {
		// Do not pay an all-or-nothing probe when there is no useful clean run
		// before the first exception or after the final boundary. The existing
		// Unicode candidate route already owns those shapes efficiently.
		return Match{}, false, false
	}
	match, ok, firstHigh := p.findASCIIOnlyAnchorWithHigh(haystack, p.singlePayload)
	if ok || firstHigh < 0 {
		return match, ok, true
	}
	if !asciiPartitionWindowWorthwhile(len(haystack), p.maxBytes) ||
		!asciiPartitionSparseEnough(haystack, firstHigh) {
		return Match{}, false, false
	}
	match, ok = p.findPartitionedASCII(haystack, firstHigh)
	return match, ok, true
}

// findPartitionedASCII resumes after the first high byte. It visits each
// maximal ASCII run with the existing candidate transition and decodes only a
// coalesced window around exceptional spans. The ASCII transition stops at the
// exceptional byte, so it can confirm matches that finish before that byte;
// the decoded window owns starts whose source crosses the span.
func (p *searchPlan) findPartitionedASCII(haystack string, firstHigh int) (Match, bool) {
	return p.findPartitionedASCIIWithStats(haystack, firstHigh, nil)
}

func (p *searchPlan) findPartitionedASCIIWithStats(haystack string, firstHigh int, stats *asciiPartitionStats) (Match, bool) {
	maxBytes := p.maxBytes
	if maxBytes < 1 || firstHigh >= len(haystack) {
		return Match{}, false
	}

	var inlineStarts [256]int
	starts := inlineStarts[:]
	if p.maxUnits > len(starts) {
		starts = make([]int, p.maxUnits)
	}

	var recentHigh [asciiPartitionMaxHigh]int
	recentHighCount := 0
	checkExceptional := func(start, end int, recent [asciiPartitionMaxHigh]int, count int) ([asciiPartitionMaxHigh]int, int, bool) {
		for at := start; at < end; at++ {
			if haystack[at] == 0 {
				return recent, count, false
			}
			if haystack[at] < utf8.RuneSelf {
				continue
			}
			if count >= asciiPartitionMaxHigh-1 &&
				at-recent[(count-(asciiPartitionMaxHigh-1))%asciiPartitionMaxHigh] < asciiPartitionSampleBytes {
				return recent, count, false
			}
			recent[count%asciiPartitionMaxHigh] = at
			count++
		}
		// Malformed bytes have no stable decoded window to amortize. Keep their
		// established opaque-byte executor rather than paying one transition per
		// isolated high byte. The density checks above run first so a contiguous
		// exceptional suffix is rejected without validating it in full.
		return recent, count, utf8.ValidString(haystack[start:end])
	}
	recordExceptional := func(start, end int) bool {
		var ok bool
		recentHigh, recentHighCount, ok = checkExceptional(start, end, recentHigh, recentHighCount)
		return ok
	}

	cursor := 0
	if p.asciiOnly {
		// The N=1 entry probe has already proved that no fixed ASCII match
		// starts before firstHigh. Retain only the first source-width halo: it
		// owns the starts that could cross the exceptional span and also keeps
		// the malformed-span fallback able to confirm those starts.
		cursor = firstHigh - maxBytes
		if cursor < 0 {
			cursor = 0
		}
	}
	fallback := func() (Match, bool) {
		if stats != nil {
			stats.fallbackEntries++
		}
		var match Match
		var ok bool
		if p.asciiOnly {
			match, _, ok = p.findWithWidthAfterASCIIOnly(haystack[cursor:])
		} else {
			match, ok = p.findUnfilteredDecodedLegacy(haystack[cursor:])
		}
		if ok {
			match.Start += cursor
		}
		return match, ok
	}
	spanStart := firstHigh
	pendingMatch := Match{}
	pendingOK := false
	for {
		spanEnd := spanStart
		for spanEnd < len(haystack) && (haystack[spanEnd] >= utf8.RuneSelf || haystack[spanEnd] == 0) {
			spanEnd++
		}
		if !recordExceptional(spanStart, spanEnd) {
			return fallback()
		}
		if stats != nil {
			for at := spanStart; at < spanEnd; at++ {
				if haystack[at] >= utf8.RuneSelf {
					stats.highBytes++
				}
			}
		}
		windowStart := spanStart - maxBytes
		if windowStart < 0 {
			windowStart = 0
		}
		windowEnd := spanEnd + maxBytes
		if windowEnd > len(haystack) {
			windowEnd = len(haystack)
		}
		lastHighEnd := spanEnd

		// Coalesce exceptional spans whose widened windows touch. This leaves
		// one decoded transition for a cluster rather than restarting it at
		// every high byte. The N=1 route uses its candidate probe to discover
		// the next high byte while it searches the intervening ASCII gap. The
		// same pass therefore supplies both the next boundary and any exact
		// ASCII match, instead of first walking the gap with rootSkipASCII and
		// then walking it again with the candidate transition.
		nextStart := -1
		gapSearched := false
		for scanAt := spanEnd; scanAt < len(haystack); {
			at := scanAt
			if p.asciiOnly {
				// The N=1 candidate transition also reports the first byte that
				// cannot belong to its all-ASCII region. One pass supplies both
				// the next boundary and any exact ASCII match in this gap.
				gapMatch, gapOK, high := p.findASCIIOnlyAnchorWithHigh(haystack[scanAt:], p.singlePayload)
				gapSearched = true
				if stats != nil {
					scanned := len(haystack) - scanAt
					if gapOK {
						scanned = gapMatch.Start
					} else if high >= 0 {
						scanned = high
					}
					stats.asciiCandidateBytes += scanned
				}
				if gapOK {
					gapMatch.Start += scanAt
					if !pendingOK || gapMatch.Start < pendingMatch.Start ||
						gapMatch.Start == pendingMatch.Start && gapMatch.Pattern < pendingMatch.Pattern {
						pendingMatch, pendingOK = gapMatch, true
					}
					lastHighEnd = len(haystack)
					break
				}
				if high < 0 {
					lastHighEnd = len(haystack)
					break
				}
				at += high
			} else {
				at += rootSkipASCII(haystack, at, rootExact, 0)
				if at == len(haystack) {
					break
				}
			}

			nextSpanEnd := at
			for nextSpanEnd < len(haystack) && (haystack[nextSpanEnd] >= utf8.RuneSelf || haystack[nextSpanEnd] == 0) {
				nextSpanEnd++
			}
			nextWindowStart := at - maxBytes
			if nextWindowStart < 0 {
				nextWindowStart = 0
			}
			nextWindowEnd := nextSpanEnd + maxBytes
			if nextWindowEnd > len(haystack) {
				nextWindowEnd = len(haystack)
			}
			// Probe nearby next spans before spending the current group's window. A
			// dense or malformed span must not make us partition work and then
			// restart the legacy decoder from the same safe cursor. For a distant
			// span, process this group first. The N=1 fused scan has already searched
			// the ASCII gap up to that span.
			if nextWindowStart > windowEnd && at-spanStart > asciiPartitionLookaheadBytes {
				nextStart = at
				break
			}
			probeHigh, probeCount, ok := checkExceptional(at, nextSpanEnd, recentHigh, recentHighCount)
			if !ok {
				return fallback()
			}
			if nextWindowStart > windowEnd {
				nextStart = at
				break
			}
			recentHigh, recentHighCount = probeHigh, probeCount
			if stats != nil {
				for high := at; high < nextSpanEnd; high++ {
					if haystack[high] >= utf8.RuneSelf {
						stats.highBytes++
					}
				}
			}
			if nextWindowEnd > windowEnd {
				windowEnd = nextWindowEnd
			}
			lastHighEnd = nextSpanEnd
			scanAt = nextSpanEnd
		}

		asciiMatch := pendingMatch
		asciiOK := pendingOK
		pendingOK = false
		if cursor < spanStart {
			if stats != nil {
				stats.asciiCandidateBytes += spanStart - cursor
			}
			regionMatch, regionOK := p.findASCIIRegion(haystack, cursor, spanStart, starts)
			if regionOK && (!asciiOK || regionMatch.Start < asciiMatch.Start ||
				regionMatch.Start == asciiMatch.Start && regionMatch.Pattern < asciiMatch.Pattern) {
				asciiMatch, asciiOK = regionMatch, true
			}
		}
		if asciiOK && asciiMatch.Start < windowStart {
			return asciiMatch, true
		}
		if stats != nil {
			stats.decodedWindowBytes += windowEnd - windowStart
			stats.decodedWindows++
		}
		windowMatch, windowOK := p.findUnfilteredWithStarts(haystack[windowStart:windowEnd], starts)
		if windowOK {
			windowMatch.Start += windowStart
		}
		if asciiOK && (!windowOK || asciiMatch.Start < windowMatch.Start ||
			asciiMatch.Start == windowMatch.Start && asciiMatch.Pattern < windowMatch.Pattern) {
			return asciiMatch, true
		}
		if windowOK {
			return windowMatch, true
		}

		if nextStart < 0 {
			if lastHighEnd < len(haystack) {
				if stats != nil {
					stats.asciiCandidateBytes += len(haystack) - lastHighEnd
				}
				return p.findASCIIRegion(haystack, lastHighEnd, len(haystack), starts)
			}
			return Match{}, false
		}

		// The next group starts after this group's widened window. Search the
		// ASCII gap before its decoded window; starts in that window's overlap
		// are checked by the next iteration against cross-span matches.
		nextWindowStart := nextStart - maxBytes
		if nextWindowStart < 0 {
			nextWindowStart = 0
		}
		if !gapSearched && lastHighEnd < nextStart {
			if stats != nil {
				stats.asciiCandidateBytes += nextStart - lastHighEnd
			}
			if match, ok := p.findASCIIRegion(haystack, lastHighEnd, nextStart, starts); ok {
				if match.Start < nextWindowStart {
					return match, true
				}
				pendingMatch, pendingOK = match, true
			}
		}
		cursor = nextWindowStart
		spanStart = nextStart
	}
}

// findNonASCII resumes the plan at its first non-ASCII byte. It retains the
// original decoded executor for bounded windows and short known-ASCII regions.
func (p *searchPlan) findNonASCII(haystack string, at, unit, state, bestUnitStart int, best Match, starts []int) (Match, bool) {
	return p.findNonASCIIDecoded(haystack, at, unit, state, bestUnitStart, best, starts)
}

func (p *searchPlan) findNonASCIIDecoded(haystack string, at, unit, state, bestUnitStart int, best Match, starts []int) (Match, bool) {
	first := unit - p.maxUnits + 1
	if first < 0 {
		first = 0
	}
	for prior := first; prior < unit; prior++ {
		starts[prior%len(starts)] = at - (unit - prior)
	}

	for at < len(haystack) {
		starts[unit%len(starts)] = at
		token, size := p.haystackToken(haystack, at)
		state = p.advance(state, token)
		if output := p.nodes[state].output; output.pattern >= 0 {
			startUnit := unit - output.units + 1
			start := starts[startUnit%len(starts)]
			if bestUnitStart < 0 || startUnit < bestUnitStart ||
				startUnit == bestUnitStart && output.pattern < best.Pattern {
				bestUnitStart = startUnit
				best = Match{Pattern: output.pattern, Start: start}
			}
		}
		if bestUnitStart >= 0 && unit >= bestUnitStart+p.maxUnits-1 {
			return best, true
		}
		at += size
		unit++
	}
	if bestUnitStart < 0 {
		return Match{}, false
	}
	return best, true
}

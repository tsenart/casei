//go:build amd64

package casei

import "unsafe"

func (f tripleBucketFilter) usable() bool {
	return len(f) >= tripleBucketFilterBytes && f[tripleBucketTableBytes] != 0 &&
		f[tripleBucketFilterBytes-1] == tripleBucketValidMarker
}

func (f tripleBucketFilter) prefixCount() int {
	if !f.usable() {
		return 0
	}
	return int(f[tripleBucketTableBytes])
}

func (p *searchPlan) tripleBucketFilter() tripleBucketFilter {
	if len(p.tripleRoots) < tripleBucketFilterBytes || p.tripleRoots[tripleBucketFilterBytes-1] != tripleBucketValidMarker {
		return nil
	}
	return tripleBucketFilter(p.tripleRoots)
}

// tripleBucketPrefix records one bounded ASCII path from the plan trie.
type tripleBucketPrefix struct {
	tokens [tripleBucketPrefixBytes]uint32
	length uint8
}

func (p *searchPlan) hasTripleBucketASCIIToken(token uint32) bool {
	for _, got := range p.ascii {
		if got == token {
			return true
		}
	}
	return false
}

func addTripleBucketPrefix(prefixes *[tripleShuftiSlots]tripleBucketPrefix, n *int, tokens [tripleBucketPrefixBytes]uint32, length uint8) bool {
	candidate := tripleBucketPrefix{tokens: tokens, length: length}
	for i := 0; i < *n; i++ {
		if prefixes[i] == candidate {
			return true
		}
	}
	if *n == len(prefixes) {
		return false
	}
	prefixes[*n] = candidate
	(*n)++
	return true
}

// makeTripleSharedShuftiFilter conservatively packs more than eight forms into
// the existing eight Shufti slots. Forms sharing a slot union their allowed
// bytes at each position, so the projection can nominate extra triples but
// cannot hide a stored form. The exact plan still decides every survivor.
func makeTripleSharedShuftiFilter(filter tripleFilter) tripleShuftiFilter {
	if filter.n <= tripleShuftiSlots || int(filter.n) > len(filter.values) {
		return tripleShuftiFilter{}
	}

	var ordered [len(filter.values)]rootTriple
	copy(ordered[:], filter.values[:filter.n])
	// makeTripleFilter walks maps, so raw form order is not stable between
	// Matcher compilations. Pairing sorted forms makes the conservative slot
	// groups stable as well as bounded.
	for i := 1; i < int(filter.n); i++ {
		triple := ordered[i]
		j := i
		for j > 0 && rootTripleLess(triple, ordered[j-1]) {
			ordered[j] = ordered[j-1]
			j--
		}
		ordered[j] = triple
	}

	var out tripleShuftiFilter
	add := func(lo, hi *[16]byte, value, bit byte) {
		(*lo)[value&0x0f] |= bit
		(*hi)[value>>4] |= bit
	}
	for i := 0; i < int(filter.n); i++ {
		triple := ordered[i]
		bit := byte(1 << uint(i%tripleShuftiSlots))
		values := [3]byte{triple.first, triple.second, triple.third}
		los := [3]*[16]byte{&out.firstLo, &out.secondLo, &out.thirdLo}
		his := [3]*[16]byte{&out.firstHi, &out.secondHi, &out.thirdHi}
		for position, value := range values {
			add(los[position], his[position], value, bit)
			if triple.fold&(1<<uint(position)) != 0 {
				add(los[position], his[position], value^0x20, bit)
			}
		}
	}
	out.valid = 1
	return out
}

func rootTripleLess(a, b rootTriple) bool {
	if a.first != b.first {
		return a.first < b.first
	}
	if a.second != b.second {
		return a.second < b.second
	}
	if a.third != b.third {
		return a.third < b.third
	}
	return a.fold < b.fold
}

// collectTripleBucketPrefixes derives the bounded ASCII paths which the bucket
// may nominate. It does not alter the plan; callers can reject overflow without
// installing either the bucket or its high-byte projection.
func (p *searchPlan) collectTripleBucketPrefixes() ([tripleShuftiSlots]tripleBucketPrefix, int, bool) {
	var prefixes [tripleShuftiSlots]tripleBucketPrefix
	n := 0
	var path [tripleBucketPrefixBytes]uint32
	for token0, state0 := range p.nodes[0].edges {
		if !p.hasTripleBucketASCIIToken(token0) {
			continue
		}
		path[0] = token0
		for token1, state1 := range p.nodes[state0].edges {
			if !p.hasTripleBucketASCIIToken(token1) {
				continue
			}
			path[1] = token1
			for token2, state2 := range p.nodes[state1].edges {
				if !p.hasTripleBucketASCIIToken(token2) {
					continue
				}
				path[2], path[3] = token2, 0
				node2 := &p.nodes[state2]
				if node2.output.pattern >= 0 && node2.output.units == 3 {
					if !addTripleBucketPrefix(&prefixes, &n, path, 3) {
						return prefixes, n, false
					}
				}
				for token3, state3 := range node2.edges {
					if !p.hasTripleBucketASCIIToken(token3) {
						continue
					}
					node3 := &p.nodes[state3]
					if (node3.output.pattern >= 0 && node3.output.units == 4) || len(node3.edges) != 0 {
						path[3] = token3
						if !addTripleBucketPrefix(&prefixes, &n, path, 4) {
							return prefixes, n, false
						}
					}
				}
			}
		}
	}
	return prefixes, n, true
}

// makeTripleBucketFilter compiles bounded ASCII-only trie prefixes. Each bucket
// bit names one prefix. A three-unit terminal leaves the fourth byte
// unconstrained; any live fourth-unit prefix is retained even when later units
// require Unicode. Existing exact Shufti tables handle high-byte blocks. For a
// complete six-to-eight-pattern union with more than eight raw forms, build a
// shared-slot conservative Shufti projection only after prefix collection fits.
// The bucket kernel uses its table only when all four overlapping input vectors
// are ASCII; high-byte blocks use the Shufti projection. The shared plan remains
// the only match authority.
func (p *searchPlan) makeTripleBucketFilter() bool {
	if p.patternCount <= 1 || p.rootKind != rootGeneric || p.rawByteMulti.usable() ||
		!p.triplesComplete || !asciiPairVBMIEnabled() {
		return false
	}

	sharedShufti := !p.triples.shufti.usable()
	if sharedShufti && (p.patternCount < 6 || p.patternCount > 8 || p.triples.n <= tripleShuftiSlots) {
		return false
	}
	prefixes, n, ok := p.collectTripleBucketPrefixes()
	if !ok || n == 0 {
		return false
	}

	shufti := p.triples.shufti
	if sharedShufti {
		shufti = makeTripleSharedShuftiFilter(p.triples)
		if !shufti.usable() {
			return false
		}
	}

	storage := make([]byte, tripleBucketFilterBytes)
	out := tripleBucketFilter(storage)
	for slot := 0; slot < n; slot++ {
		bit := byte(1 << uint(slot))
		for position := 0; position < int(prefixes[slot].length); position++ {
			for value, token := range p.ascii {
				if token == prefixes[slot].tokens[position] {
					out[position*128+value] |= bit
				}
			}
		}
		if prefixes[slot].length == 3 {
			for value := range 128 {
				out[3*128+value] |= bit
			}
		}
	}
	out[tripleBucketTableBytes] = byte(n)
	out[tripleBucketFilterBytes-1] = tripleBucketValidMarker
	p.triples.shufti = shufti
	p.tripleRoots = storage
	return true
}

// tripleBucketSkipBytes uses the compiled ASCII prefix buckets only for blocks
// whose four overlapping loads prove every byte ASCII. A high byte sends that
// 64-start block through the existing conservative Shufti filter; the scalar
// tail is unchanged.
func tripleBucketSkipBytes(s string, at int, bucket tripleBucketFilter, shufti *tripleShuftiFilter) int {
	if !bucket.usable() || !shufti.usable() || !asciiPairVBMIEnabled() {
		return tripleShuftiSkipBytes(s, at, shufti)
	}
	start := at
	remaining := len(s) - at
	if remaining >= 67 {
		full := ((remaining - 3) / 64) * 64
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		skipped := tripleBucketSkip64(ptr, remaining, unsafe.SliceData(bucket[:tripleBucketTableBytes]), shufti)
		at += skipped
		if skipped < full {
			return at - start
		}
	}
	return at - start + tripleShuftiSkipBytes(s, at, shufti)
}

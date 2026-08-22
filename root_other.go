//go:build !amd64

package casei

import "unicode/utf8"

func runtimeVectorBits() int { return 0 }

func asciiPairVBMIEnabled() bool { return false }

func asciiFixedPrefix8(s string, at int, word, fold uint64) bool {
	for i := 0; i < 8; i++ {
		if s[at+i]|byte(fold>>(8*i)) != byte(word>>(8*i)) {
			return false
		}
	}
	return true
}

func asciiFixedPrefix8Masked(s string, at int, word, fold, mask uint64) bool {
	for i := 0; i < 8; i++ {
		if byte(mask>>(8*i)) != 0 && s[at+i]|byte(fold>>(8*i)) != byte(word>>(8*i)) {
			return false
		}
	}
	return true
}

// rootSkipASCII is the portable representation of the amd64 block probe. It
// returns the root-to-root prefix and leaves the stop byte for the scalar
// UTF-8-aware transition.
func rootSkipASCII(s string, at int, kind uint8, needle byte) int {
	start := at
	for at < len(s) {
		b := s[at]
		if b >= 0x80 {
			return at - start
		}
		if kind == rootASCIIFold {
			b |= 0x20
		}
		if b == needle {
			return at - start
		}
		at++
	}
	return at - start
}

func literalSkipASCII(s string, at int, kind uint8, needle byte) int {
	start := at
	for at < len(s) {
		value := s[at]
		if kind == rootASCIIFold {
			value |= 0x20
		}
		if value == needle {
			break
		}
		at++
	}
	return at - start
}

func probeSkipBytes(s string, at, candidates int, probe *asciiProbe) int {
	start := at
	for at-start < candidates {
		if asciiProbeAt(s, at, probe) {
			break
		}
		at++
	}
	return at - start
}

func asciiOnlyProbeSkipBytes(s string, at, candidates int, probe *asciiProbe) (int, bool) {
	start := at
	for at-start < candidates {
		if s[at] >= utf8.RuneSelf {
			return at - start, false
		}
		if asciiProbeAt(s, at, probe) {
			return at - start, true
		}
		at++
	}
	return at - start, true
}

func asciiPairSkipBytes(s string, at, candidates int, probe *asciiPairProbe) int {
	start := at
	for at-start < candidates {
		if asciiPairAt(s, at, probe) {
			break
		}
		at++
	}
	return at - start
}

func asciiProbeAt(s string, at int, probe *asciiProbe) bool {
	values := [3]byte{probe.first, probe.second, probe.third}
	offsets := [3]int{probe.firstAt, probe.secondAt, probe.thirdAt}
	for i, offset := range offsets {
		value := s[at+offset]
		if probe.fold&(1<<i) != 0 {
			value |= 0x20
		}
		if value != values[i] {
			return false
		}
	}
	return true
}

func findASCIIRunBytes(s string, kind uint8, needle byte, need int) int {
	run, start := 0, 0
	for at := range s {
		value := s[at]
		if kind == rootASCIIFold {
			value |= 0x20
		}
		if value == needle {
			if run == 0 {
				start = at
			}
			run++
			if run >= need {
				return start
			}
		} else {
			run = 0
		}
	}
	return -1
}

func pairShuftiSkipBytes(s string, at int, filter *rootFilter) int {
	return pairShuftiSkipScalar(s, at, &filter.shufti)
}

func pairPairConfirmBytes(s string, at, candidates int, filter *pairPairFilter, confirm unicodePairConfirm) int {
	start := at
	for at-start < candidates {
		if pairPairAt(s, at, filter) && confirm.matchesAt(s, at-confirm.anchorAt()) {
			return at - start
		}
		at++
	}
	return candidates
}

func pairPairSkipBytes(s string, at int, filter *pairPairFilter) int {
	start := at
	for at+int(filter.offset)+1 < len(s) {
		if pairPairAt(s, at, filter) {
			break
		}
		at++
	}
	return at - start
}

func pairPairAt(s string, at int, filter *pairPairFilter) bool {
	if at+int(filter.offset)+1 >= len(s) {
		return false
	}
	first, second := s[at], s[at+1]
	if (first != filter.first0 || second != filter.second0) && (first != filter.first1 || second != filter.second1) {
		return false
	}
	at += int(filter.offset)
	first, second = s[at], s[at+1]
	return (first == filter.confirmFirst0 && second == filter.confirmSecond0) ||
		(first == filter.confirmFirst1 && second == filter.confirmSecond1)
}

func pairSecondSkipBytes(s string, at int, filter *rootFilter) int {
	if at+1 >= len(s) {
		return len(s) - at
	}
	start := at
	for at++; at < len(s); at++ {
		if s[at] == filter.pairs[0].second || s[at] == filter.pairs[1].second {
			return at - start - 1
		}
	}
	return len(s) - start
}

func filterSkipBytes(s string, at int, filter *rootFilter) int {
	if filter.shufti.usable() {
		return pairShuftiSkipBytes(s, at, filter)
	}
	return filterSkipGeneralBytes(s, at, filter)
}

func filterSkipGeneralBytes(s string, at int, filter *rootFilter) int {
	start := at
	for at < len(s) {
		b := s[at]
		for i := range filter.oneN {
			if b == filter.ones[i] {
				return at - start
			}
		}
		if at+1 < len(s) {
			next := s[at+1]
			for i := range filter.pairN {
				pair := filter.pairs[i]
				left, right := b, next
				if pair.fold&rootPairFoldFirst != 0 {
					left |= 0x20
				}
				if pair.fold&rootPairFoldSecond != 0 {
					right |= 0x20
				}
				if left == pair.first && right == pair.second {
					return at - start
				}
			}
		}
		at++
	}
	return at - start
}

func filterSkipScalar(s string, at int, filter *rootFilter) int {
	return filterSkipGeneralBytes(s, at, filter)
}

func tripleShuftiSkipBytes(s string, at int, filter *tripleShuftiFilter) int {
	return tripleShuftiSkipScalar(s, at, filter)
}

func asciiPairAnchorSkipBytes(s string, at int, filter *asciiPairAnchorFilter) int {
	return asciiPairAnchorSkipScalar(s, at, filter)
}

func tripleSkipBytes(s string, at int, filter *tripleFilter) int {
	return tripleSkipScalar(s, at, filter)
}

func tripleSkipScalar(s string, at int, filter *tripleFilter) int {
	start := at
	for at+2 < len(s) {
		for i := range filter.n {
			triple := filter.values[i]
			first, second, third := s[at], s[at+1], s[at+2]
			if triple.fold&1 != 0 {
				first |= 0x20
			}
			if triple.fold&2 != 0 {
				second |= 0x20
			}
			if triple.fold&4 != 0 {
				third |= 0x20
			}
			if first == triple.first && second == triple.second && third == triple.third {
				return at - start
			}
		}
		at++
	}
	return at - start
}

func pairSkipASCII(s string, at int, firstKind uint8, first byte, secondKind uint8, second byte) int {
	start := at
	for at+1 < len(s) {
		left, right := s[at], s[at+1]
		if left >= 0x80 || right >= 0x80 {
			break
		}
		if firstKind == rootASCIIFold {
			left |= 0x20
		}
		if secondKind == rootASCIIFold {
			right |= 0x20
		}
		if left == first && right == second {
			break
		}
		at++
	}
	return at - start
}

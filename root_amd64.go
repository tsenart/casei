//go:build amd64

package casei

import (
	"math/bits"
	"unicode/utf8"
	"unsafe"

	"golang.org/x/sys/cpu"
)

func runtimeVectorBits() int {
	if cpu.X86.HasAVX512F && cpu.X86.HasAVX512BW {
		return 512
	}
	if cpu.X86.HasAVX2 {
		return 256
	}
	return 0
}

func asciiPairVBMIEnabled() bool {
	return cpu.X86.HasAVX512F && cpu.X86.HasAVX512BW && cpu.X86.HasAVX512VBMI
}

// asciiFixedPrefix8 compares the compiled low eight pattern bytes after
// applying case bits only at ASCII-letter positions. Its callers establish an
// in-bounds eight-byte window before this unaligned amd64 load.
func asciiFixedPrefix8(s string, at int, word, fold uint64) bool {
	got := *(*uint64)(unsafe.Pointer(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at)))
	return got|fold == word
}

func asciiFixedPrefix8Masked(s string, at int, word, fold, mask uint64) bool {
	got := *(*uint64)(unsafe.Pointer(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at)))
	return (got|fold)&mask == word
}

const (
	byteHighBits uint64 = 0x8080808080808080
	byteOnes     uint64 = 0x0101010101010101
	byteCaseBit  uint64 = 0x2020202020202020
)

// rootSkip32 and rootSkip64 scan complete vector blocks and return the length
// of the root-to-root prefix. Their callers check x/sys/cpu's GODEBUG-aware
// feature flags before entering the corresponding instruction set.
func rootSkip32(ptr *byte, n int, target, fold uint64) int
func rootSkip64(ptr *byte, n int, target, fold uint64) int
func literalSkip32(ptr *byte, n int, target, fold uint64) int
func literalSkip64(ptr *byte, n int, target, fold uint64) int
func runMask32(ptr *byte, target, fold uint64) uint32
func runMask64(ptr *byte, target, fold uint64) uint64
func probeSkip32(ptr *byte, n int, probe *asciiProbe) int
func probeSkip64(ptr *byte, n int, probe *asciiProbe) int
func probeVBMISkip64(ptr *byte, n int, probe *asciiVBMIProbe) int
func asciiOnlyProbeSkip64(ptr *byte, n int, probe *asciiProbe) int
func asciiPairDirectSkip64(ptr *byte, n int, probe *asciiPairProbe) int
func asciiPairDirectVBMISkip64(ptr *byte, n int, probe *asciiPairVBMIProbe) int
func asciiPairShortSkip64(ptr *byte, n int, probe *asciiPairProbe) int
func pairSetSkip32(ptr *byte, n int, filter *rootFilter) int
func pairSetSkip64(ptr *byte, n int, filter *rootFilter) int
func pairShuftiSkip64(ptr *byte, n int, filter *pairShuftiFilter) int
func pairShuftiWithOnesSkip64(ptr *byte, n int, filter *pairShuftiFilter) int
func pairPairSkip64(ptr *byte, n int, filter *pairPairFilter) int
func pairPairVBMISkip64(ptr *byte, n int, filter *pairPairVBMIFilter) int
func pairPairConfirmVBMI64(ptr *byte, n int, filter *pairPairVBMIFilter, confirm *byte) int
func pairPairWordSkip64(ptr *byte, n int, filter *pairPairFilter) int
func pairSecondSkip32(ptr *byte, n int, filter *rootFilter) int
func pairSecondSkip64(ptr *byte, n int, filter *rootFilter) int
func pairSkip32(ptr *byte, n int, first, firstFold, second, secondFold uint64) int
func pairSkip64(ptr *byte, n int, first, firstFold, second, secondFold uint64) int
func filterSkip32(ptr *byte, n int, filter *rootFilter) int
func filterSkip64(ptr *byte, n int, filter *rootFilter) int
func tripleSkip32(ptr *byte, n int, filter *tripleFilter) int
func tripleSkip64(ptr *byte, n int, filter *tripleFilter) int
func tripleShuftiSkip64(ptr *byte, n int, filter *tripleShuftiFilter) int
func asciiPairAnchorSkip64(ptr *byte, n int, filter *asciiPairAnchorFilter) int
func asciiPairAnchorVBMISkip64(ptr *byte, n int, filter *asciiPairVBMIAnchorFilter) int
func tripleSharedPrefixSkip64(ptr *byte, n int, filter *tripleFilter) int
func tripleASCIIUTF8Skip64(ptr *byte, n int, filter *tripleFilter) int
func tripleMixedSkip64(ptr *byte, n int, filter *tripleFilter) int
func triplePairSkip32(ptr *byte, n int, filter *tripleFilter) int
func triplePairSkip64(ptr *byte, n int, filter *tripleFilter) int

// rootSkipASCII returns the length of the root-to-root prefix at at. A stop is
// left for the scalar transition, which preserves UTF-8 and opaque-byte
// semantics. AVX-512 BW uses byte-to-mask instructions for 64-byte blocks;
// AVX2 supplies the 32-byte path and SWAR is the amd64 fallback.
func rootSkipASCII(s string, at int, kind uint8, needle byte) int {
	start := at
	remaining := len(s) - at
	target := uint64(needle) * byteOnes
	fold := uint64(0)
	if kind == rootASCIIFold {
		fold = byteCaseBit
	}
	if cpu.X86.HasAVX512F && cpu.X86.HasAVX512BW && remaining >= 64 {
		full := remaining &^ 63
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		skipped := rootSkip64(ptr, remaining, target, fold)
		at += skipped
		if skipped < full {
			return at - start
		}
	} else if cpu.X86.HasAVX2 && remaining >= 32 {
		full := remaining &^ 31
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		skipped := rootSkip32(ptr, remaining, target, fold)
		at += skipped
		if skipped < full {
			return at - start
		}
	}
	return at - start + rootSkipScalar(s, at, kind, needle, target, fold)
}

// literalSkipASCII scans a known ASCII literal without treating high bytes as
// stop markers. Its callers anchor only fixed ASCII pattern bytes, so UTF-8 and
// malformed bytes cannot be a match and may be skipped like any other miss.
func literalSkipASCII(s string, at int, kind uint8, needle byte) int {
	start := at
	remaining := len(s) - at
	target := uint64(needle) * byteOnes
	fold := uint64(0)
	if kind == rootASCIIFold {
		fold = byteCaseBit
	}
	if cpu.X86.HasAVX512F && cpu.X86.HasAVX512BW && remaining >= 64 {
		full := remaining &^ 63
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		skipped := literalSkip64(ptr, remaining, target, fold)
		at += skipped
		if skipped < full {
			return at - start
		}
	} else if cpu.X86.HasAVX2 && remaining >= 32 {
		full := remaining &^ 31
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		skipped := literalSkip32(ptr, remaining, target, fold)
		at += skipped
		if skipped < full {
			return at - start
		}
	}
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
	if cpu.X86.HasAVX512F && cpu.X86.HasAVX512BW && candidates >= 64 {
		full := candidates &^ 63
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		vbmi := cpu.X86.HasAVX512VBMI && probe.vbmi.valid != 0
		var skipped int
		if vbmi {
			skipped = probeVBMISkip64(ptr, candidates, &probe.vbmi)
		} else {
			skipped = probeSkip64(ptr, candidates, probe)
		}
		at += skipped
		if skipped < full {
			return at - start
		}
		if full != candidates {
			// The final partial candidate range has a full, safely readable
			// vector immediately before it. Rechecking that overlapping vector
			// avoids scalar confirmation probes for its trailing lanes.
			tailAt := start + candidates - 64
			tailPtr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), tailAt))
			var tail int
			if vbmi {
				tail = probeVBMISkip64(tailPtr, 64, &probe.vbmi)
			} else {
				tail = probeSkip64(tailPtr, 64, probe)
			}
			if tail < 64 {
				return tailAt + tail - start
			}
			at = start + candidates
		}
	} else if cpu.X86.HasAVX2 && candidates >= 32 {
		full := candidates &^ 31
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		skipped := probeSkip32(ptr, candidates, probe)
		at += skipped
		if skipped < full {
			return at - start
		}
	}
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
	if cpu.X86.HasAVX512F && cpu.X86.HasAVX512BW && candidates >= 64 {
		full := candidates &^ 63
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		skipped := asciiOnlyProbeSkip64(ptr, candidates, probe)
		at += skipped
		if skipped < full {
			return at - start, s[at] < utf8.RuneSelf
		}
		if full != candidates {
			// Revisit the final partial block from a safe overlapping start.
			// This avoids a scalar triple probe for up to 63 trailing starts.
			tailAt := start + candidates - 64
			tailPtr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), tailAt))
			tail := asciiOnlyProbeSkip64(tailPtr, 64, probe)
			if tail < 64 {
				at = tailAt + tail
				return at - start, s[at] < utf8.RuneSelf
			}
			at = start + candidates
		}
	}
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
	// The pair probe loads a second stream after its compiled displacement.
	// Keep one carried 64-byte block after each candidate block and bound the
	// assembly input by actual readable bytes, not merely candidate starts.
	available := len(s) - at
	vectorCandidates := (available - 64) &^ 63
	if vectorCandidates > candidates {
		vectorCandidates = candidates &^ 63
	}
	if cpu.X86.HasAVX512F && cpu.X86.HasAVX512BW && vectorCandidates >= 64 {
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		var skipped int
		if vectorCandidates < 1024 {
			skipped = asciiPairShortSkip64(ptr, vectorCandidates, probe)
		} else if cpu.X86.HasAVX512VBMI && probe.vbmi.valid != 0 {
			skipped = asciiPairDirectVBMISkip64(ptr, vectorCandidates, &probe.vbmi)
		} else {
			skipped = asciiPairDirectSkip64(ptr, vectorCandidates, probe)
		}
		at += skipped
		if skipped < vectorCandidates {
			return at - start
		}
	}
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
	target := uint64(needle) * byteOnes
	fold := uint64(0)
	if kind == rootASCIIFold {
		fold = byteCaseBit
	}
	run, runStart := 0, 0
	consume := func(equal uint64, width, at int) int {
		missing := ^equal
		if width < 64 {
			missing &= (uint64(1) << width) - 1
		}
		position := 0
		for missing != 0 {
			next := bits.TrailingZeros64(missing)
			if gap := next - position; gap != 0 {
				if run == 0 {
					runStart = at + position
				}
				run += gap
				if run >= need {
					return runStart
				}
			}
			run = 0
			position = next + 1
			missing &= missing - 1
		}
		if gap := width - position; gap != 0 {
			if run == 0 {
				runStart = at + position
			}
			run += gap
			if run >= need {
				return runStart
			}
		}
		return -1
	}

	at := 0
	if cpu.X86.HasAVX512F && cpu.X86.HasAVX512BW {
		for at+64 <= len(s) {
			ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
			if start := consume(runMask64(ptr, target, fold), 64, at); start >= 0 {
				return start
			}
			at += 64
		}
	} else if cpu.X86.HasAVX2 {
		for at+32 <= len(s) {
			ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
			if start := consume(uint64(runMask32(ptr, target, fold)), 32, at); start >= 0 {
				return start
			}
			at += 32
		}
	}
	for ; at < len(s); at++ {
		value := s[at]
		if kind == rootASCIIFold {
			value |= 0x20
		}
		if value == needle {
			if run == 0 {
				runStart = at
			}
			run++
			if run >= need {
				return runStart
			}
		} else {
			run = 0
		}
	}
	return -1
}

// pairSecondSkipBytes scans second-byte values and leaves the preceding byte
// for the ordinary plan transition. Every admitted root pair has one of these
// values, while a false continuation candidate is rejected by that transition.
func pairSecondSkipBytes(s string, at int, filter *rootFilter) int {
	if at+1 >= len(s) {
		return len(s) - at
	}
	start := at
	at++
	remaining := len(s) - at
	if cpu.X86.HasAVX512F && cpu.X86.HasAVX512BW && remaining >= 64 {
		full := remaining &^ 63
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		skipped := pairSecondSkip64(ptr, remaining, filter)
		at += skipped
		if skipped < full {
			return at - start - 1
		}
	} else if cpu.X86.HasAVX2 && remaining >= 32 {
		full := remaining &^ 31
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		skipped := pairSecondSkip32(ptr, remaining, filter)
		at += skipped
		if skipped < full {
			return at - start - 1
		}
	}
	for at < len(s) {
		value := s[at]
		if value == filter.pairs[0].second || filter.pairN == 2 && value == filter.pairs[1].second {
			return at - start - 1
		}
		at++
	}
	return len(s) - start
}

// pairShuftiSkipBytes is the AVX-512 BW route for the bounded two-group
// nibble projection compiled into a dense root filter. The vector loop only
// discovers possible root pairs; findFiltered still feeds every stop through
// the ordinary decoded plan transition.
func pairShuftiSkipBytes(s string, at int, filter *rootFilter) int {
	if !cpu.X86.HasAVX512F || !cpu.X86.HasAVX512BW {
		return filterSkipGeneralBytes(s, at, filter)
	}
	start := at
	remaining := len(s) - at
	if remaining >= 65 {
		full := ((remaining - 1) / 64) * 64
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		var skipped int
		if filter.shufti.oneN != 0 {
			skipped = pairShuftiWithOnesSkip64(ptr, remaining, &filter.shufti)
		} else {
			skipped = pairShuftiSkip64(ptr, remaining, &filter.shufti)
		}
		at += skipped
		if skipped < full {
			return at - start
		}
	}
	return at - start + pairShuftiSkipScalar(s, at, &filter.shufti)
}

// pairPairConfirmBytes scans full 64-start blocks and returns the first
// fully confirmed anchor, or candidates when no full-block candidate matches.
// findUnicodePairConfirm establishes the feature and bound guards.
func pairPairConfirmBytes(s string, at, candidates int, filter *pairPairFilter, confirm unicodePairConfirm) int {
	ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
	return pairPairConfirmVBMI64(ptr, candidates, &filter.vbmi, unsafe.StringData(string(confirm)))
}

func pairPairSkipBytes(s string, at int, filter *pairPairFilter) int {
	start := at
	offset := int(filter.offset)
	candidates := len(s) - at - offset - 1
	if candidates <= 0 {
		return 0
	}
	if cpu.X86.HasAVX512F && cpu.X86.HasAVX512BW && candidates >= 64 {
		full := candidates &^ 63
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		var skipped int
		if cpu.X86.HasAVX512VBMI && filter.vbmi.valid != 0 {
			skipped = pairPairVBMISkip64(ptr, candidates, &filter.vbmi)
		} else if cpu.X86.HasBMI2 {
			skipped = pairPairWordSkip64(ptr, candidates, filter)
		} else {
			skipped = pairPairSkip64(ptr, candidates, filter)
		}
		at += skipped
		if skipped < full {
			return at - start
		}
	}
	for at-start < candidates {
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

func pairSetSkipBytes(s string, at int, filter *rootFilter) int {
	start := at
	remaining := len(s) - at
	if cpu.X86.HasAVX512F && cpu.X86.HasAVX512BW && remaining >= 65 {
		full := ((remaining - 1) / 64) * 64
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		skipped := pairSetSkip64(ptr, remaining, filter)
		at += skipped
		if skipped < full {
			return at - start
		}
	} else if cpu.X86.HasAVX2 && remaining >= 33 {
		full := ((remaining - 1) / 32) * 32
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		skipped := pairSetSkip32(ptr, remaining, filter)
		at += skipped
		if skipped < full {
			return at - start
		}
	}
	return at - start + filterSkipScalar(s, at, filter)
}

func filterSkipBytes(s string, at int, filter *rootFilter) int {
	if filter.shufti.usable() {
		return pairShuftiSkipBytes(s, at, filter)
	}
	return filterSkipGeneralBytes(s, at, filter)
}

func filterSkipGeneralBytes(s string, at int, filter *rootFilter) int {
	if filter.oneN == 0 && filter.pairN == 2 && filter.pairs[0].fold == 0 && filter.pairs[1].fold == 0 {
		return pairSetSkipBytes(s, at, filter)
	}
	start := at
	remaining := len(s) - at
	if cpu.X86.HasAVX512F && cpu.X86.HasAVX512BW && remaining >= 65 {
		full := ((remaining - 1) / 64) * 64
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		skipped := filterSkip64(ptr, remaining, filter)
		at += skipped
		if skipped < full {
			return at - start
		}
	} else if cpu.X86.HasAVX2 && remaining >= 33 {
		full := ((remaining - 1) / 32) * 32
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		skipped := filterSkip32(ptr, remaining, filter)
		at += skipped
		if skipped < full {
			return at - start
		}
	}
	return at - start + filterSkipScalar(s, at, filter)
}

func filterSkipScalar(s string, at int, filter *rootFilter) int {
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

// tripleShuftiSkipBytes evaluates the bounded multi-triple projection with
// AVX-512 BW. It is entered only by the runtime-gated caller in tripleSkipBytes;
// scalar tails retain the same conservative table predicate.
func tripleShuftiSkipBytes(s string, at int, filter *tripleShuftiFilter) int {
	start := at
	remaining := len(s) - at
	if cpu.X86.HasAVX512F && cpu.X86.HasAVX512BW && remaining >= 66 {
		full := ((remaining - 2) / 64) * 64
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		skipped := tripleShuftiSkip64(ptr, remaining, filter)
		at += skipped
		if skipped < full {
			return at - start
		}
	}
	return at - start + tripleShuftiSkipScalar(s, at, filter)
}

// asciiPairAnchorSkipBytes scans a single bounded pair table. The route that
// calls it has already proved an all-ASCII, non-NUL haystack; this function
// remains a conservative filter and its caller replays plan transitions.
func asciiPairAnchorSkipBytes(s string, at int, filter *asciiPairAnchorFilter) int {
	start := at
	remaining := len(s) - at
	if cpu.X86.HasAVX512F && cpu.X86.HasAVX512BW && remaining >= 65 {
		full := ((remaining - 1) / 64) * 64
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		var skipped int
		if cpu.X86.HasAVX512VBMI && filter.vbmi.valid != 0 {
			skipped = asciiPairAnchorVBMISkip64(ptr, remaining, &filter.vbmi)
		} else {
			skipped = asciiPairAnchorSkip64(ptr, remaining, filter)
		}
		at += skipped
		if skipped < full {
			return at - start
		}
	}
	return at - start + asciiPairAnchorSkipScalar(s, at, filter)
}

func triplePairSkipBytes(s string, at int, filter *tripleFilter) int {
	start := at
	remaining := len(s) - at
	if cpu.X86.HasAVX512F && cpu.X86.HasAVX512BW && remaining >= 66 {
		full := ((remaining - 2) / 64) * 64
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		skipped := triplePairSkip64(ptr, remaining, filter)
		at += skipped
		if skipped < full {
			return at - start
		}
	} else if cpu.X86.HasAVX2 && remaining >= 34 {
		full := ((remaining - 2) / 32) * 32
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		skipped := triplePairSkip32(ptr, remaining, filter)
		at += skipped
		if skipped < full {
			return at - start
		}
	}
	return at - start + tripleSkipScalar(s, at, filter)
}

func tripleSkipBytes(s string, at int, filter *tripleFilter) int {
	if filter.n == 2 && filter.values[0].fold == 7 && filter.values[1].fold == 7 {
		return triplePairSkipBytes(s, at, filter)
	}
	if filter.shufti.usable() && runtimeVectorBits() == 512 {
		return tripleShuftiSkipBytes(s, at, &filter.shufti)
	}
	start := at
	remaining := len(s) - at
	if cpu.X86.HasAVX512F && cpu.X86.HasAVX512BW && remaining >= 66 {
		full := ((remaining - 2) / 64) * 64
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		if filter.n == 2 && filter.values[0].fold == 7 && filter.values[1].fold == 3 &&
			filter.values[0].first == filter.values[1].first && filter.values[0].second == filter.values[1].second &&
			filter.values[1].third >= utf8.RuneSelf {
			skipped := tripleSharedPrefixSkip64(ptr, remaining, filter)
			at += skipped
			if skipped < full {
				return at - start
			}
			return at - start + tripleSkipScalar(s, at, filter)
		}
		if filter.n == 2 && filter.values[0].fold == 7 && filter.values[1].fold == 0 &&
			filter.values[1].first >= utf8.RuneSelf {
			skipped := tripleASCIIUTF8Skip64(ptr, remaining, filter)
			at += skipped
			if skipped < full {
				return at - start
			}
			return at - start + tripleSkipScalar(s, at, filter)
		}
		if filter.n == 3 && filter.values[0].fold == 7 && filter.values[1].fold == 7 &&
			filter.values[2].fold == 4 && filter.values[2].first >= utf8.RuneSelf {
			skipped := tripleMixedSkip64(ptr, remaining, filter)
			at += skipped
			if skipped < full {
				return at - start
			}
			return at - start + tripleSkipScalar(s, at, filter)
		}
		skipped := tripleSkip64(ptr, remaining, filter)
		at += skipped
		if skipped < full {
			return at - start
		}
	} else if cpu.X86.HasAVX2 && remaining >= 34 {
		full := ((remaining - 2) / 32) * 32
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		skipped := tripleSkip32(ptr, remaining, filter)
		at += skipped
		if skipped < full {
			return at - start
		}
	}
	return at - start + tripleSkipScalar(s, at, filter)
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
	remaining := len(s) - at
	firstTarget := uint64(first) * byteOnes
	firstFold := uint64(0)
	if firstKind == rootASCIIFold {
		firstFold = byteCaseBit
	}
	secondTarget := uint64(second) * byteOnes
	secondFold := uint64(0)
	if secondKind == rootASCIIFold {
		secondFold = byteCaseBit
	}
	if cpu.X86.HasAVX512F && cpu.X86.HasAVX512BW && remaining >= 65 {
		full := ((remaining - 1) / 64) * 64
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		skipped := pairSkip64(ptr, remaining, firstTarget, firstFold, secondTarget, secondFold)
		at += skipped
		if skipped < full {
			return at - start
		}
	} else if cpu.X86.HasAVX2 && remaining >= 33 {
		full := ((remaining - 1) / 32) * 32
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		skipped := pairSkip32(ptr, remaining, firstTarget, firstFold, secondTarget, secondFold)
		at += skipped
		if skipped < full {
			return at - start
		}
	}
	return at - start + pairSkipScalar(s, at, firstKind, first, secondKind, second)
}

func pairSkipScalar(s string, at int, firstKind uint8, first byte, secondKind uint8, second byte) int {
	start := at
	for at+1 < len(s) {
		left, right := s[at], s[at+1]
		if left >= utf8.RuneSelf || right >= utf8.RuneSelf {
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

func rootSkipScalar(s string, at int, kind uint8, needle byte, target, fold uint64) int {
	start := at
	for len(s)-at >= 8 {
		ptr := (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), at))
		word := *(*uint64)(unsafe.Pointer(ptr))
		difference := (word | fold) ^ target
		stops := word&byteHighBits | (difference-byteOnes)&^difference&byteHighBits
		skipped := bits.TrailingZeros64(stops) >> 3
		at += skipped
		if skipped < 8 {
			return at - start
		}
	}
	for at < len(s) {
		b := s[at]
		if b >= 0x80 {
			break
		}
		if kind == rootASCIIFold {
			b |= 0x20
		}
		if b == needle {
			break
		}
		at++
	}
	return at - start
}

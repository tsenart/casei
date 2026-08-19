//go:build amd64 && goexperiment.simd

package casei

import (
	"math/bits"
	"unsafe"

	simd "simd/archsimd"
)

// The selected backend uses archsimd values directly. The wrappers in
// root_amd64.go retain the existing x/sys/cpu feature and length dispatch, so
// these loads are reached only after their ISA and readable-width guards.
func simdLoad32(ptr *byte, at int) simd.Uint8x32 {
	return simd.LoadUint8x32((*[32]uint8)(unsafe.Add(unsafe.Pointer(ptr), at)))
}

func simdLoad64(ptr *byte, at int) simd.Uint8x64 {
	return simd.LoadUint8x64((*[64]uint8)(unsafe.Add(unsafe.Pointer(ptr), at)))
}

// The Shufti tables have one 16-byte lookup table per 128-bit lane.
func simdRepeat16x64(table *[16]byte) simd.Uint8x64 {
	v16 := simd.LoadUint8x16(table)
	var v32 simd.Uint8x32
	v32 = v32.SetLo(v16).SetHi(v16)
	var v64 simd.Uint8x64
	return v64.SetLo(v32).SetHi(v32)
}

func rootSkip32(ptr *byte, n int, target, fold uint64) int {
	needle := simd.BroadcastUint8x32(byte(target))
	folded := simd.BroadcastUint8x32(byte(fold))
	zero := simd.BroadcastInt8x32(0)
	at := 0
	for at+32 <= n {
		raw := simdLoad32(ptr, at)
		stops := raw.AsInt8x32().Less(zero).Or(raw.Or(folded).Equal(needle))
		if mask := stops.ToBits(); mask != 0 {
			return at + bits.TrailingZeros32(mask)
		}
		at += 32
	}
	return at
}

func rootSkip64(ptr *byte, n int, target, fold uint64) int {
	needle := simd.BroadcastUint8x64(byte(target))
	folded := simd.BroadcastUint8x64(byte(fold))
	zero := simd.BroadcastInt8x64(0)
	at := 0
	for at+64 <= n {
		raw := simdLoad64(ptr, at)
		stops := raw.AsInt8x64().Less(zero).Or(raw.Or(folded).Equal(needle))
		if mask := stops.ToBits(); mask != 0 {
			return at + bits.TrailingZeros64(mask)
		}
		at += 64
	}
	return at
}

func literalSkip32(ptr *byte, n int, target, fold uint64) int {
	needle := simd.BroadcastUint8x32(byte(target))
	folded := simd.BroadcastUint8x32(byte(fold))
	at := 0
	for at+32 <= n {
		if mask := simdLoad32(ptr, at).Or(folded).Equal(needle).ToBits(); mask != 0 {
			return at + bits.TrailingZeros32(mask)
		}
		at += 32
	}
	return at
}

func literalSkip64(ptr *byte, n int, target, fold uint64) int {
	needle := simd.BroadcastUint8x64(byte(target))
	folded := simd.BroadcastUint8x64(byte(fold))
	at := 0
	for at+64 <= n {
		if mask := simdLoad64(ptr, at).Or(folded).Equal(needle).ToBits(); mask != 0 {
			return at + bits.TrailingZeros64(mask)
		}
		at += 64
	}
	return at
}

func runSkip32(ptr *byte, n int, target, fold uint64) int {
	needle := simd.BroadcastUint8x32(byte(target))
	folded := simd.BroadcastUint8x32(byte(fold))
	at := 0
	for at+32 <= n {
		if different := ^simdLoad32(ptr, at).Or(folded).Equal(needle).ToBits(); different != 0 {
			return at + bits.TrailingZeros32(different)
		}
		at += 32
	}
	return at
}

func runSkip64(ptr *byte, n int, target, fold uint64) int {
	needle := simd.BroadcastUint8x64(byte(target))
	folded := simd.BroadcastUint8x64(byte(fold))
	at := 0
	for at+64 <= n {
		if different := ^simdLoad64(ptr, at).Or(folded).Equal(needle).ToBits(); different != 0 {
			return at + bits.TrailingZeros64(different)
		}
		at += 64
	}
	return at
}

func runMask32(ptr *byte, target, fold uint64) uint32 {
	needle := simd.BroadcastUint8x32(byte(target))
	folded := simd.BroadcastUint8x32(byte(fold))
	return simdLoad32(ptr, 0).Or(folded).Equal(needle).ToBits()
}

func runMask64(ptr *byte, target, fold uint64) uint64 {
	needle := simd.BroadcastUint8x64(byte(target))
	folded := simd.BroadcastUint8x64(byte(fold))
	return simdLoad64(ptr, 0).Or(folded).Equal(needle).ToBits()
}

func probeSkip32(ptr *byte, n int, probe *asciiProbe) int {
	first := simd.BroadcastUint8x32(probe.first)
	second := simd.BroadcastUint8x32(probe.second)
	third := simd.BroadcastUint8x32(probe.third)
	var firstFold, secondFold, thirdFold byte
	if probe.fold&1 != 0 {
		firstFold = 0x20
	}
	if probe.fold&2 != 0 {
		secondFold = 0x20
	}
	if probe.fold&4 != 0 {
		thirdFold = 0x20
	}
	fold0 := simd.BroadcastUint8x32(firstFold)
	fold1 := simd.BroadcastUint8x32(secondFold)
	fold2 := simd.BroadcastUint8x32(thirdFold)
	at := 0
	for at+32 <= n {
		matches := simdLoad32(ptr, at+probe.firstAt).Or(fold0).Equal(first)
		matches = matches.And(simdLoad32(ptr, at+probe.secondAt).Or(fold1).Equal(second))
		matches = matches.And(simdLoad32(ptr, at+probe.thirdAt).Or(fold2).Equal(third))
		if mask := matches.ToBits(); mask != 0 {
			return at + bits.TrailingZeros32(mask)
		}
		at += 32
	}
	return at
}

func probeSkip64(ptr *byte, n int, probe *asciiProbe) int {
	first := simd.BroadcastUint8x64(probe.first)
	second := simd.BroadcastUint8x64(probe.second)
	third := simd.BroadcastUint8x64(probe.third)
	var firstFold, secondFold, thirdFold byte
	if probe.fold&1 != 0 {
		firstFold = 0x20
	}
	if probe.fold&2 != 0 {
		secondFold = 0x20
	}
	if probe.fold&4 != 0 {
		thirdFold = 0x20
	}
	fold0 := simd.BroadcastUint8x64(firstFold)
	fold1 := simd.BroadcastUint8x64(secondFold)
	fold2 := simd.BroadcastUint8x64(thirdFold)
	at := 0
	for at+64 <= n {
		matches := simdLoad64(ptr, at+probe.firstAt).Or(fold0).Equal(first)
		matches = matches.And(simdLoad64(ptr, at+probe.secondAt).Or(fold1).Equal(second))
		matches = matches.And(simdLoad64(ptr, at+probe.thirdAt).Or(fold2).Equal(third))
		if mask := matches.ToBits(); mask != 0 {
			return at + bits.TrailingZeros64(mask)
		}
		at += 64
	}
	return at
}

func asciiOnlyProbeSkip64(ptr *byte, n int, probe *asciiProbe) int {
	first := simd.BroadcastUint8x64(probe.first)
	second := simd.BroadcastUint8x64(probe.second)
	third := simd.BroadcastUint8x64(probe.third)
	zero := simd.BroadcastInt8x64(0)
	var firstFold, secondFold, thirdFold byte
	if probe.fold&1 != 0 {
		firstFold = 0x20
	}
	if probe.fold&2 != 0 {
		secondFold = 0x20
	}
	if probe.fold&4 != 0 {
		thirdFold = 0x20
	}
	fold0 := simd.BroadcastUint8x64(firstFold)
	fold1 := simd.BroadcastUint8x64(secondFold)
	fold2 := simd.BroadcastUint8x64(thirdFold)
	at := 0
	for at+64 <= n {
		raw0 := simdLoad64(ptr, at+probe.firstAt)
		stops := raw0.AsInt8x64().Less(zero)
		matches := raw0.Or(fold0).Equal(first)
		matches = matches.And(simdLoad64(ptr, at+probe.secondAt).Or(fold1).Equal(second))
		matches = matches.And(simdLoad64(ptr, at+probe.thirdAt).Or(fold2).Equal(third))
		if mask := stops.Or(matches).ToBits(); mask != 0 {
			return at + bits.TrailingZeros64(mask)
		}
		at += 64
	}
	return at
}

func pairSkip32(ptr *byte, n int, first, firstFold, second, secondFold uint64) int {
	firstTarget := simd.BroadcastUint8x32(byte(first))
	secondTarget := simd.BroadcastUint8x32(byte(second))
	firstCase := simd.BroadcastUint8x32(byte(firstFold))
	secondCase := simd.BroadcastUint8x32(byte(secondFold))
	zero := simd.BroadcastInt8x32(0)
	at := 0
	for at+33 <= n {
		left := simdLoad32(ptr, at)
		right := simdLoad32(ptr, at+1)
		stops := left.AsInt8x32().Less(zero).Or(right.AsInt8x32().Less(zero))
		matches := left.Or(firstCase).Equal(firstTarget).And(right.Or(secondCase).Equal(secondTarget))
		if mask := stops.Or(matches).ToBits(); mask != 0 {
			return at + bits.TrailingZeros32(mask)
		}
		at += 32
	}
	return at
}

func pairSkip64(ptr *byte, n int, first, firstFold, second, secondFold uint64) int {
	firstTarget := simd.BroadcastUint8x64(byte(first))
	secondTarget := simd.BroadcastUint8x64(byte(second))
	firstCase := simd.BroadcastUint8x64(byte(firstFold))
	secondCase := simd.BroadcastUint8x64(byte(secondFold))
	zero := simd.BroadcastInt8x64(0)
	at := 0
	for at+65 <= n {
		left := simdLoad64(ptr, at)
		right := simdLoad64(ptr, at+1)
		stops := left.AsInt8x64().Less(zero).Or(right.AsInt8x64().Less(zero))
		matches := left.Or(firstCase).Equal(firstTarget).And(right.Or(secondCase).Equal(secondTarget))
		if mask := stops.Or(matches).ToBits(); mask != 0 {
			return at + bits.TrailingZeros64(mask)
		}
		at += 64
	}
	return at
}

func pairSetSkip32(ptr *byte, n int, filter *rootFilter) int {
	first0 := simd.BroadcastUint8x32(filter.pairs[0].first)
	second0 := simd.BroadcastUint8x32(filter.pairs[0].second)
	first1 := simd.BroadcastUint8x32(filter.pairs[1].first)
	second1 := simd.BroadcastUint8x32(filter.pairs[1].second)
	at := 0
	for at+33 <= n {
		left := simdLoad32(ptr, at)
		right := simdLoad32(ptr, at+1)
		matches := left.Equal(first0).And(right.Equal(second0))
		matches = matches.Or(left.Equal(first1).And(right.Equal(second1)))
		if mask := matches.ToBits(); mask != 0 {
			return at + bits.TrailingZeros32(mask)
		}
		at += 32
	}
	return at
}

func pairSetSkip64(ptr *byte, n int, filter *rootFilter) int {
	first0 := simd.BroadcastUint8x64(filter.pairs[0].first)
	second0 := simd.BroadcastUint8x64(filter.pairs[0].second)
	first1 := simd.BroadcastUint8x64(filter.pairs[1].first)
	second1 := simd.BroadcastUint8x64(filter.pairs[1].second)
	at := 0
	for at+65 <= n {
		left := simdLoad64(ptr, at)
		right := simdLoad64(ptr, at+1)
		matches := left.Equal(first0).And(right.Equal(second0))
		matches = matches.Or(left.Equal(first1).And(right.Equal(second1)))
		if mask := matches.ToBits(); mask != 0 {
			return at + bits.TrailingZeros64(mask)
		}
		at += 64
	}
	return at
}

func pairSecondSkip32(ptr *byte, n int, filter *rootFilter) int {
	second0 := simd.BroadcastUint8x32(filter.pairs[0].second)
	second1 := simd.BroadcastUint8x32(filter.pairs[1].second)
	at := 0
	for at+32 <= n {
		matches := simdLoad32(ptr, at).Equal(second0).Or(simdLoad32(ptr, at).Equal(second1))
		if mask := matches.ToBits(); mask != 0 {
			return at + bits.TrailingZeros32(mask)
		}
		at += 32
	}
	return at
}

func pairSecondSkip64(ptr *byte, n int, filter *rootFilter) int {
	second0 := simd.BroadcastUint8x64(filter.pairs[0].second)
	second1 := simd.BroadcastUint8x64(filter.pairs[1].second)
	at := 0
	for at+64 <= n {
		input := simdLoad64(ptr, at)
		if mask := input.Equal(second0).Or(input.Equal(second1)).ToBits(); mask != 0 {
			return at + bits.TrailingZeros64(mask)
		}
		at += 64
	}
	return at
}

func filterSkip32(ptr *byte, n int, filter *rootFilter) int {
	caseBit := simd.BroadcastUint8x32(0x20)
	at := 0
	for at+33 <= n {
		left := simdLoad32(ptr, at)
		right := simdLoad32(ptr, at+1)
		foldedLeft := left.Or(caseBit)
		foldedRight := right.Or(caseBit)
		matches := simd.Mask8x32FromBits(0)
		for i := 0; i < int(filter.oneN); i++ {
			matches = matches.Or(left.Equal(simd.BroadcastUint8x32(filter.ones[i])))
		}
		for i := 0; i < int(filter.pairN); i++ {
			pair := filter.pairs[i]
			pairLeft, pairRight := left, right
			if pair.fold&rootPairFoldFirst != 0 {
				pairLeft = foldedLeft
			}
			if pair.fold&rootPairFoldSecond != 0 {
				pairRight = foldedRight
			}
			matches = matches.Or(pairLeft.Equal(simd.BroadcastUint8x32(pair.first)).And(pairRight.Equal(simd.BroadcastUint8x32(pair.second))))
		}
		if mask := matches.ToBits(); mask != 0 {
			return at + bits.TrailingZeros32(mask)
		}
		at += 32
	}
	return at
}

func filterSkip64(ptr *byte, n int, filter *rootFilter) int {
	caseBit := simd.BroadcastUint8x64(0x20)
	at := 0
	for at+65 <= n {
		left := simdLoad64(ptr, at)
		right := simdLoad64(ptr, at+1)
		foldedLeft := left.Or(caseBit)
		foldedRight := right.Or(caseBit)
		matches := simd.Mask8x64FromBits(0)
		for i := 0; i < int(filter.oneN); i++ {
			matches = matches.Or(left.Equal(simd.BroadcastUint8x64(filter.ones[i])))
		}
		for i := 0; i < int(filter.pairN); i++ {
			pair := filter.pairs[i]
			pairLeft, pairRight := left, right
			if pair.fold&rootPairFoldFirst != 0 {
				pairLeft = foldedLeft
			}
			if pair.fold&rootPairFoldSecond != 0 {
				pairRight = foldedRight
			}
			matches = matches.Or(pairLeft.Equal(simd.BroadcastUint8x64(pair.first)).And(pairRight.Equal(simd.BroadcastUint8x64(pair.second))))
		}
		if mask := matches.ToBits(); mask != 0 {
			return at + bits.TrailingZeros64(mask)
		}
		at += 64
	}
	return at
}

func tripleSkip32(ptr *byte, n int, filter *tripleFilter) int {
	caseBit := simd.BroadcastUint8x32(0x20)
	at := 0
	for at+34 <= n {
		raw0 := simdLoad32(ptr, at)
		raw1 := simdLoad32(ptr, at+1)
		raw2 := simdLoad32(ptr, at+2)
		folded0 := raw0.Or(caseBit)
		folded1 := raw1.Or(caseBit)
		folded2 := raw2.Or(caseBit)
		matches := simd.Mask8x32FromBits(0)
		for i := 0; i < int(filter.n); i++ {
			triple := filter.values[i]
			first, second, third := raw0, raw1, raw2
			if triple.fold&1 != 0 {
				first = folded0
			}
			if triple.fold&2 != 0 {
				second = folded1
			}
			if triple.fold&4 != 0 {
				third = folded2
			}
			tripleMatches := first.Equal(simd.BroadcastUint8x32(triple.first))
			tripleMatches = tripleMatches.And(second.Equal(simd.BroadcastUint8x32(triple.second)))
			tripleMatches = tripleMatches.And(third.Equal(simd.BroadcastUint8x32(triple.third)))
			matches = matches.Or(tripleMatches)
		}
		if mask := matches.ToBits(); mask != 0 {
			return at + bits.TrailingZeros32(mask)
		}
		at += 32
	}
	return at
}

func tripleSkip64(ptr *byte, n int, filter *tripleFilter) int {
	caseBit := simd.BroadcastUint8x64(0x20)
	at := 0
	for at+66 <= n {
		raw0 := simdLoad64(ptr, at)
		raw1 := simdLoad64(ptr, at+1)
		raw2 := simdLoad64(ptr, at+2)
		folded0 := raw0.Or(caseBit)
		folded1 := raw1.Or(caseBit)
		folded2 := raw2.Or(caseBit)
		matches := simd.Mask8x64FromBits(0)
		for i := 0; i < int(filter.n); i++ {
			triple := filter.values[i]
			first, second, third := raw0, raw1, raw2
			if triple.fold&1 != 0 {
				first = folded0
			}
			if triple.fold&2 != 0 {
				second = folded1
			}
			if triple.fold&4 != 0 {
				third = folded2
			}
			tripleMatches := first.Equal(simd.BroadcastUint8x64(triple.first))
			tripleMatches = tripleMatches.And(second.Equal(simd.BroadcastUint8x64(triple.second)))
			tripleMatches = tripleMatches.And(third.Equal(simd.BroadcastUint8x64(triple.third)))
			matches = matches.Or(tripleMatches)
		}
		if mask := matches.ToBits(); mask != 0 {
			return at + bits.TrailingZeros64(mask)
		}
		at += 64
	}
	return at
}

func triplePairSkip32(ptr *byte, n int, filter *tripleFilter) int {
	first0 := simd.BroadcastUint8x32(filter.values[0].first)
	second0 := simd.BroadcastUint8x32(filter.values[0].second)
	third0 := simd.BroadcastUint8x32(filter.values[0].third)
	first1 := simd.BroadcastUint8x32(filter.values[1].first)
	second1 := simd.BroadcastUint8x32(filter.values[1].second)
	third1 := simd.BroadcastUint8x32(filter.values[1].third)
	caseBit := simd.BroadcastUint8x32(0x20)
	at := 0
	for at+34 <= n {
		left := simdLoad32(ptr, at).Or(caseBit)
		middle := simdLoad32(ptr, at+1).Or(caseBit)
		right := simdLoad32(ptr, at+2).Or(caseBit)
		matches := left.Equal(first0).And(middle.Equal(second0)).And(right.Equal(third0))
		matches = matches.Or(left.Equal(first1).And(middle.Equal(second1)).And(right.Equal(third1)))
		if mask := matches.ToBits(); mask != 0 {
			return at + bits.TrailingZeros32(mask)
		}
		at += 32
	}
	return at
}

func triplePairSkip64(ptr *byte, n int, filter *tripleFilter) int {
	first0 := simd.BroadcastUint8x64(filter.values[0].first)
	second0 := simd.BroadcastUint8x64(filter.values[0].second)
	third0 := simd.BroadcastUint8x64(filter.values[0].third)
	first1 := simd.BroadcastUint8x64(filter.values[1].first)
	second1 := simd.BroadcastUint8x64(filter.values[1].second)
	third1 := simd.BroadcastUint8x64(filter.values[1].third)
	caseBit := simd.BroadcastUint8x64(0x20)
	at := 0
	for at+66 <= n {
		left := simdLoad64(ptr, at).Or(caseBit)
		middle := simdLoad64(ptr, at+1).Or(caseBit)
		right := simdLoad64(ptr, at+2).Or(caseBit)
		matches := left.Equal(first0).And(middle.Equal(second0)).And(right.Equal(third0))
		matches = matches.Or(left.Equal(first1).And(middle.Equal(second1)).And(right.Equal(third1)))
		if mask := matches.ToBits(); mask != 0 {
			return at + bits.TrailingZeros64(mask)
		}
		at += 64
	}
	return at
}

func tripleMixedSkip64(ptr *byte, n int, filter *tripleFilter) int {
	first0 := simd.BroadcastUint8x64(filter.values[0].first)
	second0 := simd.BroadcastUint8x64(filter.values[0].second)
	third0 := simd.BroadcastUint8x64(filter.values[0].third)
	first1 := simd.BroadcastUint8x64(filter.values[1].first)
	second1 := simd.BroadcastUint8x64(filter.values[1].second)
	third1 := simd.BroadcastUint8x64(filter.values[1].third)
	first2 := simd.BroadcastUint8x64(filter.values[2].first)
	second2 := simd.BroadcastUint8x64(filter.values[2].second)
	third2 := simd.BroadcastUint8x64(filter.values[2].third)
	caseBit := simd.BroadcastUint8x64(0x20)
	at := 0
	for at+66 <= n {
		raw0 := simdLoad64(ptr, at)
		raw1 := simdLoad64(ptr, at+1)
		raw2 := simdLoad64(ptr, at+2)
		folded0, folded1, folded2 := raw0.Or(caseBit), raw1.Or(caseBit), raw2.Or(caseBit)
		matches := folded0.Equal(first0).And(folded1.Equal(second0)).And(folded2.Equal(third0))
		matches = matches.Or(folded0.Equal(first1).And(folded1.Equal(second1)).And(folded2.Equal(third1)))
		matches = matches.Or(raw0.Equal(first2).And(raw1.Equal(second2)).And(folded2.Equal(third2)))
		if mask := matches.ToBits(); mask != 0 {
			return at + bits.TrailingZeros64(mask)
		}
		at += 64
	}
	return at
}

func tripleASCIIUTF8Skip64(ptr *byte, n int, filter *tripleFilter) int {
	first0 := simd.BroadcastUint8x64(filter.values[0].first)
	second0 := simd.BroadcastUint8x64(filter.values[0].second)
	third0 := simd.BroadcastUint8x64(filter.values[0].third)
	first1 := simd.BroadcastUint8x64(filter.values[1].first)
	second1 := simd.BroadcastUint8x64(filter.values[1].second)
	third1 := simd.BroadcastUint8x64(filter.values[1].third)
	caseBit := simd.BroadcastUint8x64(0x20)
	at := 0
	for at+66 <= n {
		raw0 := simdLoad64(ptr, at)
		raw1 := simdLoad64(ptr, at+1)
		raw2 := simdLoad64(ptr, at+2)
		matches := raw0.Or(caseBit).Equal(first0)
		matches = matches.And(raw1.Or(caseBit).Equal(second0))
		matches = matches.And(raw2.Or(caseBit).Equal(third0))
		rawMatches := raw0.Equal(first1).And(raw1.Equal(second1)).And(raw2.Equal(third1))
		if mask := matches.Or(rawMatches).ToBits(); mask != 0 {
			return at + bits.TrailingZeros64(mask)
		}
		at += 64
	}
	return at
}

func tripleSharedPrefixSkip64(ptr *byte, n int, filter *tripleFilter) int {
	first0 := simd.BroadcastUint8x64(filter.values[0].first)
	second0 := simd.BroadcastUint8x64(filter.values[0].second)
	third0 := simd.BroadcastUint8x64(filter.values[0].third)
	first1 := simd.BroadcastUint8x64(filter.values[1].first)
	second1 := simd.BroadcastUint8x64(filter.values[1].second)
	third1 := simd.BroadcastUint8x64(filter.values[1].third)
	caseBit := simd.BroadcastUint8x64(0x20)
	at := 0
	for at+66 <= n {
		raw0 := simdLoad64(ptr, at)
		raw1 := simdLoad64(ptr, at+1)
		raw2 := simdLoad64(ptr, at+2)
		folded0, folded1, folded2 := raw0.Or(caseBit), raw1.Or(caseBit), raw2.Or(caseBit)
		ascii := folded0.Equal(first0).And(folded1.Equal(second0)).And(folded2.Equal(third0))
		utf8 := folded0.Equal(first1).And(folded1.Equal(second1)).And(raw2.Equal(third1))
		if mask := ascii.Or(utf8).ToBits(); mask != 0 {
			return at + bits.TrailingZeros64(mask)
		}
		at += 64
	}
	return at
}

func pairPairSkip64(ptr *byte, n int, filter *pairPairFilter) int {
	first0 := simd.BroadcastUint8x64(filter.first0)
	second0 := simd.BroadcastUint8x64(filter.second0)
	first1 := simd.BroadcastUint8x64(filter.first1)
	second1 := simd.BroadcastUint8x64(filter.second1)
	confirmFirst0 := simd.BroadcastUint8x64(filter.confirmFirst0)
	confirmSecond0 := simd.BroadcastUint8x64(filter.confirmSecond0)
	confirmFirst1 := simd.BroadcastUint8x64(filter.confirmFirst1)
	confirmSecond1 := simd.BroadcastUint8x64(filter.confirmSecond1)
	offset := int(filter.offset)
	at := 0
	for at+64 <= n {
		left, right := simdLoad64(ptr, at), simdLoad64(ptr, at+1)
		primary := left.Equal(first0).And(right.Equal(second0))
		primary = primary.Or(left.Equal(first1).And(right.Equal(second1)))
		confirmLeft, confirmRight := simdLoad64(ptr, at+offset), simdLoad64(ptr, at+offset+1)
		confirm := confirmLeft.Equal(confirmFirst0).And(confirmRight.Equal(confirmSecond0))
		confirm = confirm.Or(confirmLeft.Equal(confirmFirst1).And(confirmRight.Equal(confirmSecond1)))
		if mask := primary.And(confirm).ToBits(); mask != 0 {
			return at + bits.TrailingZeros64(mask)
		}
		at += 64
	}
	return at
}

// pairPairWordSkip64 keeps the word-route dispatch distinct while expressing
// its pair predicate with archsimd byte masks. The surrounding dispatcher still
// selects it only for the non-VBMI BMI2 route.
func pairPairWordSkip64(ptr *byte, n int, filter *pairPairFilter) int {
	first0 := simd.BroadcastUint8x64(filter.first0)
	second0 := simd.BroadcastUint8x64(filter.second0)
	first1 := simd.BroadcastUint8x64(filter.first1)
	second1 := simd.BroadcastUint8x64(filter.second1)
	confirmFirst0 := simd.BroadcastUint8x64(filter.confirmFirst0)
	confirmSecond0 := simd.BroadcastUint8x64(filter.confirmSecond0)
	confirmFirst1 := simd.BroadcastUint8x64(filter.confirmFirst1)
	confirmSecond1 := simd.BroadcastUint8x64(filter.confirmSecond1)
	offset := int(filter.offset)
	at := 0
	for at+64 <= n {
		left, right := simdLoad64(ptr, at), simdLoad64(ptr, at+1)
		primary := left.Equal(first0).And(right.Equal(second0))
		primary = primary.Or(left.Equal(first1).And(right.Equal(second1)))
		confirmLeft, confirmRight := simdLoad64(ptr, at+offset), simdLoad64(ptr, at+offset+1)
		confirm := confirmLeft.Equal(confirmFirst0).And(confirmRight.Equal(confirmSecond0))
		confirm = confirm.Or(confirmLeft.Equal(confirmFirst1).And(confirmRight.Equal(confirmSecond1)))
		if mask := primary.And(confirm).ToBits(); mask != 0 {
			return at + bits.TrailingZeros64(mask)
		}
		at += 64
	}
	return at
}

func pairPairVBMISkip64(ptr *byte, n int, filter *pairPairVBMIFilter) int {
	firstTable := simd.LoadUint8x64(&filter.first)
	secondTable := simd.LoadUint8x64(&filter.second)
	confirmFirstTable := simd.LoadUint8x64(&filter.confirmFirst)
	confirmSecondTable := simd.LoadUint8x64(&filter.confirmSecond)
	zero := simd.BroadcastUint8x64(0)
	offset := int(filter.offset)
	at := 0
	for at+64 <= n {
		left, right := simdLoad64(ptr, at), simdLoad64(ptr, at+1)
		primary := firstTable.Permute(left).And(secondTable.Permute(right)).NotEqual(zero)
		confirmLeft, confirmRight := simdLoad64(ptr, at+offset), simdLoad64(ptr, at+offset+1)
		confirm := confirmFirstTable.Permute(confirmLeft).And(confirmSecondTable.Permute(confirmRight)).NotEqual(zero)
		if mask := primary.And(confirm).ToBits(); mask != 0 {
			return at + bits.TrailingZeros64(mask)
		}
		at += 64
	}
	return at
}

func pairShuftiSkip64(ptr *byte, n int, filter *pairShuftiFilter) int {
	firstLo0 := simdRepeat16x64(&filter.groups[0].firstLo)
	firstHi0 := simdRepeat16x64(&filter.groups[0].firstHi)
	secondLo0 := simdRepeat16x64(&filter.groups[0].secondLo)
	secondHi0 := simdRepeat16x64(&filter.groups[0].secondHi)
	firstLo1 := simdRepeat16x64(&filter.groups[1].firstLo)
	firstHi1 := simdRepeat16x64(&filter.groups[1].firstHi)
	secondLo1 := simdRepeat16x64(&filter.groups[1].secondLo)
	secondHi1 := simdRepeat16x64(&filter.groups[1].secondHi)
	nibble := simd.BroadcastUint8x64(0x0f)
	zero := simd.BroadcastUint8x64(0)
	at := 0
	for at+65 <= n {
		first, second := simdLoad64(ptr, at), simdLoad64(ptr, at+1)
		firstLo := first.And(nibble).AsInt8x64()
		firstHi := first.AsUint16x32().ShiftAllRight(4).AsUint8x64().And(nibble).AsInt8x64()
		secondLo := second.And(nibble).AsInt8x64()
		secondHi := second.AsUint16x32().ShiftAllRight(4).AsUint8x64().And(nibble).AsInt8x64()
		group0 := firstLo0.PermuteOrZeroGrouped(firstLo).And(firstHi0.PermuteOrZeroGrouped(firstHi))
		group0 = group0.And(secondLo0.PermuteOrZeroGrouped(secondLo)).And(secondHi0.PermuteOrZeroGrouped(secondHi))
		group1 := firstLo1.PermuteOrZeroGrouped(firstLo).And(firstHi1.PermuteOrZeroGrouped(firstHi))
		group1 = group1.And(secondLo1.PermuteOrZeroGrouped(secondLo)).And(secondHi1.PermuteOrZeroGrouped(secondHi))
		if mask := group0.Or(group1).NotEqual(zero).ToBits(); mask != 0 {
			return at + bits.TrailingZeros64(mask)
		}
		at += 64
	}
	return at
}

func pairShuftiWithOnesSkip64(ptr *byte, n int, filter *pairShuftiFilter) int {
	firstLo0 := simdRepeat16x64(&filter.groups[0].firstLo)
	firstHi0 := simdRepeat16x64(&filter.groups[0].firstHi)
	secondLo0 := simdRepeat16x64(&filter.groups[0].secondLo)
	secondHi0 := simdRepeat16x64(&filter.groups[0].secondHi)
	firstLo1 := simdRepeat16x64(&filter.groups[1].firstLo)
	firstHi1 := simdRepeat16x64(&filter.groups[1].firstHi)
	secondLo1 := simdRepeat16x64(&filter.groups[1].secondLo)
	secondHi1 := simdRepeat16x64(&filter.groups[1].secondHi)
	one0 := simd.BroadcastUint8x64(filter.ones[0])
	one1 := simd.BroadcastUint8x64(filter.ones[1])
	nibble := simd.BroadcastUint8x64(0x0f)
	caseBit := simd.BroadcastUint8x64(0x20)
	zero := simd.BroadcastUint8x64(0)
	at := 0
	for at+65 <= n {
		rawFirst, rawSecond := simdLoad64(ptr, at), simdLoad64(ptr, at+1)
		first, second := rawFirst.Or(caseBit), rawSecond.Or(caseBit)
		firstLo := first.And(nibble).AsInt8x64()
		firstHi := first.AsUint16x32().ShiftAllRight(4).AsUint8x64().And(nibble).AsInt8x64()
		secondLo := second.And(nibble).AsInt8x64()
		secondHi := second.AsUint16x32().ShiftAllRight(4).AsUint8x64().And(nibble).AsInt8x64()
		group0 := firstLo0.PermuteOrZeroGrouped(firstLo).And(firstHi0.PermuteOrZeroGrouped(firstHi))
		group0 = group0.And(secondLo0.PermuteOrZeroGrouped(secondLo)).And(secondHi0.PermuteOrZeroGrouped(secondHi))
		group1 := firstLo1.PermuteOrZeroGrouped(firstLo).And(firstHi1.PermuteOrZeroGrouped(firstHi))
		group1 = group1.And(secondLo1.PermuteOrZeroGrouped(secondLo)).And(secondHi1.PermuteOrZeroGrouped(secondHi))
		matches := group0.Or(group1).NotEqual(zero).Or(rawFirst.Equal(one0)).Or(rawFirst.Equal(one1))
		if mask := matches.ToBits(); mask != 0 {
			return at + bits.TrailingZeros64(mask)
		}
		at += 64
	}
	return at
}

func tripleShuftiSkip64(ptr *byte, n int, filter *tripleShuftiFilter) int {
	firstLoTable := simdRepeat16x64(&filter.firstLo)
	firstHiTable := simdRepeat16x64(&filter.firstHi)
	secondLoTable := simdRepeat16x64(&filter.secondLo)
	secondHiTable := simdRepeat16x64(&filter.secondHi)
	thirdLoTable := simdRepeat16x64(&filter.thirdLo)
	thirdHiTable := simdRepeat16x64(&filter.thirdHi)
	nibble := simd.BroadcastUint8x64(0x0f)
	caseBit := simd.BroadcastUint8x64(0x20)
	zero := simd.BroadcastUint8x64(0)
	at := 0
	for at+66 <= n {
		first := simdLoad64(ptr, at).Or(caseBit)
		second := simdLoad64(ptr, at+1).Or(caseBit)
		third := simdLoad64(ptr, at+2).Or(caseBit)
		firstLo := first.And(nibble).AsInt8x64()
		firstHi := first.AsUint16x32().ShiftAllRight(4).AsUint8x64().And(nibble).AsInt8x64()
		secondLo := second.And(nibble).AsInt8x64()
		secondHi := second.AsUint16x32().ShiftAllRight(4).AsUint8x64().And(nibble).AsInt8x64()
		thirdLo := third.And(nibble).AsInt8x64()
		thirdHi := third.AsUint16x32().ShiftAllRight(4).AsUint8x64().And(nibble).AsInt8x64()
		matches := firstLoTable.PermuteOrZeroGrouped(firstLo).And(firstHiTable.PermuteOrZeroGrouped(firstHi))
		matches = matches.And(secondLoTable.PermuteOrZeroGrouped(secondLo)).And(secondHiTable.PermuteOrZeroGrouped(secondHi))
		matches = matches.And(thirdLoTable.PermuteOrZeroGrouped(thirdLo)).And(thirdHiTable.PermuteOrZeroGrouped(thirdHi))
		if mask := matches.NotEqual(zero).ToBits(); mask != 0 {
			return at + bits.TrailingZeros64(mask)
		}
		at += 64
	}
	return at
}

func asciiPairAnchorSkip64(ptr *byte, n int, filter *asciiPairAnchorFilter) int {
	firstLoTable := simdRepeat16x64(&filter.firstLo)
	firstHiTable := simdRepeat16x64(&filter.firstHi)
	secondLoTable := simdRepeat16x64(&filter.secondLo)
	secondHiTable := simdRepeat16x64(&filter.secondHi)
	nibble := simd.BroadcastUint8x64(0x0f)
	caseBit := simd.BroadcastUint8x64(0x20)
	zero := simd.BroadcastUint8x64(0)
	at := 0
	for at+65 <= n {
		first := simdLoad64(ptr, at).Or(caseBit)
		second := simdLoad64(ptr, at+1).Or(caseBit)
		firstLo := first.And(nibble).AsInt8x64()
		firstHi := first.AsUint16x32().ShiftAllRight(4).AsUint8x64().And(nibble).AsInt8x64()
		secondLo := second.And(nibble).AsInt8x64()
		secondHi := second.AsUint16x32().ShiftAllRight(4).AsUint8x64().And(nibble).AsInt8x64()
		matches := firstLoTable.PermuteOrZeroGrouped(firstLo).And(firstHiTable.PermuteOrZeroGrouped(firstHi))
		matches = matches.And(secondLoTable.PermuteOrZeroGrouped(secondLo)).And(secondHiTable.PermuteOrZeroGrouped(secondHi))
		if mask := matches.NotEqual(zero).ToBits(); mask != 0 {
			return at + bits.TrailingZeros64(mask)
		}
		at += 64
	}
	return at
}

func asciiPairAnchorVBMISkip64(ptr *byte, n int, filter *asciiPairVBMIAnchorFilter) int {
	firstLo := simd.LoadUint8x64(&filter.firstLo)
	firstHi := simd.LoadUint8x64(&filter.firstHi)
	secondLo := simd.LoadUint8x64(&filter.secondLo)
	secondHi := simd.LoadUint8x64(&filter.secondHi)
	zero := simd.BroadcastUint8x64(0)
	at := 0
	for at+65 <= n {
		first := firstLo.ConcatPermute(firstHi, simdLoad64(ptr, at))
		second := secondLo.ConcatPermute(secondHi, simdLoad64(ptr, at+1))
		if mask := first.And(second).NotEqual(zero).ToBits(); mask != 0 {
			return at + bits.TrailingZeros64(mask)
		}
		at += 64
	}
	return at
}

func asciiPairDirectSkip64(ptr *byte, n int, probe *asciiPairProbe) int {
	first := simd.BroadcastUint8x64(probe.first)
	second := simd.BroadcastUint8x64(probe.second)
	firstFold := simd.BroadcastUint8x64(probe.firstFold)
	secondFold := simd.BroadcastUint8x64(probe.secondFold)
	at := 0
	for at+64 <= n {
		matches := simdLoad64(ptr, at).Or(firstFold).Equal(first)
		matches = matches.And(simdLoad64(ptr, at+probe.secondAt).Or(secondFold).Equal(second))
		if mask := matches.ToBits(); mask != 0 {
			return at + bits.TrailingZeros64(mask)
		}
		at += 64
	}
	return at
}

func asciiPairShortSkip64(ptr *byte, n int, probe *asciiPairProbe) int {
	first := simd.BroadcastUint8x64(probe.first)
	second := simd.BroadcastUint8x64(probe.second)
	firstFold := simd.BroadcastUint8x64(probe.firstFold)
	secondFold := simd.BroadcastUint8x64(probe.secondFold)
	at := 0
	for at+64 <= n {
		matches := simdLoad64(ptr, at).Or(firstFold).Equal(first)
		matches = matches.And(simdLoad64(ptr, at+probe.secondAt).Or(secondFold).Equal(second))
		if mask := matches.ToBits(); mask != 0 {
			return at + bits.TrailingZeros64(mask)
		}
		at += 64
	}
	return at
}

func probeVBMISkip64(ptr *byte, n int, probe *asciiVBMIProbe) int {
	firstTable := simd.LoadUint8x64(&probe.first)
	secondTable := simd.LoadUint8x64(&probe.second)
	thirdTable := simd.LoadUint8x64(&probe.third)
	zero := simd.BroadcastUint8x64(0)
	at := 0
	for at+64 <= n {
		matches := firstTable.Permute(simdLoad64(ptr, at+probe.firstAt))
		matches = matches.And(secondTable.Permute(simdLoad64(ptr, at+probe.secondAt)))
		matches = matches.And(thirdTable.Permute(simdLoad64(ptr, at+probe.thirdAt)))
		if mask := matches.NotEqual(zero).ToBits(); mask != 0 {
			return at + bits.TrailingZeros64(mask)
		}
		at += 64
	}
	return at
}

func asciiPairDirectVBMISkip64(ptr *byte, n int, probe *asciiPairVBMIProbe) int {
	firstTable := simd.LoadUint8x64(&probe.first)
	secondTable := simd.LoadUint8x64(&probe.second)
	zero := simd.BroadcastUint8x64(0)
	offset := int(probe.secondAt)
	at := 0
	for at+64 <= n {
		matches := firstTable.Permute(simdLoad64(ptr, at))
		matches = matches.And(secondTable.Permute(simdLoad64(ptr, at+offset)))
		if mask := matches.NotEqual(zero).ToBits(); mask != 0 {
			return at + bits.TrailingZeros64(mask)
		}
		at += 64
	}
	return at
}

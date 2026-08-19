// Package casei searches UTF-8 text case-insensitively under Unicode simple
// case folding, with allocation-free searches after plan compilation.
//
// It answers two shapes of the same question. IndexFold finds one needle;
// Matcher finds any of a pattern set and reports the leftmost match. Both run
// one compiled search plan, with runtime-gated AVX-512 and AVX2 block
// transitions on x86-64 and a portable path everywhere else, so results are
// identical on every machine while throughput is not.
//
// Reach for it when the alternative is lowercasing both sides and searching:
// that idiom allocates two copies, shifts byte offsets, and is not the same
// matching. See the semantics below.
//
//	if casei.ContainsFold(line, "payment declined") { ... }
//
//	m := casei.NewMatcher([]string{"fatal panic", "oom killed"})
//	if match, ok := m.Find(line); ok { use(match.Pattern, match.Start) }
//
// The repository around this package is also an open benchmark arena: its
// tests define these semantics, arena/ measures this engine against the
// competing implementations built from source, and CONTEXT.md catalogs the
// known techniques.
//
// Semantics — Unicode simple case folding over UTF-8:
//
//   - Two code points match when they belong to the same simple case-folding
//     orbit (unicode.SimpleFold). On valid UTF-8, this is exactly the matching
//     used by Go's regexp with (?i): 'k' matches 'K' and the Kelvin sign
//     U+212A; 's' matches 'S' and long s U+017F; σ, ς and Σ all match; ß
//     matches ẞ (U+1E9E) but NOT "ss" (no full folding); İ (U+0130) and ı
//     (U+0131) fold only to themselves (locale-independent).
//   - Matching is per code point, so a match window's byte length can differ
//     from the needle's ("kelvin" is 6 bytes but matches an 8-byte window
//     starting with U+212A). IndexFold returns the byte offset of the first
//     match start; match starts are haystack rune boundaries.
//   - Bytes that are not part of a valid UTF-8 encoding are opaque units:
//     they match only an opaque occurrence of the identical byte, never a
//     fragment of a valid encoding, and are never folded.
//   - ASCII consequences: only the 52 ASCII letters fold within ASCII; the
//     0x20-adjacent punctuation pairs ('[' vs '{', '@' vs '`', ']' vs '}',
//     '\' vs '|', '^' vs '~') never match.
package casei

// IndexFold returns the byte index of the first occurrence of needle in
// haystack under Unicode simple case folding, or -1 if needle is not present.
// It is the one-pattern instantiation of the same compiled plan used by
// Matcher.Find. An empty needle matches at index 0.
func IndexFold(haystack, needle string) int {
	match, ok := cachedSinglePlan(needle).find(haystack)
	if !ok {
		return -1
	}
	return match.Start
}

// ContainsFold reports whether needle occurs in haystack under Unicode simple
// case folding. It is IndexFold(haystack, needle) >= 0, and exists because it
// is the shape most callers want: the correct, cache-hit allocation-free
// replacement for
// strings.Contains(strings.ToLower(haystack), strings.ToLower(needle)).
func ContainsFold(haystack, needle string) bool {
	return IndexFold(haystack, needle) >= 0
}

// RuntimeVectorBits reports the widest runtime-gated block transition this
// package can dispatch on the running machine: 0 for the portable path, 256
// for AVX2, and 512 for AVX-512 BW. It reports a package path, rather than an
// advertised CPU feature, so the arena can place this engine beside the field
// under the same GODEBUG feature controls.
func RuntimeVectorBits() int { return runtimeVectorBits() }

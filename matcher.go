package casei

import "unicode/utf8"

// Matcher searches for any of a fixed set of patterns. NewMatcher uses Unicode
// simple folding, while NewExactMatcher compares raw bytes. IndexFold is the
// one-pattern form of the folded compiled search plan. Searches use the shared
// compiled automaton rather than running one independent search per pattern.
//
// Contract: Find returns the leftmost match by byte offset; ties at the
// same offset go to the lowest pattern index (regexp alternation order).
// An empty pattern matches at offset 0.

// Match identifies one pattern occurrence.
type Match struct {
	Pattern int // index into the pattern set
	Start   int // byte offset of the match start in the haystack
}

// Matcher searches for any of a fixed set of patterns. Construction compiles
// an immutable plan; Find returns one answer and Each enumerates non-overlapping
// answers over the haystack.
type Matcher struct {
	patterns       []string
	plan           *searchPlan
	asciiEachProbe *asciiProbe
}

// NewMatcher builds a Matcher over the given pattern set using Unicode simple
// folding. The set is copied; later mutation of the slice does not affect either
// the exposed pattern set or the compiled plan.
func NewMatcher(patterns []string) *Matcher {
	p := make([]string, len(patterns))
	copy(p, patterns)
	plan := newSearchPlan(p)
	m := &Matcher{patterns: p, plan: plan}
	if len(p) == 1 {
		m.asciiEachProbe = plan.makeASCIIEachProbe(p[0])
	}
	return m
}

// NewExactMatcher builds a Matcher over the given pattern set using raw byte
// equality. Matching is case-sensitive and may start or end at any byte offset,
// including within a valid UTF-8 encoding. Find returns the earliest byte start,
// with ties resolved by the lowest pattern index. Each consumes the matched
// pattern's byte length; after an empty result it advances one byte through EOF.
// The set is copied; later mutation of the slice does not affect the exposed
// patterns or compiled plan.
func NewExactMatcher(patterns []string) *Matcher {
	p := make([]string, len(patterns))
	copy(p, patterns)
	return &Matcher{patterns: p, plan: newSearchPlan(p, true)}
}

// Patterns returns a copy of the pattern set.
func (m *Matcher) Patterns() []string {
	patterns := make([]string, len(m.patterns))
	copy(patterns, m.patterns)
	return patterns
}

// Find returns the leftmost match across the pattern set, or ok=false when
// no pattern occurs.
func (m *Matcher) Find(haystack string) (Match, bool) {
	if m == nil || m.plan == nil {
		return Match{}, false
	}
	return m.plan.find(haystack)
}

// Each calls yield for each non-overlapping match in haystack, in the same
// leftmost and lowest-pattern-ID order as repeated calls to Find. width is the
// exact byte width consumed by this occurrence. Under Unicode simple folding it
// can differ from the pattern's byte length; an exact matcher consumes the
// pattern's byte length. After an empty result, folded matchers advance by one
// UTF-8 unit and exact matchers by one byte, until EOF. Returning false from
// yield stops enumeration and makes Each return false.
//
// A nil Matcher or nil yield has no matches and returns true. Each is safe for
// concurrent use when yield itself is safe.
func (m *Matcher) Each(haystack string, yield func(match Match, width int) bool) bool {
	if m == nil || m.plan == nil || yield == nil {
		return true
	}
	if !m.plan.exact {
		if m.plan.empty < 0 && m.plan.rawByteMulti.usable() {
			return m.plan.eachRawByteFixedAnchored(haystack, yield)
		}
		if bucket, ok := m.plan.rootBucketEachFilter(haystack); ok {
			return m.plan.eachRootBucket(haystack, bucket, yield)
		}
		if m.plan.patternCount == 1 && m.plan.maxUnits > 0 && len(haystack) >= 4096 &&
			!m.plan.opaqueContinuation && !m.plan.asciiRun && !m.plan.asciiPair.usable() &&
			!m.plan.asciiStaticAnchor && !m.plan.asciiByteAnchor && m.plan.asciiProbe.usable() {
			return m.eachASCIIProbe(haystack, yield)
		}
	}
	for at := 0; at <= len(haystack); {
		match, width, ok := m.plan.findWithWidth(haystack[at:])
		if !ok {
			return true
		}
		match.Start += at
		if width == 0 {
			if m.plan.exact {
				width = len(m.patterns[match.Pattern])
			} else {
				units := utf8.RuneCountInString(m.patterns[match.Pattern])
				width = matcherMatchEnd(haystack, match.Start, units) - match.Start
			}
		}
		end := match.Start + width
		if !yield(match, width) {
			return false
		}
		if width != 0 {
			at = end
			continue
		}
		if match.Start == len(haystack) {
			return true
		}
		if m.plan.exact {
			at = match.Start + 1
		} else {
			_, size := utf8.DecodeRuneInString(haystack[match.Start:])
			at = match.Start + size
		}
	}
	return true
}

// eachASCIIProbe keeps the selected single-pattern probe and cursor alive across
// yields. The probe only rejects impossible starts; the compiled plan still
// confirms every survivor and determines its exact source width.
func (m *Matcher) eachASCIIProbe(haystack string, yield func(Match, int) bool) bool {
	p := m.plan
	probe := &p.asciiProbe
	if m.asciiEachProbe != nil {
		probe = m.asciiEachProbe
	}
	limit := len(haystack) - len(p.asciiNeedle) + 1
	if p.asciiVerifyTokens {
		limit = len(haystack) - probe.thirdAt
	}
	if limit < 0 {
		return true
	}

	nextStart := 0
	for at := 0; at < limit; {
		at += probeSkipBytes(haystack, at, limit-at, probe)
		if at == limit {
			break
		}
		start := at
		if probe.firstAt != 0 {
			if !asciiProbeAt(haystack, at, probe) {
				at++
				continue
			}
			start = recoverASCIIInteriorStart(haystack, at+probe.firstAt, probe.firstAt)
			if start < 0 {
				at++
				continue
			}
		}
		if start < nextStart {
			at++
			continue
		}
		width := len(p.asciiNeedle)
		if p.asciiVerifyTokens {
			asciiMatch := false
			if p.asciiOnly {
				width, asciiMatch = p.asciiOnlyMatchWidth(haystack, start)
			}
			if !asciiMatch {
				if !p.asciiAnchorMatches(haystack, start) {
					at++
					continue
				}
				width = matcherMatchEnd(haystack, start, p.maxUnits) - start
			}
		} else if !p.asciiAnchorMatches(haystack, start) {
			at++
			continue
		}
		if !yield(Match{Pattern: 0, Start: start}, width) {
			return false
		}
		nextStart = start + width
		at = nextStart
	}
	return true
}

// asciiOnlyMatchWidth accepts only a complete ASCII rendering of a token-verified
// ASCII singleton. A failed check is inconclusive, so Each keeps the decoded
// confirmer and endpoint recovery for every other candidate.
func (p *searchPlan) asciiOnlyMatchWidth(haystack string, at int) (int, bool) {
	if !p.asciiVerifyTokens || !p.asciiOnly || at < 0 || at > len(haystack) {
		return 0, false
	}
	width := len(p.asciiNeedle)
	if width > len(haystack)-at || !p.asciiOnlyPatternAt(haystack, at, p.asciiNeedle) {
		return 0, false
	}
	return width, true
}

func matcherMatchEnd(haystack string, start, units int) int {
	at := start
	for range units {
		_, size := utf8.DecodeRuneInString(haystack[at:])
		at += size
	}
	return at
}

// VectorBits reports the widest runtime-gated block transition available to
// this package, with the same contract as RuntimeVectorBits.
func (m *Matcher) VectorBits() int { return RuntimeVectorBits() }

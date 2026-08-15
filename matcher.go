package casei

// Matcher is the multi-needle face of the arena: search for any of a set of
// patterns under the same Unicode simple-fold semantics as IndexFold.
// IndexFold is the N=1 special case. This API and the tests define the
// contract: per-position sets of UTF-8 encodings under simple folding
// (elastic-degenerate byte patterns), exact search as the singleton case,
// multi-needle as the union, one anchoring/verification theory, linear worst
// case.
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
// their shared fold-orbit transition plan; Find only advances that plan over
// the haystack.
type Matcher struct {
	patterns []string
	plan     *searchPlan
}

// NewMatcher builds a Matcher over the given pattern set. The set is copied;
// later mutation of the slice does not affect either the exposed pattern set
// or the compiled plan.
func NewMatcher(patterns []string) *Matcher {
	p := make([]string, len(patterns))
	copy(p, patterns)
	return &Matcher{patterns: p, plan: newSearchPlan(p)}
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

// VectorBits reports the widest runtime-gated block transition available to
// this package, with the same contract as RuntimeVectorBits.
func (m *Matcher) VectorBits() int { return RuntimeVectorBits() }

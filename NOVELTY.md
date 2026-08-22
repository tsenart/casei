# Novelty assessment: fold-orbit alphabet

## Status

**Negative assessment for the initially proposed construction.**  Compiling the
simple-fold orbits used by a pattern into class identifiers and scanning those
identifiers is useful engineering to evaluate, but it is not claimed here as a
new search construction.  It is the quotient-alphabet form of canonical case
folding / case-expanded matching.  No performance or novelty claim should be
made for that construction unless a later state transition is identified that
is not equivalent to this quotient.

## How to read a negative assessment

A negative novelty result closes a claim that a state or transition is new. It
does **not** forbid using the technique. A known component may be implemented,
adapted, and combined with other known components when that combination is
aimed at a result the field does not hold. What does not qualify is a port,
wrapper, or repacking that produces no new measured position or capability.

Accordingly, the decisions below close standalone novelty claims. They remain
eligible engineering ingredients under the acceptance bar in `AGENTS.md`.

## Precise construction assessed

For each decoded valid rune `r`, let `q(r)` be the identifier of its
`unicode.SimpleFold` orbit.  A compiler may restrict the assigned identifiers
to orbits occurring in the pattern set: an encountered rune from every other
orbit produces a distinguished mismatch token.  Invalid UTF-8 bytes remain
individual opaque tokens, rather than entering an orbit.  Each pattern is then
a sequence of tokens `q(r)` (or opaque-byte tokens).  A scan decodes the
haystack once, maps each decoded unit through `q`, and advances a matcher over
that token sequence.  For ASCII this can be represented by a 256-entry table;
KELVIN SIGN, LONG S, sigma, and other non-ASCII members require decoded-rune
escape handling.  A multi-pattern form shares the same `q` over the union of
pattern orbits.

The intended difference from the reference is operational: a candidate does
not call `foldHasPrefix` and re-enumerate a fold orbit at each candidate start.
It compares already-classified symbols instead.  This document's conclusion is
that this operational difference does **not** create a distinct state
representation: `q` is exactly canonical simple folding, with the canonical
rune replaced by an arbitrary dense class number.

## Closest known constructions

| Construction | Source | Relation to the assessed plan |
| --- | --- | --- |
| Case-expanded literal automata and UTF-8 automata | UTS #18; RE2 (`github.com/google/re2`); rust-regex / regex-automata (`github.com/rust-lang/regex`); summarized in `CONTEXT.md` §1b | Expanding every fold orbit at a pattern position and determinizing recognizes the same language as equality after `q`.  Quotienting equal alternatives into one class is the standard equivalent representation. |
| Teddy and FDR literal engines | Hyperscan, NSDI 2019; aho-corasick Teddy documentation; `CONTEXT.md` §§1b, 1d, 3 | Their byte/nibble masks encode finite sets of accepted byte forms before a confirmation stage.  A class table changes mask representation and when classification occurs, not the accepted transition relation. |
| `veloz` ASCII prefilter plus `EqualFold` confirmation | `github.com/mhr3/veloz`, including its AVX2 lineage; `CONTEXT.md` §§1, 2, 3, 5 | `veloz` keeps folding at candidate confirmation; the assessed plan moves it to stream classification.  That is a cost placement difference, not a new language or state machine. |
| Pre-folded corpus plus exact search | `arena/bench_test.go` (`ceiling`); Unicode `CaseFolding.txt`; `CONTEXT.md` §§1b, 9 | An eagerly materialized canonical stream is `q(haystack)` with canonical runes instead of dense IDs.  An online table/escape implementation is the same stream without storing it. |
| Native fold-set byte structures | ClickHouse MultiVolnitskyCaseInsensitiveUTF8 and Quamina, cited in `CONTEXT.md` §1d | These establish that deriving byte-level structures from case-fold data is not new.  They differ in contract or scope, but neither difference turns a direct orbit quotient into a new construction. |
| Standard dictionary matching over a relabeled alphabet | Aho--Corasick (1975), cited in `CONTEXT.md` §1d | Applying a trie/DFA/AC transition to `q(patterns)` is ordinary dictionary matching after a homomorphic relabeling.  It would also be an alternate Aho--Corasick engine, which the repository rules prohibit as a runtime escape hatch. |

## Why it is an equivalent repacking

Simple-fold equality is an equivalence relation on valid runes.  For valid
runes `a` and `b`,

```
a fold-equals b  if and only if  q(a) == q(b).
```

The map is therefore a lossless quotient for this matching predicate.  Replacing
one position's finite set of case encodings with one class token does not add a
transition, remove a transition, or encode any information unavailable to a
case-expanded automaton.  UTF-8 width changes only affect decoding and the
mapping from token index back to byte offset; they do not change the quotient
identity.  Keeping invalid bytes as singleton tokens likewise matches the
existing opaque-byte rule exactly.

For a single pattern, exact matching of `q(pattern)` in `q(haystack)` is
canonical-fold-then-search.  For multiple patterns, a shared alphabet followed
by a dictionary automaton is canonical-fold-then-dictionary-search.  A 256-byte
ASCII table, vector lookup, sparse orbit table, or a non-ASCII escape path
changes storage and cost, not that equivalence.  “Never refolds during
verification” is consequently the online/precomputed-folding tradeoff already
represented by the arena ceiling, not a new matcher construction.

## Falsification search and result

This assessment would be falsified by a construction whose runtime state
cannot be reduced to a token from the fold-equivalence quotient followed by an
ordinary literal/dictionary transition.  Examples of potentially distinct
claims would be a block transition that jointly preserves variable UTF-8 width,
leftmost byte offsets, and multiple pattern states without materializing or
serially feeding quotient tokens, together with a proof that no
case-expanded/canonical-stream automaton has the same transition relation; or
a published implementation/paper that explicitly describes this exact quotient
plan as a named construction.

The repository's prior-art review was checked before this document was added:
`CONTEXT.md` §§1b--1d explicitly catalogs case-expanded UTF-8 automata,
pre-folded streams, native fold-set structures, and multi-pattern automata.
Those sources already account for every component of the assessed plan.  The
result is that the proposed orbit-class table is classified as engineering, not
an invention.  No external browsing result is being asserted beyond the
sources cited above.

## Follow-up assessment: direct raw-byte fold transitions

### Status

**Negative assessment for raw UTF-8 byte accept masks with partial-width
matcher state.**  The proposed scan can avoid constructing a Go `rune` or a
canonical-fold token, but its states and transitions are exactly a compressed
case-expanded UTF-8 byte automaton.  It is therefore a different operational
placement of decoding work, not a distinct search construction. This closes
the novelty claim; it does not decide whether the known construction can help a
combination reach a new result.

### Construction assessed

Let `E(a)` be the set of UTF-8 byte strings for all members of the simple-fold
orbit of a valid pattern rune `a`.  For an opaque invalid pattern byte `b`,
let `E(b) = {b}`.  For pattern units `a₁ … aₘ`, the proposed raw-byte matcher
accepts the concatenation language

```
E(a₁) E(a₂) … E(aₘ).
```

Its proposed representation assigns each runtime state a raw-byte accept mask
and carries a state for a prefix of a multi-byte alternative already seen.  A
`k` position, for example, accepts the one-byte alternatives `k` and `K` and
the three-byte alternative `e2 84 aa` for KELVIN SIGN.  A `s` position adds
`c5 bf` for LONG S.  A sigma position has the three two-byte alternatives
`ce b2`, `ce b3`, and `ce a3`.  The partially consumed `e2`, `e2 84`, `c5`,
and `ce` prefixes are the proposed extra states.

The arbitrary-byte contract requires one more component.  A raw scanner must
not let an opaque-byte pattern `84` match the middle byte of `e2 84 aa`.
Consequently it must carry the standard finite UTF-8 lexical/boundary state,
including invalid-sequence error behavior, or an equivalent delayed-byte
mechanism.  Otherwise it fails the existing `lone continuation vs kelvin
bytes` trap.  This may avoid a call to `utf8.DecodeRuneInString`, but it is
still the UTF-8 prefix automaton embedded in the matcher state.

### Constructive reduction to the known construction

For every byte string in `E(a)`, draw a byte-labelled path from the state
before pattern position `a` to the state after it.  The nodes after each
proper prefix of a multi-byte string are precisely the partial-width states
above.  Product that graph with the UTF-8 lexical/boundary automaton, label
its terminals with pattern indices, and carry each active path's native byte
start alongside the ordinary search state.  Selecting the leftmost start and
then the lowest terminal pattern index gives exactly the repository contract,
including opaque bytes.

A per-state byte mask is merely a packed encoding of the outgoing labelled
edges in that graph.  Expanding a mask into one edge per accepted byte
recovers the case-expanded byte graph; packing equal edge sets into masks
recovers the proposal.  A bitset of simultaneously active states is likewise
the standard subset representation of that graph.  For a pattern set, union
all pattern graphs and use the same construction; the multi-pattern state is
still a state or subset of the same expanded byte automaton.

Thus width changes add ordinary prefix paths, not a new transition relation.
No normalized stream or `q` token has to be materialized for this equivalence:
the omission changes storage and scheduling only.  A block implementation
would precompute several transitions of this same graph.  It would remain an
acceleration of the graph unless it supplies a transition not obtainable by
mask packing and byte-edge expansion.

### Closest known constructions

| Construction | Source | Relation to the assessed plan |
| --- | --- | --- |
| Case-expanded literal and UTF-8 automata | UTS #18; RE2; rust-regex / regex-automata; `CONTEXT.md` §1b | Each member of `E(a)` is an ordinary UTF-8 alternative.  The path expansion above is their literal construction. |
| Native byte-level fold-set structures | ClickHouse `MultiVolnitskyCaseInsensitiveUTF8` and Quamina; `CONTEXT.md` §1d | These already derive byte structures from fold alternatives.  The proposal changes neither their byte-language basis nor the meaning of an intermediate encoding-prefix state. |
| Masked SIMD literal engines | Teddy and FDR / Hyperscan; `CONTEXT.md` §§1d and 3 | Byte or nibble masks are a compact representation of accepted outgoing alternatives; they do not create a different accepted transition relation. |
| Canonical-fold quotient assessment above | This file; `CONTEXT.md` §1b | The earlier rejected plan materializes a quotient token.  This plan does not materialize one, but directly expands the same fold sets into UTF-8 byte paths, which is the other already-catalogued representation. |

### Falsification search and result

The negative would be falsified by a precise raw-byte state whose update and
outputs cannot be obtained from the expanded byte paths plus the UTF-8
boundary state — not merely by avoiding a `rune` allocation, packing masks
differently, or precomputing a block transition.  In particular, it would
need a proof that expanding every mask into byte edges fails to preserve an
accepted match, its byte start, or its selected pattern index.

Applying that test to the proposed state gives the opposite result: every
partial multi-byte-progress state is a proper-prefix node of an `E(a)` path,
every accept mask expands to its outgoing byte edges, and the required
boundary state is the ordinary UTF-8 lexical product.  KELVIN SIGN, LONG S,
and the sigma trio exercise the different-width and shared-prefix paths rather
than breaking the reduction.  The proposed mechanism is consequently a
relabeled/compressed case-expanded byte automaton, the closed outcome named in
the case hypothesis.

### Decision

This is a documented negative novelty finding. A direct reimplementation with
no new result would not qualify, but the byte-transition technique remains
available as one component of a combination tested against the full field.

## Follow-up assessment: fixed-width lossy projection with survivor verification

### Status

**Negative assessment for the proposed `index_fold` projection scan.**  The
published `casefold` crate already describes this exact one-byte-per-character
projection as a lossy case-insensitive index/hash key and explicitly prescribes
using it as a candidate filter followed by verification against the original
text.  Generating that same key stream online, scanning it in fixed-width SIMD
blocks, and verifying survivors changes when the key is stored and compared;
it does not identify a new search state or transition.

### Construction assessed

Let `p` be `casefold::index_fold_char` on each valid Unicode character: ASCII
is simple-folded to an ASCII byte and every non-ASCII character becomes
`0x80 | (simple_fold(r) & 0x7f)`.  A Go implementation would additionally
need an opaque-unit rule for invalid UTF-8 so that a candidate cannot begin in
the middle of a valid encoding.  That contract detail is necessary for this
repository, but does not change the projection/filter construction.

The assessed plan projects the needle once, emits `p` for haystack units in
source order, and searches the projected sequence for the projected needle.
A projected equality is only a survivor: it recovers its original byte start
with either a running unit-to-byte cursor or a replay/offset map, then calls a
true simple-fold verifier.  Fixed-stride SIMD scanning is only a batched way
to evaluate the projected equality.  The projection has no false negatives
for a fold-equal rendering, but its seven-bit non-ASCII payload admits false
positives and the verifier decides the match.

### Closest known constructions

| Construction | Source | Relation to the assessed plan |
| --- | --- | --- |
| `casefold::index_fold` | GitHub [`rust-gems` `casefold` v0.1.0 `index_fold.rs`](https://github.com/github/rust-gems/blob/d08a649afb47f1ec303f30ec3d062444291b5ec3/crates/casefold/src/index_fold.rs) and [crate README](https://github.com/github/rust-gems/blob/d08a649afb47f1ec303f30ec3d062444291b5ec3/crates/casefold/README.md#single-byte-index-fold) | The implementation defines the same ASCII/high-bit projection.  Its docs call the result a fixed-width key for indexing or hashing with acceptable collisions; the README says to use it as a candidate filter with no false negatives and verify exact hits against original text. |
| Case-insensitive n-gram indexing | The same `casefold` README, “Why one byte per character?” | A stored projected document and an online projected scan differ only in persistence and lookup scheduling.  Both compare the same projected k-character key and verify original-text candidates. |
| SIMD candidate/confirm literal engines | `CONTEXT.md` §§1d and 3--5; Teddy/FDR/Snort sources listed in `CONTEXT.md` §10 | The two-stage SIMD-candidate-plus-verifier shape is explicitly pre-conceded known art.  Replacing a byte/nibble fingerprint with this published character projection does not create a new confirmation transition. |
| Source-position recovery after a reduced-space search | `CONTEXT.md` §1b (alphabet sampling with position mapping; rust/regex reverse re-scan) | Carrying byte offsets beside projected units is a position map; replaying after a rare survivor is re-scan recovery.  They select the same source start for the same projected hit. |

### Why the online scan is an equivalent repacking

For a valid haystack decoded into units `h₀, h₁, …`, materializing
`p(h₀)p(h₁)…` and searching that byte string produces exactly the same survivor
unit indexes as emitting `p` in a streaming loop and comparing blocks as they
arrive.  Keeping a byte cursor merely decorates each emitted unit with its
source coordinate; replaying derives that coordinate later.  Neither changes
which projected windows equal `p(needle)`.

At a survivor, the proposed true-fold verifier is the only operation that
separates a collision from a match.  Thus the complete accepted predicate is
“projected-key equality, then the existing exact predicate,” which is the
filter/verify pipeline the crate documents for its n-gram key.  SIMD width,
projected-block layout, a rare-survivor replay threshold, and collision density
can affect cost, but not the representation or accepted language.  In
particular, offset recovery is not a new block transition: it is bookkeeping
outside the projected comparison and verifier.

This also cannot reopen either closed assessment above.  Unlike the lossless
fold-orbit quotient and raw-byte fold automaton, this plan deliberately loses
information before searching; its only way back to correct semantics is the
known confirmation stage, not a new recognizer for the fold language.

### Falsification search and result

This negative would be falsified by a source showing that `index_fold` is not
usable as a lossy candidate filter followed by original-text verification, or
by a precise online state/update whose outputs cannot be reproduced from the
projected stream, an ordinary projected-string search, a source-position map
(or replay), and the same verifier.  A faster SIMD implementation, a lower
collision rate on a corpus, or a different replay threshold would not falsify
that reduction.

The `casefold` source and README were read at commit
`d08a649afb47f1ec303f30ec3d062444291b5ec3`; its `Cargo.toml` identifies the
crate as version `0.1.0`.  They give the opposite of the first falsifier: the
projection is documented as a collision-tolerant index/hash key, and the README
explicitly names candidate filtering followed by exact verification.  The
repository's existing survey independently places two-stage candidate/verify
engines and reduced-space offset recovery in known art.  No distinct state or
transition remained after that comparison.

### Decision

This is a documented negative novelty finding. A Go port or SIMD scan would
not become novel by measuring its collision rate, but the projection may still
be used in a broader combination if that combination is measured against the
field and reaches a result no source holds alone. No performance claim is made
here.

## Follow-up assessment: raw-byte rolling fingerprint with run-membership correction

### Status

**Negative assessment for a fold-invariant raw-byte rolling fingerprint.**
The required algebra does not yield a stable folded window from a raw byte
window plus a cheap run summary.  In the width-preserving ASCII subset, the
correction is a second, position-weighted hash of case-run membership and must
be updated for every entering and leaving byte.  For general Unicode simple
folding, width-changing members make raw and folded window boundaries
variable; exact updates additionally need each unit's folded length, folded
byte contribution, and UTF-8 boundary status.  Keeping that state only in
registers avoids an allocation, but it is still online folding inside the
hash, not recovery from a raw-byte fingerprint without folding.

This is an algebraic negative result.  No SIMD implementation, benchmark, or
performance claim follows from it.

### Window semantics come before a rolling hash

Let `F` map each valid UTF-8 unit to one deterministic simple-fold
representative and leave an invalid byte as its own opaque one-byte unit.  The
particular representative is immaterial to the argument.  The `casefold`
run-table direction recorded in `CONTEXT.md` §1e gives concrete examples:

```
E2 84 AA       (U+212A KELVIN SIGN)  ->  6B       (k)
C8 BA          (U+023A)              ->  E2 B1 A5 (U+2C65)
```

The repository's `orbitMin` reference can choose the opposite representative
for an orbit, but the two members still have unequal input byte widths.  Thus
for a folded one-character needle such as `k`, valid source windows include
`6B`, `4B`, and `E2 84 AA`.  No fixed raw-byte window length contains all
three.  Conversely, the U+023A example shows that a fold can grow from two
source bytes to three folded bytes.

A correct search window must therefore be a sequence of decoded units whose
*folded* byte length equals the needle's folded length, with a source byte
start retained for that sequence.  Moving either endpoint requires knowing
unit boundaries and each unit's folded length.  Choosing a window by raw byte
length instead misses one of the legal renderings; choosing it by character
count is a decoded-unit stream rather than a raw-byte window.

### The favorable fixed-width algebra still needs a second classified stream

Use the usual order-sensitive linear polynomial hash over bytes, with base
`B` (byte literals below are hexadecimal):

```
H(b0 ... b(L-1)) = sum(bj * B^(L-1-j))
H(x || y) = H(x) * B^len(y) + H(y)
```

Grant the most favorable case: an ASCII window has no width changes and the
only transform is `A` through `Z` gaining `0x20`.  Put
`uj = 1` when byte `bj` is an uppercase ASCII letter and zero otherwise.  Then

```
H(F(w)) = H(w) + 0x20 * U(w)
U(w)    = sum(uj * B^(L-1-j))
```

`U`, not the number of uppercase bytes, is the required correction.  For
example, `Aa` and `aA` have the same uppercase count but corrections
`0x20 * B` and `0x20`.  The rolling update for `U` is the same kind of update
as the raw hash:

```
U(i+1) = B * (U(i) - ui * B^(L-1)) + u(i+L)
```

So this restricted case is recoverable only by maintaining a second rolling
hash of a separately classified membership stream.  Each entering byte must
be tested for membership (and each leaving byte's membership must be retained
or recomputed).  A base of one would reduce the correction to a count, but it
also discards byte order and is not an equality fingerprint for substring
search.  The `casefold` run table can compress the membership lookup; it
cannot remove the position-weighted membership state.

For non-ASCII units, the same equation needs a run-specific output-byte
contribution instead of `0x20`, and membership is not byte-local.  A valid
UTF-8 unit must first be recognized and assigned to its run before that
contribution is known.  This is the folding/classification work that the
hypothesis was required to avoid, merely with the transformed bytes fed to a
hash accumulator instead of being stored.

### Width changes break raw-window recovery

For source units `u0 ... u(m-1)`, the desired hash is

```
H(F(u0 ... u(m-1))) =
  sum(H(F(ui)) * B^(sum(len(F(ut)) for t > i)))
```

The raw hash uses `len(ut)` in those exponents instead.  A width-changing
unit changes the weight of every earlier unit, so a per-run packed-word delta
cannot be added to `H(raw)` at a fixed position.  With the Kelvin mapping,
for example,

```
H(61 E2 84 AA) = 61*B^3 + E2*B^2 + 84*B + AA
H(F(61 E2 84 AA)) = H(61 6B) = 61*B + 6B
```

The `61` term changes exponent because the following source unit shrank by
two bytes.  U+023A supplies the opposite direction:

```
H(C8 BA) = C8*B + BA
H(F(C8 BA)) = E2*B^2 + B1*B + A5
```

Knowing only a total width delta or a run count cannot repair these changing
exponents.  It would require the folded-length prefix/suffix position of each
unit and its folded-byte hash.  A state that stores and combines those values
is directly maintaining `H(F(window))`; its update applies the fold transition
for every unit.  It is not a raw-hash correction that escapes folding.

Opaque bytes add a separate necessary state.  `84` alone is an invalid opaque
unit and folds to `84`, whereas the same byte in `E2 84 AA` is a continuation
inside a valid Kelvin-sign unit that folds to `6B`.  A byte-window hash cannot
let a candidate begin at that continuation.  Correct membership therefore
also requires the UTF-8 lexical/boundary state exercised by the existing
`lone continuation vs kelvin bytes` trap in `casei_test.go`.

### A fixed character-lane hash is not raw-byte recovery

There is a valid escape from the *fixed raw-byte window*: pad each decoded
unit to a fixed-width lane, use the run table's delta on that lane, and roll a
hash over the needle's number of units.  Width changes no longer move lane
positions.  But this state must determine every unit's UTF-8 boundary,
opaque-versus-valid status, packed value, and run delta before it can update
the lane hash.  It hashes a folded unit sequence in a different fixed-width
encoding, not `H` of a raw byte window plus an aggregate correction.

Avoiding storage for those lanes does not alter that distinction.  The update
is still an online fold transition for every unit, followed by a hash update;
source offsets must still be carried beside the unit window.  This is the
fold-inside-the-hash fallback described below, not a falsifier of the requested
raw-byte algebra.

### Relation to existing work

`CONTEXT.md` §1e is the source for the run-table arithmetic and the two
width-changing examples above.  Its packed-`u32` relation can supply a
per-unit output value, but it does not make concatenation positions
width-invariant.  `casei.go`, `casei_test.go`, and `CONTEXT.md` §1b specify the
required source-boundary and opaque-byte behavior.

`CONTEXT.md` §6 already lists the viable fallback operation: fold each input
unit in the hashing loop and hash in the folded domain.  Keeping only the hash
rather than materializing the folded bytes does not change that operation.
This conclusion does not reopen the closed orbit-quotient, raw-byte automaton,
or `index_fold` assessments above: it rejects the proposed raw-hash algebra
before relying on any projected-stream reduction.

### Falsification search and result

This negative would be falsified by a precise order-sensitive linear hash and
rolling update that produces the exact folded-window hash for all legal source
windows while avoiding all three requirements: position-weighted run
membership, folded-length/output-coordinate state at width changes, and UTF-8
unit/boundary classification.  It would need to handle `k`/`K`/KELVIN SIGN,
U+023A/U+2C65, and an isolated `0x84` without changing the returned source
byte position.  Merely vectorizing membership tests, storing an implicit
folded hash, or verifying hash collisions after a fold-aware update would not
falsify the result; each retains the work identified by the equations.

Applying that test gives the negative result.  The fixed-width formula demands
`U`; width changes demand output-coordinate state; and invalid-byte opacity
demands UTF-8 lexical state.  No raw-byte rolling fingerprint satisfying the
hypothesis remains to implement.

### Decision

This is a documented negative novelty finding. A SIMD/hash implementation that
fails the window semantics is unusable; one that reduces to folding inside a
rolling hash is known art. The latter may still be an ingredient in a
combination that reaches a new result. No performance claim is made here.

## Follow-up assessment: prefix-invariant anchor with arithmetic start recovery

### Status

**Negative assessment for a width-invariant-prefix anchor.**  This would move
work out of the byte scan: choose a pattern position after a fixed-width
prefix, scan for that position's byte forms, subtract the prefix width to get
a proposed start, and confirm the whole pattern.  It is not a new matcher
state.  It is a specialization of the published safe-slice-at-an-offset plus
head/tail confirmation pipeline, combined with the already-rejected raw-byte
fold alternatives.  The subtraction partially evaluates known start recovery;
it does not add a transition or remove the required confirmation. That closes
the novelty claim, not its possible use in a measured combination.

### Construction assessed

Decode the needle once into simple-fold units.  Call a valid unit
*width-invariant* when every member of its `unicode.SimpleFold` orbit has the
same UTF-8 byte width; an opaque invalid byte is a singleton width-one unit.
Choose an anchor unit `a_j` after a width-invariant prefix
`a_0 ... a_{j-1}`, and let

```
L = sum(encodedWidth(a_i), 0 <= i < j).
```

A scan searches the haystack bytes for any member of the anchor's finite UTF-8
form set `E(a_j)`.  For a hit whose anchor bytes begin at `t`, it proposes
`start = t - L`, then validates the prefix, anchor, and suffix under simple
folding and returns the leftmost validated start.  An implementation may use
SIMD to find `E(a_j)` and may pre-expand a small set of anchor forms.  It must
still reject a byte hit in the middle of a valid UTF-8 encoding, and must treat
an invalid byte as an opaque unit.

For example, a prefix ending before `k` has a known byte width when none of
its units has a cross-width fold mate.  The anchor can then admit `k`, `K`, and
`E2 84 AA` (KELVIN SIGN); a hit at `t` implies only a *candidate* start
`t - L`.  It does not prove that the preceding bytes render the prefix, nor
that `t` is a legal unit boundary.  Those facts remain confirmation work.

The intended operational benefit is real but narrow: the streaming scan need
not call `utf8.DecodeRuneInString` or walk a `SimpleFold` orbit at every byte
position.  The novelty question is whether the resulting candidate state is
more than a known safe window and verifier.

### Current-art check

| Construction | Source | Relation to the assessed plan |
| --- | --- | --- |
| Safe folded slice at an arbitrary needle offset, SIMD probes, and head/tail verification | StringZilla current `main`, inspected at `657f21c5d8c2c2da5da06d4a9ad87c3ef80953d0`: [`utf8_uncased.h`](https://raw.githubusercontent.com/ashvardanian/stringzilla/657f21c5d8c2c2da5da06d4a9ad87c3ef80953d0/include/stringzilla/utf8_uncased.h), [`serial.h`](https://raw.githubusercontent.com/ashvardanian/stringzilla/657f21c5d8c2c2da5da06d4a9ad87c3ef80953d0/include/stringzilla/utf8_uncased/serial.h), [`haswell.h`](https://raw.githubusercontent.com/ashvardanian/stringzilla/657f21c5d8c2c2da5da06d4a9ad87c3ef80953d0/include/stringzilla/utf8_uncased/haswell.h) | Its metadata builder considers every rune-boundary slice, records `offset_in_unfolded` and `length_in_unfolded`, and its SIMD driver scans that selected safe slice.  `sz_utf8_uncased_verify_match_` reverse-verifies the head, forward-verifies the tail, and returns the candidate-window offset minus the verified head width.  The assessed subtraction is this general recovery specialized to a head whose accepted rendering width is `L`. |
| Width-preserving case-insensitive prefix acceleration | PCRE2 [`pcre2_jit_compile.c`](https://raw.githubusercontent.com/PCRE2Project/pcre2/master/src/pcre2_jit_compile.c), `scan_prefix` | For a UTF-8 caseless character, `scan_prefix` stops growing its prefix when `ord2utf(othercase) != len`.  Thus the width-invariance condition is an established acceleration boundary.  Moving the probe to the next character changes the candidate coordinate, not the finite byte alternatives or confirmation relation. |
| Raw UTF-8 form paths and boundary state | “Follow-up assessment: direct raw-byte fold transitions” above | Expanding `E(a_j)` into one-byte and multi-byte alternatives, plus rejecting mid-rune starts, is exactly the byte-path construction already reduced here to a case-expanded UTF-8 automaton. |
| SIMD candidate plus full confirmation | StringZilla sources above; Teddy/FDR/Snort sources cataloged in `CONTEXT.md` §§1b--1d and 3 | A fixed byte displacement before confirmation is ordinary candidate bookkeeping.  It does not make a filter/verify pipeline a new recognizer. |

StringZilla has full-fold rather than this repository's simple-fold contract,
so it is not a drop-in implementation and is not cited as one.  That semantic
difference does not make this state new: safe-slice selection at a pattern
offset, a SIMD candidate scan, and recovery/verification around that slice are
already used for the harder expansion and variable-width setting.  Restricting
those mechanics to simple-fold orbits removes cases; it does not introduce a
new transition.

### Reduction

Take each anchor form in `E(a_j)` and draw its ordinary byte-labelled path.
The scan's output is a pair `(t, form)`.  Because the prefix is
width-invariant, translating that pair to `t - L` is a fixed coordinate change;
it carries no information about whether the prefix actually matches.  The
validator decides precisely that remaining predicate, along with UTF-8
boundary validity, anchor equality, and the suffix.  Replacing an SIMD probe
with a scan of the expanded byte paths produces the same candidate pairs;
replacing `t - L` with StringZilla's general reverse-head recovery produces
the same start on every accepted prefix, since its consumed head length is
then exactly `L`.

Skipping confirmation is unsound: any occurrence of the anchor form can be
preceded by a nonmatching prefix, and raw matching can find a continuation-byte
location.  Pre-expanding and comparing the prefix instead merely moves the
same finite byte paths into the candidate filter.  It is the closed raw-byte
transition construction, not a new state.  A multi-pattern version only adds
`(pattern, anchor, L)` labels to those candidates; merging them in a
dictionary state is ordinary case-expanded dictionary matching, while keeping
per-pattern confirmations is the known candidate/confirm shape and does not
meet the repository's one-engine rule.

Consequently, the apparent distinction — no reverse walk after a successful
anchor probe — is a partial evaluation of a known verifier under a static
width fact.  It can save instructions on a surviving candidate, but it cannot
change the accepted language, a state update, or the offset-selection rule.
Under `AGENTS.md` §1, that is an optimization of published art, not the
required invention.

### Falsification search and result

This negative would be falsified by a block transition that uses the
width-invariant prefix to accept or advance anchored alternatives while
preserving simple-fold equality, source byte boundaries, leftmost order, and
multi-pattern ties, but cannot be expanded into finite anchor byte paths plus
a fixed coordinate translation and ordinary confirmation.  Merely eliminating
a reverse iterator, choosing a different anchor, vectorizing the form probe,
or encoding the prefix forms in a table would not suffice.

The upstream source check gives the opposite result.  StringZilla's current
metadata records an arbitrary safe-slice offset and its verifier already
recovers the start by validating the preceding head; under the invariant the
recovered width is the constant `L`.  PCRE2 independently uses the same
width-equality test to delimit its caseless fast-forward prefix.  Applying the
byte-path expansion above leaves no state or output that cannot be reproduced
by those known ingredients.  The proposed construction is therefore rejected
before performance work.

### Decision

Do not claim a prefix-anchor implementation, AVX2 kernel, or faster verifier as
a new construction. It is a specialization and retuning of known
safe-window/filter-and-verify machinery. It may nevertheless be combined and
measured if the target is a new field result; a future novelty claim would need
the falsifying block transition above.

## Follow-up construction sweep: lazy canonical Two-Way

### Status

**Negative assessment for a lazy canonical Two-Way scan.**  This construction
would use a different outer search algorithm from the reference: it avoids
calling `foldHasPrefix` at every source start by making period-based shifts.
It is nevertheless ordinary Two-Way matching over the already-closed
fold-equivalence quotient, with lazy access and a source-offset decoration.
It is an algorithm substitution on a canonical stream, not a new state
representation or transition.

This assessment deliberately does not reopen the orbit-class, raw-byte,
projection, rolling-fingerprint, or prefix-anchor cells above.  The proposed
control state is tested here because it tries to route around them by skipping
starts rather than by changing the fold representation.

### Construction assessed

Let `q` be the valid-rune orbit token / opaque-byte token map defined in the
first assessment.  The compiler computes a critical factorization and period
of `q(needle)`.  The scanner retains the usual Two-Way `memory`, factor
position, and period state, but does **not** materialize `q(haystack)`.  A
forward or reverse accessor at a source byte boundary decodes only the unit
needed for a comparison, returns its `q` token and its source width, and keeps
the byte offset of the proposed start.  Mismatches take the Two-Way shift;
only surviving candidates are compared across the factor boundary.

The attractive operational claim is that starts skipped by the period do not
invoke `utf8.DecodeRuneInString` or a simple-fold orbit walk.  A compiled
fold lookup could also remove the orbit walk at the comparison locations.
Variable UTF-8 widths would be handled by the accessor and by retaining a
unit-index-to-byte-offset cursor, rather than by assuming a fixed byte shift.

### Closest known constructions

| Construction | Source | Relation to the assessed plan |
| --- | --- | --- |
| Canonical-element Two-Way `strcasestr` | GNU libc [`strcasestr.c`](https://raw.githubusercontent.com/bminor/glibc/04e750e75b73957cf1c791535a3f4319534a52fc/string/strcasestr.c) and [`str-two-way.h`](https://raw.githubusercontent.com/bminor/glibc/04e750e75b73957cf1c791535a3f4319534a52fc/string/str-two-way.h), inspected at commit `04e750e75b73957cf1c791535a3f4319534a52fc` | `strcasestr.c` supplies `CANON_ELEMENT(c) = TOLOWER(c)` to the generic Two-Way implementation.  The header computes factorization, period, memory, and shifts only through that canonical-element interface. |
| Two-Way string matching | Crochemore and Perrin, *Two-Way String-Matching*, JACM 38(3), 1991 | The assessed factor, period, forward comparison, reverse comparison, and mismatch shifts are its normal control state. |
| Fold-orbit quotient | “Novelty assessment: fold-orbit alphabet” above | Replacing a valid source unit by `q` is exactly the previously closed representation; lazy emission changes storage and scheduling only. |
| Lazy automata over byte equivalence classes | rust-lang/regex [`regex-automata/src/hybrid/dfa.rs`](https://raw.githubusercontent.com/rust-lang/regex/ada2a5a6a08c8038daed9f77e1b4d9a3ca9930da/regex-automata/src/hybrid/dfa.rs), inspected at commit `ada2a5a6a08c8038daed9f77e1b4d9a3ca9930da` | Its lazy DFA computes transitions on demand and uses alphabet equivalence classes.  Deferring a canonical-unit access is the same kind of evaluation scheduling, not a new accepted relation. |

glibc's byte/locale contract is not this repository's Unicode contract, so it
is not a candidate implementation or semantic authority.  It is direct
current-source evidence that canonicalize-at-comparison plus Two-Way control
is an established construction.  The Unicode-specific part of the proposal is
supplied entirely by `q`, which is the closed component.

### Reduction

For a parsed haystack, write

```
Q = q(h0) q(h1) ... q(hk-1)
```

and retain the source-offset array beside it conceptually.  At every point in
the assessed scan, its accessor returns exactly `Q[t]` for some unit index `t`;
the source width merely converts a change in `t` to an entry in that offset
array.  The Two-Way factor, period, memory value, comparison result, and next
unit index are all functions of `Q`, `q(needle)`, and the standard Two-Way
control state.  Therefore materializing `Q` first and running ordinary
Two-Way produces the same comparison trace and selected match.  Laziness can
save allocation or avoid accesses to skipped units, but it adds no state that
is unavailable in that materialized execution.

Unequal source widths do not break this reduction: they change adjacent
entries of the offset map, not equality or order in `Q`.  A proposed direct
byte shift that tries to avoid this map must either classify a complete source
unit as `q` (returning to the quotient) or distinguish prefixes of its UTF-8
forms (returning to the closed raw-byte transition).  For multiple needles,
keeping one Two-Way state per needle violates the one-engine rule, while
merging the patterns is ordinary dictionary/automaton matching after `q`.

### Falsification search and result

This negative would be falsified by a shift or match-selection state that
preserves simple-fold equality, opaque-byte boundaries, leftmost byte offsets,
and multi-pattern ties but cannot be replayed from `Q`, its source offsets,
and a standard Two-Way execution.  A different factor heuristic, a packed
fold table, reverse decoding, or a faster implementation of `memory` would
not suffice.

The current glibc source check found the canonical-element parameterization
rather than a distinct state, and the quotient reduction maps every proposed
Unicode comparison to that interface. No output or transition survives the
materialized-`Q` replay, so the construction has no standalone novelty claim.

### Decision

Do not add a separate single-needle engine: that would violate the one-engine
rule. A lazy Two-Way component inside the shared plan would be known art, not a
novelty claim, and would qualify only if the resulting one-engine combination
reached a new field result.

## Follow-up construction sweep: ASCII-island seam ledger

### Status

**Negative assessment for an AVX2 ASCII-island scan with a Unicode seam
ledger.**  The proposal has a real fast-path shape: fold and search long
all-ASCII chunks in vectors, and decode only chunks that contain a non-ASCII
byte or a match spanning such a chunk.  Its seam state is, however, either a
canonical token stream at the exceptional units or the UTF-8 byte-prefix
state already closed above.  The vector filter plus confirmation arrangement
is also established current art.

### Construction assessed

For each 32-byte block, an AVX2 high-bit test decides whether the block is an
ASCII island.  In an island, a pattern-specific pair of ASCII probes is
folded with a letter-safe mask and compared in parallel.  The plan retains a
small ledger at either side of an island: partially matched pattern positions,
possible start byte offsets, and markers for the few fold orbits whose member
can cross the ASCII/UTF-8 width boundary.  On a non-ASCII block the ledger
would decode just enough units to continue a candidate, then hand the next
ASCII island back to the vector loop.  It aims to avoid a decode/orbit walk at
every byte position of common ASCII haystacks without materializing a full
folded string.

### Closest known constructions

| Construction | Source | Relation to the assessed plan |
| --- | --- | --- |
| Vector ASCII case-insensitive search with candidate confirmation | .NET [`Ordinal.cs`](https://raw.githubusercontent.com/dotnet/runtime/6e7f3434c54a58277a5d53eb30e89823e54788d6/src/libraries/System.Private.CoreLib/src/System/Globalization/Ordinal.cs), inspected at commit `6e7f3434c54a58277a5d53eb30e89823e54788d6` | `IndexOfOrdinalIgnoreCase` chooses an ASCII vector path, probes first and last characters with `Vector256`, extracts candidates, then calls full ignore-case equality; non-ASCII reaches a fallback. |
| SIMD folded islands with danger alarms and serial seams | StringZilla [`haswell.h`](https://raw.githubusercontent.com/ashvardanian/stringzilla/657f21c5d8c2c2da5da06d4a9ad87c3ef80953d0/include/stringzilla/utf8_uncased/haswell.h), inspected at commit `657f21c5d8c2c2da5da06d4a9ad87c3ef80953d0` | Its shared AVX2 driver folds a chunk once, probes a safe window, routes alarmed width-changing chunks to a serial danger-zone scanner, and verifies candidates using the stored safe-slice offset. |
| SIMD candidate plus verifier | Teddy/FDR/Snort, cataloged in `CONTEXT.md` §§1d and 3--5 | The island probes are a standard no-false-negative filter; a full matcher decides survivors. |
| Fold-orbit tokens and raw UTF-8 form paths | The first two assessments in this file | The seam's complete-unit case is `q`; its partial-unit case is a node of an expanded UTF-8 form path plus lexical boundary state. |

The .NET and StringZilla contracts differ from this repository's simple-fold
UTF-8 contract, so neither is a drop-in baseline or a correctness source.
They are cited for the state layout and control shape the proposal claims as
new.  StringZilla is particularly direct current evidence: its AVX2 loop has
both the folded-island probes and an explicit width-hazard handoff.

### Reduction

Within an all-ASCII island, every source byte is one complete unit and its
fold class is a singleton or the ordinary ASCII letter pair.  The probe state
is therefore an ordinary ASCII quotient comparison or a conventional
candidate filter.  At a seam, a correct ledger has only two choices:

1. emit one complete valid unit or opaque byte as a fold class and advance a
   pattern position; this is the closed `q` stream with a sparse offset map;
2. retain a lead byte, continuation progress, or alternative form until the
   unit completes; these are exactly the proper-prefix and lexical states of
   the closed raw-byte fold transition.

There is no third state that can distinguish an isolated opaque `0x84` from
the middle `0x84` of `E2 84 AA` while declining both a token and a UTF-8
prefix state.  This distinction is required by the repository's existing
Kelvin trap.  Decorating either representation with an ASCII-island boundary,
a candidate bit mask, or a resume pointer does not change its transition
relation.  Sending candidates to a whole-pattern checker is the published
filter/verify shape, not a new recognizer.

### Falsification search and result

This negative would be falsified by a seam record that advances or accepts a
simple-fold match across a non-ASCII unit, preserves all source-boundary and
tie rules, and cannot be expanded into either a `q` token or finite UTF-8
prefix paths followed by confirmation.  Merely widening the vector block,
choosing rarer probes, changing the alarm threshold, or eliding a decoder call
on ordinary ASCII does not meet that test.

The current .NET source contains the vector-probe/candidate/full-equality
shape, and current StringZilla contains the more relevant folded-chunk,
width-alarm, serial-seam, and safe-slice verification shape. Applying the
required opaque-byte distinction leaves only the two closed representations.
The seam ledger therefore has no standalone novelty claim.

### Decision

An ASCII-island fast path, seam cache, AVX2 kernel, or portable fallback would
be a retuning of published staged-search machinery, not a new construction. It
remains eligible only as part of a combination that reaches a new result.

## Follow-up construction sweep: width-debt bit-parallel wavefront

### Status

**Negative assessment for a width-debt Shift-Or wavefront.**  This was the
most direct attempt to make variable UTF-8 width part of the matcher state
rather than canonicalizing input or using a fixed-width anchor.  Once made
precise, every width-debt bit denotes an active case-expanded UTF-8 path.  The
proposed packed state is a standard NFA subset / bit-parallel encoding of the
closed raw-byte automaton.

### Construction assessed

Choose one representative encoding length for each compiled pattern unit.  At
a raw source byte cursor, retain a bitset `S[d]` for each feasible accumulated
width debt `d`: a bit for pattern position `j` says that a rendering of the
first `j` units can end here with source-byte consumption differing by `d`
from the representative prefix.  AVX2 would update several `S[d]` words at
once.  Lead/continuation masks decide whether an incoming byte begins,
continues, or completes a candidate form; a completed form updates `d` by its
actual source width minus the representative width.  Pattern bits carry
pattern IDs so an accepting bit can select leftmost start and lowest index.

The intended distinction is that no decoded rune or `unicode.SimpleFold`
orbit is materialized.  The scan appears to use only byte masks, width debt,
and bit shifts, while allowing Kelvin, long-s, sigma, and other unequal-width
members to converge at the same pattern position.

### Closest known constructions

| Construction | Source | Relation to the assessed plan |
| --- | --- | --- |
| Bit-parallel Shift-Or / Bitap matching | Baeza-Yates and Gonnet, *A New Approach to Text Searching*, Communications of the ACM 35(10), 1992 | A bit vector of active pattern positions is the standard packed NFA subset representation; adding another bit-vector dimension does not alter that fact. |
| Elastic-degenerate matching | Iliopoulos, Kundu, and Pissis, *Elastic-Degenerate String Matching*, Information and Computation 279 (2021), cited in `CONTEXT.md` §1b | Each pattern position here is a finite set of variable-length encodings.  The width dimension records which alternative path is active. |
| Case-expanded UTF-8 paths | “Follow-up assessment: direct raw-byte fold transitions” above | A form's lead, continuation, and terminal states are exactly the states the debt update must distinguish. |
| Lazy DFA transition tables and byte equivalence classes | rust-lang/regex [`hybrid/dfa.rs`](https://raw.githubusercontent.com/rust-lang/regex/ada2a5a6a08c8038daed9f77e1b4d9a3ca9930da/regex-automata/src/hybrid/dfa.rs), inspected at commit `ada2a5a6a08c8038daed9f77e1b4d9a3ca9930da` | Its DFA state is a cached subset transition over an equivalence-classed byte alphabet; `S[d]` changes the bit packing, not the accepted paths. |

### Reduction

Expand every simple-fold form of a pattern unit into its byte-labelled path.
For a proposed bit `(d, j)`, attach the path node reached in the form of unit
`j` and the UTF-8 lexical state necessary to distinguish valid bytes from
opaque bytes.  If `d` alone does not identify that node, the proposal must add
lead/continuation/form bits; those additions identify it exactly.  Conversely,
the path node's consumed source width and representative prefix width compute
`d`, so the mapping is lossless after the necessary prefix distinction is
included.

Thus a complete wavefront word is a subset of nodes in the expanded byte graph.
A byte update advances that subset along outgoing edges; the Shift-Or shifts
and masks merely pack the subset into machine words.  Width debt is a label on
an NFA state, not a new transition.  Pattern IDs, source starts, and tie order
are output tags on the same active paths.  Selecting their minimum preserves
contract semantics but does not change acceptance.

Without the lexical/path component the scheme is unsound: a raw `0x84` would
have the same local debt as a continuation byte unless it records the
preceding lead state, causing the existing opaque-continuation trap to fail.
With the component it is the raw-byte automaton product already assessed.

### Falsification search and result

This negative would be falsified by a width-debt update that handles every
simple-fold form and opaque invalid byte, reports the same leftmost source
start and lowest pattern index, yet cannot be expanded into an active subset
of byte-form and lexical nodes.  A wider debt range, a different representative
length, a lane-parallel update, or a SIMD prefix classifier would not suffice.

No such state remains after attaching the mandatory form-prefix information:
each `S[d]` bit maps to an expanded path node, and each raw-byte transition is
its ordinary edge update. The current regex-automata DFA source independently
uses transition caching and alphabet equivalence classes for this same
subset-transition family. The state reduction closes the novelty claim.

### Decision

A Shift-Or wavefront, width-debt tables, and SIMD kernels would be a bit packing
of a known case-expanded byte automaton, not a new construction. They may be
tested as components only against the repository's new-result acceptance bar.

## Follow-up construction sweep: boundary-tagged block transducer

### Status

**Negative assessment for a boundary-tagged block transducer.**  This was the
remaining block-oriented attempt: process a 32-byte block as one relation from
an incoming lexer/matcher state to an outgoing state, while carrying the
minimum source start and pattern index as tags.  It could eliminate scalar
per-position calls operationally, but the block relation is exactly a
composition of byte transitions from the closed raw-byte automaton.  Tags are
ordinary output decoration, not a new matching state.

### Construction assessed

Let a compiled plan have an incoming UTF-8 lexical residue `l` and an active
multi-pattern state set `s`.  For a raw 32-byte block `B`, the proposal forms

```
T_B(l, s) = (l', s', earliest-start, lowest-pattern-at-that-start).
```

An AVX2 implementation would calculate or compose several such effects in
parallel, perhaps retaining only boundary summaries for a block and using a
min-plus-like rule for the output tags.  `IndexFold` would instantiate the
one-pattern plan; `Matcher.Find` would instantiate the same plan over all
patterns.  Unlike the width-debt plan, this description does not expose a
per-position fold class or a debt dimension at runtime.

### Closest known constructions

| Construction | Source | Relation to the assessed plan |
| --- | --- | --- |
| Case-expanded byte automaton with UTF-8 lexical product | “Follow-up assessment: direct raw-byte fold transitions” above | Its state already consists of the byte-form and boundary information that `(l, s)` must retain. |
| Lazy DFA transition tables | rust-lang/regex [`hybrid/dfa.rs`](https://raw.githubusercontent.com/rust-lang/regex/ada2a5a6a08c8038daed9f77e1b4d9a3ca9930da/regex-automata/src/hybrid/dfa.rs), inspected at commit `ada2a5a6a08c8038daed9f77e1b4d9a3ca9930da` | The source describes a DFA that computes and caches next transitions, including multi-pattern DFAs and byte equivalence classes.  A block table is a repeated next-transition table. |
| Compiled matcher state across input blocks | Hyperscan [`runtime.rst`](https://raw.githubusercontent.com/intel/hyperscan/828b4fef341759e05292741a6c89cb66055986f8/doc/dev-reference/runtime.rst), inspected at commit `828b4fef341759e05292741a6c89cb66055986f8` | Its streaming interface retains compiled pattern-matching state across blocks specifically so matches can span block boundaries. |
| Tagged start-of-match state | Hyperscan SOM and tagged-regex machinery, summarized in `CONTEXT.md` §1b | Carrying an earliest start with an active state is standard match-output bookkeeping; it does not alter the input transition relation. |

### Reduction

Let `delta` be the one-byte transition of the expanded raw-byte matcher,
including its UTF-8 lexical component.  For any block
`B = b0 b1 ... b31`, the untagged part of the claimed summary is exactly

```
delta(...delta(delta((l, s), b0), b1)..., b31).
```

A table, SIMD circuit, or composed relation can evaluate that expression in a
different order, but it cannot accept a byte sequence that the expression does
not accept.  Conversely, expanding the block summary into its 32 applications
of `delta` reproduces every outgoing state.  The earliest-start / lowest-index
pair is an associative min-selection over the same accepting paths, so it is a
standard output tag layered on that transition.

This is not changed by computing effects for every possible entry state,
retaining only selected summaries, or using an AVX2 shuffle to apply many
entries.  Those are table layout and evaluation choices.  The claimed plan is
therefore a block acceleration of the closed byte graph, just as a lazy DFA
caches an ordinary next-state relation.

### Falsification search and result

This negative would be falsified by a precise block update whose accepted
paths, source-boundary behavior, or tagged leftmost/tie output cannot be
recovered by expanding it into byte transitions plus min-selection tags.  A
faster composition, a smaller table, or a proof that the SIMD circuit does
less work is not enough: it must change the state relation.

The inspected current regex-automata source describes cached DFA transition
and equivalence-class machinery, while current Hyperscan documentation records
compiled state maintained across blocks. Applying the direct expansion above
leaves no irreducible block state or output, so the proposal has no standalone
novelty claim.

### Decision

A block-summary matcher for this construction would be a repacking of the
existing case-expanded byte automaton, not a new construction. It may still be
an engineering component if the resulting one-engine system reaches a new
measured result.

## Follow-up construction sweep: elastic-offset anchor lattice

### Status

**Negative assessment for an elastic-offset anchor lattice.**  Unlike the
closed prefix-invariant anchor, this proposal does not require a constant
prefix width.  It retains every legal byte displacement from a pattern start
to a selected anchor and intersects several such anchors.  That apparent
escape fails because width sets prove only length.  To prove the intervening
contents, the state must either run an ordinary verifier or retain which
fold-form path generated each displacement; those are, respectively, the
published candidate/confirm pipeline and the closed raw-byte automaton.

### Construction assessed

For a pattern with unit form sets `E(a0), ..., E(am-1)`, choose one or more
internal anchors.  For anchor `j`, compile the displacement set

```
D[j] = { len(e0) + ... + len(e(j-1)) : ei in E(ai) }.
```

The scanner finds an anchor form at source byte `t` and inserts every proposed
start `t-d`, `d` in `D[j]`, into a small source-coordinate lattice.  A second
anchor tests the corresponding displacement set and removes incompatible
starts.  The intended AVX2 form uses vector probes for multiple
`(anchor-form, displacement)` lanes; a multi-pattern plan shares the lattice
and carries pattern IDs with starts.  Because the state holds a *set* of
widths, Kelvin or long-s can occur before an anchor without the fixed-
width-prefix assumption rejected above.

### Closest known constructions

| Construction | Source | Relation to the assessed plan |
| --- | --- | --- |
| Safe arbitrary-offset slice with head/tail verification | StringZilla [`utf8_uncased.h`](https://raw.githubusercontent.com/ashvardanian/stringzilla/657f21c5d8c2c2da5da06d4a9ad87c3ef80953d0/include/stringzilla/utf8_uncased.h), [`serial.h`](https://raw.githubusercontent.com/ashvardanian/stringzilla/657f21c5d8c2c2da5da06d4a9ad87c3ef80953d0/include/stringzilla/utf8_uncased/serial.h), and [`haswell.h`](https://raw.githubusercontent.com/ashvardanian/stringzilla/657f21c5d8c2c2da5da06d4a9ad87c3ef80953d0/include/stringzilla/utf8_uncased/haswell.h), inspected at commit `657f21c5d8c2c2da5da06d4a9ad87c3ef80953d0` | It chooses a safe slice at an arbitrary needle offset, scans it in SIMD, and verifies head/tail to recover a source match.  Multiple candidate offsets instead of one do not alter that filter/verify shape. |
| Case-expanded literal paths | “Follow-up assessment: direct raw-byte fold transitions” above | Every element of `D[j]` arises from one or more concatenated form paths; a displacement is a projection of those paths that has forgotten their content. |
| Elastic-degenerate matching | Iliopoulos, Kundu, and Pissis, *Elastic-Degenerate String Matching*, Information and Computation 279 (2021), cited in `CONTEXT.md` §1b | Per-position variable-length alternatives and the possible aggregate lengths are the formal object.  A length lattice is a coarse projection of its path state. |
| Literal prefilters that discard pattern identity | rust-lang/regex [`prefilter/mod.rs`](https://raw.githubusercontent.com/rust-lang/regex/ada2a5a6a08c8038daed9f77e1b4d9a3ca9930da/regex-automata/src/util/prefilter/mod.rs), inspected at commit `ada2a5a6a08c8038daed9f77e1b4d9a3ca9930da` | Its documentation explicitly describes no-false-negative prefilters that may have false positives and discard which pattern supplied a literal; the lattice's anchors have the same information loss. |

StringZilla has full-fold semantics and is not a candidate implementation for
this repository.  It is current-source evidence for the claimed
arbitrary-offset probe plus surrounding verification mechanics.  The proposal
changes the number of retained offsets, not the information available to an
anchor hit.

### Reduction

A lattice node `(j, d)` states only that some rendering of the pattern prefix
has source length `d`.  It does not identify which member was chosen at any
prior position, nor whether the bytes preceding the anchor match those members.
For example, two different prefixes can have the same total width and produce
the same node even when one has a mismatching non-ASCII unit.  Therefore an
anchor intersection alone cannot accept a match.

If a candidate is checked by replaying the prefix/suffix, the full predicate
is the known anchor-filter-plus-verifier pipeline.  If instead the lattice is
refined until it can accept without replay, each node must retain the chosen
form's partial/terminal identity and UTF-8 lexical boundary state.  Draw an
edge for each chosen form: the refined lattice is precisely the concatenation
of the finite `E(ai)` byte paths, with `d` as an annotation.  Its vector of
possible offsets is a subset-state encoding of the closed raw-byte matcher.

Intersections, packed displacement bits, and minimum source-start tags can
reduce candidate cost, but they do not restore the discarded content
information.  They consequently cannot introduce a transition other than
ordinary path advance or ordinary candidate confirmation.

### Falsification search and result

This negative would be falsified by an elastic-offset update that proves all
prefix and suffix fold equality, rejects opaque continuation-byte starts,
returns leftmost/lowest ties, and cannot be expanded into either form paths or
an anchor hit plus a verifier.  A larger displacement set, more anchors,
vectorized intersections, or a better scoring policy would not suffice.

The current StringZilla source check found arbitrary-offset safe-slice probing
with verification, and the current regex-automata prefilter source documents
the corresponding deliberate loss of identity. Lifting the lattice's
lost-content projection to a correct accepting state reconstructs the
case-expanded paths. No distinct state survives, so no standalone novelty
claim follows.

### Decision

An elastic-offset lattice, multi-anchor AVX2 filter, or portable fallback would
be known candidate/confirmation machinery or an annotated case-expanded byte
automaton. It remains available as an engineering ingredient, but not as an
invention claim by itself.

## Follow-up assessment: synchronization-tagged variable shift

### Status

**Negative assessment for the queued synchronization-aware shift
construction.**  The construction can avoid attempting a full fold comparison
at every byte or rune boundary, but its state is a bad-character/good-suffix
(or q-gram) shift table over the already-known case-expanded UTF-8 language,
with the ordinary UTF-8 boundary state attached.  A width tag is necessary for
correctness, but it is just a compressed encoding-path coordinate.  It does
not create a new transition or recognizer.

No implementation, AVX2 kernel, portable fallback, or benchmark is warranted
for this construction.

### Construction assessed

For a pattern decoded into units `a_0 ... a_(m-1)`, let `E(a_i)` be the finite
set of raw UTF-8 strings for its simple-fold orbit, with a singleton raw byte
for an opaque invalid unit.  The proposed scanner does not try every source
start.  Instead it keeps a synchronization descriptor `sigma` at a probe
location (enough UTF-8 lexical state to distinguish a valid unit boundary from
an in-codeword byte and from an opaque invalid byte) and looks up a shift from
an observed byte or q-gram:

```
shift[sigma, observed fragment] -> next probe displacement,
                                  candidate pattern offsets / width tags
```

A tag records the possible source-byte distance from the candidate start to
that pattern fragment.  It is needed because a preceding unit may have a
fold-equivalent form with another UTF-8 width.  For example, for `akb`, a
fragment after `k` has different start distances in `akb` and `a\u212Ab`.
The scanner would use the fragment, its boundary tag, and the associated
distance(s) to skip directly to the next possible alignment, then confirm a
surviving start under simple folding.  A SIMD version batches several
fragments and tags in a block; a multi-pattern version associates pattern IDs
with the same entries.

The descriptor cannot be omitted.  The raw byte `0x84` must not become a
candidate start inside `E2 84 AA` (KELVIN SIGN), while an isolated `0x84` is
an opaque unit.  Thus a raw-byte shift that does not carry an equivalent
boundary/error state fails the existing `lone continuation vs kelvin bytes`
trap.

### Current-art check

| Construction | Source | Relation to the assessed plan |
| --- | --- | --- |
| Bad-character and good-suffix shifts | Boyer and Moore, *A Fast String Searching Algorithm* (1977); Go 1.26.5 `src/strings/search.go` (`stringFinder`) | A table whose safety assertion is “no alignment in this interval can match this observed fragment” is exactly a generalized shift table.  Replacing a byte key with a q-gram or a boundary tag changes the table alphabet, not the shift argument. |
| UTF-8 synchronization | RFC 3629, §3 | UTF-8 lead/continuation structure supplies the boundary descriptor.  It is a lexical property of the encoded stream, not a new pattern-search state. |
| UTF-8 case-insensitive n-gram indexing with synchronization | ClickHouse [`Volnitsky.h`](https://github.com/ClickHouse/ClickHouse/blob/e1d7e5b99c63dc18b08919808f5296a5f0248b87/src/Common/Volnitsky.h) and [`StringSearcher.h`](https://github.com/ClickHouse/ClickHouse/blob/e1d7e5b99c63dc18b08919808f5296a5f0248b87/src/Common/StringSearcher.h), inspected at commit `e1d7e5b99c63dc18b08919808f5296a5f0248b87` | `putNGramUTF8CaseInsensitive` calls `UTF8::syncBackward`, enumerates case forms for a byte n-gram, and records its pattern offset in Volnitsky's skip/filter table.  It declines a form when the case variants have unequal UTF-8 widths; adding a set of width-tagged offsets is an extension of this existing representation, not a different machine. |
| Elastic-degenerate matching | Iliopoulos, Kundu, and Pissis, *Information and Computation* 279 (2021), cited in `CONTEXT.md` §1b | A sequence of finite, variable-length `E(a_i)` sets is the formal object.  Prefix-width sets and position recovery are ordinary state for that object. |
| Width-safe Unicode fast-forward | PCRE2 [`pcre2_jit_compile.c`](https://github.com/PCRE2Project/pcre2/blob/ff92e0b9cea5b5ae3af12ba930d03556684f098b/src/pcre2_jit_compile.c#L6308-L6318), inspected at commit `ff92e0b9cea5b5ae3af12ba930d03556684f098b`; StringZilla [`haswell.h`](https://github.com/ashvardanian/stringzilla/blob/657f21c5d8c2c2da5da06d4a9ad87c3ef80953d0/include/stringzilla/utf8_uncased/haswell.h), inspected at commit `657f21c5d8c2c2da5da06d4a9ad87c3ef80953d0` | PCRE2 stops an offset-based prefix at a width-changing case form.  StringZilla carries a safe-window offset, detects width hazards, and recovers/validates around the window.  Both establish the same coordinate and confirmation problem the tag is meant to solve. |
| Raw-byte fold-path state | “Follow-up assessment: direct raw-byte fold transitions” above | An `E(a_i)` form plus a UTF-8 lexical tag is an ordinary path in the already-assessed expanded byte graph. |

The ClickHouse source is especially direct prior art for the proposed control
shape: its n-gram compiler synchronizes from a byte fragment back to a UTF-8
unit, enumerates the accepted case variants, stores the source pattern offset,
and uses that entry to drive a skip/filter scan.  Its unequal-width rejection
is a semantic limitation, not evidence that carrying multiple offsets is a
new transition.  The only extra datum in the proposed table is the prefix
width delta needed to select one of those offsets.

### Constructive reduction

Expand every member of every `E(a_i)` into a byte-labelled path.  Product the
result with the finite UTF-8 lexical machine that distinguishes valid unit
boundaries and opaque bytes.  Call the resulting graph `G`.  Each
synchronization descriptor in the proposal is a state (or a short path label)
of that lexical factor; each width tag is the byte distance accumulated along a
path in `G` from the candidate start to the selected probe.

For a fixed observed fragment, the shift table says that every path in `G`
which could yield an accepted occurrence has its next possible alignment at or
after the recorded displacement.  That is the usual bad-character statement.
If the table also considers a matched suffix, it is the usual good-suffix
statement.  A q-gram merely packs several labelled edges into the lookup key.
Unpacking the tag into one row per possible path produces the same candidate
starts; packing rows with equal fragments and displacements recovers the
proposal.

Width changes do not escape this reduction.  A `k` alternative has the paths
`6B`, `4B`, and `E2 84 AA`.  If a probe lies after it, the third path contributes
two more source bytes than either ASCII path.  Keeping `{d, d+2}` as a compact
prefix-distance set is equivalent to retaining the three paths until their
common continuation.  Dropping `d+2` misses a KELVIN rendering; treating an
`84` fragment as an independent start admits the forbidden continuation-byte
match.  The required tag therefore either expands to the paths or fails the
contract.

A vector implementation evaluates several such lookup/transition rows at
once.  It remains a block transition of `G`, exactly the category rejected in
the raw-byte assessment: batching, a larger fragment, or a branchless table
layout changes scheduling and cost, not the state relation.  If the scan works
over decoded units instead, the same construction is Boyer--Moore over the
fold-orbit quotient already rejected at the beginning of this file.

For multiple patterns, attaching `(pattern ID, pattern offset, width tag)` to
a fragment is the dictionary version of the same q-gram filter.  It is covered
by MultiVolnitsky/Commentz--Walter/Wu--Manber-style candidate machinery in
`CONTEXT.md` §1d; a merged continuation state is an ordinary case-expanded
dictionary automaton.  Neither form supplies the one new package-owned engine
required by `AGENTS.md` §5.

### Falsification search and result

This negative would be falsified by a synchronization-aware shift update that
simultaneously preserves source byte boundaries, all variable-width
simple-fold forms, leftmost order, and multi-pattern ties, but cannot be
reproduced by either:

1. byte paths for `E(a_i)` producted with the UTF-8 lexical state plus a
   generalized bad-character/good-suffix or q-gram shift table; or
2. an online orbit-token stream followed by an ordinary literal/dictionary
   shift transition.

In particular, merely storing a set or bitset of cumulative width deltas,
conditioning a second probe on the first form's width, using a longer q-gram,
or evaluating the table in SIMD would not falsify the reduction.  Each is a
finite collection of the same paths and coordinate tags.

The source check produced the opposite result.  ClickHouse already has the
synchronized n-gram/offset/filter shape, while its documented unequal-width
surrender identifies precisely the path coordinate the proposal adds.  PCRE2
and StringZilla separately show the established width-safe boundary and
recovery strategies.  Expanding the proposed tags leaves no transition or
output beyond those paths and standard shift safety.  The construction is
therefore known-art composition before an implementation is considered.

### Adjacent construction batch

The following nearby states were generated after rejecting the queued shift.
They are recorded separately so that a later round does not rediscover them as
new variants of the same idea.

| Cell | Proposed state | Why it closes | What would falsify that closure |
| --- | --- | --- | --- |
| Delta-tagged Volnitsky | Store every reachable prefix-width delta beside each fold-variant n-gram and choose a candidate start from `(n-gram, delta)`. | This is the unequal-width generalization of ClickHouse's existing n-gram-plus-offset entry.  The delta is the path coordinate in `G`; it is also an elastic-degenerate prefix-width state. | A shift/output that cannot be expanded into the corresponding n-gram rows and path coordinates. |
| Form-conditioned SIMD pair probe | A first probe's matched form chooses the byte offset of a second probe, so distinct-width forms re-synchronize lanes without serial decoding. | The `(first form, second offset)` cases are finite byte paths between two probes.  A SIMD lane mask is a packed block transition of those paths; StringZilla's safe/danger split and the Sneller serial width chain in `CONTEXT.md` §1b are the closest published scheduling variants. | A joint lane transition whose matches, starts, or pattern ties cannot be reproduced by the finite form paths plus the UTF-8 boundary state. |
| Boundary bitmap plus unit shift | Classify UTF-8 starts in a SIMD bitmap, then run a bad-character shift over fold classes at only those starts. | The bitmap is only a batched decoder boundary stream and the classes are the already-rejected orbit quotient.  It moves classification in time but does not change the matcher state. | A boundary/block state that retains information unavailable to the quotient token stream while not reducing to raw byte paths. |
| Shared multi-pattern shift map | Map a fragment to all `(pattern, offset, delta)` records and resolve earliest candidates in one pass. | This is a dictionary q-gram/filter table; merging records is a case-expanded dictionary automaton, and retaining them as confirmations is the known multi-filter shape. | A transition with a proof of linearity and leftmost/tie selection that is neither a dictionary automaton nor a candidate/confirm table. |

No construction in this batch survives the novelty gate.  This closes the
**synchronization/coordinate-aware shift** class, not the whole problem.

### What would reopen this space

A next round should not try another byte tag, shift heuristic, q-gram length,
or SIMD layout.  The class would reopen only with a state representation that
uses variable-width fold alternatives *jointly* to produce a safe skip and the
leftmost `(start, pattern)` result, while proving that it cannot be expanded
into finite UTF-8 form paths plus lexical state and a standard shift/filter or
into an orbit-token matcher.  It would also need a single N=1--N=512 plan, a
linear adversarial bound, and an AVX2 block transition with a portable
fallback.  A published construction with that state would instead falsify any
claim of novelty immediately.

### Decision

Synchronization-tagged shifts and the four adjacent variants are competent
extensions, combinations, or repackings of published shift/filter and UTF-8
path machinery. They are not new constructions; they may still be measured as
ingredients in pursuit of a new result. No performance claim is made here.


## Follow-up assessment: fused classified block frontier

### Status

**Negative assessment for a fused UTF-8 classifier, fold-set matcher, and
leftmost-origin frontier.**  This was the next distinct construction after the
shift family: classify each byte block once with SIMD, feed the classification
directly into a compiled fold-set transition, and carry the earliest source
start and pattern ID in that transition.  It would remove both the per-start
`utf8.DecodeRuneInString` call and the orbit walk without allocating a stream
of decoded tokens.

The absence of a materialized token slice is operational, not a new state
representation. A lossless classifier produces the already-rejected
fold-orbit quotient with an offset map; a classifier that retains raw-form or
partial-width distinctions is the already-rejected case-expanded byte graph.
Adding a leftmost-origin tag is established tagged-automaton state. This closes
the novelty claim, not the engineering route.

### Construction assessed

For a 32-byte input block `B` and a carry `c` for a UTF-8 sequence crossing the
block edge, compile a search plan `P` that emits, without a scalar per-position
decode:

```
C_P(B, c) = (boundary mask, opaque-byte mask, fold-set masks, byte-offset map, c')
T_P(frontier, C_P(B, c)) = (next frontier, earliest (start, pattern) output)
```

A fold-set mask marks valid source-unit starts whose rune belongs to one of the
simple-fold orbits used by `P`; opaque bytes have their own singleton masks.
`T_P` would advance one shared N=1--N=512 state machine.  Instead of reporting
a bare accept bit, every active state carries the lexicographically least
`(source byte start, pattern ID)` that reaches it.  A portable scalar path
would implement the same `C_P` and `T_P` relation; an AVX2 path would batch the
masks and transitions.

This is stronger than merely running a SIMD prefilter: it proposes to use the
classified masks as the matcher input itself, retain source offsets through the
block transition, and accept without a separate `foldHasPrefix` confirmation.
The intended payoff is one classification pass per input block, including
mixed-width forms and invalid-byte opacity.

### Current-art check

| Construction | Source | Relation to the assessed plan |
| --- | --- | --- |
| SIMD UTF-8 validation, transcoding, and character counting | [`simdutf` README](https://raw.githubusercontent.com/simdutf/simdutf/449f6b00ab5529a617bcc7f95fe4f886a847d690/README.md), inspected at commit `449f6b00ab5529a617bcc7f95fe4f886a847d690` | The project supplies the established block-classification/transcoding primitive.  Fusing its output into a matcher changes scheduling, not the information emitted by classification. |
| SIMD fold/search block with width-hazard routing | StringZilla [`haswell.h`](https://raw.githubusercontent.com/ashvardanian/stringzilla/657f21c5d8c2c2da5da06d4a9ad87c3ef80953d0/include/stringzilla/utf8_uncased/haswell.h), inspected at commit `657f21c5d8c2c2da5da06d4a9ad87c3ef80953d0` | Its AVX2 driver folds one 32-byte block, intersects probe masks, carries chunk-edge hazard state, and routes variable-width forms to a serial handler.  It is not this contract, but it establishes the fused block-plus-hazard control shape. |
| Unicode-caseless bit-parallel expressions | icgrep/Parabix, cited in `CONTEXT.md` §1d | The transposed-bitstream line natively compiles Unicode-caseless expressions into one block machine.  A dictionary/tie contract is an additional output discipline, not evidence that a classified block transition is new. |
| Leftmost source-origin tracking | Hyperscan [`compilation.rst`](https://raw.githubusercontent.com/intel/hyperscan/828b4fef341759e05292741a6c89cb66055986f8/doc/dev-reference/compilation.rst), inspected at commit `828b4fef341759e05292741a6c89cb66055986f8`, “Start of Match” | `HS_FLAG_SOM_LEFTMOST` returns the leftmost source start for an accepted end and documents the extra runtime state required to carry potential starts.  Choosing the lowest terminal pattern ID is an ordinary ordered reduction over the same tagged frontier. |
| Case-expanded UTF-8 paths and fold quotient | The first two assessments in this file; UTS #18, RE2, rust-regex, and regex-automata cited there and in `CONTEXT.md` §1b | These are the two exhaustive representations available after a block has either forgotten or retained the raw encoding form. |

The source inspection also refutes the tempting “fused means unclaimed”
argument.  StringZilla's driver explicitly folds its loaded block once, creates
candidate masks from multiple probes, and manages width-danger blocks before
verification.  simdutf's classifier is a faster way to obtain block facts, but
putting a different consumer immediately after it does not make the facts or
consumer state new.  Hyperscan independently shows that carrying match origins
is established state with a documented cost.

### Constructive reduction

Consider a valid unit at each set bit in the boundary mask.  There are only two
ways `C_P` can make its match-relevant identity available to `T_P`:

1. **One identity per unit.**  If its fold-set masks determine one orbit token
   (restricted to the plan's needed alphabet is enough), define that identity
   to be `q_P(unit)`.  The masks plus byte-offset map are an online,
   non-materialized rendering of `q_P(haystack)` plus its source-position map.
   `T_P` is then a literal or dictionary transition over the orbit quotient,
   which is the initial negative assessment.
2. **More than one identity or partial form progress.**  If `T_P` needs to
   distinguish `k`, `K`, and `E2 84 AA`, or needs a mask for a continuation
   prefix, expand every such distinction into its corresponding `E(a_i)` byte
   path.  The carry `c` and boundary/opaque masks are the UTF-8 lexical product
   state.  `T_P` is then a packed block transition of the raw-byte fold graph,
   which is the second negative assessment.

There is no third match-relevant datum.  A mask that does not distinguish
orbit-equal forms belongs in case 1; one that does distinguish their bytes or
width belongs in case 2.  SIMD packing, lane compaction, or avoiding a
materialized slice does not alter that dichotomy.

The origin tag does not rescue novelty.  Let an active matcher state carry a
value from the ordered set of source starts and pattern IDs.  On a transition,
it propagates the value attached to the predecessor; when paths merge, it takes
the lexicographic minimum.  Erasing the values yields exactly the quotient or
byte-path matcher above.  Restoring them is a tagged NFA/DFA or Hyperscan-style
start-of-match product, not a new folding or block transition.  A hazard
bitmap is likewise the safe/danger partition already visible in StringZilla;
it changes which known representation handles a block, not the representation
of a match.

### Falsification search and result

This negative would be falsified by a classified-block state that carries
match-relevant information which cannot be recovered from either a fold-orbit
token plus its byte start or a finite raw UTF-8 form path plus lexical state,
while still deciding simple-fold equality, opaque-byte behavior, and
leftmost/lowest-ID selection.  It would need to show why erasing its SIMD
layout cannot yield either of those two ordinary machines.  A faster
classification kernel, no temporary token allocation, a wider vector, or a
better origin-tag packing would not suffice.

Applying that test gives the negative result.  The proposed masks either
collapse to the quotient or retain form paths; the origin field is an existing
tagged-machine output.  The fused construction is therefore a composition of
known classifier, fold representation, and match-origin mechanisms, despite
having a potentially useful cost profile.

### Adjacent construction batch

| Cell | Proposed state | Why it closes | What would falsify that closure |
| --- | --- | --- | --- |
| SIMD hazard overlay | A fast width-preserving block transition plus a sparse event map that repairs starts only near cross-width fold forms. | StringZilla's alarm/danger-zone driver already partitions safe blocks from width hazards and recovers around a selected safe window.  Restricting it to simple folding or changing the event-map layout removes cases rather than creating a state. | A repair transition that accepts/reports matches without safe-window confirmation and cannot be expanded into form paths with boundary state. |
| Tagged leftmost fold frontier | Carry `(start, pattern ID)` alongside every active native fold-set state and reduce at merges. | Erasing the tag recovers an ordinary case-expanded or quotient matcher; restoring it is start-of-match tracking, for which Hyperscan documents the state cost. | An origin update that cannot be represented as propagation plus an ordered merge over active paths. |
| Classify-then-compact rune lanes | SIMD-compacts decoded units into fixed lanes and compares all pattern positions in those lanes. | The compacted lanes are an explicit `q_P` stream with a byte-offset map, even if retained only in registers. | A lane value needed for matching that is neither an orbit identity nor raw-form/path information. |

No cell in this block-frontier batch survives the novelty gate.  This closes the
**fused classification, hazard-overlay, and tagged-origin** class, not every
possible native fold-set construction.

### What would reopen this space

A viable next construction must expose a match-relevant state that is neither a
canonical orbit token with source coordinates nor a case-expanded UTF-8 path
with lexical state, and it must do more than carry a standard origin tag.  It
would need a proof that the state composes across AVX2 blocks, gives a linear
adversarial bound, selects leftmost/lowest-ID results, and remains one plan for
N=1 through N=512.  Evidence of a published state with those properties would
instead close the claim. Until then, a faster fused classifier alone is known
engineering, and qualifies only if its combination reaches a new result.

### Decision

The fused classified frontier and its adjacent variants are not novel by
themselves. They remain eligible as components under the measured-result bar.
No performance claim is made here.

## Retained implementation transition and evidence

### Retained transition

The implementation uses one compiled fold-orbit plan for `IndexFold` and
`Matcher.Find`.  Its AVX-512 BW fast path is a conservative candidate
transition, not a second recognizer: the plan records two fixed-width raw UTF-8
pair sets at known source-byte offsets, intersects their 64-byte candidate masks
in one block, and sends each survivor through the same decoded token transition
that decides the match.  The pair sets include every simple-fold rendering of
both runes.  The construction is enabled only after fixed-width prefix analysis;
invalid bytes, continuation bytes, variable-width folds, leftmost position, and
pattern-ID ties remain decided by the common plan.

This is a direct adaptation of the dispersed-probe/candidate-confirm shape in
StringZilla's UTF-8 uncased search driver (the source and revision cited in the
fused-frontier assessment above).  Its source probes are known art.  The
repository-specific combination is a compiled pair-pair filter over two
fold-orbit byte sets, followed by the shared Unicode plan rather than a
per-pattern byte verifier.  It makes no claim that the state representation is
new; the falsifiable claim is operational: it must improve a contested field
row without dropping a semantic differential.  `BenchmarkBar` with all native
entrants and the long Unicode differential tests are the falsifier.

The same provenance applies to the retained ASCII three-probe and mixed-triple
transitions: they are conservative byte filters followed by the plan, informed
by StringZilla-style dispersed probing and AVX-512 mask arithmetic, not claimed
as a new language recognizer.

The bounded all-ASCII triple transition is specifically a Shufti/Teddy-style
adaptation: six nibble-to-slot tables intersect up to eight three-byte forms
with `VPSHUFB`, then the decoded plan confirms every survivor. For its partial
mixed-root shape, the compiler can instead select at most eight fixed-offset
adjacent pairs from the complete ASCII fold spellings. A four-table Shufti
projection finds pair survivors; each maps back to a candidate source start and
replays the same decoded plan. The bounded maximum pair offset delays selection
so filter encounter order cannot change leftmost order. `CONTEXT.md` §§3 and 8
already identify Teddy's nibble-mask buckets, static rare-pair anchors,
background rarity selection, and candidate/verification as prior art.
Restricting either known filter to roots
which the compiler proves cover an all-ASCII stream is a semantic guard for
this package's Unicode contract, not a novel matching construction.

The N=1 repeated-byte route is likewise the established single rare-byte
anchor from `CONTEXT.md` §3: a fixed unique byte avoids per-call sampling, then
the same compiled plan confirms its complete literal. The mixed-fold hit and
miss arena rows, warmed per-pattern control, and Unicode / invalid-byte
differentials are the operational falsifiers for these filters.

### VBMI table and mask-scheduling follow-up

The native Vectorscan 5.4.12 source prepared by `arena/vectorscan/prepare.sh`
was read as prior art, specifically `src/fdr/teddy.c`,
`src/fdr/teddy_avx2.c`, and `src/util/arch/x86/simd_utils.h`. Its AVX-512 VBMI
Teddy paths use generated byte-class tables with `vpermb512` and masked mask
operations. That established the relevant mechanical technique: compile a
small byte-class table once, classify a whole block, and combine survivor masks
before an exact confirmation. No Vectorscan source, generated table, or field
engine is linked, copied, or called by this package.

The retained implementation independently builds package-owned tables from the
already-compiled plan: all-letter three-byte and long-pair ASCII probes use
64-entry `VPERMB` tables, the bounded all-ASCII pair projection uses its own
128-entry `VPERMT2B` split tables, and the Unicode pair-pair projection uses
four conservative raw-byte tables. For a 9--15 byte all-letter literal, the
VBMI-only long pair may retain byte zero and replace the byte-eight filter with
a rarer later ASCII letter only when both original anchors are common. That
byte displacement is fixed at plan compilation; it does not depend on a
matched fold form, and non-VBMI or short inputs retain the ordinary
three-probe route. A low-six-bit table can only add an alias; every resulting
stop replays the same decoded plan before it can become a match. The exact
128-entry projection preserves the existing pair predicate. The kernels use
`VPTESTMB` directly on two table results where possible rather than
materializing a separate vector AND. The general pair-root loop separately
unrolls two independent blocks after its first one-block probe, which is
ordinary dependency/branch scheduling rather than a different matcher.

This is a negative novelty assessment. Byte-class table lookup, Teddy/Shufti
candidate masks, confirmation after a survivor, `VPERMB`/`VPERMT2B` selection,
and loop unrolling are established techniques. The package-specific table
layouts merely encode predicates the existing plan already owns, and the
common plan remains the sole match authority for N=1 and multi-pattern calls.
The only falsifiable claim is operational and belongs to the arena and semantic
differentials, not to a new search construction.

### In-scan bounded Unicode confirmation: known construction, targeted result

No new automaton state is claimed for this transition. For one eligible
literal, compilation packs at most twenty exact raw-token parts. Each part
stores one to three correlated width-stable forms of one or two bytes, its
source offset, width, and form count. The AVX-512 pair-pair scan proves two of
those parts while producing a 64-start mask. For each surviving bit, the same
assembly loop checks every remaining packed part and continues scanning after
a mismatch. Only an exact match returns to Go. The scalar tail checks the same
packed forms. Width-changing or opaque folds, literals that exceed the bounded
descriptor, and unsupported hosts retain the decoded transition of the same
plan.

The closest constructions are all known candidate-and-confirm machinery:

| Construction | Source | Relationship |
| --- | --- | --- |
| Safe Unicode slice, SIMD scan, and head/tail verification | StringZilla `utf8_uncased.h`, `serial.h`, and `haswell.h` at `657f21c5d8c2c2da5da06d4a9ad87c3ef80953d0`, cited in the fused-frontier assessment above | It selects a width-safe raw slice, finds candidates in SIMD, and verifies the rest of the literal. The retained transition specializes that shape to simple folding and keeps bounded confirmation in the scanner loop. |
| Vector probe followed by full ignore-case equality | .NET `Ordinal.cs` at `6e7f3434c54a58277a5d53eb30e89823e54788d6`, cited above | It establishes vector candidate production followed by exact caseless confirmation. Moving the confirmation across a Go/assembly boundary changes scheduling, not the accepted language. |
| AVX2 ASCII prefilter followed by `EqualFold` confirmation | [`mhr3/veloz`](https://github.com/mhr3/veloz) and `CONTEXT.md` sections 1, 2, 3, and 5 | It is the same broad filter-then-confirm arrangement under a narrower ASCII contract. |
| Teddy/FDR/Shufti candidate masks followed by confirmation | Vectorscan sources cited in the VBMI follow-up above; `CONTEXT.md` sections 1d and 3 through 5 | The pair-pair mask is another conservative literal filter. Keeping its mask inside the confirmation loop is an implementation schedule, not a new recognizer. |
| Width-preserving caseless prefix acceleration | PCRE2 `pcre2_jit_compile.c` at `ff92e0b9cea5b5ae3af12ba930d03556684f098b`, cited in the prefix-invariant assessment above | Its width check is the same established eligibility boundary used to keep raw offsets stable. |

The package-owned combination is still useful. It compiles the exact simple-fold
forms once, uses a 512-bit pair-pair filter for candidate production, checks the
whole bounded literal inside the full-block scan without returning false
survivors to Go, and retains the decoded executor for everything outside the
proved domain. None of the sources
above alone provides this package's byte-offset, invalid-byte, leftmost, and
simple-fold contract together with this measured AVX-512 position.

Ten randomized co-measured pairs on
`BenchmarkBar/multi/multi_N1_unicode_pair_miss_1_5mb` moved the candidate from
`4.547 x_vs_best` to `0.7328 x_vs_best`, with median time falling from 450,578
to 75,130 ns/op. That is a targeted operational result, not full acceptance.
The complete field board, all five Unicode-equivalent Rebar rows, and both
qualifying processors have not yet been measured for this candidate.

The construction is falsified by any semantic differential, unsafe tail,
invalid-byte mismatch, changed offset or leftmost result, or a full-board row
at or above `1.0 x_vs_best`. Object disassembly of the current kernel shows no
local spill, but it does reload the packed descriptor and form values inside
the survivor loop. A future register-resident layout must be measured as a
separate implementation result; those reloads are not disguised as one here.
No field implementation is imported, linked, embedded, or copied.

### Complete experimental Go SIMD backend: negative result

The complete amd64 vector backend was independently re-expressed with Go's
experimental `simd/archsimd` package behind `GOEXPERIMENT=simd`, while the
hand-written assembly remained the control. Both builds kept the same public
plan and runtime feature gates. Backend digest checks, direct tests, and
differential fuzzing passed under AVX-512, AVX2, and scalar feature modes.

The replacement failed the performance bar. Six alternating-order Ice Lake
runs of the ordinary `b.Loop()`-based
`BenchmarkBar/single/log_miss_1mb` row measured the experimental backend at
20.8--23.3 µs/op and `0.878--0.913 x_vs_best`; the assembly control measured
18.4--20.8 µs/op and `0.785--0.797 x_vs_best`. One required regression is
enough to reject an all-or-nothing backend migration, so a Sapphire Rapids run
was not needed to falsify it. The complete record is the public
[archsimd Case](https://app.perfloop.ai/t/oss/case_37sjyc8f94).

The generated code explains the result on that row. Its
`asciiPairDirectVBMISkip64` hot loop handles one 64-byte block, performs two
`VPERMB` lookups, materializes their intersection as a vector, converts it to a
k-mask, and crosses the mask to a general register on every iteration. The
assembly loop interleaves four independent 64-byte blocks, uses `VPTESTMB` to
produce four k-masks directly, and tests mask pairs before crossing to a
general register. The offline opcode table records 3-cycle latency and
1-per-cycle reciprocal throughput for unmasked ZMM `VPERMB` on Ice Lake, so the
four-way schedule exposes the independent work needed to hide that latency.

The difference is not limited to one loop. Generated Shufti kernels spill
wide lookup state into 144- and 512-byte stack frames and reload it in the hot
loop; their assembly counterparts keep the tables in vector registers with no
local frame. The attempted migration therefore confirms that the hand-written
kernels contribute to the measured result rather than merely restating code the
compiler already emits.

This negative would be falsified by a later compiler/backend that preserves the
four-way independent schedule, keeps mask arithmetic in k-registers, and avoids
the wide spills, followed by a complete-backend result that passes the same
correctness checks and all 33 field rows on both qualifying processors. Until
then, the assembly backend remains the accepted implementation.

### Rejected cells

| Cell | Result | Decision |
| --- | --- | --- |
| Unrestricted AVX-512 `VPERMB` affine pair mapping | Low-six aliases caused excessive survivors and focused Cyrillic rows lost to simpler compares. | Removed; the retained, shape-gated table projections above are different bounded predicates and retain plan replay. |
| Dynamic Unicode pair-anchor sampling | Sampling and an interior pair route added setup and regressed sparse-hit and latency shapes. | Removed; use the fixed compiled pair-pair transition only when its prefix proof holds. |
| Broad two/four-form Unicode triple anchors | Focused Cyrillic miss and latency shapes regressed substantially. | Removed. |
| Short fixed-prefix direct-root route | Regressed bracket/code and latency shapes. | Removed. |
| Triple-product specialization | Introduced while exploring the rejected Unicode-anchor path and had no independent retained use. | Removed. |

These are engineering outcomes, not novelty claims.  They would be reopened
only by a complete field measurement showing a previously rejected transition
wins without moving another row above the acceptance bar, plus the same
Unicode/invalid-byte differential evidence.

## Provenance

This contribution contains novelty assessments and implementation provenance in this file.  The
original orbit-quotient, raw-byte, fixed-width projection, rolling-fingerprint,
and prefix-invariant-anchor assessments, plus the five follow-up construction
sweeps above, were written for this repository from the current `AGENTS.md`,
`README.md`, `CONTEXT.md`, source and test files, and the cited source
locations.  They contain no copied implementation code and make no external
performance claim.  The follow-up sweep checked current upstream source via
`git ls-remote` and immutable raw-source revisions for GNU libc, .NET,
rust-lang/regex, Hyperscan, and StringZilla; semantic differences are stated
where those engines are used only as mechanical prior art.  If implementation
files are added later, each non-trivial file will identify its authorship and
source provenance here.

# How casei works

> **Casei does not make Unicode matching cheap. It avoids doing Unicode
> matching on almost every byte.**

That is the competitive advantage. The rest of this page adds resolution to
that sentence without changing it.

## Ten seconds

`casei` compiles each pattern set into two views of the same truth:

```text
                         exact fold-token plan
                       /                       \
patterns -> compile once                         -> first correct match
                       \                       /
                         cheap raw-byte sieve
                         (64 starts per block)
```

The sieve rejects impossible starting positions. It never declares a match.
Usually it rejects all 64 positions, so the exact Unicode plan does no work for
that block. If one or more positions survive, the plan checks those positions
and makes the decision.

This split gives `casei` both properties it needs:

- **Speed:** most input is handled as raw bytes in wide vectors.
- **Correctness:** every possible match is decided by the complete Unicode
  plan, never by a lossy shortcut.

## One minute: searching one block

Imagine looking for `fatal panic` without case sensitivity.

A conventional exact matcher can begin work at one input position, compare the
pattern, fail, move one byte, and try again. A regex engine can compile more
powerful machinery, but it still has to represent the case alternatives and
the rest of its regex contract.

`casei` knows at compile time that this is a literal. It can choose several
useful byte positions in the pattern—for example, positions far enough apart
that an accidental alignment is rare. On AVX-512 it loads those positions for
64 possible starts at once, compares their case-normalized bytes, and intersects
the resulting 64-bit masks.

```text
candidate starts       0 1 2 3 4 5 ... 63
probe at offset A      0 0 1 0 0 0 ...  0
probe at offset B      0 0 1 0 1 0 ...  0
probe at offset C      0 0 0 0 1 0 ...  0
                       -------------------- AND
survivors              0 0 0 0 0 0 ...  0
```

No survivor means none of those 64 starts can match, so the search advances by
a block. When a bit survives, `casei` replays the complete pattern at that byte
position. False positives only cost time; they cannot change the answer.

The compiler has several sieves because one shape is not best for every pattern
set: dispersed single-byte probes, adjacent pairs, pair-pair anchors, triples,
and bounded Shufti/Teddy-style tables. Route selection depends on facts proved
about the compiled patterns, not benchmark names.

## Why Unicode does not break the sieve

Unicode simple folding is not “lowercase both strings.” A pattern position is a
small orbit of equivalent runes, and those runes can have different UTF-8 byte
lengths:

```text
k    K    K       one byte, one byte, three bytes
s    S    ſ       one byte, one byte, two bytes
σ    ς    Σ       three different runes in one orbit
```

So `casei` first compiles the complete relation into tokens. Valid runes map to
their fold-orbit token. Invalid UTF-8 bytes map to separate opaque tokens. The
shared state machine advances over those tokens and owns all semantic decisions.

Raw-byte filters are then derived only where they are safe:

- A fixed ASCII probe is used only when its offsets remain valid for every
  relevant rendering.
- Pair and triple tables contain every raw UTF-8 form that could begin the
  corresponding token sequence.
- Low-bit table aliases and normalized Shufti buckets are allowed to add
  survivors, because the exact plan follows them.
- A route is disabled when the compiler cannot prove that its filter covers
  every possible start.

This is the correctness firewall: **the filter may say “maybe” too often, but
it may never say “impossible” about a real match.**

## Why many patterns do not mean many scans

`Matcher` does not call single-pattern search once per needle. `NewMatcher`
compiles all patterns into one trie with failure transitions over fold tokens.
Small products of states and tokens become dense transition tables; larger
plans keep sparse edges. Both representations are the same state machine.

The filters summarize possible starts across the whole pattern set. One pass
over the haystack therefore serves one pattern or hundreds:

```text
N=1       one compiled plan, one filter, one scan
N=512     one shared plan, shared filters, one scan
```

Terminals record pattern IDs. The plan delays its answer only as far as needed
to prove the leftmost byte position; ties go to the lowest original pattern
index.

## Where the advantage comes from

The advantage is not merely “casei has a prefilter.” Vectorscan and rust/regex
have excellent prefilters too. `casei` can co-design both halves around one
narrow answer: literal sets, Unicode simple folding, and the first leftmost
match. General regex engines must preserve more syntax and match behavior;
all-match engines must support continued enumeration; ASCII specialists do not
represent the Unicode relation at all. `casei` spends that saved generality on
literal-shape-specific sieves and a compact shared verifier.

There are four layers. They are useful to separate because “it uses AVX-512” is
true but incomplete.

| layer | what it buys | evidence that it matters |
|---|---|---|
| Work avoidance | Whole blocks are rejected without decoding or advancing the exact plan at every byte. | In a same-plan ablation, forcing representative rows through the unfiltered exact transition was 5.6× to 571× slower. The filter is the dominant mechanism, not decorative preprocessing. |
| Shared construction | One pattern set becomes one transition plan and one traversal instead of `N` separate searches. | The sealed engine case moved the worst full-field row from `x_vs_best=6.718` to `0.9123` while preserving the semantic suite. |
| Wider native transition | AVX-512 BW handles 64 candidate starts and keeps set arithmetic in mask registers; VBMI performs byte-table lookup in registers. | Masking AVX-512 off while retaining the same plans reduced median throughput by about 1.75× on Ice Lake and 1.89× on Sapphire Rapids across the 33-row board. |
| Kernel scheduling | Fewer dependent operations keep the wide sieve fed. | Fusing one four-way Shufti reduction improved its 64 KiB kernel by 21.6% and the field row using it by 21.8%. |

The first two layers are the construction advantage. The last two amplify it.
An AVX2 implementation of the same idea remains correct and useful; the
published field lead is specifically the AVX-512 implementation.

## What the competitors do—and what this proves

Every scoring implementation was built, profiled, and read at the level where
its answer lives: source, generated code, or hot disassembly. The detailed
prior-art record is in [`CONTEXT.md`](CONTEXT.md), and the provenance of each
adopted technique is in [`NOVELTY.md`](NOVELTY.md).

| entrant | its strength | the verified distinction |
|---|---|---|
| Vectorscan | A mature multi-regex engine with Teddy/FDR-style literal machinery and a 512-bit VBMI target. | It ran at the same vector width on the same CPUs. Beating it on full-scan miss rows proves the result is not explained by register width alone. Its all-match API is a different home field; see [`REBAR.md`](REBAR.md). |
| PCRE2-JIT | Strong prefix analysis and native JIT code for a broad regex language. | `casei` spends its compile budget on the narrower literal-set contract and can derive filters that need not preserve general regex behavior. |
| rust/regex | Byte-oriented automata plus strong literal extraction and Teddy prefilters. | It carries a general regex representation; `casei` shares one fold-token literal plan and specializes raw filters around its exact start/tie contract. |
| StringZilla | Dedicated SIMD UTF-8 search, including full-fold expansions. | Full folding is a different relation. The arena times the verification needed to reduce its candidates to simple-fold semantics; `casei` represents that relation directly. |
| veloz | Excellent hand-written AVX2 ASCII single-literal search. | It does not answer Unicode simple-fold or multi-pattern queries. Against it, both specialization and `casei`'s wider ISA contribute, so the benchmark does not pretend to separate them. |
| Rust Aho-Corasick | Strong ASCII multi-pattern DFA with a Teddy prefilter. | `casei` extends the shared-plan shape to Unicode fold orbits, variable UTF-8 widths, opaque invalid bytes, and 512-bit filters. |

This comparison supports a precise claim: **the winning result comes from a
complete Unicode plan hidden behind shape-specific raw-byte rejection, shared
across the pattern set and amplified by AVX-512 kernels.** It does not support a
claim that every component is new. Shufti, Teddy, rare anchors, tries, failure
links, `VPERMB`, and confirmation after a candidate are known techniques. The
new result is their measured combination under this contract.

## The evidence ladder

1. **Semantics:** millions of single- and multi-pattern differential cases plus
   fuzzing compare every backend with Go `regexp (?i)` on valid UTF-8 and the
   opaque-byte oracle on invalid input.
2. **Mechanism:** the unfiltered-plan and ISA ablations measure work avoidance
   and vector width separately; the Shufti case isolates one assembly change.
3. **Field result:** every native entrant is rebuilt from pinned source and
   checked for its actual dispatched width before it can enter `x_vs_best`.
4. **Two CPUs:** all 33 first-match rows lead on both Ice Lake and Sapphire
   Rapids, not one favorable microarchitecture.
5. **Counterexample hunt:** the direct rebar integration exposes the count-all
   rows where the current API loses. Those results narrow the claim instead of
   being excluded from the record.

The sealed measurements are the [engine case](https://app.perfloop.ai/t/oss/case_9r9ntnxjd1)
and the [Shufti refinement](https://app.perfloop.ai/t/oss/case_hqryrfd6j4).
The full local field is reproduced by [`scripts/reproduce.sh`](scripts/reproduce.sh).

## Where this advantage ends

- **Counting every match:** `Find` returns one leftmost match and does not
  preserve scan state for the next call. Rebar's count-all workloads reveal
  real losses. An iterator is missing, not merely a benchmark row.
- **Tiny one-shot searches:** compiling a plan costs more than
  `strings.Index` on a single short lookup.
- **Other CPUs:** AVX2 and scalar paths are correct, but the published lead is
  not claimed for them. There is no NEON kernel yet.
- **Full folding:** expansions such as `ß -> ss` are outside the relation.
- **Bad filters:** adversarial data can create many survivors. The exact plan
  keeps correctness and linearity, but throughput falls; the arena includes
  periodic, same-byte, and torture rows to make that visible.

## Map from the model to the code

- [`plan.go`](plan.go) compiles fold tokens, the shared state machine, and every
  conservative filter; it also contains the exact fallback transitions.
- [`matcher.go`](matcher.go) exposes the one-plan `Matcher` API.
- [`root_amd64.go`](root_amd64.go) performs runtime dispatch and connects plan
  shapes to block transitions.
- [`root_amd64.s`](root_amd64.s) contains the AVX2 and AVX-512 kernels.
- [`root_other.go`](root_other.go) is the portable implementation of the same
  filter contracts.
- [`arena/field.yaml`](arena/field.yaml) defines who is allowed into the field
  and what semantics and ISA each entrant actually ran.

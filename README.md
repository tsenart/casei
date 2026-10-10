# casei

`casei` searches UTF-8 text for one literal or a compiled set under Unicode
simple folding. `NewExactMatcher` adds opt-in raw-byte matching for an exact
substring or literal set.

`IndexFold` returns the byte offset of one folded literal. A `Matcher` built by
`NewMatcher` returns the leftmost folded match from a whole set in one scan;
these APIs use the same simple-fold relation as Go's `regexp (?i)` on valid
UTF-8. A `Matcher` built by `NewExactMatcher` compares bytes without folding.

## The result

The performance results below cover the folded `IndexFold` and `NewMatcher`
paths. `NewExactMatcher` adds a semantic capability; no exact-mode performance
result is claimed here.

The checked-in two-host acceptance snapshot covers the prior 36-row arena,
measured on Intel Ice Lake and Sapphire Rapids with AVX-512F, BW, and VBMI.
The current arena adds two complete-triple rows, bringing the publication gate
to 38.

`casei` finished first on every row in that snapshot against the entrants the
arena counted at the time: Go regexp, PCRE2-JIT, Vectorscan, StringZilla, veloz,
and Rust Aho-Corasick where their contracts apply. rust/regex was timed but
counted on only 1 of the 36 rows, and Rust Aho-Corasick was not counted on
`multi_N8_hit_log_1mb`, because the audit could not see their vector widths. The arena now counts every pinned entrant on every row it supports, so
the current verifier rejects these receipts. They record the prior field. They
are not a result against the full field; the 38-row run with every entrant is
the next publication gate.

| host | rows won | worst median `x_vs_best` | worst sample | median speedup |
|---|---:|---:|---:|---:|
| Ice Lake | 36/36 | 0.9624 | 0.9736 | 1.80× |
| Sapphire Rapids | 36/36 | 0.9716 | 0.9799 | 1.56× |

`x_vs_best` is casei time divided by the fastest other implementation on the
same workload. Lower is better. Every one of the snapshot's 216 measured row
samples was below 1.0. Every row had 5 to 7 entrants. `casei` reported 512-bit
dispatch, and Vectorscan reported a 512-bit VBMI database.

Rebar provides a useful independent check. It asks engines to enumerate every
non-overlapping match instead of returning the first one. `casei` now wins all
five Rebar rows that request the same Unicode folding relation on both CPUs.

| Rebar selection | Ice Lake | Sapphire Rapids |
|---|---:|---:|
| same Unicode contract | 5/5 wins, worst 0.8794 | 5/5 wins, worst 0.8999 |
| all 18 representable stress rows | 9/18 wins | 9/18 wins |

Five rows use the same Unicode contract as `casei`. The other 13 request
ASCII-only case matching. `casei` wins four of those and loses nine. Those nine
losses count: ASCII is a common subset of UTF-8, and the current publication
target is 18/18. The checked-in table and raw receipts are in
[REBAR.md](REBAR.md).

### First Initiative result

The [Casei Rebar Initiative](https://app.perfloop.ai/t/oss/init_y6kff75c02)
started with one cause behind the English losses. A long ASCII literal such as
`Sherlock Holmes` has width-changing Unicode fold mates, so a sparse `ſ` or
`K` in the haystack could send the remaining input through the Unicode path.

The accepted route keeps the existing 512-bit ASCII probe on clean gaps. It
gives a narrow source-width halo around each Unicode cluster to the exact
decoder, then resumes the probe. On Sapphire Rapids, the first verified Case
moved its focused 1 MiB field workload from a loss to a win:

| focused field result | before, loss | after, win |
|---|---:|---:|
| `x_vs_best` | 1.266 | 0.924 |

Four competitors and five entrants ran. Three repeated samples were between
0.9240 and 0.9396. The full 36-row arena countercheck had no losing sample. The
[Case, measurements, and checks](https://app.perfloop.ai/t/oss/case_gc5hfnthag)
are public. The checked-in 18-row Rebar score stays 9/18 because this Case
measured one focused workload.

The speed claim is for the AVX-512 implementation. The portable implementation
is correct and scalar. There is no NEON kernel yet.

## The idea

Most of search is proving that a match does not start here.

Imagine 64 possible starting positions in a block of text. `casei` checks a few
useful bytes for all 64 positions at once. The result is a 64-bit mask:

```text
possible starts      0 1 2 3 4 5 ... 63
first useful byte    0 0 1 0 0 0 ...  0
second useful byte   0 0 1 0 1 0 ...  0
                     -------------------- AND
survivors             0 0 1 0 0 0 ...  0
```

An empty mask skips the whole block. A surviving bit means "check this one."
The exact Unicode plan checks it before `casei` can report a match.

Compilation builds those two parts together:

```text
patterns
   |
   +--> conservative byte sieve --> reject impossible starts in blocks
   |
   +--> exact simple-fold plan ----> decide matches, offsets, order, and ties
```

That is where the advantage comes from:

1. The compiler knows the question is literal search under simple folding. It
   can choose byte tests that a general regex engine cannot assume.
2. One pattern and 512 patterns use one compiled plan and one traversal. The
   many-pattern sieve carries pattern tags, so exact replay checks only the
   literals that survived.
3. Unicode decoding happens at survivors instead of at every byte. Width-changing
   folds such as `k`/`K`/`K` are represented explicitly.
4. The AVX-512 kernels keep several independent blocks in flight and keep set
   arithmetic in mask registers. The hand-written schedule matters: a complete
   port to Go's experimental SIMD package stayed correct and lost the required
   field row it was tested on.

Mostly ASCII text with occasional Unicode takes the same route at a larger
scale:

```text
input       [ clean ASCII gap ][ ſK ][ clean ASCII gap ]
work          512-bit probe     exact     512-bit probe
                                 halo
```

The block probe resumes after each Unicode cluster. The exact plan owns the
small boundary halo, including byte widths and fold mates, while the proven
ASCII regions stay on the 512-bit path.

The [one-page explanation](HOW_IT_WORKS.md) follows this model into the actual
filters, assembly, competitor implementations, and measurements.

## Use it

```sh
go get github.com/tsenart/casei
```

```go
// One needle.
if casei.ContainsFold(line, "payment declined") {
	alert(line)
}

// Byte offset, or -1 when absent.
at := casei.IndexFold(line, "payment declined")

// Many needles, one compiled plan. Leftmost match wins. A tie goes to the
// lowest pattern index.
m := casei.NewMatcher([]string{"fatal panic", "oom killed", "segfault"})
if match, ok := m.Find(line); ok {
	fmt.Println(m.Patterns()[match.Pattern], match.Start)
}

// Enumerate non-overlapping folded matches. Width is the number of source
// bytes consumed, which can differ across a Unicode fold orbit.
m.Each(log, func(match casei.Match, width int) bool {
	fmt.Println(match.Pattern, match.Start, width)
	return true
})

// Exact byte equality is opt-in. Case matters, and matches may start inside a
// valid UTF-8 encoding.
exact := casei.NewExactMatcher([]string{"UserID=", "Authorization:"})
if match, ok := exact.Find(line); ok {
	fmt.Println(exact.Patterns()[match.Pattern], match.Start)
}
```

`NewMatcher` and `NewExactMatcher` compile once. Reuse a matcher across
searches. `Find` is safe for concurrent use. Search is allocation-free on the
published paths after compilation. A generic plan with a longest pattern above
the 256-entry inline ring may allocate an offset ring during search.

The library requires Go 1.22 or newer. Rebuilding the native benchmark field
requires Go 1.24 or newer.

## Semantics

For `IndexFold` and `NewMatcher`, valid UTF-8 matching follows Unicode simple
folding:

- `k`, `K`, and Kelvin sign `K` match.
- `s`, `S`, and long s `ſ` match.
- `σ`, `ς`, and `Σ` match.
- `ß` and `ẞ` match. `ß` and `ss` do not.

For `IndexFold` and `NewMatcher`, invalid UTF-8 bytes are opaque one-byte units.
Results use source byte offsets. `Matcher.Find` returns the leftmost start, with
ties resolved by the lowest pattern index. `Matcher.Each` emits non-overlapping
matches in that order and returns the exact source width of each occurrence.

`NewExactMatcher` compares raw bytes exactly, so case differences do not match.
Patterns and haystacks may contain arbitrary bytes, and a match may start inside
a valid UTF-8 encoding. `Find` returns the earliest byte start, with ties going
to the lowest pattern index. `Each` emits non-overlapping results and consumes
the selected pattern's byte length. After an empty result it advances one byte,
even inside UTF-8, and can emit an empty match at EOF. These rules are opt-in
and do not change the folded behavior of `NewMatcher` or `IndexFold`.

The folded APIs are checked against Go `regexp (?i)` by deterministic
differential tests and two fuzz targets. The portable, AVX2, and AVX-512 paths
run the same folded contract suite. Exact matching is checked against a raw-byte
oracle, including byte fragments, malformed input, and empty progress.

## How the field was measured

The arena builds every native entrant from pinned source. It rejects an entrant
that silently dispatches below the width promised for that tier. Each row
reports active entrants and their observed vector widths.

`BenchmarkBar` measures `casei` beside each eligible competitor six times.
The order alternates, so each operation goes first in three pairs. The median
paired ratio is computed for each competitor, and the largest ratio names the
fastest field result. The checked-in acceptance run repeats the complete board
three times on each host, pinned to one core.

The historical raw transcripts for that snapshot are here. They predate the
rule that every pinned entrant counts, and the current verifier rejects them:

- [Ice Lake BenchmarkBar](audit/acceptance/results/ice/benchmarkbar.txt)
- [Sapphire Rapids BenchmarkBar](audit/acceptance/results/spr/benchmarkbar.txt)
- [Receipt verifier and summary](audit/acceptance/README.md)

The repository also keeps the earlier sequential-window runs that exposed
measurement drift near parity. They failed the publication bar and are part of
the methodology record.

To rebuild the field and rerun all 38 rows on a qualifying Linux host:

```sh
./scripts/reproduce.sh
```

The script requires x86-64 with AVX2 and AVX-512F/BW/VBMI. It builds PCRE2,
Vectorscan, rust/regex, Rust Aho-Corasick, and StringZilla from their pinned
sources before it runs the board.

## What changed after Rebar found the gap

The original arena covered first-match search and single-needle counting. It
did not contain multi-pattern enumeration or the two focused shapes that made
Rebar's Russian rows hard. Perfloop optimized the board it was given. Rebar
showed what the board had omitted.

The follow-up added three focused arena rows and kept the Rebar rows as an
external gate. The surviving construction combines:

- tagged interior anchors for several Unicode literals;
- raw confirmation that follows one-, two-, and three-byte fold spellings and
  returns the source width it proved;
- an exact common-byte origin gate for eligible multi-pattern plans; and
- wider assembly schedules that scan several cache lines before testing masks.

The prior result closes all five same-contract Rebar rows while preserving all
36 rows in its acceptance snapshot. The current field adds complete-triple rows
to prevent the next route choice from escaping native competition.
[REBAR.md](REBAR.md) contains the before/after account and every external row.

## Perfloop record

I built `casei` as a hard, self-contained test for
[Perfloop](https://app.perfloop.ai). I supplied the problem, field, and
constraints. Perfloop proposed implementations, measured them against the
field, and sent survivors to an independent verifier.

- [Original full-engine Case](https://app.perfloop.ai/t/oss/case_9r9ntnxjd1)
- [Shared interior-anchor Case](https://app.perfloop.ai/t/oss/case_jws72csfa9)
- [Dispersed Unicode-probe Case](https://app.perfloop.ai/t/oss/case_b2m0dmh5wa)
- [Raw-confirmation Case](https://app.perfloop.ai/t/oss/case_tgkp9bs0r6)
- [Rebar streaming-enumerator receipt](audit/rebar/streaming-case.md)
- [Complete Go SIMD backend, rejected](https://app.perfloop.ai/t/oss/case_37sjyc8f94)

The remaining full-Rebar work is coordinated by the
[Casei Rebar Initiative](https://app.perfloop.ai/t/oss/init_y6kff75c02). Cases
test bounded constructions; the Initiative owns the 18/18 result and the
requirement to preserve every arena win.

Its first verified Case is
[sparse Unicode exception partitioning](https://app.perfloop.ai/t/oss/case_gc5hfnthag).
The Initiative research found the whole-input fallback. The bounded Case built
a route around it, with the independent verifier reproducing the field win.
Nine Rebar rows remain.

The public Cases are the experiment log. [`NOVELTY.md`](NOVELTY.md) records
the constructions that failed on paper or in measurements. Negative results
stay in the repo so the next attempt starts from evidence.

## Limits

- Published speed numbers cover x86-64 AVX-512F/BW/VBMI on Intel Ice Lake and
  Sapphire Rapids.
- The portable path is scalar. ARM64 is correct, with no NEON speed claim.
- The API searches literals and finite literal sets. `NewMatcher` and `IndexFold`
  use Unicode simple folding; `NewExactMatcher` uses raw-byte equality.
- Full-fold expansions such as `ß -> ss` are outside the folded contract.
- Plan compilation has a cost. Cache a matcher for repeated searches.
- The current arena has 38 rows. Its sources, field, dispatch, failed
  measurements, and verifier are open and pinned. The checked-in acceptance
  snapshot covers the prior 36-row inventory, counted rust/regex on only one
  row, and left Rust Aho-Corasick out of one row, so it is not a result against
  the full field. Rebar is the external
  cross-check.

## Read next

- [How it works](HOW_IT_WORKS.md)
- [Direct Rebar audit](REBAR.md)
- [Known field and prior art](CONTEXT.md)
- [Novelty and negative results](NOVELTY.md)
- [Historical acceptance receipts](audit/acceptance/README.md)
- [Rebar receipts](audit/rebar/README.md)

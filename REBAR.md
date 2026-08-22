# Direct Rebar audit

`casei` loses three of the five Rebar workloads that ask the same Unicode
folding question. Rebar counts every non-overlapping match. The 33-row arena
contains 28 first-match operations and five overlap-allowed single-needle
counts. It has no multi-pattern enumeration row. On Rebar's worst row, a weak
multi-pattern filter sends too much text into the exact Unicode plan. A measured
streaming version left that cost in place.

The audit below includes every caseless literal or finite-alternation workload
that `casei` can represent.

## The answer in 30 seconds

At Rebar commit
[`463d00f`](https://github.com/BurntSushi/rebar/commit/463d00f31887e84c38467805b9e3122c314b9521),
the inventory contains 18 performance workloads and three behavior checks.
`casei` passed the two checks that use compatible Unicode semantics. The third
asks `s` to miss `ſ`, which conflicts with Unicode simple folding.

Only five performance rows enable Unicode semantics and therefore ask the same
folding question as `casei`. Against Rebar's three recorded leaders (Hyperscan,
PCRE2-JIT, and rust/regex), `casei` wins **2/5** and loses **3/5** on both Ice Lake
and Sapphire Rapids. Its median `time / best-other-time` is 1.27 and 1.28; the
worst row is roughly 9× slower.

The remaining 13 performance rows request ASCII-only case insensitivity.
`casei` produced the expected answers on those corpora while retaining its
stronger Unicode relation. Their timings remain useful as stress tests under
that contract.

## Why the original gym missed these losses

The arena's scenario list had five single-needle count rows during the original
engine build. Its competitive bar mistakenly ignored their `count` flag and
timed only the first match. Commit
[`c4392e7`](https://github.com/tsenart/casei/commit/c4392e7e6bbdaa8cd263059d5a041b29bd57e9ae)
corrected that wiring before the publication runs. `casei` won those five rows
in the corrected 33-row result.

The corrected arena still counts by repeatedly calling a single-needle search.
It has no row that enumerates a compiled multi-pattern plan, and its synthetic
Unicode count rows do not cover the dense Russian corpus or the Rebar prefilter
shape measured here. The original Perfloop objective never asked it to win on
these paths.

The missing coverage let the losses survive. The measurements below identify
their direct causes: weak shared filtering on the five-pattern row and costly
native confirmation on the two single-pattern rows. The 33 arena rows remain
regression gates, and the five Unicode-equivalent Rebar rows are the acceptance
target for the open work.

## Why this is a different benchmark contract

| | casei arena | Rebar rows on this page |
|---|---|---|
| answer | first byte offset or leftmost match on 28 rows; overlap-allowed count on 5 single-needle rows | count or total span of every non-overlapping match |
| search state | one `IndexFold`/`Find` call, repeated from the next byte on the 5 count rows | one compiled engine repeatedly enumerates from the end of each match |
| pattern sets | multi-pattern rows stop at the first leftmost answer | both single-pattern and multi-pattern rows enumerate to the end |
| folding | Unicode simple folding on every row | Unicode on 5 rows, ASCII-only on 13 |
| measurement | in-process field timing, with three publication passes on each pinned host; Perfloop separately co-measured the engine's source revisions | [Rebar's sequential runner protocol](https://github.com/BurntSushi/rebar/blob/463d00f31887e84c38467805b9e3122c314b9521/METHODOLOGY.md), three independent passes here |

The audit adapter compiles `NewMatcher` outside the timed region. Each iteration
calls `Find` on successive suffixes until it reaches the end. The adapter checks
the matched byte width under simple folding before advancing. It supports
rebar's `count` and `count-spans` models, with the same compiled plan reused for
every hit. A future iterator could retain scan state and vector continuity. The
diagnosis below measures how much that would change the worst row.

## Results

`casei / best` is median casei time divided by the fastest selected competitor
on the same row and pass, then the median across three passes. Values below 1.0
are wins. The named competitor is the fastest by its three-pass median.

| rebar row | requested folding | Ice Lake `casei / best` | Sapphire Rapids `casei / best` |
|---|---|---:|---:|
| `curated/01-literal/sherlock-casei-en` | ASCII-only* | 7.57× (Hyperscan) | 8.88× (Hyperscan) |
| `curated/01-literal/sherlock-casei-ru` | Unicode | 2.57× (PCRE2-JIT) | 2.47× (PCRE2-JIT) |
| `curated/02-literal-alternate/sherlock-casei-en` | ASCII-only* | 7.02× (Hyperscan) | 7.02× (Hyperscan) |
| `curated/02-literal-alternate/sherlock-casei-ru` | Unicode | 9.86× (Hyperscan) | 9.18× (Hyperscan) |
| `hyperscan/literal-casei-english-nosom` | ASCII-only* | 4.95× (Hyperscan) | 6.40× (Hyperscan) |
| `hyperscan/literal-casei-english-som` | ASCII-only* | 4.96× (Hyperscan) | 6.52× (Hyperscan) |
| `hyperscan/literal-casei-russian-nosom` | Unicode | **0.71×** (rust/regex) | **0.58×** (rust/regex) |
| `hyperscan/literal-casei-russian-som` | Unicode | **0.72×** (rust/regex) | **0.56×** (rust/regex) |
| `imported/leipzig/tom-sawyer-huckle-fin-insensitive` | ASCII-only* | 2.70× (Hyperscan) | 3.10× (Hyperscan) |
| `imported/leipzig/twain-insensitive` | ASCII-only* | 1.35× (Hyperscan) | 1.18× (Hyperscan) |
| `imported/sherlock/name-alt3-casei` | ASCII-only* | **0.78×** (rust/regex) | **0.69×** (rust/regex) |
| `imported/sherlock/name-alt5-casei` | ASCII-only* | 1.04× (rust/regex) | **0.94×** (rust/regex) |
| `imported/sherlock/name-holmes-casei` | ASCII-only* | **0.79×** (PCRE2-JIT) | **0.75×** (PCRE2-JIT) |
| `imported/sherlock/name-sherlock-casei` | ASCII-only* | 1.57× (PCRE2-JIT) | 1.82× (PCRE2-JIT) |
| `imported/sherlock/name-sherlock-holmes-casei` | ASCII-only* | 2.64× (PCRE2-JIT) | 3.04× (PCRE2-JIT) |
| `imported/sherlock/the-casei` | ASCII-only* | 1.08× (PCRE2-JIT) | **0.90×** (PCRE2-JIT) |
| `opt/prefilter/literal-casei-english` | ASCII-only* | 1.85× (PCRE2-JIT) | 2.22× (PCRE2-JIT) |
| `opt/prefilter/literal-casei-russian` | Unicode | 1.27× (PCRE2-JIT) | 1.28× (PCRE2-JIT) |

`*` Rebar disables Unicode-aware case folding. `casei` cannot disable it, so it
does more work and would also match Unicode fold mates not present in these
particular corpora. These rows passed output verification on the recorded text.
Their requested relation is ASCII-only, so they stay outside the
contract-equivalent result.

Across all 18 stress rows, including those ASCII-only rows, `casei` wins 4/18
on Ice Lake and 6/18 on Sapphire Rapids. The product comparison uses the five
rows with the same Unicode contract.

## Why the worst row is 9× slower

The bad row searches a 1,570,556-byte Russian Sherlock Holmes corpus for five
names and counts 971 matches:

```text
one Russian pattern    rare interior byte pairs -> about 2,100 candidates
five Russian patterns  common starting letters -> about 80,000 filter stops
```

For one pattern, the compiler selects a fused pair-pair anchor from inside the
literal. The AVX-512 VBMI kernel skips 1,552,007 bytes and asks the exact
matcher to check 2,134 candidate positions.

For all five patterns, the current compiler cannot combine those interior
anchors into one shared filter. It falls back to a nine-pair Shufti filter over
the patterns' first UTF-8 bytes. Several of those starts are common Cyrillic
letters. Across the full count, the filter is invoked 79,950 times and admits
193,449 runes, or 21.7% of the corpus, into the exact plan. The plan performs
175,860 dense state transitions. The CPU profile attributes most of the time to
UTF-8 decoding and fold-token map lookup. The AVX-512 filter is a smaller part
of the row.

A one-pass diagnostic enumerator took 4.69 ms, compared with 4.65 to 4.99 ms for
repeated `Find`. Both returned 971 matches. The API restart cost is within the
noise on this row.

Replacing Shufti with the exact nine-pair AVX-512 filter made the row slower.
Disabling AVX-512 added roughly 25% to 33%. Those measurements put the cost in
the selectivity of the compiled question reaching the kernel.

Rebar's Hyperscan runner was rebuilt and checked independently. It returned 971
matches with an AVX-512 VBMI database. Start-of-match tracking changed its time
by about 2%. Hyperscan expands the folds at compile time into a byte-level
database and scans the five literals continuously. Its verification path avoids
the Go rune decoder and token map used here.

The smaller two losses have the same general shape at lower severity. On the
single Russian literal, `casei`'s rare interior anchor is effective. Each
survivor still enters a Go rune decoder and token map. PCRE2-JIT verifies in
native code and leads by about 2.5×. On the sparse one-match Russian prefilter
row, that residual verification gap is about 1.28×. The two wins compare the
same sparse shape with rust/regex.

## Correctness and inventory closure

Every recorded runner invocation returned rebar's expected result on each of
the 18 performance workloads it entered. On both hosts, `casei` also passed the
two compatible behavior checks. The incompatible behavior row is:

```text
test/unicode/case/ascii-only: pattern "s", haystack "ſ"
rebar with unicode=false: 0 matches
casei Unicode simple fold: 1 match
```

The audit also inspected the remaining case-insensitive rebar definitions:

- `curated/03-date/*` and `wild/url/*` are general regex grammars.
- `curated/13-noseyparker/*` loads a large rule file whose caseless rules also
  use classes, boundaries, captures, and bounded repetition; its search and
  compile workloads are general-regex workloads, not finite literal sets.
- `imported/sherlock/name-alt4-casei` contains a character class and `+`.
- `wild/ruff`, `reported/p893-hir-case-folding`, and
  `reported/i988-cloudflare-compile` exercise inline flags, classes,
  repetition, captures, or compile time.

Those are outside a literal/finite-alternation API. Every caseless rebar row
that can be represented as one literal or a finite set of literals is accounted
for above.

## Measurement record

The exact adapter, pinned integration script, six raw CSVs, and an independent
ratio calculator are checked in under
[`audit/rebar/`](audit/rebar/README.md). Running
`python3 audit/rebar/summarize.py` reproduces this page's table and summaries
from those receipts.

| item | recorded value |
|---|---|
| casei source | tree `d1f73802d35c29009a433eaaf9c2b51113ab5c95`, merged as [`3954dbe`](https://github.com/tsenart/casei/commit/3954dbe40e8e21c4c7b2e2716f22647dd7cd880c) |
| rebar source | `463d00f31887e84c38467805b9e3122c314b9521` |
| selected field | Hyperscan 5.4.2, PCRE2 10.47 JIT, rust/regex 1.12.4 |
| hosts | GenuineIntel family 6/model 106 Ice Lake and family 6/model 143 Sapphire Rapids, both with AVX-512F/BW/VBMI |
| passes | three per host, with up to 100 warmups/200 ms and 1,000 measured iterations/500 ms per benchmark |
| plan setup | outside rebar's timed iteration, matching the other engines |
| entrants | three or four per row including `casei`, as selected by each upstream definition |

The selected field contains the three leaders in rebar's published records for
these literal rows. A displayed win is scoped to that leader set. Any unmeasured
entrant could only make the ratio worse.

One upstream build detail is recorded for reproducibility: rebar's vendored
PCRE2 10.47 snapshot contains `pcre2posix.c` but not its unused
`pcre2posix.h`. The audit omitted that POSIX wrapper from the build; rebar's
runner uses the native PCRE2 API, so no compiled search or JIT code changed.

## Public work on the losses

The first four Perfloop experiments are public and stopped:

- [Compile shared interior UTF-8 anchors for multi-pattern plans](https://app.perfloop.ai/t/oss/case_jws72csfa9)
  explored the roughly 9x multi-pattern loss.
- [Compile dispersed width-stable Unicode byte probes](https://app.perfloop.ai/t/oss/case_b2m0dmh5wa)
  explored the two single-pattern losses.
- [Compile width-stable Unicode byte confirmations](https://app.perfloop.ai/t/oss/case_tgkp9bs0r6)
  explored compiled raw-byte confirmation.
- [Carry the confirmed end into repeated matching](https://app.perfloop.ai/t/oss/case_1jg4we7k3s)
  tested the narrower repeated-call explanation. The one-pass control and
  repeated `Find` remained effectively tied on the worst row.

Two later Cases carry the surviving work:

- [Keep N=1 confirmation inside the AVX-512 scan](https://app.perfloop.ai/t/oss/case_s8c41a1per)
  is verified on one targeted false-survivor row. It moved
  `x_vs_best` from `4.547` to `0.7328` over ten randomized pairs.
  [PR #10](https://github.com/tsenart/casei/pull/10) remains open because that
  result has not been reproduced across the complete board on both hosts.
- [Replace the decoded transition loop with one raw-byte plan](https://app.perfloop.ai/t/oss/case_rmg4fdm3me)
  is open. It targets the shared cost that remains after filtering: decoding a
  surviving position, mapping it into the fold-token alphabet, and then
  advancing the exact plan.

Any change accepted into `casei` must beat all five Unicode-equivalent Rebar
rows on Ice Lake and Sapphire Rapids, preserve the one-engine design and full
correctness contract, and keep every current `BenchmarkBar` row below 1.0
`x_vs_best`.

## Required next work

The next construction must:

1. compile once and keep one package-owned plan;
2. combine selective interior anchors from several patterns into one shared
   byte-level filter;
3. replace hot-path rune-to-token map lookups with compiled raw-byte
   transitions where the plan permits it;
4. preserve exact non-overlapping enumeration through the same state machine;
   and
5. beat the fastest eligible entrant on all five Unicode-equivalent rows before
   any count-all performance claim is made.

The public performance claim therefore covers the 33-row arena contract. It
does not claim leadership for non-overlapping enumeration.

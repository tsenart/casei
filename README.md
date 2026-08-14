# casei

**The fastest correct case-insensitive UTF-8 substring search on x86-64 —
faster than every specialist engine, on every row of an open, reproducible
benchmark.** `casei.IndexFold` finds one needle; `casei.Matcher` finds many,
both under Unicode simple case folding (the semantics of `regexp` `(?i)`).

It was not written by hand. It was produced by
[Perfloop](https://app.perfloop.ai) — a performance-proving loop, aimed by an
operator — pointed at one of the most-executed and worst-served operations in
computing. The operator chose the targets; the loop generated, measured, and
verified every change. Every candidate tried, every measurement, and the sealed
proofs are public: **[the engine case ↗](https://app.perfloop.ai/t/oss/case_9r9ntnxjd1)**.

> **Scope, up front.** These numbers are the **AVX-512** path (Intel Ice Lake or
> newer). `casei` also has an **AVX2** path (x86 without AVX-512) and a **portable
> scalar** path (other architectures — there is no NEON kernel yet); both are
> correct but **not benchmarked here**, so the result is scoped to AVX-512 and is
> not claimed for them. It is a *compile-once, search-many* engine: for a single
> short lookup, `strings.Index` is faster. It implements **simple** folding, not
> full folding (`ß` matches `ẞ`, never `ss`).

## Use it

```sh
go get github.com/tsenart/casei
```

```go
// One needle. The correct, allocation-free replacement for
// strings.Contains(strings.ToLower(haystack), strings.ToLower(needle)).
if casei.ContainsFold(line, "payment declined") {
    alert(line)
}

// Byte offset instead of a bool.
at := casei.IndexFold(line, "payment declined") // -1 when absent

// Many needles, one pass. Leftmost match wins; ties go to the lowest
// pattern index.
m := casei.NewMatcher([]string{"fatal panic", "oom killed", "segfault"})
if match, ok := m.Find(line); ok {
    fmt.Println(m.Patterns()[match.Pattern], match.Start)
}
```

`NewMatcher` compiles the pattern set once; reuse the `*Matcher` across
searches, and share it freely — `Find` is safe for concurrent use. Both entry
points allocate nothing per search.

Matching is Unicode **simple** case folding, identical to Go's `regexp` with
`(?i)`: `k` matches the Kelvin sign U+212A, `ſ` matches `s`, `σ`/`ς`/`Σ` all
match, and `ß` matches `ẞ` but never `ss`. That is not what lowercasing both
sides gives you — see [Semantics](#what-it-is).

Requires Go 1.22+. The AVX-512 and AVX2 paths are chosen at runtime on x86-64;
every other platform runs the portable path, which returns identical results
(see [Limitations](#limitations) for what that costs).

## Results

`casei` versus the full field — **every competitor built from source at full
strength, each dispatching its widest path**, on the two Intel microarchitectures
that expose the required AVX-512, independently reproduced on bare-metal cloud
hosts.

**casei is the fastest on every one of 33 rows, on both microarchitectures** — median **1.7×** (Sapphire Rapids) to **1.9×** (Ice Lake) faster than the next-fastest engine, from 1.1× on the tightest streaming row to 25× on the adversarial one. Throughput in GB/s, **bold = casei**; `casei vs #2` is casei over the fastest other engine on that row.

| workload | casei | Vectorscan | veloz | PCRE2-JIT | StringZilla | rust/regex | casei vs #2 |
|---|---|---|---|---|---|---|---|
| `log_miss_1mb` | **56.5** | 52.1 | 8.3 | 23.4 | 11.5 | 9.0 | **1.09×** |
| `code_miss_256kb` | **56.1** | 29.2 | 8.3 | 23.3 | 10.9 | 9.1 | **1.92×** |
| `prose_miss_1mb` | **56.2** | 19.7 | 8.3 | 23.4 | 12.3 | 9.0 | **2.40×** |
| `ru_miss_1mb` | **27.6** | 16.6 | – | 22.8 | 6.5 | 9.0 | **1.21×** |
| `multi_N512_miss_log_64kb` | **27.7** | 6.8 | – | 19.3 | 0.0 | 0.5 | **1.43×** |
| `latency_match_start_1kb` | **116.4** | 2.9 | 69.3 | 4.1 | 4.2 | 3.2 | **1.68×** |
| `samechar_miss_64kb` | **67.9** | 44.4 | 8.3 | 22.2 | 11.0 | 0.5 | **1.53×** |
| `periodic_miss_64kb` | **35.5** | 0.6 | 8.3 | 28.5 | 11.0 | 0.5 | **1.25×** |
| `torture_miss_64kb` | **13.0** | 0.1 | 0.5 | 0.3 | 0.1 | 0.3 | **25.70×** |
| `log_hit_sparse_1mb` | **32.0** | 1.5 | 8.0 | 7.3 | 10.4 | 6.5 | **3.08×** |

<details>
<summary><b>Full 33-row tables — Sapphire Rapids and Ice Lake, every entrant</b></summary>

> Measured at the engine as merged in [#1](https://github.com/tsenart/casei/pull/1)
> (commit `fa0dff6`). Kernel improvements merged since ([#3](https://github.com/tsenart/casei/pull/3),
> +21.5% on the Shufti kernels) are **not yet reflected** — if you reproduce today
> you should see casei slightly faster than these tables. A full refresh lands
> when the current optimization pass completes.

#### Sapphire Rapids (Xeon 8481C) — GB/s (higher is better; **bold** = casei, the fastest on every row)

| row | casei | Vectorscan | veloz | PCRE2-JIT | StringZilla | rust/regex | casei vs #2 |
|---|---|---|---|---|---|---|---|
| `latency_match_start_1kb` | **116.4** | 2.9 | 69.3 | 4.1 | 4.2 | 3.2 | **1.68×** |
| `samechar_miss_64kb` | **67.9** | 44.4 | 8.3 | 22.2 | 11.0 | 0.5 | **1.53×** |
| `log_miss_1mb` | **56.5** | 52.1 | 8.3 | 23.4 | 11.5 | 9.0 | **1.09×** |
| `prose_miss_1mb` | **56.2** | 19.7 | 8.3 | 23.4 | 12.3 | 9.0 | **2.40×** |
| `code_miss_256kb` | **56.1** | 29.2 | 8.3 | 23.3 | 10.9 | 9.1 | **1.92×** |
| `log_miss_64kb` | **53.5** | 44.3 | 8.3 | 22.0 | 12.3 | 8.9 | **1.21×** |
| `log_needle8_64kb` | **53.3** | 6.8 | 8.3 | 21.1 | 17.8 | 8.9 | **2.53×** |
| `log_needle16_64kb` | **53.3** | 35.6 | 8.3 | 22.1 | 11.8 | 8.9 | **1.50×** |
| `log_needle3_64kb` | **53.0** | 44.7 | 8.3 | 22.0 | 18.0 | 13.9 | **1.19×** |
| `log_needle32_64kb` | **52.8** | 6.8 | 8.3 | 21.5 | 10.9 | 8.9 | **2.45×** |
| `multi_N8_miss_ru_1mb` | **38.8** | 5.6 | – | 20.8 | 0.8 | 9.0 | **1.86×** |
| `multi_N64_miss_ru_64kb` | **37.1** | 7.1 | – | 21.3 | 0.1 | 0.5 | **1.74×** |
| `multi_N8_hazard_hit_1mb` | **36.6** | 6.8 | – | 2.7 | 0.9 | 31.3 | **1.17×** |
| `periodic_miss_64kb` | **35.5** | 0.6 | 8.3 | 28.5 | 11.0 | 0.5 | **1.25×** |
| `log_hit_sparse_1mb` | **32.0** | 1.5 | 8.0 | 7.3 | 10.4 | 6.5 | **3.08×** |
| `multi_N8_miss_log_1mb` | **29.1** | 6.8 | – | 14.2 | 1.6 | 9.0 | **2.05×** |
| `multi_N512_miss_log_64kb` | **27.7** | 6.8 | – | 19.3 | 0.0 | 0.5 | **1.43×** |
| `multi_N64_miss_log_64kb` | **27.7** | 6.8 | – | 21.9 | 0.2 | 0.5 | **1.27×** |
| `ru_miss_1mb` | **27.6** | 16.6 | – | 22.8 | 6.5 | 9.0 | **1.21×** |
| `ru_hit_sparse_1mb` | **24.6** | 0.8 | – | 19.7 | 6.5 | 8.5 | **1.25×** |
| `latency_match_mid_1kb` | **22.5** | 2.4 | 14.5 | 2.6 | 3.7 | 2.5 | **1.55×** |
| `kelvin_hazard_1mb` | **20.3** | 1.8 | – | 1.1 | 12.9 | 8.6 | **1.57×** |
| `multi_N8_miss_hazard_1mb` | **18.4** | 6.7 | – | 0.3 | 0.9 | 2.8 | **2.73×** |
| `multi_N2_miss_log_1mb` | **15.3** | 11.6 | – | 0.7 | 5.8 | 5.5 | **1.31×** |
| `log_miss_1kb` | **13.9** | 5.0 | 7.9 | 5.2 | 5.5 | 3.8 | **1.76×** |
| `latency_match_end_1kb` | **13.5** | 2.4 | 7.5 | 1.7 | 3.2 | 2.0 | **1.80×** |
| `latency_miss_1kb` | **13.3** | 4.6 | 7.9 | 4.9 | 5.6 | 3.8 | **1.67×** |
| `prose_hit_dense_1mb` | **13.1** | 0.0 | 6.8 | 1.0 | 4.1 | 3.0 | **1.94×** |
| `torture_miss_64kb` | **13.0** | 0.1 | 0.5 | 0.3 | 0.1 | 0.3 | **25.70×** |
| `code_hit_brackets_256kb` | **11.1** | 0.0 | 6.0 | 1.0 | 1.3 | 0.9 | **1.86×** |
| `multi_N8_hit_log_1mb` | **9.7** | 5.7 | – | 2.0 | 1.7 | 2.4 | **1.71×** |
| `ru_latency_miss_1kb` | **8.6** | 3.4 | – | 4.9 | 3.8 | 3.6 | **1.75×** |
| `multi_N512_miss_hazard_64kb` | **7.4** | 4.6 | – | 0.0 | 0.0 | 0.5 | **1.59×** |

#### Ice Lake (Xeon @ 2.6 GHz) — GB/s (higher is better; **bold** = casei, the fastest on every row)

| row | casei | Vectorscan | veloz | PCRE2-JIT | StringZilla | rust/regex | casei vs #2 |
|---|---|---|---|---|---|---|---|
| `latency_match_start_1kb` | **118.4** | 2.6 | 63.4 | 4.5 | 3.8 | 3.2 | **1.87×** |
| `samechar_miss_64kb` | **71.7** | 39.0 | 6.9 | 23.2 | 11.0 | 0.6 | **1.84×** |
| `code_miss_256kb` | **57.2** | 23.1 | 6.8 | 19.2 | 11.5 | 9.6 | **2.48×** |
| `log_miss_1mb` | **57.2** | 44.8 | 6.9 | 21.3 | 12.4 | 9.5 | **1.28×** |
| `prose_miss_1mb` | **57.0** | 15.7 | 6.8 | 16.4 | 12.1 | 9.5 | **3.48×** |
| `log_miss_64kb` | **54.7** | 39.2 | 6.9 | 20.1 | 12.2 | 9.4 | **1.39×** |
| `log_needle16_64kb` | **52.8** | 28.2 | 6.9 | 15.8 | 11.8 | 9.3 | **1.87×** |
| `log_needle32_64kb` | **52.8** | 6.9 | 6.9 | 16.1 | 11.0 | 9.3 | **3.27×** |
| `log_needle8_64kb` | **52.7** | 6.9 | 6.8 | 16.1 | 15.4 | 9.2 | **3.27×** |
| `log_needle3_64kb` | **52.6** | 39.1 | 6.9 | 16.6 | 15.4 | 14.4 | **1.35×** |
| `multi_N8_miss_ru_1mb` | **37.2** | 6.0 | – | 16.6 | 0.8 | 9.5 | **2.24×** |
| `multi_N8_hazard_hit_1mb` | **35.5** | 7.7 | – | 3.0 | 1.0 | 30.9 | **1.15×** |
| `multi_N64_miss_ru_64kb` | **35.3** | 5.8 | – | 15.7 | 0.1 | 0.5 | **2.25×** |
| `periodic_miss_64kb` | **30.9** | 0.5 | 6.9 | 23.6 | 11.0 | 0.6 | **1.31×** |
| `multi_N8_miss_log_1mb` | **30.9** | 7.0 | – | 13.4 | 1.6 | 9.5 | **2.31×** |
| `multi_N512_miss_log_64kb` | **29.5** | 6.9 | – | 13.9 | 0.0 | 0.5 | **2.13×** |
| `multi_N64_miss_log_64kb` | **29.5** | 6.9 | – | 19.3 | 0.2 | 0.5 | **1.53×** |
| `log_hit_sparse_1mb` | **27.6** | 1.5 | 6.7 | 6.9 | 10.3 | 6.8 | **2.69×** |
| `ru_miss_1mb` | **21.7** | 17.1 | – | 16.8 | 6.4 | 9.4 | **1.27×** |
| `kelvin_hazard_1mb` | **20.9** | 1.9 | – | 1.1 | 12.2 | 8.9 | **1.71×** |
| `multi_N8_miss_hazard_1mb` | **18.5** | 7.7 | – | 0.3 | 0.9 | 3.2 | **2.40×** |
| `latency_match_mid_1kb` | **18.5** | 2.1 | 12.0 | 2.3 | 3.2 | 2.4 | **1.54×** |
| `ru_hit_sparse_1mb` | **18.3** | 0.9 | – | 15.8 | 6.3 | 8.8 | **1.16×** |
| `multi_N2_miss_log_1mb` | **15.1** | 11.5 | – | 0.7 | 5.9 | 5.7 | **1.32×** |
| `log_miss_1kb` | **13.4** | 4.6 | 6.6 | 4.6 | 4.8 | 3.6 | **2.04×** |
| `latency_miss_1kb` | **12.7** | 3.9 | 6.6 | 4.2 | 4.8 | 3.6 | **1.94×** |
| `prose_hit_dense_1mb` | **12.2** | 0.0 | 5.9 | 1.0 | 3.7 | 2.9 | **2.07×** |
| `latency_match_end_1kb` | **11.0** | 2.1 | 6.3 | 1.5 | 2.9 | 2.0 | **1.76×** |
| `torture_miss_64kb` | **10.2** | 0.1 | 0.4 | 0.2 | 0.1 | 0.3 | **25.30×** |
| `multi_N8_hit_log_1mb` | **10.0** | 5.8 | – | 2.0 | 1.7 | 2.8 | **1.73×** |
| `code_hit_brackets_256kb` | **9.1** | 0.0 | 5.0 | 1.0 | 1.1 | 0.8 | **1.81×** |
| `ru_latency_miss_1kb` | **7.9** | 3.1 | – | 4.3 | 3.5 | 3.4 | **1.85×** |
| `multi_N512_miss_hazard_64kb` | **7.7** | 3.8 | – | 0.0 | 0.0 | 0.5 | **2.03×** |

Diagnostic baselines (`ToLower`+`Index`, the Go Aho-Corasick port, and the exact-match `ceiling`) are omitted from the “fastest” comparison — see [Is the benchmark fair?](#is-the-benchmark-fair). Reproduce all of it with `./scripts/reproduce.sh`.
</details>

- **Every one of the 33 rows is faster than the entire field** — ASCII and
  UTF-8, one needle and many, hit and miss — on both microarchitectures.
- **The claim that can't be waved away:** `casei` beats **Vectorscan**
  (Hyperscan's open successor, the state of the art) running its **512-bit
  AVX-512 VBMI** path — at the *same vector width, on the same silicon*
  (`vectorscan_vbmi=1`, dispatch-asserted). One compiled plan wins both cores;
  no per-CPU-model dispatch.
- The narrower engines run at their native max width — **veloz is 256-bit**
  (an AVX2 library), **PCRE2-JIT is 128-bit**. Where one of those is the fastest
  competitor, part of the margin is that `casei` targets AVX-512 and they do not
  — a real ISA advantage, not a handicap. The per-engine widths are in the table
  so you can separate that from the equal-width Vectorscan result.

Correctness is pinned to Go `regexp` `(?i)` by differential and fuzz on **every**
backend (AVX-512, AVX2, scalar): a 350k-case multi-pattern differential, a
2.8M-case single-pattern differential, and `FuzzIndexFold` / `FuzzMatcher`.

## Reproduce it

On an x86-64 Linux host **with AVX-512** (a GCP `n2`/`c3`, or a recent Intel
box — **not** Apple Silicon), one script builds the entire competitor field from
source and runs the scoreboard. This is exactly what CI runs on every push.

```sh
git clone https://github.com/tsenart/casei && cd casei
./scripts/reproduce.sh          # ~15 min: builds pcre2, vectorscan (VBMI), rure,
                                # rust-regex, stringzilla, then runs the benchmark
```

It prints, for all 33 rows, every entrant's throughput and the vector width it
dispatched, plus `x_vs_best` (`casei`'s time ÷ the fastest *correct* competitor)
and raw paired samples.

## What it is

`grep -i`, SQL `ILIKE`, log filters, header lookups — caseless search is one of
the most executed operations in computing, and it is far slower than it needs to
be. Regex engines reach it by case-expanding literals through general machinery;
dedicated engines mostly don't do these semantics at all. The idiom everyone
actually writes — `ToLower` both sides, then search — is not even correct
(`ToLower` splits the σ/ς/Σ orbit, re-encodes, and shifts byte offsets).

```go
// IndexFold returns the byte index of the first occurrence of needle in
// haystack under Unicode simple case folding, or -1.
func IndexFold(haystack, needle string) int

// Matcher finds any of a set of patterns under the same semantics; Find
// returns the leftmost match, ties to the lowest pattern index.
func NewMatcher(patterns []string) *Matcher
func (m *Matcher) Find(haystack string) (Match, bool)
```

They are the same problem: a pattern position is a small set of UTF-8 encodings
(its fold orbit), exact search is the singleton case, and multi-needle is the
union. `casei` is one adaptive engine over that object.

**Semantics** are Unicode **simple** case folding — exactly Go `regexp` `(?i)`,
pinned by differential test: `k` matches `K` and the Kelvin sign U+212A; `s`
matches long-s U+017F; `σ`/`ς`/`Σ` all match; `ß` matches `ẞ` but **not** `ss`.
Matches start at rune boundaries and a match window's byte length can differ from
the needle's. Bytes outside valid UTF-8 are opaque units. See
[`casei_test.go`](casei_test.go) for the executable definition.

## Is the benchmark fair?

This is the first thing to check, so the arena is built to answer it:

- **Only *correct* competitors count.** A baseline's time enters `x_vs_best`
  only if its output matches the arena oracle on that tier, enforced by an
  agreement test. The naive `ToLower`+`Index` idiom and the Go Aho-Corasick port
  are marked `diagnostic` — they run for profiling but **never enter the score**.
- **You compare against the *best*.** `x_vs_best` is `casei`'s time over the
  *fastest correct competitor present on that row*, not an average or a weak one.
- **No quietly-handicapped builds.** Every entrant declares and reports the ISA
  and vector width it dispatched to; Vectorscan is built with
  `BUILD_AVX512VBMI` and its 512-bit path is assertion-gated. A competitor that
  quietly ran a portable build is not a competitor.
- **Adversarial rows are included** (`periodic`, `samechar`, `torture`) so
  throughput can't be bought with a quadratic cliff.
- **It's the real thing, reproducibly.** The field is nine engines pinned to
  source versions and build flags in [`arena/field.yaml`](arena/field.yaml);
  ratios come from raw paired, order-alternated samples with confidence bounds.

The honest asterisk: the arena was developed alongside `casei`, so it is not a
neutral third-party harness. That is exactly why it is open and reproducible, and
why the competitors are the field's real specialists at full strength.

## Limitations

- **The result is AVX-512-specific.** On x86 without AVX-512, `casei` dispatches
  an AVX2 (256-bit) path; on ARM (Apple Silicon, Graviton) it runs a portable
  scalar path — there is no NEON kernel yet. Those paths are correct but
  unbenchmarked, so the result is not claimed for them. An ARM vector kernel is
  the next case.
- **Compile-once, search-many.** `NewMatcher` compiles a plan; a single tiny
  one-shot lookup pays that setup and `strings.Index` wins it.
- **Simple folding, not full.** `ß`→`ss` is a different, harder problem
  (StringZilla implements it); it is specified but not built here.
- **Not yet run inside [rebar](https://github.com/BurntSushi/rebar).** The arena
  has rows analogous to rebar's `sherlock-casei-en/ru`, and beats the same engine
  family on them, but on its own corpora. Wiring `casei` into rebar directly is
  the open follow-up.

## How it was built

`casei` is a [Perfloop](https://app.perfloop.ai) result, built in Perfloop's
operator-directed mode: an operator aimed the loop — submitting each hypothesis,
steering candidates with reviews, auditing the competitor field and the host
ISA — and Perfloop did the proving: it generated every candidate, measured each
against the pinned field under paired sampling, verified the winner
independently, and sealed the receipts. No claim here rests on the operator's
judgment; every one rests on a sealed measurement.

Three cases so far, each with its full public trail — every candidate, the
field manifest, the sealed measurements, the verification:
the [engine itself](https://app.perfloop.ai/t/oss/case_9r9ntnxjd1), a
[kernel fusion refinement](https://app.perfloop.ai/t/oss/case_hqryrfd6j4)
(merged as [#3](https://github.com/tsenart/casei/pull/3)), and a third case
now in progress. The direction of travel: the operator's search method is
being folded into the loop itself, so future finds of this class need no
operator at all.

## Details

- [`arena/field.yaml`](arena/field.yaml) — the field: versions, build flags,
  ISA, corpus hashes, semantic status.
- [`CONTEXT.md`](CONTEXT.md) — every technique known to this problem, with
  sources and measured numbers (including rebar's published results).
- [`NOVELTY.md`](NOVELTY.md) — an honest construction assessment; the fold-orbit
  representation is *not* claimed as novel, and says why.
- [`AGENTS.md`](AGENTS.md) — the arena's rules of engagement: baseline isolation,
  single-engine identity, and the acceptance bar a candidate must clear.

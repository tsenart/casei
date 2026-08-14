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

**casei is the fastest on every one of 33 rows, on both microarchitectures** — median **1.9×** (Ice Lake) to **1.7×** (Sapphire Rapids) faster than the next-fastest engine, from 1.10× on the tightest streaming row to 25.8× on the adversarial one. Throughput in GB/s, **bold = casei**; `casei vs #2` is casei over the fastest other engine on that row.

| row | casei | Vectorscan | veloz | PCRE2-JIT | StringZilla | rust/regex | casei vs #2 |
|---|---|---|---|---|---|---|---|
| `log_miss_1mb` | **56.4** | 51.3 | 8.3 | 23.3 | 12.3 | 9.0 | **1.10×** |
| `code_miss_256kb` | **56.1** | 29.1 | 8.3 | 23.3 | 11.5 | 9.1 | **1.93×** |
| `prose_miss_1mb` | **56.3** | 19.5 | 8.3 | 23.2 | 12.1 | 9.0 | **2.43×** |
| `ru_miss_1mb` | **27.5** | 16.5 | – | 22.8 | 6.5 | 9.0 | **1.21×** |
| `multi_N512_miss_log_64kb` | **27.7** | 6.8 | – | 19.5 | 0.0 | 0.5 | **1.42×** |
| `multi_N512_miss_hazard_64kb` | **9.8** | 4.6 | – | 0.0 | 0.0 | 0.5 | **2.14×** |
| `latency_match_start_1kb` | **118.1** | 2.9 | 70.1 | 4.6 | 4.4 | 3.3 | **1.68×** |
| `samechar_miss_64kb` | **67.6** | 44.7 | 8.3 | 22.3 | 11.0 | 0.5 | **1.51×** |
| `periodic_miss_64kb` | **35.5** | 0.6 | 8.3 | 28.4 | 11.0 | 0.5 | **1.25×** |
| `torture_miss_64kb` | **13.1** | 0.1 | 0.5 | 0.3 | 0.1 | 0.3 | **25.76×** |
| `log_hit_sparse_1mb` | **32.1** | 1.5 | 8.0 | 7.2 | 10.3 | 6.6 | **3.11×** |

<details>
<summary><b>Full 33-row tables — Sapphire Rapids and Ice Lake, every entrant</b></summary>

#### Sapphire Rapids (Xeon 8481C) — GB/s (higher is better; **bold** = casei, fastest on every row)

| row | casei | Vectorscan | veloz | PCRE2-JIT | StringZilla | rust/regex | casei vs #2 |
|---|---|---|---|---|---|---|---|
| `latency_match_start_1kb` | **118.1** | 2.9 | 70.1 | 4.6 | 4.4 | 3.3 | **1.68×** |
| `samechar_miss_64kb` | **67.6** | 44.7 | 8.3 | 22.3 | 11.0 | 0.5 | **1.51×** |
| `log_miss_1mb` | **56.4** | 51.3 | 8.3 | 23.3 | 12.3 | 9.0 | **1.10×** |
| `prose_miss_1mb` | **56.3** | 19.5 | 8.3 | 23.2 | 12.1 | 9.0 | **2.43×** |
| `code_miss_256kb` | **56.1** | 29.1 | 8.3 | 23.3 | 11.5 | 9.1 | **1.93×** |
| `log_miss_64kb` | **53.4** | 45.2 | 8.3 | 22.3 | 12.3 | 8.9 | **1.18×** |
| `log_needle3_64kb` | **53.3** | 45.0 | 8.3 | 22.1 | 18.0 | 13.8 | **1.19×** |
| `log_needle32_64kb` | **53.3** | 6.8 | 8.3 | 21.0 | 11.0 | 8.9 | **2.54×** |
| `log_needle16_64kb` | **53.3** | 36.0 | 8.3 | 22.1 | 11.8 | 8.9 | **1.48×** |
| `log_needle8_64kb` | **53.0** | 6.8 | 8.3 | 20.9 | 18.0 | 8.9 | **2.53×** |
| `multi_N8_miss_ru_1mb` | **38.6** | 5.7 | – | 23.3 | 0.8 | 9.0 | **1.66×** |
| `multi_N64_miss_ru_64kb` | **37.0** | 7.2 | – | 21.8 | 0.1 | 0.5 | **1.70×** |
| `multi_N8_hazard_hit_1mb` | **35.5** | 6.7 | – | 2.7 | 0.9 | 31.6 | **1.13×** |
| `periodic_miss_64kb` | **35.5** | 0.6 | 8.3 | 28.4 | 11.0 | 0.5 | **1.25×** |
| `log_hit_sparse_1mb` | **32.1** | 1.5 | 8.0 | 7.2 | 10.3 | 6.6 | **3.11×** |
| `multi_N8_miss_log_1mb` | **29.3** | 6.8 | – | 14.3 | 1.6 | 9.0 | **2.04×** |
| `multi_N64_miss_log_64kb` | **27.7** | 6.8 | – | 22.0 | 0.2 | 0.5 | **1.26×** |
| `multi_N512_miss_log_64kb` | **27.7** | 6.8 | – | 19.5 | 0.0 | 0.5 | **1.42×** |
| `ru_miss_1mb` | **27.5** | 16.5 | – | 22.8 | 6.5 | 9.0 | **1.21×** |
| `ru_hit_sparse_1mb` | **24.5** | 0.8 | – | 19.3 | 6.5 | 8.5 | **1.27×** |
| `latency_match_mid_1kb` | **22.7** | 2.4 | 14.5 | 2.6 | 3.8 | 2.5 | **1.57×** |
| `kelvin_hazard_1mb` | **20.3** | 1.8 | – | 1.2 | 12.8 | 8.5 | **1.58×** |
| `multi_N8_miss_hazard_1mb` | **18.3** | 6.8 | – | 0.3 | 0.9 | 2.8 | **2.67×** |
| `multi_N2_miss_log_1mb` | **15.2** | 11.5 | – | 0.7 | 5.8 | 5.5 | **1.32×** |
| `log_miss_1kb` | **13.7** | 5.5 | 7.9 | 5.4 | 5.5 | 4.0 | **1.74×** |
| `latency_match_end_1kb` | **13.5** | 2.4 | 7.5 | 1.7 | 3.2 | 2.0 | **1.80×** |
| `latency_miss_1kb` | **13.4** | 4.6 | 7.9 | 4.9 | 5.4 | 3.9 | **1.69×** |
| `prose_hit_dense_1mb` | **13.1** | 0.0 | 6.8 | 1.0 | 4.2 | 2.9 | **1.93×** |
| `torture_miss_64kb` | **13.1** | 0.1 | 0.5 | 0.3 | 0.1 | 0.3 | **25.76×** |
| `code_hit_brackets_256kb` | **11.1** | 0.0 | 6.0 | 1.1 | 1.3 | 0.9 | **1.86×** |
| `multi_N8_hit_log_1mb` | **10.2** | 5.7 | – | 2.0 | 1.8 | 2.5 | **1.78×** |
| `multi_N512_miss_hazard_64kb` | **9.8** | 4.6 | – | 0.0 | 0.0 | 0.5 | **2.14×** |
| `ru_latency_miss_1kb` | **8.5** | 3.6 | – | 5.1 | 3.9 | 3.6 | **1.65×** |

#### Ice Lake (Xeon @ 2.6 GHz) — GB/s (higher is better; **bold** = casei, fastest on every row)

| row | casei | Vectorscan | veloz | PCRE2-JIT | StringZilla | rust/regex | casei vs #2 |
|---|---|---|---|---|---|---|---|
| `latency_match_start_1kb` | **118.2** | 2.5 | 63.6 | 4.2 | 3.7 | 3.1 | **1.86×** |
| `samechar_miss_64kb` | **71.7** | 39.0 | 6.9 | 22.7 | 11.0 | 0.6 | **1.84×** |
| `prose_miss_1mb` | **57.2** | 16.7 | 6.8 | 16.4 | 12.2 | 9.5 | **3.43×** |
| `log_miss_1mb` | **57.1** | 45.0 | 6.8 | 21.8 | 12.2 | 9.4 | **1.27×** |
| `code_miss_256kb` | **56.9** | 23.1 | 6.9 | 19.1 | 11.5 | 9.6 | **2.47×** |
| `log_miss_64kb` | **54.5** | 38.9 | 6.8 | 19.8 | 11.9 | 9.3 | **1.40×** |
| `log_needle32_64kb` | **52.8** | 6.9 | 6.9 | 16.0 | 11.0 | 9.4 | **3.31×** |
| `log_needle8_64kb` | **52.8** | 6.9 | 6.9 | 16.1 | 15.5 | 9.2 | **3.28×** |
| `log_needle16_64kb` | **52.8** | 28.2 | 6.9 | 15.8 | 11.7 | 9.3 | **1.87×** |
| `log_needle3_64kb` | **52.6** | 38.9 | 6.9 | 16.5 | 15.3 | 14.3 | **1.35×** |
| `multi_N8_miss_ru_1mb` | **37.0** | 6.1 | – | 16.5 | 0.8 | 9.5 | **2.24×** |
| `multi_N8_hazard_hit_1mb` | **35.4** | 7.7 | – | 3.0 | 1.0 | 33.0 | **1.07×** |
| `multi_N64_miss_ru_64kb` | **35.2** | 5.8 | – | 15.7 | 0.1 | 0.5 | **2.24×** |
| `multi_N8_miss_log_1mb` | **31.1** | 7.0 | – | 13.3 | 1.6 | 9.5 | **2.33×** |
| `periodic_miss_64kb` | **30.9** | 0.5 | 6.9 | 23.5 | 11.0 | 0.6 | **1.32×** |
| `multi_N64_miss_log_64kb` | **29.5** | 6.9 | – | 20.4 | 0.2 | 0.5 | **1.45×** |
| `multi_N512_miss_log_64kb` | **29.5** | 6.9 | – | 13.8 | 0.0 | 0.5 | **2.13×** |
| `log_hit_sparse_1mb` | **27.6** | 1.5 | 6.7 | 6.9 | 10.3 | 6.8 | **2.67×** |
| `ru_miss_1mb` | **21.7** | 17.4 | – | 16.4 | 6.4 | 9.6 | **1.25×** |
| `kelvin_hazard_1mb` | **21.0** | 1.9 | – | 1.1 | 12.3 | 8.9 | **1.70×** |
| `multi_N8_miss_hazard_1mb` | **18.5** | 7.5 | – | 0.3 | 0.9 | 3.1 | **2.46×** |
| `latency_match_mid_1kb` | **18.5** | 2.0 | 12.0 | 2.3 | 3.2 | 2.4 | **1.54×** |
| `ru_hit_sparse_1mb` | **18.3** | 0.9 | – | 16.0 | 6.2 | 8.8 | **1.15×** |
| `multi_N2_miss_log_1mb` | **15.0** | 11.6 | – | 0.7 | 5.9 | 5.8 | **1.29×** |
| `log_miss_1kb` | **13.3** | 4.4 | 6.6 | 4.6 | 4.8 | 3.6 | **2.03×** |
| `latency_miss_1kb` | **12.8** | 3.8 | 6.6 | 4.2 | 4.7 | 3.6 | **1.95×** |
| `prose_hit_dense_1mb` | **12.0** | 0.0 | 5.8 | 1.0 | 3.6 | 2.8 | **2.06×** |
| `latency_match_end_1kb` | **11.0** | 2.0 | 6.2 | 1.6 | 2.9 | 2.0 | **1.79×** |
| `torture_miss_64kb` | **10.2** | 0.1 | 0.4 | 0.2 | 0.1 | 0.3 | **25.51×** |
| `multi_N8_hit_log_1mb` | **10.1** | 5.9 | – | 1.9 | 1.7 | 2.8 | **1.71×** |
| `multi_N512_miss_hazard_64kb` | **9.8** | 3.9 | – | 0.0 | 0.0 | 0.5 | **2.53×** |
| `code_hit_brackets_256kb` | **9.1** | 0.0 | 5.0 | 1.0 | 1.1 | 0.7 | **1.82×** |
| `ru_latency_miss_1kb` | **8.0** | 3.1 | – | 4.6 | 3.5 | 3.4 | **1.74×** |

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

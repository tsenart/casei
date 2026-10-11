# Direct Rebar audit

Rebar found the hole in the original `casei` benchmark. It closed the first
Unicode gap and exposed the broader ASCII performance work that remains.

## The answer in 30 seconds

At Rebar commit
[`463d00f`](https://github.com/BurntSushi/rebar/commit/463d00f31887e84c38467805b9e3122c314b9521),
18 performance workloads can be represented as one literal or a finite set of
literals. Five request Unicode-aware case folding and ask the same semantic
question as `casei`.

`casei` now wins all five on both measured CPUs:

| host | wins | median `casei / best` | worst row |
|---|---:|---:|---:|
| Ice Lake | 5/5 | 0.6441 | 0.8794 |
| Sapphire Rapids | 5/5 | 0.6716 | 0.8999 |

Values below 1.0 are wins. The selected field is Hyperscan 5.4.2, PCRE2 10.47
JIT, and rust/regex 1.12.4.

The full target is all 18 rows. Thirteen request ASCII-only case matching;
`casei` wins four and loses nine. Across the complete board it is 9/18 on each
host. All rows and losses appear below.

## The current Initiative

The [Casei Rebar Initiative](https://app.perfloop.ai/t/oss/init_y6kff75c02)
owns the 18/18 result. Its first verified Case traced one English loss to the
whole-input Unicode route used by long ASCII literals with width-changing fold
mates. The accepted executor keeps the 512-bit ASCII probe on clean gaps and
decodes bounded halos around sparse Unicode clusters.

On its focused Sapphire Rapids field workload, `x_vs_best` moved from 1.266 to
0.924 with four competitors and five entrants. Three repeated samples stayed
between 0.9240 and 0.9396. The verifier also ran the full 36-row arena and found
no losing sample. The [Case record](https://app.perfloop.ai/t/oss/case_gc5hfnthag)
contains the code, measurements, and checks.

The table below remains the latest complete two-host Rebar record. Moving it
from 9/18 requires a new three-pass run on Ice Lake and Sapphire Rapids, with
every row below 1.0.

## Before and after

The first Rebar audit was bad for `casei`:

```text
before

five Russian patterns
        |
        v
common first-byte filter -> too many survivors -> decoded Unicode replay
        |
        +----------------------------------------> about 9x behind Hyperscan

one Russian pattern
        |
        v
good interior anchor -> survivor -> decoded Unicode replay
                                      |
                                      +----------> behind native confirmation
```

The accepted follow-up keeps one plan and changes the work before replay:

```text
after

five Russian patterns
        |
        +-> exact shared-byte origin gate
        +-> tagged interior pairs for each literal
        +-> exact pair checks for surviving tags
        +-> the same raw fold-token plan ----------------------> 0.84x to 0.88x

one Russian pattern
        |
        +-> pair-pair screen
        +-> raw one/two/three-byte fold confirmation
        +-> confirmed source width ----------------------------> 0.29x to 0.90x
```

The multi-pattern screen returns pattern tags, not matches. The existing plan
still decides every result. The variable-width confirmer returns the number of
source bytes it proved, so `Matcher.Each` can continue without decoding that
match again.

The common-byte gate applies only when every simple-fold spelling of every
literal contains the same exact ASCII byte. It proves the latest possible
start before the first occurrence. Its AVX-512 kernel compares eight cache
lines before testing the ordered masks. If the proof cannot be compiled, the
route is absent.

## Why the original gym missed it

The original arena had first-match rows and five single-needle count rows. The
competitive bar once ignored the count flag on those five rows and timed only
the first match. That wiring was corrected before the earlier publication run,
and `casei` won the corrected rows.

Two gaps remained:

1. No row enumerated a compiled multi-pattern Unicode plan to the end of a
   corpus.
2. No focused row forced variable-width confirmation or the five-pattern raw
   transition shape from Rebar.

Perfloop optimized the board it was given. Rebar exposed the omitted shapes.
The acceptance board added three focused rows:

- `multi/multi_N1_unicode_pair_miss_1_5mb`
- `multi/multi_N5_raw_transition_miss_5mb`
- `multi/multi_N5_raw_transition_late_hit_5mb`

Those rows brought the checked-in acceptance board to 36. All 36 remain below
1.0 `x_vs_best` on both hosts after the Rebar work, against the field the arena
counted then; rust/regex counted on only one of those rows, and Rust
Aho-Corasick was left out of 20 of them (all 19 ASCII single rows and
`multi_N8_hit_log_1mb`). The current board adds two
complete-triple rows, bringing the next publication gate to 38.

## The benchmark contracts

| | casei arena | Rebar audit |
|---|---|---|
| answer | first byte offset, leftmost match, or overlap-allowed single-needle count | count or total span of every non-overlapping match |
| compilation | included in `IndexFold`; outside repeated `Matcher` searches | matcher compiled before timing, like the other Rebar engines |
| pattern sets | one and many | one and finite alternations |
| folding | Unicode simple folding on every row | Unicode on 5 rows, ASCII-only on 13 |
| timing | in-process paired windows, alternating order, three complete passes | Rebar runner protocol, three passes, pinned to one core |

The audit adapter validates complete enumeration against an independent
simple-fold source scan before warmup. The timed iteration calls `Matcher.Each`
with only a count or span sink.

## All 18 rows

`casei / best` is the median of three per-pass ratios. The named competitor is
the fastest by its three-pass median. Bold values are wins.

| Rebar row | requested folding | Ice Lake | Sapphire Rapids |
|---|---|---:|---:|
| `curated/01-literal/sherlock-casei-en` | ASCII-only* | 4.48× (Hyperscan) | 4.93× (Hyperscan) |
| `curated/01-literal/sherlock-casei-ru` | Unicode | **0.87×** (PCRE2-JIT) | **0.90×** (PCRE2-JIT) |
| `curated/02-literal-alternate/sherlock-casei-en` | ASCII-only* | 4.69× (Hyperscan) | 4.91× (Hyperscan) |
| `curated/02-literal-alternate/sherlock-casei-ru` | Unicode | **0.88×** (Hyperscan) | **0.84×** (Hyperscan) |
| `hyperscan/literal-casei-english-nosom` | ASCII-only* | 3.55× (Hyperscan) | 4.32× (Hyperscan) |
| `hyperscan/literal-casei-english-som` | ASCII-only* | 3.53× (Hyperscan) | 4.39× (Hyperscan) |
| `hyperscan/literal-casei-russian-nosom` | Unicode | **0.36×** (rust/regex) | **0.29×** (Hyperscan) |
| `hyperscan/literal-casei-russian-som` | Unicode | **0.36×** (rust/regex) | **0.29×** (Hyperscan) |
| `imported/leipzig/tom-sawyer-huckle-fin-insensitive` | ASCII-only* | 2.37× (Hyperscan) | 2.10× (Hyperscan) |
| `imported/leipzig/twain-insensitive` | ASCII-only* | 1.11× (Hyperscan) | 1.08× (Hyperscan) |
| `imported/sherlock/name-alt3-casei` | ASCII-only* | **0.70×** (rust/regex) | **0.63×** (rust/regex) |
| `imported/sherlock/name-alt5-casei` | ASCII-only* | **0.89×** (rust/regex) | **0.85×** (rust/regex) |
| `imported/sherlock/name-holmes-casei` | ASCII-only* | **0.57×** (PCRE2-JIT) | **0.55×** (PCRE2-JIT) |
| `imported/sherlock/name-sherlock-casei` | ASCII-only* | 1.54× (PCRE2-JIT) | 1.84× (PCRE2-JIT) |
| `imported/sherlock/name-sherlock-holmes-casei` | ASCII-only* | 2.50× (PCRE2-JIT) | 3.01× (PCRE2-JIT) |
| `imported/sherlock/the-casei` | ASCII-only* | **0.78×** (PCRE2-JIT) | **0.66×** (PCRE2-JIT) |
| `opt/prefilter/literal-casei-english` | ASCII-only* | 1.90× (PCRE2-JIT) | 2.34× (PCRE2-JIT) |
| `opt/prefilter/literal-casei-russian` | Unicode | **0.64×** (PCRE2-JIT) | **0.67×** (PCRE2-JIT) |

`*` Rebar disables Unicode-aware folding. `casei` would match Unicode fold
mates that these definitions exclude. The recorded corpora produce the
expected outputs, so every timing is comparable on that input. These rows stay
outside the historical 5/5 Unicode claim and inside the current 18/18 target.
ASCII is a subset users will search often, so a loss here remains a loss.

## Correctness and inventory closure

Every recorded invocation returned Rebar's expected answer on the 18
performance workloads. `casei` also passed the two compatible behavior checks.
The third behavior check is intentionally incompatible:

```text
test/unicode/case/ascii-only
pattern:  "s"
haystack: "ſ"
Rebar with unicode=false: 0 matches
casei simple folding:      1 match
```

The remaining case-insensitive Rebar definitions use general regex features:
classes, captures, repetition, boundaries, inline flags, or compile-time rule
sets. They cannot be represented by a literal or finite literal-set API. The
18 rows above account for every caseless Rebar workload that can.

## Measurement record

| item | recorded value |
|---|---|
| Rebar source | `463d00f31887e84c38467805b9e3122c314b9521` |
| selected field | Hyperscan 5.4.2, PCRE2 10.47 JIT, rust/regex 1.12.4 |
| hosts | GenuineIntel family 6/model 106 Ice Lake and family 6/model 143 Sapphire Rapids, both with AVX-512F/BW/VBMI |
| passes | three per host |
| warmup bound | 100 iterations or 200 ms |
| measurement bound | 1,000 iterations or 500 ms |
| CPU placement | runner and child engines pinned to core 2 |
| plan setup | outside the timed iteration |

The adapter, pinned integration script, six raw CSVs, receipt hashes, and ratio
calculator are checked in under [`audit/rebar/`](audit/rebar/README.md).

```sh
(cd audit/rebar/results && sha256sum -c SHA256SUMS)
python3 audit/rebar/summarize.py
```

One upstream build detail is preserved in the reproducer. Rebar's vendored
PCRE2 snapshot contains `pcre2posix.c` without its unused header. The audit
omits that POSIX wrapper. Rebar uses the native PCRE2 API, so its JIT search
code is unchanged.

## Experiment history

The gaps were explored in public Perfloop Cases:

- [Sparse Unicode exception partitioning](https://app.perfloop.ai/t/oss/case_gc5hfnthag)
- [Shared interior UTF-8 anchors](https://app.perfloop.ai/t/oss/case_jws72csfa9)
- [Dispersed width-stable probes](https://app.perfloop.ai/t/oss/case_b2m0dmh5wa)
- [Raw byte confirmation](https://app.perfloop.ai/t/oss/case_tgkp9bs0r6)
- [Carry confirmed ends into repeated matching](https://app.perfloop.ai/t/oss/case_1jg4we7k3s)

The last case rejected the idea that API restart cost was the main problem.
The selective filter and native confirmation work survived. The current result
combines those pieces with the exact origin gate and wider assembly schedules.

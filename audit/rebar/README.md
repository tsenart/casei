# Rebar audit artifacts

This directory contains the adapter and measurements behind
[`REBAR.md`](../../REBAR.md).

The current receipts say:

- `casei` wins all 5/5 rows with the same Unicode contract on both hosts;
- the worst same-contract ratio is 0.8794 on Ice Lake and 0.8999 on Sapphire
  Rapids;
- across all 18 representable stress rows, including 13 ASCII-only contracts,
  `casei` wins 9/18 on each host.

The
[streaming-enumerator Case receipt](streaming-case.md)
tested one complete construction for the remaining gap and did not close it.
[`NOVELTY.md`](../../NOVELTY.md) records its measured result and the cells it
ruled out. The broader 18/18 outcome belongs to the
[Casei Rebar Initiative](https://app.perfloop.ai/t/oss/init_y6kff75c02), so a
failed construction becomes input to the next Case rather than an open-ended
rewrite of the same hypothesis.

The Initiative's first verified
[sparse-exception Case](https://app.perfloop.ai/t/oss/case_gc5hfnthag)
keeps the AVX-512 ASCII probe running across clean gaps and decodes only bounded
halos around Unicode clusters. Its focused Sapphire Rapids field ratio moved
from 1.266 to 0.924 with four competitors. This is a verified mechanism result;
the checked-in Rebar board remains 9/18 until the complete two-host run closes
the remaining rows.

## What is here

- [`runner/main.go`](runner/main.go) compiles a `Matcher` once, validates full
  non-overlapping enumeration against an independent simple-fold oracle, then
  times `Matcher.Each` or a single `Matcher.Find` query with only a scalar sink.
- [`prepare.py`](prepare.py) registers that runner on all 18 performance rows
  and three behavior checks in the pinned Rebar checkout.
- [`results/`](results/) contains three CSV passes from each host and their
  SHA-256 receipt file.
- [`summarize.py`](summarize.py) validates the inventory and error columns,
  selects the fastest competitor on each pass, and recomputes every ratio in
  `REBAR.md`.

## Measure one row with Go

[`runner/bench_test.go`](runner/bench_test.go) has `BenchmarkRebar`, with one
sub-benchmark for each of the 18 rows. The name is the Rebar id with `/`
replaced by `-`, for example
`BenchmarkRebar/curated-01-literal-sherlock-casei-en`. Each row uses the
pattern, model, haystack, and expected count from the pinned Rebar definition,
listed in [`runner/testdata/rows.tsv`](runner/testdata/rows.tsv). Before
timing, it runs the runner's `verifyEnumeration` and checks Rebar's count. It
then times the runner's `countMatches`, which is the operation Rebar times.

Fetch the haystacks first. The script checks each file against a pinned
sha256 and keeps files that are already correct:

```sh
audit/rebar/haystacks.sh audit/rebar/haystacks
```

A paired claim builds the test binary in each arm and runs one row. `OUT` and
the haystack directory must be absolute paths:

```sh
cd audit/rebar/runner && go test -c -o "$OUT/rebar.test" .
CASEI_REBAR_HAYSTACKS=/abs/path/to/haystacks "$OUT/rebar.test" \
  -test.run '^$' -test.bench '^BenchmarkRebar/<row>$' -test.benchtime=1s
```

The runner module's `replace` directive points at the repository root, so
each arm's binary measures that arm's own `casei` source. Without
`CASEI_REBAR_HAYSTACKS`, the benchmark reads `../haystacks` relative to the
current directory. Under `go test` in the runner directory, that is
`audit/rebar/haystacks`.

`BenchmarkSixPatternCountSpans` is a separate boundary guard, not a nineteenth
Rebar row or a field ratio. It retains a six-literal Matcher over the pinned
Sherlock fixture, preflights every match with `verifyEnumeration`, and times the
same count-spans `countMatches` operation. Its literal set is
`Sherlock`, `Sailor`, `Kelvin`, `Kettle`, `Irene`, and `India`, which compiles to
nine raw root triples and six ASCII prefixes.

## Verify the record

Verify the checked-in record from any directory:

```sh
(cd audit/rebar/results && sha256sum -c SHA256SUMS)
python3 audit/rebar/summarize.py
```

## Reproduce on a qualifying Linux host

Check out `casei` and Rebar as siblings:

```sh
git clone https://github.com/tsenart/casei.git
git clone https://github.com/BurntSushi/rebar.git
git -C rebar checkout 463d00f31887e84c38467805b9e3122c314b9521
cd rebar
python3 ../casei/audit/rebar/prepare.py
cargo build --release --bin rebar
./target/release/rebar build -e '^(casei|hyperscan|pcre2/jit|rust/regex)$'
```

`prepare.py` refuses any other Rebar commit. It also omits the unused
`pcre2posix.c` wrapper because this pinned snapshot lacks its header. Rebar's
native PCRE2 API and JIT sources are unchanged.

The exact 18-row selection is:

```sh
filter='^(curated/(01-literal|02-literal-alternate)/sherlock-casei-(en|ru)|hyperscan/literal-casei-(english|russian)-(no)?som|imported/leipzig/(twain-insensitive|tom-sawyer-huckle-fin-insensitive)|imported/sherlock/(name-(sherlock|holmes|sherlock-holmes|alt3|alt5)-casei|the-casei)|opt/prefilter/literal-casei-(english|russian))$'

taskset -c 2 ./target/release/rebar measure \
  -e '^(casei|hyperscan|pcre2/jit|rust/regex)$' \
  -f "$filter" \
  --max-warmup-iters 100 --max-warmup-time 200ms \
  --max-iters 1000 --max-time 500ms > rebar-audit-pass1.csv
```

Repeat for passes two and three. The checked-in record used core 2 on both
hosts. The runner validates every answer in an untimed preflight; a mismatch
appears in the CSV `err` column and makes `summarize.py` fail.

The compatible behavior checks can be run directly:

```sh
taskset -c 2 ./target/release/rebar measure --verify --verbose \
  -e '^casei$' \
  -f '^test/unicode/case/(ascii-with-unicode|unicode)$'
```

The excluded `test/unicode/case/ascii-only` row expects `s` to miss `ſ`.
Unicode simple folding requires them to match.

# Rebar audit artifacts

This directory contains the adapter and aggregate measurements behind
[`REBAR.md`](../../REBAR.md). It is evidence for the boundary of the public
claim, not another favorable benchmark suite.

- [`runner/main.go`](runner/main.go) is the exact adapter used on both hosts.
  It compiles a `Matcher` once, enumerates non-overlapping matches by calling
  `Find` on each remaining suffix, verifies each matched byte width, and emits
  rebar's timing/count samples after reading its KLV request.
- [`prepare.py`](prepare.py) registers that runner on all 18 representable
  performance workloads and all three relevant semantic checks in the pinned
  rebar checkout. It refuses any other rebar commit.
- [`results/`](results/) contains all six rebar CSVs: three passes on Ice Lake
  and three on Sapphire Rapids, plus their SHA-256 receipt file.
- [`summarize.py`](summarize.py) validates the receipts and recomputes every
  ratio and summary in `REBAR.md`.

Run the receipt calculation from any directory:

```sh
(cd audit/rebar/results && sha256sum -c SHA256SUMS)
python3 audit/rebar/summarize.py
```

## Reproduce on a qualifying Linux host

Check out casei and rebar as siblings. The audit files landed after the measured
casei commit, but the search implementation must still be byte-identical to
`3954dbe40e8e21c4c7b2e2716f22647dd7cd880c`; the first command below proves
that before rebar is prepared:

```sh
git clone https://github.com/tsenart/casei.git
git clone https://github.com/BurntSushi/rebar.git
git -C casei diff --exit-code \
  3954dbe40e8e21c4c7b2e2716f22647dd7cd880c -- \
  casei.go matcher.go plan.go root_amd64.go root_amd64.s root_other.go go.mod go.sum
git -C rebar checkout 463d00f31887e84c38467805b9e3122c314b9521
cd rebar
python3 ../casei/audit/rebar/prepare.py
cargo build --release --bin rebar
./target/release/rebar build -e '^(casei|hyperscan|pcre2/jit|rust/regex)$'
```

`prepare.py` also omits the unused `pcre2posix.c` wrapper because this pinned
rebar snapshot lacks its header. The native PCRE2 API and JIT sources used by
the runner are unchanged.

The exact performance selection is the 18 rows to which the script adds
`casei`. Run three passes using rebar's protocol, with 100 warm-up iterations
or 200 ms and 1,000 measured iterations or 500 ms. The checked-in CSVs were
produced with:

```sh
./target/release/rebar measure \
  -e '^(casei|hyperscan|pcre2/jit|rust/regex)$' \
  -f '^(curated/(01-literal|02-literal-alternate)/sherlock-casei-(en|ru)|hyperscan/literal-casei-(english|russian)-(no)?som|imported/leipzig/(twain-insensitive|tom-sawyer-huckle-fin-insensitive)|imported/sherlock/(name-(sherlock|holmes|sherlock-holmes|alt3|alt5)-casei|the-casei)|opt/prefilter/literal-casei-(english|russian))$' \
  --max-warmup-iters 100 --max-warmup-time 200ms \
  --max-iters 1000 --max-time 500ms > rebar-audit-pass1.csv
```

Repeat for passes two and three. Rebar's runner validates every answer while
measuring; any mismatch appears in the CSV's `err` column. The two compatible
Unicode behavior checks can also be isolated with:

```sh
./target/release/rebar measure --verify --verbose -e '^casei$' \
  -f '^test/unicode/case/(ascii-with-unicode|unicode)$'
```

The excluded `test/unicode/case/ascii-only` check is intentionally not made to
pass: it expects `s` not to match `ſ`, contrary to casei's fixed Unicode
simple-fold contract.

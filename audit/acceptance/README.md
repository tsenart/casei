# Historical two-host acceptance record

This directory holds the raw `BenchmarkBar` transcripts for the prior 36-row
board. The current board has 38 rows, so these receipts remain evidence for the
published snapshot rather than evidence for the new complete-triple rows.

| host | rows | worst median | worst sample | median speedup | entrants |
|---|---:|---:|---:|---:|---:|
| Ice Lake, family 6/model 106 | 36/36 | 0.9624 | 0.9736 | 1.80× | 5-7 |
| Sapphire Rapids, family 6/model 143 | 36/36 | 0.9716 | 0.9799 | 1.56× | 5-7 |

Every row has three samples. Every sample has `x_vs_best < 1`. `casei` reports
512-bit dispatch, Vectorscan reports a 512-bit VBMI database, and the verifier
of the time checked every other active entrant against its declared width.

These receipts predate the rule that every pinned entrant counts toward
`x_vs_best` (#29). The arena then left rust/regex out of a row when the audit
could not see its vector width. On each host it counted rust/regex only on
`single/code_hit_brackets_256kb`: 66 of 69 single-row samples and all 39
multi-row samples have `rure_active=0`. Rust Aho-Corasick was also left out of
20 rows on each host: the arena never timed it on single rows, so the 19 ASCII
single rows it supports carry no `rustac_active` metric, and all three samples
of `multi/multi_N8_hit_log_1mb` have `rustac_active=0`. They record the prior field. They are not a result against
the full field.

## Verify the receipts

```sh
(cd audit/acceptance && sha256sum -c SHA256SUMS)
sha256sum -c audit/acceptance/SOURCE_SHA256SUMS
python3 audit/acceptance/summarize.py
python3 scripts/verify_benchmarkbar.py --historical-36 \
  audit/acceptance/results/ice/benchmarkbar.txt
python3 scripts/verify_benchmarkbar.py --historical-36 \
  audit/acceptance/results/spr/benchmarkbar.txt
```

The receipt checksums in `SHA256SUMS` pass. The two verifier commands and
`summarize.py` now fail with `rure_active=0, want 1`, because the verifier requires every pinned
entrant. The historical mode requires the exact 36-row inventory, three samples
per row, at least two entrants, all dispatch metrics, and every `x_vs_best`
below 1.0. Its normal mode requires the current 38-row inventory.

[`ablations/`](ablations/README.md) removes the origin gate, variable raw
confirmation, and returned pattern tags one at a time. Each removal breaks its
claimed field or Rebar consumer on at least one target host.

## Measurement order

For each row, `BenchmarkBar` pairs `casei` separately with every eligible
competitor. A pair contains six 25 ms windows. The order alternates, giving
each operation three first positions. The median paired ratio is the row's
ratio against that competitor. The largest ratio is `x_vs_best` because it
corresponds to the fastest competitor.

The complete board was run three times with:

```sh
taskset -c 2 go test -run '^$' -bench '^BenchmarkBar$' \
  -benchtime 30x -count 3
```

Before timing, both hosts passed the full arena agreement suite against the
native field. The field was built from the pinned prepare scripts with
Vectorscan's AVX-512 VBMI target enabled.

## Native reachability

The three native entries added or materially changed by this result are
`literalSkipExact64`, `pairPairConfirmVBMI64`, and
`rawByteMultiAnchorSkip64`. One-shot GDB breakpoints observed all three while
their direct model tests passed on both hosts.

```sh
go test -c -o casei.test .
gdb -q -batch -x audit/acceptance/native-reachability.gdb ./casei.test
```

The command file is [`native-reachability.gdb`](native-reachability.gdb). The
captured outputs are [`results/ice/gdb-native.txt`](results/ice/gdb-native.txt)
and [`results/spr/gdb-native.txt`](results/spr/gdb-native.txt). Each receipt
must contain `HIT 1`, `HIT 2`, `HIT 3`, and a final `PASS`.
[`SOURCE_SHA256SUMS.historical`](SOURCE_SHA256SUMS.historical) pins the source,
direct tests, and GDB command file used to build both historical binaries. Both
hosts reported the same checksum stream,
`043dd919faa1d34cb26f2993f5f756e7f3265a10291e5889eecab67530cb27e9`.
[`SOURCE_SHA256SUMS`](SOURCE_SHA256SUMS) separately pins the current checkout;
it is not substituted into the historical receipts.

## Why failed runs are kept

[`methodology/`](methodology/) contains the earlier sequential-window runs.
They measured the candidate and field in distant windows and produced unstable
near-parity N=5 rows, including losing samples. Those receipts failed the
publication bar. They are retained so the change to paired timing and the
reason for it remain auditable.

[`results/`](results/) contains the paired acceptance run. The acceptance rule
did not change: one row at or above 1.0 fails the board.

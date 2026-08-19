# Contributing rules

These are binding rules for any change to the search implementation, not
advice. A change that violates one should not be proposed, however fast it is.

## 1. The deliverable is a result the field does not hold

This repository exists to produce a caseless search engine that does not
currently exist: the fastest correct one, on workloads the field does not
serve. Twelve constructions are closed by proof in `NOVELTY.md`, so a wholly
new state or transition is not the expected route and is no longer the bar.

The bar is a new **result**, reached however it is reached. Assembling known
techniques into an engine that holds a position nobody holds -- `x_vs_best < 1`
on rows the field contests, or correct caseless UTF-8 multi-needle search that
no shipped engine provides -- is the deliverable, and building it is required
rather than forbidden.

Two outcomes are still failures:

- a re-implementation of an existing engine's approach that produces no new
  result;
- a wrapper or port of a published technique.

## 2. Novelty must be argued, not assumed

A change claiming a new construction ships `NOVELTY.md` containing:

1. the claimed new state representation or block transition, stated precisely
   enough to disagree with;
2. the closest known constructions, each with a source (paper, repository, or
   engine), including the ones in `CONTEXT.md`;
3. what is combined from them and what result that combination produces.
   **Combining known techniques is legitimate and is how most real advances
   happen.** The test is whether the RESULT is new -- a capability nobody has,
   or a measured position nobody holds -- not whether every component is. A
   repacking that produces no new result is not enough; a combination that
   produces one is;
4. what evidence would falsify the claim, and the result of looking for it.

**Absence from `CONTEXT.md` is not evidence of novelty.** That document is one
sweep's catalog against eighty years of literature. Point 3 is where these
claims usually die.

## 3. A negative result is a result

If the assessment concludes every component is known art, record that in
`NOVELTY.md` with the sources -- and then build the thing anyway if the
combination reaches a result the field does not hold. Known components are not
a reason to stop; only a known *result* is.

Do not use a novelty finding as a reason to ship nothing.

State what would falsify the negative. That is what makes it usable by whoever
looks next, instead of merely discouraging.

## 3b. One refutation is not a round

A closed cell is a result, not a stopping condition. Refuting a construction
frees you to route around it, so generate the next one and refute that. Keep
going until the budget is spent or a construction survives.

Stopping after a single refutation is the failure mode this rule exists to
prevent: the reward for "still closed" and for "here is something that
survives" are the same, and the first is reachable in minutes. Report every cell
you closed, not just the last.

Record what you ruled out cheaply on paper as well as what you implemented. A
cell closed by a two-line argument is worth as much to the next reader as one
closed by a benchmark, and costs far less.

## 3c. Read the field's implementations

Every entrant in `arena/field.yaml` is open source and its implementation is
reachable. Read them -- at whatever level the answer lives: source, intrinsics,
generated assembly, or the disassembly of a built object -- and take every
technique worth taking.

This is the standing method for any row this engine does not lead. The winner is
doing something specific, that something is readable, and not having read it is
never the reason a row cannot move. Name what the leading implementation does
that this one does not, adopt it, adapt it to the shared plan, and where wider
registers allow more than the original technique needed, push past it rather
than porting it.

Combining known techniques is legitimate here; what has to be new is the result.
A construction assembled from the best of the field and then taken further with
an instruction set none of them target is exactly the deliverable.

Two limits. The candidate module must not import, link, execute or embed any
field implementation -- `scripts/check-baseline-isolation.sh` decides that. And
these are licensed works: reimplement from the technique, never copy the code.
Record in `NOVELTY.md` which technique came from where and what the combination
reaches that no source reaches alone.

## 4. Measure against the field, not against yourself

The scoreboard is `BenchmarkBar` in the `arena/` module. It reports
`x_vs_best`: this implementation's time divided by the fastest correct
alternative present.

**Build the native field first.** The arena's competitors are compiled from
pinned sources by the prepare scripts; without them the arena does not build,
which is deliberate -- a bar run that silently dropped the field once measured
rows at 0.31-0.90 that read 1.09 to 1202 the moment a real matcher entered.
The full acceptance run is:

```
sudo apt-get install -y cargo cmake curl libboost-dev pkg-config python3-pip
native="$(mktemp -d)"
cd arena
for dep in pcre2 vectorscan rure rustac stringzilla; do "./$dep/prepare.sh" "$native"; done
export PKG_CONFIG_PATH="$native/root/usr/lib/x86_64-linux-gnu/pkgconfig"
export PKG_CONFIG_SYSROOT_DIR="$native/root"
export LD_LIBRARY_PATH="$native/root/usr/lib/x86_64-linux-gnu"
go test -run '^$' -bench BenchmarkBar
```

CI builds this same pinned field and runs its agreement tests on every push.
Hosted CI runners do not satisfy the AVX-512 VBMI performance-host contract, so
the full `BenchmarkBar` acceptance run happens separately on a qualifying host.

Every row reports an `entrants` count and each entrant's dispatched vector
width. A row with `entrants` below 2 was measured against the floor alone: say
so when reporting it, and treat closing that gap as the work rather than the
number as a win. An entrant reporting a narrower width than the machine offers
is a handicapped opponent, not a beaten one.

A measurement with no competitor in it is not evidence. Two ways to produce
one, both of which have happened here:

- a benchmark written for this change, compared against its own previous value;
- a per-implementation lane of an arena benchmark
  (`BenchmarkIndexFold/<row>/candidate`, and likewise `/veloz`, `/regexp`,
  `/ceiling`). Those lanes exist for profiling. Reporting `/candidate` borrows
  the arena's authority for a number that never looked at the field.

### The acceptance bar

**Every row of `BenchmarkBar` must measure `x_vs_best` below 1.0.** Not a
subset, not a listed few, not "the mandatory ones" -- every row, ASCII and
UTF-8, single needle and multi needle. The goal is to be the fastest thing in
existence at this problem, so a single row above 1.0 is a row the field still
wins and the work is not done.

There are exactly two ways a row is excused, both narrow:

- **Ceiling-limited.** The best field implementation is within 5% of the
  exact-match ceiling, so a large multiple would mean beating memory bandwidth.
  Such a row instead requires this engine within 5% of that ceiling, and is
  reported separately with the ceiling number shown.
- **Not yet measurable.** No entrant exists that answers the same question.
  Then the row is not excused so much as unmeasured: say so, and wiring an
  entrant in becomes the work.

**A row is only measured if at least two entrants ran in it.** `x_vs_best`
against Go's `regexp` alone is a comparison with a scalar NFA floor, and a
number below 1.0 there says nothing about the field. Report the entrant count
for every row. A UTF-8 row with one entrant is an unoccupied tier, which is
missing work in this repository -- go wire a competitor in and measure again.

Do not report a lost row as a guardrail, a non-regression, or a diagnostic. A
row above 1.0 is a row that is losing. Name it, give its number, and say what
you intend to do about it.

These rows are the ones most likely to expose a construction that only looks
fast, so lead with them -- but they are a starting point for reporting, never
the definition of passing:

| row | competitor |
|---|---|
| `single/log_miss_1mb` | veloz NEON/AVX2 |
| `single/latency_miss_1kb` | short-input shape where plan setup shows up |
| `single/samechar_miss_64kb` | adversarial; linearity |
| `single/periodic_miss_64kb` | adversarial; self-similar input |
| `multi/multi_N512_miss_log_64kb` | aho-corasick |
| `multi/multi_N512_miss_hazard_64kb` | width-changing folds at N=512 |

## 5. One engine

`IndexFold` and `Matcher.Find` must be one package-owned compiled search plan
and one block-transition state machine. A single needle is the `N=1` plan of
that machine.

Prohibited as alternate engines: per-pattern `IndexFold` loops, regex
delegation, `strings.Index` fallback lookup, an unrelated KMP or Aho-Corasick
engine reachable at runtime, and benchmark-specific dispatch.

## 6. Baseline isolation

Search code must not import, link, execute, embed, or delegate lookup to any
implementation in `arena/field.yaml`. Baselines live in the `arena/` module.
`scripts/check-baseline-isolation.sh` enforces this.

**Calling a field competitor disqualifies the result regardless of the
benchmark** -- an engine that calls `veloz` cannot beat `veloz`, it can only add
overhead to it, and a ratio cannot tell you no search was invented.

## 7. Scope

Target amd64, plus a correct portable fallback. arm64/NEON is out of scope for
now and its absence is not a defect.

**The performance hosts have AVX-512 VBMI, not merely AVX2 or base AVX-512.**
Published measurements cover Intel Ice Lake (`genuineintel/6/106`) and Sapphire
Rapids (`genuineintel/6/143`), with `avx512f`, `avx512bw`, and `avx512vbmi`
present. Use them: 512-bit vectors, byte-granularity compares under BW,
`vpermb` for in-register table lookup under VBMI, and k-mask registers that
make set/liveness arithmetic native rather than emulated. A Skylake-SP host
(`genuineintel/6/85`) lacks VBMI and does not qualify for the full-strength
field comparison because Vectorscan cannot dispatch its strongest path there.

Gate every path on runtime feature detection with a portable fallback. Say
which ISA a measurement covers; cross-compilation proves portability, not
performance.

## 8. Correctness is not negotiable

Unicode simple folding, pinned to Go `regexp` `(?i)` by differential test and
fuzz. Byte offsets, leftmost match, and ties to the lowest pattern index are
part of the contract. The portable path must be exercised, not merely compiled.

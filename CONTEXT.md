# CONTEXT — the known frontier of case-insensitive substring search

This document catalogs techniques known to this problem as of August 2026,
with sources and measured numbers. Anything below is fair game to use,
combine, and tune — but using it is engineering, not invention.

## Novelty gate

**Absence from this document is not evidence of novelty.** This catalog is
incomplete by construction: it holds what one sweep happened to find, and the
literature on string search is eighty years deep. Treating "not in CONTEXT.md"
as "new" is circular, and it is how a re-implementation gets published as a
discovery.

A construction is a novelty candidate only after an adversarial exclusion pass
against the frozen field and the literature, stating:

1. the claimed new state representation or transition;
2. the closest known constructions, with sources;
3. why the claim is not an equivalent combination, repacking, or threshold
   variation of those constructions;
4. what evidence — a paper, an implementation, a benchmark — would falsify the
   claim, and the result of looking for it;
5. provenance for every non-trivial implementation file: who or what authored
   it, and from what.

Point 3 is where these claims usually die. Point 5 is not paperwork: if the
deliverable is that a machine invented this, then authorship has to be a
record, not an assertion in a README.

## 1. State of the art, measured

From rebar's published curated results (2025-12-19, x86-64), the
case-insensitive literal benchmark `sherlock-casei-en` (English, ASCII-heavy):

| engine | exact | caseless | caseless penalty |
|---|---|---|---|
| hyperscan | 32.0 GB/s | **29.2 GB/s** | ~1.1× |
| pcre2/jit | 26.3 GB/s | 18.8 GB/s | 1.4× |
| rust/regex | 29.4 GB/s | 10.5 GB/s | 2.8× |
| re2 | 12.1 GB/s | 2.6 GB/s | 4.7× |
| go/regexp | 4.2 GB/s | 46.3 MB/s | ~90× |

And `sherlock-casei-ru` (Russian — true UTF-8 case folding):

| engine | exact (ru) | caseless (ru) |
|---|---|---|
| pcre2/jit | 32.8 GB/s | **18.0 GB/s** |
| hyperscan | 4.3 GB/s | 7.4 GB/s |
| rust/regex | 35.6 GB/s | 8.4 GB/s |
| re2 | 768 MB/s | 948 MB/s |
| go/regexp | 2.1 GB/s | 48.6 MB/s |

Notably, rebar includes `rust/memchr/memmem` — the reference exact-match
substring engine — and it is absent from every caseless benchmark: no
dedicated caseless substring engine exists in that suite at all; the
caseless columns are contested only by general regex engines.

**Direct rebar audit (2026-08-17):** `casei` was wired into every caseless
literal or finite-alternation definition at rebar commit `463d00f`: 18
performance workloads and three semantic checks. Rebar's pinned
[performance models](https://github.com/BurntSushi/rebar/blob/463d00f31887e84c38467805b9e3122c314b9521/MODELS.md)
enumerate every non-overlapping match, rather than return the first. Five rows
enable Unicode folding and therefore share this repository's folding contract;
against the selected current leaders, the loop-over-`Find` adapter wins two and
loses three on both Ice Lake and Sapphire Rapids, with the five-pattern Russian
row losing by roughly 9× to Hyperscan. Thirteen additional performance rows
request ASCII-only case matching; they were run and output-verified, but
`casei` retains stronger Unicode semantics. [`REBAR.md`](REBAR.md) records
every row, both-host ratios,
the incompatible `s`/`ſ` behavior check, and the missing iterator work. These
measurements bound the result here to first-match search; rebar's count-all
numbers cannot be borrowed in support of it.

**Correction (v2, after a three-way prior-art sweep):** dedicated engines
DO exist, with different contracts:

- **StringZilla v4.5** (Dec 2025, "Full Unicode Search at 50× ICU Speed
  with AVX-512") ships a dedicated engine under **full** case folding
  (ß→ss, ligatures, up to 3-codepoint expansions). Architecture: fold-safe
  needle window (`[head][safe window][tail]`) scored by byte diversity,
  SIMD scan of only the window, per-hit head/tail verification, per-script
  hazard "alarms", folded-rune-stream and rolling-hash fallbacks. Full
  folding is a DIFFERENT contract from this arena: ClickHouse explicitly
  declined StringZilla because it "can return matches whose byte length
  differs from the needle, which diverges from the one-code-point
  contract". Under this arena's simple-fold (regex-compatible) semantics,
  some StringZilla matches are wrong.
- **ClickHouse `positionCaseInsensitiveUTF8`**: Volnitsky bigram hash with
  case-variant enumeration, falling back to a first-char two-form SIMD
  searcher; **hard-codes surrender** (`force_fallback = true`) whenever
  case forms differ in encoded length; the escape hatch is O(n·m).
- **Sneller** (Apr 2023): AVX-512 Unicode-aware ILIKE via fold-variant
  expansion (≤4 alternates/char) with serial runtime width-chaining ("the
  UTF-8 byte sequences of characters equal under case-folding do not need
  to have the same byte length").

**Measured quantification (StringZilla's own post, Zen 5, Leipzig 100MB
corpora):** cased scripts — English 12.8 GB/s, Vietnamese 4.3,
Russian/Ukrainian/Armenian **below their own 5 GB/s target**. Caseless
scripts (folding is a near-no-op) — Hebrew 34.5, Bengali 28.2, Chinese
20.1. Zen 5 single-core streaming bandwidth is ~40-50 GB/s, so on scripts
that actually have case the shipped state of the art runs at ~10-30% of
bandwidth. rebar's pcre2/jit posts 18.0 GB/s on Russian caseless
(different machine/corpus — same-box reproduction required before any
comparative claim). **The open target is the cased scripts.**

Before this repository, no dedicated engine in the surveyed field implemented
**simple folding** — the semantics of `regexp (?i)`, rust/regex, and this
arena. `casei` now does; [`NOVELTY.md`](NOVELTY.md) is explicit that its
components are known art and the claimed advance is the measured result.

In Go, the strongest published caseless search is
[mhr3/veloz](https://github.com/mhr3/veloz) `ascii.IndexFold` (NEON on arm64
with AVX2/SSE amd64 ports). Open PRs on that repository
([#2](https://github.com/mhr3/veloz/pull/2),
[#4](https://github.com/mhr3/veloz/pull/4),
[#5](https://github.com/mhr3/veloz/pull/5),
[#6](https://github.com/mhr3/veloz/pull/6)) carry a staged
adaptive-prefilter design measured (Apple M3 Max) at **74–77 GB/s** on
64KB–1MB miss scans — about 6× the single-pass kernel it replaces — with
published tables across M3 Max, Graviton 3, and Graviton 4. Those PRs, their
benchmark tables, and their design are all prior art for this arena, and
their techniques are itemized below.

The physics: on cached haystacks a modern core streams tens of GB/s
single-threaded, and the M3 Max numbers above show ~76 GB/s is reachable
*with* folding. Incumbent engines at 10–29 GB/s are nowhere near the
bandwidth ceiling. The headroom is real.

## 1b. Unicode simple folding: the terrain

Facts that shape any fast UTF-8 caseless algorithm (see Unicode
CaseFolding.txt, C+S mappings; implemented as `unicode.SimpleFold` in Go):

- Fold orbits are tiny — almost all have 2 members, a few have 3 (σ/ς/Σ,
  k/K/U+212A KELVIN, s/S/U+017F LONG S, å/Å/U+212B ANGSTROM, µ/U+03BC/Μ) —
  and there are only ~1,400 multi-member orbits in all of Unicode.
- **The ASCII hazard set is tiny and needle-computable**: for a pure-ASCII
  needle, the only non-ASCII code points that can participate in a match
  are the non-ASCII fold-mates of its letters — in practice K (U+212A,
  3 bytes) for k, ſ (U+017F, 2 bytes) for s, and nothing else. A caseless
  ASCII scan is *almost* semantically complete; what stops it is a finite,
  precomputable byte-pattern set.
- Windows change byte length across fold-mates (k = 1 byte, K = 3), so a
  fixed-window verify strategy cannot be sound in general; matching is per
  code point with an offset map back to bytes.
- Simple folding is locale-independent: İ (U+0130) and ı (U+0131) fold only
  to themselves. Full folding (ß→ss, length-changing in code points) is a
  DIFFERENT relation and explicitly out of scope here — it is what makes
  "caseless" ill-defined in many libraries; simple folding is what regex
  engines implement.
- `ToLower`/`ToUpper` normalization is NOT folding: it splits the sigma
  orbit (ς stays ς, Σ→σ), re-encodes bytes, and shifts offsets. The common
  idiom is both slow and wrong.

What engines do today for caseless UTF-8 literals: expand the literal's
case variants into alternations / byte classes and feed general multi-
pattern machinery — Teddy buckets (rust/regex; `(?i)She` becomes
`[Ssſ][Hh][Ee]` with mixed byte lengths), FDR (Hyperscan), UTF-8 automata
(regex-automata, RE2). That is the known approach, and the §1 Russian
numbers are its cost. Also known but unfused: simdutf-class SIMD UTF-8
validation/classification runs at tens of GB/s — nobody has fused that
classification with a search loop.

Anchoring and offset machinery already in shipped engines (v2 additions):

- **PCRE2 JIT `scan_prefix`** (~2014): builds its SIMD fast-forward window
  of per-offset code-unit alternatives and **truncates the window at the
  first caseless character whose fold mate encodes to a different UTF-8
  length** (`ord2utf(othercase) != len`) — offsets before that point are
  fold-width invariant. It then SIMD-scans the two most selective offsets
  in the safe window. Width-changing orbits can never BE the anchor
  (≤2 byte-forms per offset, first 12 code units, structural priority).
- **rust/regex inner-literal seeking**: picks a rare literal anywhere in
  the pattern, scans for it, then resolves the match START with an
  anchored **reverse regex scan** (with an explicit quadratic-behavior
  guard) — offset recovery by re-scan, never by invariance.
- **Hyperscan SOM**: start-of-match tracked at runtime with documented
  state cost; literals carry only byte-level `nocase`.
- **RE2**: caseless prefix acceleration is ASCII-only (`prefix_foldcase_`).
- **V8 irregexp**: precomputes finite cross-encoding hazard sets
  (`RangeContainsLatin1Equivalents`; Kelvin/long-s special-cased).
- **.NET `Ordinal.EqualsIgnoreCase_Vector`**: vectorized ASCII caseless
  kernel with all-ASCII check and Unicode-correct hand-off resuming at the
  failing chunk (UTF-16 equality, not UTF-8 search).
- **Scherer, Unicode mailing list, 2003**: the complete enumeration of
  simple case mappings that cross UTF-8 length boundaries — U+0130,
  U+0131, U+017F, U+1FBE, U+2126, U+212A, U+212B. The hazard set has been
  public for 23 years.

Literature frame (v2 additions): rare-position guards (Sunday 1990;
Hume & Sunday 1991), deterministic sampling (Vishkin 1991), alphabet
sampling / pivot-character scanning with position mapping back to source
(Claude-Navarro et al. 2012; Faro-Marino-Pavone CDS 2020), the caseless
Convert-Search-Verify pipeline and its worst-case repair (Lu et al. 2007),
**elastic-degenerate string matching** (sets of variable-length strings
per position vs solid text — the formal problem class; Iliopoulos-Kundu-
Pissis 2021), **generalized degenerate strings** (WABI 2018: fixed
per-position width ⇒ fixed total width ⇒ invariant internal offsets — the
formal ingredient, used for comparison problems, never for anchoring,
never over an encoding), and codeword-synchronization analysis in
compressed matching (tagged Huffman; Klein-Shapira). The SMART exact-
matching corpus (80+ algorithms) contains zero caseless variants.

## 1c. Unclaimed territory (v2 — the only known-open ground)

Three adversarial prior-art sweeps (engines, products, literature) could
not falsify exactly one construction: **the prefix-invariance lemma** —
select an anchor rune such that every PRECEDING pattern rune has a
fixed-encoded-width simple-fold orbit; the anchor's byte offset from any
match start is then provably invariant, so match start is recovered
arithmetically, with no head verification (StringZilla verifies heads per
hit) and no re-alignment (compressed-matching lineage), while the anchor
itself and later runes may have width-changing orbits (PCRE2 excludes
those from its window entirely). Secondary thin delta: a SIMD probe whose
second-form offset is conditioned on the first form's matched width
(Sneller chains widths serially instead). Everything else in this
document's §1b/§2-§8 space is dated public art; new work must exceed THIS
line, not the v1 line. For the system-level open cells (multi-pattern,
unified-engine territory), see §1d.

## 1d. Multi-pattern art and the open cells (v3)

The arena's end goal is one adaptive engine for elastic-degenerate byte
patterns: each pattern position is a small set of UTF-8 encodings under
simple folding; exact search is the singleton case, multi-needle the union.
The complete known art for the multi-pattern side:

**Classic algorithms.** Aho-Corasick (1975): linear worst case independent
of pattern count; the reference Rust crate ships three memory layouts and
ASCII-only caselessness, stating: "It is unlikely that support for Unicode
case folding will be added in the future... full Unicode handling requires
a fair bit of sophistication." Commentz-Walter (1979): BM-style shifts over
a set trie, O(mn) worst. Wu-Manber (1994): q-gram shift tables,
average-optimal, no worst-case bound; caseless via ASCII pre-lowering.
SBOM/factor oracles: average-optimal, quadratic worst, no caseless story.
Multi Rabin-Karp: the small-set fallback (aho-corasick packed layer).

**SIMD multi-literal engines.** Teddy (Hyperscan lineage, in aho-corasick):
1-3-byte nibble-mask fingerprints into <=8/16 buckets via PSHUFB, naive
per-bucket verify; best under ~100 patterns; caseless only as pre-expanded
alternates. FDR (Hyperscan, NSDI'19): SIMD extended shift-or over bucketed
domains, 80 to tens of thousands of literals, end-offsets only, separate
costly start-of-match subsystem; confirm-stage flood is the worst-case
hole. Harry (INFOCOM 2023): column-vector successor to FDR, 2-300 strings.
Vectorscan: community ARM/POWER fork pinned to Hyperscan 5.4 API; Intel
took Hyperscan proprietary from 5.5 (open development over). Snort/Suricata
MPSE: ASCII-caseless prefilter + exact re-verify, the two-stage shape in
production IDS for two decades.

**Dispatch frameworks.** Hyperscan: principled literal/FA decomposition
feeding an engine zoo — no single theory, per its own authors' posts.
rust/regex meta: unified composition whose author calls literal dispatch
"a dark art" — "it is impossible to know, before a search begins, how to
optimally choose." RE2::Set: linear, fold-correct, pattern-IDs only (no
offsets). Nobody claims dispatch DERIVED from one theory; the two
strongest frameworks explicitly disclaim it.

**Native fold-set attempts (weakened forms, shipped twice).** ClickHouse
MultiVolnitskyCaseInsensitiveUTF8: case-variant bigram hashing (up to 4
variants per boundary) — self-documented as failing whenever "characters
with lower and upper cases are represented by different number of bytes or
code points" (fallback to a second, slower engine; average-case only).
Quamina (Bray): CaseFolding.txt-driven byte-level automata unions — but
anchored whole-field equality, no SIMD, acknowledged DFA blowup (4k
patterns -> 4.4M states). icgrep/Parabix: bit-parallel transposed streams,
natively Unicode-caseless, single compiled expression, no per-pattern
dictionary semantics, no linearity theorem.

**Academic.** No published SIMD multi-pattern algorithm carries a linear
worst-case guarantee — the Faro-Kulekci/EPSM/MSSEF line is uniformly
O(n/m) best, O(nm) worst; Belazzougui's worst-case-optimal Word-RAM line
has no SIMD and no folding. Elastic-degenerate DICTIONARY matching exists
(SEA 2018) but with per-position sets on the TEXT side (genomics) — the
dual of this arena's object. Character-class dictionaries take user-given
classes, never derived fold-encoding sets.

**The open cells** (no shipped or published occupant; nearest miss noted):

1. simple-fold x multi x SIMD-speed — nearest: ClickHouse (partial +
   surrender).
2. simple-fold x multi x linear worst case, with offsets and substring
   semantics — nearest: RE2::Set (no offsets), Quamina (anchored, no
   complexity story).
3. simple-fold x multi x SIMD AND linear simultaneously — nothing close;
   this cell is empty even for EXACT multi-pattern (Teddy/FDR have no
   linearity theorem).
4. Per-position encoding sets as the native machine representation (not
   case-expansion) — shipped only in weakened forms (ClickHouse bigrams,
   Quamina automata).
5. One-theory-derived dispatch, any tier — disclaimed by every framework
   that comes close.
6. N=1-to-thousands continuity under one engine — Hyperscan switches
   families (noodle/Teddy/FDR) with separate confirm logic; aho-corasick
   switches layouts heuristically.

**Where novelty cannot live** (adversarially pre-conceded): the two-stage
SIMD-candidate+verify shape (Teddy/FDR/Snort own it); the per-position
fold-expansion SEMANTICS (UTS #18 specifies it); the bare idea of byte-
level fold-variant structures (ClickHouse and Quamina shipped weakened
versions independently). Novelty must live in: the fold-set formalism as
the native representation with engines DERIVED from it, the SIMD+linear
combination (open even for exact multi), offset-correct simple-fold multi
semantics at SIMD speed, and N-continuity under one theory.

### 1e. GitHub `casefold` / Blackbird (Jul 2026) -- the pre-folded ceiling, shipped

Neubeck & Orzell, GitHub engineering blog 2026-07-31, open-sourced as the Rust
crate `casefold` (github/rust-gems, v0.1.0, 2026-07-09). This is the closest
published art to anything that treats folding as a primitive rather than a cost,
and it is six days old. It is a citation in the source list above only because
that list predates reading it; the content is here.

**What it does.** `simple_fold(String) -> String` over exactly our semantics --
CaseFolding.txt statuses C and S, no full folds (`ss`), no Turkic. Table is a
paged bitmap plus run-length encoding, **~1.7 KB**, roughly 10x smaller than a
hash map of the same data.

**The trick.** The fold is byte-space arithmetic, not decode-fold-encode: the
folded character's UTF-8 bytes read as a `u32` equal the source bytes as a `u32`
plus a per-run constant. No code point is ever materialized. Width-changing
folds are handled in that arithmetic (U+212A KELVIN SIGN -> `k`, U+023A -> U+2C65).

**The counterintuitive result.** Removing the early-exit branch is what let it
vectorize: ASCII went 3.1 -> **>45 GiB/s** on one core, i.e. memory bandwidth.
Non-ASCII runs ~1 GiB/s. ASCII is lowercased in place in the caller's buffer;
a second allocation happens only on the first real fold.

**What this closes.** The `ceiling` row in `arena/` is not a theoretical limit.
Pre-folded exact search is a shipped design at memory bandwidth, so
"fold then search exactly" is an engineering option and not a thought
experiment. Any claim of the form "we avoid re-folding during verification" is
competing against this, not against a hypothetical.

**What it does not do.** It folds a *stream* and materializes the output, for
index building. It is not a search: there is no pattern, no offsets, no
leftmost/tie contract. A scan that materialized a folded haystack would pay the
memory traffic this design exists to stream through.

**`index_fold` -- the part worth staring at.** `index_fold(String) -> Vec<u8>`
applies the same simple fold and then projects **every character to exactly one
byte**: ASCII to its lowercased byte with the high bit clear, and every
multibyte character to `0x80 | (cp & 0x7F)` -- the low seven bits of the *folded*
code point with the high bit set unconditionally, so a multibyte character that
folds to ASCII (KELVIN SIGN) still yields `0x80 | b'k'` and never a bare ASCII
byte. GitHub built it as a primitive for case-insensitive n-gram *indexing*.

Two properties fall out that matter for search and that the blog does not
pursue, because indexing is not searching: the projection is **fixed-width**
(m characters become exactly m bytes, so variable UTF-8 width disappears from
the projected space), and it is **lossy** (a 7-bit collision is a false
positive, so any hit needs verification against the real fold). A projected-space
position also does not map to a haystack byte offset without carrying the
mapping.

## 2. Case-folding primitives (in-register, branchless)

All known ways to fold or compare ASCII case without a branch per byte:

- **OR 0x20 with a precomputed mask**: when the byte you compare against is
  known (a prefilter byte), fold the haystack with a single `OR` whose mask
  is 0x20 if that byte is a letter, 0x00 otherwise — chosen once at setup.
  The compare target is stored pre-lowercased. Variant: duplicate the whole
  scan loop for the non-letter case to delete even that OR.
- **Signed-range trick (x86)**: `PADDB 0x1f` maps 'a'..'z' to 0x80..0x99, a
  single signed `PCMPGTB` against 0x9a isolates lowercase, then subtract
  0x20 under that mask. Four ops, no table (veloz amd64).
- **Unsigned-wraparound detects**: `(b+133) ≥ᵤ 230` detects 'a'..'z' (the
  wraparound doubles as the upper bound); `(b+191) <ᵤ 26` detects 'A'..'Z';
  `((b|0x20)+159) <ᵤ 26` detects letters of either case.
- **Shifted-domain TBL fold (NEON)**: pre-subtract 0x60 so 'a'..'z' land on
  table indexes 1..26; a two-register `TBL` (0x20 at those slots, 0
  elsewhere, out-of-range → 0) yields the fold delta; comparisons can stay
  in the shifted domain, skipping the un-shift.
- **Allow-mask pair fold (veloz PR #7)**: to compare two vectors caselessly,
  compute `allow = TBL(table, (a & b) − 0x40)` and test
  `(a ^ b) BIC allow == 0`. One table lookup per *pair* instead of folding
  each operand: if the XOR difference is exactly bit 5 and both operands are
  letters, `a & b` indexes the 'A'..'Z' rows. 5 vector ops per 16 bytes.
- **XOR-tolerance verify (raw needle)**: `tolerable = 0x20 where
  (diff == 0x20 AND is_letter)`, then `diff ^ tolerable == 0`. Symmetric,
  no tables, ~9 ops.
- **Needle-derived fold mask (prefolded needle)**: when the needle is known
  lowercase, fold the haystack *only at positions where the needle byte is a
  letter* and compare exactly elsewhere: ~6 ops. The pattern-side fold is
  paid once at setup.
- **SWAR (scalar words)**: Mycroft-style range mask over 8 bytes,
  `folded = x − (mask >> 2)`.
- **Hash-domain fold**: map `c ∈ 'a'..'z' → c − 0x80, else c − 0x60` before
  hashing; both cases collapse to one value and the uniform shift cancels in
  comparison.
- **256-byte fold LUT** for scalar tails; **32-byte TBL** beats it in vector
  code.

## 3. Prefilter designs

- **Single rare-byte broadcast scan** (memchr-style stage 1): compare every
  haystack byte against one needle byte chosen for rarity, verify at hits.
- **Two-byte anchor at fixed offsets**: two comparison streams at
  `base+off1` and `base+off2` advanced in lockstep (two cursors — no
  cross-vector shifting or realignment needed); AND the two match masks.
  Candidates must match both anchors, quadratically rarer false positives.
- **first2/last2 word anchors, case-folded (veloz amd64)**: match 16-bit
  pairs from both ends of the needle with `PCMPEQW` on folded data; odd
  alignments via `PALIGNR(15)` against the *previous folded block* (no
  reload, no re-fold); collapse even/odd movemasks with
  `m & (m>>1) & 0x5555`. Middle-only verification afterwards — the four
  anchor bytes are already proven, needles ≤ 4 need no verify at all.
- **Anchor choice policies**: (a) positional — first+last byte, middle if
  they're equal (Two-Way/Muła lineage); (b) statistical — the two rarest
  distinct bytes by a background frequency table; (c) hybrid — statistical
  choice, overridden to first+last spread when the chosen pair is adjacent
  *and* both bytes are common (adjacent common bytes give correlated false
  positives on periodic text).
- **Teddy** (Rust regex / Hyperscan lineage): nibble-indexed shuffle
  prefilter matching up to 8 short literals at once; handles caseless by
  putting both case variants in the mask tables.
- **FDR** (Hyperscan): bucketed shift-or over reversed domain masks —
  Hyperscan's main literal engine; caseless via bucket duplication.
- **SVE2 `MATCH`**: one instruction tests each haystack byte against a
  16-token set — both case variants of two rare bytes interleaved gives a
  dual-anchor caseless prefilter in two instructions per vector; positions
  via `BRKB`+`CNTP`; tails via `WHILELT` predication (no mask tables).

## 4. Candidate extraction and iteration (movemask-less ISAs)

- **2-bit syndrome (ARM optimized-routines lineage)**: AND the compare mask
  with `0x4010040140100401`, two `ADDP.B16` folds compress 16 lanes into a
  32-bit word (2 bits/byte); first hit via `RBIT`+`CLZ`, then `>>1`.
- **SHRN-#4 nibble movemask**: narrow a 16-byte 0x00/0xFF mask to a 64-bit
  word (4 bits/byte); full-equality test via `CMN #1`.
- **Any-match gate before extraction**: `ADDP.D2` + `FMOV` (2 ops) tests
  "any lane set" in the hot loop; syndrome extraction is deferred until a
  hit is known. `UMAXV.4S` (4 wide lanes, fewer reduction stages) where the
  vector holds raw difference bits rather than lane masks.
- **Syndrome-persistent iteration**: keep the syndrome live in a GPR across
  candidate verifications; retire a failed candidate by clearing its bit
  (`BIC`), re-extract the next — never rescan memory for the next candidate
  in a chunk.
- **Superblock OR-trees with preserved partials**: 128-byte iterations
  reduce 8 compare masks through an OR tree but *retain* the per-64B
  intermediates, so locating the hit re-reduces the saved partials instead
  of re-comparing memory.

## 5. Verification

- **Full-window SIMD recompare** with any §2 primitive; `VEOR` + reduction.
- **Skip-known-bytes**: after a dual-anchor hit, verify only the middle
  (`needle[2 : n−2]`).
- **Masked-tail verify**: load a full 16B block past the tail, AND the
  difference with `tail_mask_table[remaining]` (16 prefix-ones rows,
  scaled-register load). Beware: this overreads by up to 15 bytes — safe
  only with explicit buffer-slack guarantees, otherwise it page-faults. The
  overread-free alternative assembles the tail from 8/4/2/1-byte scalar
  loads.
- **Positional slack as the bounds proof**: anchoring the scan at
  `haystack + off1` with trip count in candidate positions means every
  prefilter load is in bounds by construction — no page logic in the scan.

## 6. Rolling hashes, vectorized

- **Scalar Rabin-Karp** (Go stdlib): polynomial hash, PrimeRK = 16777619
  mod 2³², roll by one multiply and two multiply-adds per position.
- **SIMD block hashing**: extract bytes per 32-bit lane, per-lane Horner
  with p, p², p³, weight lanes by {p¹², p⁸, p⁴, 1}, horizontal add; 64-byte
  blocks combine via {p⁴⁸, p³², p¹⁶, 1} and a per-iteration p⁶⁴ multiply.
  Cuts the sequential dependency chain ~64×. Hash the needle and the first
  window in the same interleaved loop.
- **4-way parallel rolling (veloz PRs)**: keep rolling hashes for four
  consecutive alignments in vector lanes; compute per-step deltas
  `new − pow·old` (negate `pow` so removal becomes a multiply-add), build
  the four shifted delta windows with `EXT #12/#8/#4`, chain multiply-adds
  by p⁴..p; the only loop-carried operation is one multiply by p⁴ per four
  positions. Candidate gate: broadcast target hash, `CMEQ.S4`, max-reduce.
- **Fold inside the hash**: apply a §2 wraparound fold in the hashing loop
  and the hash domain itself becomes caseless. Prefolded-needle variant:
  hash the needle raw (already lowercase) and fold only haystack bytes —
  but see §8, this exact variant shipped broken.
- **Reversed-polynomial rolling hash** (Matt Sills): alternative formulation
  avoiding the leading-power multiply; implemented in the veloz lineage but
  never shipped.

## 7. Adaptive control

- **Stage escalation**: 1-byte filter → 2-byte filter → rolling hash, each
  stage stronger and costlier. Known ABI: kernels return
  `position | NOT_FOUND | (EXCEEDED-flag + resume position)` in one word,
  and the orchestrator re-slices the haystack so the next stage never
  rescans cleared territory.
- **Attrition budgets**: count *failed verifications*, bail to the next
  stage when `failures > B + scanned/K`. Published values: Go stdlib
  cutover ≈ 4 + n/16; veloz C lineage 4 + n/16 and 4 + n/256 with
  needle-length-adaptive thresholds; veloz hand kernels 32 + n/8
  (recomputed per failure). Evaluate the budget only when candidates exist
  so the clean hot path pays nothing.
- **Stage-skip heuristics by pattern statistics**: skip the 1-byte stage
  when the filter byte is too common (rank cutoffs ~200/240 of 255) or the
  haystack is small (< 2KB); go straight to the hash stage for long needles
  (> 64B) whose anchors are common (rank > 180).
- **Guaranteed-linear claims**: a hard budget per filter stage plus a
  linear terminal stage bounds *filter* work; note a 32-bit rolling hash
  still admits adversarial collision inputs (O(n·m) verification), so
  worst-case claims must be phrased carefully.

## 8. Rare-byte statistics

- **Background byte-frequency rank table** (Rust memchr lineage; corpus:
  CIA World Factbook, rustc source, Septuaginta), 0 = rarest.
- **UTF-8 lead-byte override**: force 0xC0–0xFF to "most common" so the
  selector prefers continuation bytes (which discriminate between
  characters) over lead bytes (shared across whole ranges).
- **Fold-aware ranks**: `rank_ci(letter) = rank(upper) + rank(lower)`, both
  case slots — a folded filter byte fires on both cases, so 'e'/'E' is even
  worse as a caseless filter than 'e' is as an exact one. (Caveat: rank
  tables are rank-ordered, not frequency-proportional, so the sum is a
  heuristic.)
- **One-pass rarest-distinct-pair selection** with demotion; O(1) sampled
  variants (8 spread positions) for one-shot calls.
- **Corpus-tuned tables**: build the rank table from a sample of the actual
  data (fold counts while building); published claims of ~2× from
  domain-tuned ranks.

## 9. Known traps (all bitten real implementations of this exact problem)

These are encoded as tests in this repository:

1. **0x20-adjacent non-letters**: `[`/`{`, `@`/`` ` ``, `]`/`}`, `\`/`|`,
   `^`/`~` differ by exactly bit 5 and must NOT match. A fold sequence that
   ORs 0x20 without a letter test, or a verify that compares against the
   wrong constant register, accepts them. A shipped NEON kernel in this
   problem's lineage had exactly that bug in its scalar-tail verify path,
   reachable for needles ≥ 16 bytes.
2. **Mixed fold directions**: one shipped rolling-hash variant folded the
   haystack toward *uppercase* while its verifier folded toward *lowercase*
   against a pre-lowercased needle — every letter-containing needle
   silently returned "not found," and no test caught it. Hash domain,
   verify domain, and needle normalization must agree, and the agreement
   must be tested.
3. **Verify-tail overreads**: masked-tail verification reads up to 15 bytes
   past the end of the needle or haystack. Language runtimes do not
   guarantee that slack; unmapped-page edges fault.
4. **Quadratic cliffs**: dual-anchor prefilters degrade to near-full
   verification per position on periodic inputs (`abab…`, `aaaa…`,
   `(a³¹b)ⁿ`). Without an attrition budget and a linear fallback, worst
   case is O(n·m). The adversarial scenarios exist to catch this.
5. **`ToLower` is not folding**: normalization idioms silently diverge from
   fold semantics on the sigma orbit and re-encode bytes, shifting offsets.
   Any "normalize then exact-search" design must use canonical *fold*
   normalization and carry an offset map.
6. **Length-changing windows**: fold-mates differ in UTF-8 byte length
   (k vs U+212A), so fixed-stride window verification is unsound beyond
   ASCII; the trap tests include such windows.
7. **Estimation is not measurement**: predicted speedups in this domain are
   routinely wrong in both directions. Every claim in this arena is a
   benchmark run, never an extrapolation.

## 10. Sources

- rebar (BurntSushi): benchmark harness + published results —
  github.com/BurntSushi/rebar
- Rust memchr crate: `memmem`, packed-pair prefilter, byte-frequency ranks
- mhr3/veloz + PRs #1–#8: Go SIMD ASCII library; staged caseless kernels,
  allow-mask EqualFold, amd64 folded dual-anchor
- Hyperscan (Intel) / Vectorscan: FDR and Teddy literal engines
- ARM optimized-routines: the 2-bit syndrome technique
- Wojciech Muła: SIMD-friendly string matching catalog
- SMART (Faro & Lecroq): the exact-matching algorithm corpus
- Go stdlib `strings`/`internal/bytealg`: Rabin-Karp cutover shape
- Matt Sills: reversed-polynomial Rabin-Karp
- Unicode CaseFolding.txt + UTS#18 (case-insensitive matching semantics)
- simdutf (Lemire et al.): SIMD UTF-8 validation/classification at GB/s
- StringZilla v4.5 (Vardanian): ashvardanian.com/posts/search-utf8
- PCRE2 JIT scan_prefix: src/pcre2_jit_compile.c
- Sneller: sneller.ai/blog/accelerating-ilike-using-avx-512
- ClickHouse: src/Common/Volnitsky.h, src/Common/StringSearcher.h
- V8: src/regexp/special-case.h; .NET: System/Globalization/Ordinal.cs
- Scherer 2003: unicode.org/mail-arch/unicode-ml/y2003-m07/0007.html
- ED strings: Iliopoulos, Kundu, Pissis, Inf. & Comp. 279 (2021); GD
  strings: Alzamel et al., WABI 2018; sampling: Vishkin SICOMP 1991,
  Claude-Navarro et al. JDA 2012, Faro-Marino-Pavone Algorithmica 2020;
  guards: Sunday CACM 1990, Hume & Sunday SPE 1991; caseless CSV: Lu et
  al. ICNS 2007; GitHub casefold (Jul 2026), see 1e
- Multi-pattern (v3): Aho-Corasick CACM 1975 + BurntSushi aho-corasick
  DESIGN.md/Teddy README; Hyperscan NSDI'19 + Langdale posts (2019, 2024);
  Harry INFOCOM 2023; Vectorscan (VectorCamp); ClickHouse Volnitsky.h
  (MultiVolnitskyCaseInsensitiveUTF8); Quamina (Bray) + Union-of-FA post
  2024; icgrep PACT 2014; RE2::Set; Belazzougui arXiv:1011.3441;
  multi-EDSM SEA 2018; Faro-Kulekci arXiv:1209.6449, MSSEF 2013; Snort
  fast_pattern docs + US 7,756,885; Suricata mpm docs; UTS #18.

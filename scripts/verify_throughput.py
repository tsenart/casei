#!/usr/bin/env python3
"""Verify and render the per-engine throughput surface used by the README."""

import argparse
from collections import defaultdict
import math
from pathlib import Path
import re
from statistics import median
import sys


sys.dont_write_bytecode = True
SCRIPT_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPT_DIR))
from verify_benchmarkbar import REQUIRED_ROWS, is_utf8_row  # noqa: E402


VISIBLE = (
    ("candidate", "casei"),
    ("vectorscan", "Vectorscan"),
    ("veloz", "veloz"),
    ("pcre2-jit", "PCRE2-JIT"),
    ("stringzilla", "StringZilla"),
    ("rure", "rust/regex"),
)


class VerificationError(ValueError):
    pass


def benchmark_name(raw):
    return re.sub(r"-[0-9]+$", "", raw)


def required_engines(row):
    common = {"candidate", "pcre2-jit", "rure", "vectorscan", "stringzilla"}
    if row.startswith("single/"):
        common.add("regexp")
        if not is_utf8_row(row):
            common.add("veloz")
    else:
        common.add("regexpAlt")
        if not is_utf8_row(row):
            common.add("rustac")
    return common


def eligible_engines(row):
    # The Go Aho-Corasick and per-pattern lanes are diagnostics. The exact-match
    # ceiling and ToLower+Index are controls, not answers to the same contract.
    return required_engines(row) - {"candidate"}


def parse(path):
    rows = defaultdict(lambda: defaultdict(list))
    with Path(path).open(errors="replace") as source:
        for line_number, raw in enumerate(source, 1):
            # GCE serial-console captures spell horizontal tabs as #011.
            fields = raw.replace("#011", " ").replace("\r", "").split()
            if not fields:
                continue
            name = benchmark_name(fields[0])
            if name.startswith("BenchmarkIndexFold/"):
                tier = "single"
                rest = name.removeprefix("BenchmarkIndexFold/")
            elif name.startswith("BenchmarkMatcher/"):
                tier = "multi"
                rest = name.removeprefix("BenchmarkMatcher/")
            else:
                continue
            parts = rest.rsplit("/", 1)
            if len(parts) != 2:
                raise VerificationError(
                    f"{path}:{line_number}: malformed benchmark name {fields[0]!r}"
                )
            scenario, engine = parts
            try:
                at = fields.index("MB/s")
            except ValueError:
                continue
            if at == 0:
                raise VerificationError(f"{path}:{line_number}: MB/s has no value")
            try:
                value = float(fields[at - 1])
            except ValueError as err:
                raise VerificationError(
                    f"{path}:{line_number}: invalid MB/s value {fields[at - 1]!r}"
                ) from err
            if not math.isfinite(value) or value <= 0:
                raise VerificationError(
                    f"{path}:{line_number}: non-positive or non-finite MB/s value"
                )
            rows[f"{tier}/{scenario}"][engine].append(value)
    return rows


def verify(path, expected_samples=3, require_wins=True):
    rows = parse(path)
    found = set(rows)
    if found != REQUIRED_ROWS:
        raise VerificationError(
            f"{path}: row inventory differs; "
            f"missing={sorted(REQUIRED_ROWS - found)}, "
            f"unexpected={sorted(found - REQUIRED_ROWS)}"
        )

    medians = {}
    for row, engines in rows.items():
        missing = required_engines(row) - set(engines)
        if missing:
            raise VerificationError(f"{path}: {row} missing engines {sorted(missing)}")
        wrong_counts = {
            engine: len(samples)
            for engine, samples in engines.items()
            if len(samples) != expected_samples
        }
        if wrong_counts:
            raise VerificationError(
                f"{path}: {row} has wrong sample counts {wrong_counts}"
            )
        values = {engine: median(samples) for engine, samples in engines.items()}
        alternatives = eligible_engines(row)
        best_engine = max(alternatives, key=lambda engine: values[engine])
        ratio = values["candidate"] / values[best_engine]
        if require_wins and ratio <= 1:
            raise VerificationError(
                f"{path}: {row} loses its throughput display to "
                f"{best_engine} ({ratio:.4f}x)"
            )
        medians[row] = (values, best_engine, ratio)
    return medians


def render(medians, title, selected=None):
    rows = REQUIRED_ROWS if selected is None else set(selected)
    unknown = rows - set(medians)
    if unknown:
        raise VerificationError(f"unknown selected rows: {sorted(unknown)}")
    ordered = sorted(rows, key=lambda row: medians[row][0]["candidate"], reverse=True)
    labels = [label for _, label in VISIBLE]
    lines = [
        f"#### {title}, GB/s (higher is better)",
        "",
        "| row | " + " | ".join(labels) + " | casei vs throughput #2 |",
        "|---|" + "---:|" * (len(labels) + 1),
    ]
    for row in ordered:
        values, _best, ratio = medians[row]
        cells = []
        for engine, _label in VISIBLE:
            if engine not in values:
                cells.append("–")
                continue
            value = f"{values[engine] / 1000:.1f}"
            cells.append(f"**{value}**" if engine == "candidate" else value)
        lines.append(
            f"| `{row.removeprefix('single/').removeprefix('multi/')}` | "
            + " | ".join(cells)
            + f" | **{ratio:.2f}×** |"
        )
    return "\n".join(lines)


def summary(medians):
    ratios = {row: medians[row][2] for row in REQUIRED_ROWS}
    narrowest = min(ratios, key=ratios.get)
    widest = max(ratios, key=ratios.get)
    return (
        f"PASS: {len(REQUIRED_ROWS)}/{len(REQUIRED_ROWS)} throughput rows; "
        f"narrowest {narrowest}={ratios[narrowest]:.2f}x; "
        f"widest {widest}={ratios[widest]:.2f}x; three samples per lane"
    )


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("transcript", type=Path)
    parser.add_argument("--samples", type=int, default=3)
    parser.add_argument("--markdown-title")
    parser.add_argument("--rows", help="comma-separated tier/name rows")
    args = parser.parse_args()
    if args.samples < 1:
        parser.error("--samples must be positive")
    try:
        medians = verify(args.transcript, args.samples)
        print(summary(medians))
        if args.markdown_title:
            selected = args.rows.split(",") if args.rows else None
            print()
            print(render(medians, args.markdown_title, selected))
    except (OSError, VerificationError) as err:
        print(f"FAIL: {err}", file=sys.stderr)
        raise SystemExit(1)


if __name__ == "__main__":
    main()

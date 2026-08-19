#!/usr/bin/env python3
"""Recompute the REBAR.md ratios from the six checked-in rebar CSVs."""

from collections import defaultdict
import csv
from pathlib import Path
from statistics import median

SAME_CONTRACT = {
    "curated/01-literal/sherlock-casei-ru",
    "curated/02-literal-alternate/sherlock-casei-ru",
    "hyperscan/literal-casei-russian-nosom",
    "hyperscan/literal-casei-russian-som",
    "opt/prefilter/literal-casei-russian",
}


def duration_ns(raw):
    scales = {"ns": 1, "us": 1_000, "ms": 1_000_000, "s": 1_000_000_000}
    for suffix, scale in scales.items():
        if raw.endswith(suffix):
            return float(raw[:-len(suffix)]) * scale
    raise ValueError(f"unrecognized duration {raw!r}")


def load_results(root):
    timings = {}
    for host in ("ice", "spr"):
        paths = sorted((root / host).glob("rebar-audit-pass*.csv"))
        if len(paths) != 3:
            raise SystemExit(f"{host}: found {len(paths)} passes, expected 3")
        for path in paths:
            run = int(path.stem.rsplit("pass", 1)[1])
            with path.open(newline="") as source:
                for row in csv.DictReader(source):
                    if row["err"]:
                        raise SystemExit(f"{path}: {row['name']}/{row['engine']}: {row['err']}")
                    key = (host, run, row["name"], row["engine"])
                    if key in timings:
                        raise SystemExit(f"duplicate result {key}")
                    timings[key] = duration_ns(row["median"])
    return timings


def ratios(timings, host):
    names = sorted({key[2] for key in timings if key[0] == host})
    if len(names) != 18:
        raise SystemExit(f"{host}: found {len(names)} benchmarks, expected 18")
    answer = {}
    for name in names:
        per_pass = []
        by_engine = defaultdict(list)
        for run in (1, 2, 3):
            row = {
                key[3]: value
                for key, value in timings.items()
                if key[:3] == (host, run, name)
            }
            casei = row.pop("casei")
            if not row:
                raise SystemExit(f"{host}/{name}/pass{run}: no competitor")
            per_pass.append(casei / min(row.values()))
            for engine, value in row.items():
                by_engine[engine].append(value)
        leader = min(by_engine, key=lambda engine: median(by_engine[engine]))
        answer[name] = (median(per_pass), leader)
    return answer


def summary(rows, names):
    values = [rows[name][0] for name in names]
    return sum(value < 1 for value in values), len(values), median(values), max(values)


def main():
    root = Path(__file__).resolve().parent / "results"
    timings = load_results(root)
    ice = ratios(timings, "ice")
    spr = ratios(timings, "spr")
    if ice.keys() != spr.keys():
        raise SystemExit("host benchmark inventories differ")

    print("| rebar row | Ice Lake | Sapphire Rapids |")
    print("|---|---:|---:|")
    for name in ice:
        iratio, ileader = ice[name]
        sratio, sleader = spr[name]
        print(f"| `{name}` | {iratio:.2f}x ({ileader}) | {sratio:.2f}x ({sleader}) |")

    for label, names in (("same-contract", SAME_CONTRACT), ("all", ice.keys())):
        for host, rows in (("Ice Lake", ice), ("Sapphire Rapids", spr)):
            wins, count, middle, worst = summary(rows, names)
            print(
                f"{label} / {host}: wins={wins}/{count}, "
                f"median={middle:.4f}x, worst={worst:.4f}x"
            )


if __name__ == "__main__":
    main()

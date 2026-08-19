#!/usr/bin/env python3
"""Wire casei into the pinned rebar checkout used by the direct audit."""

from pathlib import Path
import os
import re
import subprocess

REBAR_COMMIT = "463d00f31887e84c38467805b9e3122c314b9521"

SELECTED = {
    "benchmarks/definitions/curated/01-literal.toml": {
        "sherlock-casei-en", "sherlock-casei-ru",
    },
    "benchmarks/definitions/curated/02-literal-alternate.toml": {
        "sherlock-casei-en", "sherlock-casei-ru",
    },
    "benchmarks/definitions/hyperscan.toml": {
        "literal-casei-english-nosom", "literal-casei-english-som",
        "literal-casei-russian-nosom", "literal-casei-russian-som",
    },
    "benchmarks/definitions/imported/leipzig.toml": {
        "twain-insensitive", "tom-sawyer-huckle-fin-insensitive",
    },
    "benchmarks/definitions/imported/sherlock.toml": {
        "name-sherlock-casei", "name-holmes-casei",
        "name-sherlock-holmes-casei", "name-alt3-casei",
        "name-alt5-casei", "the-casei",
    },
    "benchmarks/definitions/opt/prefilter.toml": {
        "literal-casei-english", "literal-casei-russian",
    },
    "benchmarks/definitions/test/unicode/case.toml": {
        "ascii-only", "ascii-with-unicode", "unicode",
    },
}


def main():
    rebar = Path.cwd().resolve()
    if not (rebar / "benchmarks/engines.toml").is_file():
        raise SystemExit("run this from the root of a rebar checkout")
    head = subprocess.check_output(
        ["git", "rev-parse", "HEAD"], cwd=rebar, text=True,
    ).strip()
    if head != REBAR_COMMIT:
        raise SystemExit(f"rebar HEAD is {head}; expected {REBAR_COMMIT}")

    casei = Path(__file__).resolve().parents[2]
    runner = casei / "audit/rebar/runner"
    add_engine(rebar, runner)
    changed = add_benchmarks(rebar)
    patch_pcre2(rebar)
    print(f"casei is registered on all 21 audit definitions ({changed} newly changed)")


def add_engine(rebar, runner):
    path = rebar / "benchmarks/engines.toml"
    text = path.read_text()
    if re.search(r'^\s*name\s*=\s*"casei"\s*$', text, flags=re.M):
        return
    cwd = Path(os.path.relpath(runner, path.parent)).as_posix()
    block = f'''[[engine]]
  name = "casei"
  cwd = "{cwd}"
  [engine.version]
    bin = "./casei-rebar"
    args = ["version"]
  [engine.run]
    bin = "./casei-rebar"
  [[engine.build]]
    bin = "go"
    args = ["build", "-o", "casei-rebar", "."]

'''
    marker = "[[engine]]\n"
    if marker not in text:
        raise SystemExit(f"{path}: no engine insertion point")
    path.write_text(text.replace(marker, block + marker, 1))


def add_benchmarks(rebar):
    changed = 0
    total = 0
    for rel, names in SELECTED.items():
        path = rebar / rel
        text = path.read_text()
        parts = re.split(r"(?=^\[\[bench\]\]\s*$)", text, flags=re.M)
        found = set()
        for i, part in enumerate(parts):
            match = re.search(r'^name\s*=\s*"([^"]+)"', part, flags=re.M)
            if not match or match.group(1) not in names:
                continue
            found.add(match.group(1))
            total += 1
            if re.search(r"^\s*'casei',\s*$", part, flags=re.M):
                continue
            part, count = re.subn(
                r"^(engines\s*=\s*\[\s*)$",
                r"\1\n  'casei',",
                part,
                count=1,
                flags=re.M,
            )
            if count != 1:
                raise SystemExit(f"{rel}/{match.group(1)}: no engines list")
            parts[i] = part
            changed += 1
        missing = names - found
        if missing:
            raise SystemExit(f"{rel}: missing {sorted(missing)}")
        path.write_text("".join(parts))
    if total != 21:
        raise SystemExit(f"found {total} definitions, expected 21")
    return changed


def patch_pcre2(rebar):
    path = rebar / "engines/pcre2/build.rs"
    text = path.read_text()
    if 'path.ends_with("pcre2posix.c")' in text:
        return
    old = '''            || path.ends_with("pcre2_ucptables.c")
'''
    new = '''            || path.ends_with("pcre2_ucptables.c")
            // The vendored 10.47 snapshot does not include pcre2posix.h.
            // Rebar uses the native PCRE2 API and does not need this wrapper.
            || path.ends_with("pcre2posix.c")
'''
    if old not in text:
        raise SystemExit(f"{path}: unexpected PCRE2 build source")
    path.write_text(text.replace(old, new, 1))


if __name__ == "__main__":
    main()

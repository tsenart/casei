#!/usr/bin/env bash
# Reproduce casei's benchmark: build the entire competitor field from source,
# then run the scoreboard. CI builds and correctness-checks the same pinned
# field on every push; the performance board requires the host contract below.
# Set CASEI_NATIVE_DIR to retain the native field outside the default temporary
# directory. CASEI_PREPARE_ONLY=1 stops after that unprivileged field build.
#
# Requirements: Go 1.24+ on x86-64 Linux with AVX2 and AVX-512F/BW/VBMI
# (Intel Ice Lake or newer).
# casei's benchmarked result is the AVX-512 path, and VBMI is required so
# Vectorscan can enter at full strength. casei also has AVX2 and portable
# scalar paths, but they are not benchmarked, and the native x86 field only
# builds on x86-64 — so the script refuses elsewhere rather than print an
# off-scope number.
set -euo pipefail

if ! command -v go >/dev/null 2>&1; then
  echo "Go 1.24 or newer is required." >&2
  exit 1
fi
go_version="$(go env GOVERSION)"
if [[ ! "$go_version" =~ ^go([0-9]+)\.([0-9]+) ]] ||
  (( BASH_REMATCH[1] < 1 || (BASH_REMATCH[1] == 1 && BASH_REMATCH[2] < 24) )); then
  echo "Go 1.24 or newer is required; found '$go_version'." >&2
  exit 1
fi

arch="$(uname -m)"
if [ "$arch" != "x86_64" ]; then
  echo "casei's benchmarked result is the x86-64 AVX-512 path; this host is '$arch'." >&2
  echo "casei still runs correctly here (portable scalar path — no NEON kernel yet), but that path is not benchmarked and the native x86 field will not build here." >&2
  exit 1
fi
missing=()
for feature in avx2 avx512f avx512bw avx512vbmi; do
  if ! grep -qw "$feature" /proc/cpuinfo 2>/dev/null; then
    missing+=("$feature")
  fi
done
if [ "${#missing[@]}" -ne 0 ]; then
  echo "This host is missing required CPU features: ${missing[*]}." >&2
  echo "Use Intel Ice Lake or newer (for example, pin a GCP n2 to Ice Lake or use c3)." >&2
  echo "casei may run another backend here, but this script reproduces only the published AVX-512 result." >&2
  exit 1
fi

if command -v sudo >/dev/null 2>&1 && sudo -n true >/dev/null 2>&1; then
  echo "==> Installing build dependencies (cargo, cmake, boost, pkg-config)"
  sudo apt-get update -qq
  sudo apt-get install -y -qq cargo cmake curl libboost-dev pkg-config python3-pip build-essential
else
  # The arena builders are unprivileged. Permit a prepared container to use its
  # existing toolchain when sudo is unavailable or cannot run noninteractively.
  missing_tools=()
  for tool in cargo cc c++ cmake curl dpkg-deb make pkg-config python3 sha256sum tar; do
    if ! command -v "$tool" >/dev/null 2>&1; then
      missing_tools+=("$tool")
    fi
  done
  if [ "${#missing_tools[@]}" -ne 0 ] || [ ! -r /usr/include/boost/version.hpp ]; then
    echo "Build dependencies are missing and sudo is unavailable." >&2
    if [ "${#missing_tools[@]}" -ne 0 ]; then
      echo "Missing tools: ${missing_tools[*]}." >&2
    fi
    if [ ! -r /usr/include/boost/version.hpp ]; then
      echo "Missing Boost headers (install libboost-dev)." >&2
    fi
    exit 1
  fi
  echo "==> Using preinstalled build dependencies (sudo is unavailable)"
fi

root="$(cd "$(dirname "$0")/.." && pwd)"
native="${CASEI_NATIVE_DIR:-$(mktemp -d)}"
mkdir -p "$native"
export GOPATH="${GOPATH:-$native/go}"
export GOCACHE="${GOCACHE:-$native/go-build}"
# rure's Cargo registry and target tree are part of this field build. Keep them
# in the caller-owned native directory instead of a shared host CARGO_HOME.
export CARGO_HOME="$native/cargo-home"
cd "$root/arena"
for dep in pcre2 vectorscan rure rustac stringzilla; do
  echo "==> Building competitor from source: $dep"
  "./$dep/prepare.sh" "$native"
done

if [ "${CASEI_PREPARE_ONLY:-}" = 1 ]; then
  echo "==> Native field prepared in $native"
  exit 0
fi

export PKG_CONFIG_PATH="$native/root/usr/lib/x86_64-linux-gnu/pkgconfig"
export PKG_CONFIG_SYSROOT_DIR="$native/root"
export LD_LIBRARY_PATH="$native/root/usr/lib/x86_64-linux-gnu"

echo "==> Checking arena adapters"
go vet ./...
go test ./...

echo "==> Running the scoreboard (BenchmarkBar: x_vs_best per row, with per-entrant dispatched width)"
bar_output="$native/benchmarkbar.txt"
go test -run '^$' -bench '^BenchmarkBar$' -benchtime 30x -count 3 | tee "$bar_output"
python3 "$root/scripts/verify_benchmarkbar.py" "$bar_output" --samples 3

echo
echo "==> Per-competitor throughput (BenchmarkIndexFold / BenchmarkMatcher, MB/s per engine)"
throughput_output="$native/throughput.txt"
{
  go test -run '^$' -bench '^BenchmarkIndexFold$' -benchtime 100ms -count 3
  go test -run '^$' -bench '^BenchmarkMatcher$'   -benchtime 100ms -count 3
} | tee "$throughput_output"
python3 "$root/scripts/verify_throughput.py" "$throughput_output" --samples 3

echo
echo "==> Raw receipts retained in $native"

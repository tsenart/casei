#!/usr/bin/env bash
# Reproduce casei's benchmark: build the entire competitor field from source,
# then run the scoreboard. CI builds and correctness-checks the same pinned
# field on every push; the performance board requires the host contract below.
#
# Requirements: x86-64 Linux with AVX-512 VBMI (Intel Ice Lake or newer).
# casei's benchmarked result is the AVX-512 path, and VBMI is required so
# Vectorscan can enter at full strength. casei also has AVX2 and portable
# scalar paths, but they are not benchmarked, and the native x86 field only
# builds on x86-64 — so the script refuses elsewhere rather than print an
# off-scope number.
set -euo pipefail

arch="$(uname -m)"
if [ "$arch" != "x86_64" ]; then
  echo "casei's benchmarked result is the x86-64 AVX-512 path; this host is '$arch'." >&2
  echo "casei still runs correctly here (portable scalar path — no NEON kernel yet), but that path is not benchmarked and the native x86 field will not build here." >&2
  exit 1
fi
if ! grep -qw avx512f /proc/cpuinfo 2>/dev/null; then
  echo "This host has no AVX-512 (need Intel Ice Lake or newer, e.g. a GCP n2/c3)." >&2
  echo "casei would run its AVX2 path here, but the benchmarked result and the field build both require AVX-512." >&2
  exit 1
fi
if ! grep -qw avx512vbmi /proc/cpuinfo 2>/dev/null; then
  echo "This host has no AVX-512 VBMI, so Vectorscan cannot dispatch its strongest path." >&2
  echo "Use Intel Ice Lake or newer (for example, pin a GCP n2 to Ice Lake or use c3)." >&2
  exit 1
fi

echo "==> Installing build dependencies (cargo, cmake, boost, pkg-config)"
sudo apt-get update -qq
sudo apt-get install -y -qq cargo cmake curl libboost-dev pkg-config python3-pip build-essential

root="$(cd "$(dirname "$0")/.." && pwd)"
native="$(mktemp -d)"
cd "$root/arena"
for dep in pcre2 vectorscan rure rustac stringzilla; do
  echo "==> Building competitor from source: $dep"
  "./$dep/prepare.sh" "$native"
done

export PKG_CONFIG_PATH="$native/root/usr/lib/x86_64-linux-gnu/pkgconfig"
export PKG_CONFIG_SYSROOT_DIR="$native/root"
export LD_LIBRARY_PATH="$native/root/usr/lib/x86_64-linux-gnu"

echo "==> Running the scoreboard (BenchmarkBar: x_vs_best per row, with per-entrant dispatched width)"
go test -run '^$' -bench '^BenchmarkBar$' -benchtime 30x -count 3

echo
echo "==> Per-competitor throughput (BenchmarkIndexFold / BenchmarkMatcher, MB/s per engine)"
go test -run '^$' -bench '^BenchmarkIndexFold$' -benchtime 100ms -count 3
go test -run '^$' -bench '^BenchmarkMatcher$'   -benchtime 100ms -count 3

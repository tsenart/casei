#!/usr/bin/env bash
# Build the arena's pinned native entrants and test both amd64 backend builds.
#
# An optional first argument keeps the prepared native prefix for inspection.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
if [ "$#" -eq 0 ]; then
	native=$(mktemp -d)
	trap 'rm -rf "$native"' EXIT
else
	native=$1
fi

cd "$root/arena"
for dep in pcre2 vectorscan rure rustac stringzilla; do
	"./$dep/prepare.sh" "$native"
done
export PKG_CONFIG_PATH="$native/root/usr/lib/x86_64-linux-gnu/pkgconfig"
export PKG_CONFIG_SYSROOT_DIR="$native/root"
export LD_LIBRARY_PATH="$native/root/usr/lib/x86_64-linux-gnu${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
# cgo build cache keys do not include PKG_CONFIG_PATH. Keep this transient
# native prefix in its own Go cache so its archives cannot borrow another
# prefix's linker flags.
export GOCACHE="$native/go-build"
go test -count=1 ./...
GOEXPERIMENT=simd go test -count=1 ./...

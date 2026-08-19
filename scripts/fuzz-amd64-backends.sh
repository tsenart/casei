#!/usr/bin/env bash
# Fuzz one public API against both complete amd64 backends under one feature set.
#
# Usage: fuzz-amd64-backends.sh FuzzIndexFold|FuzzMatcher [GODEBUG] [fuzztime]
set -euo pipefail

case "${1:-}" in
FuzzIndexFold | FuzzMatcher) target="$1" ;;
*)
	echo "usage: $0 FuzzIndexFold|FuzzMatcher [GODEBUG] [fuzztime]" >&2
	exit 2
	;;
esac

godebug="${2:-}"
fuzztime="${3:-10s}"

GODEBUG="$godebug" go test -run '^$' -fuzz "^${target}$" -fuzztime="$fuzztime" -parallel=1 .
GODEBUG="$godebug" GOEXPERIMENT=simd go test -run '^$' -fuzz "^${target}$" -fuzztime="$fuzztime" -parallel=1 .

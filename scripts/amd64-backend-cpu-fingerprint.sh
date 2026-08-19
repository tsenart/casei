#!/usr/bin/env bash
# Print the CPU identity and ISA gates used by an amd64 backend A/B run.
set -euo pipefail

cpuinfo=${CPUINFO:-/proc/cpuinfo}
if [ ! -r "$cpuinfo" ]; then
	echo "cannot read $cpuinfo" >&2
	exit 1
fi

field() {
	awk -F: -v key="$1" '$1 ~ "^[[:space:]]*" key "[[:space:]]*$" { gsub(/^[[:space:]]+|[[:space:]]+$/, "", $2); print $2; exit }' "$cpuinfo"
}

flags=" $(field flags) "
printf 'vendor_id=%s\n' "$(field vendor_id)"
printf 'cpu_family=%s\n' "$(field 'cpu family')"
printf 'model=%s\n' "$(field model)"
printf 'model_name=%s\n' "$(field 'model name')"
for feature in avx2 avx512f avx512bw avx512vbmi; do
	case "$flags" in
	*" $feature "*) printf '%s=1\n' "$feature" ;;
	*) printf '%s=0\n' "$feature" ;;
	esac
done

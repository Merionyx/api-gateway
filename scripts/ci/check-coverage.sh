#!/usr/bin/env bash

# Run unit tests with coverage and compare total with COVERAGE_MIN_PERCENT, .coverage-min or default.
# Packages merionyx/api-gateway/pkg/* (generated protobuf) are excluded from the calculation

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

THRESHOLD_FILE="${ROOT}/scripts/ci/.coverage-min"
if [[ -n "${COVERAGE_MIN_PERCENT:-}" ]]; then
	MIN="${COVERAGE_MIN_PERCENT}"
elif [[ -f "$THRESHOLD_FILE" ]]; then
	MIN="$(tr -d '[:space:]' < "$THRESHOLD_FILE" | head -1)"
else
	MIN="25.0"
fi

PROFILE="${COVERAGE_PROFILE:-coverage.out}"

PKGS=$(go list ./... | grep -v 'merionyx/api-gateway/pkg/' | paste -sd' ' -)
if [[ -z "${PKGS// }" ]]; then
	echo "no packages to test" >&2
	exit 1
fi

go test $PKGS -count=1 -timeout=10m -coverprofile="$PROFILE" -covermode=atomic

line="$(go tool cover -func="$PROFILE" | grep '^total:' || true)"
if [[ -z "$line" ]]; then
	echo "no total line in cover -func output" >&2
	exit 1
fi

pct="$(echo "$line" | sed -n 's/.*(statements)[[:space:]]*\([0-9.]*\)%.*/\1/p')"
if [[ -z "$pct" ]]; then
	echo "could not parse coverage from: $line" >&2
	exit 1
fi

awk -v got="$pct" -v min="$MIN" 'BEGIN {
  if (got+0 < min+0) {
    printf "coverage %.1f%% is below minimum %.1f%%\n", got, min > "/dev/stderr"
    exit 1
  }
  printf "coverage %.1f%% (minimum %.1f%%, pkg/ excluded from denominator)\n", got, min
}'

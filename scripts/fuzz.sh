#!/bin/bash

# Run the fuzz targets in this repository.
#
# Each target runs for FUZZTIME (default 30s) with FUZZWORKERS workers.
# A crash is reported by the go tool; the failing input is written to the
# package's testdata/fuzz directory so it can be replayed with:
#
#   go test ./lib -run=FuzzCmdImportJSON/<hash>
#
# Usage: ./scripts/fuzz.sh [package...]
#
# With no arguments, every package holding fuzz targets is run.

set -u

DIR=$(dirname "$0")
ROOT=$DIR/..

FUZZTIME=${FUZZTIME:-30s}
FUZZWORKERS=${FUZZWORKERS:-}

if [ -n "$FUZZWORKERS" ]; then
    WORKERS_FLAG="-parallel=$FUZZWORKERS"
else
    WORKERS_FLAG=""
fi

if [ "$#" -gt 0 ]; then
    PACKAGES=("$@")
else
    PACKAGES=()
    for pkg in ./lib ./mmdbshrink/lib/shrink ./mmdbshrink/lib/verify ./mmdbshrink/lib/format ./mmdbshrink/lib/bench; do
        if grep -qr "^func Fuzz" "$ROOT/$pkg"/*_test.go 2>/dev/null; then
            PACKAGES+=("$pkg")
        fi
    done
fi

# The go tool fuzzes one target at a time, so each is run separately.
failed=0
for pkg in "${PACKAGES[@]}"; do
    targets=$(grep -ho "^func Fuzz[A-Za-z0-9_]*" "$ROOT/$pkg"/*_test.go 2>/dev/null |
        sed 's/func //' | sort -u)

    for target in $targets; do
        echo "==> $pkg $target"
        if ! go test "$pkg" -run "^${target}\$" -fuzz "^${target}\$" \
            -fuzztime="$FUZZTIME" $WORKERS_FLAG; then
            echo "FAIL: $pkg $target"
            failed=1
        fi
    done
done

exit $failed

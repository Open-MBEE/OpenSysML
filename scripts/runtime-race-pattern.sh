#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Split every runtime test, example and fuzz seed target between two CI runners.
set -euo pipefail

if [[ $# -ne 1 || ! "$1" =~ ^[12]$ ]]; then
  echo "usage: bash scripts/runtime-race-pattern.sh 1|2" >&2
  exit 2
fi

# Complementary first-letter ranges also cover future names and Unicode names.
# Short expressions avoid command-line length limits on Windows.
if [[ "$1" = 1 ]]; then
  pattern='^(Test|Example|Fuzz)[A-M]'
else
  pattern='^(Test|Example|Fuzz)([^A-M]|$)'
fi
listing=$(go test -list . ./internal/exec/runtime)
if ! printf '%s\n' "$listing" | tr -d '\r' | grep -E "$pattern" > /dev/null; then
  echo "error: runtime partition $1 has no tests" >&2
  exit 1
fi
printf '%s\n' "$pattern"

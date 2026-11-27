#!/usr/bin/env bash

# Checks scripts/ci-go-build-cache.sh against the go command itself: what a run used, a linked
# executable included, survives the pruning, and what it did not use is gone.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
script=$root/scripts/ci-go-build-cache.sh
# The toolchain go.mod names, whose cache layout the saved entry has.
GOTOOLCHAIN=$(go -C "$root" env GOVERSION)
work=$(mktemp -d)
trap 'chmod -R u+w "$work" 2>/dev/null; rm -rf "$work"' EXIT
export GOTOOLCHAIN GOCACHE=$work/cache GOFLAGS='' GOWORK=off CI_GO_BUILD_CACHE_STAMP=$work/aged

fail() { echo "FAIL: $*" >&2; exit 1; }

mkdir -p "$work/module/cmd" "$work/module/used" "$work/module/unused"
cd "$work/module"
printf 'module cachetest\n' >go.mod
printf 'package used\n\nfunc Value() int { return 1 }\n' >used/used.go
printf 'package unused\n\nfunc Value() int { return 2 }\n' >unused/unused.go
printf 'package main\n\nimport "cachetest/used"\n\nfunc main() { println(used.Value()) }\n' >cmd/main.go

# builds <subcommand> <package>: how many compile and link commands the go command runs for it.
builds() {
	local subcommand=$1 package=$2
	go "$subcommand" -x "$package" 2>&1 | grep -cE '/(compile|link) ' || true
}
entries() { find "$GOCACHE" -mindepth 2 -maxdepth 2 \( -name '*-a' -o -name '*-d' \) "$@" | wc -l; }

# A cold run: nothing to mark, and the pruning keeps all the run built.
"$script" age >/dev/null
go run ./cmd 2>/dev/null
go build ./unused
"$script" prune >/dev/null
[[ $(builds run ./cmd) -eq 0 ]] || fail "a cold run's pruning dropped what the run built"
[[ $(builds build ./unused) -eq 0 ]] || fail "a cold run's pruning dropped what the run built"
[[ $(entries -type d) -ge 1 ]] || fail "go run kept no linked executable in the build cache; the check below would be vacuous"

# A warm run that uses the command and not the other package.
"$script" age >/dev/null
[[ $(entries -newer "$CI_GO_BUILD_CACHE_STAMP") -eq 0 ]] || fail "age left an entry unmarked"
[[ $(builds run ./cmd) -eq 0 ]] || fail "marking the entries unused made the go command rebuild"
"$script" prune >/dev/null
[[ $(entries -type d) -ge 1 ]] || fail "the linked executable the run reused was pruned"
[[ $(builds run ./cmd) -eq 0 ]] || fail "an entry the run used was pruned: the command is rebuilt"
[[ $(builds build ./unused) -ge 1 ]] || fail "the package the run did not use is still cached"

# Without the marking, nothing is known to be unused and nothing is removed.
count=$(entries)
if "$script" prune >/dev/null 2>&1; then fail "prune ran without age"; fi
[[ $(entries) -eq $count ]] || fail "a refused prune removed entries"

echo "ok"

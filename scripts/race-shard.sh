#!/usr/bin/env bash
# Prints the root-module package paths assigned to one race-test shard.
# Usage: scripts/race-shard.sh runtime|runtime-corpus|model|export|rest|migrate
# runtime and runtime-corpus are one package, split by test name in make test-shard.
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ $# -ne 1 ]]; then
  echo "usage: scripts/race-shard.sh runtime|runtime-corpus|model|export|rest|migrate" >&2
  exit 2
fi

shard=$1
module=github.com/Open-MBEE/OpenSysML
runtime="$module/internal/exec/runtime"
workspace_model="$module/internal/workspace/model"
corpus="$module/tests/corpus"
smt="$module/internal/exec/smt"
export="$module/tests/export"
model="$module/tests/model"
passes="$module/internal/check/passes"
# tests/migrate runs the SysML v1 migrator over whole models and is the next-longest
# package under -race after the runtime, so it is a shard of its own.
migrate="$module/tests/migrate"
# tests/wasm runs in the WebAssembly gate job, not a race shard.
wasm="$module/tests/wasm"
named_packages=("$runtime" "$workspace_model" "$corpus" "$smt" "$export" "$model" "$passes" "$wasm" "$migrate")

case "$shard" in
  runtime|runtime-corpus|model|export|rest|migrate) ;;
  *)
    echo "usage: scripts/race-shard.sh runtime|runtime-corpus|model|export|rest|migrate" >&2
    exit 2
    ;;
esac

packages=$(go list ./...)
for package in "${named_packages[@]}"; do
  if ! grep -Fxq -- "$package" <<<"$packages"; then
    echo "error: scripts/race-shard.sh names $package, which is not a package of this module" >&2
    exit 1
  fi
done

case "$shard" in
  runtime|runtime-corpus)
    printf '%s\n' "$runtime"
    ;;
  model)
    printf '%s\n' "$workspace_model" "$corpus" "$smt"
    ;;
  export)
    printf '%s\n' "$export" "$model" "$passes"
    ;;
  migrate)
    printf '%s\n' "$migrate"
    ;;
  rest)
    while IFS= read -r package; do
      case " ${named_packages[*]} " in
        *" $package "*) ;;
        *) printf '%s\n' "$package" ;;
      esac
    done <<<"$packages"
    ;;
  *)
    echo "error: scripts/race-shard.sh: unknown shard $shard" >&2
    exit 2
    ;;
esac

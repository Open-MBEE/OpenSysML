#!/usr/bin/env bash
# Prints the root-module package paths assigned to one race-test shard.
# Usage: scripts/race-shard.sh runtime|runtime-corpus|model|export|graph|codegen|tools|rest|migrate|cli
# runtime and runtime-corpus are one package, split by test name in make test-shard.
set -euo pipefail

cd "$(dirname "$0")/.."

shards='runtime|runtime-corpus|model|export|graph|codegen|tools|rest|migrate|cli'

if [[ $# -ne 1 ]]; then
  echo "usage: scripts/race-shard.sh $shards" >&2
  exit 2
fi

shard=$1
module=github.com/Open-MBEE/OpenSysML
runtime="$module/internal/exec/runtime"
workspace_model="$module/internal/workspace/model"
corpus="$module/tests/corpus"
smt="$module/internal/exec/smt"
# tests/export and tests/model each run about 30 minutes under -race on a four-vCPU
# node and shared one until that node ran 35 minutes, so each is a shard of its own.
export="$module/tests/export"
model="$module/tests/model"
# internal/check/passes (~15 min under -race) rides with cmd/sysml.
passes="$module/internal/check/passes"
# tests/migrate runs the SysML v1 migrator over whole models and is the next-longest
# package under -race after the runtime, so it is a shard of its own.
migrate="$module/tests/migrate"
# tests/codegen's compiled-step budget alone runs 9-15 minutes under -race and held
# the rest node at 30, so it is a shard of its own.
codegen="$module/tests/codegen"
# tests/wasm is the tools shard, with the tools module (make test-shard); on GitHub
# Actions it runs in the WebAssembly gate job instead, not in a race shard.
wasm="$module/tests/wasm"
# cmd/sysml's exploration budgets take 23-30 of the coverage job's 30 minutes under
# -coverpkg=./... while sharing a node, so the command is a shard of its own.
cli="$module/cmd/sysml"
named_packages=("$runtime" "$workspace_model" "$corpus" "$smt" "$export" "$model" "$passes" "$migrate" "$codegen" "$wasm" "$cli")

case "$shard" in
  runtime|runtime-corpus|model|export|graph|codegen|tools|rest|migrate|cli) ;;
  *)
    echo "usage: scripts/race-shard.sh $shards" >&2
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
    printf '%s\n' "$export"
    ;;
  graph)
    printf '%s\n' "$model"
    ;;
  codegen)
    printf '%s\n' "$codegen"
    ;;
  tools)
    printf '%s\n' "$wasm"
    ;;
  migrate)
    printf '%s\n' "$migrate"
    ;;
  cli)
    printf '%s\n' "$cli" "$passes"
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

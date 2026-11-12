#!/usr/bin/env bash
# Prints the root-module package paths assigned to one race-test shard.
# Usage: scripts/race-shard.sh runtime|runtime-corpus|runtime-exploration|model|smt|corpus|corpus-roundtrip|corpus-apijson|export|export-library|export-examples|export-corpus|export-settled|graph|codegen|translate|grpc|tools|rest|migrate|cli | --sweeps
# A hyphenated shard is a by-test-name part of its package (runtime, corpus and export are each split
# that way in make test-shard); the other shards are whole packages, rest everything not named here.
set -euo pipefail

cd "$(dirname "$0")/.."

shards='runtime|runtime-corpus|runtime-exploration|model|smt|corpus|corpus-roundtrip|corpus-apijson|export|export-library|export-examples|export-corpus|export-settled|graph|codegen|translate|grpc|tools|rest|migrate|cli'
# The shards whose one test walks every example model, which the queue tier (make test-queue) runs race-free over the subset;
# export-library walks the bundled libraries, and grpc's and translate's sweeps share a shard with race-prone tests: all keep -race.
sweeps='export-corpus export-settled export-examples corpus-roundtrip corpus-apijson'

if [[ $# -eq 1 && $1 == --sweeps ]]; then
  printf '%s\n' $sweeps
  exit 0
fi

if [[ $# -ne 1 ]]; then
  echo "usage: scripts/race-shard.sh $shards | --sweeps" >&2
  exit 2
fi

shard=$1
module=github.com/Open-MBEE/OpenSysML
runtime="$module/internal/exec/runtime"
# internal/workspace/model (~26 min), tests/corpus (~39 min: its two round trips ~20 each) and
# internal/exec/smt (~17 min) shared one node that ran 41 minutes under -race, so each is its own.
workspace_model="$module/internal/workspace/model"
corpus="$module/tests/corpus"
smt="$module/internal/exec/smt"
# tests/export and tests/model each run about 30 minutes under -race on a four-vCPU
# node and shared one until that node ran 35 minutes, so each is a shard of its own.
export="$module/tests/export"
model="$module/tests/model"
# internal/translate/export (~20 min under -race) and tests/grpc with internal/frontend/grpc
# (~18 and ~13 min) held the rest node at 40-57 minutes, so they are shards of their own.
translate="$module/internal/translate/export"
grpc_tests="$module/tests/grpc"
grpc_frontend="$module/internal/frontend/grpc"
# internal/check/passes (~8 min under -race) rides with cmd/sysml.
passes="$module/internal/check/passes"
# tests/migrate runs the SysML v1 migrator over whole models (~13 min under -race).
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
named_packages=("$runtime" "$workspace_model" "$corpus" "$smt" "$export" "$model" "$translate" "$grpc_tests" "$grpc_frontend" "$passes" "$migrate" "$codegen" "$wasm" "$cli")

case "$shard" in
  runtime|runtime-corpus|runtime-exploration|model|smt|corpus|corpus-roundtrip|corpus-apijson|export|export-library|export-examples|export-corpus|export-settled|graph|codegen|translate|grpc|tools|rest|migrate|cli) ;;
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
  runtime|runtime-corpus|runtime-exploration)
    printf '%s\n' "$runtime"
    ;;
  model)
    printf '%s\n' "$workspace_model"
    ;;
  smt)
    printf '%s\n' "$smt"
    ;;
  corpus|corpus-roundtrip|corpus-apijson)
    printf '%s\n' "$corpus"
    ;;
  export|export-library|export-examples|export-corpus|export-settled)
    printf '%s\n' "$export"
    ;;
  graph)
    printf '%s\n' "$model"
    ;;
  codegen)
    printf '%s\n' "$codegen"
    ;;
  translate)
    printf '%s\n' "$translate"
    ;;
  grpc)
    printf '%s\n' "$grpc_tests" "$grpc_frontend"
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

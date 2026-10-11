#!/usr/bin/env bash
# Checks scripts/ci-changed-areas.sh against representative pull requests, in a
# throwaway repository so the cases are the only history.
set -euo pipefail

script=$(cd "$(dirname "$0")" && pwd)/ci-changed-areas.sh
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

git -C "$work" init -q
git -C "$work" config user.email ci@example.com
git -C "$work" config user.name CI
mkdir -p "$work/scripts"
cp "$script" "$work/scripts/"
git -C "$work" add scripts
git -C "$work" -c commit.gpgsign=false commit -qm base
base=$(git -C "$work" rev-parse HEAD)

failures=0

# case <name> <expected areas> <files...>
case_() {
  local name=$1 expected=$2
  shift 2
  git -C "$work" -c advice.detachedHead=false checkout -q "$base"
  git -C "$work" checkout -q -B "test-$name"
  for file in "$@"; do
    mkdir -p "$work/$(dirname "$file")"
    echo change >>"$work/$file"
    git -C "$work" add "$file"
  done
  git -C "$work" -c commit.gpgsign=false commit -qm "$name"

  local actual
  actual=$(cd "$work" && CI_TIER="${CI_TIER:-full}" bash scripts/ci-changed-areas.sh "$base" HEAD 2>/dev/null |
    grep '=true$' | cut -d= -f1 | sort | paste -sd, -)
  if [[ "$actual" != "$expected" ]]; then
    echo "FAIL $name (${CI_TIER:-full}): expected [$expected], got [$actual]" >&2
    failures=$((failures + 1))
  else
    echo "ok   $name (${CI_TIER:-full}): $actual"
  fi
}

# quick_case_ <name> <expected areas> <files...>: the same pull request under CI_TIER=quick.
quick_case_() {
  CI_TIER=quick case_ "$@"
}

case_ docs-only docs docs/guide/index.md
case_ overrides-only docs overrides/home.html
case_ changelog-only docs CHANGELOG.md
case_ changelog-fragment docs changes/unreleased/repl-thing.added.md
case_ java-module java,mdk client/java/opensysml-client/pom.xml
case_ java-manifest java,mdk,python client/java/pom.xml
case_ node-only node client/node/src/node/binary.ts
case_ node-manifest node,python client/node/package.json
case_ python-only python client/python/opensysml/connection.py
case_ rust-only rust client/rust/opensysml/src/connection.rs
case_ rust-manifest python,rust client/rust/opensysml/Cargo.toml
case_ rust-lock python,rust client/rust/Cargo.lock
case_ julia-only julia client/julia/OpenSysML/src/connection.jl
case_ matlab-only matlab client/matlab/+opensysml/call.m
case_ vscode-manifest python,vscode editors/vscode/package.json
case_ vscode-lock python,vscode editors/vscode/package-lock.json
case_ jupyterlab-only jupyterlab editors/jupyterlab/src/sysml.ts
case_ jupyterlab-test jupyterlab editors/jupyterlab/test/sysml.test.ts
case_ jupyterlab-manifest jupyterlab,python editors/jupyterlab/package.json
case_ jupyterlab-lock jupyterlab,python editors/jupyterlab/package-lock.json
case_ syson-frontend-manifest python,syson editors/syson/frontend/package.json
case_ syson-frontend-lock python,syson editors/syson/frontend/package-lock.json
case_ syson-manifest python,syson editors/syson/pom.xml
case_ mdk-manifest mdk,python editors/mdk/pom.xml
case_ mdk-child-manifest mdk,python editors/mdk/plugin/pom.xml
case_ syson-api-stubs-manifest python,syson editors/syson/syson-api-stubs/pom.xml
case_ syson-backend-manifest python,syson editors/syson/backend/pom.xml
case_ syson-readme docs,syson editors/syson/README.md
# The grammar generator and its committed output are held together by a Go test.
case_ vscode-grammar docs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode editors/vscode/tools/gengrammar/grammar.go
case_ vscode-syntaxes docs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode editors/vscode/syntaxes/sysml.tmLanguage.json
case_ jupyterlab-syntax docs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode editors/jupyterlab/src/syntax.json
# Any markdown counts as documentation: the site links out to repository files.
case_ man-page docs packaging/man/man1/sysml.1
case_ two-client-readmes docs,java,mdk,node client/java/README.md client/node/README.md
case_ two-clients java,mdk,node,python client/java/pom.xml client/node/tsconfig.json
case_ go-source docs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode internal/syntax/parser/parser.go
case_ go-client docs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode client/opensysml/client.go
case_ release-digests docs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode client/release-digests.json
case_ go-tools docs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode tools/gen/snapshot/main.go
case_ proto docs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode api/proto/sysml.proto
case_ install-script docs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode install.sh
case_ install-script-windows docs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode install.ps1
case_ conformance docs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode conformance/scenarios/01-server-info.json
case_ workflow docs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode .github/workflows/pr.yml
case_ unclaimed docs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode some-new-top-level/thing.txt

# The quick tier runs the Go suite for a service change but not every client; a
# client's or the docs' own paths still run it, and an unclaimed path is only Go.
quick_case_ go-source go internal/syntax/parser/parser.go
quick_case_ proto go api/proto/sysml.proto
quick_case_ workflow go .github/workflows/pr.yml
quick_case_ unclaimed go some-new-top-level/thing.txt
quick_case_ go-and-node go,node internal/syntax/parser/parser.go client/node/src/node/binary.ts
quick_case_ go-and-docs docs,go internal/syntax/parser/parser.go docs/guide/index.md
quick_case_ docs-only docs docs/guide/index.md
quick_case_ java-manifest java,mdk,python client/java/pom.xml
quick_case_ node-only node client/node/src/node/binary.ts
quick_case_ vscode-grammar go,vscode editors/vscode/tools/gengrammar/grammar.go

if [[ "$failures" -ne 0 ]]; then
  echo "$failures case(s) failed" >&2
  exit 1
fi
echo "all cases passed"

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
# Auto maintenance detaches after a commit and writes under .git while the trap
# removes it; a throwaway repository needs none.
git -C "$work" config maintenance.auto false
git -C "$work" config gc.auto 0
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
case_ java-readme docs,java,mdk,python client/java/README.md
case_ java-reference docs,python docs/reference/java-api.md
case_ java-skill docs,python packaging/agent-skills/skills/opensysml-java/SKILL.md
case_ node-only node client/node/src/node/binary.ts
case_ node-manifest node,python client/node/package.json
case_ python-only python client/python/opensysml/connection.py
case_ signed-release-fixture dotnet,python client/python/tests/fixtures/signed_release/SHA256SUMS.txt.bundle
# The .NET generator's committed goldens are generated from the Python client's multi-file model too.
case_ generator-fixture dotnet,python client/python/tests/fixtures/multifile/lib.sysml
case_ metamodel-reader-fixture dotnet,python client/python/tests/fixtures/metamodel/bigger.full.json
# The .NET metamodel tests hold the generated types against the metamodel table and hierarchy.
case_ metamodel-table docs,dotnet,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode internal/translate/rdf/ontology/table.go
case_ rust-only rust client/rust/opensysml/src/connection.rs
case_ rust-manifest python,rust client/rust/opensysml/Cargo.toml
case_ rust-lock python,rust client/rust/Cargo.lock
case_ julia-only julia client/julia/OpenSysML/src/connection.jl
case_ dotnet-only dotnet client/dotnet/src/OpenMBEE.OpenSysML/Internal/ReleaseDigests.cs
case_ dotnet-lock dotnet client/dotnet/src/OpenMBEE.OpenSysML/packages.lock.json
case_ dotnet-props dotnet,python client/dotnet/Directory.Build.props
case_ dotnet-sdk-pin dotnet global.json
case_ dotnet-readme docs,dotnet client/dotnet/README.md
# The .NET guide's walkthrough is the Vehicle sample's code, which the .NET suite holds together.
case_ dotnet-guide docs,dotnet docs/clients/dotnet.md
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
case_ vscode-grammar docs,dotnet,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode editors/vscode/tools/gengrammar/grammar.go
case_ vscode-syntaxes docs,dotnet,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode editors/vscode/syntaxes/sysml.tmLanguage.json
case_ jupyterlab-syntax docs,dotnet,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode editors/jupyterlab/src/syntax.json
# Any markdown counts as documentation: the site links out to repository files.
case_ man-page docs packaging/man/man1/sysml.1
case_ two-client-readmes docs,java,mdk,node,python client/java/README.md client/node/README.md
case_ two-clients java,mdk,node,python client/java/pom.xml client/node/tsconfig.json
case_ go-source docs,dotnet,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode internal/syntax/parser/parser.go
case_ go-client docs,dotnet,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode client/opensysml/client.go
case_ release-digests docs,dotnet,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode client/release-digests.json
case_ go-tools docs,dotnet,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode tools/gen/snapshot/main.go
case_ proto docs,dotnet,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode api/proto/sysml.proto
case_ install-script docs,dotnet,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode install.sh
case_ install-script-windows docs,dotnet,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode install.ps1
case_ action docs,dotnet,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode action.yml
case_ conformance docs,dotnet,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode conformance/scenarios/01-server-info.json
case_ workflow docs,dotnet,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode .github/workflows/pr.yml
case_ dependabot docs,dotnet,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode .github/dependabot.yml
# The npm runner is tested and its lockfile audited in the Go static job, so its package is a service path.
case_ mcp-lockfile docs,dotnet,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode packaging/npm/sysml-mcp/package-lock.json
quick_case_ mcp-lockfile go packaging/npm/sysml-mcp/package-lock.json
# The pysysml stub is audited in the Python job.
case_ pysysml python packaging/pypi-pysysml/pyproject.toml
# The Go hygiene suite caps every tracked fixture and test file, so these run it too.
case_ client-fixture go,python client/python/tests/testdata/large.json
case_ client-golden go,node client/node/test/render.golden.json
# The committed baselines and figure pages under docs/project are Go test inputs.
case_ project-baseline docs,go docs/project/pilot-differential-baseline.json
case_ project-figures docs,go docs/project/api-json-differential.md
case_ project-archive docs,go docs/project/archive/CHANGELOG-0.0-0.8.md
case_ unclaimed docs,dotnet,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode some-new-top-level/thing.txt

# The quick tier runs the Go suite for a service change but not every client; a
# client's or the docs' own paths still run it, and an unclaimed path is only Go.
quick_case_ go-source go internal/syntax/parser/parser.go
quick_case_ proto go api/proto/sysml.proto
quick_case_ workflow go .github/workflows/pr.yml
quick_case_ unclaimed go some-new-top-level/thing.txt
quick_case_ go-and-node go,node internal/syntax/parser/parser.go client/node/src/node/binary.ts
quick_case_ go-and-docs docs,go internal/syntax/parser/parser.go docs/guide/index.md
quick_case_ docs-only docs docs/guide/index.md
quick_case_ project-baseline docs,go docs/project/pilot-differential-baseline.json
quick_case_ java-manifest java,mdk,python client/java/pom.xml
quick_case_ node-only node client/node/src/node/binary.ts
quick_case_ dotnet-props dotnet,python client/dotnet/Directory.Build.props
# A draft touching only the .NET client, its gates, its SDK pin or the signing fixtures it
# serves still runs the .NET client job; a schema or scenario change waits for the full tier.
quick_case_ dotnet-only dotnet client/dotnet/src/OpenMBEE.OpenSysML/Internal/PrivateService.cs
quick_case_ dotnet-gates dotnet client/dotnet/scripts/dotnet-gates.py
quick_case_ dotnet-sdk-pin dotnet global.json
quick_case_ signed-release-fixture dotnet,python client/python/tests/fixtures/signed_release/SHA256SUMS.txt.bundle
quick_case_ generator-fixture dotnet,python client/python/tests/fixtures/multifile/top.sysml
quick_case_ metamodel-reader-fixture dotnet,python client/python/tests/fixtures/metamodel/bigger.full.json
quick_case_ dotnet-guide docs,dotnet docs/clients/dotnet.md
quick_case_ metamodel-table dotnet,go internal/translate/rdf/ontology/table.go
quick_case_ metamodel-hierarchy dotnet,go internal/translate/rdf/ontology/sysml-metaclasses.ttl
quick_case_ conformance go conformance/scenarios/01-server-info.json
quick_case_ vscode-grammar go,vscode editors/vscode/tools/gengrammar/grammar.go

if [[ "$failures" -ne 0 ]]; then
  echo "$failures case(s) failed" >&2
  exit 1
fi
echo "all cases passed"

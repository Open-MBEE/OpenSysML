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

# tier_case_ <name> <expected tier> <files...>: the tier the script decides with CI_TIER unset.
tier_case_() {
  local name=$1 expected=$2
  shift 2
  git -C "$work" -c advice.detachedHead=false checkout -q "$base"
  git -C "$work" checkout -q -B "tier-$name"
  for file in "$@"; do
    mkdir -p "$work/$(dirname "$file")"
    echo change >>"$work/$file"
    git -C "$work" add "$file"
  done
  git -C "$work" -c commit.gpgsign=false commit -qm "$name"

  local actual
  actual=$(cd "$work" && env -u CI_TIER -u CI_EVENT -u CI_FULL_LABEL bash scripts/ci-changed-areas.sh "$base" HEAD 2>/dev/null | sed -n 's/^tier=//p')
  if [[ "$actual" != "$expected" ]]; then
    echo "FAIL tier $name: expected $expected, got $actual" >&2
    failures=$((failures + 1))
  else
    echo "ok   tier $name: $actual"
  fi
}

# event_case_ <event> <label:true|false> <expected tier>: the tier for that event on the last case's head.
event_case_() {
  local event=$1 label=$2 expected=$3
  local actual
  actual=$(cd "$work" && env -u CI_TIER CI_EVENT="$event" CI_FULL_LABEL="$label" bash scripts/ci-changed-areas.sh "$base" HEAD 2>/dev/null | sed -n 's/^tier=//p')
  if [[ "$actual" != "$expected" ]]; then
    echo "FAIL tier for $event (ci:full=$label): expected $expected, got $actual" >&2
    failures=$((failures + 1))
  else
    echo "ok   tier for $event (ci:full=$label): $actual (on the last case's head)"
  fi
}

case_ docs-only docs docs/guide/index.md
case_ overrides-only docs overrides/home.html
case_ changelog-only docs CHANGELOG.md
case_ changelog-fragment docs changes/unreleased/repl-thing.added.md
case_ java-module java,mdk,syson client/java/opensysml-client/pom.xml
case_ java-source java,mdk,syson client/java/opensysml-client/src/main/java/org/openmbee/opensysml/Exploration.java
case_ java-manifest java,mdk,python,syson client/java/pom.xml
case_ java-readme docs,java,mdk,python,syson client/java/README.md
case_ java-reference docs,python docs/reference/java-api.md
case_ java-skill docs,python packaging/agent-skills/skills/opensysml-java/SKILL.md
case_ node-only node client/node/src/node/binary.ts
case_ node-manifest node,python client/node/package.json
case_ python-only python client/python/opensysml/connection.py
case_ signed-release-fixture python client/python/tests/fixtures/signed_release/SHA256SUMS.txt.bundle
# The .NET generator's committed goldens are generated from the Python client's multi-file model too.
case_ generator-fixture dotnet,python client/python/tests/fixtures/multifile/lib.sysml
case_ metamodel-reader-fixture dotnet,python client/python/tests/fixtures/metamodel/bigger.full.json
# The .NET metamodel tests hold the generated types against the metamodel table and hierarchy.
case_ metamodel-table docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode internal/translate/rdf/ontology/table.go
case_ rust-only rust client/rust/opensysml/src/connection.rs
case_ rust-manifest python,rust client/rust/opensysml/Cargo.toml
case_ rust-lock python,rust client/rust/Cargo.lock
case_ julia-only julia client/julia/OpenSysML/src/connection.jl
case_ dotnet-only dotnet client/dotnet/src/OpenMBEE.OpenSysML/Internal/Provisioning/ReleaseDigests.cs
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
case_ emacs-only emacs editors/emacs/sysml-mode.el
case_ emacs-test emacs editors/emacs/test/sysml-mode-test.el
# The grammar generator and its committed output are held together by a Go test.
case_ vscode-grammar docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode editors/vscode/tools/gengrammar/grammar.go
case_ emacs-genkeywords docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode editors/emacs/tools/genkeywords/elisp.go
case_ emacs-keyword-lists docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode editors/emacs/sysml-mode-keywords.el
case_ shared-keyword-groups docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode editors/internal/keywords/keywords.go
case_ vscode-syntaxes docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode editors/vscode/syntaxes/sysml.tmLanguage.json
case_ jupyterlab-syntax docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode editors/jupyterlab/src/syntax.json
# Any markdown counts as documentation: the site links out to repository files.
case_ man-page docs packaging/man/man1/sysml.1
case_ two-client-readmes docs,java,mdk,node,python,syson client/java/README.md client/node/README.md
case_ two-clients java,mdk,node,python,syson client/java/pom.xml client/node/tsconfig.json
case_ go-source docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode internal/syntax/parser/parser.go
# The derived-property differential runs on what can move its baseline (the writer, the referee,
# the bridge, the pin, the baseline itself) and on unclaimed paths, like every area; not on a parser change.
case_ apijson-writer apijson,docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode internal/translate/export/api_json_absence.go
case_ apijson-referee apijson,docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode tools/referee/apijson/provenance.go
case_ apijson-store apijson,docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode tools/referee/baseline/store.go
case_ apijson-bridge apijson,docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode scripts/pilot-json-exporter/ExportJSON.java
case_ apijson-pin apijson,docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode scripts/pilot-pin.sh
case_ apijson-baseline apijson,docs,go docs/project/api-json-differential/index.json
case_ apijson-fragment apijson,docs,go docs/project/api-json-differential/probes/probe.sysml.json
case_ go-client docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode client/opensysml/client.go
case_ release-digests docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode client/release-digests.json
case_ go-tools docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode tools/gen/snapshot/main.go
case_ proto docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode api/proto/sysml.proto
case_ install-script docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode install.sh
case_ install-script-windows docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode install.ps1
case_ action docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode action.yml
case_ conformance docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode conformance/scenarios/01-server-info.json
# A workflow-only change ran the twenty-minute differential for nothing, so pr.yml is not among its paths.
case_ workflow docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode .github/workflows/pr.yml
case_ other-workflow docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode .github/workflows/oracle-reproduction.yml
case_ dependabot docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode .github/dependabot.yml
# The npm runner is tested and its lockfile audited in the Go static job, so its package is a service path.
case_ mcp-lockfile docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode packaging/npm/sysml-mcp/package-lock.json
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
case_ project-compliance-source docs,go docs/project/spec-compliance/02-detailed-semantic-compliance-map/13-action.md
case_ unclaimed apijson,docs,dotnet,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode some-new-top-level/thing.txt

# The quick tier runs the Go suite for a service change but not every client; a
# client's or the docs' own paths still run it, and an unclaimed path is only Go.
quick_case_ go-source go internal/syntax/parser/parser.go
quick_case_ apijson-writer apijson,go internal/translate/export/api_json_absence.go
quick_case_ proto go api/proto/sysml.proto
quick_case_ workflow go .github/workflows/pr.yml
quick_case_ unclaimed go some-new-top-level/thing.txt
quick_case_ go-and-node go,node internal/syntax/parser/parser.go client/node/src/node/binary.ts
quick_case_ go-and-docs docs,go internal/syntax/parser/parser.go docs/guide/index.md
quick_case_ docs-only docs docs/guide/index.md
quick_case_ project-baseline docs,go docs/project/pilot-differential-baseline.json
quick_case_ java-manifest java,mdk,python,syson client/java/pom.xml
quick_case_ node-only node client/node/src/node/binary.ts
quick_case_ dotnet-props dotnet,python client/dotnet/Directory.Build.props
# A draft touching only the .NET client, its gates or its SDK pin still runs the .NET
# client job; a schema or scenario change waits for the full tier.
quick_case_ dotnet-only dotnet client/dotnet/src/OpenMBEE.OpenSysML/Internal/Lifecycle/PrivateService.cs
quick_case_ dotnet-gates dotnet client/dotnet/scripts/dotnet-gates.py
quick_case_ dotnet-sdk-pin dotnet global.json
quick_case_ signed-release-fixture python client/python/tests/fixtures/signed_release/SHA256SUMS.txt.bundle
quick_case_ generator-fixture dotnet,python client/python/tests/fixtures/multifile/top.sysml
quick_case_ metamodel-reader-fixture dotnet,python client/python/tests/fixtures/metamodel/bigger.full.json
quick_case_ dotnet-guide docs,dotnet docs/clients/dotnet.md
quick_case_ metamodel-table dotnet,go internal/translate/rdf/ontology/table.go
quick_case_ metamodel-hierarchy dotnet,go internal/translate/rdf/ontology/sysml-metaclasses.ttl
quick_case_ conformance go conformance/scenarios/01-server-info.json
quick_case_ vscode-grammar go,vscode editors/vscode/tools/gengrammar/grammar.go


# The tier: quick for a pull-request push whatever it touches (the merge queue runs the -race
# suite on every landing), full only for a path no area claims; CI_TIER forces it.
tier_case_ docs-only quick docs/guide/index.md
tier_case_ parser quick internal/syntax/parser/parser.go
tier_case_ node-only quick client/node/src/node/binary.ts
tier_case_ workflow quick .github/workflows/pr.yml
tier_case_ race-shards quick scripts/race-shard.sh Makefile
tier_case_ runtime quick internal/exec/runtime/executor.go
tier_case_ runtime-test quick internal/exec/runtime/robustness_test.go
tier_case_ smt quick internal/exec/smt/referee.go
tier_case_ grpc-frontend quick internal/frontend/grpc/cache.go
tier_case_ grpc-tests quick tests/grpc/conformance_test.go
tier_case_ grpc-command quick cmd/sysml-grpc/main.go
tier_case_ repl quick internal/frontend/repl/session.go
tier_case_ lsp quick internal/frontend/lsp/server.go
tier_case_ workspace quick internal/workspace/model/index.go
tier_case_ libs quick internal/workspace/libs/bundle.go
tier_case_ export-writer quick internal/translate/export/writer.go
tier_case_ symbols quick internal/semantic/symbols/index.go
tier_case_ unclaimed full some-new-top-level/thing.txt
tier_case_ parser-and-runtime quick internal/syntax/parser/parser.go internal/exec/runtime/executor.go

# The derived-property differential runs on a pull-request push (quick, on its paths) and under the full tier,
# never in the merge queue: the push already proved the same diff, and the nightly reproduces the baseline.
queue_case_() {
  CI_TIER=queue case_ "$@"
}
queue_case_ apijson-writer docs,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode internal/translate/export/api_json_absence.go
queue_case_ apijson-baseline docs,go docs/project/api-json-differential/index.json
queue_case_ unclaimed docs,emacs,go,java,julia,jupyterlab,matlab,mdk,node,python,rust,syson,vscode some-new-top-level/thing.txt
area_for_event_() {
  local event=$1 label=$2 area=$3 expected=$4
  local actual
  actual=$(cd "$work" && env -u CI_TIER CI_EVENT="$event" CI_FULL_LABEL="$label" bash scripts/ci-changed-areas.sh "$base" HEAD 2>/dev/null | sed -n "s/^$area=//p")
  if [[ "$actual" != "$expected" ]]; then
    echo "FAIL $area for $event (ci:full=$label): expected $expected, got $actual" >&2
    failures=$((failures + 1))
  else
    echo "ok   $area for $event (ci:full=$label): $actual (on the last case's head)"
  fi
}
tier_case_ apijson-writer quick internal/translate/export/api_json_absence.go
area_for_event_ pull_request false apijson true
area_for_event_ merge_group false apijson false
area_for_event_ pull_request true apijson true
area_for_event_ schedule false apijson true
area_for_event_ workflow_dispatch false apijson true
tier_case_ workflow-only quick .github/workflows/pr.yml
area_for_event_ pull_request false apijson false
area_for_event_ merge_group false apijson false

# The event decides above the paths: a merge-group commit runs the queue tier, a schedule or a
# dispatch the full tier, a pull request labelled ci:full the full tier, whatever they touch.
event_case_ pull_request false quick
event_case_ pull_request true full
event_case_ merge_group false queue
event_case_ schedule false full
event_case_ workflow_dispatch false full
# An unclaimed path is full on a pull request, but the queue stays the queue tier.
tier_case_ unclaimed-again full some-new-top-level/other.txt
event_case_ merge_group false queue
event_case_ pull_request false full

forced_case_() {
  local expected=$1
  shift
  local actual
  actual=$(cd "$work" && CI_TIER="$expected" bash scripts/ci-changed-areas.sh "$base" HEAD 2>/dev/null | sed -n 's/^tier=//p')
  if [[ "$actual" != "$expected" ]]; then
    echo "FAIL forced tier $expected: got $actual" >&2
    failures=$((failures + 1))
  else
    echo "ok   forced tier $expected (on the last case's head)"
  fi
}
forced_case_ full
forced_case_ queue
forced_case_ quick

if [[ "$failures" -ne 0 ]]; then
  echo "$failures case(s) failed" >&2
  exit 1
fi
echo "all cases passed"

#!/usr/bin/env bash
# Decides which CI areas a pull request has to exercise, from the files it touches.
#
# Usage: scripts/ci-changed-areas.sh <base-ref-or-sha> [head-ref-or-sha]
#
# Prints one `area=true|false` line per area, in the shape a GitHub Actions job
# output file takes. A file no area claims turns every area on, so a path this
# script has not been taught about is never silently untested.
set -euo pipefail

base=${1:?usage: ci-changed-areas.sh <base> [head]}
head=${2:-HEAD}

changed=$(git diff --name-only "$(git merge-base "$base" "$head")" "$head")

# The service and the schema the clients speak to, plus the shared conformance
# scenarios: a change here can change every client's answers, so all of them run.
# The extension's grammar generator and its committed output are here too: the
# test that holds them together is a Go test, run by the Go suite.
service_pattern='^(api/proto/|cmd/|internal/|tools/|client/opensysml/|client/release-digests\.json$|conformance/|tests/|scripts/|examples/|editors/vscode/tools/|editors/vscode/syntaxes/|Makefile$|go\.mod$|go\.sum$|\.github/workflows/|\.circleci/)'
docs_pattern='^(docs/|packaging/man/|mkdocs\.yml$|README\.md$|CHANGELOG\.md$|CONTRIBUTING\.md$|AGENTS\.md$|.*\.md$)'
node_pattern='^client/node/'
python_pattern='^client/python/'
java_pattern='^client/java/'
rust_pattern='^client/rust/'
vscode_pattern='^editors/vscode/'
cameo_pattern='^editors/cameo/'
syson_pattern='^editors/syson/'

known_pattern="$service_pattern|$docs_pattern|$node_pattern|$python_pattern|$java_pattern|$rust_pattern|$vscode_pattern|$cameo_pattern|$syson_pattern|^\.agents/|^\.gitignore$|^\.gitattributes$|^LICENSE|^packaging/"

matches() {
  local pattern=$1
  grep -Eq "$pattern" <<<"$changed"
}

if [[ -z "$changed" ]] || grep -Ev "$known_pattern" <<<"$changed" | grep -q .; then
  unclaimed=$(grep -Ev "$known_pattern" <<<"$changed" || true)
  [[ -n "$unclaimed" ]] && echo "unclaimed paths, so every area runs:" >&2 && echo "$unclaimed" >&2
  service=true
else
  matches "$service_pattern" && service=true || service=false
fi

emit() {
  local area=$1 enabled=$2
  echo "$area=$enabled"
}

emit go "$service"
emit docs "$( { [[ "$service" = true ]] || matches "$docs_pattern"; } && echo true || echo false)"
emit node "$( { [[ "$service" = true ]] || matches "$node_pattern"; } && echo true || echo false)"
emit python "$( { [[ "$service" = true ]] || matches "$python_pattern"; } && echo true || echo false)"
emit java "$( { [[ "$service" = true ]] || matches "$java_pattern"; } && echo true || echo false)"
emit rust "$( { [[ "$service" = true ]] || matches "$rust_pattern"; } && echo true || echo false)"
emit vscode "$( { [[ "$service" = true ]] || matches "$vscode_pattern"; } && echo true || echo false)"
# The Cameo plugin builds on the Java client, so a client change re-runs it too.
emit cameo "$( { [[ "$service" = true ]] || matches "$cameo_pattern" || matches "$java_pattern"; } && echo true || echo false)"
emit syson "$( { [[ "$service" = true ]] || matches "$syson_pattern"; } && echo true || echo false)"

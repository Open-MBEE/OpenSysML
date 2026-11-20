#!/usr/bin/env bash
# Configures this clone, once, to merge the count baselines cell by cell (CONTRIBUTING.md, "GitHub Actions").
set -euo pipefail

root=$(git rev-parse --show-toplevel)
driver="$(git rev-parse --git-common-dir)/baseline-merge"
case "$driver" in /*) ;; *) driver="$root/$driver" ;; esac
go build -C "$root/tools" -o "$driver" ./cmd/baseline-merge
git config merge.opensysml-cells.name "cell-wise merge of the count baselines"
git config merge.opensysml-cells.driver "'$driver' %O %A %B %P"
echo "configured merge.opensysml-cells: $driver"

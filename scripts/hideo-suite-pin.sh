#!/usr/bin/env bash

# The pin of the cross-MDK suite hideo67/sysmlv2-tests: it publishes no tags, so the pin is the head
# of its default branch on 2026-11-11. Move it with its baseline; see docs/project/hideo-suite.md.
HIDEO_SUITE_REPO="${HIDEO_SUITE_REPO:-https://github.com/hideo67/sysmlv2-tests.git}"
HIDEO_SUITE_COMMIT="${HIDEO_SUITE_COMMIT:-99eae49af071a79c81bf4262eb0c73d014fbb983}"

# hideo_suite_pin is the stamp a fetched checkout records.
hideo_suite_pin() {
	printf '%s %s' "$HIDEO_SUITE_COMMIT" "$HIDEO_SUITE_REPO"
	return 0
}

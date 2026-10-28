#!/usr/bin/env bash
# Single source of the extension-libraries pin: the OpenSysML-Extensions-Library
# repository, the commit scripts/sync-extension-libraries.sh vendors
# "OpenSysML Libraries/" from, and the ref that commit came from.
#
# Kept in one file so what CI checks and what a maintainer syncs cannot drift.
# The ref names where the pin was taken; the commit is what every fetch
# verifies, because a ref is mutable. Change them together.
EXTENSIONS_REPO="${EXTENSIONS_REPO:-https://github.com/Open-MBEE/OpenSysML-Extensions-Library.git}"
EXTENSIONS_REF="${EXTENSIONS_REF:-main}"
EXTENSIONS_COMMIT="${EXTENSIONS_COMMIT:-745963b151f925e4543b03d5bb09b978aa492e12}"

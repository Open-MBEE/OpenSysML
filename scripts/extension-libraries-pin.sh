#!/usr/bin/env bash
# Single source of the extension-libraries pin: the OpenSysML-Extensions-Library
# repository, the commit scripts/sync-extension-libraries.sh vendors
# "OpenSysML Libraries/" from, and the ref that commit came from.
#
# Kept in one file so what CI checks and what a maintainer syncs cannot drift.
# The ref names where the pin was taken; the commit is what every fetch
# verifies, because a ref is mutable. Change them together.
EXTENSIONS_REPO="${EXTENSIONS_REPO:-https://github.com/Open-MBEE/OpenSysML-Extensions-Library.git}"
EXTENSIONS_REF="${EXTENSIONS_REF:-feature/wheretype-exact}"
EXTENSIONS_COMMIT="${EXTENSIONS_COMMIT:-b6784f2305a9d63b2c4045b5979d18b05635f161}"

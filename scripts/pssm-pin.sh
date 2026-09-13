#!/usr/bin/env bash
# Single source of the OMG PSSM test-suite pin, sourced by the script that fetches
# it and read by the referee that records the pin in its baseline.
#
# The suite is the normative XMI of "Precise Semantics of UML State Machines"
# (PSSM) 1.0, OMG document ptc/18-11-06, published at the specification's
# permanent URL. The document number names the release; the checksum is what the
# fetch verifies, because the file behind a URL can change. Change them together.
#
# The suite's only external references are the UML PrimitiveTypes, the fUML and
# Alf libraries and the UML standard profile, all by URL; the PSSM syntax and
# semantics models (ptc/18-11-04, -05) are not referenced by the test suite and
# are not fetched.
PSSM_DOCUMENT="${PSSM_DOCUMENT:-ptc/18-11-06}"
PSSM_VERSION="${PSSM_VERSION:-1.0}"
PSSM_SUITE_URL="${PSSM_SUITE_URL:-https://www.omg.org/spec/PSSM/20181101/PSSM_TestSuite.xmi}"
PSSM_SUITE_SHA256="${PSSM_SUITE_SHA256:-c355b249c356774377a46b60345019d827af1ce417bde88e533aa5f39206ae07}"
PSSM_SUITE_FILE="PSSM_TestSuite.xmi"

# pssm_pin is the stamp a fetched destination records, and the value the referee
# compares against the committed baseline's provenance.
pssm_pin() {
	printf '%s %s %s %s' "$PSSM_DOCUMENT" "$PSSM_VERSION" "$PSSM_SUITE_SHA256" "$PSSM_SUITE_URL"
	return 0
}

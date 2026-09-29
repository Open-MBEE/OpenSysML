#!/usr/bin/env bash
# Single source of the Modelica Reference-FMUs pin, sourced by the script that
# fetches it: one GitHub release asset, verified by its sha256. The tag names the
# release; the checksum is what the fetch verifies — bump the tag and the digest
# together, because the file behind a URL can change.
#
# The asset is the release's Reference-FMUs.zip: the Reference-FMUs themselves
# (BouncingBall, Dahlquist, VanDerPol, …) for FMI 2.0 and 3.0, with linux64 /
# x86_64-linux binaries among the platforms, licensed BSD-3-Clause by the
# Modelica Association project (see the zip's LICENSE.txt).
REFERENCE_FMUS_TAG="${REFERENCE_FMUS_TAG:-v0.0.41}"
REFERENCE_FMUS_URL="${REFERENCE_FMUS_URL:-https://github.com/modelica/Reference-FMUs/releases/download/${REFERENCE_FMUS_TAG}/Reference-FMUs.zip}"
REFERENCE_FMUS_SHA256="${REFERENCE_FMUS_SHA256:-62babca76b9c23a51c3096be4bb5930ff8b4388659056be3c0c4ef7a3aeb5403}"
REFERENCE_FMUS_FILE="Reference-FMUs.zip"

# reference_fmus_pin is the stamp a fetched destination records.
reference_fmus_pin() {
	printf '%s %s %s' "$REFERENCE_FMUS_TAG" "$REFERENCE_FMUS_SHA256" "$REFERENCE_FMUS_URL"
	return 0
}

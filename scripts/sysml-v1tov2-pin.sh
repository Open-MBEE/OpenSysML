#!/usr/bin/env bash
# Single source of the OMG SysML v1 to v2 transformation-model pin, sourced by
# the script that fetches it and read by the census that records the pin in its
# baseline.
#
# The model is the normative XMI of the SysML v1 to v2 transformation, OMG
# document ptc/25-04-10, published at the specification's permanent URL. The
# document number names the release; the checksum is what the fetch verifies,
# because the file behind a URL can change. Change them together.
SYSML_V1TOV2_DOCUMENT="${SYSML_V1TOV2_DOCUMENT:-ptc/25-04-10}"
SYSML_V1TOV2_VERSION="${SYSML_V1TOV2_VERSION:-2.0}"
SYSML_V1TOV2_URL="${SYSML_V1TOV2_URL:-https://www.omg.org/spec/SysML/20250201/SysMLv1Tov2.xmi}"
SYSML_V1TOV2_SHA256="${SYSML_V1TOV2_SHA256:-093359439fb62cb3a9b1e89fd850ac4ac0b4bbe135b4b54f7861492722cc8459}"
SYSML_V1TOV2_FILE="SysMLv1Tov2.xmi"

# sysml_v1tov2_pin is the stamp a fetched destination records, and the value the
# census compares against the committed baseline's provenance.
sysml_v1tov2_pin() {
	printf '%s %s %s %s' "$SYSML_V1TOV2_DOCUMENT" "$SYSML_V1TOV2_VERSION" "$SYSML_V1TOV2_SHA256" "$SYSML_V1TOV2_URL"
	return 0
}

#!/usr/bin/env bash
# Build DeciSym's sysmlv2-validator (a thin CLI over the OMG SysML v2 Pilot
# Implementation) into build/pilot-validator/, for the advisory differential
# harness in cmd/pilot-diff. See docs/project/pilot-differential.md.
#
# Both the wrapper commit and the pilot release are pinned; the pilot tag and
# artifact version come from scripts/pilot-pin.sh and are passed to Maven.
set -euo pipefail

# shellcheck source=scripts/pilot-pin.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/pilot-pin.sh"

VALIDATOR_REPO="${VALIDATOR_REPO:-https://github.com/DeciSym/sysmlv2-validator.git}"
VALIDATOR_COMMIT="${VALIDATOR_COMMIT:-63abbd9fbc7851dc437d01b2dc07836b919770b8}"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
target="$repo_root/build/pilot-validator"
pilot_jar="$target/target/sysml-download/sysml/jupyter-sysml-kernel-${PILOT_ARTIFACT_VERSION}-all.jar"
# The build records what it was made from, so a re-pin re-provisions it.
stamp="$target/.pilot-pin"
pin="$(pilot_pin) $PILOT_ARTIFACT_VERSION $VALIDATOR_COMMIT"

if [[ -x "$target/validate-sysml" ]] && [[ -f "$target/target/sysmlv2-validator-1.0.0-SNAPSHOT.jar" ]]; then
	if [[ -f "$pilot_jar" ]] && [[ -f "$stamp" ]] && [[ "$(cat "$stamp")" == "$pin" ]]; then
		echo "Pilot validator already built at $target (pilot $PILOT_TAG, $PILOT_ARTIFACT_VERSION)"
		echo "Remove that directory to re-provision."
		exit 0
	fi
	if [[ -f "$stamp" ]]; then
		echo "Stale build at $target: built from $(cat "$stamp"), pin is now $pin; rebuilding."
	else
		echo "Unstamped build at $target; rebuilding at pilot $PILOT_TAG ($PILOT_ARTIFACT_VERSION)."
	fi
fi

for tool in git java mvn; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "error: $tool is required to build the pilot validator" >&2
		exit 1
	fi
done

java_major="$(java -version 2>&1 | sed -n '1s/.*version "\([0-9][0-9]*\).*/\1/p')"
if [[ -z "$java_major" ]] || [[ "$java_major" -lt 21 ]]; then
	echo "error: the pilot implementation requires Java 21+, found: $(java -version 2>&1 | head -1)" >&2
	exit 1
fi

mkdir -p "$(dirname "$target")"
rm -rf "$target"

echo "Cloning $VALIDATOR_REPO at $VALIDATOR_COMMIT ..."
git init --quiet "$target"
git -C "$target" remote add origin "$VALIDATOR_REPO"
git -C "$target" fetch --quiet --depth 1 origin "$VALIDATOR_COMMIT"
git -C "$target" checkout --quiet FETCH_HEAD

# The wrapper must still select the release through these properties, or the
# overrides below are silently ignored.
for property in sysml.release.tag sysml.artifact.version; do
	if ! grep -Fq "<${property}>" "$target/pom.xml" || ! grep -Fq "\${${property}}" "$target/pom.xml"; then
		echo "error: $VALIDATOR_COMMIT no longer selects the pilot release through the $property property" >&2
		echo "       re-pin VALIDATOR_COMMIT to a wrapper that does" >&2
		exit 1
	fi
done

# The pilot is not on Maven Central: the setup-dependency profile downloads the
# jupyter-sysml-kernel release ZIP (jar + sysml.library) and installs the jar
# into ~/.m2, which `mvn package` then shades into the validator jar.
pin_properties=("-Dsysml.release.tag=$PILOT_TAG" "-Dsysml.artifact.version=$PILOT_ARTIFACT_VERSION")
echo "Downloading the pilot $PILOT_TAG ($PILOT_ARTIFACT_VERSION) release and building the validator ..."
(cd "$target" &&
	mvn -B -q "${pin_properties[@]}" -Psetup-dependency initialize &&
	mvn -B -q "${pin_properties[@]}" package)

library="$target/target/sysml-download/sysml/sysml.library"
if [[ ! -d "$library" ]]; then
	echo "error: the pilot standard library is missing from $library" >&2
	exit 1
fi
if [[ ! -f "$pilot_jar" ]]; then
	echo "error: the pilot shaded jar is missing from $pilot_jar" >&2
	exit 1
fi
printf '%s' "$pin" >"$stamp"

echo "Built $target/validate-sysml (pilot $PILOT_TAG, $PILOT_ARTIFACT_VERSION)"
echo "Compare it against this implementation with:"
echo "  go run ./cmd/pilot-diff"

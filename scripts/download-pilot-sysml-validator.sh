#!/usr/bin/env bash
# Build the plain-Java batch SysML validator oracle into build/pilot-sysml-validator/.
#
# The SysML twin of download-pilot-kerml-validator.sh: it compiles against the same
# pinned pilot jar that download-pilot-validator.sh provisions, and loads a whole
# corpus root as one resource set instead of one accumulating interactive session.
set -euo pipefail

# shellcheck disable=SC1091
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/pilot-pin.sh"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source="$repo_root/scripts/pilot-sysml-validator/ValidateSysML.java"
target="$repo_root/build/pilot-sysml-validator"
classes="$target/classes"
validator_target="$repo_root/build/pilot-validator/target/sysml-download/sysml"
pilot_jar="$validator_target/jupyter-sysml-kernel-${PILOT_ARTIFACT_VERSION}-all.jar"
library="$validator_target/sysml.library"
# Written when the validator build completes; the jar itself keeps its release-time mtime.
validator_stamp="$repo_root/build/pilot-validator/.pilot-pin"
launcher="$target/validate-sysml-batch"
force=0

case "${1:-}" in
	--force)
		force=1
		shift
		;;
	"")
		;;
	*)
		echo "error: unknown option: $1 (only --force is supported)" >&2
		exit 1
		;;
esac

# Always delegate: its fast path is a stamp check, and a stale or re-pinned build is rebuilt.
"$repo_root/scripts/download-pilot-validator.sh"

if [[ ! -f "$pilot_jar" ]]; then
	echo "error: pilot shaded jar not found at $pilot_jar" >&2
	exit 1
fi
if [[ ! -d "$library" ]]; then
	echo "error: pilot standard library not found at $library" >&2
	exit 1
fi
if [[ ! -f "$source" ]]; then
	echo "error: SysML validator source not found at $source" >&2
	exit 1
fi

if command -v java >/dev/null 2>&1; then
	java_bin="$(command -v java)"
elif [[ -x /usr/local/jdk-21/bin/java ]]; then
	java_bin=/usr/local/jdk-21/bin/java
else
	echo "error: Java 21+ is required to build the SysML validator" >&2
	exit 1
fi

if command -v javac >/dev/null 2>&1; then
	javac_bin="$(command -v javac)"
elif [[ -x /usr/local/jdk-21/bin/javac ]]; then
	javac_bin=/usr/local/jdk-21/bin/javac
else
	echo "error: javac 21+ is required to build the SysML validator" >&2
	exit 1
fi

if command -v jar >/dev/null 2>&1; then
	jar_bin="$(command -v jar)"
elif [[ -x /usr/local/jdk-21/bin/jar ]]; then
	jar_bin=/usr/local/jdk-21/bin/jar
else
	echo "error: jar is required to inspect the pilot SysML validator" >&2
	exit 1
fi

java_major="$("$java_bin" -version 2>&1 | sed -n '1s/.*version "\([0-9][0-9]*\).*/\1/p')"
if [[ -z "$java_major" ]] || [[ "$java_major" -lt 21 ]]; then
	echo "error: the pilot implementation requires Java 21+, found: $("${java_bin}" -version 2>&1 | head -1)" >&2
	exit 1
fi

for class in \
	"org/omg/sysml/xtext/validation/SysMLValidator.class" \
	"org/omg/sysml/xtext/SysMLStandaloneSetup.class" \
	"org/omg/sysml/io/SysMLUtil.class"; do
	if ! "$jar_bin" tf "$pilot_jar" | grep -Fqx "$class"; then
		echo "error: pilot jar $pilot_jar is missing $class" >&2
		exit 1
	fi
done

mkdir -p "$classes"
output_class="$classes/io/opensysml/pilot/ValidateSysML.class"
if [[ "$force" -eq 1 ]] || [[ ! -f "$output_class" ]] ||
	[[ "$output_class" -ot "$source" ]] || [[ "$output_class" -ot "$validator_stamp" ]]; then
	echo "Compiling $source ..."
	"$javac_bin" -cp "$pilot_jar" -d "$classes" "$source"
else
	echo "SysML validator already compiled at $output_class"
fi

# The pin cmd/pilot-diff reports, written from pilot-pin.sh rather than read out of the
# DeciSym wrapper's pom.xml, which download-pilot-validator.sh has already checked against it.
cat >"$target/pilot-pin.txt" <<EOF
sysml.release.tag=$PILOT_TAG
sysml.artifact.version=$PILOT_ARTIFACT_VERSION
EOF

# Always rewritten, so a changed pin never leaves a launcher pointing at the old jar.
launcher_tmp="$(mktemp "${launcher}.XXXXXX")"
trap 'rm -f "$launcher_tmp" "${launcher_tmp}.out"' EXIT
cat >"$launcher_tmp" <<'EOF'
#!/bin/sh
set -e

SCRIPT="$0"
while [ -L "$SCRIPT" ]; do
	SCRIPT_DIR="$(cd "$(dirname "$SCRIPT")" && pwd)"
	SCRIPT="$(readlink "$SCRIPT")"
	[ "${SCRIPT%"${SCRIPT#?}"}" != "/" ] && SCRIPT="$SCRIPT_DIR/$SCRIPT"
done
SCRIPT_DIR="$(cd "$(dirname "$SCRIPT")" && pwd)"

CLASSES="$SCRIPT_DIR/classes"
JAR="$SCRIPT_DIR/../pilot-validator/target/sysml-download/sysml/jupyter-sysml-kernel-__PILOT_ARTIFACT_VERSION__-all.jar"
LIBRARY="$SCRIPT_DIR/../pilot-validator/target/sysml-download/sysml/sysml.library"

if command -v java >/dev/null 2>&1; then
	JAVA=java
elif [ -x /usr/local/jdk-21/bin/java ]; then
	JAVA=/usr/local/jdk-21/bin/java
else
	echo "Error: Java 21+ not found" >&2
	exit 1
fi

if [ ! -f "$JAR" ]; then
	echo "Error: pilot SysML jar not found at $JAR" >&2
	exit 1
fi

has_library=0
for arg in "$@"; do
	[ "$arg" = "--library" ] && has_library=1
done
if [ "$has_library" -eq 0 ]; then
	if [ ! -d "$LIBRARY" ]; then
		echo "Error: SysML library not found at $LIBRARY" >&2
		exit 1
	fi
	LIBRARY="$(cd "$LIBRARY" && pwd)"
	set -- --library "$LIBRARY" "$@"
fi

exec "$JAVA" -cp "$CLASSES:$JAR" io.opensysml.pilot.ValidateSysML "$@"
EOF
sed "s|__PILOT_ARTIFACT_VERSION__|${PILOT_ARTIFACT_VERSION}|g" \
	"$launcher_tmp" >"${launcher_tmp}.out"
mv "${launcher_tmp}.out" "$launcher"
rm -f "$launcher_tmp"
trap - EXIT
chmod +x "$launcher"

echo "Built $launcher (pilot $PILOT_TAG, $PILOT_ARTIFACT_VERSION)"

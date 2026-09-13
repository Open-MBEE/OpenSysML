#!/usr/bin/env bash
# Build the plain-Java KerML validator oracle into build/pilot-kerml-validator/.
set -euo pipefail

# shellcheck disable=SC1091
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/pilot-pin.sh"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source="$repo_root/scripts/pilot-kerml-validator/ValidateKerML.java"
target="$repo_root/build/pilot-kerml-validator"
classes="$target/classes"
validator_target="$repo_root/build/pilot-validator/target/sysml-download/sysml"
pilot_jar="$validator_target/jupyter-sysml-kernel-${PILOT_ARTIFACT_VERSION}-all.jar"
library="$validator_target/sysml.library"
# Written when the validator build completes; the jar itself keeps its release-time mtime.
validator_stamp="$repo_root/build/pilot-validator/.pilot-pin"
launcher="$target/validate-kerml"
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
	echo "error: KerML validator source not found at $source" >&2
	exit 1
fi

if command -v java >/dev/null 2>&1; then
	java_bin="$(command -v java)"
elif [[ -x /usr/local/jdk-21/bin/java ]]; then
	java_bin=/usr/local/jdk-21/bin/java
else
	echo "error: Java 21+ is required to build the KerML validator" >&2
	exit 1
fi

if command -v javac >/dev/null 2>&1; then
	javac_bin="$(command -v javac)"
elif [[ -x /usr/local/jdk-21/bin/javac ]]; then
	javac_bin=/usr/local/jdk-21/bin/javac
else
	echo "error: javac 21+ is required to build the KerML validator" >&2
	exit 1
fi

if command -v jar >/dev/null 2>&1; then
	jar_bin="$(command -v jar)"
elif [[ -x /usr/local/jdk-21/bin/jar ]]; then
	jar_bin=/usr/local/jdk-21/bin/jar
else
	echo "error: jar is required to inspect the pilot KerML validator" >&2
	exit 1
fi

java_major="$("$java_bin" -version 2>&1 | sed -n '1s/.*version "\([0-9][0-9]*\).*/\1/p')"
if [[ -z "$java_major" ]] || [[ "$java_major" -lt 21 ]]; then
	echo "error: the pilot implementation requires Java 21+, found: $("${java_bin}" -version 2>&1 | head -1)" >&2
	exit 1
fi

for class in \
	"org/omg/kerml/xtext/validation/KerMLValidator.class" \
	"org/omg/kerml/xtext/KerMLStandaloneSetup.class"; do
	if ! "$jar_bin" tf "$pilot_jar" | grep -Fqx "$class"; then
		echo "error: pilot jar $pilot_jar is missing $class" >&2
		exit 1
	fi
done

mkdir -p "$classes"
output_class="$classes/io/opensysml/pilot/ValidateKerML.class"
if [[ "$force" -eq 1 ]] || [[ ! -f "$output_class" ]] ||
	[[ "$output_class" -ot "$source" ]] || [[ "$output_class" -ot "$validator_stamp" ]]; then
	echo "Compiling $source ..."
	"$javac_bin" -cp "$pilot_jar" -d "$classes" "$source"
else
	echo "KerML validator already compiled at $output_class"
fi

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
	echo "Error: pilot KerML jar not found at $JAR" >&2
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

exec "$JAVA" -cp "$CLASSES:$JAR" io.opensysml.pilot.ValidateKerML "$@"
EOF
sed "s|__PILOT_ARTIFACT_VERSION__|${PILOT_ARTIFACT_VERSION}|g" \
	"$launcher_tmp" >"${launcher_tmp}.out"
mv "${launcher_tmp}.out" "$launcher"
rm -f "$launcher_tmp"
trap - EXIT
chmod +x "$launcher"

echo "Built $launcher (pilot $PILOT_TAG, $PILOT_ARTIFACT_VERSION)"

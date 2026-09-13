#!/usr/bin/env bash
# Build the plain-Java pilot expression evaluator into build/pilot-evaluator/.
set -euo pipefail

# shellcheck disable=SC1091
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/pilot-pin.sh"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source="$repo_root/scripts/pilot-evaluator/EvalSysML.java"
target="$repo_root/build/pilot-evaluator"
classes="$target/classes"
validator_target="$repo_root/build/pilot-validator/target/sysml-download/sysml"
# download-pilot-validator.sh provisions this artifact at PILOT_TAG; locate it here by version.
pilot_jar="$validator_target/jupyter-sysml-kernel-${PILOT_ARTIFACT_VERSION}-all.jar"
library="$validator_target/sysml.library"
launcher="$target/eval-sysml"

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
	echo "error: pilot evaluator source not found at $source" >&2
	exit 1
fi

if command -v java >/dev/null 2>&1; then
	java_bin="$(command -v java)"
elif [[ -x /usr/local/jdk-21/bin/java ]]; then
	java_bin=/usr/local/jdk-21/bin/java
else
	echo "error: Java 21+ is required to build the pilot evaluator" >&2
	exit 1
fi

if command -v javac >/dev/null 2>&1; then
	javac_bin="$(command -v javac)"
elif [[ -x /usr/local/jdk-21/bin/javac ]]; then
	javac_bin=/usr/local/jdk-21/bin/javac
else
	echo "error: javac 21+ is required to build the pilot evaluator" >&2
	exit 1
fi

if command -v jar >/dev/null 2>&1; then
	jar_bin="$(command -v jar)"
elif [[ -x /usr/local/jdk-21/bin/jar ]]; then
	jar_bin=/usr/local/jdk-21/bin/jar
else
	echo "error: jar is required to inspect the pilot evaluator" >&2
	exit 1
fi

java_major="$("$java_bin" -version 2>&1 | sed -n '1s/.*version "\([0-9][0-9]*\).*/\1/p')"
if [[ -z "$java_major" ]] || [[ "$java_major" -lt 21 ]]; then
	echo "error: the pilot implementation requires Java 21+, found: $("${java_bin}" -version 2>&1 | head -1)" >&2
	exit 1
fi

for class in \
	"org/omg/sysml/interactive/SysMLInteractive.class" \
	"org/omg/sysml/expressions/ModelLevelExpressionEvaluator.class"; do
	if ! "$jar_bin" tf "$pilot_jar" | grep -Fqx "$class"; then
		echo "error: pilot jar $pilot_jar is missing $class" >&2
		exit 1
	fi
done

mkdir -p "$classes"
echo "Compiling $source ..."
"$javac_bin" -cp "$pilot_jar" -d "$classes" "$source"

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
	echo "Error: pilot expression jar not found at $JAR" >&2
	exit 1
fi
if [ ! -d "$LIBRARY" ]; then
	echo "Error: SysML library not found at $LIBRARY" >&2
	exit 1
fi

has_library=0
for arg in "$@"; do
	[ "$arg" = "--library" ] && has_library=1
done
if [ "$has_library" -eq 0 ]; then
	LIBRARY="$(cd "$LIBRARY" && pwd)"
	set -- --library "$LIBRARY" "$@"
fi

exec "$JAVA" -cp "$CLASSES:$JAR" io.opensysml.pilot.EvalSysML "$@"
EOF
sed "s|__PILOT_ARTIFACT_VERSION__|${PILOT_ARTIFACT_VERSION}|g" \
	"$launcher_tmp" >"${launcher_tmp}.out"
mv "${launcher_tmp}.out" "$launcher"
rm -f "$launcher_tmp"
trap - EXIT
chmod +x "$launcher"

echo "Built $launcher (pilot artifact $PILOT_ARTIFACT_VERSION)"

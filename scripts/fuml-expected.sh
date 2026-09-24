#!/usr/bin/env bash
# Regenerate docs/project/fuml-referee-expected.json by running the pinned fUML
# reference implementation (needs a JDK) through scripts/fuml-driver/; the only
# place Java runs. The gate and the referee read the committed record.
set -euo pipefail

# shellcheck source=scripts/fuml-pin.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/fuml-pin.sh"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
suite="${FUML_SUITE_ROOT:-$repo_root/build/fuml}"
out="${FUML_EXPECTED_OUT:-$repo_root/docs/project/fuml-referee-expected.json}"
classes="$suite/.driver-classes"

case "${1:-}" in
	"") ;;
	*)
		echo "error: unknown option: $1 (this script takes no options)" >&2
		exit 1
		;;
esac

javac="${JAVA_HOME:+$JAVA_HOME/bin/}javac"
java="${JAVA_HOME:+$JAVA_HOME/bin/}java"
if ! command -v "$javac" >/dev/null 2>&1 || ! command -v "$java" >/dev/null 2>&1; then
	echo "error: a JDK is required (javac and java on PATH, or JAVA_HOME set)" >&2
	exit 1
fi

FUML_SUITE_ROOT="$suite" "$repo_root/scripts/download-fuml-suite.sh"

classpath="$suite/$FUML_JAR_FILE"
while read -r name; do
	classpath="$classpath:$suite/lib/$name"
done < <(fuml_dep_files)

rm -rf "$classes"
mkdir -p "$classes"
echo "Compiling scripts/fuml-driver ..."
# -path is off: some dependency manifests name sibling jars under other file names.
"$javac" -encoding UTF-8 -Xlint:all,-path -Werror -d "$classes" -cp "$classpath" \
	"$repo_root"/scripts/fuml-driver/io/opensysml/fuml/*.java

# The implementation reads its configuration (DefaultFumlConfig.xml) and the
# foundational library it executes against from resources inside its own jar.
echo "Running the fUML reference implementation over $FUML_TESTS_FILE and $FUML_EXCEPTION_TESTS_FILE ..."
"$java" -cp "$classes:$classpath" \
	io.opensysml.fuml.FumlExpected \
	--model "$suite/$FUML_TESTS_FILE=$FUML_TESTS_URI" \
	--model "$suite/$FUML_EXCEPTION_TESTS_FILE=$FUML_EXCEPTION_TESTS_URI" \
	--jar "$suite/$FUML_JAR_FILE" \
	--ri-tag "$FUML_RI_TAG" --ri-commit "$FUML_RI_COMMIT" \
	--out "$out"

echo "Wrote $out"

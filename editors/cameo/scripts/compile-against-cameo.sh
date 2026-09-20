#!/bin/sh
set -eu

CAMEO_HOME="${1:-${CAMEO_HOME:-}}"
if [ -z "$CAMEO_HOME" ] || [ ! -d "$CAMEO_HOME" ]; then
  echo "usage: CAMEO_HOME=/path/to/Cameo $0" >&2
  exit 2
fi
root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
client_dir="$root/../../client/java"
client_cp=$(mvn -q -f "$client_dir/pom.xml" -pl opensysml-client -am dependency:build-classpath -Dmdep.outputAbsoluteArtifactFilename=true -Dmdep.outputFile="$root/target/cameo-client.cp" >/dev/null && cat "$root/target/cameo-client.cp")
cameo_cp=$(find "$CAMEO_HOME/lib" "$CAMEO_HOME/plugins" -name '*.jar' -print | tr '\n' ':')
cp="${cameo_cp}${client_cp}"
out=$(mktemp -d)
trap 'rm -rf "$out"' EXIT HUP INT TERM
find "$root/plugin/src/main/java" -name '*.java' -print0 | xargs -0 javac --release 17 -cp "$cp" -d "$out"

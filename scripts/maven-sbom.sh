#!/usr/bin/env bash

# Writes <tree>/target/bom.json, the CycloneDX SBOM `make audit-java` scans. The plugin that
# writes it is no dependency of any pom.xml, so a run that finds it in no cache asks Maven Central.
set -euo pipefail

[[ $# -eq 2 ]] || { echo "usage: $0 <Maven tree> <cyclonedx-maven-plugin version>" >&2; exit 2; }
tree=$1
version=$2

attempts=${MAVEN_SBOM_ATTEMPTS:-4}
mirror=$(cd "$(dirname "$0")" && pwd)/maven-central-mirror-settings.xml
goal=(-B -q -f "$tree/pom.xml" "org.cyclonedx:cyclonedx-maven-plugin:$version:makeAggregateBom"
  -DoutputFormat=json -DoutputName=bom -DincludeTestScope=false)

log=$(mktemp)
trap 'rm -f "$log"' EXIT
for ((attempt = 1; attempt <= attempts; attempt++)); do
  # Every second attempt asks the mirror; -U asks again for what the attempt before was refused.
  extra=()
  ((attempt == 1)) || extra+=(-U)
  ((attempt % 2 == 1)) || extra+=(-s "$mirror")
  status=0
  mvn ${extra[@]+"${extra[@]}"} "${goal[@]}" 2>&1 | tee "$log" || status=$?
  ((status != 0)) || exit 0
  # Only an artifact that did not arrive is worth asking for again: a broken pom.xml fails at once.
  if ! grep -Eq 'Could not (find|transfer) artifact|could not be resolved|Could not resolve dependencies' "$log"; then
    exit "$status"
  fi
  echo "Maven did not resolve what the SBOM of $tree needs (attempt $attempt of $attempts, exit $status)" >&2
  ((attempt == attempts)) || sleep $((10 * attempt))
done
echo "error: no SBOM of $tree in $attempts attempts: neither Maven Central nor its mirror answered" >&2
exit "$status"

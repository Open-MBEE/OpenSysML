#!/usr/bin/env bash

# Writes <tree>/target/bom.json, the CycloneDX SBOM `make audit-java` scans. The plugin that
# writes it is no dependency of any pom.xml, so a run that finds it in no cache asks a mirror of
# Maven Central for it, and Maven Central when the mirror refuses.
set -euo pipefail

[[ $# -eq 2 ]] || { echo "usage: $0 <Maven tree> <cyclonedx-maven-plugin version>" >&2; exit 2; }
tree=$1
version=$2

attempts=${MAVEN_SBOM_ATTEMPTS:-4}
bound=${MAVEN_SBOM_TIMEOUT_SECONDS:-90}
mirror=$(cd "$(dirname "$0")" && pwd)/maven-central-mirror-settings.xml
goal=(-B -q -f "$tree/pom.xml" "org.cyclonedx:cyclonedx-maven-plugin:$version:makeAggregateBom"
  -DoutputFormat=json -DoutputName=bom -DincludeTestScope=false)

# GNU timeout bounds an attempt where there is one (gtimeout on macOS with coreutils): the resolver
# backs off from a 429 for minutes, and a stalled transfer has no bound of its own.
bounded=()
for command in timeout gtimeout; do
  # A timeout that knows no -k, as BusyBox 1.30's, bounds the attempt without the kill.
  for kill in "-k 10" ""; do
    # shellcheck disable=SC2086 # the options are words, or none
    if "$command" $kill 1 true >/dev/null 2>&1; then
      # shellcheck disable=SC2206
      bounded=("$command" $kill "$bound")
      break 2
    fi
  done
done

log=$(mktemp)
trap 'rm -f "$log"' EXIT
for ((attempt = 1; attempt <= attempts; attempt++)); do
  # The first attempt and every second one after it ask the mirror, which sets no rate limit, as global settings
  # so that a user's proxy still applies; the others ask Maven Central. -U asks again for what was refused.
  extra=()
  ((attempt == 1)) || extra+=(-U)
  ((attempt % 2 == 0)) || extra+=(-gs "$mirror")
  status=0
  ${bounded[@]+"${bounded[@]}"} mvn ${extra[@]+"${extra[@]}"} "${goal[@]}" 2>&1 | tee "$log" || status=$?
  ((status != 0)) || exit 0
  # timeout exits 124, or 137 once it had to kill, and BusyBox's with the 143 of the Maven it ended:
  # an attempt cut short is asked again, as a refusal is.
  if ((${#bounded[@]} > 0)) && ((status == 124 || status == 137 || status == 143)); then
    echo "Maven did not write the SBOM of $tree in $bound seconds (attempt $attempt of $attempts, exit $status)" >&2
  # Only an artifact that did not arrive is worth asking for again: a broken pom.xml fails at once.
  elif grep -Eq 'Could not (find|transfer) artifact|could not be resolved|Could not resolve dependencies' "$log"; then
    echo "Maven did not resolve what the SBOM of $tree needs (attempt $attempt of $attempts, exit $status)" >&2
  else
    exit "$status"
  fi
  ((attempt == attempts)) || sleep $((10 * attempt))
done
echo "error: no SBOM of $tree in $attempts attempts: neither Maven Central nor its mirror answered in time" >&2
exit "$status"

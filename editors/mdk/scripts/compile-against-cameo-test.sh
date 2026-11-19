#!/usr/bin/env bash

# Runs compile-against-cameo.sh from a client with no target/classes against a stand-in Cameo
# whose lib/ holds only the compile-only OpenAPI stubs, so no licensed installation is needed.
set -euo pipefail

repo=$(cd "$(dirname "$0")/../../.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

fail() {
  echo "FAIL $*" >&2
  exit 1
}

# The copy leaves every target/ behind: the client starts uncompiled and the checkout is not written to.
tar -C "$repo" -cf - --exclude=target api/proto client/java editors/mdk/scripts \
  editors/mdk/plugin/src/main/java editors/mdk/openapi-stubs/src/main/java | tar -C "$work" -xf -
classes=$work/client/java/opensysml-client/target/classes
[[ ! -e $classes ]] || fail "the client copy already has target/classes"

cameo=$work/cameo
mkdir -p "$cameo/lib" "$cameo/plugins" "$work/stubs" "$work/bin"
find "$work/editors/mdk/openapi-stubs/src/main/java" -name '*.java' -print0 |
  xargs -0 javac --release 17 -d "$work/stubs"
jar --create --file "$cameo/lib/openapi-stubs.jar" -C "$work/stubs" .

# The stand-in javac records its arguments, one per line, and hands them to the real compiler.
real_javac=$(command -v javac)
cat >"$work/bin/javac" <<STUB
#!/usr/bin/env bash
printf '%s\n' "\$@" >>"$work/javac.args"
exec "$real_javac" "\$@"
STUB
chmod +x "$work/bin/javac"

PATH="$work/bin:$PATH" CAMEO_HOME="$cameo" "$work/editors/mdk/scripts/compile-against-cameo.sh" ||
  fail "the plugin did not compile from a client without target/classes (exit $?)"

compgen -G "$classes/org/openmbee/opensysml/*.class" >/dev/null ||
  fail "Maven did not compile the client into $classes"

class_path=$(grep -x -A1 -e '-cp' "$work/javac.args" | sed -n 2p)
client_classes=false
dependency_jars=0
IFS=: read -r -a entries <<<"$class_path"
for entry in "${entries[@]}"; do
  case $entry in
    "$cameo"/* | '') ;;
    *.jar) [[ -f $entry ]] && dependency_jars=$((dependency_jars + 1)) ;;
    *) [[ -d $entry && $(cd "$entry" && pwd -P) == "$(cd "$classes" && pwd -P)" ]] && client_classes=true ;;
  esac
done
[[ $client_classes == true ]] || fail "javac's class path lacks the client's target/classes: $class_path"
((dependency_jars > 0)) || fail "javac's class path lacks the client's dependencies: $class_path"

# Offline against an empty local repository Maven resolves nothing: the helper must fail and say why.
if MAVEN_ARGS="${MAVEN_ARGS:-} -o -Dmaven.repo.local=$work/empty-repository" CAMEO_HOME="$cameo" \
  "$work/editors/mdk/scripts/compile-against-cameo.sh" >"$work/failed.out" 2>"$work/failed.err"; then
  fail "the helper succeeded although Maven could resolve nothing"
fi
grep -q -F '[ERROR]' "$work/failed.err" || fail "the helper discarded Maven's error: $(cat "$work/failed.err")"
[[ ! -s $work/failed.out ]] || fail "the helper wrote Maven's error to standard output: $(cat "$work/failed.out")"

echo "ok: compiled against the stand-in Cameo with $dependency_jars client dependencies and the client's classes"

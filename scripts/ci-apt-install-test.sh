#!/usr/bin/env bash
# Checks scripts/ci-apt-install.sh against a stand-in apt that plays a mirror outage, so
# the offline path a real outage takes is exercised on every pull request.
set -euo pipefail

script=$(cd "$(dirname "$0")" && pwd)/ci-apt-install.sh
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
stubs=$work/bin
mkdir -p "$stubs"

# sudo and the retry pause are stand-ins; apt-get records its calls and plays the scenario
# in FAKE_APT_*; dpkg-query answers from the packages the stand-in installed.
cat >"$stubs/sudo" <<'STUB'
#!/usr/bin/env bash
exec "$@"
STUB
cat >"$stubs/sleep" <<'STUB'
#!/usr/bin/env bash
exit 0
STUB
cat >"$stubs/dpkg-query" <<'STUB'
#!/usr/bin/env bash
# dpkg-query -W -f=<format> <package>: installed, or absent.
package=${*: -1}
if grep -qx "$package" "$FAKE_DPKG_STATUS"; then
  printf 'installed'
else
  echo "dpkg-query: no packages found matching $package" >&2
  exit 1
fi
STUB
cat >"$stubs/apt-get" <<'STUB'
#!/usr/bin/env bash
# FAKE_APT_UPDATE and FAKE_APT_FETCH (ok|fail) say whether the mirror answers; FAKE_APT_SET
# names the archives (<package>_<version>_<arch>.deb) the request needs, space separated.
printf '%s\n' "$*" >>"$FAKE_APT_LOG"
command= cache= files=() names=()
for arg in "$@"; do
  case $arg in
    Dir::Cache::Archives=*) cache=${arg#*=} ;;
    -*|*=*) ;;
    */*.deb) files+=("$arg") ;;
    update|install) command=$arg ;;
    *) names+=("$arg") ;;
  esac
done
read -r -a wanted <<<"${FAKE_APT_SET:-}"
case $command in
  update)
    [[ ${FAKE_APT_UPDATE:-ok} == ok ]] && exit 0
    echo "Err: no reply from the mirror" >&2
    exit 100 ;;
  install) ;;
  *) echo "apt-get stand-in: unexpected command: $*" >&2; exit 2 ;;
esac
if [[ " $* " == *" --print-uris "* ]]; then
  # As apt does, names only what the cache under Dir::Cache::Archives does not hold.
  for deb in "${wanted[@]}"; do
    [[ -e $cache/$deb ]] || echo "'http://mirror.example/pool/$deb' $deb 1 MD5Sum:0"
  done
  exit 0
fi
if [[ " $* " == *" --download-only "* ]]; then
  [[ ${FAKE_APT_FETCH:-ok} == ok ]] || { echo "Err: no reply from the mirror" >&2; exit 100; }
  for deb in "${wanted[@]}"; do
    [[ -e $cache/$deb ]] || echo fetched >"$cache/$deb"
  done
  exit 0
fi
if [[ " $* " == *" --no-download "* ]]; then
  # A bare name would be resolved against the lists, which the outage left stale.
  ((${#names[@]} == 0)) || { echo "apt-get stand-in: package names offline: ${names[*]}" >&2; exit 2; }
  given=$(printf '%s\n' "${files[@]##*/}")
  for deb in "${wanted[@]}"; do
    grep -qx "$deb" <<<"$given" || { echo "E: Unable to fetch some archives, maybe run apt-get update or try with --fix-missing?" >&2; exit 100; }
  done
  for file in "${files[@]}"; do
    deb=${file##*/}
    echo "${deb%%_*}" >>"$FAKE_DPKG_STATUS"
  done
  exit 0
fi
for deb in "${wanted[@]}"; do
  [[ -e $cache/$deb ]] || { echo "E: $deb is not in the cache" >&2; exit 100; }
  echo "${deb%%_*}" >>"$FAKE_DPKG_STATUS"
done
STUB
chmod +x "$stubs"/*

failures=0
fail() {
  echo "FAIL $*" >&2
  failures=$((failures + 1))
}

# run_case <name> <expected exit> [VAR=value ...] -- <script arguments...>: runs the script
# with the stand-ins, CI_APT_CACHE_DIR at $work/<name>/cache and two mirrors listed.
run_case() {
  local name=$1 expected=$2 dir=$work/$1
  shift 2
  local vars=()
  while [[ $1 != -- ]]; do
    vars+=("$1")
    shift
  done
  shift
  mkdir -p "$dir/cache"
  : >"$dir/apt.log"
  : >"$dir/dpkg.status"
  printf 'http://mirror.one/ubuntu\tpriority:1\nhttp://mirror.two/ubuntu\tpriority:2\n' >"$dir/mirrors"
  local actual=0
  env PATH="$stubs:$PATH" FAKE_APT_LOG="$dir/apt.log" FAKE_DPKG_STATUS="$dir/dpkg.status" \
    CI_APT_CACHE_DIR="$dir/cache" CI_APT_MIRRORS="$dir/mirrors" "${vars[@]}" \
    bash "$script" "$@" >"$dir/out" 2>&1 || actual=$?
  if [[ $actual != "$expected" ]]; then
    fail "$name: exit $actual, expected $expected"
    sed 's/^/  /' "$dir/out" >&2
    return 1
  fi
  echo "ok   $name: exit $actual"
}

# seed <name> <archive>...: the set an earlier run kept under $work/<name>/set.
seed() {
  local dir=$work/$1/set
  shift
  mkdir -p "$dir"
  for deb in "$@"; do echo seeded >"$dir/$deb"; done
}

calls() { grep -c -e "$2" "$work/$1/apt.log" || true; }
# installs <name>: the apt-get install calls that install (neither fetch nor list).
installs() { grep -- ' install ' "$work/$1/apt.log" | grep -v -e '--download-only' -e '--print-uris' || true; }
mentions() { grep -q -- "$2" "$work/$1/out" || fail "$1: output lacks \"$2\""; }
installed() { grep -qx "$2" "$work/$1/dpkg.status" || fail "$1: $2 was not installed"; }
set_holds() {
  local name=$1 actual
  shift
  actual=$(find "$work/$name/set" -maxdepth 1 -name '*.deb' -printf '%f\n' | sort | paste -sd ' ' -)
  [[ $actual == "$*" ]] || fail "$name: the set holds [$actual], expected [$*]"
}

z3=(z3_4.13.3-1_amd64.deb libz3-4_4.13.3-1_amd64.deb)

# Online, no set kept: update, one fetch, one install, the mirrors untouched.
if run_case online 0 FAKE_APT_SET="${z3[*]}" -- z3; then
  [[ $(calls online '^-o .* update ') == 1 ]] || fail "online: update called $(calls online ' update ') times"
  [[ $(calls online '--download-only') == 1 ]] || fail "online: fetched $(calls online '--download-only') times"
  [[ $(calls online '--print-uris') == 0 ]] || fail "online: listed the set without a directory to keep it in"
  [[ $(installs online | wc -l) == 1 ]] || fail "online: $(installs online | wc -l) installs"
  installs online | grep -q -- ' install -y z3$' || fail "online: install was not 'install -y z3': $(installs online)"
  installed online z3
  installed online libz3-4
  [[ $(grep -c . "$work/online/mirrors") == 2 ]] || fail "online: a mirror was dropped without a failure"
fi

# Online with a set kept: its current archive seeds the cache and is kept, its stale one
# leaves, the new one joins, and the listing is asked against an empty cache.
seed online-refill libz3-4_4.13.3-1_amd64.deb z3_4.12.0-1_amd64.deb
if run_case online-refill 0 FAKE_APT_SET="${z3[*]}" CI_APT_ARCHIVES="$work/online-refill/set" -- --no-install-recommends z3; then
  set_holds online-refill libz3-4_4.13.3-1_amd64.deb z3_4.13.3-1_amd64.deb
  [[ $(cat "$work/online-refill/cache/libz3-4_4.13.3-1_amd64.deb") == seeded ]] || fail "online-refill: the seeded archive was fetched again"
  [[ $(cat "$work/online-refill/set/libz3-4_4.13.3-1_amd64.deb") == seeded ]] || fail "online-refill: the kept archive is not the seeded one"
  [[ $(calls online-refill '--print-uris') == 1 ]] || fail "online-refill: listed the set $(calls online-refill '--print-uris') times"
  grep -- '--print-uris' "$work/online-refill/apt.log" | grep -q "Dir::Cache::Archives=$work/online-refill/cache/" && fail "online-refill: the set was listed against the seeded cache, which hides what it holds"
  grep -- '--print-uris' "$work/online-refill/apt.log" | grep -q -- '--no-install-recommends z3$' || fail "online-refill: the listing did not carry the install options"
  installs online-refill | grep -q -- ' install -y --no-install-recommends z3$' || fail "online-refill: install was $(installs online-refill)"
  [[ -O "$work/online-refill/set/z3_4.13.3-1_amd64.deb" ]] || fail "online-refill: the kept archive is not owned by the caller"
  mentions online-refill 'kept the 2 archives'
fi

# Every update attempt fails: the mirrors are dropped in turn, then the set is installed
# offline from its files alone, with the install options, and the set is left as it was.
seed offline "${z3[@]}"
if run_case offline 0 FAKE_APT_SET="${z3[*]}" FAKE_APT_UPDATE=fail CI_APT_ARCHIVES="$work/offline/set" -- --no-install-recommends z3; then
  [[ $(calls offline ' update ') == 3 ]] || fail "offline: update attempted $(calls offline ' update ') times, expected 3"
  [[ $(cat "$work/offline/mirrors") == $'http://mirror.two/ubuntu\tpriority:2' ]] || fail "offline: mirrors left: $(cat "$work/offline/mirrors")"
  [[ $(installs offline | wc -l) == 1 ]] || fail "offline: $(installs offline | wc -l) installs"
  installs offline | grep -q -- "install -y --no-download --no-install-recommends $work/offline/set/libz3-4_4.13.3-1_amd64.deb $work/offline/set/z3_4.13.3-1_amd64.deb$" || fail "offline: install was $(installs offline)"
  installed offline z3
  mentions offline 'no mirror served z3'
  mentions offline 'installing z3 offline from the 2 archives'
  set_holds offline libz3-4_4.13.3-1_amd64.deb z3_4.13.3-1_amd64.deb
fi

# No set to fall back on: the failure names both the mirrors and the missing set.
if run_case offline-nothing-kept 1 FAKE_APT_SET="${z3[*]}" FAKE_APT_UPDATE=fail -- z3; then
  [[ -z $(installs offline-nothing-kept) ]] || fail "offline-nothing-kept: installed anyway: $(installs offline-nothing-kept)"
  mentions offline-nothing-kept 'no mirror served z3, and CI_APT_ARCHIVES holds no archives'
fi
mkdir -p "$work/offline-empty-set/set"
if run_case offline-empty-set 1 FAKE_APT_SET="${z3[*]}" FAKE_APT_UPDATE=fail CI_APT_ARCHIVES="$work/offline-empty-set/set" -- z3; then
  [[ -z $(installs offline-empty-set) ]] || fail "offline-empty-set: installed anyway"
  mentions offline-empty-set "$work/offline-empty-set/set holds no archives"
fi

# A set lacking a dependency fails in apt; one lacking the requested package itself is
# caught after apt, which installs what it is given without complaint.
seed offline-missing-dependency z3_4.13.3-1_amd64.deb
run_case offline-missing-dependency 100 FAKE_APT_SET="${z3[*]}" FAKE_APT_UPDATE=fail CI_APT_ARCHIVES="$work/offline-missing-dependency/set" -- z3 &&
  mentions offline-missing-dependency 'Unable to fetch some archives'
seed offline-missing-package libz3-4_4.13.3-1_amd64.deb
run_case offline-missing-package 1 FAKE_APT_SET=libz3-4_4.13.3-1_amd64.deb FAKE_APT_UPDATE=fail CI_APT_ARCHIVES="$work/offline-missing-package/set" -- z3 &&
  mentions offline-missing-package "$work/offline-missing-package/set do not install z3"

# The mirror answers update and then stops: the fetch's attempts fail and the set is installed offline.
seed fetch-fails "${z3[@]}"
if run_case fetch-fails 0 FAKE_APT_SET="${z3[*]}" FAKE_APT_FETCH=fail CI_APT_ARCHIVES="$work/fetch-fails/set" -- z3; then
  [[ $(calls fetch-fails '--download-only') == 3 ]] || fail "fetch-fails: fetched $(calls fetch-fails '--download-only') times, expected 3"
  installs fetch-fails | grep -q -- '--no-download' || fail "fetch-fails: install was $(installs fetch-fails)"
  installed fetch-fails z3
fi

# The deadline is spent before the first attempt: no attempt is made and the set is installed offline.
seed deadline-spent "${z3[@]}"
if run_case deadline-spent 0 FAKE_APT_SET="${z3[*]}" CI_APT_DEADLINE_SECONDS=0 CI_APT_ARCHIVES="$work/deadline-spent/set" -- z3; then
  [[ $(calls deadline-spent ' update ') == 0 ]] || fail "deadline-spent: update attempted with no time left"
  mentions deadline-spent 'no time left for attempt 1 of 3'
  installed deadline-spent z3
fi

run_case no-arguments 2 -- || true
run_case options-only 2 -- --no-install-recommends || true

if ((failures > 0)); then
  echo "$failures failure(s)" >&2
  exit 1
fi
echo "all cases passed"

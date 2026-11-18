#!/usr/bin/env bash

# Checks scripts/ci-pip-install.sh against a stand-in pip that plays a truncated answer from
# PyPI, so the retry and the offline paths a real one takes are exercised on every pull request.
set -euo pipefail

script=$(cd "$(dirname "$0")" && pwd)/ci-pip-install.sh
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
stubs=$work/bin
mkdir -p "$stubs"

cat >"$stubs/sleep" <<'STUB'
#!/usr/bin/env bash
exit 0
STUB
# The interpreter stand-in answers `-m pip` with the wheels FAKE_PIP_SET names, after FAKE_PIP_FAILURES
# downloads that fail as a truncated index page does and FAKE_PIP_DAMAGED that deliver a damaged wheel.
cat >"$stubs/python" <<'STUB'
#!/usr/bin/env bash
[[ ${1-} == -m && ${2-} == pip ]] || exec python3 "$@"
shift 2
printf '%s\n' "$*" >>"$FAKE_PIP_LOG"
dest= previous=
for arg in "$@"; do
  [[ $previous == --dest ]] && dest=$arg
  previous=$arg
done
case $1 in
  download)
    count=$(grep -c '^download ' "$FAKE_PIP_LOG")
    if ((count <= ${FAKE_PIP_FAILURES:-0})); then
      echo "json.decoder.JSONDecodeError: Unterminated string starting at: line 1 column 57371" >&2
      exit 2
    fi
    for wheel in $FAKE_PIP_SET; do
      python3 -c 'import sys, zipfile; zipfile.ZipFile(sys.argv[1], "w").writestr("METADATA", "as published")' "$dest/$wheel"
    done
    if ((count <= ${FAKE_PIP_FAILURES:-0} + ${FAKE_PIP_DAMAGED:-0})); then
      # The stored member's bytes change under its recorded CRC.
      sed -i 's/as published/as truncated/' "$dest/${FAKE_PIP_SET%% *}"
    fi ;;
  install) ;;
  *) echo "pip stand-in: unexpected command: $*" >&2; exit 2 ;;
esac
STUB
chmod +x "$stubs"/*

failures=0
fail() {
  echo "FAIL $*" >&2
  failures=$((failures + 1))
}

# run_case <name> <expected exit> [VAR=value ...] -- <script arguments...>: runs the script with
# the stand-ins and CI_PIP_WHEELS at $work/<name>/wheels; the log continues a case run before.
run_case() {
  local name=$1 expected=$2
  shift 2
  local dir=$work/$name vars=() arg
  for arg in "$@"; do
    shift
    [[ $arg == -- ]] && break
    vars+=("$arg")
  done
  [[ ${arg-} == -- ]] || { echo "run_case $name: no -- before the script arguments" >&2; exit 2; }
  mkdir -p "$dir"
  local actual=0
  env PATH="$stubs:$PATH" FAKE_PIP_LOG="$dir/pip.log" CI_PIP_WHEELS="$dir/wheels" "${vars[@]}" \
    bash "$script" --python "$stubs/python" "$@" >"$dir/out" 2>&1 || actual=$?
  if [[ $actual != "$expected" ]]; then
    fail "$name: exit $actual, expected $expected"
    sed 's/^/  /' "$dir/out" >&2
    return 1
  fi
  echo "ok   $name: exit $actual"
}

# calls <name> <command>: how many pip calls of the case ran the command.
calls() {
  grep -c "^$2 " "$work/$1/pip.log" 2>/dev/null || true
}
expect_calls() {
  local name=$1 command=$2 expected=$3
  [[ $(calls "$name" "$command") == "$expected" ]] || fail "$name: $(calls "$name" "$command") pip $command calls, expected $expected"
}
mentions() {
  grep -q -- "$2" "$work/$1/out" || fail "$1: output lacks \"$2\""
}
holds() {
  local name=$1 expected=$2 actual
  actual=$(find "$work/$name/wheels" -maxdepth 1 -name '*.whl' -printf '%f\n' 2>/dev/null | sort | paste -sd ' ' -)
  [[ $actual == "$expected" ]] || fail "$name: the directory holds [$actual], expected [$expected]"
}

fmpy="fmpy-0.3.32-py3-none-any.whl lark-1.3.1-py3-none-any.whl"

# Cold: one download, then an install from the directory alone.
if run_case cold 0 FAKE_PIP_SET="$fmpy" -- fmpy==0.3.32; then
  expect_calls cold download 1
  expect_calls cold install 1
  grep -q "^download .*--only-binary :all: .*--dest $work/cold/wheels.partial fmpy==0.3.32$" "$work/cold/pip.log" || fail "cold: the download is not wheel-only into the partial directory: $(grep '^download' "$work/cold/pip.log")"
  grep -q "^install .*--no-index --find-links $work/cold/wheels --only-binary :all: fmpy==0.3.32$" "$work/cold/pip.log" || fail "cold: the install does not read the directory alone: $(grep '^install' "$work/cold/pip.log")"
  holds cold "$fmpy"
  [[ ! -e $work/cold/wheels.partial ]] || fail "cold: the partial directory was left behind"
fi

# Warm: the directory answers the request, so pip is asked to install and nothing else.
if run_case cold 0 FAKE_PIP_SET="$fmpy" -- fmpy==0.3.32; then
  expect_calls cold download 1
  expect_calls cold install 2
  mentions cold "PyPI is not asked"
fi

# A wheel that changed since it was recorded, and one the record does not name, are a miss.
echo damaged >>"$work/cold/wheels/lark-1.3.1-py3-none-any.whl"
if run_case cold 0 FAKE_PIP_SET="$fmpy" -- fmpy==0.3.32; then
  expect_calls cold download 2
  mentions cold "does not answer the request"
fi
cp "$work/cold/wheels/lark-1.3.1-py3-none-any.whl" "$work/cold/wheels/extra-1.0-py3-none-any.whl"
if run_case cold 0 FAKE_PIP_SET="$fmpy" -- fmpy==0.3.32; then
  expect_calls cold download 3
  holds cold "$fmpy"
fi

# Another request is a miss too, whatever the directory holds.
if run_case cold 0 FAKE_PIP_SET="$fmpy" -- fmpy==0.3.33; then
  expect_calls cold download 4
fi

# A truncated index page twice, then an answer: the third attempt installs.
if run_case truncated 0 FAKE_PIP_SET="$fmpy" FAKE_PIP_FAILURES=2 -- fmpy==0.3.32; then
  expect_calls truncated download 3
  expect_calls truncated install 1
  mentions truncated "pip download failed (attempt 2 of 4)"
  holds truncated "$fmpy"
fi

# A wheel that does not read back is a failed attempt, not a set.
if run_case damaged 0 FAKE_PIP_SET="$fmpy" FAKE_PIP_DAMAGED=1 -- fmpy==0.3.32; then
  expect_calls damaged download 2
  mentions damaged "fails its checksum"
  holds damaged "$fmpy"
fi

# PyPI never answers: the attempts are bounded, nothing is installed and no directory is left.
if run_case outage 1 FAKE_PIP_SET="$fmpy" FAKE_PIP_FAILURES=9 CI_PIP_ATTEMPTS=3 -- fmpy==0.3.32; then
  expect_calls outage download 3
  expect_calls outage install 0
  mentions outage "nothing was installed"
  [[ ! -e $work/outage/wheels && ! -e $work/outage/wheels.partial ]] || fail "outage: a directory was left behind"
fi

# A project adds its build requirements and dependencies to the download and installs nothing.
mkdir -p "$work/project"
printf '[build-system]\nrequires = ["setuptools>=77"]\n[project]\ndependencies = ["grpcio>=1.83.0"]\n' >"$work/project/pyproject.toml"
if run_case project 0 FAKE_PIP_SET="$fmpy" -- --project "$work/project"; then
  grep -q '^download .* setuptools>=77 grpcio>=1.83.0$' "$work/project/pip.log" || fail "project: the download lacks the project's requirements: $(grep '^download' "$work/project/pip.log")"
  expect_calls project install 0
fi

# A requirements file is downloaded and installed by name, and its contents are part of the request.
echo "mkdocs==1.6.1" >"$work/requirements.txt"
if run_case file 0 FAKE_PIP_SET="$fmpy" -- --requirements "$work/requirements.txt"; then
  grep -q "^install .* --requirement $work/requirements.txt$" "$work/file/pip.log" || fail "file: the install does not name the file: $(grep '^install' "$work/file/pip.log")"
  echo "mkdocs==1.6.2" >"$work/requirements.txt"
  run_case file 0 FAKE_PIP_SET="$fmpy" -- --requirements "$work/requirements.txt" && expect_calls file download 2
fi

# --no-install provides the wheels only; no request at all is a usage error.
if run_case wheels-only 0 FAKE_PIP_SET="$fmpy" -- --no-install fmpy==0.3.32; then
  expect_calls wheels-only download 1
  expect_calls wheels-only install 0
fi
run_case empty 2 -- || true

if ((failures > 0)); then
  echo "$failures check(s) failed" >&2
  exit 1
fi
echo "all checks passed"

#!/usr/bin/env bash

# Installs pinned Python packages on a CI runner from a directory of their wheels, so that a
# truncated or failed answer from PyPI is retried and a run that has the wheels never asks it.
set -euo pipefail

usage() {
  echo "usage: CI_PIP_WHEELS=<dir> $0 [--python <interpreter>] [--no-install] [--requirements <file>]... [--project <dir>]... [<requirement>...]" >&2
  exit 2
}
python=python3
install=1
files=()
projects=()
while [[ $# -gt 0 && $1 == --* ]]; do
  case $1 in
    --python) [[ $# -ge 2 ]] || usage; python=$2; shift 2 ;;
    --requirements) [[ $# -ge 2 ]] || usage; files+=("$2"); shift 2 ;;
    --project) [[ $# -ge 2 ]] || usage; projects+=("$2"); shift 2 ;;
    --no-install) install=0; shift ;;
    *) usage ;;
  esac
done
requirements=("$@")
for requirement in "${requirements[@]}"; do
  [[ $requirement != -* ]] || usage
done
wheels=${CI_PIP_WHEELS:-}
[[ -n $wheels ]] || usage
((${#requirements[@]} + ${#files[@]} + ${#projects[@]} > 0)) || usage

# A project adds what building and running it needs, not itself: a later `pip install -e` of it
# under PIP_NO_INDEX and PIP_FIND_LINKS then finds its build backend and its dependencies here.
needs=()
if ((${#projects[@]} > 0)); then
  mapfile -t needs < <("$python" - "${projects[@]}" <<'PY'
import sys
import tomllib

for directory in sys.argv[1:]:
    with open(f"{directory}/pyproject.toml", "rb") as source:
        project = tomllib.load(source)
    for requirement in project.get("build-system", {}).get("requires", []) + project.get("project", {}).get("dependencies", []):
        print(requirement)
PY
  )
  ((${#needs[@]} > 0)) || { echo "error: ${projects[*]} declare no build requirement and no dependency" >&2; exit 1; }
fi

# What is asked for, with the interpreter the wheels are for: a directory filled for another
# request, or for another Python, is not this one's.
request() {
  "$python" -c 'import sys, sysconfig; print("python", "%d.%d" % sys.version_info[:2], sysconfig.get_platform())'
  printf 'requirement %s\n' "${requirements[@]}" "${needs[@]}"
  local file
  for file in "${files[@]}"; do
    echo "requirements $(sha256sum <"$file" | cut -d ' ' -f 1)"
  done
}
wanted=$(request)

# complete <dir>: the directory answers this request and every wheel in it is the one recorded.
complete() {
  local dir=$1
  [[ -f $dir/REQUEST && -f $dir/SHA256SUMS ]] || return 1
  [[ $(<"$dir/REQUEST") == "$wanted" ]] || return 1
  (cd "$dir" && sha256sum --check --strict --quiet SHA256SUMS >&2) || return 1
  [[ $(cd "$dir" && find . -maxdepth 1 -name '*.whl' -printf '%f\n' | sort) == $(awk '{print $2}' "$dir/SHA256SUMS" | sort) ]]
}

download=("${requirements[@]}" "${needs[@]}")
for file in "${files[@]}"; do
  download+=(--requirement "$file")
done

# pip retries a connection, not an index page or a wheel that arrived truncated, so the whole download
# is retried, uncached and bounded (CI_PIP_ATTEMPTS, CI_PIP_TIMEOUT_SECONDS), keeping the wheels it has.
fetch() {
  local attempts=${CI_PIP_ATTEMPTS:-4} bound=${CI_PIP_TIMEOUT_SECONDS:-240} partial=$wheels.partial attempt
  rm -rf "$partial"
  mkdir -p "$partial"
  for ((attempt = 1; attempt <= attempts; attempt++)); do
    if timeout --kill-after=10s "$bound" "$python" -m pip download --disable-pip-version-check --no-cache-dir \
      --only-binary :all: --retries 5 --timeout 30 --dest "$partial" "${download[@]}" &&
      "$python" - "$partial" <<'PY'; then
import pathlib
import sys
import zipfile

# pip checked each wheel against the digest its index page gave; this reads every member back.
wheels = sorted(pathlib.Path(sys.argv[1]).glob("*.whl"))
if not wheels:
    sys.exit("no wheel was downloaded")
for wheel in wheels:
    with zipfile.ZipFile(wheel) as archive:
        damaged = archive.testzip()
    if damaged is not None:
        wheel.unlink()
        sys.exit(f"{wheel.name}: {damaged} fails its checksum")
PY
      (cd "$partial" && find . -maxdepth 1 -name '*.whl' -printf '%f\n' | sort | xargs -d '\n' sha256sum >SHA256SUMS)
      printf '%s\n' "$wanted" >"$partial/REQUEST"
      rm -rf "$wheels"
      mv "$partial" "$wheels"
      return 0
    fi
    echo "pip download failed (attempt $attempt of $attempts)" >&2
    ((attempt == attempts)) || sleep $((15 * attempt))
  done
  rm -rf "$partial"
  return 1
}

if complete "$wheels"; then
  echo "$(grep -c . "$wheels/SHA256SUMS") wheels in $wheels answer the request; PyPI is not asked"
else
  [[ ! -e $wheels ]] || echo "$wheels does not answer the request; downloading it again" >&2
  fetch || { echo "error: PyPI did not serve ${download[*]} in ${CI_PIP_ATTEMPTS:-4} attempts; nothing was installed" >&2; exit 1; }
  echo "$(grep -c . "$wheels/SHA256SUMS") wheels downloaded into $wheels"
fi

# Installed from the directory alone: pip resolves the whole set before it changes anything,
# so a wheel the directory lacks fails the step with nothing installed.
if ((install)) && ((${#requirements[@]} + ${#files[@]} > 0)); then
  only=("${requirements[@]}")
  for file in "${files[@]}"; do
    only+=(--requirement "$file")
  done
  "$python" -m pip install --disable-pip-version-check --no-index --find-links "$wheels" --only-binary :all: "${only[@]}"
fi

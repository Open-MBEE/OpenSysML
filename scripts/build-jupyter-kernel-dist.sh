#!/usr/bin/env bash
#
# Build the jupyter-opensysml-kernel distributions from a release's kernel
# binaries, the way the CircleCI `build-jupyter-kernel-package` job and the
# nightly workflow do: the sdist, then one wheel per released platform with
# that platform's sysml-jupyter-kernel bundled, so `pip install` alone
# registers the kernel.
#
# Usage: scripts/build-jupyter-kernel-dist.sh <kernels-dir> <out-dir>
#
# <kernels-dir> holds the five sysml-jupyter-kernel-<os>-<arch>[.exe] binaries
# build-release-artifacts.sh wrote to dist/jupyter, each checked against its
# .sha256 sidecar before it is staged. <out-dir> is emptied first and ends up
# holding:
#
#   jupyter_opensysml_kernel-<version>.tar.gz                 the sdist, kernel-less
#   jupyter_opensysml_kernel-<version>-py3-none-<platform>.whl  one per platform
#
# The JupyterLab extension is built first (`make jupyterlab-build`, so Node and
# the jupyterlab package are needed); every distribution ships the same bundle.
# Every wheel is then opened and checked to carry its kernel, the kernelspec
# that registers it and the extension, and the sdist to carry the extension and
# no kernel. PYTHON names the interpreter that runs `python -m build` (default
# `python`).
set -euo pipefail

KERNELS="${1:?usage: $0 <kernels-dir> <out-dir>}"
OUT="${2:?usage: $0 <kernels-dir> <out-dir>}"
PYTHON="${PYTHON:-python}"
PACKAGE=client/jupyter-kernel
BIN="$PACKAGE/jupyter_opensysml_kernel/bin"
PLATFORMS=(linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64)

# The staging directory and setup.py's kernelspec scratch never outlive a build.
cleanup() { rm -rf "$BIN" "$PACKAGE/build"; }
trap cleanup EXIT
cleanup

rm -rf "$OUT"
mkdir -p "$OUT"

# The extension once, before any distribution: setup.py refuses to build one
# without it.
make jupyterlab-build

# The sdist first, while no kernel is staged: setup.py refuses to pack one.
"$PYTHON" -m build --sdist --outdir "$OUT" "$PACKAGE"

for platform in "${PLATFORMS[@]}"; do
  asset="sysml-jupyter-kernel-${platform}"
  staged="sysml-jupyter-kernel"
  if [[ "$platform" == windows-* ]]; then
    asset+=".exe"
    staged+=".exe"
  fi
  if [[ ! -f "$KERNELS/$asset" ]]; then
    echo "Error: $KERNELS/$asset is missing; build the release binaries first" >&2
    exit 1
  fi
  # The sidecar is what the package's download path verifies; the bundled copy
  # is held to the same bytes.
  (cd "$KERNELS" && sha256sum --check --quiet "${asset}.sha256")
  mkdir -p "$BIN"
  cp "$KERNELS/$asset" "$BIN/$staged"
  chmod 755 "$BIN/$staged"
  JUPYTER_OPENSYSML_KERNEL_PLATFORM="$platform" \
    "$PYTHON" -m build --wheel --outdir "$OUT" "$PACKAGE"
  rm -rf "$BIN"
done

ls -l "$OUT"

# Each distribution is what its name claims: the wheels bundle the kernel built
# for their platform and the kernelspec that starts it, the sdist carries
# neither, and all of them carry the one JupyterLab extension build.
"$PYTHON" - "$OUT" <<'PY'
import json
import os
import sys
import tarfile
import zipfile

sys.path.insert(0, os.path.join("client", "jupyter-kernel"))
from jupyter_opensysml_kernel.binary import WHEEL_PLATFORM_TAGS, binary_name  # noqa: E402
from jupyter_opensysml_kernel.labextension import SHARED_DATA  # noqa: E402

out = sys.argv[1]
spec = "share/jupyter/kernels/sysml/"
labextension = SHARED_DATA + "/"


def labextension_files(what, members, prefix):
    """The extension's files under `prefix` in a distribution, checked to be a
    loadable federated extension: name -> bytes."""
    files = {name[len(prefix):]: read for name, read in members.items() if name.startswith(prefix)}
    for required in ("package.json", "install.json"):
        if required not in files:
            raise SystemExit(f"Error: {what} lacks {prefix}{required}")
    load = json.loads(files["package.json"]()).get("jupyterlab", {}).get("_build", {}).get("load", "")
    if not load.startswith("static/remoteEntry.") or load not in files:
        raise SystemExit(f"Error: {what} ships a labextension whose package.json loads {load!r}, which it lacks")
    return {name: read() for name, read in files.items()}


bundles = {}
wheels = sorted(f for f in os.listdir(out) if f.endswith(".whl"))
sdists = sorted(f for f in os.listdir(out) if f.endswith(".tar.gz"))
if len(sdists) != 1:
    raise SystemExit(f"Error: expected one sdist in {out}, found {sdists}")
if len(wheels) != len(WHEEL_PLATFORM_TAGS):
    raise SystemExit(f"Error: expected {len(WHEEL_PLATFORM_TAGS)} wheels in {out}, found {wheels}")

for (goos, goarch), tag in WHEEL_PLATFORM_TAGS.items():
    matching = [w for w in wheels if w.endswith(f"-py3-none-{tag}.whl")]
    if len(matching) != 1:
        raise SystemExit(f"Error: no wheel tagged {tag} for {goos}-{goarch} in {out}: {wheels}")
    with zipfile.ZipFile(os.path.join(out, matching[0])) as wheel:
        names = set(wheel.namelist())
        kernel = f"jupyter_opensysml_kernel/bin/{binary_name(goos)}"
        bundled = sorted(n for n in names if n.startswith("jupyter_opensysml_kernel/bin/"))
        if bundled != [kernel]:
            raise SystemExit(
                f"Error: {matching[0]} must bundle exactly {kernel}; found {', '.join(bundled) or 'nothing'}"
            )
        data = next((n for n in names if n.endswith(".data/data/" + spec + "kernel.json")), None)
        missing = []
        if data is None:
            missing.append(spec + "kernel.json")
        else:
            prefix = data[: -len("kernel.json")]
            missing += [p for p in ("logo-32x32.png", "logo-64x64.png") if prefix + p not in names]
        if missing:
            raise SystemExit(f"Error: {matching[0]} lacks {', '.join(missing)}")
        meta = next(n for n in names if n.endswith(".dist-info/WHEEL"))
        if b"Root-Is-Purelib: false" not in wheel.read(meta):
            raise SystemExit(f"Error: {matching[0]} is not marked platform-specific in its WHEEL metadata")
        data_dir = data[: -len(spec + "kernel.json")]
        members = {n: (lambda n=n: wheel.read(n)) for n in names}
        bundles[matching[0]] = labextension_files(matching[0], members, data_dir + labextension)
    print(f"ok: {matching[0]} bundles {kernel}, the kernelspec and the labextension")

with tarfile.open(os.path.join(out, sdists[0]), "r:gz") as sdist:
    names = sdist.getnames()
    bundled = [n for n in names if "/jupyter_opensysml_kernel/bin/" in n]
    if bundled:
        raise SystemExit(f"Error: the sdist {sdists[0]} bundles a kernel: {bundled}")
    root = names[0].split("/")[0] + "/labextension/"
    members = {n: (lambda n=n: sdist.extractfile(n).read()) for n in names if sdist.getmember(n).isfile()}
    bundles[sdists[0]] = labextension_files(sdists[0], members, root)
print(f"ok: {sdists[0]} bundles no kernel and the labextension")

# The bundle is pure JavaScript: one build, byte for byte, in all six.
reference = bundles[sdists[0]]
for name, files in bundles.items():
    if files != reference:
        raise SystemExit(f"Error: {name} ships a labextension other than the sdist's: {sorted(set(files) ^ set(reference)) or 'same files, different bytes'}")
print(f"ok: all {len(bundles)} distributions ship the same labextension ({', '.join(sorted(reference))})")
PY

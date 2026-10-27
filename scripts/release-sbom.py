#!/usr/bin/env python3
"""Write one SPDX 2.3 JSON SBOM per release asset, for the checksum manifest to list.

Usage: scripts/release-sbom.py --syft syft --version v0.12.0 --out dist/sbom ASSET...

Every ASSET (a release archive, a wheel or sdist, or a raw binary) gets
`<out>/<asset file name>.spdx.json`, written by syft and then checked against
the asset itself before it is accepted: each Go executable the asset holds is
read with `go version -m`, and every module it was built from must be a package
of the SBOM at that version, with the Go toolchain as the SBOM's `stdlib`
package; an SBOM naming no component at all is refused, as is one mentioning
the builder's own paths. The document namespace is the URL the SBOM is
published at, so two builds of one tag differ only in the creation timestamp
syft records.

A wheel is a zip archive syft does not recognise by its suffix, so it is
scanned through a `.zip` copy under the asset's own name.
"""

from __future__ import annotations

import argparse
import json
import os
import pathlib
import shutil
import subprocess
import sys
import tarfile
import tempfile
import typing
import urllib.parse
import zipfile

RELEASE_URL = "https://github.com/Open-MBEE/OpenSysML/releases/download"
SUFFIX = ".spdx.json"


class SBOMError(Exception):
    pass


class BuildInfo(typing.NamedTuple):
    """What `go version -m` reports for one executable: the toolchain and the modules built in."""

    toolchain: str
    modules: tuple

    @property
    def purls(self):
        want = {f"pkg:golang/{path}@{version}" for path, version in self.modules}
        want.add(f"pkg:golang/stdlib@{self.toolchain.removeprefix('go')}")
        return want


def parse_build_info(text):
    """Parse `go version -m` output. A `=>` line replaces the module on the line before it."""
    lines = text.splitlines()
    if not lines or ": " not in lines[0]:
        raise SBOMError(f"not `go version -m` output: {text[:80]!r}")
    toolchain = lines[0].rsplit(": ", 1)[1].strip()
    if not toolchain.startswith("go"):
        raise SBOMError(f"no Go toolchain in {lines[0]!r}")
    modules = []
    for line in lines[1:]:
        fields = line.split("\t")
        if len(fields) < 4 or fields[0] != "":
            continue
        kind, path, version = fields[1], fields[2], fields[3]
        if kind in ("mod", "dep"):
            modules.append((path, version))
        elif kind == "=>" and modules:
            modules[-1] = (path, version)
    return BuildInfo(toolchain, tuple(modules))


def go_build_info(go, path):
    """The build info of a Go executable, or None when `path` is not one."""
    result = subprocess.run([go, "version", "-m", str(path)], capture_output=True, text=True)
    if result.returncode != 0 or "\tmod\t" not in result.stdout:
        return None
    return parse_build_info(result.stdout)


def unpacked(asset, into):
    """The regular files of an archive asset, written flat under `into`; a plain file is itself."""
    asset = pathlib.Path(asset)
    if zipfile.is_zipfile(asset):
        with zipfile.ZipFile(asset) as archive:
            members = [m for m in archive.infolist() if not m.is_dir()]
            return [_extract(m.filename, archive.open(m), into, i) for i, m in enumerate(members)]
    if tarfile.is_tarfile(asset):
        with tarfile.open(asset) as archive:
            members = [m for m in archive.getmembers() if m.isfile()]
            return [_extract(m.name, archive.extractfile(m), into, i) for i, m in enumerate(members)]
    return [(asset.name, asset)]


def _extract(member, stream, into, index):
    target = pathlib.Path(into) / str(index) / pathlib.PurePosixPath(member).name
    target.parent.mkdir(parents=True, exist_ok=True)
    with stream, open(target, "wb") as out:
        shutil.copyfileobj(stream, out)
    return (member, target)


def scan_target(asset, tmp):
    """The path syft scans for `asset`: the asset, or a `.zip` copy of a wheel."""
    asset = pathlib.Path(asset)
    if asset.suffix != ".whl":
        return asset
    copy = pathlib.Path(tmp) / (asset.name + ".zip")
    shutil.copyfile(asset, copy)
    return copy


def scan(syft, asset, version, out, tmp):
    """Run syft over `asset`, writing the SPDX JSON document to `out`, and return it."""
    asset = pathlib.Path(asset)
    command = [
        syft, "scan", f"file:{scan_target(asset, tmp)}",
        "--source-name", asset.name, "--source-version", version,
        "-o", f"spdx-json={out}", "-q",
    ]
    env = dict(os.environ, SYFT_CHECK_FOR_APP_UPDATE="false")
    result = subprocess.run(command, env=env, capture_output=True, text=True)
    if result.returncode != 0:
        raise SBOMError(f"syft failed over {asset.name}: {result.stderr.strip()}")
    with open(out, encoding="utf-8") as handle:
        return json.load(handle)


def components(doc):
    """The packages an SBOM names, apart from the asset itself."""
    return [p for p in doc.get("packages", []) if not p["SPDXID"].startswith("SPDXRef-DocumentRoot")]


def purls(doc):
    """The package URLs an SBOM carries, percent-decoded (a `+dirty` version is written `%2Bdirty`)."""
    return {
        urllib.parse.unquote(ref["referenceLocator"])
        for package in doc.get("packages", [])
        for ref in package.get("externalRefs", [])
        if ref.get("referenceType") == "purl"
    }


def check(doc, name, infos, forbidden):
    """Refuse an SBOM that does not describe the asset the manifest will vouch for."""
    if not components(doc):
        raise SBOMError(f"{name}: the SBOM names no component")
    found = purls(doc)
    for member, info in infos:
        missing = sorted(info.purls - found)
        if missing:
            raise SBOMError(f"{name}: {member} was built from {', '.join(missing)}, which the SBOM lacks")
    text = json.dumps(doc)
    for path in forbidden:
        if path and path in text:
            raise SBOMError(f"{name}: the SBOM mentions the builder's path {path}")


def finalize(doc, name, version):
    """Give the document its published URL as namespace, in place of syft's random one."""
    doc["documentNamespace"] = f"{RELEASE_URL}/{version}/{name}{SUFFIX}"
    return doc


def describe(doc, name, infos):
    toolchains = sorted({info.toolchain for _, info in infos})
    detail = f", Go {', '.join(toolchains)}" if toolchains else ""
    return f"{name}{SUFFIX}: {len(components(doc))} components{detail}"


def write_sbom(asset, version, out_dir, syft, go):
    asset = pathlib.Path(asset)
    out = pathlib.Path(out_dir) / (asset.name + SUFFIX)
    with tempfile.TemporaryDirectory() as tmp:
        doc = scan(syft, asset, version, out, tmp)
        infos = []
        for member, path in unpacked(asset, pathlib.Path(tmp) / "unpacked"):
            info = go_build_info(go, path)
            if info is not None:
                infos.append((member, info))
        forbidden = [str(asset.resolve().parent), tmp, os.getcwd()]
        check(doc, asset.name, infos, forbidden)
    finalize(doc, asset.name, version)
    out.write_text(json.dumps(doc, indent=1) + "\n", encoding="utf-8")
    return describe(doc, asset.name, infos)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    parser.add_argument("--syft", default="syft", help="the syft executable")
    parser.add_argument("--go", default="go", help="the go executable, for `go version -m`")
    parser.add_argument("--version", required=True, help="the release tag the assets belong to")
    parser.add_argument("--out", required=True, type=pathlib.Path, help="directory to write the SBOMs to")
    parser.add_argument("assets", nargs="+", type=pathlib.Path, help="the release assets to describe")
    args = parser.parse_args(argv)
    args.out.mkdir(parents=True, exist_ok=True)
    try:
        for asset in args.assets:
            print(write_sbom(asset, args.version, args.out, args.syft, args.go))
    except (SBOMError, OSError) as err:
        print(f"release-sbom: {err}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())

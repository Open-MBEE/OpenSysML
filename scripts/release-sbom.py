#!/usr/bin/env python3
"""Write one SPDX 2.3 JSON SBOM per release asset, for the checksum manifest to list.

Usage: scripts/release-sbom.py --syft syft --version v0.12.0 --out dist/sbom ASSET...
       scripts/release-sbom.py --version v0.12.0 --out dist/sbom --npm-lock client/node/package-lock.json TARBALL
       scripts/release-sbom.py --version v0.12.0 --out dist/sbom --msi-inputs dist/unsigned \\
           --z3-pin packaging/msi/z3.pin --z3-zip z3-5.1.0-x64-win.zip

Every ASSET (a release archive, a wheel or sdist, or a raw binary) gets
`<out>/<asset file name>.spdx.json`, written by syft and then checked against
the asset itself before it is accepted: each Go executable the asset holds is
read with `go version -m`, and every module it was built from must be a package
of the SBOM at that version, with the Go toolchain as the SBOM's `stdlib`
package; an SBOM naming no component at all is refused, as is one mentioning
the builder's own paths. The document namespace is the URL the SBOM is
published at, so two builds of one tag differ only in the creation timestamp.

A wheel is a zip archive syft does not recognise by its suffix, so it is
scanned through a `.zip` copy under the asset's own name.

A WebAssembly asset (`.wasm`, or `.wasm.gz`) is read directly: Go embeds the
same module information in a wasm module as in any executable, but neither
`debug/buildinfo` nor syft reads this file format, so the document is written
here from the embedded module list and the toolchain string.

An npm tarball's SBOM describes the packed manifest (`package/package.json`,
with the exact version npm packed) and resolves its dependencies through the
lockfile `--npm-lock` names, which syft catalogues; every runtime dependency
the manifest declares must be resolved there, at the pinned version when the
pin is exact.

The Windows installer is built elsewhere from inputs this script can see: the
SBOM of `opensysml-<x.y.z>-windows-amd64.msi` (`--msi-signed`: of the `-signed`
one, from the Authenticode-signed executables) is written from the Go
executables in `--msi-inputs`, with the bundled Z3 declared from
`packaging/msi/z3.pin` and the release zip it names, whose SHA-256 must match
the pin.
"""

from __future__ import annotations

import argparse
import datetime
import gzip
import hashlib
import json
import os
import pathlib
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
import typing
import urllib.parse
import zipfile

RELEASE_URL = "https://github.com/Open-MBEE/OpenSysML/releases/download"
NPM_REGISTRY = "https://registry.npmjs.org"
Z3_RELEASES = "https://github.com/Z3Prover/z3/releases/download"
SUFFIX = ".spdx.json"
WASM_SUFFIXES = (".wasm", ".wasm.gz")

# Go's linker brackets the module information with these two markers (debug/buildinfo's infoStart/infoEnd).
MODINFO_START = bytes.fromhex("3077af0c9274080241e1c107e6d618e6")
MODINFO_END = bytes.fromhex("f932433186182072008242104116d8f2")
_TOOLCHAIN = re.compile(rb"go1\.[0-9]+\.[0-9]+")
_EXACT_VERSION = re.compile(r"^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$")
_MSI_TAG = re.compile(r"^v([0-9]+)\.([0-9]+)\.([0-9]+)(?:[-+].*)?$")

# What build-msi.sh takes from the Z3 zip, under the names the MSI installs them as.
Z3_FILES = (
    ("bin/z3.exe", "z3.exe"),
    ("bin/msvcp140.dll", "msvcp140.dll"),
    ("bin/vcruntime140.dll", "vcruntime140.dll"),
    ("bin/vcruntime140_1.dll", "vcruntime140_1.dll"),
    ("LICENSE.txt", "LICENSE-z3.txt"),
)


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


def wasm_build_info(data, name):
    """The module information Go embedded in a wasm module, which `go version -m` cannot read."""
    start = data.find(MODINFO_START)
    end = data.find(MODINFO_END, start) if start >= 0 else -1
    if start < 0 or end < 0:
        raise SBOMError(f"{name}: no Go module information in the module")
    toolchains = sorted({match.decode() for match in _TOOLCHAIN.findall(data)})
    if len(toolchains) != 1:
        raise SBOMError(f"{name}: the module names {len(toolchains)} Go toolchains: {', '.join(toolchains)}")
    modinfo = data[start + len(MODINFO_START):end].decode()
    lines = [f"{name}: {toolchains[0]}"] + [f"\t{line}" for line in modinfo.splitlines()]
    return parse_build_info("\n".join(lines))


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


def scan(syft, source, name, version, out):
    """Run syft over `source` (`file:...` or `dir:...`) as the SBOM of `name`, writing SPDX JSON to `out`."""
    command = [
        syft, "scan", source,
        "--source-name", name, "--source-version", version,
        "-o", f"spdx-json={out}", "-q",
    ]
    env = dict(os.environ, SYFT_CHECK_FOR_APP_UPDATE="false")
    result = subprocess.run(command, env=env, capture_output=True, text=True)
    if result.returncode != 0:
        raise SBOMError(f"syft failed over {name}: {result.stderr.strip()}")
    with open(out, encoding="utf-8") as handle:
        return json.load(handle)


def spdx_id(kind, name):
    return "SPDXRef-Package-" + re.sub(r"[^A-Za-z0-9.-]", "-", f"{kind}-{name}")


def root_id(doc):
    """The SPDX element the document describes: the asset itself."""
    for relationship in doc.get("relationships", []):
        if relationship["relationshipType"] == "DESCRIBES" and relationship["spdxElementId"] == "SPDXRef-DOCUMENT":
            return relationship["relatedSpdxElement"]
    raise SBOMError(f"{doc.get('name')}: the SBOM describes nothing")


def purl_ref(purl):
    return {"referenceCategory": "PACKAGE-MANAGER", "referenceType": "purl", "referenceLocator": purl}


def sha256(path):
    with open(path, "rb") as handle:
        return hashlib.file_digest(handle, "sha256").hexdigest()


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


def go_module_package(path, version, member):
    """A Go module as syft records one read from a binary's module information."""
    purl_version = version.removeprefix("go") if path == "stdlib" else version
    return {
        "name": path,
        "SPDXID": spdx_id("go-module", f"{path}@{version}"),
        "versionInfo": version,
        "supplier": "NOASSERTION",
        "downloadLocation": "NOASSERTION",
        "filesAnalyzed": False,
        "sourceInfo": f"acquired package info from go module information: /{member}",
        "licenseConcluded": "NOASSERTION",
        "licenseDeclared": "BSD-3-Clause" if path == "stdlib" else "NOASSERTION",
        "copyrightText": "NOASSERTION",
        "externalRefs": [purl_ref(f"pkg:golang/{path}@{urllib.parse.quote(purl_version, safe='')}")],
    }


def authored_document(name, version, packages):
    """An SPDX document of this script's own, describing `name` as holding `packages`."""
    root = "SPDXRef-DocumentRoot-File-" + re.sub(r"[^A-Za-z0-9.-]", "-", name)
    created = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    return {
        "spdxVersion": "SPDX-2.3",
        "dataLicense": "CC0-1.0",
        "SPDXID": "SPDXRef-DOCUMENT",
        "name": name,
        "documentNamespace": f"{RELEASE_URL}/{version}/{name}{SUFFIX}",
        "creationInfo": {"created": created, "creators": ["Organization: Open-MBEE", "Tool: release-sbom.py"]},
        "packages": [
            {
                "name": name,
                "SPDXID": root,
                "versionInfo": version,
                "supplier": "Organization: Open-MBEE",
                "downloadLocation": f"{RELEASE_URL}/{version}/{name}",
                "filesAnalyzed": False,
                "licenseConcluded": "NOASSERTION",
                "licenseDeclared": "NOASSERTION",
                "copyrightText": "NOASSERTION",
            },
            *packages,
        ],
        "relationships": [
            {"spdxElementId": "SPDXRef-DOCUMENT", "relatedSpdxElement": root, "relationshipType": "DESCRIBES"},
            *(
                {"spdxElementId": root, "relatedSpdxElement": p["SPDXID"], "relationshipType": "CONTAINS"}
                for p in packages
            ),
        ],
    }


def wasm_document(asset, version):
    """The SBOM of a wasm module, from the module information Go embedded in it."""
    asset = pathlib.Path(asset)
    data = asset.read_bytes()
    if asset.name.endswith(".gz"):
        data = gzip.decompress(data)
    info = wasm_build_info(data, asset.name)
    packages = [go_module_package(path, v, asset.name) for path, v in info.modules]
    packages.append(go_module_package("stdlib", info.toolchain, asset.name))
    return authored_document(asset.name, version, packages), info


def npm_manifest(tarball):
    """The manifest npm packed into `tarball`."""
    with tarfile.open(tarball) as archive:
        try:
            member = archive.extractfile("package/package.json")
        except KeyError:
            member = None
        if member is None:
            raise SBOMError(f"{pathlib.Path(tarball).name}: no package/package.json in the tarball")
        with member:
            return json.load(member)


def npm_tarball_name(manifest):
    """The file name `npm pack` gives a manifest: scope without `@`, `/` as `-`, then the version."""
    return f"{manifest['name'].lstrip('@').replace('/', '-')}-{manifest['version']}.tgz"


def npm_dependencies(manifest):
    """Every dependency npm installs with the package, with its range; peers are the installer's own."""
    deps = {}
    for key in ("dependencies", "optionalDependencies"):
        deps.update(manifest.get(key) or {})
    return deps


def npm_package(manifest, tarball):
    """The packed manifest as an SPDX package, with the tarball's digest."""
    name, version = manifest["name"], manifest["version"]
    return {
        "name": name,
        "SPDXID": spdx_id("npm", f"{name}@{version}"),
        "versionInfo": version,
        "supplier": "Organization: Open-MBEE",
        "downloadLocation": f"{NPM_REGISTRY}/{name}/-/{name.split('/')[-1]}-{version}.tgz",
        "filesAnalyzed": False,
        "checksums": [{"algorithm": "SHA256", "checksumValue": sha256(tarball)}],
        "sourceInfo": f"acquired package info from the packed manifest: package/package.json of {pathlib.Path(tarball).name}",
        "licenseConcluded": "NOASSERTION",
        "licenseDeclared": manifest.get("license", "NOASSERTION"),
        "copyrightText": "NOASSERTION",
        "externalRefs": [purl_ref(f"pkg:npm/{urllib.parse.quote(name, safe='/')}@{version}")],
    }


def declared_npm_package(name, version):
    """An optional dependency the packed manifest pins exactly but the lockfile could not resolve yet."""
    return {
        "name": name,
        "SPDXID": spdx_id("npm-declared", f"{name}@{version}"),
        "versionInfo": version,
        "supplier": "NOASSERTION",
        "downloadLocation": f"{NPM_REGISTRY}/{name}/-/{name.split('/')[-1]}-{version}.tgz",
        "filesAnalyzed": False,
        "sourceInfo": "declared as an optional dependency by the packed manifest; "
                      "published with this release, so not yet resolvable through the lockfile",
        "licenseConcluded": "NOASSERTION",
        "licenseDeclared": "NOASSERTION",
        "copyrightText": "NOASSERTION",
        "externalRefs": [purl_ref(f"pkg:npm/{urllib.parse.quote(name, safe='/')}@{version}")],
    }


def describe_npm(doc, manifest, tarball, lock_root):
    """Make the lockfile's SBOM describe the packed tarball: its manifest is the package the dependencies hang off.

    The platform packages a release publishes beside the tarball are pinned exactly
    but cannot be in the lockfile before they exist, so an optional dependency the
    lockfile lacks is recorded from the manifest's pin, saying so.
    """
    package = npm_package(manifest, tarball)
    replaced = [p["SPDXID"] for p in components(doc)
                if p["name"] == lock_root.get("name") and p.get("versionInfo") == lock_root.get("version")]
    doc["packages"] = [p for p in doc["packages"] if p["SPDXID"] not in replaced] + [package]
    for relationship in doc.get("relationships", []):
        for key in ("spdxElementId", "relatedSpdxElement"):
            if relationship[key] in replaced:
                relationship[key] = package["SPDXID"]
    doc.setdefault("relationships", []).append(
        {"spdxElementId": root_id(doc), "relatedSpdxElement": package["SPDXID"], "relationshipType": "CONTAINS"}
    )
    resolved = {p["name"] for p in components(doc)}
    for dependency, wanted in sorted((manifest.get("optionalDependencies") or {}).items()):
        if dependency not in resolved and _EXACT_VERSION.match(wanted):
            declared = declared_npm_package(dependency, wanted)
            doc["packages"].append(declared)
            doc["relationships"].append(
                {"spdxElementId": declared["SPDXID"], "relatedSpdxElement": package["SPDXID"], "relationshipType": "DEPENDENCY_OF"}
            )
    return doc


def check_npm(doc, name, manifest):
    """Refuse an npm SBOM that lacks a dependency the packed manifest declares, or has it at another version."""
    versions = {}
    for package in components(doc):
        versions.setdefault(package["name"], set()).add(package.get("versionInfo"))
    for dependency, wanted in sorted(npm_dependencies(manifest).items()):
        found = versions.get(dependency)
        if not found:
            raise SBOMError(f"{name}: depends on {dependency}@{wanted}, which the SBOM lacks")
        if _EXACT_VERSION.match(wanted) and wanted not in found:
            raise SBOMError(f"{name}: pins {dependency}@{wanted}, the SBOM has {', '.join(sorted(found))}")


def write_npm_sbom(tarball, lock, version, out_dir, syft):
    tarball, lock = pathlib.Path(tarball), pathlib.Path(lock)
    manifest = npm_manifest(tarball)
    if tarball.name != npm_tarball_name(manifest):
        raise SBOMError(f"{tarball.name}: packed as {manifest['name']}@{manifest['version']}, "
                        f"which npm names {npm_tarball_name(manifest)}")
    out = pathlib.Path(out_dir) / (tarball.name + SUFFIX)
    doc = scan(syft, f"file:{lock}", tarball.name, version, out)
    with open(lock, encoding="utf-8") as handle:
        lock_root = json.load(handle).get("packages", {}).get("", {})
    describe_npm(doc, manifest, tarball, lock_root)
    check_npm(doc, tarball.name, manifest)
    check(doc, tarball.name, [], [str(tarball.resolve().parent), str(lock.resolve().parent), os.getcwd()])
    finalize(doc, tarball.name, version)
    out.write_text(json.dumps(doc, indent=1) + "\n", encoding="utf-8")
    return describe(doc, tarball.name, [])


def read_pin(path):
    """The `KEY=VALUE` assignments of a pin file build-msi.sh sources."""
    pin = {}
    for line in pathlib.Path(path).read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if line and not line.startswith("#") and "=" in line:
            key, value = line.split("=", 1)
            pin[key.strip()] = value.strip().strip('"')
    for key in ("Z3_VERSION", "Z3_ZIP", "Z3_SHA256"):
        if key not in pin:
            raise SBOMError(f"{path}: no {key}")
    return pin


def z3_package(pin, zip_path):
    """The bundled Z3 as a declared SPDX package, with the files the installer takes from its zip."""
    digest = sha256(zip_path)
    if digest != pin["Z3_SHA256"]:
        raise SBOMError(f"{pathlib.Path(zip_path).name} hashes to {digest}, the pin expects {pin['Z3_SHA256']}")
    top = pin["Z3_ZIP"].removesuffix(".zip")
    files = []
    with zipfile.ZipFile(zip_path) as archive:
        for member, installed in Z3_FILES:
            try:
                data = archive.read(f"{top}/{member}")
            except KeyError:
                raise SBOMError(f"{pin['Z3_ZIP']} has no {top}/{member}") from None
            files.append({
                "fileName": f"./z3/{installed}",
                "SPDXID": "SPDXRef-File-z3-" + re.sub(r"[^A-Za-z0-9.-]", "-", installed),
                "checksums": [{"algorithm": "SHA256", "checksumValue": hashlib.sha256(data).hexdigest()}],
                "licenseConcluded": "NOASSERTION",
                "copyrightText": "NOASSERTION",
            })
    package = {
        "name": "z3",
        "SPDXID": spdx_id("github", f"z3@{pin['Z3_VERSION']}"),
        "versionInfo": pin["Z3_VERSION"],
        "supplier": "Organization: Microsoft Research",
        "downloadLocation": f"{Z3_RELEASES}/z3-{pin['Z3_VERSION']}/{pin['Z3_ZIP']}",
        "filesAnalyzed": False,
        "checksums": [{"algorithm": "SHA256", "checksumValue": digest}],
        "sourceInfo": f"declared from packaging/msi/z3.pin: {', '.join(i for _, i in Z3_FILES)} taken from {pin['Z3_ZIP']}",
        "licenseConcluded": "NOASSERTION",
        "licenseDeclared": "MIT",
        "copyrightText": "NOASSERTION",
        "externalRefs": [purl_ref(f"pkg:github/z3prover/z3@z3-{pin['Z3_VERSION']}")],
    }
    return package, files


def msi_name(version, signed):
    """The installer name a tag yields, as build-msi.sh derives the ProductVersion: `v0.12.0-rc1` is `0.12.0`."""
    match = _MSI_TAG.match(version)
    if not match:
        raise SBOMError(f"{version!r} is not a release tag v<major>.<minor>.<patch>[-suffix]")
    return f"opensysml-{'.'.join(match.groups())}-windows-amd64{'-signed' if signed else ''}.msi"


def write_msi_sbom(inputs, pin_path, zip_path, version, out_dir, syft, go, signed):
    """The installer's SBOM from its inputs: the executables it is built from (signed ones for the signed MSI) and the pinned Z3."""
    inputs = pathlib.Path(inputs)
    executables = sorted(p for p in inputs.iterdir() if p.suffix == ".exe")
    if not executables:
        raise SBOMError(f"{inputs}: no .exe to describe")
    infos = []
    for executable in executables:
        info = go_build_info(go, executable)
        if info is None:
            raise SBOMError(f"{executable.name}: not a Go executable")
        infos.append((executable.name, info))
    package, files = z3_package(read_pin(pin_path), zip_path)
    name = msi_name(version, signed)
    out = pathlib.Path(out_dir) / (name + SUFFIX)
    doc = scan(syft, f"dir:{inputs}", name, version, out)
    doc["packages"].append(package)
    doc["files"] = doc.get("files", []) + files
    doc["relationships"] = doc.get("relationships", []) + [
        {"spdxElementId": root_id(doc), "relatedSpdxElement": package["SPDXID"], "relationshipType": "CONTAINS"},
        *({"spdxElementId": package["SPDXID"], "relatedSpdxElement": f["SPDXID"], "relationshipType": "CONTAINS"}
          for f in files),
    ]
    check(doc, name, infos, [str(inputs.resolve()), str(pathlib.Path(zip_path).resolve().parent), os.getcwd()])
    finalize(doc, name, version)
    out.write_text(json.dumps(doc, indent=1) + "\n", encoding="utf-8")
    return describe(doc, name, infos) + f", Z3 {package['versionInfo']}"


def write_sbom(asset, version, out_dir, syft, go):
    asset = pathlib.Path(asset)
    out = pathlib.Path(out_dir) / (asset.name + SUFFIX)
    with tempfile.TemporaryDirectory() as tmp:
        if asset.name.endswith(WASM_SUFFIXES):
            doc, info = wasm_document(asset, version)
            infos = [(asset.name, info)]
        else:
            doc = scan(syft, f"file:{scan_target(asset, tmp)}", asset.name, version, out)
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
    parser.add_argument("--npm-lock", type=pathlib.Path,
                        help="the package-lock.json resolving the dependencies of the npm tarballs given")
    parser.add_argument("--msi-inputs", type=pathlib.Path,
                        help="directory of the Go executables the Windows installer is built from")
    parser.add_argument("--z3-pin", type=pathlib.Path, help="packaging/msi/z3.pin, the bundled Z3 release")
    parser.add_argument("--z3-zip", type=pathlib.Path, help="the Z3 release zip the pin names, as downloaded")
    parser.add_argument("--msi-signed", action="store_true",
                        help="the inputs are the Authenticode-signed executables: name the SBOM after the -signed installer")
    parser.add_argument("assets", nargs="*", type=pathlib.Path, help="the release assets to describe")
    args = parser.parse_args(argv)
    msi = (args.msi_inputs, args.z3_pin, args.z3_zip)
    if any(msi) and not all(msi):
        parser.error("--msi-inputs, --z3-pin and --z3-zip go together")
    if not any(msi) and not args.assets:
        parser.error("no assets to describe")
    if args.npm_lock and any(asset.suffix != ".tgz" for asset in args.assets):
        parser.error("--npm-lock describes npm tarballs (.tgz) only")
    args.out.mkdir(parents=True, exist_ok=True)
    try:
        if all(msi):
            print(write_msi_sbom(args.msi_inputs, args.z3_pin, args.z3_zip, args.version, args.out,
                                 args.syft, args.go, args.msi_signed))
        for asset in args.assets:
            if args.npm_lock:
                print(write_npm_sbom(asset, args.npm_lock, args.version, args.out, args.syft))
            else:
                print(write_sbom(asset, args.version, args.out, args.syft, args.go))
    except (SBOMError, OSError) as err:
        print(f"release-sbom: {err}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())

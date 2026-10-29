#!/usr/bin/env python3
"""Tests for scripts/release-sbom.py. Run: python3 scripts/release-sbom-test.py"""

from __future__ import annotations

import gzip
import hashlib
import importlib.util
import io
import json
import pathlib
import tarfile
import tempfile
import unittest
import zipfile

HERE = pathlib.Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("release_sbom", HERE / "release-sbom.py")
sbom = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(sbom)

GO_VERSION_M = """dist/sysml-linux-amd64: go1.25.11
\tpath\tgithub.com/Open-MBEE/OpenSysML/cmd/sysml
\tmod\tgithub.com/Open-MBEE/OpenSysML\tv0.12.0\th1:aaaa
\tdep\tgithub.com/chzyer/readline\tv1.5.1\th1:bbbb
\tdep\tgolang.org/x/net\tv0.23.0
\t=>\tgithub.com/example/net-fork\tv0.23.1\th1:cccc
\tbuild\t-buildmode=exe
\tbuild\tCGO_ENABLED=0
"""


def package(name, version, purl, spdxid=None):
    return {
        "name": name,
        "SPDXID": spdxid or f"SPDXRef-Package-{name.replace('/', '-')}",
        "versionInfo": version,
        "externalRefs": [{"referenceCategory": "PACKAGE-MANAGER", "referenceType": "purl",
                          "referenceLocator": purl}],
    }


def document(*packages):
    root = {"name": "sysml-linux-amd64.tar.gz", "SPDXID": "SPDXRef-DocumentRoot-File-sysml-linux-amd64.tar.gz"}
    return {
        "spdxVersion": "SPDX-2.3",
        "name": "sysml-linux-amd64.tar.gz",
        "documentNamespace": "https://anchore.com/syft/file/sysml-linux-amd64.tar.gz-0b4301f7-17a6-4369-aade-6ccf72b617c3",
        "packages": [root, *packages],
    }


def built_from(info):
    return [package(path, version, f"pkg:golang/{path}@{version}") for path, version in info.modules] + [
        package("stdlib", info.toolchain, f"pkg:golang/stdlib@{info.toolchain[2:]}")
    ]


class ParseBuildInfo(unittest.TestCase):
    def test_reads_toolchain_main_module_and_dependencies(self):
        info = sbom.parse_build_info(GO_VERSION_M)
        self.assertEqual(info.toolchain, "go1.25.11")
        self.assertEqual(
            info.modules,
            (
                ("github.com/Open-MBEE/OpenSysML", "v0.12.0"),
                ("github.com/chzyer/readline", "v1.5.1"),
                ("github.com/example/net-fork", "v0.23.1"),
            ),
        )

    def test_a_replacement_stands_in_for_the_module_it_replaces(self):
        info = sbom.parse_build_info(GO_VERSION_M)
        self.assertNotIn(("golang.org/x/net", "v0.23.0"), info.modules)

    def test_purls_cover_every_module_and_the_toolchain(self):
        info = sbom.parse_build_info(GO_VERSION_M)
        self.assertEqual(
            info.purls,
            {
                "pkg:golang/github.com/Open-MBEE/OpenSysML@v0.12.0",
                "pkg:golang/github.com/chzyer/readline@v1.5.1",
                "pkg:golang/github.com/example/net-fork@v0.23.1",
                "pkg:golang/stdlib@1.25.11",
            },
        )

    def test_refuses_text_that_is_not_go_version_output(self):
        with self.assertRaises(sbom.SBOMError):
            sbom.parse_build_info("sysml: not executable file\n")


class Check(unittest.TestCase):
    def setUp(self):
        self.info = sbom.parse_build_info(GO_VERSION_M)
        self.doc = document(*built_from(self.info))
        self.infos = [("sysml-linux-amd64", self.info)]

    def test_accepts_an_sbom_naming_every_module_built_in(self):
        sbom.check(self.doc, "sysml-linux-amd64.tar.gz", self.infos, ["/home/circleci/project/dist"])

    def test_refuses_an_sbom_missing_a_module(self):
        self.doc["packages"] = [p for p in self.doc["packages"] if p["name"] != "github.com/chzyer/readline"]
        with self.assertRaises(sbom.SBOMError) as caught:
            sbom.check(self.doc, "sysml-linux-amd64.tar.gz", self.infos, [])
        self.assertIn("pkg:golang/github.com/chzyer/readline@v1.5.1", str(caught.exception))

    def test_refuses_an_sbom_naming_another_version_of_a_module(self):
        for p in self.doc["packages"]:
            if p["name"] == "github.com/chzyer/readline":
                p["externalRefs"][0]["referenceLocator"] = "pkg:golang/github.com/chzyer/readline@v1.5.0"
        with self.assertRaises(sbom.SBOMError):
            sbom.check(self.doc, "sysml-linux-amd64.tar.gz", self.infos, [])

    def test_refuses_an_sbom_whose_toolchain_differs(self):
        for p in self.doc["packages"]:
            if p["name"] == "stdlib":
                p["externalRefs"][0]["referenceLocator"] = "pkg:golang/stdlib@1.24.0"
        with self.assertRaises(sbom.SBOMError) as caught:
            sbom.check(self.doc, "sysml-linux-amd64.tar.gz", self.infos, [])
        self.assertIn("pkg:golang/stdlib@1.25.11", str(caught.exception))

    def test_refuses_an_sbom_naming_no_component(self):
        doc = document()
        with self.assertRaises(sbom.SBOMError) as caught:
            sbom.check(doc, "sysml-wasm.wasm", [], [])
        self.assertIn("names no component", str(caught.exception))

    def test_refuses_an_sbom_mentioning_the_builders_path(self):
        self.doc["packages"][1]["sourceInfo"] = "acquired from /home/circleci/project/dist/sysml-linux-amd64"
        with self.assertRaises(sbom.SBOMError) as caught:
            sbom.check(self.doc, "sysml-linux-amd64.tar.gz", self.infos, ["/home/circleci/project/dist"])
        self.assertIn("/home/circleci/project/dist", str(caught.exception))

    def test_reads_a_percent_encoded_version_as_go_version_prints_it(self):
        info = sbom.parse_build_info(GO_VERSION_M.replace("v0.12.0", "v0.12.1-0.20261027144936-82cd1c2071dd+dirty"))
        doc = document(*built_from(info))
        for p in doc["packages"]:
            if p["name"] == "github.com/Open-MBEE/OpenSysML":
                p["externalRefs"][0]["referenceLocator"] = (
                    "pkg:golang/github.com/Open-MBEE/OpenSysML@v0.12.1-0.20261027144936-82cd1c2071dd%2Bdirty"
                )
        sbom.check(doc, "sysml-linux-amd64.tar.gz", [("sysml-linux-amd64", info)], [])

    def test_an_asset_without_go_executables_needs_only_a_component(self):
        doc = document(package("opensysml", "0.12.0", "pkg:pypi/opensysml@0.12.0"))
        sbom.check(doc, "opensysml-0.12.0-py3-none-any.whl", [], [])


class Finalize(unittest.TestCase):
    def test_namespace_is_the_sboms_release_url(self):
        doc = sbom.finalize(document(), "sysml-linux-amd64.tar.gz", "v0.12.0")
        self.assertEqual(
            doc["documentNamespace"],
            "https://github.com/Open-MBEE/OpenSysML/releases/download/v0.12.0/sysml-linux-amd64.tar.gz.spdx.json",
        )

    def test_two_runs_differ_only_in_the_namespace_syft_drew(self):
        first, second = document(), document()
        second["documentNamespace"] = "https://anchore.com/syft/file/sysml-linux-amd64.tar.gz-ffffffff-0000-0000-0000-000000000000"
        self.assertEqual(
            sbom.finalize(first, "sysml-linux-amd64.tar.gz", "v0.12.0"),
            sbom.finalize(second, "sysml-linux-amd64.tar.gz", "v0.12.0"),
        )


class Unpacked(unittest.TestCase):
    def test_a_plain_file_is_itself(self):
        with tempfile.TemporaryDirectory() as tmp:
            asset = pathlib.Path(tmp) / "sysml-grpc-linux-amd64"
            asset.write_bytes(b"\x7fELF")
            self.assertEqual(sbom.unpacked(asset, pathlib.Path(tmp) / "u"), [("sysml-grpc-linux-amd64", asset)])

    def test_tarball_members_are_written_flat_under_the_directory(self):
        with tempfile.TemporaryDirectory() as tmp:
            asset = pathlib.Path(tmp) / "opensysml-linux-amd64.tar.gz"
            with tarfile.open(asset, "w:gz") as archive:
                for name, data in (("sysml", b"one"), ("sysml-lsp", b"two"), ("../escape", b"no")):
                    member = tarfile.TarInfo(name)
                    member.size = len(data)
                    archive.addfile(member, io.BytesIO(data))
                directory = tarfile.TarInfo("dir")
                directory.type = tarfile.DIRTYPE
                archive.addfile(directory)
            into = pathlib.Path(tmp) / "u"
            members = sbom.unpacked(asset, into)
            self.assertEqual([m for m, _ in members], ["sysml", "sysml-lsp", "../escape"])
            self.assertEqual([p.read_bytes() for _, p in members], [b"one", b"two", b"no"])
            for _, path in members:
                self.assertEqual(path.parent.parent, into)

    def test_zip_and_wheel_members_are_unpacked_too(self):
        with tempfile.TemporaryDirectory() as tmp:
            asset = pathlib.Path(tmp) / "jupyter_opensysml_kernel-0.12.0-py3-none-manylinux_2_17_x86_64.whl"
            with zipfile.ZipFile(asset, "w") as archive:
                archive.writestr("jupyter_opensysml_kernel/bin/sysml-jupyter-kernel", b"kernel")
                archive.writestr("jupyter_opensysml_kernel-0.12.0.dist-info/METADATA", b"Name: x")
            members = sbom.unpacked(asset, pathlib.Path(tmp) / "u")
            self.assertEqual(
                [(m, p.read_bytes()) for m, p in members],
                [("jupyter_opensysml_kernel/bin/sysml-jupyter-kernel", b"kernel"),
                 ("jupyter_opensysml_kernel-0.12.0.dist-info/METADATA", b"Name: x")],
            )


class ScanTarget(unittest.TestCase):
    def test_a_wheel_is_scanned_through_a_zip_copy_under_its_own_name(self):
        with tempfile.TemporaryDirectory() as tmp:
            asset = pathlib.Path(tmp) / "opensysml-0.12.0-py3-none-any.whl"
            asset.write_bytes(b"PK")
            target = sbom.scan_target(asset, tmp)
            self.assertEqual(target.name, "opensysml-0.12.0-py3-none-any.whl.zip")
            self.assertEqual(target.read_bytes(), b"PK")

    def test_anything_else_is_scanned_in_place(self):
        with tempfile.TemporaryDirectory() as tmp:
            asset = pathlib.Path(tmp) / "sysml-linux-amd64.tar.gz"
            self.assertEqual(sbom.scan_target(asset, tmp), asset)


MODINFO = (
    "path\tgithub.com/Open-MBEE/OpenSysML/cmd/sysml-wasm\n"
    "mod\tgithub.com/Open-MBEE/OpenSysML\tv0.12.0\t\n"
    "dep\tgithub.com/yuin/goldmark\tv1.8.6\th1:dddd\n"
    "dep\tgolang.org/x/text\tv0.37.0\th1:eeee\n"
    "build\t-buildmode=exe\n"
    "build\tGOARCH=wasm\n"
    "build\tGOOS=js\n"
)


def wasm_bytes(modinfo=MODINFO, toolchains=("go1.25.11",)):
    """A stand-in wasm module: the sentinel-wrapped module information and the runtime's version string."""
    body = b"\x00asm\x01\x00\x00\x00" + b"\x00" * 64
    body += sbom.MODINFO_START + modinfo.encode() + sbom.MODINFO_END + b"\x00" * 16
    for toolchain in toolchains:
        body += toolchain.encode() + b"\x00" * 8
    return body


class WasmBuildInfo(unittest.TestCase):
    def test_reads_the_embedded_module_information_and_the_toolchain(self):
        info = sbom.wasm_build_info(wasm_bytes(), "sysml-wasm.wasm")
        self.assertEqual(info.toolchain, "go1.25.11")
        self.assertEqual(
            info.modules,
            (
                ("github.com/Open-MBEE/OpenSysML", "v0.12.0"),
                ("github.com/yuin/goldmark", "v1.8.6"),
                ("golang.org/x/text", "v0.37.0"),
            ),
        )
        self.assertIn("pkg:golang/stdlib@1.25.11", info.purls)

    def test_refuses_a_module_without_module_information(self):
        with self.assertRaises(sbom.SBOMError) as caught:
            sbom.wasm_build_info(b"\x00asm\x01\x00\x00\x00go1.25.11", "sysml-wasm.wasm")
        self.assertIn("no Go module information", str(caught.exception))

    def test_refuses_a_module_naming_two_toolchains(self):
        data = wasm_bytes(toolchains=("go1.25.11", "go1.24.3"))
        with self.assertRaises(sbom.SBOMError) as caught:
            sbom.wasm_build_info(data, "sysml-wasm.wasm")
        self.assertIn("2 Go toolchains", str(caught.exception))

    def test_a_short_version_string_is_not_a_toolchain(self):
        info = sbom.wasm_build_info(wasm_bytes(toolchains=("go1.25.11", "go1.21")), "sysml-wasm.wasm")
        self.assertEqual(info.toolchain, "go1.25.11")


class WasmDocument(unittest.TestCase):
    def write(self, tmp, name, data):
        path = pathlib.Path(tmp) / name
        path.write_bytes(data)
        return path

    def test_names_every_module_and_the_toolchain_as_syft_would(self):
        with tempfile.TemporaryDirectory() as tmp:
            doc, info = sbom.wasm_document(self.write(tmp, "sysml-wasm.wasm", wasm_bytes()), "v0.12.0")
        self.assertEqual(doc["name"], "sysml-wasm.wasm")
        self.assertEqual(sbom.purls(doc), info.purls)
        stdlib = next(p for p in doc["packages"] if p["name"] == "stdlib")
        self.assertEqual(stdlib["versionInfo"], "go1.25.11")
        self.assertEqual(stdlib["licenseDeclared"], "BSD-3-Clause")
        self.assertEqual(len(sbom.components(doc)), 4)
        sbom.check(doc, "sysml-wasm.wasm", [("sysml-wasm.wasm", info)], [tmp])
        described = [r["relatedSpdxElement"] for r in doc["relationships"] if r["relationshipType"] == "DESCRIBES"]
        self.assertEqual(described, [doc["packages"][0]["SPDXID"]])
        contained = {r["relatedSpdxElement"] for r in doc["relationships"] if r["relationshipType"] == "CONTAINS"}
        self.assertEqual(contained, {p["SPDXID"] for p in sbom.components(doc)})

    def test_a_gzip_compressed_module_is_read_through_the_compression(self):
        with tempfile.TemporaryDirectory() as tmp:
            doc, info = sbom.wasm_document(
                self.write(tmp, "sysml-prod.wasm.gz", gzip.compress(wasm_bytes())), "v0.12.0"
            )
        self.assertEqual(doc["name"], "sysml-prod.wasm.gz")
        self.assertEqual(info.toolchain, "go1.25.11")

    def test_a_plus_in_a_version_is_percent_encoded_in_the_purl(self):
        dirty = MODINFO.replace("v0.12.0", "v0.12.1-0.20261027144936-82cd1c2071dd+dirty")
        with tempfile.TemporaryDirectory() as tmp:
            doc, info = sbom.wasm_document(self.write(tmp, "sysml-wasm.wasm", wasm_bytes(dirty)), "v0.12.0")
        locators = {r["referenceLocator"] for p in doc["packages"] for r in p.get("externalRefs", [])}
        self.assertIn("pkg:golang/github.com/Open-MBEE/OpenSysML@v0.12.1-0.20261027144936-82cd1c2071dd%2Bdirty", locators)
        sbom.check(doc, "sysml-wasm.wasm", [("sysml-wasm.wasm", info)], [])

    def test_two_runs_differ_only_in_the_creation_time(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = self.write(tmp, "sysml-wasm.wasm", wasm_bytes())
            first, _ = sbom.wasm_document(path, "v0.12.0")
            second, _ = sbom.wasm_document(path, "v0.12.0")
        first["creationInfo"]["created"] = second["creationInfo"]["created"] = "T"
        self.assertEqual(first, second)
        self.assertEqual(
            sbom.finalize(first, "sysml-wasm.wasm", "v0.12.0")["documentNamespace"],
            "https://github.com/Open-MBEE/OpenSysML/releases/download/v0.12.0/sysml-wasm.wasm.spdx.json",
        )


CLIENT_MANIFEST = {
    "name": "@openmbee/opensysml",
    "version": "0.12.0",
    "license": "Apache-2.0",
    "dependencies": {"@bufbuild/protobuf": "2.14.0", "@connectrpc/connect": "2.1.2"},
    "optionalDependencies": {"@openmbee/opensysml-sysml-grpc-linux-x64": "0.12.0", "@sigstore/tuf": "4.0.2"},
    "peerDependencies": {"@openmbee/opensysml-wasm": "0.12.0"},
    "devDependencies": {"typescript": "5.7.3"},
}


def npm_tarball(tmp, manifest):
    path = pathlib.Path(tmp) / sbom.npm_tarball_name(manifest)
    with tarfile.open(path, "w:gz") as archive:
        data = json.dumps(manifest).encode()
        member = tarfile.TarInfo("package/package.json")
        member.size = len(data)
        archive.addfile(member, io.BytesIO(data))
    return path


def npm_document(root, *packages):
    """What syft writes over a lockfile under `--source-name <tarball>`: the lock's root beside its dependencies."""
    doc = document(*packages)
    doc["name"] = "openmbee-opensysml-0.12.0.tgz"
    doc["packages"][0] = {"name": doc["name"], "SPDXID": "SPDXRef-DocumentRoot-File-openmbee-opensysml-0.12.0.tgz"}
    doc["relationships"] = [
        {"spdxElementId": "SPDXRef-DOCUMENT", "relatedSpdxElement": doc["packages"][0]["SPDXID"], "relationshipType": "DESCRIBES"},
        *({"spdxElementId": p["SPDXID"], "relatedSpdxElement": root["SPDXID"], "relationshipType": "DEPENDENCY_OF"} for p in packages),
    ]
    doc["packages"].append(root)
    return doc


def npm(name, version):
    return package(name, version, f"pkg:npm/{name.replace('@', '%40')}@{version}")


class NpmSBOM(unittest.TestCase):
    def setUp(self):
        self.root = npm("@openmbee/opensysml", "0.12.0")
        self.deps = [
            npm("@bufbuild/protobuf", "2.14.0"),
            npm("@connectrpc/connect", "2.1.2"),
            npm("@openmbee/opensysml-sysml-grpc-linux-x64", "0.12.0"),
            npm("@sigstore/tuf", "4.0.2"),
            npm("make-fetch-happen", "14.0.3"),
        ]

    def test_the_tarball_is_named_as_npm_pack_names_the_manifest(self):
        self.assertEqual(sbom.npm_tarball_name(CLIENT_MANIFEST), "openmbee-opensysml-0.12.0.tgz")
        self.assertEqual(sbom.npm_tarball_name({"name": "@openmbee/sysml-mcp", "version": "0.12.0-rc.1"}), "openmbee-sysml-mcp-0.12.0-rc.1.tgz")

    def test_reads_the_packed_manifest(self):
        with tempfile.TemporaryDirectory() as tmp:
            self.assertEqual(sbom.npm_manifest(npm_tarball(tmp, CLIENT_MANIFEST)), CLIENT_MANIFEST)
            empty = pathlib.Path(tmp) / "empty.tgz"
            with tarfile.open(empty, "w:gz"):
                pass
            with self.assertRaises(sbom.SBOMError):
                sbom.npm_manifest(empty)

    def test_the_installed_dependencies_are_required_and_peers_are_not(self):
        self.assertEqual(
            sbom.npm_dependencies(CLIENT_MANIFEST),
            {"@bufbuild/protobuf": "2.14.0", "@connectrpc/connect": "2.1.2",
             "@openmbee/opensysml-sysml-grpc-linux-x64": "0.12.0", "@sigstore/tuf": "4.0.2"},
        )

    def test_the_packed_manifest_replaces_the_lockfiles_root_and_keeps_its_dependencies(self):
        doc = npm_document(self.root, *self.deps)
        with tempfile.TemporaryDirectory() as tmp:
            tarball = npm_tarball(tmp, CLIENT_MANIFEST)
            digest = hashlib.sha256(tarball.read_bytes()).hexdigest()
            sbom.describe_npm(doc, CLIENT_MANIFEST, tarball, {"name": "@openmbee/opensysml", "version": "0.12.0"})
        names = [p["name"] for p in sbom.components(doc)]
        self.assertEqual(names.count("@openmbee/opensysml"), 1)
        root = next(p for p in sbom.components(doc) if p["name"] == "@openmbee/opensysml")
        self.assertNotEqual(root["SPDXID"], self.root["SPDXID"])
        self.assertEqual(root["versionInfo"], "0.12.0")
        self.assertEqual(root["checksums"], [{"algorithm": "SHA256", "checksumValue": digest}])
        self.assertEqual(root["downloadLocation"], "https://registry.npmjs.org/@openmbee/opensysml/-/opensysml-0.12.0.tgz")
        self.assertEqual(root["licenseDeclared"], "Apache-2.0")
        self.assertIn("pkg:npm/@openmbee/opensysml@0.12.0", sbom.purls(doc))
        self.assertEqual(root["externalRefs"][0]["referenceLocator"], "pkg:npm/%40openmbee/opensysml@0.12.0")
        dependents = {r["relatedSpdxElement"] for r in doc["relationships"] if r["relationshipType"] == "DEPENDENCY_OF"}
        self.assertEqual(dependents, {root["SPDXID"]})
        self.assertIn(
            {"spdxElementId": "SPDXRef-DocumentRoot-File-openmbee-opensysml-0.12.0.tgz", "relatedSpdxElement": root["SPDXID"], "relationshipType": "CONTAINS"},
            doc["relationships"],
        )
        sbom.check_npm(doc, "openmbee-opensysml-0.12.0.tgz", CLIENT_MANIFEST)

    def test_an_alias_is_described_from_the_runners_lockfile_under_its_own_manifest(self):
        runner = npm("@openmbee/sysml-mcp", "0.12.0")
        platforms = [npm(f"@openmbee/sysml-mcp-{p}", "0.12.0") for p in ("linux-x64", "win32-x64")]
        alias = {"name": "@openmbee/opensysml-mcp", "version": "0.12.0", "license": "Apache-2.0",
                 "optionalDependencies": {f"@openmbee/sysml-mcp-{p}": "0.12.0" for p in ("linux-x64", "win32-x64")}}
        doc = npm_document(runner, *platforms)
        with tempfile.TemporaryDirectory() as tmp:
            sbom.describe_npm(doc, alias, npm_tarball(tmp, alias), {"name": "@openmbee/sysml-mcp", "version": "0.12.0"})
        self.assertEqual(sorted(p["name"] for p in sbom.components(doc)),
                         ["@openmbee/opensysml-mcp", "@openmbee/sysml-mcp-linux-x64", "@openmbee/sysml-mcp-win32-x64"])
        sbom.check_npm(doc, "openmbee-opensysml-mcp-0.12.0.tgz", alias)

    def test_an_optional_platform_package_not_yet_published_is_declared_from_the_manifest(self):
        # At a release tag the lockfile cannot resolve the platform packages the same
        # release publishes, so the client's exact pin stands in, saying so.
        unresolved = [d for d in self.deps if d["name"] != "@openmbee/opensysml-sysml-grpc-linux-x64"]
        doc = npm_document(self.root, *unresolved)
        with tempfile.TemporaryDirectory() as tmp:
            sbom.describe_npm(doc, CLIENT_MANIFEST, npm_tarball(tmp, CLIENT_MANIFEST), {"name": "@openmbee/opensysml", "version": "0.12.0"})
        declared = next(p for p in sbom.components(doc) if p["name"] == "@openmbee/opensysml-sysml-grpc-linux-x64")
        self.assertEqual(declared["versionInfo"], "0.12.0")
        self.assertIn("not yet resolvable through the lockfile", declared["sourceInfo"])
        self.assertNotIn("checksums", declared)
        self.assertIn("pkg:npm/@openmbee/opensysml-sysml-grpc-linux-x64@0.12.0", sbom.purls(doc))
        root = next(p for p in sbom.components(doc) if p["name"] == "@openmbee/opensysml")
        self.assertIn({"spdxElementId": declared["SPDXID"], "relatedSpdxElement": root["SPDXID"], "relationshipType": "DEPENDENCY_OF"}, doc["relationships"])
        sbom.check_npm(doc, "openmbee-opensysml-0.12.0.tgz", CLIENT_MANIFEST)

    def test_a_third_party_optional_dependency_the_lockfile_lacks_is_still_refused(self):
        doc = npm_document(self.root, *[d for d in self.deps if d["name"] != "@sigstore/tuf"])
        with tempfile.TemporaryDirectory() as tmp:
            sbom.describe_npm(doc, CLIENT_MANIFEST, npm_tarball(tmp, CLIENT_MANIFEST), {"name": "@openmbee/opensysml", "version": "0.12.0"})
        self.assertNotIn("@sigstore/tuf", {p["name"] for p in sbom.components(doc)})
        with self.assertRaises(sbom.SBOMError) as caught:
            sbom.check_npm(doc, "openmbee-opensysml-0.12.0.tgz", CLIENT_MANIFEST)
        self.assertIn("@sigstore/tuf@4.0.2", str(caught.exception))

    def test_only_our_packages_pinned_at_the_release_version_count_as_platform_packages(self):
        manifest = dict(CLIENT_MANIFEST, optionalDependencies={
            "@openmbee/opensysml-sysml-grpc-linux-x64": "0.12.0", "@openmbee/opensysml-sysml-grpc-win32-x64": "0.11.0", "@sigstore/tuf": "4.0.2"})
        self.assertEqual(sbom.release_platform_packages(manifest), {"@openmbee/opensysml-sysml-grpc-linux-x64": "0.12.0"})

    def test_a_required_dependency_the_lockfile_lacks_is_still_refused(self):
        doc = npm_document(self.root, *[d for d in self.deps if d["name"] != "@bufbuild/protobuf"])
        with tempfile.TemporaryDirectory() as tmp:
            sbom.describe_npm(doc, CLIENT_MANIFEST, npm_tarball(tmp, CLIENT_MANIFEST), {"name": "@openmbee/opensysml", "version": "0.12.0"})
        with self.assertRaises(sbom.SBOMError) as caught:
            sbom.check_npm(doc, "openmbee-opensysml-0.12.0.tgz", CLIENT_MANIFEST)
        self.assertIn("@bufbuild/protobuf@2.14.0", str(caught.exception))

    def test_refuses_an_sbom_lacking_a_declared_dependency(self):
        doc = npm_document(self.root, *[d for d in self.deps if d["name"] != "@sigstore/tuf"])
        with self.assertRaises(sbom.SBOMError) as caught:
            sbom.check_npm(doc, "openmbee-opensysml-0.12.0.tgz", CLIENT_MANIFEST)
        self.assertIn("@sigstore/tuf@4.0.2", str(caught.exception))

    def test_refuses_an_sbom_resolving_an_exact_pin_to_another_version(self):
        deps = [npm("@connectrpc/connect", "2.1.1") if d["name"] == "@connectrpc/connect" else d for d in self.deps]
        doc = npm_document(self.root, *deps)
        with self.assertRaises(sbom.SBOMError) as caught:
            sbom.check_npm(doc, "openmbee-opensysml-0.12.0.tgz", CLIENT_MANIFEST)
        self.assertIn("pins @connectrpc/connect@2.1.2", str(caught.exception))

    def test_a_range_is_satisfied_by_any_resolved_version(self):
        manifest = dict(CLIENT_MANIFEST, dependencies={"@bufbuild/protobuf": "^2.14.0"})
        sbom.check_npm(npm_document(self.root, *self.deps), "openmbee-opensysml-0.12.0.tgz", manifest)

    def test_refuses_a_tarball_named_for_another_version_than_it_packs(self):
        with tempfile.TemporaryDirectory() as tmp:
            tarball = npm_tarball(tmp, CLIENT_MANIFEST).rename(pathlib.Path(tmp) / "openmbee-opensysml-0.11.0.tgz")
            with self.assertRaises(sbom.SBOMError) as caught:
                sbom.write_npm_sbom(tarball, pathlib.Path(tmp) / "package-lock.json", "v0.12.0", tmp, "syft")
            self.assertIn("packed as @openmbee/opensysml@0.12.0", str(caught.exception))


Z3_PIN = """# The Z3 release bundled by the optional "SMT solver (Z3)" MSI feature.
Z3_VERSION=5.1.0
Z3_ZIP=z3-5.1.0-x64-win.zip
Z3_SHA256={digest}
"""


def z3_zip(tmp, members=("bin/z3.exe", "bin/msvcp140.dll", "bin/vcruntime140.dll", "bin/vcruntime140_1.dll", "LICENSE.txt")):
    path = pathlib.Path(tmp) / "z3-5.1.0-x64-win.zip"
    with zipfile.ZipFile(path, "w") as archive:
        for member in members:
            archive.writestr(f"z3-5.1.0-x64-win/{member}", member.encode())
        archive.writestr("z3-5.1.0-x64-win/bin/vcomp140.dll", b"not bundled")
    pin = pathlib.Path(tmp) / "z3.pin"
    pin.write_text(Z3_PIN.format(digest=hashlib.sha256(path.read_bytes()).hexdigest()))
    return path, pin


class MSISBOM(unittest.TestCase):
    def test_the_installer_name_follows_the_tag_as_build_msi_derives_the_product_version(self):
        self.assertEqual(sbom.msi_name("v0.12.0", signed=False), "opensysml-0.12.0-windows-amd64.msi")
        self.assertEqual(sbom.msi_name("v0.12.0", signed=True), "opensysml-0.12.0-windows-amd64-signed.msi")
        self.assertEqual(sbom.msi_name("v0.12.0-rc1", signed=False), "opensysml-0.12.0-windows-amd64.msi")
        self.assertEqual(sbom.msi_name("v0.12.0+build", signed=False), "opensysml-0.12.0-windows-amd64.msi")
        self.assertEqual(sbom.msi_name("v0.0.0-dry-run", signed=False), "opensysml-0.0.0-windows-amd64.msi")
        for tag in ("0.12.0", "v0.12"):
            with self.subTest(tag=tag), self.assertRaises(sbom.SBOMError):
                sbom.msi_name(tag, signed=False)

    def test_reads_the_pin_build_msi_sources(self):
        with tempfile.TemporaryDirectory() as tmp:
            _, pin = z3_zip(tmp)
            read = sbom.read_pin(pin)
        self.assertEqual((read["Z3_VERSION"], read["Z3_ZIP"]), ("5.1.0", "z3-5.1.0-x64-win.zip"))
        self.assertEqual(len(read["Z3_SHA256"]), 64)
        with tempfile.TemporaryDirectory() as tmp:
            short = pathlib.Path(tmp) / "z3.pin"
            short.write_text("Z3_VERSION=5.1.0\n")
            with self.assertRaises(sbom.SBOMError) as caught:
                sbom.read_pin(short)
            self.assertIn("Z3_ZIP", str(caught.exception))

    def test_declares_z3_with_the_pins_digest_and_the_files_the_installer_takes(self):
        with tempfile.TemporaryDirectory() as tmp:
            zip_path, pin = z3_zip(tmp)
            pinned = sbom.read_pin(pin)
            package, files = sbom.z3_package(pinned, zip_path)
        self.assertEqual(package["name"], "z3")
        self.assertEqual(package["versionInfo"], "5.1.0")
        self.assertEqual(package["licenseDeclared"], "MIT")
        self.assertEqual(package["downloadLocation"], "https://github.com/Z3Prover/z3/releases/download/z3-5.1.0/z3-5.1.0-x64-win.zip")
        self.assertEqual(package["checksums"], [{"algorithm": "SHA256", "checksumValue": pinned["Z3_SHA256"]}])
        self.assertEqual(package["externalRefs"][0]["referenceLocator"], "pkg:github/z3prover/z3@z3-5.1.0")
        self.assertEqual(
            [f["fileName"] for f in files],
            ["./z3/z3.exe", "./z3/msvcp140.dll", "./z3/vcruntime140.dll", "./z3/vcruntime140_1.dll", "./z3/LICENSE-z3.txt"],
        )
        self.assertEqual(files[0]["checksums"], [{"algorithm": "SHA256", "checksumValue": hashlib.sha256(b"bin/z3.exe").hexdigest()}])

    def test_refuses_a_zip_whose_digest_is_not_the_pins(self):
        with tempfile.TemporaryDirectory() as tmp:
            zip_path, pin = z3_zip(tmp)
            pinned = dict(sbom.read_pin(pin), Z3_SHA256="0" * 64)
            with self.assertRaises(sbom.SBOMError) as caught:
                sbom.z3_package(pinned, zip_path)
            self.assertIn("the pin expects", str(caught.exception))

    def test_refuses_a_zip_missing_a_file_the_installer_takes(self):
        with tempfile.TemporaryDirectory() as tmp:
            zip_path, pin = z3_zip(tmp, members=("bin/z3.exe", "LICENSE.txt"))
            pinned = sbom.read_pin(pin)
            with self.assertRaises(sbom.SBOMError) as caught:
                sbom.z3_package(pinned, zip_path)
            self.assertIn("bin/msvcp140.dll", str(caught.exception))

    def test_an_installer_sbom_passes_the_binary_check_with_z3_declared_beside_the_modules(self):
        info = sbom.parse_build_info(GO_VERSION_M.replace("dist/sysml-linux-amd64", "sysml-windows-amd64.exe"))
        with tempfile.TemporaryDirectory() as tmp:
            zip_path, pin = z3_zip(tmp)
            package, files = sbom.z3_package(sbom.read_pin(pin), zip_path)
        doc = document(*built_from(info), package)
        doc["files"] = files
        sbom.check(doc, "opensysml-0.12.0-windows-amd64.msi", [("sysml-windows-amd64.exe", info)], ["/home/runner/work"])
        self.assertIn("pkg:github/z3prover/z3@z3-5.1.0", sbom.purls(doc))


class Describe(unittest.TestCase):
    def test_counts_components_and_names_the_toolchain(self):
        info = sbom.parse_build_info(GO_VERSION_M)
        doc = document(*built_from(info))
        self.assertEqual(
            sbom.describe(doc, "sysml-linux-amd64.tar.gz", [("sysml-linux-amd64", info)]),
            "sysml-linux-amd64.tar.gz.spdx.json: 4 components, Go go1.25.11",
        )


if __name__ == "__main__":
    unittest.main()

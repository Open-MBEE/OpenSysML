#!/usr/bin/env python3
"""Tests for scripts/release-sbom.py. Run: python3 scripts/release-sbom-test.py"""

from __future__ import annotations

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
        with self.assertRaises(sbom.SBOMError) as caught:
            sbom.check(document(), "sysml-wasm.wasm", [], [])
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

"""Tests for scripts/check_version.py, the release gate the release workflow runs."""

import importlib.util
import pathlib
import re
import xml.etree.ElementTree as ET

import pytest
from packaging.version import Version

SCRIPT = pathlib.Path(__file__).resolve().parents[1] / "scripts" / "check_version.py"
spec = importlib.util.spec_from_file_location("check_version", SCRIPT)
check_version = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(check_version)


def _core_tag(version):
    """The SemVer core tag naming a PEP 440 version: 0.5.0rc1 is tagged v0.5.0-rc1."""
    parsed = Version(version)
    tag = f"v{parsed.base_version}"
    if parsed.pre is not None:
        phase, number = parsed.pre
        tag += f"-{ {'a': 'alpha', 'b': 'beta', 'rc': 'rc'}[phase] }.{number}"
    return tag


def test_declared_version_reads_the_shipped_version_file():
    from opensysml import __version__

    assert check_version.declared_version() == __version__


def test_declared_version_rejects_a_file_without_a_version(tmp_path):
    version_file = tmp_path / "_version.py"
    version_file.write_text("OTHER = '1.0'\n", encoding="utf-8")
    with pytest.raises(check_version.VersionError, match="declares no VERSION"):
        check_version.declared_version(str(version_file))


def test_version_from_tag_accepts_the_declared_version():
    assert check_version.version_from_tag("v0.4.0", version="0.4.0") == "0.4.0"


@pytest.mark.parametrize(
    "tag, declared",
    [
        ("v0.4.0-rc1", "0.4.0rc1"),
        ("v0.4.0-rc.1", "0.4.0rc1"),
        ("v0.4.0-alpha.2", "0.4.0a2"),
        ("v0.4.0-beta1", "0.4.0b1"),
        ("v0.4.0-rc.10", "0.4.0rc10"),
        ("v10.0.0-rc0", "10.0.0rc0"),
    ],
)
def test_version_from_tag_translates_a_semver_pre_release_to_pep_440(tag, declared):
    """A core pre-release tag is SemVer; the package's version is its PEP 440 form."""
    assert check_version.version_from_tag(tag, version=declared) == declared
    assert check_version.is_pre_release(declared)


@pytest.mark.parametrize(
    "tag",
    [
        "v0.4.0-1",  # PEP 440 would read a post-release, SemVer a pre-release
        "v0.4.0-rc",
        "v0.4.0-pre.1",
        "v0.4.0-rc1+build.5",
        "v0.4.0rc1",
        "v0.4.0.post1",
        "v0.4",
        "v0.4.0.1",
        "v0.4.0-not-a-version",
        "v0.4.0-rc.010",  # leading zero: not a SemVer numeric identifier
        "v0.4.0-rc010",
        "v0.04.0",
        "v0.4.0-rc\u0661",  # Arabic-Indic one: a digit to \\d, not to SemVer
        "v\u0660.4.0",
    ],
)
def test_version_from_tag_rejects_a_suffix_without_one_pep_440_meaning(tag):
    with pytest.raises(check_version.VersionError, match="is not a core release tag of the form"):
        check_version.version_from_tag(tag, version="0.4.0.post1")


@pytest.mark.parametrize(
    "tag, message",
    [
        ("", "No tag given"),
        ("opensysml-v0.4.0", "does not start with 'v'"),
        ("0.4.0", "does not start with 'v'"),
        ("v0.4.1", "names version '0.4.1', but"),
        ("v0.4.0-rc1", "names version '0.4.0rc1', but"),
    ],
)
def test_version_from_tag_rejects_a_tag_that_names_another_version(tag, message):
    with pytest.raises(check_version.VersionError, match=message):
        check_version.version_from_tag(tag, version="0.4.0")


@pytest.mark.parametrize("declared", ["0.4.0-rc1", "0.4.0.RC1", "v0.4.0"])
def test_version_from_tag_requires_the_declared_version_to_be_canonical(declared):
    """The artifacts are named by the canonical form, so the declaration must be it."""
    with pytest.raises(check_version.VersionError, match="canonical PEP 440 form"):
        check_version.version_from_tag("v0.4.0-rc1", version=declared)


def test_version_from_tag_rejects_a_declared_non_version():
    with pytest.raises(check_version.VersionError, match="not a PEP 440 version"):
        check_version.version_from_tag("v0.4.0", version="latest")


@pytest.mark.parametrize(
    "version, pre_release",
    [("0.4.0", False), ("0.4.0rc1", True), ("1.0.0a2", True), ("1.0.0.post1", False)],
)
def test_is_pre_release_follows_pep_440(version, pre_release):
    assert check_version.is_pre_release(version) is pre_release


def test_is_pre_release_rejects_a_non_pep_440_version():
    with pytest.raises(check_version.VersionError, match="not a PEP 440 version"):
        check_version.is_pre_release("latest")


def test_main_prints_the_version_the_tag_names(capsys):
    declared = check_version.declared_version()
    assert check_version.main(["--tag", _core_tag(declared)]) == 0
    assert capsys.readouterr().out.strip() == declared


def test_main_routes_a_pre_release_by_printing_yes_or_no(capsys):
    declared = check_version.declared_version()
    assert check_version.main(["--tag", _core_tag(declared), "--pre-release"]) == 0
    expected = "yes" if check_version.is_pre_release(declared) else "no"
    assert capsys.readouterr().out.strip() == expected


def test_main_fails_on_a_tag_for_another_version(capsys):
    assert check_version.main(["--tag", "v0.0.0"]) == 1
    captured = capsys.readouterr()
    assert captured.out == ""
    assert "error:" in captured.err
    assert "declares" in captured.err


def test_main_fails_on_a_tag_that_names_no_version(capsys):
    assert check_version.main(["--tag", "v0.0.0-not-declared"]) == 1
    captured = capsys.readouterr()
    assert captured.out == ""
    assert "is not a core release tag of the form" in captured.err


def test_main_reads_the_tag_from_circle_tag(monkeypatch, capsys):
    declared = check_version.declared_version()
    monkeypatch.setenv("CIRCLE_TAG", _core_tag(declared))
    assert check_version.main([]) == 0
    assert capsys.readouterr().out.strip() == declared


def test_node_version_agrees_with_the_real_tree():
    """package.json and _version.py must stay in lockstep on every commit."""
    assert check_version.node_version() == check_version.node_declared_version()


def test_node_version_accepts_a_matching_release():
    assert (
        check_version.node_version(declared="0.9.0", node="0.9.0", tag="v0.9.0") == "0.9.0"
    )


def test_node_version_accepts_a_matching_pre_release():
    assert (
        check_version.node_version(declared="0.9.0rc1", node="0.9.0-rc.1", tag="v0.9.0-rc.1")
        == "0.9.0-rc.1"
    )


def test_node_version_rejects_a_disagreeing_package_json():
    with pytest.raises(
        check_version.VersionError, match="client/node/package.json declares"
    ):
        check_version.node_version(declared="0.9.1", node="0.9.0")


def test_node_version_rejects_a_tag_that_misspells_the_npm_version():
    with pytest.raises(check_version.VersionError, match="must spell it exactly"):
        check_version.node_version(declared="0.9.0rc1", node="0.9.0-rc.1", tag="v0.9.0-rc1")


def test_node_version_rejects_a_suffix_without_one_pep_440_meaning():
    with pytest.raises(check_version.VersionError, match="is not of the form"):
        check_version.node_version(declared="0.9.0", node="0.9.0-1")


def test_node_declared_version_rejects_a_file_without_a_version(tmp_path):
    package_json = tmp_path / "package.json"
    package_json.write_text('{"name": "x"}\n', encoding="utf-8")
    with pytest.raises(check_version.VersionError, match="declares no version"):
        check_version.node_declared_version(str(package_json))


def test_main_prints_the_npm_version_the_tag_names(capsys):
    declared = check_version.node_declared_version()
    assert check_version.main(["--tag", f"v{declared}", "--node"]) == 0
    assert capsys.readouterr().out.strip() == declared


def test_main_prints_no_for_a_stable_npm_version(capsys):
    declared = check_version.node_declared_version()
    assert check_version.main(["--tag", f"v{declared}", "--node", "--pre-release"]) == 0
    assert capsys.readouterr().out.strip() == "no"


def test_java_version_agrees_with_the_real_tree():
    """client/java/pom.xml and _version.py must stay in lockstep on every commit."""
    assert check_version.java_version() == check_version.java_declared_version()


def test_java_version_accepts_a_matching_release():
    assert (
        check_version.java_version(declared="0.9.0", java="0.9.0", tag="v0.9.0") == "0.9.0"
    )


def test_java_version_accepts_a_matching_pre_release():
    assert (
        check_version.java_version(declared="0.9.0rc1", java="0.9.0-rc1", tag="v0.9.0-rc1")
        == "0.9.0-rc1"
    )


def test_java_version_rejects_a_disagreeing_pom():
    with pytest.raises(check_version.VersionError, match="client/java/pom.xml declares"):
        check_version.java_version(declared="0.9.1", java="0.9.0")


def test_java_version_rejects_a_tag_that_misspells_the_maven_version():
    with pytest.raises(check_version.VersionError, match="must spell it exactly"):
        check_version.java_version(declared="0.9.0rc1", java="0.9.0-rc1", tag="v0.9.0-rc.1")


def test_java_version_rejects_a_snapshot_version():
    with pytest.raises(check_version.VersionError, match="is not of the form"):
        check_version.java_version(declared="0.9.0", java="0.9.0-SNAPSHOT")


def test_java_declared_version_rejects_a_pom_without_own_version(tmp_path):
    pom = tmp_path / "pom.xml"
    pom.write_text(
        "<project xmlns='http://maven.apache.org/POM/4.0.0'>"
        "<parent><version>1.0</version></parent></project>\n",
        encoding="utf-8",
    )
    with pytest.raises(check_version.VersionError, match="declares no version"):
        check_version.java_declared_version(str(pom))


def test_every_in_repo_reference_names_the_poms_version():
    """The parent pom's version is the one the checkout's consumers must name."""
    ns = "{http://maven.apache.org/POM/4.0.0}"
    version = check_version.java_declared_version()
    repo = pathlib.Path(check_version.REPO_ROOT)

    for module in ["opensysml-client", "opensysml-conformance"]:
        pom = ET.parse(repo / "client/java" / module / "pom.xml").getroot()
        parent_version = pom.findtext(f"{ns}parent/{ns}version")
        assert parent_version == version, module

    cameo = ET.parse(repo / "editors/cameo/pom.xml").getroot()
    assert cameo.findtext(f"{ns}properties/{ns}opensysml.client.version") == version

    syson = ET.parse(repo / "editors/syson/backend/pom.xml").getroot()
    for dep in syson.iter(f"{ns}dependency"):
        if dep.findtext(f"{ns}artifactId") == "opensysml-client":
            assert dep.findtext(f"{ns}version") == version
            break
    else:
        pytest.fail("editors/syson/backend/pom.xml names no opensysml-client")


def test_main_prints_the_maven_version_the_tag_names(capsys):
    declared = check_version.java_declared_version()
    assert check_version.main(["--tag", f"v{declared}", "--java"]) == 0
    assert capsys.readouterr().out.strip() == declared


def test_main_refuses_node_and_java_together():
    with pytest.raises(SystemExit):
        check_version.main(["--tag", "v0.9.0", "--node", "--java"])


def test_rust_version_agrees_with_the_real_tree():
    """client/rust/opensysml/Cargo.toml and _version.py must stay in lockstep on every commit."""
    assert check_version.rust_version() == check_version.rust_declared_version()


def test_rust_version_accepts_a_matching_release():
    assert (
        check_version.rust_version(declared="0.9.0", rust="0.9.0", tag="v0.9.0")
        == "0.9.0"
    )


def test_rust_version_accepts_a_matching_pre_release():
    assert (
        check_version.rust_version(
            declared="0.9.0rc1", rust="0.9.0-rc.1", tag="v0.9.0-rc.1"
        )
        == "0.9.0-rc.1"
    )


def test_rust_version_rejects_a_disagreeing_cargo_toml():
    with pytest.raises(
        check_version.VersionError, match="client/rust/opensysml/Cargo.toml declares"
    ):
        check_version.rust_version(declared="0.9.1", rust="0.9.0")


def test_rust_version_rejects_a_tag_that_misspells_the_crate_version():
    with pytest.raises(check_version.VersionError, match="must spell it exactly"):
        check_version.rust_version(
            declared="0.9.0rc1", rust="0.9.0-rc.1", tag="v0.9.0-rc1"
        )


def test_rust_declared_version_accepts_a_literal_string(tmp_path):
    cargo = tmp_path / "Cargo.toml"
    cargo.write_text("[package]\nname = 'x'\nversion = '0.9.0'\n", encoding="utf-8")
    assert check_version.rust_declared_version(str(cargo)) == "0.9.0"


def test_rust_declared_version_rejects_a_toml_without_a_package_version(tmp_path):
    cargo = tmp_path / "Cargo.toml"
    cargo.write_text(
        '[package]\nname = "x"\n\n[dependencies]\nfoo = { version = "1.0" }\n',
        encoding="utf-8",
    )
    with pytest.raises(
        check_version.VersionError, match="declares no \\[package\\] version"
    ):
        check_version.rust_declared_version(str(cargo))


def test_cargo_lock_names_the_cargo_tomls_version():
    """Cargo.lock's opensysml entry must follow client/rust/opensysml/Cargo.toml."""
    version = check_version.rust_declared_version()
    lock = pathlib.Path(check_version.REPO_ROOT) / "client/rust/Cargo.lock"
    lines = lock.read_text(encoding="utf-8").splitlines()
    for i, line in enumerate(lines):
        if line.strip() == 'name = "opensysml"':
            for entry in lines[i + 1 :]:
                if entry.startswith("["):
                    break
                match = re.match(r'^version = "(.+)"', entry)
                if match:
                    assert match[1] == version
                    break
            else:
                pytest.fail("Cargo.lock's opensysml entry declares no version")
            break
    else:
        pytest.fail("client/rust/Cargo.lock names no opensysml package")


def test_main_prints_the_crates_version_the_tag_names(capsys):
    declared = check_version.rust_declared_version()
    assert check_version.main(["--tag", f"v{declared}", "--rust"]) == 0
    assert capsys.readouterr().out.strip() == declared


def test_main_refuses_rust_and_java_together():
    with pytest.raises(SystemExit):
        check_version.main(["--tag", "v0.9.0", "--rust", "--java"])


_EDITOR_PARENTS = (
    "editors/cameo/plugin/pom.xml",
    "editors/cameo/tools/pom.xml",
    "editors/cameo/openapi-stubs/pom.xml",
    "editors/cameo/dist/pom.xml",
    "editors/syson/backend/pom.xml",
    "editors/syson/syson-api-stubs/pom.xml",
)


def _editor_tree(root, version):
    """The minimal editor manifest tree editors_version reads, at one version."""
    ns = 'xmlns="http://maven.apache.org/POM/4.0.0"'
    for pkg in ("editors/vscode", "editors/syson/frontend"):
        (root / pkg).mkdir(parents=True, exist_ok=True)
        (root / pkg / "package.json").write_text(
            f'{{"version": "{version}"}}', encoding="utf-8"
        )
        (root / pkg / "package-lock.json").write_text(
            f'{{"version": "{version}", '
            f'"packages": {{"": {{"version": "{version}"}}}}}}',
            encoding="utf-8",
        )
    for relpath in ("editors/cameo/pom.xml", "editors/syson/pom.xml"):
        (root / relpath).parent.mkdir(parents=True, exist_ok=True)
        (root / relpath).write_text(
            f'<project {ns}><version>{version}</version></project>',
            encoding="utf-8",
        )
    for relpath in _EDITOR_PARENTS:
        (root / relpath).parent.mkdir(parents=True, exist_ok=True)
        (root / relpath).write_text(
            f'<project {ns}><parent><version>{version}</version></parent></project>',
            encoding="utf-8",
        )


def test_editors_version_agrees_with_the_real_tree():
    """Every editor manifest must carry _version.py's SemVer spelling on every commit."""
    declared = check_version.declared_version()
    version = check_version.editors_version()
    assert check_version.pep440_from_semver(version, "v") == declared


def test_main_prints_the_editors_version_the_tag_names(capsys):
    version = check_version.editors_version()
    assert check_version.main(["--tag", f"v{version}", "--editors"]) == 0
    assert capsys.readouterr().out.strip() == version


def test_editors_version_accepts_an_agreeing_tree(tmp_path):
    _editor_tree(tmp_path, "0.9.0")
    assert (
        check_version.editors_version(
            declared="0.9.0", tag="v0.9.0", root=str(tmp_path)
        )
        == "0.9.0"
    )


def test_editors_version_rejects_one_disagreeing_json(tmp_path):
    _editor_tree(tmp_path, "0.9.0")
    (tmp_path / "editors/vscode/package.json").write_text(
        '{"version": "0.9.1"}', encoding="utf-8"
    )
    with pytest.raises(
        check_version.VersionError, match="editors/vscode/package.json declares"
    ):
        check_version.editors_version(declared="0.9.0", root=str(tmp_path))


def test_lock_declared_version_rejects_mismatched_copies(tmp_path):
    lock = tmp_path / "package-lock.json"
    lock.write_text(
        '{"version": "0.9.0", "packages": {"": {"version": "0.9.1"}}}',
        encoding="utf-8",
    )
    with pytest.raises(check_version.VersionError, match="must agree"):
        check_version.lock_declared_version(str(lock))


def test_pom_parent_version_rejects_a_pom_without_parent_version(tmp_path):
    pom = tmp_path / "pom.xml"
    pom.write_text(
        '<project xmlns="http://maven.apache.org/POM/4.0.0"><parent/></project>',
        encoding="utf-8",
    )
    with pytest.raises(
        check_version.VersionError, match="declares no <parent><version>"
    ):
        check_version.pom_parent_version(str(pom))


def test_editors_version_accepts_a_matching_pre_release(tmp_path):
    _editor_tree(tmp_path, "0.9.1-rc.1")
    assert (
        check_version.editors_version(
            declared="0.9.1rc1", tag="v0.9.1-rc.1", root=str(tmp_path)
        )
        == "0.9.1-rc.1"
    )


def test_editors_version_rejects_a_tag_that_misspells_the_version(tmp_path):
    _editor_tree(tmp_path, "0.9.0")
    with pytest.raises(check_version.VersionError, match="must spell it exactly"):
        check_version.editors_version(
            declared="0.9.0", tag="v0.9.1", root=str(tmp_path)
        )


def test_main_refuses_editors_and_node_together():
    with pytest.raises(SystemExit):
        check_version.main(["--tag", "v0.9.0", "--editors", "--node"])

"""Tests that the opensysml version has exactly one source of truth.

The version used to be written in three places (`pyproject.toml`, `setup.py`
and `opensysml/__init__.py`) and drifted from the repository's own line. It is now
declared once, in `opensysml/_version.py`: the packaging metadata reads it and
`opensysml.__version__` reports it, which is the version of the code imported
even from an editable install. These tests fail if a second declaration
reappears or the two stop agreeing.
"""

import importlib
import importlib.util
import json
import os
import re
import subprocess
import sys
from importlib.metadata import version as distribution_version

import pytest
from packaging.version import Version

import opensysml
from opensysml import _dist
from opensysml._version import VERSION

PYTHON_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
CHECK_VERSION = os.path.join(PYTHON_DIR, "scripts", "check_version.py")
PYPROJECT = os.path.join(PYTHON_DIR, "pyproject.toml")


def _installed_distribution():
    """The installed opensysml distribution, or None when none is installed."""
    return _dist.installed_distribution()


def _skew(dist):
    """How an installed distribution differs from the tree under test, if it does.

    An editable install's metadata is written once, so a checkout ahead of it
    declares a version its dist-info has never heard of; that is the checkout
    being newer, not two declarations drifting apart, so only the modules being
    another tree's is skew.

    Args:
        dist (importlib.metadata.Distribution): The installed distribution

    Returns:
        str or None: What is wrong and how to fix it, or None when the
            distribution is the tree these tests import
    """
    imported = os.path.realpath(opensysml.__file__)
    installed = _dist.package_location(dist)
    if installed == imported and (dist.version == VERSION or _dist.editable_install(dist)):
        return None
    return (
        f"opensysml {dist.version!r} is installed from {installed}, while the tests "
        f"import {imported}, which declares {VERSION!r}. These tests require the "
        f"tree under test to be the installed distribution: run "
        f"`pip install -e clients/python/` (an artifact of another version installed "
        f"beside the tree reports its own version, and the declaration is the "
        f"single source of truth, so the artifact is what is stale)."
    )


def _load_check_version():
    """Load scripts/check_version.py, which is release tooling, not a module."""
    spec = importlib.util.spec_from_file_location("check_version", CHECK_VERSION)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


check_version = _load_check_version()


def test_declared_version_is_pep440():
    """The declared version is a version PyPI will accept."""
    assert str(Version(VERSION)) == VERSION


def test_dunder_version_matches_declaration():
    """`opensysml.__version__` agrees with the single declaration."""
    dist = _installed_distribution()
    if dist is not None:
        assert _skew(dist) is None, _skew(dist)
    assert opensysml.__version__ == VERSION


def test_installed_metadata_matches_declaration():
    """The installed distribution's version comes from the declaration.

    Skipped where nothing is installed, since a source tree has no metadata to
    compare; a *stale* artifact is not a reason to skip, it is the skew. An
    editable install's metadata is only asserted to have come from a declaration
    of this tree: it is written at install time and cannot follow a later bump.
    """
    dist = _installed_distribution()
    if dist is None:
        pytest.skip("opensysml is not installed; nothing to compare metadata against")
    assert _skew(dist) is None, _skew(dist)
    if _dist.editable_install(dist):
        assert _dist.project_directory(dist) == os.path.realpath(PYTHON_DIR)
        assert str(Version(dist.version)) == dist.version
        return
    assert distribution_version("opensysml") == VERSION


def test_the_build_takes_the_version_from_the_declaration():
    """The metadata is generated from the declaration, in any environment.

    This holds whatever is installed, so it states the contract itself rather
    than the environment the two tests above require.
    """
    with open(PYPROJECT, encoding="utf-8") as f:
        pyproject = f.read()
    assert 'dynamic = ["version"]' in pyproject
    assert re.search(
        r'version\s*=\s*\{\s*attr\s*=\s*"opensysml\._version\.VERSION"\s*\}', pyproject
    )


def test_no_second_version_declaration():
    """No file under clients/python/ hard-codes a version literal of its own.

    `_version.py` holds the declaration and the tests compare against it; any
    other `version = "x.y.z"` is the duplication this collapsed.
    """
    pattern = re.compile(r"""^\s*(?:__version__|version)\s*=\s*["']\d+\.\d+""")
    offenders = []
    for root, dirs, files in os.walk(PYTHON_DIR):
        dirs[:] = [
            d for d in dirs
            if d not in {".git", "build", "dist", "__pycache__", ".venv"}
            and not d.endswith(".egg-info")
        ]
        for name in files:
            if not name.endswith((".py", ".toml", ".cfg")):
                continue
            path = os.path.join(root, name)
            if os.path.samefile(path, os.path.join(PYTHON_DIR, "opensysml", "_version.py")):
                continue
            with open(path, encoding="utf-8") as f:
                for lineno, line in enumerate(f, 1):
                    if pattern.match(line):
                        offenders.append(f"{os.path.relpath(path, PYTHON_DIR)}:{lineno}")
    assert offenders == [], (
        "version literals outside opensysml/_version.py: " + ", ".join(offenders)
    )


def test_setup_py_is_gone():
    """pyproject.toml declares the build; a setup.py would be a second answer."""
    assert not os.path.exists(os.path.join(PYTHON_DIR, "setup.py"))


def _core_tag(version):
    """The SemVer core tag naming a PEP 440 version: 0.5.0rc1 is tagged v0.5.0-rc1."""
    parsed = Version(version)
    tag = f"v{parsed.base_version}"
    if parsed.pre is not None:
        phase, number = parsed.pre
        tag += f"-{ {'a': 'alpha', 'b': 'beta', 'rc': 'rc'}[phase] }.{number}"
    return tag


TAG = _core_tag(VERSION)


def test_version_from_tag_accepts_the_matching_tag():
    """The core tag naming the declared version yields that version."""
    assert check_version.version_from_tag(TAG) == VERSION


@pytest.mark.parametrize("tag, expected", [
    ("", "reads CIRCLE_TAG"),
    (f"opensysml-v{VERSION}", "does not start with"),
    ("0.1.0", "does not start with"),
    ("v9.9.9", "declares"),
])
def test_version_from_tag_rejects(tag, expected):
    """A tag that would publish the wrong version fails, with a reason."""
    with pytest.raises(check_version.VersionError, match=expected):
        check_version.version_from_tag(tag)


@pytest.mark.parametrize("version, pre", [
    ("0.1.0", False),
    ("1.2.3", False),
    ("0.1.0rc1", True),
    ("0.1.0a1", True),
    ("0.1.0b2", True),
])
def test_pre_release_detection(version, pre):
    """Pre-release versions route to TestPyPI; final ones to PyPI."""
    assert check_version.is_pre_release(version) is pre


def test_check_version_cli_reports_the_version():
    """The job reads the version to publish off this script's stdout."""
    out = subprocess.run(
        [sys.executable, CHECK_VERSION, "--tag", TAG],
        capture_output=True, text=True, check=True,
    )
    assert out.stdout.strip() == VERSION


def test_check_version_cli_fails_on_a_mismatched_tag():
    """A mismatch fails the job before anything is built or uploaded."""
    out = subprocess.run(
        [sys.executable, CHECK_VERSION, "--tag", "v9.9.9"],
        capture_output=True, text=True,
    )
    assert out.returncode == 1
    assert "9.9.9" in out.stderr
    assert VERSION in out.stderr


class TestEditableInstall:
    """Version resolution for an install whose modules are a checkout.

    A PEP 660 editable install puts its dist-info in site-packages and leaves the
    modules in the checkout, so `locate_file` names a `opensysml/` that is not
    there and metadata written at install time cannot follow a later bump. Both
    used to make `test_version.py` fail for a developer working from a checkout.
    """

    class FakeDistribution:
        """A dist-info as an editable install writes it, without installing one."""

        def __init__(self, version, dist_info, project=None):
            self.version = version
            self._dist_info = str(dist_info)
            self._project = None if project is None else str(project)

        def locate_file(self, path):
            return os.path.join(os.path.dirname(self._dist_info), str(path))

        def read_text(self, name):
            if name != "direct_url.json" or self._project is None:
                return None
            return json.dumps({
                "url": f"file://{self._project}",
                "dir_info": {"editable": True},
            })

    def editable(self, tmp_path, version=VERSION, layout=""):
        """An editable install of a checkout laid out under tmp_path."""
        site_packages = tmp_path / "site-packages"
        (site_packages / f"opensysml-{version}.dist-info").mkdir(parents=True)
        project = tmp_path / "checkout" / "python"
        package = project.joinpath(*filter(None, [layout, "opensysml"]))
        package.mkdir(parents=True)
        (package / "__init__.py").write_text("")
        return self.FakeDistribution(
            version, site_packages / f"opensysml-{version}.dist-info", project
        ), package / "__init__.py"

    def test_the_package_is_found_in_the_checkout_not_in_site_packages(self, tmp_path):
        """locate_file answers from the dist-info's directory, which holds nothing."""
        dist, init = self.editable(tmp_path)
        assert _dist.editable_install(dist)
        assert not os.path.exists(dist.locate_file("opensysml/__init__.py"))
        assert _dist.package_location(dist) == os.path.realpath(str(init))

    def test_a_src_layout_checkout_is_found_too(self, tmp_path):
        dist, init = self.editable(tmp_path, layout="src")
        assert _dist.package_location(dist) == os.path.realpath(str(init))

    def test_a_non_editable_install_is_located_by_its_dist_info(self, tmp_path):
        """A wheel's modules sit beside its dist-info, and record no directory."""
        dist_info = tmp_path / "site-packages" / f"opensysml-{VERSION}.dist-info"
        dist_info.mkdir(parents=True)
        dist = self.FakeDistribution(VERSION, dist_info)
        assert not _dist.editable_install(dist)
        assert _dist.project_directory(dist) is None
        assert _dist.package_location(dist) == os.path.realpath(
            str(tmp_path / "site-packages" / "opensysml" / "__init__.py")
        )

    def test_a_checkout_ahead_of_its_metadata_is_not_skew(self, tmp_path, monkeypatch):
        """The bug: bumping VERSION after `pip install -e` failed the suite."""
        dist, init = self.editable(tmp_path, version="0.0.1")
        monkeypatch.setattr(opensysml, "__file__", str(init))
        assert dist.version != VERSION
        assert _skew(dist) is None

    def test_another_tree_installed_beside_this_one_is_still_skew(self, tmp_path):
        """Only a checkout ahead of its own metadata is forgiven, not a stranger."""
        dist, _ = self.editable(tmp_path, version="0.0.1")
        assert "is installed from" in _skew(dist)

    def test_the_version_reported_is_the_declaration_not_the_metadata(self, monkeypatch):
        """__version__ is the code's own declaration, so a frozen dist-info cannot lie."""
        monkeypatch.setattr("importlib.metadata.version", lambda name: "0.0.1")
        reloaded = importlib.reload(opensysml)
        try:
            assert reloaded.__version__ == VERSION
        finally:
            importlib.reload(opensysml)

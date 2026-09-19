"""Tests the final `pysysml` release, which only reports the rename.

`packaging/pypi-pysysml/` is a distribution of its own, published once so that
`pip install pysysml` says the name moved instead of resolving to 0.2.0. These
tests import it from source, since it is never installed beside `opensysml`.
"""

import importlib.util
import os
import re
import sys

import pytest

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
PLACEHOLDER_DIR = os.path.join(REPO_ROOT, "packaging", "pypi-pysysml")
PLACEHOLDER_INIT = os.path.join(PLACEHOLDER_DIR, "pysysml", "__init__.py")
PLACEHOLDER_PYPROJECT = os.path.join(PLACEHOLDER_DIR, "pyproject.toml")


def _import_placeholder():
    """Execute the placeholder package under a name that cannot shadow anything."""
    spec = importlib.util.spec_from_file_location("_pysysml_placeholder", PLACEHOLDER_INIT)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)


def _pyproject():
    with open(PLACEHOLDER_PYPROJECT, encoding="utf-8") as f:
        return f.read()


def test_importing_the_placeholder_fails_with_the_rename():
    """Import raises rather than working: the old name is over, not aliased."""
    with pytest.raises(ImportError) as excinfo:
        _import_placeholder()
    message = str(excinfo.value)
    assert "pip install opensysml" in message
    assert "import opensysml" in message
    for old, new in [
        ("PYSYSML_", "OPENSYSML_"),
        ("~/.pysysml", "~/.opensysml"),
        ("PySysMLError", "OpenSysMLError"),
        ("pysysml-generate", "opensysml-generate"),
    ]:
        assert old in message
        assert new in message


def test_the_placeholder_pulls_in_nothing():
    """No dependency on opensysml: installing it would revive the old name."""
    assert re.search(r"^dependencies\s*=\s*\[\]", _pyproject(), re.MULTILINE)
    # No requires-python floor either: below it pip would resolve back to 0.2.0.
    assert not re.search(r"^requires-python\s*=", _pyproject(), re.MULTILINE)


def test_the_placeholder_supersedes_the_last_real_release():
    """pip must prefer this over 0.2.0, or an install of the old name still works."""
    from packaging.version import Version

    declared = re.search(r'^version\s*=\s*"([^"]+)"', _pyproject(), re.MULTILINE)
    assert declared, "the placeholder declares no version"
    assert Version(declared.group(1)) > Version("0.2.0")


def test_the_placeholder_is_not_installed():
    """It is a published artifact only; importing pysysml here must still fail."""
    assert importlib.util.find_spec("pysysml") is None
    assert "pysysml" not in sys.modules

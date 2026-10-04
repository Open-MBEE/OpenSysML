import importlib.util
import os
import shutil
import subprocess
import sys
from pathlib import Path

import pytest


PYTHON_ROOT = Path(__file__).resolve().parents[1]
TYPECHECK_SOURCE = Path(__file__).parent / "typecheck" / "metamodel_usage.py"
TYPECHECK_DIR = TYPECHECK_SOURCE.parent


def test_metamodel_types_with_mypy_and_pyright():
    missing = []
    if importlib.util.find_spec("mypy") is None:
        missing.append("mypy")
    if shutil.which("pyright") is None:
        missing.append("pyright")
    if missing:
        message = "missing type checker(s): " + ", ".join(missing)
        if os.environ.get("OPENSYSML_REQUIRE_TYPECHECKERS", "").strip().lower() not in (
            "",
            "0",
            "false",
            "no",
        ):
            pytest.fail(message)
        pytest.skip(message)

    package = PYTHON_ROOT / "opensysml" / "metamodel"
    mypy = subprocess.run(
        [
            sys.executable,
            "-m",
            "mypy",
            "--strict",
            "--config-file",
            str(TYPECHECK_DIR / "mypy.ini"),
            str(TYPECHECK_SOURCE),
            str(package),
        ],
        cwd=PYTHON_ROOT,
        text=True,
        capture_output=True,
    )
    assert mypy.returncode == 0, mypy.stdout + mypy.stderr

    pyright = subprocess.run(
        ["pyright", "-p", str(TYPECHECK_DIR)],
        cwd=PYTHON_ROOT,
        text=True,
        capture_output=True,
    )
    assert pyright.returncode == 0, pyright.stdout + pyright.stderr

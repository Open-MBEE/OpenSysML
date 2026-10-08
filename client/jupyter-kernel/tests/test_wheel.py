"""The platform wheels setup.py builds: tagged for their kernel, carrying it and the kernelspec."""

import json
import os
import shutil
import stat
import subprocess
import sys
import tarfile
import zipfile
from types import SimpleNamespace

import pytest

from jupyter_opensysml_kernel import binary
from jupyter_opensysml_kernel._version import VERSION

from .conftest import KERNEL_BYTES

SOURCE = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PLATFORM_ENV = "JUPYTER_OPENSYSML_KERNEL_PLATFORM"

pytest.importorskip("build", reason="the distributions are built as the release builds them, with python -m build")


@pytest.fixture
def source_tree(tmp_path):
    """A copy of the package source, so staging a kernel never touches the checkout."""
    tree = tmp_path / "src"
    shutil.copytree(
        SOURCE,
        tree,
        ignore=shutil.ignore_patterns("build", "dist", "*.egg-info", "__pycache__", ".mypy_cache", ".pytest_cache", "tests"),
    )
    return tree


def stage(tree, goos):
    bin_dir = tree / "jupyter_opensysml_kernel" / "bin"
    bin_dir.mkdir(exist_ok=True)
    path = bin_dir / binary.binary_name(goos)
    path.write_bytes(KERNEL_BYTES)
    path.chmod(path.stat().st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
    return path


def build(tree, command, platform=None):
    """`python -m build --wheel` or `--sdist`, as the release pipeline runs it."""
    env = dict(os.environ)
    env.pop(PLATFORM_ENV, None)
    if platform is not None:
        env[PLATFORM_ENV] = platform
    result = subprocess.run(
        [sys.executable, "-m", "build", "--" + command, "--outdir", "dist", "."],
        cwd=tree,
        env=env,
        capture_output=True,
        text=True,
    )
    return SimpleNamespace(returncode=result.returncode, output=result.stdout + result.stderr)


def built(tree, suffix):
    files = [f for f in os.listdir(tree / "dist") if f.endswith(suffix)]
    assert len(files) == 1, files
    return tree / "dist" / files[0]


def test_a_platform_wheel_bundles_the_kernel_and_registers_it(source_tree):
    stage(source_tree, "linux")
    result = build(source_tree, "wheel", "linux-amd64")
    assert result.returncode == 0, result.output
    wheel = built(source_tree, ".whl")
    assert wheel.name == (
        f"jupyter_opensysml_kernel-{VERSION}-py3-none-"
        "manylinux_2_17_x86_64.manylinux2014_x86_64.musllinux_1_1_x86_64.whl"
    )
    with zipfile.ZipFile(wheel) as zf:
        names = set(zf.namelist())
        data = f"jupyter_opensysml_kernel-{VERSION}.data/data/share/jupyter/kernels/sysml/"
        assert "jupyter_opensysml_kernel/bin/sysml-jupyter-kernel" in names
        assert {data + "kernel.json", data + "logo-32x32.png", data + "logo-64x64.png"} <= names
        info = zf.getinfo("jupyter_opensysml_kernel/bin/sysml-jupyter-kernel")
        assert (info.external_attr >> 16) & stat.S_IXUSR, "the bundled kernel lost its execute bit"
        assert zf.read("jupyter_opensysml_kernel/bin/sysml-jupyter-kernel") == KERNEL_BYTES
        spec = json.loads(zf.read(data + "kernel.json"))
        assert spec["argv"] == ["python", "-m", "jupyter_opensysml_kernel", "-connection-file", "{connection_file}"]
        assert spec["display_name"] == "SysML v2 (OpenSysML)"
        wheel_meta = zf.read(f"jupyter_opensysml_kernel-{VERSION}.dist-info/WHEEL").decode()
    assert "Root-Is-Purelib: false" in wheel_meta
    assert "Tag: py3-none-manylinux_2_17_x86_64" in wheel_meta


def test_the_windows_wheel_bundles_the_exe(source_tree):
    stage(source_tree, "windows")
    result = build(source_tree, "wheel", "windows-amd64")
    assert result.returncode == 0, result.output
    wheel = built(source_tree, ".whl")
    assert wheel.name.endswith("-py3-none-win_amd64.whl")
    with zipfile.ZipFile(wheel) as zf:
        assert "jupyter_opensysml_kernel/bin/sysml-jupyter-kernel.exe" in zf.namelist()


def test_a_wheel_built_after_another_platform_carries_only_its_own_kernel(source_tree):
    """The release builds every platform's wheel from one tree, and setuptools keeps
    build/lib between them; a kernel staged for an earlier platform must not ride along."""
    staged = stage(source_tree, "darwin")
    result = build(source_tree, "wheel", "darwin-arm64")
    assert result.returncode == 0, result.output
    staged.unlink()
    stage(source_tree, "windows")
    result = build(source_tree, "wheel", "windows-amd64")
    assert result.returncode == 0, result.output
    wheel = next(f for f in os.listdir(source_tree / "dist") if f.endswith("-py3-none-win_amd64.whl"))
    with zipfile.ZipFile(source_tree / "dist" / wheel) as zf:
        bundled = sorted(n for n in zf.namelist() if n.startswith("jupyter_opensysml_kernel/bin/"))
    assert bundled == ["jupyter_opensysml_kernel/bin/sysml-jupyter-kernel.exe"]


def test_a_pure_wheel_bundles_nothing_and_registers_nothing(source_tree):
    result = build(source_tree, "wheel")
    assert result.returncode == 0, result.output
    wheel = built(source_tree, ".whl")
    assert wheel.name == f"jupyter_opensysml_kernel-{VERSION}-py3-none-any.whl"
    with zipfile.ZipFile(wheel) as zf:
        assert not any("/bin/" in n or "share/jupyter" in n for n in zf.namelist())


def test_a_staged_kernel_without_a_platform_is_refused(source_tree):
    stage(source_tree, "linux")
    result = build(source_tree, "wheel")
    assert result.returncode != 0
    assert "a pure wheel bundles no kernel" in result.output


def test_the_wrong_kernel_for_the_platform_is_refused(source_tree):
    stage(source_tree, "linux")
    result = build(source_tree, "wheel", "windows-amd64")
    assert result.returncode != 0
    assert "needs exactly sysml-jupyter-kernel.exe" in result.output


def test_an_unreleased_platform_is_refused(source_tree):
    stage(source_tree, "linux")
    result = build(source_tree, "wheel", "plan9-mips")
    assert result.returncode != 0
    assert "no kernel is released for plan9-mips" in result.output


def test_the_sdist_refuses_a_staged_kernel(source_tree):
    stage(source_tree, "linux")
    result = build(source_tree, "sdist")
    assert result.returncode != 0
    assert "an sdist bundles no kernel" in result.output


def test_the_sdist_carries_setup_py_and_no_bin(source_tree):
    result = build(source_tree, "sdist")
    assert result.returncode == 0, result.output
    with tarfile.open(built(source_tree, ".tar.gz")) as tf:
        names = tf.getnames()
    assert any(n.endswith("/setup.py") for n in names)
    assert not any("/bin/" in n for n in names)


@pytest.mark.parametrize(
    ("goos", "goarch", "tag"),
    [
        ("linux", "amd64", "manylinux_2_17_x86_64.manylinux2014_x86_64.musllinux_1_1_x86_64"),
        ("linux", "arm64", "manylinux_2_17_aarch64.manylinux2014_aarch64.musllinux_1_1_aarch64"),
        ("darwin", "amd64", "macosx_11_0_x86_64"),
        ("darwin", "arm64", "macosx_11_0_arm64"),
        ("windows", "amd64", "win_amd64"),
    ],
)
def test_every_released_platform_has_a_wheel_tag(goos, goarch, tag):
    assert binary.wheel_platform_tag(goos, goarch) == tag


def test_an_unreleased_pair_has_no_wheel_tag():
    with pytest.raises(binary.UnsupportedPlatformError):
        binary.wheel_platform_tag("windows", "arm64")

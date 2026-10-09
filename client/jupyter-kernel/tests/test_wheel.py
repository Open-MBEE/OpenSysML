"""The distributions setup.py builds: platform wheels tagged for their kernel, carrying it and
the kernelspec, and every one of them carrying the JupyterLab extension."""

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

import jupyter_opensysml_kernel
from jupyter_opensysml_kernel import binary, labextension
from jupyter_opensysml_kernel._version import VERSION

from .conftest import KERNEL_BYTES

SOURCE = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PLATFORM_ENV = "JUPYTER_OPENSYSML_KERNEL_PLATFORM"
LABEXTENSION = "share/jupyter/labextensions/jupyterlab-opensysml/"
REMOTE_ENTRY = "static/remoteEntry.0123456789abcdef.js"
LABEXTENSION_FILES = {
    "package.json": json.dumps(
        {"name": "jupyterlab-opensysml", "jupyterlab": {"extension": True, "_build": {"load": REMOTE_ENTRY}}}
    ).encode(),
    "install.json": b'{"packageManager": "python", "packageName": "jupyter-opensysml-kernel"}',
    REMOTE_ENTRY: b"// the federated entry\n",
    "static/style.js": b"",
}

pytest.importorskip("build", reason="the distributions are built as the release builds them, with python -m build")


@pytest.fixture
def bare_tree(tmp_path):
    """A copy of the package source without any build of the extension, so staging a
    kernel or an extension never touches the checkout."""
    tree = tmp_path / "src"
    shutil.copytree(
        SOURCE,
        tree,
        ignore=shutil.ignore_patterns(
            "build", "dist", "*.egg-info", "__pycache__", ".mypy_cache", ".pytest_cache", "tests", "labextension"
        ),
    )
    return tree


@pytest.fixture
def source_tree(bare_tree):
    """The source with a build of the JupyterLab extension staged, as `make jupyterlab-build` leaves it."""
    stage_labextension(bare_tree)
    return bare_tree


def stage_labextension(tree):
    root = tree / "labextension"
    for name, data in LABEXTENSION_FILES.items():
        path = root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)
    return root


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


def labextension_members(names, prefix):
    """The extension's files among an archive's `names`, keyed under its share/ directory."""
    return {n[len(prefix) + len(LABEXTENSION):] for n in names if n.startswith(prefix + LABEXTENSION)}


def assert_carries_the_labextension(zf, data_prefix):
    names = set(zf.namelist())
    assert labextension_members(names, data_prefix) == set(LABEXTENSION_FILES)
    for name, data in LABEXTENSION_FILES.items():
        assert zf.read(data_prefix + LABEXTENSION + name) == data


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


def test_a_pure_wheel_bundles_no_kernel_and_registers_only_the_labextension(source_tree):
    result = build(source_tree, "wheel")
    assert result.returncode == 0, result.output
    wheel = built(source_tree, ".whl")
    assert wheel.name == f"jupyter_opensysml_kernel-{VERSION}-py3-none-any.whl"
    with zipfile.ZipFile(wheel) as zf:
        names = zf.namelist()
        assert not any("/bin/" in n or "share/jupyter/kernels" in n for n in names)
        assert_carries_the_labextension(zf, f"jupyter_opensysml_kernel-{VERSION}.data/data/")
        assert not any(n.startswith("jupyter_opensysml_kernel/labextension/") for n in names), "the extension is shared data, not package data"


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


def test_the_sdist_carries_setup_py_and_the_labextension_and_no_bin(source_tree, tmp_path):
    result = build(source_tree, "sdist")
    assert result.returncode == 0, result.output
    sdist = built(source_tree, ".tar.gz")
    with tarfile.open(sdist) as tf:
        names = [m.name for m in tf.getmembers() if m.isfile()]
        tf.extractall(tmp_path / "unpacked", filter="data")
    assert any(n.endswith("/setup.py") for n in names)
    assert not any("/bin/" in n for n in names)
    root = f"jupyter_opensysml_kernel-{VERSION}/"
    staged = root + "labextension/"
    assert {n[len(staged):] for n in names if n.startswith(staged)} == set(LABEXTENSION_FILES)
    # A wheel built from the sdist, as pip builds one, installs the extension too.
    unpacked = tmp_path / "unpacked" / root
    result = build(unpacked, "wheel")
    assert result.returncode == 0, result.output
    with zipfile.ZipFile(built(unpacked, ".whl")) as zf:
        assert_carries_the_labextension(zf, f"jupyter_opensysml_kernel-{VERSION}.data/data/")


@pytest.mark.parametrize("command", ["sdist", "wheel"])
def test_no_distribution_is_built_without_the_labextension(bare_tree, command):
    result = build(bare_tree, command)
    assert result.returncode != 0
    assert "run `make jupyterlab-build` first" in result.output


def test_a_platform_wheel_is_refused_without_the_labextension(bare_tree):
    stage(bare_tree, "linux")
    result = build(bare_tree, "wheel", "linux-amd64")
    assert result.returncode != 0
    assert "run `make jupyterlab-build` first" in result.output


def test_the_labextension_module_describes_a_staged_build(bare_tree):
    assert not labextension.is_built(str(bare_tree / "labextension"))
    root = stage_labextension(bare_tree)
    assert labextension.is_built(str(root))
    assert labextension.data_files(str(root)) == [
        (
            "share/jupyter/labextensions/jupyterlab-opensysml",
            ["labextension/install.json", "labextension/package.json"],
        ),
        (
            "share/jupyter/labextensions/jupyterlab-opensysml/static",
            [
                "labextension/static/remoteEntry.0123456789abcdef.js",
                "labextension/static/style.js",
            ],
        ),
    ]
    (root / "static" / "remoteEntry.0123456789abcdef.js").unlink()
    assert not labextension.is_built(str(root)), "a build without its federated entry is no build"


def test_jupyter_labextension_develop_finds_the_extension_through_the_package():
    assert jupyter_opensysml_kernel._jupyter_labextension_paths() == [{"src": "../labextension", "dest": "jupyterlab-opensysml"}]


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

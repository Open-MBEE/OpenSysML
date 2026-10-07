"""`python -m jupyter_opensysml_kernel -connection-file ...` starts the bundled kernel."""

import os
import stat

import pytest

from jupyter_opensysml_kernel import __main__ as cli
from jupyter_opensysml_kernel import binary, kernelspec

from .conftest import KERNEL_BYTES


@pytest.fixture
def bundled(tmp_path, monkeypatch):
    """A platform wheel's install: the kernel under the package's bin/."""
    bin_dir = tmp_path / "bin"
    bin_dir.mkdir()
    path = bin_dir / binary.binary_name()
    path.write_bytes(KERNEL_BYTES)
    path.chmod(path.stat().st_mode | stat.S_IXUSR)
    monkeypatch.setattr(binary, "BUNDLED_DIR", str(bin_dir))
    return str(path)


@pytest.fixture
def unbundled(tmp_path, monkeypatch):
    """An install from the sdist: no bin/ at all."""
    monkeypatch.setattr(binary, "BUNDLED_DIR", str(tmp_path / "no-bin"))


def test_kernel_flags_are_told_from_subcommands():
    assert cli.is_kernel_invocation(["-connection-file", "/tmp/k.json"])
    assert cli.is_kernel_invocation(["-version"])
    assert not cli.is_kernel_invocation(["install", "--user"])
    assert not cli.is_kernel_invocation(["--version"])
    assert not cli.is_kernel_invocation(["-h"])
    assert not cli.is_kernel_invocation([])


@pytest.mark.skipif(os.name == "nt", reason="the kernel replaces the process where exec exists")
def test_launch_execs_the_bundled_kernel_with_the_flags(bundled, monkeypatch):
    calls = []
    monkeypatch.setattr(os, "execv", lambda path, argv: calls.append((path, argv)))
    assert cli.main(["-connection-file", "/tmp/k.json"]) == 0
    assert calls == [(bundled, [bundled, "-connection-file", "/tmp/k.json"])]


@pytest.mark.skipif(os.name == "nt", reason="the kernel replaces the process where exec exists")
def test_launch_restores_a_lost_execute_bit(bundled, monkeypatch):
    os.chmod(bundled, stat.S_IRUSR | stat.S_IWUSR)
    monkeypatch.setattr(os, "execv", lambda path, argv: None)
    cli.main(["-connection-file", "/tmp/k.json"])
    assert os.access(bundled, os.X_OK)


def test_launch_without_a_bundled_kernel_says_how_to_install_one(unbundled, capsys):
    assert cli.main(["-connection-file", "/tmp/k.json"]) == 1
    err = capsys.readouterr().err
    assert "bundles no sysml-jupyter-kernel" in err
    assert "python -m jupyter_opensysml_kernel install" in err


def test_the_launcher_spec_starts_the_kernel_through_python():
    spec = kernelspec.launcher_kernel_json()
    assert spec["argv"] == ["python", "-m", "jupyter_opensysml_kernel", "-connection-file", "{connection_file}"]
    assert spec["language"] == "sysml"
    assert spec["interrupt_mode"] == "message"
    assert spec["metadata"]["package"] == "jupyter-opensysml-kernel"


def test_install_registers_the_bundled_kernel_without_a_download(bundled, release, isolated_jupyter):
    path = kernelspec.install()
    assert release.requests == []
    installed = os.path.join(path, binary.binary_name())
    with open(installed, "rb") as f:
        assert f.read() == KERNEL_BYTES
    assert os.access(installed, os.X_OK)


def test_release_downloads_instead_of_the_bundled_kernel(bundled, release, isolated_jupyter):
    kernelspec.install(version=release.version)
    assert any(url.endswith(binary.release_asset_name()) for url in release.requests)


def test_binary_is_preferred_to_the_bundled_kernel(bundled, release, local_binary, isolated_jupyter, tmp_path):
    other = tmp_path / "other-kernel"
    other.write_bytes(b"#!/bin/sh\necho other\n")
    other.chmod(other.stat().st_mode | stat.S_IXUSR)
    path = kernelspec.install(binary=str(other))
    with open(os.path.join(path, binary.binary_name()), "rb") as f:
        assert f.read() == b"#!/bin/sh\necho other\n"


def test_without_a_bundled_kernel_install_downloads(unbundled, release, isolated_jupyter):
    kernelspec.install()
    assert release.requests

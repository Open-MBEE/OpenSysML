"""Installing the kernelspec: the binary inside it, kernel.json beside it."""

import json
import os
import stat
import sys

import pytest

from jupyter_opensysml_kernel import __main__ as cli
from jupyter_opensysml_kernel import binary, kernelspec

from .conftest import KERNEL_BYTES


def read_spec(spec_dir):
    with open(os.path.join(spec_dir, "kernel.json"), encoding="utf-8") as f:
        return json.load(f)


def test_kernel_json_runs_the_binary_from_the_spec_directory():
    spec = kernelspec.kernel_json("sysml-jupyter-kernel")
    assert spec["argv"] == ["{resource_dir}/sysml-jupyter-kernel", "-connection-file", "{connection_file}"]
    assert spec["language"] == "sysml"
    assert spec["interrupt_mode"] == "message"
    assert spec["display_name"] == "SysML v2 (OpenSysML)"
    assert spec["metadata"]["implementation"] == "sysml-jupyter-kernel"
    assert spec["metadata"]["package_version"] == kernelspec.VERSION


def test_a_user_install_downloads_into_the_spec(release, isolated_jupyter):
    path = kernelspec.install(user=True)
    assert path == str(isolated_jupyter / "kernels" / "sysml")
    spec = read_spec(path)
    binary_file = spec["argv"][0].replace("{resource_dir}/", "")
    installed = os.path.join(path, binary_file)
    with open(installed, "rb") as f:
        assert f.read() == KERNEL_BYTES
    assert os.access(installed, os.X_OK)
    from jupyter_client.kernelspec import KernelSpecManager

    found = KernelSpecManager().get_kernel_spec("sysml")
    assert found.resource_dir == path
    assert found.argv[0] == "{resource_dir}/" + binary_file


def test_a_prefix_install_goes_under_share_jupyter(release, tmp_path):
    prefix = tmp_path / "env"
    path = kernelspec.install(prefix=str(prefix))
    assert path == str(prefix / "share" / "jupyter" / "kernels" / "sysml")
    assert os.path.isfile(os.path.join(path, binary.binary_name()))


def test_a_local_binary_is_installed_instead_of_a_download(release, local_binary, isolated_jupyter):
    path = kernelspec.install(binary=local_binary, user=True, name="sysml-dev", display_name="SysML (dev)")
    assert path == str(isolated_jupyter / "kernels" / "sysml-dev")
    assert read_spec(path)["display_name"] == "SysML (dev)"
    assert release.requests == []
    mode = os.stat(os.path.join(path, binary.binary_name())).st_mode
    assert mode & stat.S_IXUSR
    for name in kernelspec.LOGO_FILES:
        with open(os.path.join(path, name), "rb") as f:
            assert f.read(8) == b"\x89PNG\r\n\x1a\n"


def test_a_binary_that_is_not_executable_is_refused(release, tmp_path):
    path = tmp_path / "not-a-kernel"
    path.write_bytes(b"data")
    if sys.platform != "win32":
        with pytest.raises(binary.KernelBinaryError, match="not executable"):
            kernelspec.install(binary=str(path), user=True)
    with pytest.raises(binary.KernelBinaryError, match="not a file"):
        kernelspec.install(binary=str(tmp_path / "missing"), user=True)


def test_binary_and_release_are_not_both_accepted(release, local_binary):
    with pytest.raises(binary.KernelBinaryError, match="--binary"):
        kernelspec.install(binary=local_binary, version="v1.0.0", user=True)


def test_a_refused_download_leaves_no_kernelspec(release, isolated_jupyter):
    release.assets[binary.release_asset_name()] = b"tampered"
    with pytest.raises(binary.ChecksumMismatchError):
        kernelspec.install(user=True)
    assert not (isolated_jupyter / "kernels").exists()


def test_reinstalling_replaces_the_spec(release, local_binary, isolated_jupyter):
    kernelspec.install(user=True)
    path = kernelspec.install(binary=local_binary, user=True, display_name="Second")
    assert read_spec(path)["display_name"] == "Second"


def test_uninstall_removes_the_spec_and_its_binary(release, isolated_jupyter):
    path = kernelspec.install(user=True)
    assert kernelspec.uninstall() == path
    assert not os.path.exists(path)
    with pytest.raises(binary.KernelBinaryError, match="no kernelspec named 'sysml'"):
        kernelspec.uninstall()


def test_the_default_location_is_the_environment_or_the_user(monkeypatch):
    monkeypatch.setattr(sys, "prefix", "/env", raising=False)
    monkeypatch.setattr(sys, "base_prefix", "/env", raising=False)
    assert kernelspec.default_location() == {"user": True}
    monkeypatch.setattr(sys, "base_prefix", "/usr", raising=False)
    assert kernelspec.default_location() == {"prefix": "/env"}
    monkeypatch.setattr(sys, "base_prefix", "/env", raising=False)
    monkeypatch.setenv("CONDA_PREFIX", "/env")
    assert kernelspec.default_location() == {"prefix": "/env"}


def test_the_command_line_installs_prints_and_uninstalls(release, isolated_jupyter, capsys):
    assert cli.main(["install", "--user"]) == 0
    out = capsys.readouterr().out
    assert "Installed kernelspec sysml in" in out
    assert cli.main(["kernelspec"]) == 0
    assert json.loads(capsys.readouterr().out)["language"] == "sysml"
    assert cli.main(["uninstall"]) == 0
    assert "Removed kernelspec sysml" in capsys.readouterr().out
    assert cli.main(["uninstall"]) == 1
    assert "no kernelspec named" in capsys.readouterr().err


def test_the_command_line_reports_a_refused_download(release, isolated_jupyter, capsys):
    assert cli.main(["install", "--user", "--release", "v0.0.1"]) == 1
    assert "pins no digest" in capsys.readouterr().err


def test_sys_prefix_installs_into_the_running_environment(release, tmp_path, monkeypatch):
    monkeypatch.setattr(sys, "prefix", str(tmp_path / "env"), raising=False)
    assert cli.main(["install", "--sys-prefix"]) == 0
    assert (tmp_path / "env" / "share" / "jupyter" / "kernels" / "sysml" / "kernel.json").is_file()

"""Fixtures: an offline release and a kernelspec directory of the test's own."""

import hashlib
import os
import stat
import sys

import pytest

from jupyter_opensysml_kernel import binary

RELEASE = "v9.9.9"
REPO = "Open-MBEE/OpenSysML"
KERNEL_BYTES = b"#!/bin/sh\necho sysml-jupyter-kernel test build\n"


@pytest.fixture(autouse=True)
def isolated_jupyter(tmp_path, monkeypatch):
    """Every kernelspec write goes under the test's directory: neither the user's
    nor the Python environment the tests run in, which the default location is."""
    data_dir = tmp_path / "jupyter-data"
    monkeypatch.setenv("JUPYTER_DATA_DIR", str(data_dir))
    monkeypatch.setenv("JUPYTER_PATH", str(data_dir))
    monkeypatch.setattr(sys, "prefix", str(tmp_path / "env"))
    monkeypatch.setattr(sys, "base_prefix", str(tmp_path / "base"))
    monkeypatch.delenv("CONDA_PREFIX", raising=False)
    monkeypatch.delenv(binary.GITHUB_REPO_ENV, raising=False)
    return data_dir


class FakeRelease:
    """A release served from memory: asset name -> bytes, with sidecars."""

    def __init__(self, version=RELEASE):
        self.version = version
        self.assets = {}
        self.requests = []

    def publish(self, asset, data, sidecar=None):
        self.assets[asset] = data
        digest = hashlib.sha256(data).hexdigest()
        self.assets[asset + ".sha256"] = (f"{sidecar or digest}  {asset}\n").encode()
        return digest

    def fetch(self, url, limit):
        self.requests.append(url)
        prefix = binary.release_download_url(self.version, "", REPO)
        if not url.startswith(prefix):
            raise binary.DownloadError(f"cannot download {url}: unknown release")
        asset = url[len(prefix):]
        if asset not in self.assets:
            raise binary.DownloadError(f"cannot download {url}: HTTP 404")
        data = self.assets[asset]
        if len(data) > limit:
            raise binary.DownloadError(f"{url} serves more than the {limit} bytes allowed for it")
        return data


@pytest.fixture
def release(monkeypatch):
    """A fake release with this platform's kernel, pinned in the table."""
    fake = FakeRelease()
    asset = binary.release_asset_name()
    digest = fake.publish(asset, KERNEL_BYTES)
    monkeypatch.setattr(binary, "fetch", fake.fetch)
    monkeypatch.setattr(binary, "PINNED_SHA256", {REPO: {RELEASE: {asset: digest}}})
    monkeypatch.setattr(binary, "VERSION", "9.9.9")
    return fake


@pytest.fixture
def local_binary(tmp_path):
    """A kernel binary built by hand."""
    path = tmp_path / "sysml-jupyter-kernel"
    path.write_bytes(KERNEL_BYTES)
    path.chmod(path.stat().st_mode | stat.S_IXUSR)
    return str(path)

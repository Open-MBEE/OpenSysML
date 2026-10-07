"""The `sysml` kernelspec, and installing it where Jupyter looks.

A kernelspec is a directory holding a `kernel.json`; this one also holds the
kernel binary and the logos front ends show for the kernel, so `jupyter kernelspec remove sysml` removes everything the
install wrote. The command line in `kernel.json` names the binary through the
`{resource_dir}` placeholder Jupyter substitutes with the directory the spec
was found in, so the spec can be installed per user, into a prefix, or copied.
"""

from __future__ import annotations

import json
import os
from importlib import resources
import shutil
import stat
import sys
import tempfile
from typing import Any

from ._version import VERSION
from .binary import KernelBinaryError, binary_name, download_binary

KERNEL_NAME = "sysml"
DISPLAY_NAME = "SysML v2 (OpenSysML)"
LANGUAGE = "sysml"
IMPLEMENTATION = "sysml-jupyter-kernel"
PACKAGE = "jupyter-opensysml-kernel"
SPEC_FILE = "kernel.json"
LOGO_FILES = ("logo-32x32.png", "logo-64x64.png")


def kernel_json(binary_file: str, display_name: str = DISPLAY_NAME) -> dict[str, Any]:
    """The `kernel.json` for a kernel binary installed beside it."""
    return {
        "argv": ["{resource_dir}/" + binary_file, "-connection-file", "{connection_file}"],
        "display_name": display_name,
        "language": LANGUAGE,
        "interrupt_mode": "message",
        "metadata": {"implementation": IMPLEMENTATION, "package": PACKAGE, "package_version": VERSION},
    }


def stage_binary(binary: str, staging_dir: str) -> str:
    """Copy a kernel binary supplied by hand into the spec being built.

    Raises:
        KernelBinaryError: If the path is not an executable file
    """
    if not os.path.isfile(binary):
        raise KernelBinaryError(f"{binary} is not a file")
    if sys.platform != "win32" and not os.access(binary, os.X_OK):
        raise KernelBinaryError(f"{binary} is not executable")
    dest = os.path.join(staging_dir, binary_name())
    shutil.copyfile(binary, dest)
    os.chmod(dest, os.stat(dest).st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
    return dest


def write_spec(staging_dir: str, binary_path: str, display_name: str = DISPLAY_NAME) -> str:
    """Write `kernel.json` beside a staged binary."""
    spec = kernel_json(os.path.basename(binary_path), display_name)
    path = os.path.join(staging_dir, SPEC_FILE)
    with open(path, "w", encoding="utf-8") as f:
        json.dump(spec, f, indent=2, sort_keys=True)
        f.write("\n")
    return path


def write_logos(staging_dir: str) -> list[str]:
    """Put the OpenSysML mark beside `kernel.json`, at the sizes front ends read."""
    written = []
    for name in LOGO_FILES:
        path = os.path.join(staging_dir, name)
        with open(path, "wb") as f:
            f.write(resources.files(__package__).joinpath(name).read_bytes())
        written.append(path)
    return written


def default_location() -> dict[str, Any]:
    """Where an install goes when no location is asked for.

    Inside a virtual environment or a conda environment the kernel belongs to
    that environment (`--sys-prefix`); elsewhere it is installed for the user.
    """
    in_env = sys.prefix != getattr(sys, "base_prefix", sys.prefix) or "CONDA_PREFIX" in os.environ
    return {"prefix": sys.prefix} if in_env else {"user": True}


def install(
    binary: str | None = None,
    version: str | None = None,
    user: bool = False,
    prefix: str | None = None,
    name: str = KERNEL_NAME,
    display_name: str = DISPLAY_NAME,
    github_repo: str | None = None,
) -> str:
    """Install the kernelspec, with the kernel binary inside it.

    Args:
        binary: A `sysml-jupyter-kernel` to install instead of downloading one
        version: The release to download; the package's pinned release when omitted
        user: Install for the current user (`~/.local/share/jupyter`, or $JUPYTER_DATA_DIR)
        prefix: Install under `<prefix>/share/jupyter`
        name: The kernelspec's directory name, which notebooks bind to
        display_name: The name front ends list the kernel under
        github_repo: GitHub repository (owner/repo) the release is downloaded from

    Returns:
        The directory the kernelspec was installed into

    Raises:
        KernelBinaryError: If no verified binary could be obtained
    """
    from jupyter_client.kernelspec import KernelSpecManager

    if binary is not None and version is not None:
        raise KernelBinaryError("--binary names the kernel to install; --version chooses one to download")
    if not user and prefix is None:
        location = default_location()
        user = bool(location.get("user"))
        prefix = location.get("prefix")
    with tempfile.TemporaryDirectory(prefix="jupyter-opensysml-kernel-") as staging:
        if binary is not None:
            staged = stage_binary(binary, staging)
        else:
            staged = download_binary(staging, version=version, github_repo=github_repo)
        write_spec(staging, staged, display_name)
        write_logos(staging)
        destination: str = KernelSpecManager().install_kernel_spec(
            staging, kernel_name=name, user=user, prefix=prefix
        )
    return destination


def uninstall(name: str = KERNEL_NAME) -> str:
    """Remove the installed kernelspec and the binary inside it.

    Returns:
        The directory that was removed

    Raises:
        KernelBinaryError: If no kernelspec of that name is installed
    """
    from jupyter_client.kernelspec import KernelSpecManager, NoSuchKernel

    manager = KernelSpecManager()
    try:
        removed: str = manager.remove_kernel_spec(name)
    except (NoSuchKernel, KeyError) as e:
        raise KernelBinaryError(f"no kernelspec named {name!r} is installed") from e
    return removed

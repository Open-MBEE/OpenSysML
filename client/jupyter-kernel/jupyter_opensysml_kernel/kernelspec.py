"""The `sysml` kernelspec, and installing it where Jupyter looks.

A kernelspec is a directory holding a `kernel.json` and, here, the logos front
ends show for the kernel. A platform wheel installs one under
`<prefix>/share/jupyter/kernels/sysml` as package data, whose command line is
`python -m jupyter_opensysml_kernel`, which starts the kernel bundled in the
wheel; `pip install` alone registers the kernel. `install` writes a kernelspec
holding the kernel binary itself, named through the `{resource_dir}`
placeholder Jupyter substitutes with the directory the spec was found in, so
that spec can be installed per user, into a prefix, or copied, and
`jupyter kernelspec remove sysml` removes everything it wrote.
"""

from __future__ import annotations

import json
import os
import re
from importlib import resources
import shutil
import stat
import sys
import tempfile
from typing import Any

from ._version import VERSION
from .binary import BINARY_MODE, KernelBinaryError, binary_name, bundled_binary, download_binary

KERNEL_NAME = "sysml"
DISPLAY_NAME = "SysML v2 (OpenSysML)"
LANGUAGE = "sysml"
IMPLEMENTATION = "sysml-jupyter-kernel"
PACKAGE = "jupyter-opensysml-kernel"
SPEC_FILE = "kernel.json"
# Permissions of kernel.json and the logos: owner writes, everyone reads.
SPEC_MODE = 0o644
LOGO_FILES = ("logo-32x32.png", "logo-64x64.png")

# One directory name under the kernels directory: no path, and no name beginning
# with a dot, `..` above all, which jupyter_client's own check lets through.
KERNEL_NAME_PATTERN = re.compile(r"[a-z0-9][a-z0-9._-]*", re.IGNORECASE)


class InvalidKernelNameError(ValueError):
    """A kernelspec name that is not one directory name."""


def validate_name(name: str) -> str:
    """The kernelspec name, if it is one that stays inside the kernels directory.

    Raises:
        InvalidKernelNameError: For a path, an empty name, or one beginning with a dot
    """
    if KERNEL_NAME_PATTERN.fullmatch(name) is None:
        raise InvalidKernelNameError(
            f"kernelspec name {name!r} must be letters, digits, '.', '_' or '-', "
            "beginning with a letter or digit"
        )
    return name


def _spec(argv: list[str], display_name: str) -> dict[str, Any]:
    return {
        "argv": argv,
        "display_name": display_name,
        "language": LANGUAGE,
        "interrupt_mode": "message",
        "metadata": {"implementation": IMPLEMENTATION, "package": PACKAGE, "package_version": VERSION},
    }


def kernel_json(binary_file: str, display_name: str = DISPLAY_NAME) -> dict[str, Any]:
    """The `kernel.json` for a kernel binary installed beside it."""
    return _spec(["{resource_dir}/" + binary_file, "-connection-file", "{connection_file}"], display_name)


def launcher_kernel_json(display_name: str = DISPLAY_NAME) -> dict[str, Any]:
    """The `kernel.json` a platform wheel installs: the kernel it bundles, started
    through the `python` of the environment Jupyter runs in, as ipykernel's is."""
    return _spec(["python", "-m", __package__, "-connection-file", "{connection_file}"], display_name)


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
    os.chmod(dest, BINARY_MODE)
    return dest


def _write_json(spec: dict[str, Any], staging_dir: str) -> str:
    path = os.path.join(staging_dir, SPEC_FILE)
    with open(path, "w", encoding="utf-8") as f:
        json.dump(spec, f, indent=2, sort_keys=True)
        f.write("\n")
    os.chmod(path, SPEC_MODE)
    return path


def write_spec(staging_dir: str, binary_path: str, display_name: str = DISPLAY_NAME) -> str:
    """Write `kernel.json` beside a staged binary."""
    return _write_json(kernel_json(os.path.basename(binary_path), display_name), staging_dir)


def write_launcher_spec(staging_dir: str) -> list[str]:
    """Write the kernelspec a platform wheel ships as data: `kernel.json` and the logos."""
    return [_write_json(launcher_kernel_json(), staging_dir), *write_logos(staging_dir)]


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

    The binary is the one bundled in this package when it was installed from a
    platform wheel; otherwise it is downloaded and verified.

    Args:
        binary: A `sysml-jupyter-kernel` to install instead of the bundled or downloaded one
        version: The release to download, instead of the bundled kernel; the package's
            pinned release when omitted
        user: Install for the current user (`~/.local/share/jupyter`, or $JUPYTER_DATA_DIR)
        prefix: Install under `<prefix>/share/jupyter`
        name: The kernelspec's directory name, which notebooks bind to
        display_name: The name front ends list the kernel under
        github_repo: GitHub repository (owner/repo) the release is downloaded from

    Returns:
        The directory the kernelspec was installed into

    Raises:
        KernelBinaryError: If no verified binary could be obtained
        InvalidKernelNameError: If name is not one directory name
    """
    from jupyter_client.kernelspec import KernelSpecManager

    validate_name(name)
    if binary is not None and version is not None:
        raise KernelBinaryError("--binary names the kernel to install; --version chooses one to download")
    if binary is None and version is None:
        binary = bundled_binary()
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
        InvalidKernelNameError: If name is not one directory name
    """
    from jupyter_client.kernelspec import KernelSpecManager, NoSuchKernel

    validate_name(name)
    manager = KernelSpecManager()
    try:
        removed: str = manager.remove_kernel_spec(name)
    except (NoSuchKernel, KeyError) as e:
        raise KernelBinaryError(f"no kernelspec named {name!r} is installed") from e
    return removed

"""`python -m jupyter_opensysml_kernel`: start the bundled kernel, or install or remove the kernelspec.

Given the kernel's own flags (`-connection-file FILE`, the form the kernelspec a
platform wheel installs uses), the bundled `sysml-jupyter-kernel` is started
with them; given a subcommand, the kernelspec is managed.
"""

from __future__ import annotations

import argparse
import json
import os
import stat
import subprocess
import sys
from collections.abc import Sequence

from . import kernelspec
from ._version import VERSION
from .binary import BINARY_MODE, KernelBinaryError, binary_name, bundled_binary


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="jupyter-opensysml-kernel",
        description="Install the SysML v2 (OpenSysML) Jupyter kernel.",
        epilog="Kernel flags (-connection-file FILE, -version, ...) start the kernel bundled in this package.",
    )
    parser.add_argument("--version", action="version", version=f"%(prog)s {VERSION}")
    commands = parser.add_subparsers(dest="command", required=True)

    install = commands.add_parser(
        "install",
        help="register the kernelspec, with the bundled kernel binary or a verified download of the release's",
    )
    where = install.add_mutually_exclusive_group()
    where.add_argument("--user", action="store_true", help="install for the current user")
    where.add_argument("--sys-prefix", action="store_true", help="install into the current Python environment")
    where.add_argument("--prefix", metavar="DIR", help="install under DIR/share/jupyter")
    where.add_argument("--system", action="store_true", help="install system-wide")
    install.add_argument("--binary", metavar="PATH", help="install this sysml-jupyter-kernel instead of the bundled or downloaded one")
    install.add_argument("--release", metavar="TAG", help="download the kernel of this release (default: the bundled kernel, or the release this package pins)")
    install.add_argument("--name", default=kernelspec.KERNEL_NAME, help="kernelspec name (default: %(default)s)")
    install.add_argument("--display-name", default=kernelspec.DISPLAY_NAME, help="name shown by front ends (default: %(default)s)")

    uninstall = commands.add_parser("uninstall", help="remove the kernelspec and the kernel binary in it")
    uninstall.add_argument("--name", default=kernelspec.KERNEL_NAME, help="kernelspec name (default: %(default)s)")

    commands.add_parser("kernelspec", help="print the kernel.json an install writes")
    return parser


def is_kernel_invocation(argv: Sequence[str]) -> bool:
    """Whether the arguments are the kernel's (single-dash flags), not a subcommand."""
    return bool(argv) and argv[0].startswith("-") and not argv[0].startswith("--") and argv[0] != "-h"


# The kernel's flags the launcher forwards: the one Jupyter passes, and the
# ones that only print. Those of the native `-install` are not among them; the
# `install` subcommand is the package's way to the same end.
CONNECTION_FILE_FLAG = "-connection-file"
KERNEL_SWITCHES = frozenset({"-verbose", "-version", "-man", "-print-kernelspec"})


class KernelArgumentError(KernelBinaryError):
    """Arguments that are not the kernel's flags, so are not passed to it."""


def kernel_arguments(argv: Sequence[str]) -> list[str]:
    """The kernel's command line for the arguments, each checked against its flag.

    Raises:
        KernelArgumentError: If an argument is not a kernel flag, or names no
            connection file
    """
    command: list[str] = []
    connection_file: str | None = None
    args = iter(argv)
    for arg in args:
        flag, has_value, value = arg.partition("=")
        if flag == CONNECTION_FILE_FLAG:
            if not has_value:
                value = next(args, "")
            if connection_file is not None:
                raise KernelArgumentError(f"kernel flag {flag} is given twice")
            if not value or not os.path.isfile(value):
                raise KernelArgumentError(f"kernel flag {flag} needs the connection file Jupyter wrote, not {value!r}")
            connection_file = value
            command += [flag, value]
        elif flag in KERNEL_SWITCHES and not has_value:
            command.append(flag)
        else:
            raise KernelArgumentError(
                f"{arg!r} is not a kernel flag; the kernel takes {CONNECTION_FILE_FLAG} FILE and "
                + ", ".join(sorted(KERNEL_SWITCHES))
            )
    return command


def launch(argv: Sequence[str]) -> int:
    """Run the bundled kernel with the given arguments, in this process where the OS allows.

    Raises:
        KernelBinaryError: If this install bundles no kernel, or the arguments
            are not its flags
    """
    path = bundled_binary()
    if path is None:
        raise KernelBinaryError(
            f"this install of {kernelspec.PACKAGE} bundles no {binary_name()} (it was not installed "
            "from a platform wheel); `python -m jupyter_opensysml_kernel install` registers a verified one"
        )
    command = [path, *kernel_arguments(argv)]
    if sys.platform == "win32":
        process = subprocess.Popen(command)
        try:
            return process.wait()
        except BaseException:
            process.terminate()
            raise
    if stat.S_IMODE(os.stat(path).st_mode) != BINARY_MODE:
        try:
            os.chmod(path, BINARY_MODE)
        except PermissionError:
            # Another user's install: it is theirs to tighten, as long as it runs.
            if not os.access(path, os.X_OK):
                raise
    os.execv(path, command)
    return 0  # pragma: no cover - execv does not return


def main(argv: Sequence[str] | None = None) -> int:
    if argv is None:
        argv = sys.argv[1:]
    try:
        if is_kernel_invocation(argv):
            return launch(argv)
        args = build_parser().parse_args(argv)
        if args.command == "install":
            prefix = args.prefix
            if args.sys_prefix:
                prefix = sys.prefix
            user = args.user
            if args.system:
                prefix = "/usr/local" if sys.platform != "win32" else sys.prefix
            path = kernelspec.install(
                binary=args.binary,
                version=args.release,
                user=user,
                prefix=prefix,
                name=args.name,
                display_name=args.display_name,
            )
            print(f"Installed kernelspec {args.name} in {path}")
        elif args.command == "uninstall":
            path = kernelspec.uninstall(args.name)
            print(f"Removed kernelspec {args.name} from {path}")
        else:
            json.dump(kernelspec.kernel_json(binary_name()), sys.stdout, indent=2, sort_keys=True)
            print()
    except (KernelBinaryError, kernelspec.InvalidKernelNameError) as e:
        print(f"error: {e}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())

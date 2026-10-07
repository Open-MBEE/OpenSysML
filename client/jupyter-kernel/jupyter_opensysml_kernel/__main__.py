"""`python -m jupyter_opensysml_kernel`: install or remove the SysML kernel."""

from __future__ import annotations

import argparse
import json
import sys
from collections.abc import Sequence

from . import kernelspec
from ._version import VERSION
from .binary import KernelBinaryError, binary_name


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="jupyter-opensysml-kernel",
        description="Install the SysML v2 (OpenSysML) Jupyter kernel.",
    )
    parser.add_argument("--version", action="version", version=f"%(prog)s {VERSION}")
    commands = parser.add_subparsers(dest="command", required=True)

    install = commands.add_parser(
        "install",
        help="download the kernel binary of this package's release, verify it, and register the kernelspec",
    )
    where = install.add_mutually_exclusive_group()
    where.add_argument("--user", action="store_true", help="install for the current user")
    where.add_argument("--sys-prefix", action="store_true", help="install into the current Python environment")
    where.add_argument("--prefix", metavar="DIR", help="install under DIR/share/jupyter")
    where.add_argument("--system", action="store_true", help="install system-wide")
    install.add_argument("--binary", metavar="PATH", help="install this sysml-jupyter-kernel instead of downloading one")
    install.add_argument("--release", metavar="TAG", help="download the kernel of this release (default: the one this package pins)")
    install.add_argument("--name", default=kernelspec.KERNEL_NAME, help="kernelspec name (default: %(default)s)")
    install.add_argument("--display-name", default=kernelspec.DISPLAY_NAME, help="name shown by front ends (default: %(default)s)")

    uninstall = commands.add_parser("uninstall", help="remove the kernelspec and the kernel binary in it")
    uninstall.add_argument("--name", default=kernelspec.KERNEL_NAME, help="kernelspec name (default: %(default)s)")

    commands.add_parser("kernelspec", help="print the kernel.json an install writes")
    return parser


def main(argv: Sequence[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
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
    except KernelBinaryError as e:
        print(f"error: {e}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())

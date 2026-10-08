"""Builds the platform wheels, which carry the kernel and register it as data.

`JUPYTER_OPENSYSML_KERNEL_PLATFORM=<goos>-<goarch>` names the kernel in
`jupyter_opensysml_kernel/bin/` the wheel is built for: the wheel is tagged for
that platform, bundles the binary, and installs the `sysml` kernelspec under
`<prefix>/share/jupyter/kernels` so `pip install` alone registers the kernel.
Unset, the wheel is pure and carries neither, like the sdist: an install from
either registers a verified download with `python -m jupyter_opensysml_kernel
install`.
"""

import os
import shutil
import sys

from setuptools import Distribution, setup
from setuptools.command.bdist_wheel import bdist_wheel
from setuptools.command.sdist import sdist

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)

from jupyter_opensysml_kernel.binary import BUNDLED_DIR, binary_name, wheel_platform_tag
from jupyter_opensysml_kernel.kernelspec import KERNEL_NAME, write_launcher_spec

PLATFORM_ENV = "JUPYTER_OPENSYSML_KERNEL_PLATFORM"
SHARED_DATA = f"share/jupyter/kernels/{KERNEL_NAME}"


def target_platform() -> tuple[str, str] | None:
    value = os.environ.get(PLATFORM_ENV, "")
    if not value:
        return None
    goos, sep, goarch = value.partition("-")
    if not sep or not goarch:
        raise SystemExit(f"{PLATFORM_ENV}={value!r}: expected <goos>-<goarch>, as linux-amd64")
    return goos, goarch


def bundled_files() -> list[str]:
    if not os.path.isdir(BUNDLED_DIR):
        return []
    return sorted(f for f in os.listdir(BUNDLED_DIR) if os.path.isfile(os.path.join(BUNDLED_DIR, f)))


class PlatformDistribution(Distribution):
    """Platform-specific when a kernel is bundled, so the package installs as platlib
    and the wheel is tagged for the machine, though it has no extension modules."""

    def has_ext_modules(self) -> bool:
        return target_platform() is not None or super().has_ext_modules()


class PlatformWheel(bdist_wheel):
    """A wheel tagged for the kernel it bundles, with the kernelspec as shared data."""

    def finalize_options(self) -> None:
        self.target = target_platform()
        if self.target is not None:
            self.plat_name = wheel_platform_tag(*self.target)
            self.plat_name_supplied = True
        super().finalize_options()

    def get_tag(self) -> tuple[str, str, str]:
        if self.target is None:
            return super().get_tag()
        return ("py3", "none", self.plat_name)

    def run(self) -> None:
        self.drop_stale_build_kernel()
        bundled = bundled_files()
        if self.target is None:
            if bundled:
                raise SystemExit(
                    f"{BUNDLED_DIR} holds {', '.join(bundled)} but {PLATFORM_ENV} is unset: "
                    "a pure wheel bundles no kernel"
                )
            super().run()
            return
        expected = binary_name(self.target[0])
        if bundled != [expected]:
            raise SystemExit(
                f"{PLATFORM_ENV}={'-'.join(self.target)} needs exactly {expected} in {BUNDLED_DIR}; "
                f"found {', '.join(bundled) or 'nothing'}"
            )
        spec_dir = os.path.join(HERE, "build", "kernelspec")
        os.makedirs(spec_dir, exist_ok=True)
        self.distribution.data_files = [(SHARED_DATA, write_launcher_spec(spec_dir))]
        super().run()

    def drop_stale_build_kernel(self) -> None:
        """setuptools copies package data into build/lib and keeps what is there, so a
        kernel staged for an earlier platform's wheel would ride along into this one."""
        build_lib = self.get_finalized_command("build").build_lib
        shutil.rmtree(os.path.join(build_lib, "jupyter_opensysml_kernel", "bin"), ignore_errors=True)


class SourceOnly(sdist):
    """The sdist is built before any kernel is staged; it never carries one."""

    def run(self) -> None:
        bundled = bundled_files()
        if bundled:
            raise SystemExit(f"{BUNDLED_DIR} holds {', '.join(bundled)}: an sdist bundles no kernel")
        super().run()


setup(distclass=PlatformDistribution, cmdclass={"bdist_wheel": PlatformWheel, "sdist": SourceOnly})

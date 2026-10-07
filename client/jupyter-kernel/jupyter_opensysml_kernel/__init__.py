"""Jupyter kernel for SysML v2, backed by the OpenSysML REPL.

The kernel itself is the `sysml-jupyter-kernel` binary every OpenSysML release
publishes. This package downloads the build for this machine, verifies it
against the digest pinned here for that release, and registers it as the
`sysml` kernelspec::

    python -m jupyter_opensysml_kernel install
"""

from ._version import VERSION as __version__

__all__ = ["__version__"]

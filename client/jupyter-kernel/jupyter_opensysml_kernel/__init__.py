"""Jupyter kernel for SysML v2, backed by the OpenSysML REPL.

The kernel itself is the `sysml-jupyter-kernel` binary every OpenSysML release
publishes. A platform wheel of this package bundles the build for its machine
and registers it as the `sysml` kernelspec when it is installed. An install
from the sdist, or on another platform, downloads the build for the machine,
verifies it against the digest pinned here for that release, and registers it::

    python -m jupyter_opensysml_kernel install
"""

from ._version import VERSION as __version__

__all__ = ["__version__"]

"""Jupyter kernel for SysML v2, backed by the OpenSysML REPL.

The kernel itself is the `sysml-jupyter-kernel` binary every OpenSysML release
publishes. A platform wheel of this package bundles the build for its machine
and registers it as the `sysml` kernelspec when it is installed. An install
from the sdist, or on another platform, downloads the build for the machine,
verifies it against the digest pinned here for that release, and registers it::

    python -m jupyter_opensysml_kernel install

Every distribution also installs the prebuilt JupyterLab extension that
highlights SysML v2 in notebook cells, under ``share/jupyter/labextensions``.
"""

from ._version import VERSION as __version__
from .labextension import labextension_paths as _jupyter_labextension_paths

__all__ = ["__version__", "_jupyter_labextension_paths"]

"""The prebuilt JupyterLab extension that highlights SysML v2 and KerML.

``editors/jupyterlab`` builds it (``make jupyterlab-build``) into the
``labextension/`` directory beside ``setup.py``; every distribution of the
package then installs it as shared data under
``share/jupyter/labextensions/jupyterlab-opensysml``, where JupyterLab 4 and
Notebook 7 load federated extensions from, so ``pip install`` alone enables it.
"""

import os
import re

LABEXTENSION_NAME = "jupyterlab-opensysml"
LABEXTENSION_DIR = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "labextension")
SHARED_DATA = f"share/jupyter/labextensions/{LABEXTENSION_NAME}"

# What a federated extension needs to load: its manifest and the webpack entry
# the manifest's `_build.load` names.
MANIFEST = "package.json"
REMOTE_ENTRY = re.compile(r"^remoteEntry\.[0-9a-f]+\.js$")


def labextension_paths() -> list[dict[str, str]]:
    """The extension `jupyter labextension develop` links, in the shape it reads:
    `src` is relative to this package."""
    return [{"src": "../labextension", "dest": LABEXTENSION_NAME}]


def is_built(root: str = LABEXTENSION_DIR) -> bool:
    """Whether a build of the extension is staged at `root`."""
    static = os.path.join(root, "static")
    if not os.path.isfile(os.path.join(root, MANIFEST)) or not os.path.isdir(static):
        return False
    return any(REMOTE_ENTRY.match(name) for name in os.listdir(static))


def data_files(root: str = LABEXTENSION_DIR) -> list[tuple[str, list[str]]]:
    """setup.py's `data_files` entries for every file of the build at `root`:
    paths relative to the package source, grouped by their share/ directory."""
    entries: list[tuple[str, list[str]]] = []
    source = os.path.dirname(root)
    for directory, subdirectories, files in os.walk(root):
        subdirectories.sort()
        if not files:
            continue
        relative = os.path.relpath(directory, root)
        target = SHARED_DATA if relative == os.curdir else f"{SHARED_DATA}/{relative.replace(os.sep, '/')}"
        entries.append((target, [os.path.relpath(os.path.join(directory, f), source) for f in sorted(files)]))
    return entries

"""Publish the install scripts at the site root.

install.sh and install.ps1 live at the repository root, where a checkout runs them;
the site serves the same files at /install.sh and /install.ps1, which the stubs at
https://opensysml.org/install.sh and https://opensysml.org/install.ps1 (the addresses
the install guide's one-liners fetch, published from Open-MBEE/opensysml.github.io)
hand off to. One copy, so the published script cannot drift from the committed one.
"""

from pathlib import Path

from mkdocs.structure.files import File

SCRIPTS = ("install.sh", "install.ps1")


def on_files(files, config):
    root = Path(config.config_file_path).resolve().parent
    for script in install_scripts(root):
        files.append(File(script.name, str(root), config.site_dir, config.use_directory_urls))
    return files


def install_scripts(root: Path):
    """The install scripts under the repository root, each of which must exist."""
    scripts = [root / name for name in SCRIPTS]
    missing = [script.name for script in scripts if not script.is_file()]
    if missing:
        raise FileNotFoundError(f"install scripts missing from {root}: {', '.join(missing)}")
    return scripts

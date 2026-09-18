"""Where the installed opensysml distribution lives, and how it was installed.

An editable install keeps its dist-info in site-packages while its modules stay
in the checkout, so `Distribution.locate_file` answers with a site-packages path
that holds no `opensysml/` at all. Resolution therefore reads the PEP 610 record
the installer wrote, which names the directory the install was made from.
"""

import json
import os
from importlib.metadata import PackageNotFoundError, distribution
from urllib.parse import urlparse
from urllib.request import url2pathname

#: Layouts a project directory may hold its packages in.
_PACKAGE_ROOTS = ('', 'src')


def installed_distribution():
    """The installed opensysml distribution.

    Returns:
        importlib.metadata.Distribution or None: None when the source tree is
            not installed at all
    """
    try:
        return distribution('opensysml')
    except PackageNotFoundError:
        return None


def editable_install(dist):
    """Whether a distribution's modules are a checkout rather than copies of one.

    Args:
        dist (importlib.metadata.Distribution): The installed distribution

    Returns:
        bool: True for a PEP 660 editable install
    """
    return _direct_url_info(dist).get('dir_info', {}).get('editable') is True


def project_directory(dist):
    """The directory an editable install was made from.

    Args:
        dist (importlib.metadata.Distribution): The installed distribution

    Returns:
        str or None: The directory, or None when the install records no local
            one (a wheel from an index records no directory)
    """
    url = _direct_url_info(dist).get('url', '')
    if not url.startswith('file:'):
        return None
    return os.path.realpath(url2pathname(urlparse(url).path))


def package_location(dist):
    """Path of the ``opensysml/__init__.py`` a distribution installed.

    Args:
        dist (importlib.metadata.Distribution): The installed distribution

    Returns:
        str: The path, resolved through the install's own record for an editable
            install and through the dist-info's directory otherwise. The path is
            not guaranteed to exist: a distribution whose files are gone is
            reported as it stands rather than guessed at.
    """
    project = project_directory(dist) if editable_install(dist) else None
    if project is not None:
        for root in _PACKAGE_ROOTS:
            candidate = os.path.join(project, root, 'opensysml', '__init__.py')
            if os.path.isfile(candidate):
                return os.path.realpath(candidate)
    return os.path.realpath(str(dist.locate_file('opensysml/__init__.py')))


def _direct_url_info(dist):
    """The PEP 610 record of how a distribution was installed, as a dict."""
    recorded = dist.read_text('direct_url.json')
    if not recorded:
        return {}
    try:
        info = json.loads(recorded)
    except json.JSONDecodeError:
        return {}
    return info if isinstance(info, dict) else {}

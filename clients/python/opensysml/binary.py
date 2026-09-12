"""Binary management for sysml-grpc service."""

import contextlib
import hashlib
import json
import os
import platform
import shutil
import stat
import threading
import warnings
from opensysml.errors import (
    ChecksumMismatchError,
    ConnectionError,
    UnpinnedReleaseError,
    UnsignedReleaseError,
)
from opensysml.signing import (
    BUNDLE_ASSET,
    MANIFEST_ASSET,
    signer_for,
    verified_manifest_digest,
)

# Releases publish sysml-grpc-<goos>-<goarch> raw, with a .sha256 sidecar.
# OPENSYSML_GITHUB_REPO overrides the repository they are fetched from.
DEFAULT_GITHUB_REPO = 'Open-MBEE/OpenSysML'

# A cached binary is now checked against the releases API before being reused, and
# that happens while the service-start lock is held, so it must not hang there.
NETWORK_TIMEOUT = 15

#: A release binary is tens of megabytes, so anything this large is not one.
MAX_BINARY_BYTES = 512 * 1024 * 1024

#: Checksums, manifests, signature bundles and release JSON are kilobytes.
MAX_METADATA_BYTES = 8 * 1024 * 1024


def read_bounded(response, url, limit):
    """Read a response, refusing one too large to be what was asked for.

    The cache lock is held over a download, so an endless body would otherwise
    hold every other client out while filling this process's memory.

    Args:
        response: An open HTTP response
        url (str): What is being read, for the refusal
        limit (int): Most bytes the body may be

    Returns:
        bytes: The body

    Raises:
        ConnectionError: If the body is longer than the limit
    """
    body = response.read(limit + 1)
    if len(body) > limit:
        raise ConnectionError(f"{url} is larger than {limit} bytes")
    return body


def fetch(url, limit):
    """GET a URL and read its body, bounded.

    Only a download needs the HTTP stack, so it is imported here, not with the module.

    Args:
        url (str): What to fetch
        limit (int): Most bytes the body may be

    Returns:
        bytes: The body

    Raises:
        OSError: If the request or the read fails, however the HTTP stack
            reports it
        ConnectionError: If the body is longer than the limit
    """
    import http.client
    import urllib.request

    try:
        with urllib.request.urlopen(url, timeout=NETWORK_TIMEOUT) as response:
            return read_bounded(response, url, limit)
    # urlopen leaves read-phase failures unwrapped, so a truncated body or a
    # dropped connection is an HTTPException, not a URLError.
    except http.client.HTTPException as e:
        raise OSError(str(e)) from e


#: The pinned digests, shipped beside this module as package data.
PINNED_DIGESTS_FILE = os.path.join(
    os.path.dirname(os.path.abspath(__file__)), 'release-digests.json'
)


def _load_pinned_digests():
    """The pinned digest table this distribution ships.

    Returns:
        dict: repo -> release tag -> asset name -> SHA-256 hex digest
    """
    with open(PINNED_DIGESTS_FILE, encoding='utf-8') as f:
        return json.load(f)


#: SHA-256 digest expected of each release asset, keyed by repository, release tag
#: and asset name: independent of the origin serving the download, so a republished
#: release is refused. Synced from clients/release-digests.json; see "Pinned release
#: digests" in clients/python/README.md.
PINNED_SHA256 = _load_pinned_digests()

#: Set to the repository whose unpinned downloads may be accepted (`1` for any),
#: which is same-origin trust: the checksum then comes from whoever served the
#: binary.
ALLOW_UNPINNED_ENV = 'OPENSYSML_ALLOW_UNPINNED_DOWNLOAD'

#: Names a build to start in place of the cache, a download or one on $PATH,
#: under the name the Node client reads for the same purpose.
BINARY_ENV = 'OPENSYSML_BINARY'


def default_github_repo():
    """Repository releases are downloaded from.

    Returns:
        str: owner/repo, from $OPENSYSML_GITHUB_REPO or the default
    """
    return os.environ.get('OPENSYSML_GITHUB_REPO') or DEFAULT_GITHUB_REPO


def pinned_digest(version, asset, github_repo=None):
    """The digest this release of opensysml pins for a release asset.

    Args:
        version (str): Release tag, resolved (never 'latest')
        asset (str): Asset name (e.g. 'sysml-grpc-linux-amd64')
        github_repo (str, optional): GitHub repository (owner/repo)

    Returns:
        str or None: SHA-256 hex digest, or None when nothing is pinned for it
    """
    repo = github_repo or default_github_repo()
    return PINNED_SHA256.get(repo, {}).get(version, {}).get(asset)


def unpinned_downloads_allowed(github_repo=None):
    """Whether an unpinned download from a repository may fall back to same-origin trust.

    Args:
        github_repo (str, optional): GitHub repository (owner/repo)

    Returns:
        bool: True when the variable is set to '1' (any repository) or names this one
    """
    allowed = os.environ.get(ALLOW_UNPINNED_ENV, '').strip()
    if allowed.lower() in ('', '0', 'false', 'no'):
        return False
    if allowed.lower() in ('1', 'true', 'yes'):
        return True
    repo = github_repo or default_github_repo()
    # Naming repositories keeps the trust with the fork it is granted for.
    return repo in [named.strip() for named in allowed.split(',')]


def expected_digest(version, asset, served_digest, github_repo=None,
                    verified_digest=None, unverified_reason=None):
    """The digest a download must have, from the pin or the signed manifest.

    The pin wins wherever there is one, and a verified manifest contradicting it
    is a failure rather than a reason to prefer either. Where there is no pin, a
    digest taken from a manifest signed by the release pipeline stands in for
    one; nothing else does.

    Args:
        version (str): Release tag, resolved (never 'latest')
        asset (str): Asset name (e.g. 'sysml-grpc-linux-amd64')
        served_digest (str): Digest from the .sha256 served beside the binary
        github_repo (str, optional): GitHub repository (owner/repo)
        verified_digest (str, optional): Digest from the release's signed
            checksum manifest, once its signature verified
        unverified_reason (str, optional): Why there is no verified digest, said
            in the refusal when there is no pin either

    Returns:
        str: The digest to verify the download against

    Raises:
        ChecksumMismatchError: If the served digest contradicts the pinned one,
            which is a release republished with another binary, or if a verified
            manifest contradicts the pin
        UnpinnedReleaseError: If nothing is pinned for it, no signed manifest
            vouched for it, and same-origin trust was not allowed explicitly
    """
    repo = github_repo or default_github_repo()
    pinned = pinned_digest(version, asset, repo)
    if pinned is None:
        if verified_digest is not None:
            # Signed by the pipeline that built the release, so it is not the
            # origin vouching for itself: as good as a pin.
            return verified_digest
        if not unpinned_downloads_allowed(repo):
            unverified = f" and {unverified_reason}" if unverified_reason else ""
            raise UnpinnedReleaseError(
                f"opensysml pins no SHA-256 digest for {asset} of {version} of {repo}"
                f"{unverified}, so the only checksum available is the one served beside "
                f"the binary, which a compromised release would serve too. Upgrade "
                f"opensysml to a release that pins {version}, ask for a pinned release "
                f"with version=, or accept same-origin trust for this repository by "
                f"setting ${ALLOW_UNPINNED_ENV}={repo} (or =1 for any repository)."
            )
        warnings.warn(
            f"opensysml pins no digest for {asset} of {version} of {repo}; verifying it "
            f"against the checksum served beside it, which detects corruption but not a "
            f"compromised release (${ALLOW_UNPINNED_ENV} is set).",
            RuntimeWarning,
            stacklevel=3,
        )
        return served_digest
    if verified_digest is not None and verified_digest != pinned:
        raise ChecksumMismatchError(
            f"Checksum mismatch for {asset} of {version}: the signed {MANIFEST_ASSET} "
            f"of {repo} lists {verified_digest}, but opensysml pins {pinned}. The "
            f"release was rebuilt after this opensysml pinned it; it was not installed."
        )
    if served_digest != pinned:
        raise ChecksumMismatchError(
            f"Checksum mismatch for {asset} of {version}: {repo} serves {served_digest}, "
            f"but opensysml pins {pinned}. The release was republished with another binary, "
            f"or the download is being tampered with; it was not installed."
        )
    return pinned


def release_download_url(version, asset, github_repo=None):
    """URL a release publishes an asset at.

    Args:
        version (str): Release tag, resolved (never 'latest')
        asset (str): Asset name (e.g. 'SHA256SUMS.txt')
        github_repo (str, optional): GitHub repository (owner/repo)

    Returns:
        str: Download URL
    """
    repo = github_repo or default_github_repo()
    return f'https://github.com/{repo}/releases/download/{version}/{asset}'


def signed_manifest_digest(version, asset, github_repo=None):
    """The digest for an asset from the release's signed checksum manifest.

    Args:
        version (str): Release tag, resolved (never 'latest')
        asset (str): Asset name (e.g. 'sysml-grpc-linux-amd64')
        github_repo (str, optional): GitHub repository (owner/repo)

    Returns:
        str: SHA-256 hex digest, from a manifest the release pipeline signed

    Raises:
        UnsignedReleaseError: If the release publishes no signature this client
            can check, or nothing could be verified
        ManifestSignatureError: If the signature does not verify
    """
    repo = github_repo or default_github_repo()
    signer = signer_for(repo)
    if signer is None:
        raise UnsignedReleaseError(
            f"opensysml knows no release pipeline identity for {repo}, so a signed "
            f"{MANIFEST_ASSET} of it would not be verifiable"
        )

    downloaded = {}
    for name in (MANIFEST_ASSET, BUNDLE_ASSET):
        url = release_download_url(version, name, repo)
        try:
            downloaded[name] = fetch(url, MAX_METADATA_BYTES)
        except OSError as e:
            raise UnsignedReleaseError(
                f"{version} of {repo} publishes no readable {name} ({url}: {e}), so "
                f"its checksums carry no signature to verify"
            )

    return verified_manifest_digest(
        downloaded[MANIFEST_ASSET], downloaded[BUNDLE_ASSET], asset, signer
    )


def resolve_latest_version(github_repo=None):
    """Resolve the tag of the repository's latest published release.

    Args:
        github_repo (str, optional): GitHub repository (owner/repo)

    Returns:
        str: Release tag (e.g. 'v0.0.4')

    Raises:
        ConnectionError: If the release cannot be queried or carries no tag
    """
    repo = github_repo or default_github_repo()
    url = f'https://api.github.com/repos/{repo}/releases/latest'
    try:
        release = json.loads(fetch(url, MAX_METADATA_BYTES).decode('utf-8'))
    except (OSError, ValueError) as e:
        raise ConnectionError(f"Failed to resolve latest release from {url}: {e}")

    tag = release.get('tag_name')
    if not tag:
        raise ConnectionError(f"Latest release of {repo} has no tag name")
    return tag


def detect_platform():
    """Detect current platform and architecture.
    
    Returns:
        tuple: (os_name, arch) where os_name in ('linux', 'darwin', 'windows')
               and arch in ('amd64', 'arm64')
    """
    # Map Python platform names to Go GOOS
    system = platform.system().lower()
    if system == 'linux':
        os_name = 'linux'
    elif system == 'darwin':
        os_name = 'darwin'
    elif system == 'windows':
        os_name = 'windows'
    else:
        raise ConnectionError(f"Unsupported operating system: {system}")
    
    # Map Python machine architecture to Go GOARCH
    machine = platform.machine().lower()
    if machine in ('x86_64', 'amd64'):
        arch = 'amd64'
    elif machine in ('aarch64', 'arm64'):
        arch = 'arm64'
    else:
        raise ConnectionError(f"Unsupported architecture: {machine}")
    
    return os_name, arch


def release_asset_name(goos=None, goarch=None):
    """Name a release publishes the service binary for a platform under.

    Args:
        goos (str, optional): GOOS, detected when omitted
        goarch (str, optional): GOARCH, detected when omitted

    Returns:
        str: Asset name (e.g. 'sysml-grpc-linux-amd64')
    """
    if goos is None or goarch is None:
        detected_os, detected_arch = detect_platform()
        goos = goos or detected_os
        goarch = goarch or detected_arch
    name = f'sysml-grpc-{goos}-{goarch}'
    return name + '.exe' if goos == 'windows' else name


def service_binary_name(goos=None):
    """Name the service binary is installed under, on the cache and on $PATH.

    Only the operating system names it, so this answers on a platform no release
    is published for, where a local build is the only sysml-grpc there is.

    Args:
        goos (str, optional): GOOS, detected when omitted

    Returns:
        str: 'sysml-grpc', or 'sysml-grpc.exe' on Windows
    """
    if goos is None:
        goos = platform.system().lower()
    return 'sysml-grpc.exe' if goos == 'windows' else 'sysml-grpc'


def get_binary_path():
    """Get the local path where sysml-grpc binary should be stored.
    
    Returns:
        str: Absolute path to binary (e.g. ~/.opensysml/bin/sysml-grpc)
    """
    base_dir = os.path.expanduser('~/.opensysml/bin')
    return os.path.join(base_dir, service_binary_name())


def is_executable(path):
    """Whether a path is a file this process may execute.

    Args:
        path (str): Path to a candidate binary

    Returns:
        bool: True when it is a regular file with the execute bit for this user
    """
    return os.path.isfile(path) and os.access(path, os.X_OK)


def named_binary():
    """The build $OPENSYSML_BINARY names, used as it is and verified against nothing.

    There is no release to pin such a binary to, so it is neither copied into the
    shared cache nor checked against the pinned digests: naming it is trusting it.

    Returns:
        str or None: The path named, exactly as named, or None when the
            variable is unset or empty

    Raises:
        ConnectionError: If it names something that is not an executable file,
            since an explicit instruction that cannot be honoured is an error
            rather than a reason to look elsewhere
    """
    named = os.environ.get(BINARY_ENV) or ''
    if named == '':
        return None
    if not is_executable(named):
        raise ConnectionError(
            f"${BINARY_ENV} names {named}, which is not an executable file. Point it "
            f"at a sysml-grpc build, or unset it to resolve one from "
            f"{get_binary_path()} or $PATH."
        )
    return named


def binary_on_path():
    """The first executable sysml-grpc on $PATH, used as it is and unverified.

    A binary an operator installed is of no known release, so it is neither
    copied into the shared cache nor checked against the pinned digests.

    Returns:
        str or None: Path to the binary, or None when $PATH holds none
    """
    name = service_binary_name()
    for directory in (os.environ.get('PATH') or '').split(os.pathsep):
        if not directory:
            continue
        candidate = os.path.join(directory, name)
        if is_executable(candidate):
            return candidate
    return None


def lock_path():
    """Path of the advisory lock every client holds while replacing the cache.

    Returns:
        str: Absolute path to the lock file (e.g. ~/.opensysml/bin/sysml-grpc.lock)
    """
    return get_binary_path() + '.lock'


#: One lock per process, because a file lock is held by the process, not the thread.
_CACHE_LOCK = threading.RLock()

#: Nesting of the thread inside _CACHE_LOCK, which only one thread is ever in.
_CACHE_LOCK_DEPTH = 0


@contextlib.contextmanager
def cache_lock():
    """Hold the shared cache against other threads, processes and clients.

    The Java client locks the same file over the same span, so the two never pair
    one release's bytes with another's metadata.

    Yields:
        None: With the lock held for the body
    """
    global _CACHE_LOCK_DEPTH
    with _CACHE_LOCK:
        # Closing any descriptor for a file drops this process's fcntl lock on it,
        # so a nested call must not open one of its own.
        held = _CACHE_LOCK_DEPTH
        _CACHE_LOCK_DEPTH = held + 1
        try:
            if held:
                yield
                return
            yield from _locked_once()
        finally:
            _CACHE_LOCK_DEPTH = held


def _reset_cache_lock():
    """Start a child of fork() unlocked, whoever held the lock in the parent."""
    global _CACHE_LOCK, _CACHE_LOCK_DEPTH
    _CACHE_LOCK = threading.RLock()
    _CACHE_LOCK_DEPTH = 0


if hasattr(os, 'register_at_fork'):
    # A thread holding the lock does not exist in the child, so its lock must not
    # either, or the child deadlocks the first time it resolves a binary.
    os.register_at_fork(after_in_child=_reset_cache_lock)


def _locked_once():
    """The lock file, held for one yield."""
    path = lock_path()
    try:
        os.makedirs(os.path.dirname(path), exist_ok=True)
        fd = os.open(path, os.O_RDWR | os.O_CREAT, 0o600)
    except OSError as e:
        warnings.warn(f"Could not lock {path} against other processes: {e}", stacklevel=3)
        yield
        return
    try:
        with _file_locked(fd, path):
            yield
    finally:
        os.close(fd)


@contextlib.contextmanager
def _file_locked(fd, path):
    """An exclusive lock on an open file, in whatever way the platform has one."""
    locked = False
    try:
        if os.name == 'nt':
            import msvcrt

            msvcrt.locking(fd, msvcrt.LK_LOCK, 1)
        else:
            import fcntl

            # lockf is fcntl(F_SETLKW), the same lock a Java FileLock takes.
            fcntl.lockf(fd, fcntl.LOCK_EX)
        locked = True
    except OSError as e:
        warnings.warn(f"Could not lock {path} against other processes: {e}", stacklevel=4)
    try:
        yield
    finally:
        if locked:
            try:
                if os.name == 'nt':
                    os.lseek(fd, 0, os.SEEK_SET)
                    import msvcrt

                    msvcrt.locking(fd, msvcrt.LK_UNLCK, 1)
                else:
                    import fcntl

                    fcntl.lockf(fd, fcntl.LOCK_UN)
            except OSError:
                pass


def metadata_path():
    """Path of the record of which release the cached binary was downloaded from.

    Returns:
        str: Absolute path to the sidecar (e.g. ~/.opensysml/bin/sysml-grpc.json)
    """
    return get_binary_path() + '.json'


def read_metadata():
    """Read the record beside the cached binary.

    Returns:
        dict: What was recorded, or empty when nothing readable was
    """
    try:
        with open(metadata_path(), 'r') as f:
            recorded = json.load(f)
    except (OSError, ValueError):
        return {}
    return recorded if isinstance(recorded, dict) else {}


def write_metadata(version, sha256, github_repo=None):
    """Record which release of which repository the cached binary is, and its digest.

    Args:
        version (str): Release tag downloaded, resolved (never 'latest')
        sha256 (str): SHA-256 hex digest of the binary written
        github_repo (str, optional): GitHub repository (owner/repo) downloaded from
    """
    path = metadata_path()
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, 'w') as f:
        json.dump({
            'version': version,
            'sha256': sha256,
            'repo': github_repo or default_github_repo(),
        }, f)


def cached_release(github_repo=None):
    """Release tag the cached binary was downloaded from, if from github_repo.

    The digest is re-checked, so a binary swapped in by hand is not read as the
    release it displaced. Forks publish the same tags, so a cache from another
    repository does not answer for this one either.

    Args:
        github_repo (str, optional): GitHub repository (owner/repo) asked about

    Returns:
        str or None: Release tag, or None when the cache cannot be vouched for
    """
    recorded = read_metadata()
    version, digest = recorded.get('version'), recorded.get('sha256')
    if not version or not digest:
        return None
    # A record without a repository predates one being kept, so it says nothing.
    if recorded.get('repo') != (github_repo or default_github_repo()):
        return None
    try:
        if not verify_checksum(get_binary_path(), digest):
            return None
    except OSError:
        return None
    return version


def stale_cache_reason(version, github_repo=None):
    """Why the cached binary is not the release that was asked for.

    A cache left by an earlier release otherwise serves a client asking for a
    newer one, which then fails on whatever that build cannot do.

    Args:
        version (str or None): Release tag asked for, 'latest', or None when
            nothing was asked for, which any cached binary answers
        github_repo (str, optional): GitHub repository (owner/repo)

    Returns:
        str or None: Why the cache does not answer for version, or None when it does
    """
    if version is None:
        return None
    repo = github_repo or default_github_repo()
    if version == 'latest':
        try:
            version = resolve_latest_version(repo)
        except ConnectionError:
            # Unreachable releases are no reason to discard a working cache.
            return None

    have = cached_release(repo)
    if have == version:
        return None
    if have is None:
        recorded_repo = read_metadata().get('repo')
        if recorded_repo and recorded_repo != repo:
            return (
                f"the binary cached at {get_binary_path()} was downloaded from "
                f"{recorded_repo}, but {version} of {repo} was asked for"
            )
        return (
            f"the binary cached at {get_binary_path()} was not downloaded by this "
            f"client, so which release it is cannot be told, and {version} was asked for"
        )
    return f"the binary cached at {get_binary_path()} is {have}, but {version} was asked for"


def download_binary(version='latest', github_repo=None):
    """Download sysml-grpc binary from GitHub releases with checksum verification.
    
    Args:
        version (str): Release version tag (e.g., 'v0.1.0'), or 'latest'
        github_repo (str, optional): GitHub repository (owner/repo)
    
    Returns:
        str: Path to downloaded binary
    
    Raises:
        ChecksumMismatchError: If the download does not match the digest expected
            for it, the release serves a digest contradicting the pin, or the
            signature on its checksum manifest does not verify
        UnpinnedReleaseError: If nothing is pinned for the release and its
            checksum manifest carries no signature that could be verified
        ConnectionError: If the download or its installation fails
    """
    with cache_lock():
        return _download_binary_locked(version, github_repo)


def _download_binary_locked(version, github_repo):
    """download_binary, with the shared cache held over binary and metadata."""
    github_repo = github_repo or default_github_repo()
    if version == 'latest':
        version = resolve_latest_version(github_repo)

    binary_name = release_asset_name()

    # Construct URLs
    binary_url = release_download_url(version, binary_name, github_repo)
    checksum_url = release_download_url(
        version, binary_name + '.sha256', github_repo
    )
    
    binary_path = get_binary_path()
    os.makedirs(os.path.dirname(binary_path), exist_ok=True)
    
    try:
        # Download checksum file first
        checksum_content = fetch(checksum_url, MAX_METADATA_BYTES).decode('utf-8')
        
        # Parse checksum (format: "hexdigest  filename\n")
        served_checksum = checksum_content.split()[0]
        # A release nothing is pinned for is vouched for by the signature on its
        # checksum manifest instead, which the origin cannot forge.
        verified_checksum = unverified_reason = None
        if pinned_digest(version, binary_name, github_repo) is None:
            try:
                verified_checksum = signed_manifest_digest(
                    version, binary_name, github_repo
                )
            except UnsignedReleaseError as e:
                unverified_reason = str(e)
        # The pinned digest wins, so a release republished with another binary
        # cannot vouch for itself through the sidecar it serves.
        expected_checksum = expected_digest(
            version, binary_name, served_checksum, github_repo,
            verified_digest=verified_checksum,
            unverified_reason=unverified_reason,
        )
        
        # Download binary
        binary_data = fetch(binary_url, MAX_BINARY_BYTES)
        
        # Write to temporary file first
        temp_path = binary_path + '.tmp'
        with open(temp_path, 'wb') as f:
            f.write(binary_data)
        
        # Verify checksum
        if not verify_checksum(temp_path, expected_checksum):
            os.remove(temp_path)
            raise ChecksumMismatchError(
                f"Checksum mismatch for {binary_name}. "
                f"Expected {expected_checksum}, but download does not match. "
                f"Binary may be corrupted or tampered with."
            )
        
        # Checksum valid - move to final location. os.replace overwrites a cache
        # being replaced, which os.rename refuses to do on Windows.
        try:
            os.replace(temp_path, binary_path)
        except OSError as e:
            os.remove(temp_path)
            raise ConnectionError(
                f"Downloaded {version} but could not install it at {binary_path}: {e}. "
                f"A running service holding that file is the usual cause."
            )
        
        # Make executable. The cache is this user's, so no one else needs it.
        os.chmod(binary_path, 0o700)
        
        # Record the release, so a later run can tell what the cache holds.
        write_metadata(version, expected_checksum, github_repo)
        
        return binary_path
        
    except ConnectionError:
        raise
    except OSError as e:
        raise ConnectionError(f"Failed to download binary from {binary_url}: {e}")


def file_digest(binary_path):
    """SHA-256 hex digest of a file, read in chunks rather than into memory.

    Args:
        binary_path (str): Path to the file

    Returns:
        str: Hex digest
    """
    sha256 = hashlib.sha256()
    with open(binary_path, 'rb') as f:
        while True:
            chunk = f.read(8192)
            if not chunk:
                break
            sha256.update(chunk)
    return sha256.hexdigest()


def verify_checksum(binary_path, expected_sha256):
    """Verify SHA-256 checksum of binary file.
    
    Args:
        binary_path (str): Path to binary file
        expected_sha256 (str): Expected SHA-256 hex digest
    
    Returns:
        bool: True if checksum matches, False otherwise
    """
    return file_digest(binary_path) == expected_sha256


def stable_binary_path(digest):
    """Path the cached binary is linked to under its own digest.

    The Java client names this file the same way, so both start the same file.

    Args:
        digest (str): SHA-256 hex digest of the cached binary

    Returns:
        str: Absolute path (e.g. ~/.opensysml/bin/sysml-grpc-0123456789abcdef)
    """
    binary_path = get_binary_path()
    root, extension = (
        (binary_path[: -len('.exe')], '.exe')
        if binary_path.endswith('.exe')
        else (binary_path, '')
    )
    return f'{root}-{digest[:16]}{extension}'


def stable_binary():
    """Link the cached binary under its own digest and return that path.

    The cache is one path every client installs over, so a service started from
    it can be running a release other than the one that was verified. This must
    be called with the cache lock held, so nothing replaces the cache between
    hashing it and linking it.

    Returns:
        str: The digest-named path, or the cache itself when it cannot be linked
    """
    binary_path = get_binary_path()
    try:
        stable = stable_binary_path(file_digest(binary_path))
    except OSError as e:
        warnings.warn(
            f"Could not hash {binary_path}, so a release installed over it may be "
            f"started: {e}",
            stacklevel=3,
        )
        return binary_path
    if os.access(stable, os.X_OK):
        return stable
    try:
        _link(binary_path, stable)
        os.chmod(stable, 0o700)
        return stable
    except OSError as e:
        warnings.warn(
            f"Could not link {binary_path} to {stable}, so a release installed over "
            f"it may be started: {e}",
            stacklevel=3,
        )
        _remove_quietly(stable)
        return binary_path


def _link(source, target):
    """Hard link source to target where the filesystem has links, else copy it."""
    temp_path = target + '.tmp'
    _remove_quietly(temp_path)
    try:
        os.link(source, temp_path)
    except (OSError, AttributeError, NotImplementedError):
        shutil.copyfile(source, temp_path)
    try:
        os.replace(temp_path, target)
    except OSError:
        _remove_quietly(temp_path)
        raise


def _remove_quietly(path):
    """Remove a path, if it is there and removable."""
    try:
        os.remove(path)
    except OSError:
        pass


def ensure_binary(force_download=False, version=None, github_repo=None):
    """Ensure sysml-grpc binary is available, downloading if necessary.

    Resolution is the order every client shares: $OPENSYSML_BINARY, then the
    shared cache at ~/.opensysml/bin, then a release download when one is asked
    for, then a sysml-grpc on $PATH.

    A cached binary is reused only when it is the release asked for; when no
    version is asked for, whatever is cached stands, locally built included. A
    replacement that cannot be downloaded leaves the working cache in place, and
    a download that was asked for and failed is an error rather than a fall
    through to $PATH, whose binary is of no known release. The whole cache
    decision is made holding the shared cache lock, so a concurrent installer -
    in another process or in the Java client - cannot install between the check
    and the replacement.

    What is returned for the shared cache is a link to it under its own digest,
    not the cache path itself: the cache is replaced in place, so starting it
    after the lock is dropped could start whatever was installed since. A binary
    from $OPENSYSML_BINARY or $PATH is returned at its own path instead, and is
    used as it is found: it belongs to no release, so it is neither copied into
    the cache nor checked against the pinned digests.
    
    Args:
        force_download (bool): If True, download even if binary exists
        version (str, optional): Specific version tag to download (e.g. 'v0.1.0'),
                                 or 'latest' for the newest release. If None,
                                 $OPENSYSML_GRPC_VERSION is used; without it
                                 auto-download is disabled and the binary must be
                                 pre-installed via `make build`, named by
                                 $OPENSYSML_BINARY, or found on $PATH.
    
    Returns:
        str: Path to binary, digest-named when it is the shared cache
    
    Raises:
        ConnectionError: If binary cannot be obtained
    """
    binary_path = get_binary_path()
    if version is None:
        version = os.environ.get('OPENSYSML_GRPC_VERSION') or None

    # Neither of these touches the cache, so neither takes the lock over it.
    named = named_binary()
    if named is not None:
        return named

    with cache_lock():
        chosen = _ensure_binary_locked(
            force_download, version, github_repo, binary_path
        )
        if chosen is not None:
            return stable_binary() if chosen == binary_path else chosen

    on_path = binary_on_path()
    if on_path is not None:
        return on_path

    raise ConnectionError(
        f"Binary not found at {binary_path}, named by ${BINARY_ENV} or on $PATH, and "
        f"auto-download disabled. Looked at: ${BINARY_ENV}, {binary_path}, $PATH.\n"
        f"  fix: build it (`make build-grpc`) and set ${BINARY_ENV} to the result, or\n"
        f"       ask for a release to download by setting $OPENSYSML_GRPC_VERSION "
        f"(e.g. latest), or passing version= here, or\n"
        f"       install a sysml-grpc on $PATH, or\n"
        f"       start a service yourself and pass its address to connect()."
    )


def _ensure_binary_locked(force_download, version, github_repo, binary_path):
    """The cache or a download of the release asked for, with the shared cache held.

    Returns:
        str or None: The binary chosen, or None when nothing is cached and no
            release was asked for, which leaves $PATH to answer
    """
    # Check if binary already exists and is executable
    cached = None
    if not force_download and os.path.exists(binary_path):
        if os.access(binary_path, os.X_OK):
            stale = stale_cache_reason(version, github_repo)
            if stale is None:
                return binary_path
            cached = binary_path
            warnings.warn(
                f"Replacing the cached sysml-grpc: {stale}. Downloading {version}.",
                stacklevel=3,
            )
    
    # Nothing cached and no release asked for, so $PATH is next, outside the lock.
    if version is None:
        # A download asked for with nothing to download is an error, as one that
        # fails is: neither is answered by a $PATH binary of unknown release.
        if force_download:
            raise ConnectionError(
                "A download was asked for without a release to download. Set "
                "$OPENSYSML_GRPC_VERSION, or pass version= here."
            )
        return None
    
    # Download binary with explicit version
    try:
        return download_binary(version=version, github_repo=github_repo)
    except UnpinnedReleaseError as e:
        # A release this opensysml pins nothing for contradicts nothing, so a
        # working cache stands.
        if cached is None:
            raise
        warnings.warn(
            f"Keeping the cached sysml-grpc at {cached}: {version} was not downloaded "
            f"({e}). It may be an older release than asked for.",
            stacklevel=3,
        )
        return cached
    except ChecksumMismatchError:
        # A download that may have been tampered with is never answered from the
        # cache, so it must not reach the ConnectionError fallback below.
        raise
    except ConnectionError as e:
        if cached is None:
            raise
        # A release with no binary to fetch is no reason to lose a working one.
        warnings.warn(
            f"Keeping the cached sysml-grpc at {cached}: {version} could not be "
            f"downloaded ({e}). It may be an older release than asked for.",
            stacklevel=3,
        )
        return cached

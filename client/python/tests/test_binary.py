"""Tests for binary management module."""

import copy
import hashlib
import http.client
import json
import os
import platform
import re
import signal
import subprocess
import sys
import threading
import pytest
from unittest.mock import patch, Mock, mock_open
from opensysml.binary import (
    PINNED_SHA256,
    binary_on_path,
    cache_lock,
    cached_release,
    default_github_repo,
    detect_platform,
    get_binary_path,
    download_binary,
    expected_digest,
    metadata_path,
    named_binary,
    pinned_digest,
    lock_path,
    release_asset_name,
    resolve_latest_version,
    service_binary_name,
    stable_binary_path,
    stale_cache_reason,
    verify_checksum,
    write_metadata,
    ensure_binary
)
from opensysml.errors import ChecksumMismatchError, UnpinnedReleaseError
from opensysml.errors import ConnectionError as OpenSysMLConnectionError


def linked(content=b'cached binary'):
    """The digest-named path ensure_binary hands a caller for cached content."""
    return stable_binary_path(hashlib.sha256(content).hexdigest())


@pytest.fixture(autouse=True)
def resolution_environment(tmp_path, monkeypatch):
    """Resolve binaries against this test's environment, not the developer's.

    Every test in this file decides what $OPENSYSML_BINARY and $PATH offer, so a
    sysml-grpc the host happens to have installed cannot answer for one of them.
    """
    empty = tmp_path / 'empty-path'
    empty.mkdir()
    monkeypatch.setenv('PATH', str(empty))
    monkeypatch.delenv('OPENSYSML_BINARY', raising=False)
    monkeypatch.delenv('OPENSYSML_GRPC_BINARY', raising=False)


def executable(path, content=b'a sysml-grpc build'):
    """Write executable content at a path, and return it as a string."""
    path.write_bytes(content)
    path.chmod(0o755)
    return str(path)


@pytest.fixture
def cache(tmp_path, monkeypatch):
    """A cache directory this test owns, with helpers to fill it.

    Yields:
        A callable placing binary content, optionally recorded as a release
    """
    binary_path = str(tmp_path / 'sysml-grpc')
    monkeypatch.setattr('opensysml.binary.get_binary_path', lambda: binary_path)
    monkeypatch.delenv('OPENSYSML_GRPC_VERSION', raising=False)

    def place(content=b'cached binary', version=None):
        with open(binary_path, 'wb') as f:
            f.write(content)
        os.chmod(binary_path, 0o755)
        if version is not None:
            write_metadata(version, hashlib.sha256(content).hexdigest())
        return binary_path

    return place


@pytest.fixture(autouse=True)
def pins(monkeypatch):
    """Pin digests for the fake releases downloaded below.

    Stands in for the table a release of opensysml ships, so nothing here depends
    on the digests of the real published assets.
    """
    table = {}
    monkeypatch.setattr('opensysml.binary.PINNED_SHA256', table)
    monkeypatch.delenv('OPENSYSML_ALLOW_UNPINNED_DOWNLOAD', raising=False)

    def pin(digest, version, goos='linux', goarch='amd64', repo='Open-MBEE/OpenSysML'):
        asset = release_asset_name(goos, goarch)
        table.setdefault(repo, {}).setdefault(version, {})[asset] = digest
        return digest

    return pin


def test_detect_platform():
    """Test platform detection returns valid tuple."""
    os_name, arch = detect_platform()
    assert os_name in ('linux', 'darwin', 'windows')
    assert arch in ('amd64', 'arm64')


def test_get_binary_path():
    """Test binary path construction."""
    path = get_binary_path()
    assert path.startswith(os.path.expanduser('~/.opensysml/bin/'))
    assert path.endswith('sysml-grpc') or path.endswith('sysml-grpc.exe')


def test_service_binary_name_is_the_executable_one_on_windows():
    """Test the name looked for on $PATH and in the cache carries the .exe suffix."""
    assert service_binary_name('linux') == 'sysml-grpc'
    assert service_binary_name('darwin') == 'sysml-grpc'
    assert service_binary_name('windows') == 'sysml-grpc.exe'


def test_binary_on_path_looks_for_the_windows_name_on_windows(tmp_path, monkeypatch):
    """Test a Windows host scans $PATH for sysml-grpc.exe, not for sysml-grpc."""
    monkeypatch.setattr('opensysml.binary.platform.system', lambda: 'Windows')
    monkeypatch.setenv('PATH', str(tmp_path))
    executable(tmp_path / 'sysml-grpc')
    assert binary_on_path() is None

    on_path = executable(tmp_path / 'sysml-grpc.exe')
    assert binary_on_path() == on_path


def test_download_binary(pins):
    """Test binary download from GitHub releases."""
    # Mock binary download
    mock_binary_data = b'fake binary content'
    actual_checksum = hashlib.sha256(mock_binary_data).hexdigest()
    pins(actual_checksum, 'v0.1.0')
    
    # Mock checksum file download
    mock_checksum_data = f"{actual_checksum}  sysml-grpc-linux-amd64\n".encode()
    
    with patch('urllib.request.urlopen') as mock_urlopen:
        # First call: checksum file
        # Second call: binary
        mock_urlopen.side_effect = [
            Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=mock_checksum_data))), __exit__=Mock(return_value=False)),
            Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=mock_binary_data))), __exit__=Mock(return_value=False))
        ]
        
        with patch('builtins.open', mock_open(read_data=mock_binary_data)):
            with patch('os.makedirs'):
                with patch('os.chmod'):
                    with patch('os.replace'):
                        result = download_binary(version='v0.1.0')
                        
                        expected_path = get_binary_path()
                        assert result == expected_path
                        assert mock_urlopen.call_count == 2
                        # Verify URL format
                        call_args = mock_urlopen.call_args_list[1][0][0]
                        assert 'github.com/Open-MBEE/OpenSysML/releases/download/v0.1.0' in call_args


def test_verify_checksum():
    """Test SHA-256 checksum verification."""
    data = b"test binary content"
    expected = hashlib.sha256(data).hexdigest()
    
    with patch('builtins.open', mock_open(read_data=data)):
        assert verify_checksum('/fake/path', expected) == True
        assert verify_checksum('/fake/path', 'wrong_hash') == False


def test_ensure_binary_exists(cache):
    """Test ensure_binary returns a digest-named link to the binary it found."""
    binary_path = cache()
    path = ensure_binary()
    assert path == linked()
    assert os.path.samefile(path, binary_path)
    assert os.access(path, os.X_OK)


def test_ensure_binary_returns_a_path_no_later_install_can_replace(cache):
    """Test what is handed to Popen is not the cache another client installs over."""
    binary_path = cache(b'the release asked for', version='v0.0.7')
    path = ensure_binary(version='v0.0.7')
    assert path != binary_path

    # Another process, or the Java client, installing the next release over the cache.
    replacement = binary_path + '.next'
    with open(replacement, 'wb') as f:
        f.write(b'the release after')
    os.replace(replacement, binary_path)

    with open(path, 'rb') as f:
        assert f.read() == b'the release asked for'


def test_ensure_binary_reuses_the_link_it_already_made(cache):
    """Test the digest-named link is made once, not copied per connection."""
    cache(version='v0.0.7')
    first = ensure_binary(version='v0.0.7')
    second = ensure_binary(version='v0.0.7')
    assert first == second == linked()


def test_ensure_binary_falls_back_to_the_cache_when_it_cannot_be_linked(cache):
    """Test an unlinkable cache still starts, with a warning that it may be replaced."""
    binary_path = cache(version='v0.0.7')
    with patch('opensysml.binary._link', side_effect=OSError('read-only')):
        with pytest.warns(UserWarning, match='may be started'):
            assert ensure_binary(version='v0.0.7') == binary_path


def test_ensure_binary_downloads():
    """Test ensure_binary downloads if binary missing and version provided."""
    with patch('os.path.exists', return_value=False):
        with patch('opensysml.binary.download_binary') as mock_download:
            mock_download.return_value = '/fake/path/sysml-grpc'
            path = ensure_binary(version='v0.1.0')
            assert path == '/fake/path/sysml-grpc'
            mock_download.assert_called_once_with(version='v0.1.0', github_repo=None)


def test_ensure_binary_raises_without_version():
    """Test ensure_binary raises ConnectionError when binary missing and no version."""
    from opensysml.errors import ConnectionError
    with patch('os.path.exists', return_value=False):
        with pytest.raises(ConnectionError, match="Binary not found.*auto-download disabled"):
            ensure_binary()


def test_ensure_binary_starts_the_build_named_for_it(cache, tmp_path, monkeypatch):
    """Test $OPENSYSML_BINARY names the binary started, at exactly that path."""
    named = executable(tmp_path / 'my-build')
    monkeypatch.setenv('OPENSYSML_BINARY', named)
    assert ensure_binary() == named


def test_a_named_build_beats_the_cache_and_path(cache, tmp_path, monkeypatch):
    """Test naming a build overrides every other place one could come from."""
    cache(version='v0.0.7')
    on_path = tmp_path / 'path-dir'
    on_path.mkdir()
    executable(on_path / 'sysml-grpc')
    monkeypatch.setenv('PATH', str(on_path))
    named = executable(tmp_path / 'my-build')
    monkeypatch.setenv('OPENSYSML_BINARY', named)

    assert ensure_binary() == named


def test_a_named_build_that_is_missing_is_an_error(cache, tmp_path, monkeypatch):
    """Test an instruction that cannot be honoured fails instead of looking elsewhere."""
    cache(version='v0.0.7')
    on_path = tmp_path / 'path-dir'
    on_path.mkdir()
    executable(on_path / 'sysml-grpc')
    monkeypatch.setenv('PATH', str(on_path))
    missing = str(tmp_path / 'not-here')
    monkeypatch.setenv('OPENSYSML_BINARY', missing)

    with pytest.raises(OpenSysMLConnectionError) as raised:
        ensure_binary()
    assert '$OPENSYSML_BINARY' in str(raised.value)
    assert missing in str(raised.value)


def test_a_named_build_that_is_not_executable_is_an_error(tmp_path, monkeypatch):
    """Test a file that cannot be executed is reported, not silently skipped."""
    unrunnable = tmp_path / 'my-build'
    unrunnable.write_bytes(b'not chmod +x')
    unrunnable.chmod(0o644)
    monkeypatch.setenv('OPENSYSML_BINARY', str(unrunnable))

    with pytest.raises(OpenSysMLConnectionError, match='not an executable file'):
        ensure_binary()


def test_ensure_binary_starts_a_service_installed_on_path(cache, tmp_path, monkeypatch):
    """Test a sysml-grpc installed by a package manager answers for an empty cache."""
    directory = tmp_path / 'usr-local-bin'
    directory.mkdir()
    on_path = executable(directory / 'sysml-grpc')
    monkeypatch.setenv('PATH', str(directory))

    # As it is found: its own path, not a digest-named link into the cache.
    assert ensure_binary() == on_path


def test_path_is_scanned_in_order_past_what_cannot_be_executed(tmp_path, monkeypatch):
    """Test a non-executable sysml-grpc earlier on $PATH does not shadow a later one."""
    first, second = tmp_path / 'first', tmp_path / 'second'
    first.mkdir()
    second.mkdir()
    unrunnable = first / 'sysml-grpc'
    unrunnable.write_bytes(b'not chmod +x')
    unrunnable.chmod(0o644)
    runnable = executable(second / 'sysml-grpc')
    monkeypatch.setenv('PATH', os.pathsep.join([str(first), str(second)]))

    assert binary_on_path() == runnable


def test_the_cache_beats_a_service_on_path(cache, tmp_path, monkeypatch):
    """Test the cache, whose release is known, is preferred to an unknown one on $PATH."""
    cache(version='v0.0.7')
    directory = tmp_path / 'usr-local-bin'
    directory.mkdir()
    executable(directory / 'sysml-grpc')
    monkeypatch.setenv('PATH', str(directory))

    assert ensure_binary() == linked()


def test_a_download_that_fails_is_not_answered_from_path(cache, tmp_path, monkeypatch):
    """Test a $PATH binary, of no known release, does not stand in for one asked for."""
    directory = tmp_path / 'usr-local-bin'
    directory.mkdir()
    executable(directory / 'sysml-grpc')
    monkeypatch.setenv('PATH', str(directory))

    with patch('opensysml.binary.download_binary',
               side_effect=OpenSysMLConnectionError('release not found')):
        with pytest.raises(OpenSysMLConnectionError, match='release not found'):
            ensure_binary(version='v9.9.9')


def test_a_download_with_no_release_to_download_is_not_answered_from_path(
    cache, tmp_path, monkeypatch
):
    """Test asking for a download without a release is an error, not a $PATH binary."""
    directory = tmp_path / 'usr-local-bin'
    directory.mkdir()
    executable(directory / 'sysml-grpc')
    monkeypatch.setenv('PATH', str(directory))

    with pytest.raises(OpenSysMLConnectionError, match='without a release'):
        ensure_binary(force_download=True)


def test_a_local_build_answers_on_a_platform_no_release_is_published_for(
    cache, tmp_path, monkeypatch
):
    """Test an architecture releases skip still resolves a named build and one on $PATH."""
    monkeypatch.setattr('opensysml.binary.platform.machine', lambda: 'riscv64')
    directory = tmp_path / 'usr-local-bin'
    directory.mkdir()
    on_path = executable(directory / 'sysml-grpc')
    monkeypatch.setenv('PATH', str(directory))
    assert ensure_binary() == on_path

    named = executable(tmp_path / 'my-build')
    monkeypatch.setenv('OPENSYSML_BINARY', named)
    assert ensure_binary() == named


def test_ensure_binary_reports_everywhere_it_looked(cache):
    """Test the refusal names each place a binary could have come from."""
    import opensysml.binary

    with pytest.raises(OpenSysMLConnectionError) as raised:
        ensure_binary()
    message = str(raised.value)
    assert '$OPENSYSML_BINARY' in message
    assert opensysml.binary.get_binary_path() in message
    assert '$PATH' in message
    assert '$OPENSYSML_GRPC_VERSION' in message


def test_download_binary_verifies_checksum(pins):
    """Test that download_binary fetches and verifies checksum."""
    import pytest
    version = 'v0.1.0'
    github_repo = 'Open-MBEE/OpenSysML'
    
    # Mock binary download
    mock_binary_data = b'fake binary content'
    actual_checksum = hashlib.sha256(mock_binary_data).hexdigest()
    pins(actual_checksum, version)
    
    # Mock checksum file download
    mock_checksum_data = f"{actual_checksum}  sysml-grpc-linux-amd64\n".encode()
    
    with patch('urllib.request.urlopen') as mock_urlopen:
        # First call: checksum file
        # Second call: binary
        mock_urlopen.side_effect = [
            Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=mock_checksum_data))), __exit__=Mock(return_value=False)),
            Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=mock_binary_data))), __exit__=Mock(return_value=False))
        ]
        
        with patch('opensysml.binary.detect_platform', return_value=('linux', 'amd64')):
            with patch('builtins.open', mock_open(read_data=mock_binary_data)):
                with patch('os.makedirs'):
                    with patch('os.chmod'):
                        with patch('os.replace'):
                            result = download_binary(version, github_repo)
                            
                            # Should have called urlopen twice (checksum + binary)
                            assert mock_urlopen.call_count == 2
                            
                            # Verify checksum URL was constructed correctly
                            checksum_url = str(mock_urlopen.call_args_list[0][0][0])
                            assert '.sha256' in checksum_url


def test_download_binary_fails_on_checksum_mismatch(pins):
    """Test that download fails if the binary does not match the digest expected."""
    import pytest
    version = 'v0.1.0'
    github_repo = 'Open-MBEE/OpenSysML'
    
    # Mock binary download
    mock_binary_data = b'fake binary content'
    
    # Wrong checksum, agreed on by the pin and the sidecar: the binary differs.
    wrong_checksum = 'deadbeef' * 8
    pins(wrong_checksum, version)
    mock_checksum_data = f"{wrong_checksum}  sysml-grpc-linux-amd64\n".encode()
    
    with patch('urllib.request.urlopen') as mock_urlopen:
        mock_urlopen.side_effect = [
            Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=mock_checksum_data))), __exit__=Mock(return_value=False)),
            Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=mock_binary_data))), __exit__=Mock(return_value=False))
        ]
        
        with patch('opensysml.binary.detect_platform', return_value=('linux', 'amd64')):
            with patch('builtins.open', mock_open(read_data=mock_binary_data)):
                with patch('os.makedirs'):
                    with patch('os.chmod'):
                        with patch('os.remove'):
                            from opensysml.errors import ConnectionError
                            with pytest.raises(ConnectionError, match="Checksum mismatch"):
                                download_binary(version, github_repo)


def test_default_github_repo_env_override(monkeypatch):
    """Test $OPENSYSML_GITHUB_REPO overrides the default repository."""
    monkeypatch.delenv('OPENSYSML_GITHUB_REPO', raising=False)
    assert default_github_repo() == 'Open-MBEE/OpenSysML'

    monkeypatch.setenv('OPENSYSML_GITHUB_REPO', 'JPL-Devin/OpenSysML')
    assert default_github_repo() == 'JPL-Devin/OpenSysML'


def test_resolve_latest_version():
    """Test latest release tag is read from the GitHub API."""
    payload = b'{"tag_name": "v0.0.4"}'
    with patch('urllib.request.urlopen') as mock_urlopen:
        mock_urlopen.return_value = Mock(
            __enter__=Mock(return_value=Mock(read=Mock(return_value=payload))),
            __exit__=Mock(return_value=False))
        assert resolve_latest_version('Open-MBEE/OpenSysML') == 'v0.0.4'
        url = str(mock_urlopen.call_args_list[0][0][0])
        assert url == 'https://api.github.com/repos/Open-MBEE/OpenSysML/releases/latest'


def test_resolve_latest_version_without_tag():
    """Test a release carrying no tag is reported rather than returned."""
    from opensysml.errors import ConnectionError
    with patch('urllib.request.urlopen') as mock_urlopen:
        mock_urlopen.return_value = Mock(
            __enter__=Mock(return_value=Mock(read=Mock(return_value=b'{}'))),
            __exit__=Mock(return_value=False))
        with pytest.raises(ConnectionError, match="no tag name"):
            resolve_latest_version('Open-MBEE/OpenSysML')


def test_download_binary_latest_resolves_tag(pins):
    """Test version='latest' downloads from the resolved tag."""
    mock_binary_data = b'fake binary content'
    actual_checksum = hashlib.sha256(mock_binary_data).hexdigest()
    pins(actual_checksum, 'v0.0.4')
    mock_checksum_data = f"{actual_checksum}  sysml-grpc-linux-amd64\n".encode()

    with patch('opensysml.binary.resolve_latest_version', return_value='v0.0.4') as mock_resolve:
        with patch('urllib.request.urlopen') as mock_urlopen:
            mock_urlopen.side_effect = [
                Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=mock_checksum_data))), __exit__=Mock(return_value=False)),
                Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=mock_binary_data))), __exit__=Mock(return_value=False))
            ]
            with patch('opensysml.binary.detect_platform', return_value=('linux', 'amd64')):
                with patch('builtins.open', mock_open(read_data=mock_binary_data)):
                    with patch('os.makedirs'), patch('os.chmod'), patch('os.replace'):
                        download_binary(version='latest')

            mock_resolve.assert_called_once_with('Open-MBEE/OpenSysML')
            for call in mock_urlopen.call_args_list:
                assert 'releases/download/v0.0.4/sysml-grpc-linux-amd64' in str(call[0][0])


def test_ensure_binary_downloads_version_from_env(monkeypatch):
    """Test $OPENSYSML_GRPC_VERSION enables auto-download when no binary is present."""
    monkeypatch.setenv('OPENSYSML_GRPC_VERSION', 'v0.0.4')
    with patch('os.path.exists', return_value=False):
        with patch('opensysml.binary.download_binary') as mock_download:
            mock_download.return_value = '/fake/path/sysml-grpc'
            assert ensure_binary() == '/fake/path/sysml-grpc'
            mock_download.assert_called_once_with(version='v0.0.4', github_repo=None)


def test_download_binary_records_the_release(cache, pins):
    """Test a download records which release the cache now holds."""
    data = b'fake binary content'
    checksum = hashlib.sha256(data).hexdigest()
    pins(checksum, 'v0.0.7')
    responses = [
        Mock(__enter__=Mock(return_value=Mock(read=Mock(
            return_value=f"{checksum}  sysml-grpc-linux-amd64\n".encode()))),
            __exit__=Mock(return_value=False)),
        Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=data))),
             __exit__=Mock(return_value=False)),
    ]
    with patch('urllib.request.urlopen', side_effect=responses):
        with patch('opensysml.binary.detect_platform', return_value=('linux', 'amd64')):
            download_binary(version='v0.0.7')

    with open(metadata_path()) as f:
        assert json.load(f) == {
            'version': 'v0.0.7',
            'sha256': checksum,
            'repo': 'Open-MBEE/OpenSysML',
        }
    assert cached_release() == 'v0.0.7'


def test_download_binary_overwrites_the_cache_it_replaces(cache, pins):
    """Test a download installs over an existing cache, as replacing one must."""
    cache(b'the release before', version='v0.0.5')
    data = b'the release asked for'
    checksum = hashlib.sha256(data).hexdigest()
    pins(checksum, 'v0.0.7')
    responses = [
        Mock(__enter__=Mock(return_value=Mock(read=Mock(
            return_value=f"{checksum}  sysml-grpc-linux-amd64\n".encode()))),
            __exit__=Mock(return_value=False)),
        Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=data))),
             __exit__=Mock(return_value=False)),
    ]
    with patch('urllib.request.urlopen', side_effect=responses):
        with patch('opensysml.binary.detect_platform', return_value=('linux', 'amd64')):
            path = download_binary(version='v0.0.7')

    with open(path, 'rb') as f:
        assert f.read() == data
    assert cached_release() == 'v0.0.7'
    assert not os.path.exists(path + '.tmp')


class _EndlessBody:
    """A response that is always longer than whatever is read of it."""

    def __enter__(self):
        return self

    def __exit__(self, *closing):
        return False

    def read(self, size=None):
        assert size is not None, 'an unbounded read of this would never return'
        return b'\0' * size


@pytest.mark.skipif(os.name == 'nt', reason='the probe takes a POSIX fcntl lock')
def test_download_binary_refuses_a_body_too_large_to_be_a_release(cache, pins, monkeypatch):
    """Test an endless body is refused, rather than held for while filling memory."""
    monkeypatch.setattr('opensysml.binary.MAX_BINARY_BYTES', 4096)
    binary_path = cache(b'the release before', version='v0.0.5')
    checksum = pins(hashlib.sha256(b'the release asked for').hexdigest(), 'v0.0.7')
    responses = [
        Mock(__enter__=Mock(return_value=Mock(read=Mock(
            return_value=f"{checksum}  sysml-grpc-linux-amd64\n".encode()))),
            __exit__=Mock(return_value=False)),
        _EndlessBody(),
    ]
    with patch('urllib.request.urlopen', side_effect=responses):
        with patch('opensysml.binary.detect_platform', return_value=('linux', 'amd64')):
            with pytest.raises(OpenSysMLConnectionError, match='is larger than'):
                download_binary(version='v0.0.7')

    with open(binary_path, 'rb') as f:
        assert f.read() == b'the release before'
    assert not os.path.exists(binary_path + '.tmp')
    assert _lock_is_free(lock_path())


def test_download_binary_reports_a_cache_it_cannot_install_over(cache, pins):
    """Test a binary held open by a running service is reported, not raised raw."""
    binary_path = cache(version='v0.0.5')
    data = b'the release asked for'
    checksum = hashlib.sha256(data).hexdigest()
    pins(checksum, 'v0.0.7')
    responses = [
        Mock(__enter__=Mock(return_value=Mock(read=Mock(
            return_value=f"{checksum}  sysml-grpc-linux-amd64\n".encode()))),
            __exit__=Mock(return_value=False)),
        Mock(__enter__=Mock(return_value=Mock(read=Mock(return_value=data))),
             __exit__=Mock(return_value=False)),
    ]
    with patch('urllib.request.urlopen', side_effect=responses):
        with patch('opensysml.binary.detect_platform', return_value=('linux', 'amd64')):
            with patch('os.replace', side_effect=PermissionError('in use')):
                with pytest.raises(OpenSysMLConnectionError, match='could not install it'):
                    download_binary(version='v0.0.7')

    assert not os.path.exists(binary_path + '.tmp')
    assert cached_release() == 'v0.0.5'


def test_cached_release_unknown_when_binary_was_replaced(cache):
    """Test a binary swapped under a recorded name is not read as that release."""
    binary_path = cache(b'downloaded', version='v0.0.7')
    with open(binary_path, 'wb') as f:
        f.write(b'something else entirely')
    assert cached_release() is None


def test_stale_cache_reason_accepts_the_release_asked_for(cache):
    """Test the cache is reused when it is the release asked for."""
    cache(version='v0.0.7')
    assert stale_cache_reason('v0.0.7') is None
    assert stale_cache_reason(None) is None


def test_stale_cache_reason_names_another_release(cache):
    """Test a cache from another release is reported, naming both tags."""
    cache(version='v0.0.5')
    reason = stale_cache_reason('v0.0.7')
    assert 'v0.0.5' in reason
    assert 'v0.0.7' in reason


def test_stale_cache_reason_for_an_unidentifiable_binary(cache):
    """Test a cache this client did not download cannot answer for a release."""
    cache()  # No record beside it: a hand-placed or pre-existing binary.
    assert 'cannot be told' in stale_cache_reason('v0.0.7')


def test_stale_cache_reason_keeps_the_cache_when_offline(cache):
    """Test version='latest' keeps a working cache when releases are unreachable."""
    from opensysml.errors import ConnectionError
    cache(version='v0.0.7')
    with patch('opensysml.binary.resolve_latest_version',
               side_effect=ConnectionError('no network')):
        assert stale_cache_reason('latest') is None


def test_ensure_binary_reuses_the_release_asked_for(cache):
    """Test no download happens when the cache is already that release."""
    binary_path = cache(version='v0.0.7')
    with patch('opensysml.binary.download_binary') as mock_download:
        assert ensure_binary(version='v0.0.7') == linked()
        mock_download.assert_not_called()


def test_ensure_binary_replaces_a_cache_from_another_release(cache):
    """Test a stale cache is replaced rather than served, with a warning saying so."""
    cache(version='v0.0.5')
    with patch('opensysml.binary.download_binary') as mock_download:
        mock_download.return_value = '/downloaded/sysml-grpc'
        with pytest.warns(UserWarning, match='v0.0.5'):
            assert ensure_binary(version='v0.0.7') == '/downloaded/sysml-grpc'
        mock_download.assert_called_once_with(version='v0.0.7', github_repo=None)


def test_ensure_binary_keeps_a_cache_when_no_version_is_asked_for(cache):
    """Test a locally built binary is left alone when nothing names a release."""
    cache()
    with patch('opensysml.binary.download_binary') as mock_download:
        assert ensure_binary() == linked()
        mock_download.assert_not_called()


def test_cache_from_another_repository_is_not_the_release_asked_for(cache, monkeypatch):
    """Test a fork's build is not served for the same tag of another repository."""
    cache(version='v0.0.7')  # recorded against the default repository
    monkeypatch.setenv('OPENSYSML_GITHUB_REPO', 'someone/fork')

    assert cached_release() is None
    reason = stale_cache_reason('v0.0.7')
    assert 'downloaded from Open-MBEE/OpenSysML' in reason
    assert 'someone/fork' in reason


def test_resolve_latest_version_gives_up_rather_than_hanging():
    """Test the releases query is bounded, being on the cached-binary fast path."""
    release = Mock()
    release.read.return_value = b'{"tag_name": "v0.0.7"}'
    response = Mock(__enter__=Mock(return_value=release), __exit__=Mock(return_value=False))
    with patch('urllib.request.urlopen', return_value=response) as urlopen:
        assert resolve_latest_version() == 'v0.0.7'

    assert urlopen.call_args.kwargs['timeout'] > 0


def test_a_timed_out_release_query_keeps_a_working_cache(cache):
    """Test a network that drops traffic leaves the cached binary in use."""
    cache(version='v0.0.7')
    with patch('urllib.request.urlopen', side_effect=TimeoutError('timed out')):
        assert stale_cache_reason('latest') is None
        with pytest.raises(OpenSysMLConnectionError, match='Failed to resolve latest release'):
            resolve_latest_version()


def test_a_cache_survives_a_replacement_that_cannot_be_downloaded(cache):
    """Test a working binary keeps serving when the release asked for cannot be had."""
    cache(version='v0.0.5')
    with patch('opensysml.binary.download_binary',
               side_effect=OpenSysMLConnectionError('404 Not Found')):
        with pytest.warns(UserWarning, match='Keeping the cached sysml-grpc'):
            kept = ensure_binary(version='v0.0.7')

    assert kept == linked()
    assert cached_release() == 'v0.0.5'


def test_a_tampered_download_is_not_answered_from_the_cache(cache):
    """Test an integrity failure refuses to start rather than serving the old binary."""
    cache(version='v0.0.5')
    with patch('opensysml.binary.download_binary',
               side_effect=ChecksumMismatchError('Checksum mismatch')):
        refused = pytest.raises(ChecksumMismatchError)
        with pytest.warns(UserWarning, match='Replacing the cached sysml-grpc'):
            with refused:
                ensure_binary(version='v0.0.7')


def test_a_dropped_connection_keeps_a_working_cache(cache):
    """Test a reset connection is a transport failure, not a startup failure."""
    cache(version='v0.0.7')
    with patch('urllib.request.urlopen',
               side_effect=http.client.RemoteDisconnected('closed')):
        assert stale_cache_reason('latest') is None
        assert ensure_binary(version='latest') == linked()


# The pins the table actually ships, kept before the fixture above replaces it.
SHIPPED_PINS = copy.deepcopy(PINNED_SHA256)


class TestPinnedDigests:
    """The digest a download is checked against comes from opensysml, not the origin."""

    def test_every_shipped_pin_is_a_sha256_of_a_known_asset(self):
        """A malformed or misnamed pin would silently never match a download."""
        assert SHIPPED_PINS, "no release is pinned, so every download is refused"
        for repo, releases in SHIPPED_PINS.items():
            assert '/' in repo
            for version, assets in releases.items():
                assert version.startswith('v')
                for asset, digest in assets.items():
                    assert asset.startswith('sysml-grpc-')
                    assert re.fullmatch(r'[0-9a-f]{64}', digest), (repo, version, asset)

    def test_each_pinned_release_covers_every_platform_published(self):
        """A platform left unpinned refuses to install that release."""
        published = {
            release_asset_name(goos, goarch)
            for goos in ('linux', 'darwin', 'windows')
            for goarch in ('amd64', 'arm64')
            if not (goos == 'windows' and goarch == 'arm64')
        }
        for repo, releases in SHIPPED_PINS.items():
            for version, assets in releases.items():
                assert set(assets) == published, f'{repo} {version}'

    def test_a_release_with_no_pin_is_refused(self):
        """Same-origin trust is not the silent fallback for an unpinned release."""
        assert pinned_digest('v9.9.9', 'sysml-grpc-linux-amd64') is None
        with pytest.raises(ChecksumMismatchError, match='pins no SHA-256 digest'):
            expected_digest('v9.9.9', 'sysml-grpc-linux-amd64', 'ab' * 32)

    def test_an_unpinned_release_may_be_accepted_explicitly(self, monkeypatch):
        """Opting in falls back to the served checksum, saying what that means."""
        monkeypatch.setenv('OPENSYSML_ALLOW_UNPINNED_DOWNLOAD', '1')
        served = 'ab' * 32
        with pytest.warns(RuntimeWarning, match='pins no digest'):
            assert expected_digest('v9.9.9', 'sysml-grpc-linux-amd64', served) == served

    def test_opting_in_for_one_repository_is_not_opting_in_for_another(self, monkeypatch):
        """A fork's unpinned releases are its own; trusting it trusts nothing else."""
        monkeypatch.setenv('OPENSYSML_ALLOW_UNPINNED_DOWNLOAD', 'a-fork/OpenSysML')
        served = 'ab' * 32
        with pytest.warns(RuntimeWarning, match='pins no digest'):
            assert expected_digest(
                'v9.9.9', 'sysml-grpc-linux-amd64', served, github_repo='a-fork/OpenSysML'
            ) == served

        with pytest.raises(ChecksumMismatchError, match='pins no SHA-256 digest'):
            expected_digest('v9.9.9', 'sysml-grpc-linux-amd64', served)

    def test_the_repository_opted_in_for_may_be_the_one_being_downloaded_from(self, monkeypatch):
        """$OPENSYSML_GITHUB_REPO is what a bare opt-in for that repository names."""
        monkeypatch.setenv('OPENSYSML_GITHUB_REPO', 'a-fork/OpenSysML')
        monkeypatch.setenv('OPENSYSML_ALLOW_UNPINNED_DOWNLOAD', 'a-fork/OpenSysML')
        served = 'ab' * 32
        with pytest.warns(RuntimeWarning, match='pins no digest'):
            assert expected_digest('v0.0.5', 'sysml-grpc-linux-amd64', served) == served

    def test_a_pin_is_preferred_to_the_digest_the_origin_serves(self, pins):
        """The pin is what a download is verified against when the two agree."""
        digest = pins('cd' * 32, 'v9.9.9')
        assert expected_digest('v9.9.9', 'sysml-grpc-linux-amd64', digest) == digest

    def test_a_release_republished_with_another_binary_is_refused(self, pins):
        """A sidecar contradicting the pin is the case same-origin trust misses."""
        pins('cd' * 32, 'v9.9.9')
        with pytest.raises(ChecksumMismatchError, match='but opensysml pins'):
            expected_digest('v9.9.9', 'sysml-grpc-linux-amd64', 'ef' * 32)

    def test_a_download_of_an_unpinned_release_is_refused(self, cache):
        """No pin and no verifiable manifest stops the download before the binary."""
        with patch('urllib.request.urlopen') as urlopen:
            with pytest.raises(ChecksumMismatchError, match='pins no SHA-256 digest'):
                download_binary(version='v9.9.9')
        # The sidecar, the manifest and its bundle, and no binary.
        fetched = [str(call[0][0]) for call in urlopen.call_args_list]
        assert len(fetched) == 3
        assert fetched[1].endswith('/v9.9.9/SHA256SUMS.txt')
        assert fetched[2].endswith('/v9.9.9/SHA256SUMS.txt.bundle')
        assert not any(url.endswith(release_asset_name()) for url in fetched)

    def test_a_missing_pin_is_told_apart_from_a_contradicted_one(self, pins):
        """Only a contradiction is evidence of tampering, so only it is unrecoverable."""
        with pytest.raises(UnpinnedReleaseError) as missing:
            expected_digest('v9.9.9', 'sysml-grpc-linux-amd64', 'ab' * 32)
        assert missing.value.unpinned

        pins('cd' * 32, 'v9.9.8')
        with pytest.raises(ChecksumMismatchError) as contradicted:
            expected_digest('v9.9.8', 'sysml-grpc-linux-amd64', 'ef' * 32)
        assert not isinstance(contradicted.value, UnpinnedReleaseError)
        assert not contradicted.value.unpinned

    def test_the_unpinned_refusal_is_its_own_class_inside_the_old_one(self):
        """An except clause written against the old base still catches it."""
        assert issubclass(UnpinnedReleaseError, ChecksumMismatchError)
        assert issubclass(UnpinnedReleaseError, OpenSysMLConnectionError)
        with pytest.raises(ChecksumMismatchError):
            raise UnpinnedReleaseError('pins no SHA-256 digest')
        assert UnpinnedReleaseError('x').unpinned
        assert not ChecksumMismatchError('x').unpinned

    def test_no_pin_raises_the_unpinned_class_rather_than_a_mismatch(self):
        """Nothing contradicts anything, so the cause reported is the real one."""
        with pytest.raises(UnpinnedReleaseError, match='pins no SHA-256 digest'):
            expected_digest('v9.9.9', 'sysml-grpc-linux-amd64', 'ab' * 32)

    def test_a_contradicted_pin_is_not_the_unpinned_class(self, pins):
        pins('cd' * 32, 'v9.9.7')
        with pytest.raises(ChecksumMismatchError, match='but opensysml pins') as exc:
            expected_digest('v9.9.7', 'sysml-grpc-linux-amd64', 'ef' * 32)
        assert type(exc.value) is ChecksumMismatchError

    def test_an_unpinned_release_keeps_a_working_cache(self, cache):
        """A release newer than this opensysml is a build it cannot get, not a tampered one."""
        cache(version='v0.0.5')
        with patch('opensysml.binary.download_binary',
                   side_effect=UnpinnedReleaseError('pins no SHA-256 digest')):
            with pytest.warns(UserWarning, match='Keeping the cached sysml-grpc'):
                kept = ensure_binary(version='v9.9.9')

        assert kept == linked()
        assert cached_release() == 'v0.0.5'

    def test_an_unpinned_release_with_no_cache_is_still_refused(self, cache):
        """Nothing to fall back to leaves the refusal, with its own explanation."""
        with patch('opensysml.binary.download_binary',
                   side_effect=UnpinnedReleaseError('pins no SHA-256 digest')):
            with pytest.raises(ChecksumMismatchError, match='pins no SHA-256 digest'):
                ensure_binary(version='v9.9.9')


def _lock_is_free(path):
    """Whether another process can take the cache lock right now."""
    probe = (
        'import fcntl, os, sys\n'
        'fd = os.open(sys.argv[1], os.O_RDWR | os.O_CREAT, 0o600)\n'
        'try:\n'
        '    fcntl.lockf(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)\n'
        'except OSError:\n'
        '    sys.exit(1)\n'
        'sys.exit(0)\n'
    )
    return subprocess.run([sys.executable, '-c', probe, path]).returncode == 0


@pytest.mark.skipif(os.name == 'nt', reason='the probe takes a POSIX fcntl lock')
def test_the_cache_lock_shuts_other_processes_out(cache):
    """Test the lock is a real one, so the Java client contends for it too."""
    binary_path = cache()
    assert lock_path() == binary_path + '.lock'

    with cache_lock():
        assert not _lock_is_free(lock_path())
    assert _lock_is_free(lock_path())


@pytest.mark.skipif(not hasattr(os, 'fork'), reason='fork is POSIX only')
def test_a_fork_child_can_take_a_cache_lock_a_parent_thread_held(cache):
    """Test a child never inherits the lock of a thread that does not exist there."""
    cache()
    holding = threading.Event()
    release = threading.Event()

    def hold():
        with cache_lock():
            holding.set()
            release.wait(30)

    holder = threading.Thread(target=hold)
    holder.start()
    try:
        assert holding.wait(10)
        pid = os.fork()
        if pid == 0:
            taken = 1
            try:
                signal.alarm(20)
                with cache_lock():
                    taken = 0
            finally:
                os._exit(taken)
    finally:
        release.set()
        holder.join(10)

    assert os.waitpid(pid, 0)[1] == 0, 'the child deadlocked on an inherited lock'


@pytest.mark.skipif(os.name == 'nt', reason='the probe takes a POSIX fcntl lock')
def test_a_nested_cache_lock_does_not_release_the_outer_one(cache):
    """Test closing a nested descriptor does not drop this process's fcntl lock."""
    cache()
    with cache_lock():
        with cache_lock():
            pass
        assert not _lock_is_free(lock_path())

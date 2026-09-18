"""Tests for verifying the signature on a release's checksum manifest.

Every test here is offline: the bundles under tests/fixtures/signed_release were
recorded by clients/python/scripts/make_signed_release_fixture.py against a root of
trust recorded beside them, and the release they belong to is served from those
files rather than fetched. Nothing reaches the network, and nothing depends on
the digests of the real published assets.
"""

import hashlib
import http.client
import json
import os
import sys
import urllib.error
import pytest
from unittest.mock import patch
from opensysml.binary import (
    download_binary,
    expected_digest,
    read_metadata,
    release_asset_name,
    signed_manifest_digest,
    write_metadata,
)
from opensysml.errors import (
    ChecksumMismatchError,
    ManifestSignatureError,
    UnpinnedReleaseError,
    UnsignedReleaseError,
)
from opensysml.signing import (
    BUNDLE_ASSET,
    MANIFEST_ASSET,
    SIGNED_MANIFEST_SIGNERS,
    ReleaseSigner,
    manifest_digest,
    signer_for,
    verified_manifest_digest,
    verify_manifest,
)

FIXTURES = os.path.join(os.path.dirname(__file__), 'fixtures', 'signed_release')
BINARY_ASSET = 'sysml-grpc-linux-amd64'
REPO = 'Open-MBEE/OpenSysML'
VERSION = 'v9.9.9'


def fixture(name):
    """Contents of a recorded fixture.

    Args:
        name (str): File name under tests/fixtures/signed_release

    Returns:
        bytes: Its contents
    """
    with open(os.path.join(FIXTURES, name), 'rb') as f:
        return f.read()


@pytest.fixture
def identity():
    """The identity the recorded bundles were signed with.

    Returns:
        dict: Issuer, project and pipeline definition
    """
    return json.loads(fixture('identity.json'))


@pytest.fixture
def signer(identity, monkeypatch):
    """The signer the recorded bundles verify against.

    The recorded root of trust stands in for Sigstore's production one, which is
    the only difference from what the client pins.

    Returns:
        ReleaseSigner: Signer expecting the fixtures' identity
    """
    recorded = ReleaseSigner(
        issuer=identity['issuer'],
        project=identity['project'],
        trusted_root=os.path.join(FIXTURES, 'trusted_root.json'),
    )
    monkeypatch.setitem(SIGNED_MANIFEST_SIGNERS, REPO, recorded)
    return recorded


@pytest.fixture
def manifest():
    """The recorded checksum manifest.

    Returns:
        bytes: Manifest that was signed
    """
    return fixture(MANIFEST_ASSET)


@pytest.fixture
def bundle():
    """The recorded sigstore bundle over that manifest.

    Returns:
        bytes: Bundle, as JSON
    """
    return fixture(BUNDLE_ASSET)


@pytest.fixture
def release(monkeypatch, tmp_path):
    """A release served from the fixtures, with the cache in a directory per test.

    Yields:
        A callable taking asset names to serve, returning the served mapping
    """
    binary_path = str(tmp_path / 'sysml-grpc')
    monkeypatch.setattr('opensysml.binary.get_binary_path', lambda: binary_path)
    monkeypatch.setattr('opensysml.binary.release_asset_name', lambda *a: BINARY_ASSET)
    monkeypatch.delenv('OPENSYSML_GRPC_VERSION', raising=False)
    monkeypatch.delenv('OPENSYSML_ALLOW_UNPINNED_DOWNLOAD', raising=False)
    monkeypatch.setattr('opensysml.binary.PINNED_SHA256', {})

    def serve(assets):
        def urlopen(url, timeout=None):
            asset = str(url).rsplit('/', 1)[-1]
            served = assets.get(asset)
            if served is None:
                raise urllib.error.HTTPError(url, 404, 'Not Found', None, None)
            if isinstance(served, Exception):
                raise served
            return _Response(served)

        monkeypatch.setattr('urllib.request.urlopen', urlopen)
        return assets

    return serve


class _Response:
    """The little of a urlopen response that the download path uses."""

    def __init__(self, content):
        self._content = content

    def __enter__(self):
        return self

    def __exit__(self, *exc):
        return False

    def read(self, size=None):
        """The body served, at most as much of it as was asked for.

        Args:
            size (int, optional): Most bytes to return

        Returns:
            bytes: Content
        """
        return self._content if size is None else self._content[:size]


def served_release(binary=None, manifest=None, bundle=None):
    """The assets a release publishes, as the fake origin serves them.

    Args:
        binary (bytes, optional): Content served for the platform binary
        manifest (bytes, optional): Content served for SHA256SUMS.txt
        bundle (bytes, optional): Content served for its sigstore bundle

    Returns:
        dict: Asset name to content served
    """
    binary = fixture(BINARY_ASSET) if binary is None else binary
    assets = {
        BINARY_ASSET: binary,
        f'{BINARY_ASSET}.sha256': (
            f'{hashlib.sha256(binary).hexdigest()}  {BINARY_ASSET}\n'.encode()
        ),
    }
    if manifest is not None:
        assets[MANIFEST_ASSET] = manifest
    if bundle is not None:
        assets[BUNDLE_ASSET] = bundle
    return assets


class TestTheIdentityPinned:
    """The identity a release's signature must carry."""

    def test_the_release_repository_pins_a_circleci_organization_and_project(self):
        """The trust anchor is the pipeline, so both halves of it are pinned."""
        pinned = signer_for(REPO)
        assert pinned.issuer.startswith('https://oidc.circleci.com/org/')
        assert pinned.project.startswith('https://circleci.com/api/v2/projects/')

    def test_no_identity_is_known_for_another_repository(self):
        """A fork signs nothing this client can check, so it vouches for nothing."""
        assert signer_for('someone/fork') is None

    def test_a_repository_without_an_identity_verifies_nothing(self, monkeypatch):
        """Not knowing whose signature to expect refuses rather than accepting any."""
        monkeypatch.setattr('opensysml.binary.PINNED_SHA256', {})
        with pytest.raises(UnsignedReleaseError, match='knows no release pipeline'):
            signed_manifest_digest(VERSION, BINARY_ASSET, 'someone/fork')

    def test_a_pinned_definition_is_the_subject_expected(self, identity):
        """Naming a pipeline definition pins the certificate subject exactly."""
        pinned = ReleaseSigner(
            issuer=identity['issuer'],
            project=identity['project'],
            definition=identity['definition'],
        )
        assert pinned.subject == (
            f"{identity['project']}/pipeline-definitions/{identity['definition']}"
        )

    def test_no_pinned_definition_pins_the_project_it_must_belong_to(self, identity):
        """Any definition of the project is accepted; nothing outside it is."""
        assert signer_for(REPO).subject is None
        assert signer_for(REPO).project in signer_for(REPO).describe()


class TestVerifyingAManifest:
    """What a recorded bundle does and does not vouch for."""

    def test_a_signed_manifest_is_accepted(self, signer, manifest, bundle):
        """The recorded release pipeline's signature verifies over its manifest."""
        verify_manifest(manifest, bundle, signer)

    def test_the_digest_comes_from_the_verified_manifest(
        self, signer, manifest, bundle
    ):
        """The asset's digest is read out of the manifest the signature covers."""
        digest = verified_manifest_digest(manifest, bundle, BINARY_ASSET, signer)
        assert digest == hashlib.sha256(fixture(BINARY_ASSET)).hexdigest()

    def test_the_pinned_pipeline_definition_is_accepted(
        self, identity, manifest, bundle
    ):
        """Pinning the exact subject verifies the same signature."""
        exact = ReleaseSigner(
            issuer=identity['issuer'],
            project=identity['project'],
            definition=identity['definition'],
            trusted_root=os.path.join(FIXTURES, 'trusted_root.json'),
        )
        verify_manifest(manifest, bundle, exact)

    def test_another_pipeline_definition_is_rejected(
        self, identity, manifest, bundle
    ):
        """A signature from another pipeline of the same project is not this one's."""
        other = ReleaseSigner(
            issuer=identity['issuer'],
            project=identity['project'],
            definition='00000000-0000-4000-8000-000000000000',
            trusted_root=os.path.join(FIXTURES, 'trusted_root.json'),
        )
        with pytest.raises(ManifestSignatureError, match='does not verify'):
            verify_manifest(manifest, bundle, other)

    def test_another_identity_is_rejected(self, signer, manifest):
        """Another organization's pipeline cannot vouch for this project's release."""
        other = fixture('SHA256SUMS.txt.other-identity.bundle')
        with pytest.raises(ManifestSignatureError, match='does not verify'):
            verify_manifest(manifest, other, signer)

    def test_a_tampered_manifest_is_rejected(self, signer, manifest, bundle):
        """A digest changed after signing is what the signature exists to catch."""
        tampered = manifest.replace(
            hashlib.sha256(fixture(BINARY_ASSET)).hexdigest().encode(), b'ab' * 32
        )
        assert tampered != manifest
        with pytest.raises(ManifestSignatureError, match='does not verify'):
            verify_manifest(tampered, bundle, signer)

    def test_an_expired_certificate_is_rejected(self, signer, manifest):
        """A certificate that had expired when the log recorded the entry is refused."""
        expired = fixture('SHA256SUMS.txt.expired.bundle')
        with pytest.raises(ManifestSignatureError, match='does not verify'):
            verify_manifest(manifest, expired, signer)

    def test_a_tampered_bundle_is_rejected(self, signer, manifest, bundle):
        """Replacing the signature inside the bundle does not make it verify."""
        inner = json.loads(bundle)
        inner['messageSignature']['signature'] = (
            'MEQCIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAiAAAAAAAAAAAAAAAAA'
            'AAAAAAAAAAAAAAAAAAAAAAAAAAA=='
        )
        resigned = json.dumps(inner).encode()
        with pytest.raises(ManifestSignatureError, match='does not verify'):
            verify_manifest(manifest, resigned, signer)

    def test_an_unreadable_bundle_verifies_nothing(self, signer, manifest):
        """A truncated download is not evidence, so it refuses as an unsigned one."""
        with pytest.raises(UnsignedReleaseError, match='could not be read'):
            verify_manifest(manifest, b'{"mediaType": "nonsense"}', signer)

    def test_sigstore_being_unavailable_verifies_nothing(
        self, signer, manifest, bundle, monkeypatch
    ):
        """An install without the verifier refuses rather than skipping the check."""
        monkeypatch.setitem(sys.modules, 'sigstore.models', None)
        with pytest.raises(UnsignedReleaseError, match='cannot verify the signature'):
            verify_manifest(manifest, bundle, signer)

    def test_an_asset_the_manifest_does_not_cover_is_refused(
        self, signer, manifest, bundle
    ):
        """A verified manifest vouches for the assets it lists, and no others.

        An old release whose manifest predates an asset is the case this covers.
        """
        with pytest.raises(UnsignedReleaseError, match='lists no SHA-256 digest'):
            verified_manifest_digest(
                manifest, bundle, 'sysml-grpc-windows-amd64.exe', signer
            )


class TestReadingAManifest:
    """Digests are taken out of the manifest as sha256sum writes it."""

    def test_the_digest_of_a_listed_asset_is_found(self):
        """A name is matched exactly, not by prefix or suffix."""
        listed = manifest_digest(
            b'ab' * 32 + b'  sysml-grpc-linux-amd64\n', 'sysml-grpc-linux-amd64'
        )
        assert listed == 'ab' * 32

    def test_a_binary_mode_entry_is_read(self):
        """sha256sum marks a file read in binary mode, which is not part of its name."""
        assert manifest_digest(b'cd' * 32 + b' *grpc\n', 'grpc') == 'cd' * 32

    def test_an_unlisted_asset_has_no_digest(self):
        """Nothing is guessed for an asset the manifest says nothing about."""
        assert manifest_digest(b'ab' * 32 + b'  other\n', 'grpc') is None

    def test_a_digest_that_is_not_one_is_not_returned(self):
        """A malformed entry is no digest, rather than a digest nothing can match."""
        assert manifest_digest(b'not-a-digest  grpc\n', 'grpc') is None

    def test_an_upper_case_digest_is_read_as_a_digest(self):
        """Case is not part of a hex digest, and comparisons here are lower case."""
        assert manifest_digest(b'AB' * 32 + b'  grpc\n', 'grpc') == 'ab' * 32


class TestPinsAndSignedManifestsTogether:
    """Which of the two answers for a release, and what disagreement means."""

    def test_a_verified_digest_stands_in_for_a_pin(self):
        """A signed manifest is as good as a pin, so no opt-in is needed."""
        verified = 'ab' * 32
        assert expected_digest(
            VERSION, BINARY_ASSET, verified, verified_digest=verified
        ) == verified

    def test_a_verified_digest_is_preferred_to_the_one_the_origin_serves(self):
        """The manifest is signed and the sidecar is not, so the manifest wins."""
        verified = 'ab' * 32
        assert expected_digest(
            VERSION, BINARY_ASSET, 'cd' * 32, verified_digest=verified
        ) == verified

    def test_a_pin_wins_over_a_verified_manifest_that_agrees(self, monkeypatch):
        """Where both vouch for the same digest, there is nothing to choose."""
        digest = 'ab' * 32
        monkeypatch.setattr(
            'opensysml.binary.PINNED_SHA256', {REPO: {VERSION: {BINARY_ASSET: digest}}}
        )
        assert expected_digest(
            VERSION, BINARY_ASSET, digest, verified_digest=digest
        ) == digest

    def test_a_pin_contradicted_by_a_verified_manifest_is_an_error(self, monkeypatch):
        """A release rebuilt after being pinned is reported, not silently downgraded."""
        monkeypatch.setattr(
            'opensysml.binary.PINNED_SHA256',
            {REPO: {VERSION: {BINARY_ASSET: 'ab' * 32}}},
        )
        with pytest.raises(ChecksumMismatchError, match='but opensysml pins') as exc:
            expected_digest(
                VERSION, BINARY_ASSET, 'ab' * 32, verified_digest='cd' * 32
            )
        assert not exc.value.unpinned

    def test_nothing_verified_and_nothing_pinned_says_why(self, monkeypatch):
        """The refusal names both halves: no pin, and why the signature was no help."""
        monkeypatch.setattr('opensysml.binary.PINNED_SHA256', {})
        monkeypatch.delenv('OPENSYSML_ALLOW_UNPINNED_DOWNLOAD', raising=False)
        with pytest.raises(UnpinnedReleaseError) as exc:
            expected_digest(
                VERSION, BINARY_ASSET, 'ab' * 32,
                unverified_reason='publishes no readable SHA256SUMS.txt.bundle',
            )
        assert 'pins no SHA-256 digest' in str(exc.value)
        assert 'publishes no readable SHA256SUMS.txt.bundle' in str(exc.value)


class TestDownloadingAnUnpinnedRelease:
    """What the download path does for a release published after this client."""

    def test_a_signed_release_installs_without_a_pin(self, signer, release):
        """A release newer than this opensysml installs on its signature alone."""
        assets = served_release(manifest=fixture(MANIFEST_ASSET), bundle=fixture(BUNDLE_ASSET))
        release(assets)
        binary_path = download_binary(version=VERSION)

        with open(binary_path, 'rb') as f:
            assert f.read() == assets[BINARY_ASSET]
        recorded = read_metadata()
        assert recorded['version'] == VERSION
        assert recorded['sha256'] == hashlib.sha256(assets[BINARY_ASSET]).hexdigest()

    def test_a_binary_that_is_not_the_signed_one_is_refused(self, signer, release):
        """The signed manifest is what the download is checked against."""
        tampered = b'not the binary that was signed\n'
        assets = served_release(
            manifest=fixture(MANIFEST_ASSET), bundle=fixture(BUNDLE_ASSET)
        )
        assets[BINARY_ASSET] = tampered
        release(assets)
        with pytest.raises(ChecksumMismatchError, match='Checksum mismatch'):
            download_binary(version=VERSION)

    def test_a_binary_whose_sidecar_agrees_with_it_is_still_refused(
        self, signer, release
    ):
        """A republished release serving a matching sidecar does not vouch for itself."""
        tampered = b'not the binary that was signed\n'
        assets = served_release(
            binary=tampered,
            manifest=fixture(MANIFEST_ASSET),
            bundle=fixture(BUNDLE_ASSET),
        )
        release(assets)
        with pytest.raises(ChecksumMismatchError, match='Checksum mismatch'):
            download_binary(version=VERSION)

    def test_a_release_without_a_bundle_is_refused(self, signer, release):
        """An old release, published before the pipeline signed anything."""
        release(served_release(manifest=fixture(MANIFEST_ASSET)))
        with pytest.raises(UnpinnedReleaseError) as exc:
            download_binary(version=VERSION)
        assert 'pins no SHA-256 digest' in str(exc.value)
        assert BUNDLE_ASSET in str(exc.value)

    def test_a_release_without_a_manifest_is_refused(self, signer, release):
        """Nothing to verify is nothing to trust, whatever the sidecar says."""
        release(served_release())
        with pytest.raises(UnpinnedReleaseError, match='pins no SHA-256 digest'):
            download_binary(version=VERSION)

    def test_a_manifest_that_cannot_be_fetched_is_refused(self, signer, release):
        """A truncated manifest download is refused rather than half-trusted."""
        assets = served_release(bundle=fixture(BUNDLE_ASSET))
        assets[MANIFEST_ASSET] = http.client.IncompleteRead(b'')
        release(assets)
        with pytest.raises(UnpinnedReleaseError, match='pins no SHA-256 digest'):
            download_binary(version=VERSION)

    def test_a_tampered_manifest_refuses_the_download(self, signer, release):
        """Verification failure is evidence, so it is a mismatch, not an absence."""
        manifest = fixture(MANIFEST_ASSET)
        tampered = manifest.replace(
            hashlib.sha256(fixture(BINARY_ASSET)).hexdigest().encode(), b'ab' * 32
        )
        release(served_release(manifest=tampered, bundle=fixture(BUNDLE_ASSET)))
        with pytest.raises(ManifestSignatureError, match='does not verify'):
            download_binary(version=VERSION)

    def test_a_pin_contradicting_the_signed_manifest_refuses_the_download(
        self, signer, release, monkeypatch
    ):
        """Both vouch for a digest and they disagree: the release was rebuilt."""
        release(served_release(manifest=fixture(MANIFEST_ASSET), bundle=fixture(BUNDLE_ASSET)))
        monkeypatch.setattr(
            'opensysml.binary.PINNED_SHA256',
            {REPO: {VERSION: {BINARY_ASSET: 'ab' * 32}}},
        )
        with pytest.raises(ChecksumMismatchError, match='but opensysml pins'):
            download_binary(version=VERSION)

    def test_a_pinned_release_does_not_need_the_manifest(self, signer, release):
        """Pinned releases install exactly as before, fetching no signature."""
        binary = fixture(BINARY_ASSET)
        digest = hashlib.sha256(binary).hexdigest()
        assets = served_release()
        release(assets)
        with patch('opensysml.binary.PINNED_SHA256', {REPO: {VERSION: {BINARY_ASSET: digest}}}):
            with patch('opensysml.binary.signed_manifest_digest') as manifest_call:
                assert download_binary(version=VERSION)
                manifest_call.assert_not_called()

    def test_the_same_origin_opt_in_is_not_needed_for_a_signed_release(
        self, signer, release, monkeypatch
    ):
        """Signed releases install without being trusted on their own word."""
        monkeypatch.delenv('OPENSYSML_ALLOW_UNPINNED_DOWNLOAD', raising=False)
        release(served_release(manifest=fixture(MANIFEST_ASSET), bundle=fixture(BUNDLE_ASSET)))
        assert download_binary(version=VERSION)


def test_the_cache_records_a_signed_release_like_any_other(signer, release):
    """A signed release is recorded, so a later run can tell what the cache holds."""
    release(served_release(manifest=fixture(MANIFEST_ASSET), bundle=fixture(BUNDLE_ASSET)))
    download_binary(version=VERSION)
    assert read_metadata()['repo'] == REPO


def test_release_asset_names_are_unchanged_by_signing():
    """The manifest covers the assets the client already asks for by name."""
    listed = fixture(MANIFEST_ASSET).decode()
    assert release_asset_name('linux', 'amd64') in listed


def test_write_metadata_is_untouched_by_verification(tmp_path, monkeypatch):
    """The record beside the cache is the same one, whatever vouched for the digest."""
    monkeypatch.setattr(
        'opensysml.binary.get_binary_path', lambda: str(tmp_path / 'sysml-grpc')
    )
    write_metadata(VERSION, 'ab' * 32, REPO)
    assert read_metadata() == {
        'version': VERSION, 'sha256': 'ab' * 32, 'repo': REPO
    }

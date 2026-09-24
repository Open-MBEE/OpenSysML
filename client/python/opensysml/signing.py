"""Verification of the signed checksum manifest a release publishes.

A core release signs its ``SHA256SUMS.txt`` in CI with cosign keyless, using the
release pipeline's CircleCI OIDC identity, and publishes the sigstore bundle
beside it as ``SHA256SUMS.txt.bundle``. A bundle that verifies against the
identity pinned here says the manifest was produced by that pipeline, so its
digests are trustworthy without this client having written them down: a release
published after this client can still be installed. Nothing else changes — a
manifest that does not verify is refused exactly as an unpinned release is.

The ``sigstore`` package is imported on demand rather than at module import, so
an install without it refuses to verify rather than failing to import.
"""

import re

from opensysml.errors import ManifestSignatureError, UnsignedReleaseError

#: Manifest of every published artifact's digest, and its sigstore bundle.
MANIFEST_ASSET = 'SHA256SUMS.txt'
BUNDLE_ASSET = MANIFEST_ASSET + '.bundle'

#: A CircleCI signing certificate's subject names the pipeline definition that
#: produced it, under the project it belongs to.
_PIPELINE_DEFINITIONS = '{project}/pipeline-definitions/'
_DEFINITION_ID = re.compile(r'[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}\Z')

_SHA256 = re.compile(r'[0-9a-f]{64}\Z')


class _Sigstore:
    """The sigstore and cryptography API this module verifies with.

    Kept in one place because it is imported on demand: an install without the
    verifier must refuse to verify, not fail to import opensysml.
    """

    def __init__(self):
        from cryptography.x509 import (
            SubjectAlternativeName, UniformResourceIdentifier,
        )
        from sigstore.errors import Error, VerificationError
        from sigstore.models import Bundle, TrustedRoot
        from sigstore.verify import Verifier, policy

        self.subject_alternative_name = SubjectAlternativeName
        self.uniform_resource_identifier = UniformResourceIdentifier
        self.error = Error
        self.verification_error = VerificationError
        self.bundle = Bundle
        self.trusted_root = TrustedRoot
        self.verifier = Verifier
        self.policy = policy


def _load_sigstore():
    """The verifier API, or a refusal explaining that it is not installed.

    Returns:
        _Sigstore: The API this module verifies with

    Raises:
        UnsignedReleaseError: If it cannot be imported, so nothing can be verified
    """
    try:
        return _Sigstore()
    except ImportError as e:
        raise UnsignedReleaseError(
            f"opensysml cannot verify the signature on {MANIFEST_ASSET}: {e}. "
            f"Install opensysml's sigstore dependency, or ask for a release this "
            f"opensysml pins a digest for with version=."
        )


class _PipelineDefinitionOfProject:
    """Accepts any pipeline definition of one CircleCI project.

    A CircleCI signing certificate's subject is the pipeline definition that
    produced it, whose identifier no unauthenticated API publishes, so a client
    that cannot read it off a signature pins the project it must belong to.
    """

    def __init__(self, project, sigstore):
        self._prefix = _PIPELINE_DEFINITIONS.format(project=project)
        self._sigstore = sigstore

    def verify(self, cert):
        """Verify the certificate names a pipeline definition of the project.

        Args:
            cert (cryptography.x509.Certificate): Signing certificate

        Raises:
            sigstore.errors.VerificationError: If no subject names one
        """
        san = cert.extensions.get_extension_for_class(
            self._sigstore.subject_alternative_name
        ).value
        subjects = san.get_values_for_type(
            self._sigstore.uniform_resource_identifier
        )
        for subject in subjects:
            if subject.startswith(self._prefix):
                if _DEFINITION_ID.match(subject[len(self._prefix):]):
                    return
        raise self._sigstore.verification_error(
            f"Certificate's SANs name no pipeline definition of "
            f"{self._prefix.rstrip('/')}; actual SANs: {sorted(subjects)}"
        )


class ReleaseSigner:
    """The identity a release manifest's signature must carry.

    Attributes:
        issuer (str): OIDC issuer the certificate must have been issued for
        project (str): CircleCI project API URL the subject must belong to
        definition (str or None): Pipeline definition identifier the subject
            must name exactly; when None any definition of ``project`` is
            accepted
        trusted_root (str or None): Path to a sigstore trusted root to verify
            against, or None for Sigstore's production instance
    """

    def __init__(self, issuer, project, definition=None, trusted_root=None):
        self.issuer = issuer
        self.project = project
        self.definition = definition
        self.trusted_root = trusted_root

    @property
    def subject(self):
        """Certificate subject required, or None when any definition is accepted.

        Returns:
            str or None: Pipeline definition URL
        """
        if self.definition is None:
            return None
        return _PIPELINE_DEFINITIONS.format(project=self.project) + self.definition

    def describe(self):
        """The identity, for an error message.

        Returns:
            str: Issuer and subject required
        """
        subject = self.subject or (
            _PIPELINE_DEFINITIONS.format(project=self.project) + '*'
        )
        return f"issuer {self.issuer}, subject {subject}"

    def policy(self, sigstore):
        """The sigstore verification policy for this identity.

        Args:
            sigstore (_Sigstore): Verifier API to build the policy from

        Returns:
            sigstore.verify.policy.VerificationPolicy: Policy to verify with
        """
        checks = [sigstore.policy.OIDCIssuerV2(self.issuer)]
        if self.definition is None:
            checks.append(_PipelineDefinitionOfProject(self.project, sigstore))
        else:
            checks.append(sigstore.policy.Identity(identity=self.subject))
        return sigstore.policy.AllOf(checks)

    def verifier(self, sigstore):
        """A verifier holding the root of trust this signer's certificates chain to.

        Args:
            sigstore (_Sigstore): Verifier API to build the verifier from

        Returns:
            sigstore.verify.Verifier: Verifier to verify with
        """
        if self.trusted_root is None:
            return sigstore.verifier.production()
        return sigstore.verifier(
            trusted_root=sigstore.trusted_root.from_file(self.trusted_root)
        )


#: Identity the signature on each repository's release manifest must carry. The
#: issuer is CircleCI's OIDC issuer for the organization that owns the release
#: pipeline and the project is its CircleCI project, both as CircleCI's API
#: reports them; ``definition`` pins one pipeline of that project once its
#: identifier is known. A repository absent here signs nothing this client can
#: check, so its unpinned releases are refused as before.
SIGNED_MANIFEST_SIGNERS = {
    'Open-MBEE/OpenSysML': ReleaseSigner(
        issuer='https://oidc.circleci.com/org/1169df8b-0b59-400f-82d2-c9d8e98bdb62',
        project='https://circleci.com/api/v2/projects/eeb0dddd-237f-4f02-9e51-8e24caef589d',
    ),
}


def signer_for(github_repo):
    """The signer whose signature a repository's release manifest must carry.

    Args:
        github_repo (str): GitHub repository (owner/repo)

    Returns:
        ReleaseSigner or None: The signer, or None when the repository publishes
        no signature this client knows how to check
    """
    return SIGNED_MANIFEST_SIGNERS.get(github_repo)


def manifest_digest(manifest, asset):
    """The digest a checksum manifest lists for an asset.

    Args:
        manifest (bytes): Manifest content, as ``sha256sum`` writes it
        asset (str): Asset name (e.g. 'sysml-grpc-linux-amd64')

    Returns:
        str or None: SHA-256 hex digest, or None when the manifest lists no
        well-formed digest for that asset
    """
    for line in manifest.decode('utf-8', 'replace').splitlines():
        fields = line.split()
        if len(fields) != 2:
            continue
        digest, name = fields
        # sha256sum marks a file it read in binary mode with a leading '*'.
        if name.lstrip('*') != asset:
            continue
        digest = digest.lower()
        return digest if _SHA256.match(digest) else None
    return None


def verify_manifest(manifest, bundle, signer):
    """Verify a manifest's sigstore bundle against the signer expected.

    Args:
        manifest (bytes): Manifest content that was signed
        bundle (bytes): Sigstore bundle published beside it
        signer (ReleaseSigner): Identity the signature must carry

    Raises:
        UnsignedReleaseError: If the bundle cannot be read, or sigstore is not
            installed or cannot load its root of trust — no signature was
            checked either way, so nothing is known about the manifest
        ManifestSignatureError: If the bundle was read and does not verify:
            another signer, an expired certificate, or a manifest that was
            changed after it was signed
    """
    sigstore = _load_sigstore()

    try:
        read = sigstore.bundle.from_json(bundle)
    except (sigstore.error, ValueError) as e:
        # A truncated download looks like this too, so it is an absent
        # signature rather than evidence of a tampered one.
        raise UnsignedReleaseError(
            f"the sigstore bundle published for {MANIFEST_ASSET} could not be read "
            f"({type(e).__name__}: {e}), so the manifest was not verified."
        )

    try:
        verifier = signer.verifier(sigstore)
    except Exception as e:
        # Whatever keeps the root of trust from loading, nothing was verified.
        raise UnsignedReleaseError(
            f"opensysml could not load the sigstore root of trust needed to verify "
            f"{MANIFEST_ASSET} ({type(e).__name__}: {e}), so the manifest was not "
            f"verified."
        )

    try:
        verifier.verify_artifact(manifest, read, signer.policy(sigstore))
    except (sigstore.error, ValueError) as e:
        raise ManifestSignatureError(
            f"the signature on {MANIFEST_ASSET} does not verify against the release "
            f"pipeline of this repository ({signer.describe()}): {e}. The manifest, "
            f"the signature, or both were replaced; nothing was installed."
        )


def verified_manifest_digest(manifest, bundle, asset, signer):
    """The digest a signed manifest lists for an asset, once verified.

    Args:
        manifest (bytes): Manifest content downloaded from the release
        bundle (bytes): Sigstore bundle downloaded beside it
        asset (str): Asset name (e.g. 'sysml-grpc-linux-amd64')
        signer (ReleaseSigner): Identity the signature must carry

    Returns:
        str: SHA-256 hex digest, from a manifest signed by that pipeline

    Raises:
        UnsignedReleaseError: If nothing was verified, or the verified manifest
            lists no digest for the asset
        ManifestSignatureError: If the signature does not verify
    """
    verify_manifest(manifest, bundle, signer)
    digest = manifest_digest(manifest, asset)
    if digest is None:
        raise UnsignedReleaseError(
            f"the signed {MANIFEST_ASSET} of this release lists no SHA-256 digest "
            f"for {asset}, so that asset is not covered by the signature."
        )
    return digest

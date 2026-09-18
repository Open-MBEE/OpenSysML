#!/usr/bin/env python3
"""Record the signed-release fixtures clients/python/tests/test_signing.py verifies against.

The client verifies the sigstore bundle a release publishes for its
SHA256SUMS.txt, so its tests need bundles — and they must not reach the network,
which rules out fetching one from Sigstore's public instances at test time. This
records the whole thing offline instead: a root of trust (a Fulcio CA, a CT log
key and a Rekor key), a signing certificate carrying the CircleCI OIDC identity
a release would carry, and bundles that sigstore verifies against that root
exactly as it verifies a production one. Every signature is real; only the
instance that issued them is local, so the crypto and the bundle format the
client parses are the real ones.

The Node client verifies the same fixtures, recorded into its own tree with --out.

Run it to re-record the fixtures after changing what they must contain:

    python clients/python/scripts/make_signed_release_fixture.py
    python clients/python/scripts/make_signed_release_fixture.py \
        --out clients/node/test/fixtures/signed_release

The fixtures are committed, so this is not run by the test suite.
"""

import argparse
import base64
import datetime
import hashlib
import json
import os
import struct
import sys

import rfc8785
from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives.asymmetric.utils import Prehashed
from cryptography.x509.oid import ExtendedKeyUsageOID, NameOID

# The pipeline identity the fixtures are signed with: the issuer and project of
# the real release pipeline, and a placeholder definition, since no signature of
# the real one exists yet.
ISSUER = 'https://oidc.circleci.com/org/1169df8b-0b59-400f-82d2-c9d8e98bdb62'
PROJECT = 'https://circleci.com/api/v2/projects/eeb0dddd-237f-4f02-9e51-8e24caef589d'
DEFINITION = 'c3d1a44e-6cb7-4a2f-8f60-2b1d0e3f9a15'

# Another organization's pipeline, for the fixture a wrong identity is rejected on.
OTHER_ISSUER = 'https://oidc.circleci.com/org/00000000-0000-4000-8000-000000000000'
OTHER_PROJECT = 'https://circleci.com/api/v2/projects/11111111-1111-4111-8111-111111111111'
OTHER_DEFINITION = '22222222-2222-4222-8222-222222222222'

# The fixture log's base URL, whose host is also its checkpoint origin: a verifier
# derives the origin it expects from the base URL, so the two must agree.
REKOR_BASE_URL = 'https://rekor.fixture.invalid'
CHECKPOINT_ORIGIN = 'rekor.fixture.invalid'

# Fixed times, so a re-recorded fixture verifies the same way: nothing in
# verification is compared against the current time, and a signature's validity
# is established by the log's integrated time, not by the clock.
SIGNED_AT = datetime.datetime(2026, 1, 1, 0, 0, 0, tzinfo=datetime.timezone.utc)
CERT_LIFETIME = datetime.timedelta(minutes=10)
INTEGRATED_AT = SIGNED_AT + datetime.timedelta(minutes=1)
CA_LIFETIME = datetime.timedelta(days=3650)

# The release assets the recorded manifest covers.
FIXTURE_ASSETS = {
    'opensysml-v9.9.9-linux-amd64.tar.gz': b'source archive fixture\n',
    'sysml-grpc-linux-amd64': b'sysml-grpc fixture binary\n',
    'sysml-grpc-darwin-arm64': b'sysml-grpc fixture binary, another platform\n',
}
BINARY_ASSET = 'sysml-grpc-linux-amd64'

_SCT_LIST_OID = x509.ObjectIdentifier('1.3.6.1.4.1.11129.2.4.2')
_OIDC_ISSUER_V2_OID = x509.ObjectIdentifier('1.3.6.1.4.1.57264.1.8')


def _der_utf8_string(value):
    """DER encoding of an ASN.1 UTF8String.

    Args:
        value (str): String to encode

    Returns:
        bytes: DER encoding
    """
    return _der_tagged(0x0C, value.encode('utf-8'))


def _der_octet_string(payload):
    """DER encoding of an ASN.1 OCTET STRING.

    Args:
        payload (bytes): Contents to wrap

    Returns:
        bytes: DER encoding
    """
    return _der_tagged(0x04, payload)


def _der_tagged(tag, payload):
    """DER encoding of a primitive value with definite length.

    Args:
        tag (int): ASN.1 tag byte
        payload (bytes): Contents

    Returns:
        bytes: DER encoding
    """
    length = len(payload)
    if length < 0x80:
        header = bytes([tag, length])
    else:
        encoded = length.to_bytes((length.bit_length() + 7) // 8, 'big')
        header = bytes([tag, 0x80 | len(encoded)]) + encoded
    return header + payload


def _spki(key):
    """DER SubjectPublicKeyInfo of a public key.

    Args:
        key: Public key

    Returns:
        bytes: DER encoding
    """
    return key.public_bytes(
        encoding=serialization.Encoding.DER,
        format=serialization.PublicFormat.SubjectPublicKeyInfo,
    )


def _key_id(key):
    """The identifier sigstore keys a public key by.

    Args:
        key: Public key

    Returns:
        bytes: SHA-256 of its DER SubjectPublicKeyInfo
    """
    return hashlib.sha256(_spki(key)).digest()


def _b64(raw):
    """Base64, as protobuf's JSON mapping encodes bytes.

    Args:
        raw (bytes): Bytes to encode

    Returns:
        str: Base64 encoding
    """
    return base64.b64encode(raw).decode('ascii')


def _certificate_authority(name):
    """A self-signed CA to stand in for Fulcio's.

    Args:
        name (str): Common name to issue it under

    Returns:
        tuple: (private key, certificate)
    """
    key = ec.generate_private_key(ec.SECP256R1())
    subject = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, name)])
    cert = (
        x509.CertificateBuilder()
        .subject_name(subject)
        .issuer_name(subject)
        .public_key(key.public_key())
        .serial_number(x509.random_serial_number())
        .not_valid_before(SIGNED_AT - CA_LIFETIME)
        .not_valid_after(SIGNED_AT + CA_LIFETIME)
        .add_extension(x509.BasicConstraints(ca=True, path_length=None), critical=True)
        .add_extension(
            x509.KeyUsage(
                digital_signature=False,
                content_commitment=False,
                key_encipherment=False,
                data_encipherment=False,
                key_agreement=False,
                key_cert_sign=True,
                crl_sign=True,
                encipher_only=False,
                decipher_only=False,
            ),
            critical=True,
        )
        .add_extension(
            x509.SubjectKeyIdentifier.from_public_key(key.public_key()), critical=False
        )
        .sign(key, hashes.SHA256())
    )
    return key, cert


def _leaf_certificate(ca_key, ca_cert, identity, issuer, ct_key, not_valid_after):
    """A signing certificate as Fulcio issues them, with an embedded SCT.

    Args:
        ca_key: CA private key
        ca_cert (x509.Certificate): CA certificate
        identity (str): Subject the certificate attests, as a URI SAN
        issuer (str): OIDC issuer, as Fulcio's issuer extension
        ct_key: CT log private key, which signs the embedded timestamp
        not_valid_after (datetime): Expiry

    Returns:
        tuple: (private key, certificate)
    """
    key = ec.generate_private_key(ec.SECP256R1())
    serial = x509.random_serial_number()

    def build(sct_list):
        return (
            x509.CertificateBuilder()
            .subject_name(x509.Name([]))
            .issuer_name(ca_cert.subject)
            .public_key(key.public_key())
            .serial_number(serial)
            .not_valid_before(SIGNED_AT)
            .not_valid_after(not_valid_after)
            .add_extension(
                x509.KeyUsage(
                    digital_signature=True,
                    content_commitment=False,
                    key_encipherment=False,
                    data_encipherment=False,
                    key_agreement=False,
                    key_cert_sign=False,
                    crl_sign=False,
                    encipher_only=False,
                    decipher_only=False,
                ),
                critical=True,
            )
            .add_extension(
                x509.ExtendedKeyUsage([ExtendedKeyUsageOID.CODE_SIGNING]),
                critical=False,
            )
            .add_extension(x509.BasicConstraints(ca=False, path_length=None), critical=True)
            .add_extension(
                x509.SubjectAlternativeName(
                    [x509.UniformResourceIdentifier(identity)]
                ),
                critical=True,
            )
            .add_extension(
                x509.UnrecognizedExtension(
                    _OIDC_ISSUER_V2_OID, _der_utf8_string(issuer)
                ),
                critical=False,
            )
            .add_extension(
                x509.SubjectKeyIdentifier.from_public_key(key.public_key()),
                critical=False,
            )
            .add_extension(
                x509.AuthorityKeyIdentifier.from_issuer_public_key(
                    ca_cert.public_key()
                ),
                critical=False,
            )
            .add_extension(
                x509.UnrecognizedExtension(_SCT_LIST_OID, sct_list), critical=False
            )
            .sign(ca_key, hashes.SHA256())
        )

    # The timestamp is signed over the certificate with its own SCT extension
    # stripped, so a placeholder is enough to derive the bytes to sign.
    placeholder = build(_der_octet_string(struct.pack('!H', 0)))
    precert = x509.load_der_x509_certificate(
        placeholder.public_bytes(serialization.Encoding.DER)
    )
    sct_list = _signed_certificate_timestamp(precert, ca_cert, ct_key)
    return key, build(sct_list)


def _signed_certificate_timestamp(precert, ca_cert, ct_key):
    """The SCT list extension a CT log would embed in a certificate.

    Args:
        precert (x509.Certificate): Certificate whose precertificate is logged
        ca_cert (x509.Certificate): Its issuer, bound into the signature
        ct_key: CT log private key

    Returns:
        bytes: DER extension value holding one SCT
    """
    log_id = _key_id(ct_key.public_key())
    timestamp_ms = int(SIGNED_AT.timestamp() * 1000)
    tbs = precert.tbs_precertificate_bytes
    issuer_key_id = _key_id(ca_cert.public_key())

    # RFC 6962 digitally-signed over the precertificate entry.
    signed = struct.pack(
        f'!BBQH32sBBB{len(tbs)}sH',
        0,  # sct_version: v1
        0,  # signature_type: certificate_timestamp
        timestamp_ms,
        1,  # entry_type: precert_entry
        issuer_key_id,
        *_u24(len(tbs)),
        tbs,
        0,  # extensions length
    )
    signature = ct_key.sign(signed, ec.ECDSA(hashes.SHA256()))

    sct = struct.pack(
        f'!B32sQHBBH{len(signature)}s',
        0,  # sct_version: v1
        log_id,
        timestamp_ms,
        0,  # extensions length
        4,  # hash algorithm: sha256
        3,  # signature algorithm: ecdsa
        len(signature),
        signature,
    )
    entry = struct.pack(f'!H{len(sct)}s', len(sct), sct)
    return _der_octet_string(struct.pack(f'!H{len(entry)}s', len(entry), entry))


def _u24(length):
    """A length as the three bytes TLS encodes a u24 in.

    Args:
        length (int): Length below 2^24

    Returns:
        tuple: Three ints, most significant first
    """
    return struct.unpack('!4B', struct.pack('!I', length))[1:]


def _hashedrekord_body(cert, signature, digest):
    """The Rekor entry body a hashedrekord/0.0.1 log entry canonicalizes.

    Args:
        cert (x509.Certificate): Signing certificate
        signature (bytes): Signature over the artifact digest
        digest (bytes): SHA-256 of the artifact

    Returns:
        bytes: Canonical JSON body
    """
    pem = cert.public_bytes(serialization.Encoding.PEM)
    body = {
        'apiVersion': '0.0.1',
        'kind': 'hashedrekord',
        'spec': {
            'data': {'hash': {'algorithm': 'sha256', 'value': digest.hex()}},
            'signature': {
                'content': _b64(signature),
                'publicKey': {'content': _b64(pem)},
            },
        },
    }
    return json.dumps(body, separators=(',', ':'), sort_keys=True).encode('utf-8')


def _checkpoint(rekor_key, root_hash, tree_size):
    """A signed checkpoint over a log root, as Rekor publishes them.

    Args:
        rekor_key: Rekor private key
        root_hash (bytes): Merkle root the checkpoint attests
        tree_size (int): Size of the log at that root

    Returns:
        str: Signed note envelope
    """
    note = f'{CHECKPOINT_ORIGIN} - 0\n{tree_size}\n{_b64(root_hash)}\n'
    signature = rekor_key.sign(note.encode('utf-8'), ec.ECDSA(hashes.SHA256()))
    key_id = _key_id(rekor_key.public_key())
    signed = _b64(key_id[:4] + signature)
    return f'{note}\n\u2014 {CHECKPOINT_ORIGIN} {signed}\n'


def _bundle(manifest, cert, signing_key, rekor_key, integrated_at=INTEGRATED_AT):
    """A sigstore bundle over a manifest, as cosign publishes one.

    Args:
        manifest (bytes): Manifest that is signed
        cert (x509.Certificate): Signing certificate
        signing_key: Its private key
        rekor_key: Rekor private key, which signs the log entry
        integrated_at (datetime): When the log integrated the entry

    Returns:
        bytes: Bundle, as JSON
    """
    digest = hashlib.sha256(manifest).digest()
    signature = signing_key.sign(digest, ec.ECDSA(Prehashed(hashes.SHA256())))
    body = _hashedrekord_body(cert, signature, digest)

    # A one-entry log: the root is the leaf hash, and the inclusion proof of the
    # only leaf needs no hashes.
    root_hash = hashlib.sha256(b'\x00' + body).digest()
    integrated_time = int(integrated_at.timestamp())
    log_id = _key_id(rekor_key.public_key())
    promise = rekor_key.sign(
        rfc8785.dumps(
            {
                'body': _b64(body),
                'integratedTime': integrated_time,
                'logID': log_id.hex(),
                'logIndex': 0,
            }
        ),
        ec.ECDSA(hashes.SHA256()),
    )

    bundle = {
        'mediaType': 'application/vnd.dev.sigstore.bundle.v0.3+json',
        'verificationMaterial': {
            'certificate': {
                'rawBytes': _b64(cert.public_bytes(serialization.Encoding.DER))
            },
            'tlogEntries': [
                {
                    'logIndex': '0',
                    'logId': {'keyId': _b64(log_id)},
                    'kindVersion': {'kind': 'hashedrekord', 'version': '0.0.1'},
                    'integratedTime': str(integrated_time),
                    'inclusionPromise': {'signedEntryTimestamp': _b64(promise)},
                    'inclusionProof': {
                        'logIndex': '0',
                        'rootHash': _b64(root_hash),
                        'treeSize': '1',
                        'hashes': [],
                        'checkpoint': {
                            'envelope': _checkpoint(rekor_key, root_hash, 1)
                        },
                    },
                    'canonicalizedBody': _b64(body),
                }
            ],
        },
        'messageSignature': {
            'messageDigest': {'algorithm': 'SHA2_256', 'digest': _b64(digest)},
            'signature': _b64(signature),
        },
    }
    return json.dumps(bundle, indent=2).encode('utf-8') + b'\n'


def _trusted_root(ca_cert, ct_key, rekor_key):
    """A trusted root holding the fixture instance's keys.

    Args:
        ca_cert (x509.Certificate): Fulcio CA certificate
        ct_key: CT log private key
        rekor_key: Rekor private key

    Returns:
        bytes: Trusted root, as JSON
    """

    def public_key(key):
        return {
            'rawBytes': _b64(_spki(key.public_key())),
            'keyDetails': 'PKIX_ECDSA_P256_SHA_256',
            'validFor': {'start': _timestamp(SIGNED_AT - CA_LIFETIME)},
        }

    root = {
        'mediaType': 'application/vnd.dev.sigstore.trustedroot+json;version=0.1',
        'tlogs': [
            {
                'baseUrl': REKOR_BASE_URL,
                'hashAlgorithm': 'SHA2_256',
                'publicKey': public_key(rekor_key),
                'logId': {'keyId': _b64(_key_id(rekor_key.public_key()))},
            }
        ],
        'certificateAuthorities': [
            {
                'subject': {'organization': 'opensysml fixture', 'commonName': 'fulcio'},
                'uri': 'https://fulcio.fixture.invalid',
                'certChain': {
                    'certificates': [
                        {'rawBytes': _b64(ca_cert.public_bytes(serialization.Encoding.DER))}
                    ]
                },
                'validFor': {'start': _timestamp(SIGNED_AT - CA_LIFETIME)},
            }
        ],
        'ctlogs': [
            {
                'baseUrl': 'https://ctfe.fixture.invalid',
                'hashAlgorithm': 'SHA2_256',
                'publicKey': public_key(ct_key),
                'logId': {'keyId': _b64(_key_id(ct_key.public_key()))},
            }
        ],
        'timestampAuthorities': [],
    }
    return json.dumps(root, indent=2).encode('utf-8') + b'\n'


def _timestamp(when):
    """RFC 3339, as protobuf's JSON mapping encodes a timestamp.

    Args:
        when (datetime): Time to encode

    Returns:
        str: Encoded timestamp
    """
    return when.astimezone(datetime.timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ')


def _manifest():
    """A checksum manifest over the fixture's release assets.

    Returns:
        bytes: Manifest, as sha256sum writes it
    """
    lines = [
        f'{hashlib.sha256(content).hexdigest()}  {name}'
        for name, content in FIXTURE_ASSETS.items()
    ]
    return ('\n'.join(lines) + '\n').encode('utf-8')


def main():
    """Record the fixtures.

    Returns:
        int: Exit status
    """
    default = os.path.join(
        os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
        'tests', 'fixtures', 'signed_release',
    )
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument(
        '--out', default=default, help='directory to write the fixtures to'
    )
    args = parser.parse_args()
    os.makedirs(args.out, exist_ok=True)

    ca_key, ca_cert = _certificate_authority('opensysml fixture fulcio')
    ct_key = ec.generate_private_key(ec.SECP256R1())
    rekor_key = ec.generate_private_key(ec.SECP256R1())

    manifest = _manifest()
    subject = f'{PROJECT}/pipeline-definitions/{DEFINITION}'
    signing_key, cert = _leaf_certificate(
        ca_key, ca_cert, subject, ISSUER, ct_key, SIGNED_AT + CERT_LIFETIME
    )

    other_subject = f'{OTHER_PROJECT}/pipeline-definitions/{OTHER_DEFINITION}'
    other_key, other_cert = _leaf_certificate(
        ca_key, ca_cert, other_subject, OTHER_ISSUER, ct_key,
        SIGNED_AT + CERT_LIFETIME,
    )

    # A certificate that had already expired when the log integrated the entry,
    # which is how an expired signature presents itself.
    expired_key, expired_cert = _leaf_certificate(
        ca_key, ca_cert, subject, ISSUER, ct_key,
        INTEGRATED_AT - datetime.timedelta(seconds=1),
    )

    written = {
        'trusted_root.json': _trusted_root(ca_cert, ct_key, rekor_key),
        'SHA256SUMS.txt': manifest,
        'SHA256SUMS.txt.bundle': _bundle(manifest, cert, signing_key, rekor_key),
        'SHA256SUMS.txt.other-identity.bundle': _bundle(
            manifest, other_cert, other_key, rekor_key
        ),
        'SHA256SUMS.txt.expired.bundle': _bundle(
            manifest, expired_cert, expired_key, rekor_key
        ),
        BINARY_ASSET: FIXTURE_ASSETS[BINARY_ASSET],
        'identity.json': json.dumps(
            {
                'issuer': ISSUER,
                'project': PROJECT,
                'definition': DEFINITION,
                'other_issuer': OTHER_ISSUER,
                'other_project': OTHER_PROJECT,
            },
            indent=2,
        ).encode('utf-8') + b'\n',
    }
    for name, content in written.items():
        with open(os.path.join(args.out, name), 'wb') as f:
            f.write(content)
        print(f'wrote {os.path.join(args.out, name)}')
    return 0


if __name__ == '__main__':
    sys.exit(main())

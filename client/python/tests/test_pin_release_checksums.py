"""Tests for the release tooling that pins per-version asset digests.

The pins are what makes the download check independent of the serving origin, so
the script that produces them is held to hashing what it downloaded, refusing a
release whose sidecar disagrees, and syncing what it wrote into every client
that ships a copy.
"""

import importlib.util
import json
import os

import pytest

from opensysml.binary import PINNED_DIGESTS_FILE, PINNED_SHA256, pinned_digest

PYTHON_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PIN_SCRIPT = os.path.join(PYTHON_DIR, "scripts", "pin_release_checksums.py")


def _load_pin_script():
    """Load scripts/pin_release_checksums.py, which is tooling, not a module."""
    spec = importlib.util.spec_from_file_location("pin_release_checksums", PIN_SCRIPT)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


pin = _load_pin_script()
sync = pin.sync_script


@pytest.fixture(autouse=True)
def token(monkeypatch):
    """Every release API call is authenticated, so the tests carry a token too."""
    monkeypatch.setenv("GITHUB_TOKEN", "test-token")
    monkeypatch.delenv("GH_TOKEN", raising=False)


def test_the_table_it_reads_is_the_one_the_package_uses():
    """The package ships a copy of the table the script maintains."""
    assert pin.pinned_table() == PINNED_SHA256


def test_rendering_the_table_it_read_changes_nothing():
    """Rewriting without a new version is a no-op, so a diff is only new pins."""
    with open(pin.DIGESTS_FILE, encoding="utf-8") as f:
        assert f.read() == pin.render_table(pin.pinned_table())


def test_a_pinned_version_is_reachable_through_the_package():
    """A digest written by the script is what the download check looks up."""
    version = sorted(PINNED_SHA256["Open-MBEE/OpenSysML"])[-1]
    for asset, digest in PINNED_SHA256["Open-MBEE/OpenSysML"][version].items():
        assert pinned_digest(version, asset, "Open-MBEE/OpenSysML") == digest


def test_writing_a_release_adds_only_its_pins(tmp_path, monkeypatch):
    """Pinning a release leaves the releases already pinned as they were."""
    digests_file = tmp_path / "release-digests.json"
    digests_file.write_text(pin.render_table(pin.pinned_table()))
    monkeypatch.setattr(pin, "DIGESTS_FILE", str(digests_file))
    monkeypatch.setattr(pin, "sync_clients", lambda digests_file=None: None)
    monkeypatch.setattr(
        pin, "digests_of", lambda repo, version: {"sysml-grpc-linux-amd64": "ab" * 32}
    )

    assert pin.main(["--version", "v9.9.9", "--write"]) == 0

    table = pin.pinned_table(str(digests_file))
    assert table["Open-MBEE/OpenSysML"]["v9.9.9"] == {"sysml-grpc-linux-amd64": "ab" * 32}
    assert table["Open-MBEE/OpenSysML"]["v0.0.7"] == (
        PINNED_SHA256["Open-MBEE/OpenSysML"]["v0.0.7"]
    )


def test_writing_syncs_the_copy_the_package_ships(tmp_path, monkeypatch):
    """A client verifies against its own copy, so writing must reach it."""
    digests_file = tmp_path / "release-digests.json"
    digests_file.write_text(pin.render_table(pin.pinned_table()))
    shipped = tmp_path / "opensysml" / "release-digests.json"
    shipped.parent.mkdir()
    shipped.write_text("{}\n")
    monkeypatch.setattr(pin, "DIGESTS_FILE", str(digests_file))
    monkeypatch.setattr(sync, "REPO_ROOT", str(tmp_path))
    monkeypatch.setattr(sync, "COPIES", (os.path.join("opensysml", "release-digests.json"),))
    monkeypatch.setattr(
        pin, "digests_of", lambda repo, version: {"sysml-grpc-linux-amd64": "ab" * 32}
    )

    assert pin.main(["--version", "v9.9.9", "--write"]) == 0

    assert json.loads(shipped.read_text()) == pin.pinned_table(str(digests_file))


def test_the_shipped_copy_is_the_table():
    """The committed copies do not drift from the table they are synced from."""
    with open(pin.DIGESTS_FILE, encoding="utf-8") as f:
        table = f.read()
    with open(PINNED_DIGESTS_FILE, encoding="utf-8") as f:
        assert f.read() == table
    assert sync.sync(check=True) == []


def test_a_release_whose_sidecar_disagrees_is_not_pinned(monkeypatch):
    """The digest is what was downloaded; a sidecar that differs fails the release."""
    monkeypatch.setattr(
        pin, "release_assets",
        lambda repo, version: {"sysml-grpc-linux-amd64": "https://example/asset"},
    )
    monkeypatch.setattr(pin, "download_digest", lambda url: "ab" * 32)
    monkeypatch.setattr(pin, "served_digest", lambda url: "cd" * 32)

    with pytest.raises(pin.PinError, match="the release is inconsistent"):
        pin.digests_of("Open-MBEE/OpenSysML", "v9.9.9")


def test_a_release_without_a_sidecar_is_still_pinned(monkeypatch):
    """The sidecar is a cross-check, not the source of the digest."""
    monkeypatch.setattr(
        pin, "release_assets",
        lambda repo, version: {"sysml-grpc-linux-amd64": "https://example/asset"},
    )
    monkeypatch.setattr(pin, "download_digest", lambda url: "ab" * 32)
    monkeypatch.setattr(pin, "served_digest", lambda url: None)

    assert pin.digests_of("Open-MBEE/OpenSysML", "v9.9.9") == {
        "sysml-grpc-linux-amd64": "ab" * 32
    }


def test_a_release_with_no_binaries_is_refused(monkeypatch):
    """A release publishing no assets is refused, not pinned as an empty version."""
    monkeypatch.setattr(pin.urllib.request, "urlopen", _fake_urlopen('{"assets": []}'))

    with pytest.raises(pin.PinError, match="publishes no"):
        pin.release_assets("Open-MBEE/OpenSysML", "v9.9.9")


def test_only_service_binaries_are_pinned(monkeypatch):
    """Checksums and other release files are not assets to pin."""
    body = (
        '{"assets": ['
        '{"name": "sysml-grpc-linux-amd64", "browser_download_url": "u1"},'
        '{"name": "sysml-grpc-linux-amd64.sha256", "browser_download_url": "u2"},'
        '{"name": "sysml-linux-amd64", "browser_download_url": "u3"}]}'
    )
    monkeypatch.setattr(pin.urllib.request, "urlopen", _fake_urlopen(body))

    assert pin.release_assets("Open-MBEE/OpenSysML", "v9.9.9") == {
        "sysml-grpc-linux-amd64": "u1"
    }


def _fake_urlopen(body):
    """A urlopen serving one body, so the release API is not called for real."""
    class Response:
        def read(self, *args):
            nonlocal body
            chunk, body = body, ""
            return chunk.encode()

        def __enter__(self):
            return self

        def __exit__(self, *exc):
            return False

    return lambda request, timeout=None: Response()


def test_checking_reports_a_republished_release(monkeypatch):
    """A pinned asset that now hashes differently is a republished release."""
    table = {"Open-MBEE/OpenSysML": {"v0.0.7": {"sysml-grpc-linux-amd64": "ab" * 32}}}
    monkeypatch.setattr(
        pin, "digests_of", lambda repo, version: {"sysml-grpc-linux-amd64": "cd" * 32}
    )

    problems = pin.check(table)
    assert len(problems) == 1
    assert "was republished" in problems[0]


def test_checking_reports_an_asset_that_is_no_longer_published(monkeypatch):
    """A pin with nothing behind it can never be satisfied, so it is reported."""
    table = {"Open-MBEE/OpenSysML": {"v0.0.7": {"sysml-grpc-linux-amd64": "ab" * 32}}}
    monkeypatch.setattr(pin, "digests_of", lambda repo, version: {})

    assert pin.check(table) == [
        "sysml-grpc-linux-amd64 of v0.0.7 of Open-MBEE/OpenSysML is no longer published"
    ]


def test_checking_reports_a_published_asset_nothing_pins(monkeypatch):
    """A platform added to a release without a pin would download unpinned."""
    table = {"Open-MBEE/OpenSysML": {"v0.0.7": {"sysml-grpc-linux-amd64": "ab" * 32}}}
    monkeypatch.setattr(
        pin, "digests_of",
        lambda repo, version: {
            "sysml-grpc-linux-amd64": "ab" * 32,
            "sysml-grpc-linux-riscv64": "cd" * 32,
        },
    )

    problems = pin.check(table)
    assert problems == [
        "sysml-grpc-linux-riscv64 of v0.0.7 of Open-MBEE/OpenSysML is published but unpinned"
    ]


def test_checking_passes_when_every_pin_still_holds(monkeypatch):
    """The release the package pins is the release being served."""
    table = {"Open-MBEE/OpenSysML": {"v0.0.7": {"sysml-grpc-linux-amd64": "ab" * 32}}}
    monkeypatch.setattr(
        pin, "digests_of", lambda repo, version: {"sysml-grpc-linux-amd64": "ab" * 32}
    )

    assert pin.check(table) == []


RUST_SERVICE_ASSETS = (
    "sysml-grpc-darwin-amd64",
    "sysml-grpc-darwin-arm64",
    "sysml-grpc-linux-amd64",
    "sysml-grpc-linux-arm64",
    "sysml-grpc-windows-amd64.exe",
)


def _release_manifest(tmp_path, assets=RUST_SERVICE_ASSETS, digest_overrides=None):
    digests = {asset: f"{index + 1:064x}" for index, asset in enumerate(assets)}
    digests.update(digest_overrides or {})
    lines = [f"{digest}  {asset}" for asset, digest in digests.items()]
    lines.extend(
        (
            f"{'a' * 64}  opensysml-0.9.1-linux-amd64.tar.gz",
            f"{'b' * 64}  opensysml-0.9.1-py3-none-any.whl",
            f"{'c' * 64}  sysml-grpc-linux-amd64.sha256",
        )
    )
    manifest = tmp_path / "SHA256SUMS.txt"
    manifest.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return manifest, digests


def _temporary_table(tmp_path):
    table = tmp_path / "release-digests.json"
    table.write_text(pin.render_table(pin.pinned_table()), encoding="utf-8")
    return table


def _client_digest_snapshot():
    paths = (sync.SOURCE,) + tuple(
        os.path.join(sync.REPO_ROOT, relative) for relative in sync.COPIES
    )
    snapshot = {}
    for path in paths:
        with open(path, "rb") as table:
            snapshot[path] = table.read()
    return snapshot


def test_manifest_stamps_only_service_assets_and_uses_rendered_table(tmp_path):
    manifest, digests = _release_manifest(tmp_path)
    table_path = _temporary_table(tmp_path)
    before_copies = _client_digest_snapshot()
    table = pin.pinned_table(str(table_path))
    table.setdefault(pin.DEFAULT_REPO, {})["v0.9.1"] = digests

    assert pin.stamp_from_manifest(manifest, "v0.9.1", table_path=table_path)

    assert pin.pinned_table(str(table_path))[pin.DEFAULT_REPO]["v0.9.1"] == digests
    assert table_path.read_text(encoding="utf-8") == pin.render_table(table)
    assert _client_digest_snapshot() == before_copies


def test_manifest_stamping_requires_every_service_platform(tmp_path):
    manifest, _ = _release_manifest(tmp_path, assets=RUST_SERVICE_ASSETS[:-1])
    table_path = _temporary_table(tmp_path)

    with pytest.raises(pin.PinError, match="sysml-grpc-windows-amd64.exe"):
        pin.stamp_from_manifest(manifest, "v0.9.1", table_path=table_path)


def test_manifest_stamping_rejects_malformed_service_digest(tmp_path):
    asset = "sysml-grpc-linux-amd64"
    manifest, _ = _release_manifest(tmp_path, digest_overrides={asset: "A" * 64})
    table_path = _temporary_table(tmp_path)

    with pytest.raises(pin.PinError, match=f"malformed SHA-256 digest for {asset}"):
        pin.stamp_from_manifest(manifest, "v0.9.1", table_path=table_path)


def test_manifest_restamp_is_byte_identical(tmp_path):
    manifest, _ = _release_manifest(tmp_path)
    table_path = _temporary_table(tmp_path)
    assert pin.stamp_from_manifest(manifest, "v0.9.1", table_path=table_path)
    stamped = table_path.read_bytes()

    assert not pin.stamp_from_manifest(manifest, "v0.9.1", table_path=table_path)
    assert table_path.read_bytes() == stamped


def test_manifest_stamping_refuses_conflicting_pin_without_writing(tmp_path):
    manifest, _ = _release_manifest(tmp_path)
    table_path = _temporary_table(tmp_path)
    table = pin.pinned_table(str(table_path))
    table.setdefault(pin.DEFAULT_REPO, {})["v0.9.1"] = {
        asset: "f" * 64 for asset in RUST_SERVICE_ASSETS
    }
    table_path.write_text(pin.render_table(table), encoding="utf-8")
    original = table_path.read_bytes()

    with pytest.raises(pin.PinError, match="different digest pin already exists"):
        pin.stamp_from_manifest(manifest, "v0.9.1", table_path=table_path)

    assert table_path.read_bytes() == original


def test_manifest_cli_needs_no_token_and_uses_the_selected_table(tmp_path, monkeypatch):
    manifest, digests = _release_manifest(tmp_path)
    table_path = _temporary_table(tmp_path)
    monkeypatch.delenv("GITHUB_TOKEN", raising=False)
    monkeypatch.delenv("GH_TOKEN", raising=False)

    assert pin.main(
        [
            "--version",
            "v0.9.1",
            "--from-manifest",
            str(manifest),
            "--table",
            str(table_path),
        ]
    ) == 0

    assert pin.pinned_table(str(table_path))[pin.DEFAULT_REPO]["v0.9.1"] == digests


class TestGitHubToken:
    """The release API is read authenticated, and says so when it cannot be."""

    def test_the_token_is_read_from_the_environment(self, monkeypatch):
        assert pin.github_token() == "test-token"
        monkeypatch.delenv("GITHUB_TOKEN")
        monkeypatch.setenv("GH_TOKEN", "fallback-token")
        assert pin.github_token() == "fallback-token"

    def test_no_token_is_a_typed_error_naming_the_variable_and_the_scope(self, monkeypatch):
        """The failure names what to set and what it must be able to do."""
        monkeypatch.delenv("GITHUB_TOKEN")
        with pytest.raises(pin.MissingTokenError) as exc:
            pin.github_token()
        message = str(exc.value)
        assert "$GITHUB_TOKEN" in message
        assert "$GH_TOKEN" in message
        assert "public_repo" in message
        assert "releasing.md" in message
        assert isinstance(exc.value, pin.PinError)

    def test_a_release_is_not_read_unauthenticated(self, monkeypatch):
        """No token stops the call rather than making it and hitting a 403."""
        monkeypatch.delenv("GITHUB_TOKEN")
        called = []
        monkeypatch.setattr(pin.urllib.request, "urlopen",
                            lambda *a, **k: called.append(a) or None)
        with pytest.raises(pin.MissingTokenError):
            pin.release_assets("Open-MBEE/OpenSysML", "v9.9.9")
        assert called == []

    def test_the_token_authenticates_the_request(self, monkeypatch):
        """The token reaches the request, so the call is not rate-limited per address."""
        requests = []

        def urlopen(request, timeout=None):
            requests.append(request)
            return _fake_urlopen(
                '{"assets": [{"name": "sysml-grpc-linux-amd64", '
                '"browser_download_url": "u1"}]}'
            )(request)

        monkeypatch.setattr(pin.urllib.request, "urlopen", urlopen)
        pin.release_assets("Open-MBEE/OpenSysML", "v9.9.9")
        assert requests[0].get_header("Authorization") == "Bearer test-token"

    def test_the_command_reports_a_missing_token_as_an_error(self, monkeypatch, capsys):
        """It is the script's failure too, not a traceback."""
        monkeypatch.delenv("GITHUB_TOKEN")
        assert pin.main(["--version", "v9.9.9"]) == 1
        assert "$GITHUB_TOKEN" in capsys.readouterr().err

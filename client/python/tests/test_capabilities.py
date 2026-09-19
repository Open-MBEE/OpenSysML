"""Tests for the service capability handshake.

Each test runs a real gRPC server that answers like a particular build of
sysml-grpc — one too old to know the handshake, one that answers it without the
type_facts capability, one current — and drives the generator against it. The
point is the failure paths: a service that cannot supply type facts must stop
generation, because the module it would produce types every feature `object`,
which is indistinguishable from a feature that is genuinely untyped.
"""

from concurrent import futures

from unittest.mock import patch

import grpc
import pytest

from opensysml.capabilities import (
    CAPABILITY_QUERY,
    CAPABILITY_TYPE_FACTS,
    MissingCapabilityError,
    upgrade_remedy,
)
from opensysml.connection import Connection
from opensysml.errors import ConnectionError
from opensysml.generate import main, require_type_facts
from opensysml.proto import sysml_pb2, sysml_pb2_grpc

MODEL = """
package Demo {
    part def Engine {
        attribute power : Real = 300.0;
    }
}
"""


class FakeService(sysml_pb2_grpc.SysMLServiceServicer):
    """A sysml-grpc that parses, with a configurable handshake answer."""

    def __init__(self, capabilities, version="v0.0.5", answers_handshake=True):
        self._capabilities = capabilities
        self._version = version
        self._answers_handshake = answers_handshake

    def GetServerInfo(self, request, context):
        if not self._answers_handshake:
            # What a service built before the RPC existed does.
            context.abort(grpc.StatusCode.UNIMPLEMENTED, "unknown method GetServerInfo")
        return sysml_pb2.ServerInfoResponse(
            version=self._version, capabilities=self._capabilities
        )

    def GetDiagnostics(self, request, context):
        # The client's health probe reads NOT_FOUND for an unknown hash as "up".
        context.abort(grpc.StatusCode.NOT_FOUND, "model not found")

    def ParseFile(self, request, context):
        # A symbol tree without type_info: exactly what a pre-type-facts service
        # sends, and the reason generation must not proceed.
        root = sysml_pb2.SymbolInfo(id="Demo", name="Demo", kind="Package")
        return sysml_pb2.ParseFileResponse(model_hash="fake-hash", root=root)


class FakeServices:
    """The fake services one test starts, by port."""

    def __init__(self):
        self._servers = {}

    def __call__(self, port=0, **kwargs):
        """Start a FakeService on ``port`` (0 for an ephemeral one); returns the port."""
        server = grpc.server(futures.ThreadPoolExecutor(max_workers=2))
        sysml_pb2_grpc.add_SysMLServiceServicer_to_server(FakeService(**kwargs), server)
        port = server.add_insecure_port(f"localhost:{port}")
        server.start()
        self._servers[port] = server
        return port

    def stop(self, port):
        """Stop the service on ``port``, returning once it has let the port go."""
        self._servers.pop(port).stop(None).wait()

    def stop_all(self):
        for port in list(self._servers):
            self.stop(port)


@pytest.fixture
def fake_service():
    """Yields the FakeServices of this test, stopped after it."""
    services = FakeServices()
    yield services
    services.stop_all()


def test_old_service_reports_no_capabilities(fake_service):
    """A service predating the handshake claims nothing, and says why."""
    port = fake_service(capabilities=[], answers_handshake=False)
    with Connection(port=port, auto_start=False) as conn:
        info = conn.server_info()
        assert info.answered is False
        assert info.capabilities == frozenset()
        assert "too old to answer GetServerInfo" in info.describe()
        with pytest.raises(MissingCapabilityError) as excinfo:
            require_type_facts(conn)
    assert excinfo.value.capability == CAPABILITY_TYPE_FACTS


def test_current_service_reports_type_facts(fake_service):
    """A service that reports the capability is accepted."""
    port = fake_service(capabilities=[CAPABILITY_TYPE_FACTS], version="v9.9.9")
    with Connection(port=port, auto_start=False) as conn:
        info = conn.server_info()
        assert info.answered is True
        assert info.has(CAPABILITY_TYPE_FACTS)
        assert info.version == "v9.9.9"
        require_type_facts(conn)


def test_service_answering_without_type_facts_is_rejected(fake_service):
    """A newer service that simply lacks the capability is rejected too.

    The handshake is not a version comparison: answering is not enough.
    """
    port = fake_service(capabilities=["something_else"], version="v1.0.0")
    with Connection(port=port, auto_start=False) as conn:
        with pytest.raises(MissingCapabilityError):
            require_type_facts(conn)


def _run_generate(monkeypatch, tmp_path, port, source, output):
    """Run the generator CLI against ``port``, isolated from the real ~/.opensysml."""
    monkeypatch.setenv("HOME", str(tmp_path))
    return main(
        [str(source), "-o", str(output), "--host", "localhost", "--port", str(port)]
    )


def test_generation_fails_against_a_service_without_type_facts(
    monkeypatch, tmp_path, capsys, fake_service
):
    """Generation stops with an actionable message and writes nothing."""
    port = fake_service(capabilities=[], answers_handshake=False)
    source = tmp_path / "demo.sysml"
    source.write_text(MODEL)
    output = tmp_path / "demo_types.py"

    assert _run_generate(monkeypatch, tmp_path, port, source, output) == 1

    assert not output.exists(), "a failed generation must not leave an untyped module"
    message = capsys.readouterr().err
    assert CAPABILITY_TYPE_FACTS in message
    # Names the binary in use and where it came from.
    assert f"localhost:{port}" in message
    # Names the fix.
    assert "make build-grpc" in message
    assert "OPENSYSML_GRPC_VERSION" in message


def test_generation_succeeds_against_a_service_reporting_type_facts(
    monkeypatch, tmp_path, fake_service
):
    """The capability check does not stand in the way of a current service.

    The fake parses to an empty package, so the module it produces has no
    classes; what matters here is that the preflight lets it through.
    """
    port = fake_service(capabilities=[CAPABILITY_TYPE_FACTS])
    source = tmp_path / "demo.sysml"
    source.write_text(MODEL)
    output = tmp_path / "demo_types.py"

    assert _run_generate(monkeypatch, tmp_path, port, source, output) == 0
    assert "SYSML_MODEL_HASH" in output.read_text()


def test_generation_asks_the_service_now_on_a_port_an_older_one_had(
    monkeypatch, tmp_path, fake_service
):
    """A run against a recycled port is answered by the service on it now.

    Ephemeral ports get reused, so a run must not carry over the handshake a
    previous run had with the service that held the port before.
    """
    source = tmp_path / "demo.sysml"
    source.write_text(MODEL)
    output = tmp_path / "demo_types.py"
    port = fake_service(capabilities=[], answers_handshake=False)
    assert _run_generate(monkeypatch, tmp_path, port, source, output) == 1
    fake_service.stop(port)

    assert fake_service(port=port, capabilities=[CAPABILITY_TYPE_FACTS]) == port
    assert _run_generate(monkeypatch, tmp_path, port, source, output) == 0


def test_remedy_survives_a_platform_with_no_release_build():
    """The advice is still given where no release binary exists to name."""
    with patch('opensysml.capabilities.get_binary_path',
               side_effect=ConnectionError('Unsupported operating system')):
        remedy = upgrade_remedy(CAPABILITY_QUERY)
    assert "cached locally" in remedy
    assert "OPENSYSML_GRPC_VERSION" in remedy
    assert "make build-grpc" in remedy

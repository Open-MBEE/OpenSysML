"""Tests for what happens when the service named is another release.

A cached binary is release-aware, but the process serving the address named is
what actually answers, so the same check has to be made against it. Each test
runs a real gRPC server answering the handshake like a particular build, and
drives ``Connection`` against it with a release asked for. Such a service is
somebody else's: it is reported, never stopped or replaced.
"""

import socket
from concurrent import futures

import grpc
import pytest

from opensysml.capabilities import (
    CAPABILITY_CONVERT,
    CAPABILITY_QUERY,
    MissingCapabilityError,
    ServerInfo,
    mismatch_reason,
)
from opensysml.connection import Connection, _private_services
from opensysml.errors import ConnectionError, ServiceError, StaleServiceError
from opensysml.proto import sysml_pb2, sysml_pb2_grpc


class OldService(sysml_pb2_grpc.SysMLServiceServicer):
    """A sysml-grpc of an earlier release, healthy and answering."""

    def __init__(self, version="v0.0.5", capabilities=(CAPABILITY_CONVERT,),
                 answers_handshake=True, handshake_failures=0):
        self._version = version
        self._capabilities = list(capabilities)
        self._answers_handshake = answers_handshake
        self._handshake_failures = handshake_failures

    def GetServerInfo(self, request, context):
        if not self._answers_handshake:
            context.abort(grpc.StatusCode.UNIMPLEMENTED, "unknown method GetServerInfo")
        if self._handshake_failures > 0:
            self._handshake_failures -= 1
            context.abort(grpc.StatusCode.INTERNAL, "busy")
        return sysml_pb2.ServerInfoResponse(
            version=self._version, capabilities=self._capabilities
        )

    def GetDiagnostics(self, request, context):
        # The health probe reads NOT_FOUND for an unknown hash as "up".
        context.abort(grpc.StatusCode.NOT_FOUND, "model not found")


@pytest.fixture
def running_service():
    """Start an OldService on an ephemeral port; yields a factory of ports."""
    servers = []

    def start(**kwargs):
        server = grpc.server(futures.ThreadPoolExecutor(max_workers=2))
        sysml_pb2_grpc.add_SysMLServiceServicer_to_server(OldService(**kwargs), server)
        port = server.add_insecure_port("localhost:0")
        server.start()
        servers.append(server)
        return port

    yield start
    for server in servers:
        server.stop(None)


@pytest.fixture(autouse=True)
def no_private_service(monkeypatch):
    """Every test here names a service, so none of them may start one."""
    def refuse(**kwargs):
        raise AssertionError("a named service must not be started by the client")

    monkeypatch.delenv("OPENSYSML_SERVICE", raising=False)
    monkeypatch.setattr("opensysml.connection.ensure_binary", refuse)
    yield
    assert not _private_services


def test_a_foreign_service_of_another_release_is_reported_not_killed(
    running_service, monkeypatch
):
    """The user's own service is left running, and the mismatch is raised.

    Reusing it would serve the client an older build, whose first newer call
    fails as a MissingCapabilityError naming a capability the release asked for
    does have — the wrong error, several calls later.
    """
    port = running_service(version="v0.0.5")
    monkeypatch.setenv("OPENSYSML_GRPC_VERSION", "v0.0.7")

    with pytest.raises(StaleServiceError) as excinfo:
        Connection(port=port)

    error = excinfo.value
    assert error.address == f"localhost:{port}"
    assert "v0.0.5" in error.reason
    assert "v0.0.7" in error.reason
    assert f"stop the service listening on localhost:{port} yourself" in error.remedy
    assert error.info.version == "v0.0.5"
    # Still serving: nothing was killed.
    monkeypatch.delenv("OPENSYSML_GRPC_VERSION")
    with Connection(port=port) as conn:
        assert conn.server_info().version == "v0.0.5"


def test_a_foreign_service_that_cannot_say_what_it_is_is_reported(
    running_service, monkeypatch
):
    """A service that cannot say what it is cannot be the release asked for."""
    port = running_service(answers_handshake=False)
    monkeypatch.setenv("OPENSYSML_GRPC_VERSION", "v0.0.7")

    with pytest.raises(StaleServiceError) as excinfo:
        Connection(port=port)

    assert "did not answer GetServerInfo" in excinfo.value.reason


def test_a_foreign_service_lacking_a_required_capability_is_reported(
    running_service, monkeypatch
):
    """A missing capability is the documented error, whoever started the service.

    Which process happens to be listening must not change the class a caller
    has to catch for one condition.
    """
    monkeypatch.delenv("OPENSYSML_GRPC_VERSION", raising=False)
    port = running_service(capabilities=[CAPABILITY_CONVERT])

    with pytest.raises(MissingCapabilityError) as excinfo:
        Connection(port=port, require_capabilities=[CAPABILITY_QUERY])

    assert CAPABILITY_QUERY in str(excinfo.value)


def test_a_handshake_that_fails_is_not_taken_for_an_answer(
    running_service, monkeypatch
):
    """A call that failed says nothing, so it is neither cached nor trusted.

    Recording it as a service reporting no capabilities would refuse every
    capability-gated call for the life of the connection.
    """
    monkeypatch.delenv("OPENSYSML_GRPC_VERSION", raising=False)
    port = running_service(version="v0.0.5", capabilities=[CAPABILITY_QUERY],
                           handshake_failures=1)

    with Connection(port=port) as conn:
        with pytest.raises(ServiceError):
            conn.server_info()

        info = conn.server_info()
        assert info.version == "v0.0.5"
        assert info.has(CAPABILITY_QUERY)


def test_a_handshake_that_fails_is_never_read_as_a_capability_missing(
    running_service, monkeypatch
):
    """A call that failed is a transport failure, not a service lacking something.

    Which error a caller catches must not turn on RPC luck, so a failed
    handshake is reported as itself and asking again settles the capability.
    """
    monkeypatch.delenv("OPENSYSML_GRPC_VERSION", raising=False)
    port = running_service(capabilities=[CAPABILITY_QUERY], handshake_failures=1)

    with pytest.raises(ServiceError) as excinfo:
        Connection(port=port, require_capabilities=[CAPABILITY_QUERY])

    assert not isinstance(excinfo.value, MissingCapabilityError)

    with Connection(port=port, require_capabilities=[CAPABILITY_QUERY]) as conn:
        assert conn.server_info().has(CAPABILITY_QUERY)


def test_a_handshake_that_fails_is_checked_again_when_a_release_was_asked_for(
    running_service, monkeypatch
):
    """A service that could not be asked yet is asked at the first call.

    A named service may not be listening when a connection is made, so a failed
    handshake defers the check rather than refusing an address the caller chose.
    """
    port = running_service(version="v0.0.5", handshake_failures=1)
    monkeypatch.setenv("OPENSYSML_GRPC_VERSION", "v0.0.7")

    conn = Connection(port=port)
    with pytest.raises(StaleServiceError) as excinfo:
        conn.server_info()
    conn.close()

    assert "v0.0.5" in excinfo.value.reason


def test_a_service_the_caller_manages_is_checked_too(
    running_service, monkeypatch
):
    """A release asked for is checked even when this client starts nothing.

    Reporting the mismatch needs no ownership, since nothing is stopped; only
    replacing the service does.
    """
    port = running_service(version="v0.0.5")

    with pytest.raises(StaleServiceError) as excinfo:
        Connection(port=port, auto_start=False, version="v0.0.7")

    assert "v0.0.5" in excinfo.value.reason
    assert "v0.0.7" in excinfo.value.reason
    with Connection(port=port, auto_start=False) as conn:
        assert conn.server_info().version == "v0.0.5"


def test_the_newest_release_is_looked_up_once_per_connection(
    running_service, monkeypatch
):
    """A lookup of 'latest' that later fails cannot turn the check off.

    Resolving it per check would let a flaky second lookup read as "no release
    asked for", and would cost a lookup on every check besides.
    """
    port = running_service(version="v0.0.5")
    lookups = []

    def resolve_latest_version():
        lookups.append(None)
        if len(lookups) > 1:
            raise ConnectionError("release lookup unavailable")
        return "v0.0.7"

    monkeypatch.setattr(
        "opensysml.connection.resolve_latest_version", resolve_latest_version
    )

    with pytest.raises(StaleServiceError) as excinfo:
        Connection(port=port, auto_start=False, version="latest")

    assert "v0.0.5" in excinfo.value.reason
    assert "v0.0.7" in excinfo.value.reason
    assert len(lookups) == 1


def test_a_service_the_caller_has_not_started_yet_is_checked_at_the_first_call(
    running_service, monkeypatch
):
    """auto_start=False stays lazy: an unreachable service is not refused early.

    Callers build the client before starting their own service, so the release
    is checked once the service answers rather than at construction.
    """
    monkeypatch.setenv("OPENSYSML_GRPC_VERSION", "v0.0.7")

    conn = Connection(port=_free_port(), auto_start=False)

    try:
        with pytest.raises(ConnectionError):
            conn.server_info()
    finally:
        conn.close()

    port = running_service(version="v0.0.5")
    # As if that service had come up only after the client was built.
    monkeypatch.setattr(
        Connection, "_running_service_info", lambda self, timeout=5.0: None
    )
    conn = Connection(host=f"localhost:{port}", auto_start=False)
    try:
        with pytest.raises(StaleServiceError) as excinfo:
            conn.server_info()
        assert "v0.0.5" in excinfo.value.reason
        # A caller that carries on past the first refusal is refused again,
        # rather than served by the release it did not ask for.
        with pytest.raises(StaleServiceError):
            conn.server_info()
    finally:
        conn.close()


def test_a_deferred_check_is_made_by_whichever_call_comes_first(
    running_service, monkeypatch
):
    """Any call answers a check still owed, not only the ones asking what it is.

    Scripts load and evaluate without ever asking for capabilities, so a check
    reached only through server_info() would never be made for them.
    """
    port = running_service(version="v0.0.5")
    monkeypatch.setenv("OPENSYSML_GRPC_VERSION", "v0.0.7")
    # As if that service had come up only after the client was built.
    monkeypatch.setattr(
        Connection, "_running_service_info", lambda self, timeout=5.0: None
    )

    conn = Connection(port=port, auto_start=False)
    try:
        with pytest.raises(StaleServiceError):
            conn.load("/does/not/matter.sysml")
    finally:
        conn.close()


def test_a_matching_service_named_is_used(running_service, monkeypatch):
    """A named service that is what was asked for is used, and left alone.

    This client did not start it, so it holds nothing to release: closing the
    connection cannot stop somebody else's service.
    """
    port = running_service(version="v0.0.7", capabilities=[CAPABILITY_CONVERT])
    monkeypatch.setenv("OPENSYSML_GRPC_VERSION", "v0.0.7")

    with Connection(port=port, require_capabilities=[CAPABILITY_CONVERT]) as conn:
        assert conn.server_info().version == "v0.0.7"
        assert conn._private is None

    # Closing left it serving.
    with Connection(port=port) as conn:
        assert conn.server_info().version == "v0.0.7"


def test_a_service_asked_for_no_release_is_used_whatever_it_is(
    running_service, monkeypatch
):
    """With no release asked for, whichever build answers is accepted.

    This is the same rule the binary cache follows: a binary the user put there
    is left alone until a client says which release it needs.
    """
    monkeypatch.delenv("OPENSYSML_GRPC_VERSION", raising=False)
    port = running_service(version="v0.0.5")

    with Connection(port=port) as conn:
        assert conn.server_info().version == "v0.0.5"


def _free_port():
    """A port nothing is listening on, for a service started by the client."""
    with socket.socket() as sock:
        sock.bind(("localhost", 0))
        return sock.getsockname()[1]


class TestMismatchReason:
    """The comparison itself, without a service to run."""

    def _info(self, version="v0.0.5", capabilities=(), answered=True):
        return ServerInfo(
            version=version,
            capabilities=frozenset(capabilities),
            answered=answered,
            origin="origin",
        )

    def test_nothing_asked_for_is_no_mismatch(self):
        assert mismatch_reason(self._info()) is None

    def test_the_same_release_is_no_mismatch(self):
        assert mismatch_reason(self._info(), version="v0.0.5") is None

    def test_another_release_is_a_mismatch(self):
        assert "v0.0.5" in mismatch_reason(self._info(), version="v0.0.7")

    def test_an_unanswered_handshake_cannot_be_the_release_asked_for(self):
        reason = mismatch_reason(self._info(answered=False), version="v0.0.7")
        assert "did not answer GetServerInfo" in reason

    def test_a_missing_capability_is_a_mismatch(self):
        reason = mismatch_reason(
            self._info(capabilities=[CAPABILITY_CONVERT]),
            capabilities=[CAPABILITY_QUERY, CAPABILITY_CONVERT],
        )
        assert repr(CAPABILITY_QUERY) in reason
        assert repr(CAPABILITY_CONVERT) not in reason

    def test_both_differences_are_named(self):
        reason = mismatch_reason(
            self._info(), version="v0.0.7", capabilities=[CAPABILITY_QUERY]
        )
        assert "v0.0.7" in reason
        assert repr(CAPABILITY_QUERY) in reason

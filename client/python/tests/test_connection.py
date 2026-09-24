"""Tests for Connection class."""

import grpc
import io
import os
import pytest
import subprocess
import time
from unittest.mock import Mock, patch
from opensysml.connection import (
    CHANNEL_OPTIONS,
    Connection,
    _private_services,
)
from opensysml.errors import ConnectionError, OpenSysMLError, ServiceError
from opensysml.proto import sysml_pb2


def test_connection_init():
    with patch('grpc.insecure_channel') as mock_channel:
        mock_stub = Mock()
        with patch('opensysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection(port=50051, auto_start=False)
            
            assert conn.port == 50051
            mock_channel.assert_called_once_with(
                'localhost:50051', options=CHANNEL_OPTIONS
            )


def test_connection_custom_host():
    with patch('grpc.insecure_channel') as mock_channel:
        with patch('opensysml.proto.sysml_pb2_grpc.SysMLServiceStub'):
            conn = Connection(host='example.com', port=9000, auto_start=False)
            
            mock_channel.assert_called_once_with(
                'example.com:9000', options=CHANNEL_OPTIONS
            )


def test_connection_load():
    with patch('grpc.insecure_channel'):
        mock_stub = Mock()
        
        # Mock ParseFile RPC response
        pb_root = sysml_pb2.SymbolInfo(
            id="TestModel",
            name="TestModel",
            kind="Package",
            metadata={},
            child_ids=[],
            attributes=[],
        )
        
        pb_response = sysml_pb2.ParseFileResponse(
            model_hash="abc123",
            root=pb_root,
            diagnostics=[],
        )
        
        mock_stub.ParseFile.return_value = pb_response
        
        with patch('opensysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection(auto_start=False)
            model = conn.load("test.sysml")
            
            # Verify ParseFile was called with file path
            call_args = mock_stub.ParseFile.call_args
            request = call_args[0][0]
            assert request.file_path == "test.sysml"
            
            # Verify Model object returned
            assert model.hash == "abc123"
            assert model.root.name == "TestModel"


def test_connection_load_with_diagnostics():
    with patch('grpc.insecure_channel'):
        mock_stub = Mock()
        
        pb_root = sysml_pb2.SymbolInfo(
            id="Model",
            name="Model",
            kind="Package",
            metadata={},
            child_ids=[],
            attributes=[],
        )
        
        pb_diag = sysml_pb2.Diagnostic(
            severity="error",
            message="Parse error",
            span=sysml_pb2.Span(file="bad.sysml", start_line=1, start_col=1, end_line=1, end_col=1),
        )
        
        pb_response = sysml_pb2.ParseFileResponse(
            model_hash="hash",
            root=pb_root,
            diagnostics=[pb_diag],
        )
        
        mock_stub.ParseFile.return_value = pb_response
        
        with patch('opensysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection(auto_start=False)
            model = conn.load("bad.sysml")
            
            assert len(model.diagnostics) == 1
            assert model.diagnostics[0].message == "Parse error"


def test_connection_load_grpc_error():
    with patch('grpc.insecure_channel'):
        mock_stub = Mock()

        # A gRPC failure carrying no status still arrives inside the hierarchy,
        # with the original reachable as __cause__.
        original = grpc.RpcError()
        mock_stub.ParseFile.side_effect = original

        with patch('opensysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection(auto_start=False)

            with pytest.raises(ServiceError) as excinfo:
                conn.load("missing.sysml")
            assert isinstance(excinfo.value, OpenSysMLError)
            assert excinfo.value.__cause__ is original


def test_connection_get_symbol():
    with patch('grpc.insecure_channel'):
        mock_stub = Mock()
        
        pb_sym = sysml_pb2.SymbolInfo(
            id="Vehicle::Engine",
            name="Engine",
            kind="PartUsage",
            metadata={},
            child_ids=[],
            attributes=[],
        )
        
        pb_response = sysml_pb2.SymbolResponse(
            symbol=pb_sym,
            error="",
        )
        
        mock_stub.GetSymbol.return_value = pb_response
        
        with patch('opensysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection(auto_start=False)
            pb_result = conn.get_symbol("model_hash", "Vehicle::Engine")
            
            # Verify GetSymbol was called correctly
            call_args = mock_stub.GetSymbol.call_args
            request = call_args[0][0]
            assert request.model_hash == "model_hash"
            assert request.symbol_id == "Vehicle::Engine"
            
            # Verify SymbolInfo returned
            assert pb_result.id == "Vehicle::Engine"
            assert pb_result.name == "Engine"


def test_connection_get_symbol_not_found():
    with patch('grpc.insecure_channel'):
        mock_stub = Mock()
        
        pb_response = sysml_pb2.SymbolResponse(
            symbol=None,
            error="Symbol not found: NonExistent",
        )
        
        mock_stub.GetSymbol.return_value = pb_response
        
        with patch('opensysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=mock_stub):
            conn = Connection(auto_start=False)
            pb_result = conn.get_symbol("hash", "NonExistent")
            
            # Should return None when error is set
            assert pb_result is None


def test_connection_context_manager():
    """Test that context manager returns self and calls close() on exit."""
    with patch('grpc.insecure_channel') as mock_channel:
        mock_chan_instance = Mock()
        mock_channel.return_value = mock_chan_instance
        
        with patch('opensysml.proto.sysml_pb2_grpc.SysMLServiceStub'):
            with Connection(auto_start=False) as conn:
                # __enter__ should return self
                assert conn is not None
                assert isinstance(conn, Connection)
            
            # __exit__ should have called close() which closes the channel
            mock_chan_instance.close.assert_called_once()

# --- Private service tests, against a child that is only mocked ---


class _FakeChild:
    """A Popen stand-in that has bound an address and is still running."""

    def __init__(self, stdout=b"", stderr=b"", code=None):
        self.pid = os.getpid()
        self.stdin = Mock()
        self.stdout = io.BytesIO(stdout)
        self.stderr = io.BytesIO(stderr)
        self.args = []
        self._code = code
        self.terminated = False

    def poll(self):
        return self._code

    def terminate(self):
        self.terminated = True
        self._code = -15

    def kill(self):
        self.terminate()

    def wait(self, timeout=None):
        return self._code


@pytest.fixture
def private_child(monkeypatch):
    """Patch out the binary and the spawn; yields a factory of fake children."""
    monkeypatch.delenv("OPENSYSML_GRPC_VERSION", raising=False)
    monkeypatch.delenv("OPENSYSML_SERVICE", raising=False)
    monkeypatch.setattr(
        "opensysml.connection.ensure_binary", lambda version=None: "/bin/sysml-grpc"
    )
    monkeypatch.setattr(os.path, "exists", lambda path: True)
    stub = Mock()
    stub.GetServerInfo.return_value = sysml_pb2.ServerInfoResponse(
        version="v0.0.7", capabilities=[]
    )
    monkeypatch.setattr(
        "opensysml.proto.sysml_pb2_grpc.SysMLServiceStub", lambda channel: stub
    )
    monkeypatch.setattr(grpc, "insecure_channel", Mock())
    calls = []

    def start(child):
        def popen(args, **kwargs):
            calls.append((args, kwargs))
            child.args = args
            return child

        monkeypatch.setattr(subprocess, "Popen", popen)
        return calls

    yield start
    _private_services.clear()


def test_a_private_child_is_given_a_port_rather_than_sent_to_one(private_child):
    """The child binds :0 and reports it, so no free port is chosen and raced for."""
    calls = private_child(_FakeChild(stdout=b"127.0.0.1:34567\n"))

    with Connection() as conn:
        assert conn.host == "127.0.0.1"
        assert conn.port == 34567

    args, kwargs = calls[0]
    assert args == [
        "/bin/sysml-grpc", "-port", "0", "-health-port", "0",
        "-report-address", "-exit-with-parent",
    ]
    assert kwargs["stdin"] is subprocess.PIPE
    assert kwargs["start_new_session"] is True


def test_a_child_that_serves_nothing_is_reported_not_dialled(private_child):
    """A child that exits without an address is an error naming what it logged."""
    private_child(_FakeChild(stderr=b"bind: address already in use\n", code=1))

    with pytest.raises(ConnectionError) as excinfo:
        Connection()

    assert "without serving an address" in str(excinfo.value)
    assert "address already in use" in str(excinfo.value)


def test_the_wait_for_an_address_is_the_module_constant(private_child, monkeypatch):
    """START_TIMEOUT is the seam: shortening it shortens the wait for an address."""
    read_fd, write_fd = os.pipe()
    silent = _FakeChild()
    silent.stdout = os.fdopen(read_fd, "rb")
    private_child(silent)
    monkeypatch.setattr("opensysml.connection.START_TIMEOUT", 0.2)

    started = time.monotonic()
    with pytest.raises(ConnectionError) as excinfo:
        Connection()
    elapsed = time.monotonic() - started
    os.close(write_fd)

    assert "did not report a listening address" in str(excinfo.value)
    assert 0.2 <= elapsed < 1.0


def test_a_child_that_could_not_start_leaves_no_service_held(private_child):
    """A connection that raised holds nothing, so the next one starts afresh."""
    private_child(_FakeChild(code=1))

    with pytest.raises(ConnectionError):
        Connection()

    assert not _private_services


def test_naming_an_address_starts_nothing(private_child):
    """Naming a service is the opt-in to one this client does not manage."""
    private_child(_FakeChild(stdout=b"127.0.0.1:34567\n"))

    with Connection(port=50099) as conn:
        assert conn._private is None
        assert conn.port == 50099

    assert not _private_services


def test_naming_a_service_in_the_environment_starts_nothing(
    private_child, monkeypatch
):
    """$OPENSYSML_SERVICE names one too, for a caller that passes no arguments."""
    private_child(_FakeChild(stdout=b"127.0.0.1:34567\n"))
    monkeypatch.setenv("OPENSYSML_SERVICE", "example.com:9111")

    with Connection() as conn:
        assert conn._private is None
        assert (conn.host, conn.port) == ("example.com", 9111)

    assert not _private_services


def _reporting_child(address=b"127.0.0.1:34567\n"):
    """A fake child whose stdout stays open after the address, as a pipe does."""
    read_fd, write_fd = os.pipe()
    os.write(write_fd, address)
    child = _FakeChild()
    child.stdout = os.fdopen(read_fd, "rb")
    child.stdin = os.fdopen(write_fd, "wb")
    return child


def test_the_connections_of_one_interpreter_share_one_child(private_child):
    """One child per interpreter, so several connections cost one spawn."""
    calls = private_child(_reporting_child())

    with Connection() as first, Connection() as second:
        assert first.port == second.port == 34567
        assert first._private is second._private
        assert first._private.refs == 2

    assert len(calls) == 1


def test_the_last_connection_released_stops_the_child(private_child):
    """The child is stopped when nothing in this process still holds it."""
    child = _reporting_child()
    private_child(child)

    first = Connection()
    second = Connection()
    first.close()
    assert not child.terminated

    second.close()
    assert child.terminated
    assert not _private_services


def test_auto_start_enabled(private_child):
    """A connection that names no address starts a private child."""
    private_child(_FakeChild(stdout=b"127.0.0.1:34567\n"))

    with Connection(auto_start=True) as conn:
        assert conn._private is not None


def test_auto_start_disabled(private_child):
    """auto_start=False starts nothing and dials the standard port."""
    private_child(_FakeChild(stdout=b"127.0.0.1:34567\n"))

    with Connection(auto_start=False) as conn:
        assert conn._private is None
        assert conn.port == 50051

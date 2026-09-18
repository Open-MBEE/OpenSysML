"""The client asks the service for uncompressed responses.

Under load, grpcio intermittently hands a gzip-compressed response to the
protobuf parser, which surfaces as ``Exception deserializing response!`` or
``Wire format was corrupt`` on an RPC the service answered correctly. The
service compresses only what the client's ``grpc-accept-encoding`` allows, so
the client allows identity alone. Neither end exposes the encoding it settled
on, so these tests watch the bytes the service sends through a TCP relay.
"""

import socket
import subprocess
import threading

import grpc
import pytest

from opensysml.connection import Connection
from opensysml.proto import sysml_pb2
from tests.service_gate import service_binary, skip_or_fail_without_service

#: The handshake RPC, called with a deserializer that keeps the bytes as sent.
GET_SERVER_INFO = "/sysml.SysMLService/GetServerInfo"


class Relay:
    """A TCP relay to a service that keeps the bytes the service sent each client."""

    def __init__(self, upstream):
        self._upstream = upstream
        self._listener = socket.socket()
        self._listener.bind(("127.0.0.1", 0))
        self._listener.listen()
        self.port = self._listener.getsockname()[1]
        self._lock = threading.Lock()
        self._sockets = []
        self._from_service = []
        threading.Thread(target=self._accept, daemon=True).start()

    def _accept(self):
        while True:
            try:
                client, _ = self._listener.accept()
            except OSError:
                return
            service = socket.create_connection(self._upstream)
            captured = bytearray()
            with self._lock:
                self._sockets += [client, service]
                self._from_service.append(captured)
            threading.Thread(target=self._pump, args=(client, service, None), daemon=True).start()
            threading.Thread(target=self._pump, args=(service, client, captured), daemon=True).start()

    @staticmethod
    def _pump(source, sink, captured):
        while True:
            try:
                data = source.recv(65536)
            except OSError:
                data = b""
            if not data:
                try:
                    sink.shutdown(socket.SHUT_WR)
                except OSError:
                    pass
                return
            if captured is not None:
                captured += data
            try:
                sink.sendall(data)
            except OSError:
                return

    def service_sent_verbatim(self, payload):
        """Whether the service sent ``payload`` uncompressed to some client."""
        with self._lock:
            return any(payload in bytes(captured) for captured in self._from_service)

    def close(self):
        self._listener.close()
        with self._lock:
            for sock in self._sockets:
                sock.close()


@pytest.fixture(scope="module")
def service_address():
    """Start the built sysml-grpc on a port of the kernel's; yields (host, port)."""
    binary = service_binary()
    if binary is None:
        skip_or_fail_without_service("no sysml-grpc binary is available to spawn")
    process = subprocess.Popen(
        [binary, "-port", "0", "-health-port", "0", "-report-address"],
        stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True,
    )
    try:
        host, port = process.stdout.readline().strip().rsplit(":", 1)
        yield host, int(port)
    finally:
        process.terminate()
        process.wait(timeout=10)


@pytest.fixture
def relay(service_address):
    relay = Relay(service_address)
    yield relay
    relay.close()


def handshake_bytes(channel):
    """The service's answer to the handshake, as the bytes grpc handed the parser."""
    call = channel.unary_unary(
        GET_SERVER_INFO,
        request_serializer=sysml_pb2.ServerInfoRequest.SerializeToString,
        response_deserializer=bytes,
    )
    return call(sysml_pb2.ServerInfoRequest(), timeout=10)


@pytest.fixture
def answer(service_address):
    """The handshake answer's serialization, read from the service directly."""
    with grpc.insecure_channel("%s:%d" % service_address) as channel:
        payload = handshake_bytes(channel)
    assert sysml_pb2.ServerInfoResponse.FromString(payload).version
    return payload


def test_the_service_compresses_for_a_channel_with_grpcio_defaults(relay, answer):
    """Control: the same service gzips its answer when the client lets it."""
    with grpc.insecure_channel(f"127.0.0.1:{relay.port}") as channel:
        assert handshake_bytes(channel) == answer
    assert not relay.service_sent_verbatim(answer)


def test_the_client_is_answered_uncompressed(relay, answer):
    with Connection(host="127.0.0.1", port=relay.port, auto_start=False) as conn:
        info = conn.server_info()
    assert info.version == sysml_pb2.ServerInfoResponse.FromString(answer).version
    assert relay.service_sent_verbatim(answer)

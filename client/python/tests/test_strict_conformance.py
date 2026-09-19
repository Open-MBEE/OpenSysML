"""Tests for asking the service the strict-conformance question.

A service that does not know the field ignores it and answers the default
question, which would read as "this file is conforming SysML v2" when nobody
checked. The client requires the capability instead, so the stale service is
named.
"""

from concurrent import futures

import grpc
import pytest

from opensysml.capabilities import CAPABILITY_STRICT_CONFORMANCE, MissingCapabilityError
from opensysml.connection import Connection
from opensysml.proto import sysml_pb2, sysml_pb2_grpc


class FakeService(sysml_pb2_grpc.SysMLServiceServicer):
    """A sysml-grpc whose ParseFile records the request it was sent."""

    def __init__(self, capabilities=(CAPABILITY_STRICT_CONFORMANCE,)):
        self._capabilities = list(capabilities)
        self.requests = []

    def GetServerInfo(self, request, context):
        return sysml_pb2.ServerInfoResponse(
            version="fake", capabilities=self._capabilities
        )

    def ParseFile(self, request, context):
        self.requests.append(request)
        return sysml_pb2.ParseFileResponse(model_hash="fake-hash")


@pytest.fixture
def fake_service():
    """Start a FakeService on an ephemeral port; yields a (port, service) factory."""
    servers = []

    def start(**kwargs):
        service = FakeService(**kwargs)
        server = grpc.server(futures.ThreadPoolExecutor(max_workers=2))
        sysml_pb2_grpc.add_SysMLServiceServicer_to_server(service, server)
        port = server.add_insecure_port("localhost:0")
        server.start()
        servers.append(server)
        return port, service

    yield start
    for server in servers:
        server.stop(None)


def test_strict_conformance_reaches_the_service(fake_service):
    port, service = fake_service()
    with Connection(port=port, auto_start=False) as conn:
        conn.load("model.sysml", strict_conformance=True)
        conn.load_from_content("package P;", strict_conformance=True)

    assert [request.strict_conformance for request in service.requests] == [True, True]


def test_default_asks_no_strict_question(fake_service):
    """The field is off unless asked for, and needs no capability."""
    port, service = fake_service(capabilities=())
    with Connection(port=port, auto_start=False) as conn:
        conn.load("model.sysml")

    assert [request.strict_conformance for request in service.requests] == [False]


def test_strict_conformance_requires_the_capability(fake_service):
    port, service = fake_service(capabilities=())
    with Connection(port=port, auto_start=False) as conn:
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.load("model.sysml", strict_conformance=True)

    assert excinfo.value.capability == CAPABILITY_STRICT_CONFORMANCE
    assert service.requests == [], "the ask was sent to a service that ignores it"

"""Tests for the capability gate on evaluating against a subject.

A service that does not understand ``EvaluateRequest.subject_symbol_id`` drops
it as an unknown field and answers with the declared default, which cannot be
told from the object's own value — so the client requires the capability rather
than sending the request and trusting the answer.
"""

from concurrent import futures

import grpc
import pytest

from opensysml.capabilities import CAPABILITY_EVALUATE_SUBJECT, MissingCapabilityError
from opensysml.connection import Connection
from opensysml.proto import sysml_pb2, sysml_pb2_grpc


class FakeService(sysml_pb2_grpc.SysMLServiceServicer):
    """A sysml-grpc whose Evaluate records the request and answers a fixed value."""

    def __init__(self, capabilities=(CAPABILITY_EVALUATE_SUBJECT,)):
        self._capabilities = list(capabilities)
        self.requests = []

    def GetServerInfo(self, request, context):
        return sysml_pb2.ServerInfoResponse(
            version="fake", capabilities=self._capabilities
        )

    def GetDiagnostics(self, request, context):
        context.abort(grpc.StatusCode.NOT_FOUND, "model not found")

    def Evaluate(self, request, context):
        self.requests.append(request)
        return sysml_pb2.EvaluateResponse(result=sysml_pb2.Value(real_value=1200.0))


@pytest.fixture
def fake_service():
    """Start a FakeService on an ephemeral port; yields (port, service) factory."""
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


def test_a_subject_requires_the_capability(fake_service):
    """A service without subject evaluation is named, not asked."""
    port, service = fake_service(capabilities=())
    with Connection(port=port, auto_start=False) as conn:
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.eval("mass", "fake-hash", subject_symbol_id="Demo::sedan")

    assert excinfo.value.capability == CAPABILITY_EVALUATE_SUBJECT
    assert service.requests == [], "the request was sent to a service that ignores it"


def test_evaluation_without_a_subject_asks_for_nothing(fake_service):
    """A plain evaluation keeps working against a service that predates the subject."""
    port, service = fake_service(capabilities=())
    with Connection(port=port, auto_start=False) as conn:
        assert conn.eval("2 + 2", "fake-hash") == 1200.0

    (request,) = service.requests
    assert request.subject_symbol_id == ""


def test_a_subject_reaches_a_service_that_reports_the_capability(fake_service):
    """The subject is carried on the request the service receives."""
    port, service = fake_service()
    with Connection(port=port, auto_start=False) as conn:
        assert conn.eval("mass", "fake-hash", subject_symbol_id="Demo::sedan") == 1200.0

    (request,) = service.requests
    assert request.subject_symbol_id == "Demo::sedan"

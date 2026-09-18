"""Tests for the capability gate on reading an object's values.

Every service published before 0.1.0 populates only the removed ``Instance.slots``
field, whose number is now reserved: such a response decodes into unknown fields,
so an instance would arrive with no values at all — which cannot be told from an
object whose features are genuinely unset. The client requires the capability
instead, so the stale service is named.
"""

from concurrent import futures

import grpc
import pytest

from opensysml.capabilities import CAPABILITY_FEATURE_VALUES, MissingCapabilityError
from opensysml.connection import Connection
from opensysml.proto import sysml_pb2, sysml_pb2_grpc


class FakeService(sysml_pb2_grpc.SysMLServiceServicer):
    """A sysml-grpc whose Instantiate records the request and answers one object."""

    def __init__(self, capabilities=(CAPABILITY_FEATURE_VALUES,)):
        self._capabilities = list(capabilities)
        self.requests = []

    def GetServerInfo(self, request, context):
        return sysml_pb2.ServerInfoResponse(
            version="fake", capabilities=self._capabilities
        )

    def GetDiagnostics(self, request, context):
        context.abort(grpc.StatusCode.NOT_FOUND, "model not found")

    def Instantiate(self, request, context):
        self.requests.append(request)
        instance = sysml_pb2.Instance(
            id=1,
            type_symbol_id="Demo::Sedan",
            feature_values={
                "mass": sysml_pb2.FeatureValue(value=sysml_pb2.Value(real_value=1200.0))
            },
        )
        return sysml_pb2.InstantiateResponse(instance=instance, instances=[instance])


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


def test_instantiating_requires_the_capability(fake_service):
    """A service that would answer with no values is named, not asked."""
    port, service = fake_service(capabilities=())
    with Connection(port=port, auto_start=False) as conn:
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.instantiate("Demo::sedan", "fake-hash")

    assert excinfo.value.capability == CAPABILITY_FEATURE_VALUES
    assert service.requests == [], "the request was sent to a service with no values"


def test_instantiating_against_a_reporting_service_reads_the_values(fake_service):
    """The gate is transparent to a service that populates feature_values."""
    port, service = fake_service()
    with Connection(port=port, auto_start=False) as conn:
        instance = conn.instantiate("Demo::sedan", "fake-hash")

    assert instance.mass == 1200.0
    assert [request.symbol_id for request in service.requests] == ["Demo::sedan"]

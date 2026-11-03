"""A service that requires a bearer token admits the right one and refuses the rest."""

import os
import subprocess

import pytest

import opensysml
from opensysml import AuthenticationError
from tests.service_gate import service_binary

TOKEN = "integration-token_1"


@pytest.fixture
def guarded_service():
    """A service on a loopback port the kernel chose, serving only with TOKEN."""
    process = subprocess.Popen(
        [service_binary(), "-port", "0", "-health-port", "0", "-report-address", "-token", TOKEN],
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
    )
    try:
        address = process.stdout.readline().decode().strip()
        assert address, "the service reported no address"
        yield address
    finally:
        process.kill()
        process.wait(timeout=10)


@pytest.mark.integration
class TestBearerToken:
    def test_the_right_token_is_admitted(self, guarded_service):
        with opensysml.connect(guarded_service, token=TOKEN) as conn:
            assert conn.server_info().version

    def test_the_wrong_token_is_refused(self, guarded_service):
        # An externally managed service is asked at the first call, not at connect.
        with opensysml.connect(guarded_service, token=TOKEN + "x") as conn:
            with pytest.raises(AuthenticationError) as excinfo:
                conn.server_info()
        assert "token" in str(excinfo.value)

    def test_no_token_is_refused(self, guarded_service, monkeypatch):
        monkeypatch.delenv("OPENSYSML_GRPC_TOKEN", raising=False)
        with opensysml.connect(guarded_service) as conn:
            with pytest.raises(AuthenticationError):
                conn.server_info()

    def test_the_environment_names_the_token(self, guarded_service, monkeypatch):
        monkeypatch.setenv("OPENSYSML_GRPC_TOKEN", TOKEN)
        with opensysml.connect(guarded_service) as conn:
            assert conn.server_info().version


def test_the_token_rides_every_call_as_bearer_metadata():
    from opensysml.connection import _BearerToken

    class Details:
        metadata = (("x", "y"),)

        def _replace(self, metadata):
            return metadata

    sent = _BearerToken("abc").intercept_unary_unary(lambda d, r: d, Details(), None)
    assert ("authorization", "Bearer abc") in sent
    assert ("x", "y") in sent


def test_a_private_child_does_not_inherit_the_environment_token(monkeypatch):
    from opensysml.connection import _PrivateService

    binary = service_binary()
    if binary is None:
        pytest.skip("no sysml-grpc binary to start")
    monkeypatch.setenv("OPENSYSML_BINARY", binary)
    monkeypatch.setenv("OPENSYSML_GRPC_TOKEN", TOKEN)
    service = _PrivateService.start(None, None)
    try:
        # The child is this process's own, on loopback: a call carrying no token is served.
        monkeypatch.delenv("OPENSYSML_GRPC_TOKEN")
        with opensysml.connect(service.address) as conn:
            assert conn.server_info().version
    finally:
        service.stop()

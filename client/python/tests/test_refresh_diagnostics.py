"""Asking a model's diagnostics again under a lint selection.

``Model.diagnostics`` is what the parse answered under the default selection;
``Model.refresh_diagnostics`` asks the service again, leaving out the lints
``disable_lint`` names and keeping the opt-in ones ``enable_lint`` names, as
``sysml -disable-lint``/``-enable-lint`` select them.
"""

from concurrent import futures

import grpc
import pytest

from opensysml import Connection
from opensysml.errors import InvalidRequestError, ModelNotFoundError
from opensysml.proto import sysml_pb2, sysml_pb2_grpc

LINTED = """package test {
	private import ScalarValues::*;
	attribute x : Real = 0.1;
	state def M { entry; then a; state a; state b; transition first a when Pnig then b; }
}
"""


class FakeService(sysml_pb2_grpc.SysMLServiceServicer):
    """A sysml-grpc whose GetDiagnostics records the request it was sent."""

    def __init__(self, status=None):
        self._status = status
        self.requests = []

    def GetServerInfo(self, request, context):
        return sysml_pb2.ServerInfoResponse(version="fake", capabilities=[])

    def ParseFile(self, request, context):
        return sysml_pb2.ParseFileResponse(
            model_hash="fake-hash",
            root=sysml_pb2.SymbolInfo(id="top.sysml", name="", kind="Namespace"),
            diagnostics=[_diagnostic("undeclared-signal")],
        )

    def GetDiagnostics(self, request, context):
        self.requests.append(request)
        if self._status is not None:
            code, details = self._status
            context.abort(code, details)
        kept = [_diagnostic("undeclared-signal")]
        kept = [d for d in kept if d.code not in request.disable_lint]
        kept += [_diagnostic(code) for code in request.enable_lint]
        return sysml_pb2.DiagnosticsResponse(diagnostics=kept)


def _diagnostic(code):
    return sysml_pb2.Diagnostic(
        severity="warning", message=code, code=code,
        span=sysml_pb2.Span(file="top.sysml", start_line=4, start_col=1, end_line=4, end_col=5),
    )


@pytest.fixture
def fake_service():
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


def test_the_selection_reaches_the_service_as_sent(fake_service):
    port, service = fake_service()
    with Connection(port=port, auto_start=False) as conn:
        model = conn.load_from_content(LINTED)
        default = model.refresh_diagnostics()
        selected = model.refresh_diagnostics(
            disable_lint=["undeclared-signal"], enable_lint=("rounded-real-literal",)
        )
    assert service.requests == [
        sysml_pb2.DiagnosticsRequest(model_hash="fake-hash"),
        sysml_pb2.DiagnosticsRequest(
            model_hash="fake-hash",
            disable_lint=["undeclared-signal"],
            enable_lint=["rounded-real-literal"],
        ),
    ]
    assert [d.code for d in default] == ["undeclared-signal"]
    assert [d.code for d in selected] == ["rounded-real-literal"]
    assert [d.code for d in model.diagnostics] == ["undeclared-signal"]


def test_a_code_naming_no_lint_is_an_invalid_request(fake_service):
    port, _ = fake_service(status=(grpc.StatusCode.INVALID_ARGUMENT, "no lint is named no-such-lint"))
    with Connection(port=port, auto_start=False) as conn:
        model = conn.load_from_content(LINTED)
        with pytest.raises(InvalidRequestError, match="no-such-lint"):
            model.refresh_diagnostics(disable_lint=["no-such-lint"])


def test_a_model_the_service_dropped_is_not_found(fake_service):
    port, _ = fake_service(status=(grpc.StatusCode.NOT_FOUND, "model not found: fake-hash"))
    with Connection(port=port, auto_start=False) as conn:
        model = conn.load_from_content(LINTED)
        with pytest.raises(ModelNotFoundError):
            model.refresh_diagnostics()


class TestAgainstTheService:
    """The selection as the service applies it, over the gRPC conformance model."""

    def setup_method(self):
        self.conn = Connection()

    def teardown_method(self):
        self.conn.close()

    def test_disabling_a_lint_leaves_its_diagnostics_out(self):
        model = self.conn.load_from_content(LINTED)
        assert "undeclared-signal" in {d.code for d in model.diagnostics}
        assert "undeclared-signal" in {d.code for d in model.refresh_diagnostics()}
        refreshed = model.refresh_diagnostics(disable_lint=["undeclared-signal"])
        assert "undeclared-signal" not in {d.code for d in refreshed}

    def test_enabling_an_opt_in_lint_keeps_its_diagnostics(self):
        model = self.conn.load_from_content(LINTED)
        assert "rounded-real-literal" not in {d.code for d in model.diagnostics}
        codes = {d.code for d in model.refresh_diagnostics(enable_lint=["rounded-real-literal"])}
        assert {"undeclared-signal", "rounded-real-literal"} <= codes

    def test_a_code_naming_no_lint_is_refused_naming_the_lints(self):
        model = self.conn.load_from_content(LINTED)
        with pytest.raises(InvalidRequestError, match="no-such-lint"):
            model.refresh_diagnostics(enable_lint=["no-such-lint"])

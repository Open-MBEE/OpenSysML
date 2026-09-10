"""Tests for writing a model back out through the service.

Two layers. Against a fake service, the client's own behavior: the capability
gate, how a request is shaped, and how a reported failure becomes an exception.
Against the real ``sysml-grpc`` binary, the round trip itself — notation back to
notation, and a trip through RDF Turtle — which is the part a mock cannot tell
you anything about.
"""

import os
import subprocess
import time
import warnings
from concurrent import futures

import grpc
import pytest

from opensysml.capabilities import CAPABILITY_CONVERT, MissingCapabilityError
from opensysml.connection import Connection
from opensysml.conversion import (
    FORMAT_SYSML,
    FORMAT_TURTLE,
    ExperimentalFeatureWarning,
    format_of_path,
    is_experimental,
)
from opensysml.errors import ConversionError, InvalidRequestError, ModelNotFoundError
from opensysml.proto import sysml_pb2, sysml_pb2_grpc

MODEL = """package Demo {
    // a comment, which notation keeps and RDF does not
    part def Engine {
        attribute power : Real = 300.0;
    }
}
"""

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
# A local build, or the copy opensysml installs; CI publishes the binary as an
# artifact, which does not carry the executable bit into the repo's bin.
GRPC_BINARIES = (
    os.path.join(REPO_ROOT, "bin", "sysml-grpc"),
    os.path.join(os.path.expanduser("~"), ".opensysml", "bin", "sysml-grpc"),
)


class FakeService(sysml_pb2_grpc.SysMLServiceServicer):
    """A sysml-grpc whose Convert records the request and answers as told."""

    def __init__(self, capabilities=(CAPABILITY_CONVERT,), error="", diagnostics=0,
                 experimental_notice="", refuse=False):
        self._capabilities = list(capabilities)
        self._error = error
        self._diagnostics = diagnostics
        self._experimental_notice = experimental_notice
        self._refuse = refuse
        self.requests = []

    def GetServerInfo(self, request, context):
        return sysml_pb2.ServerInfoResponse(
            version="fake", capabilities=self._capabilities
        )

    def GetDiagnostics(self, request, context):
        # The client's health probe reads NOT_FOUND for an unknown hash as "up".
        context.abort(grpc.StatusCode.NOT_FOUND, "model not found")

    def ParseFile(self, request, context):
        root = sysml_pb2.SymbolInfo(id="Demo", name="Demo", kind="Package")
        return sysml_pb2.ParseFileResponse(model_hash="fake-hash", root=root)

    def Convert(self, request, context):
        self.requests.append(request)
        if self._refuse:
            context.abort(
                grpc.StatusCode.UNIMPLEMENTED,
                "capability 'convert' is unavailable",
            )
        diagnostics = [
            sysml_pb2.Diagnostic(
                severity="error",
                message=f"syntax error {i}",
                span=sysml_pb2.Span(file="<content>", start_line=1),
            )
            for i in range(self._diagnostics)
        ]
        return sysml_pb2.ConvertResponse(
            content="" if self._error else "converted",
            from_format=request.from_format or "sysml",
            to_format=request.to_format,
            error=self._error,
            diagnostics=diagnostics,
            experimental=bool(self._experimental_notice),
            experimental_notice=self._experimental_notice,
        )


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


def test_format_of_path_infers_and_refuses():
    """A file name names the format, or the caller is told to say it."""
    assert format_of_path("model.sysml") == FORMAT_SYSML
    assert format_of_path("model.KerML") == FORMAT_SYSML
    assert format_of_path("model.ttl") == FORMAT_TURTLE
    assert format_of_path("model.turtle") == FORMAT_TURTLE
    with pytest.raises(ValueError, match="cannot tell the format"):
        format_of_path("model.json")


def test_is_experimental_names_the_rdf_mapping():
    """Either side being RDF, or v1 XMI input, makes a conversion experimental."""
    assert is_experimental(FORMAT_SYSML, FORMAT_TURTLE)
    assert is_experimental(FORMAT_TURTLE, FORMAT_SYSML)
    assert is_experimental("turtle", "rdf")
    assert is_experimental("xmi", FORMAT_SYSML)
    assert is_experimental("uml", FORMAT_TURTLE)
    assert not is_experimental(FORMAT_SYSML, FORMAT_SYSML)


def test_rdf_conversion_warns_and_says_so_on_the_result(fake_service):
    """An RDF conversion warns, and carries the status a caller can read."""
    port, _ = fake_service(experimental_notice="RDF conversion is experimental: only structure")
    with Connection(port=port, auto_start=False) as conn:
        with pytest.warns(ExperimentalFeatureWarning, match="only structure"):
            result = conn.convert("ttl", content=MODEL, from_format="sysml")
    assert result.experimental is True
    assert result.experimental_notice == "RDF conversion is experimental: only structure"


def test_rdf_conversion_is_experimental_even_against_an_older_service(fake_service):
    """The mapping's status is the mapping's, so a silent service is read from
    the formats it reports rather than taken as stable."""
    port, _ = fake_service()
    with Connection(port=port, auto_start=False) as conn:
        with pytest.warns(ExperimentalFeatureWarning, match="rdf-mapping.md"):
            result = conn.convert("ttl", content=MODEL, from_format="sysml")
    assert result.experimental is True


def test_notation_conversion_does_not_warn(fake_service):
    """Writing notation is stable, so nothing is warned about."""
    port, _ = fake_service()
    with warnings.catch_warnings():
        warnings.simplefilter("error", ExperimentalFeatureWarning)
        with Connection(port=port, auto_start=False) as conn:
            result = conn.convert("sysml", content=MODEL, from_format="sysml")
    assert result.experimental is False
    assert result.experimental_notice == ""


def test_a_refused_rdf_conversion_still_warns(fake_service):
    """A refusal is the experimental behavior, so it is warned about before the
    error is raised."""
    port, _ = fake_service(error="cannot convert the initial node")
    with Connection(port=port, auto_start=False) as conn:
        refused = pytest.raises(ConversionError)
        with pytest.warns(ExperimentalFeatureWarning):
            with refused:
                conn.convert("ttl", content=MODEL, from_format="sysml")


def test_convert_requires_the_capability(fake_service):
    """A service that cannot convert is named, not asked."""
    port, service = fake_service(capabilities=())
    with Connection(port=port, auto_start=False) as conn:
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.convert("ttl", content=MODEL, from_format="sysml")
    assert excinfo.value.capability == CAPABILITY_CONVERT
    assert service.requests == [], "the request was sent to a service that cannot serve it"

def test_service_side_capability_refusal_is_a_missing_capability_error(fake_service):
    """A refusal stays typed even when a stale handshake claimed support."""
    port, service = fake_service(refuse=True)
    with Connection(port=port, auto_start=False) as conn:
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.convert("ttl", content=MODEL, from_format="sysml")
    assert excinfo.value.capability == CAPABILITY_CONVERT
    assert isinstance(excinfo.value.__cause__, grpc.RpcError)
    assert excinfo.value.__cause__.code() == grpc.StatusCode.UNIMPLEMENTED
    assert len(service.requests) == 1


def test_convert_sends_the_source_and_formats(fake_service):
    """The request carries what the caller asked for, unchanged."""
    port, service = fake_service()
    with Connection(port=port, auto_start=False) as conn:
        result = conn.convert(
            "ttl", content=MODEL, from_format="sysml", tolerate_syntax_errors=True
        )

    (request,) = service.requests
    assert request.content == MODEL
    assert request.WhichOneof("source") == "content"
    assert (request.from_format, request.to_format) == ("sysml", "ttl")
    assert request.tolerate_syntax_errors is True
    assert str(result) == "converted"
    assert (result.from_format, result.to_format) == ("sysml", "ttl")


def test_convert_needs_exactly_one_source(fake_service):
    """A source that is both or neither is a caller error, not a request."""
    port, service = fake_service()
    with Connection(port=port, auto_start=False) as conn:
        with pytest.raises(ValueError):
            conn.convert("ttl", from_format="sysml")
        with pytest.raises(ValueError):
            conn.convert("ttl", file_path="m.sysml", content=MODEL)
        with pytest.raises(ValueError):
            conn.convert("ttl", content=MODEL, model_hash="abc", from_format="sysml")
    assert service.requests == []


def test_convert_raises_with_the_diagnostics_behind_a_failure(fake_service):
    """A reported failure becomes an exception carrying its diagnostics."""
    port, _ = fake_service(error="line 1: unexpected end of input", diagnostics=2)
    with Connection(port=port, auto_start=False) as conn:
        with pytest.raises(ConversionError) as excinfo:
            conn.convert("ttl", content="package P { part def ", from_format="sysml")
    assert "unexpected end of input" in str(excinfo.value)
    assert [d.message for d in excinfo.value.diagnostics] == [
        "syntax error 0",
        "syntax error 1",
    ]


def test_tolerated_syntax_errors_are_reported_on_the_result(fake_service):
    """Output the service tolerated arrives with the errors it tolerated."""
    port, _ = fake_service(diagnostics=1)
    with Connection(port=port, auto_start=False) as conn:
        result = conn.convert(
            "sysml", content=MODEL, from_format="sysml", tolerate_syntax_errors=True
        )
    assert [d.message for d in result.diagnostics] == ["syntax error 0"]


def test_model_is_written_out_by_its_hash(fake_service, tmp_path):
    """A model converts what the service parsed, not the file as it stands now."""
    port, service = fake_service()
    path = tmp_path / "model.sysml"
    path.write_text(MODEL)

    with Connection(port=port, auto_start=False) as conn:
        model = conn.load(str(path))
        path.write_text("package Replaced { part def Other; }\n")
        model.to_turtle()

    (request,) = service.requests
    assert request.WhichOneof("source") == "model_hash"
    assert request.model_hash == model.hash
    assert request.to_format == "ttl"
    # The path is still remembered, for a caller who does want the file as it is.
    assert model.source_path == str(path)


def test_conversion_writes_a_file(fake_service, tmp_path):
    """Saving picks the format from the extension and writes the text."""
    port, service = fake_service()
    out = tmp_path / "out.ttl"
    with Connection(port=port, auto_start=False) as conn:
        conn.load_from_content(MODEL).save(str(out))
    assert out.read_text() == "converted"
    assert service.requests[-1].to_format == "ttl"


def test_saving_an_unknown_extension_is_refused(fake_service, tmp_path):
    """An extension naming no format is refused before anything is written."""
    port, _ = fake_service()
    out = tmp_path / "out.json"
    with Connection(port=port, auto_start=False) as conn:
        model = conn.load_from_content(MODEL)
        with pytest.raises(ValueError, match="cannot tell the format"):
            model.save(str(out))
    assert not out.exists()


@pytest.fixture(scope="module")
def real_service():
    """Run the built sysml-grpc on an ephemeral port, or skip."""
    binary = next((b for b in GRPC_BINARIES if os.access(b, os.X_OK)), None)
    if binary is None:
        pytest.skip(f"no executable sysml-grpc in {GRPC_BINARIES}; run: make build-grpc")

    port = 51151
    process = subprocess.Popen(
        [binary, "-port", str(port)],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    try:
        deadline = time.time() + 10
        while time.time() < deadline:
            with grpc.insecure_channel(f"localhost:{port}") as channel:
                try:
                    grpc.channel_ready_future(channel).result(timeout=0.5)
                    break
                except grpc.FutureTimeoutError:
                    continue
        else:
            pytest.fail("sysml-grpc did not start")
        yield port
    finally:
        process.terminate()
        process.wait(timeout=10)


@pytest.mark.integration
class TestRoundTripAgainstRealService:
    """The round trip itself, through the real converter."""

    def test_notation_round_trip_keeps_the_model_and_its_comments(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(MODEL)
            notation = str(model.to_sysml())

            assert "part def Engine" in notation
            assert "// a comment" in notation
            # The written notation is the same model: it parses to the same
            # symbols and reports the same diagnostics, and formatting is stable.
            again = conn.load_from_content(notation)
            assert again.find("Engine") is not None
            assert [(d.severity, d.message) for d in again.diagnostics] == [
                (d.severity, d.message) for d in model.diagnostics
            ]
            assert str(again.to_sysml()) == notation

    def test_turtle_round_trip_returns_an_equivalent_model(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            turtle = str(conn.load_from_content(MODEL).to_turtle())
            assert "Demo::Engine" in turtle

            back = conn.convert("sysml", content=turtle, from_format="ttl")
            assert "part def Engine" in str(back)
            assert back.from_format == "ttl"
            # A model, not just text: it parses and resolves the same element.
            assert conn.load_from_content(str(back)).find("Engine") is not None

    def test_saving_a_loaded_file_writes_both_formats(self, real_service, tmp_path):
        source = tmp_path / "model.sysml"
        source.write_text(MODEL)
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load(str(source))
            assert model.source_path == str(source)
            model.save(str(tmp_path / "out.sysml"))
            model.save(str(tmp_path / "out.ttl"))

        assert "part def Engine" in (tmp_path / "out.sysml").read_text()
        assert "Demo::Engine" in (tmp_path / "out.ttl").read_text()

    def test_unreadable_notation_fails_with_spans(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            with pytest.raises(ConversionError) as excinfo:
                conn.convert("ttl", content="package P { part def ", from_format="sysml")
        assert excinfo.value.diagnostics, "a syntax error was reported without diagnostics"
        assert excinfo.value.diagnostics[0].span.start_line >= 1

    def test_tolerating_syntax_errors_writes_notation_anyway(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            result = conn.convert(
                "sysml",
                content="package P { part def }",
                from_format="sysml",
                tolerate_syntax_errors=True,
            )
        assert str(result), "tolerant conversion wrote nothing"
        assert result.diagnostics, "tolerant conversion hid the errors it tolerated"

    def test_a_loaded_model_writes_the_source_it_was_parsed_from(
        self, real_service, tmp_path
    ):
        source = tmp_path / "model.sysml"
        source.write_text(MODEL)
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load(str(source))
            source.write_text("package Replaced { part def Other; }\n")

            assert "part def Engine" in str(model.to_sysml())
            # The path, asked for explicitly, is the file as it stands now.
            current = conn.convert("sysml", file_path=str(source))
            assert "Other" in str(current)

    def test_a_model_the_service_no_longer_holds_names_the_eviction(self, real_service):
        # Translated at the client boundary now: the status the service failed
        # the call with stays reachable through the class and through __cause__.
        with Connection(port=real_service, auto_start=False) as conn:
            with pytest.raises(ModelNotFoundError) as excinfo:
                conn.convert("sysml", model_hash="nosuchmodel")
        assert "no longer cached" in str(excinfo.value)
        assert excinfo.value.code == grpc.StatusCode.NOT_FOUND
        cause = excinfo.value.__cause__
        assert isinstance(cause, grpc.RpcError)
        assert cause.code() == grpc.StatusCode.NOT_FOUND

    def test_an_unknown_format_is_an_invalid_request(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            with pytest.raises(InvalidRequestError) as excinfo:
                conn.convert("xmi", content=MODEL, from_format="sysml")
        assert excinfo.value.code == grpc.StatusCode.INVALID_ARGUMENT
        # Also a ValueError, so a caller checking arguments catches it.
        assert isinstance(excinfo.value, ValueError)
        assert excinfo.value.__cause__.code() == grpc.StatusCode.INVALID_ARGUMENT

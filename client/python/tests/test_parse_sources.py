"""Tests for parsing several documents as one model through ParseSources.

The request the wrapper sends, the capabilities it asks for and the errors it
raises are tested against a fake service that records what it was sent. That a
model of several documents resolves an import from one into another, and that
each diagnostic names its document, is tested against a real sysml-grpc on
localhost:50051 — the one CI starts, so with $OPENSYSML_REQUIRE_SERVICE set an
absent service fails rather than skips.
"""

from concurrent import futures
from pathlib import Path

import grpc

import pytest

import opensysml
from opensysml.capabilities import (
    CAPABILITY_INLINE_LANGUAGE,
    CAPABILITY_PARSE_SOURCES,
    CAPABILITY_STRICT_CONFORMANCE,
    MissingCapabilityError,
)
from opensysml.connection import Connection
from opensysml.errors import (
    InvalidRequestError,
    ModelError,
    ModelFileNotFoundError,
    SymbolNotFoundError,
)
from opensysml.generate import generate_source
from opensysml.model import Model
from opensysml.proto import sysml_pb2, sysml_pb2_grpc
from opensysml.sources import SourceDocument, source_documents
from tests.service_gate import fail_if_service_promised, is_server_available

LIBRARY = """
package Lib {
    part def Engine {
        attribute power : ScalarValues::Real = 120.0;
    }
}
"""

TOP = """
package Top {
    import Lib::*;
    part def Car {
        part engine : Engine;
    }
    part car : Car;
}
"""

BROKEN = """
package Broken {
    import Lib::*;
    part def Boat {
        part motor : Missing;
    }
}
"""

ALL_CAPABILITIES = (
    CAPABILITY_PARSE_SOURCES,
    CAPABILITY_INLINE_LANGUAGE,
    CAPABILITY_STRICT_CONFORMANCE,
)


class TestSourceDocument:
    """The document forms parse_sources accepts, and what each sends."""

    def test_a_path_is_a_file_document(self, tmp_path):
        docs = source_documents(["lib.sysml", tmp_path / "top.sysml"])

        assert [doc.path for doc in docs] == ["lib.sysml", str(tmp_path / "top.sysml")]
        assert [doc.document_name for doc in docs] == [docs[0].path, docs[1].path]
        assert docs[0].to_pb() == sysml_pb2.SourceDocument(file_path="lib.sysml")

    def test_a_pair_is_inline_content_under_a_name(self):
        (doc,) = source_documents([("lib.sysml", LIBRARY)])

        assert doc == SourceDocument.inline("lib.sysml", LIBRARY)
        assert doc.document_name == "lib.sysml"
        assert doc.to_pb() == sysml_pb2.SourceDocument(
            content=LIBRARY, name="lib.sysml", language=""
        )

    def test_inline_content_may_declare_its_language(self):
        doc = SourceDocument.inline("lib.kerml", "package Lib;", language="kerml")

        assert doc.to_pb() == sysml_pb2.SourceDocument(
            content="package Lib;", name="lib.kerml", language="kerml"
        )

    @pytest.mark.parametrize(
        "kwargs, complaint",
        [
            ({}, "either a file path or inline content"),
            ({"path": "a.sysml", "content": "package A;"}, "either a file path or inline content"),
            ({"path": ""}, "needs a path"),
            ({"path": "a.sysml", "name": "a"}, "named by its path"),
            ({"path": "a.sysml", "language": "sysml"}, "extension says"),
            ({"content": "package A;"}, "needs a name"),
            ({"content": "package A;", "name": "a", "language": "java"}, "'sysml' or 'kerml'"),
        ],
    )
    def test_an_ill_formed_document_is_refused(self, kwargs, complaint):
        with pytest.raises(ValueError, match=complaint):
            SourceDocument(**kwargs)

    def test_no_documents_is_refused(self):
        with pytest.raises(ValueError, match="at least one document"):
            source_documents([])

    def test_two_documents_of_one_name_are_refused(self):
        with pytest.raises(ValueError, match="two documents are named 'a.sysml'"):
            source_documents([("a.sysml", "package A;"), ("a.sysml", "package B;")])

    def test_a_document_of_another_form_is_refused(self):
        with pytest.raises(ValueError, match="document 1 is int"):
            source_documents(["a.sysml", 3])
        with pytest.raises(ValueError, match="document 0 is tuple"):
            source_documents([("a.sysml", "package A;", "extra")])
        with pytest.raises(ValueError, match="document 0 is tuple"):
            source_documents([("a.sysml", b"package A;")])

    @pytest.mark.parametrize(
        "document",
        ["lib.sysml", Path("lib.sysml"), ("lib.sysml", LIBRARY),
         SourceDocument.inline("lib.sysml", LIBRARY)],
        ids=["str", "path", "pair", "SourceDocument"],
    )
    def test_one_document_outside_a_sequence_is_refused(self, document):
        """A bare string is not read as its characters, each a document."""
        with pytest.raises(ValueError, match="sequence of documents"):
            source_documents(document)

    @pytest.mark.parametrize(
        "kwargs, complaint",
        [
            ({"path": b"a.sysml"}, "path must be str, not bytes"),
            ({"path": 3}, "path must be str, not int"),
            ({"content": b"package A;", "name": "a"}, "content must be str, not bytes"),
            ({"content": "package A;", "name": 7}, "name must be str, not int"),
            ({"content": "package A;", "name": "a", "language": 1}, "language must be str, not int"),
        ],
    )
    def test_a_field_of_another_type_is_refused(self, kwargs, complaint):
        with pytest.raises(TypeError, match=complaint):
            SourceDocument(**kwargs)


class FakeService(sysml_pb2_grpc.SysMLServiceServicer):
    """A sysml-grpc whose ParseSources records the request it was sent.

    It answers one root per document, named after the document, and can be
    told to fail the call with a status or to fill the response's ``error``.
    """

    def __init__(self, capabilities=ALL_CAPABILITIES, error="", status=None):
        self._capabilities = list(capabilities)
        self._error = error
        self._status = status
        self.requests = []

    def GetServerInfo(self, request, context):
        return sysml_pb2.ServerInfoResponse(
            version="fake", capabilities=self._capabilities
        )

    def ParseSources(self, request, context):
        self.requests.append(request)
        if self._status is not None:
            code, details = self._status
            context.abort(code, details)
        if self._error:
            return sysml_pb2.ParseSourcesResponse(
                error=self._error,
                diagnostics=[_diagnostic("error", "unreadable", "top.sysml")],
            )
        roots = [
            sysml_pb2.SymbolInfo(id=doc.name or doc.file_path, name="", kind="Namespace")
            for doc in request.documents
        ]
        return sysml_pb2.ParseSourcesResponse(
            model_hash="fake-hash",
            roots=roots,
            diagnostics=[_diagnostic("warning", "unused import", "top.sysml")],
        )


def _diagnostic(severity, message, file):
    return sysml_pb2.Diagnostic(
        severity=severity,
        message=message,
        span=sysml_pb2.Span(file=file, start_line=2, start_col=1, end_line=2, end_col=5),
    )


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


class TestParseSourcesRequest:
    """What the wrapper sends, and what it makes of the answer."""

    def test_documents_reach_the_service_in_order(self, fake_service, tmp_path):
        port, service = fake_service()
        path = tmp_path / "lib.sysml"
        with Connection(port=port, auto_start=False) as conn:
            model = conn.parse_sources([
                path,
                ("top.sysml", TOP),
                SourceDocument.inline("extra.kerml", "package Extra;", language="kerml"),
            ])

        (request,) = service.requests
        assert request == sysml_pb2.ParseSourcesRequest(documents=[
            sysml_pb2.SourceDocument(file_path=str(path)),
            sysml_pb2.SourceDocument(content=TOP, name="top.sysml"),
            sysml_pb2.SourceDocument(content="package Extra;", name="extra.kerml", language="kerml"),
        ])
        assert model.hash == "fake-hash"
        assert model.documents == (str(path), "top.sysml", "extra.kerml")
        assert [root.id for root in model.roots] == [str(path), "top.sysml", "extra.kerml"]
        assert model.root is model.roots[0]
        assert [(d.file, d.message) for d in model.diagnostics] == [("top.sysml", "unused import")]
        assert model.ok

    def test_strict_conformance_reaches_the_service(self, fake_service):
        port, service = fake_service()
        with Connection(port=port, auto_start=False) as conn:
            conn.parse_sources([("a.sysml", "package A;")], strict_conformance=True)
            conn.parse_sources([("a.sysml", "package A;")])

        assert [request.strict_conformance for request in service.requests] == [True, False]

    def test_the_parse_sources_capability_is_required(self, fake_service):
        port, service = fake_service(capabilities=())
        with Connection(port=port, auto_start=False) as conn:
            with pytest.raises(MissingCapabilityError) as excinfo:
                conn.parse_sources([("a.sysml", "package A;")])

        assert excinfo.value.capability == CAPABILITY_PARSE_SOURCES
        assert service.requests == [], "the request was sent to a service that cannot answer it"

    def test_strict_conformance_requires_its_capability(self, fake_service):
        port, service = fake_service(capabilities=(CAPABILITY_PARSE_SOURCES,))
        with Connection(port=port, auto_start=False) as conn:
            with pytest.raises(MissingCapabilityError) as excinfo:
                conn.parse_sources([("a.sysml", "package A;")], strict_conformance=True)

        assert excinfo.value.capability == CAPABILITY_STRICT_CONFORMANCE
        assert service.requests == []

    def test_a_declared_language_requires_its_capability(self, fake_service):
        port, service = fake_service(capabilities=(CAPABILITY_PARSE_SOURCES,))
        with Connection(port=port, auto_start=False) as conn:
            with pytest.raises(MissingCapabilityError) as excinfo:
                conn.parse_sources([
                    SourceDocument.inline("a.kerml", "package A;", language="kerml"),
                ])

        assert excinfo.value.capability == CAPABILITY_INLINE_LANGUAGE
        assert service.requests == []

    def test_an_unimplemented_status_names_the_capability(self, fake_service):
        """A service advertising the capability but refusing the call is stale."""
        port, _ = fake_service(
            status=(grpc.StatusCode.UNIMPLEMENTED, "parse_sources is not implemented")
        )
        with Connection(port=port, auto_start=False) as conn:
            with pytest.raises(MissingCapabilityError) as excinfo:
                conn.parse_sources([("a.sysml", "package A;")])

        assert excinfo.value.capability == CAPABILITY_PARSE_SOURCES

    def test_the_responses_error_is_a_model_error(self, fake_service):
        port, _ = fake_service(error="documents could not be parsed as one model")
        with Connection(port=port, auto_start=False) as conn:
            with pytest.raises(ModelError) as excinfo:
                conn.parse_sources([("a.sysml", "package A;"), ("top.sysml", TOP)])

        assert excinfo.value.message == "documents could not be parsed as one model"
        assert [(d.file, d.message) for d in excinfo.value.diagnostics] == [
            ("top.sysml", "unreadable")
        ]
        assert excinfo.value.model is None

    def test_a_missing_file_is_a_file_not_found(self, fake_service):
        port, _ = fake_service(
            status=(grpc.StatusCode.NOT_FOUND, "file not found: missing.sysml")
        )
        with Connection(port=port, auto_start=False) as conn:
            with pytest.raises(ModelFileNotFoundError):
                conn.parse_sources(["missing.sysml"])

    def test_a_refused_request_is_an_invalid_request(self, fake_service):
        port, _ = fake_service(
            status=(grpc.StatusCode.INVALID_ARGUMENT, "duplicate document name")
        )
        with Connection(port=port, auto_start=False) as conn:
            with pytest.raises(InvalidRequestError, match="duplicate document name"):
                conn.parse_sources([("a.sysml", "package A;")])

    def test_no_documents_is_refused_before_the_call(self, fake_service):
        port, service = fake_service()
        with Connection(port=port, auto_start=False) as conn:
            with pytest.raises(ValueError, match="at least one document"):
                conn.parse_sources([])

        assert service.requests == []

    def test_strict_refuses_a_model_with_errors(self, fake_service):
        port, _ = fake_service()
        with Connection(port=port, auto_start=False) as conn:
            model = conn.parse_sources([("top.sysml", TOP)])
            model._diagnostics.append(
                opensysml.Diagnostic(_diagnostic("error", "unresolved", "top.sysml"))
            )
            with pytest.raises(ModelError) as excinfo:
                model.raise_for_errors()

        assert excinfo.value.model is model
        assert [d.file for d in excinfo.value.diagnostics] == ["top.sysml"]


def test_module_level_parse_sources_uses_the_default_connection(monkeypatch):
    """opensysml.parse_sources delegates as opensysml.loads does."""
    calls = []

    class Recorder:
        def parse_sources(self, documents, strict=False, strict_conformance=False):
            calls.append((documents, strict, strict_conformance))
            return "model"

    monkeypatch.setattr(opensysml, "_default_connection", None)
    monkeypatch.setattr(opensysml, "_default_connection_params", None)
    monkeypatch.setattr(
        opensysml, "_get_default_connection", lambda *args: Recorder()
    )

    docs = [("a.sysml", "package A;")]
    assert opensysml.parse_sources(docs, strict=True, strict_conformance=True) == "model"
    assert calls == [(docs, True, True)]


class TestModelOfSeveralRoots:
    """A Model built from a ParseSourcesResponse has one root per document."""

    @staticmethod
    def _model():
        response = sysml_pb2.ParseSourcesResponse(
            model_hash="h",
            roots=[
                sysml_pb2.SymbolInfo(id="Lib", name="Lib", kind="Package"),
                sysml_pb2.SymbolInfo(id="Top", name="Top", kind="Package"),
            ],
        )
        return Model(response, None, documents=["lib.sysml", "top.sysml"])

    def test_roots_and_documents_are_in_document_order(self):
        model = self._model()

        assert [root.id for root in model.roots] == ["Lib", "Top"]
        assert model.root.id == "Lib"
        assert model.documents == ("lib.sysml", "top.sysml")
        assert model.source_path is None

    def test_every_root_is_found_by_name_without_a_call(self):
        """A root of any document answers find/get, as the sole root does."""
        model = self._model()

        assert model.find("Top").id == "Top"
        assert model.get("Top").id == "Top"
        assert model["Lib"].id == "Lib"

    def test_a_model_of_one_document_has_one_root(self):
        response = sysml_pb2.ParseFileResponse(
            model_hash="h", root=sysml_pb2.SymbolInfo(id="Demo", name="Demo", kind="Package")
        )
        model = Model(response, None, source_path="demo.sysml")

        assert model.roots == (model.root,)
        assert model.documents == ("demo.sysml",)
        assert Model(response, None).documents == ()


def _typed_by(symbol):
    """The id of the definition ``symbol`` is typed by, through its facts."""
    (typing,) = [spec for spec in symbol.specializations if spec.kind == "typing"]
    return typing.target_id


_AVAILABLE = is_server_available()
fail_if_service_promised(_AVAILABLE)


@pytest.mark.integration
@pytest.mark.skipif(not _AVAILABLE, reason="sysml-grpc server not running on localhost:50051")
class TestParseSourcesAgainstRealService:
    """Two documents, the second importing the first, parsed as one model."""

    def test_an_import_between_documents_resolves(self):
        with Connection(auto_start=False) as conn:
            model = conn.parse_sources([("lib.sysml", LIBRARY), ("top.sysml", TOP)])

            assert model.diagnostics == []
            assert model.ok
            assert model.documents == ("lib.sysml", "top.sysml")
            assert len(model.roots) == 2
            assert model["Lib::Engine"].kind == "partDef"
            assert model["Top::Car"].kind == "partDef"
            assert _typed_by(model["Top::Car::engine"]) == "Lib::Engine"
            assert model.find("Engine").id == "Lib::Engine"
            assert model.eval("Top::car.engine.power") == 120.0

    def test_files_and_inline_content_mix(self, tmp_path):
        lib = tmp_path / "lib.sysml"
        lib.write_text(LIBRARY)
        with Connection(auto_start=False) as conn:
            model = conn.parse_sources([lib, ("top.sysml", TOP)])
            assert model.ok
            assert model.documents == (str(lib), "top.sysml")
            assert _typed_by(model["Top::Car::engine"]) == "Lib::Engine"

            conn.parse_sources([str(lib), ("top.sysml", TOP)], strict=True)

    def test_a_diagnostic_names_the_document_it_came_from(self):
        with Connection(auto_start=False) as conn:
            model = conn.parse_sources([("lib.sysml", LIBRARY), ("broken.sysml", BROKEN)])

            assert not model.ok
            errors = [d for d in model.diagnostics if d.severity == "error"]
            assert errors, model.diagnostics
            assert {d.file for d in model.diagnostics} == {"broken.sysml"}
            assert any("Missing" in d.message for d in errors)
            # The sound document is still there to inspect.
            assert model["Lib::Engine"].kind == "partDef"

            with pytest.raises(ModelError) as excinfo:
                conn.parse_sources(
                    [("lib.sysml", LIBRARY), ("broken.sysml", BROKEN)], strict=True
                )
            assert {d.file for d in excinfo.value.diagnostics} == {"broken.sysml"}
            assert excinfo.value.model is not None

    def test_the_same_documents_concatenated_lose_the_names(self):
        """What the wrapper exists to avoid: one string names no document."""
        with Connection(auto_start=False) as conn:
            model = conn.load_from_content(LIBRARY + BROKEN)
            assert model.diagnostics
            assert "broken.sysml" not in {d.file for d in model.diagnostics}

    def test_the_service_refuses_duplicate_names(self, tmp_path):
        with Connection(auto_start=False) as conn:
            with pytest.raises(ValueError):
                conn.parse_sources([("a.sysml", LIBRARY), ("a.sysml", TOP)])
            with pytest.raises(ModelFileNotFoundError):
                conn.parse_sources([str(tmp_path / "missing.sysml")])

    def test_a_missing_symbol_raises_as_for_one_document(self):
        with Connection(auto_start=False) as conn:
            model = conn.parse_sources([("lib.sysml", LIBRARY), ("top.sysml", TOP)])
            with pytest.raises(SymbolNotFoundError):
                model["Top::Boat"]

    def test_strict_conformance_is_answered(self):
        with Connection(auto_start=False) as conn:
            model = conn.parse_sources(
                [("lib.sysml", LIBRARY), ("top.sysml", TOP)], strict_conformance=True
            )
            assert model.ok

    def test_module_level_parse_sources(self, monkeypatch):
        """Naming the service's address joins it rather than starting a child."""
        monkeypatch.setattr(opensysml, "_default_connection", None)
        monkeypatch.setattr(opensysml, "_default_connection_params", None)
        try:
            model = opensysml.parse_sources(
                [("lib.sysml", LIBRARY), ("top.sysml", TOP)], port=50051
            )
            assert model.ok
            assert model["Lib::Engine"].name == "Engine"
        finally:
            # monkeypatch puts the module back; only the connection needs closing.
            opensysml._default_connection.close()

    def test_a_typed_module_is_generated_from_every_document(self):
        """generate_source renders the definitions of each document, and a part
        typed across documents is annotated with the other document's class."""
        with Connection(auto_start=False) as conn:
            model = conn.parse_sources([("lib.sysml", LIBRARY), ("top.sysml", TOP)])
            source = generate_source(model, LIBRARY + TOP)

        assert "class Engine(" in source
        assert "class Car(" in source
        assert '_t.feature_value(self, "engine", _t.as_typed(Engine))' in source

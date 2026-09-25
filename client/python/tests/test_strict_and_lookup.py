"""Tests for strict loading, model validity and the raising symbol lookup."""

from unittest.mock import Mock, patch

import pytest

from opensysml.capabilities import CAPABILITY_QUERY, ServerInfo
from opensysml.connection import Connection
from opensysml.errors import ModelError, SymbolNotFoundError
from opensysml.model import Model
from opensysml.proto import sysml_pb2
from opensysml.query import QueryElement


def _response(diagnostics=(), children=()):
    root = sysml_pb2.SymbolInfo(
        id="Demo",
        name="Demo",
        kind="Package",
        child_ids=[child.id for child in children],
    )
    return sysml_pb2.ParseFileResponse(
        model_hash="abc123",
        root=root,
        diagnostics=list(diagnostics),
    )


def _diag(severity, message):
    return sysml_pb2.Diagnostic(
        severity=severity,
        message=message,
        span=sysml_pb2.Span(
            file="demo.sysml", start_line=1, start_col=1, end_line=1, end_col=2
        ),
    )


def _child(name, fqn=None):
    return sysml_pb2.SymbolInfo(
        id=fqn or f"Demo::{name}", name=name, kind="PartDefinition"
    )


def _model_with_children(*children, diagnostics=()):
    """A model over a service that answers an unknown id with None, as the real one does."""
    client = Mock()
    client.server_info.return_value = ServerInfo(
        version="test", capabilities=frozenset({CAPABILITY_QUERY}),
        answered=True, origin="test",
    )
    client.get_symbol.side_effect = lambda model_hash, symbol_id: next(
        (child for child in children if child.id == symbol_id), None
    )
    client.query.side_effect = lambda model_hash, select=None, where=None: [
        QueryElement(id=child.id, type=child.kind, properties={"name": child.name})
        for child in children
        if where is None or child.name == where["value"]
    ]
    return Model(_response(diagnostics, children), client)


class TestValidity:
    def test_a_clean_model_is_ok(self):
        model = Model(_response([_diag("warning", "unused symbol")]), Mock())
        assert model.ok
        assert model.errors == []

    def test_a_model_with_errors_is_not_ok(self):
        model = Model(_response([_diag("error", "expected ';'")]), Mock())
        assert not model.ok
        assert len(model.errors) == 1
        assert model.errors[0].message == "expected ';'"

    def test_severity_is_read_case_insensitively(self):
        # The service has reported both "error" and "ERROR" over this wire.
        model = Model(_response([_diag("ERROR", "expected ';'")]), Mock())
        assert not model.ok

    def test_raise_for_errors_returns_a_clean_model(self):
        model = Model(_response(), Mock())
        assert model.raise_for_errors() is model

    def test_raise_for_errors_names_the_source_and_keeps_the_model(self):
        response = _response([_diag("error", "expected ';'")])
        model = Model(response, Mock(), source_path="/tmp/demo.sysml")
        with pytest.raises(ModelError) as excinfo:
            model.raise_for_errors()
        assert "/tmp/demo.sysml" in str(excinfo.value)
        assert "expected ';'" in str(excinfo.value)
        assert excinfo.value.model is model
        assert len(excinfo.value.diagnostics) == 1

    def test_many_errors_are_summarized_not_dumped(self):
        diags = [_diag("error", f"problem {i}") for i in range(5)]
        model = Model(_response(diags), Mock())
        with pytest.raises(ModelError) as excinfo:
            model.raise_for_errors()
        assert "and 2 more" in str(excinfo.value)
        assert len(excinfo.value.diagnostics) == 5


class TestStrictLoading:
    def _connection(self, response):
        with patch('grpc.insecure_channel'):
            stub = Mock()
            stub.ParseFile.return_value = response
            with patch(
                'opensysml.proto.sysml_pb2_grpc.SysMLServiceStub', return_value=stub
            ):
                return Connection(auto_start=False)

    def test_loading_is_lenient_by_default(self):
        conn = self._connection(_response([_diag("error", "expected ';'")]))
        model = conn.load("broken.sysml")
        assert not model.ok

    def test_strict_loading_refuses_a_broken_model(self):
        conn = self._connection(_response([_diag("error", "expected ';'")]))
        with pytest.raises(ModelError) as excinfo:
            conn.load("broken.sysml", strict=True)
        assert "broken.sysml" in str(excinfo.value)
        # The model stays reachable, so the caller can report every diagnostic.
        assert excinfo.value.model.diagnostics

    def test_strict_loading_passes_a_clean_model_through(self):
        conn = self._connection(_response([_diag("warning", "unused")]))
        model = conn.load("fine.sysml", strict=True)
        assert model.hash == "abc123"

    def test_strict_loading_from_content(self):
        conn = self._connection(_response([_diag("error", "expected ';'")]))
        with pytest.raises(ModelError):
            conn.load_from_content("part def", strict=True)
        assert not conn.load_from_content("part def").ok


class TestRaisingLookup:
    def test_find_still_returns_none(self):
        model = _model_with_children(_child("Vehicle"))
        assert model.find("Nope") is None

    def test_subscript_returns_the_symbol(self):
        model = _model_with_children(_child("Vehicle"))
        assert model["Vehicle"].name == "Vehicle"
        assert model["Demo::Vehicle"].id == "Demo::Vehicle"

    def test_subscript_names_the_missing_symbol(self):
        model = _model_with_children(_child("Vehicle"))
        with pytest.raises(SymbolNotFoundError) as excinfo:
            model["Nope"]
        assert excinfo.value.name == "Nope"
        assert "'Nope'" in str(excinfo.value)

    def test_subscript_suggests_a_near_name(self):
        model = _model_with_children(_child("Vehicle"))
        with pytest.raises(SymbolNotFoundError) as excinfo:
            model["Vehicel"]
        assert "Vehicle" in str(excinfo.value)

    def test_membership_asks_without_raising(self):
        model = _model_with_children(_child("Vehicle"))
        assert "Vehicle" in model
        assert "Nope" not in model

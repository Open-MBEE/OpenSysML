"""Tests for Model class."""

import inspect

import pytest
from unittest.mock import Mock
from opensysml.connection import Connection
from opensysml.proto import sysml_pb2
from opensysml.errors import ExecutionError
from opensysml.model import Model


def test_model_properties():
    pb_root = sysml_pb2.SymbolInfo(
        id="MyModel",
        name="MyModel",
        kind="Package",
        metadata={},
        child_ids=["MyModel::Vehicle"],
        attributes=[],
    )
    
    pb_diag1 = sysml_pb2.Diagnostic(
        severity="error",
        message="Syntax error",
        span=sysml_pb2.Span(file="test.sysml", start_line=1, start_col=1, end_line=1, end_col=1),
    )
    
    pb_diag2 = sysml_pb2.Diagnostic(
        severity="warning",
        message="Unused symbol",
        span=sysml_pb2.Span(file="test.sysml", start_line=5, start_col=1, end_line=5, end_col=1),
    )
    
    pb_response = sysml_pb2.ParseFileResponse(
        model_hash="abc123",
        root=pb_root,
        diagnostics=[pb_diag1, pb_diag2],
    )
    
    mock_client = Mock()
    model = Model(pb_response, mock_client)
    
    # Check hash
    assert model.hash == "abc123"
    
    # Check root is a Symbol
    assert model.root.id == "MyModel"
    assert model.root.name == "MyModel"
    assert model.root.kind == "Package"
    
    # Check diagnostics are Diagnostic objects
    assert len(model.diagnostics) == 2
    assert model.diagnostics[0].severity == "error"
    assert model.diagnostics[0].message == "Syntax error"
    assert model.diagnostics[1].severity == "warning"
    assert model.diagnostics[1].message == "Unused symbol"


def test_model_str():
    pb_root = sysml_pb2.SymbolInfo(
        id="TestModel",
        name="TestModel",
        kind="Package",
        metadata={},
        child_ids=[],
        attributes=[],
    )
    
    pb_response = sysml_pb2.ParseFileResponse(
        model_hash="hash123",
        root=pb_root,
        diagnostics=[],
    )
    
    model = Model(pb_response, None)
    result = str(model)
    
    assert "TestModel" in result
    assert "Package" in result


def test_model_find():
    # Model with nested symbols
    pb_root = sysml_pb2.SymbolInfo(
        id="MyModel",
        name="MyModel",
        kind="Package",
        metadata={},
        child_ids=["MyModel::Vehicle", "MyModel::Sensor"],
        attributes=[],
    )
    
    pb_response = sysml_pb2.ParseFileResponse(
        model_hash="hash",
        root=pb_root,
        diagnostics=[],
    )
    
    mock_client = Mock()
    
    # Mock children for root
    pb_vehicle = sysml_pb2.SymbolInfo(
        id="MyModel::Vehicle",
        name="Vehicle",
        kind="PartDef",
        metadata={},
        child_ids=["MyModel::Vehicle::Engine"],
        attributes=[],
    )
    
    pb_sensor = sysml_pb2.SymbolInfo(
        id="MyModel::Sensor",
        name="Sensor",
        kind="PartDef",
        metadata={},
        child_ids=[],
        attributes=[],
    )
    
    pb_engine = sysml_pb2.SymbolInfo(
        id="MyModel::Vehicle::Engine",
        name="Engine",
        kind="PartUsage",
        metadata={},
        child_ids=[],
        attributes=[],
    )
    
    # Mock get_symbol calls
    def mock_get_symbol(model_hash, symbol_id):
        mapping = {
            "MyModel::Vehicle": pb_vehicle,
            "MyModel::Sensor": pb_sensor,
            "MyModel::Vehicle::Engine": pb_engine,
        }
        return mapping.get(symbol_id)
    
    mock_client.get_symbol.side_effect = mock_get_symbol
    
    model = Model(pb_response, mock_client)
    
    # Find top-level symbol
    vehicle = model.find("Vehicle")
    assert vehicle is not None
    assert vehicle.id == "MyModel::Vehicle"
    assert vehicle.name == "Vehicle"
    
    # Find nested symbol
    engine = model.find("Engine")
    assert engine is not None
    assert engine.id == "MyModel::Vehicle::Engine"
    assert engine.name == "Engine"
    
    # Find non-existent symbol
    missing = model.find("NonExistent")
    assert missing is None


def test_model_find_accepts_fully_qualified_name():
    """A symbol's own id round-trips back into find()."""
    pb_root = sysml_pb2.SymbolInfo(
        id="Lander",
        name="Lander",
        kind="package",
        metadata={},
        child_ids=["Lander::Rhs"],
        attributes=[],
    )

    pb_rhs = sysml_pb2.SymbolInfo(
        id="Lander::Rhs",
        name="Rhs",
        kind="calcDef",
        metadata={},
        child_ids=[],
        attributes=[],
    )

    mock_client = Mock()
    mock_client.get_symbol.side_effect = lambda model_hash, symbol_id: {
        "Lander::Rhs": pb_rhs,
    }.get(symbol_id)

    model = Model(
        sysml_pb2.ParseFileResponse(model_hash="hash", root=pb_root, diagnostics=[]),
        mock_client,
    )

    by_short_name = model.find("Rhs")
    by_fqn = model.find("Lander::Rhs")

    assert by_short_name is not None
    assert by_fqn is not None
    assert by_fqn.id == by_short_name.id == "Lander::Rhs"
    assert by_fqn.kind == "calcDef"

    # The root's own id is accepted too, and a name no symbol carries is not.
    assert model.find("Lander") is not None
    assert model.find("Lander::Missing") is None


def test_model_find_short_circuit():
    # Model with one child
    pb_root = sysml_pb2.SymbolInfo(
        id="Root",
        name="Root",
        kind="Package",
        metadata={},
        child_ids=["Root::Target"],
        attributes=[],
    )
    
    pb_response = sysml_pb2.ParseFileResponse(
        model_hash="hash",
        root=pb_root,
        diagnostics=[],
    )
    
    mock_client = Mock()
    
    pb_target = sysml_pb2.SymbolInfo(
        id="Root::Target",
        name="Target",
        kind="PartDef",
        metadata={},
        child_ids=["Root::Target::Nested"],
        attributes=[],
    )
    
    mock_client.get_symbol.return_value = pb_target
    
    model = Model(pb_response, mock_client)
    
    # Find should stop at first match (don't traverse into Target's children)
    target = model.find("Target")
    
    assert target is not None
    assert target.name == "Target"
    
    # Should have called get_symbol only once (for Target, not its children)
    # Note: children() is lazy, so get_symbol only called when explicitly requested


def test_model_get_by_fqn():
    pb_root = sysml_pb2.SymbolInfo(
        id="MyModel",
        name="MyModel",
        kind="Package",
        metadata={},
        child_ids=["MyModel::Vehicle"],
        attributes=[],
    )

    pb_vehicle = sysml_pb2.SymbolInfo(
        id="MyModel::Vehicle",
        name="Vehicle",
        kind="PartDef",
        metadata={},
        child_ids=["MyModel::Vehicle::engine"],
        attributes=[],
    )

    pb_engine = sysml_pb2.SymbolInfo(
        id="MyModel::Vehicle::engine",
        name="engine",
        kind="partUsage",
        metadata={},
        child_ids=[],
        attributes=[],
    )

    pb_response = sysml_pb2.ParseFileResponse(
        model_hash="hash",
        root=pb_root,
        diagnostics=[],
    )

    mock_client = Mock()
    mapping = {
        "MyModel::Vehicle": pb_vehicle,
        "MyModel::Vehicle::engine": pb_engine,
    }
    mock_client.get_symbol.side_effect = lambda model_hash, symbol_id: mapping.get(symbol_id)

    model = Model(pb_response, mock_client)

    assert model.get("MyModel").id == "MyModel"
    assert model.get("MyModel::Vehicle").name == "Vehicle"
    assert model.get("MyModel::Vehicle::engine").name == "engine"

    # A short name is not an FQN, and an unknown FQN is not an error.
    assert model.get("Vehicle") is None
    assert model.get("MyModel::Missing") is None


class TestModelEval:
    """Evaluation is on the model, so a caller need not carry its hash."""

    def _model(self, client):
        pb_response = sysml_pb2.ParseFileResponse(
            model_hash="hash1",
            root=sysml_pb2.SymbolInfo(id="Demo", name="Demo", kind="Package"),
        )
        return Model(pb_response, client)

    def test_eval_passes_the_models_hash(self):
        client = Mock()
        client.eval.return_value = 2

        assert self._model(client).eval("1+1") == 2
        client.eval.assert_called_once_with(
            "1+1", "hash1", context_symbol_id=None, subject_symbol_id=None
        )

    def test_eval_passes_a_context_symbol(self):
        client = Mock()
        client.eval.return_value = 1500.0

        model = self._model(client)
        assert model.eval("mass", context_symbol_id="Demo::sedan") == 1500.0
        client.eval.assert_called_once_with(
            "mass",
            "hash1",
            context_symbol_id="Demo::sedan",
            subject_symbol_id=None,
        )

    def test_eval_passes_a_subject_as_verify_constraint_does(self):
        client = Mock()
        client.eval.return_value = 1200.0

        model = self._model(client)
        assert model.eval("mass", subject="Demo::sedan") == 1200.0
        client.eval.assert_called_once_with(
            "mass",
            "hash1",
            context_symbol_id=None,
            subject_symbol_id="Demo::sedan",
        )

    def test_eval_raises_what_the_connection_raises(self):
        client = Mock()
        client.eval.side_effect = ExecutionError("division by zero")
        model = self._model(client)

        with pytest.raises(ExecutionError):
            model.eval("1/0")

class TestModelRuntimeCalls:
    """Instantiating and executing are on the model too, for the same reason."""

    def _model(self, client):
        pb_response = sysml_pb2.ParseFileResponse(
            model_hash="hash1",
            root=sysml_pb2.SymbolInfo(id="Demo", name="Demo", kind="Package"),
        )
        return Model(pb_response, client)

    def test_instantiate_passes_the_models_hash(self):
        client = Mock()
        client.instantiate.return_value = "instance"

        assert self._model(client).instantiate("Demo::Vehicle") == "instance"
        client.instantiate.assert_called_once_with("Demo::Vehicle", "hash1")

    def test_execute_action_passes_the_models_hash_and_inputs(self):
        client = Mock()
        client.execute_action.return_value = {"result": 15}

        model = self._model(client)
        assert model.execute_action("Demo::add", inputs={"result": 10}) == {"result": 15}
        client.execute_action.assert_called_once_with(
            "Demo::add", "hash1", inputs={"result": 10}, schedule=None
        )

    def test_execute_state_passes_the_models_hash_and_events(self):
        client = Mock()
        client.execute_state.return_value = {"states_visited": ["init"]}

        model = self._model(client)
        assert model.execute_state("Demo::Machine", events=["go"]) == {
            "states_visited": ["init"]
        }
        client.execute_state.assert_called_once_with(
            "Demo::Machine", "hash1", events=["go"], schedule=None
        )

    def test_instantiate_raises_what_the_connection_raises(self):
        client = Mock()
        client.instantiate.side_effect = ExecutionError("not instantiable")
        model = self._model(client)

        with pytest.raises(ExecutionError):
            model.instantiate("Demo::Vehicle")


#: Connection calls whose model-level counterpart is named differently:
#: get_symbol is the raw protobuf lookup behind Model.get and model[name], and
#: apply_edits is the call Model.edit()'s editor makes.
MODEL_LEVEL_ALIASES = {"get_symbol": "get", "apply_edits": "edit"}


def test_every_call_about_a_loaded_model_is_reachable_on_the_model():
    """A Connection call taking a model_hash must have a Model counterpart.

    Without one, the call a script reaches for after load() raises
    AttributeError and the hash has to be carried back to the connection by
    hand — which is what a Model is for.
    """
    hash_taking = {
        name
        for name, member in inspect.getmembers(Connection, inspect.isfunction)
        if not name.startswith("_")
        and "model_hash" in inspect.signature(member).parameters
    }
    # A rename that empties this set would pass the assertion below vacuously.
    assert "instantiate" in hash_taking

    missing = {
        name
        for name in hash_taking
        if not callable(getattr(Model, MODEL_LEVEL_ALIASES.get(name, name), None))
    }
    assert missing == set()

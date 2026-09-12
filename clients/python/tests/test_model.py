"""Tests for Model class."""

import inspect

import pytest
from unittest.mock import Mock
from opensysml.capabilities import CAPABILITY_QUERY, ServerInfo
from opensysml.connection import Connection
from opensysml.proto import sysml_pb2
from opensysml.errors import ExecutionError, SymbolNotFoundError
from opensysml.model import Model
from opensysml.query import QueryElement


def _server_info(*capabilities):
    return ServerInfo(
        version="test", capabilities=frozenset(capabilities), answered=True, origin="test"
    )


def _client(symbols, capabilities=(CAPABILITY_QUERY,), library=()):
    """A connection to a service holding ``symbols``, keyed and answered by id.

    ``get_symbol`` answers by id and ``query`` filters on ``name`` or ``@id``, in
    the declaration order the dict states, as the service does. ``library``
    symbols are resolvable by id but, not being the model's own, are never
    queried. An element's ``owner`` is the symbol whose ``child_ids`` list it,
    else its id's parent segment.
    """
    client = Mock()
    client.server_info.return_value = _server_info(*capabilities)
    client.get_symbol.side_effect = lambda model_hash, symbol_id: (
        symbols.get(symbol_id) or next((s for s in library if s.id == symbol_id), None)
    )

    def owner(info):
        for parent in symbols.values():
            if info.id in parent.child_ids:
                return parent.id
        return info.id.rpartition("::")[0]

    def query(model_hash, payload=None, scope=None, select=None, where=None):
        wanted = None if where is None else where["value"]
        if isinstance(wanted, str):
            wanted = [wanted]
        return [
            QueryElement(
                id=info.id, type=info.kind,
                properties={"name": info.name, "owner": owner(info)},
            )
            for info in symbols.values()
            if wanted is None
            or (where["property"] == "name" and info.name in wanted)
            or (where["property"] == "@id" and info.id in wanted)
        ]

    client.query.side_effect = query
    return client


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
    
    mock_client = _client({
        "MyModel::Vehicle": pb_vehicle,
        "MyModel::Sensor": pb_sensor,
        "MyModel::Vehicle::Engine": pb_engine,
    })
    
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

    mock_client = _client({"Lander::Rhs": pb_rhs})

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
    
    pb_target = sysml_pb2.SymbolInfo(
        id="Root::Target",
        name="Target",
        kind="PartDef",
        metadata={},
        child_ids=["Root::Target::Nested"],
        attributes=[],
    )
    pb_nested = sysml_pb2.SymbolInfo(
        id="Root::Target::Nested", name="Nested", kind="PartDef"
    )

    mock_client = _client({"Root::Target": pb_target, "Root::Target::Nested": pb_nested})
    
    model = Model(pb_response, mock_client)
    
    target = model.find("Target")
    
    assert target is not None
    assert target.name == "Target"

    # Only the match itself was fetched, not the symbols around or below it.
    fetched = [call.args[1] for call in mock_client.get_symbol.call_args_list]
    assert "Root::Target::Nested" not in fetched


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

    mock_client = _client({
        "MyModel::Vehicle": pb_vehicle,
        "MyModel::Vehicle::engine": pb_engine,
    })

    model = Model(pb_response, mock_client)

    assert model.get("MyModel").id == "MyModel"
    assert model.get("MyModel::Vehicle").name == "Vehicle"
    assert model.get("MyModel::Vehicle::engine").name == "engine"

    # A short name is not an FQN, and an unknown FQN is not an error.
    assert model.get("Vehicle") is None
    assert model.get("MyModel::Missing") is None

    # An FQN is one call to the service, however deep it is.
    mock_client.get_symbol.reset_mock()
    assert model.get("MyModel::Vehicle::engine") is not None
    assert mock_client.get_symbol.call_count == 1
    mock_client.query.assert_not_called()


class TestModelLookup:
    """Lookups are answered by the service's index, not by walking the tree."""

    ROOT = sysml_pb2.SymbolInfo(
        id="Demo", name="Demo", kind="package",
        child_ids=["Demo::Vehicle", "Demo::Engine", "Demo::Sub"],
    )
    SYMBOLS = {
        "Demo::Vehicle": sysml_pb2.SymbolInfo(
            id="Demo::Vehicle", name="Vehicle", kind="partDef",
            child_ids=["Demo::Vehicle::engine"],
        ),
        "Demo::Vehicle::engine": sysml_pb2.SymbolInfo(
            id="Demo::Vehicle::engine", name="engine", kind="partUsage",
        ),
        "Demo::Engine": sysml_pb2.SymbolInfo(
            id="Demo::Engine", name="Engine", kind="partDef",
        ),
        "Demo::Sub": sysml_pb2.SymbolInfo(
            id="Demo::Sub", name="Sub", kind="package",
            child_ids=["Demo::Sub::Engine"],
        ),
        "Demo::Sub::Engine": sysml_pb2.SymbolInfo(
            id="Demo::Sub::Engine", name="Engine", kind="partDef",
        ),
    }

    def _model(self, client, diagnostics=()):
        return Model(
            sysml_pb2.ParseFileResponse(
                model_hash="hash", root=self.ROOT, diagnostics=list(diagnostics)
            ),
            client,
        )

    def test_short_name_is_one_query_and_one_fetch(self):
        client = _client(self.SYMBOLS)
        model = self._model(client)

        assert model.find("engine").id == "Demo::Vehicle::engine"

        assert client.query.call_count == 1
        fetched = [call.args[1] for call in client.get_symbol.call_args_list]
        assert fetched == ["Demo::Vehicle::engine"]

    def test_a_missing_short_name_is_one_query_and_one_fetch(self):
        client = _client(self.SYMBOLS)
        model = self._model(client)

        assert model.find("Nope") is None

        assert client.query.call_count == 1
        # A bare name may still be the id of a library package, so that is tried.
        fetched = [call.args[1] for call in client.get_symbol.call_args_list]
        assert fetched == ["Nope"]

    LIBRARY = [sysml_pb2.SymbolInfo(id="Base", name="Base", kind="package")]

    def test_a_library_package_is_found_by_its_id(self):
        model = self._model(_client(self.SYMBOLS, library=self.LIBRARY))

        assert model.find("Base").id == "Base"
        assert model.get("Base").id == "Base"

    def test_a_name_declared_in_the_model_wins_over_a_library_id(self):
        own = sysml_pb2.SymbolInfo(id="Demo::Base", name="Base", kind="partDef")
        client = _client({**self.SYMBOLS, "Demo::Base": own}, library=self.LIBRARY)
        model = self._model(client)

        assert model.find("Base").id == "Demo::Base"
        assert model.get("Base").id == "Base"

    def test_an_erroring_model_is_walked_for_a_name_the_query_cannot_see(self):
        """A feature named by an unresolved redefinition has no effective name to query."""
        unresolved = sysml_pb2.SymbolInfo(
            id="Demo::Vehicle::mass", name="mass", kind="attributeUsage"
        )
        symbols = {**self.SYMBOLS, "Demo::Vehicle::mass": unresolved}
        symbols["Demo::Vehicle"] = sysml_pb2.SymbolInfo(
            id="Demo::Vehicle", name="Vehicle", kind="partDef",
            child_ids=["Demo::Vehicle::engine", "Demo::Vehicle::mass"],
        )
        client = _client(symbols)
        client.query.side_effect = lambda *args, **kwargs: []
        error = sysml_pb2.Diagnostic(severity="error", message="unresolved reference: Base")

        assert self._model(client).find("mass") is None
        assert self._model(client, [error]).find("mass").id == "Demo::Vehicle::mass"

    def test_outermost_of_a_shared_short_name_wins(self):
        symbols = dict(self.SYMBOLS)
        # Declared first in the model, but nested deeper than Demo::Engine.
        symbols = {"Demo::Sub::Engine": symbols.pop("Demo::Sub::Engine"), **symbols}
        model = self._model(_client(symbols))

        assert model.find("Engine").id == "Demo::Engine"

    def test_depth_is_the_owner_chain_not_the_ids_segments(self):
        """A quoted name may contain ``::``; ``'Z::First'`` is one namespace, not two."""
        symbols = {
            "Demo": sysml_pb2.SymbolInfo(
                id="Demo", name="Demo", kind="package",
                child_ids=["Demo::Z::First", "Demo::ASecond"],
            ),
            "Demo::Z::First": sysml_pb2.SymbolInfo(
                id="Demo::Z::First", name="Z::First", kind="package",
                child_ids=["Demo::Z::First::Engine"],
            ),
            "Demo::Z::First::Engine": sysml_pb2.SymbolInfo(
                id="Demo::Z::First::Engine", name="Engine", kind="partDef",
            ),
            "Demo::ASecond": sysml_pb2.SymbolInfo(
                id="Demo::ASecond", name="ASecond", kind="package",
                child_ids=["Demo::ASecond::Engine"],
            ),
            "Demo::ASecond::Engine": sysml_pb2.SymbolInfo(
                id="Demo::ASecond::Engine", name="Engine", kind="partDef",
            ),
        }
        client = _client(symbols)
        model = self._model(client)

        assert model.find("Engine").id == "Demo::Z::First::Engine"
        # Both owners are packages of the root, so one hop up settled the depths.
        assert client.query.call_count == 2

    def test_an_id_the_service_resolves_to_another_symbol_is_not_found(self):
        """The service follows imports; an id lookup names only the symbol carrying that id."""
        library = sysml_pb2.SymbolInfo(
            id="ISQBase::MassValue", name="MassValue", kind="attributeDef"
        )
        client = _client({**self.SYMBOLS, "Demo::MassValue": library})
        model = self._model(client)

        assert model.get("Demo::MassValue") is None
        assert model.find("Demo::MassValue") is None
        assert "Demo::MassValue" not in model

    def test_missing_symbol_names_the_closest_declared_ones(self):
        client = _client(self.SYMBOLS)
        model = self._model(client)

        with pytest.raises(SymbolNotFoundError) as excinfo:
            model["Vehicel"]

        assert "Vehicle" in str(excinfo.value)
        # The declared names came from one query, not from walking the tree.
        fetched = [call.args[1] for call in client.get_symbol.call_args_list]
        assert fetched == ["Vehicel"]

    def test_a_service_without_query_is_walked(self):
        client = _client(self.SYMBOLS, capabilities=())
        model = self._model(client)

        assert model.find("Engine").id == "Demo::Engine"
        assert model.find("Demo::Sub::Engine").id == "Demo::Sub::Engine"
        assert model.find("Nope") is None
        with pytest.raises(SymbolNotFoundError):
            model["Vehicel"]
        client.query.assert_not_called()


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

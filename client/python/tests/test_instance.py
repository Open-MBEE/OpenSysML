"""Tests for Instance class."""
import pytest
from opensysml.errors import FeatureValueError
from opensysml.proto import sysml_pb2
from opensysml.instance import Instance
from opensysml.values import InstanceRef


def scalar_feature(name, **value_kwargs):
    """Build a materialized scalar FeatureValue."""
    return sysml_pb2.FeatureValue(
        feature_name=name,
        value=sysml_pb2.Value(**value_kwargs),
        materialized=True,
    )


def vehicle_graph():
    """Build a Vehicle instance holding a nested Engine, as Instantiate returns."""
    engine = sysml_pb2.Instance(
        id=2,
        type_symbol_id="Demo::Engine",
        feature_values={"power": scalar_feature("power", real_value=300.0)},
    )
    vehicle = sysml_pb2.Instance(
        id=1,
        type_symbol_id="Demo::Vehicle",
        feature_values={
            "mass": scalar_feature("mass", real_value=1500.0),
            "engine": scalar_feature("engine", instance_id=2),
        },
    )
    return vehicle, {1: vehicle, 2: engine}


def test_instance_properties():
    """Test Instance wraps protobuf correctly."""
    pb_inst = sysml_pb2.Instance(
        id=123,
        type_symbol_id="Test::MyPart",
        feature_values={"mass": scalar_feature("mass", int_value=100)},
    )

    inst = Instance(pb_inst)
    assert inst.id == 123
    assert inst.type_symbol_id == "Test::MyPart"
    assert inst.features == {"mass": 100}


def test_instance_get_feature_returns_protobuf():
    """get_feature keeps the raw protobuf reachable."""
    pb_inst = sysml_pb2.Instance(
        id=456,
        type_symbol_id="Test::Vehicle",
        feature_values={"engine": scalar_feature("engine", instance_id=9)},
    )

    inst = Instance(pb_inst)
    slot = inst.get_feature("engine")
    assert isinstance(slot, sysml_pb2.FeatureValue)
    assert slot.feature_name == "engine"
    assert slot.materialized is True
    assert slot.value.WhichOneof("kind") == "instance_id"

    assert inst.get_feature("nonexistent") is None
    assert inst.raw_features["engine"] is slot


def test_slots_convert_scalars_and_sequences():
    """Slot values are converted to plain Python values."""
    pb_inst = sysml_pb2.Instance(
        id=1,
        type_symbol_id="Test::P",
        feature_values={
            "count": scalar_feature("count", int_value=3),
            "mass": scalar_feature("mass", real_value=1500.0),
            "ready": scalar_feature("ready", bool_value=True),
            "name": scalar_feature("name", string_value="v1"),
            "sizes": sysml_pb2.FeatureValue(
                feature_name="sizes",
                values=[sysml_pb2.Value(int_value=1), sysml_pb2.Value(int_value=2)],
                materialized=True,
            ),
        },
    )

    inst = Instance(pb_inst)
    assert inst.features == {
        "count": 3,
        "mass": 1500.0,
        "ready": True,
        "name": "v1",
        "sizes": [1, 2],
    }


def test_attribute_and_item_access():
    """Slots are reachable as attributes and by name."""
    pb_vehicle, graph = vehicle_graph()
    inst = Instance(pb_vehicle, graph)

    assert inst.mass == 1500.0
    assert inst["mass"] == 1500.0
    assert "mass" in inst

    with pytest.raises(AttributeError):
        inst.nonexistent
    with pytest.raises(KeyError):
        inst["nonexistent"]
    assert not hasattr(inst, "nonexistent")


def test_attribute_access_does_not_shadow_members():
    """Real attributes and methods win over slots of the same name."""
    pb_inst = sysml_pb2.Instance(
        id=7,
        type_symbol_id="Test::P",
        feature_values={
            "id": scalar_feature("id", int_value=99),
            "slots": scalar_feature("slots", int_value=5),
            "get_feature": scalar_feature("get_feature", int_value=5),
        },
    )

    inst = Instance(pb_inst)
    assert inst.id == 7
    assert isinstance(inst.features, dict)
    assert callable(inst.get_feature)
    # Shadowed slots stay reachable by name.
    assert inst["id"] == 99


def test_dunder_lookup_raises_attribute_error():
    """Dunder lookups must not be answered from slots, so copy/pickle work."""
    import copy

    pb_inst = sysml_pb2.Instance(id=1, type_symbol_id="Test::P")
    inst = Instance(pb_inst)

    with pytest.raises(AttributeError):
        inst.__deepcopy__
    assert copy.copy(inst) is not None


def test_nested_instance_resolution():
    """An instance-valued slot resolves to an Instance from the graph."""
    pb_vehicle, graph = vehicle_graph()
    inst = Instance(pb_vehicle, graph)

    engine = inst.engine
    assert isinstance(engine, Instance)
    assert engine.id == 2
    assert engine.type_symbol_id == "Demo::Engine"
    assert engine.power == 300.0
    assert inst.features["engine"].id == 2
    # Wrappers are shared within one graph.
    assert inst.engine is engine


def test_unresolvable_instance_id_falls_back_to_a_reference():
    """Without the child in the graph, a reference holding the id is returned."""
    pb_inst = sysml_pb2.Instance(
        id=1,
        type_symbol_id="Test::P",
        feature_values={"engine": scalar_feature("engine", instance_id=42)},
    )

    inst = Instance(pb_inst)
    assert inst.engine == InstanceRef(42)
    assert inst.engine != 42


def test_error_slot_raises_slot_error():
    """A slot the service failed to evaluate raises instead of returning None."""
    pb_inst = sysml_pb2.Instance(
        id=1,
        type_symbol_id="Test::Cyclic",
        feature_values={
            "a": sysml_pb2.FeatureValue(feature_name="a", error="cyclic slot dependency: a"),
        },
    )

    inst = Instance(pb_inst)
    with pytest.raises(FeatureValueError) as excinfo:
        inst.a
    assert excinfo.value.feature_name == "a"
    assert "cyclic" in excinfo.value.message
    # slots exposes the error rather than raising, so the instance stays inspectable.
    assert isinstance(inst.features["a"], FeatureValueError)


def test_unmaterialized_slot_raises_slot_error():
    """An unmaterialized slot is not silently reported as None."""
    pb_inst = sysml_pb2.Instance(
        id=1,
        type_symbol_id="Test::P",
        feature_values={"lazy": sysml_pb2.FeatureValue(feature_name="lazy")},
    )

    inst = Instance(pb_inst)
    with pytest.raises(FeatureValueError, match="not materialized"):
        inst.lazy
    assert inst.get_feature("lazy").materialized is False


def test_unsupported_value_surfaces_as_slot_error():
    """A null carrying a reason is an error, an empty null is a real None."""
    pb_inst = sysml_pb2.Instance(
        id=1,
        type_symbol_id="Test::P",
        feature_values={
            "quantity": scalar_feature("quantity", null="unsupported: quantity value"),
            "empty": scalar_feature("empty", null=""),
        },
    )

    inst = Instance(pb_inst)
    with pytest.raises(FeatureValueError, match="quantity"):
        inst.quantity
    assert inst.empty is None


def test_get_returns_default_for_missing_slot():
    """get() mirrors dict.get for unknown slots."""
    pb_vehicle, graph = vehicle_graph()
    inst = Instance(pb_vehicle, graph)

    assert inst.get("mass") == 1500.0
    assert inst.get("missing") is None
    assert inst.get("missing", 0) == 0


def test_instance_str():
    """Test string representation."""
    pb_inst = sysml_pb2.Instance(id=789, type_symbol_id="Test::Part")
    inst = Instance(pb_inst)

    assert "789" in str(inst)
    assert "Test::Part" in str(inst)

def test_graph_is_shared_not_copied():
    """Nested instances reuse one graph dict instead of copying it per node."""
    pb_vehicle, graph = vehicle_graph()
    inst = Instance(pb_vehicle, graph)

    assert inst._graph is graph
    assert inst.engine._graph is graph


def reach(instance_id, feature_name, feature_symbol_id, path, index=0):
    return sysml_pb2.InstanceReach(
        instance_id=instance_id,
        feature_name=feature_name,
        feature_symbol_id=feature_symbol_id,
        index=index,
        path=path,
    )


def reached_graph():
    """A vehicle whose engine's port is also the interface end's object, each
    instance saying how the traversal reached it, as Instantiate answers."""
    port = sysml_pb2.Instance(
        id=4,
        type_symbol_id="Demo::FuelPort",
        reached_by=reach(3, "fuelIn", "Demo::Engine::fuelIn", "engine.fuelIn"),
    )
    engine = sysml_pb2.Instance(
        id=3,
        type_symbol_id="Demo::Engine",
        feature_values={"fuelIn": scalar_feature("fuelIn", instance_id=4)},
        reached_by=reach(1, "engine", "Demo::Vehicle::engine", "engine"),
    )
    wheel = sysml_pb2.Instance(
        id=5,
        type_symbol_id="Demo::Wheel",
        reached_by=reach(1, "wheels", "Demo::Vehicle::wheels", "wheels[2]", index=2),
    )
    vehicle = sysml_pb2.Instance(
        id=1,
        type_symbol_id="Demo::Vehicle",
        feature_values={
            "engine": scalar_feature("engine", instance_id=3),
            "wheels": sysml_pb2.FeatureValue(
                feature_name="wheels",
                materialized=True,
                values=[sysml_pb2.Value(instance_id=6), sysml_pb2.Value(instance_id=5)],
            ),
        },
    )
    return vehicle, {1: vehicle, 3: engine, 4: port, 5: wheel}


class TestInstanceReach:
    def test_root_has_no_owner(self):
        vehicle, graph = reached_graph()
        root = Instance(vehicle, graph)
        assert root.reached_by is None
        assert root.owner is None
        assert root.feature is None
        assert root.feature_symbol_id is None
        assert root.index is None
        assert root.path == ""

    def test_nested_instance_reports_owner_feature_and_path(self):
        vehicle, graph = reached_graph()
        root = Instance(vehicle, graph)
        port = root.engine.fuelIn
        assert port.owner is root.engine
        assert port.owner.owner is root
        assert port.feature == "fuelIn"
        assert port.feature_symbol_id == "Demo::Engine::fuelIn"
        assert port.index == 0
        assert port.path == "engine.fuelIn"

    def test_multi_valued_feature_reports_its_index_from_one(self):
        vehicle, graph = reached_graph()
        wheel = Instance(vehicle, graph).wheels[1]
        assert wheel.feature == "wheels"
        assert wheel.index == 2
        assert wheel.path == "wheels[2]"
        assert wheel.owner.id == 1

    def test_an_instance_from_an_older_service_has_no_reach(self):
        vehicle, graph = vehicle_graph()
        root = Instance(vehicle, graph)
        assert root.engine.reached_by is None
        assert root.engine.owner is None
        assert root.engine.path == ""

    def test_paths_needs_the_connection(self):
        vehicle, graph = reached_graph()
        with pytest.raises(RuntimeError, match="connection"):
            Instance(vehicle, graph).paths()

    def test_paths_asks_the_connection_for_every_path_to_the_object(self):
        from opensysml.instance import InstancePath, InstancePaths

        class FakeConnection:
            def __init__(self):
                self.asked = []

            def instance_paths(self, instance, model_hash):
                self.asked.append((instance, model_hash))
                response = sysml_pb2.InstancePathsResponse(paths=[
                    sysml_pb2.InstancePath(
                        root=sysml_pb2.InstanceRoot(instance_id=1, symbol_id="Demo::vehicle"),
                        path="engine.fuelIn",
                        steps=[
                            sysml_pb2.InstancePathStep(feature_name="engine", feature_symbol_id="Demo::Vehicle::engine", instance_id=3),
                            sysml_pb2.InstancePathStep(feature_name="fuelIn", feature_symbol_id="Demo::Engine::fuelIn", instance_id=4),
                        ],
                    ),
                    sysml_pb2.InstancePath(
                        root=sysml_pb2.InstanceRoot(instance_id=1, symbol_id="Demo::vehicle"),
                        path="fuelLine.consumer",
                        steps=[
                            sysml_pb2.InstancePathStep(feature_name="fuelLine", feature_symbol_id="Demo::Vehicle::fuelLine", instance_id=2),
                            sysml_pb2.InstancePathStep(feature_name="consumer", feature_symbol_id="Demo::FuelLine::consumer", instance_id=4),
                        ],
                    ),
                ])
                return InstancePaths((InstancePath(path) for path in response.paths), truncated=response.truncated)

        vehicle, graph = reached_graph()
        connection = FakeConnection()
        root = Instance(vehicle, graph, _connection=connection, _model_hash="abc")
        paths = root.engine.fuelIn.paths()
        assert connection.asked == [(4, "abc")]
        assert [str(path) for path in paths] == ["Demo::vehicle.engine.fuelIn", "Demo::vehicle.fuelLine.consumer"]
        assert paths[0].root_id == 1
        assert paths[0].root_symbol_id == "Demo::vehicle"
        assert paths[0].path == "engine.fuelIn"
        assert [step.feature_name for step in paths[1].steps] == ["fuelLine", "consumer"]
        assert paths[1].steps[1].feature_symbol_id == "Demo::FuelLine::consumer"
        assert paths[1].steps[1].instance_id == 4
        assert paths[1].steps[1].index == 0
        assert repr(paths[1].steps[0]) == "PathStep(fuelLine -> #2)"
        assert paths.truncated is False

    def test_a_feature_named_like_a_reach_attribute_is_read_by_name(self):
        route = sysml_pb2.Instance(
            id=7,
            type_symbol_id="Demo::Route",
            feature_values={
                "path": scalar_feature("path", string_value="A-B"),
                "owner": scalar_feature("owner", string_value="fleet"),
            },
            reached_by=reach(1, "route", "Demo::Vehicle::route", "route"),
        )
        vehicle = sysml_pb2.Instance(
            id=1,
            type_symbol_id="Demo::Vehicle",
            feature_values={"route": scalar_feature("route", instance_id=7)},
        )
        inst = Instance(vehicle, graph={1: vehicle, 7: route}).route
        assert inst.path == "route"
        assert inst.owner.id == 1
        assert inst["path"] == "A-B"
        assert inst.get("path") == "A-B"
        assert inst["owner"] == "fleet"
        assert "path" in inst

    def test_paths_say_when_the_service_cut_them_short(self):
        from opensysml.instance import InstancePath, InstancePaths

        pb_path = sysml_pb2.InstancePath(root=sysml_pb2.InstanceRoot(instance_id=1, symbol_id="Demo::vehicle"), path="engine")
        paths = InstancePaths([InstancePath(pb_path)], truncated=True)
        assert isinstance(paths, list)
        assert len(paths) == 1
        assert paths.truncated is True
        assert repr(paths) == "InstancePaths([InstancePath('Demo::vehicle.engine')], truncated=True)"
        assert repr(InstancePaths()) == "InstancePaths([])"

    def test_a_displaced_root_is_named_by_its_id(self):
        from opensysml.instance import InstancePath

        path = InstancePath(sysml_pb2.InstancePath(
            root=sysml_pb2.InstanceRoot(instance_id=7),
            path="wheels[2]",
            steps=[sysml_pb2.InstancePathStep(feature_name="wheels", index=2, instance_id=9)],
        ))
        assert str(path) == "#7.wheels[2]"
        assert repr(path.steps[0]) == "PathStep(wheels[2] -> #9)"
        root = InstancePath(sysml_pb2.InstancePath(root=sysml_pb2.InstanceRoot(instance_id=7)))
        assert str(root) == "#7"
        assert root.steps == []

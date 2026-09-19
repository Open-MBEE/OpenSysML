"""Tests for the feature-value names on Instance."""
import pytest
from opensysml.errors import FeatureValueError
from opensysml.instance import Instance
from opensysml.proto import sysml_pb2


def feature_value(name, **value_kwargs):
    """Build a materialized single-valued FeatureValue."""
    return sysml_pb2.FeatureValue(
        feature_name=name,
        value=sysml_pb2.Value(**value_kwargs),
        materialized=True,
    )


def test_feature_values_are_read_from_the_current_field():
    inst = Instance(sysml_pb2.Instance(
        id=1,
        type_symbol_id="Demo::Vehicle",
        feature_values={"mass": feature_value("mass", real_value=1500.0)},
    ))

    assert inst.features == {"mass": 1500.0}
    assert inst.mass == 1500.0
    assert inst["mass"] == 1500.0
    assert "mass" in inst
    assert inst.get("mass") == 1500.0
    assert inst.get_feature("mass").feature_name == "mass"
    assert set(inst.raw_features) == {"mass"}


def test_the_deprecated_slot_spellings_are_gone():
    """`slots`, `raw_slots`, `get_slot` and SlotError were removed before 0.1.0."""
    import opensysml
    import opensysml.errors as errors
    import opensysml.typed as typed
    import opensysml.values as values

    inst = Instance(sysml_pb2.Instance(
        id=1,
        type_symbol_id="Demo::Vehicle",
        feature_values={"mass": feature_value("mass", real_value=1500.0)},
    ))

    assert "slots" not in sysml_pb2.Instance.DESCRIPTOR.fields_by_name
    assert not hasattr(sysml_pb2, "SlotValue")
    for name in ("raw_slots", "get_slot"):
        assert not hasattr(Instance, name)
    for module in (opensysml, errors):
        assert not hasattr(module, "SlotError")
    for name in ("slot", "optional_slot", "list_slot"):
        assert not hasattr(typed, name)
    assert not hasattr(values, "slot_to_python")
    assert inst.features == {"mass": 1500.0}


def test_a_feature_value_error_reports_rather_than_returns():
    inst = Instance(sysml_pb2.Instance(
        id=1,
        type_symbol_id="Demo::Vehicle",
        feature_values={"mass": sysml_pb2.FeatureValue(
            feature_name="mass", error="cyclic feature value dependency")},
    ))

    with pytest.raises(FeatureValueError, match="feature value 'mass'"):
        inst.mass
    assert isinstance(inst.features["mass"], FeatureValueError)


def test_unknown_feature_says_feature():
    inst = Instance(sysml_pb2.Instance(id=1, type_symbol_id="Demo::Vehicle"))

    with pytest.raises(AttributeError, match="no attribute or feature 'nope'"):
        inst.nope
    with pytest.raises(KeyError):
        inst["nope"]

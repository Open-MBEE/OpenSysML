import inspect
from types import MappingProxyType

import pytest

from opensysml.errors import OpenSysMLError
from opensysml.metamodel import (
    REGISTRY,
    RUNTIME_MEMBERS,
    Element,
    ActionUsage,
    Feature,
    FeatureDirectionKind,
    Many,
    MetamodelError,
    Namespace,
    OccurrenceUsage,
    Opt,
    PartUsage,
    Step,
    Type,
    UnknownJSONKey,
    read_json,
)
from opensysml.metamodel._runtime import _ElementBase


def test_registry_contains_every_table_class():
    assert len(REGISTRY) == 175
    assert REGISTRY["Element"] is Element
    assert issubclass(PartUsage, Feature)
    assert issubclass(Feature, Type)
    assert issubclass(Type, Namespace)
    assert issubclass(Namespace, Element)
    assert issubclass(PartUsage, Element)
    assert issubclass(ActionUsage, Step)
    assert issubclass(ActionUsage, OccurrenceUsage)
    assert issubclass(MetamodelError, OpenSysMLError)


def test_aliases_and_json_key_mappings_are_consistent():
    for metaclass in REGISTRY.values():
        for key, snake in metaclass.JSON_KEYS.items():
            assert inspect.getattr_static(metaclass, key) is inspect.getattr_static(
                metaclass, snake
            )
            assert metaclass.json_key(metaclass.from_json_key(key)) == key


def test_every_descriptor_is_declared_once():
    class_owners = {}
    for metaclass in REGISTRY.values():
        for value in vars(metaclass).values():
            if isinstance(value, (Many, Opt)):
                class_owners.setdefault(id(value), set()).add(metaclass)
    assert all(len(owners) == 1 for owners in class_owners.values())


def test_runtime_members_do_not_collide_with_json_keys():
    assert isinstance(_ElementBase.JSON_KEYS, MappingProxyType)
    with pytest.raises(TypeError):
        _ElementBase.JSON_KEYS["unexpected"] = "unexpected"

    public = {
        name
        for name in set(dir(_ElementBase)) - set(dir(object))
        if not name.startswith("_")
    }
    assert RUNTIME_MEMBERS == public
    for metaclass in REGISTRY.values():
        assert not (RUNTIME_MEMBERS & set(metaclass.JSON_KEYS))
        assert not (RUNTIME_MEMBERS & set(metaclass.JSON_KEYS.values()))


def test_json_key_lookup_rejects_unknown_names():
    with pytest.raises(UnknownJSONKey, match="Element.*@id"):
        Element.from_json_key("@id")
    with pytest.raises(UnknownJSONKey, match="Element.*bogus"):
        Element.json_key("bogus")


def test_descriptor_metadata_and_read_only_properties():
    owned_feature = inspect.getattr_static(Type, "owned_feature")
    assert isinstance(owned_feature, Many)
    assert owned_feature.key == "ownedFeature"
    assert owned_feature.range == "Feature"
    assert owned_feature.many is True
    assert owned_feature.ordered is True
    assert owned_feature.derived is True

    assert Element.declared_name.derived is False
    assert Type.is_abstract.range == "bool"
    assert FeatureDirectionKind.IN == "in"

    element = read_json(
        [{"@id": "e", "@type": "Element", "declaredName": "x"}]
    )["e"]
    with pytest.raises(AttributeError, match="read-only"):
        element.declared_name = "y"

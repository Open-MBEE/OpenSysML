import json
from pathlib import Path

import pytest

from opensysml.metamodel import (
    Feature,
    Namespace,
    NotSupplied,
    PartDefinition,
    PartUsage,
    Type,
    read_json,
)


FIXTURE = Path(__file__).parent / "fixtures" / "metamodel" / "bigger.full.json"


def test_reads_toolkit_full_json_fixture():
    document = json.loads(FIXTURE.read_text())
    graph = read_json(FIXTURE)
    assert len(graph) == 87

    parts = graph.all(PartUsage)
    assert [part.declared_name for part in parts] == ["wheel", "v"]
    assert all(isinstance(part, Feature) for part in parts)

    vehicle = next(item for item in graph.all(PartDefinition) if item.declared_name == "Vehicle")
    assert isinstance(vehicle, Type)
    raw_vehicle = next(
        raw
        for raw in document
        if raw.get("@type") == "PartDefinition" and raw.get("declaredName") == "Vehicle"
    )
    assert vehicle.owned_feature == tuple(
        graph[element["@id"]] for element in raw_vehicle["ownedFeature"]
    )
    assert [feature.declared_name for feature in vehicle.owned_feature] == [
        "mass2", "wheel", "p", "cargo", "a", "c", "s"
    ]
    assert len(vehicle.feature) == 7
    assert all(isinstance(feature, Feature) for feature in vehicle.feature)
    assert all(feature.json_id in graph for feature in vehicle.feature)

    usage = next(part for part in parts if part.declared_name == "v")
    assert usage.type == (vehicle,)
    assert usage.type[0] is graph[vehicle.json_id]
    assert len(graph.roots()) == 1
    assert isinstance(graph.roots()[0], Namespace)

    missing_name = dict(raw_vehicle)
    del missing_name["qualifiedName"]
    incomplete = read_json([missing_name])[missing_name["@id"]]
    with pytest.raises(NotSupplied) as excinfo:
        incomplete.qualified_name
    assert excinfo.value.derived is True

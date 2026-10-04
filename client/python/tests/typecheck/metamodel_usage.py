import builtins

from opensysml.metamodel import (
    Element,
    Feature,
    FeatureDirectionKind,
    PartDefinition,
    PartUsage,
    REGISTRY,
    Type,
    read_json,
)


def check_usage() -> None:
    graph = read_json([{"@id": "part", "@type": "PartUsage"}])
    parts: tuple[PartUsage, ...] = graph.all(PartUsage)
    part = parts[0]
    name: str | None = part.declared_name
    features: tuple[Feature, ...] = part.feature
    camel_alias: tuple[Feature, ...] = part.ownedFeature
    part_types: tuple[Type, ...] = part.type
    abstract: bool | None = part.is_abstract
    direction: FeatureDirectionKind | None = part.direction
    part_class: builtins.type[Element] = REGISTRY["PartUsage"]
    element: Element = graph.all()[0]
    if isinstance(element, PartUsage):
        narrowed_name: str | None = element.declared_name
        assert narrowed_name == name
    assert features == part.feature
    assert camel_alias == part.owned_feature
    assert part_types == part.type
    assert abstract is None
    assert direction is None
    assert part_class is PartUsage

    misspelled: object = part.ownedFeatur  # type: ignore[attr-defined]  # pyright: ignore[reportAttributeAccessIssue, reportUnknownMemberType, reportUnknownVariableType]
    wrong_parts: tuple[PartDefinition, ...] = graph.all(PartUsage)  # type: ignore[arg-type]  # pyright: ignore[reportAssignmentType]
    part.declared_name = 1  # type: ignore[assignment]  # pyright: ignore[reportAttributeAccessIssue]
    wrong_owned: list[Feature] = part.owned_feature  # type: ignore[assignment]  # pyright: ignore[reportAssignmentType]
    assert misspelled
    assert wrong_parts
    assert wrong_owned

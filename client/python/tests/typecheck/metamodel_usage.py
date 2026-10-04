from opensysml.metamodel import (
    Element,
    Feature,
    FeatureDirectionKind,
    PartUsage,
    read_json,
)


def check_usage() -> None:
    graph = read_json([{"@id": "part", "@type": "PartUsage"}])
    parts: tuple[PartUsage, ...] = graph.all(PartUsage)
    part = parts[0]
    name: str | None = part.declared_name
    features: tuple[Feature, ...] = part.feature
    direction: FeatureDirectionKind | None = part.direction
    element: Element = graph.all()[0]
    if isinstance(element, PartUsage):
        narrowed_name: str | None = element.declared_name
        assert narrowed_name == name
    assert features == part.feature
    assert direction is None

    wrong_name: int = part.declared_name  # type: ignore[assignment]  # pyright: ignore[reportAssignmentType]
    wrong_features: str = part.feature  # type: ignore[assignment]  # pyright: ignore[reportAssignmentType]
    assert wrong_name >= 0
    assert bool(wrong_features)

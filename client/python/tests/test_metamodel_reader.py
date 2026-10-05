import json
import subprocess
import time

import grpc
import pytest

from opensysml.connection import Connection
from opensysml.conversion import ExperimentalFeatureWarning
from opensysml.metamodel import (
    Element,
    FeatureDirectionKind,
    LiteralInteger,
    LiteralRational,
    MalformedDocument,
    MalformedValue,
    Namespace,
    NotSupplied,
    PartDefinition,
    PartUsage,
    UnresolvedReference,
    read_json,
)


def element(element_id="e", element_type="PartUsage", **properties):
    return {"@id": element_id, "@type": element_type, **properties}


@pytest.fixture(scope="module")
def real_service():
    from tests.service_gate import free_port, service_binary, skip_or_fail_without_service

    binary = service_binary()
    if binary is None:
        skip_or_fail_without_service("no executable sysml-grpc is available; run: make build-grpc")
    port = free_port()
    process = subprocess.Popen(
        [binary, "-port", str(port)],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    try:
        deadline = time.time() + 10
        while time.time() < deadline:
            with grpc.insecure_channel(f"localhost:{port}") as channel:
                try:
                    grpc.channel_ready_future(channel).result(timeout=0.5)
                    break
                except grpc.FutureTimeoutError:
                    continue
        else:
            pytest.fail("sysml-grpc did not start")
        yield port
    finally:
        process.terminate()
        process.wait(timeout=10)


def test_reads_single_objects_bytes_and_path_inputs(tmp_path):
    document = element(declaredName="engine")
    graph = read_json(document)
    assert graph["e"].declared_name == "engine"
    assert read_json([document]).all(PartUsage)[0].json_id == "e"
    encoded = json.dumps(document).encode()
    assert read_json(encoded)["e"].json_type == "PartUsage"
    assert read_json(bytearray(encoded))["e"].declared_name == "engine"
    assert read_json(memoryview(encoded))["e"].declared_name == "engine"

    path = tmp_path / "elements.json"
    path.write_text(json.dumps([document]))
    assert read_json(str(path))["e"].declared_name == "engine"
    assert read_json(path)["e"].declared_name == "engine"


def test_unwraps_data_version_and_commit_envelopes():
    first = element("first", declaredName="one")
    second = element("second", declaredName="two")
    data_version = [
        {"identity": {"@id": "identity-one"}, "payload": first},
        {"identity": {"@id": "identity-null"}, "payload": None},
    ]
    assert read_json(data_version).all()[0].json_id == "first"
    assert len(read_json(data_version)) == 1

    commit = {"@type": "Commit", "change": [
        {"identity": {"@id": "identity-two"}, "payload": second}
    ]}
    assert read_json(commit)["second"].declared_name == "two"


def test_unwraps_standalone_data_version_envelopes():
    payload = element("p", declaredName="standalone")
    envelope = {"identity": {"@id": "identity"}, "payload": payload}
    assert read_json(envelope)["p"].declared_name == "standalone"
    assert len(read_json({"identity": {"@id": "identity"}, "payload": None})) == 0


def test_normalizes_known_types_and_preserves_written_type():
    for written in (
        "sysml:PartUsage",
        "https://www.omg.org/spec/SysML#PartUsage",
        "https://www.omg.org/spec/SysML/20250201/PartUsage",
    ):
        item = read_json(element(element_type=written))["e"]
        assert isinstance(item, PartUsage)
        assert item.json_type == written


def test_unknown_types_fall_back_or_follow_supertypes():
    fallback = read_json(element(element_type="vendor:SpecialPart"))["e"]
    assert isinstance(fallback, Element)
    assert fallback.json_type == "vendor:SpecialPart"

    specialized = read_json(
        element(element_type="vendor:SpecialPart"),
        supertypes={"vendor:SpecialPart": "Intermediate", "Intermediate": "sysml:PartUsage"},
    )["e"]
    assert isinstance(specialized, PartUsage)
    assert specialized.json_type == "vendor:SpecialPart"

    cyclic = read_json(
        element(element_type="VendorTypeA"),
        supertypes={"VendorTypeA": "VendorTypeB", "VendorTypeB": "VendorTypeA"},
    )
    with pytest.raises(MalformedDocument, match="supertype cycle"):
        cyclic["e"]


def test_reference_resolution_is_lazy_and_cached():
    document = [
        element("usage", partDefinition=[{"@id": "definition"}]),
        element("definition", "PartDefinition", declaredName="Engine"),
    ]
    graph = read_json(document)
    usage = graph["usage"]
    assert graph["usage"] is usage
    assert usage.part_definition == (graph["definition"],)
    assert isinstance(usage.part_definition[0], PartDefinition)

    dangling = read_json(element(partDefinition=[{"@id": "missing"}]))["e"]
    with pytest.raises(UnresolvedReference) as excinfo:
        dangling.part_definition
    assert excinfo.value.id == "missing"
    assert excinfo.value.ref is None
    assert excinfo.value.property == "PartUsage.partDefinition"

    named_reference = read_json(element(partDefinition=[{"@ref": "Engine"}]))["e"]
    with pytest.raises(UnresolvedReference) as excinfo:
        named_reference.part_definition
    assert excinfo.value.id is None
    assert excinfo.value.ref == "Engine"


def test_reference_ranges_require_the_declared_metaclass():
    usage = element("usage", "PartUsage", partDefinition=[{"@id": "literal"}])
    literal = element("literal", "LiteralInteger", value=1)
    graph = read_json([usage, literal])
    with pytest.raises(
        MalformedValue,
        match=r"@id 'literal'.*json_type 'LiteralInteger'.*expected PartDefinition",
    ):
        graph["usage"].part_definition

    usage = element("usage", "PartUsage", partDefinition=[{"@id": "custom"}])
    custom = element("custom", "vendor:SpecialPartDefinition")
    fallback = read_json([usage, custom])
    with pytest.raises(MalformedValue, match="supertypes="):
        fallback["usage"].part_definition

    specialized = read_json(
        [usage, custom],
        supertypes={"vendor:SpecialPartDefinition": "PartDefinition"},
    )
    assert isinstance(specialized["custom"], PartDefinition)
    assert specialized["usage"].part_definition == (specialized["custom"],)


def test_many_reference_ranges_are_checked_per_item():
    definition = element(
        "definition",
        "PartDefinition",
        ownedFeature=[{"@id": "feature"}, {"@id": "namespace"}],
    )
    feature = element("feature", "Feature")
    namespace = element("namespace", "Namespace")
    graph = read_json([definition, feature, namespace])

    with pytest.raises(MalformedValue, match=r"@id 'namespace'.*expected Feature"):
        graph["definition"].owned_feature


def test_range_checks_can_be_disabled_for_derived_references():
    document = [
        element(
            "requirement",
            "RequirementDefinition",
            ownedInterface=[{"@id": "reference"}],
            result={"@id": "requirement"},
        ),
        element("reference", "ReferenceUsage"),
    ]

    checked = read_json(document)
    assert checked.check_ranges is True
    with pytest.raises(MalformedValue):
        checked["requirement"].owned_interface
    with pytest.raises(MalformedValue):
        checked["requirement"].result

    unchecked = read_json(document, check_ranges=False)
    assert unchecked.check_ranges is False
    assert unchecked["requirement"].owned_interface == (unchecked["reference"],)
    assert unchecked["requirement"].result is unchecked["requirement"]
    with pytest.raises(AttributeError):
        unchecked.check_ranges = True


def test_missing_and_null_properties_preserve_cardinality():
    usage = read_json(element())["e"]
    with pytest.raises(NotSupplied) as excinfo:
        usage.declared_name
    assert excinfo.value.property == "PartUsage.declaredName"
    assert excinfo.value.key == "declaredName"
    assert excinfo.value.element_id == "e"
    assert excinfo.value.derived is False

    with pytest.raises(NotSupplied) as excinfo:
        usage.feature
    assert excinfo.value.derived is True
    with pytest.raises(NotSupplied) as excinfo:
        usage.json_value("feature")
    assert excinfo.value.derived is True
    with pytest.raises(NotSupplied):
        hasattr(usage, "feature")

    feature = read_json(element(direction=None, feature=None))["e"]
    assert feature.direction is None
    assert feature.feature == ()
    assert read_json(element(feature=[]))["e"].feature == ()


def test_enum_and_primitive_values_and_malformed_values():
    assert read_json(element(direction="in"))["e"].direction is FeatureDirectionKind.IN
    assert read_json(element("t", "Type", isAbstract=True))["t"].is_abstract is True

    for malformed, attribute in (
        (element(partDefinition="definition"), "part_definition"),
        (element(partDefinition=[None]), "part_definition"),
        (element(partDefinition=["definition"]), "part_definition"),
        (element(direction=["in"]), "direction"),
        (element(direction="sideways"), "direction"),
        (element("t", "Type", isAbstract="true"), "is_abstract"),
    ):
        read = read_json(malformed)[malformed["@id"]]
        with pytest.raises(MalformedValue):
            getattr(read, attribute)

    boolean_integer = read_json(element("i", "LiteralInteger", value=True))["i"]
    assert isinstance(boolean_integer, LiteralInteger)
    with pytest.raises(MalformedValue):
        boolean_integer.value
    integer = read_json(element("valid-integer", "LiteralInteger", value=3))["valid-integer"]
    assert integer.value == 3

    rational = read_json(element("r", "LiteralRational", value=3))["r"]
    assert isinstance(rational, LiteralRational)
    assert rational.value == 3.0


@pytest.mark.parametrize(
    "value", [10**400, float("inf"), float("-inf"), float("nan")]
)
def test_rational_values_must_be_finite(value):
    document = json.dumps(
        element("r", "LiteralRational", value=value), allow_nan=True
    ).encode()
    rational = read_json(document)["r"]

    with pytest.raises(MalformedValue, match="finite"):
        rational.value


@pytest.mark.parametrize(
    "document",
    [
        b"null",
        b"42",
        b'"text"',
        b"{",
        [{"@type": "Element"}],
        [{"@id": "missing-type"}],
        [element(element_id=3)],
        [element(element_type=3)],
        [element(), element()],
    ],
)
def test_malformed_documents_fail_eagerly(document):
    with pytest.raises(MalformedDocument):
        read_json(document)


def test_graph_collection_behavior_and_roots():
    namespace = element("root", "Namespace")
    child = element("child", "Namespace")
    reference = element("owner", "Element", ownedRelationship=[{"@id": "child"}])
    graph = read_json([namespace, child, reference])

    assert len(graph) == 3
    assert [item.json_id for item in graph] == ["root", "child", "owner"]
    assert "root" in graph
    assert "absent" not in graph
    assert graph.get("absent") is None
    assert graph.get("absent", "fallback") == "fallback"
    with pytest.raises(KeyError):
        graph["absent"]
    assert graph.all(Namespace) == (graph["root"], graph["child"])
    assert graph.roots() == (graph["root"],)
    assert graph.roots() is graph.roots()


@pytest.mark.parametrize("key", ["owner", "owningNamespace"])
@pytest.mark.parametrize("owner_id", ["root", "outside"])
def test_roots_excludes_namespaces_with_owner_metadata(key, owner_id):
    root = element("root", "Namespace")
    nested = element("nested", "Namespace", **{key: {"@id": owner_id}})
    graph = read_json([root, nested])

    assert graph.roots() == (graph["root"],)


def test_elements_compare_by_graph_identity_and_id():
    first_graph = read_json(element(declaredName="engine"))
    second_graph = read_json(element(declaredName="engine"))
    first = first_graph["e"]
    same = first_graph["e"]
    other = second_graph["e"]

    assert first == same
    assert hash(first) == hash(same)
    assert first != other
    assert len({first, same, other}) == 2


@pytest.mark.integration
def test_reads_api_json_from_the_open_sysml_service(real_service):
    source = """package ReaderDemo {
    part def Engine {
        attribute power : Real = 300.0;
    }
    part engine : Engine;
}
"""
    with Connection(port=real_service, auto_start=False) as connection:
        model = connection.load_from_content(source)
        with pytest.warns(ExperimentalFeatureWarning):
            exported = model.to_api_json()

    graph = read_json(json.loads(str(exported)))
    usage = next(part for part in graph.all(PartUsage) if part.declared_name == "engine")
    assert usage.declared_name == "engine"
    for name in ("feature", "inherited_feature", "definition"):
        with pytest.raises(NotSupplied) as excinfo:
            getattr(usage, name)
        assert excinfo.value.derived is True

    definition = next(
        item for item in graph.all(PartDefinition) if item.declared_name == "Engine"
    )
    assert usage.type == (definition,)
    assert definition.owned_feature
    assert all(isinstance(feature, Element) for feature in definition.owned_feature)
    assert all(feature in graph.all() for feature in definition.owned_feature)

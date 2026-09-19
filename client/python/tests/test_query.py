"""Tests for querying a model the way the SysML v2 API & Services standard does.

Two layers. Against a fake service, the client's own behavior: that the
standard's JSON translates to the RPC's protobuf unchanged, that a payload the
standard does not describe is refused before anything is sent, and the capability
gate. Against the real ``sysml-grpc`` binary, the answers themselves — including
the cookbook payload verbatim — which is the part a fake cannot tell you.
"""

import os
import subprocess
import time
from concurrent import futures

import grpc
import pytest

from opensysml.capabilities import CAPABILITY_QUERY, MissingCapabilityError
from opensysml.connection import Connection
from opensysml.errors import InvalidRequestError, ModelNotFoundError
from opensysml.query import QueryElement, QueryError, build_query
from opensysml.proto import sysml_pb2, sysml_pb2_grpc

MODEL = """package Demo {
    abstract part def Vehicle {
        attribute mass;
    }
    part def Wheel;
    part vehicle : Vehicle {
        part wheels : Wheel[4];
    }
}
"""

# The payload the SysML v2 API Cookbook notebooks send, verbatim.
COOKBOOK_QUERY = {
    "@type": "Query",
    "where": {
        "@type": "PrimitiveConstraint",
        "operator": "=",
        "property": "@type",
        "value": ["PartUsage"],
    },
}

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
GRPC_BINARIES = (
    os.path.join(REPO_ROOT, "bin", "sysml-grpc"),
    os.path.join(os.path.expanduser("~"), ".opensysml", "bin", "sysml-grpc"),
)


class FakeService(sysml_pb2_grpc.SysMLServiceServicer):
    """A sysml-grpc whose Query records the request and answers as told."""

    def __init__(self, capabilities=(CAPABILITY_QUERY,), elements=()):
        self._capabilities = list(capabilities)
        self._elements = list(elements)
        self.requests = []

    def GetServerInfo(self, request, context):
        return sysml_pb2.ServerInfoResponse(
            version="fake", capabilities=self._capabilities
        )

    def GetDiagnostics(self, request, context):
        context.abort(grpc.StatusCode.NOT_FOUND, "model not found")

    def ParseFile(self, request, context):
        root = sysml_pb2.SymbolInfo(id="Demo", name="Demo", kind="Package")
        return sysml_pb2.ParseFileResponse(model_hash="fake-hash", root=root)

    def Query(self, request, context):
        self.requests.append(request)
        return sysml_pb2.QueryResponse(elements=self._elements)


@pytest.fixture
def fake_service():
    """Start a FakeService on an ephemeral port; yields a factory."""
    servers = []

    def start(**kwargs):
        service = FakeService(**kwargs)
        server = grpc.server(futures.ThreadPoolExecutor(max_workers=2))
        sysml_pb2_grpc.add_SysMLServiceServicer_to_server(service, server)
        port = server.add_insecure_port("localhost:0")
        server.start()
        servers.append(server)
        return port, service

    yield start
    for server in servers:
        server.stop(None)


def test_cookbook_payload_translates_verbatim():
    """The standard's JSON becomes the RPC's protobuf, field for field."""
    query = build_query(COOKBOOK_QUERY)
    assert list(query.scope) == []
    assert list(query.select) == []
    primitive = query.where.primitive
    assert query.where.WhichOneof("constraint") == "primitive"
    assert primitive.property == "@type"
    assert primitive.operator == sysml_pb2.PRIMITIVE_OPERATOR_EQUAL
    assert list(primitive.value) == ["PartUsage"]
    assert primitive.inverse is False


def test_keyword_form_builds_the_same_query():
    """Keywords are the payload's fields, so both spell the same query."""
    assert build_query(**{k[1:] if k.startswith("@") else k: v
                          for k, v in COOKBOOK_QUERY.items() if k != "@type"}) == \
        build_query(COOKBOOK_QUERY)


def test_scope_takes_names_and_element_references():
    """A scope entry is a qualified name or the standard's ``{"@id": ...}``."""
    query = build_query({
        "scope": ["Demo::vehicle", {"@id": "Demo::Wheel"}],
        "select": ["name", "@type"],
    })
    assert list(query.scope) == ["Demo::vehicle", "Demo::Wheel"]
    assert list(query.select) == ["name", "@type"]


def test_composite_constraints_nest():
    """and/or nest, including a composite inside a composite."""
    query = build_query(where={
        "@type": "CompositeConstraint",
        "operator": "and",
        "constraint": [
            {"@type": "PrimitiveConstraint", "operator": "=",
             "property": "@type", "value": ["PartUsage"]},
            {"@type": "CompositeConstraint", "operator": "or", "constraint": [
                {"operator": "=", "property": "name", "value": "wheels"},
                {"operator": "=", "property": "name", "value": "spare"},
            ]},
        ],
    })
    composite = query.where.composite
    assert composite.operator == sysml_pb2.COMPOSITE_OPERATOR_AND
    assert len(composite.constraint) == 2
    nested = composite.constraint[1].composite
    assert nested.operator == sysml_pb2.COMPOSITE_OPERATOR_OR
    assert [list(c.primitive.value) for c in nested.constraint] == [["wheels"], ["spare"]]


def test_inverse_and_ordered_operators_translate():
    """``inverse`` and the ordered operators are carried, not dropped."""
    query = build_query(where={
        "operator": ">", "property": "multiplicityLower", "value": 1, "inverse": True,
    })
    primitive = query.where.primitive
    assert primitive.operator == sysml_pb2.PRIMITIVE_OPERATOR_GREATER
    assert primitive.inverse is True
    # A number is compared as its text, since the service parses what it compares.
    assert list(primitive.value) == ["1"]


def test_a_query_without_a_constraint_selects_its_scope():
    """An absent ``where`` filters nothing, and is absent on the wire."""
    query = build_query({"scope": ["Demo"]})
    assert not query.HasField("where")


@pytest.mark.parametrize("payload, message", [
    ({"@type": "Element"}, "expected a 'Query' payload"),
    ({"filter": "x"}, "a query has no filter"),
    ("Demo", "a query is an object"),
    ({"where": {"@type": "Filter", "operator": "="}}, "unknown constraint type"),
    ({"where": {"operator": "~", "property": "name", "value": "x"}},
     "unknown primitive operator"),
    ({"where": {"operator": "=", "value": "x"}}, "names one property"),
    ({"where": {"operator": "=", "property": "name", "value": "x", "not": True}},
     "has no not"),
    ({"where": {"@type": "CompositeConstraint", "operator": "and", "constraint": []}},
     "non-empty list"),
    ({"where": {"@type": "CompositeConstraint", "operator": "xor",
                "constraint": [{"operator": "=", "property": "name", "value": "x"}]}},
     "unknown composite operator"),
    ({"scope": [42]}, "a scope entry is"),
    ({"select": [42]}, "a selected property is a name"),
    ({"where": {"operator": "=", "property": "name", "value": [object()]}},
     "cannot compare against"),
])
def test_a_payload_the_standard_does_not_describe_is_refused(payload, message):
    """A malformed query is a caller error, named, rather than a request sent."""
    with pytest.raises(QueryError, match=message):
        build_query(payload)


def test_payload_and_keywords_are_not_combined():
    """Given both, the client refuses rather than silently preferring one."""
    with pytest.raises(QueryError, match="not both"):
        build_query(COOKBOOK_QUERY, scope=["Demo"])


def test_query_requires_the_capability(fake_service):
    """A service that cannot query is named, not asked."""
    port, service = fake_service(capabilities=())
    with Connection(port=port, auto_start=False) as conn:
        model = conn.load_from_content(MODEL)
        with pytest.raises(MissingCapabilityError) as excinfo:
            model.query(COOKBOOK_QUERY)
    assert excinfo.value.capability == CAPABILITY_QUERY
    assert service.requests == [], "the request was sent to a service that cannot serve it"


def test_model_query_sends_its_own_hash(fake_service):
    """A model queries itself: the request names the model it was loaded as."""
    port, service = fake_service()
    with Connection(port=port, auto_start=False) as conn:
        model = conn.load_from_content(MODEL)
        model.query(COOKBOOK_QUERY)
    (request,) = service.requests
    assert request.model_hash == model.hash
    assert request.query == build_query(COOKBOOK_QUERY)


def test_reported_elements_become_records(fake_service):
    """A reported element is a record of its identity, type and properties."""
    port, _ = fake_service(elements=[
        sysml_pb2.QueryResultElement(
            id="Demo::vehicle", type="PartUsage",
            properties={"name": "vehicle", "owner": "Demo"},
        ),
        sysml_pb2.QueryResultElement(id="Demo::spare", type="PartUsage"),
    ])
    with Connection(port=port, auto_start=False) as conn:
        elements = conn.load_from_content(MODEL).query(COOKBOOK_QUERY)

    assert elements[0] == QueryElement(
        id="Demo::vehicle", type="PartUsage",
        properties={"name": "vehicle", "owner": "Demo"},
    )
    assert elements[0].get("name") == "vehicle"
    assert elements[0].as_dict() == {
        "@id": "Demo::vehicle", "@type": "PartUsage",
        "name": "vehicle", "owner": "Demo",
    }
    assert str(elements[0]) == "Demo::vehicle (PartUsage)"
    # A property the element does not have is absent, not empty.
    assert elements[1].get("name") is None
    assert elements[1].get("name", "unnamed") == "unnamed"


@pytest.fixture(scope="module")
def real_service():
    """Run the built sysml-grpc on an ephemeral port, or skip."""
    binary = next((b for b in GRPC_BINARIES if os.access(b, os.X_OK)), None)
    if binary is None:
        pytest.skip(f"no executable sysml-grpc in {GRPC_BINARIES}; run: make build-grpc")

    port = 51153
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


@pytest.mark.integration
class TestQueryAgainstRealService:
    """The answers themselves, from the real evaluator."""

    def test_the_cookbook_payload_selects_the_part_usages(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            elements = conn.load_from_content(MODEL).query(COOKBOOK_QUERY)
        assert [e.id for e in elements] == ["Demo::vehicle", "Demo::vehicle::wheels"]
        assert {e.type for e in elements} == {"PartUsage"}

    def test_scope_and_select_narrow_the_answer(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            elements = conn.load_from_content(MODEL).query(
                scope=["Demo::vehicle"], select=["name", "qualifiedName"],
                where={"operator": "=", "property": "@type", "value": ["PartUsage"]},
            )
        assert [e.properties for e in elements] == [
            {"name": "vehicle", "qualifiedName": "Demo::vehicle"},
            {"name": "wheels", "qualifiedName": "Demo::vehicle::wheels"},
        ]

    def test_composite_and_inverse_evaluate(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            elements = conn.load_from_content(MODEL).query(where={
                "@type": "CompositeConstraint",
                "operator": "and",
                "constraint": [
                    {"operator": "=", "property": "@type", "value": ["PartUsage"]},
                    {"operator": "=", "property": "name", "value": "vehicle",
                     "inverse": True},
                ],
            })
        assert [e.id for e in elements] == ["Demo::vehicle::wheels"]

    def test_an_unknown_property_is_an_error_not_an_empty_answer(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(MODEL)
            with pytest.raises(InvalidRequestError) as excinfo:
                model.query(
                    where={"operator": "=", "property": "colour", "value": "red"},
                )
        assert "unknown query property" in str(excinfo.value)

    def test_an_evicted_model_raises_this_library_s_error(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            with pytest.raises(ModelNotFoundError):
                conn.query("deadbeef", COOKBOOK_QUERY)

    def test_an_unnamed_element_is_not_answered(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            elements = conn.load_from_content(
                "package Anon { doc /* unnamed */ part def Rig; part : Rig; }"
            ).query()
        ids = [e.id for e in elements]
        assert ids == ["Anon", "Anon::Rig"]

    def test_a_query_matching_nothing_answers_with_nothing(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            assert conn.load_from_content(MODEL).query(
                where={"operator": "=", "property": "name", "value": "Nonexistent"},
            ) == []

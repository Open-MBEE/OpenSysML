"""Tests for named document queries and document rendering.

Two layers, as :mod:`tests.test_query` has. Against a fake service, the
client's own behavior: binding translation, decoding, and the capability
gates. Against the real ``sysml-grpc`` binary, the answers themselves — the
typed rows and the rendered Markdown.
"""

import os
import subprocess
import time
from concurrent import futures

import grpc
import pytest

from opensysml.capabilities import (
    CAPABILITY_DOCUMENT_QUERY,
    CAPABILITY_RENDER_DOCUMENT,
    CAPABILITY_RENDER_DOCUMENT_HTML,
    MissingCapabilityError,
)
from opensysml.connection import Connection
from opensysml.document import (
    INFINITY,
    DocumentQueryError,
    DocumentEvent,
    DocumentQueryResult,
    DocumentRow,
    DocumentState,
    DocumentVerdict,
    ElementRef,
    ObjectRef,
    build_bindings,
)
from opensysml.errors import (
    InvalidRequestError,
    ModelNotFoundError,
    SymbolNotFoundError,
    UnsupportedValueError,
)
from opensysml.proto import sysml_pb2, sysml_pb2_grpc
from opensysml.values import Quantity, Unit, UnitFactor

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
GRPC_BINARIES = (
    os.path.join(REPO_ROOT, "bin", "sysml-grpc"),
    os.path.join(os.path.expanduser("~"), ".opensysml", "bin", "sysml-grpc"),
)

#: The document pipeline's own telescope fixture and its golden Markdown, so
#: the client sees exactly what the renderer's tests lock in.
FIXTURE = os.path.join(
    REPO_ROOT, "internal", "doc", "docrender", "testdata", "telescope_report.sysml"
)
GOLDEN = os.path.join(
    REPO_ROOT, "internal", "doc", "docrender", "testdata", "telescope_report.golden.md"
)
HTML_GOLDEN = os.path.join(
    REPO_ROOT, "internal", "doc", "docrender", "testdata", "telescope_report.golden.html"
)
#: The renderer's verdict fixture: assertions on a car and queries over them.
VERDICT_FIXTURE = os.path.join(
    REPO_ROOT, "internal", "doc", "docrender", "testdata", "verdict_report.sysml"
)
#: The service's object fixture: a car with wheels, a spare wheel, and
#: queries over the objects the service holds — bound, enumerated, checked.
OBJECT_FIXTURE = os.path.join(
    REPO_ROOT, "tests", "grpc", "testdata", "conformance",
    "document_query_object_by_path.sysml",
)
#: The service's state fixture: a lamp exhibiting a state machine with an
#: orthogonal region, and queries over its states and its trace.
STATE_FIXTURE = os.path.join(
    REPO_ROOT, "tests", "grpc", "testdata", "conformance",
    "document_query_states.sysml",
)

CAPABILITIES = (
    CAPABILITY_DOCUMENT_QUERY, CAPABILITY_RENDER_DOCUMENT, CAPABILITY_RENDER_DOCUMENT_HTML,
)


class FakeService(sysml_pb2_grpc.SysMLServiceServicer):
    """A sysml-grpc whose document RPCs record requests and answer as told."""

    def __init__(self, capabilities=CAPABILITIES, response=None, markdown="", html=""):
        self._capabilities = list(capabilities)
        self._response = response or sysml_pb2.RunDocumentQueryResponse()
        self._markdown = markdown
        self._html = html
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

    def RunDocumentQuery(self, request, context):
        self.requests.append(request)
        return self._response

    def RenderDocument(self, request, context):
        self.requests.append(request)
        if request.form == "html":
            return sysml_pb2.RenderDocumentResponse(html=self._html)
        return sysml_pb2.RenderDocumentResponse(markdown=self._markdown)


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


def test_bindings_translate_to_typed_values():
    """Each Python kind becomes its own wire kind, bool before int."""
    bindings = build_bindings({
        "root": ElementRef("Observatory::telescope"),
        "name": "mount",
        "count": 3,
        "mass": 1.5,
        "heavy": True,
        "several": ["a", "b"],
    })
    by_parameter = {b.parameter: b for b in bindings}
    assert by_parameter["root"].values[0].element_id == "Observatory::telescope"
    assert by_parameter["name"].values[0].string_value == "mount"
    assert by_parameter["count"].values[0].int_value == 3
    assert by_parameter["mass"].values[0].real_value == 1.5
    assert by_parameter["heavy"].values[0].bool_value is True
    assert [v.string_value for v in by_parameter["several"].values] == ["a", "b"]


def test_a_quantity_binding_keeps_its_magnitude_and_unit():
    """A Quantity binds as the wire's Quantity, unit and reduction intact."""
    kg = Unit(text="kg", factors=(UnitFactor("SI::kg", 1),), reduction_given=True)
    bindings = build_bindings({"limit": Quantity(2290000, kg)})
    (value,) = bindings[0].values
    assert value.WhichOneof("kind") == "quantity"
    assert value.quantity.int_magnitude == 2290000
    assert value.quantity.unit == "kg"
    assert value.quantity.unit_term.factors[0].unit_id == "SI::kg"


def test_a_binding_the_wire_cannot_carry_is_refused():
    """An untranslatable value is a caller error, named before anything is sent."""
    with pytest.raises(DocumentQueryError, match="'root'"):
        build_bindings({"root": object()})


def test_a_verdict_binding_is_refused():
    """A verdict is answered by queries; binding one is a caller error."""
    verdict = DocumentVerdict(
        assertion=ElementRef("Garage::Car::massOk"), kind="constraint",
        text="assert constraint massOk", path="Garage::car", status="holds",
    )
    with pytest.raises(DocumentQueryError, match="'root'.*answered by queries"):
        build_bindings({"root": verdict})


def test_a_state_or_event_binding_is_refused():
    """States and events are answered by queries; binding one is a caller error."""
    lamp = ObjectRef(id=1, path="Lamps::lamp")
    state = DocumentState(object=lamp, machine="lp", name="off", path="off")
    event = DocumentEvent(kind="entry", time=0, text="enter: off")
    with pytest.raises(DocumentQueryError, match="'root'.*a state row is answered by queries"):
        build_bindings({"root": state})
    with pytest.raises(DocumentQueryError, match="'root'.*an event row is answered by queries"):
        build_bindings({"root": event})


def test_an_object_binds_by_id_by_path_or_both():
    """An ObjectRef binds the object the service holds, by id, path or both."""
    bindings = build_bindings({
        "by_id": ObjectRef(id=2),
        "by_path": ObjectRef(path="car.wheels[2]"),
        "both": ObjectRef(id=3, path="Garage::car.wheels[2]"),
    })
    by_parameter = {b.parameter: b for b in bindings}
    (by_id,) = by_parameter["by_id"].values
    assert by_id.WhichOneof("kind") == "object"
    assert (by_id.object.instance_id, by_id.object.path) == (2, "")
    (by_path,) = by_parameter["by_path"].values
    assert (by_path.object.instance_id, by_path.object.path) == (0, "car.wheels[2]")
    (both,) = by_parameter["both"].values
    assert (both.object.instance_id, both.object.path) == (3, "Garage::car.wheels[2]")
    assert not both.object.HasField("element")


def test_an_object_naming_nothing_is_refused():
    """An ObjectRef with neither id nor path is a caller error, named early."""
    unnamed = {"root": ObjectRef()}
    with pytest.raises(DocumentQueryError, match="'root'.*neither was given"):
        build_bindings(unnamed)


def test_an_oversized_int_binding_is_refused():
    """An int outside int64 is a caller error, not a protobuf ValueError."""
    with pytest.raises(DocumentQueryError, match="signed 64-bit"):
        build_bindings({"threshold": 1 << 63})
    with pytest.raises(DocumentQueryError, match="signed 64-bit"):
        build_bindings({"threshold": -(1 << 63) - 1})


def test_a_quantity_the_wire_cannot_carry_is_refused():
    """An unreduced unit or an oversized magnitude is a DocumentQueryError, not
    the Quantity's own UnsupportedValueError or a protobuf ValueError."""
    kg = Unit(text="kg", factors=(UnitFactor("SI::kg", 1),), reduction_given=True)
    unreduced = {"limit": Quantity(1, Unit(text="furlong"))}
    with pytest.raises(DocumentQueryError, match="'limit'.*no reduction") as caught:
        build_bindings(unreduced)
    assert isinstance(caught.value.__cause__, UnsupportedValueError)
    oversized = {"limit": Quantity(1 << 63, kg)}
    with pytest.raises(DocumentQueryError, match="'limit'.*signed 64-bit"):
        build_bindings(oversized)
    boolean = {"limit": Quantity(True, kg)}
    with pytest.raises(DocumentQueryError, match="'limit'.*neither an Integer nor a Real"):
        build_bindings(boolean)


def test_no_bindings_is_an_empty_request():
    assert build_bindings(None) == []
    assert build_bindings({}) == []


def test_document_query_requires_the_capability(fake_service):
    """A service that cannot run document queries is named, not asked."""
    port, service = fake_service(capabilities=())
    with Connection(port=port, auto_start=False) as conn:
        model = conn.load_from_content("package Demo;")
        with pytest.raises(MissingCapabilityError) as excinfo:
            model.run_document_query("Demo::Q")
    assert excinfo.value.capability == CAPABILITY_DOCUMENT_QUERY
    assert service.requests == []


def test_render_document_requires_the_capability(fake_service):
    """A service that cannot render documents is named, not asked."""
    port, service = fake_service(capabilities=())
    with Connection(port=port, auto_start=False) as conn:
        model = conn.load_from_content("package Demo;")
        with pytest.raises(MissingCapabilityError) as excinfo:
            model.render_document("Demo::Doc")
    assert excinfo.value.capability == CAPABILITY_RENDER_DOCUMENT
    assert service.requests == []


def test_render_document_html_requires_its_own_capability(fake_service):
    """A service that renders Markdown only is not asked for HTML; Markdown still works."""
    port, service = fake_service(
        capabilities=(CAPABILITY_DOCUMENT_QUERY, CAPABILITY_RENDER_DOCUMENT), markdown="# R\n"
    )
    with Connection(port=port, auto_start=False) as conn:
        model = conn.load_from_content("package Demo;")
        with pytest.raises(MissingCapabilityError) as excinfo:
            model.render_document("Demo::Doc", form="html")
        assert excinfo.value.capability == CAPABILITY_RENDER_DOCUMENT_HTML
        assert service.requests == []
        assert model.render_document("Demo::Doc") == "# R\n"


def test_render_document_refuses_a_form_it_does_not_know(fake_service):
    port, service = fake_service()
    with Connection(port=port, auto_start=False) as conn:
        model = conn.load_from_content("package Demo;")
        with pytest.raises(ValueError, match="markdown"):
            model.render_document("Demo::Doc", form="pdf")
    assert service.requests == []


def test_the_request_names_the_model_query_and_bindings(fake_service):
    """The request carries the model's own hash, the query, and the bindings."""
    port, service = fake_service()
    with Connection(port=port, auto_start=False) as conn:
        model = conn.load_from_content("package Demo;")
        model.run_document_query(
            "Demo::Q", bindings={"root": ElementRef("Demo::part")}
        )
    (request,) = service.requests
    assert request.model_hash == model.hash
    assert request.query_id == "Demo::Q"
    assert request.bindings[0].parameter == "root"
    assert request.bindings[0].values[0].element_id == "Demo::part"


def test_answered_rows_decode_to_typed_records(fake_service):
    """Every wire kind decodes to its Python value, elements with their type."""
    response = sysml_pb2.RunDocumentQueryResponse(
        columns=[
            sysml_pb2.DocumentQueryColumn(name="name"),
            sysml_pb2.DocumentQueryColumn(name="bound"),
        ],
        rows=[sysml_pb2.DocumentQueryRow(
            element=sysml_pb2.DocumentValue(
                element_id="Demo::part", element_type="PartUsage"
            ),
            cells=[
                sysml_pb2.DocumentQueryCell(values=[
                    sysml_pb2.DocumentValue(string_value="part"),
                ]),
                sysml_pb2.DocumentQueryCell(values=[
                    sysml_pb2.DocumentValue(int_value=0),
                    sysml_pb2.DocumentValue(infinity=True),
                    sysml_pb2.DocumentValue(real_value=1.5),
                    sysml_pb2.DocumentValue(bool_value=True),
                    sysml_pb2.DocumentValue(quantity=sysml_pb2.Quantity(
                        int_magnitude=2290000,
                        unit="kg",
                        unit_term=sysml_pb2.UnitTerm(
                            scale_num=1.0,
                            scale_den=1.0,
                            factors=[sysml_pb2.UnitFactor(unit_id="SI::kg", exponent=1)],
                        ),
                    )),
                ]),
            ],
        )],
    )
    port, _ = fake_service(response=response)
    with Connection(port=port, auto_start=False) as conn:
        result = conn.load_from_content("package Demo;").run_document_query("Demo::Q")

    assert isinstance(result, DocumentQueryResult)
    assert result.columns == ("name", "bound")
    (row,) = result
    assert isinstance(row, DocumentRow)
    assert row.element == ElementRef(id="Demo::part", type="PartUsage")
    assert str(row.element) == "Demo::part (PartUsage)"
    assert row[0] == ("part",)
    kg = Unit(text="kg", factors=(UnitFactor("SI::kg", 1),), reduction_given=True)
    assert row[1] == (0, INFINITY, 1.5, True, Quantity(2290000, kg))
    assert str(row[1][4]) == "2290000 [kg]"
    assert len(result) == 1


def test_a_verdict_row_decodes_to_its_assertion_and_verdict(fake_service):
    """A row a Verdicts query answered stands for the assertion it checked
    and carries the verdict; a verdict-valued cell decodes the same way."""
    wire = sysml_pb2.DocumentVerdict(
        assertion=sysml_pb2.DocumentValue(
            element_id="Garage::Engine::powerLow", element_type="ConstraintUsage"
        ),
        kind="constraint",
        text="assert constraint powerLow",
        path="Garage::car.engine",
        verdict="violated",
        condition="power < 200.0",
        reason="power < 200.0 is false",
        verification=["pass"],
    )
    response = sysml_pb2.RunDocumentQueryResponse(
        columns=[sysml_pb2.DocumentQueryColumn(name="self")],
        rows=[sysml_pb2.DocumentQueryRow(
            element=sysml_pb2.DocumentValue(verdict=wire),
            cells=[sysml_pb2.DocumentQueryCell(values=[
                sysml_pb2.DocumentValue(verdict=wire),
            ])],
        )],
    )
    port, _ = fake_service(response=response)
    with Connection(port=port, auto_start=False) as conn:
        result = conn.load_from_content("package Demo;").run_document_query("Demo::Q")

    (row,) = result
    expected = DocumentVerdict(
        assertion=ElementRef(id="Garage::Engine::powerLow", type="ConstraintUsage"),
        kind="constraint",
        text="assert constraint powerLow",
        path="Garage::car.engine",
        status="violated",
        condition="power < 200.0",
        reason="power < 200.0 is false",
        verification=("pass",),
    )
    assert row.verdict == expected
    assert row.element == expected.assertion
    assert row[0] == (expected,)
    assert str(row.verdict) == "assert constraint powerLow on Garage::car.engine: violated"


def test_an_object_row_decodes_to_the_object_and_its_usage(fake_service):
    """A row over held objects stands for the usage the object is held under
    and carries the object; an object-valued cell decodes the same way."""
    def wire(instance_id, path, usage):
        return sysml_pb2.DocumentValue(object=sysml_pb2.DocumentObject(
            instance_id=instance_id,
            path=path,
            element=sysml_pb2.DocumentValue(element_id=usage, element_type="PartUsage"),
        ))

    response = sysml_pb2.RunDocumentQueryResponse(
        columns=[sysml_pb2.DocumentQueryColumn(name="wheels")],
        rows=[sysml_pb2.DocumentQueryRow(
            element=wire(1, "Garage::car", "Garage::car"),
            cells=[sysml_pb2.DocumentQueryCell(values=[
                wire(2, "Garage::car.wheels[1]", "Garage::Car::wheels"),
                wire(3, "Garage::car.wheels[2]", "Garage::Car::wheels"),
            ])],
        )],
    )
    port, _ = fake_service(response=response)
    with Connection(port=port, auto_start=False) as conn:
        result = conn.load_from_content("package Demo;").run_document_query("Demo::Q")

    (row,) = result
    car = ObjectRef(id=1, path="Garage::car", element=ElementRef("Garage::car", "PartUsage"))
    assert row.object == car
    assert row.element == car.element
    assert row.verdict is None
    assert row[0] == (
        ObjectRef(2, "Garage::car.wheels[1]", ElementRef("Garage::Car::wheels", "PartUsage")),
        ObjectRef(3, "Garage::car.wheels[2]", ElementRef("Garage::Car::wheels", "PartUsage")),
    )
    assert [str(wheel) for wheel in row[0]] == ["Garage::car.wheels[1]", "Garage::car.wheels[2]"]
    assert str(ObjectRef(id=2)) == "#2"


def _lamp_wire(instance_id=1, path="Lamps::lamp"):
    return sysml_pb2.DocumentObject(
        instance_id=instance_id,
        path=path,
        element=sysml_pb2.DocumentValue(element_id="Lamps::lamp", element_type="PartUsage"),
    )


def test_a_state_row_decodes_to_the_object_and_its_state(fake_service):
    """A row a States query answered stands for the object's usage and
    carries the object and the state; a state-valued cell decodes the same way."""
    wire = sysml_pb2.DocumentState(
        object=_lamp_wire(),
        machine="lp",
        name="run",
        state_path="on.run",
        state=sysml_pb2.DocumentValue(
            element_id="Lamps::LampMachine::on::light::run", element_type="StateUsage"
        ),
        region="light",
        enclosing=["on"],
    )
    response = sysml_pb2.RunDocumentQueryResponse(
        columns=[sysml_pb2.DocumentQueryColumn(name="self")],
        rows=[sysml_pb2.DocumentQueryRow(
            element=sysml_pb2.DocumentValue(state=wire),
            cells=[sysml_pb2.DocumentQueryCell(values=[sysml_pb2.DocumentValue(state=wire)])],
        )],
    )
    port, _ = fake_service(response=response)
    with Connection(port=port, auto_start=False) as conn:
        result = conn.load_from_content("package Demo;").run_document_query("Demo::Q")

    (row,) = result
    lamp = ObjectRef(id=1, path="Lamps::lamp", element=ElementRef("Lamps::lamp", "PartUsage"))
    expected = DocumentState(
        object=lamp,
        machine="lp",
        name="run",
        path="on.run",
        state=ElementRef("Lamps::LampMachine::on::light::run", "StateUsage"),
        region="light",
        enclosing=("on",),
    )
    assert row.state == expected
    assert row.object == lamp
    assert row.element == lamp.element
    assert row.verdict is None
    assert row.event is None
    assert row[0] == (expected,)
    assert str(row.state) == "Lamps::lamp.lp in on.run"


def test_an_event_row_decodes_to_the_trace_record(fake_service):
    """A row an Events query answered carries the record's kind, instant,
    object, machine, states, target, event, payload and choice; an event
    without an object is a record of the run as a whole."""
    at = sysml_pb2.DocumentValue(quantity=sysml_pb2.Quantity(
        real_magnitude=1.5, unit="s",
        unit_term=sysml_pb2.UnitTerm(
            scale_num=1, scale_den=1, factors=[sysml_pb2.UnitFactor(unit_id="SI::second", exponent=1)]
        ),
    ))
    accept = sysml_pb2.DocumentEvent(
        kind="accept", time=at, object=_lamp_wire(), machine="lp",
        event="Toggle", payload=["level = 2"], text="accept Toggle",
    )
    send = sysml_pb2.DocumentEvent(
        kind="send", time=at, object=_lamp_wire(), machine="lp",
        target=_lamp_wire(2, "#2"), event="Toggle", text="send Toggle to #2",
    )
    fired = sysml_pb2.DocumentEvent(
        kind="transition", time=at, object=_lamp_wire(), machine="lp",
        **{"from": "off"}, to="on", event="Toggle", text="off -> on",
    )
    choice = sysml_pb2.DocumentEvent(
        kind="choice", time=at, alternatives=["light", "fan"], taken="fan",
        text="choice: region order [fan, light]",
    )
    response = sysml_pb2.RunDocumentQueryResponse(
        columns=[sysml_pb2.DocumentQueryColumn(name="self")],
        rows=[
            sysml_pb2.DocumentQueryRow(
                element=sysml_pb2.DocumentValue(event=record),
                cells=[sysml_pb2.DocumentQueryCell(values=[sysml_pb2.DocumentValue(event=record)])],
            )
            for record in (accept, send, fired, choice)
        ],
    )
    port, _ = fake_service(response=response)
    with Connection(port=port, auto_start=False) as conn:
        result = conn.load_from_content("package Demo;").run_document_query("Demo::Q")

    lamp = ObjectRef(id=1, path="Lamps::lamp", element=ElementRef("Lamps::lamp", "PartUsage"))
    other = ObjectRef(id=2, path="#2", element=ElementRef("Lamps::lamp", "PartUsage"))
    when = Quantity(1.5, Unit(text="s", factors=(UnitFactor("SI::second", 1),)))
    rows = list(result)
    assert [row.event.kind for row in rows] == ["accept", "send", "transition", "choice"]
    assert rows[0].event == DocumentEvent(
        kind="accept", time=when, text="accept Toggle", object=lamp, machine="lp",
        event="Toggle", payload=("level = 2",),
    )
    assert rows[0].object == lamp
    assert rows[0].element == lamp.element
    assert rows[0].state is None
    assert rows[0].verdict is None
    assert rows[0][0] == (rows[0].event,)
    assert rows[1].event.target == other
    assert (rows[2].event.from_state, rows[2].event.to_state) == ("off", "on")
    assert rows[3].event.object is None
    assert rows[3].object is None
    assert rows[3].element == ElementRef("")
    assert (rows[3].event.alternatives, rows[3].event.taken) == (("light", "fan"), "fan")
    assert str(rows[0].event) == "1.5 [s]: accept Toggle"


def test_a_row_that_is_no_verdict_carries_none(fake_service):
    response = sysml_pb2.RunDocumentQueryResponse(
        rows=[sysml_pb2.DocumentQueryRow(
            element=sysml_pb2.DocumentValue(element_id="Demo::part", element_type="PartUsage"),
        )],
    )
    port, _ = fake_service(response=response)
    with Connection(port=port, auto_start=False) as conn:
        (row,) = conn.load_from_content("package Demo;").run_document_query("Demo::Q")
    assert row.verdict is None


def test_render_document_answers_the_markdown(fake_service):
    port, service = fake_service(markdown="# Report\n")
    with Connection(port=port, auto_start=False) as conn:
        model = conn.load_from_content("package Demo;")
        assert model.render_document("Demo::Doc") == "# Report\n"
    (request,) = service.requests
    assert request.model_hash == model.hash
    assert request.document_id == "Demo::Doc"
    assert request.form == ""


def test_render_document_asks_for_and_answers_the_html(fake_service):
    port, service = fake_service(html="<!DOCTYPE html>\n")
    with Connection(port=port, auto_start=False) as conn:
        model = conn.load_from_content("package Demo;")
        assert model.render_document("Demo::Doc", form="html") == "<!DOCTYPE html>\n"
    (request,) = service.requests
    assert request.form == "html"


@pytest.fixture(scope="module")
def real_service():
    """Run the built sysml-grpc on an ephemeral port, or skip."""
    binary = next((b for b in GRPC_BINARIES if os.access(b, os.X_OK)), None)
    if binary is None:
        pytest.skip(f"no executable sysml-grpc in {GRPC_BINARIES}; run: make build-grpc")

    port = 51157
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


@pytest.fixture(scope="module")
def telescope():
    """The telescope fixture's source, read from the repo it tests."""
    with open(FIXTURE, encoding="utf-8") as f:
        return f.read()


@pytest.fixture(scope="module")
def garage():
    """The verdict fixture's source, read from the repo it tests."""
    with open(VERDICT_FIXTURE, encoding="utf-8") as f:
        return f.read()


@pytest.fixture(scope="module")
def garage_objects():
    """The object fixture's source, stamped so each test gets a model of its
    own: the service holds the objects it instantiates per model, and the
    model is its content's hash."""
    with open(OBJECT_FIXTURE, encoding="utf-8") as f:
        source = f.read()

    def stamped(tag):
        return f"{source}\n// {tag}\n"

    return stamped


@pytest.fixture(scope="module")
def lamps():
    """The state fixture's source, stamped per test as the object fixture is."""
    with open(STATE_FIXTURE, encoding="utf-8") as f:
        source = f.read()

    def stamped(tag):
        return f"{source}\n// {tag}\n"

    return stamped


@pytest.mark.integration
class TestDocumentsAgainstRealService:
    """The answers themselves, from the real engine."""

    def test_states_and_events_over_the_object_instantiate_built(self, real_service, lamps):
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(lamps("states"))
            model.instantiate("Lamps::lamp")
            lamp = {"root": ObjectRef(path="Lamps::lamp")}
            states = model.run_document_query("Lamps::CurrentStates", bindings=lamp)
            off = model.run_document_query("Lamps::Off")
            steps = model.run_document_query("Lamps::Steps", bindings=lamp)
        assert states.columns == ("machine", "statePath", "region")
        (state,) = states
        assert state.state == DocumentState(
            object=ObjectRef(1, "Lamps::lamp", ElementRef("Lamps::lamp", "PartUsage")),
            machine="lp", name="off", path="off",
            state=ElementRef("Lamps::LampMachine::off", "StateUsage"),
        )
        assert str(state.object) == "Lamps::lamp"
        assert (state[0], state[1], state[2]) == (("lp",), ("off",), ("",))
        assert [str(row.object) for row in off] == ["Lamps::lamp"]
        (entered,) = steps
        assert entered.event.kind == "entry"
        assert entered.event.state == "off"
        assert entered.event.object == state.state.object
        assert entered.event.time == Quantity(0.0, Unit(text="s", factors=(UnitFactor("SI::second", 1),)))
        assert entered[0] == (entered.event.time,)
        assert entered[1] == ("off",)

    def test_a_verdicts_query_checks_the_element_as_declared(self, real_service, garage):
        with Connection(port=real_service, auto_start=False) as conn:
            result = conn.load_from_content(garage).run_document_query(
                "Garage::Checks", bindings={"root": ElementRef("Garage::car")}
            )
        assert result.columns == ("path", "name", "verdict", "reason")
        by_text = {f"{row.verdict.text} on {row.verdict.path}": row for row in result}
        mass_ok = by_text["assert constraint massOk on Garage::car"]
        assert mass_ok.verdict.status == "holds"
        assert mass_ok.verdict.kind == "constraint"
        assert mass_ok.verdict.reason == ""
        assert mass_ok.element == ElementRef("Garage::Car::massOk", "ConstraintUsage")
        assert mass_ok[0] == ("Garage::car",)
        assert mass_ok[2] == ("holds",)
        power_low = by_text["assert constraint powerLow on Garage::car.engine"].verdict
        assert power_low.status == "violated"
        assert power_low.condition
        assert power_low.reason
        fits = by_text["assert constraint fits on Garage::car"].verdict
        assert fits.status == "undecided"
        assert "capacity" in fits.reason
        satisfied = by_text["satisfy strongEngine by car.engine on Garage::car.engine"]
        assert satisfied.verdict.kind == "satisfaction"
        assert satisfied.verdict.status == "holds"
        assert satisfied.verdict.verification == ("pass",)
        assert satisfied.element == ElementRef("", "SatisfyRequirementUsage")

    def test_a_query_binds_the_object_instantiate_built_by_id(
        self, real_service, garage_objects
    ):
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(garage_objects("by id"))
            car = model.instantiate("Garage::car")
            result = model.run_document_query(
                "Garage::Parts", bindings={"root": ObjectRef(id=car.id)}
            )
        assert result.columns == ("name", "pressure")
        # Bound by id, the wheels are reached from "#<id>"; each is an object
        # of its own, standing for the usage it is held under.
        assert [str(row.object) for row in result] == [
            f"#{car.id}.wheels[1]", f"#{car.id}.wheels[2]",
        ]
        assert [row.element for row in result] == [
            ElementRef("Garage::Car::wheels", "PartUsage"),
            ElementRef("Garage::Car::wheels", "PartUsage"),
        ]
        assert len({row.object.id for row in result} | {car.id}) == 3
        assert [(row[0], row[1]) for row in result] == [
            (("wheels[1]",), (30,)), (("wheels[2]",), (30,)),
        ]
        assert result.rows[0].object.element == ElementRef("Garage::Car::wheels", "PartUsage")

    def test_a_query_binds_the_object_instantiate_built_by_path(
        self, real_service, garage_objects
    ):
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(garage_objects("by path"))
            car = model.instantiate("Garage::car")
            whole = model.run_document_query(
                "Garage::Drive", bindings={"root": ObjectRef(path="car")}
            )
            wheel = model.run_document_query(
                "Garage::Pressure", bindings={"root": ObjectRef(path="car.wheels[2]")}
            )
            both = model.run_document_query(
                "Garage::Drive", bindings={"root": ObjectRef(id=car.id, path="Garage::car")}
            )
        (row,) = whole
        assert row.object == ObjectRef(car.id, "Garage::car", ElementRef("Garage::car", "PartUsage"))
        assert row.element == row.object.element
        assert row[0] == (1200,)
        assert [str(w) for w in row[1]] == ["Garage::car.wheels[1]", "Garage::car.wheels[2]"]
        assert {type(value) for value in row[1]} == {ObjectRef}
        assert {w.element for w in row[1]} == {ElementRef("Garage::Car::wheels", "PartUsage")}
        (row,) = wheel
        assert str(row.object) == "Garage::car.wheels[2]"
        assert row.object.id == whole.rows[0][1][1].id
        assert row.element == ElementRef("Garage::Car::wheels", "PartUsage")
        assert (row[0], row[1]) == (("wheels[2]",), (30,))
        assert both.rows == whole.rows

    def test_objects_enumerates_what_the_model_holds(self, real_service, garage_objects):
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(garage_objects("objects"))
            before = model.run_document_query("Garage::Wheels")
            model.instantiate("Garage::car")
            model.instantiate("Garage::spare")
            after = model.run_document_query("Garage::Wheels")
        assert before.columns == ("pressure",)
        assert len(before) == 0
        assert [(str(row.object), row[0]) for row in after] == [
            ("Garage::spare", (20,)),
            ("Garage::car.wheels[1]", (30,)),
            ("Garage::car.wheels[2]", (30,)),
        ]

    def test_verdicts_over_the_object_instantiate_built(self, real_service, garage_objects):
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(garage_objects("verdicts"))
            model.instantiate("Garage::car")
            model.instantiate("Garage::spare")
            car = model.run_document_query(
                "Garage::Checks", bindings={"root": ObjectRef(path="car")}
            )
            spare = model.run_document_query(
                "Garage::Checks", bindings={"root": ObjectRef(path="spare")}
            )
        assert car.columns == ("path", "verdict")
        assert [(row.verdict.text, row[0], row[1]) for row in car] == [
            ("assert constraint light", ("Garage::car",), ("holds",)),
            ("assert constraint inflated", ("Garage::car.wheels[1]",), ("holds",)),
            ("assert constraint inflated", ("Garage::car.wheels[2]",), ("holds",)),
        ]
        assert car.rows[0].element == ElementRef("Garage::Car::light", "ConstraintUsage")
        (row,) = spare
        assert row.verdict.status == "violated"
        assert row.verdict.path == "Garage::spare"
        assert row.verdict.reason

    def test_an_object_binding_the_model_cannot_reach_is_refused(
        self, real_service, garage_objects
    ):
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(garage_objects("refused"))
            unpopulated = {"root": ObjectRef(id=1)}
            with pytest.raises(SymbolNotFoundError, match="holds no objects"):
                model.run_document_query("Garage::Parts", bindings=unpopulated)
            model.instantiate("Garage::car")
            unknown_id = {"root": ObjectRef(id=99)}
            with pytest.raises(SymbolNotFoundError, match="no object #99"):
                model.run_document_query("Garage::Parts", bindings=unknown_id)
            unknown_path = {"root": ObjectRef(path="spare")}
            with pytest.raises(SymbolNotFoundError, match="no instance of"):
                model.run_document_query("Garage::Parts", bindings=unknown_path)
            unknown_feature = {"root": ObjectRef(path="car.hood")}
            with pytest.raises(InvalidRequestError, match="hood"):
                model.run_document_query("Garage::Parts", bindings=unknown_feature)
            value_path = {"root": ObjectRef(path="car.mass")}
            with pytest.raises(InvalidRequestError, match="not an object"):
                model.run_document_query("Garage::Parts", bindings=value_path)
            malformed_path = {"root": ObjectRef(path="car..wheels")}
            with pytest.raises(InvalidRequestError, match="not an object reference"):
                model.run_document_query("Garage::Parts", bindings=malformed_path)
            disagreeing = {"root": ObjectRef(id=99, path="car")}
            with pytest.raises(InvalidRequestError, match="is object #"):
                model.run_document_query("Garage::Parts", bindings=disagreeing)

    def test_a_document_query_answers_typed_ordered_rows(self, real_service, telescope):
        with Connection(port=real_service, auto_start=False) as conn:
            result = conn.load_from_content(telescope).run_document_query(
                "Observatory::SubsystemTable",
                bindings={"root": ElementRef("Observatory::telescope")},
            )
        assert result.columns == ("name", "mass")
        assert [row.element.id for row in result] == [
            "Observatory::telescope::baffle|shroud *tricky*",
            "Observatory::telescope::mount",
            "Observatory::telescope::optics",
            "Observatory::telescope::segmentControl",
        ]
        assert [row[0] for row in result] == [
            ("baffle|shroud *tricky*",), ("mount",), ("optics",), ("segmentControl",),
        ]
        assert [row[1] for row in result] == [(1.5,), (15.0,), (8.5,), (20.0,)]
        assert {row.element.type for row in result} == {"PartUsage"}

    def test_a_query_matching_nothing_answers_columns_and_no_rows(
        self, real_service, telescope
    ):
        with Connection(port=real_service, auto_start=False) as conn:
            result = conn.load_from_content(telescope).run_document_query(
                "Observatory::MissingSubsystems",
                bindings={"root": ElementRef("Observatory::telescope")},
            )
        assert result.columns == ("name", "mass")
        assert len(result) == 0

    def test_a_rendered_document_is_the_renderer_s_golden(self, real_service, telescope):
        with open(GOLDEN, encoding="utf-8") as f:
            golden = f.read()
        with Connection(port=real_service, auto_start=False) as conn:
            markdown = conn.load_from_content(telescope).render_document(
                "Observatory::MassReport"
            )
        assert markdown == golden

    def test_a_rendered_html_document_is_the_renderer_s_golden(self, real_service, telescope):
        with open(HTML_GOLDEN, encoding="utf-8") as f:
            golden = f.read()
        with Connection(port=real_service, auto_start=False) as conn:
            html = conn.load_from_content(telescope).render_document(
                "Observatory::MassReport", form="html"
            )
        assert html == golden

    def test_an_unknown_query_raises_symbol_not_found(self, real_service, telescope):
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(telescope)
            with pytest.raises(SymbolNotFoundError):
                model.run_document_query("Observatory::NoSuchQuery")

    def test_a_symbol_that_is_not_a_query_is_refused(self, real_service, telescope):
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(telescope)
            with pytest.raises(InvalidRequestError, match="not a document query"):
                model.run_document_query("Observatory::Subsystem")

    def test_a_symbol_that_is_not_a_document_is_refused(self, real_service, telescope):
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(telescope)
            with pytest.raises(InvalidRequestError, match="not a document"):
                model.render_document("Observatory::SubsystemTable")

    def test_a_wrong_binding_is_refused_with_the_engine_s_message(
        self, real_service, telescope
    ):
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(telescope)
            bindings = {"depth": 3, "root": ElementRef("Observatory::telescope")}
            with pytest.raises(InvalidRequestError):
                model.run_document_query("Observatory::SubsystemTable", bindings=bindings)

    def test_an_evicted_model_raises_this_library_s_error(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            with pytest.raises(ModelNotFoundError):
                conn.run_document_query("deadbeef", "Observatory::SubsystemTable")
            with pytest.raises(ModelNotFoundError):
                conn.render_document("deadbeef", "Observatory::MassReport")

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
    MissingCapabilityError,
)
from opensysml.connection import Connection
from opensysml.document import (
    INFINITY,
    DocumentQueryError,
    DocumentQueryResult,
    DocumentRow,
    ElementRef,
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
    REPO_ROOT, "internal", "core", "docrender", "testdata", "telescope_report.sysml"
)
GOLDEN = os.path.join(
    REPO_ROOT, "internal", "core", "docrender", "testdata", "telescope_report.golden.md"
)

CAPABILITIES = (CAPABILITY_DOCUMENT_QUERY, CAPABILITY_RENDER_DOCUMENT)


class FakeService(sysml_pb2_grpc.SysMLServiceServicer):
    """A sysml-grpc whose document RPCs record requests and answer as told."""

    def __init__(self, capabilities=CAPABILITIES, response=None, markdown=""):
        self._capabilities = list(capabilities)
        self._response = response or sysml_pb2.RunDocumentQueryResponse()
        self._markdown = markdown
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


def test_render_document_answers_the_markdown(fake_service):
    port, service = fake_service(markdown="# Report\n")
    with Connection(port=port, auto_start=False) as conn:
        model = conn.load_from_content("package Demo;")
        assert model.render_document("Demo::Doc") == "# Report\n"
    (request,) = service.requests
    assert request.model_hash == model.hash
    assert request.document_id == "Demo::Doc"


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


@pytest.mark.integration
class TestDocumentsAgainstRealService:
    """The answers themselves, from the real engine."""

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

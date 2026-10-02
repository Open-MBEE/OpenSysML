"""Tests for migrating a SysML v1 model through the service.

Two layers, as for conversion. Against a fake service, the client's own
behavior: the capability gate, what it refuses before asking, how a request is
shaped, and how the answer becomes a :class:`Migration`. Against the real
``sysml-grpc`` binary, the migration itself: the model that comes out, the
ledger that accounts for every element, and the options that reach it.
"""

import json
import os
import subprocess
import time
import warnings
from concurrent import futures

import grpc
import pytest

import opensysml
from opensysml.capabilities import CAPABILITY_MIGRATE, MissingCapabilityError
from opensysml.connection import Connection
from opensysml.conversion import (
    ExperimentalFeatureWarning,
    Migration,
    MigrationEntry,
    MigrationReport,
    is_v1,
    path_is_v1,
)
from opensysml.errors import InvalidRequestError, MigrationError, ModelFileNotFoundError
from opensysml.proto import sysml_pb2, sysml_pb2_grpc

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
FIXTURES = os.path.join(REPO_ROOT, "tests", "migrate", "testdata", "xmi")
VEHICLE = os.path.join(FIXTURES, "vehicle.xmi")
LAYOUT_MODEL = os.path.join(FIXTURES, "layout.xmi")
LAYOUT = os.path.join(FIXTURES, "layout.layout.xml")
GRPC_BINARIES = (
    os.path.join(REPO_ROOT, "bin", "sysml-grpc"),
    os.path.join(os.path.expanduser("~"), ".opensysml", "bin", "sysml-grpc"),
)

NOTICE = "SysML v1 migration is experimental: fake notice"


class FakeService(sysml_pb2_grpc.SysMLServiceServicer):
    """A sysml-grpc whose Migrate records the request and answers as told."""

    def __init__(self, capabilities=(CAPABILITY_MIGRATE,), error="", refuse=False,
                 missing_file=False):
        self._capabilities = list(capabilities)
        self._error = error
        self._refuse = refuse
        self._missing_file = missing_file
        self.requests = []

    def GetServerInfo(self, request, context):
        return sysml_pb2.ServerInfoResponse(version="fake", capabilities=self._capabilities)

    def GetDiagnostics(self, request, context):
        context.abort(grpc.StatusCode.NOT_FOUND, "model not found")

    def Migrate(self, request, context):
        self.requests.append(request)
        if self._refuse:
            context.abort(grpc.StatusCode.UNIMPLEMENTED, "capability 'migrate' is unavailable")
        if self._missing_file:
            context.abort(grpc.StatusCode.NOT_FOUND, "file not found: " + request.file_path)
        report = sysml_pb2.MigrationReport(
            source=request.file_path or "<content>",
            exporter="Fake Tool",
            summary="migrated 3 element(s): 1 mapped, 1 approximated, 1 unmapped (1 skipped)",
            mapped=1, approximated=1, unmapped=1, skipped=1,
        )
        if request.report:
            report.entries.extend([
                sysml_pb2.MigrationEntry(id="_a", kind="Class", name="P::A", target="A", verdict="mapped"),
                sysml_pb2.MigrationEntry(id="_b", kind="Package", name="Requirements", target="RequirementsModel",
                                         verdict="approximated", note="renamed"),
                sysml_pb2.MigrationEntry(id="_c", kind="«Unit» InstanceSpecification", name="P::kg",
                                         verdict="unmapped", note="units are not migrated"),
                sysml_pb2.MigrationEntry(id="_d", kind="Profile", name="SysML", verdict="skipped"),
            ])
            report.text = "# SysML v1 to v2 migration report\n" + report.summary + "\n"
        return sysml_pb2.MigrateResponse(
            content="" if self._error else "part def A;\n",
            from_format="xmi",
            to_format=request.to_format,
            error=self._error,
            experimental=True,
            experimental_notice=NOTICE,
            report=report,
            results='{"runs": []}' if request.results else "",
            files=[sysml_pb2.MigrationFile(path="images/a.png", content=b"\x89PNG")] if request.results else [],
        )


@pytest.fixture
def fake_service():
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


def test_v1_is_named_by_format_or_extension():
    assert is_v1("xmi") and is_v1("uml") and is_v1("mdzip")
    assert is_v1("XMI") and is_v1(" mdzip ") and not is_v1(None)
    assert not is_v1("sysml") and not is_v1("ttl") and not is_v1("")
    assert path_is_v1("Model.mdzip") and path_is_v1("Model.XMI") and path_is_v1("/tmp/m.uml")
    assert not path_is_v1("Model.sysml") and not path_is_v1("Model.json")


def test_migrate_sends_the_source_and_options(fake_service):
    """The request carries what the caller asked for, unchanged, and the
    answer is a Migration whose report always has the summary and counts."""
    port, service = fake_service()
    with Connection(port=port, auto_start=False) as conn:
        with pytest.warns(ExperimentalFeatureWarning, match="fake notice"):
            result = conn.migrate(
                "ttl", content=b"<xmi/>", from_format="mdzip",
                layout_content="<mtip/>", image_base_url="https://img/", strict=True,
            )
    (request,) = service.requests
    assert request.WhichOneof("source") == "content"
    assert request.content == b"<xmi/>"
    assert (request.from_format, request.to_format) == ("mdzip", "ttl")
    assert request.WhichOneof("layout") == "layout_content"
    assert request.layout_content == "<mtip/>"
    assert request.image_base_url == "https://img/"
    assert request.strict is True
    assert request.report is False and request.results is False

    assert isinstance(result, Migration)
    assert str(result) == "part def A;\n"
    assert (result.from_format, result.to_format) == ("xmi", "ttl")
    assert result.experimental is True
    assert result.experimental_notice == NOTICE
    report = result.report
    assert isinstance(report, MigrationReport)
    assert (report.mapped, report.approximated, report.unmapped, report.skipped) == (1, 1, 1, 1)
    assert report.summary.startswith("migrated 3 element(s)")
    assert report.exporter == "Fake Tool"
    assert report.entries == [] and report.text == ""
    assert str(report) == report.summary
    assert result.results == "" and result.files == {}


def test_migrate_asks_for_the_report_results_and_layout_file(fake_service, tmp_path):
    port, service = fake_service()
    with Connection(port=port, auto_start=False) as conn:
        with pytest.warns(ExperimentalFeatureWarning):
            result = conn.migrate(
                "sysml", file_path="Model.mdzip", report=True, results=True,
                layout_path="Model_mtip.xml",
            )
    (request,) = service.requests
    assert request.WhichOneof("source") == "file_path"
    assert request.file_path == "Model.mdzip"
    assert request.from_format == ""
    assert request.report is True and request.results is True
    assert request.WhichOneof("layout") == "layout_path"
    assert request.layout_path == "Model_mtip.xml"

    report = result.report
    assert [e.verdict for e in report.entries] == ["mapped", "approximated", "unmapped", "skipped"]
    assert report.entries[1] == MigrationEntry(
        id="_b", kind="Package", name="Requirements", target="RequirementsModel",
        verdict="approximated", note="renamed",
    )
    assert [e.id for e in report.by_verdict("unmapped")] == ["_c"]
    assert report.text.startswith("# SysML v1 to v2 migration report")
    assert str(report) == report.text
    assert json.loads(result.results) == {"runs": []}
    assert result.files == {"images/a.png": b"\x89PNG"}

    out = tmp_path / "out" / "Model.sysml"
    out.parent.mkdir()
    assert result.write(str(out)) == str(out)
    assert out.read_text() == "part def A;\n"
    assert (tmp_path / "out" / "images" / "a.png").read_bytes() == b"\x89PNG"


def test_migrate_refuses_what_is_not_a_v1_model_before_asking(fake_service):
    """A v2 format is converted, not migrated; inline bytes must say their
    form; a layout is one of path or content; a source is exactly one."""
    port, service = fake_service()
    with Connection(port=port, auto_start=False) as conn:
        with pytest.raises(InvalidRequestError, match="converted, not migrated"):
            conn.migrate("sysml", file_path="Model.sysml", from_format="sysml")
        with pytest.raises(InvalidRequestError, match="call convert\\(\\)"):
            conn.migrate("sysml", content=b"package P;", from_format="ttl")
        with pytest.raises(ValueError, match="from_format is required"):
            conn.migrate("sysml", content=b"<xmi/>")
        with pytest.raises(ValueError, match="exactly one"):
            conn.migrate("sysml")
        with pytest.raises(ValueError, match="exactly one"):
            conn.migrate("sysml", file_path="a.xmi", content=b"<xmi/>", from_format="xmi")
        with pytest.raises(ValueError, match="at most one"):
            conn.migrate("sysml", file_path="a.xmi", layout_path="l.xml", layout_content="<mtip/>")
    assert service.requests == []


def test_migrate_requires_the_capability(fake_service):
    port, service = fake_service(capabilities=("convert",))
    with Connection(port=port, auto_start=False) as conn:
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.migrate("sysml", file_path="Model.mdzip")
    assert excinfo.value.capability == CAPABILITY_MIGRATE
    assert service.requests == []


def test_service_side_refusal_is_a_missing_capability_error(fake_service):
    port, service = fake_service(refuse=True)
    with Connection(port=port, auto_start=False) as conn:
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.migrate("sysml", file_path="Model.mdzip")
    assert excinfo.value.capability == CAPABILITY_MIGRATE
    assert excinfo.value.__cause__.code() == grpc.StatusCode.UNIMPLEMENTED
    assert len(service.requests) == 1


def test_a_missing_file_and_a_refused_model_are_typed(fake_service):
    port, _ = fake_service(missing_file=True)
    with Connection(port=port, auto_start=False) as conn:
        with pytest.raises(ModelFileNotFoundError):
            conn.migrate("sysml", file_path="/nonexistent/Model.mdzip")
    port, _ = fake_service(error="not a SysML v1 model: no uml:Model")
    with Connection(port=port, auto_start=False) as conn:
        # Warned before the error is raised: the refusal is the experimental
        # behavior, not a reason to say nothing about it.
        with pytest.warns(ExperimentalFeatureWarning):
            with pytest.raises(MigrationError, match="no uml:Model"):
                conn.migrate("sysml", file_path="Model.mdzip")


def test_module_level_migrate_forwards_every_option(fake_service, monkeypatch):
    port, service = fake_service()
    opensysml._default_connection = None
    with warnings.catch_warnings():
        warnings.simplefilter("ignore", ExperimentalFeatureWarning)
        result = opensysml.migrate(
            "sysml", file_path="Model.xmi", report=True, strict=True, port=port,
        )
    opensysml._default_connection.close()
    opensysml._default_connection = None
    (request,) = service.requests
    assert request.file_path == "Model.xmi"
    assert request.report is True and request.strict is True
    assert isinstance(result, Migration)


@pytest.fixture(scope="module")
def real_service():
    """Run the built sysml-grpc on an ephemeral port, or skip."""
    binary = next((b for b in GRPC_BINARIES if os.access(b, os.X_OK)), None)
    if binary is None:
        pytest.skip(f"no executable sysml-grpc in {GRPC_BINARIES}; run: make build-grpc")

    port = 51152
    process = subprocess.Popen(
        [binary, "-port", str(port)], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
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
class TestMigrationAgainstRealService:
    """The migration itself, through the real migrator."""

    def test_a_v1_model_is_migrated_and_accounted_for(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            with pytest.warns(ExperimentalFeatureWarning, match="SysML v1 migration"):
                by_path = conn.migrate("sysml", file_path=VEHICLE)
            with open(VEHICLE, "rb") as handle:
                data = handle.read()
            with pytest.warns(ExperimentalFeatureWarning):
                by_content = conn.migrate("sysml", content=data, from_format="xmi", report=True)

        assert "part def Vehicle" in str(by_path)
        assert str(by_path) == str(by_content), "the file and its bytes migrate differently"
        assert (by_path.from_format, by_path.to_format) == ("xmi", "sysml")
        summary = by_path.report
        assert summary.summary.startswith("migrated ")
        assert summary.mapped > 0 and summary.entries == [] and summary.text == ""

        report = by_content.report
        assert report.entries, "the report was asked for"
        verdicts = {verdict: len(report.by_verdict(verdict))
                    for verdict in ("mapped", "approximated", "unmapped", "skipped")}
        assert verdicts == {
            "mapped": report.mapped, "approximated": report.approximated,
            "unmapped": report.unmapped, "skipped": report.skipped,
        }
        assert sum(verdicts.values()) == len(report.entries)
        assert report.text.startswith("# SysML v1 to v2 migration report")
        assert report.summary in report.text

    def test_the_layout_and_results_reach_the_migration(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn, warnings.catch_warnings():
            warnings.simplefilter("ignore", ExperimentalFeatureWarning)
            laid_out = conn.migrate("sysml", file_path=LAYOUT_MODEL, layout_path=LAYOUT, results=True)
            with open(LAYOUT, encoding="utf-8") as handle:
                inline = conn.migrate("sysml", file_path=LAYOUT_MODEL, layout_content=handle.read())
            as_turtle = conn.migrate("ttl", file_path=VEHICLE)
            strict = conn.migrate("sysml", file_path=VEHICLE, strict=True)
        assert "laid out" in laid_out.report.summary
        assert str(laid_out) == str(inline)
        assert isinstance(json.loads(laid_out.results), dict)
        assert as_turtle.to_format == "ttl" and "@prefix" in str(as_turtle)
        assert strict.report.mapped > 0

    def test_convert_refuses_v1_and_migrate_refuses_v2(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            with pytest.raises(InvalidRequestError, match="migrated, not converted"):
                conn.convert("sysml", file_path=VEHICLE)
            with pytest.raises(InvalidRequestError, match="converted, not migrated"):
                conn.migrate("sysml", file_path=os.path.join(REPO_ROOT, "examples", "action-executor-demo.sysml"))
            with pytest.raises(ModelFileNotFoundError):
                conn.migrate("sysml", file_path=os.path.join(FIXTURES, "nonexistent.xmi"))

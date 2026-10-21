"""Tests for the data import and the results comparison: the request sent, the
result built from the answer, and the capability the client requires first."""

from unittest.mock import Mock, patch

import pytest

from opensysml.capabilities import (
    CAPABILITY_COMPARE_RESULTS,
    CAPABILITY_FEATURE_VALUES,
    CAPABILITY_IMPORT_DATA,
    MissingCapabilityError,
)
from opensysml.connection import Connection
from opensysml.errors import ExecutionError
from opensysml.importcompare import Comparison, ImportResult
from opensysml.proto import sysml_pb2


def make_connection(stub, capabilities):
    stub.GetServerInfo.return_value = sysml_pb2.ServerInfoResponse(
        version="test", capabilities=list(capabilities)
    )
    with patch("grpc.insecure_channel"), \
            patch("opensysml.proto.sysml_pb2_grpc.SysMLServiceStub", return_value=stub):
        return Connection(auto_start=False)


CURRENT = (CAPABILITY_FEATURE_VALUES, CAPABILITY_IMPORT_DATA, CAPABILITY_COMPARE_RESULTS)


def import_response():
    return sysml_pb2.ImportDataResponse(
        files=[sysml_pb2.ImportedFile(
            name="fleet.csv", values=2, elements=1,
            assignments=[
                sysml_pb2.ImportedValue(element="V::car", feature="count", value="9", where="model.sysml:9:3",
                                        description="V::car::count = 9"),
                sysml_pb2.ImportedValue(element="V::car", feature="label", value='"fast"',
                                        redefines="V::Vehicle::label", where="model.sysml:9:3",
                                        description='V::car::label = "fast" (redefines V::Vehicle::label)'),
            ])],
        documents=[sysml_pb2.EditedDocument(name="model.sysml", content="package V { }")],
        content="package V { }",
        model_hash="h2",
    )


def test_import_data_sends_each_import_in_order_and_builds_the_result():
    stub = Mock()
    stub.ImportData.return_value = import_response()
    conn = make_connection(stub, CURRENT)
    result = conn.import_data("h", [("fleet.csv", "element,count\nV::car,9\n"),
                                    {"name": "more.json", "content": "[]", "format": "json", "map": "{}"}])
    request = stub.ImportData.call_args[0][0]
    assert request.model_hash == "h"
    assert [i.name for i in request.imports] == ["fleet.csv", "more.json"]
    assert request.imports[0].content == "element,count\nV::car,9\n"
    assert request.imports[1].import_format == "json"
    assert request.imports[1].import_map == "{}"
    assert request.import_as == "values"
    assert not request.import_dry_run
    assert isinstance(result, ImportResult)
    assert result.model_hash == "h2"
    assert result.content == "package V { }"
    assert [f.name for f in result.files] == ["fleet.csv"]
    assert result.files[0].values == 2
    assert result.files[0].elements == 1
    assert result.values == 2
    assert [a.description for a in result.files[0].assignments] == [
        "V::car::count = 9", 'V::car::label = "fast" (redefines V::Vehicle::label)']
    assert result.files[0].assignments[1].redefines == "V::Vehicle::label"
    assert [d.name for d in result.documents] == ["model.sysml"]


def test_import_data_dry_run_is_sent_and_a_refusal_raises():
    stub = Mock()
    stub.ImportData.return_value = sysml_pb2.ImportDataResponse(error="fleet.csv line 2: no element named V::Nowhere")
    conn = make_connection(stub, CURRENT)
    with pytest.raises(ExecutionError, match="V::Nowhere"):
        conn.import_data("h", [("fleet.csv", "element\nV::Nowhere\n")], dry_run=True, accept_documents=True)
    request = stub.ImportData.call_args[0][0]
    assert request.import_dry_run
    assert request.accept_documents


def test_import_data_requires_the_capability():
    stub = Mock()
    conn = make_connection(stub, (CAPABILITY_FEATURE_VALUES,))
    with pytest.raises(MissingCapabilityError, match=CAPABILITY_IMPORT_DATA):
        conn.import_data("h", [("fleet.csv", "")])
    stub.ImportData.assert_not_called()


def compare_response():
    return sysml_pb2.CompareResultsResponse(
        source="probe.xmi",
        engine="interpreter", strength="observed",
        configurations=[
            sysml_pb2.ConfigurationComparison(
                id="_c1", name="Cfg::'Group 1'", behavior="Cfg::'Group 1'::run", target="target",
                status="holds", stored_runs=2, stored_snapshots=2, runs=2, completed=2, draws="random", seed=7,
                rows=[sysml_pb2.SweepRow(outputs=[sysml_pb2.CalcOutput(
                    name="target.total", value=sysml_pb2.Value(real_value=4.0))]) for _ in (1, 2)],
                observables=[sysml_pb2.ObservableComparison(
                    stored="total", feature="target.total", compared=True, produced=2,
                    tool=sysml_pb2.ComparedDistribution(count=2, min=4.0, mean=5.0, p50=4.0, p90=6.0, max=6.0,
                                                        deviation=1.0),
                    run=sysml_pb2.ComparedDistribution(count=2, min=4.0, mean=4.0, p50=4.0, p90=4.0, max=4.0),
                    difference=sysml_pb2.StatisticDifference(min="+0.0%", mean="-20.0%", p50="+0.0%",
                                                              p90="-33.3%", max="-33.3%"),
                    declared=sysml_pb2.CaseStatistics(
                        analysis="Cfg::Study",
                        rows=[sysml_pb2.CaseStatistic(statistic="mean", of="total", tool="5.0", run="4.0",
                                                      difference="-20.0%")]),
                    notes=["the tool pooled two summaries"])],
                lines=["total: tool mean 5.0, runs mean 4.0 (-20.0%)"]),
            sysml_pb2.ConfigurationComparison(
                id="_cn", name="Cfg::Nothing", status="unresolved",
                unresolved="the configuration names no behavior", lines=["Cfg::Nothing: unresolved"]),
        ],
    )


def test_compare_results_sends_the_options_and_builds_the_comparison():
    stub = Mock()
    stub.CompareResults.return_value = compare_response()
    conn = make_connection(stub, CURRENT)
    result = conn.compare_results("h", '{"configurations":[]}', actions=["Group 1"], runs=2, seed=7,
                                  draws="random", clock_step=0.5, observe=[("total", "target.total")])
    request = stub.CompareResults.call_args[0][0]
    assert request.model_hash == "h"
    assert request.results == '{"configurations":[]}'
    assert list(request.actions) == ["Group 1"]
    assert request.runs == 2
    assert request.HasField("seed")
    assert request.seed == 7
    assert request.draws == "random"
    assert request.HasField("clock_step")
    assert request.clock_step == 0.5
    assert [(o.stored, o.feature) for o in request.observe] == [("total", "target.total")]
    assert request.engine == ""
    assert isinstance(result, Comparison)
    assert result.source == "probe.xmi"
    assert result.standing.engine == "interpreter"
    assert not result.holds
    first, second = result.configurations
    assert first.holds
    assert first.name == "Cfg::'Group 1'"
    assert first.runs == 2
    assert first.completed == 2
    assert first.stored_runs == 2
    assert first.seed == 7
    assert first.draws == "random"
    assert len(first.rows) == 2
    assert first.rows[0].outputs["target.total"] == 4.0
    observable = first.observables[0]
    assert observable.stored == "total"
    assert observable.feature == "target.total"
    assert observable.compared
    assert observable.tool.count == 2
    assert observable.tool.mean == 5.0
    assert observable.tool.deviation == 1.0
    assert observable.run.mean == 4.0
    assert observable.difference.mean == "-20.0%"
    assert observable.difference.max == "-33.3%"
    assert observable.declared.analysis == "Cfg::Study"
    assert observable.declared.rows[0].statistic == "mean"
    assert observable.declared.rows[0].difference == "-20.0%"
    assert observable.notes == ["the tool pooled two summaries"]
    assert first.lines == ["total: tool mean 5.0, runs mean 4.0 (-20.0%)"]
    assert second.status == "unresolved"
    assert not second.holds
    assert second.seed is None
    assert "the configuration names no behavior" in second.unresolved
    assert "Cfg::Nothing: unresolved" in str(result)


def test_compare_results_leaves_unset_options_unset_and_raises_a_failure():
    stub = Mock()
    stub.CompareResults.return_value = sysml_pb2.CompareResultsResponse(
        error="results: no source or configurations")
    conn = make_connection(stub, CURRENT)
    with pytest.raises(ExecutionError, match="no source or configurations"):
        conn.compare_results("h", "{}")
    request = stub.CompareResults.call_args[0][0]
    assert not request.HasField("seed")
    assert not request.HasField("clock_step")
    assert request.runs == 0
    assert request.draws == ""
    assert list(request.observe) == []


def test_compare_results_requires_the_capability():
    stub = Mock()
    conn = make_connection(stub, (CAPABILITY_FEATURE_VALUES,))
    with pytest.raises(MissingCapabilityError, match=CAPABILITY_COMPARE_RESULTS):
        conn.compare_results("h", "{}")
    stub.CompareResults.assert_not_called()

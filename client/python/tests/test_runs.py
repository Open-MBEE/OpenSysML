"""Tests for repeated runs, the reflective self-check and run recording: the
request sent, the result built from the answer, and the capability the client
requires first."""

from unittest.mock import Mock, patch

import pytest

from opensysml.capabilities import (
    CAPABILITY_FEATURE_VALUES,
    CAPABILITY_RECORD_RUN,
    CAPABILITY_RUNS,
    CAPABILITY_SELF_CHECK,
    MissingCapabilityError,
)
from opensysml.connection import Connection
from opensysml.errors import ExecutionError
from opensysml.proto import sysml_pb2
from opensysml.runs import Recording, RunsResult, SelfCheckResult


def make_connection(stub, capabilities):
    stub.GetServerInfo.return_value = sysml_pb2.ServerInfoResponse(
        version="test", capabilities=list(capabilities)
    )
    with patch("grpc.insecure_channel"):
        with patch("opensysml.proto.sysml_pb2_grpc.SysMLServiceStub", return_value=stub):
            return Connection(auto_start=False)


CURRENT = (CAPABILITY_FEATURE_VALUES, CAPABILITY_RUNS, CAPABILITY_SELF_CHECK, CAPABILITY_RECORD_RUN)


def real(x):
    return sysml_pb2.Value(real_value=x)


def runs_response():
    return sysml_pb2.RunRunsResponse(
        rows=[
            sysml_pb2.SweepRow(inputs=[sysml_pb2.CalcOutput(name="run", value=sysml_pb2.Value(int_value=i))],
                               outputs=[sysml_pb2.CalcOutput(name="t", value=real(float(i)))])
            for i in (1, 2)
        ],
        observables=["t"],
        distributions=[sysml_pb2.RunDistribution(
            name="t", count=2, numeric=True, min=real(1.0), max=real(2.0), mean=real(1.5), deviation=0.5,
            percentiles=[sysml_pb2.RunPercentile(percentile=50, value=real(1.0))],
            histogram=[sysml_pb2.RunHistogramBin(low=real(1.0), high=real(2.0), count=2)],
        )],
        lines=["t: 2 run(s), min 1.0, mean 1.5, max 2.0"],
        seed=7,
    )


def test_run_runs_sends_the_request_and_builds_the_table():
    stub = Mock()
    stub.RunRuns.return_value = runs_response()
    conn = make_connection(stub, CURRENT)
    result = conn.run_runs("h", action="MC::Settle", runs=2, seed=7, observe=["t"])
    request = stub.RunRuns.call_args[0][0]
    assert request.action_symbol_id == "MC::Settle"
    assert request.runs == 2
    assert request.seed == 7
    assert list(request.observe) == ["t"]
    assert isinstance(result, RunsResult)
    assert len(result) == 2
    assert result.seed == 7
    assert result.rows[1].outputs == {"t": 2.0}
    assert result.observables == ["t"]
    dist = result.distributions[0]
    assert dist.numeric
    assert dist.min == 1.0
    assert dist.mean == 1.5
    assert dist.percentiles[0].percentile == 50
    assert dist.histogram[0].count == 2
    assert str(result).startswith("t: 2 run(s)")


def test_run_runs_without_seed_leaves_it_unset():
    stub = Mock()
    stub.RunRuns.return_value = sysml_pb2.RunRunsResponse()
    conn = make_connection(stub, CURRENT)
    result = conn.run_runs("h", action="MC::Settle", runs=1, draws="min")
    assert not stub.RunRuns.call_args[0][0].HasField("seed")
    assert result.seed is None


def test_run_runs_error_raises():
    stub = Mock()
    stub.RunRuns.return_value = sysml_pb2.RunRunsResponse(error="no such action")
    conn = make_connection(stub, CURRENT)
    with pytest.raises(ExecutionError, match="no such action"):
        conn.run_runs("h", action="MC::Nope", runs=1, seed=1)


def test_run_runs_requires_the_capability():
    conn = make_connection(Mock(), (CAPABILITY_FEATURE_VALUES,))
    with pytest.raises(MissingCapabilityError):
        conn.run_runs("h", action="MC::Settle", runs=1, seed=1)


def test_self_check_builds_the_findings():
    stub = Mock()
    stub.SelfCheck.return_value = sysml_pb2.SelfCheckResponse(
        status="fails", elements=3, checks=4, violations=1,
        findings=[sysml_pb2.SelfCheckFinding(status="fails", subject="V::c for P::x", location="m.sysml:2:3",
                                              constraint="V::c", element="P::x", line="m.sysml:2:3: error: V::c fails for P::x")],
        applications=[sysml_pb2.SelfCheckApplication(constraint="V::c", count=4)],
        summary="Self-model check: 3 elements, 4 checks, 1 violations, 0 unevaluated",
    )
    conn = make_connection(stub, CURRENT)
    result = conn.self_check("h", packages=["Rules"])
    assert list(stub.SelfCheck.call_args[0][0].self_check_package) == ["Rules"]
    assert isinstance(result, SelfCheckResult)
    assert not result.holds
    assert result.violations == 1
    assert result.findings[0].element == "P::x"
    assert result.applications == {"V::c": 4}
    assert str(result).endswith("0 unevaluated")


def test_self_check_error_and_capability():
    stub = Mock()
    stub.SelfCheck.return_value = sysml_pb2.SelfCheckResponse(error="Nope names no package")
    conn = make_connection(stub, CURRENT)
    with pytest.raises(ExecutionError, match="Nope"):
        conn.self_check("h", packages=["Nope"])
    conn = make_connection(Mock(), (CAPABILITY_FEATURE_VALUES,))
    with pytest.raises(MissingCapabilityError):
        conn.self_check("h")


def test_record_run_builds_the_recording():
    stub = Mock()
    stub.RecordRun.return_value = sysml_pb2.RecordRunResponse(
        run=sysml_pb2.RunAnalysisResponse(outputs=[sysml_pb2.CalcOutput(name="total", value=real(6.0))]),
        package="Records", definition="Records::CostRun", records=["Records::Cost_run1"],
        source="package Records { }", documents=[sysml_pb2.EditedDocument(name="Records.sysml", content="package Records { }")],
        model_hash="h2",
    )
    conn = make_connection(stub, CURRENT)
    rec = conn.record_run("A::Cost", "h", subject="A::p", into="Logs", runs=2, seed=3)
    request = stub.RecordRun.call_args[0][0]
    assert request.analysis_symbol_id == "A::Cost"
    assert request.record_into == "Logs"
    assert request.runs == 2
    assert request.seed == 3
    assert request.accept_documents
    assert isinstance(rec, Recording)
    assert rec.run.outputs == {"total": 6.0}
    assert rec.sweep is None
    assert rec.records == ["Records::Cost_run1"]
    assert rec.documents[0].name == "Records.sysml"
    assert rec.model_hash == "h2"


def test_record_run_error_and_capability():
    stub = Mock()
    stub.RecordRun.return_value = sysml_pb2.RecordRunResponse(error="nothing recorded: no run completed")
    conn = make_connection(stub, CURRENT)
    with pytest.raises(ExecutionError, match="nothing recorded"):
        conn.record_run("A::Cost", "h")
    conn = make_connection(Mock(), (CAPABILITY_FEATURE_VALUES,))
    with pytest.raises(MissingCapabilityError):
        conn.record_run("A::Cost", "h")

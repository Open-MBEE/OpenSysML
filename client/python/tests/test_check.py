"""Tests for checking a behavior over every schedule: the request sent, the
result built from the answer, and the capability the client requires first."""

from unittest.mock import Mock, patch

import pytest

from opensysml.capabilities import (
    CAPABILITY_CHECK,
    CAPABILITY_FEATURE_VALUES,
    CAPABILITY_PERFORMER,
    MissingCapabilityError,
)
from opensysml.check import CheckResult, CheckViolation, CheckWitness
from opensysml.connection import Connection
from opensysml.errors import ExecutionError
from opensysml.proto import sysml_pb2


def make_connection(stub, capabilities):
    stub.GetServerInfo.return_value = sysml_pb2.ServerInfoResponse(
        version="test", capabilities=list(capabilities)
    )
    with patch("grpc.insecure_channel"):
        with patch("opensysml.proto.sysml_pb2_grpc.SysMLServiceStub", return_value=stub):
            return Connection(auto_start=False)


CURRENT = (CAPABILITY_FEATURE_VALUES, CAPABILITY_CHECK, CAPABILITY_PERFORMER)


def violation_response():
    return sysml_pb2.CheckBehaviorResponse(
        verdict="fails",
        lines=["✗ Plant::Tank::fill: violation (11 states, 10 moves, depth 6)",
               "  violation: Plant::Tank::low is false after 3 moves"],
        search=sysml_pb2.CheckSearch(states=11, moves=10, max_depth=6),
        violations=[sysml_pb2.CheckViolation(
            kind="property", property="Plant::Tank::low", depth=3,
            witness=sysml_pb2.CheckWitness(schedule="replay", choices=["step 3: 3@b first of 2@a, 3@b"],
                                           file_name="witness-1.json", file_content="{}\n"))],
        divergences=[sysml_pb2.CheckDivergence(feature="this.level", values=[
            sysml_pb2.CheckDivergentValue(value="1"), sysml_pb2.CheckDivergentValue(value="2")])],
        engine="check", strength="proved",
    )


def test_check_action_sends_every_option_and_builds_the_result():
    stub = Mock()
    stub.CheckBehavior.return_value = violation_response()
    conn = make_connection(stub, CURRENT)

    result = conn.check_action("Plant::Tank::fill", "hash", inputs={"n": 2}, performer="Plant::tank",
                               properties=["Plant::Tank::low"], diverge=["this.level"], witness=True,
                               depth=8, states=100, timeout=2.5, engine="check")

    request = stub.CheckBehavior.call_args[0][0]
    assert request.model_hash == "hash"
    assert request.action_symbol_id == "Plant::Tank::fill"
    assert request.state_machine_symbol_id == ""
    assert request.inputs["n"].int_value == 2
    assert request.performer_symbol_id == "Plant::tank"
    assert list(request.check_property) == ["Plant::Tank::low"]
    assert list(request.check_diverge) == ["this.level"]
    assert request.check_witness is True
    assert (request.check_depth, request.check_states, request.check_unroll) == (8, 100, 0)
    assert request.check_timeout == 2.5
    assert request.engine == "check"

    assert isinstance(result, CheckResult)
    assert result.verdict == "fails"
    assert not result.holds
    assert str(result).startswith("✗ Plant::Tank::fill: violation")
    assert result.search.states == 11
    assert result.search.bounds_hit == ()
    assert result.violations == (CheckViolation(
        kind="property", property="Plant::Tank::low", depth=3,
        witness=CheckWitness(schedule="replay", choices=("step 3: 3@b first of 2@a, 3@b",),
                             file_name="witness-1.json", file_content="{}\n"),
    ),)
    assert result.divergences[0].feature == "this.level"
    assert [v.value for v in result.divergences[0].values] == ["1", "2"]
    assert result.divergences[0].values[0].witness is None
    assert result.standing.engine == "check"
    assert result.standing.strength == "proved"


def test_check_state_machine_names_the_state_machine():
    stub = Mock()
    stub.CheckBehavior.return_value = sysml_pb2.CheckBehaviorResponse(verdict="holds", engine="check")
    conn = make_connection(stub, CURRENT)
    result = conn.check_state_machine("M::machine", "hash", properties=["M::safe"])
    request = stub.CheckBehavior.call_args[0][0]
    assert request.action_symbol_id == ""
    assert request.state_machine_symbol_id == "M::machine"
    assert result.holds
    assert result.search is None
    assert result.violations == ()


def test_an_in_band_error_is_an_execution_error():
    stub = Mock()
    stub.CheckBehavior.return_value = sysml_pb2.CheckBehaviorResponse(
        verdict="undecided", error="behavior not found: Plant::Tank::drain")
    conn = make_connection(stub, CURRENT)
    with pytest.raises(ExecutionError, match="Plant::Tank::drain"):
        conn.check_action("Plant::Tank::drain", "hash")


def test_a_service_without_check_is_refused_before_anything_is_sent():
    stub = Mock()
    conn = make_connection(stub, (CAPABILITY_FEATURE_VALUES,))
    with pytest.raises(MissingCapabilityError, match="check"):
        conn.check_action("Plant::Tank::fill", "hash")
    stub.CheckBehavior.assert_not_called()


def test_a_performer_needs_the_performer_capability():
    stub = Mock()
    conn = make_connection(stub, (CAPABILITY_FEATURE_VALUES, CAPABILITY_CHECK))
    with pytest.raises(MissingCapabilityError, match="performer"):
        conn.check_action("Plant::Tank::fill", "hash", performer="Plant::tank")
    stub.CheckBehavior.assert_not_called()

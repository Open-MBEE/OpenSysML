"""Tests for the analysis engines: listing them, selecting one, and the standing
every answer carries.

A service that predates ``engines`` drops the ``engine`` field as unknown and
answers under ``auto``, which cannot be told from the engine asked for, so the
client requires the capability before sending a selection. An unset selection
is ``auto`` and needs nothing.
"""

from unittest.mock import Mock, patch

import pytest

from opensysml.capabilities import (
    CAPABILITY_ENGINES,
    CAPABILITY_FEATURE_VALUES,
    CAPABILITY_SCHEDULE_EXPLORE,
    CAPABILITY_VERIFICATION,
    MissingCapabilityError,
)
from opensysml.connection import Connection
from opensysml.engines import Bound, EngineInfo, Standing
from opensysml.proto import sysml_pb2


def make_connection(stub, capabilities):
    """Build a Connection over a mock stub reporting ``capabilities``."""
    stub.GetServerInfo.return_value = sysml_pb2.ServerInfoResponse(
        version="test", capabilities=list(capabilities)
    )
    with patch("grpc.insecure_channel"):
        with patch(
            "opensysml.proto.sysml_pb2_grpc.SysMLServiceStub", return_value=stub
        ):
            return Connection(auto_start=False)


CURRENT = (CAPABILITY_FEATURE_VALUES, CAPABILITY_VERIFICATION, CAPABILITY_ENGINES)
OLD = (CAPABILITY_FEATURE_VALUES, CAPABILITY_VERIFICATION)

STANDING = dict(
    engine="run",
    strength="observed",
    bounds=[sysml_pb2.Bound(name="steps", limit=10_000_000)],
)


def stub_answering():
    """A stub answering every question with a standing of one observed run."""
    stub = Mock()
    stub.VerifyConstraint.return_value = sysml_pb2.VerifyConstraintResponse(
        verdict=sysml_pb2.Verdict(kind="constraint", element_id="M::c", holds=True, **STANDING)
    )
    stub.VerifyRequirement.return_value = sysml_pb2.VerifyRequirementResponse(
        verdict=sysml_pb2.Verdict(kind="requirement", element_id="M::r", holds=False, **STANDING)
    )
    stub.VerifySatisfaction.return_value = sysml_pb2.VerifySatisfactionResponse(
        verdicts=[sysml_pb2.Verdict(kind="satisfy", element="satisfy r by p", holds=True, **STANDING)]
    )
    stub.EvaluateCalc.return_value = sysml_pb2.EvaluateCalcResponse(
        result=sysml_pb2.Value(int_value=3), **STANDING
    )
    stub.RunAnalysis.return_value = sysml_pb2.RunAnalysisResponse(**STANDING)
    stub.RunSweep.return_value = sysml_pb2.RunSweepResponse(
        parameters=["n"], engine="sweep", strength="observed"
    )
    return stub


def ask_everything(conn, **kwargs):
    """Every question the engine field is carried on, with the arguments given."""
    return [
        conn.verify_constraint("M::c", "hash", **kwargs),
        conn.verify_requirement("M::r", "hash", **kwargs),
        conn.verify_satisfaction("hash", **kwargs)[0],
        conn.calc("M::k", "hash", arguments=[1], **kwargs),
        conn.run_analysis("M::a", "hash", **kwargs),
        conn.run_sweep("M::k", "hash", {"n": (1, 2)}, **kwargs),
    ]


def test_list_engines_reads_every_engine():
    stub = Mock()
    stub.ListEngines.return_value = sysml_pb2.ListEnginesResponse(engines=[
        sysml_pb2.EngineInfo(name="run", authority="observed", answers=["evaluate"], ready=True),
        sysml_pb2.EngineInfo(
            name="solve", authority="proved", answers=["satisfiable"], process="z3",
            ready=False, unavailable="z3 not found",
        ),
    ])
    conn = make_connection(stub, CURRENT)

    engines = conn.list_engines()
    assert engines == [
        EngineInfo(name="run", authority="observed", answers=("evaluate",)),
        EngineInfo(
            name="solve", authority="proved", answers=("satisfiable",), process="z3",
            ready=False, unavailable="z3 not found",
        ),
    ]
    assert str(engines[0]) == "run: observed, answers evaluate; ready"
    assert str(engines[1]) == "solve: proved, answers satisfiable; unavailable: z3 not found"


def test_list_engines_needs_the_capability():
    stub = Mock()
    conn = make_connection(stub, OLD)
    with pytest.raises(MissingCapabilityError) as excinfo:
        conn.list_engines()
    assert excinfo.value.capability == CAPABILITY_ENGINES
    stub.ListEngines.assert_not_called()


def test_an_engine_is_carried_on_every_question():
    stub = stub_answering()
    conn = make_connection(stub, CURRENT)
    ask_everything(conn, engine="all")
    for call in (
        stub.VerifyConstraint, stub.VerifyRequirement, stub.VerifySatisfaction,
        stub.EvaluateCalc, stub.RunAnalysis, stub.RunSweep,
    ):
        assert call.call_args.args[0].engine == "all"


def test_auto_and_no_engine_send_an_empty_field():
    """An unset selection works against a service that predates engines."""
    stub = stub_answering()
    conn = make_connection(stub, OLD)
    ask_everything(conn)
    ask_everything(conn, engine="auto")
    for call in (stub.VerifyConstraint, stub.EvaluateCalc, stub.RunAnalysis, stub.RunSweep):
        assert all(c.args[0].engine == "" for c in call.call_args_list)


def test_an_engine_is_not_sent_to_a_service_without_the_capability():
    """An older service would answer under auto instead, so nothing is sent."""
    stub = stub_answering()
    conn = make_connection(stub, OLD)
    for ask in (
        lambda: conn.verify_constraint("M::c", "hash", engine="run"),
        lambda: conn.verify_requirement("M::r", "hash", engine="run"),
        lambda: conn.verify_satisfaction("hash", engine="run"),
        lambda: conn.calc("M::k", "hash", engine="run"),
        lambda: conn.run_analysis("M::a", "hash", engine="run"),
        lambda: conn.run_sweep("M::k", "hash", {"n": (1, 2)}, engine="run"),
    ):
        with pytest.raises(MissingCapabilityError) as excinfo:
            ask()
        assert excinfo.value.capability == CAPABILITY_ENGINES
    for call in (
        stub.VerifyConstraint, stub.VerifyRequirement, stub.VerifySatisfaction,
        stub.EvaluateCalc, stub.RunAnalysis, stub.RunSweep,
    ):
        call.assert_not_called()


def test_the_explore_engine_is_the_exploring_schedule():
    """engine='explore' answers with outcomes, so run_analysis refuses it as it does the schedule."""
    stub = stub_answering()
    conn = make_connection(stub, CURRENT)
    with pytest.raises(ValueError, match="explore_analysis"):
        conn.run_analysis("M::a", "hash", engine="explore")
    stub.RunAnalysis.assert_not_called()
    with pytest.raises(MissingCapabilityError) as excinfo:
        conn.verify_constraint("M::c", "hash", engine="explore")
    assert excinfo.value.capability == CAPABILITY_SCHEDULE_EXPLORE


def test_every_answer_carries_its_standing():
    stub = stub_answering()
    conn = make_connection(stub, CURRENT)
    constraint, requirement, satisfy, calc, analysis, sweep = ask_everything(conn, engine="run")

    want = Standing(engine="run", strength="observed", bounds=(Bound("steps", 10_000_000),))
    for answer in (constraint, requirement, satisfy, calc, analysis):
        assert answer.standing == want
        assert (answer.engine, answer.strength, answer.bounds) == ("run", "observed", [Bound("steps", 10_000_000)])
    assert sweep.standing == Standing(engine="sweep", strength="observed")
    assert str(want) == "observed by run"
    assert constraint.explain() == "\u2713 constraint M::c holds \u2014 observed by run"
    assert requirement.explain() == (
        "\u2717 requirement M::r fails: condition evaluated to false \u2014 observed by run"
    )


def test_a_reached_bound_is_named_in_the_standing():
    standing = Standing.of(sysml_pb2.RunAnalysisResponse(
        engine="explore", strength="bounded",
        bounds=[
            sysml_pb2.Bound(name="runs", limit=64, reached=True),
            sysml_pb2.Bound(name="depth", limit=8),
        ],
    ))
    assert standing.reached == [Bound("runs", 64, reached=True)]
    assert str(standing) == "bounded by explore (runs 64 reached)"


def test_an_old_service_reports_no_standing():
    verdict = sysml_pb2.Verdict(kind="constraint", element_id="M::c", holds=True)
    stub = Mock()
    stub.VerifyConstraint.return_value = sysml_pb2.VerifyConstraintResponse(verdict=verdict)
    conn = make_connection(stub, OLD)

    answer = conn.verify_constraint("M::c", "hash")
    assert not answer.standing.reported
    assert (answer.engine, answer.strength, answer.bounds) == ("", "", [])
    assert answer.explain() == "\u2713 constraint M::c holds"

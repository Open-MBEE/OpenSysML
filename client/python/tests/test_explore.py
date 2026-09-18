"""Tests for exploring a run under the ``explore`` scheduling policy.

An exploring run answers with every distinct outcome and an exploration status
rather than one run's result, so it has methods of its own: ``explore_action``,
``explore_state`` and ``explore_analysis``. The single-run methods refuse an
exploring schedule rather than reading an empty result off a response whose
answer is in ``outcomes``. A service that predates ``schedule_explore`` refuses
the schedule, so the client requires the capability before sending.
"""

from unittest.mock import Mock, patch

import pytest

from opensysml.capabilities import (
    CAPABILITY_FEATURE_VALUES,
    CAPABILITY_SCHEDULE,
    CAPABILITY_SCHEDULE_EXPLORE,
    CAPABILITY_VERIFICATION,
    MissingCapabilityError,
)
from opensysml.connection import Connection
from opensysml.errors import ExecutionError, InvalidRequestError, WrongKindError
from opensysml.exploration import Exploration, Outcome
from opensysml.proto import sysml_pb2
from tests.service_gate import skip_or_fail_without_service


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


CURRENT = (
    CAPABILITY_FEATURE_VALUES,
    CAPABILITY_VERIFICATION,
    CAPABILITY_SCHEDULE,
    CAPABILITY_SCHEDULE_EXPLORE,
)
SCHEDULE_ONLY = (CAPABILITY_FEATURE_VALUES, CAPABILITY_VERIFICATION, CAPABILITY_SCHEDULE)


def outcome_pb(winner, linearizations, witness, error=""):
    return sysml_pb2.Outcome(
        outputs={"winner": sysml_pb2.Value(int_value=winner)} if not error else {},
        linearizations=linearizations,
        witness=witness,
        error=error,
    )


def test_an_explored_action_answers_with_every_outcome_and_the_status():
    stub = Mock()
    stub.ExecuteAction.return_value = sysml_pb2.ExecuteActionResponse(
        outcomes=[
            outcome_pb(1, 3, ["step 3: 3@left first of 2@right, 3@left"]),
            outcome_pb(2, 3, ["step 3: 2@right first of 2@right, 3@left"]),
        ],
        exploration=sysml_pb2.ExplorationStatus(
            complete=True, runs=2, runs_budget=1024, depth_budget=64
        ),
    )
    conn = make_connection(stub, CURRENT)

    exploration = conn.explore_action("M::race", "hash", schedule="explore:runs=8")

    assert stub.ExecuteAction.call_args.args[0].schedule == "explore:runs=8"
    assert isinstance(exploration, Exploration)
    assert [o.outputs["winner"] for o in exploration] == [1, 2]
    assert [o.linearizations for o in exploration] == [3, 3]
    assert exploration.outcomes[0].witness == ["step 3: 3@left first of 2@right, 3@left"]
    assert exploration.complete
    assert exploration.status == "complete (2 runs)"
    assert bool(exploration)
    assert str(exploration).splitlines()[-1] == "complete (2 runs)"


def test_a_budget_hit_is_incomplete_and_named_never_an_error():
    stub = Mock()
    stub.ExecuteState.return_value = sysml_pb2.ExecuteStateResponse(
        outcomes=[
            sysml_pb2.Outcome(
                final_state="low", states_visited=["idle", "low"], linearizations=1,
                witness=["Go from idle: low first of low, high"],
            )
        ],
        exploration=sysml_pb2.ExplorationStatus(
            complete=False, runs=1, budgets_hit=["runs"], runs_budget=1, depth_budget=64
        ),
    )
    conn = make_connection(stub, CURRENT)

    exploration = conn.explore_state(
        "M::Dispatcher", "hash", events=["Go"], schedule="explore:runs=1"
    )

    assert not exploration.complete
    assert exploration.budgets_hit == ["runs"]
    assert exploration.status == "incomplete: runs budget 1 hit after 1 runs"
    assert not bool(exploration)
    outcome = exploration.outcomes[0]
    assert outcome.final_state == "low"
    assert outcome.states_visited == ["idle", "low"]
    assert str(outcome) == "finalState low; visits idle, low"
    with pytest.raises(ExecutionError, match="runs budget 1 hit"):
        exploration.raise_for_incomplete()


def test_a_failed_run_is_an_outcome_of_its_own():
    stub = Mock()
    stub.ExecuteAction.return_value = sysml_pb2.ExecuteActionResponse(
        outcomes=[
            outcome_pb(10, 1, ["step 2: 2@safe first of 2@safe, 3@risky"]),
            outcome_pb(0, 1, ["step 2: 3@risky first of 2@safe, 3@risky"],
                       error="action execution failed: division by zero"),
        ],
        exploration=sysml_pb2.ExplorationStatus(complete=True, runs=2),
    )
    conn = make_connection(stub, CURRENT)

    exploration = conn.explore_action("M::divide", "hash")

    assert exploration.complete
    assert not bool(exploration)
    failed = exploration.outcomes[1]
    assert failed.failed
    assert failed.outputs == {}
    assert str(failed) == "error: action execution failed: division by zero"
    with pytest.raises(ExecutionError, match="division by zero"):
        failed.raise_for_error()
    exploration.outcomes[0].raise_for_error()


def test_an_explored_analysis_reads_its_verdicts_among_the_outputs():
    stub = Mock()
    stub.RunAnalysis.return_value = sysml_pb2.RunAnalysisResponse(
        outcomes=[
            sysml_pb2.Outcome(
                outputs={
                    "winner": sysml_pb2.Value(int_value=2),
                    "objective obj": sysml_pb2.Value(string_value="satisfied"),
                },
                linearizations=1,
            )
        ],
        exploration=sysml_pb2.ExplorationStatus(complete=True, runs=1),
    )
    conn = make_connection(stub, CURRENT)

    exploration = conn.explore_analysis("M::raced", "hash", arguments=[3])

    request = stub.RunAnalysis.call_args.args[0]
    assert request.schedule == "explore"
    assert request.arguments[0].int_value == 3
    assert exploration.outcomes[0].outputs == {"winner": 2, "objective obj": "satisfied"}
    assert exploration.outcomes[0].witness == []
    assert exploration.status == "complete (1 runs)"


def test_a_failure_to_explore_at_all_is_raised():
    stub = Mock()
    stub.ExecuteAction.return_value = sysml_pb2.ExecuteActionResponse(
        error="action not found: M::missing"
    )
    stub.RunAnalysis.return_value = sysml_pb2.RunAnalysisResponse(
        error="M::part is not an analysis case",
        failure_reason=sysml_pb2.FAILURE_REASON_WRONG_KIND,
    )
    conn = make_connection(stub, CURRENT)

    with pytest.raises(ExecutionError, match="not found"):
        conn.explore_action("M::missing", "hash")
    with pytest.raises(WrongKindError):
        conn.explore_analysis("M::part", "hash")


def test_the_single_run_methods_refuse_an_exploring_schedule():
    stub = Mock()
    conn = make_connection(stub, CURRENT)

    with pytest.raises(ValueError, match="explore_action"):
        conn.execute_action("M::race", "hash", schedule="explore")
    with pytest.raises(ValueError, match="explore_state"):
        conn.execute_state("M::Dispatcher", "hash", schedule="explore:depth=2")
    with pytest.raises(ValueError, match="explore_analysis"):
        conn.run_analysis("M::raced", "hash", schedule="explore:runs=4,depth=2")
    stub.ExecuteAction.assert_not_called()
    stub.ExecuteState.assert_not_called()
    stub.RunAnalysis.assert_not_called()


def test_the_exploring_methods_refuse_a_single_run_schedule():
    stub = Mock()
    conn = make_connection(stub, CURRENT)

    for spelling in ("declared", "reverse", "seed:3", "", None):
        with pytest.raises(ValueError, match="explore"):
            conn.explore_action("M::race", "hash", schedule=spelling)
        with pytest.raises(ValueError, match="explore"):
            conn.explore_state("M::Dispatcher", "hash", schedule=spelling)
        with pytest.raises(ValueError, match="explore"):
            conn.explore_analysis("M::raced", "hash", schedule=spelling)
    stub.ExecuteAction.assert_not_called()
    stub.ExecuteState.assert_not_called()
    stub.RunAnalysis.assert_not_called()


def test_an_exploring_schedule_is_not_sent_to_a_service_without_the_capability():
    """A service with schedule but not schedule_explore would refuse it, so nothing is sent."""
    stub = Mock()
    conn = make_connection(stub, SCHEDULE_ONLY)

    with pytest.raises(MissingCapabilityError) as excinfo:
        conn.explore_action("M::race", "hash")
    assert excinfo.value.capability == CAPABILITY_SCHEDULE_EXPLORE
    with pytest.raises(MissingCapabilityError) as excinfo:
        conn.explore_state("M::Dispatcher", "hash", events=["Go"])
    assert excinfo.value.capability == CAPABILITY_SCHEDULE_EXPLORE
    with pytest.raises(MissingCapabilityError) as excinfo:
        conn.explore_analysis("M::raced", "hash")
    assert excinfo.value.capability == CAPABILITY_SCHEDULE_EXPLORE
    stub.ExecuteAction.assert_not_called()
    stub.ExecuteState.assert_not_called()
    stub.RunAnalysis.assert_not_called()


def test_a_single_run_schedule_still_needs_only_schedule():
    stub = Mock()
    stub.ExecuteAction.return_value = sysml_pb2.ExecuteActionResponse()
    conn = make_connection(stub, SCHEDULE_ONLY)

    conn.execute_action("M::race", "hash", schedule="declared")
    assert stub.ExecuteAction.call_args.args[0].schedule == "declared"


def test_an_outcome_renders_its_observables_sorted():
    outcome = Outcome(
        {"b": 2, "a": 1}, final_state="", states_visited=[], error="",
        linearizations=4, witness=["x"], diagnostics=[],
    )
    assert str(outcome) == "a = 1; b = 2"
    assert "linearizations=4" in repr(outcome)
    empty = Outcome({}, "", [], "", 1, [], [])
    assert str(empty) == "no outputs"


EXPLORE_MODEL = """
package Sched {
    private import ScalarValues::*;

    action race {
        attribute winner : Integer = 0;
        first start;
        fork split;
        action left { assign winner := 1; }
        action right { assign winner := 2; }
        join sync;
        done;
        succession first start then split;
        succession first split then left;
        succession first split then right;
        succession first left then sync;
        succession first right then sync;
        succession first sync then done;
    }

    state Dispatcher {
        attribute level : Integer = 8;
        entry; then idle;
        state idle;
        state low;
        state high;
        transition first idle accept Go if level > 5 then low;
        transition first idle accept Go if level > 7 then high;
    }

    action def Race {
        out winner : Integer = 0;
        first start;
        fork split;
        action left { assign winner := 1; }
        action right { assign winner := 2; }
        join sync;
        done;
        succession first start then split;
        succession first split then left;
        succession first split then right;
        succession first left then sync;
        succession first right then sync;
        succession first sync then done;
    }

    analysis raced {
        out winner : Integer;
        perform action race : Race;
        return : Integer = winner;
    }
}
"""


@pytest.mark.integration
class TestExploreAgainstTheService:
    """What a caller actually gets back from the real service for a real model."""

    def setup_method(self):
        import grpc

        try:
            self.conn = Connection(auto_start=False)
            self.conn._stub.GetDiagnostics(sysml_pb2.DiagnosticsRequest(model_hash=""))
        except grpc.RpcError as exc:
            if exc.code() != grpc.StatusCode.NOT_FOUND:
                self.conn = None
                skip_or_fail_without_service(
                    f"the sysml-grpc service on localhost:50051 answered {exc.code()}"
                )
        except Exception as exc:
            self.conn = None
            skip_or_fail_without_service(
                f"no sysml-grpc service could be reached on localhost:50051 ({exc})"
            )
        self.model = self.conn.load_from_content(EXPLORE_MODEL)

    def teardown_method(self):
        conn = self.__dict__.get("conn")
        if conn is not None:
            conn.close()

    def test_the_service_advertises_schedule_explore(self):
        assert self.conn.server_info().has(CAPABILITY_SCHEDULE_EXPLORE)

    def test_both_writers_are_reached_and_the_exploration_is_complete(self):
        exploration = self.model.explore_action("Sched::race")
        assert sorted(o.outputs["winner"] for o in exploration) == [1, 2]
        assert all(o.linearizations == 1 for o in exploration)
        assert all(o.witness for o in exploration)
        assert exploration.status == "complete (2 runs)"

    def test_both_transitions_are_reached(self):
        exploration = self.model.explore_state("Sched::Dispatcher", events=["Go"])
        assert sorted(o.final_state for o in exploration) == ["high", "low"]
        assert exploration.complete

    def test_an_analysis_explores_the_actions_it_performs(self):
        exploration = self.model.explore_analysis("Sched::raced")
        assert sorted(o.outputs["winner"] for o in exploration) == [1, 2]
        assert exploration.complete

    def test_a_runs_budget_of_one_is_incomplete(self):
        exploration = self.model.explore_action("Sched::race", schedule="explore:runs=1")
        assert len(exploration) == 1
        assert exploration.status == "incomplete: runs budget 1 hit after 1 runs"

    def test_the_same_model_explores_to_the_same_table(self):
        first = str(self.model.explore_action("Sched::race"))
        assert all(
            str(self.model.explore_action("Sched::race")) == first for _ in range(3)
        )

    def test_malformed_options_are_an_invalid_request(self):
        for spelling in ("explore:", "explore:runs=0", "explore:depth=-1", "explore:runs=x"):
            with pytest.raises(InvalidRequestError):
                self.model.explore_action("Sched::race", schedule=spelling)

"""Tests for the scheduling policy of an action, state or analysis run.

A service that predates ``schedule`` drops the field as unknown and runs under
the default policy, which cannot be told from the policy asked for — so the
client requires the capability rather than sending the request and trusting
the answer. A spelling naming no policy is the service's to refuse, as an
INVALID_ARGUMENT this client reports as an :class:`InvalidRequestError`.
"""

from unittest.mock import Mock, patch

import pytest

from opensysml.capabilities import (
    CAPABILITY_FEATURE_VALUES,
    CAPABILITY_SCHEDULE,
    CAPABILITY_VERIFICATION,
    MissingCapabilityError,
)
from opensysml.connection import Connection
from opensysml.errors import InvalidRequestError
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


CURRENT = (CAPABILITY_FEATURE_VALUES, CAPABILITY_VERIFICATION, CAPABILITY_SCHEDULE)
OLD = (CAPABILITY_FEATURE_VALUES, CAPABILITY_VERIFICATION)


def test_a_schedule_is_carried_on_every_run_request():
    stub = Mock()
    stub.ExecuteAction.return_value = sysml_pb2.ExecuteActionResponse()
    stub.ExecuteState.return_value = sysml_pb2.ExecuteStateResponse()
    stub.RunAnalysis.return_value = sysml_pb2.RunAnalysisResponse()
    conn = make_connection(stub, CURRENT)

    conn.execute_action("M::race", "hash", schedule="declared")
    assert stub.ExecuteAction.call_args.args[0].schedule == "declared"
    conn.execute_state("M::Machine", "hash", events=["go"], schedule="seed:7")
    assert stub.ExecuteState.call_args.args[0].schedule == "seed:7"
    conn.run_analysis("M::raced", "hash", schedule="reverse")
    assert stub.RunAnalysis.call_args.args[0].schedule == "reverse"


def test_no_schedule_asks_for_nothing():
    """A run under the default keeps working against a service that predates schedule."""
    stub = Mock()
    stub.ExecuteAction.return_value = sysml_pb2.ExecuteActionResponse()
    stub.ExecuteState.return_value = sysml_pb2.ExecuteStateResponse()
    conn = make_connection(stub, OLD)

    conn.execute_action("M::race", "hash")
    assert stub.ExecuteAction.call_args.args[0].schedule == ""
    conn.execute_state("M::Machine", "hash")
    assert stub.ExecuteState.call_args.args[0].schedule == ""


def test_a_schedule_is_not_sent_to_a_service_without_the_capability():
    """An older service would run under the default instead, so nothing is sent."""
    stub = Mock()
    conn = make_connection(stub, OLD)

    with pytest.raises(MissingCapabilityError) as excinfo:
        conn.execute_action("M::race", "hash", schedule="declared")
    assert excinfo.value.capability == CAPABILITY_SCHEDULE
    with pytest.raises(MissingCapabilityError) as excinfo:
        conn.execute_state("M::Machine", "hash", schedule="declared")
    assert excinfo.value.capability == CAPABILITY_SCHEDULE
    with pytest.raises(MissingCapabilityError) as excinfo:
        conn.run_analysis("M::raced", "hash", schedule="declared")
    assert excinfo.value.capability == CAPABILITY_SCHEDULE
    stub.ExecuteAction.assert_not_called()
    stub.ExecuteState.assert_not_called()
    stub.RunAnalysis.assert_not_called()


SCHEDULE_MODEL = """
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
class TestScheduleAgainstTheService:
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
        self.model = self.conn.load_from_content(SCHEDULE_MODEL)

    def teardown_method(self):
        conn = self.__dict__.get("conn")
        if conn is not None:
            conn.close()

    def test_the_service_advertises_schedule(self):
        assert self.conn.server_info().has(CAPABILITY_SCHEDULE)

    def test_the_policy_selects_which_write_stands(self):
        """The default runs the later-declared branch first; declared the earlier one."""
        assert self.model.execute_action("Sched::race")["winner"] == 1
        assert self.model.execute_action("Sched::race", schedule="reverse")["winner"] == 1
        assert self.model.execute_action("Sched::race", schedule="declared")["winner"] == 2

    def test_the_policy_selects_the_transition_taken(self):
        declared = self.model.execute_state("Sched::Dispatcher", events=["Go"], schedule="declared")
        assert declared["states_visited"] == ["idle", "low"]
        seeded = self.model.execute_state("Sched::Dispatcher", events=["Go"], schedule="seed:3")
        assert seeded["states_visited"] == ["idle", "high"]

    def test_an_analysis_performs_its_actions_under_the_policy(self):
        result = self.model.run_analysis("Sched::raced", schedule="declared")
        assert result.outputs["winner"] == 2

    def test_the_same_seed_answers_the_same_way(self):
        answers = {
            self.model.execute_action("Sched::race", schedule="seed:11")["winner"]
            for _ in range(3)
        }
        assert len(answers) == 1

    def test_a_spelling_naming_no_policy_is_an_invalid_request(self):
        for spelling in ("random", "seed", "seed:", "seed:-1", "seed:abc"):
            with pytest.raises(InvalidRequestError, match="scheduling policy"):
                self.model.execute_action("Sched::race", schedule=spelling)
            with pytest.raises(InvalidRequestError, match="scheduling policy"):
                self.model.execute_state("Sched::Dispatcher", schedule=spelling)
            with pytest.raises(InvalidRequestError, match="scheduling policy"):
                self.model.run_analysis("Sched::raced", schedule=spelling)

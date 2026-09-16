"""Tests for the object an action or state machine runs on (``performer=``).

A performer names a part definition or usage to make an object of, or a path
from one into its parts: ``Wire::pair.craft`` makes the pair and runs the
behavior on its craft, inside the assembly, so what the ground station sends
over their connector reaches it. A service that predates ``performer`` would
drop the field and run the behavior outside any object, so the client requires
the capability before sending.
"""

from unittest.mock import Mock, patch

import pytest

from opensysml.capabilities import (
    CAPABILITY_FEATURE_VALUES,
    CAPABILITY_PERFORMER,
    CAPABILITY_SCHEDULE,
    CAPABILITY_SCHEDULE_EXPLORE,
    CAPABILITY_VERIFICATION,
    MissingCapabilityError,
)
from opensysml.connection import Connection
from opensysml.errors import ExecutionError
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


EXPLORING = (
    CAPABILITY_FEATURE_VALUES,
    CAPABILITY_VERIFICATION,
    CAPABILITY_SCHEDULE,
    CAPABILITY_SCHEDULE_EXPLORE,
)
CURRENT = EXPLORING + (CAPABILITY_PERFORMER,)


def test_a_performer_is_carried_on_every_run_request():
    stub = Mock()
    stub.ExecuteAction.return_value = sysml_pb2.ExecuteActionResponse()
    stub.ExecuteState.return_value = sysml_pb2.ExecuteStateResponse()
    conn = make_connection(stub, CURRENT)

    conn.execute_action("Wire::Craft::look", "hash", performer="Wire::pair.craft")
    assert stub.ExecuteAction.call_args.args[0].performer_symbol_id == "Wire::pair.craft"
    conn.explore_action("Wire::Craft::look", "hash", performer="Wire::pair.spares[1]")
    assert stub.ExecuteAction.call_args.args[0].performer_symbol_id == "Wire::pair.spares[1]"
    conn.execute_state("Wire::Craft::modes", "hash", performer="Wire::Craft")
    assert stub.ExecuteState.call_args.args[0].performer_symbol_id == "Wire::Craft"
    conn.explore_state("Wire::Craft::modes", "hash", performer="Wire::pair.craft")
    assert stub.ExecuteState.call_args.args[0].performer_symbol_id == "Wire::pair.craft"


def test_no_performer_asks_for_nothing():
    """A run outside any object keeps working against a service that predates performer."""
    stub = Mock()
    stub.ExecuteAction.return_value = sysml_pb2.ExecuteActionResponse()
    stub.ExecuteState.return_value = sysml_pb2.ExecuteStateResponse()
    conn = make_connection(stub, EXPLORING)

    conn.execute_action("Wire::Craft::look", "hash")
    assert stub.ExecuteAction.call_args.args[0].performer_symbol_id == ""
    conn.explore_state("Wire::Craft::modes", "hash")
    assert stub.ExecuteState.call_args.args[0].performer_symbol_id == ""


def test_a_performer_is_not_sent_to_a_service_without_the_capability():
    """An older service would run the behavior outside any object, so nothing is sent."""
    stub = Mock()
    conn = make_connection(stub, EXPLORING)

    for call in (
        lambda: conn.execute_action("Wire::Craft::look", "hash", performer="Wire::pair.craft"),
        lambda: conn.explore_action("Wire::Craft::look", "hash", performer="Wire::pair.craft"),
        lambda: conn.execute_state("Wire::Craft::modes", "hash", performer="Wire::pair.craft"),
        lambda: conn.explore_state("Wire::Craft::modes", "hash", performer="Wire::pair.craft"),
    ):
        with pytest.raises(MissingCapabilityError) as excinfo:
            call()
        assert excinfo.value.capability == CAPABILITY_PERFORMER
    stub.ExecuteAction.assert_not_called()
    stub.ExecuteState.assert_not_called()


PERFORMER_MODEL = """
package Wire {
    private import ScalarValues::*;
    item def Ping;
    port def Link { in item ping : Ping; }
    part def Ground {
        port p : ~Link;
        exhibit state hail { entry; then go; state go { entry send new Ping() via p; } }
    }
    part def Craft {
        port p : Link;
        attribute pinged : Boolean = false;
        exhibit state modes {
            entry; then waiting;
            state waiting;
            transition first waiting accept Ping via p then active;
            state active { entry assign pinged := true; }
        }
        action look { out seen : Boolean; first start; then action read assign seen := pinged; then done; }
    }
    part def Pair {
        part ground : Ground;
        part craft : Craft;
        connect craft.p to ground.p;
    }
    part pair : Pair;
}
"""


@pytest.mark.integration
class TestPerformerAgainstTheService:
    """What a caller actually gets back from the real service for a nested performer."""

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
        self.model = self.conn.load_from_content(PERFORMER_MODEL)

    def teardown_method(self):
        conn = self.__dict__.get("conn")
        if conn is not None:
            conn.close()

    def test_the_service_advertises_performer(self):
        assert self.conn.server_info().has(CAPABILITY_PERFORMER)

    def test_the_machine_of_a_nested_part_hears_its_sibling(self):
        run = self.model.execute_state("Wire::Craft::modes", performer="Wire::pair.craft")
        assert run["states_visited"][-1] == "active"
        alone = self.model.execute_state("Wire::Craft::modes", performer="Wire::Craft")
        assert alone["states_visited"][-1] == "waiting"

    def test_an_explored_run_makes_the_assembly_anew(self):
        exploration = self.model.explore_state("Wire::Craft::modes", performer="Wire::pair.craft")
        assert [o.final_state for o in exploration] == ["active"]
        assert exploration.status == "complete (1 runs)"

    def test_an_action_reads_what_the_nested_part_holds(self):
        outputs = self.model.execute_action("Wire::Craft::look", performer="Wire::pair.craft")
        assert outputs["seen"] is True

    def test_a_path_to_no_feature_is_refused(self):
        with pytest.raises(ExecutionError, match='Wire::pair has no feature "tug"'):
            self.model.execute_state("Wire::Craft::modes", performer="Wire::pair.tug")

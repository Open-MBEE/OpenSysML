"""Tests for the arguments a verification binds to a requirement's or constraint's ``in`` parameters.

The unit half checks the client sends the bindings as values and refuses to
send them to a service without ``verification_arguments``. The service half
checks a real service binds them, so its tests need a service.
"""

from unittest.mock import Mock, patch

import pytest

from opensysml.capabilities import (
    CAPABILITY_VERIFICATION,
    CAPABILITY_VERIFICATION_ARGUMENTS,
    MissingCapabilityError,
)
from opensysml.connection import Connection
from opensysml.errors import InvalidRequestError
from opensysml.proto import sysml_pb2
from tests.service_gate import fail_if_service_promised, is_server_available

MODEL_SOURCE = '''
package Demo {
    private import ScalarValues::*;
    part def Thing { attribute v : Integer = 2; }
    part t : Thing;
    requirement def Under {
        subject s : Thing;
        in limit : Integer;
        in slack : Integer = 0;
        require constraint { s.v + slack < limit }
    }
    constraint def Between {
        in low : Integer;
        in high : Integer = 10;
        low < high
    }
}
'''


def make_connection(stub, capabilities=None):
    """Build a Connection over a mock stub reporting the given capabilities."""
    stub.GetServerInfo.return_value = sysml_pb2.ServerInfoResponse(
        version="test",
        capabilities=list(capabilities or [CAPABILITY_VERIFICATION]),
    )
    with patch('grpc.insecure_channel'):
        with patch(
            'opensysml.proto.sysml_pb2_grpc.SysMLServiceStub',
            return_value=stub,
        ):
            return Connection(auto_start=False)


def holding(kind):
    return sysml_pb2.Verdict(kind=kind, element_id="Demo::x", holds=True, status="holds")


def test_requirement_arguments_are_sent_as_values():
    stub = Mock()
    stub.VerifyRequirement.return_value = sysml_pb2.VerifyRequirementResponse(
        verdict=holding("requirement"),
    )
    conn = make_connection(
        stub, capabilities=[CAPABILITY_VERIFICATION, CAPABILITY_VERIFICATION_ARGUMENTS],
    )
    verdict = conn.verify_requirement(
        "Demo::Under", "hash1", subject_symbol_id="Demo::t",
        arguments=[5], named_arguments={"slack": 1},
    )
    request = stub.VerifyRequirement.call_args[0][0]
    assert request.subject_symbol_id == "Demo::t"
    assert [arg.int_value for arg in request.arguments] == [5]
    assert request.named_arguments["slack"].int_value == 1
    assert verdict.holds


def test_constraint_arguments_are_sent_as_values():
    stub = Mock()
    stub.VerifyConstraint.return_value = sysml_pb2.VerifyConstraintResponse(
        verdict=holding("constraint"),
    )
    conn = make_connection(
        stub, capabilities=[CAPABILITY_VERIFICATION, CAPABILITY_VERIFICATION_ARGUMENTS],
    )
    conn.verify_constraint("Demo::Between", "hash1", named_arguments={"low": 3, "high": 4.5})
    request = stub.VerifyConstraint.call_args[0][0]
    assert request.named_arguments["low"].int_value == 3
    assert request.named_arguments["high"].real_value == 4.5
    assert list(request.arguments) == []


def test_omitted_arguments_send_none_and_need_no_capability():
    stub = Mock()
    stub.VerifyRequirement.return_value = sysml_pb2.VerifyRequirementResponse(
        verdict=holding("requirement"),
    )
    conn = make_connection(stub, capabilities=[CAPABILITY_VERIFICATION])
    conn.verify_requirement("Demo::Under", "hash1", subject_symbol_id="Demo::t")
    request = stub.VerifyRequirement.call_args[0][0]
    assert list(request.arguments) == []
    assert dict(request.named_arguments) == {}


@pytest.mark.parametrize("kwargs", [
    {"arguments": [5]},
    {"named_arguments": {"limit": 5}},
])
def test_arguments_are_refused_before_sending_without_the_capability(kwargs):
    stub = Mock()
    conn = make_connection(stub, capabilities=[CAPABILITY_VERIFICATION])
    with pytest.raises(MissingCapabilityError) as refused:
        conn.verify_requirement("Demo::Under", "hash1", **kwargs)
    assert CAPABILITY_VERIFICATION_ARGUMENTS in str(refused.value)
    stub.VerifyRequirement.assert_not_called()
    with pytest.raises(MissingCapabilityError):
        conn.verify_constraint("Demo::Between", "hash1", **kwargs)
    stub.VerifyConstraint.assert_not_called()


_AVAILABLE = is_server_available()
fail_if_service_promised(_AVAILABLE)


@pytest.mark.integration
@pytest.mark.skipif(not _AVAILABLE, reason="sysml-grpc server not running")
class TestVerificationArguments:
    """Argument bindings against a real service."""

    def setup_method(self):
        self.conn = Connection(auto_start=False)
        self.model = self.conn.load_from_content(MODEL_SOURCE)

    def teardown_method(self):
        self.conn.close()

    def test_requirement_binds_its_subject_and_named_arguments(self):
        verdict = self.model.verify_requirement(
            "Demo::Under", subject="Demo::t", named_arguments={"limit": 5},
        )
        assert verdict.holds, verdict.error
        failing = self.model.verify_requirement(
            "Demo::Under", subject="Demo::t", arguments=[5, 4],
        )
        assert not failing.holds
        assert not failing.error

    def test_constraint_binds_positional_and_named_arguments(self):
        assert self.model.verify_constraint("Demo::Between", arguments=[3]).holds
        verdict = self.model.verify_constraint(
            "Demo::Between", arguments=[3], named_arguments={"high": 1},
        )
        assert not verdict.holds

    def test_an_unbound_or_unknown_parameter_is_undecided(self):
        unbound = self.model.verify_requirement("Demo::Under", subject="Demo::t")
        assert unbound.error
        unknown = self.model.verify_requirement(
            "Demo::Under", subject="Demo::t", named_arguments={"limit": 5, "bound": 1},
        )
        assert "bound" in unknown.error

    def test_a_proof_question_takes_no_arguments(self):
        with pytest.raises(InvalidRequestError):
            self.model.verify_constraint(
                "Demo::Between", question="holds", arguments=[3],
            )

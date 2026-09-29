"""Tests for the question a verification asks: holds and satisfiable.

The unit half checks the client sends the field and reads the verdict's
question, status and witness. The service half checks the answers a solver
proves and witnesses, so its tests need a service and, where an answer does, a
solver — like the REPL's, one on PATH or named by $OPENSYSML_SMT.
"""

import os
import shutil
from unittest.mock import Mock, patch

import grpc
import pytest

from opensysml.capabilities import (
    CAPABILITY_FEATURE_VALUES,
    CAPABILITY_VERIFICATION,
    CAPABILITY_VERIFICATION_QUESTIONS,
    MissingCapabilityError,
)
from opensysml.connection import Connection
from opensysml.errors import InvalidRequestError, WrongKindError
from opensysml.proto import sysml_pb2
from opensysml.verdict import QUESTION_EVALUATE, QUESTION_HOLDS, Verdict
from tests.service_gate import fail_if_service_promised, is_server_available

#: A solver the tests needing an answer need of the service.
REQUIRE_SMT_ENV = 'OPENSYSML_REQUIRE_SMT'


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


def test_a_question_is_sent_and_capability_checked():
    stub = Mock()
    stub.VerifyConstraint.return_value = sysml_pb2.VerifyConstraintResponse(
        verdict=sysml_pb2.Verdict(
            kind="constraint",
            element_id="Demo::lemma",
            holds=True,
            question="holds",
            status="holds",
        ),
    )
    conn = make_connection(
        stub,
        capabilities=[CAPABILITY_VERIFICATION, CAPABILITY_VERIFICATION_QUESTIONS],
    )

    verdict = conn.verify_constraint("Demo::lemma", "hash1", question="holds")

    request = stub.VerifyConstraint.call_args[0][0]
    assert request.question == "holds"
    assert verdict.question == "holds"
    assert verdict.status == "holds"
    assert verdict.holds


def test_an_omitted_question_sends_no_field():
    stub = Mock()
    stub.VerifyConstraint.return_value = sysml_pb2.VerifyConstraintResponse(
        verdict=sysml_pb2.Verdict(
            kind="constraint",
            element_id="Demo::c",
            holds=True,
            question="evaluate",
            status="holds",
        ),
    )
    conn = make_connection(stub)

    verdict = conn.verify_constraint("Demo::c", "hash1")

    request = stub.VerifyConstraint.call_args[0][0]
    assert request.question == ""
    assert verdict.question == "evaluate"
    assert verdict.status == "holds"


def test_a_question_needs_the_capability_before_anything_is_sent():
    stub = Mock()
    conn = make_connection(stub)

    with pytest.raises(MissingCapabilityError):
        conn.verify_constraint("Demo::c", "hash1", question="holds")
    stub.VerifyConstraint.assert_not_called()


def test_question_constants_spell_the_questions():
    assert QUESTION_EVALUATE == "evaluate"
    assert QUESTION_HOLDS == "holds"


def test_a_witness_decodes_the_assignment():
    stub = Mock()
    stub.VerifyConstraint.return_value = sysml_pb2.VerifyConstraintResponse(
        verdict=sysml_pb2.Verdict(
            kind="constraint",
            element_id="Demo::bad",
            holds=False,
            question="holds",
            status="violated",
            witness=[
                sysml_pb2.WitnessAssignment(
                    feature="Demo::hg.power",
                    value=sysml_pb2.Value(real_value=1.5),
                    exact="3/2",
                ),
                sysml_pb2.WitnessAssignment(
                    feature="Demo::d",
                    value=sysml_pb2.Value(real_value=2.0),
                    exact="2",
                ),
            ],
        ),
    )
    conn = make_connection(
        stub,
        capabilities=[CAPABILITY_VERIFICATION, CAPABILITY_VERIFICATION_QUESTIONS],
    )

    verdict = conn.verify_constraint("Demo::bad", "hash1", question="holds")

    assert verdict.status == "violated"
    assert not verdict.holds
    values = {w.feature: w for w in verdict.witness}
    assert values["Demo::hg.power"].value == 1.5
    assert values["Demo::hg.power"].exact == "3/2"
    assert values["Demo::hg.power"].unit == ""
    assert values["Demo::d"].value == 2.0


def test_requirement_and_satisfaction_carry_the_question():
    stub = Mock()
    stub.VerifyRequirement.return_value = sysml_pb2.VerifyRequirementResponse(
        verdict=sysml_pb2.Verdict(kind="requirement", holds=True),
    )
    stub.VerifySatisfaction.return_value = sysml_pb2.VerifySatisfactionResponse(
        verdicts=[sysml_pb2.Verdict(kind="satisfy", holds=True)],
    )
    conn = make_connection(
        stub,
        capabilities=[CAPABILITY_VERIFICATION, CAPABILITY_VERIFICATION_QUESTIONS],
    )

    conn.verify_requirement("Demo::r", "hash1", question="satisfiable")
    conn.verify_satisfaction("hash1", question="holds")

    assert stub.VerifyRequirement.call_args[0][0].question == "satisfiable"
    assert stub.VerifySatisfaction.call_args[0][0].question == "holds"


def rpc_error(code, details=""):
    """A gRPC failure carrying a status, as the service's would."""

    class Failure(grpc.RpcError, grpc.Call):
        def code(self):
            return code

        def details(self):
            return details

        def trailing_metadata(self):
            return ()

    return Failure()


def test_a_bogus_question_raises_invalid_request():
    stub = Mock()
    stub.VerifyConstraint.side_effect = rpc_error(
        grpc.StatusCode.INVALID_ARGUMENT, "unknown question bogus"
    )
    conn = make_connection(
        stub,
        capabilities=[CAPABILITY_VERIFICATION, CAPABILITY_VERIFICATION_QUESTIONS],
    )

    with pytest.raises(InvalidRequestError):
        conn.verify_constraint("Demo::c", "hash1", question="bogus")


MODEL_SOURCE = '''
package P {
    private import ScalarValues::*;
    part hg { attribute power : Real; }
    attribute d : Real;
    assert constraint lemma { hg.power * hg.power >= 0.0 }
    assert constraint bad { hg.power * d <= hg.power }
    assert constraint never { hg.power > 1.0 and hg.power < 0.0 }
    assert constraint sq { hg.power ** 2.0 >= 0.0 }
}
'''

QUANTITY_SOURCE = '''
import ISQ::*;
import SI::*;
package Q {
    part engine { attribute power : PowerValue; }
    assert constraint nonneg {
        engine.power >= 0 [W] implies engine.power * 2.0 >= engine.power
    }
    assert constraint cap { engine.power <= 100 [W] }
}
'''

_AVAILABLE = is_server_available()
fail_if_service_promised(_AVAILABLE)


def solver_available():
    """Whether a solver answers the service's questions: one on PATH or named."""
    if os.environ.get('OPENSYSML_SMT'):
        return True
    return shutil.which('z3') is not None or shutil.which('cvc5') is not None


def require_solver():
    """Skip for no solver; fail where $OPENSYSML_REQUIRE_SMT promised one."""
    if solver_available():
        return
    if os.environ.get(REQUIRE_SMT_ENV):
        pytest.fail(
            f"${REQUIRE_SMT_ENV} is set, so a solver must answer, but neither "
            "$OPENSYSML_SMT names one nor z3 or cvc5 is on PATH"
        )
    pytest.skip("no solver: set $OPENSYSML_SMT or put z3 or cvc5 on PATH")


@pytest.mark.integration
@pytest.mark.skipif(not _AVAILABLE, reason="sysml-grpc server not running")
class TestVerificationQuestions:
    """Verification questions against a real service."""

    def setup_method(self):
        self.conn = Connection(auto_start=False)
        self.model = self.conn.load_from_content(MODEL_SOURCE)

    def teardown_method(self):
        self.conn.close()

    def test_lemma_holds_proved(self):
        require_solver()
        verdict = self.model.verify_constraint("P::lemma", question="holds")
        assert verdict.question == "holds"
        assert verdict.status == "holds"
        assert verdict.holds
        assert verdict.strength == "proved"

    def test_bad_is_violated_with_a_witness(self):
        require_solver()
        verdict = self.model.verify_constraint("P::bad", question="holds")
        assert verdict.status == "violated"
        assert not verdict.holds
        values = {w.feature: w for w in verdict.witness}
        power = values["P::hg.power"].value
        d = values["P::d"].value
        assert power * d > power, (
            f"witness {power} * {d} does not violate hg.power * d <= hg.power"
        )
        for assignment in verdict.witness:
            assert assignment.exact

    def test_bad_is_satisfiable_with_a_witness(self):
        require_solver()
        verdict = self.model.verify_constraint("P::bad", question="satisfiable")
        assert verdict.status == "satisfiable"
        assert verdict.holds
        assert len(verdict.witness) > 0

    def test_never_is_unsatisfiable(self):
        require_solver()
        verdict = self.model.verify_constraint("P::never", question="satisfiable")
        assert verdict.status == "unsatisfiable"
        assert not verdict.holds

    def test_nonlinear_is_undecided(self):
        # No solver needed: the translation itself refuses exponentiation.
        verdict = self.model.verify_constraint("P::sq", question="holds")
        assert verdict.status == "undecided"
        assert not verdict.holds
        assert verdict.error

    def test_a_quantity_claim_proves(self):
        require_solver()
        model = self.conn.load_from_content(QUANTITY_SOURCE)
        nonneg = model.verify_constraint("Q::nonneg", question="holds")
        assert nonneg.status == "holds"
        assert nonneg.strength == "proved"

    def test_a_quantity_violation_reports_units(self):
        require_solver()
        model = self.conn.load_from_content(QUANTITY_SOURCE)
        cap = model.verify_constraint("Q::cap", question="holds")
        assert cap.status == "violated"
        units = [w.unit for w in cap.witness]
        assert any(unit for unit in units)

    def test_the_capability_is_advertised(self):
        info = self.conn.server_info()
        assert CAPABILITY_VERIFICATION_QUESTIONS in info.capabilities

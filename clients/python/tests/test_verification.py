"""Tests for the verification wrappers: constraints, requirements, satisfy, calc.

These are the answers an engineer scripts ("does this model satisfy its
requirements?"), so what matters here is that a condition evaluating to false
comes back as a verdict while a failure to evaluate comes back as an exception.
"""

import grpc
import pytest
from unittest.mock import Mock, patch

from opensysml.capabilities import (
    CAPABILITY_FEATURE_VALUES,
    CAPABILITY_VERIFICATION,
    MissingCapabilityError,
)
from opensysml.connection import Connection
from opensysml.errors import (
    AnalysisRunError,
    ExecutionError,
    ModelNotFoundError,
    UnsupportedValueError,
    WrongKindError,
)
from opensysml.proto import sysml_pb2
from opensysml.verdict import AnalysisResult, CalcResult, Verdict, VerificationVerdict


def make_connection(stub):
    """Build a Connection over a mock stub that reports verification support."""
    stub.GetServerInfo.return_value = sysml_pb2.ServerInfoResponse(
        version="test",
        capabilities=[CAPABILITY_VERIFICATION, CAPABILITY_FEATURE_VALUES],
    )
    with patch('grpc.insecure_channel'):
        with patch(
            'opensysml.proto.sysml_pb2_grpc.SysMLServiceStub',
            return_value=stub,
        ):
            return Connection(auto_start=False)


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


def test_verify_constraint_holds():
    stub = Mock()
    stub.VerifyConstraint.return_value = sysml_pb2.VerifyConstraintResponse(
        verdict=sysml_pb2.Verdict(
            kind="constraint",
            element_id="Demo::Vehicle::massOK",
            holds=True,
            instance_id=1,
            instance_type_id="Demo::Vehicle",
        ),
        instances=[sysml_pb2.Instance(id=1, type_symbol_id="Demo::Vehicle")],
    )
    conn = make_connection(stub)

    verdict = conn.verify_constraint(
        "Demo::Vehicle::massOK", "hash1", subject_symbol_id="Demo::sedan"
    )

    request = stub.VerifyConstraint.call_args[0][0]
    assert request.model_hash == "hash1"
    assert request.symbol_id == "Demo::Vehicle::massOK"
    assert request.subject_symbol_id == "Demo::sedan"

    assert isinstance(verdict, Verdict)
    assert verdict.holds
    assert bool(verdict) is True
    assert verdict.kind == "constraint"
    assert verdict.element == "Demo::Vehicle::massOK"
    assert verdict.instance_id == 1
    assert verdict.evaluated
    assert [inst.id for inst in verdict.instances] == [1]
    # A holding verdict raises nothing, so this can guard a script.
    assert verdict.raise_for_error() is verdict


def test_verify_constraint_false_is_a_verdict_not_an_exception():
    stub = Mock()
    stub.VerifyConstraint.return_value = sysml_pb2.VerifyConstraintResponse(
        verdict=sysml_pb2.Verdict(
            kind="constraint",
            element_id="Demo::Vehicle::massOK",
            holds=False,
            condition="mass < 100.0",
        ),
    )
    conn = make_connection(stub)

    verdict = conn.verify_constraint("Demo::Vehicle::massOK", "hash1")

    assert not verdict
    assert verdict.evaluated
    assert verdict.condition == "mass < 100.0"
    assert "mass < 100.0" in verdict.explain()
    verdict.raise_for_error()


def test_verify_constraint_evaluation_failure_raises_on_request():
    stub = Mock()
    stub.VerifyConstraint.return_value = sysml_pb2.VerifyConstraintResponse(
        verdict=sysml_pb2.Verdict(
            kind="constraint",
            element_id="Demo::Vehicle::massOK",
            holds=False,
            error="feature mass is unbound",
        ),
    )
    conn = make_connection(stub)

    verdict = conn.verify_constraint("Demo::Vehicle::massOK", "hash1")

    assert not verdict.evaluated
    assert verdict.error == "feature mass is unbound"
    with pytest.raises(ExecutionError) as exc_info:
        verdict.raise_for_error()
    assert "unbound" in str(exc_info.value)


def test_verify_constraint_unanswerable_request_raises():
    stub = Mock()
    stub.VerifyConstraint.return_value = sysml_pb2.VerifyConstraintResponse(
        error="symbol not found: Demo::Nope",
        diagnostics=[sysml_pb2.Diagnostic(severity="error", message="boom")],
    )
    conn = make_connection(stub)

    with pytest.raises(ExecutionError) as exc_info:
        conn.verify_constraint("Demo::Nope", "hash1")
    assert "Demo::Nope" in str(exc_info.value)
    assert [d.message for d in exc_info.value.diagnostics] == ["boom"]


def test_verify_requirement_holds():
    stub = Mock()
    stub.VerifyRequirement.return_value = sysml_pb2.VerifyRequirementResponse(
        verdict=sysml_pb2.Verdict(
            kind="requirement",
            element_id="Demo::Vehicle::lightEnough",
            holds=True,
        ),
    )
    conn = make_connection(stub)

    verdict = conn.verify_requirement(
        "Demo::Vehicle::lightEnough", "hash1", subject_symbol_id="Demo::sedan"
    )

    request = stub.VerifyRequirement.call_args[0][0]
    assert request.symbol_id == "Demo::Vehicle::lightEnough"
    assert request.subject_symbol_id == "Demo::sedan"
    assert verdict.kind == "requirement"
    assert verdict.holds


def test_verify_requirement_reports_the_body_verdicts_beside_satisfaction():
    stub = Mock()
    stub.VerifyRequirement.return_value = sysml_pb2.VerifyRequirementResponse(
        verdict=sysml_pb2.Verdict(
            kind="requirement",
            element_id="Demo::Vehicle::lightEnough",
            holds=True,
        ),
        verification_verdicts=[
            sysml_pb2.VerificationVerdict(
                case_id="Demo::checkMass", kind="fail",
            ),
            sysml_pb2.VerificationVerdict(
                case_id="Demo::checkMass::inner", kind="inconclusive",
                detail="body produced no verdict", subcase=True,
            ),
        ],
    )
    conn = make_connection(stub)

    verdict = conn.verify_requirement("Demo::Vehicle::lightEnough", "hash1")

    # Satisfaction is unchanged; the bodies answer beside it.
    assert verdict.holds
    assert [v.kind for v in verdict.verifications] == ["fail", "inconclusive"]
    assert isinstance(verdict.verifications[0], VerificationVerdict)
    assert verdict.verifications[0].case_id == "Demo::checkMass"
    assert not verdict.verifications[0]
    assert not verdict.verifications[0].subcase
    subcase = verdict.verifications[1]
    assert subcase.subcase
    assert subcase.detail == "body produced no verdict"
    assert "(subcase)" in subcase.explain()
    assert "body produced no verdict" in str(subcase)
    assert "inconclusive" in repr(subcase)


def test_a_passing_body_verdict_reads_as_a_pass():
    stub = Mock()
    stub.VerifyRequirement.return_value = sysml_pb2.VerifyRequirementResponse(
        verdict=sysml_pb2.Verdict(kind="requirement", element_id="Demo::r", holds=True),
        verification_verdicts=[
            sysml_pb2.VerificationVerdict(case_id="Demo::check", kind="pass"),
        ],
    )
    conn = make_connection(stub)

    passing = conn.verify_requirement("Demo::r", "hash1").verifications[0]

    assert passing
    assert passing.kind == "pass"
    assert passing.detail == ""


def test_verify_satisfaction_reports_the_body_verdicts_of_the_run():
    stub = Mock()
    stub.VerifySatisfaction.return_value = sysml_pb2.VerifySatisfactionResponse(
        verdicts=[
            sysml_pb2.Verdict(
                kind="satisfy",
                element="satisfy r by sedan",
                holds=True,
                requirement_id="Demo::r",
            ),
        ],
        verification_verdicts=[
            sysml_pb2.VerificationVerdict(
                case_id="Demo::check",
                kind="error",
                detail="no value for feature m",
                requirement_id="Demo::r",
            ),
        ],
    )
    conn = make_connection(stub)

    verdicts = conn.verify_satisfaction("hash1")

    assert verdicts[0].holds
    assert verdicts[0].requirement_id == "Demo::r"
    assert [v.kind for v in verdicts[0].verifications] == ["error"]
    assert verdicts[0].verifications[0].detail == "no value for feature m"
    assert verdicts[0].verifications[0].requirement_id == "Demo::r"


def test_verify_satisfaction_gives_each_verdict_only_its_own_requirements_cases():
    stub = Mock()
    stub.VerifySatisfaction.return_value = sysml_pb2.VerifySatisfactionResponse(
        verdicts=[
            sysml_pb2.Verdict(
                kind="satisfy",
                element="satisfy massLimit by sedan",
                holds=True,
                requirement_id="Demo::massLimit",
            ),
            sysml_pb2.Verdict(
                kind="satisfy",
                element="satisfy rangeLimit by sedan",
                holds=True,
                requirement_id="Demo::rangeLimit",
            ),
            sysml_pb2.Verdict(kind="satisfy", element="satisfy by sedan", holds=True),
        ],
        verification_verdicts=[
            sysml_pb2.VerificationVerdict(
                case_id="Demo::checkMass", kind="pass", requirement_id="Demo::massLimit",
            ),
            sysml_pb2.VerificationVerdict(
                case_id="Demo::checkRange", kind="fail", requirement_id="Demo::rangeLimit",
            ),
        ],
    )
    conn = make_connection(stub)

    verdicts = conn.verify_satisfaction("hash1")

    # One call covering several requirements keeps their cases apart.
    assert [v.case_id for v in verdicts[0].verifications] == ["Demo::checkMass"]
    assert [v.case_id for v in verdicts[1].verifications] == ["Demo::checkRange"]
    # An assertion of a requirement no FQN names takes none rather than all.
    assert verdicts[2].verifications == []


def test_run_analysis_of_a_verification_case_reports_its_body_verdict():
    stub = Mock()
    stub.RunAnalysis.return_value = sysml_pb2.RunAnalysisResponse(
        verification_verdicts=[
            sysml_pb2.VerificationVerdict(case_id="Demo::check", kind="pass"),
            sysml_pb2.VerificationVerdict(
                case_id="Demo::check::sub", kind="fail", subcase=True,
            ),
        ],
    )
    conn = make_connection(stub)

    result = conn.run_analysis("Demo::check", "hash1")

    assert [v.kind for v in result.verifications] == ["pass", "fail"]
    assert result.verifications[1].subcase
    assert "verification Demo::check verdict: pass" in str(result)


def test_run_analysis_keeps_an_evaluation_whose_argument_has_no_wire_form():
    """One argument the service could not send stands in for itself, not the run."""
    stub = Mock()
    stub.RunAnalysis.return_value = sysml_pb2.RunAnalysisResponse(
        outputs=[sysml_pb2.CalcOutput(name="result", value=sysml_pb2.Value(real_value=10.0))],
        evaluations=[
            sysml_pb2.CaseEvaluation(
                function_id="Demo::eval",
                arguments=[sysml_pb2.Value(null="unsupported: quantity value")],
                result=sysml_pb2.Value(real_value=30.0),
            ),
            sysml_pb2.CaseEvaluation(
                function_id="Demo::eval",
                arguments=[sysml_pb2.Value(int_value=2)],
                result=sysml_pb2.Value(null="unsupported: quantity value"),
                selected=True,
            ),
        ],
    )
    conn = make_connection(stub)

    result = conn.run_analysis("Demo::study", "hash1")

    assert result.outputs["result"] == 10.0
    first, second = result.evaluations
    assert isinstance(first.arguments[0], UnsupportedValueError)
    assert first.result == 30.0
    assert second.arguments == [2]
    assert isinstance(second.result, UnsupportedValueError)
    assert second.selected


def test_run_sweep_keeps_an_evaluation_whose_argument_has_no_wire_form():
    stub = Mock()
    stub.RunSweep.return_value = sysml_pb2.RunSweepResponse(
        parameters=["k"],
        rows=[sysml_pb2.SweepRow(
            inputs=[sysml_pb2.CalcOutput(name="k", value=sysml_pb2.Value(int_value=1))],
            outputs=[sysml_pb2.CalcOutput(name="result", value=sysml_pb2.Value(real_value=10.0))],
            evaluations=[sysml_pb2.CaseEvaluation(
                function_id="Demo::eval",
                arguments=[
                    sysml_pb2.Value(null="unsupported: quantity value"),
                    sysml_pb2.Value(int_value=1),
                ],
                result=sysml_pb2.Value(real_value=30.0),
            )],
        )],
    )
    conn = make_connection(stub)

    table = conn.run_sweep("Demo::study", "hash1", {"k": (1, 1)})

    row = table.rows[0]
    assert row.outputs["result"] == 10.0
    (evaluation,) = row.evaluations
    assert isinstance(evaluation.arguments[0], UnsupportedValueError)
    assert evaluation.arguments[1] == 1
    assert evaluation.result == 30.0


def test_a_service_without_body_verdicts_reports_none():
    stub = Mock()
    stub.VerifyRequirement.return_value = sysml_pb2.VerifyRequirementResponse(
        verdict=sysml_pb2.Verdict(kind="requirement", element_id="Demo::r", holds=True),
    )
    conn = make_connection(stub)

    assert conn.verify_requirement("Demo::r", "hash1").verifications == []


def test_a_service_without_requirement_association_reports_no_satisfaction_cases():
    stub = Mock()
    stub.VerifySatisfaction.return_value = sysml_pb2.VerifySatisfactionResponse(
        verdicts=[sysml_pb2.Verdict(kind="satisfy", element="satisfy r", holds=True)],
        verification_verdicts=[
            sysml_pb2.VerificationVerdict(case_id="Demo::check", kind="pass"),
        ],
    )
    conn = make_connection(stub)

    # Naming no requirement, the response says of no verdict that these are its
    # own cases, so none are read as such rather than all of them being.
    assert conn.verify_satisfaction("hash1")[0].verifications == []


def test_verify_satisfaction_reports_one_verdict_per_assertion():
    stub = Mock()
    stub.VerifySatisfaction.return_value = sysml_pb2.VerifySatisfactionResponse(
        verdicts=[
            sysml_pb2.Verdict(
                kind="satisfy",
                element="satisfy massLimit by sedan",
                holds=True,
                instance_id=1,
            ),
            sysml_pb2.Verdict(
                kind="satisfy",
                element="satisfy massTiny by sedan",
                holds=False,
                condition="vehicle.mass <= maxMass",
                instance_id=2,
            ),
        ],
        instances=[
            sysml_pb2.Instance(id=1, type_symbol_id="Demo::Vehicle"),
            sysml_pb2.Instance(id=2, type_symbol_id="Demo::Vehicle"),
        ],
        diagnostics=[sysml_pb2.Diagnostic(severity="warning", message="heads up")],
    )
    conn = make_connection(stub)

    verdicts = conn.verify_satisfaction("hash1")

    assert stub.VerifySatisfaction.call_args[0][0].symbol_id == ""
    assert [v.holds for v in verdicts] == [True, False]
    assert verdicts[1].element == "satisfy massTiny by sedan"
    assert verdicts[1].condition == "vehicle.mass <= maxMass"
    # The diagnostics of the run are readable from any of its verdicts.
    assert [d.message for d in verdicts[0].diagnostics] == ["heads up"]
    assert [inst.id for inst in verdicts[0].instances] == [1, 2]


def test_verify_satisfaction_narrowed_to_a_symbol():
    stub = Mock()
    stub.VerifySatisfaction.return_value = sysml_pb2.VerifySatisfactionResponse()
    conn = make_connection(stub)

    assert conn.verify_satisfaction("hash1", symbol_id="Demo::analysis") == []
    assert stub.VerifySatisfaction.call_args[0][0].symbol_id == "Demo::analysis"


def test_calc_invocation_returns_its_value():
    stub = Mock()
    stub.EvaluateCalc.return_value = sysml_pb2.EvaluateCalcResponse(
        result=sysml_pb2.Value(real_value=6.5),
    )
    conn = make_connection(stub)

    result = conn.calc("Demo::add", "hash1", arguments=[2.5, 4.0])

    request = stub.EvaluateCalc.call_args[0][0]
    assert request.symbol_id == "Demo::add"
    assert [arg.real_value for arg in request.arguments] == [2.5, 4.0]
    assert isinstance(result, CalcResult)
    assert result.value == 6.5
    assert result.outputs == {}


def test_calc_usage_returns_its_outputs():
    stub = Mock()
    stub.EvaluateCalc.return_value = sysml_pb2.EvaluateCalcResponse(
        outputs=[
            sysml_pb2.CalcOutput(name="a", value=sysml_pb2.Value(int_value=6)),
            sysml_pb2.CalcOutput(name="b", value=sysml_pb2.Value(int_value=10)),
        ],
    )
    conn = make_connection(stub)

    result = conn.calc("Demo::c", "hash1")

    assert stub.EvaluateCalc.call_args[0][0].arguments == []
    assert result.outputs == {"a": 6, "b": 10}
    assert result.value is None
    assert "a = 6" in str(result)


def test_calc_failure_raises_with_diagnostics():
    stub = Mock()
    stub.EvaluateCalc.return_value = sysml_pb2.EvaluateCalcResponse(
        error="calc invocation failed: unbound input y",
        diagnostics=[sysml_pb2.Diagnostic(severity="error", message="unbound input y")],
    )
    conn = make_connection(stub)

    with pytest.raises(ExecutionError) as exc_info:
        conn.calc("Demo::add", "hash1", arguments=[1.0])
    assert "unbound input y" in str(exc_info.value)
    assert len(exc_info.value.diagnostics) == 1
    # ExecutionError is a builtin RuntimeError too, so `except RuntimeError`
    # catches it.
    assert isinstance(exc_info.value, RuntimeError)


def test_verification_requires_the_capability():
    stub = Mock()
    stub.GetServerInfo.return_value = sysml_pb2.ServerInfoResponse(
        version="old", capabilities=["typefacts"]
    )
    with patch('grpc.insecure_channel'):
        with patch(
            'opensysml.proto.sysml_pb2_grpc.SysMLServiceStub',
            return_value=stub,
        ):
            conn = Connection(auto_start=False)

    for call in (
        lambda: conn.verify_constraint("Demo::c", "hash1"),
        lambda: conn.verify_requirement("Demo::r", "hash1"),
        lambda: conn.verify_satisfaction("hash1"),
        lambda: conn.calc("Demo::add", "hash1"),
        lambda: conn.run_analysis("Demo::a", "hash1"),
    ):
        with pytest.raises(MissingCapabilityError):
            call()
    stub.VerifyConstraint.assert_not_called()
    stub.EvaluateCalc.assert_not_called()
    stub.RunAnalysis.assert_not_called()


def test_run_analysis_reports_outputs_and_verdicts():
    stub = Mock()
    stub.RunAnalysis.return_value = sysml_pb2.RunAnalysisResponse(
        outputs=[
            sysml_pb2.CalcOutput(name="total", value=sysml_pb2.Value(real_value=12.0)),
        ],
        verdicts=[
            sysml_pb2.Verdict(
                kind="objective", element_id="An::CostAnalysis::affordable",
                element="affordable", holds=True, instance_id=1,
                instance_type_id="An::ship",
            ),
            sysml_pb2.Verdict(
                kind="assertion", element="cheap", holds=False,
                condition="total < 10.0",
            ),
        ],
        instances=[sysml_pb2.Instance(id=1, type_symbol_id="An::ship")],
    )
    conn = make_connection(stub)

    result = conn.run_analysis(
        "An::CostAnalysis", "hash1", subject="An::ship",
        arguments=[2.5], named_arguments={"limit": 20.0},
    )

    request = stub.RunAnalysis.call_args[0][0]
    assert request.model_hash == "hash1"
    assert request.symbol_id == "An::CostAnalysis"
    assert request.subject_symbol_id == "An::ship"
    assert [arg.real_value for arg in request.arguments] == [2.5]
    assert request.named_arguments["limit"].real_value == 20.0
    assert isinstance(result, AnalysisResult)
    assert result.outputs == {"total": 12.0}
    assert [v.kind for v in result.verdicts] == ["objective", "assertion"]
    assert result.verdicts[0].holds
    assert result.verdicts[0].element_id == "An::CostAnalysis::affordable"
    assert result.verdicts[1].condition == "total < 10.0"
    assert not result.satisfied
    assert not result
    assert [inst.id for inst in result.instances] == [1]
    assert result.verdicts[0].instances == result.instances
    text = str(result)
    assert "total = 12.0" in text
    assert "objective affordable holds" in text
    assert "assertion cheap fails" in text


def test_run_analysis_without_objective_is_satisfied():
    stub = Mock()
    stub.RunAnalysis.return_value = sysml_pb2.RunAnalysisResponse(
        outputs=[sysml_pb2.CalcOutput(name="x", value=sysml_pb2.Value(real_value=3.0))],
    )
    conn = make_connection(stub)

    result = conn.run_analysis("An::plain", "hash1")

    request = stub.RunAnalysis.call_args[0][0]
    assert request.subject_symbol_id == ""
    assert list(request.arguments) == []
    assert dict(request.named_arguments) == {}
    assert result.outputs == {"x": 3.0}
    assert result.verdicts == []
    assert result.satisfied
    assert result


def test_run_analysis_undecided_objective_is_an_error_on_the_verdict():
    stub = Mock()
    stub.RunAnalysis.return_value = sysml_pb2.RunAnalysisResponse(
        verdicts=[
            sysml_pb2.Verdict(
                kind="objective", element="obj", holds=False,
                error="no value for feature limit",
                failure_reason=sysml_pb2.FAILURE_REASON_EVALUATION,
            ),
        ],
    )
    conn = make_connection(stub)

    result = conn.run_analysis("An::a", "hash1")

    assert not result.satisfied
    assert not result.verdicts[0].evaluated
    with pytest.raises(ExecutionError) as exc_info:
        result.verdicts[0].raise_for_error()
    assert "no value for feature limit" in str(exc_info.value)


def test_run_analysis_failure_raises_with_diagnostics():
    stub = Mock()
    stub.RunAnalysis.return_value = sysml_pb2.RunAnalysisResponse(
        error="analysis run failed: analysis An::a: s subject is unbound",
        failure_reason=sysml_pb2.FAILURE_REASON_EVALUATION,
        diagnostics=[sysml_pb2.Diagnostic(severity="error", message="subject is unbound")],
    )
    conn = make_connection(stub)

    with pytest.raises(ExecutionError) as exc_info:
        conn.run_analysis("An::a", "hash1")
    assert "subject is unbound" in str(exc_info.value)
    assert not isinstance(exc_info.value, WrongKindError)
    assert len(exc_info.value.diagnostics) == 1


def test_run_analysis_wrong_kind_raises():
    stub = Mock()
    stub.RunAnalysis.return_value = sysml_pb2.RunAnalysisResponse(
        error="not an analysis case: An::ship is a part usage",
        failure_reason=sysml_pb2.FAILURE_REASON_WRONG_KIND,
    )
    conn = make_connection(stub)

    with pytest.raises(WrongKindError):
        conn.run_analysis("An::ship", "hash1")


def test_evicted_model_is_a_model_not_found_error():
    stub = Mock()
    stub.VerifySatisfaction.side_effect = rpc_error(
        grpc.StatusCode.NOT_FOUND, "model not found: hash1"
    )
    conn = make_connection(stub)

    with pytest.raises(ModelNotFoundError) as exc_info:
        conn.verify_satisfaction("hash1")
    assert isinstance(exc_info.value.__cause__, grpc.RpcError)


def test_model_verification_helpers_pass_the_models_hash():
    stub = Mock()
    stub.ParseFile.return_value = sysml_pb2.ParseFileResponse(
        model_hash="hash1",
        root=sysml_pb2.SymbolInfo(id="Demo", name="Demo", kind="Package"),
    )
    stub.VerifyConstraint.return_value = sysml_pb2.VerifyConstraintResponse(
        verdict=sysml_pb2.Verdict(kind="constraint", holds=True),
    )
    stub.VerifyRequirement.return_value = sysml_pb2.VerifyRequirementResponse(
        verdict=sysml_pb2.Verdict(kind="requirement", holds=True),
    )
    stub.VerifySatisfaction.return_value = sysml_pb2.VerifySatisfactionResponse(
        verdicts=[
            sysml_pb2.Verdict(kind="satisfy", holds=True),
            sysml_pb2.Verdict(kind="satisfy", holds=False),
        ],
    )
    stub.EvaluateCalc.return_value = sysml_pb2.EvaluateCalcResponse(
        result=sysml_pb2.Value(int_value=4),
    )
    conn = make_connection(stub)
    model = conn.load("demo.sysml")

    assert model.verify_constraint("Demo::c", subject="Demo::sedan").holds
    assert stub.VerifyConstraint.call_args[0][0].model_hash == "hash1"
    assert stub.VerifyConstraint.call_args[0][0].subject_symbol_id == "Demo::sedan"

    assert model.verify_requirement("Demo::r").holds
    assert stub.VerifyRequirement.call_args[0][0].model_hash == "hash1"

    assert len(model.verify_satisfaction()) == 2
    # One failing assertion is enough for the model not to satisfy them.
    assert model.satisfied() is False

    assert model.calc("Demo::add", arguments=[2, 2]).value == 4
    assert stub.EvaluateCalc.call_args[0][0].model_hash == "hash1"


def test_satisfied_is_false_when_an_assertion_could_not_be_evaluated():
    stub = Mock()
    stub.ParseFile.return_value = sysml_pb2.ParseFileResponse(
        model_hash="hash1",
        root=sysml_pb2.SymbolInfo(id="Demo", name="Demo", kind="Package"),
    )
    stub.VerifySatisfaction.return_value = sysml_pb2.VerifySatisfactionResponse(
        verdicts=[
            sysml_pb2.Verdict(kind="satisfy", holds=False, error="unbound feature"),
        ],
    )
    conn = make_connection(stub)
    model = conn.load("demo.sysml")

    assert model.satisfied() is False


class TestVerdictLines:
    def test_an_assertions_line_does_not_repeat_the_kind_it_is(self):
        # The assertion's own text begins "satisfy …", so "satisfy satisfy …"
        # is not what a reader is shown.
        verdict = Verdict(
            sysml_pb2.Verdict(
                kind="satisfy",
                element="satisfy massLimit by sedan",
                holds=False,
                condition="mass <= maxMass",
            )
        )
        assert str(verdict) == (
            "\u2717 satisfy massLimit by sedan fails: condition evaluated to "
            "false: mass <= maxMass"
        )

    def test_a_named_element_is_still_said_to_be_a_constraint(self):
        verdict = Verdict(
            sysml_pb2.Verdict(
                kind="constraint", element="Demo::Vehicle::massOK", holds=True
            )
        )
        assert str(verdict) == "\u2713 constraint Demo::Vehicle::massOK holds"

    def test_a_negated_assertions_line_does_not_repeat_the_kind_either(self):
        verdict = Verdict(
            sysml_pb2.Verdict(
                kind="satisfy",
                element="not satisfy touchdown by fastLander",
                holds=True,
            )
        )
        assert str(verdict) == (
            "\u2713 not satisfy touchdown by fastLander holds"
        )


class TestWrongKind:
    """A wrong-kind request raises, as naming no element at all already does.

    The kind is read from the response's typed reason, never from the message.
    """

    def _wrong_kind_verdict(self, kind, element_id):
        return sysml_pb2.Verdict(
            kind=kind,
            element_id=element_id,
            holds=False,
            error=f"not a {kind}: {element_id} is a part def",
            failure_reason=sysml_pb2.FAILURE_REASON_WRONG_KIND,
        )

    def test_verify_constraint_raises(self):
        stub = Mock()
        stub.VerifyConstraint.return_value = sysml_pb2.VerifyConstraintResponse(
            verdict=self._wrong_kind_verdict("constraint", "Demo::Wheel"),
        )
        conn = make_connection(stub)

        with pytest.raises(WrongKindError) as exc_info:
            conn.verify_constraint("Demo::Wheel", "hash1")
        assert "not a constraint" in str(exc_info.value)
        # It stays an ExecutionError, which is what such a failure used to be.
        assert isinstance(exc_info.value, ExecutionError)

    def test_verify_requirement_raises(self):
        stub = Mock()
        stub.VerifyRequirement.return_value = sysml_pb2.VerifyRequirementResponse(
            verdict=self._wrong_kind_verdict("requirement", "Demo::Wheel"),
        )
        conn = make_connection(stub)

        with pytest.raises(WrongKindError):
            conn.verify_requirement("Demo::Wheel", "hash1")

    def test_verify_satisfaction_raises(self):
        stub = Mock()
        stub.VerifySatisfaction.return_value = sysml_pb2.VerifySatisfactionResponse(
            error="Demo::V states no satisfaction assertion",
            failure_reason=sysml_pb2.FAILURE_REASON_WRONG_KIND,
        )
        conn = make_connection(stub)

        with pytest.raises(WrongKindError):
            conn.verify_satisfaction("hash1", symbol_id="Demo::V")

    def test_verify_satisfaction_raises_for_a_wrong_kind_verdict(self):
        stub = Mock()
        stub.VerifySatisfaction.return_value = sysml_pb2.VerifySatisfactionResponse(
            verdicts=[self._wrong_kind_verdict("satisfy", "Demo::Wheel")],
        )
        conn = make_connection(stub)

        with pytest.raises(WrongKindError):
            conn.verify_satisfaction("hash1", symbol_id="Demo::Wheel")

    def test_calc_raises(self):
        stub = Mock()
        stub.EvaluateCalc.return_value = sysml_pb2.EvaluateCalcResponse(
            error="not a calc: Demo::Wheel is a part def",
            failure_reason=sysml_pb2.FAILURE_REASON_WRONG_KIND,
        )
        conn = make_connection(stub)

        with pytest.raises(WrongKindError):
            conn.calc("Demo::Wheel", "hash1", arguments=[1])

    def test_a_failure_to_evaluate_is_not_a_wrong_kind(self):
        stub = Mock()
        stub.EvaluateCalc.return_value = sysml_pb2.EvaluateCalcResponse(
            error="calc invocation failed: division by zero",
            failure_reason=sysml_pb2.FAILURE_REASON_EVALUATION,
        )
        conn = make_connection(stub)

        with pytest.raises(ExecutionError) as exc_info:
            conn.calc("Demo::div", "hash1", arguments=[1, 0])
        assert not isinstance(exc_info.value, WrongKindError)

    def test_a_verdict_of_false_is_still_a_verdict(self):
        stub = Mock()
        stub.VerifyConstraint.return_value = sysml_pb2.VerifyConstraintResponse(
            verdict=sysml_pb2.Verdict(
                kind="constraint",
                element_id="Demo::Vehicle::massLight",
                holds=False,
                condition="mass < 100.0",
            ),
        )
        conn = make_connection(stub)

        verdict = conn.verify_constraint("Demo::Vehicle::massLight", "hash1")
        assert verdict.holds is False
        assert verdict.condition == "mass < 100.0"

    def test_an_unevaluated_verdict_is_still_a_verdict(self):
        # A condition the runtime could not evaluate is reported on the verdict,
        # as it was; only a wrong request became an exception.
        stub = Mock()
        stub.VerifyConstraint.return_value = sysml_pb2.VerifyConstraintResponse(
            verdict=sysml_pb2.Verdict(
                kind="constraint",
                element_id="Demo::Vehicle::massOK",
                holds=False,
                error="unbound feature: mass",
            ),
        )
        conn = make_connection(stub)

        verdict = conn.verify_constraint("Demo::Vehicle::massOK", "hash1")
        assert verdict.evaluated is False
        assert verdict.error == "unbound feature: mass"


def test_run_analysis_reports_the_evaluations_of_a_trade_study():
    stub = Mock()
    stub.RunAnalysis.return_value = sysml_pb2.RunAnalysisResponse(
        outputs=[
            sysml_pb2.CalcOutput(name="selectedAlternative", value=sysml_pb2.Value(instance_id=2)),
        ],
        verdicts=[
            sysml_pb2.Verdict(kind="objective", element="tradeStudyObjective", holds=True),
        ],
        instances=[
            sysml_pb2.Instance(id=1, type_symbol_id="Trade::a"),
            sysml_pb2.Instance(id=2, type_symbol_id="Trade::b"),
            sysml_pb2.Instance(id=3, type_symbol_id="Trade::c"),
        ],
        evaluations=[
            sysml_pb2.CaseEvaluation(
                function_id="Trade::lightest::evaluationFunction",
                arguments=[sysml_pb2.Value(instance_id=1)],
                result=sysml_pb2.Value(real_value=30.0),
            ),
            sysml_pb2.CaseEvaluation(
                function_id="Trade::lightest::evaluationFunction",
                arguments=[sysml_pb2.Value(instance_id=2)],
                result=sysml_pb2.Value(real_value=10.0), selected=True,
            ),
            sysml_pb2.CaseEvaluation(
                function_id="Trade::lightest::evaluationFunction",
                arguments=[sysml_pb2.Value(instance_id=3)],
                result=sysml_pb2.Value(real_value=10.0), tied=True,
            ),
        ],
    )
    conn = make_connection(stub)

    result = conn.run_analysis("Trade::lightest", "hash1")

    assert result.satisfied
    assert result.outputs["selectedAlternative"].id == 2
    assert result.outputs["selectedAlternative"].type_symbol_id == "Trade::b"
    assert [e.arguments[0].type_symbol_id for e in result.evaluations] == [
        "Trade::a", "Trade::b", "Trade::c",
    ]
    assert [e.result for e in result.evaluations] == [30.0, 10.0, 10.0]
    assert [e.selected for e in result.evaluations] == [False, True, False]
    assert [e.tied for e in result.evaluations] == [False, False, True]
    assert all(e.evaluated for e in result.evaluations)
    assert [e.arguments[0].id for e in result.selected] == [2]
    text = str(result)
    assert "objective tradeStudyObjective holds" in text
    assert "= 10.0 [selected]" in text
    assert "= 10.0 [tied]" in text


def test_run_analysis_failure_keeps_the_evaluations_made():
    stub = Mock()
    stub.RunAnalysis.return_value = sysml_pb2.RunAnalysisResponse(
        error="analysis run failed: analysis Trade::perCylinder: division by zero",
        failure_reason=sysml_pb2.FAILURE_REASON_EVALUATION,
        verdicts=[
            sysml_pb2.Verdict(
                kind="objective", element="tradeStudyObjective", holds=False,
                error="division by zero", failure_reason=sysml_pb2.FAILURE_REASON_EVALUATION,
            ),
        ],
        instances=[
            sysml_pb2.Instance(id=1, type_symbol_id="Trade::a"),
            sysml_pb2.Instance(id=2, type_symbol_id="Trade::c"),
        ],
        evaluations=[
            sysml_pb2.CaseEvaluation(
                function_id="Trade::perCylinder::evaluationFunction",
                arguments=[sysml_pb2.Value(instance_id=1)],
                result=sysml_pb2.Value(real_value=5.0),
            ),
            sysml_pb2.CaseEvaluation(
                function_id="Trade::perCylinder::evaluationFunction",
                arguments=[sysml_pb2.Value(instance_id=2)],
                error="division by zero",
            ),
        ],
    )
    conn = make_connection(stub)

    with pytest.raises(AnalysisRunError) as exc_info:
        conn.run_analysis("Trade::perCylinder", "hash1")
    err = exc_info.value
    assert isinstance(err, ExecutionError)
    assert "division by zero" in str(err)
    result = err.result
    assert result.outputs == {}
    assert not result.satisfied
    assert not result.verdicts[0].evaluated
    assert [e.evaluated for e in result.evaluations] == [True, False]
    assert result.evaluations[0].result == 5.0
    assert result.evaluations[1].result is None
    assert result.evaluations[1].error == "division by zero"
    assert result.selected == []
    assert "error: division by zero" in str(result.evaluations[1])


def test_run_analysis_of_a_service_without_evaluations_reports_none():
    stub = Mock()
    stub.RunAnalysis.return_value = sysml_pb2.RunAnalysisResponse(
        outputs=[sysml_pb2.CalcOutput(name="x", value=sysml_pb2.Value(real_value=3.0))],
    )
    conn = make_connection(stub)

    result = conn.run_analysis("An::plain", "hash1")

    assert result.evaluations == []
    assert result.selected == []

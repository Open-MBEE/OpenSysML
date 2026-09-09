"""Integration tests for running analysis cases against a real service.

These are about what the service actually computes and decides for a case —
its outputs, its objective's verdict, how the subject and inputs bind — rather
than about how the client wraps a canned response.
"""

import pytest

from opensysml import Connection
from opensysml.errors import AnalysisRunError, ExecutionError, WrongKindError
from opensysml.verdict import AnalysisResult

MODEL_SOURCE = '''
package An {
    private import ScalarValues::*;

    part def Ship {
        attribute cost : Real default = 5.0;
        attribute other : Real default = 7.0;
    }

    calc def Sum {
        in a : Real;
        in b : Real;
        return : Real = a + b;
    }

    analysis def CostAnalysis {
        subject s : Ship;
        in limit : Real = 20.0;
        out total : Real = Sum(s.cost, s.other);
        objective affordable {
            require constraint { total <= limit }
        }
    }

    part ship : Ship;
    part barge : Ship {
        attribute :>> cost = 30.0;
    }

    analysis shipCost : CostAnalysis {
        subject s = ship;
    }

    analysis plain {
        out x : Real = 1.0 + 2.0;
    }
}
'''


@pytest.mark.integration
class TestAnalysisIntegration:
    def setup_method(self):
        self.conn = Connection()
        self.model = self.conn.load_from_content(MODEL_SOURCE)

    def teardown_method(self):
        self.conn.close()

    def test_a_usage_binding_its_subject_computes_its_outputs(self):
        result = self.model.run_analysis("An::shipCost")
        assert isinstance(result, AnalysisResult)
        assert result.outputs == {"total": 12.0}
        assert [v.kind for v in result.verdicts] == ["objective"]
        assert result.verdicts[0].holds
        assert result.verdicts[0].element_id == "An::CostAnalysis::affordable"
        assert result.verdicts[0].instance_type_id == "An::ship"
        assert [inst.id for inst in result.instances][0] == result.verdicts[0].instance_id
        assert result.satisfied

    def test_a_case_without_objective_has_no_verdict(self):
        result = self.model.run_analysis("An::plain")
        assert result.outputs == {"x": 3.0}
        assert result.verdicts == []
        assert result.satisfied

    def test_a_definition_runs_on_the_subject_named(self):
        result = self.model.run_analysis("An::CostAnalysis", subject="An::barge")
        assert result.outputs == {"total": 37.0}
        verdict = result.verdicts[0]
        assert not verdict.holds
        assert verdict.evaluated
        assert verdict.condition == "total <= limit"
        assert verdict.instance_id
        assert [inst.id for inst in result.instances][0] == verdict.instance_id
        assert not result

    def test_arguments_bind_the_inputs(self):
        by_name = self.model.run_analysis(
            "An::CostAnalysis", subject="An::barge", named_arguments={"limit": 50.0}
        )
        assert by_name.satisfied
        positional = self.model.run_analysis(
            "An::CostAnalysis", subject="An::barge", arguments=[10.0]
        )
        assert not positional.satisfied
        assert positional.outputs == {"total": 37.0}

    def test_an_unbound_subject_leaves_the_objective_undecided(self):
        with pytest.raises(AnalysisRunError) as exc_info:
            self.model.run_analysis("An::CostAnalysis")
        assert "subject" in str(exc_info.value)
        assert not isinstance(exc_info.value, WrongKindError)
        result = exc_info.value.result
        assert result.outputs == {} and result.evaluations == []
        assert [v.element for v in result.verdicts] == ["affordable"]
        assert not result.verdicts[0].evaluated
        assert "subject" in result.verdicts[0].error

    def test_a_wrong_kind_raises(self):
        with pytest.raises(WrongKindError) as exc_info:
            self.model.run_analysis("An::ship")
        assert not isinstance(exc_info.value, AnalysisRunError)

    def test_an_unknown_symbol_raises(self):
        with pytest.raises(ExecutionError) as exc_info:
            self.model.run_analysis("An::Nope")
        assert not isinstance(exc_info.value, AnalysisRunError)


TRADE_STUDY_SOURCE = '''
package Trade {
    private import ScalarValues::*;
    private import TradeStudies::*;

    part def Engine { attribute mass : Real; attribute cylinders : Integer; }
    part a : Engine { attribute :>> mass = 30.0; attribute :>> cylinders = 6; }
    part b : Engine { attribute :>> mass = 10.0; attribute :>> cylinders = 4; }
    part c : Engine { attribute :>> mass = 10.0; attribute :>> cylinders = 0; }

    analysis lightest : TradeStudy {
        subject : Engine[1..*] = (a, b, c);
        objective : MinimizeObjective;
        calc :>> evaluationFunction {
            in part e :>> alternative : Engine;
            return :>> result : Real = e.mass;
        }
        return part :>> selectedAlternative : Engine;
    }

    analysis perCylinder : TradeStudy {
        subject : Engine[1..*] = (a, c);
        objective : MinimizeObjective;
        calc :>> evaluationFunction {
            in part e :>> alternative : Engine;
            return :>> result : Real = e.mass / e.cylinders;
        }
        return part :>> selectedAlternative : Engine;
    }

    analysis perOffset : TradeStudy {
        subject : Engine[1..*] = (a, b);
        in attribute offset : Integer;
        objective : MinimizeObjective;
        calc :>> evaluationFunction {
            in part e :>> alternative : Engine;
            return :>> result : Real = e.mass / (e.cylinders - offset);
        }
        return part :>> selectedAlternative : Engine;
    }
}
'''


@pytest.mark.integration
class TestTradeStudyIntegration:
    def setup_method(self):
        self.conn = Connection()
        self.model = self.conn.load_from_content(TRADE_STUDY_SOURCE)

    def teardown_method(self):
        self.conn.close()

    def test_service_reports_the_case_evaluations_capability(self):
        from opensysml.capabilities import CAPABILITY_CASE_EVALUATIONS

        assert self.conn.server_info().has(CAPABILITY_CASE_EVALUATIONS)

    def test_a_trade_study_evaluates_each_alternative_and_selects_the_best(self):
        result = self.model.run_analysis("Trade::lightest")
        assert result.satisfied
        assert [v.element for v in result.verdicts] == ["tradeStudyObjective"]
        selected = result.outputs["selectedAlternative"]
        assert selected.type_symbol_id == "Trade::b"
        assert [e.arguments[0].type_symbol_id for e in result.evaluations] == [
            "Trade::a", "Trade::b", "Trade::c",
        ]
        assert [e.result for e in result.evaluations] == [30.0, 10.0, 10.0]
        assert [e.selected for e in result.evaluations] == [False, True, False]
        assert [e.tied for e in result.evaluations] == [False, False, True]
        assert result.selected[0].arguments[0].id == selected.id
        assert all(e.function_id == "Trade::lightest::evaluationFunction" for e in result.evaluations)

    def test_a_failing_alternative_keeps_the_evaluations_made(self):
        with pytest.raises(AnalysisRunError) as exc_info:
            self.model.run_analysis("Trade::perCylinder")
        assert "division by zero" in str(exc_info.value)
        result = exc_info.value.result
        assert result.outputs == {}
        assert not result.verdicts[0].evaluated
        assert "division by zero" in result.verdicts[0].error
        assert [e.evaluated for e in result.evaluations] == [True, False]
        assert result.evaluations[0].result == 5.0
        assert "division by zero" in result.evaluations[1].error
        assert result.evaluations[1].arguments[0].type_symbol_id == "Trade::c"

    def test_a_swept_trade_study_reports_each_rows_evaluations(self):
        table = self.model.run_sweep("Trade::perOffset", {"offset": (3, 4)})
        assert [row.inputs["offset"] for row in table] == [3, 4]
        assert [row.failed for row in table] == [False, True]
        assert not table

        ok = table.rows[0]
        assert ok.outputs["selectedAlternative"].type_symbol_id == "Trade::a"
        assert [e.result for e in ok.evaluations] == [10.0, 10.0]
        assert [e.selected for e in ok.evaluations] == [True, False]
        assert [e.tied for e in ok.evaluations] == [False, True]
        assert ok.selected[0].arguments[0].id == ok.outputs["selectedAlternative"].id

        failed = table.rows[1]
        assert "division by zero" in failed.error
        assert failed.outputs == {}
        assert not failed.verdicts[0].evaluated
        assert [e.evaluated for e in failed.evaluations] == [True, False]
        assert failed.evaluations[0].result == 15.0
        assert "division by zero" in failed.evaluations[1].error
        assert failed.evaluations[1].arguments[0].type_symbol_id == "Trade::b"
        assert failed.selected == []

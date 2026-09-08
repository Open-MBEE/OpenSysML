"""Integration tests for parameter sweeps against a real service.

These are about the table the service actually produces — the rows it runs, the
order it runs them in, what a failing run reports, and that a seed reproduces a
sampled table — rather than about how the client wraps a canned response.
"""

import pytest

from opensysml import Connection
from opensysml.errors import ExecutionError, WrongKindError
from opensysml.verdict import SweepTable

MODEL_SOURCE = '''
package Sw {
    private import ScalarValues::*;

    part def Ship {
        attribute cost : Real default = 5.0;
    }

    calc def Twice {
        in n : Integer;
        return : Integer = n * 2;
    }

    calc def Ratio {
        in a : Real;
        in b : Real;
        return : Real = a / b;
    }

    calc def Plus {
        in a : Integer;
        in b : Integer;
        return : Integer = a + b;
    }

    analysis def CostAnalysis {
        subject s : Ship;
        in limit : Real = 20.0;
        out total : Real = s.cost * limit;
        objective affordable {
            require constraint { total <= 100.0 }
        }
    }

    part ship : Ship;
}
'''


@pytest.mark.integration
class TestSweepIntegration:
    def setup_method(self):
        self.conn = Connection()
        self.model = self.conn.load_from_content(MODEL_SOURCE)

    def teardown_method(self):
        self.conn.close()

    def test_an_integer_range_steps_by_one(self):
        table = self.model.run_sweep("Sw::Twice", {"n": (1, 4)})
        assert isinstance(table, SweepTable)
        assert table.parameters == ["n"]
        assert not table.sampled
        assert [row.inputs["n"] for row in table] == [1, 2, 3, 4]
        assert [row.outputs["result"] for row in table] == [2, 4, 6, 8]
        assert all(row.seconds >= 0 for row in table)
        assert not table.failures

    def test_a_stated_step_advances_by_it(self):
        table = self.model.run_sweep("Sw::Twice", {"n": (0, 6, 3)})
        assert [row.inputs["n"] for row in table] == [0, 3, 6]

    def test_several_ranges_run_their_cartesian_product(self):
        table = self.model.run_sweep("Sw::Plus", {"a": (1, 2), "b": (10, 11)})
        assert table.parameters == ["a", "b"]
        assert [(row.inputs["a"], row.inputs["b"]) for row in table] == [
            (1, 10), (1, 11), (2, 10), (2, 11),
        ]
        assert [row.outputs["result"] for row in table] == [11, 12, 12, 13]

    def test_a_failing_run_is_a_row_and_the_table_goes_on(self):
        table = self.model.run_sweep(
            "Sw::Ratio", {"b": (-1, 1)}, named_arguments={"a": 4.0}
        )
        assert [row.inputs["b"] for row in table] == [-1, 0, 1]
        assert [bool(row.failed) for row in table] == [False, True, False]
        assert "division by zero" in table.rows[1].error
        assert table.rows[1].outputs == {}
        assert [row.outputs.get("result") for row in table] == [-4.0, None, 4.0]
        assert len(table.failures) == 1
        assert not table

    def test_an_analysis_case_row_carries_its_verdict(self):
        table = self.model.run_sweep(
            "Sw::CostAnalysis", {"limit": (2.0, 30.0, 14.0)}, subject="Sw::ship"
        )
        assert [row.inputs["limit"] for row in table] == [2.0, 16.0, 30.0]
        assert [row.outputs["total"] for row in table] == [10.0, 80.0, 150.0]
        assert [v.kind for row in table for v in row.verdicts] == ["objective"] * 3
        assert [row.verdicts[0].holds for row in table] == [True, True, False]
        assert not table

    def test_the_same_seed_draws_the_same_table(self):
        first = self.model.run_sweep("Sw::Twice", {"n": (1, 100)}, samples=8, seed=7)
        again = self.model.run_sweep("Sw::Twice", {"n": (1, 100)}, samples=8, seed=7)
        other = self.model.run_sweep("Sw::Twice", {"n": (1, 100)}, samples=8, seed=8)
        drawn = [row.inputs["n"] for row in first]
        assert drawn == [row.inputs["n"] for row in again]
        assert drawn != [row.inputs["n"] for row in other]
        assert first.sampled and first.seed == 7
        assert len(drawn) == 8

    def test_a_parameter_the_target_does_not_declare_raises(self):
        with pytest.raises(ExecutionError) as exc_info:
            self.model.run_sweep("Sw::Twice", {"nope": (1, 2)})
        assert "nope" in str(exc_info.value)

    def test_a_parameter_both_bound_and_swept_raises(self):
        with pytest.raises(ExecutionError):
            self.model.run_sweep(
                "Sw::Twice", {"n": (1, 2)}, named_arguments={"n": 3}
            )

    def test_a_real_range_without_a_step_raises(self):
        with pytest.raises(ExecutionError) as exc_info:
            self.model.run_sweep(
                "Sw::Ratio", {"b": (1.0, 2.0)}, named_arguments={"a": 1.0}
            )
        assert "step" in str(exc_info.value)

    def test_a_zero_step_raises(self):
        with pytest.raises(ExecutionError):
            self.model.run_sweep("Sw::Twice", {"n": (1, 4, 0)})

    def test_a_step_that_never_reaches_the_end_raises(self):
        with pytest.raises(ExecutionError):
            self.model.run_sweep("Sw::Twice", {"n": (1, 4, -1)})

    def test_no_range_raises(self):
        with pytest.raises(ExecutionError):
            self.model.run_sweep("Sw::Twice", {})

    def test_a_sampled_range_stating_a_step_raises(self):
        with pytest.raises(ExecutionError):
            self.model.run_sweep("Sw::Twice", {"n": (1, 9, 2)}, samples=3, seed=1)

    def test_a_wrong_kind_raises(self):
        with pytest.raises(WrongKindError):
            self.model.run_sweep("Sw::ship", {"n": (1, 2)})

    def test_a_calc_given_a_subject_raises(self):
        with pytest.raises(ExecutionError) as exc_info:
            self.model.run_sweep("Sw::Twice", {"n": (1, 2)}, subject="Sw::ship")
        assert "subject" in str(exc_info.value)

    def test_a_budget_refusal_names_the_variable(self):
        with pytest.raises(ExecutionError) as exc_info:
            self.model.run_sweep("Sw::Twice", {"n": (1, 100000)})
        assert "OPENSYSML_MAX_SWEEP_RUNS" in str(exc_info.value)

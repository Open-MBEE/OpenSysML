"""Integration tests for the model surface against a real service.

Covers evaluation on the model itself and the wrong-kind requests the service
now classifies, since both are about what the service actually answers rather
than about how the client wraps a canned response.
"""

import pytest

from opensysml import Connection
from opensysml.errors import ExecutionError, ModelNotFoundError, WrongKindError
from opensysml.model import Model
from opensysml.proto import sysml_pb2

MODEL_SOURCE = '''
package Demo {
    part def Vehicle {
        attribute mass default = 1500.0;
        constraint massPositive {
            assert mass > 0.0;
        }
        constraint massLight {
            assert mass < 100.0;
        }
        requirement lightEnough {
            require constraint { mass < 2000.0 }
        }
    }

    part sedan : Vehicle {
        attribute :>> mass = 1200.0;
    }

    calc add {
        in x;
        in y;
        x + y
    }
}
'''


@pytest.mark.integration
class TestModelSurfaceIntegration:
    def setup_method(self):
        self.conn = Connection()
        self.model = self.conn.load_from_content(MODEL_SOURCE)

    def teardown_method(self):
        self.conn.close()

    def test_eval_on_the_model(self):
        assert self.model.eval("1+1") == 2

    def test_eval_in_a_context(self):
        assert self.model.eval("mass", context_symbol_id="Demo::sedan") == 1200.0

    def test_eval_against_a_subject_reads_that_object(self):
        # The object's redefinition wins over the definition's default, the way
        # %eval does after %instantiate.
        assert self.model.eval("mass", context_symbol_id="Demo::Vehicle") == 1500.0
        assert self.model.eval("mass", subject="Demo::sedan") == 1200.0
        assert self.model.eval("mass * 2", subject="Demo::sedan") == 2400.0

    def test_eval_against_a_subject_in_a_named_context(self):
        assert (
            self.model.eval(
                "mass",
                context_symbol_id="Demo::Vehicle",
                subject="Demo::sedan",
            )
            == 1200.0
        )

    def test_eval_raises_for_an_unknown_subject(self):
        with pytest.raises(ExecutionError):
            self.model.eval("mass", subject="Demo::nope")

    @pytest.mark.parametrize("expression", ["1/0", "nope", '1 + "a"'])
    def test_eval_raises_for_an_expression_it_cannot_evaluate(self, expression):
        with pytest.raises(ExecutionError):
            self.model.eval(expression)

    def test_eval_raises_when_the_service_no_longer_holds_the_model(self):
        # A model whose hash the service's bounded cache has evicted.
        evicted = Model(
            sysml_pb2.ParseFileResponse(
                model_hash="0" * 64,
                root=sysml_pb2.SymbolInfo(id="Demo", name="Demo", kind="Package"),
            ),
            self.conn,
        )
        with pytest.raises(ModelNotFoundError):
            evicted.eval("1+1")

    def test_a_verdict_is_still_a_verdict(self):
        assert self.model.verify_constraint(
            "Demo::Vehicle::massPositive", subject="Demo::sedan"
        ).holds
        assert self.model.verify_constraint(
            "Demo::Vehicle::massLight", subject="Demo::sedan"
        ).holds is False
        assert self.model.verify_requirement(
            "Demo::Vehicle::lightEnough", subject="Demo::sedan"
        ).holds

    def test_a_wrong_kind_verification_raises(self):
        for call in (
            lambda: self.model.verify_constraint("Demo::Vehicle"),
            lambda: self.model.verify_requirement("Demo::Vehicle"),
            lambda: self.model.calc("Demo::Vehicle", arguments=[1]),
        ):
            with pytest.raises(WrongKindError):
                call()

    def test_an_unknown_symbol_still_raises(self):
        for call in (
            lambda: self.model.verify_constraint("Demo::Nope"),
            lambda: self.model.verify_requirement("Demo::Nope"),
            lambda: self.model.verify_satisfaction("Demo::Nope"),
            lambda: self.model.calc("Demo::Nope", arguments=[1]),
        ):
            with pytest.raises(ExecutionError):
                call()

    def test_an_element_stating_no_assertion_still_answers_with_none(self):
        assert self.model.verify_satisfaction("Demo::Vehicle") == []

    def test_validating_an_object_answers_every_assertion_in_its_tree(self):
        model = self.conn.load_from_content('''
            package Fleet {
                part def Wheel {
                    attribute pressure default = 32.0;
                    assert constraint pressureOk { pressure >= 30.0 }
                }
                part def Car {
                    attribute mass = 1500.0;
                    part wheels : Wheel[2] {
                        attribute :>> pressure = 20.0;
                    }
                    assert constraint massOk { mass < 2000.0 }
                    requirement light { require constraint { mass < 1000.0 } }
                }
                part car : Car;
                part spare : Wheel;
                part def Crate;
                part crate : Crate;
            }
        ''')

        validation = model.validate_instance("Fleet::car")
        assert not validation
        assert validation.summary.kind == "object"
        assert validation.summary.element_id == "Fleet::car"
        assert validation.bounded is False
        assert validation.undecided == []
        answered = sorted(
            (v.element_id, v.instance_path, v.holds) for v in validation
        )
        assert answered == [
            ("Fleet::Car::light", "", False),
            ("Fleet::Car::massOk", "", True),
            ("Fleet::Wheel::pressureOk", "wheels[1]", False),
            ("Fleet::Wheel::pressureOk", "wheels[2]", False),
        ]
        assert len(validation.violated) == 3

        spare = model.validate_instance("Fleet::spare")
        assert spare.valid
        assert [(v.element_id, v.instance_path) for v in spare] == [
            ("Fleet::Wheel::pressureOk", ""),
        ]

        # Nothing was decided about an object no assertion is about, so it
        # is neither valid nor violated.
        crate = model.validate_instance("Fleet::crate")
        assert len(crate) == 0
        assert not crate.valid
        assert crate.violated == []
        assert "states no assertion" in crate.summary.error

        # A package and an attribute have no object to validate.
        for symbol in ("Fleet", "Fleet::Car::mass"):
            with pytest.raises(ExecutionError, match="no object to validate"):
                model.validate_instance(symbol)

    def test_validating_an_unknown_object_raises(self):
        with pytest.raises(ExecutionError):
            self.model.validate_instance("Demo::Nope")

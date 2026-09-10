"""Tests for the quantity value: decoding, comparison and arithmetic.

Two layers. Against protobuf messages built by hand, what a quantity is and how
it behaves; against the real ``sysml-grpc`` binary, that a model's quantity slots
— written, derived, compound-unit and nested — read back as quantities.
"""

from unittest.mock import Mock, patch

import pytest

from opensysml.errors import ExecutionError, FeatureValueError, UnsupportedValueError
from opensysml.instance import Instance
from opensysml.proto import sysml_pb2
from opensysml.values import (
    IncommensurableUnitsError,
    Quantity,
    Unit,
    UnitFactor,
    value_to_python,
)
from tests.service_gate import skip_or_fail_without_service

# Reductions of the units these tests measure with, as the service sends them.
METRE = (("SI::metre", 1.0),)
SECOND = (("SI::second", 1.0),)
GRAM = (("SI::gram", 1.0),)
METRE_PER_SECOND = (("SI::metre", 1.0), ("SI::second", -1.0))


def unit(text, factors=METRE, scale_num=1.0, scale_den=1.0):
    """A unit as written, over the base units it reduces to."""
    return Unit(
        text=text,
        scale_num=scale_num,
        scale_den=scale_den,
        factors=tuple(UnitFactor(unit_id, exponent) for unit_id, exponent in factors),
    )


def pb_quantity(unit_text, factors=METRE, scale_num=1.0, scale_den=1.0, **magnitude):
    """A Quantity message as the service sends one."""
    return sysml_pb2.Quantity(
        unit=unit_text,
        unit_term=sysml_pb2.UnitTerm(
            scale_num=scale_num,
            scale_den=scale_den,
            factors=[
                sysml_pb2.UnitFactor(unit_id=unit_id, exponent=exponent)
                for unit_id, exponent in factors
            ],
        ),
        **magnitude,
    )


def test_a_quantity_decodes_with_its_magnitude_and_unit_as_written():
    """The magnitude arrives in the unit written, not reduced to base units."""
    value = sysml_pb2.Value(
        quantity=pb_quantity(
            "SI::km/SI::h",
            factors=METRE_PER_SECOND,
            scale_num=5.0,
            scale_den=18.0,
            real_magnitude=5.4,
        )
    )

    got = value_to_python(value)

    assert isinstance(got, Quantity)
    assert got.magnitude == 5.4
    assert got.unit.text == "SI::km/SI::h"
    assert got.unit.exponents() == {"SI::metre": 1.0, "SI::second": -1.0}
    assert str(got) == "5.4 [SI::km/SI::h]"


def test_an_integer_magnitude_stays_an_integer():
    """Integer and Real are distinct in the model, so they are here too."""
    got = value_to_python(sysml_pb2.Value(quantity=pb_quantity("SI::m", int_magnitude=3)))

    assert got.magnitude == 3
    assert isinstance(got.magnitude, int)
    assert str(got) == "3 [SI::m]"


def test_a_quantity_with_no_magnitude_is_reported():
    """A magnitude-less quantity is a value the client cannot read, not a zero."""
    value = sysml_pb2.Value(quantity=sysml_pb2.Quantity(unit="SI::m"))

    with pytest.raises(UnsupportedValueError, match="carries no magnitude"):
        value_to_python(value)


def test_a_named_unit_with_no_reduction_is_reported():
    """Reading an unreduced unit as dimension one would relate it to a bare count."""
    named = sysml_pb2.Value(
        quantity=sysml_pb2.Quantity(unit="Furlongs::furlong", real_magnitude=5.0)
    )
    with pytest.raises(UnsupportedValueError, match="no reduction to base units"):
        value_to_python(named)

    # A magnitude under no unit at all is dimension one, which is what it means.
    unnamed = value_to_python(sysml_pb2.Value(quantity=sysml_pb2.Quantity(real_magnitude=5.0)))
    assert unnamed.unit.dimensionless


def test_a_quantity_encodes_as_the_message_the_service_decodes():
    """What is sent is what is read back: magnitude in the unit written, reduced."""
    kmh = Quantity(5.4, unit("SI::km/SI::h", METRE_PER_SECOND, 5.0, 18.0))

    sent = kmh.to_pb()

    assert sent.unit == "SI::km/SI::h"
    assert sent.WhichOneof('magnitude') == 'real_magnitude'
    assert sent.real_magnitude == 5.4
    assert (sent.unit_term.scale_num, sent.unit_term.scale_den) == (5.0, 18.0)
    assert [(f.unit_id, f.exponent) for f in sent.unit_term.factors] == list(METRE_PER_SECOND)
    assert Quantity.from_pb(sent) == kmh


def test_an_integer_magnitude_is_sent_as_an_integer():
    """Integer and Real stay apart on the way out, as they do on the way in."""
    sent = Quantity(3, unit("SI::m")).to_pb()

    assert sent.WhichOneof('magnitude') == 'int_magnitude'
    assert sent.int_magnitude == 3
    assert isinstance(Quantity.from_pb(sent).magnitude, int)


def test_a_dimensionless_quantity_is_sent_as_dimension_one():
    """A magnitude under no unit means dimension one, and says so on the wire."""
    sent = Quantity(2.0, Unit()).to_pb()

    assert sent.unit == ""
    assert sent.HasField('unit_term')
    assert list(sent.unit_term.factors) == []
    assert Quantity.from_pb(sent).unit.dimensionless


def test_an_unreduced_unit_is_refused_before_it_is_sent():
    """A unit named with no reduction measures nothing the service can relate."""
    furlongs = Quantity(5.0, Unit(text="Furlongs::furlong"))
    with pytest.raises(UnsupportedValueError, match="no reduction to base units"):
        furlongs.to_pb()

    assert not Unit(text="Furlongs::furlong").reduced
    assert Unit(text="SI::m", factors=(UnitFactor("SI::metre", 1.0),)).reduced
    # A named unit that is a bare scale — a percentage — carries a reduction.
    assert Unit(text="Percent::percent", scale_num=1.0, scale_den=100.0).reduced


def test_a_named_unit_that_reduces_to_dimension_one_is_sent_back():
    """``SI::rad`` is ``m/m``: reduced, and told apart from an absent reduction
    by the reduction having been given rather than by what it reduces to."""
    radians = Quantity.from_pb(
        sysml_pb2.Quantity(
            unit="SI::rad",
            unit_term=sysml_pb2.UnitTerm(scale_num=1.0, scale_den=1.0),
            real_magnitude=1.5,
        )
    )

    assert radians.unit.reduced
    assert radians.unit.dimensionless
    assert not Unit(text="SI::rad").reduced

    sent = radians.to_pb()

    assert sent.unit == "SI::rad"
    assert sent.HasField('unit_term')
    assert list(sent.unit_term.factors) == []
    assert Quantity.from_pb(sent) == radians


def test_a_magnitude_that_is_not_a_number_is_refused():
    """Booleans and strings are not magnitudes, however they cast."""
    for magnitude in (True, "5.0", None):
        quantity = Quantity(magnitude, unit("SI::m"))
        with pytest.raises(UnsupportedValueError, match="neither an Integer nor a Real"):
            quantity.to_pb()


def test_a_quantity_argument_is_encoded_as_a_quantity_value():
    """The request carries the quantity itself, not a bare magnitude."""
    from opensysml import Connection

    with patch('grpc.insecure_channel'), \
            patch('opensysml.proto.sysml_pb2_grpc.SysMLServiceStub') as stub_cls:
        stub_cls.return_value = Mock()
        conn = Connection(auto_start=False)

        mass = Quantity(5.0, unit("SI::kg", GRAM, scale_num=1000.0))
        value = conn._python_to_value(mass)
        nested = conn._python_to_value([mass])

        assert value.WhichOneof('kind') == 'quantity'
        assert Quantity.from_pb(value.quantity) == mass
        assert Quantity.from_pb(nested.sequence.elements[0].quantity) == mass


def test_a_quantity_slot_reads_off_an_instance():
    """The slot path returns the quantity, which is the whole point of the wire form."""
    pb_inst = sysml_pb2.Instance(
        id=1,
        type_symbol_id="P::Car",
        feature_values={
            "m": sysml_pb2.FeatureValue(
                feature_name="m",
                materialized=True,
                value=sysml_pb2.Value(
                    quantity=pb_quantity(
                        "SI::kg", factors=GRAM, scale_num=1000.0, real_magnitude=5.0
                    )
                ),
            ),
            "n": sysml_pb2.FeatureValue(
                feature_name="n",
                materialized=True,
                value=sysml_pb2.Value(real_value=2.0),
            ),
        },
    )

    inst = Instance(pb_inst)

    assert inst.m == Quantity(5.0, unit("SI::kg", GRAM, scale_num=1000.0))
    assert inst.n == 2.0


def test_a_quantity_in_a_collection_slot_decodes_per_element():
    """Every element of a collection slot is decoded, quantities included."""
    pb_slot = sysml_pb2.FeatureValue(
        feature_name="masses",
        materialized=True,
        values=[
            sysml_pb2.Value(quantity=pb_quantity("SI::m", real_magnitude=1.0)),
            sysml_pb2.Value(quantity=pb_quantity("SI::m", real_magnitude=2.0)),
        ],
    )
    pb_inst = sysml_pb2.Instance(id=1, type_symbol_id="P::Car", feature_values={"masses": pb_slot})

    assert Instance(pb_inst).masses == [
        Quantity(1.0, unit("SI::m")),
        Quantity(2.0, unit("SI::m")),
    ]


def test_commensurable_quantities_compare_over_their_reduction():
    """`5.4 [km/h]` and `1.5 [m/s]` are one value, exactly, at the boundary."""
    kmh = Quantity(5.4, unit("SI::km/SI::h", METRE_PER_SECOND, 5.0, 18.0))
    ms = Quantity(1.5, unit("SI::m/SI::s", METRE_PER_SECOND))

    assert kmh == ms
    assert not kmh < ms
    assert kmh <= ms
    assert kmh >= ms
    assert Quantity(3.6, unit("SI::km/SI::h", METRE_PER_SECOND, 5.0, 18.0)) < ms
    assert hash(kmh) == hash(ms)


def test_a_reduction_compares_whatever_order_its_factors_arrive_in():
    """Commensurability is over base unit → exponent, not the order sent."""
    reversed_factors = (("SI::second", -1.0), ("SI::metre", 1.0))
    cancelling = (("SI::metre", 2.0), ("SI::metre", -1.0), ("SI::second", -1.0))

    speed = Quantity(1.5, unit("SI::m/SI::s", METRE_PER_SECOND))

    assert speed == Quantity(1.5, unit("m/s", reversed_factors))
    assert speed == Quantity(1.5, unit("m/s", cancelling))


def test_a_converted_magnitude_stays_exact():
    """The conversion applies as one ratio, so 5.4 km/h is 1.5 m/s and not 1.4999…"""
    kmh = Quantity(5.4, unit("SI::km/SI::h", METRE_PER_SECOND, 5.0, 18.0))

    assert kmh.in_unit(unit("SI::m/SI::s", METRE_PER_SECOND)) == 1.5
    assert kmh.to(unit("SI::m/SI::s", METRE_PER_SECOND)).magnitude == 1.5


def test_incommensurable_ordering_is_an_error_not_a_comparison():
    """Ordering quantities that measure different things has no answer."""
    mass = Quantity(5.0, unit("SI::kg", GRAM, scale_num=1000.0))
    length = Quantity(2.0, unit("SI::m"))

    with pytest.raises(IncommensurableUnitsError) as excinfo:
        mass < length
    assert "SI::kg" in str(excinfo.value)
    assert "SI::m" in str(excinfo.value)

    for compare in (lambda: mass > length, lambda: mass <= length, lambda: mass >= length):
        with pytest.raises(IncommensurableUnitsError):
            compare()

    # Equality is not an error: two values measuring different things are simply
    # not the same value. What it must never do is compare bare magnitudes.
    assert mass != length
    assert Quantity(5.0, unit("SI::kg", GRAM, scale_num=1000.0)) != Quantity(5.0, unit("SI::m"))


def test_a_quantity_does_not_order_against_a_bare_number():
    """A bare number carries no unit, so it is not comparable with a quantity."""
    metres = Quantity(5.0, unit("SI::m"))
    with pytest.raises(TypeError, match="carries no unit"):
        metres < 5.0
    assert metres != 5.0


def test_addition_answers_in_the_left_unit_and_rejects_incommensurable_units():
    """A sum converts the right operand, and has no answer across dimensions."""
    metres = Quantity(1.0, unit("SI::m"))
    kilometres = Quantity(1.0, unit("SI::km", METRE, scale_num=1000.0))

    assert metres + kilometres == Quantity(1001.0, unit("SI::m"))
    assert (metres + kilometres).unit.text == "SI::m"
    assert kilometres - metres == Quantity(0.999, unit("SI::km", METRE, scale_num=1000.0))

    seconds = Quantity(1.0, unit("SI::s", SECOND))

    # The error names the operation the caller wrote, not the shared helper's.
    with pytest.raises(IncommensurableUnitsError, match="cannot add"):
        metres + seconds
    with pytest.raises(IncommensurableUnitsError, match="cannot subtract"):
        metres - seconds


def test_scaling_keeps_the_unit_and_negation_the_sign():
    """A quantity times a number is the same measurement, scaled."""
    speed = Quantity(1.5, unit("SI::m/SI::s", METRE_PER_SECOND))

    assert speed * 2 == Quantity(3.0, unit("SI::m/SI::s", METRE_PER_SECOND))
    assert 2 * speed == speed * 2
    assert speed / 2 == Quantity(0.75, unit("SI::m/SI::s", METRE_PER_SECOND))
    assert -speed == Quantity(-1.5, unit("SI::m/SI::s", METRE_PER_SECOND))
    assert abs(-speed) == speed

    length = Quantity(2.0, unit("SI::m"))
    with pytest.raises(TypeError):
        speed * length


def test_a_unit_renders_as_written_and_reduced():
    """A unit reads as the model wrote it; its reduction names base units."""
    kmh = unit("SI::km/SI::h", METRE_PER_SECOND, 5.0, 18.0)

    assert str(kmh) == "SI::km/SI::h"
    assert kmh.reduction() == "5/18·SI::metre·SI::second^-1"
    assert str(unit("", ())) == "1"
    assert unit("", ()).dimensionless
    assert unit("SI::m").reduction() == "SI::metre"

    # Powers that cancel measure nothing, however the factors were written.
    cancelled = unit(
        "SI::m*SI::m/SI::m/SI::m",
        (("SI::metre", 2.0), ("SI::metre", -2.0)),
    )
    assert cancelled.dimensionless
    assert cancelled.commensurable(unit("", ()))


def test_a_slot_that_failed_is_still_an_error():
    """A quantity slot the service could not evaluate reports, not returns."""
    pb_inst = sysml_pb2.Instance(
        id=1,
        type_symbol_id="P::Car",
        feature_values={
            "m": sysml_pb2.FeatureValue(
                feature_name="m", materialized=True, error="incommensurable units"
            )
        },
    )

    with pytest.raises(FeatureValueError, match="incommensurable units"):
        Instance(pb_inst).m


QUANTITY_MODEL = """
package Q {
    part def Engine {
        attribute power : ISQ::PowerValue = 300.0 [SI::W];
    }

    part def Car {
        attribute mass : ISQ::MassValue = 1200.0 [SI::kg];
        attribute plainMass : ScalarValues::Real = 2.0;
        attribute derivedSpeed = 10.0 [SI::m] / 2.0 [SI::s];
        attribute writtenSpeed = 5.4 [SI::km/SI::h];
        // A ratio unit: SI declares rad as m/m, so it reduces to dimension one.
        attribute turn : ISQ::AngularMeasureValue = 1.5 [SI::rad];
        attribute length = 3 [SI::m];
        part engine : Engine;

        constraint withinLimit {
            mass <= 2000.0 [SI::kg]
        }
    }

    calc echo {
        in q;
        q
    }

    calc overHalfATonne {
        in q;
        q > 500.0 [SI::kg]
    }

    action addHalfATonne {
        attribute m = 0.0 [SI::kg];
        first start;
        action inner {
            assign m := m + 500.0 [SI::kg];
        }
        done;
        succession first start then inner;
        succession first inner then done;
    }
}
"""


@pytest.mark.integration
class TestQuantityAgainstTheService:
    """What a caller actually gets back from the real service for a real model."""

    def setup_method(self):
        import grpc

        from opensysml import Connection

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
        self.model = self.conn.load_from_content(QUANTITY_MODEL)

    def teardown_method(self):
        if getattr(self, "conn", None) is not None:
            self.conn.close()

    def test_written_derived_and_compound_unit_slots_read_as_quantities(self):
        """Written slots read back in their unit; computed ones in the coherent unit."""
        car = self.conn.instantiate("Q::Car", self.model.hash)

        assert car.mass == Quantity(1200.0, unit("SI::kg", GRAM, scale_num=1000.0))
        assert car.plainMass == 2.0
        assert str(car.derivedSpeed) == "5 [SI::'m/s']"
        assert str(car.writtenSpeed) == "5.4 [SI::km/SI::h]"
        assert car.length.magnitude == 3
        assert isinstance(car.length.magnitude, int)

        # A compound unit compares across its scale rather than by magnitude.
        assert car.writtenSpeed == Quantity(1.5, car.derivedSpeed.unit)
        with pytest.raises(IncommensurableUnitsError):
            car.mass < car.writtenSpeed

    def test_a_nested_object_carries_its_own_quantity(self):
        """`inst.engine.power` is a quantity, not an unreadable slot."""
        car = self.conn.instantiate("Q::Car", self.model.hash)

        assert str(car.engine.power) == "300 [SI::W]"
        assert car.engine.power.unit.exponents() == {
            "SI::gram": 1.0,
            "SI::metre": 2.0,
            "SI::second": -3.0,
        }

    def test_an_evaluated_quantity_expression_reads_as_a_quantity(self):
        """eval() of a quantity expression answers in the coherent unit of its dimension."""
        speed = self.conn.eval("10.0 [SI::m] / 4.0 [SI::s]", self.model.hash)

        assert speed == Quantity(2.5, speed.unit)
        assert speed.unit.text == "SI::'m/s'"

    def test_a_quantity_sent_as_a_calc_argument_round_trips(self):
        """Send → evaluate → read back: the magnitude and the unit as written."""
        mass = self.conn.instantiate("Q::Car", self.model.hash).mass

        echoed = self.conn.calc("Q::echo", self.model.hash, arguments=[mass]).value

        assert isinstance(echoed, Quantity)
        assert echoed.magnitude == mass.magnitude
        assert echoed.unit.text == mass.unit.text == "SI::kg"
        assert echoed.unit.exponents() == {"SI::gram": 1.0}
        assert echoed == mass

        # An integer magnitude sent stays an integer, in its own unit.
        length = Quantity(3, unit("SI::m"))
        back = self.conn.calc("Q::echo", self.model.hash, arguments=[length]).value
        assert back.magnitude == 3
        assert isinstance(back.magnitude, int)
        assert back.unit.text == "SI::m"

    def test_a_quantity_in_a_ratio_unit_round_trips(self):
        """An angle read off the wire goes back out: dimension one is a reduction."""
        turn = self.conn.instantiate("Q::Car", self.model.hash).turn

        assert turn.unit.text == "SI::rad"
        assert turn.unit.dimensionless

        echoed = self.conn.calc("Q::echo", self.model.hash, arguments=[turn]).value

        assert echoed == turn
        assert echoed.magnitude == 1.5
        assert echoed.unit.text == "SI::rad"

    def test_a_sent_quantity_is_commensurable_with_the_models_own_units(self):
        """The reduction sent is what the service compares against, not the magnitude."""
        kilograms = Quantity(1200.0, unit("SI::kg", GRAM, scale_num=1000.0))
        grams = Quantity(400.0, unit("SI::g", GRAM))

        assert self.conn.calc("Q::overHalfATonne", self.model.hash, arguments=[kilograms]).value
        assert not self.conn.calc("Q::overHalfATonne", self.model.hash, arguments=[grams]).value

    def test_a_quantity_input_binds_into_an_action(self):
        """An action input given as a quantity is added to, not discarded."""
        base = self.conn.execute_action("Q::addHalfATonne", self.model.hash)
        assert base["m"] == Quantity(500.0, unit("SI::kg", GRAM, scale_num=1000.0))

        out = self.conn.execute_action(
            "Q::addHalfATonne",
            self.model.hash,
            inputs={"m": Quantity(1200.0, unit("SI::kg", GRAM, scale_num=1000.0))},
        )

        assert out["m"] == Quantity(1700.0, unit("SI::kg", GRAM, scale_num=1000.0))
        assert out["m"].unit.text == "SI::kg"

    def test_a_unit_the_model_does_not_declare_is_refused_with_its_name(self):
        """A base unit no model declares is an error naming it, never a bare number."""
        invented = Quantity(5.0, unit("Furlongs::furlong", (("Furlongs::furlong", 1.0),)))

        with pytest.raises(ExecutionError) as excinfo:
            self.conn.calc("Q::echo", self.model.hash, arguments=[invented])
        assert "Furlongs::furlong" in str(excinfo.value)

    def test_a_verdict_over_quantities_carries_the_quantity_it_is_about(self):
        """A constraint over quantities holds, and its subject's slot is readable."""
        verdict = self.conn.verify_constraint(
            "Q::Car::withinLimit", self.model.hash, subject_symbol_id="Q::Car"
        )

        assert verdict.holds
        subject = next(i for i in verdict.instances if i.id == verdict.instance_id)
        assert subject.mass == Quantity(1200.0, unit("SI::kg", GRAM, scale_num=1000.0))

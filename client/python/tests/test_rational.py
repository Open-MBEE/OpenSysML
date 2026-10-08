"""KerML Rationals are exact, and cross the wire exactly.

The service sends a Rational a double holds exactly, as ``0.5``, as
``real_value`` and any other, as ``1/3`` or ``0.1``, as the numerator and
denominator of ``rational_value``; the client decodes the latter to a
``fractions.Fraction``. The client sends every ``Fraction`` as
``rational_value`` to a service with ``rational_values``, one a double holds
included, since the service reads an inbound ``real_value`` as a Real.
"""

from fractions import Fraction
from unittest.mock import Mock, patch

import pytest

from opensysml.capabilities import (
    CAPABILITY_DOCUMENT_QUERY,
    CAPABILITY_RATIONAL_VALUES,
    CAPABILITY_SET_VALUES,
    CAPABILITY_STRUCTURED_VALUES,
    CAPABILITY_TENSOR_VALUES,
    CAPABILITY_VERIFICATION,
    MissingCapabilityError,
)
from opensysml.connection import Connection
from opensysml.document import binding_rationals_as_reals, build_bindings, _value_of
from opensysml.errors import UnsupportedValueError
from opensysml.proto import sysml_pb2
from tests.service_gate import skip_or_fail_without_service
from opensysml.values import (
    Quantity,
    TensorQuantity,
    Unit,
    UnitFactor,
    Vector,
    VectorQuantity,
    format_rational,
    holds_exactly_as_double,
    rational_from_pb,
    rational_value_to_pb,
    rationals_as_reals,
    value_to_python,
)

KG = Unit(text="kg", factors=(UnitFactor("SI::kg", 1),), reduction_given=True)


@pytest.mark.parametrize("value", [Fraction(1, 3), Fraction(1, 10), Fraction(-7, 3), Fraction(1, 3 ** 200)])
def test_a_rational_no_double_holds_round_trips_exactly(value):
    pb = rational_value_to_pb(value)
    assert pb.WhichOneof("kind") == "rational_value"
    assert pb.rational_value.numerator == str(value.numerator)
    assert pb.rational_value.denominator == str(value.denominator)
    assert value_to_python(sysml_pb2.Value.FromString(pb.SerializeToString())) == value


@pytest.mark.parametrize("value", [Fraction(1, 2), Fraction(-3, 4), Fraction(5), Fraction(0)])
def test_a_rational_a_double_holds_is_sent_as_rational_value(value):
    pb = rational_value_to_pb(value)
    assert pb.WhichOneof("kind") == "rational_value"
    assert (pb.rational_value.numerator, pb.rational_value.denominator) == (str(value.numerator), str(value.denominator))


@pytest.mark.parametrize("value", [Fraction(1, 2), Fraction(-3, 4), Fraction(5), Fraction(0)])
def test_a_rational_a_double_holds_is_that_double_for_an_older_service(value):
    pb = rational_value_to_pb(value)
    rationals_as_reals(pb)
    assert pb.WhichOneof("kind") == "real_value"
    assert pb.real_value == float(value)


def test_a_rational_no_double_holds_stays_rational_for_an_older_service():
    pb = rational_value_to_pb(Fraction(1, 3))
    rationals_as_reals(pb)
    assert pb.WhichOneof("kind") == "rational_value"


@pytest.mark.parametrize(
    "value, text",
    [(Fraction(27, 5), "5.4"), (Fraction(-1, 8), "-0.125"), (Fraction(1, 1000), "0.001"),
     (Fraction(5), "5"), (Fraction(1, 3), "1/3"), (Fraction(-7, 6), "-7/6")],
)
def test_a_rational_prints_as_the_service_prints_it(value, text):
    assert format_rational(value) == text
    assert str(Quantity(value, KG)) == f"{text} [kg]"


def test_a_rational_beyond_the_double_range_is_not_a_double():
    assert not holds_exactly_as_double(Fraction(10 ** 400))
    assert rational_value_to_pb(Fraction(10 ** 400)).WhichOneof("kind") == "rational_value"


@pytest.mark.parametrize(
    "numerator, denominator, reason",
    [
        ("2", "6", "lowest terms"),
        ("1", "-3", "positive denominator"),
        ("1", "0", "positive denominator"),
        ("1", "2", "travels as real_value"),
        ("1.5", "3", "not decimal"),
    ],
)
def test_a_noncanonical_rational_is_refused(numerator, denominator, reason):
    with pytest.raises(UnsupportedValueError, match=reason):
        rational_from_pb(sysml_pb2.Rational(numerator=numerator, denominator=denominator))


def test_a_rational_quantity_magnitude_round_trips():
    pb = Quantity(Fraction(1, 3), KG).to_pb()
    assert pb.WhichOneof("magnitude") == "rational_magnitude"
    assert Quantity.from_pb(pb).magnitude == Fraction(1, 3)
    half = Quantity(Fraction(1, 2), KG).to_pb()
    assert (half.rational_magnitude.numerator, half.rational_magnitude.denominator) == ("1", "2")


def test_rational_quantities_compare_exactly():
    gram = Unit(text="g", factors=(UnitFactor("SI::kg", 1),), scale_num=1, scale_den=1000, reduction_given=True)
    assert Quantity(Fraction(1, 3), KG) == Quantity(Fraction(1000, 3), gram)


def test_a_rational_vector_component_round_trips():
    pb = Vector((Fraction(1, 3), 2)).to_pb()
    assert pb.components[0].WhichOneof("kind") == "rational_value"
    assert Vector.from_pb(pb).components == (Fraction(1, 3), 2)


def test_a_rational_document_value_round_trips():
    (binding,) = build_bindings({"x": Fraction(1, 3)})
    assert binding.values[0].WhichOneof("kind") == "rational_value"
    assert _value_of(binding.values[0]) == Fraction(1, 3)
    (half,) = build_bindings({"x": Fraction(1, 2)})
    assert half.values[0].WhichOneof("kind") == "rational_value"
    binding_rationals_as_reals(half)
    assert half.values[0].real_value == 0.5


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


OLD = (
    CAPABILITY_SET_VALUES,
    CAPABILITY_STRUCTURED_VALUES,
    CAPABILITY_TENSOR_VALUES,
    CAPABILITY_VERIFICATION,
)


def test_a_rational_is_not_sent_to_a_service_without_the_capability():
    """An older service would read the unknown arm as null, so nothing is sent."""
    third = Quantity(Fraction(1, 3), KG)
    stub = Mock()
    conn = make_connection(stub, OLD)
    for value in (
        Fraction(1, 3),
        [1, [Fraction(1, 10)]],
        third,
        Vector((1, Fraction(1, 3))),
        VectorQuantity((third,)),
        TensorQuantity((1,), (third,)),
    ):
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.calc("W::f", "hash", arguments=[value])
        assert excinfo.value.capability == CAPABILITY_RATIONAL_VALUES
    stub.EvaluateCalc.assert_not_called()
    assert conn._python_to_value(Fraction(1, 2)).real_value == 0.5
    assert conn._python_to_value([Fraction(1, 2)]).sequence.elements[0].real_value == 0.5
    assert conn._python_to_value(Quantity(Fraction(1, 2), KG)).quantity.real_magnitude == 0.5


def test_a_rational_is_sent_to_a_service_with_the_capability():
    conn = make_connection(Mock(), OLD + (CAPABILITY_RATIONAL_VALUES,))
    pb = conn._python_to_value([1, Fraction(1, 3)]).sequence.elements[1]
    assert (pb.rational_value.numerator, pb.rational_value.denominator) == ("1", "3")
    half = conn._python_to_value(Fraction(1, 2))
    assert (half.rational_value.numerator, half.rational_value.denominator) == ("1", "2")
    assert conn._python_to_value(0.5).WhichOneof("kind") == "real_value"


def test_a_rational_document_binding_needs_the_capability():
    stub = Mock()
    conn = make_connection(stub, OLD + (CAPABILITY_DOCUMENT_QUERY,))
    for value in (Fraction(1, 3), Quantity(Fraction(1, 3), KG)):
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.run_document_query("hash", "Q::q", bindings={"x": value})
        assert excinfo.value.capability == CAPABILITY_RATIONAL_VALUES
    stub.RunDocumentQuery.assert_not_called()


def test_a_document_binding_a_double_holds_reaches_an_older_service_as_that_double():
    stub = Mock()
    stub.RunDocumentQuery.return_value = sysml_pb2.RunDocumentQueryResponse()
    conn = make_connection(stub, OLD + (CAPABILITY_DOCUMENT_QUERY,))
    conn.run_document_query("hash", "Q::q", bindings={"x": Fraction(1, 2)})
    (request,), _ = stub.RunDocumentQuery.call_args
    assert request.bindings[0].values[0].real_value == 0.5


RATIONAL_MODEL = """
package R {
    private import ScalarValues::*;
    calc def Third { in x : Rational; return : Rational = x / 3; }
    calc third : Third;
    action addThird {
        in x;
        out y;
        first start;
        action inner { assign y := x + 1 / 3; }
        then done;
        succession first start then inner;
    }
}
"""


@pytest.mark.integration
class TestRationalsAgainstTheService:
    """What the real service makes of each wire spelling of a number."""

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
        self.model = self.conn.load_from_content(RATIONAL_MODEL)

    def teardown_method(self):
        if getattr(self, "conn", None) is not None:
            self.conn.close()

    def test_a_fraction_a_double_holds_stays_exact(self):
        assert self.conn.calc("R::third", self.model.hash, arguments=[Fraction(1, 4)]).value == Fraction(1, 12)
        assert self.conn.calc("R::third", self.model.hash, arguments=[Fraction(1, 10)]).value == Fraction(1, 30)

    def test_a_canonical_answer_a_double_holds_is_a_float(self):
        value = self.conn.calc("R::third", self.model.hash, arguments=[Fraction(3, 2)]).value
        assert value == 0.5 and isinstance(value, float)

    def test_a_float_input_is_a_real(self):
        exact = self.conn.execute_action("R::addThird", self.model.hash, inputs={"x": Fraction(1, 4)})
        real = self.conn.execute_action("R::addThird", self.model.hash, inputs={"x": 0.25})
        assert exact["y"] == Fraction(7, 12)
        assert real["y"] == 0.25 + 1 / 3 and isinstance(real["y"], float)

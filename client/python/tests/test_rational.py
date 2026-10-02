"""KerML Rationals are exact: one no double holds crosses the wire in full.

The service sends a Rational a double holds exactly, as ``0.5``, as
``real_value`` and any other, as ``1/3`` or ``0.1``, as the numerator and
denominator of ``rational_value``; the client decodes the latter to a
``fractions.Fraction`` and encodes a ``Fraction`` the same way.
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
from opensysml.document import build_bindings, _value_of
from opensysml.errors import UnsupportedValueError
from opensysml.proto import sysml_pb2
from opensysml.values import (
    Quantity,
    TensorQuantity,
    Unit,
    UnitFactor,
    Vector,
    VectorQuantity,
    holds_exactly_as_double,
    rational_from_pb,
    rational_value_to_pb,
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
def test_a_rational_a_double_holds_travels_as_real_value(value):
    pb = rational_value_to_pb(value)
    assert pb.WhichOneof("kind") == "real_value"
    assert pb.real_value == float(value)


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
    assert Quantity(Fraction(1, 2), KG).to_pb().real_magnitude == 0.5


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


def test_a_rational_is_sent_to_a_service_with_the_capability():
    conn = make_connection(Mock(), OLD + (CAPABILITY_RATIONAL_VALUES,))
    pb = conn._python_to_value([1, Fraction(1, 3)]).sequence.elements[1]
    assert (pb.rational_value.numerator, pb.rational_value.denominator) == ("1", "3")


def test_a_rational_document_binding_needs_the_capability():
    stub = Mock()
    conn = make_connection(stub, OLD + (CAPABILITY_DOCUMENT_QUERY,))
    for value in (Fraction(1, 3), Quantity(Fraction(1, 3), KG)):
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.run_document_query("hash", "Q::q", bindings={"x": value})
        assert excinfo.value.capability == CAPABILITY_RATIONAL_VALUES
    stub.RunDocumentQuery.assert_not_called()

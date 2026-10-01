"""KerML Integers are unbounded: one beyond int64 crosses the wire in full.

The service sends an Integer within int64 as ``int_value`` and a larger one as
the decimal ``big_int_value``; the client decodes both to a Python ``int`` and
encodes an ``int`` the same way, so the two arms are one value to a caller.
"""

import pytest

from opensysml.errors import UnsupportedValueError
from opensysml.proto import sysml_pb2
from opensysml.values import (
    Quantity,
    Unit,
    UnitFactor,
    Vector,
    fits_int64,
    integer_from_decimal,
    integer_to_decimal,
    integer_to_pb,
    value_to_python,
)

INT64_MAX = (1 << 63) - 1
INT64_MIN = -(1 << 63)


@pytest.mark.parametrize("value", [0, 7, -7, INT64_MAX, INT64_MIN])
def test_an_integer_within_int64_keeps_int_value(value):
    pb = integer_to_pb(value)
    assert pb.WhichOneof("kind") == "int_value"
    assert value_to_python(pb) == value


@pytest.mark.parametrize("value", [INT64_MAX + 1, INT64_MIN - 1, 2 ** 70, -(3 ** 200)])
def test_an_integer_beyond_int64_round_trips_as_its_decimal(value):
    pb = integer_to_pb(value)
    assert pb.WhichOneof("kind") == "big_int_value"
    assert pb.big_int_value == str(value)
    assert value_to_python(sysml_pb2.Value.FromString(pb.SerializeToString())) == value


def test_an_integer_past_the_str_conversion_limit_crosses_in_full():
    value = 7 ** 20000  # some 16900 digits, past the interpreter's default limit
    text = integer_to_decimal(value)
    assert len(text) > 5000
    assert integer_from_decimal(text) == value
    assert integer_from_decimal("-" + text) == -value
    assert value_to_python(integer_to_pb(-value)) == -value


@pytest.mark.parametrize("text", ["", "-", "+5", "1_000", " 5", "5.0", "١٢"])
def test_a_big_integer_that_is_not_decimal_is_refused(text):
    with pytest.raises(UnsupportedValueError, match="not decimal"):
        integer_from_decimal(text)


def test_fits_int64_bounds():
    assert fits_int64(INT64_MAX) and fits_int64(INT64_MIN)
    assert not fits_int64(INT64_MAX + 1) and not fits_int64(INT64_MIN - 1)


def test_a_big_quantity_magnitude_round_trips():
    kg = Unit(text="kg", factors=(UnitFactor("SI::kg", 1),), reduction_given=True)
    pb = Quantity(2 ** 70, kg).to_pb()
    assert pb.WhichOneof("magnitude") == "big_int_magnitude"
    assert Quantity.from_pb(pb).magnitude == 2 ** 70
    assert Quantity(5, kg).to_pb().WhichOneof("magnitude") == "int_magnitude"


def test_a_big_vector_component_decodes():
    pb = sysml_pb2.Vector(components=[integer_to_pb(2 ** 64), integer_to_pb(1)])
    assert Vector.from_pb(pb).components == (2 ** 64, 1)

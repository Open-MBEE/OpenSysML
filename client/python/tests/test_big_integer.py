"""KerML Integers are unbounded: one beyond int64 crosses the wire in full.

The service sends an Integer within int64 as ``int_value`` and a larger one as
the decimal ``big_int_value``; the client decodes both to a Python ``int`` and
encodes an ``int`` the same way, so the two arms are one value to a caller.
"""

from unittest.mock import Mock, patch

import pytest

from opensysml.capabilities import (
    CAPABILITY_BIG_INT_VALUES,
    CAPABILITY_SET_VALUES,
    CAPABILITY_STRUCTURED_VALUES,
    CAPABILITY_TENSOR_VALUES,
    CAPABILITY_VERIFICATION,
    MissingCapabilityError,
)
from opensysml.connection import Connection
from opensysml.errors import UnsupportedValueError
from opensysml.proto import sysml_pb2
from opensysml.values import (
    Array,
    Quantity,
    SetValue,
    TensorQuantity,
    Unit,
    UnitFactor,
    Vector,
    VectorQuantity,
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


def test_a_big_integer_is_not_sent_to_a_service_without_the_capability():
    """An older service would read the unknown arm as null, so nothing is sent."""
    kg = Unit(text="kg", factors=(UnitFactor("SI::kg", 1),), reduction_given=True)
    wide = Quantity(2 ** 70, kg)
    stub = Mock()
    conn = make_connection(stub, OLD)

    for value in (
        INT64_MAX + 1,
        INT64_MIN - 1,
        [1, [2 ** 70]],
        SetValue((2 ** 70,)),
        Array((1,), (2 ** 70,)),
        wide,
        Vector((1, 2 ** 70)),
        VectorQuantity((wide,)),
        TensorQuantity((1,), (wide,)),
    ):
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.execute_action("W::act", "hash", inputs={"x": value})
        assert excinfo.value.capability == CAPABILITY_BIG_INT_VALUES
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.calc("W::f", "hash", arguments=[value])
        assert excinfo.value.capability == CAPABILITY_BIG_INT_VALUES
    stub.ExecuteAction.assert_not_called()
    stub.EvaluateCalc.assert_not_called()

    assert conn._python_to_value(INT64_MAX).int_value == INT64_MAX
    assert conn._python_to_value(Quantity(5, kg)).quantity.int_magnitude == 5


def test_a_big_integer_is_sent_to_a_service_with_the_capability():
    conn = make_connection(Mock(), OLD + (CAPABILITY_BIG_INT_VALUES,))
    assert conn._python_to_value([1, 2 ** 70]).sequence.elements[1].big_int_value == str(2 ** 70)

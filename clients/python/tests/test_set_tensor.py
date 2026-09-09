"""Tests for a set and a tensor quantity as Python values.

A ``Collections::Set``'s elements travel in an arm of their own, ``Value.set``,
so they must arrive as one :class:`SetValue` holding each element once, equal to
another set whatever the order; a tensor quantity of any rank travels as
``Value.tensor_quantity`` and must arrive as one :class:`TensorQuantity` with
its shape. Neither is the unsupported null a service without the capability sends.
"""

from unittest.mock import Mock, patch

import pytest

from opensysml.capabilities import (
    CAPABILITY_COMPLEX_VALUES,
    CAPABILITY_FEATURE_VALUES,
    CAPABILITY_MEASUREMENT_REFS,
    CAPABILITY_SET_VALUES,
    CAPABILITY_STRUCTURED_VALUES,
    CAPABILITY_TENSOR_VALUES,
    CAPABILITY_VERIFICATION,
    MissingCapabilityError,
)
from opensysml import typed
from opensysml.connection import Connection
from opensysml.enumeration import EnumLiteral
from opensysml.errors import ExecutionError, TypeMismatchError, UnsupportedValueError
from opensysml.proto import sysml_pb2
from opensysml.values import (
    Array,
    InstanceRef,
    MeasurementRef,
    Quantity,
    UNSET,
    SetValue,
    TensorQuantity,
    Unit,
    UnitFactor,
    Vector,
    VectorQuantity,
    same_value,
    value_to_python,
)

from tests.service_gate import skip_or_fail_without_service


def pb_int(value):
    return sysml_pb2.Value(int_value=value)


def pb_set(*elements):
    return sysml_pb2.Value(set=sysml_pb2.ValueSet(elements=list(elements)))


PB_PASCAL = sysml_pb2.UnitTerm(
    scale_num=1.0, scale_den=1.0,
    factors=[
        sysml_pb2.UnitFactor(unit_id="SI::kilogram", exponent=1.0),
        sysml_pb2.UnitFactor(unit_id="SI::metre", exponent=-1.0),
        sysml_pb2.UnitFactor(unit_id="SI::second", exponent=-2.0),
    ],
)
PASCAL = Unit(
    "Pa", 1.0, 1.0,
    (UnitFactor("SI::kilogram", 1.0), UnitFactor("SI::metre", -1.0), UnitFactor("SI::second", -2.0)),
    reduction_given=True,
)


def pb_pascal(magnitude):
    return sysml_pb2.Quantity(real_magnitude=magnitude, unit="Pa", unit_term=PB_PASCAL)


def pb_tensor(dimensions, *components):
    return sysml_pb2.Value(tensor_quantity=sysml_pb2.TensorQuantity(
        dimensions=list(dimensions), components=list(components),
    ))


CUBE = TensorQuantity((2, 2, 2), [Quantity(float(i), PASCAL) for i in range(1, 9)])
PB_CUBE = pb_tensor((2, 2, 2), *(pb_pascal(float(i)) for i in range(1, 9)))


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
    CAPABILITY_COMPLEX_VALUES,
    CAPABILITY_STRUCTURED_VALUES,
    CAPABILITY_MEASUREMENT_REFS,
    CAPABILITY_FEATURE_VALUES,
    CAPABILITY_VERIFICATION,
)
CURRENT = OLD + (CAPABILITY_SET_VALUES, CAPABILITY_TENSOR_VALUES)


# --- Sets: reading ---------------------------------------------------------


def test_a_set_decodes_as_its_elements_in_the_order_sent():
    got = value_to_python(pb_set(pb_int(1), pb_int(2), pb_int(3)))

    assert isinstance(got, SetValue)
    assert got.elements == (1, 2, 3)
    assert len(got) == 3
    assert 2 in got and 4 not in got
    assert list(got) == [1, 2, 3]
    assert str(got) == "{1, 2, 3}"


def test_an_empty_set_is_a_set_of_nothing():
    got = value_to_python(pb_set())
    assert got == SetValue()
    assert len(got) == 0
    assert str(got) == "{}"
    assert got != []


def test_set_equality_ignores_order_and_a_python_set_compares_equal():
    assert SetValue((1, 2, 3)) == SetValue((3, 1, 2))
    assert SetValue((1, 2, 3)) == {3, 1, 2}
    assert {3, 1, 2} == SetValue((1, 2, 3))
    assert SetValue((1, 2, 3)) == frozenset((3, 1, 2))
    assert SetValue((1, 2)) != SetValue((1, 2, 3))
    assert SetValue((1, 2)) != [1, 2]
    assert SetValue(([1, 2], "a")) == SetValue(("a", [1, 2]))


def test_a_set_nests_and_is_nested_in_place():
    nested = pb_set(pb_set(pb_int(1)), pb_set())
    assert value_to_python(nested) == SetValue((SetValue((1,)), SetValue()))

    sequence = sysml_pb2.Value(sequence=sysml_pb2.ValueSequence(elements=[
        pb_set(pb_int(1)), pb_int(2),
    ]))
    assert value_to_python(sequence) == [SetValue((1,)), 2]

    array = sysml_pb2.Value(array=sysml_pb2.Array(dimensions=[1], elements=[pb_set(pb_int(1))]))
    assert value_to_python(array) == Array((1,), (SetValue((1,)),))


def pb_seq(*elements):
    return sysml_pb2.Value(sequence=sysml_pb2.ValueSequence(elements=list(elements)))


def pb_array(dimensions, *elements):
    return sysml_pb2.Value(array=sysml_pb2.Array(dimensions=list(dimensions), elements=list(elements)))


def pb_instance(instance_id):
    return sysml_pb2.Value(instance_id=instance_id)


@pytest.mark.parametrize("elements", [
    (pb_int(1), pb_int(2), pb_int(1)),
    (pb_int(1), sysml_pb2.Value(real_value=1.0)),
    (pb_int(2), sysml_pb2.Value(complex=sysml_pb2.Complex(real=2.0, imaginary=0.0))),
    (sysml_pb2.Value(real_value=2.5), sysml_pb2.Value(complex=sysml_pb2.Complex(real=2.5))),
    (pb_int(0), sysml_pb2.Value(real_value=-0.0)),
    (pb_seq(pb_int(1), pb_int(2)), pb_seq(pb_int(1), pb_int(2))),
    (pb_set(pb_int(1), pb_int(2)), pb_set(pb_int(2), pb_int(1))),
    (pb_set(), pb_set()),
    (sysml_pb2.Value(quantity=pb_pascal(1.0)), sysml_pb2.Value(quantity=pb_pascal(1.0))),
    (sysml_pb2.Value(bool_value=True), pb_seq(), sysml_pb2.Value(bool_value=True)),
    (pb_instance(1), pb_int(2), pb_instance(1)),
    (pb_array((2,), pb_int(1), pb_int(2)), pb_array((2,), pb_int(1), sysml_pb2.Value(real_value=2.0))),
    (sysml_pb2.Value(null=""), pb_seq()),
    (pb_int(1), sysml_pb2.Value(null=""), pb_set()),
    (pb_seq(), pb_set()),
    (pb_set(sysml_pb2.Value(null=""), pb_int(1)), pb_set(pb_int(1), pb_seq())),
])
def test_a_set_listing_a_member_twice_is_malformed(elements):
    with pytest.raises(UnsupportedValueError, match="malformed set: set lists a member twice"):
        value_to_python(pb_set(*elements))


@pytest.mark.parametrize("elements", [
    (1, 2, 1),
    (1, 1.0),
    (2, 2 + 0j),
    (0, -0.0),
    ([1, 2], [1, 2]),
    (SetValue((1, 2)), SetValue((2, 1))),
    (SetValue(), SetValue()),
    (Quantity(1.0, Unit("m", factors=(UnitFactor("SI::metre", 1.0),))),
     Quantity(1, Unit("m", factors=(UnitFactor("SI::metre", 1.0),)))),
    (True, [], True),
    (InstanceRef(1), 2, InstanceRef(1)),
    (Array((2,), (1, 2)), Array((2,), (1, 2.0))),
    (None, []),
    (1, None, SetValue()),
    ([], SetValue()),
    (None, set()),
    ([], frozenset()),
    (SetValue((SetValue(), 1)), SetValue((1, []))),
])
def test_a_set_is_never_assembled_with_a_member_twice(elements):
    with pytest.raises(ValueError, match="set lists a member twice"):
        SetValue(elements)
    assert len(SetValue((1, 2 ** 53 + 1, float(2 ** 53), True, [1, 2], [2, 1], [1], SetValue((1,))))) == 8


def test_null_and_the_empty_collections_are_one_value():
    """As the service judges them: the absent value, however spelt."""
    for empty in ([], SetValue(), set(), frozenset()):
        assert same_value(None, empty) and same_value(empty, None) and same_value(empty, [])
        assert empty in SetValue((1, None)) and None in SetValue((empty,))
    assert SetValue((None, 1)) == SetValue((1, [])) == SetValue((SetValue(), 1))
    assert not same_value(None, [1]) and not same_value([], SetValue((SetValue(),)))
    assert not same_value(None, 0) and not same_value([], False) and not same_value(None, UNSET)
    assert len(SetValue((None, [1], SetValue((1,)), SetValue((SetValue(),))))) == 4
    assert value_to_python(pb_set(sysml_pb2.Value(null=""), pb_seq(pb_int(1)))) == SetValue((None, [1]))


def length(text, magnitude, scale_num=1.0, scale_den=1.0, factors=(("SI::metre", 1.0),)):
    return Quantity(magnitude, Unit(text, scale_num, scale_den, tuple(UnitFactor(*f) for f in factors)))


def test_quantities_are_the_same_value_over_their_base_units():
    """As the service judges them: 1 m is 100 cm, exactly while both are ints over whole scales."""
    m, cm, km = length("m", 1), length("cm", 100, 1.0, 100.0), length("km", 1, 1000.0)
    assert m == cm == length("cm", 100, 0.01)
    assert length("m", 1.0) == length("cm", 100.0, 1.0, 100.0) == cm
    assert m != length("cm", 1, 1.0, 100.0)
    assert length("m", 1000) == km and length("m", 1001) != km
    huge = 2 ** 53 + 1
    assert length("m", 1000 * huge) == length("km", huge, 1000.0)
    assert length("m", 1000 * huge + 1) != length("km", huge, 1000.0)
    assert m != length("s", 1, factors=(("SI::second", 1.0),))
    speed = (("SI::metre", 1.0), ("SI::second", -1.0))
    assert length("km/h", 5.4, 1000.0, 3600.0, speed) == length("m/s", 1.5, factors=reversed(speed))
    assert length("km/h", 36, 1000.0, 3600.0, speed) == length("m/s", 10, factors=speed)
    assert length("km/h", 36, 1000.0, 3600.0, speed) != length("m/s", 11, factors=speed)
    assert m == length("m", 1, factors=(("SI::metre", 1.0), ("SI::second", -1.0), ("SI::second", 1.0)))
    assert m != length("x", 0, 0.0) and length("x", 0, 0.0) != length("x", 0, 0.0)
    # A unit named without its reduction compares as written.
    assert Quantity(1, Unit("m")) == Quantity(1.0, Unit("m"))
    assert Quantity(1, Unit("m")) != Quantity(100, Unit("cm"))
    assert Quantity(1, Unit("m")) != Quantity(1, Unit("s"))
    assert Quantity(1, Unit("m")) != m
    assert hash(m) == hash(cm) == hash(length("m", 1.0))

    # Membership, duplicate detection and set equality follow.
    lengths = SetValue((m, length("s", 1, factors=(("SI::second", 1.0),))))
    assert cm in lengths and length("cm", 1, 1.0, 100.0) not in lengths
    with pytest.raises(ValueError, match="set lists a member twice"):
        SetValue((m, cm))
    with pytest.raises(UnsupportedValueError, match="malformed set: set lists a member twice"):
        value_to_python(pb_set(sysml_pb2.Value(quantity=m.to_pb()), sysml_pb2.Value(quantity=cm.to_pb())))
    assert len(SetValue((m, length("cm", 1, 1.0, 100.0)))) == 2
    assert SetValue((m, length("km", 2, 1000.0))) == SetValue((length("m", 2000), cm))
    assert SetValue((m, length("km", 2, 1000.0))) != SetValue((length("m", 2000), length("cm", 1, 1.0, 100.0)))
    assert VectorQuantity((m, km)) == VectorQuantity((cm, length("m", 1000)))
    assert TensorQuantity((1, 1), (m,)) == TensorQuantity((1, 1), (cm,))


def ref(text, unit_id="", scale_num=1.0, scale_den=1.0, factors=(("SI::metre", 1.0),)):
    unit = Unit(text, scale_num, scale_den, tuple(UnitFactor(*f) for f in factors), reduction_given=True)
    return MeasurementRef(unit, unit_id)


def test_measurement_refs_are_the_same_value_over_one_reduction():
    """As the service judges them: one reduction at one scale, however spelt."""
    speed = (("SI::metre", 1.0), ("SI::second", -1.0))
    named = ref("SI::'m/s'", "SI::'m/s'", factors=speed)
    composed = ref("m / s", factors=reversed(speed))
    assert named == composed and hash(named) == hash(composed)
    assert ref("km/m", scale_num=1000.0) == ref("m/mm", scale_num=1.0, scale_den=0.001)
    assert ref("km", "SI::kilometre", 1000.0) == ref("km", "SI::km", 2000.0, 2.0)
    assert ref("km", "SI::kilometre", 1000.0) != ref("m", "SI::metre")
    assert ref("m", "SI::metre") != ref("s", "SI::second", factors=(("SI::second", 1.0),))
    # A scale nothing converts through is no one's reduction, not even its own copy's.
    zero_scaled, zero_scaled_again = ref("x", scale_num=0.0), ref("x", scale_num=0.0)
    assert zero_scaled != zero_scaled_again
    assert len(SetValue((zero_scaled, zero_scaled_again))) == 2
    # A named unit of dimension one reduces to nothing, so it is only itself.
    rad, sr = ref("rad", "SI::radian", factors=()), ref("sr", "SI::steradian", factors=())
    assert rad != sr and rad == ref("SI::rad", "SI::radian", factors=(("SI::metre", 1.0), ("SI::metre", -1.0)))
    assert ref("m/m", factors=(("SI::metre", 1.0), ("SI::metre", -1.0))) == ref("", factors=())
    assert rad != ref("m/m", factors=(("SI::metre", 1.0), ("SI::metre", -1.0)))
    # One named without its reduction compares as written.
    unreduced = MeasurementRef(Unit("m"), "SI::metre")
    assert unreduced == MeasurementRef(Unit("m"), "SI::metre")
    assert unreduced != MeasurementRef(Unit("metre"), "SI::metre")
    assert unreduced != MeasurementRef(Unit("m"), "SI::m")
    assert unreduced != ref("m", "SI::metre")

    # Membership, duplicate detection and set equality follow.
    assert composed in SetValue((named,)) and sr not in SetValue((rad,))
    for twice in ((named, composed), (rad, ref("SI::rad", "SI::radian", factors=()))):
        with pytest.raises(ValueError, match="set lists a member twice"):
            SetValue(twice)
    with pytest.raises(UnsupportedValueError, match="malformed set: set lists a member twice"):
        value_to_python(pb_set(
            sysml_pb2.Value(measurement_ref=named.to_pb()),
            sysml_pb2.Value(measurement_ref=composed.to_pb()),
        ))
    assert len(SetValue((rad, sr))) == 2
    assert SetValue((named, rad)) == SetValue((rad, composed))


def test_enumeration_literals_are_the_same_value_by_literal_id():
    red = EnumLiteral("D::Color::red", "D::Color", "Color::red")
    same = EnumLiteral("D::Color::red", "E::Palette", "red")
    assert red == same and EnumLiteral("D::Color::red") in SetValue((red,))
    assert red != EnumLiteral("D::Color::green", "D::Color", "Color::red")
    with pytest.raises(ValueError, match="set lists a member twice"):
        SetValue((red, same))
    with pytest.raises(UnsupportedValueError, match="malformed set: set lists a member twice"):
        value_to_python(pb_set(
            sysml_pb2.Value(enum_literal=sysml_pb2.EnumLiteral(
                literal_id="D::Color::red", enumeration_id="D::Color", name="Color::red")),
            sysml_pb2.Value(enum_literal=sysml_pb2.EnumLiteral(literal_id="D::Color::red")),
        ))
    assert SetValue((red, EnumLiteral("D::Color::green"))) == SetValue((EnumLiteral("D::Color::green", name="g"), same))


@pytest.mark.parametrize("elements, expected", [
    ((sysml_pb2.Value(bool_value=True), pb_int(1)), SetValue((True, 1))),
    ((pb_int(1), sysml_pb2.Value(real_value=1.5)), SetValue((1, 1.5))),
    ((pb_int(2 ** 53 + 1), sysml_pb2.Value(real_value=float(2 ** 53))), SetValue((2 ** 53 + 1, float(2 ** 53)))),
    ((pb_int(1), sysml_pb2.Value(complex=sysml_pb2.Complex(real=1.0, imaginary=1.0))), SetValue((1, 1 + 1j))),
    ((pb_seq(pb_int(1), pb_int(2)), pb_seq(pb_int(2), pb_int(1))), SetValue(([1, 2], [2, 1]))),
    ((pb_seq(pb_int(1)), pb_set(pb_int(1))), SetValue(([1], SetValue((1,))))),
    ((pb_seq(sysml_pb2.Value(bool_value=True)), pb_seq(pb_int(1))), SetValue(([True], [1]))),
    ((pb_set(), pb_set(pb_set())), SetValue((SetValue(), SetValue((SetValue(),))))),
    ((pb_instance(1), pb_int(1)), SetValue((InstanceRef(1), 1))),
    ((pb_array((1,), sysml_pb2.Value(bool_value=True)), pb_array((1,), pb_int(1))),
     SetValue((Array((1,), (True,)), Array((1,), (1,))))),
    ((pb_array((1,), pb_instance(1)), pb_array((1,), pb_int(1))),
     SetValue((Array((1,), (InstanceRef(1),)), Array((1,), (1,))))),
])
def test_members_that_only_look_alike_are_distinct(elements, expected):
    got = value_to_python(pb_set(*elements))
    assert len(got) == len(elements)
    assert got == expected
    assert 1 not in SetValue((True,)) and True not in SetValue((1,))


def test_an_unresolved_instance_reference_holds_its_id_but_is_not_an_integer():
    ref = value_to_python(pb_instance(7))
    assert isinstance(ref, InstanceRef) and ref.id == 7 and ref == InstanceRef(7)
    assert ref != 7 and not isinstance(ref, int) and str(ref) == "instance(7)"
    assert ref in SetValue((InstanceRef(7),)) and ref not in SetValue((7,))
    assert Array((1,), (ref,)) != Array((1,), (7,))
    assert Array((2,), (True, 1)) != Array((2,), (1, 1))
    assert Array((2,), (1, 2)) == Array((2,), (1, 2.0))

    conn = make_connection(Mock(), CURRENT)
    sent = conn._python_to_value(SetValue((ref, 7)))
    assert [e.WhichOneof("kind") for e in sent.set.elements] == ["instance_id", "int_value"]
    assert sent.set.elements[0].instance_id == 7
    assert value_to_python(sent) == SetValue((InstanceRef(7), 7))


@pytest.mark.parametrize("instance_id", [True, 7.0, "7", None])
def test_an_instance_reference_holds_only_an_integer_id(instance_id):
    with pytest.raises(ValueError, match="is not an integer"):
        InstanceRef(instance_id)


def test_an_instance_reference_is_refused_where_a_number_is_meant():
    ref = InstanceRef(7)
    with pytest.raises(ValueError, match="not a number"):
        Vector((1, ref))
    with pytest.raises(ValueError, match="not a positive integer"):
        Array((ref,), (1,))
    with pytest.raises(ValueError, match="not a positive integer"):
        TensorQuantity((ref,), [Quantity(1.0, PASCAL)])
    with pytest.raises(TypeError):
        Quantity(1.0, PASCAL) * ref
    with pytest.raises(TypeMismatchError):
        typed.as_int("n", ref)
    with pytest.raises(TypeMismatchError):
        typed.as_float("x", ref)
    with pytest.raises(TypeMismatchError):
        typed.as_complex("z", ref)


def test_a_set_survives_the_wire_bytes():
    value = pb_set(pb_int(1), sysml_pb2.Value(string_value="a"), pb_set())
    again = sysml_pb2.Value()
    again.ParseFromString(value.SerializeToString())
    assert again == value
    assert value_to_python(again) == value_to_python(value)


def test_a_service_without_set_values_still_reports_unsupported():
    null = sysml_pb2.Value(null="unsupported: set Set{1, 2, 3}")
    with pytest.raises(UnsupportedValueError, match="set Set"):
        value_to_python(null)


# --- Sets: sending ---------------------------------------------------------


def test_a_set_is_sent_as_its_own_arm():
    conn = make_connection(Mock(), CURRENT)

    sent = conn._python_to_value(SetValue((3, 1, 2)))
    assert sent.WhichOneof("kind") == "set"
    assert [e.int_value for e in sent.set.elements] == [3, 1, 2]
    assert value_to_python(sent) == SetValue((1, 2, 3))

    sent = conn._python_to_value({1, 2})
    assert sent.WhichOneof("kind") == "set"
    assert value_to_python(sent) == {1, 2}

    sent = conn._python_to_value(frozenset())
    assert sent.WhichOneof("kind") == "set"
    assert value_to_python(sent) == SetValue()

    nested = conn._python_to_value([1, SetValue(([2], SetValue()))])
    assert value_to_python(nested) == [1, SetValue(([2], SetValue()))]


def test_a_set_is_not_sent_to_a_service_without_the_capability():
    stub = Mock()
    conn = make_connection(stub, OLD)

    for value in (SetValue((1,)), {1}, [1, [SetValue()]], Array((1,), (SetValue((1,)),))):
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.execute_action("W::act", "hash", inputs={"c": value})
        assert excinfo.value.capability == CAPABILITY_SET_VALUES
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.calc("W::sizeOf", "hash", arguments=[value])
        assert excinfo.value.capability == CAPABILITY_SET_VALUES
    stub.ExecuteAction.assert_not_called()
    stub.EvaluateCalc.assert_not_called()

    # A list still travels: it never needed the capability.
    stub.EvaluateCalc.return_value = sysml_pb2.EvaluateCalcResponse(result=pb_int(3))
    assert conn.calc("W::sizeOf", "hash", arguments=[[1, 2, 3]]).value == 3
    stub.EvaluateCalc.assert_called_once()


# --- Tensors: reading ------------------------------------------------------


def test_a_tensor_decodes_with_its_shape_and_components():
    got = value_to_python(PB_CUBE)

    assert isinstance(got, TensorQuantity)
    assert got == CUBE
    assert got.rank == 3
    assert got.dimensions == (2, 2, 2)
    assert len(got) == 8
    assert got[7] == Quantity(8.0, PASCAL)
    assert got[(1, 1, 1)] == Quantity(8.0, PASCAL)
    assert got[(0, 1, 0)] == Quantity(3.0, PASCAL)
    assert got.unit == PASCAL
    assert str(got) == "Tensor(2, 2, 2)[1, 2, 3, 4, 5, 6, 7, 8] [Pa]"
    assert not isinstance(got, VectorQuantity)


def test_a_tensor_of_rank_one_is_not_a_vector_quantity():
    got = value_to_python(pb_tensor((2,), pb_pascal(1.0), pb_pascal(2.0)))
    assert isinstance(got, TensorQuantity)
    assert got.rank == 1
    assert not isinstance(got, VectorQuantity)


def test_a_tensor_with_mixed_units_renders_each_component():
    metre = sysml_pb2.Quantity(
        int_magnitude=3, unit="m",
        unit_term=sysml_pb2.UnitTerm(scale_num=1, scale_den=1, factors=[
            sysml_pb2.UnitFactor(unit_id="SI::metre", exponent=1.0),
        ]),
    )
    got = value_to_python(pb_tensor((1, 2), metre, pb_pascal(2.0)))
    assert got.unit is None
    assert str(got) == "Tensor(1, 2)[3 [m], 2 [Pa]]"


def test_tensor_indexing_is_shape_checked():
    with pytest.raises(IndexError, match="has 2 coordinate"):
        CUBE[(1, 1)]
    with pytest.raises(IndexError, match="has 4 coordinate"):
        CUBE[(1, 1, 1, 1)]
    with pytest.raises(IndexError, match="outside dimensions"):
        CUBE[(0, 0, 2)]
    with pytest.raises(IndexError, match="outside dimensions"):
        CUBE[(-1, 0, 0)]


@pytest.mark.parametrize("dimensions, components, message", [
    ((2, 2), [pb_pascal(1.0)] * 3, "holds 3 component"),
    ((2,), [pb_pascal(1.0)] * 3, "holds 3 component"),
    ((0,), [], "not a positive integer"),
    ((-1,), [pb_pascal(1.0)], "not a positive integer"),
    ((), [], "holds 0 component"),
])
def test_a_malformed_tensor_is_reported(dimensions, components, message):
    with pytest.raises(UnsupportedValueError, match=message):
        value_to_python(pb_tensor(dimensions, *components))


def test_a_tensor_component_without_a_magnitude_is_reported():
    with pytest.raises(UnsupportedValueError):
        value_to_python(pb_tensor((1,), sysml_pb2.Quantity(unit="Pa", unit_term=PB_PASCAL)))


def test_a_tensor_built_by_hand_is_shape_checked():
    with pytest.raises(ValueError, match="holds 1 component"):
        TensorQuantity((2,), [Quantity(1.0, PASCAL)])
    with pytest.raises(ValueError, match="not a positive integer"):
        TensorQuantity((0,), [])
    with pytest.raises(ValueError, match="not a Quantity"):
        TensorQuantity((1,), [1.0])


def test_a_tensor_survives_the_wire_bytes():
    again = sysml_pb2.Value()
    again.ParseFromString(PB_CUBE.SerializeToString())
    assert again == PB_CUBE
    assert value_to_python(again) == CUBE


def test_a_service_without_tensor_values_still_reports_unsupported():
    null = sysml_pb2.Value(null="unsupported: tensor quantity Tensor(2, 2, 2)[1.0] [Pa]")
    with pytest.raises(UnsupportedValueError, match="tensor quantity"):
        value_to_python(null)


# --- Tensors: sending ------------------------------------------------------


def test_a_tensor_is_sent_as_its_own_arm():
    conn = make_connection(Mock(), CURRENT)

    sent = conn._python_to_value(CUBE)
    assert sent.WhichOneof("kind") == "tensor_quantity"
    assert list(sent.tensor_quantity.dimensions) == [2, 2, 2]
    assert [c.real_magnitude for c in sent.tensor_quantity.components] == [float(i) for i in range(1, 9)]
    assert value_to_python(sent) == CUBE

    nested = conn._python_to_value([CUBE, SetValue((CUBE,))])
    assert value_to_python(nested) == [CUBE, SetValue((CUBE,))]


def test_a_tensor_is_not_sent_to_a_service_without_the_capability():
    stub = Mock()
    conn = make_connection(stub, OLD + (CAPABILITY_SET_VALUES,))

    for value in (CUBE, [CUBE], SetValue((CUBE,)), Array((1,), (CUBE,))):
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.execute_action("W::act", "hash", inputs={"t": value})
        assert excinfo.value.capability == CAPABILITY_TENSOR_VALUES
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.calc("W::corner", "hash", arguments=[value])
        assert excinfo.value.capability == CAPABILITY_TENSOR_VALUES
    stub.ExecuteAction.assert_not_called()
    stub.EvaluateCalc.assert_not_called()


def test_a_refusal_by_the_service_names_the_capability():
    """A service claiming the capability yet refusing is reported by its own words."""
    import grpc

    class Refusal(grpc.RpcError, grpc.Call):
        def code(self):
            return grpc.StatusCode.UNIMPLEMENTED

        def details(self):
            return 'capability "tensor_values" is unavailable'

        def trailing_metadata(self):
            return ()

    stub = Mock()
    conn = make_connection(stub, CURRENT)
    stub.EvaluateCalc.side_effect = Refusal()

    with pytest.raises(MissingCapabilityError) as excinfo:
        conn.calc("W::corner", "hash", arguments=[CUBE])
    assert excinfo.value.capability == CAPABILITY_TENSOR_VALUES


SET_TENSOR_MODEL = """
package W {
    private import ScalarValues::*;
    private import Collections::*;
    private import Quantities::*;
    private import MeasurementReferences::*;
    private import SI::*;
    private import TensorCalculations::*;

    attribute s : Set { :>> elements = (3, 1, 2, 2, 3); }
    attribute e : Set { :>> elements = (); }
    attribute mixed : Set { :>> elements = ("b", 2, true, "a"); }

    attribute cubeRef : TensorMeasurementReference {
        :>> dimensions = (2, 2, 2);
        :>> mRefs = (Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa);
    }
    attribute cube : TensorQuantityValue = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0), cubeRef);

    calc def SizeOf { in c : Integer[0..*]; return : Natural = SequenceFunctions::size(c); }
    calc sizeOf : SizeOf;
    calc def Corner { in t : TensorQuantityValue; return : ScalarQuantityValue = t#(2, 2, 2); }
    calc corner : Corner;
}
"""


@pytest.mark.integration
class TestSetsAndTensorsAgainstTheService:
    """What a caller actually gets back from the real service for a real model."""

    def setup_method(self):
        import grpc

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
        self.model = self.conn.load_from_content(SET_TENSOR_MODEL)

    def teardown_method(self):
        conn = self.__dict__.get("conn")
        if conn is not None:
            conn.close()

    def test_the_service_advertises_both_capabilities(self):
        info = self.conn.server_info()
        assert CAPABILITY_SET_VALUES in info.capabilities
        assert CAPABILITY_TENSOR_VALUES in info.capabilities

    def test_a_set_reads_each_element_once_in_canonical_order(self):
        assert self.conn.eval("W::s.elements", self.model.hash).elements == (1, 2, 3)
        assert self.conn.eval("W::e.elements", self.model.hash) == SetValue()
        assert self.conn.eval("W::mixed.elements", self.model.hash).elements == (True, 2, "a", "b")

    def test_a_tensor_reads_with_its_rank_and_indexes_by_shape(self):
        cube = self.conn.eval("W::cube", self.model.hash)
        assert isinstance(cube, TensorQuantity)
        assert cube.dimensions == (2, 2, 2)
        assert cube[(1, 1, 1)].magnitude == 8.0
        assert cube.unit.text == "Pa"

        corner = self.conn.calc("W::corner", self.model.hash, arguments=[cube]).value
        assert corner == cube[(1, 1, 1)]

    def test_a_set_sent_in_any_order_is_read_as_its_elements(self):
        assert self.conn.calc("W::sizeOf", self.model.hash, arguments=[{3, 1, 2}]).value == 3
        with pytest.raises(ValueError, match="set lists a member twice"):
            SetValue((1, 1))

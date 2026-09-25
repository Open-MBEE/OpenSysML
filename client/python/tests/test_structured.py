"""Tests for an array, a vector and a vector quantity as Python values.

Each travels in an arm of its own — ``Value.array``, ``Value.vector`` and
``Value.vector_quantity`` — so it must arrive as one :class:`Array`,
:class:`Vector` or :class:`VectorQuantity`: not as a flat list of its parts, and
not as the unsupported null a service without the capability sends.
"""

from unittest.mock import Mock, patch

import pytest

from opensysml.capabilities import (
    CAPABILITY_COMPLEX_VALUES,
    CAPABILITY_FEATURE_VALUES,
    CAPABILITY_STRUCTURED_VALUES,
    CAPABILITY_VERIFICATION,
    MissingCapabilityError,
)
from opensysml.connection import Connection
from opensysml.errors import FeatureValueError, UnsupportedValueError
from opensysml.proto import sysml_pb2
from opensysml.values import (
    Array,
    Quantity,
    Unit,
    UnitFactor,
    Vector,
    VectorQuantity,
    feature_value_to_python,
    value_to_python,
)

from tests.service_gate import skip_or_fail_without_service


def pb_int(value):
    return sysml_pb2.Value(int_value=value)


def pb_real(value):
    return sysml_pb2.Value(real_value=value)


def pb_metre(magnitude):
    """A Quantity message in metres as the service sends one."""
    return sysml_pb2.Quantity(
        real_magnitude=magnitude,
        unit="m",
        unit_term=sysml_pb2.UnitTerm(
            factors=[sysml_pb2.UnitFactor(unit_id="SI::metre", exponent=1.0)],
            scale_num=1.0,
            scale_den=1.0,
        ),
    )


def pb_array(dimensions, *elements):
    return sysml_pb2.Value(array=sysml_pb2.Array(dimensions=dimensions, elements=list(elements)))


def pb_vector(*components):
    return sysml_pb2.Value(vector=sysml_pb2.Vector(components=list(components)))


def pb_vector_quantity(*components):
    return sysml_pb2.Value(
        vector_quantity=sysml_pb2.VectorQuantity(components=list(components))
    )


METRE = Unit("m", 1.0, 1.0, (UnitFactor("SI::metre", 1.0),), reduction_given=True)
RADIAN = Unit("rad", 1.0, 1.0, (), reduction_given=True)


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


CURRENT = (
    CAPABILITY_COMPLEX_VALUES,
    CAPABILITY_STRUCTURED_VALUES,
    CAPABILITY_FEATURE_VALUES,
    CAPABILITY_VERIFICATION,
)
OLD = (CAPABILITY_COMPLEX_VALUES, CAPABILITY_FEATURE_VALUES, CAPABILITY_VERIFICATION)


# --- Array ---------------------------------------------------------------


def test_an_array_decodes_with_its_shape_and_row_major_elements():
    got = value_to_python(pb_array([2, 3], *(pb_int(i) for i in range(1, 7))))

    assert isinstance(got, Array)
    assert got == Array((2, 3), (1, 2, 3, 4, 5, 6))
    assert got.dimensions == (2, 3)
    assert got.rank == 2
    assert len(got) == 6
    assert got[5] == 6
    assert got[(1, 2)] == 6
    assert got[(0, 1)] == 2
    assert got.nested() == [[1, 2, 3], [4, 5, 6]]
    assert str(got) == "Array(2, 3)[1, 2, 3, 4, 5, 6]"


def test_arrays_of_rank_zero_one_and_three_keep_their_shape():
    scalar = value_to_python(pb_array([], pb_real(7.0)))
    assert scalar == Array((), (7.0,))
    assert scalar.rank == 0
    assert scalar.nested() == 7.0
    assert scalar[()] == 7.0

    line = value_to_python(pb_array([3], pb_int(1), pb_int(2), pb_int(3)))
    assert line == Array((3,), (1, 2, 3))
    assert line.nested() == [1, 2, 3]

    cube = value_to_python(pb_array([2, 2, 2], *(pb_int(i) for i in range(8))))
    assert cube.dimensions == (2, 2, 2)
    assert cube.nested() == [[[0, 1], [2, 3]], [[4, 5], [6, 7]]]
    assert cube[(1, 0, 1)] == 5


def test_an_array_element_is_any_value_a_nested_array_or_a_quantity_included():
    got = value_to_python(pb_array(
        [2],
        pb_array([1], sysml_pb2.Value(quantity=pb_metre(3.0))),
        pb_vector(pb_real(1.0), pb_real(2.0)),
    ))

    assert got == Array((2,), (
        Array((1,), (Quantity(3.0, METRE),)),
        Vector((1.0, 2.0)),
    ))
    assert got[0][0].unit == METRE


def test_an_array_whose_elements_do_not_fill_its_shape_is_reported():
    short = pb_array([2, 3], pb_int(1), pb_int(2))
    with pytest.raises(UnsupportedValueError, match="malformed array"):
        value_to_python(short)
    empty = pb_array([0], )
    with pytest.raises(UnsupportedValueError, match="malformed array"):
        value_to_python(empty)
    with pytest.raises(ValueError):
        Array((2,), (1, 2, 3))
    with pytest.raises(ValueError):
        Array((-1,), ())
    with pytest.raises(IndexError):
        Array((2, 3), tuple(range(6)))[(2, 0)]
    with pytest.raises(IndexError):
        Array((2, 3), tuple(range(6)))[(1,)]


# --- Vector --------------------------------------------------------------


def test_a_vector_decodes_as_one_vector_keeping_integer_and_real_apart():
    reals = value_to_python(pb_vector(pb_real(3.0), pb_real(4.0)))
    assert isinstance(reals, Vector)
    assert reals == Vector((3.0, 4.0))
    assert reals != [3.0, 4.0]
    assert len(reals) == 2
    assert reals[1] == 4.0
    assert list(reals) == [3.0, 4.0]
    assert str(reals) == "⟨3, 4⟩"

    ints = value_to_python(pb_vector(pb_int(1), pb_int(2)))
    assert ints == Vector((1, 2))
    assert all(isinstance(c, int) for c in ints)

    mixed = value_to_python(pb_vector(pb_int(1), pb_real(2.5)))
    assert isinstance(mixed[0], int)
    assert isinstance(mixed[1], float)


def test_an_empty_vector_is_a_vector_of_nothing():
    assert value_to_python(pb_vector()) == Vector(())


def test_a_vector_with_a_component_that_is_not_a_number_is_reported():
    text = pb_vector(pb_real(1.0), sysml_pb2.Value(string_value="two"))
    with pytest.raises(UnsupportedValueError, match="not a number"):
        value_to_python(text)
    unset = pb_vector(sysml_pb2.Value())
    with pytest.raises(UnsupportedValueError, match="not a number"):
        value_to_python(unset)
    with pytest.raises(ValueError):
        Vector((1.0, True))
    with pytest.raises(ValueError):
        Vector((1.0, "two"))


def test_a_vector_encodes_its_components_by_kind():
    sent = Vector((3.0, 4)).to_pb()

    assert sent.components[0].WhichOneof("kind") == "real_value"
    assert sent.components[0].real_value == 3.0
    assert sent.components[1].WhichOneof("kind") == "int_value"
    assert sent.components[1].int_value == 4
    assert value_to_python(sysml_pb2.Value(vector=sent)) == Vector((3.0, 4))


# --- VectorQuantity ------------------------------------------------------


def test_a_vector_quantity_decodes_one_quantity_per_component():
    got = value_to_python(pb_vector_quantity(pb_metre(3.0), pb_metre(4.0)))

    assert isinstance(got, VectorQuantity)
    assert got == VectorQuantity((Quantity(3.0, METRE), Quantity(4.0, METRE)))
    assert len(got) == 2
    assert got[0].magnitude == 3.0
    assert got[0].unit.text == "m"
    assert got[0].unit.factors == (UnitFactor("SI::metre", 1.0),)
    assert got.unit == METRE
    assert got.magnitudes() == Vector((3.0, 4.0))
    assert str(got) == "⟨3, 4⟩ [m]"


def test_a_vector_quantity_may_carry_a_different_unit_per_component():
    radian = sysml_pb2.Quantity(
        real_magnitude=2.0, unit="rad", unit_term=sysml_pb2.UnitTerm(scale_num=1.0, scale_den=1.0)
    )
    got = value_to_python(pb_vector_quantity(pb_metre(1.0), radian))

    assert got == VectorQuantity((Quantity(1.0, METRE), Quantity(2.0, RADIAN)))
    assert got.unit is None
    assert str(got) == "⟨1 [m], 2 [rad]⟩"


def test_a_vector_quantity_in_a_composed_unit_keeps_the_reduction():
    speed = sysml_pb2.Quantity(
        real_magnitude=5.0,
        unit="m/s",
        unit_term=sysml_pb2.UnitTerm(
            factors=[
                sysml_pb2.UnitFactor(unit_id="SI::metre", exponent=1.0),
                sysml_pb2.UnitFactor(unit_id="SI::second", exponent=-1.0),
            ],
            scale_num=1.0,
            scale_den=1.0,
        ),
    )
    got = value_to_python(pb_vector_quantity(speed))

    assert got[0].unit.text == "m/s"
    assert got[0].unit.exponents() == {"SI::metre": 1.0, "SI::second": -1.0}
    again = value_to_python(sysml_pb2.Value(vector_quantity=got.to_pb()))
    assert again == got
    assert again[0].unit.factors == got[0].unit.factors


def test_a_malformed_vector_quantity_is_reported():
    empty = pb_vector_quantity()
    with pytest.raises(UnsupportedValueError, match="no components"):
        value_to_python(empty)
    unreduced = pb_vector_quantity(sysml_pb2.Quantity(real_magnitude=1.0, unit="Furlongs::furlong"))
    with pytest.raises(UnsupportedValueError, match="no reduction"):
        value_to_python(unreduced)
    with pytest.raises(ValueError):
        VectorQuantity(())
    with pytest.raises(ValueError):
        VectorQuantity((1.0,))


def test_an_unreduced_vector_quantity_is_refused_before_it_is_sent():
    furlongs = VectorQuantity((Quantity(5.0, Unit(text="Furlongs::furlong")),))
    with pytest.raises(UnsupportedValueError):
        furlongs.to_pb()


# --- Feature values and the wire ----------------------------------------


def test_structured_slots_read_as_structured_values():
    slot = sysml_pb2.FeatureValue(
        feature_name="v", value=pb_vector(pb_real(3.0), pb_real(4.0)), materialized=True
    )
    assert feature_value_to_python("v", slot) == Vector((3.0, 4.0))

    slots = sysml_pb2.FeatureValue(
        feature_name="vs",
        values=[pb_vector(pb_real(1.0)), pb_array([1], pb_int(2))],
        materialized=True,
    )
    assert feature_value_to_python("vs", slots) == [Vector((1.0,)), Array((1,), (2,))]

    bad = sysml_pb2.FeatureValue(
        feature_name="v", value=pb_array([2], pb_int(1)), materialized=True
    )
    with pytest.raises(FeatureValueError):
        feature_value_to_python("v", bad)


def test_structured_values_survive_the_wire_bytes():
    for value in (
        pb_array([2, 3], *(pb_int(i) for i in range(6))),
        pb_vector(pb_real(3.0), pb_int(4)),
        pb_vector_quantity(pb_metre(3.0), pb_metre(4.0)),
    ):
        again = sysml_pb2.Value()
        again.ParseFromString(value.SerializeToString())
        assert again == value
        assert value_to_python(again) == value_to_python(value)


def test_a_service_without_the_capability_still_reports_unsupported():
    """An older service sends a null naming the value, which stays an error."""
    for text in (
        "unsupported: array Array(2, 3)[1, 2, 3, 4, 5, 6]",
        "unsupported: vector ⟨3.0, 4.0⟩",
        "unsupported: vector quantity ⟨3.0, 4.0⟩ [m]",
    ):
        null = sysml_pb2.Value(null=text)
        with pytest.raises(UnsupportedValueError):
            value_to_python(null)


def test_a_value_arm_this_client_predates_is_an_error_not_none():
    """A newer service's arm parses as an unknown field, which is not a null."""
    unknown = sysml_pb2.Value()
    unknown.ParseFromString(sysml_pb2.ServerInfoRequest().SerializeToString())
    with pytest.raises(UnsupportedValueError, match="does not know"):
        value_to_python(unknown)
    assert value_to_python(sysml_pb2.Value(null="")) is None


# --- Sending -------------------------------------------------------------


def test_structured_values_are_sent_as_their_own_arms():
    """Round trip: what the client sends is what it reads back."""
    conn = make_connection(Mock(), CURRENT)
    grid = Array((2, 2), (1, 2.5, Quantity(3.0, METRE), Vector((1, 2))))
    sent = conn._python_to_value(grid)

    assert sent.WhichOneof("kind") == "array"
    assert list(sent.array.dimensions) == [2, 2]
    assert [e.WhichOneof("kind") for e in sent.array.elements] == [
        "int_value", "real_value", "quantity", "vector",
    ]
    assert value_to_python(sent) == grid

    sent = conn._python_to_value(Vector((3.0, 4.0)))
    assert sent.WhichOneof("kind") == "vector"
    assert value_to_python(sent) == Vector((3.0, 4.0))

    sent = conn._python_to_value(VectorQuantity((Quantity(3.0, METRE), Quantity(4.0, METRE))))
    assert sent.WhichOneof("kind") == "vector_quantity"
    assert sent.vector_quantity.components[1].unit == "m"
    assert sent.vector_quantity.components[1].unit_term.factors[0].unit_id == "SI::metre"
    assert value_to_python(sent) == VectorQuantity((Quantity(3.0, METRE), Quantity(4.0, METRE)))

    nested = conn._python_to_value([1, [Vector((1.0,))]])
    assert value_to_python(nested) == [1, [Vector((1.0,))]]


def test_a_structured_value_is_not_sent_to_a_service_without_the_capability():
    """An older service would read the unknown arm as null, so nothing is sent."""
    stub = Mock()
    conn = make_connection(stub, OLD)

    for value in (
        Vector((3.0, 4.0)),
        Array((1,), (1,)),
        VectorQuantity((Quantity(1.0, METRE),)),
        [1, [Vector((3.0, 4.0))]],
        Array((1,), (Vector((1.0,)),)),
    ):
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.execute_action("S::scale", "hash", inputs={"x": value})
        assert excinfo.value.capability == CAPABILITY_STRUCTURED_VALUES
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.calc("S::length", "hash", arguments=[1, value])
        assert excinfo.value.capability == CAPABILITY_STRUCTURED_VALUES
    stub.ExecuteAction.assert_not_called()
    stub.EvaluateCalc.assert_not_called()

    stub.ExecuteAction.return_value = sysml_pb2.ExecuteActionResponse()
    assert conn.execute_action("S::scale", "hash", inputs={"x": [3.0, 4.0]}) == {}
    stub.ExecuteAction.assert_called_once()


def test_a_complex_in_an_array_still_needs_complex_values():
    stub = Mock()
    conn = make_connection(stub, (CAPABILITY_STRUCTURED_VALUES, CAPABILITY_VERIFICATION))

    arguments = [Array((1,), (1 + 2j,))]
    with pytest.raises(MissingCapabilityError) as excinfo:
        conn.calc("S::length", "hash", arguments=arguments)
    assert excinfo.value.capability == CAPABILITY_COMPLEX_VALUES
    stub.EvaluateCalc.assert_not_called()


def test_a_structured_value_is_sent_to_a_service_with_the_capability():
    stub = Mock()
    stub.EvaluateCalc.return_value = sysml_pb2.EvaluateCalcResponse(
        result=pb_vector(pb_real(6.0), pb_real(8.0))
    )
    conn = make_connection(stub, CURRENT)

    assert conn.calc("S::doubled", "hash", arguments=[Vector((3.0, 4.0))]).value == Vector((6.0, 8.0))
    request = stub.EvaluateCalc.call_args.args[0]
    assert request.arguments[0].WhichOneof("kind") == "vector"
    assert [c.real_value for c in request.arguments[0].vector.components] == [3.0, 4.0]


STRUCTURED_MODEL = """
package S {
    private import ScalarValues::*;
    private import Collections::*;
    private import VectorValues::*;
    private import VectorFunctions::*;
    private import Quantities::*;
    private import SI::*;
    attribute grid : Array { :>> dimensions = (2, 3); :>> elements = (1, 2, 3, 4, 5, 6); }
    attribute v : CartesianVectorValue = VectorOf((3.0, 4.0));
    attribute d : VectorQuantityValue = VectorOf((3.0, 4.0)) [m];
    calc def Length { in x : CartesianVectorValue; return : Real = norm(x); }
    calc length : Length;
    calc def Doubled { in x : CartesianVectorValue; return : CartesianVectorValue = cartesianVectorScalarMult(x, 2.0); }
    calc doubled : Doubled;
}
"""


@pytest.mark.integration
class TestStructuredAgainstTheService:
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
        self.model = self.conn.load_from_content(STRUCTURED_MODEL)

    def teardown_method(self):
        conn = self.__dict__.get("conn")
        if conn is not None:
            conn.close()

    def test_the_service_advertises_structured_values(self):
        assert self.conn.server_info().has(CAPABILITY_STRUCTURED_VALUES)

    def test_an_array_reads_with_its_shape(self):
        grid = self.conn.eval("S::grid", self.model.hash)

        assert grid == Array((2, 3), (1, 2, 3, 4, 5, 6))
        assert grid.nested() == [[1, 2, 3], [4, 5, 6]]

    def test_a_vector_reads_as_a_vector_of_reals(self):
        v = self.conn.eval("S::v", self.model.hash)

        assert v == Vector((3.0, 4.0))
        assert all(isinstance(c, float) for c in v)

    def test_a_vector_quantity_reads_with_its_units(self):
        d = self.conn.eval("S::d", self.model.hash)

        assert isinstance(d, VectorQuantity)
        assert d.magnitudes() == Vector((3.0, 4.0))
        assert d.unit.text == "m"
        assert d.unit.exponents() == {"SI::metre": 1.0}

    def test_a_vector_sent_as_a_calc_argument_is_the_vector_the_calc_sees(self):
        assert self.conn.calc("S::length", self.model.hash, arguments=[Vector((3.0, 4.0))]).value == 5.0
        doubled = self.conn.calc("S::doubled", self.model.hash, arguments=[Vector((3.0, 4.0))]).value
        assert doubled == Vector((6.0, 8.0))

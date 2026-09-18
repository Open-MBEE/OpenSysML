"""Tests for a bare measurement reference as a Python value.

A unit held by itself — ``SI::m``, a quantity's ``mRef``, ``m / s`` composed
by an operation — travels in an arm of its own, ``Value.measurement_ref``, so it
must arrive as one :class:`MeasurementRef` carrying the unit as written, its
reduction, and the declaration it names: not as the unsupported null a service
without the capability sends.
"""

from unittest.mock import Mock, patch

import pytest

from opensysml.capabilities import (
    CAPABILITY_COMPLEX_VALUES,
    CAPABILITY_FEATURE_VALUES,
    CAPABILITY_MEASUREMENT_REFS,
    CAPABILITY_STRUCTURED_VALUES,
    CAPABILITY_VERIFICATION,
    MissingCapabilityError,
)
from opensysml.connection import Connection
from opensysml.errors import UnsupportedValueError
from opensysml.proto import sysml_pb2
from opensysml.values import (
    Array,
    MeasurementRef,
    Quantity,
    Unit,
    UnitFactor,
    value_to_python,
)

from tests.service_gate import skip_or_fail_without_service


def pb_term(scale_num, scale_den, *factors):
    return sysml_pb2.UnitTerm(
        scale_num=scale_num,
        scale_den=scale_den,
        factors=[sysml_pb2.UnitFactor(unit_id=u, exponent=e) for u, e in factors],
    )


def pb_ref(unit, unit_term=None, unit_id=""):
    return sysml_pb2.Value(measurement_ref=sysml_pb2.MeasurementRef(
        unit=unit, unit_term=unit_term, unit_id=unit_id,
    ))


METRE = Unit("m", 1.0, 1.0, (UnitFactor("SI::metre", 1.0),), reduction_given=True)
KILOMETRE = Unit("km", 1000.0, 1.0, (UnitFactor("SI::metre", 1.0),), reduction_given=True)
METRE_PER_SECOND = Unit(
    "m/s", 1.0, 1.0,
    (UnitFactor("SI::metre", 1.0), UnitFactor("SI::second", -1.0)),
    reduction_given=True,
)
PB_METRE = pb_term(1.0, 1.0, ("SI::metre", 1.0))
PB_KILOMETRE = pb_term(1000.0, 1.0, ("SI::metre", 1.0))
PB_METRE_PER_SECOND = pb_term(1.0, 1.0, ("SI::metre", 1.0), ("SI::second", -1.0))


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
    CAPABILITY_MEASUREMENT_REFS,
    CAPABILITY_FEATURE_VALUES,
    CAPABILITY_VERIFICATION,
)
OLD = (
    CAPABILITY_COMPLEX_VALUES,
    CAPABILITY_STRUCTURED_VALUES,
    CAPABILITY_FEATURE_VALUES,
    CAPABILITY_VERIFICATION,
)


# --- Reading -------------------------------------------------------------


def test_a_named_reference_decodes_with_its_unit_and_declaration():
    got = value_to_python(pb_ref("km", PB_KILOMETRE, "SI::kilometre"))

    assert isinstance(got, MeasurementRef)
    assert got == MeasurementRef(KILOMETRE, "SI::kilometre")
    assert got.unit.text == "km"
    assert got.unit.scale_num == 1000.0
    assert got.unit.exponents() == {"SI::metre": 1.0}
    assert got.unit_id == "SI::kilometre"
    assert str(got) == "km"


def test_a_composed_reference_names_no_declaration():
    got = value_to_python(pb_ref("m/s", PB_METRE_PER_SECOND))

    assert got == MeasurementRef(METRE_PER_SECOND)
    assert got.unit_id == ""
    assert got.unit.exponents() == {"SI::metre": 1.0, "SI::second": -1.0}
    assert str(got) == "m/s"


def test_a_reference_converts_the_quantities_it_measures():
    km = value_to_python(pb_ref("km", PB_KILOMETRE, "SI::kilometre"))
    assert Quantity(3.0, km.unit).in_unit(METRE) == 3000.0
    assert km.unit.commensurable(METRE)
    assert not km.unit.commensurable(METRE_PER_SECOND)


def test_a_reference_nested_in_a_sequence_or_array_decodes_in_place():
    sequence = sysml_pb2.Value(sequence=sysml_pb2.ValueSequence(elements=[
        pb_ref("m", PB_METRE, "SI::metre"), sysml_pb2.Value(int_value=1),
    ]))
    assert value_to_python(sequence) == [MeasurementRef(METRE, "SI::metre"), 1]

    array = sysml_pb2.Value(array=sysml_pb2.Array(
        dimensions=[1], elements=[pb_ref("m", PB_METRE, "SI::metre")],
    ))
    assert value_to_python(array) == Array((1,), (MeasurementRef(METRE, "SI::metre"),))


def test_a_reference_naming_no_unit_is_reported():
    unnamed = pb_ref("")
    with pytest.raises(UnsupportedValueError, match="naming no unit"):
        value_to_python(unnamed)


def test_a_named_reference_without_its_reduction_is_reported():
    named = pb_ref("km")
    with pytest.raises(UnsupportedValueError, match="km carries no reduction"):
        value_to_python(named)
    identified = pb_ref("", unit_id="SI::kilometre")
    with pytest.raises(UnsupportedValueError, match="SI::kilometre carries no reduction"):
        value_to_python(identified)


def test_a_reference_survives_the_wire_bytes():
    for value in (
        pb_ref("km", PB_KILOMETRE, "SI::kilometre"),
        pb_ref("m/s", PB_METRE_PER_SECOND),
    ):
        again = sysml_pb2.Value()
        again.ParseFromString(value.SerializeToString())
        assert again == value
        assert value_to_python(again) == value_to_python(value)


def test_a_service_without_the_capability_still_reports_unsupported():
    """An older service sends a null naming the unit, which stays an error."""
    null = sysml_pb2.Value(null="unsupported: measurement reference m")
    with pytest.raises(UnsupportedValueError, match="measurement reference m"):
        value_to_python(null)


# --- Sending -------------------------------------------------------------


def test_a_reference_is_sent_as_its_own_arm():
    """Round trip: what the client sends is what it reads back."""
    conn = make_connection(Mock(), CURRENT)
    km = MeasurementRef(KILOMETRE, "SI::kilometre")

    sent = conn._python_to_value(km)
    assert sent.WhichOneof("kind") == "measurement_ref"
    assert sent.measurement_ref.unit == "km"
    assert sent.measurement_ref.unit_id == "SI::kilometre"
    assert sent.measurement_ref.unit_term == PB_KILOMETRE
    assert value_to_python(sent) == km

    speed = MeasurementRef(METRE_PER_SECOND)
    sent = conn._python_to_value(speed)
    assert sent.measurement_ref.unit_id == ""
    assert value_to_python(sent) == speed

    nested = conn._python_to_value([1, [km]])
    assert value_to_python(nested) == [1, [km]]


def test_an_unreduced_reference_is_refused_before_it_is_sent():
    conn = make_connection(Mock(), CURRENT)
    unreduced = MeasurementRef(Unit("km"), "SI::kilometre")
    with pytest.raises(UnsupportedValueError, match="carries no reduction"):
        conn._python_to_value(unreduced)


def test_a_reference_is_not_sent_to_a_service_without_the_capability():
    """An older service would read the unknown arm as null, so nothing is sent."""
    stub = Mock()
    conn = make_connection(stub, OLD)
    metre = MeasurementRef(METRE, "SI::metre")

    for value in (metre, [1, [metre]], Array((1,), (metre,))):
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.execute_action("M::convert", "hash", inputs={"target": value})
        assert excinfo.value.capability == CAPABILITY_MEASUREMENT_REFS
        arguments = [Quantity(3.0, KILOMETRE), value]
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.calc("M::toUnit", "hash", arguments=arguments)
        assert excinfo.value.capability == CAPABILITY_MEASUREMENT_REFS
    stub.ExecuteAction.assert_not_called()
    stub.EvaluateCalc.assert_not_called()

    # A quantity still travels: it never needed the capability.
    stub.EvaluateCalc.return_value = sysml_pb2.EvaluateCalcResponse(
        result=sysml_pb2.Value(real_value=3000.0)
    )
    assert conn.calc("M::magnitude", "hash", arguments=[Quantity(3.0, KILOMETRE)]).value == 3000.0
    stub.EvaluateCalc.assert_called_once()


def test_a_reference_is_sent_to_a_service_with_the_capability():
    stub = Mock()
    stub.EvaluateCalc.return_value = sysml_pb2.EvaluateCalcResponse(
        result=sysml_pb2.Value(quantity=sysml_pb2.Quantity(
            real_magnitude=3000.0, unit="m", unit_term=PB_METRE,
        ))
    )
    conn = make_connection(stub, CURRENT)

    got = conn.calc(
        "M::toUnit", "hash",
        arguments=[Quantity(3, KILOMETRE), MeasurementRef(METRE, "SI::metre")],
    ).value
    assert got == Quantity(3000.0, METRE)
    request = stub.EvaluateCalc.call_args.args[0]
    assert request.arguments[1].WhichOneof("kind") == "measurement_ref"
    assert request.arguments[1].measurement_ref.unit_id == "SI::metre"


MEASUREMENT_REF_MODEL = """
package M {
    private import ScalarValues::*;
    private import Quantities::*;
    private import MeasurementReferences::*;
    private import SI::*;
    private import QuantityCalculations::*;

    attribute q : ISQ::LengthValue = 3 [km];
    attribute u : MeasurementUnit = m;
    attribute speed = m / s;
    attribute units : MeasurementUnit[*] = (m, s);

    calc def ToUnit { in x : ScalarQuantityValue; in target : MeasurementUnit; return : ScalarQuantityValue = ConvertQuantity(x, target); }
    calc toUnit : ToUnit;
    calc def UnitOf { in x : ScalarQuantityValue; return : MeasurementUnit = x.mRef; }
    calc unitOf : UnitOf;
}
"""


@pytest.mark.integration
class TestMeasurementRefsAgainstTheService:
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
        self.model = self.conn.load_from_content(MEASUREMENT_REF_MODEL)

    def teardown_method(self):
        conn = self.__dict__.get("conn")
        if conn is not None:
            conn.close()

    def test_the_service_advertises_measurement_refs(self):
        assert self.conn.server_info().has(CAPABILITY_MEASUREMENT_REFS)

    def test_a_named_unit_reads_with_its_declaration(self):
        u = self.conn.eval("M::u", self.model.hash)

        assert isinstance(u, MeasurementRef)
        assert u.unit.text == "m"
        assert u.unit.exponents() == {"SI::metre": 1.0}
        assert u.unit_id == "SI::metre"

    def test_a_quantitys_unit_reads_as_the_declaration_it_was_written_in(self):
        km = self.conn.eval("M::q.mRef", self.model.hash)

        assert km.unit.text == "km"
        assert km.unit.scale_num == 1000.0
        assert km.unit_id == "SI::kilometre"

    def test_a_composed_unit_reads_with_its_reduction_and_no_declaration(self):
        speed = self.conn.eval("M::speed", self.model.hash)

        assert isinstance(speed, MeasurementRef)
        assert speed.unit.exponents() == {"SI::metre": 1.0, "SI::second": -1.0}
        assert speed.unit_id == ""

    def test_a_sequence_of_units_reads_one_reference_each(self):
        units = self.conn.eval("M::units", self.model.hash)

        assert [u.unit_id for u in units] == ["SI::metre", "SI::second"]

    def test_a_reference_sent_as_a_calc_argument_converts_the_quantity(self):
        km = self.conn.eval("M::q.mRef", self.model.hash)
        metre = self.conn.eval("M::u", self.model.hash)

        converted = self.conn.calc(
            "M::toUnit", self.model.hash, arguments=[Quantity(3, km.unit), metre]
        ).value
        assert converted == Quantity(3000.0, METRE)
        assert converted.unit.text == "m"

        assert self.conn.calc("M::unitOf", self.model.hash, arguments=[Quantity(3, km.unit)]).value == km

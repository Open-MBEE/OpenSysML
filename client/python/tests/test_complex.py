"""Tests for a complex number as a Python value.

A Complex travels as ``Value.complex``, one number with a real and an imaginary
part, so it must arrive as one Python ``complex`` — not as two floats, and not
as the unsupported null a service without the capability sends.
"""

from unittest.mock import Mock, patch

import pytest

from opensysml import typed as _t
from opensysml.capabilities import (
    CAPABILITY_COMPLEX_VALUES,
    CAPABILITY_FEATURE_VALUES,
    CAPABILITY_VERIFICATION,
    MissingCapabilityError,
)
from opensysml.connection import Connection
from opensysml.errors import FeatureValueError, TypeMismatchError, UnsupportedValueError
from opensysml.proto import sysml_pb2
from opensysml.values import feature_value_to_python, value_to_python

from tests.service_gate import skip_or_fail_without_service


def pb_complex(real, imaginary):
    return sysml_pb2.Value(complex=sysml_pb2.Complex(real=real, imaginary=imaginary))


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


CURRENT = (CAPABILITY_COMPLEX_VALUES, CAPABILITY_FEATURE_VALUES, CAPABILITY_VERIFICATION)
OLD = (CAPABILITY_FEATURE_VALUES, CAPABILITY_VERIFICATION)


def test_value_to_python_returns_one_complex():
    got = value_to_python(pb_complex(1.5, -2.0))

    assert got == complex(1.5, -2.0)
    assert isinstance(got, complex)
    assert got.real == 1.5
    assert got.imag == -2.0
    assert str(got) == "(1.5-2j)"


def test_both_parts_survive_at_their_extremes():
    for z in (0j, 1e300 + 1e-300j, -3.25 - 0j, complex(-0.0, 7.0)):
        assert value_to_python(pb_complex(z.real, z.imag)) == z


def test_an_empty_complex_message_is_zero():
    """Proto3 defaults are zero, so a Complex with neither part set is 0+0j."""
    assert value_to_python(sysml_pb2.Value(complex=sysml_pb2.Complex())) == 0j


def test_a_complex_is_not_a_float_nor_a_pair():
    got = value_to_python(pb_complex(1.5, 0.0))

    assert isinstance(got, complex)
    assert not isinstance(got, float)
    assert got != [1.5, 0.0]
    # Equal to the real it is, as Python numbers compare across kinds.
    assert got == 1.5


def test_complex_values_hash_and_compare_by_value():
    a = value_to_python(pb_complex(1.0, 2.0))
    b = value_to_python(pb_complex(1.0, 2.0))
    c = value_to_python(pb_complex(1.0, -2.0))

    assert a == b
    assert a != c
    assert len({a, b, c}) == 2


def test_a_sequence_of_complex_keeps_each_one():
    seq = sysml_pb2.Value(sequence=sysml_pb2.ValueSequence(elements=[
        pb_complex(1.0, 2.0),
        pb_complex(3.0, -4.0),
        sysml_pb2.Value(sequence=sysml_pb2.ValueSequence(elements=[pb_complex(0.0, 1.0)])),
    ]))

    assert value_to_python(seq) == [1 + 2j, 3 - 4j, [1j]]


def test_a_complex_slot_reads_as_a_complex():
    slot = sysml_pb2.FeatureValue(
        feature_name="z", value=pb_complex(1.5, -2.0), materialized=True
    )
    assert feature_value_to_python("z", slot) == 1.5 - 2j

    slots = sysml_pb2.FeatureValue(
        feature_name="zs", values=[pb_complex(1.0, 2.0), pb_complex(3.0, 4.0)],
        materialized=True,
    )
    assert feature_value_to_python("zs", slots) == [1 + 2j, 3 + 4j]


def test_a_complex_is_sent_as_a_complex():
    """Round trip: what the client sends is what it reads back."""
    conn = make_connection(Mock(), CURRENT)
    pb_value = conn._python_to_value(1.5 - 2j)

    assert pb_value.WhichOneof("kind") == "complex"
    assert pb_value.complex.real == 1.5
    assert pb_value.complex.imaginary == -2.0
    assert value_to_python(pb_value) == 1.5 - 2j

    nested = conn._python_to_value([1 + 2j, [3 - 4j]])
    assert value_to_python(nested) == [1 + 2j, [3 - 4j]]


def test_a_complex_is_not_sent_to_a_service_without_the_capability():
    """An older service would read the unknown arm as null, so nothing is sent."""
    stub = Mock()
    conn = make_connection(stub, OLD)

    for value in (1 + 2j, [1, [1 + 2j]]):
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.execute_action("C::conj", "hash", inputs={"z": value})
        assert excinfo.value.capability == CAPABILITY_COMPLEX_VALUES
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.calc("C::echo", "hash", arguments=[1, value])
        assert excinfo.value.capability == CAPABILITY_COMPLEX_VALUES
    stub.ExecuteAction.assert_not_called()
    stub.EvaluateCalc.assert_not_called()

    stub.ExecuteAction.return_value = sysml_pb2.ExecuteActionResponse()
    assert conn.execute_action("C::conj", "hash", inputs={"z": [1, 2.5]}) == {}
    stub.ExecuteAction.assert_called_once()


def test_a_complex_is_sent_to_a_service_with_the_capability():
    stub = Mock()
    stub.EvaluateCalc.return_value = sysml_pb2.EvaluateCalcResponse(
        result=pb_complex(1.5, -2.0)
    )
    conn = make_connection(stub, CURRENT)

    assert conn.calc("C::echo", "hash", arguments=[[1.5 - 2j]]).value == 1.5 - 2j
    request = stub.EvaluateCalc.call_args.args[0]
    assert request.arguments[0].sequence.elements[0].WhichOneof("kind") == "complex"


def test_a_complex_survives_the_wire_bytes():
    value = pb_complex(1.5, -2.0)
    again = sysml_pb2.Value()
    again.ParseFromString(value.SerializeToString())

    assert again == value
    assert value_to_python(again) == 1.5 - 2j


def test_as_complex_decodes_a_complex_and_widens_a_real():
    assert _t.as_complex("z", 1.5 - 2j) == 1.5 - 2j
    assert _t.as_complex("z", 1.5) == 1.5 + 0j
    assert _t.as_complex("z", 3) == 3 + 0j
    with pytest.raises(TypeMismatchError):
        _t.as_complex("z", True)
    with pytest.raises(TypeMismatchError):
        _t.as_complex("z", "1.5-2j")
    with pytest.raises(TypeMismatchError):
        _t.as_complex("z", [1.5, -2.0])


def test_a_service_without_the_capability_still_reports_unsupported():
    """An older service sends a null naming the value, which stays an error."""
    unsupported = sysml_pb2.Value(null="unsupported: complex number 1.5 - 2.0i")
    with pytest.raises(UnsupportedValueError):
        value_to_python(unsupported)

    slot = sysml_pb2.FeatureValue(feature_name="z", value=unsupported, materialized=True)
    with pytest.raises(FeatureValueError):
        feature_value_to_python("z", slot)


COMPLEX_MODEL = """
package C {
    private import ScalarValues::*;
    private import ComplexFunctions::*;
    part def Signal {
        attribute z : Complex = rect(1.5, -2.0);
        attribute zs : Complex[2] = (rect(1.0, 2.0), rect(3.0, 4.0));
    }
    calc def Echo { in z : Complex; return : Complex = z; }
    calc echo : Echo;
}
"""


@pytest.mark.integration
class TestComplexAgainstTheService:
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
        self.model = self.conn.load_from_content(COMPLEX_MODEL)

    def teardown_method(self):
        conn = self.__dict__.get("conn")
        if conn is not None:
            conn.close()

    def test_the_service_advertises_complex_values(self):
        assert self.conn.server_info().has(CAPABILITY_COMPLEX_VALUES)

    def test_complex_slots_read_as_complex(self):
        signal = self.conn.instantiate("C::Signal", self.model.hash)

        assert signal.z == 1.5 - 2j
        assert isinstance(signal.z, complex)
        assert signal.zs == [1 + 2j, 3 + 4j]

    def test_an_evaluated_complex_expression_reads_as_a_complex(self):
        assert self.conn.eval("ComplexFunctions::rect(1.0, -1.0)", self.model.hash) == 1 - 1j

    def test_a_complex_sent_as_a_calc_argument_round_trips(self):
        echoed = self.conn.calc("C::echo", self.model.hash, arguments=[1.5 - 2j]).value

        assert echoed == 1.5 - 2j
        assert isinstance(echoed, complex)

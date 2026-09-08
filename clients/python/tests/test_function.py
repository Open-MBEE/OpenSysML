"""Tests for a calc held as a value as a Python value.

A calc definition, or a calc usage with an input no read could supply — ``Sq``
in ``Fn(Sq, 3.0)``, the ``f`` of ``in calc f {...}`` — travels in an arm of its
own, ``Value.function``, so it must arrive as one :class:`Function` naming the
declaration it is a value of and the object it was read against: not as the
unsupported null a service without the capability sends.
"""

from unittest.mock import Mock

import pytest

from opensysml.capabilities import (
    CAPABILITY_COMPLEX_VALUES,
    CAPABILITY_FEATURE_VALUES,
    CAPABILITY_FUNCTION_VALUES,
    CAPABILITY_MEASUREMENT_REFS,
    CAPABILITY_STRUCTURED_VALUES,
    CAPABILITY_VERIFICATION,
    MissingCapabilityError,
)
from opensysml.connection import Connection
from opensysml.errors import ExecutionError, UnsupportedValueError
from opensysml.proto import sysml_pb2
from opensysml.values import Array, Function, value_to_python

from tests.service_gate import skip_or_fail_without_service
from tests.test_measurement_ref import make_connection


def pb_fn(calc_id, self_id=0):
    return sysml_pb2.Value(function=sysml_pb2.Function(calc_id=calc_id, self_id=self_id))


CURRENT = (
    CAPABILITY_COMPLEX_VALUES,
    CAPABILITY_STRUCTURED_VALUES,
    CAPABILITY_MEASUREMENT_REFS,
    CAPABILITY_FUNCTION_VALUES,
    CAPABILITY_FEATURE_VALUES,
    CAPABILITY_VERIFICATION,
)
OLD = tuple(c for c in CURRENT if c != CAPABILITY_FUNCTION_VALUES)


# --- Reading -------------------------------------------------------------


def test_a_function_decodes_as_the_calc_it_names():
    sq = value_to_python(pb_fn("Demo::Sq"))
    assert sq == Function("Demo::Sq")
    assert sq.self_id == 0
    assert str(sq) == "Demo::Sq"


def test_a_function_read_off_an_object_keeps_the_object():
    scale = value_to_python(pb_fn("Demo::Scaler::scale", 7))
    assert scale == Function("Demo::Scaler::scale", 7)
    # The object is part of the identity: another object's read is another value.
    assert scale != Function("Demo::Scaler::scale", 8)
    assert scale != Function("Demo::Scaler::scale")


def test_a_function_nested_in_a_sequence_or_array_decodes_in_place():
    seq = sysml_pb2.Value(sequence=sysml_pb2.ValueSequence(
        elements=[pb_fn("Demo::Sq"), pb_fn("Demo::Cube")]
    ))
    assert value_to_python(seq) == [Function("Demo::Sq"), Function("Demo::Cube")]
    arr = sysml_pb2.Value(array=sysml_pb2.Array(
        dimensions=[1], elements=[pb_fn("Demo::Sq")]
    ))
    assert value_to_python(arr) == Array((1,), (Function("Demo::Sq"),))


def test_a_function_naming_no_calc_is_reported():
    with pytest.raises(UnsupportedValueError, match="naming no calc"):
        value_to_python(pb_fn(""))
    with pytest.raises(UnsupportedValueError, match="naming no calc"):
        value_to_python(pb_fn("", 3))


def test_a_function_survives_the_wire_bytes():
    for value in (pb_fn("Demo::Sq"), pb_fn("Demo::Scaler::scale", 7)):
        again = sysml_pb2.Value()
        again.ParseFromString(value.SerializeToString())
        assert again == value
        assert value_to_python(again) == value_to_python(value)


def test_a_service_without_the_capability_still_reports_unsupported():
    """An older service sends a null naming the calc, which stays an error."""
    null = sysml_pb2.Value(null="unsupported: function Demo::Sq")
    with pytest.raises(UnsupportedValueError, match="function Demo::Sq"):
        value_to_python(null)


# --- Sending -------------------------------------------------------------


def test_a_function_is_sent_as_its_own_arm():
    """Round trip: what the client sends is what it reads back."""
    conn = make_connection(Mock(), CURRENT)
    sq = Function("Demo::Sq")

    sent = conn._python_to_value(sq)
    assert sent.WhichOneof("kind") == "function"
    assert sent.function.calc_id == "Demo::Sq"
    assert sent.function.self_id == 0
    assert value_to_python(sent) == sq

    scale = Function("Demo::Scaler::scale", 7)
    sent = conn._python_to_value(scale)
    assert sent.function.self_id == 7
    assert value_to_python(sent) == scale

    nested = conn._python_to_value([1, [sq]])
    assert value_to_python(nested) == [1, [sq]]


def test_a_function_naming_no_calc_is_refused_before_it_is_sent():
    conn = make_connection(Mock(), CURRENT)
    with pytest.raises(UnsupportedValueError, match="naming no calc"):
        conn._python_to_value(Function(""))


def test_a_function_is_not_sent_to_a_service_without_the_capability():
    """An older service would read the unknown arm as null, so nothing is sent."""
    stub = Mock()
    conn = make_connection(stub, OLD)
    sq = Function("Demo::Sq")

    for value in (sq, [1, [sq]], Array((1,), (sq,))):
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.execute_action("Demo::apply", "hash", inputs={"f": value})
        assert excinfo.value.capability == CAPABILITY_FUNCTION_VALUES
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.calc("Demo::fn", "hash", arguments=[value, 3.0])
        assert excinfo.value.capability == CAPABILITY_FUNCTION_VALUES
    stub.ExecuteAction.assert_not_called()
    stub.EvaluateCalc.assert_not_called()

    # A number still travels: it never needed the capability.
    stub.EvaluateCalc.return_value = sysml_pb2.EvaluateCalcResponse(
        result=sysml_pb2.Value(real_value=9.0)
    )
    assert conn.calc("Demo::sq", "hash", arguments=[3.0]).value == 9.0
    stub.EvaluateCalc.assert_called_once()


def test_a_function_is_sent_to_a_service_with_the_capability():
    stub = Mock()
    stub.EvaluateCalc.return_value = sysml_pb2.EvaluateCalcResponse(
        result=sysml_pb2.Value(real_value=9.0)
    )
    conn = make_connection(stub, CURRENT)

    got = conn.calc("Demo::fn", "hash", arguments=[Function("Demo::Sq"), 3.0]).value
    assert got == 9.0
    request = stub.EvaluateCalc.call_args.args[0]
    assert request.arguments[0].WhichOneof("kind") == "function"
    assert request.arguments[0].function.calc_id == "Demo::Sq"


FUNCTION_MODEL = """
package Demo {
    private import ScalarValues::*;

    calc def Sq { in v : Real; return : Real = v * v; }
    calc def Cube { in v : Real; return : Real = v * v * v; }
    calc def Fn { in calc f { in v : Real; return : Real; } in a : Real; return : Real = f(a); }
    calc fn : Fn;
    calc def UseFn { return : Real = Fn(Sq, 3.0); }
    calc useFn : UseFn;

    calc def Identity { in calc f { in v : Real; return : Real; } return r = f; }
    attribute pick = Identity(Sq);
    attribute picks = (Identity(Sq), Identity(Cube));

    part def Scaler {
        attribute k : Real = 2.0;
        calc scale { in x : Real; return : Real = x * k; }
    }
    part holder : Scaler;
    attribute scaler = holder.scale;
}
"""


@pytest.mark.integration
class TestFunctionsAgainstTheService:
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
        self.model = self.conn.load_from_content(FUNCTION_MODEL)

    def teardown_method(self):
        conn = self.__dict__.get("conn")
        if conn is not None:
            conn.close()

    def test_the_service_advertises_function_values(self):
        assert self.conn.server_info().has(CAPABILITY_FUNCTION_VALUES)

    def test_a_calc_definition_reads_as_a_function(self):
        sq = self.conn.eval("Demo::pick", self.model.hash)
        assert sq == Function("Demo::Sq")

        assert self.conn.eval("Demo::picks", self.model.hash) == [
            Function("Demo::Sq"), Function("Demo::Cube"),
        ]

    def test_a_calc_read_off_an_object_names_the_object(self):
        scale = self.conn.eval("Demo::scaler", self.model.hash)
        assert isinstance(scale, Function)
        assert scale.calc_id == "Demo::Scaler::scale"
        assert scale.self_id != 0

    def test_a_function_sent_as_a_calc_argument_is_invoked(self):
        assert self.conn.calc("Demo::useFn", self.model.hash).value == 9.0
        got = self.conn.calc(
            "Demo::fn", self.model.hash, arguments=[Function("Demo::Cube"), 2.0]
        ).value
        assert got == 8.0

    def test_a_function_naming_no_calc_of_the_model_is_refused(self):
        with pytest.raises(ExecutionError):
            self.conn.calc(
                "Demo::fn", self.model.hash, arguments=[Function("Demo::Missing"), 2.0]
            )

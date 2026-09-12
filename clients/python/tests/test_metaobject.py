"""Tests for an element reflected on as a Python value.

``x meta KerML::Feature``, and the last element of ``x.metadata``, travel in an
arm of their own, ``Value.metaobject``, so they must arrive as one
:class:`Metaobject` naming the element and its own metaclass: not as the
unsupported null a service without the capability sends.
"""

from unittest.mock import Mock

import pytest

from opensysml.capabilities import (
    CAPABILITY_COMPLEX_VALUES,
    CAPABILITY_FEATURE_VALUES,
    CAPABILITY_FUNCTION_VALUES,
    CAPABILITY_MEASUREMENT_REFS,
    CAPABILITY_METAOBJECT_VALUES,
    CAPABILITY_STRUCTURED_VALUES,
    CAPABILITY_VERIFICATION,
    MissingCapabilityError,
)
from opensysml.connection import Connection
from opensysml.errors import ExecutionError, UnsupportedValueError
from opensysml.proto import sysml_pb2
from opensysml.values import Array, Metaobject, SetValue, same_value, value_to_python

from tests.service_gate import skip_or_fail_without_service
from tests.test_measurement_ref import make_connection


def pb_meta(element_id, metaclass_id=""):
    return sysml_pb2.Value(metaobject=sysml_pb2.Metaobject(
        element_id=element_id, metaclass_id=metaclass_id,
    ))


CURRENT = (
    CAPABILITY_COMPLEX_VALUES,
    CAPABILITY_STRUCTURED_VALUES,
    CAPABILITY_MEASUREMENT_REFS,
    CAPABILITY_FUNCTION_VALUES,
    CAPABILITY_METAOBJECT_VALUES,
    CAPABILITY_FEATURE_VALUES,
    CAPABILITY_VERIFICATION,
)
OLD = tuple(c for c in CURRENT if c != CAPABILITY_METAOBJECT_VALUES)

SEAT_BELT = Metaobject("Demo::seatBelt", "SysML::Systems::PartUsage")


# --- Reading -------------------------------------------------------------


def test_a_metaobject_decodes_as_the_element_and_its_metaclass():
    meta = value_to_python(pb_meta("Demo::seatBelt", "SysML::Systems::PartUsage"))
    assert meta == SEAT_BELT
    assert meta.element_id == "Demo::seatBelt"
    assert meta.metaclass_id == "SysML::Systems::PartUsage"
    assert str(meta) == "meta(Demo::seatBelt : SysML::Systems::PartUsage)"


def test_a_metaobject_is_its_element_whatever_it_was_cast_to():
    """Identity is the element's: the metaclass names, it does not distinguish."""
    assert Metaobject("Demo::seatBelt") == SEAT_BELT
    assert hash(Metaobject("Demo::seatBelt")) == hash(SEAT_BELT)
    assert same_value(Metaobject("Demo::seatBelt"), SEAT_BELT)
    assert SEAT_BELT != Metaobject("Demo::Vehicle", "SysML::Systems::PartDefinition")
    assert SEAT_BELT != "Demo::seatBelt"
    assert Metaobject("Demo::seatBelt") in SetValue([SEAT_BELT])


def test_a_metaobject_nested_in_a_sequence_or_array_decodes_in_place():
    seq = sysml_pb2.Value(sequence=sysml_pb2.ValueSequence(
        elements=[sysml_pb2.Value(instance_id=1), pb_meta("Demo::seatBelt", "SysML::Systems::PartUsage")]
    ))
    assert value_to_python(seq)[1] == SEAT_BELT
    arr = sysml_pb2.Value(array=sysml_pb2.Array(
        dimensions=[1], elements=[pb_meta("Demo::seatBelt")]
    ))
    assert value_to_python(arr) == Array((1,), (SEAT_BELT,))


def test_a_metaobject_naming_no_element_is_reported():
    with pytest.raises(UnsupportedValueError, match="naming no element"):
        value_to_python(pb_meta(""))
    with pytest.raises(UnsupportedValueError, match="naming no element"):
        value_to_python(pb_meta("", "KerML::Feature"))


def test_a_metaobject_survives_the_wire_bytes():
    value = pb_meta("Demo::seatBelt", "SysML::Systems::PartUsage")
    again = sysml_pb2.Value()
    again.ParseFromString(value.SerializeToString())
    assert again == value
    assert value_to_python(again) == value_to_python(value)
    assert value_to_python(again).metaclass_id == "SysML::Systems::PartUsage"


def test_a_service_without_the_capability_still_reports_unsupported():
    """An older service sends a null naming the element, which stays an error."""
    null = sysml_pb2.Value(null="unsupported: metaobject Demo::seatBelt : SysML::Systems::PartUsage")
    with pytest.raises(UnsupportedValueError, match="metaobject Demo::seatBelt"):
        value_to_python(null)


# --- Sending -------------------------------------------------------------


def test_a_metaobject_is_sent_as_its_own_arm():
    """Round trip: what the client sends is what it reads back."""
    conn = make_connection(Mock(), CURRENT)

    sent = conn._python_to_value(SEAT_BELT)
    assert sent.WhichOneof("kind") == "metaobject"
    assert sent.metaobject.element_id == "Demo::seatBelt"
    assert sent.metaobject.metaclass_id == "SysML::Systems::PartUsage"
    assert value_to_python(sent) == SEAT_BELT

    # The metaclass may be left to the model.
    bare = conn._python_to_value(Metaobject("Demo::seatBelt"))
    assert bare.metaobject.element_id == "Demo::seatBelt"
    assert bare.metaobject.metaclass_id == ""

    nested = conn._python_to_value([1, [SEAT_BELT]])
    assert value_to_python(nested) == [1, [SEAT_BELT]]


def test_a_metaobject_naming_no_element_is_refused_before_it_is_sent():
    conn = make_connection(Mock(), CURRENT)
    with pytest.raises(UnsupportedValueError, match="naming no element"):
        conn._python_to_value(Metaobject(""))


def test_a_metaobject_is_not_sent_to_a_service_without_the_capability():
    """An older service would read the unknown arm as null, so nothing is sent."""
    stub = Mock()
    conn = make_connection(stub, OLD)

    for value in (SEAT_BELT, [1, [SEAT_BELT]], Array((1,), (SEAT_BELT,))):
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.execute_action("Demo::describe", "hash", inputs={"m": value})
        assert excinfo.value.capability == CAPABILITY_METAOBJECT_VALUES
        with pytest.raises(MissingCapabilityError) as excinfo:
            conn.calc("Demo::nameOf", "hash", arguments=[value])
        assert excinfo.value.capability == CAPABILITY_METAOBJECT_VALUES
    stub.ExecuteAction.assert_not_called()
    stub.EvaluateCalc.assert_not_called()

    # A number still travels: it never needed the capability.
    stub.EvaluateCalc.return_value = sysml_pb2.EvaluateCalcResponse(
        result=sysml_pb2.Value(real_value=9.0)
    )
    assert conn.calc("Demo::sq", "hash", arguments=[3.0]).value == 9.0
    stub.EvaluateCalc.assert_called_once()


def test_a_metaobject_is_sent_to_a_service_with_the_capability():
    stub = Mock()
    stub.EvaluateCalc.return_value = sysml_pb2.EvaluateCalcResponse(
        result=sysml_pb2.Value(string_value="seatBelt")
    )
    conn = make_connection(stub, CURRENT)

    got = conn.calc("Demo::nameOf", "hash", arguments=[SEAT_BELT]).value
    assert got == "seatBelt"
    request = stub.EvaluateCalc.call_args.args[0]
    assert request.arguments[0].WhichOneof("kind") == "metaobject"
    assert request.arguments[0].metaobject.element_id == "Demo::seatBelt"


METAOBJECT_MODEL = """
package Demo {
    private import ScalarValues::*;

    metadata def Safety { attribute level : Integer = 2; }
    part def Vehicle { attribute mass : Real; }
    part seatBelt : Vehicle { @Safety { level = 4; } }

    attribute asFeature [*] = seatBelt meta KerML::Feature;
    attribute everything [*] = seatBelt.metadata;
    attribute notADefinition [*] = seatBelt meta SysML::PartDefinition;

    calc def NameOf { in m : Metaobjects::Metaobject; return : String = m.declaredName; }
    calc nameOf : NameOf;
    calc def SameAs {
        in m : Metaobjects::Metaobject;
        return : Boolean = m === (seatBelt meta KerML::Type)#(1);
    }
    calc sameAs : SameAs;
}
"""


@pytest.mark.integration
class TestMetaobjectsAgainstTheService:
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
        self.model = self.conn.load_from_content(METAOBJECT_MODEL)

    def teardown_method(self):
        conn = self.__dict__.get("conn")
        if conn is not None:
            conn.close()

    def test_the_service_advertises_metaobject_values(self):
        assert self.conn.server_info().has(CAPABILITY_METAOBJECT_VALUES)

    def test_a_meta_cast_reads_as_the_element_under_its_own_metaclass(self):
        assert self.conn.eval("Demo::asFeature", self.model.hash) == [SEAT_BELT]
        assert self.conn.eval("Demo::notADefinition", self.model.hash) == []

    def test_metadata_lists_the_annotations_then_the_metaobject(self):
        everything = self.conn.eval("Demo::everything", self.model.hash)
        assert len(everything) == 2
        assert not isinstance(everything[0], Metaobject)
        assert everything[1] == SEAT_BELT
        assert everything[1].metaclass_id == "SysML::Systems::PartUsage"

    def test_a_metaobject_sent_as_a_calc_argument_is_the_element(self):
        assert self.conn.calc("Demo::nameOf", self.model.hash, arguments=[SEAT_BELT]).value == "seatBelt"
        bare = Metaobject("Demo::seatBelt")
        assert self.conn.calc("Demo::sameAs", self.model.hash, arguments=[bare]).value is True

    def test_a_metaobject_naming_no_element_of_the_model_is_refused(self):
        for bad in (Metaobject("Demo::missing"), Metaobject("Demo::seatBelt", "SysML::Systems::PartDefinition")):
            with pytest.raises(ExecutionError):
                self.conn.calc("Demo::nameOf", self.model.hash, arguments=[bad])

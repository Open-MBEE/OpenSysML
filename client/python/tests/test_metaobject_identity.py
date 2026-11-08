"""An element with no qualified name reflected on as a Python value.

A metaobject names its element by the id a client keeps for any element: the
qualified name, the positional id of an unnamed member (``Owner::@2``), or the
element form's ``@id`` of an element the model synthesizes (an accept's or
send's argument). The same id reads with :meth:`Connection.get_element_properties`
and binds the same element when sent back.
"""

import pytest

from opensysml.connection import Connection
from opensysml.proto import sysml_pb2
from opensysml.values import Metaobject

from tests.service_gate import skip_or_fail_without_service

ANONYMOUS_MODEL = """
package SendActions {
    private import ScalarValues::*;

    item def Alarm { attribute level : Integer; }
    port def AlarmPort { out item alarm : Alarm; }
    part def Monitor {
        port alarmPort : AlarmPort;
        action supervise {
            action notifyVia send new Alarm(3) via alarmPort;
        }
        state sm {
            entry; then idle;
            state idle;
            transition relay first idle accept alarm : Alarm via alarmPort then idle;
        }
    }
    part def V;
    part outer : V { part : V; part named : V; }

    attribute features [*] = (outer meta KerML::Type).ownedFeature;
    attribute receiverArgument [*] =
        (Monitor::sm::relay meta SysML::TransitionUsage).triggerAction.receiverArgument;
    attribute payloadArgument [*] =
        (Monitor::supervise::notifyVia meta SysML::SendActionUsage).payloadArgument;

    calc def IsTheAnonymousPart {
        in m : Metaobjects::Metaobject;
        return : Boolean[1] = m === (outer meta KerML::Type).ownedFeature#(1);
    }
    calc isTheAnonymousPart : IsTheAnonymousPart;
    calc def IsTheReceiverArgument {
        in m : Metaobjects::Metaobject;
        return : Boolean[1] = m === receiverArgument#(1);
    }
    calc isTheReceiverArgument : IsTheReceiverArgument;
}
"""

RECEIVER_ARGUMENT_ID = "SendActions__Monitor__sm__relay___40trigger___401_pvalue"


@pytest.mark.integration
class TestUnnamedMetaobjectsAgainstTheService:
    """A metaobject of an unnamed element arrives, reads back, and is sent back."""

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
        self.model = self.conn.load_from_content(ANONYMOUS_MODEL)

    def teardown_method(self):
        conn = self.__dict__.get("conn")
        if conn is not None:
            conn.close()

    def test_an_unnamed_member_arrives_under_its_positional_id(self):
        features = self.conn.eval("SendActions::features", self.model.hash)
        assert features == [
            Metaobject("SendActions::outer::@0"),
            Metaobject("SendActions::outer::named"),
        ]
        assert all(m.metaclass_id == "SysML::Systems::PartUsage" for m in features)

    def test_an_accept_argument_arrives_under_the_element_forms_id(self):
        [receiver] = self.conn.eval("SendActions::receiverArgument", self.model.hash)
        assert receiver == Metaobject(RECEIVER_ARGUMENT_ID)
        assert receiver.metaclass_id.endswith("::FeatureReferenceExpression")

    def test_a_metaobjects_element_reads_with_get_element_properties(self):
        for expression, expected_id in (
            ("SendActions::features", "SendActions__outer___400"),
            ("SendActions::receiverArgument", RECEIVER_ARGUMENT_ID),
        ):
            meta = self.conn.eval(expression, self.model.hash)[0]
            properties = self.conn.get_element_properties(self.model.hash, meta.element_id)
            assert properties.element.id == expected_id
            assert properties.element.metaclass in ("PartUsage", "FeatureReferenceExpression")

    def test_a_metaobject_of_an_unnamed_element_sent_back_is_the_same_element(self):
        anonymous = self.conn.eval("SendActions::features", self.model.hash)[0]
        assert self.conn.calc(
            "SendActions::isTheAnonymousPart", self.model.hash, arguments=[anonymous]
        ).value is True
        [receiver] = self.conn.eval("SendActions::receiverArgument", self.model.hash)
        assert self.conn.calc(
            "SendActions::isTheReceiverArgument", self.model.hash, arguments=[receiver]
        ).value is True
        bare = Metaobject(receiver.element_id)
        assert self.conn.calc(
            "SendActions::isTheReceiverArgument", self.model.hash, arguments=[bare]
        ).value is True

    def test_a_send_actions_payload_argument_is_a_metaobject_or_nothing(self):
        # The send's arguments reach the wire by the same ids once the model
        # classifies `notifyVia` as a SendActionUsage; until then nothing is derived.
        payload = self.conn.eval("SendActions::payloadArgument", self.model.hash)
        for meta in payload:
            assert isinstance(meta, Metaobject)
            properties = self.conn.get_element_properties(self.model.hash, meta.element_id)
            assert properties.element.metaclass == "ConstructorExpression"

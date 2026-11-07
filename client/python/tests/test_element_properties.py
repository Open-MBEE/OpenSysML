"""The element-properties read: decoding, the capability gate, and the model
and symbol entry points, plus what the real service answers."""

from unittest.mock import Mock, patch

import pytest

from opensysml import (
    CAPABILITY_ELEMENT_PROPERTIES,
    Connection,
    ElementIdentity,
    ElementProperties,
    ElementProperty,
    Model,
    Symbol,
)
from opensysml.capabilities import MissingCapabilityError, ServerInfo
from opensysml.element_properties import element_properties_of, property_value_of
from opensysml.errors import ConversionError, SymbolNotFoundError
from opensysml.proto import sysml_pb2

from tests.service_gate import skip_or_fail_without_service


def _response():
    return sysml_pb2.GetElementPropertiesResponse(
        element=sysml_pb2.ElementRef(id="EP__Pair", qualified_name="EP::Pair", metaclass="PartDefinition"),
        properties=[
            sysml_pb2.ElementProperty(
                name="inheritedFeature", defining_class="Type", many=True, derived=True,
                values=[
                    sysml_pb2.ElementPropertyValue(element=sysml_pb2.ElementRef(
                        id="EP__Unit__p", qualified_name="EP::Unit::p", metaclass="PortUsage")),
                    sysml_pb2.ElementPropertyValue(element=sysml_pb2.ElementRef(
                        id="4e03b9fa", qualified_name="Parts::Part::subparts", metaclass="PartUsage")),
                ],
            ),
            sysml_pb2.ElementProperty(
                name="isAbstract", defining_class="Type",
                values=[sysml_pb2.ElementPropertyValue(bool_value=False)],
            ),
            sysml_pb2.ElementProperty(name="ownedSubclassification", defining_class="Classifier", many=True, derived=True),
            sysml_pb2.ElementProperty(name="connectorEnd", unsupported="PartDefinition declares no property connectorEnd"),
        ],
    )


def test_a_response_decodes_as_a_mapping_of_properties():
    read = element_properties_of(_response())
    assert read.element == ElementIdentity("EP__Pair", "EP::Pair", "PartDefinition")
    assert str(read.element) == "EP::Pair"
    assert len(read) == 4 and "isAbstract" in read and "name" not in read
    inherited = read["inheritedFeature"]
    assert isinstance(inherited, ElementProperty)
    assert inherited.defining_class == "Type" and inherited.many and inherited.derived
    assert [str(e) for e in inherited.elements] == ["EP::Unit::p", "Parts::Part::subparts"]
    assert inherited.elements[1].metaclass == "PartUsage"
    assert read["isAbstract"].value is False and not read["isAbstract"].many
    assert read["ownedSubclassification"].supported and not read["ownedSubclassification"]
    assert read["ownedSubclassification"].value is None
    assert not read["connectorEnd"].supported
    assert "declares no property" in read["connectorEnd"].unsupported
    assert read.get("name") is None
    assert [p.name for p in read] == ["inheritedFeature", "isAbstract", "ownedSubclassification", "connectorEnd"]


def test_every_value_arm_decodes_to_its_python_value():
    assert property_value_of(sysml_pb2.ElementPropertyValue(string_value="out")) == "out"
    assert property_value_of(sysml_pb2.ElementPropertyValue(bool_value=True)) is True
    assert property_value_of(sysml_pb2.ElementPropertyValue(int_value=3)) == 3
    assert property_value_of(sysml_pb2.ElementPropertyValue(real_value=2.5)) == 2.5
    assert property_value_of(sysml_pb2.ElementPropertyValue(number="1" * 30)) == "1" * 30
    identity = property_value_of(sysml_pb2.ElementPropertyValue(element=sysml_pb2.ElementRef(id="x")))
    assert identity == ElementIdentity("x") and str(identity) == "x"
    with pytest.raises(ValueError):
        property_value_of(sysml_pb2.ElementPropertyValue())


def _connection(stub, capabilities=(CAPABILITY_ELEMENT_PROPERTIES,)):
    with patch("grpc.insecure_channel"), patch(
        "opensysml.proto.sysml_pb2_grpc.SysMLServiceStub", return_value=stub
    ):
        conn = Connection(auto_start=False)
    conn.server_info = Mock(return_value=ServerInfo(
        version="v0.13.0", capabilities=frozenset(capabilities), answered=True, origin="sysml-grpc",
    ))
    return conn


def test_the_connection_sends_the_element_and_the_names():
    stub = Mock()
    stub.GetElementProperties.return_value = _response()
    conn = _connection(stub)
    read = conn.get_element_properties("hash1", "EP::Pair", ["inheritedFeature", "isAbstract"])
    request = stub.GetElementProperties.call_args[0][0]
    assert request.model_hash == "hash1" and request.element_id == "EP::Pair"
    assert list(request.properties) == ["inheritedFeature", "isAbstract"]
    assert isinstance(read, ElementProperties) and read.element.qualified_name == "EP::Pair"


def test_a_service_without_the_capability_is_refused_before_the_call():
    stub = Mock()
    conn = _connection(stub, capabilities=("query",))
    with pytest.raises(MissingCapabilityError, match="element_properties"):
        conn.get_element_properties("hash1", "EP::Pair")
    stub.GetElementProperties.assert_not_called()


def test_an_in_band_error_is_raised_by_kind():
    stub = Mock()
    stub.GetElementProperties.return_value = sysml_pb2.GetElementPropertiesResponse(
        error="element not found: EP::Nowhere")
    conn = _connection(stub)
    with pytest.raises(SymbolNotFoundError, match="EP::Nowhere"):
        conn.get_element_properties("hash1", "EP::Nowhere")
    stub.GetElementProperties.return_value = sysml_pb2.GetElementPropertiesResponse(
        error="broken.sysml: 1 syntax error")
    with pytest.raises(ConversionError, match="syntax error"):
        conn.get_element_properties("hash1", "EP")


def test_the_model_and_the_symbol_reach_the_connection():
    client = Mock()
    client.get_element_properties.return_value = element_properties_of(_response())
    parsed = sysml_pb2.ParseFileResponse(
        model_hash="hash1", root=sysml_pb2.SymbolInfo(id="EP", name="EP", kind="Package"))
    model = Model(parsed, client)
    read = model.element_properties("EP::Pair", "inheritedFeature")
    client.get_element_properties.assert_called_once_with("hash1", "EP::Pair", ("inheritedFeature",))
    assert read["isAbstract"].value is False

    symbol = Symbol(sysml_pb2.SymbolInfo(id="EP::Pair", name="Pair", kind="PartDef"), client, "hash1")
    assert symbol.property("isAbstract").value is False
    client.get_element_properties.assert_called_with("hash1", "EP::Pair", ("isAbstract",))
    symbol.properties()
    client.get_element_properties.assert_called_with("hash1", "EP::Pair", ())
    with pytest.raises(ValueError):
        Symbol(sysml_pb2.SymbolInfo(id="EP::Pair"), None, "hash1").properties()


ELEMENT_PROPERTIES_MODEL = """
package EP {
    port def CmdPort { out attribute cmd : ScalarValues::Integer; in attribute ack : ScalarValues::Boolean; }
    part def Unit { port p : CmdPort; port q : ~CmdPort; }
    part def Pair :> Unit {
        part u1 : Unit;
        part u2 : Unit;
        connect u1 to u2;
        attribute x : ScalarValues::Real = 2.0 + 1.0;
    }
}
"""


@pytest.mark.integration
class TestElementPropertiesAgainstTheService:
    """What the real service answers for a real model."""

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
        self.model = self.conn.load_from_content(ELEMENT_PROPERTIES_MODEL)

    def teardown_method(self):
        conn = self.__dict__.get("conn")
        if conn is not None:
            conn.close()

    def test_the_service_advertises_element_properties(self):
        assert self.conn.server_info().has(CAPABILITY_ELEMENT_PROPERTIES)

    def test_inherited_features_name_the_general_and_the_library(self):
        read = self.model.element_properties("EP::Pair", "inheritedFeature")
        names = [str(e) for e in read["inheritedFeature"].elements]
        assert names[:2] == ["EP::Unit::p", "EP::Unit::q"]
        assert any(name.startswith("Parts::") for name in names)

    def test_direction_and_booleans_read_as_literals(self):
        assert self.model["EP::CmdPort::cmd"].property("direction").value == "out"
        assert self.model["EP::CmdPort::ack"].property("direction").value == "in"
        assert self.model["EP::Pair::u1"].property("isComposite").value is True

    def test_an_unnamed_connector_is_read_by_position_and_walked_by_form_id(self):
        connector = self.model.element_properties("EP::Pair::@2", "relatedFeature", "sourceFeature", "connectorEnd")
        assert connector.element.metaclass == "ConnectionUsage" and connector.element.qualified_name == ""
        assert [str(e) for e in connector["relatedFeature"].elements] == ["EP::Pair::u1", "EP::Pair::u2"]
        assert str(connector["sourceFeature"].value) == "EP::Pair::u1"
        end = connector["connectorEnd"].elements[0]
        again = self.model.element_properties(end.id, "ownedReferenceSubsetting")
        assert again.element.metaclass == "ReferenceUsage"
        assert again["ownedReferenceSubsetting"].value.metaclass == "ReferenceSubsetting"

    def test_a_bound_expression_is_walked_to_its_operator(self):
        value = self.model["EP::Pair::x"].property("value").value
        assert value.metaclass == "OperatorExpression"
        assert self.model.element_properties(value.id, "operator")["operator"].value == "+"

    def test_why_a_property_has_no_answer_is_said(self):
        read = self.model.element_properties("EP::Pair", "connectorEnd", "ownedSubclassification")
        assert "declares no property connectorEnd" in read["connectorEnd"].unsupported
        assert read["ownedSubclassification"].supported and len(read["ownedSubclassification"].values) == 1
        with pytest.raises(SymbolNotFoundError):
            self.model.element_properties("EP::Nowhere")

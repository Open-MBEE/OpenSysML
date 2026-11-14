"""Tests for Symbol class."""

import pytest
from unittest.mock import Mock
from opensysml.capabilities import (
    CAPABILITY_SYMBOL_ATTRIBUTES,
    MissingCapabilityError,
    ServerInfo,
)
from opensysml.errors import SymbolNotFoundError, UnresolvedChildError
from opensysml.symbol import Symbol
from opensysml.proto import sysml_pb2
from tests.lookups import answering


class TestSymbolBasicProperties:
    """Test basic Symbol properties: id, name, kind."""

    def test_symbol_properties(self):
        """Test that Symbol exposes basic properties from SymbolInfo."""
        # Create mock client
        mock_client = Mock()
        
        # Create SymbolInfo protobuf with metadata
        info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle",
            name="Vehicle",
            kind="partDef",
            metadata={"source": "test.sysml", "line": "5"}
        )
        
        # Create Symbol
        sym = Symbol(info, mock_client, "model_abc123")
        
        # Verify properties
        assert sym.id == "Vehicles::Vehicle"
        assert sym.name == "Vehicle"
        assert sym.kind == "partDef"
        assert sym.metadata == {"source": "test.sysml", "line": "5"}
        assert isinstance(sym.metadata, dict)
    
    def test_symbol_without_client(self):
        """Test that Symbol works without client (for Model root symbol)."""
        # Create SymbolInfo protobuf
        info = sysml_pb2.SymbolInfo(
            id="Model",
            name="Model",
            kind="package"
        )
        
        # Create Symbol without client
        sym = Symbol(info, None, "model_abc123")
        
        # Verify properties still work
        assert sym.id == "Model"
        assert sym.name == "Model"
        assert sym.kind == "package"
    
    def test_symbol_str(self):
        """Test __str__ returns 'name (kind)' format."""
        info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle",
            name="Vehicle",
            kind="partDef"
        )
        sym = Symbol(info, None, "model_abc123")
        
        # Verify __str__ format
        assert str(sym) == "Vehicle (partDef)"


class TestSymbolChildren:
    """Test Symbol.children() method."""

    def test_symbol_children_lazy_loading(self):
        """Test children() returns Symbol objects for child_ids."""
        # Create mock client
        mock_client = Mock()
        
        # Mock get_symbol to return child SymbolInfo objects
        child1_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle::mass",
            name="mass",
            kind="attributeUsage"
        )
        child2_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle::speed",
            name="speed",
            kind="attributeUsage"
        )
        mock_client.get_symbol.side_effect = [child1_info, child2_info]
        
        # Create parent SymbolInfo with child_ids
        parent_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle",
            name="Vehicle",
            kind="partDef",
            child_ids=["Vehicles::Vehicle::mass", "Vehicles::Vehicle::speed"]
        )
        
        # Create parent Symbol
        parent = Symbol(parent_info, mock_client, "model_abc123")
        
        # Get children
        children = parent.children()
        
        # Verify
        assert len(children) == 2
        assert children[0].id == "Vehicles::Vehicle::mass"
        assert children[0].name == "mass"
        assert children[1].id == "Vehicles::Vehicle::speed"
        assert children[1].name == "speed"
        
        # Verify RPC calls with model_hash
        assert mock_client.get_symbol.call_count == 2
        mock_client.get_symbol.assert_any_call("model_abc123", id="Vehicles::Vehicle::mass")
        mock_client.get_symbol.assert_any_call("model_abc123", id="Vehicles::Vehicle::speed")
    
    def test_symbol_children_caching(self):
        """Test children() caches results and doesn't hit client twice."""
        # Create mock client
        mock_client = Mock()
        
        # Mock get_symbol to return child
        child_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle::mass",
            name="mass",
            kind="attributeUsage"
        )
        mock_client.get_symbol.return_value = child_info
        
        # Create parent SymbolInfo with one child
        parent_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle",
            name="Vehicle",
            kind="partDef",
            child_ids=["Vehicles::Vehicle::mass"]
        )
        
        # Create parent Symbol
        parent = Symbol(parent_info, mock_client, "model_abc123")
        
        # First call - should fetch from client
        children1 = parent.children()
        assert len(children1) == 1
        assert mock_client.get_symbol.call_count == 1
        
        # Second call - should return cached, no additional RPC
        children2 = parent.children()
        assert len(children2) == 1
        assert mock_client.get_symbol.call_count == 1  # Still 1, not 2
        assert children1 is children2  # Same list object
    
    def test_symbol_children_empty(self):
        """Test children() with empty child_ids returns empty list."""
        # Create mock client
        mock_client = Mock()
        
        # Create SymbolInfo with no child_ids
        info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle",
            name="Vehicle",
            kind="partDef",
            child_ids=[]
        )
        
        # Create Symbol
        sym = Symbol(info, mock_client, "model_abc123")
        
        # Get children - should return empty list
        children = sym.children()
        
        assert children == []
        assert mock_client.get_symbol.call_count == 0  # No RPC calls
    
    def test_children_without_client(self):
        """Test children() returns empty list when no client."""
        # Create SymbolInfo with child_ids but no client
        info = sysml_pb2.SymbolInfo(
            id="Model",
            name="Model",
            kind="package",
            child_ids=["Model::Vehicles", "Model::Parts"]
        )
        
        # Create Symbol without client
        sym = Symbol(info, None, "model_abc123")
        
        # Get children - should return empty list
        children = sym.children()
        
        assert children == []
    
    def test_children_raises_on_a_child_the_service_does_not_resolve(self):
        """children() reports a listed child GetSymbol cannot resolve instead of dropping it."""
        mock_client = Mock()

        child1_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle::mass",
            name="mass",
            kind="attributeUsage"
        )
        # Second call returns None (symbol not found or RPC error)
        mock_client.get_symbol.side_effect = [child1_info, None]

        parent_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle",
            name="Vehicle",
            kind="partDef",
            child_ids=["Vehicles::Vehicle::mass", "Vehicles::Vehicle::invalid"]
        )
        parent = Symbol(parent_info, mock_client, "model_abc123")

        with pytest.raises(UnresolvedChildError) as excinfo:
            parent.children()
        assert excinfo.value.owner == "Vehicles::Vehicle"
        assert excinfo.value.child_ids == ["Vehicles::Vehicle::invalid"]
        assert isinstance(excinfo.value, SymbolNotFoundError)
        assert "Vehicles::Vehicle::invalid" in str(excinfo.value)
        # Nothing is cached, so a later call asks the service again.
        mock_client.get_symbol.side_effect = [child1_info, None]
        with pytest.raises(UnresolvedChildError):
            parent.children()


class TestSymbolAttributes:
    """Test Symbol.attributes(), parts() and get_attr() against real service kinds."""

    def _vehicle(self, mock_client):
        """Build a Vehicle symbol whose children use the service's kind strings."""
        children = [
            sysml_pb2.SymbolInfo(id="Vehicles::Vehicle::mass", name="mass", kind="attributeUsage"),
            sysml_pb2.SymbolInfo(id="Vehicles::Vehicle::engine", name="engine", kind="partUsage"),
            sysml_pb2.SymbolInfo(id="Vehicles::Vehicle::Wheel", name="Wheel", kind="partDef"),
            sysml_pb2.SymbolInfo(id="Vehicles::Vehicle::Color", name="Color", kind="attributeDef"),
            sysml_pb2.SymbolInfo(id="Vehicles::Vehicle::cargo", name="cargo", kind="itemUsage"),
        ]
        mock_client.get_symbol.side_effect = children
        parent_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle",
            name="Vehicle",
            kind="partDef",
            child_ids=[child.id for child in children],
            members=[
                sysml_pb2.MemberInfo(id=child.id, name=child.name, kind=child.kind)
                for child in children
            ],
        )
        return Symbol(parent_info, mock_client, "model_abc123")

    def test_attributes_matches_service_kinds(self):
        """attributes() selects attributeUsage/attributeDef children."""
        parent = self._vehicle(Mock())

        attributes = parent.attributes()

        assert [attr.name for attr in attributes] == ["mass", "Color"]

    def test_parts_matches_service_kinds(self):
        """parts() selects partUsage/partDef children."""
        parent = self._vehicle(Mock())

        parts = parent.parts()

        assert [part.name for part in parts] == ["engine", "Wheel"]

    def test_get_attr_finds_attribute(self):
        """get_attr() returns the named attribute, None otherwise."""
        parent = self._vehicle(Mock())

        assert parent.get_attr("mass").kind == "attributeUsage"
        assert parent.get_attr("engine") is None
        assert parent.get_attr("missing") is None

    def test_kind_matching_is_case_insensitive(self):
        """Kinds are matched case-insensitively, so PascalCase producers work."""
        mock_client = Mock()
        mock_client.get_symbol.side_effect = [
            sysml_pb2.SymbolInfo(id="P::mass", name="mass", kind="AttributeUsage"),
            sysml_pb2.SymbolInfo(id="P::engine", name="engine", kind="PartUsage"),
        ]
        parent = Symbol(
            sysml_pb2.SymbolInfo(
                id="P", name="P", kind="PartDef", child_ids=["P::mass", "P::engine"],
                members=[
                    sysml_pb2.MemberInfo(id="P::mass", name="mass", kind="AttributeUsage"),
                    sysml_pb2.MemberInfo(id="P::engine", name="engine", kind="PartUsage"),
                ],
            ),
            mock_client,
            "model_abc123",
        )

        assert [attr.name for attr in parent.attributes()] == ["mass"]
        assert [part.name for part in parent.parts()] == ["engine"]


class TestSymbolAttributeFacts:
    """Test the attribute facts the service resolves, which must never be empty
    for an element that has attributes."""

    def _car(self, mock_client):
        """Wrap the reported Car as a Symbol."""
        return Symbol(self._car_info(mock_client), mock_client, "model_abc123")

    def _car_info(self, mock_client):
        """Build a Car as the service reports it: one own attribute, one
        inherited from Base, each with resolved type and default value."""
        base_info = sysml_pb2.SymbolInfo(
            id="Demo::Base",
            name="Base",
            kind="partDef",
            child_ids=["Demo::Base::label"],
            attributes=[
                sysml_pb2.AttributeInfo(
                    name="label",
                    type="ScalarValues::String",
                    value=sysml_pb2.Value(string_value="base"),
                ),
            ],
        )
        symbols = {
            "Demo::Car::mass": sysml_pb2.SymbolInfo(
                id="Demo::Car::mass",
                name="mass",
                kind="attributeUsage",
                type_info=sysml_pb2.TypeInfo(
                    declared="ISQ::MassValue",
                    resolved_id="ISQBase::MassValue",
                    quantity=True,
                    unit="SI::kg",
                ),
                multiplicity=sysml_pb2.MultiplicityInfo(lower="1", upper="1"),
            ),
            "Demo::Car::engine": sysml_pb2.SymbolInfo(
                id="Demo::Car::engine", name="engine", kind="partUsage"
            ),
            "Demo::Base": base_info,
            "Demo::Base::label": sysml_pb2.SymbolInfo(
                id="Demo::Base::label",
                name="label",
                kind="attributeUsage",
                type_info=sysml_pb2.TypeInfo(resolved_id="ScalarValues::String"),
            ),
        }
        mock_client.get_symbol.side_effect = answering(lambda _hash, symbol_id: symbols.get(symbol_id))

        car_members = [
            sysml_pb2.MemberInfo(id="Demo::Car::mass", name="mass", kind="attributeUsage"),
            sysml_pb2.MemberInfo(id="Demo::Car::engine", name="engine", kind="partUsage"),
            sysml_pb2.MemberInfo(
                id="Demo::Base::label", name="label", kind="attributeUsage",
                inherited=True, inherited_from="Demo::Base",
            ),
        ]
        car_info = sysml_pb2.SymbolInfo(
            id="Demo::Car",
            name="Car",
            kind="partDef",
            child_ids=["Demo::Car::mass", "Demo::Car::engine"],
            members=car_members,
            withheld_library_members=2,
            specializations=[
                sysml_pb2.Specialization(
                    kind="specializes",
                    declared="Base",
                    target_id="Demo::Base",
                    target_kind="partDef",
                ),
            ],
            attributes=[
                sysml_pb2.AttributeInfo(
                    name="mass",
                    type="ISQBase::MassValue",
                    value=sysml_pb2.Value(real_value=1600.0),
                    unit="SI::kg",
                ),
                sysml_pb2.AttributeInfo(
                    name="label",
                    type="ScalarValues::String",
                    value=sysml_pb2.Value(string_value="base"),
                ),
                sysml_pb2.AttributeInfo(name="derived", type="ScalarValues::Real"),
            ],
        )
        return car_info

    def test_attribute_facts_reports_type_value_and_unit(self):
        """attribute_facts() carries the facts the service resolved, not an empty list."""
        facts = self._car(Mock()).attribute_facts()

        assert [fact.name for fact in facts] == ["mass", "label", "derived"]
        assert facts[0].type == "ISQBase::MassValue"
        assert facts[0].value == 1600.0
        assert facts[0].unit == "SI::kg"
        assert facts[1].value == "base"
        # An attribute whose default is not a model-level constant has no value.
        assert facts[2].value is None
        assert facts[2].unit == ""

    def test_facts_include_attributes(self):
        """facts() carries the attribute set, so a generated view can use it."""
        facts = self._car(Mock()).facts()

        assert [attr.name for attr in facts.attributes] == ["mass", "label", "derived"]

    def test_attributes_include_inherited_ones(self):
        """attributes() reports an inherited attribute, fetched from its declaring type."""
        car = self._car(Mock())

        attributes = car.attributes()

        assert [attr.name for attr in attributes] == ["mass", "label"]
        assert attributes[1].id == "Demo::Base::label"
        assert car.get_attr("label").id == "Demo::Base::label"

    def test_attributes_walk_the_specializations_on_an_older_service(self):
        """A service without symbol_members still answers inherited attributes:
        the client follows the declared specializations itself, as it did
        before the service enumerated members."""
        mock_client = Mock()
        mock_client.server_info.return_value = ServerInfo(
            version="v0.9.0", capabilities=frozenset({"symbol_attributes"}),
            answered=True, origin="sysml-grpc",
        )
        car_info = self._car_info(mock_client)
        del car_info.members[:]
        car_info.withheld_library_members = 0
        car_info.withheld_library_attributes = 3
        car = Symbol(car_info, mock_client, "model_abc123")

        attributes = car.attributes()

        assert [attr.name for attr in attributes] == ["mass", "label"]
        assert attributes.withheld_library == 3
        with pytest.raises(MissingCapabilityError):
            car.attributes(library=True)
        assert not attributes[0].inherited
        assert attributes[1].id == "Demo::Base::label"
        assert attributes[1].inherited_from == "Demo::Base"
        assert car.get_attr("label").id == "Demo::Base::label"
        assert car.attributes(inherited=False) == [attributes[0]]
        with pytest.raises(MissingCapabilityError):
            car.children(inherited=True)

    def test_attributes_of_a_typed_usage_come_from_its_type(self):
        """A usage written as `part car : Car` has the attributes Car declares."""
        mock_client = Mock()
        car_info = self._car_info(mock_client)
        usage_info = sysml_pb2.SymbolInfo(
            id="Demo::car",
            name="car",
            kind="partUsage",
            specializations=[
                sysml_pb2.Specialization(
                    kind="typing",
                    declared="Car",
                    target_id="Demo::Car",
                    target_kind="partDef",
                ),
            ],
            attributes=list(car_info.attributes),
            members=[
                sysml_pb2.MemberInfo(
                    id=m.id, name=m.name, kind=m.kind, inherited=True,
                    inherited_from=m.inherited_from or "Demo::Car",
                )
                for m in car_info.members
            ],
        )
        symbols = mock_client.get_symbol.side_effect
        mock_client.get_symbol.side_effect = answering(
            lambda _hash, symbol_id: car_info if symbol_id == "Demo::Car"
            else symbols(_hash, symbol_id)
        )

        usage = Symbol(usage_info, mock_client, "model_abc123")

        assert [attr.name for attr in usage.attributes()] == ["mass", "label"]
        assert usage.get_attr("mass").id == "Demo::Car::mass"

    def test_to_dataframe_does_not_lend_a_value_to_a_same_named_member(self):
        """A member of another kind sharing an attribute's name carries no value."""
        pd = pytest.importorskip("pandas")
        mock_client = Mock()
        base_info = sysml_pb2.SymbolInfo(
            id="Demo::Base",
            name="Base",
            kind="partDef",
            child_ids=["Demo::Base::mass"],
            attributes=[
                sysml_pb2.AttributeInfo(
                    name="mass",
                    type="ScalarValues::Real",
                    value=sysml_pb2.Value(real_value=1000.0),
                ),
            ],
        )
        symbols = {
            "Demo::Base": base_info,
            "Demo::Base::mass": sysml_pb2.SymbolInfo(
                id="Demo::Base::mass", name="mass", kind="attributeUsage"
            ),
            # A part usage of the same simple name as the inherited attribute.
            "Demo::Car::mass": sysml_pb2.SymbolInfo(
                id="Demo::Car::mass", name="mass", kind="partUsage"
            ),
        }
        mock_client.get_symbol.side_effect = answering(lambda _hash, symbol_id: symbols.get(symbol_id))
        car = Symbol(
            sysml_pb2.SymbolInfo(
                id="Demo::Car",
                name="Car",
                kind="partDef",
                child_ids=["Demo::Car::mass"],
                specializations=[
                    sysml_pb2.Specialization(
                        kind="specializes", declared="Base", target_id="Demo::Base"
                    ),
                ],
                attributes=list(base_info.attributes),
                members=[
                    sysml_pb2.MemberInfo(id="Demo::Car::mass", name="mass", kind="partUsage"),
                    sysml_pb2.MemberInfo(
                        id="Demo::Base::mass", name="mass", kind="attributeUsage",
                        inherited=True, inherited_from="Demo::Base",
                    ),
                ],
            ),
            mock_client,
            "model_abc123",
        )

        frame = car.to_dataframe()

        part = frame[frame["id"] == "Demo::Car::mass"].iloc[0]
        assert pd.isna(part["value"])
        assert pd.isna(part["unit"])
        attribute = frame[frame["id"] == "Demo::Base::mass"].iloc[0]
        assert attribute["value"] == 1000.0

    def test_to_dataframe_reports_attribute_facts(self):
        """to_dataframe() reports each member's type, multiplicity, value and unit."""
        pd = pytest.importorskip("pandas")
        frame = self._car(Mock()).to_dataframe()

        assert list(frame.columns) == [
            "name", "kind", "id", "type", "multiplicity", "value", "unit", "inherited",
        ]
        assert list(frame["name"]) == ["mass", "engine", "label"]

        mass = frame[frame["name"] == "mass"].iloc[0]
        assert mass["type"] == "ISQBase::MassValue"
        assert mass["multiplicity"] == "1..1"
        assert mass["value"] == 1600.0
        assert mass["unit"] == "SI::kg"
        assert not mass["inherited"]

        engine = frame[frame["name"] == "engine"].iloc[0]
        assert pd.isna(engine["value"])
        assert pd.isna(engine["unit"])

        label = frame[frame["name"] == "label"].iloc[0]
        assert label["value"] == "base"
        assert label["inherited"]

    def test_attribute_facts_name_a_service_that_cannot_resolve_them(self):
        """An older service's empty set is reported as a missing capability, not as none."""
        mock_client = Mock()
        mock_client.server_info.return_value = ServerInfo(
            version="v0.0.1", capabilities=frozenset({"type_facts"}),
            answered=True, origin="an old sysml-grpc",
        )
        car = self._car(mock_client)

        with pytest.raises(MissingCapabilityError) as excinfo:
            car.attribute_facts()
        assert excinfo.value.capability == CAPABILITY_SYMBOL_ATTRIBUTES

        with pytest.raises(MissingCapabilityError):
            car.to_dataframe()

        # Navigation that does not depend on the resolved set keeps working.
        assert [child.name for child in car.children()] == ["mass", "engine"]

    def test_to_dataframe_of_a_symbol_with_no_members(self):
        """An element with no members still has the full column set."""
        pytest.importorskip("pandas")
        empty = Symbol(sysml_pb2.SymbolInfo(id="Demo::Engine", name="Engine", kind="partDef"), None, "h")

        frame = empty.to_dataframe()

        assert list(frame.columns) == [
            "name", "kind", "id", "type", "multiplicity", "value", "unit", "inherited",
        ]
        assert frame.empty


class TestSendActionSymbol:
    """A send action's symbol reports its kind and arguments."""

    def test_send_action_kind_and_arguments(self):
        info = sysml_pb2.SymbolInfo(
            id="Monitor::supervise::notifyVia",
            name="notifyVia",
            kind="sendActionUsage",
            metadata={"payload": "new Alarm(3)", "sender": "alarmPort", "visibility": "public"},
        )
        sym = Symbol(info, None, "model_abc123")
        assert sym.kind == "sendActionUsage"
        assert sym.metadata["payload"] == "new Alarm(3)"
        assert sym.metadata["sender"] == "alarmPort"
        assert "receiver" not in sym.metadata

    def test_send_action_to_only_has_a_receiver_and_no_sender(self):
        info = sysml_pb2.SymbolInfo(
            id="Monitor::supervise::notifyTo",
            name="notifyTo",
            kind="sendActionUsage",
            metadata={"payload": "new Alarm(5)", "receiver": "display"},
        )
        sym = Symbol(info, None, "model_abc123")
        assert sym.metadata["receiver"] == "display"
        assert "sender" not in sym.metadata


class TestSymbolMembers:
    """Test the member methods against the service's member enumeration:
    own members by default, inherited ones on request, standard-library
    members withheld and counted."""

    def _controller(self, mock_client, capabilities=frozenset({"symbol_members"})):
        """Build `part controller : Controller` as the service reports it: no
        own member, four inherited from Controller (one of them a binding),
        the standard-library members withheld."""
        mock_client.server_info.return_value = ServerInfo(
            version="v1.0.0", capabilities=capabilities, answered=True, origin="sysml-grpc",
        )
        controller_members = [
            sysml_pb2.MemberInfo(id="Demo::Controller::gain", name="gain", kind="attributeUsage"),
            sysml_pb2.MemberInfo(id="Demo::Controller::cmdIn", name="cmdIn", kind="portUsage"),
            sysml_pb2.MemberInfo(id="Demo::Controller::logic", name="logic", kind="partUsage"),
            sysml_pb2.MemberInfo(id="Demo::Controller::delegate", name="delegate", kind="bindingConnectorAsUsage"),
        ]
        inherited = [
            sysml_pb2.MemberInfo(
                id=m.id, name=m.name, kind=m.kind, inherited=True, inherited_from="Demo::Controller",
            )
            for m in controller_members
        ]
        library = [
            sysml_pb2.MemberInfo(
                id="Parts::Part::subparts", name="subparts", kind="partUsage",
                inherited=True, inherited_from="Parts::Part", library=True,
            ),
        ]
        symbols = {
            "Demo::Controller": sysml_pb2.SymbolInfo(
                id="Demo::Controller", name="Controller", kind="partDef",
                child_ids=[m.id for m in controller_members],
                members=controller_members, withheld_library_members=1,
            ),
            "Demo::system::controller": sysml_pb2.SymbolInfo(
                id="Demo::system::controller", name="controller", kind="partUsage",
                members=inherited, withheld_library_members=1,
            ),
            "Parts::Part::subparts": sysml_pb2.SymbolInfo(
                id="Parts::Part::subparts", name="subparts", kind="partUsage",
            ),
        }
        for m in controller_members:
            symbols[m.id] = sysml_pb2.SymbolInfo(id=m.id, name=m.name, kind=m.kind)
        with_library = sysml_pb2.SymbolInfo(
            id="Demo::system::controller", name="controller", kind="partUsage",
            members=inherited + library,
        )

        def get_symbol(_hash, symbol_id, include_library_members=False):
            if include_library_members and symbol_id == "Demo::system::controller":
                return with_library
            return symbols.get(symbol_id)

        mock_client.get_symbol.side_effect = answering(get_symbol)
        return Symbol(symbols["Demo::system::controller"], mock_client, "model_abc123")

    def test_children_are_own_members_by_default(self):
        """A usage declaring nothing has no children, however much it inherits."""
        controller = self._controller(Mock())

        assert controller.children() == []
        assert controller.parts() == []
        assert controller.withheld_library_members == 1

    def test_inherited_members_are_listed_on_request(self):
        """children(inherited=True) lists what the type declares, each naming it."""
        controller = self._controller(Mock())

        members = controller.children(inherited=True)

        assert [m.id for m in members] == [
            "Demo::Controller::gain", "Demo::Controller::cmdIn",
            "Demo::Controller::logic", "Demo::Controller::delegate",
        ]
        assert all(m.inherited and m.inherited_from == "Demo::Controller" for m in members)
        assert members.withheld_library == 1
        assert controller.children(inherited=True) is members

    def test_kind_wrappers_filter_the_same_enumeration(self):
        """parts/ports/bindings/connections/attributes agree on one member list."""
        controller = self._controller(Mock())

        assert [p.id for p in controller.parts(inherited=True)] == ["Demo::Controller::logic"]
        assert [p.name for p in controller.ports(inherited=True)] == ["cmdIn"]
        assert [b.name for b in controller.bindings(inherited=True)] == ["delegate"]
        assert [c.name for c in controller.connections(inherited=True)] == ["delegate"]
        assert [a.name for a in controller.attributes()] == ["gain"]
        assert controller.attributes(inherited=False) == []
        assert [m.name for m in controller.members(kind="partUsage", inherited=True)] == ["logic"]

    def test_library_members_are_fetched_on_request(self):
        """library=True asks the service for the withheld members, marked by origin."""
        controller = self._controller(Mock())

        members = controller.children(inherited=True, library=True)

        assert [m.id for m in members][-1] == "Parts::Part::subparts"
        assert members[-1].inherited_from == "Parts::Part"
        assert members.withheld_library == 0
        assert [p.name for p in controller.parts(inherited=True, library=True)] == ["logic", "subparts"]

    def test_owned_members_count_the_withheld_library_ones(self):
        """The owned list reports the withheld count the service attached."""
        controller = self._controller(Mock())

        assert controller.children().withheld_library == 1
        assert controller.parts().withheld_library == 1

    def test_library_alone_keeps_the_owned_members(self):
        """library=True without inherited=True changes nothing: an owned member
        is never withheld, so no inherited member slips in and nothing is refetched."""
        controller = self._controller(Mock())

        assert controller.children(library=True) == []
        assert controller.parts(library=True) == []
        assert controller.children(library=True).withheld_library == 1
        for call in controller._client.get_symbol.call_args_list:
            assert not call.kwargs.get("include_library_members")

    def test_inherited_members_need_the_capability(self):
        """A service without symbol_members cannot enumerate inherited members."""
        controller = self._controller(Mock(), capabilities=frozenset({"symbol_attributes"}))

        with pytest.raises(MissingCapabilityError) as excinfo:
            controller.children(inherited=True)

        assert "symbol_members" in str(excinfo.value)
        assert controller.children() == []

    def test_own_members_fall_back_to_child_ids(self):
        """A symbol an older service reported still lists its owned members."""
        mock_client = Mock()
        mock_client.get_symbol.side_effect = answering(lambda _hash, symbol_id, include_library_members=False: (
            sysml_pb2.SymbolInfo(id=symbol_id, name=symbol_id.rsplit("::", 1)[-1], kind="partUsage")
        ))
        parent = Symbol(
            sysml_pb2.SymbolInfo(id="P", name="P", kind="partDef", child_ids=["P::a", "P::b"]),
            mock_client, "model_abc123",
        )

        assert [m.name for m in parent.children()] == ["a", "b"]
        assert not parent.children()[0].inherited
        assert parent.children().withheld_library == 0

    def test_unresolvable_member_ids_raise(self):
        """A member id the service lists but cannot resolve is reported, not dropped."""
        controller = self._controller(Mock())
        symbols = controller._client.get_symbol.side_effect
        controller._client.get_symbol.side_effect = answering(
            lambda _hash, symbol_id, **kw: None if symbol_id.endswith("delegate") else symbols(_hash, symbol_id, **kw)
        )

        with pytest.raises(UnresolvedChildError) as excinfo:
            controller.children(inherited=True)
        assert excinfo.value.owner == controller.id
        assert [i.rsplit("::", 1)[-1] for i in excinfo.value.child_ids] == ["delegate"]


class TestSymbolTypeFacts:
    """The written typing and the derived types are reported side by side."""

    def _symbol(self, info):
        return Symbol(info, Mock(), "model_abc123")

    def test_derived_types_are_listed_beside_the_written_typing(self):
        info = sysml_pb2.SymbolInfo(
            id="P::Owner::pBoth",
            name="pBoth",
            kind="partUsage",
            type_info=sysml_pb2.TypeInfo(
                declared="V",
                resolved_id="P::V",
                resolved_kind="partDef",
                types=[
                    sysml_pb2.TypeRef(id="P::V", kind="partDef"),
                    sysml_pb2.TypeRef(id="P::T", kind="partDef"),
                ],
            ),
        )
        facts = self._symbol(info).type_facts
        assert facts.declared == "V"
        assert facts.resolved_id == "P::V"
        assert [(ref.id, ref.kind) for ref in facts.types] == [
            ("P::V", "partDef"),
            ("P::T", "partDef"),
        ]

    def test_a_feature_typed_only_through_subsetting_has_derived_types(self):
        info = sysml_pb2.SymbolInfo(
            id="P::Owner::pRef",
            name="pRef",
            kind="partUsage",
            type_info=sysml_pb2.TypeInfo(types=[sysml_pb2.TypeRef(id="P::T", kind="partDef")]),
            specializations=[
                sysml_pb2.Specialization(
                    kind="references", declared="q", target_id="P::Owner::q", target_kind="partUsage"
                )
            ],
        )
        symbol = self._symbol(info)
        assert symbol.type_facts.declared == ""
        assert symbol.type_facts.resolved_id == ""
        assert [ref.id for ref in symbol.type_facts.types] == ["P::T"]
        assert [(spec.kind, spec.target_id) for spec in symbol.specializations] == [
            ("references", "P::Owner::q")
        ]

    def test_an_older_service_reports_no_derived_types(self):
        info = sysml_pb2.SymbolInfo(
            id="P::Owner::q", name="q", kind="partUsage",
            type_info=sysml_pb2.TypeInfo(declared="T", resolved_id="P::T"),
        )
        assert self._symbol(info).type_facts.types == ()


@pytest.mark.parametrize("kind", ["bindingConnectorAsUsage", "bindingUsage"])
def test_bindings_select_the_kind_of_either_service_generation(kind):
    """bindings() reads the metaclass kind and the kind an older service reports."""
    member = sysml_pb2.MemberInfo(id="Demo::C::@0", kind=kind)
    owner = sysml_pb2.SymbolInfo(id="Demo::C", name="C", kind="partDef", child_ids=[member.id], members=[member])
    client = Mock()
    client.server_info.return_value = ServerInfo(
        version="v1.0.0", capabilities=frozenset({"symbol_members"}), answered=True, origin="sysml-grpc",
    )
    client.get_symbol.side_effect = answering(lambda _hash, symbol_id, include_library_members=False: (
        sysml_pb2.SymbolInfo(id=member.id, kind=kind) if symbol_id == member.id else owner))
    symbol = Symbol(owner, client, "model_abc123")
    assert [b.id for b in symbol.bindings()] == ["Demo::C::@0"]
    assert [c.id for c in symbol.connections()] == ["Demo::C::@0"]

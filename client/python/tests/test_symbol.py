"""Tests for Symbol class."""

import pytest
from unittest.mock import Mock
from opensysml.capabilities import (
    CAPABILITY_SYMBOL_ATTRIBUTES,
    MissingCapabilityError,
    ServerInfo,
)
from opensysml.symbol import Symbol
from opensysml.proto import sysml_pb2


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
        mock_client.get_symbol.assert_any_call("model_abc123", "Vehicles::Vehicle::mass")
        mock_client.get_symbol.assert_any_call("model_abc123", "Vehicles::Vehicle::speed")
    
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
    
    def test_children_skips_none_results(self):
        """Test children() skips None when get_symbol fails."""
        # Create mock client that returns None for second child
        mock_client = Mock()
        
        child1_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle::mass",
            name="mass",
            kind="attributeUsage"
        )
        # Second call returns None (symbol not found or RPC error)
        mock_client.get_symbol.side_effect = [child1_info, None]
        
        # Create parent SymbolInfo
        parent_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle",
            name="Vehicle",
            kind="partDef",
            child_ids=["Vehicles::Vehicle::mass", "Vehicles::Vehicle::invalid"]
        )
        
        # Create parent Symbol
        parent = Symbol(parent_info, mock_client, "model_abc123")
        
        # Get children - should skip None
        children = parent.children()
        
        # Verify only valid child returned
        assert len(children) == 1
        assert children[0].id == "Vehicles::Vehicle::mass"


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
            sysml_pb2.SymbolInfo(id="P", name="P", kind="PartDef", child_ids=["P::mass", "P::engine"]),
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
        mock_client.get_symbol.side_effect = lambda _hash, symbol_id: symbols.get(symbol_id)

        car_info = sysml_pb2.SymbolInfo(
            id="Demo::Car",
            name="Car",
            kind="partDef",
            child_ids=["Demo::Car::mass", "Demo::Car::engine"],
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
        )
        symbols = mock_client.get_symbol.side_effect
        mock_client.get_symbol.side_effect = (
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
        mock_client.get_symbol.side_effect = lambda _hash, symbol_id: symbols.get(symbol_id)
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

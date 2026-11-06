"""Tests for the value expression a Symbol declares (Symbol.valuation)."""
import pytest
from opensysml.capabilities import CAPABILITY_VALUATION, MissingCapabilityError, ServerInfo

from opensysml.proto import sysml_pb2
from opensysml.symbol import Symbol
from opensysml.typefacts import AttributeFacts, Valuation, ValuationReference


def chain_valuation():
    return sysml_pb2.Valuation(
        text="powerPort.power",
        kind="featureChain",
        metaclass="FeatureChainExpression",
        references=[sysml_pb2.ValuationReference(
            text="powerPort.power",
            path=["ParameterValues::Motor::powerPort", "ParameterValues::PowerPort::power"],
            target_id="ParameterValues::PowerPort::power",
        )],
    )


def test_symbol_valuation_reads_text_kind_and_references():
    pb = sysml_pb2.SymbolInfo(
        id="ParameterValues::Motor::consume::power", name="power", kind="referenceUsage",
        valuation=chain_valuation(),
    )
    valuation = Symbol(pb, None, "hash").valuation
    assert valuation == Valuation(
        text="powerPort.power",
        kind="featureChain",
        metaclass="FeatureChainExpression",
        references=(ValuationReference(
            text="powerPort.power",
            path=("ParameterValues::Motor::powerPort", "ParameterValues::PowerPort::power"),
            target_id="ParameterValues::PowerPort::power",
        ),),
    )
    assert valuation.references[0].path[-1] == valuation.references[0].target_id
    assert not valuation.is_default and not valuation.is_initial


def test_symbol_without_a_value_has_no_valuation():
    pb = sysml_pb2.SymbolInfo(id="P::x", name="x", kind="attributeUsage")
    symbol = Symbol(pb, None, "hash")
    assert symbol.valuation is None
    assert symbol.facts().valuation is None


def test_default_and_initial_flags_cross():
    pb = sysml_pb2.SymbolInfo(
        id="P::limit", name="limit", kind="attributeUsage",
        valuation=sysml_pb2.Valuation(text="3.0", kind="literal", metaclass="LiteralRational", is_default=True),
    )
    valuation = Symbol(pb, None, "hash").valuation
    assert valuation.is_default and not valuation.is_initial
    assert valuation.references == ()


def test_attribute_facts_carry_the_valuation_beside_the_constant():
    attr = sysml_pb2.AttributeInfo(
        name="power", type="ScalarValues::Real",
        valuation=sysml_pb2.Valuation(
            text="powerPort.power * 2.0", kind="operator", metaclass="OperatorExpression",
            references=[sysml_pb2.ValuationReference(
                text="powerPort.power",
                path=["M::powerPort", "PowerPort::power"],
                target_id="PowerPort::power",
            )],
        ),
    )
    facts = AttributeFacts.from_pb(attr)
    assert facts.value is None
    assert facts.valuation.kind == "operator"
    assert facts.valuation.references[0].target_id == "PowerPort::power"
    constant = AttributeFacts.from_pb(sysml_pb2.AttributeInfo(
        name="gain", type="ScalarValues::Real",
        value=sysml_pb2.Value(real_value=0.5),
        valuation=sysml_pb2.Valuation(text="0.5", kind="literal", metaclass="LiteralRational"),
    ))
    assert constant.value == 0.5
    assert constant.valuation.text == "0.5"


def _service(*capabilities):
    class Service:
        def server_info(self):
            return ServerInfo(version="1", capabilities=frozenset(capabilities), answered=True, origin="test")

    return Service()


_OldService = lambda: _service("type_facts", "symbol_attributes")  # noqa: E731
_NewService = lambda: _service("type_facts", "symbol_attributes", CAPABILITY_VALUATION)  # noqa: E731


def test_valuation_names_a_service_that_predates_it():
    pb = sysml_pb2.SymbolInfo(id="P::x", name="x", kind="attributeUsage")
    with pytest.raises(MissingCapabilityError):
        Symbol(pb, _OldService(), "hash").valuation
    assert Symbol(pb, _NewService(), "hash").valuation is None


def test_valuation_carries_the_evaluated_value_beside_a_missing_constant():
    pb = sysml_pb2.AttributeInfo(
        name="prop_b", type="ScalarValues::Real",
        valuation=sysml_pb2.Valuation(
            text="usage_a.prop_a", kind="featureChain", metaclass="FeatureChainExpression",
            evaluated=sysml_pb2.Value(real_value=100.0),
        ),
    )
    facts = AttributeFacts.from_pb(pb)
    assert facts.value is None
    assert facts.valuation.evaluated == 100.0


def test_valuation_without_a_determined_value_evaluates_to_none():
    pb = sysml_pb2.Valuation(text="usage_d.prop_b", kind="featureChain", metaclass="FeatureChainExpression")
    assert Valuation.from_pb(pb).evaluated is None

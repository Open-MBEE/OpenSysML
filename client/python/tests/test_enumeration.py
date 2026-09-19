"""Tests for an enumeration literal as a Python value.

A literal travels as the declaration it names, so it must arrive as an
:class:`EnumLiteral` — not as a string, which would be indistinguishable from a
string attribute, and not as an unsupported null.
"""

import pytest

from opensysml import EnumLiteral
from opensysml.connection import Connection
from opensysml.errors import FeatureValueError, UnsupportedValueError
from opensysml.proto import sysml_pb2
from opensysml.values import feature_value_to_python, value_to_python

RED = sysml_pb2.EnumLiteral(
    literal_id="D::Color::red", enumeration_id="D::Color", name="Color::red"
)
GREEN = sysml_pb2.EnumLiteral(
    literal_id="D::Color::green", enumeration_id="D::Color", name="Color::green"
)


def test_value_to_python_returns_the_literal():
    got = value_to_python(sysml_pb2.Value(enum_literal=RED))

    assert got == EnumLiteral("D::Color::red", "D::Color", "Color::red")
    assert str(got) == "Color::red"


def test_a_literal_is_not_a_string():
    """The literal and its rendering are different values."""
    assert value_to_python(sysml_pb2.Value(enum_literal=RED)) != "Color::red"


def test_literal_identity_is_the_declaration():
    same = value_to_python(sysml_pb2.Value(enum_literal=RED))
    other = value_to_python(sysml_pb2.Value(enum_literal=GREEN))

    assert same == value_to_python(sysml_pb2.Value(enum_literal=RED))
    assert same != other
    # Hashable, so a literal can key a dict and a repeated one collapses.
    assert len({same, other, value_to_python(sysml_pb2.Value(enum_literal=RED))}) == 2


def test_the_declaration_alone_identifies_a_literal():
    """The description is not part of the identity, so a bare id is the same value."""
    populated = value_to_python(sysml_pb2.Value(enum_literal=RED))
    bare = EnumLiteral(RED.literal_id)

    assert bare == populated
    assert hash(bare) == hash(populated)
    assert len({bare, populated}) == 1
    assert {bare: "R"}[populated] == "R"
    assert bare != EnumLiteral(GREEN.literal_id)


def test_a_sequence_of_literals_keeps_them():
    seq = sysml_pb2.Value(sequence=sysml_pb2.ValueSequence(elements=[
        sysml_pb2.Value(enum_literal=RED),
        sysml_pb2.Value(enum_literal=GREEN),
    ]))

    assert value_to_python(seq) == [
        EnumLiteral("D::Color::red", "D::Color", "Color::red"),
        EnumLiteral("D::Color::green", "D::Color", "Color::green"),
    ]


def test_an_enum_slot_is_no_longer_unsupported():
    slot = sysml_pb2.FeatureValue(
        feature_name="c", value=sysml_pb2.Value(enum_literal=RED), materialized=True
    )

    assert feature_value_to_python("c", slot) == EnumLiteral(
        "D::Color::red", "D::Color", "Color::red"
    )


def test_a_literal_is_sent_as_a_literal():
    """Round trip: what the client sends is what it reads back."""
    literal = EnumLiteral("D::Color::red", "D::Color", "Color::red")

    # _python_to_value uses no connection state, so no service is needed.
    pb_value = Connection._python_to_value(None, literal)

    assert pb_value.WhichOneof("kind") == "enum_literal"
    assert pb_value.enum_literal.literal_id == "D::Color::red"
    assert value_to_python(pb_value) == literal


HIGH = sysml_pb2.EnumLiteral(
    literal_id="D::Level::high", enumeration_id="D::Level", name="Level::high",
    value=sysml_pb2.Value(int_value=3),
)


def test_a_scalar_valued_literal_carries_its_scalar():
    """`high = 3` arrives as the literal, with the 3 it equals recoverable."""
    got = value_to_python(sysml_pb2.Value(enum_literal=HIGH))

    assert got == EnumLiteral("D::Level::high")
    assert got.value == 3
    assert got != 3
    assert value_to_python(sysml_pb2.Value(enum_literal=RED)).value is None


def test_a_scalar_valued_literal_round_trips_its_scalar():
    literal = EnumLiteral("D::Level::high", "D::Level", "Level::high", 3)

    # The scalar is encoded recursively, so a connection object without a service.
    pb_value = Connection.__new__(Connection)._python_to_value(literal)

    assert pb_value.enum_literal.value.int_value == 3
    assert value_to_python(pb_value).value == 3
    assert not Connection.__new__(Connection)._python_to_value(
        EnumLiteral("D::Color::red")).enum_literal.HasField("value")


def test_a_service_without_the_capability_still_reports_unsupported():
    """An older service sends a null naming the reason, which stays an error."""
    unsupported = sysml_pb2.Value(null="unsupported")
    with pytest.raises(UnsupportedValueError):
        value_to_python(unsupported)

    slot = sysml_pb2.FeatureValue(
        feature_name="c", value=unsupported, materialized=True
    )
    with pytest.raises(FeatureValueError):
        feature_value_to_python("c", slot)

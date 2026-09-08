"""Tests for the unbounded value ``*`` as a Python value.

The infinity arm is a boolean, and only ``true`` asserts the value: a ``false``
arm is malformed transport data rather than an unbounded number.
"""

import pytest

from opensysml.errors import UnsupportedValueError
from opensysml.proto import sysml_pb2
from opensysml.values import INFINITY, value_to_python


def test_an_asserted_infinity_arm_is_the_unbounded_value():
    assert value_to_python(sysml_pb2.Value(infinity=True)) is INFINITY


def test_a_denied_infinity_arm_is_malformed():
    with pytest.raises(UnsupportedValueError):
        value_to_python(sysml_pb2.Value(infinity=False))


def test_a_denied_infinity_arm_nested_in_a_sequence_is_malformed():
    sequence = sysml_pb2.Value(
        sequence=sysml_pb2.ValueSequence(
            elements=[sysml_pb2.Value(int_value=1), sysml_pb2.Value(infinity=False)]
        )
    )

    with pytest.raises(UnsupportedValueError):
        value_to_python(sequence)

"""A model-level result the model leaves open.

The service sends an expression over an unbound feature, or over one whose
multiplicity fixes no count, as the ``undetermined`` arm of ``Value``, so the
client reads it as an :class:`opensysml.Undetermined`: a successful answer spelled
``<undetermined>`` as every other surface spells it, distinct from
:data:`opensysml.UNSET` and from ``None``, and neither true nor false.
"""

import opensysml
from opensysml.capabilities import CAPABILITY_UNDETERMINED_VALUE
from opensysml.proto import sysml_pb2
from opensysml.values import UNSET, Undetermined, value_to_python

import pytest


def undetermined_value(reason="u has no value in the model", lower="1", upper="1"):
    """The wire form of an undetermined result, as the service sends one."""
    return sysml_pb2.Value(undetermined=sysml_pb2.Undetermined(
        reason=reason,
        count=sysml_pb2.MultiplicityInfo(lower=lower, upper=upper),
    ))


def test_undetermined_reads_with_its_reason_and_count():
    got = value_to_python(undetermined_value())
    assert got == Undetermined(reason="u has no value in the model", count_lower="1", count_upper="1")
    assert value_to_python(undetermined_value("gear fixes no count", "1", "*")).count_upper == "*"


def test_undetermined_is_spelled_as_every_surface_spells_it():
    assert str(Undetermined(reason="x")) == "<undetermined>"


def test_undetermined_is_neither_true_nor_false():
    with pytest.raises(TypeError, match="neither true nor false"):
        bool(Undetermined(reason="u has no value in the model"))


def test_undetermined_is_neither_unset_nor_none():
    got = value_to_python(undetermined_value())
    assert got is not UNSET
    assert got is not None
    assert got != UNSET
    assert value_to_python(sysml_pb2.Value(unset=True)) is UNSET
    assert value_to_python(sysml_pb2.Value(null="")) is None


def test_undetermined_in_a_sequence_is_read_element_by_element():
    sequence = sysml_pb2.Value(sequence=sysml_pb2.ValueSequence(elements=[
        sysml_pb2.Value(int_value=1),
        undetermined_value(),
    ]))
    assert value_to_python(sequence) == [1, Undetermined("u has no value in the model", "1", "1")]


def test_undetermined_is_exported_with_its_capability():
    assert opensysml.Undetermined is Undetermined
    assert CAPABILITY_UNDETERMINED_VALUE == "undetermined_value"

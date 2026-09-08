"""Native document queries and document rendering.

The service runs a *document query* — a calc def specializing
``DocumentQueries::Query`` — and answers typed rows, and renders a *document* —
a part def specializing ``DocumentQueries::Document`` — to Markdown. These are
the model's own named queries and documents, not the SysML v2 API & Services
Query that :mod:`opensysml.query` builds.

A binding value is a plain Python value (``str``, ``int``, ``float``,
``bool``), a :class:`~opensysml.values.Quantity`, or an :class:`ElementRef`
naming a model element by qualified name. Answered cells decode back to the
same kinds, plus :data:`INFINITY` for an unbounded multiplicity.
"""

from dataclasses import dataclass
from typing import Sequence, Union

from opensysml.errors import OpenSysMLError, UnsupportedValueError
from opensysml.proto import sysml_pb2
from opensysml.values import INFINITY, Quantity, _Infinity


class DocumentQueryError(OpenSysMLError, ValueError):
    """Raised when a binding cannot be written before anything is sent."""


@dataclass(frozen=True)
class ElementRef:
    """A model element, named by qualified name.

    Attributes:
        id: Qualified name of the element
        type: Metamodel type name ("PartUsage", ...); empty when bound by a
            caller, reported when answered by the service
    """

    id: str
    type: str = ""

    def __str__(self):
        return f"{self.id} ({self.type})" if self.type else self.id


#: What a binding value or an answered cell value may be.
DocumentValue = Union[ElementRef, str, int, float, bool, Quantity, _Infinity]

#: What ``bindings`` accepts for one parameter: one value or several.
BindingValues = Union[DocumentValue, Sequence[DocumentValue]]


@dataclass(frozen=True)
class DocumentRow:
    """One selected element and its projected cells, one per column.

    Attributes:
        element: The selected element itself
        cells: One value sequence per column, in column order
    """

    element: ElementRef
    cells: tuple

    def __getitem__(self, index):
        return self.cells[index]


@dataclass(frozen=True)
class DocumentQueryResult:
    """A document query's answer: projected columns and typed rows, both in the
    deterministic order the engine reports.

    Attributes:
        columns: Projected property names, in projection order
        rows: The selected rows, in the engine's order
    """

    columns: tuple
    rows: tuple

    def __iter__(self):
        return iter(self.rows)

    def __len__(self):
        return len(self.rows)


def build_bindings(bindings=None):
    """Translate a bindings mapping into the RPC's protobuf.

    Args:
        bindings (Mapping, optional): Parameter name to one value or a list of
            values. A ``list``/``tuple`` binds several values; anything else,
            including ``str``, binds one.

    Returns:
        list[sysml_pb2.DocumentQueryBinding]: What the request carries

    Raises:
        DocumentQueryError: If a value is not one a binding can carry
    """
    if not bindings:
        return []
    out = []
    for parameter, values in bindings.items():
        if not isinstance(values, (list, tuple)):
            values = [values]
        out.append(sysml_pb2.DocumentQueryBinding(
            parameter=parameter,
            values=[_bound_value(parameter, value) for value in values],
        ))
    return out


def _bound_value(parameter, value):
    """One binding value as the wire writes it. bool before int: it is one."""
    if isinstance(value, ElementRef):
        return sysml_pb2.DocumentValue(element_id=value.id)
    if isinstance(value, bool):
        return sysml_pb2.DocumentValue(bool_value=value)
    if isinstance(value, str):
        return sysml_pb2.DocumentValue(string_value=value)
    if isinstance(value, int):
        if not -(1 << 63) <= value < (1 << 63):
            raise DocumentQueryError(
                f"binding {parameter!r} cannot carry {value!r}: an int must "
                f"fit in a signed 64-bit integer"
            )
        return sysml_pb2.DocumentValue(int_value=value)
    if isinstance(value, float):
        return sysml_pb2.DocumentValue(real_value=value)
    if isinstance(value, Quantity):
        return sysml_pb2.DocumentValue(quantity=_bound_quantity(parameter, value))
    raise DocumentQueryError(
        f"binding {parameter!r} cannot carry {value!r}: a binding is a str, "
        f"int, float, bool, Quantity or ElementRef"
    )


def _bound_quantity(parameter, value):
    """A Quantity as the wire writes it; one it cannot carry is a caller error."""
    if isinstance(value.magnitude, int) and not isinstance(value.magnitude, bool):
        if not -(1 << 63) <= value.magnitude < (1 << 63):
            raise DocumentQueryError(
                f"binding {parameter!r} cannot carry {value!r}: an Integer magnitude "
                f"must fit in a signed 64-bit integer"
            )
    try:
        return value.to_pb()
    except UnsupportedValueError as exc:
        raise DocumentQueryError(
            f"binding {parameter!r} cannot carry {value!r}: {exc}"
        ) from exc


def result_of(response):
    """Decode a ``RunDocumentQueryResponse`` into a :class:`DocumentQueryResult`."""
    return DocumentQueryResult(
        columns=tuple(column.name for column in response.columns),
        rows=tuple(
            DocumentRow(
                element=_element_of(row.element),
                cells=tuple(
                    tuple(_value_of(value) for value in cell.values)
                    for cell in row.cells
                ),
            )
            for row in response.rows
        ),
    )


def _element_of(value):
    """The row's selected element, or an anonymous one when unnamed."""
    if value.WhichOneof("kind") == "element_id":
        return ElementRef(id=value.element_id, type=value.element_type)
    return ElementRef(id="", type=value.element_type)


def _value_of(value):
    """One answered value as the Python value it is."""
    kind = value.WhichOneof("kind")
    if kind == "element_id":
        return ElementRef(id=value.element_id, type=value.element_type)
    if kind == "string_value":
        return value.string_value
    if kind == "int_value":
        return value.int_value
    if kind == "real_value":
        return value.real_value
    if kind == "bool_value":
        return value.bool_value
    if kind == "infinity":
        return INFINITY
    if kind == "quantity":
        return Quantity.from_pb(value.quantity)
    raise UnsupportedValueError(
        f"the service answered a document value this client cannot read: {value}"
    )

"""Querying a model the way the SysML v2 API & Services standard queries one.

The standard's ``Query`` resource is ``scope`` / ``select`` / ``where``, where
``where`` is a ``Constraint``: a ``PrimitiveConstraint`` (one property, one of
``=`` ``>`` ``<``, values, and ``inverse``) or a ``CompositeConstraint`` (nested
constraints combined with ``and`` / ``or``). This module accepts that JSON
verbatim — a cookbook payload works unchanged — as well as keyword form, and
translates it to the ``Query`` RPC's protobuf.

It is an interop surface: the standard's query model has no graph traversal and
no transitive closure, so a question like "everything under this part" is a
scope, not a constraint. ``docs/reference/api.md`` states exactly what is supported.
"""

from dataclasses import dataclass
from typing import Dict, Mapping, Sequence

from opensysml.errors import OpenSysMLError
from opensysml.proto import sysml_pb2

# The property the standard names an object's type by.
_TYPE_KEY = "@type"

#: ``@type`` of the standard's query resource.
TYPE_QUERY = "Query"
#: ``@type`` of a single-property constraint.
TYPE_PRIMITIVE_CONSTRAINT = "PrimitiveConstraint"
#: ``@type`` of a constraint over nested constraints.
TYPE_COMPOSITE_CONSTRAINT = "CompositeConstraint"

# The standard's operator spellings, mapped onto the RPC's enums.
_PRIMITIVE_OPERATORS = {
    "=": sysml_pb2.PRIMITIVE_OPERATOR_EQUAL,
    ">": sysml_pb2.PRIMITIVE_OPERATOR_GREATER,
    "<": sysml_pb2.PRIMITIVE_OPERATOR_LESS,
}
_COMPOSITE_OPERATORS = {
    "and": sysml_pb2.COMPOSITE_OPERATOR_AND,
    "or": sysml_pb2.COMPOSITE_OPERATOR_OR,
}


class QueryError(OpenSysMLError):
    """Raised when a query payload is not one the standard's query model describes."""


@dataclass(frozen=True)
class QueryElement:
    """An element a query selected.

    Attributes:
        id: Qualified name of the element, which is what a scope names it by.
        type: The element's metamodel type, e.g. ``PartUsage``.
        properties: The selected properties it has. A property an element does
            not have is absent rather than empty.
    """

    id: str
    type: str
    properties: Dict[str, str]

    def get(self, property_name, default=None):
        """Value of a selected property, or ``default`` if the element has none."""
        return self.properties.get(property_name, default)

    def as_dict(self):
        """The element as the standard's JSON names it: ``@id``, ``@type``, properties."""
        return {"@id": self.id, _TYPE_KEY: self.type, **self.properties}

    def __str__(self):
        return f"{self.id} ({self.type})"


def build_query(payload=None, scope=None, select=None, where=None):
    """Translate a query, given as the standard's JSON or as keywords, to protobuf.

    Args:
        payload (dict, optional): The standard's ``Query`` object. Its ``@type``,
            when present, must be ``Query``. Keywords may not be combined with it.
        scope (list, optional): Elements to consider, each a qualified name or an
            element reference (``{"@id": ...}``). Empty means the whole model.
        select (list, optional): Properties to report. Empty reports every one.
        where (dict, optional): Constraint to filter by. Absent selects the scope.

    Returns:
        sysml_pb2.Query: The query, ready to send.

    Raises:
        QueryError: If the payload is not a query the standard describes.
    """
    if payload is not None:
        if scope is not None or select is not None or where is not None:
            raise QueryError(
                "pass a query payload or scope/select/where keywords, not both"
            )
        if not isinstance(payload, Mapping):
            raise QueryError(f"a query is an object, not {type(payload).__name__}")
        declared = payload.get(_TYPE_KEY, TYPE_QUERY)
        if declared != TYPE_QUERY:
            raise QueryError(f"expected a {TYPE_QUERY!r} payload, got {declared!r}")
        unknown = set(payload) - {_TYPE_KEY, "@id", "owningProject", "scope", "select", "where"}
        if unknown:
            raise QueryError(
                f"a query has no {', '.join(sorted(unknown))}; the standard's query "
                f"is scope, select and where"
            )
        scope, select, where = (
            payload.get("scope"), payload.get("select"), payload.get("where"),
        )

    query = sysml_pb2.Query(
        scope=[_scope_id(entry) for entry in _sequence("scope", scope)],
        select=[_property_name(entry) for entry in _sequence("select", select)],
    )
    if where is not None:
        query.where.CopyFrom(_constraint(where))
    return query


def _scope_id(entry):
    """A scope entry, as a qualified name or as an element reference."""
    if isinstance(entry, str):
        return entry
    if isinstance(entry, Mapping) and isinstance(entry.get("@id"), str):
        return entry["@id"]
    raise QueryError(
        f"a scope entry is an element's qualified name or a {{'@id': ...}} "
        f"reference, not {entry!r}"
    )


def _property_name(entry):
    if not isinstance(entry, str):
        raise QueryError(f"a selected property is a name, not {entry!r}")
    return entry


def _sequence(field, value):
    """A list-valued field, tolerating a single entry given on its own."""
    if value is None:
        return []
    if isinstance(value, (str, Mapping)):
        return [value]
    if not isinstance(value, Sequence):
        raise QueryError(f"{field} is a list, not {type(value).__name__}")
    return list(value)


def _constraint(payload):
    """Translate one constraint, dispatching on the ``@type`` the standard names it by."""
    if not isinstance(payload, Mapping):
        raise QueryError(f"a constraint is an object, not {type(payload).__name__}")
    declared = payload.get(_TYPE_KEY)
    if declared is None:
        # The variant a constraint without an explicit @type must be, since the
        # two carry disjoint fields.
        declared = (
            TYPE_COMPOSITE_CONSTRAINT if "constraint" in payload
            else TYPE_PRIMITIVE_CONSTRAINT
        )
    if declared == TYPE_PRIMITIVE_CONSTRAINT:
        return sysml_pb2.Constraint(primitive=_primitive(payload))
    if declared == TYPE_COMPOSITE_CONSTRAINT:
        return sysml_pb2.Constraint(composite=_composite(payload))
    raise QueryError(
        f"unknown constraint type {declared!r}; the standard's constraints are "
        f"{TYPE_PRIMITIVE_CONSTRAINT} and {TYPE_COMPOSITE_CONSTRAINT}"
    )


def _primitive(payload):
    _reject_unknown(payload, {_TYPE_KEY, "@id", "inverse", "property", "operator", "value"},
                    TYPE_PRIMITIVE_CONSTRAINT)
    operator = payload.get("operator")
    if operator not in _PRIMITIVE_OPERATORS:
        raise QueryError(
            f"unknown primitive operator {operator!r}; expected one of "
            f"{', '.join(sorted(_PRIMITIVE_OPERATORS))}"
        )
    name = payload.get("property")
    if not isinstance(name, str) or not name:
        raise QueryError(f"a primitive constraint names one property, not {name!r}")
    return sysml_pb2.PrimitiveConstraint(
        inverse=bool(payload.get("inverse", False)),
        property=name,
        operator=_PRIMITIVE_OPERATORS[operator],
        value=_values(payload.get("value")),
    )


def _composite(payload):
    _reject_unknown(payload, {_TYPE_KEY, "@id", "constraint", "operator"},
                    TYPE_COMPOSITE_CONSTRAINT)
    operator = payload.get("operator")
    if operator not in _COMPOSITE_OPERATORS:
        raise QueryError(
            f"unknown composite operator {operator!r}; expected one of "
            f"{', '.join(sorted(_COMPOSITE_OPERATORS))}"
        )
    nested = payload.get("constraint")
    if not isinstance(nested, Sequence) or isinstance(nested, (str, bytes)) or not nested:
        raise QueryError(
            f"a composite constraint combines a non-empty list of constraints, "
            f"not {nested!r}"
        )
    return sysml_pb2.CompositeConstraint(
        operator=_COMPOSITE_OPERATORS[operator],
        constraint=[_constraint(entry) for entry in nested],
    )


def _reject_unknown(payload, known, what):
    unknown = set(payload) - known
    if unknown:
        raise QueryError(f"a {what} has no {', '.join(sorted(unknown))}")


def _values(value):
    """The values compared against, as a list even when one was given on its own."""
    if value is None:
        return []
    if isinstance(value, (list, tuple)):
        return [_value(v) for v in value]
    return [_value(value)]


def _value(value):
    """A compared value, as the text the service compares against."""
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, (str, int, float)):
        return str(value)
    raise QueryError(f"cannot compare against {value!r}")


def elements_of(response):
    """The elements a ``QueryResponse`` reports, as :class:`QueryElement` records."""
    return [
        QueryElement(id=e.id, type=e.type, properties=dict(e.properties))
        for e in response.elements
    ]

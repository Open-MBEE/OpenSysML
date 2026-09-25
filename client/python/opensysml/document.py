"""Native document queries and document rendering.

The service runs a *document query* — a calc def specializing
``DocumentQueries::Query`` — and answers typed rows, and renders a *document* —
a part def specializing ``DocumentQueries::Document`` — to Markdown. These are
the model's own named queries and documents, not the SysML v2 API & Services
Query that :mod:`opensysml.query` builds.

A binding value is a plain Python value (``str``, ``int``, ``float``,
``bool``), a :class:`~opensysml.values.Quantity`, an :class:`ElementRef`
naming a model element by qualified name, or an :class:`ObjectRef` naming an
object the service holds for the model since ``instantiate`` — by id or by
path (``"car.wheels[2]"``). Answered cells decode back to the same kinds, plus
:data:`INFINITY` for an unbounded multiplicity, a :class:`DocumentVerdict`
for a row a ``Verdicts`` query answered, a :class:`DocumentState` for a row a
``States`` query answered and a :class:`DocumentEvent` for a row an ``Events``
query answered.
"""

from dataclasses import dataclass
from typing import Optional, Sequence, Union

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


@dataclass(frozen=True)
class ObjectRef:
    """An object the service holds for the model, created by ``instantiate``.

    Bound, it names the object by ``path`` when set and by ``id`` otherwise;
    one setting both must name one object by both. Answered, it carries all
    three fields.

    Attributes:
        id: The object's id, as ``instantiate`` answered it
        path: The object by the label a session reaches it under: the
            qualified name it was instantiated as (``"Garage::car"``), its id
            (``"#2"``), or a path through feature values of either
            (``"Garage::car.wheels[2]"``, ``"#2.wheels[2]"``; indexes count
            from 1)
        element: The usage the object is held under, its definition or usage;
            reported when answered, ignored when bound
    """

    id: int = 0
    path: str = ""
    element: Optional[ElementRef] = None

    def __str__(self):
        return self.path or f"#{self.id}"


@dataclass(frozen=True)
class DocumentVerdict:
    """A row a ``Verdicts`` query answered: an assertion checked on one object.

    Answered only; binding one is refused.

    Attributes:
        assertion: The constraint, requirement, satisfy usage or verification
            case checked; its ``id`` is empty when the assertion is anonymous
        kind: ``"constraint"``, ``"requirement"``, ``"satisfaction"`` or
            ``"verification"``
        text: The assertion as written (``"assert constraint massKnown"``)
        path: The object checked, named from the element the query was bound
            to (``"Garage::car.wheels[2]"``)
        status: ``"holds"``, ``"violated"`` or ``"undecided"``
        condition: The condition that evaluated to false, as written; empty
            otherwise
        reason: Why the assertion is violated or undecided; empty when it holds
        verification: Verdict kinds (``"pass"``, ``"fail"``, ...) of the
            verification cases verifying the requirement the row is about; a
            verification row's own kind
    """

    assertion: ElementRef
    kind: str
    text: str
    path: str
    status: str
    condition: str = ""
    reason: str = ""
    verification: tuple = ()

    def __str__(self):
        where = f" on {self.path}" if self.path else ""
        return f"{self.text}{where}: {self.status}"


@dataclass(frozen=True)
class DocumentState:
    """A row a ``States`` query answered: one active leaf state of one object.

    Answered only; binding one is refused.

    Attributes:
        object: The object whose state machine the row reads
        machine: The exhibited state usage (``"lp"``), or the state def's name
        name: The active leaf state's own name (``"dim"``)
        path: The leaf's path from the machine's top level (``"on.dim"``)
        state: The state usage's declaration
        region: The orthogonal region declaring the leaf (``"light"``); empty
            when the leaf is not in one
        enclosing: The active composite states around the leaf, outermost first
    """

    object: ObjectRef
    machine: str
    name: str
    path: str
    state: Optional[ElementRef] = None
    region: str = ""
    enclosing: tuple = ()

    def __str__(self):
        return f"{self.object}.{self.machine} in {self.path}"


@dataclass(frozen=True)
class DocumentEvent:
    """A row an ``Events`` query answered: one record of a session's trace.

    Answered only; binding one is refused.

    Attributes:
        kind: ``"accept"``, ``"send"``, ``"transition"``, ``"entry"``,
            ``"exit"``, ``"do"``, ``"choice"`` or ``"guard"``
        time: The instant the record was written at, in the runtime clock's
            unit — a :class:`~opensysml.values.Quantity` when the clock carries
            one, a plain number otherwise
        object: The object the record is about; ``None`` for a record of the
            run as a whole
        machine: The state machine the record is about, as
            :attr:`DocumentState.machine` names it
        state: The state entered, exited or run (entry, exit and do records)
        from_state: The transition's source state
        to_state: The transition's target state
        target: The object a send was delivered to
        event: The accepted or sent event's type name
        payload: The accept's payload, one ``name = value`` text per attribute
        alternatives: What a choice drew from, in order
        taken: The alternative the choice took
        text: The record as the trace prints it
    """

    kind: str
    time: Quantity | int | float
    text: str
    object: Optional[ObjectRef] = None
    machine: str = ""
    state: str = ""
    from_state: str = ""
    to_state: str = ""
    target: Optional[ObjectRef] = None
    event: str = ""
    payload: tuple = ()
    alternatives: tuple = ()
    taken: str = ""

    def __str__(self):
        return f"{self.time}: {self.text}"


#: What a binding value or an answered cell value may be.
DocumentValue = Union[
    ElementRef, ObjectRef, str, int, float, bool, Quantity, _Infinity,
    DocumentVerdict, DocumentState, DocumentEvent,
]

#: What ``bindings`` accepts for one parameter: one value or several.
BindingValues = Union[DocumentValue, Sequence[DocumentValue]]


@dataclass(frozen=True)
class DocumentRow:
    """One selected element and its projected cells, one per column.

    Attributes:
        element: The selected element itself; for an object row, the usage the
            object is held under; for a row a ``Verdicts`` query answered, the
            assertion checked; for a state or event row, the usage of the
            object the row is about
        cells: One value sequence per column, in column order
        verdict: The :class:`DocumentVerdict` a row a ``Verdicts`` query
            answered carries; ``None`` for any other row
        object: The :class:`ObjectRef` a row over held objects is about — one
            an ``Objects`` query enumerated, a bound object's part, or the
            object a state or event row is about; ``None`` for any other row
        state: The :class:`DocumentState` a row a ``States`` query answered
            carries; ``None`` for any other row
        event: The :class:`DocumentEvent` a row an ``Events`` query answered
            carries; ``None`` for any other row
    """

    element: ElementRef
    cells: tuple
    verdict: Optional[DocumentVerdict] = None
    object: Optional[ObjectRef] = None
    state: Optional[DocumentState] = None
    event: Optional[DocumentEvent] = None

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
    if isinstance(value, ObjectRef):
        if not value.id and not value.path:
            raise DocumentQueryError(
                f"binding {parameter!r} cannot carry {value!r}: an object is "
                f"bound by id or by path; neither was given"
            )
        return sysml_pb2.DocumentValue(
            object=sysml_pb2.DocumentObject(instance_id=value.id, path=value.path)
        )
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
    if isinstance(value, DocumentVerdict):
        raise DocumentQueryError(
            f"binding {parameter!r} cannot carry {value!r}: a verdict is "
            f"answered by queries, not bound to them"
        )
    if isinstance(value, (DocumentState, DocumentEvent)):
        what = "a state" if isinstance(value, DocumentState) else "an event"
        raise DocumentQueryError(
            f"binding {parameter!r} cannot carry {value!r}: {what} row is "
            f"answered by queries, not bound to them"
        )
    raise DocumentQueryError(
        f"binding {parameter!r} cannot carry {value!r}: a binding is a str, "
        f"int, float, bool, Quantity, ElementRef or ObjectRef"
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
            _row_of(row)
            for row in response.rows
        ),
    )


def _row_of(row):
    """One answered row; a verdict, state or event row keeps its typed value."""
    cells = tuple(
        tuple(_value_of(value) for value in cell.values)
        for cell in row.cells
    )
    kind = row.element.WhichOneof("kind")
    if kind == "verdict":
        verdict = _value_of(row.element)
        return DocumentRow(element=verdict.assertion, cells=cells, verdict=verdict)
    if kind == "object":
        obj = _value_of(row.element)
        return DocumentRow(element=obj.element, cells=cells, object=obj)
    if kind == "state":
        state = _value_of(row.element)
        return DocumentRow(
            element=state.object.element, cells=cells, object=state.object, state=state,
        )
    if kind == "event":
        event = _value_of(row.element)
        element = event.object.element if event.object else ElementRef(id="")
        return DocumentRow(element=element, cells=cells, object=event.object, event=event)
    return DocumentRow(element=_element_of(row.element), cells=cells)


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
    if kind == "object":
        obj = value.object
        return ObjectRef(
            id=obj.instance_id,
            path=obj.path,
            element=_element_of(obj.element),
        )
    if kind == "verdict":
        verdict = value.verdict
        return DocumentVerdict(
            assertion=_element_of(verdict.assertion),
            kind=verdict.kind,
            text=verdict.text,
            path=verdict.path,
            status=verdict.verdict,
            condition=verdict.condition,
            reason=verdict.reason,
            verification=tuple(verdict.verification),
        )
    if kind == "state":
        state = value.state
        return DocumentState(
            object=_object_of(state.object),
            machine=state.machine,
            name=state.name,
            path=state.state_path,
            state=_element_of(state.state) if state.HasField("state") else None,
            region=state.region,
            enclosing=tuple(state.enclosing),
        )
    if kind == "event":
        event = value.event
        return DocumentEvent(
            kind=event.kind,
            time=_value_of(event.time),
            text=event.text,
            object=_object_of(event.object) if event.HasField("object") else None,
            machine=event.machine,
            state=event.state,
            from_state=getattr(event, "from"),
            to_state=event.to,
            target=_object_of(event.target) if event.HasField("target") else None,
            event=event.event,
            payload=tuple(event.payload),
            alternatives=tuple(event.alternatives),
            taken=event.taken,
        )
    raise UnsupportedValueError(
        f"the service answered a document value this client cannot read: {value}"
    )


def _object_of(obj):
    """An answered ``DocumentObject`` as the :class:`ObjectRef` it names."""
    return ObjectRef(id=obj.instance_id, path=obj.path, element=_element_of(obj.element))

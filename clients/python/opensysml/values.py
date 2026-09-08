"""Conversion between protobuf Value/FeatureValue messages and Python values."""

import math
from dataclasses import dataclass, field
from typing import Any, Callable, Dict, Iterator, List, Optional, Sequence, Tuple, Union

from opensysml.enumeration import EnumLiteral
from opensysml.errors import FeatureValueError, OpenSysMLError, UnsupportedValueError
from opensysml.proto import sysml_pb2

#: What a quantity's magnitude can be: the service keeps Integer and Real apart.
Magnitude = Union[int, float]


class IncommensurableUnitsError(OpenSysMLError):
    """Raised when quantities measuring different things are compared or combined.

    ``5.0 [kg] < 2.0 [m]`` relates no two magnitudes, so it is an error rather
    than an answer — the same answer the runtime gives the model.

    Attributes:
        left (Unit): Unit of the left operand
        right (Unit): Unit of the right operand
    """

    def __init__(self, operation, left, right):
        super().__init__(
            f"cannot {operation} a quantity in [{left}] and one in [{right}]: "
            f"{left} reduces to {left.reduction()} and {right} to {right.reduction()}, "
            f"which measure different things"
        )
        self.left = left
        self.right = right


@dataclass(frozen=True)
class UnitFactor:
    """One base unit of a unit's reduction, raised to an exponent.

    Attributes:
        unit_id (str): FQN of the base unit, e.g. ``SI::m``
        exponent (float): Power it is raised to
    """

    unit_id: str
    exponent: float


@dataclass(frozen=True)
class Unit:
    """A measurement reference as a quantity carries it.

    ``text`` is the unit as the model wrote it, for reading; the reduction —
    ``scale_num``/``scale_den`` over ``factors`` — is what decides whether two
    quantities are commensurable and by what factor one converts into the other.
    ``km/h`` is the text ``"km/h"`` reduced to 1000/3600 over ``SI::m`` and
    ``SI::s^-1``.

    Attributes:
        text (str): Unit as written; empty for one never written down
        scale_num (float): Numerator of the scale factor over the base units
        scale_den (float): Denominator of that scale factor
        factors (tuple[UnitFactor, ...]): The base units it reduces to
        reduction_given (bool): Whether that reduction was given rather than
            defaulted, which is what a unit read off the wire carries
    """

    text: str = ""
    scale_num: float = 1.0
    scale_den: float = 1.0
    factors: Tuple[UnitFactor, ...] = ()
    # Not part of the value: a reduction to dimension one at scale 1 is
    # otherwise indistinguishable from no reduction at all.
    reduction_given: bool = field(default=False, compare=False, repr=False)

    @classmethod
    def from_pb(cls, text, pb_unit_term=None) -> "Unit":
        """Build from the ``unit``/``unit_term`` fields of a ``Quantity`` message."""
        if pb_unit_term is None:
            return cls(text=text)
        return cls(
            text=text,
            scale_num=pb_unit_term.scale_num,
            scale_den=pb_unit_term.scale_den,
            factors=tuple(
                UnitFactor(factor.unit_id, factor.exponent)
                for factor in pb_unit_term.factors
            ),
            reduction_given=True,
        )

    def to_pb(self) -> "sysml_pb2.UnitTerm":
        """The reduction as a ``UnitTerm`` message, for a quantity being sent."""
        return sysml_pb2.UnitTerm(
            scale_num=self.scale_num,
            scale_den=self.scale_den,
            factors=[
                sysml_pb2.UnitFactor(unit_id=factor.unit_id, exponent=factor.exponent)
                for factor in self.factors
            ],
        )

    @property
    def reduced(self) -> bool:
        """Whether the unit carries the reduction commensurability is decided over.

        A unit named with no reduction at all is unreduced; an unnamed one means
        dimension one, which is a reduction, as does a named unit whose
        reduction the service gave — ``SI::rad`` is ``m/m``, so dimension one.
        """
        return bool(
            not self.text
            or self.reduction_given
            or self.factors
            or (self.scale_num, self.scale_den) != (1.0, 1.0)
        )

    @property
    def dimensionless(self) -> bool:
        """Whether the unit reduces to no base unit, as a count or a ratio does."""
        return not self.exponents()

    def exponents(self) -> Dict[str, float]:
        """The reduction as base unit → exponent, order-independent: repeated
        base units summed and cancelled ones dropped, as the service reduces."""
        totals: Dict[str, float] = {}
        for factor in self.factors:
            totals[factor.unit_id] = totals.get(factor.unit_id, 0.0) + factor.exponent
        return {unit_id: exp for unit_id, exp in totals.items() if exp != 0}

    def commensurable(self, other: "Unit") -> bool:
        """Whether a magnitude in this unit converts into ``other``."""
        return self.exponents() == other.exponents()

    def reduction(self) -> str:
        """The reduction as text ("1000/3600·SI::m·SI::s^-1"), for a diagnostic."""
        parts = []
        if (self.scale_num, self.scale_den) != (1.0, 1.0):
            scale = (
                f"{self.scale_num:g}"
                if self.scale_den == 1.0
                else f"{self.scale_num:g}/{self.scale_den:g}"
            )
            parts.append(scale)
        for factor in self.factors:
            rendered = factor.unit_id
            if factor.exponent != 1.0:
                rendered += f"^{factor.exponent:g}"
            parts.append(rendered)
        return "·".join(parts) if parts else "1"

    def __str__(self) -> str:
        return self.text or self.reduction()


class Quantity:
    """A magnitude and the measurement reference it is expressed in.

    This is what a quantity-valued feature holds: ``inst.mass`` of
    ``attribute mass : ISQ::MassValue = 5.0 [SI::kg]`` is ``Quantity(5.0,
    Unit("kg"))``. The magnitude is the one the model wrote, in the unit it wrote
    it in — nothing is reduced to base units behind the caller's back — while
    comparisons and arithmetic go through the reduction, so ``1 [km]`` equals
    ``1000 [m]``.

    Equality holds between commensurable quantities of equal magnitude, and is
    false between quantities measuring different things: two such values are
    simply not the same value. Ordering and addition, which have no answer at
    all across incommensurable units, raise
    :class:`IncommensurableUnitsError` rather than compare bare magnitudes.

    Attributes:
        magnitude (int | float): The magnitude, in :attr:`unit`
        unit (Unit): The unit it is expressed in
    """

    __slots__ = ("magnitude", "unit")

    def __init__(self, magnitude: Magnitude, unit: Unit) -> None:
        self.magnitude = magnitude
        self.unit = unit

    @classmethod
    def from_pb(cls, pb_quantity) -> "Quantity":
        """Build from a ``Quantity`` protobuf message.

        Raises:
            UnsupportedValueError: If the message carries no magnitude, or names a
                unit without the reduction commensurability is decided over.
        """
        which = pb_quantity.WhichOneof('magnitude')
        if which == 'int_magnitude':
            magnitude: Magnitude = pb_quantity.int_magnitude
        elif which == 'real_magnitude':
            magnitude = pb_quantity.real_magnitude
        else:
            raise UnsupportedValueError(
                f"quantity in [{pb_quantity.unit}] carries no magnitude"
            )
        if not pb_quantity.HasField('unit_term'):
            # Dimension one is what an unnamed unit means, not what an unreduced
            # one means; the service rejects the latter and so does the client.
            if pb_quantity.unit:
                raise UnsupportedValueError(
                    f"quantity in [{pb_quantity.unit}] carries no reduction to base units"
                )
            return cls(magnitude, Unit())
        return cls(magnitude, Unit.from_pb(pb_quantity.unit, pb_quantity.unit_term))

    def to_pb(self) -> "sysml_pb2.Quantity":
        """Encode as a ``Quantity`` message, magnitude and unit as written.

        Raises:
            UnsupportedValueError: If the magnitude is neither an Integer nor a
                Real, or the unit is named without its reduction — the service
                decides commensurability over the reduction and rejects a unit
                sent without one.
        """
        if isinstance(self.magnitude, bool) or not isinstance(self.magnitude, (int, float)):
            raise UnsupportedValueError(
                f"quantity magnitude {self.magnitude!r} is neither an Integer nor a Real"
            )
        if not self.unit.reduced:
            raise UnsupportedValueError(
                f"quantity in [{self.unit.text}] carries no reduction to base units, "
                f"so the service cannot tell what it measures: build it from a unit "
                f"the service sent, or from one the model declares"
            )
        pb_quantity = sysml_pb2.Quantity(unit=self.unit.text, unit_term=self.unit.to_pb())
        if isinstance(self.magnitude, int):
            pb_quantity.int_magnitude = self.magnitude
        else:
            pb_quantity.real_magnitude = self.magnitude
        return pb_quantity

    def base_magnitude(self) -> float:
        """The magnitude over the base units the unit reduces to."""
        return self.magnitude * self.unit.scale_num / self.unit.scale_den

    def in_unit(self, unit: Unit) -> float:
        """The magnitude expressed in ``unit``.

        The conversion is applied as one ratio, so an exact factor stays exact:
        ``5.4 [km/h]`` in ``m/s`` is 1.5, not 1.4999999999999998.

        Raises:
            IncommensurableUnitsError: If ``unit`` measures something else.
            ZeroDivisionError: If ``unit`` reduces to a zero scale factor.
        """
        if not self.unit.commensurable(unit):
            raise IncommensurableUnitsError("express", self.unit, unit)
        return (
            self.magnitude
            * (self.unit.scale_num * unit.scale_den)
            / (self.unit.scale_den * unit.scale_num)
        )

    def to(self, unit: Unit) -> "Quantity":
        """This quantity expressed in ``unit``, magnitude converted."""
        return Quantity(self.in_unit(unit), unit)

    def __eq__(self, other: object) -> bool:
        if not isinstance(other, Quantity):
            return NotImplemented
        if not self.unit.commensurable(other.unit):
            return False
        return self.base_magnitude() == other.base_magnitude()

    def __hash__(self) -> int:
        # Keyed on the base-unit form, so commensurable equal quantities — `1
        # [km]` and `1000 [m]` — hash alike, as equality requires.
        return hash((self.base_magnitude(), tuple(sorted(self.unit.exponents().items()))))

    def __lt__(self, other: "Quantity") -> bool:
        return self._compare(other, "order") < 0

    def __le__(self, other: "Quantity") -> bool:
        return self._compare(other, "order") <= 0

    def __gt__(self, other: "Quantity") -> bool:
        return self._compare(other, "order") > 0

    def __ge__(self, other: "Quantity") -> bool:
        return self._compare(other, "order") >= 0

    def _compare(self, other: "Quantity", operation: str) -> float:
        """Order two quantities over their base units, as the runtime does."""
        if not isinstance(other, Quantity):
            raise TypeError(
                f"cannot {operation} a quantity and {type(other).__name__}: "
                f"a bare number carries no unit"
            )
        if not self.unit.commensurable(other.unit):
            raise IncommensurableUnitsError(operation, self.unit, other.unit)
        return self.base_magnitude() - other.base_magnitude()

    def __add__(self, other: "Quantity") -> "Quantity":
        return self._sum(other, 1)

    def __sub__(self, other: "Quantity") -> "Quantity":
        return self._sum(other, -1)

    def _sum(self, other: "Quantity", sign: float) -> "Quantity":
        """Add or subtract, in this quantity's unit, as the runtime does."""
        if not isinstance(other, Quantity):
            return NotImplemented
        if not self.unit.commensurable(other.unit):
            operation = "add" if sign > 0 else "subtract"
            raise IncommensurableUnitsError(operation, self.unit, other.unit)
        return Quantity(self.magnitude + sign * other.in_unit(self.unit), self.unit)

    def __neg__(self) -> "Quantity":
        return Quantity(-self.magnitude, self.unit)

    def __abs__(self) -> "Quantity":
        return Quantity(abs(self.magnitude), self.unit)

    def __mul__(self, other: Magnitude) -> "Quantity":
        """Scale the magnitude by a number, keeping the unit.

        A product of two quantities has a unit composed from theirs, which this
        client does not compose: the runtime does, so `x * y` over quantities
        belongs in the model.
        """
        if isinstance(other, bool) or not isinstance(other, (int, float)):
            return NotImplemented
        return Quantity(self.magnitude * other, self.unit)

    __rmul__ = __mul__

    def __truediv__(self, other: Magnitude) -> "Quantity":
        """Divide the magnitude by a number, keeping the unit."""
        if isinstance(other, bool) or not isinstance(other, (int, float)):
            return NotImplemented
        return Quantity(self.magnitude / other, self.unit)

    def __str__(self) -> str:
        magnitude = f"{self.magnitude:g}" if isinstance(self.magnitude, float) else str(self.magnitude)
        return f"{magnitude} [{self.unit}]"

    def __repr__(self) -> str:
        return f"Quantity({self.magnitude!r}, {self.unit!r})"


@dataclass(frozen=True)
class MeasurementRef:
    """A measurement unit held as a value by itself, with no magnitude.

    ``SI::m``, ``km``, or ``m / s`` as an operation composed it: what a
    ``MeasurementUnit``-typed attribute or a quantity's ``mRef`` evaluates to,
    and what ``ConvertQuantity`` takes as its target. It carries the unit as a
    :class:`Quantity` does — text and reduction — plus the declaration it names.

    Attributes:
        unit (Unit): The unit as written and its reduction to base units
        unit_id (str): FQN of the one unit declaration the reference names
            (``SI::kilometre``); empty for a unit an operation composed, which
            names none
    """

    unit: Unit
    unit_id: str = ""

    @classmethod
    def from_pb(cls, pb_ref) -> "MeasurementRef":
        """Build from a ``MeasurementRef`` protobuf message.

        Raises:
            UnsupportedValueError: If the message names no unit at all, or names
                one without the reduction commensurability is decided over.
        """
        if not pb_ref.unit and not pb_ref.unit_id and not pb_ref.HasField('unit_term'):
            raise UnsupportedValueError("measurement reference naming no unit")
        if not pb_ref.HasField('unit_term'):
            raise UnsupportedValueError(
                f"measurement reference {pb_ref.unit or pb_ref.unit_id} "
                f"carries no reduction to base units"
            )
        return cls(Unit.from_pb(pb_ref.unit, pb_ref.unit_term), pb_ref.unit_id)

    def to_pb(self) -> "sysml_pb2.MeasurementRef":
        """Encode as a ``MeasurementRef`` message, unit as written.

        Raises:
            UnsupportedValueError: If the unit is named without its reduction —
                the service decides commensurability over the reduction and
                rejects a unit sent without one.
        """
        if not self.unit.reduced:
            raise UnsupportedValueError(
                f"measurement reference {self.unit.text} carries no reduction to "
                f"base units, so the service cannot tell what it measures: build it "
                f"from a unit the service sent, or from one the model declares"
            )
        return sysml_pb2.MeasurementRef(
            unit=self.unit.text, unit_term=self.unit.to_pb(), unit_id=self.unit_id
        )

    def __str__(self) -> str:
        return str(self.unit)


def _is_number(value: object) -> bool:
    """Whether a value is an Integer or a Real as the wire keeps them apart: a bool is neither."""
    return isinstance(value, (int, float)) and not isinstance(value, bool)


def _number_to_pb(value: Magnitude) -> "sysml_pb2.Value":
    return (
        sysml_pb2.Value(int_value=value)
        if isinstance(value, int)
        else sysml_pb2.Value(real_value=value)
    )


def _format_number(value: Magnitude) -> str:
    return f"{value:g}" if isinstance(value, float) else str(value)


@dataclass(frozen=True)
class Array:
    """A multidimensional array: its shape and its elements in row-major order.

    ``attribute grid : Array { :>> dimensions = (2, 3); :>> elements = (1, 2, 3,
    4, 5, 6); }`` is ``Array((2, 3), (1, 2, 3, 4, 5, 6))``. An element is any
    value a feature can hold, an :class:`Array` or a :class:`Quantity` included;
    ``elements`` holds them flattened as the model states them, with the last
    dimension varying fastest, and :meth:`nested` unfolds them. A rank-0 array
    holds exactly one element.

    Attributes:
        dimensions (tuple[int, ...]): Extent of each dimension, all positive
        elements (tuple): The elements, row-major

    Raises:
        ValueError: If a dimension is not positive, or the elements do not fill
            the dimensions exactly.
    """

    dimensions: Tuple[int, ...]
    elements: Tuple[Any, ...]

    def __init__(self, dimensions: Sequence[int], elements: Sequence[Any]) -> None:
        dims = tuple(dimensions)
        elems = tuple(elements)
        for extent in dims:
            if isinstance(extent, bool) or not isinstance(extent, int) or extent <= 0:
                raise ValueError(f"array dimension {extent!r} is not a positive integer")
        if len(elems) != math.prod(dims):
            raise ValueError(
                f"array of dimensions {dims} holds {len(elems)} element(s), "
                f"want {math.prod(dims)}"
            )
        object.__setattr__(self, "dimensions", dims)
        object.__setattr__(self, "elements", elems)

    @property
    def rank(self) -> int:
        """Number of dimensions."""
        return len(self.dimensions)

    def __len__(self) -> int:
        return len(self.elements)

    def __iter__(self) -> Iterator[Any]:
        return iter(self.elements)

    def __getitem__(self, index: int | Tuple[int, ...]) -> Any:
        """The element at a row-major position, or at a full multi-index."""
        if isinstance(index, tuple):
            if len(index) != self.rank:
                raise IndexError(
                    f"index {index} has {len(index)} coordinate(s), array has rank {self.rank}"
                )
            flat = 0
            for extent, coordinate in zip(self.dimensions, index):
                if not 0 <= coordinate < extent:
                    raise IndexError(f"index {index} is outside dimensions {self.dimensions}")
                flat = flat * extent + coordinate
            return self.elements[flat]
        return self.elements[index]

    def nested(self) -> Any:
        """The elements as nested lists, one level per dimension; a rank-0 array is its element."""
        def unfold(dims: Tuple[int, ...], flat: Sequence[Any]) -> Any:
            if not dims:
                return flat[0]
            stride = len(flat) // dims[0]
            return [unfold(dims[1:], flat[i * stride:(i + 1) * stride]) for i in range(dims[0])]

        return unfold(self.dimensions, self.elements)

    @classmethod
    def from_pb(cls, pb_array, resolve_instance=None) -> "Array":
        """Build from an ``Array`` protobuf message.

        Raises:
            UnsupportedValueError: If the message's shape and elements disagree,
                or an element is one the service reported as unsupported.
        """
        try:
            return cls(
                tuple(pb_array.dimensions),
                tuple(value_to_python(v, resolve_instance) for v in pb_array.elements),
            )
        except ValueError as exc:
            raise UnsupportedValueError(f"malformed array: {exc}") from exc

    def to_pb(self, encode: Callable[[Any], "sysml_pb2.Value"]) -> "sysml_pb2.Array":
        """Encode as an ``Array`` message, each element through ``encode``."""
        return sysml_pb2.Array(
            dimensions=list(self.dimensions),
            elements=[encode(element) for element in self.elements],
        )

    def __str__(self) -> str:
        dims = ", ".join(str(d) for d in self.dimensions)
        return f"Array({dims})[{', '.join(str(e) for e in self.elements)}]"


@dataclass(frozen=True)
class Vector:
    """A vector of numbers, each an Integer or a Real as the model computed it.

    ``VectorOf((3.0, 4.0))`` is ``Vector((3.0, 4.0))``. A component that is a
    bool or not a number is refused, as the service refuses it.

    Attributes:
        components (tuple[int | float, ...]): The components, in order
    """

    components: Tuple[Magnitude, ...]

    def __init__(self, components: Sequence[Magnitude]) -> None:
        comps = tuple(components)
        for component in comps:
            if not _is_number(component):
                raise ValueError(f"vector component {component!r} is not a number")
        object.__setattr__(self, "components", comps)

    def __len__(self) -> int:
        return len(self.components)

    def __iter__(self) -> Iterator[Magnitude]:
        return iter(self.components)

    def __getitem__(self, index: int) -> Magnitude:
        return self.components[index]

    @classmethod
    def from_pb(cls, pb_vector) -> "Vector":
        """Build from a ``Vector`` protobuf message.

        Raises:
            UnsupportedValueError: If a component is not an Integer or a Real.
        """
        components: List[Magnitude] = []
        for pb_component in pb_vector.components:
            kind = pb_component.WhichOneof("kind")
            if kind == "int_value":
                components.append(pb_component.int_value)
            elif kind == "real_value":
                components.append(pb_component.real_value)
            else:
                raise UnsupportedValueError(
                    f"malformed vector: component is {kind or 'empty'}, not a number"
                )
        return cls(components)

    def to_pb(self) -> "sysml_pb2.Vector":
        """Encode as a ``Vector`` message, Integer and Real components kept apart."""
        return sysml_pb2.Vector(components=[_number_to_pb(c) for c in self.components])

    def __str__(self) -> str:
        return "⟨" + ", ".join(_format_number(c) for c in self.components) + "⟩"


@dataclass(frozen=True)
class VectorQuantity:
    """A vector whose components are quantities, each with its own unit.

    ``VectorOf((3.0, 4.0)) [SI::m]`` is ``VectorQuantity((Quantity(3.0, m),
    Quantity(4.0, m)))``; the units usually agree but need not — a position in
    polar coordinates holds a metre and a radian. Each component carries the unit
    as written and its reduction, as a :class:`Quantity` does.

    Attributes:
        components (tuple[Quantity, ...]): The components, at least one
    """

    components: Tuple[Quantity, ...]

    def __init__(self, components: Sequence[Quantity]) -> None:
        comps = tuple(components)
        if not comps:
            raise ValueError("vector quantity has no components")
        for component in comps:
            if not isinstance(component, Quantity):
                raise ValueError(f"vector quantity component {component!r} is not a Quantity")
        object.__setattr__(self, "components", comps)

    def __len__(self) -> int:
        return len(self.components)

    def __iter__(self) -> Iterator[Quantity]:
        return iter(self.components)

    def __getitem__(self, index: int) -> Quantity:
        return self.components[index]

    @property
    def unit(self) -> Optional[Unit]:
        """The one unit every component shares, or ``None`` when they differ."""
        units = {component.unit for component in self.components}
        return next(iter(units)) if len(units) == 1 else None

    def magnitudes(self) -> Vector:
        """The magnitudes as a :class:`Vector`, each in its component's unit."""
        return Vector([component.magnitude for component in self.components])

    @classmethod
    def from_pb(cls, pb_vector_quantity) -> "VectorQuantity":
        """Build from a ``VectorQuantity`` protobuf message.

        Raises:
            UnsupportedValueError: If it has no components, or a component is a
                quantity the client cannot read.
        """
        if not pb_vector_quantity.components:
            raise UnsupportedValueError("malformed vector quantity: no components")
        return cls([Quantity.from_pb(c) for c in pb_vector_quantity.components])

    def to_pb(self) -> "sysml_pb2.VectorQuantity":
        """Encode as a ``VectorQuantity`` message, one ``Quantity`` per component."""
        return sysml_pb2.VectorQuantity(components=[c.to_pb() for c in self.components])

    def __str__(self) -> str:
        unit = self.unit
        if unit is not None:
            return f"{self.magnitudes()} [{unit}]"
        return "⟨" + ", ".join(str(c) for c in self.components) + "⟩"


class UnsetType:
    """A feature holding no value: a valueless feature of a value type.

    Distinct from ``None``, the model's ``null``. Falsy, reads as ``<unset>``
    as every other surface spells it, and is a singleton, so ``is UNSET`` tests it.
    """

    _instance = None

    def __new__(cls):
        if cls._instance is None:
            cls._instance = super().__new__(cls)
        return cls._instance

    def __bool__(self) -> bool:
        return False

    def __repr__(self) -> str:
        return "<unset>"

    __str__ = __repr__


#: The value of a feature that holds none. See :class:`UnsetType`.
UNSET = UnsetType()


class _Infinity:
    """The unbounded value ``*``: no number, ordered above every finite one.

    A singleton, so ``is INFINITY`` tests it; also the unbounded multiplicity a
    document query may answer with.
    """

    _instance = None

    def __new__(cls):
        if cls._instance is None:
            cls._instance = super().__new__(cls)
        return cls._instance

    def __repr__(self) -> str:
        return "INFINITY"

    __str__ = __repr__


#: The unbounded value ``*``. See :class:`_Infinity`.
INFINITY = _Infinity()


def value_to_python(pb_value, resolve_instance=None):
    """Convert a protobuf Value into a plain Python value.

    Args:
        pb_value: sysml_pb2.Value message
        resolve_instance: optional callable mapping an instance id to an object;
            when omitted, instance references are returned as their integer id.

    Returns:
        int, float, complex, bool, str, list, None, :data:`UNSET`,
        :data:`INFINITY`, a
        :class:`Quantity`, a :class:`MeasurementRef`, an :class:`Array`, a
        :class:`Vector`, a :class:`VectorQuantity`, an
        :class:`~opensysml.enumeration.EnumLiteral`,
        or the resolved instance object. A Complex is one ``complex``, never two
        floats; a Vector is one :class:`Vector`, never a list of numbers.

    Raises:
        UnsupportedValueError: If the service reported the value as unsupported,
            or sent a value arm this client predates.
    """
    kind = pb_value.WhichOneof('kind')
    if kind == 'int_value':
        return pb_value.int_value
    if kind == 'real_value':
        return pb_value.real_value
    if kind == 'complex':
        return complex(pb_value.complex.real, pb_value.complex.imaginary)
    if kind == 'bool_value':
        return pb_value.bool_value
    if kind == 'string_value':
        return pb_value.string_value
    if kind == 'quantity':
        return Quantity.from_pb(pb_value.quantity)
    if kind == 'measurement_ref':
        return MeasurementRef.from_pb(pb_value.measurement_ref)
    if kind == 'instance_id':
        if resolve_instance is None:
            return pb_value.instance_id
        return resolve_instance(pb_value.instance_id)
    if kind == 'sequence':
        return [value_to_python(v, resolve_instance) for v in pb_value.sequence.elements]
    if kind == 'array':
        return Array.from_pb(pb_value.array, resolve_instance)
    if kind == 'vector':
        return Vector.from_pb(pb_value.vector)
    if kind == 'vector_quantity':
        return VectorQuantity.from_pb(pb_value.vector_quantity)
    if kind == 'enum_literal':
        lit = pb_value.enum_literal
        return EnumLiteral(lit.literal_id, lit.enumeration_id, lit.name)
    if kind == 'unset':
        return UNSET
    if kind == 'infinity':
        return INFINITY
    if kind == 'null':
        # A non-empty null carries the reason the value could not be sent.
        if pb_value.null:
            raise UnsupportedValueError(pb_value.null)
        return None
    # A newer service's arm parses as an unknown field: no kind at all.
    raise UnsupportedValueError("a value arm this client does not know")


def feature_value_to_python(feature_name, pb_value_msg, resolve_instance=None):
    """Convert a protobuf FeatureValue into a plain Python value.

    Args:
        feature_name (str): Feature name, used for error reporting
        pb_value_msg: sysml_pb2.FeatureValue message
        resolve_instance: optional callable mapping an instance id to an object

    Returns:
        The single value, :data:`UNSET` for one holding no value, or a list for
        a multi-valued feature.

    Raises:
        FeatureValueError: If evaluation failed or nothing was materialized.
    """
    if pb_value_msg.error:
        raise FeatureValueError(feature_name, pb_value_msg.error)

    if pb_value_msg.HasField('value'):
        try:
            return value_to_python(pb_value_msg.value, resolve_instance)
        except UnsupportedValueError as exc:
            raise FeatureValueError(feature_name, str(exc)) from exc

    if pb_value_msg.values:
        try:
            return [value_to_python(v, resolve_instance) for v in pb_value_msg.values]
        except UnsupportedValueError as exc:
            raise FeatureValueError(feature_name, str(exc)) from exc

    if pb_value_msg.materialized:
        return []

    raise FeatureValueError(feature_name, "feature value is not materialized")


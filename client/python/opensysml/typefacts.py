"""Static type facts a symbol carries: its type, multiplicity and supertypes.

These mirror the `TypeInfo`, `MultiplicityInfo` and `Specialization` messages the
service sends. They are plain frozen dataclasses so that code generation can be
exercised over synthetic symbol trees.
"""

from dataclasses import dataclass, field
from typing import Optional, Tuple

from opensysml.values import value_to_python


@dataclass(frozen=True)
class TypeFacts:
    """The static type of a usage, or the classification of a definition."""

    declared: str = ""
    """Type name as written; empty when none is declared."""

    resolved_id: str = ""
    """FQN of the resolved type; empty when unresolved or undeclared."""

    resolved_kind: str = ""
    """Symbol kind of the resolved type, e.g. ``partDef``."""

    primitive: str = ""
    """Library scalar the type reduces to, e.g. ``Real``; empty when it is not one."""

    primitive_source: str = ""
    """Origin of :attr:`primitive`: ``declared``, ``value`` or empty."""

    quantity: bool = False
    """Values carry a measurement unit."""

    unit: str = ""
    """Unit as written, when the default value names one."""

    @classmethod
    def from_pb(cls, pb_type_info) -> "TypeFacts":
        """Build from a ``TypeInfo`` protobuf message."""
        return cls(
            declared=pb_type_info.declared,
            resolved_id=pb_type_info.resolved_id,
            resolved_kind=pb_type_info.resolved_kind,
            primitive=pb_type_info.primitive,
            primitive_source=pb_type_info.primitive_source,
            quantity=pb_type_info.quantity,
            unit=pb_type_info.unit,
        )


@dataclass(frozen=True)
class Multiplicity:
    """A declared multiplicity range. A bound the service could not evaluate is empty."""

    lower: str = ""
    upper: str = ""

    @classmethod
    def from_pb(cls, pb_multiplicity) -> "Multiplicity":
        """Build from a ``MultiplicityInfo`` protobuf message."""
        return cls(lower=pb_multiplicity.lower, upper=pb_multiplicity.upper)

    @property
    def is_collection(self) -> Optional[bool]:
        """Whether the range admits more than one value, or None when unknown."""
        if self.upper == "":
            return None
        if self.upper == "*":
            return True
        try:
            return int(self.upper) > 1
        except ValueError:
            return None

    @property
    def is_optional(self) -> Optional[bool]:
        """Whether the range admits no value, or None when unknown."""
        if self.lower == "":
            return None
        try:
            return int(self.lower) == 0
        except ValueError:
            return None


@dataclass(frozen=True)
class Specialization:
    """One generalization edge: ``specializes``, ``subsets``, ``redefines`` or ``typing``."""

    kind: str
    declared: str = ""
    target_id: str = ""
    target_kind: str = ""

    @classmethod
    def from_pb(cls, pb_specialization) -> "Specialization":
        """Build from a ``Specialization`` protobuf message."""
        return cls(
            kind=pb_specialization.kind,
            declared=pb_specialization.declared,
            target_id=pb_specialization.target_id,
            target_kind=pb_specialization.target_kind,
        )


@dataclass(frozen=True)
class ValuationReference:
    """One feature a value expression reads.

    ``path`` lists the FQNs of the features a chain walks, outermost first
    (``powerPort.power`` gives ``("Motor::powerPort", "PowerPort::power")``);
    a plain reference has one entry. ``target_id`` is the last of them, the
    feature the reference denotes. Every entry resolves with ``Model.get``.
    """

    text: str
    path: Tuple[str, ...] = field(default_factory=tuple)
    target_id: str = ""

    @classmethod
    def from_pb(cls, pb_reference) -> "ValuationReference":
        """Build from a ``ValuationReference`` protobuf message."""
        return cls(
            text=pb_reference.text,
            path=tuple(pb_reference.path),
            target_id=pb_reference.target_id,
        )


@dataclass(frozen=True)
class Valuation:
    """The value expression a feature declares (``= e``, ``default e``, ``:= e``).

    This is KerML's ``Feature::valuation`` read without the expression tree:
    the expression's source text, its kind, the FeatureValue's flags and the
    features it refers to. :attr:`AttributeFacts.value` stays the folded
    constant beside it, so a binding such as ``powerPort.power * 2.0`` is
    visible here even though it has no constant value.
    """

    text: str
    """The expression as written in the model."""

    kind: str = ""
    """``literal``, ``null``, ``featureReference``, ``featureChain``,
    ``operator``, ``invocation``, ``constructor``, ``index``, ``collect``,
    ``select``, ``metadataAccess`` or ``body``."""

    metaclass: str = ""
    """The expression's KerML metaclass, such as ``FeatureChainExpression``."""

    is_default: bool = False
    """The value is a default (``default e``)."""

    is_initial: bool = False
    """The value is an initial value (``:= e``)."""

    references: Tuple[ValuationReference, ...] = field(default_factory=tuple)
    """The features the expression reads, in source order; empty for a literal."""

    evaluated: object = None
    """The expression's value at model level once the features it reads are
    followed to the values they are bound to (``usage_a.prop_a`` with
    ``:>> prop_a = 100.0`` gives ``100.0``); None when that determines none.
    Unlike :attr:`AttributeFacts.value` it is evaluated, not a constant."""

    @classmethod
    def from_pb(cls, pb_valuation) -> "Valuation":
        """Build from a ``Valuation`` protobuf message."""
        evaluated = None
        if pb_valuation.HasField("evaluated"):
            evaluated = value_to_python(pb_valuation.evaluated)
        return cls(
            text=pb_valuation.text,
            kind=pb_valuation.kind,
            metaclass=pb_valuation.metaclass,
            is_default=pb_valuation.is_default,
            is_initial=pb_valuation.is_initial,
            references=tuple(ValuationReference.from_pb(ref) for ref in pb_valuation.references),
            evaluated=evaluated,
        )


@dataclass(frozen=True)
class AttributeFacts:
    """One attribute an element has, own or inherited, as the service resolves it."""

    name: str
    type: str = ""
    """FQN of the resolved type, else the type as written, else the library scalar."""

    value: object = None
    """Default value, when it is a model-level constant; None when there is none."""

    unit: str = ""
    """Unit the default value is written in; empty when it carries none."""

    valuation: Optional[Valuation] = None
    """The value expression as declared, constant or not; None when none is declared."""

    @classmethod
    def from_pb(cls, pb_attribute) -> "AttributeFacts":
        """Build from an ``AttributeInfo`` protobuf message."""
        value = None
        if pb_attribute.HasField("value"):
            value = value_to_python(pb_attribute.value)
        valuation = None
        if pb_attribute.HasField("valuation"):
            valuation = Valuation.from_pb(pb_attribute.valuation)
        return cls(
            name=pb_attribute.name,
            type=pb_attribute.type,
            value=value,
            unit=pb_attribute.unit,
            valuation=valuation,
        )


@dataclass(frozen=True)
class SymbolFacts:
    """Everything code generation needs about one symbol."""

    id: str
    name: str
    kind: str
    type: Optional[TypeFacts] = None
    multiplicity: Optional[Multiplicity] = None
    specializations: Tuple[Specialization, ...] = field(default_factory=tuple)
    attributes: Tuple[AttributeFacts, ...] = field(default_factory=tuple)
    #: Library-inherited attributes ``attributes`` leaves out, counted rather
    #: than silently dropped.
    withheld_library_attributes: int = 0
    #: The value expression the symbol declares, None when it declares none.
    valuation: Optional[Valuation] = None

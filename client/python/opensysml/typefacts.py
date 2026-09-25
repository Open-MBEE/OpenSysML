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
class AttributeFacts:
    """One attribute an element has, own or inherited, as the service resolves it."""

    name: str
    type: str = ""
    """FQN of the resolved type, else the type as written, else the library scalar."""

    value: object = None
    """Default value, when it is a model-level constant; None when there is none."""

    unit: str = ""
    """Unit the default value is written in; empty when it carries none."""

    @classmethod
    def from_pb(cls, pb_attribute) -> "AttributeFacts":
        """Build from an ``AttributeInfo`` protobuf message."""
        value = None
        if pb_attribute.HasField("value"):
            value = value_to_python(pb_attribute.value)
        return cls(
            name=pb_attribute.name,
            type=pb_attribute.type,
            value=value,
            unit=pb_attribute.unit,
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

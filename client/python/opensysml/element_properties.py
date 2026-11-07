"""Reading an element's metamodel properties by their specification names.

The ``GetElementProperties`` RPC answers the KerML 1.1 / SysML 2.1 abstract
syntax of one element — ``inheritedFeature``, ``connectorEnd``,
``directedFeature``, ``payloadType``, ... — as the model's API element form
(``Convert`` to ``api-json``) spells it, without converting the whole model.
An element is named by qualified name, by the positional id of an unnamed
member (``Owner::@2``) or by the element form's own id, which every element
reference in an answer carries, so a client walks the graph with one vocabulary.
"""

from dataclasses import dataclass, field
from typing import Iterator, List, Optional, Sequence, Union

from opensysml.proto import sysml_pb2


@dataclass(frozen=True)
class ElementIdentity:
    """An element the model's element form knows: one of the model, or a
    standard-library element it references.

    Attributes:
        id: The element form's ``@id``, accepted back as the element to read.
        qualified_name: Empty for an unnamed element; an unnamed
            standard-library member carries its library path
            (``owner/Metaclass[i]``).
        metaclass: ``"ConnectionUsage"``, ``"PartDefinition"``, ...; empty when
            the id is known to nothing.
    """

    id: str
    qualified_name: str = ""
    metaclass: str = ""

    def __str__(self):
        return self.qualified_name or self.id


#: One value of a property: an element reference, or a literal as the element
#: form spells it. A number beyond ``int64`` and ``float64`` arrives as its
#: decimal text.
PropertyValue = Union[ElementIdentity, str, bool, int, float]


@dataclass(frozen=True)
class ElementProperty:
    """One metamodel property of an element.

    Attributes:
        name: The specification name (``"connectorEnd"``).
        defining_class: The metaclass declaring the property (``"Connector"``);
            empty when the element's metaclass declares no such property.
        many: Whether the property holds a collection.
        derived: Whether the specification derives the property.
        values: The values in the collection's order; empty for an empty or
            null property.
        unsupported: Why there are no values when the property cannot be
            answered: the metaclass declares no such property, or OpenSysML does
            not derive it yet. Empty for a property that is answered.
    """

    name: str
    defining_class: str = ""
    many: bool = False
    derived: bool = False
    values: List[PropertyValue] = field(default_factory=list)
    unsupported: str = ""

    @property
    def value(self) -> Optional[PropertyValue]:
        """The one value of a single-valued property, ``None`` when it has none."""
        return self.values[0] if self.values else None

    @property
    def elements(self) -> List[ElementIdentity]:
        """The element references among the values, in order."""
        return [v for v in self.values if isinstance(v, ElementIdentity)]

    @property
    def supported(self) -> bool:
        """Whether the property was answered, with or without values."""
        return not self.unsupported

    def __bool__(self):
        return bool(self.values)


class ElementProperties:
    """What one read answered: the element and its properties, in the order
    asked for, or the element form's order when none was named.

    Behaves as a mapping from property name to :class:`ElementProperty`::

        >>> read = model.element_properties("Demo::Pair", "inheritedFeature")
        >>> [str(e) for e in read["inheritedFeature"].elements]
        ['Demo::Unit::p', ...]
    """

    def __init__(self, element: ElementIdentity, properties: Sequence[ElementProperty]):
        self.element = element
        self.properties = list(properties)
        self._by_name = {p.name: p for p in self.properties}

    def __getitem__(self, name: str) -> ElementProperty:
        return self._by_name[name]

    def __contains__(self, name) -> bool:
        return name in self._by_name

    def __iter__(self) -> Iterator[ElementProperty]:
        return iter(self.properties)

    def __len__(self) -> int:
        return len(self.properties)

    def get(self, name: str, default=None):
        """The entry for ``name``, or ``default`` when the read did not ask for it."""
        return self._by_name.get(name, default)

    def __repr__(self):
        return f"ElementProperties({self.element!s}, {[p.name for p in self.properties]})"


def element_identity_of(pb: sysml_pb2.ElementRef) -> ElementIdentity:
    """Decode an ``ElementIdentity`` message."""
    return ElementIdentity(id=pb.id, qualified_name=pb.qualified_name, metaclass=pb.metaclass)


def property_value_of(pb: sysml_pb2.ElementPropertyValue) -> PropertyValue:
    """Decode one ``ElementPropertyValue`` message."""
    arm = pb.WhichOneof("value")
    if arm == "element":
        return element_identity_of(pb.element)
    if arm == "string_value":
        return pb.string_value
    if arm == "bool_value":
        return pb.bool_value
    if arm == "int_value":
        return pb.int_value
    if arm == "real_value":
        return pb.real_value
    if arm == "number":
        return pb.number
    raise ValueError(f"an ElementPropertyValue with no value ({pb})")


def element_properties_of(response: sysml_pb2.GetElementPropertiesResponse) -> ElementProperties:
    """Decode a ``GetElementPropertiesResponse`` the service answered without error."""
    properties = [
        ElementProperty(
            name=p.name,
            defining_class=p.defining_class,
            many=p.many,
            derived=p.derived,
            values=[property_value_of(v) for v in p.values],
            unsupported=p.unsupported,
        )
        for p in response.properties
    ]
    return ElementProperties(element_identity_of(response.element), properties)

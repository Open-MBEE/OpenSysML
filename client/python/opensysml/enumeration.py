"""An enumeration literal as a Python value."""

from dataclasses import dataclass, field


@dataclass(frozen=True)
class EnumLiteral:
    """One literal of an enumeration definition.

    A literal is its own identity, so it arrives as the declaration it names
    rather than as a number or a string: two literals are the same exactly when
    their ``literal_id`` is. It is frozen so it can be a dict key or set member,
    as the same literal in a model is one value.

    ``enumeration_id``, ``name`` and ``value`` describe the literal rather than
    identify it, so they take no part in equality or hashing: a literal named by
    its id alone is the same value as the fully described one the service sends.

    Attributes:
        literal_id: FQN of the literal's declaration (``"D::Color::red"``)
        enumeration_id: FQN of the enumeration declaring it (``"D::Color"``)
        name: The literal as a reader writes it (``"Color::red"``)
        value: The scalar the literal equals when its enumeration specializes a
            scalar type (``3`` for ``high = 3`` in ``enum def Level :> Integer``),
            else ``None``
    """

    literal_id: str
    enumeration_id: str = field(default="", compare=False)
    name: str = field(default="", compare=False)
    value: object = field(default=None, compare=False)

    def __str__(self):
        return self.name or self.literal_id

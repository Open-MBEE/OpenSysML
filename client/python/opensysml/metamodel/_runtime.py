"""Runtime descriptors and errors for generated metamodel classes."""

from __future__ import annotations

import math
from inspect import getattr_static
from types import MappingProxyType
from typing import TYPE_CHECKING, Any, ClassVar, Generic, Mapping, TypeVar, cast, overload

from opensysml.errors import OpenSysMLError

if TYPE_CHECKING:
    from ._reader import ElementGraph

T = TypeVar("T")


def _as_json_list(value: Any) -> list[Any]:
    return cast(list[Any], value)


class _UnresolvedExpectedRange:
    pass


_UNRESOLVED_EXPECTED_RANGE = _UnresolvedExpectedRange()


class MetamodelError(OpenSysMLError):
    """Base class for errors raised while reading metamodel JSON."""


class NotSupplied(MetamodelError):
    """A property is absent from the JSON document."""

    def __init__(self, property: str, key: str, element_id: str, derived: bool) -> None:
        self.property = property
        self.key = key
        self.element_id = element_id
        self.derived = derived
        message = f'document carries no "{key}" for element {element_id}'
        if derived:
            message += "; owned-only exports omit derived properties"
        super().__init__(message)


class UnresolvedReference(MetamodelError):
    """A JSON reference does not resolve to an element in the document."""

    def __init__(
        self,
        id: str | None,
        ref: str | None,
        property: str,
        element_id: str,
    ) -> None:
        self.id = id
        self.ref = ref
        self.property = property
        self.element_id = element_id
        target = f"@id {id!r}" if id is not None else f"@ref {ref!r}"
        super().__init__(f"{property} on element {element_id} refers to unresolved {target}")


class MalformedValue(MetamodelError):
    """A property value does not match its declared metamodel range."""


class MalformedDocument(MetamodelError, ValueError):
    """The JSON document does not contain a valid element graph."""


class UnknownJSONKey(MetamodelError, KeyError):
    """A JSON key is not declared for a metaclass."""

    def __init__(self, metaclass: str, key: str) -> None:
        self.metaclass = metaclass
        self.key = key
        super().__init__(f"{metaclass} has no JSON property {key!r}")


class _ElementBase:
    __slots__ = ("_graph", "_id", "_data", "_type")

    METACLASS: ClassVar[str] = "Element"
    JSON_KEYS: ClassVar[Mapping[str, str]] = MappingProxyType({})

    def __init__(self, graph: ElementGraph, data: Mapping[str, Any]) -> None:
        self._graph = graph
        self._id = cast(str, data["@id"])
        self._data = data
        self._type = cast(str, data["@type"])

    @property
    def json_id(self) -> str:
        return self._id

    @property
    def json_type(self) -> str:
        return self._type

    @property
    def metaclass_name(self) -> str:
        return type(self).METACLASS

    @property
    def graph(self) -> ElementGraph:
        return self._graph

    def json_value(self, key: str) -> Any:
        try:
            return self._data[key]
        except KeyError:
            name = type(self).JSON_KEYS.get(key)
            descriptor = getattr_static(type(self), name, None) if name is not None else None
            derived = descriptor.derived if isinstance(descriptor, _Property) else False
            raise NotSupplied(
                f"{self.metaclass_name}.{key}", key, self.json_id, derived
            ) from None

    @classmethod
    def from_json_key(cls, key: str) -> str:
        try:
            return cls.JSON_KEYS[key]
        except KeyError:
            raise UnknownJSONKey(cls.METACLASS, key) from None

    @classmethod
    def json_key(cls, name: str) -> str:
        if name in cls.JSON_KEYS:
            return name
        for key, snake in cls.JSON_KEYS.items():
            if snake == name:
                return key
        raise UnknownJSONKey(cls.METACLASS, name)

    def __eq__(self, other: object) -> bool:
        if not isinstance(other, _ElementBase):
            return NotImplemented
        return self._graph is other._graph and self._id == other._id

    def __hash__(self) -> int:
        return hash((id(self._graph), self._id))

    def __repr__(self) -> str:
        name = self._data.get("declaredName")
        suffix = f" declaredName={name!r}" if isinstance(name, str) else ""
        return f"<{self.metaclass_name} {self._id!r}{suffix}>"


class _Property(Generic[T]):
    __slots__ = (
        "key",
        "range",
        "ordered",
        "derived",
        "many",
        "owner",
        "_name",
        "_expected_range",
    )

    def __init__(
        self,
        key: str,
        range: str,
        *,
        ordered: bool = False,
        derived: bool = False,
        many: bool,
    ) -> None:
        self.key = key
        self.range = range
        self.ordered = ordered
        self.derived = derived
        self.many = many
        self.owner: type[_ElementBase] | None = None
        self._name: str | None = None
        self._expected_range: (
            type[_ElementBase] | None | _UnresolvedExpectedRange
        ) = _UNRESOLVED_EXPECTED_RANGE

    def __set_name__(self, owner: type[_ElementBase], name: str) -> None:
        if self.owner is None:
            self.owner = owner
            self._name = name

    def __get__(
        self, instance: _ElementBase | None, owner: type[Any] | None = None
    ) -> Any:
        if instance is None:
            return self
        try:
            raw = instance.json_value(self.key)
        except NotSupplied:
            raise NotSupplied(
                self._property(instance), self.key, instance.json_id, self.derived
            ) from None
        if self.many:
            if raw is None:
                return ()
            if not isinstance(raw, list):
                raise self._malformed(instance, f"expected an array, got {raw!r}")
            values: list[T] = []
            for item in _as_json_list(raw):
                if item is None:
                    raise self._malformed(instance, "array contains null")
                values.append(self._convert(item, instance))
            return tuple(values)
        if raw is None:
            return None
        if isinstance(raw, list):
            raise self._malformed(instance, f"expected a scalar, got {raw!r}")
        return self._convert(raw, instance)

    def __set__(self, instance: _ElementBase, value: T) -> None:
        raise AttributeError("metamodel properties are read-only")

    def __delete__(self, instance: _ElementBase) -> None:
        raise AttributeError("metamodel properties are read-only")

    def _convert(self, value: Any, instance: _ElementBase) -> T:
        if not isinstance(value, Mapping):
            return self._primitive(value, instance)
        reference = cast(Mapping[str, Any], value)
        if "@ref" in reference:
            ref = reference["@ref"]
            if not isinstance(ref, str):
                raise self._malformed(instance, f"@ref must be a string, got {ref!r}")
            raise UnresolvedReference(None, ref, self._property(instance), instance.json_id)
        if "@id" in reference:
            element_id = reference["@id"]
            if not isinstance(element_id, str):
                raise self._malformed(instance, f"@id must be a string, got {element_id!r}")
            try:
                target = instance.graph[element_id]
            except KeyError:
                raise UnresolvedReference(
                    element_id, None, self._property(instance), instance.json_id
                ) from None
            if not instance.graph.check_ranges:
                return cast(T, target)
            expected = self._expected_metaclass()
            if expected is not None and not isinstance(target, expected):
                raise self._malformed(
                    instance,
                    f"reference target @id {element_id!r} has json_type "
                    f"{target.json_type!r}; expected {expected.METACLASS}; "
                    "pass supertypes=... for unknown @types",
                )
            return cast(T, target)
        raise self._malformed(instance, f"expected a reference object, got {value!r}")

    def _expected_metaclass(self) -> type[_ElementBase] | None:
        expected = self._expected_range
        if isinstance(expected, _UnresolvedExpectedRange):
            from ._generated import REGISTRY

            expected = cast(type[_ElementBase] | None, REGISTRY.get(self.range))
            self._expected_range = expected
        return expected

    def _primitive(self, value: Any, instance: _ElementBase) -> T:
        if self.range == "bool":
            if type(value) is not bool:
                raise self._malformed(instance, f"expected bool, got {value!r}")
            return cast(T, value)
        if self.range == "str":
            if not isinstance(value, str):
                raise self._malformed(instance, f"expected str, got {value!r}")
            return cast(T, value)
        if self.range == "int":
            if type(value) is not int:
                raise self._malformed(instance, f"expected int, got {value!r}")
            return cast(T, value)
        if self.range == "float":
            if type(value) not in (int, float):
                raise self._malformed(instance, f"expected float, got {value!r}")
            try:
                converted = float(value)
            except OverflowError:
                raise self._malformed(
                    instance, f"expected a finite float, got {value!r}"
                ) from None
            if not math.isfinite(converted):
                raise self._malformed(
                    instance, f"expected a finite float, got {value!r}"
                )
            return cast(T, converted)

        from ._generated import ENUMERATIONS

        enum = ENUMERATIONS.get(self.range)
        if enum is not None:
            try:
                return cast(T, enum(value))
            except (TypeError, ValueError):
                raise self._malformed(
                    instance, f"{value!r} is not a {self.range} literal"
                ) from None
        raise self._malformed(
            instance, f"expected a reference to {self.range}, got {value!r}"
        )

    def _property(self, instance: _ElementBase) -> str:
        return f"{instance.metaclass_name}.{self.key}"

    def _malformed(self, instance: _ElementBase, reason: str) -> MalformedValue:
        return MalformedValue(f"{self._property(instance)} on element {instance.json_id}: {reason}")


class Opt(_Property[T], Generic[T]):
    """A read-only, optional single-valued metamodel property."""

    def __init__(self, key: str, range: str, *, derived: bool = False) -> None:
        super().__init__(key, range, derived=derived, many=False)

    @overload
    def __get__(self, instance: None, owner: type[Any] | None = None) -> Opt[T]: ...

    @overload
    def __get__(
        self, instance: _ElementBase, owner: type[Any] | None = None
    ) -> T | None: ...

    def __get__(
        self, instance: _ElementBase | None, owner: type[Any] | None = None
    ) -> Opt[T] | T | None:
        return cast(Opt[T] | T | None, super().__get__(instance, owner))


class Many(_Property[T], Generic[T]):
    """A read-only, tuple-valued metamodel property."""

    def __init__(
        self,
        key: str,
        range: str,
        *,
        ordered: bool = False,
        derived: bool = False,
    ) -> None:
        super().__init__(key, range, ordered=ordered, derived=derived, many=True)

    @overload
    def __get__(self, instance: None, owner: type[Any] | None = None) -> Many[T]: ...

    @overload
    def __get__(
        self, instance: _ElementBase, owner: type[Any] | None = None
    ) -> tuple[T, ...]: ...

    def __get__(
        self, instance: _ElementBase | None, owner: type[Any] | None = None
    ) -> Many[T] | tuple[T, ...]:
        return cast(Many[T] | tuple[T, ...], super().__get__(instance, owner))

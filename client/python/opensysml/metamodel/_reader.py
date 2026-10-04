"""Read exported API or toolkit JSON into a lazy element graph."""

from __future__ import annotations

import json
import os
import re
from collections.abc import Iterator, Mapping, Sequence
from pathlib import Path
from typing import Any, TypeVar, cast, overload

from ._generated import REGISTRY, Element, Namespace
from ._runtime import MalformedDocument

_E = TypeVar("_E", bound=Element)
_VERSIONED_SYSML = re.compile(r"^https://www\.omg\.org/spec/SysML/\d{8}/")
_SYSML_BASE = "https://www.omg.org/spec/SysML#"


def read_json(
    source: str | os.PathLike[str] | bytes | bytearray | memoryview
    | Sequence[Mapping[str, Any]] | Mapping[str, Any],
    *,
    supertypes: Mapping[str, str] | None = None,
) -> ElementGraph:
    """Read an exported JSON document; strings and paths name files, not JSON text."""
    value = _read_source(source)
    if isinstance(value, Mapping):
        document = cast(Mapping[str, Any], value)
        changes = document.get("change")
        if isinstance(changes, list):
            items = _unwrap_items(cast(Sequence[Any], changes))
        else:
            items = _unwrap_items([document])
    elif isinstance(value, Sequence) and not isinstance(value, (str, bytes, bytearray)):
        items = _unwrap_items(_as_sequence(value))
    else:
        raise MalformedDocument("document must be an object or an array of elements")
    return ElementGraph(items, supertypes=supertypes)


def _read_source(
    source: str | os.PathLike[str] | bytes | bytearray | memoryview
    | Sequence[Mapping[str, Any]] | Mapping[str, Any],
) -> Any:
    if isinstance(source, (str, os.PathLike)):
        data = Path(source).read_bytes()
        return _decode(data)
    if isinstance(source, (bytes, bytearray, memoryview)):
        return _decode(bytes(source))
    return source


def _decode(data: bytes) -> Any:
    try:
        return json.loads(data)
    except (UnicodeDecodeError, json.JSONDecodeError) as error:
        raise MalformedDocument(f"invalid JSON document: {error}") from error


def _as_sequence(value: Any) -> Sequence[Any]:
    return cast(Sequence[Any], value)


def _unwrap_items(items: Sequence[Any]) -> list[dict[str, Any]]:
    result: list[dict[str, Any]] = []
    for item in items:
        if not isinstance(item, Mapping):
            raise MalformedDocument("each document element must be an object")
        document = cast(Mapping[str, Any], item)
        if "identity" in document and "payload" in document:
            payload = document["payload"]
            if payload is None:
                continue
            if not isinstance(payload, Mapping):
                raise MalformedDocument("envelope payload must be an object or null")
            result.append(dict(cast(Mapping[str, Any], payload)))
        else:
            result.append(dict(document))
    return result


class ElementGraph:
    """Indexed document elements with lazy, cached metaclass wrappers."""

    __slots__ = ("_data_by_id", "_order", "_cache", "_supertypes", "_roots")

    def __init__(
        self,
        elements: Sequence[Mapping[str, Any]],
        *,
        supertypes: Mapping[str, str] | None = None,
    ) -> None:
        data_by_id: dict[str, dict[str, Any]] = {}
        order: list[str] = []
        for element in elements:
            element_id = element.get("@id")
            element_type = element.get("@type")
            if not isinstance(element_id, str):
                raise MalformedDocument("element is missing a string @id")
            if not isinstance(element_type, str):
                raise MalformedDocument(f"element {element_id} is missing a string @type")
            if element_id in data_by_id:
                raise MalformedDocument(f"duplicate element @id {element_id!r}")
            data_by_id[element_id] = dict(element)
            order.append(element_id)
        self._data_by_id = data_by_id
        self._order = tuple(order)
        self._cache: dict[str, Element] = {}
        self._supertypes = supertypes or {}
        self._roots: tuple[Namespace, ...] | None = None

    def __getitem__(self, element_id: str) -> Element:
        if element_id not in self._data_by_id:
            raise KeyError(element_id)
        cached = self._cache.get(element_id)
        if cached is not None:
            return cached
        data = self._data_by_id[element_id]
        metaclass = _metaclass(data["@type"], self._supertypes)
        element = metaclass(self, data)
        self._cache[element_id] = element
        return element

    def get(self, element_id: str, default: Any = None) -> Element | Any:
        if element_id not in self._data_by_id:
            return default
        return self[element_id]

    def __contains__(self, element_id: object) -> bool:
        return isinstance(element_id, str) and element_id in self._data_by_id

    def __len__(self) -> int:
        return len(self._order)

    def __iter__(self) -> Iterator[Element]:
        for element_id in self._order:
            yield self[element_id]

    @overload
    def all(self) -> tuple[Element, ...]: ...

    @overload
    def all(self, cls: type[_E]) -> tuple[_E, ...]: ...

    def all(self, cls: type[_E] | None = None) -> tuple[Element, ...] | tuple[_E, ...]:
        elements = tuple(self[element_id] for element_id in self._order)
        if cls is None:
            return elements
        return tuple(element for element in elements if isinstance(element, cls))

    def roots(self) -> tuple[Namespace, ...]:
        if self._roots is not None:
            return self._roots
        referenced: set[str] = set()
        for data in self._data_by_id.values():
            for key in ("ownedRelationship", "ownedRelatedElement"):
                referenced.update(_reference_ids(data.get(key)))
        self._roots = tuple(
            element
            for element in self
            if isinstance(element, Namespace)
            and all(
                self._data_by_id[element.json_id].get(key) is None
                for key in ("owningRelationship", "owner", "owningNamespace")
            )
            and element.json_id not in referenced
        )
        return self._roots


def _metaclass(
    written_type: str, supertypes: Mapping[str, str]
) -> type[Element]:
    name = _normalize_type(written_type)
    seen: set[str] = set()
    while True:
        if name in REGISTRY:
            return REGISTRY[name]
        if name in seen:
            raise MalformedDocument(f"supertype cycle while resolving {written_type!r}")
        seen.add(name)
        parent = supertypes.get(name)
        if parent is None:
            return Element
        name = _normalize_type(parent)


def _normalize_type(written_type: str) -> str:
    if written_type.startswith("sysml:"):
        return written_type[len("sysml:") :]
    if written_type.startswith(_SYSML_BASE):
        return written_type[len(_SYSML_BASE) :]
    return _VERSIONED_SYSML.sub("", written_type, count=1)


def _reference_ids(value: Any) -> set[str]:
    if isinstance(value, Mapping):
        mapping = cast(Mapping[str, Any], value)
        element_id = mapping.get("@id")
        if isinstance(element_id, str):
            return {element_id}
        result: set[str] = set()
        for child in mapping.values():
            result.update(_reference_ids(child))
        return result
    if isinstance(value, list):
        result = set()
        for child in _as_sequence(value):
            result.update(_reference_ids(child))
        return result
    return set()

#!/usr/bin/env python3
"""Rewrite Sphinx roles in Python docstrings as API cross-references."""

from __future__ import annotations

import re
from collections.abc import Callable
from typing import Any

from griffe import Alias, Class, Extension, Module


_ROLE = re.compile(r":(?:py:)?(?P<role>class|meth|func|attr|exc|mod|data|obj):`(?P<short>~?)(?P<name>[^`]+)`")
_FENCE_START = re.compile(r"^[ \t]*(`{3,})([^`]*)$")
_FENCE_END = re.compile(r"^[ \t]*(`{3,})[ \t]*$")


def _rewrite_plain_text(text: str, resolve: Callable[[str, str], str | None]) -> str:
    def replace(match: re.Match[str]) -> str:
        name = match.group("name")
        display = name.rsplit(".", 1)[-1] if match.group("short") else name
        target = resolve(name, match.group("role"))
        if target:
            return f"[`{display}`][{target}]"
        return f"`{display}`"

    return _ROLE.sub(replace, text)


def rewrite_roles(text: str, resolve: Callable[[str, str], str | None]) -> str:
    rewritten: list[str] = []
    plain_text: list[str] = []
    fence_length: int | None = None

    for line in text.splitlines(keepends=True):
        line_content = line.rstrip("\r\n")
        if fence_length is None:
            opening = _FENCE_START.fullmatch(line_content)
            if opening is None:
                plain_text.append(line)
                continue

            rewritten.append(_rewrite_plain_text("".join(plain_text), resolve))
            plain_text.clear()
            rewritten.append(line)
            fence_length = len(opening.group(1))
            continue

        rewritten.append(line)
        closing = _FENCE_END.fullmatch(line_content)
        if closing is not None and len(closing.group(1)) >= fence_length:
            fence_length = None

    rewritten.append(_rewrite_plain_text("".join(plain_text), resolve))
    return "".join(rewritten)


def _resolved_target(obj: Any) -> Any | None:
    while isinstance(obj, Alias):
        if not obj.resolved and not obj.target_path.startswith("opensysml."):
            return None
        try:
            obj = obj.target
        except Exception:
            return None
    return obj


def _lookup_member(package: Module, parts: list[str]) -> Any | None:
    current: Any = package
    for part in parts:
        if isinstance(current, Alias):
            current = _resolved_target(current)
            if current is None:
                return None
        try:
            current = current.get_member(part)
        except Exception:
            return None
        if current is None:
            return None
    return current


def _anchorable_member(member: Any, path: str) -> bool:
    parts = path.split(".")
    nested_parts = parts[2:]
    if any(part.startswith("_") for part in nested_parts):
        return False
    if nested_parts:
        target = _resolved_target(member)
        return target is not None and getattr(target, "docstring", None) is not None
    return True


def _exported_paths(package: Module, export_names: list[str]) -> dict[str, str]:
    paths: dict[str, str] = {}
    visited: set[tuple[int, str]] = set()
    active: set[int] = set()

    def visit(obj: Any, path: str) -> None:
        target = _resolved_target(obj)
        if target is None or not _anchorable_member(target, path):
            return

        identity = id(target)
        visit_key = (identity, path)
        if visit_key in visited:
            return
        visited.add(visit_key)

        canonical_path = getattr(target, "path", None)
        if isinstance(canonical_path, str) and canonical_path.startswith("opensysml."):
            paths.setdefault(canonical_path, path)

        if identity in active:
            return
        active.add(identity)
        try:
            for name, member in getattr(target, "members", {}).items():
                visit(member, f"{path}.{name}")
        finally:
            active.remove(identity)

    for name in export_names:
        try:
            member = package.get_member(name)
        except Exception:
            continue
        visit(member, f"opensysml.{name}")
    return paths


def _resolve_reference(
    package: Module,
    exports: set[str],
    name: str,
    owning_class: str | None,
    exported_paths: dict[str, str],
) -> str | None:
    candidates: list[str] = []
    if name.startswith("opensysml."):
        candidates.append(name)
    candidates.append(f"opensysml.{name}")
    if "." not in name and owning_class is not None:
        candidates.append(f"{owning_class}.{name}")

    seen: set[str] = set()
    for candidate in candidates:
        if candidate in seen:
            continue
        seen.add(candidate)
        parts = candidate.split(".")
        if len(parts) < 2 or parts[0] != "opensysml":
            continue
        if parts[1] in exports:
            member_parts = parts[1:]
            member = _lookup_member(package, member_parts)
            if member is not None and _anchorable_member(member, candidate):
                return candidate

        exported_path = exported_paths.get(candidate)
        if exported_path is not None:
            return exported_path
    return None


class SphinxRolesExtension(Extension):
    def on_package(self, *, pkg: Module, **_: Any) -> None:
        if pkg.path != "opensysml":
            return

        export_names = list(pkg.exports or ())
        exports = set(export_names)
        exported_paths = _exported_paths(pkg, export_names)
        rewritten_docstrings: set[int] = set()
        visited: set[tuple[int, str]] = set()
        active: set[int] = set()

        def visit(obj: Any, path: str, owning_class: str | None = None) -> None:
            target = _resolved_target(obj)
            if target is None:
                return

            identity = id(target)
            visit_key = (identity, path)
            if visit_key in visited:
                return
            visited.add(visit_key)

            class_path = path if isinstance(target, Class) else owning_class
            docstring = getattr(target, "docstring", None)
            if docstring is not None and id(docstring) not in rewritten_docstrings:
                docstring.value = rewrite_roles(
                    docstring.value,
                    lambda name, role: _resolve_reference(pkg, exports, name, class_path, exported_paths),
                )
                rewritten_docstrings.add(id(docstring))

            if identity in active:
                return
            active.add(identity)
            try:
                for name, member in getattr(target, "members", {}).items():
                    visit(member, f"{path}.{name}", class_path)
            finally:
                active.remove(identity)

        for name in export_names:
            try:
                member = pkg.get_member(name)
            except Exception:
                continue
            visit(member, f"opensysml.{name}")

        for name, member in pkg.members.items():
            if name not in exports:
                visit(member, f"opensysml.{name}")

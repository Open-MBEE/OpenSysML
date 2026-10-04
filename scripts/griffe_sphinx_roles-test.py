#!/usr/bin/env python3
"""Tests for scripts/griffe_sphinx_roles.py."""

import importlib.util
import pathlib
import sys
import unittest

HERE = pathlib.Path(__file__).resolve().parent

if importlib.util.find_spec("griffe") is None:
    roles = None
else:
    spec = importlib.util.spec_from_file_location("griffe_sphinx_roles", HERE / "griffe_sphinx_roles.py")
    roles = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(roles)


@unittest.skipIf(roles is None, "Griffe is not importable; skipping Sphinx-role extension tests")
class RewriteRolesTests(unittest.TestCase):
    def test_resolved_class_becomes_autoref(self):
        result = roles.rewrite_roles(
            "See :class:`Thing`.",
            lambda name, role: "opensysml.Thing",
        )

        self.assertEqual(result, "See [`Thing`][opensysml.Thing].")

    def test_short_name_displays_final_component(self):
        result = roles.rewrite_roles(
            "See :class:`~opensysml.models.Thing`.",
            lambda name, role: "opensysml.Thing",
        )

        self.assertEqual(result, "See [`Thing`][opensysml.Thing].")

    def test_dotted_method_role_resolves(self):
        result = roles.rewrite_roles(
            "Call :meth:`Connection.parse_sources`.",
            lambda name, role: "opensysml.Connection.parse_sources" if role == "meth" else None,
        )

        self.assertEqual(result, "Call [`Connection.parse_sources`][opensysml.Connection.parse_sources].")

    def test_labeled_role_resolves_target_and_records_it(self):
        target = "opensysml.connection.Connection.parse_sources"
        calls = []

        def resolve(name, role):
            calls.append((name, role))
            if name == target and role == "meth":
                return "opensysml.Connection.parse_sources"
            return None

        result = roles.rewrite_roles(
            ":meth:`Connection.parse_sources <opensysml.connection.Connection.parse_sources>`",
            resolve,
        )

        self.assertEqual(result, "[`Connection.parse_sources`][opensysml.Connection.parse_sources]")
        self.assertEqual(calls, [(target, "meth")])

    def test_unresolved_labeled_role_displays_its_label(self):
        result = roles.rewrite_roles(
            ":class:`label <opensysml.private.Missing>`",
            lambda name, role: None,
        )

        self.assertEqual(result, "`label`")

    def test_labeled_role_keeps_explicit_label_with_short_prefix(self):
        result = roles.rewrite_roles(
            ":class:`~package.LongName <opensysml.LongName>`",
            lambda name, role: "opensysml.LongName",
        )

        self.assertEqual(result, "[`package.LongName`][opensysml.LongName]")

    def test_extension_resolves_bare_methods_before_top_level_functions(self):
        from griffe import temporary_visited_package

        source = '''\
__all__ = ["Conn", "load"]

def load():
    """Load from the package."""

class Conn:
    """A connection."""

    def load(self):
        """Load this connection."""

    def convert(self):
        """See :meth:`load` and :func:`load`."""
        '''
        with temporary_visited_package("opensysml", modules={"__init__.py": source}) as package:
            roles.SphinxRolesExtension().on_package(pkg=package)
            convert = package.get_member("Conn").get_member("convert")
            self.assertEqual(
                convert.docstring.value,
                "See [`load`][opensysml.Conn.load] and [`load`][opensysml.load].",
            )

    def test_extension_resolves_labeled_canonical_submodule_target_to_export(self):
        from griffe import temporary_visited_package

        modules = {
            "__init__.py": '''\
from .connection import Connection
__all__ = ["Connection"]
''',
            "connection.py": '''\
class Connection:
    """A connection."""

    def parse_sources(self):
        """See :meth:`Connection.parse_sources <opensysml.connection.Connection.parse_sources>`."""
''',
        }
        with temporary_visited_package("opensysml", modules=modules) as package:
            roles.SphinxRolesExtension().on_package(pkg=package)
            connection = package.get_member("Connection").target
            method = connection.get_member("parse_sources")
            self.assertEqual(
                method.docstring.value,
                "See [`Connection.parse_sources`][opensysml.Connection.parse_sources].",
            )

    def test_unresolved_name_becomes_code_span(self):
        result = roles.rewrite_roles(":class:`int`", lambda name, role: None)

        self.assertEqual(result, "`int`")

    def test_python_prefixed_role_resolves(self):
        result = roles.rewrite_roles(
            "See :py:class:`Thing`.",
            lambda name, role: "opensysml.Thing" if role == "class" else None,
        )

        self.assertEqual(result, "See [`Thing`][opensysml.Thing].")

    def test_roles_inside_fenced_code_are_unchanged(self):
        text = "Example:\n```\n:class:`Thing`\n```\nSee :class:`Thing`."
        result = roles.rewrite_roles(text, lambda name, role: "opensysml.Thing")

        self.assertEqual(result, "Example:\n```\n:class:`Thing`\n```\nSee [`Thing`][opensysml.Thing].")

    def test_text_without_roles_is_unchanged(self):
        result = roles.rewrite_roles("Plain text.", lambda name, role: self.fail("resolver should not be called"))

        self.assertEqual(result, "Plain text.")

    def test_two_roles_on_one_line_are_rewritten(self):
        result = roles.rewrite_roles(
            ":class:`Left` and :exc:`Right`.",
            lambda name, role: f"opensysml.{name}",
        )

        self.assertEqual(result, "[`Left`][opensysml.Left] and [`Right`][opensysml.Right].")


if __name__ == "__main__":
    sys.exit(unittest.main(verbosity=1).result.wasSuccessful() is False)

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

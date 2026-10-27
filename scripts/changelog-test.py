#!/usr/bin/env python3
"""Tests for scripts/changelog.py. Run: python3 scripts/changelog-test.py"""

from __future__ import annotations

import contextlib
import importlib.util
import io
import re
import pathlib
import sys
import tempfile
import unittest

HERE = pathlib.Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("changelog", HERE / "changelog.py")
changelog = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(changelog)

BASE = """# Changelog

Intro.

## Unreleased

### Added

- **Old added entry.** Text.

### Fixed

- **Old fixed entry.** Text.

## 0.4.3 — 2026-09-01

### Added

- Released thing.
"""

# BASE with an older release behind the latest one; both carry "- Shared line." under Fixed.
TWO_RELEASES = BASE + """
### Fixed

- Shared line.

## 0.4.2 — 2026-08-01

### Added

- Older released thing.

### Fixed

- Shared line.
"""


FIXED_FRAGMENT = "a.fixed.md"


class FoldTest(unittest.TestCase):
    def test_appends_to_existing_section_and_creates_missing_in_order(self):
        out = changelog.fold(
            BASE,
            {
                "Added": ["- **New added.** A."],
                "Changed": ["- **New changed.** C."],
                "Performance": ["- **Faster.** P."],
            },
        )
        unreleased = out[out.index("## Unreleased") : out.index("## 0.4.3")]
        order = [l for l in unreleased.splitlines() if l.startswith("### ")]
        self.assertEqual(order, ["### Added", "### Changed", "### Fixed", "### Performance"])
        self.assertIn("- **Old added entry.** Text.\n\n- **New added.** A.\n\n### Changed", unreleased)
        self.assertIn("### Changed\n\n- **New changed.** C.\n\n### Fixed", unreleased)
        self.assertTrue(unreleased.endswith("- **Faster.** P.\n\n"))
        # Released sections are untouched.
        self.assertTrue(out.endswith("## 0.4.3 — 2026-09-01\n\n### Added\n\n- Released thing.\n"))

    def test_empty_unreleased(self):
        text = "# Changelog\n\n## Unreleased\n\n## 0.1.0 — 2026-01-01\n\n- x.\n"
        out = changelog.fold(text, {"Fixed": ["- **F.** f."]})
        self.assertEqual(
            out, "# Changelog\n\n## Unreleased\n\n### Fixed\n\n- **F.** f.\n\n## 0.1.0 — 2026-01-01\n\n- x.\n"
        )

    def test_unreleased_last_in_file(self):
        text = "# Changelog\n\n## Unreleased\n"
        out = changelog.fold(text, {"Added": ["- a."]})
        self.assertEqual(out, "# Changelog\n\n## Unreleased\n\n### Added\n\n- a.\n\n")

    def test_new_section_lands_after_its_canonical_predecessors(self):
        # The real file's Unreleased section is not in canonical order; existing
        # sections stay put and a new one goes after the last that precedes it.
        text = "## Unreleased\n\n### Added\n\n- a.\n\n### Performance\n\n- p.\n\n### Fixed\n\n- f.\n\n### Changed\n\n- c.\n"
        out = changelog.fold(text, {"Security": ["- s."], "Deprecated": ["- d."]})
        order = [l for l in out.splitlines() if l.startswith("### ")]
        self.assertEqual(
            order, ["### Added", "### Performance", "### Fixed", "### Changed", "### Deprecated", "### Security"]
        )


class RenderTest(unittest.TestCase):
    def setUp(self):
        d = pathlib.Path(tempfile.mkdtemp())
        self.saved = (changelog.CHANGELOG, changelog.FRAGMENTS)
        changelog.CHANGELOG = d / "CHANGELOG.md"
        changelog.FRAGMENTS = d / "changes" / "unreleased"
        changelog.FRAGMENTS.mkdir(parents=True)
        changelog.CHANGELOG.write_text(BASE, encoding="utf-8")

    def tearDown(self):
        changelog.CHANGELOG, changelog.FRAGMENTS = self.saved

    def _notice(self, name: str, section: str, heading: str) -> str:
        return f"{changelog.FRAGMENTS / name}: not folded, '### {section}' of '{heading}' already carries it\n"

    def _render(self) -> str:
        """Run render with stdout swallowed; return what it wrote to stderr."""
        err = io.StringIO()
        with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(err):
            changelog.render()
        return err.getvalue()

    def test_rerun_after_interrupted_render_does_not_duplicate(self):
        (changelog.FRAGMENTS / "a.added.md").write_text("- **A.** a.\n", encoding="utf-8")
        (changelog.FRAGMENTS / "b.fixed.md").write_text("- **B.** b.\n\n- **B2.** b2.\n", encoding="utf-8")
        self.assertEqual(self._render(), "")
        once = changelog.CHANGELOG.read_text(encoding="utf-8")
        self.assertEqual(sorted(p.name for p in changelog.FRAGMENTS.iterdir()), [])
        # Simulate a run that wrote the changelog but died before deleting a fragment.
        (changelog.FRAGMENTS / "b.fixed.md").write_text("- **B.** b.\n\n- **B2.** b2.\n", encoding="utf-8")
        err = self._render()
        self.assertEqual(changelog.CHANGELOG.read_text(encoding="utf-8"), once)
        self.assertEqual(sorted(p.name for p in changelog.FRAGMENTS.iterdir()), [])
        self.assertEqual(err, self._notice("b.fixed.md", "Fixed", "## Unreleased"))

    def test_entry_the_latest_release_already_carries_is_not_folded_again(self):
        # BASE's 0.4.3 already lists "- Released thing." under Added; a fragment that outlived
        # its release is only deleted, and the skip is reported.
        (changelog.FRAGMENTS / "a.added.md").write_text("- Released thing.\n", encoding="utf-8")
        (changelog.FRAGMENTS / FIXED_FRAGMENT).write_text("- Released thing.\n", encoding="utf-8")
        err = self._render()
        out = changelog.CHANGELOG.read_text(encoding="utf-8")
        self.assertEqual(out.count("- Released thing."), 2)
        self.assertIn("### Fixed\n\n- **Old fixed entry.** Text.\n\n- Released thing.\n\n## 0.4.3", out)
        self.assertEqual(sorted(p.name for p in changelog.FRAGMENTS.iterdir()), [])
        self.assertEqual(err, self._notice("a.added.md", "Added", "## 0.4.3 \u2014 2026-09-01"))

    def test_entry_only_an_older_release_carries_is_folded(self):
        # 0.4.2 lists "- Older released thing." under Added, but 0.4.3 does not: no live fragment
        # can date from before the latest release, so this is a new entry that repeats an old line.
        changelog.CHANGELOG.write_text(TWO_RELEASES, encoding="utf-8")
        (changelog.FRAGMENTS / "a.added.md").write_text("- Older released thing.\n", encoding="utf-8")
        (changelog.FRAGMENTS / FIXED_FRAGMENT).write_text("- Shared line.\n", encoding="utf-8")
        err = self._render()
        out = changelog.CHANGELOG.read_text(encoding="utf-8")
        self.assertIn("- **Old added entry.** Text.\n\n- Older released thing.\n\n### Fixed", out)
        self.assertEqual(out.count("- Older released thing."), 2)
        self.assertEqual(out.count("- Shared line."), 2)
        self.assertEqual(err, self._notice(FIXED_FRAGMENT, "Fixed", "## 0.4.3 \u2014 2026-09-01"))
        self.assertEqual(sorted(p.name for p in changelog.FRAGMENTS.iterdir()), [])

    def test_render_without_duplicates_is_a_plain_fold(self):
        changelog.CHANGELOG.write_text(TWO_RELEASES, encoding="utf-8")
        (changelog.FRAGMENTS / "a.added.md").write_text("- **A.** a.\n", encoding="utf-8")
        (changelog.FRAGMENTS / "b.changed.md").write_text("- **C.** c.\n", encoding="utf-8")
        self.assertEqual(self._render(), "")
        expected = changelog.fold(TWO_RELEASES, {"Added": ["- **A.** a."], "Changed": ["- **C.** c."]})
        self.assertEqual(changelog.CHANGELOG.read_text(encoding="utf-8"), expected)

    def test_already_folded_names_the_section_it_searched(self):
        found = changelog._already_folded
        self.assertEqual(found(TWO_RELEASES, "Added", "- **Old added entry.** Text."), "## Unreleased")
        self.assertEqual(found(TWO_RELEASES, "Added", "- Released thing."), "## 0.4.3 \u2014 2026-09-01")
        self.assertEqual(found(TWO_RELEASES, "Fixed", "- Shared line."), "## 0.4.3 \u2014 2026-09-01")
        self.assertIsNone(found(TWO_RELEASES, "Added", "- Older released thing."))
        self.assertIsNone(found(TWO_RELEASES, "Fixed", "- Released thing."))
        self.assertIsNone(found("# Changelog\n\n## Unreleased\n", "Added", "- a."))

    def test_check_reports_a_fragment_a_fold_would_skip_without_failing(self):
        (changelog.FRAGMENTS / "a.added.md").write_text("- Released thing.\n", encoding="utf-8")
        (changelog.FRAGMENTS / "b.added.md").write_text("- Brand new.\n", encoding="utf-8")
        err = io.StringIO()
        with contextlib.redirect_stderr(err):
            self.assertEqual(changelog.check(), 0)
        self.assertEqual(err.getvalue(), self._notice("a.added.md", "Added", "## 0.4.3 \u2014 2026-09-01"))
        (changelog.FRAGMENTS / "c.fixed.md").write_text("Plain prose.\n", encoding="utf-8")
        with contextlib.redirect_stderr(io.StringIO()):
            self.assertEqual(changelog.check(), 1)

    def test_same_text_in_another_section_or_inside_a_longer_entry_is_still_folded(self):
        # BASE already has "- **Old added entry.** Text." under Added only.
        (changelog.FRAGMENTS / FIXED_FRAGMENT).write_text("- **Old added entry.** Text.\n", encoding="utf-8")
        (changelog.FRAGMENTS / "b.added.md").write_text("- **Old added entry.**\n", encoding="utf-8")
        with contextlib.redirect_stdout(io.StringIO()):
            changelog.render()
        out = changelog.CHANGELOG.read_text(encoding="utf-8")
        self.assertIn("### Added\n\n- **Old added entry.** Text.\n\n- **Old added entry.**\n\n### Fixed", out)
        self.assertIn("### Fixed\n\n- **Old fixed entry.** Text.\n\n- **Old added entry.** Text.\n\n## 0.4.3", out)

    def test_release_rejects_a_malformed_version_or_date(self):
        with self.assertRaises(SystemExit):
            changelog.release("0.5", None)
        with self.assertRaises(SystemExit):
            changelog.release("0.5.0", "2026-13-01")
        self.assertEqual(changelog.CHANGELOG.read_text(encoding="utf-8"), BASE)

    def test_version_follows_semver(self):
        for ok in ("1.0.0", "0.5.0-rc.1", "1.0.0+build.7", "1.0.0-alpha-1.2+sha.5114f85"):
            self.assertIsNotNone(changelog.VERSION.match(ok), ok)
        for bad in ("1.0", "01.0.0", "1.0.0-", "1.0.0-rc..1", "1.0.0-01", "1.0.0+", "1.0.0 "):
            self.assertIsNone(changelog.VERSION.match(bad), bad)

    def test_summary_lists_one_lead_per_entry_fragments_included(self):
        (changelog.FRAGMENTS / "a.added.md").write_text(
            "- **A lead that wraps\n  onto a second line.** The rest, which is not quoted.\n"
            "  - a nested item\n\n- No bold lead. Second sentence.\n",
            encoding="utf-8",
        )
        (changelog.FRAGMENTS / "b.changed.md").write_text("- `FLAG` is\n  now honored\n", encoding="utf-8")
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            changelog.summary()
        self.assertEqual(
            out.getvalue(),
            "### Added\n\n"
            "- Old added entry.\n"
            "- A lead that wraps onto a second line.\n"
            "- No bold lead.\n\n"
            "### Changed\n\n"
            "- `FLAG` is now honored\n\n"
            "### Fixed\n\n"
            "- Old fixed entry.\n",
        )
        self.assertEqual(changelog.CHANGELOG.read_text(encoding="utf-8"), BASE)
        self.assertEqual(sorted(p.name for p in changelog.FRAGMENTS.iterdir()), ["a.added.md", "b.changed.md"])

    def test_release_heads_the_unreleased_entries_with_the_version(self):
        with contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(changelog.release("v0.5.0", "2026-09-04"), 0)
        out = changelog.CHANGELOG.read_text(encoding="utf-8")
        self.assertIn("## Unreleased\n\n## 0.5.0 \u2014 2026-09-04\n", out)


class FragmentTest(unittest.TestCase):
    def _frag(self, name: str, body: str) -> pathlib.Path:
        d = pathlib.Path(tempfile.mkdtemp())
        p = d / name
        p.write_text(body, encoding="utf-8")
        return p

    def test_valid(self):
        p = self._frag("repl-send.added.md", "- **X.** y.\n  more.\n")
        self.assertEqual(changelog.parse_fragment(p), ("Added", "- **X.** y.\n  more."))

    def test_bad_section(self):
        p = self._frag("repl-send.new.md", "- x")
        with self.assertRaises(changelog.FragmentError):
            changelog.parse_fragment(p)

    def test_bad_name(self):
        p = self._frag("repl-send.md", "- x")
        with self.assertRaises(changelog.FragmentError):
            changelog.parse_fragment(p)

    def test_empty(self):
        p = self._frag(FIXED_FRAGMENT, "\n\n")
        with self.assertRaises(changelog.FragmentError):
            changelog.parse_fragment(p)

    def test_heading_rejected(self):
        p = self._frag(FIXED_FRAGMENT, "### Fixed\n\n- x")
        with self.assertRaises(changelog.FragmentError):
            changelog.parse_fragment(p)

    def test_not_a_list_item(self):
        p = self._frag(FIXED_FRAGMENT, "Plain prose.")
        with self.assertRaises(changelog.FragmentError):
            changelog.parse_fragment(p)


class ReleaseNotesTest(unittest.TestCase):
    def setUp(self):
        d = pathlib.Path(tempfile.mkdtemp())
        self.saved = (changelog.CHANGELOG, changelog.SECURITY, changelog.MKDOCS)
        changelog.CHANGELOG = d / "CHANGELOG.md"
        changelog.SECURITY = d / "SECURITY.md"
        changelog.MKDOCS = d / "mkdocs.yml"
        changelog.CHANGELOG.write_text(TWO_RELEASES, encoding="utf-8")
        changelog.SECURITY.write_text(
            "# Security policy\n\n## Supported versions\n\n<!-- support-statement:begin -->\n"
            "Each line is supported\nuntil the next.\n<!-- support-statement:end -->\n\n## Scope\n",
            encoding="utf-8",
        )
        changelog.MKDOCS.write_text(
            "site_name: t\nsite_url: https://docs.example.org/\nrepo_url: https://github.com/Acme/Tool\n",
            encoding="utf-8",
        )

    def tearDown(self):
        changelog.CHANGELOG, changelog.SECURITY, changelog.MKDOCS = self.saved

    def test_support_statement_is_one_line_from_the_markers(self):
        self.assertEqual(changelog.support_statement(), "Each line is supported until the next.")

    def test_missing_markers_fail(self):
        changelog.SECURITY.write_text("# Security policy\n", encoding="utf-8")
        with self.assertRaises(SystemExit):
            changelog.support_statement()

    def test_footer_alone(self):
        out = changelog.release_notes(None)
        self.assertEqual(
            out,
            "**Supported versions.** Each line is supported until the next. How to report a"
            " vulnerability, and what to expect, is in"
            " [SECURITY.md](https://github.com/Acme/Tool/security/policy).\n\n"
            "**Verify your download.** Check `SHA256SUMS.txt` and its cosign signature before running"
            " anything: [https://docs.example.org/guide/security/#verifying-a-release]"
            "(https://docs.example.org/guide/security/#verifying-a-release).\n",
        )

    def test_a_version_prepends_its_section_only(self):
        out = changelog.release_notes("v0.4.3")
        self.assertTrue(out.startswith("### Added\n\n- Released thing.\n\n### Fixed\n\n- Shared line.\n\n**Supported"))
        self.assertNotIn("Older released thing", out)
        self.assertNotIn("Old added entry", out)

    def test_an_unknown_version_fails(self):
        with self.assertRaises(SystemExit):
            changelog.release_notes("9.9.9")
        with self.assertRaises(SystemExit):
            changelog.release_notes("not-a-version")

    def test_main_prints_the_notes(self):
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            self.assertEqual(changelog.main(["release-notes", "0.4.2"]), 0)
        self.assertTrue(buf.getvalue().startswith("### Added\n\n- Older released thing.\n"))
        self.assertIn("**Verify your download.**", buf.getvalue())

    def _long_release(self, entries: int, detail: int) -> None:
        items = "".join(f"- **Entry {i} lands.** {'x' * detail}\n" for i in range(entries))
        changelog.CHANGELOG.write_text(f"{BASE}\n## 0.5.0 — 2026-09-01\n\n### Added\n\n{items}", encoding="utf-8")

    def test_a_section_over_budget_becomes_one_line_per_entry(self):
        self._long_release(entries=2000, detail=80)
        self.assertGreater(len(changelog.version_section("0.5.0")), changelog.SECTION_BUDGET)
        out = changelog.release_notes("0.5.0")
        self.assertLessEqual(len(out), 125_000)
        self.assertIn("[`CHANGELOG.md`](https://github.com/Acme/Tool/blob/v0.5.0/CHANGELOG.md) at this tag", out)
        self.assertIn("\n- Entry 1999 lands.\n", out)
        self.assertNotIn("xxx", out)
        self.assertNotIn("more entries", out)
        self.assertTrue(out.endswith("#verifying-a-release).\n"))

    def test_an_outline_over_budget_is_cut_at_an_entry(self):
        self._long_release(entries=6000, detail=0)
        section = changelog.within_budget(changelog.version_section("0.5.0"), "0.5.0", "https://github.com/Acme/Tool")
        self.assertLessEqual(len(section), changelog.SECTION_BUDGET)
        out = changelog.release_notes("0.5.0")
        self.assertLessEqual(len(out), 125_000)
        kept = sum(1 for line in out.splitlines() if line.startswith("- Entry"))
        self.assertIn(f"… and {6000 - kept} more entries, in [`CHANGELOG.md`]", out)
        self.assertRegex(out, r"- Entry \d+ lands\.\n\n… and")
        self.assertIn("**Supported versions.**", out)

    def test_the_sections_already_released_fit_a_release_body(self):
        changelog.CHANGELOG = self.saved[0]
        text = changelog.CHANGELOG.read_text(encoding="utf-8")
        for version in re.findall(r"^## (\d+\.\d+\.\d+)", text, re.MULTILINE):
            with self.subTest(version=version):
                self.assertLessEqual(len(changelog.release_notes(version)), 125_000)


if __name__ == "__main__":
    sys.exit(unittest.main())

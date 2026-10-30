#!/usr/bin/env python3
"""Tests for scripts/release-downloads.py. Run: python3 scripts/release-downloads-test.py"""

from __future__ import annotations

import contextlib
import importlib.util
import io
import json
import pathlib
import unittest
import unittest.mock

HERE = pathlib.Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("release_downloads", HERE / "release-downloads.py")
downloads = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(downloads)


def release(tag, published, assets, prerelease=False, draft=False, name=None):
    return {
        "tag_name": tag,
        "name": name,
        "published_at": published,
        "prerelease": prerelease,
        "draft": draft,
        "assets": [{"name": n, "download_count": c, "size": 10} for n, c in assets],
    }


RELEASES = [
    release("nightly", "2026-10-28T10:09:38Z", [("sysml-linux-amd64.tar.gz", 2)], prerelease=True),
    release("v0.11.0", "2026-10-24T06:59:12Z",
            [("sysml-linux-amd64.tar.gz", 85), ("SHA256SUMS.txt", 94), ("sysml-darwin-arm64.tar.gz", 0)]),
    release("v0.10.1", "2026-10-16T02:28:52Z", [("sysml-linux-amd64.tar.gz", 1600), ("SHA256SUMS.txt", 43)]),
    release("v0.0.1", "2026-08-05T19:32:51Z", [], draft=True),
]


class FakeResponse(io.BytesIO):
    def __init__(self, body, link=""):
        super().__init__(json.dumps(body).encode("utf-8"))
        self.headers = {"Link": link}

    def __enter__(self):
        return self

    def __exit__(self, *exc):
        self.close()
        return False


def paged(pages):
    """A urlopen stand-in serving the pages in order, with a `next` link on all but the last."""
    seen = []

    def urlopen(request, timeout=0):
        seen.append(request)
        index = len(seen) - 1
        link = ""
        if index < len(pages) - 1:
            link = f'<https://api.github.com/page/{index + 2}>; rel="next", <https://x>; rel="last"'
        return FakeResponse(pages[index], link)

    return urlopen, seen


class FetchTest(unittest.TestCase):
    def test_follows_pagination_and_sends_token(self):
        urlopen, seen = paged([RELEASES[:2], RELEASES[2:]])
        with unittest.mock.patch("urllib.request.urlopen", urlopen):
            releases = downloads.fetch_releases("o/r", "tok")
        self.assertEqual([r["tag_name"] for r in releases], ["nightly", "v0.11.0", "v0.10.1", "v0.0.1"])
        self.assertEqual(seen[0].full_url, "https://api.github.com/repos/o/r/releases?per_page=100")
        self.assertEqual(seen[1].full_url, "https://api.github.com/page/2")
        self.assertEqual(seen[0].get_header("Authorization"), "Bearer tok")

    def test_no_token_means_no_authorization_header(self):
        urlopen, seen = paged([[]])
        with unittest.mock.patch("urllib.request.urlopen", urlopen):
            self.assertEqual(downloads.fetch_releases("o/r"), [])
        self.assertFalse(seen[0].has_header("Authorization"))

    def test_traffic_reads_clones_then_views(self):
        urlopen, seen = paged([{"count": 3, "uniques": 2, "clones": []}, {"count": 9, "uniques": 4, "views": []}])
        with unittest.mock.patch("urllib.request.urlopen", urlopen):
            traffic = downloads.fetch_traffic("o/r", "tok")
        self.assertEqual([r.full_url.rsplit("/", 1)[1] for r in seen], ["clones", "views"])
        self.assertEqual(traffic["clones"]["count"], 3)
        self.assertEqual(traffic["views"]["uniques"], 4)


class SummarizeTest(unittest.TestCase):
    def test_sums_assets_and_orders_newest_first(self):
        rows = downloads.summarize(list(reversed(RELEASES)))
        self.assertEqual([r["tag"] for r in rows], ["nightly", "v0.11.0", "v0.10.1", "v0.0.1"])
        self.assertEqual([r["downloads"] for r in rows], [2, 179, 1643, 0])
        self.assertEqual(rows[1]["published"], "2026-10-24")
        self.assertEqual(rows[1]["name"], "")

    def test_stable_drops_prereleases_and_drafts(self):
        rows = downloads.summarize(RELEASES, stable_only=True)
        self.assertEqual([r["tag"] for r in rows], ["v0.11.0", "v0.10.1"])

    def test_totals_by_asset_most_downloaded_first(self):
        totals = downloads.totals_by_asset(downloads.summarize(RELEASES))
        self.assertEqual(
            list(totals.items()),
            [
                ("sysml-linux-amd64.tar.gz", {"downloads": 1687, "releases": 3}),
                ("SHA256SUMS.txt", {"downloads": 137, "releases": 2}),
                ("sysml-darwin-arm64.tar.gz", {"downloads": 0, "releases": 1}),
            ],
        )


class RenderTest(unittest.TestCase):
    def test_table_marks_flags_and_totals(self):
        text = downloads.render_table(downloads.summarize(RELEASES))
        lines = text.splitlines()
        self.assertEqual(lines[0], "Release  Published   Assets  Downloads")
        self.assertEqual(lines[1], "nightly  2026-10-28       1          2  pre-release")
        self.assertEqual(lines[3], "v0.10.1  2026-10-16       2      1,643")
        self.assertEqual(lines[4], "v0.0.1   2026-08-05       0          0  draft")
        self.assertEqual(lines[-1], "4 releases, 1,824 downloads")

    def test_table_lists_assets_most_downloaded_first(self):
        text = downloads.render_table(downloads.summarize(RELEASES, stable_only=True), with_assets=True)
        lines = text.splitlines()
        self.assertEqual(lines[2], "           94  SHA256SUMS.txt")
        self.assertEqual(lines[3], "           85  sysml-linux-amd64.tar.gz")
        self.assertEqual(lines[4], "            0  sysml-darwin-arm64.tar.gz")

    def test_empty_inputs(self):
        self.assertEqual(downloads.render_table([]), "No releases.\n")
        self.assertEqual(downloads.render_by_asset([]), "No assets.\n")

    def test_by_asset(self):
        lines = downloads.render_by_asset(downloads.summarize(RELEASES)).splitlines()
        self.assertEqual(lines[1], "    1,687         3  sysml-linux-amd64.tar.gz")
        self.assertEqual(lines[-1], "3 asset names, 1,824 downloads")

    def test_traffic_joins_days_and_totals(self):
        traffic = {
            "clones": {"count": 7, "uniques": 3, "clones": [
                {"timestamp": "2026-10-27T00:00:00Z", "count": 5, "uniques": 2},
                {"timestamp": "2026-10-28T00:00:00Z", "count": 2, "uniques": 1},
            ]},
            "views": {"count": 40, "uniques": 10, "views": [
                {"timestamp": "2026-10-28T00:00:00Z", "count": 40, "uniques": 10},
            ]},
        }
        lines = downloads.render_traffic(traffic).splitlines()
        self.assertEqual(lines[0], "Day         Clones  Unique   Views  Unique")
        self.assertEqual(lines[1], "2026-10-27       5       2       0       0")
        self.assertEqual(lines[2], "2026-10-28       2       1      40      10")
        self.assertEqual(lines[-1], "Last 14 days: 7 clones (3 unique cloners), 40 views (10 unique visitors)")


class MainTest(unittest.TestCase):
    def run_main(self, argv, env=None, pages=None, urlopen=None):
        if urlopen is None:
            urlopen, seen = paged(pages or [RELEASES])
        else:
            seen = []
        stdout, stderr = io.StringIO(), io.StringIO()
        with unittest.mock.patch.dict("os.environ", env or {}, clear=True):
            with unittest.mock.patch("urllib.request.urlopen", urlopen):
                with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
                    code = downloads.main(argv)
        return code, stdout.getvalue(), stderr.getvalue(), seen

    def test_default_lists_the_default_repository(self):
        code, out, _, seen = self.run_main([])
        self.assertEqual(code, 0)
        self.assertIn("/repos/Open-MBEE/OpenSysML/releases?", seen[0].full_url)
        self.assertTrue(out.endswith("4 releases, 1,824 downloads\n"))

    def test_json_carries_repo_and_rows(self):
        code, out, _, _ = self.run_main(["--json", "--stable", "--repo", "o/r"], env={"GITHUB_TOKEN": "t"})
        self.assertEqual(code, 0)
        payload = json.loads(out)
        self.assertEqual(payload["repo"], "o/r")
        self.assertEqual([r["tag"] for r in payload["releases"]], ["v0.11.0", "v0.10.1"])
        self.assertEqual(payload["releases"][0]["assets"][1], {"name": "SHA256SUMS.txt", "downloads": 94, "size": 10})

    def test_gh_token_overrides_github_token(self):
        _, _, _, seen = self.run_main(["--stable"], env={"GITHUB_TOKEN": "read-only", "GH_TOKEN": "push"})
        self.assertEqual(seen[0].get_header("Authorization"), "Bearer push")

    def test_github_token_alone_is_used(self):
        _, _, _, seen = self.run_main(["--stable"], env={"GITHUB_TOKEN": "read-only"})
        self.assertEqual(seen[0].get_header("Authorization"), "Bearer read-only")

    def test_traffic_without_token_fails_before_the_network(self):
        code, out, err, seen = self.run_main(["--traffic"])
        self.assertEqual(code, 1)
        self.assertEqual(out, "")
        self.assertEqual(seen, [])
        self.assertIn("--traffic needs GITHUB_TOKEN or GH_TOKEN", err)

    def test_traffic_api_error_names_the_push_access_requirement(self):
        def forbidden(request, timeout=0):
            raise OSError("HTTP Error 403: Forbidden")

        code, out, err, _ = self.run_main(["--traffic"], env={"GH_TOKEN": "t"}, urlopen=forbidden)
        self.assertEqual(code, 1)
        self.assertEqual(out, "")
        self.assertIn("403", err)
        self.assertIn("push access", err)

    def test_release_api_error_is_reported_without_a_stack(self):
        def failing(request, timeout=0):
            raise OSError("HTTP Error 404: Not Found")

        code, out, err, _ = self.run_main(["--repo", "o/missing"], urlopen=failing)
        self.assertEqual(code, 1)
        self.assertEqual(out, "")
        self.assertEqual(err, "Error: listing releases of o/missing failed: HTTP Error 404: Not Found\n")

    def test_views_are_mutually_exclusive(self):
        with contextlib.redirect_stderr(io.StringIO()):
            with self.assertRaises(SystemExit):
                downloads.main(["--json", "--assets"])


if __name__ == "__main__":
    unittest.main()

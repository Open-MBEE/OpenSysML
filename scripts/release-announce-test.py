#!/usr/bin/env python3
"""Tests for scripts/release-announce.py. Run: python3 scripts/release-announce-test.py"""

from __future__ import annotations

import contextlib
import importlib.util
import io
import json
import pathlib
import sys
import threading
import unittest
import unittest.mock
from http.server import BaseHTTPRequestHandler, HTTPServer

HERE = pathlib.Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("release_announce", HERE / "release-announce.py")
announce = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(announce)


@contextlib.contextmanager
def webhook_server(response_body: bytes):
    requests = []

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self):
            length = int(self.headers.get("Content-Length", "0"))
            requests.append(
                {
                    "content_type": self.headers.get("Content-Type"),
                    "body": self.rfile.read(length),
                }
            )
            self.send_response(200)
            self.end_headers()
            self.wfile.write(response_body)

        def log_message(self, format, *args):
            pass

    server = HTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield f"http://127.0.0.1:{server.server_port}/webhook", requests
    finally:
        server.shutdown()
        thread.join()
        server.server_close()


def run_main(args: list[str]) -> None:
    with unittest.mock.patch.object(sys, "argv", ["release-announce.py", *args]):
        announce.main()


class PayloadTest(unittest.TestCase):
    def test_falls_back_to_tag_and_adds_url_and_snippet(self):
        url = "https://github.com/Open-MBEE/OpenSysML/releases/tag/v1.2.3"
        self.assertEqual(
            announce.build_payload("  ", url, "v1.2.3", "Install the release"),
            {
                "text": f":tada: *v1.2.3* is out!\nRelease notes and downloads: {url}"
                        "\n\nInstall the release",
            },
        )

    def test_escapes_title_controls_and_preserves_snippet_mrkdwn(self):
        url = "https://github.com/Open-MBEE/OpenSysML/releases/tag/v1.2.3"
        snippet = "<https://example.org|docs>"
        text = announce.build_payload(
            "v1.2.3 <!channel> & <@U123>", url, "v1.2.3", snippet
        )["text"]
        self.assertIn(
            "v1.2.3 &lt;!channel&gt; &amp; &lt;@U123&gt;", text
        )
        self.assertNotIn("<!channel>", text)
        self.assertTrue(text.endswith(f"\n\n{snippet}"))


class MainTest(unittest.TestCase):
    def test_skips_pre_release_tag_without_network_or_webhook(self):
        stdout = io.StringIO()
        with unittest.mock.patch.dict("os.environ", {}, clear=True):
            with unittest.mock.patch(
                "urllib.request.urlopen", side_effect=AssertionError("network used")
            ) as urlopen:
                with contextlib.redirect_stdout(stdout):
                    result = announce.main(["--tag", "v1.2.3-rc.1", "--dry-run"])

        self.assertEqual(result, 0)
        self.assertEqual(
            stdout.getvalue(),
            "Not announcing pre-release v1.2.3-rc.1; only stable releases are announced.\n",
        )
        urlopen.assert_not_called()

    def test_dry_run_prints_json_without_network_access(self):
        stdout = io.StringIO()
        with unittest.mock.patch.dict("os.environ", {}, clear=True):
            with unittest.mock.patch(
                "urllib.request.urlopen", side_effect=AssertionError("network used")
            ):
                with contextlib.redirect_stdout(stdout):
                    run_main(
                        [
                            "--tag",
                            "v1.2.3",
                            "--release-name",
                            "OpenSysML 1.2.3",
                            "--release-url",
                            "https://example.test/release",
                            "--dry-run",
                        ]
                    )

        payload = json.loads(stdout.getvalue())
        self.assertEqual(payload["text"].splitlines()[0], ":tada: *OpenSysML 1.2.3* is out!")
        self.assertIn("https://example.test/release", payload["text"])

    def test_posts_json_with_content_type_to_webhook(self):
        with webhook_server(b"ok") as (url, requests):
            with unittest.mock.patch.dict(
                "os.environ", {"OPENSYSML_WEBHOOK_URL": url}, clear=True
            ):
                with contextlib.redirect_stdout(io.StringIO()):
                    run_main(
                        [
                            "--tag",
                            "v1.2.3",
                            "--release-name",
                            "OpenSysML 1.2.3",
                            "--release-url",
                            "https://example.test/release",
                        ]
                    )

        self.assertEqual(len(requests), 1)
        self.assertEqual(requests[0]["content_type"], "application/json")
        self.assertEqual(
            json.loads(requests[0]["body"]),
            {
                "text": ":tada: *OpenSysML 1.2.3* is out!\n"
                        "Release notes and downloads: https://example.test/release",
            },
        )

    def test_refuses_a_non_ok_webhook_response(self):
        with webhook_server(b"nope") as (url, _):
            with unittest.mock.patch.dict(
                "os.environ", {"OPENSYSML_WEBHOOK_URL": url}, clear=True
            ):
                with self.assertRaisesRegex(SystemExit, "nope"):
                    run_main(
                        [
                            "--tag",
                            "v1.2.3",
                            "--release-name",
                            "OpenSysML 1.2.3",
                            "--release-url",
                            "https://example.test/release",
                        ]
                    )

    def test_requires_the_slack_context_webhook(self):
        with unittest.mock.patch.dict("os.environ", {}, clear=True):
            with unittest.mock.patch(
                "urllib.request.urlopen", side_effect=AssertionError("network used")
            ):
                with self.assertRaisesRegex(SystemExit, "OPENSYSML_WEBHOOK_URL"):
                    run_main(
                        [
                            "--tag",
                            "v1.2.3",
                            "--release-name",
                            "OpenSysML 1.2.3",
                            "--release-url",
                            "https://example.test/release",
                        ]
                    )


if __name__ == "__main__":
    unittest.main()

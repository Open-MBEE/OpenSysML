#!/usr/bin/env python3
"""Re-emit `go test -json` events as the text `go test -v` prints, as they arrive.

`go test -v a b c` holds each package's output back until that package finishes, so
a CI step over several slow packages is silent for as long as the fastest one takes
and a no-output timeout fires on buffering rather than on a hang. `-json` streams
every package's events at once; this filter turns them back into the -v lines.

Usage: go test -json ... | go-test-stream.py
Exits 1 when any package failed (or no package reported), so the pipeline fails
even without pipefail; with pipefail, go test's own status is kept as well.
"""

import json
import sys


def stream(lines, out) -> int:
    """Write each event's text to out; return the exit status for the run."""
    packages = 0
    failed = 0
    for line in lines:
        try:
            event = json.loads(line)
        except ValueError:
            out.write(line)
            out.flush()
            continue
        output = event.get("Output")
        if output is not None:
            out.write(output)
            out.flush()
        if event.get("Test") is None and event.get("Action") in ("pass", "fail", "skip"):
            packages += 1
            if event["Action"] == "fail":
                failed += 1
    return 1 if failed or packages == 0 else 0


def main() -> int:
    return stream(sys.stdin, sys.stdout)


if __name__ == "__main__":
    sys.exit(main())

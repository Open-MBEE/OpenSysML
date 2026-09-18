#!/usr/bin/env python3
"""Copy the pinned release digests into each client that ships them.

`client/release-digests.json` is the single table of per-release asset digests,
written by `client/python/scripts/pin_release_checksums.py`. A client verifies a
download against the copy it ships, because a copy resolved at run time from
outside the published artifact is not a pin, so each packaged client carries its
own — a wheel cannot include a file above its project directory, and neither can
a crate, a jar or an npm tarball.

    python3 scripts/sync-release-digests.py            # rewrite the copies
    python3 scripts/sync-release-digests.py --check    # fail on any that drifted

A client that starts shipping the table adds its copy to COPIES below.
"""

import argparse
import os
import sys

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
TABLE = "release-digests.json"
SOURCE = os.path.join(REPO_ROOT, "client", TABLE)

#: Path, relative to the repository root, of every client's shipped copy.
COPIES = (
    os.path.join("client", "python", "opensysml", TABLE),
    os.path.join("client", "node", TABLE),
    os.path.join("client", "java", "opensysml-client", "src", "main", "resources", TABLE),
    os.path.join("client", "rust", "opensysml", TABLE),
)


def sync(source=None, check=False):
    """Rewrite the copies, or report the ones that no longer match the source.

    Args:
        source (str, optional): Path to client/release-digests.json
        check (bool): Report drift instead of correcting it

    Returns:
        list[str]: One message per copy that drifted, empty when all agree
    """
    with open(source or SOURCE, encoding="utf-8") as f:
        table = f.read()

    drifted = []
    for relative in COPIES:
        path = os.path.join(REPO_ROOT, relative)
        current = None
        if os.path.exists(path):
            with open(path, encoding="utf-8") as f:
                current = f.read()
        if current == table:
            continue
        if check:
            drifted.append(
                f"{relative} is not the table in client/release-digests.json; "
                f"run python3 scripts/sync-release-digests.py"
            )
            continue
        with open(path, "w", encoding="utf-8") as f:
            f.write(table)
        print(f"wrote {relative}", file=sys.stderr)
    return drifted


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--check",
        action="store_true",
        help="fail if a client's copy differs instead of rewriting it",
    )
    args = parser.parse_args(argv)

    drifted = sync(check=args.check)
    for problem in drifted:
        print(f"error: {problem}", file=sys.stderr)
    return 1 if drifted else 0


if __name__ == "__main__":
    sys.exit(main())

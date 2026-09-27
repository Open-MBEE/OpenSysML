#!/usr/bin/env python3
"""Write the SLSA provenance statement for a release's checksum manifest.

Usage: scripts/release-provenance.py --manifest dist/SHA256SUMS.txt --out dist/provenance.intoto.json

The statement is an in-toto Statement v1 whose subjects are every artifact the
manifest lists, with the digest the manifest records, and whose predicate is
SLSA Provenance v1 describing the build that produced them: the repository and
tag built, the commit resolved, and the CircleCI job that ran. The release
pipeline signs the file with cosign keyless, so the statement itself carries no
signature and names no secret.

The build is described from CircleCI's environment: CIRCLE_TAG, CIRCLE_SHA1,
CIRCLE_BUILD_URL, CIRCLE_PROJECT_ID, CIRCLE_ORGANIZATION_ID, CIRCLE_WORKFLOW_ID,
CIRCLE_JOB, CIRCLE_PROJECT_USERNAME and CIRCLE_PROJECT_REPONAME. Every one is
required: a statement missing any of them would describe a build that cannot be
told apart from another, so the script refuses rather than write a vaguer one.
"""

from __future__ import annotations

import argparse
import json
import os
import pathlib
import re
import sys

STATEMENT_TYPE = "https://in-toto.io/Statement/v1"
PREDICATE_TYPE = "https://slsa.dev/provenance/v1"

# Names this repository's build-release job; bump the version when the job's meaning changes.
BUILD_TYPE = "https://github.com/Open-MBEE/OpenSysML/.circleci/build-release/v1"

_MANIFEST_LINE = re.compile(r"^([0-9a-f]{64}) [ *](\S.*)$")
_TAG = re.compile(r"^v[0-9]")
_SHA1 = re.compile(r"^[0-9a-f]{40}$")

REQUIRED_ENV = (
    "CIRCLE_TAG",
    "CIRCLE_SHA1",
    "CIRCLE_BUILD_URL",
    "CIRCLE_PROJECT_ID",
    "CIRCLE_ORGANIZATION_ID",
    "CIRCLE_WORKFLOW_ID",
    "CIRCLE_JOB",
    "CIRCLE_PROJECT_USERNAME",
    "CIRCLE_PROJECT_REPONAME",
)


class ProvenanceError(Exception):
    """The manifest or the build description cannot make a truthful statement."""


class Build:
    """What the provenance says about the build that produced the artifacts."""

    def __init__(self, tag, commit, build_url, project_id, organization_id, workflow_id,
                 job, owner, repository):
        self.tag = tag
        self.commit = commit
        self.build_url = build_url
        self.project_id = project_id
        self.organization_id = organization_id
        self.workflow_id = workflow_id
        self.job = job
        self.owner = owner
        self.repository = repository

    @classmethod
    def from_env(cls, env):
        missing = [name for name in REQUIRED_ENV if not env.get(name)]
        if missing:
            raise ProvenanceError(
                "cannot describe the build: " + ", ".join(missing) + " not set"
            )
        if not _TAG.match(env["CIRCLE_TAG"]):
            raise ProvenanceError(
                f"CIRCLE_TAG {env['CIRCLE_TAG']!r} is not a release tag (v<version>)"
            )
        if not _SHA1.match(env["CIRCLE_SHA1"]):
            raise ProvenanceError(f"CIRCLE_SHA1 {env['CIRCLE_SHA1']!r} is not a commit hash")
        return cls(
            tag=env["CIRCLE_TAG"],
            commit=env["CIRCLE_SHA1"],
            build_url=env["CIRCLE_BUILD_URL"],
            project_id=env["CIRCLE_PROJECT_ID"],
            organization_id=env["CIRCLE_ORGANIZATION_ID"],
            workflow_id=env["CIRCLE_WORKFLOW_ID"],
            job=env["CIRCLE_JOB"],
            owner=env["CIRCLE_PROJECT_USERNAME"],
            repository=env["CIRCLE_PROJECT_REPONAME"],
        )

    @property
    def source(self):
        return f"https://github.com/{self.owner}/{self.repository}"


def subjects(manifest_text):
    """The artifacts a `sha256sum` manifest lists, as in-toto subjects.

    Every line must be `<64 hex digits>  <name>`; a name may appear once. An
    empty manifest is refused, since provenance over nothing is not provenance.
    """
    seen = set()
    result = []
    for number, line in enumerate(manifest_text.splitlines(), start=1):
        if not line.strip():
            continue
        match = _MANIFEST_LINE.match(line)
        if not match:
            raise ProvenanceError(f"manifest line {number} is not a sha256sum line: {line!r}")
        digest, name = match.group(1), match.group(2)
        if name in seen:
            raise ProvenanceError(f"manifest lists {name!r} twice")
        seen.add(name)
        result.append({"name": name, "digest": {"sha256": digest}})
    if not result:
        raise ProvenanceError("manifest lists no artifacts")
    return result


def statement(manifest_text, build):
    """The in-toto statement over the manifest's artifacts for this build."""
    ref = f"refs/tags/{build.tag}"
    return {
        "_type": STATEMENT_TYPE,
        "subject": subjects(manifest_text),
        "predicateType": PREDICATE_TYPE,
        "predicate": {
            "buildDefinition": {
                "buildType": BUILD_TYPE,
                "externalParameters": {
                    "repository": build.source,
                    "ref": ref,
                    "job": build.job,
                },
                "internalParameters": {
                    "organization": build.organization_id,
                    "project": build.project_id,
                    "workflow": build.workflow_id,
                },
                "resolvedDependencies": [
                    {
                        "uri": f"git+{build.source}@{ref}",
                        "digest": {"gitCommit": build.commit},
                    }
                ],
            },
            "runDetails": {
                "builder": {
                    "id": f"https://circleci.com/api/v2/projects/{build.project_id}",
                },
                "metadata": {
                    "invocationId": build.build_url,
                },
            },
        },
    }


def render(manifest_text, build):
    return json.dumps(statement(manifest_text, build), indent=2) + "\n"


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    parser.add_argument("--manifest", required=True, type=pathlib.Path,
                        help="the sha256sum manifest over the release artifacts")
    parser.add_argument("--out", required=True, type=pathlib.Path,
                        help="where to write the statement")
    args = parser.parse_args(argv)
    try:
        build = Build.from_env(os.environ)
        text = render(args.manifest.read_text(encoding="utf-8"), build)
    except (ProvenanceError, OSError) as err:
        print(f"release-provenance: {err}", file=sys.stderr)
        return 1
    args.out.write_text(text, encoding="utf-8")
    return 0


if __name__ == "__main__":
    sys.exit(main())

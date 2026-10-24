# Security policy

The response times and the disclosure process below are the commitments reporters can hold
us to. They are set to the bar OpenSysML will be held to as a tool used on safety-critical
projects, not to the project's current release stage.

## Reporting a vulnerability

Report vulnerabilities **privately**, through GitHub's private vulnerability reporting:
[Security → Report a vulnerability](https://github.com/Open-MBEE/OpenSysML/security/advisories/new)
on this repository. Do not open a public issue, pull request or discussion for anything you
believe is a security problem, and do not include a working exploit in a public place before the
advisory is published.

A report is most useful when it has:

- the affected component (`sysml`, `sysml-lsp`, `sysml-grpc`, `sysml-mcp`, `sysml-jupyter-kernel`,
  the Python or Node client, the VS Code extension, `install.sh`/`install.ps1`, or a release
  workflow), and the
  version (`sysml --version`, a release tag, or a nightly's commit);
- a minimal reproducer: the model, command line, request or input that triggers the problem;
- the impact as you understand it (what an attacker gains, and from where).

Reports are handled by the release authority, **Jason Han ([@HuiJun](https://github.com/HuiJun))**,
who is the code owner of this repository (see [`.github/CODEOWNERS`](.github/CODEOWNERS)). If
a report has gone a week without an acknowledgement, reach the same person through an
`@`-mention on any issue of this repository, saying only that a private report is waiting.

## What to expect

| Step | Deadline, from receipt |
|---|---|
| Acknowledgement | 3 business days |
| Triage: confirmed or not, severity (CVSS v3.1), affected versions | 7 calendar days |
| Fix released, **critical** severity | 14 calendar days |
| Fix released, **high** or **medium** severity | 60 calendar days |
| Fix released, **low** severity | next scheduled release |
| Advisory published | with the fix release |

Severity is scored with [CVSS v3.1](https://www.first.org/cvss/v3.1/specification-document)
and may be revised during triage; the reporter is told the score and the reasoning. A fix that
cannot meet its deadline is not hidden: the reporter is told why, given the new date, and the
advisory says what to do in the meantime.

Fixes ship as a release of the affected component (a patch release of the current line, or a
hotfix cut from `main`, see [docs/project/releasing.md](docs/project/releasing.md)) and a
[GitHub Security Advisory](https://github.com/Open-MBEE/OpenSysML/security/advisories) that
names the affected and fixed versions, the CVE (requested through GitHub), the reporter if they
want credit, and any mitigation for users who cannot update at once. Nightly builds get the fix
as soon as it merges, before the advisory.

## Coordinated disclosure

We ask reporters to keep the report private until the advisory is published. We publish within
**90 days** of receipt whether or not a fix is ready, unless the reporter and the project agree in
writing to a longer embargo; a vulnerability already being exploited is published, with
mitigations, as soon as there is something actionable to say. Reporters are credited in the
advisory unless they ask not to be. There is no bug bounty.

## Automated and AI-assisted reports

Reports produced by scanners, fuzzers or AI assistants are welcome on the same terms as any
other, with one requirement: the report must contain a reproducer that we can run against a
current release or nightly and that demonstrates the claimed impact. A report that consists of
a tool's unverified output, a theoretical pattern with no reproducer, or a reproducer that does
not demonstrate the stated impact is closed as unverified without triage, and the reporter is
told so within the acknowledgement deadline. Repeated unverified submissions from one source are
not acknowledged individually. Please say in the report when it was produced or assisted by a
tool; it does not count against the report.

## Supported versions

| Version | Supported |
|---|---|
| The latest release (`vX.Y.Z` on [Releases](https://github.com/Open-MBEE/OpenSysML/releases)) | Yes: fixes ship as a patch release of that line |
| Earlier releases | No: update to the latest release |
| `nightly` prereleases | Fixed on `develop` as soon as they merge; no advisory covers a nightly |

Until 1.0.0, each release line is supported only until the next release. From 1.0.0 the previous
minor release line is also supported for 90 days after its successor ships, so that a fix can
be taken without a feature update.

## What counts as a vulnerability

In scope:

- memory safety, code execution, path traversal or file disclosure from a crafted model, project
  file, import, XMI/RDF input, FMU, template or query — the parser and the runtime must fail
  with a diagnostic, never execute or disclose;
- the network surfaces: `sysml-grpc` and the MCP server's HTTP mode (`sysml-mcp -http`),
  including authentication, authorization and request handling;
- the supply chain: `install.sh`/`install.ps1`, the release workflows, the signed checksum
  manifest and provenance, the Python and Node clients' download and verification path, the
  VS Code extension and the published packages (PyPI, npm, Maven Central, Homebrew);
- secrets or credentials in the repository, releases, or logs;
- a denial of service that a crafted input causes in a server (`sysml-grpc`, `sysml-lsp`,
  `sysml-mcp`) — a crash of the command-line tool on a bad model is a bug, not a vulnerability,
  unless it breaks an invariant above.

Out of scope: the SysML v2 and KerML specifications and the OMG pilot implementation (report to
their maintainers), the contents of a user's own models, and findings that require an
already-compromised host or user account.

## Verifying what you run

Every release ships a checksum manifest, `SHA256SUMS.txt`, signed with cosign (keyless, from the
release pipeline's identity) as `SHA256SUMS.txt.bundle`, and a SLSA provenance statement signed
the same way as `provenance.intoto.json.bundle`. Windows builds are Authenticode signed through
SignPath Foundation once the project's application is approved. How to verify each of these,
and the exact signer identity to pin, is in
[docs/project/releasing.md § The signed checksum manifest](docs/project/releasing.md#the-signed-checksum-manifest)
and [§ The release provenance](docs/project/releasing.md#the-release-provenance); the
installer's `--verify-signature` flag does the manifest check for you (see
[docs/guide/01-install.md](docs/guide/01-install.md)). A release asset that does not verify is
a vulnerability report in itself: send it through the channel above.

## Dependencies and static analysis

Dependabot alerts and the code-scanning results that SonarCloud uploads to this repository are
watched by the code owners and remediated on the deadlines in the table above (critical within
14 days, high and medium within 60). Dependency updates that fix a published vulnerability are
released on the same schedule as a vulnerability in our own code.

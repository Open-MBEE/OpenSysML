- **The security policy is published where users look.** The documentation site has a
  Security page (`docs/guide/security.md`) that embeds `SECURITY.md` at build time — how to
  report a vulnerability, the response deadlines, coordinated disclosure and the supported
  versions — and adds how to verify a downloaded release asset by asset (checksum manifest and
  cosign signature, provenance, wheels, npm, Maven Central, crates.io, the container image's
  provenance and SBOM), the service-hardening pointers and where advisories are published. The
  same build serves RFC 9116 `security.txt` at `/.well-known/security.txt` and `/security.txt`,
  with `Expires` a year after the commit built. `python3 scripts/changelog.py release-notes
  [VERSION]` renders a GitHub release body that ends with the support period from
  `SECURITY.md` and a link to the verification section; the nightly notes carry the same footer,
  and the README gains a Security section.

# Homebrew packaging

`brew install` is the recommended macOS install path: Homebrew fetches archives with `curl`,
which does not set the `com.apple.quarantine` extended attribute, so a Homebrew-installed
`sysml` never triggers the Gatekeeper "developer cannot be verified" prompt. (Homebrew
applies quarantine only to *casks* — hence the cask-only `--no-quarantine` flag — not to
formulae.) It is the accepted stopgap until the releases are Developer ID signed and
notarized; see [docs/project/macos-distribution.md](../../docs/project/macos-distribution.md).

`Formula/opensysml.rb` here is the maintained source of the formula. It carries
`__TAG__` / `__SHA256_*__` placeholders and is **not installable as-is**;
`scripts/render-homebrew-formula.sh` substitutes them from a release's `SHA256SUMS.txt` and
strips the maintainer-facing header comment.

The formula `depends_on "z3"`, so `brew install` gives the experimental `%check`/`%explain`
solver path a solver out of the box (`z3` is in `homebrew/core`, MIT, bottled). The dependency
is a convenience only: the solver is discovered at runtime on `PATH` or via `OPENSYSML_SMT`,
nothing links it, and every other command works without it — see
[docs/guide/01-install.md](../../docs/guide/01-install.md#installing-a-solver-optional). The
`test do` block asserts that the dependency is on `PATH` and answers SMT-LIB2, rather than
driving `%check` through the installed binary, so it holds for release binaries that predate
those commands.

The formula deliberately has **no `version` line**: Homebrew scans the version from the tag
in the release URL, and `brew audit --strict` fails with `version ... is redundant with
version scanned from URL` if it is also stated explicitly.

## The tap

The tap lives in the separate repository [`Open-MBEE/homebrew-tap`][tap] (public, default
branch `master`), holding one generated file, `Formula/opensysml.rb`. That repository updates
itself: a workflow there runs on a schedule (and on `workflow_dispatch`), resolves the latest
`Open-MBEE/OpenSysML` release tag, fetches `scripts/render-homebrew-formula.sh` and
`packaging/homebrew/Formula/opensysml.rb` from this repository *at that tag*, renders
`Formula/opensysml.rb`, and commits only when the file changed. It uses the tap repository's
own `GITHUB_TOKEN`, so there is no cross-repository secret and nothing here triggers it.

The script and the template here stay the source it renders from, so a change to either must
keep working when fetched standalone at a tag: the script may not depend on anything else in
this repository except the template path it already reads.
The repository name must keep the `homebrew-` prefix: `brew tap <user>/<repo>` always expands
to `github.com/<user>/homebrew-<repo>`, so a repository named plain `tap` cannot be tapped.

[tap]: https://github.com/Open-MBEE/homebrew-tap

### Tap trust

Since Homebrew 6.0 only official taps are trusted by default; a third-party tap's Ruby is not
loaded until it is trusted, and there is no way to make a third-party tap trusted for everyone
(see [Tap Trust](https://docs.brew.sh/Tap-Trust)). Install by **fully-qualified name** —
`brew install Open-MBEE/tap/opensysml` — which trusts just that formula and needs no separate
step. Tapping first requires `brew trust --formula Open-MBEE/tap/opensysml` (or
`brew trust Open-MBEE/tap` for every current and future formula in the tap) before
`brew install opensysml` will load it. Taps created by `brew tap-new` are trusted
automatically, which is why the local-tap recipe below needs no trust step.

The only route to trusted-by-default is `homebrew/core`, which needs no tap at all but has
[notability requirements](https://docs.brew.sh/Package-Acceptance-Policy#notability) —
75 stars / 30 forks / 30 watchers, or 225 / 90 / 90 for a self-submission by the repository
owner, on a repository at least 30 days old. OpenSysML is well short of those today.

### Verifying a formula

```bash
brew install --verbose Open-MBEE/tap/opensysml
brew test Open-MBEE/tap/opensysml
brew audit --strict --online Open-MBEE/tap/opensysml
```

The same commands verify a *rendered but unpublished* formula, by pointing them at a
throwaway local tap — this is how the v0.0.4 render was checked before the tap existed:

```bash
./scripts/render-homebrew-formula.sh v0.0.4 > /tmp/opensysml.rb
brew tap-new local/systest --no-git
cp /tmp/opensysml.rb "$(brew --repository local/systest)/Formula/opensysml.rb"
brew install local/systest/opensysml && brew test local/systest/opensysml
brew audit --strict --online local/systest/opensysml
```

## Per release

The release job publishes stable artifact names, so cutting a release takes one step:

1. Tag `vX.Y.Z` and let CircleCI publish the release (per-binary archives,
   `opensysml-<os>-<arch>.tar.gz`/`.zip` bundles, the `sysml-mcp-<os>-<arch>.tar.gz`/`.zip`
   archives, and `SHA256SUMS.txt`).
2. Nothing else: the tap's own scheduled workflow renders and commits the formula within its
   schedule interval. Nine values change — the tag in the eight URLs (which is also where
   Homebrew reads the version from) and the eight `sha256` lines.

The manual route remains the fallback when the tap has to be corrected out of band — render
from a checkout of this repository and commit the result in the tap:

```bash
./scripts/render-homebrew-formula.sh vX.Y.Z > Formula/opensysml.rb
```

A release without `SHA256SUMS.txt`, or without a `opensysml-<os>-<arch>.tar.gz` line in it,
fails the render loudly rather than producing a formula with a wrong or missing checksum.

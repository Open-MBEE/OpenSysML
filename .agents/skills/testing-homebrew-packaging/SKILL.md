---
name: testing-homebrew-packaging
description: How to verify the OpenSysML Homebrew formula end to end on Linux — rendering packaging/homebrew/Formula/opensysml.rb with scripts/render-homebrew-formula.sh, installing/testing/auditing it through a throwaway local tap, and the adversarial paths worth checking.
---

# Verifying the OpenSysML Homebrew formula (Linux)

`packaging/homebrew/Formula/opensysml.rb` is a **template** with `__TAG__` / `__SHA256_*__`
placeholders. `scripts/render-homebrew-formula.sh <tag>` substitutes them from the release's
`SHA256SUMS.txt` and strips the maintainer header. The tap repo `Open-MBEE/homebrew-tap` does
not exist yet, so everything below uses a throwaway **local** tap.

## Prerequisites

- Homebrew is not preinstalled by the repo blueprint. On this box it lives at
  `/home/linuxbrew/.linuxbrew/bin/brew` — `export PATH=/home/linuxbrew/.linuxbrew/bin:$PATH`.
  If it is absent, install it (`/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`).
- Network access to `github.com` is required (release archives + `brew audit --online`).
- No Go build is needed: the formula installs **prebuilt release binaries**, so this path is
  independent of `make build`. But release binaries can *predate* a REPL feature (the v0.0.9
  `sysml` answers `unknown command "%check"`), so never test a new runtime feature through the
  brew-installed binary — build it with `make build` and test brew's *environment* instead.
- Release asset names have drifted across renames (`systemica-*` in v0.0.9 vs the formula's
  `opensysml-*`). When `render-homebrew-formula.sh <tag>` fails with `error: no checksum for
  opensysml-darwin-arm64.tar.gz`, that is the release, not the PR — confirm by fetching the
  release's `SHA256SUMS.txt` and reading the names.

## Always start from a clean slate

A previous run leaves state that makes a rerun a no-op and silently proves nothing:

```bash
brew uninstall opensysml 2>/dev/null
brew untap local/<previous-tap-name> 2>/dev/null
which sysml   # must print nothing before you begin
brew tap      # confirm no leftover local/* taps
```

Use a **fresh** tap name each run (`local/pr73check`, not a reused one) so `tap-new` itself is
exercised. First `brew install` pulls ~12 dependencies (gcc, glibc, binutils…) and takes
2–4 minutes; budget for it and answer the `[y/n]` prompt (or `--yes`).

## The verification recipe (mirrors packaging/homebrew/README.md)

```bash
./scripts/render-homebrew-formula.sh v0.0.4 > /tmp/opensysml.rb
brew tap-new local/<fresh> --no-git
cp /tmp/opensysml.rb "$(brew --repository local/<fresh>)/Formula/opensysml.rb"
brew install local/<fresh>/opensysml && brew test local/<fresh>/opensysml
brew audit --strict --online local/<fresh>/opensysml
```

All four must exit 0.

### When the release has no `opensysml-*` assets (install/test without a real release)

Render from a **hand-made manifest** and serve a renamed copy of a real archive over local HTTP:

```bash
mkdir -p /tmp/bc/serve/Open-MBEE/OpenSysML/releases/download/<tag>
cp <real systemica-linux-amd64.tar.gz> /tmp/bc/serve/Open-MBEE/OpenSysML/releases/download/<tag>/opensysml-linux-amd64.tar.gz
SUM=$(sha256sum .../opensysml-linux-amd64.tar.gz | cut -d' ' -f1)
for a in darwin-arm64 darwin-amd64 linux-arm64 linux-amd64; do echo "$SUM  opensysml-$a.tar.gz"; done > /tmp/bc/sums.txt
./scripts/render-homebrew-formula.sh <tag> /tmp/bc/sums.txt > /tmp/opensysml.rb
(cd /tmp/bc/serve && python3 -m http.server 8099 --bind 127.0.0.1 &)
# in the tap copy only: s|https://github.com/Open-MBEE/OpenSysML/releases/download|http://127.0.0.1:8099/Open-MBEE/OpenSysML/releases/download|g
```

Caveat: with a `127.0.0.1` URL Homebrew's version scanner no longer sees the tag — it installs
as `Cellar/opensysml/64` (from `…-amd64`), so `assert_match version.to_s` in `test do` becomes
meaningless (it can pass by accident on a commit hash containing `64`). Run
`brew audit --strict` (offline) against the **unmodified github.com** rendering, and report the
`--online` audit separately: it fails with `The source URL … is not reachable (HTTP status code
404)` whenever the release lacks `opensysml-*` assets.

Benign noise to ignore: `fatal: ambiguous argument
'refs/remotes/origin/main'` during auto-update, `Sandbox unavailable: building without
sandboxing!`, and ``AllCops/UseProjectIndex` is enabled but the `rubydex` gem is not installed``.

## Assertions that actually discriminate

- **No `version` line.** Homebrew scans the version from the tag in the release URL;
  `brew audit --strict` fails with `` `version 0.0.4` is redundant with version scanned from
  URL``. Prove the removal matters with a **counterfactual**: re-insert the line into the tap
  copy and rerun the audit — it must fail with exactly that wording, then restore the file.
  Just seeing a green audit does not prove the fix.
- **Checksums match the release**, not merely "look like hashes". Download
  `https://github.com/Open-MBEE/OpenSysML/releases/download/<tag>/SHA256SUMS.txt` and compare
  the four `opensysml-<os>-<arch>.tar.gz` entries against the `url`/`sha256` pairs parsed out
  of the rendered file. Note the manifest also contains per-binary `sysml-*` / `sysml-lsp-*`
  entries — the formula only uses the `opensysml-*` bundles.
- **Zero placeholders**: `grep -c '__[A-Z0-9_]*__' /tmp/opensysml.rb` → `0`. The script has its
  own guard for this (exit 1), so a rendering regression usually surfaces as a script failure.
- **`depends_on "z3"`** (the solver dependency): a green `brew test` proves nothing on a box
  that already has apt's `/usr/bin/z3`. Two checks that do discriminate: (1) uninstall brew z3
  first, then the install log must say `==> Installing local/<tap>/opensysml dependency: z3`
  and `brew deps opensysml` must list `z3`; (2) temporarily replace the tested executable in
  the `test do` block with a nonexistent name — `brew test` must fail — then restore. To see
  *which* z3 `brew test` puts on PATH, temporarily assert a never-matching string against
  `shell_output("/bin/sh -c 'command -v z3'")`; the Minitest failure prints the real value
  (`/home/linuxbrew/.linuxbrew/bin/z3`). Note `shell_output("command -v z3")` fails with
  `Errno::ENOENT` — `command` is a shell builtin, so wrap it in `/bin/sh -c`.
- **The installed binaries are the brew ones**: check `which sysml` resolves under
  `/home/linuxbrew/...`, not `./bin/sysml`, before smoke-testing. `sysml --version` must print
  the release tag (`sysml v0.0.4`), not a dev/commit string.

## Adversarial paths (all cheap, all worth running)

| Command | Expected |
|---|---|
| `render-homebrew-formula.sh v9.9.9` | exit 22, `curl: (22) ... error: 404`, no `class OpenSysML` in output |
| `render-homebrew-formula.sh <tag> <sums-with-one-line>` | exit 1, `error: no checksum for opensysml-darwin-amd64.tar.gz` |
| `render-homebrew-formula.sh` (no args) | exit 2, `usage: ... <tag> [SHA256SUMS.txt]` |
| `brew install` of the **unrendered** template | must FAIL — download URL contains literal `__TAG__` and 404s |

Rehearse the 404 case sparingly: repeated unauthenticated GitHub hits can turn it into a
misleading `HTTP Error 403: rate limit exceeded`. Report the 404 wording, not the 403.

## Homebrew 6.x tap trust

Homebrew 6 requires tap trust. Taps you create locally with `brew tap-new` are trusted
automatically (`==> Trusted formula local/<tap>/opensysml`), but installing from tap B will warn
that tap A "is not trusted" if A is still tapped. That warning is cosmetic for this workflow;
untap old taps to silence it, or `brew trust local/<tap>`.

## Proving the solver stays optional at runtime (companion to `depends_on "z3"`)

Discovery lives in `internal/exec/solve/solver.go` (`OPENSYSML_SMT`, then `z3`, then `cvc5`) and
its messages in `errors.go`. Drive `bin/sysml` with a scrubbed PATH — one directory per
scenario, symlinking only the solver you want — and feed meta-commands on stdin:

```bash
mkdir -p /tmp/pz3 /tmp/pcvc5 /tmp/pnone
ln -sf /usr/bin/z3 /tmp/pz3/z3; ln -sf <cvc5 dir>/bin/cvc5 /tmp/pcvc5/cvc5
timeout 60 env -i HOME=$HOME PATH=/tmp/pnone /bin/sh -c 'bin/sysml model.sysml < cmds.txt'
```

Gotchas: `env -i` without a PATH containing `sh` fails with `env: 'sh': No such file or
directory` (use `/bin/sh`); and put the meta-commands in a **file** rather than
`printf "%check …"` — `printf` eats `%c`/`%r` as directives and silently corrupts the input.
A good discriminating matrix: z3 only → verdict names `z3`; cvc5 only → names `cvc5`; both with
cvc5 first in PATH → still `z3`; `OPENSYSML_SMT=cvc5` (bare name) → `cvc5`;
`OPENSYSML_SMT=<non-executable file>` → `error: no SMT solver found: OPENSYSML_SMT names "…",
which is not an executable file` and *no* fallback verdict; `OPENSYSML_SMT_TIMEOUT=1ms` →
`? … is undecided (z3, 1ms)` + `Reason: the solver ran out of time after 1ms`; no solver →
the long `install z3 (…)` error, and `%eval`/`%requirement`/`%satisfy` output byte-identical to
a run with z3 present (`diff` the two runs — that is the real "solver is optional" proof).

## Manual pages in the bundle (formula installs `share/man/man1/*.1`)

The release bundle archives now carry the pages beside the binaries
(`tar czf … sysml sysml-lsp share`), and the formula does
`man1.install Dir["share/man/man1/*.1"]`. No *published* release has that layout yet, so the
only way to exercise it is a hand-built bundle served over `file://`:

```bash
make build VERSION=v9.9.9
R=/tmp/rel/Open-MBEE/OpenSysML/releases/download/v9.9.9; mkdir -p "$R" /tmp/stage/share/man/man1
cp bin/sysml bin/sysml-lsp /tmp/stage/; cp packaging/man/man1/*.1 /tmp/stage/share/man/man1/
tar czf "$R/opensysml-linux-amd64.tar.gz" -C /tmp/stage sysml sysml-lsp share
# hand-made SHA256SUMS with that one sum repeated for all four assets, then render, then in the
# tap copy only: s|https://github.com/…/download|file:///tmp/rel/…/download|g
```

Caveats learned the hard way:

- Homebrew cannot scan a version out of a `file://` (or `127.0.0.1`) URL — it reads `64` from
  `-amd64`, so `assert_match version.to_s` in `test do` becomes meaningless or fails. Add an
  explicit `version "9.9.9"` line **to the tap copy only** and build the binaries with
  `make build VERSION=v9.9.9` so the version assertions stay real. Disclose it as a harness
  artifact; never add it to the committed template (`brew audit --strict` rejects a redundant
  `version` when the URL is a real github release URL).
- `man` may not exist on this box: `/usr/bin/man` is the Ubuntu "system has been minimized"
  stub that prints a notice and exits 0, which silently looks like a page that renders to
  nothing. Install the real one first — `sudo apt-get install -y man-db` (passwordless sudo
  works here) — then `MANPATH=/home/linuxbrew/.linuxbrew/share/man man -w sysml` must print a
  path under `Cellar/opensysml/<v>/share/man/man1`.
- Make the man assertion load-bearing with a counterfactual: delete `man1.install …` from the
  tap copy, `brew reinstall`, and `brew test` must fail with
  `Minitest::Assertion: Expected #<Pathname:…/share/man/man1/sysml.1> to be exist?`. Restore and
  reinstall. A green `brew test` alone proves nothing.
- `brew audit --strict` is stricter than the formula's own style: `assert_predicate man1/"x.1",
  :exist?` fails with ``Use `assert_path_exists <path_to_file>` instead of `assert_predicate
  …```. Prefer `assert_path_exists man1/"sysml.1"`. Audit the **unmodified github.com** rendering
  (offline) and attribute any failure by deleting just the suspect line and re-auditing.

## Recording

This is CLI work, so record a real terminal: see the "Recording setup" section of
`.agents/skills/testing-sysml-repl/SKILL.md` (Konsole on `DISPLAY=:0`, `ctrl+plus` to enlarge
the font, `wmctrl` to maximize). `clear` between steps keeps each assertion legible on camera.

## Devin Secrets Needed

None. All release assets are public; `brew audit --online` works unauthenticated.

# 1. Install

This chapter covers installing `sysml`, `sysml-lsp` and `sysml-grpc` and checking that they
work. Nothing else is needed for the rest of this guide.

## From a release build (recommended)

Download the latest release for your platform from [GitHub Releases](https://github.com/Open-MBEE/OpenSysML/releases):

**Linux (x64; use `opensysml-linux-arm64.tar.gz` on arm64):**
```bash
wget https://github.com/Open-MBEE/OpenSysML/releases/latest/download/opensysml-linux-amd64.tar.gz
tar xzf opensysml-linux-amd64.tar.gz
sudo mv sysml sysml-lsp /usr/local/bin/
chmod +x /usr/local/bin/sysml /usr/local/bin/sysml-lsp
```

**macOS (Intel or Apple Silicon) — Homebrew is the recommended path:**
```bash
brew install Open-MBEE/tap/opensysml
```
This avoids the Gatekeeper prompt described in [macOS: Gatekeeper](#macos-gatekeeper).

Use the fully qualified name rather than tapping first. Homebrew 6 requires third-party taps
to be trusted before their code is loaded. Installing by fully qualified name trusts only this
formula; the two-step form needs an explicit trust step in between:
```bash
brew tap Open-MBEE/tap
brew trust --formula Open-MBEE/tap/opensysml   # or: brew trust Open-MBEE/tap, for the whole tap
brew install opensysml
```

**macOS, direct download (fallback):** use `curl`, not a browser.
```bash
# Apple Silicon; use opensysml-darwin-amd64.tar.gz on Intel
curl -fL -o opensysml.tar.gz https://github.com/Open-MBEE/OpenSysML/releases/latest/download/opensysml-darwin-arm64.tar.gz
tar xzf opensysml.tar.gz
sudo mv sysml sysml-lsp /usr/local/bin/
```

**Windows — use the installer.** Download `opensysml-<x.y.z>-windows-amd64.msi` from
[releases](https://github.com/Open-MBEE/OpenSysML/releases/latest) and run it. A setup wizard
lets you pick the destination folder and the optional components; by default it installs
`sysml.exe`, `sysml-lsp.exe` and `sysml-grpc.exe` into `C:\Program Files\OpenSysML`, puts that
directory on the system `PATH`, and installs the [Z3](https://github.com/Z3Prover/z3) SMT solver
under `C:\Program Files\OpenSysML\z3` (also on `PATH`) as the optional *SMT solver (Z3)*
feature, so the experimental `%check`/`%explain` commands work out of the box. Deselect the
bundled solver or the gRPC service on the wizard's *Choose components* page, or from an elevated
prompt with `msiexec /i opensysml-<x.y.z>-windows-amd64.msi REMOVE=Z3` (or `REMOVE=Z3,GrpcService`); to use
another solver, point `OPENSYSML_SMT` at it (see [Installing a solver](#installing-a-solver-optional)).
A newer installer upgrades an older one in place, and *Apps & features* uninstalls it. Windows
SmartScreen may warn that the publisher is unrecognized: the installer is not yet
Authenticode-signed. Once the project's [SignPath Foundation](https://signpath.org) application
is approved, releases also carry `opensysml-<x.y.z>-windows-amd64-signed.msi`, whose executables
and the MSI itself are signed through SignPath (the bundled `z3.exe` stays unsigned) — prefer it
when it is there (see the [Code signing policy](../../README.md#code-signing-policy)).

**Windows — zip.** Download `opensysml-windows-amd64.zip` (or `opensysml-windows-amd64-signed.zip`
when present) instead, extract, and add the directory to `PATH`; the zip has `sysml.exe` and
`sysml-lsp.exe` only and no solver. [Scoop](https://scoop.sh) and
[winget](https://learn.microsoft.com/windows/package-manager/) manifests are maintained under
[`packaging/`](https://github.com/Open-MBEE/OpenSysML/tree/main/packaging) and will be listed
here once accepted into the public bucket / `winget-pkgs`; both install the zip and declare Z3
as a dependency.

**Available binaries:**
- `sysml` — Interactive REPL
- `sysml-lsp` — Language Server Protocol server

`sysml-grpc`, the service the Python bindings talk to, is published as a bare
`sysml-grpc-<os>-<arch>` file with a `.sha256` sidecar rather than inside an archive, because
the `opensysml` Python package downloads and verifies it itself (see [clients/python/README.md](../../clients/python/README.md)).
`make build-grpc` builds it from source.

**Archive layout:** the `opensysml-<os>-<arch>.tar.gz` bundles contain both binaries under their
plain names (`sysml`, `sysml-lsp`). The older single-binary `sysml-<os>-<arch>.tar.gz` and
`sysml-lsp-<os>-<arch>.tar.gz` archives are still published. The bundles and
`SHA256SUMS.txt` exist from v0.0.4 onward; for earlier releases use the
single-binary archives. The `sysml-grpc` binaries and their sidecars are published from the
next release onward. `SHA256SUMS.txt` covers every archive and every published
`sysml-grpc` binary:

```bash
curl -fLO https://github.com/Open-MBEE/OpenSysML/releases/latest/download/SHA256SUMS.txt
shasum -a 256 -c SHA256SUMS.txt --ignore-missing   # macOS; use sha256sum -c on Linux
```

**Ahead of the next release:** the same archives are built from `develop` every night and
published as the prerelease `nightly`. It is a development build, replaced nightly and never
the `latest` release; [Nightly snapshots](../project/nightly.md) says what it contains, how to
verify one, and what to expect from it.

## macOS: Gatekeeper

If macOS refuses to run a downloaded binary with **"cannot be opened because the developer
cannot be verified"**, the binary is not broken. Browsers attach a `com.apple.quarantine`
extended attribute to downloads, and these binaries are not signed with an Apple Developer ID
or notarized, so Gatekeeper blocks them.

Ways to avoid it, best first:

1. **Install with Homebrew** (`brew install Open-MBEE/tap/opensysml`). Homebrew
   downloads with `curl` and does not quarantine formula binaries. This is the recommended
   path until the releases are signed and notarized.
2. **Download with `curl` or `wget`** (as shown above). They do not set the quarantine
   attribute, so no prompt appears.
3. **Install with a Go toolchain** — built locally, never quarantined:
   ```bash
   go install github.com/Open-MBEE/OpenSysML/cmd/sysml@latest
   go install github.com/Open-MBEE/OpenSysML/cmd/sysml-lsp@latest
   ```
4. **Clear the attribute** if you already downloaded the archive in a browser. Verify the
   checksum first: clearing the attribute disables a security check, so make sure the file
   really is the published one:
   ```bash
   shasum -a 256 opensysml-darwin-arm64.tar.gz   # compare against SHA256SUMS.txt
   xattr -d com.apple.quarantine /usr/local/bin/sysml /usr/local/bin/sysml-lsp
   ```
   `xattr -d: No such xattr` means the file was never quarantined. Use
   `xattr -c <file>` to clear all attributes, or `xattr -dr com.apple.quarantine <dir>` for a
   directory.

See [MACOS_DISTRIBUTION.md](../project/macos-distribution.md) for the root-cause analysis and for what
signing and notarizing the releases would take.

## Installing a solver (optional)

Nothing above needs an SMT solver. The whole guide, and every normative check
(`%constraint`, `%requirement`, `%satisfy`, `%eval`), runs on the concrete evaluator, which is
the normative implementation. A solver is needed only by the **experimental**
`%check` and `%explain` commands, which ask whether a constraint *can* be satisfied rather than
whether it holds for a given object (see [reference/repl-commands.md](../reference/repl-commands.md)).

The solver is a separate program. OpenSysML runs it as a child process and talks to it in
SMT-LIB2; nothing is linked in and nothing is bundled in the release archives, which stay single
static binaries. Either [z3](https://github.com/Z3Prover/z3) (MIT) or [cvc5](https://github.com/cvc5/cvc5)
works; install z3 unless you have a specific reason to prefer cvc5. The solving commands
follow the design of the `ConstraintSolverService` in OpenMBEE's
[HMF](https://github.com/hivecore-dev/hmf) (Apache 2.0); see
[Acknowledgements](../../README.md#acknowledgements).

**macOS and Linux, Homebrew — automatic:** z3 is a dependency of the formula, so the
recommended install already gives you a working `%check`:
```bash
brew install Open-MBEE/tap/opensysml   # installs z3 too
brew install z3                        # or just the solver, next to a non-brew sysml
```

**Debian and Ubuntu:**
```bash
sudo apt install z3          # provides /usr/bin/z3
```

**Other Linux distributions** — each of these packages provides a `z3` executable:
```bash
sudo dnf install z3          # Fedora
sudo pacman -S z3            # Arch (extra/z3)
sudo apk add z3              # Alpine (community repository)
nix-shell -p z3              # nixpkgs, for one shell; or: nix profile install nixpkgs#z3
```
Only the apt command above was run during testing; the others come from the
distributions' package indexes (Fedora's `z3`, Arch's `extra/z3`, Alpine's `community/z3` and nixpkgs'
`z3`, each of which ships a `z3` program). If one of them fails, the most likely reason is that
the distribution has since renamed or dropped the package.

**Windows:** download the official prebuilt archive from
[z3's releases](https://github.com/Z3Prover/z3/releases): `z3-<version>-x64-win.zip` (for
example `z3-5.1.0-x64-win.zip`; `arm64` and `x86` builds are published too). Unzip it and
either add the archive's `bin` directory to `PATH`, or point `OPENSYSML_SMT` at the executable:
```powershell
$env:OPENSYSML_SMT = "C:\tools\z3-5.1.0-x64-win\bin\z3.exe"
```
[Scoop](https://scoop.sh) packages the same archive, so `scoop install z3` places `z3.exe` on
`PATH` automatically. The OpenSysML MSI installer bundles this same release under
`C:\Program Files\OpenSysML\z3` (on `PATH`) as its *SMT solver (Z3)* feature; deselect the
feature (`REMOVE=Z3`) if you want a different solver, or just set `OPENSYSML_SMT`, which takes
precedence over `PATH`.

**Any platform with Python — the `pip` fallback:** the `z3-solver` wheels (MIT) are published
for Linux, macOS and Windows and include the executable, not just the Python module:
```bash
python3 -m venv .venv
.venv/bin/pip install z3-solver     # z3 lands in .venv/bin/z3
```
Activating the virtual environment puts `z3` on `PATH`, and nothing else is needed. If you
prefer not to activate it, point at the executable instead:
```bash
OPENSYSML_SMT=$PWD/.venv/bin/z3 sysml model.sysml
```

**cvc5, the alternative backend:** there is no Homebrew formula and no Debian/Ubuntu package.
Download a prebuilt archive from [cvc5's releases](https://github.com/cvc5/cvc5/releases)
(`cvc5-Linux-x86_64-static.zip`, `cvc5-macOS-arm64-static.zip`, `cvc5-Win64-x86_64-static.zip`
and so on) and put its `bin/cvc5` on `PATH`. cvc5 is under a modified BSD licence, but
its default build links GMP under LGPL-3, and it can be built against GPL libraries (the
`*-gpl` archives are those builds). Those terms govern **redistributing** cvc5; they
do not affect the terms under which you may use OpenSysML, because OpenSysML links neither
solver.

### Solver compatibility — pointing the driver at another solver

`OPENSYSML_SMT` accepts **any** executable that reads SMT-LIB2 on standard input and answers on
standard output, not only z3 and cvc5. The backend has to support the subset of SMT-LIB the
generated scripts use:

| Feature | What is emitted | z3 4.8.12 | cvc5 1.3.4 |
|---|---|---|---|
| Model output | `(set-option :produce-models true)` with `(get-value …)` | yes | yes |
| Unsat cores | `(set-option :produce-unsat-cores true)`, `:named` assertions, `(get-unsat-core)` | yes | yes |
| Incremental dialogue | more than one `(check-sat)` in a script, for `%configure … all` | yes | yes |
| Enumerations and variants | `(declare-datatypes …)` with nullary constructors | yes | yes |
| Strings | the `String` sort, compared for equality | yes | yes |
| Integer remainder | `div` and `mod` from the Ints theory | yes | yes |
| Nonlinear arithmetic | a product or quotient of two non-literal terms | yes | yes |
| Mixed arithmetic | the `AUFLIRA`/`AUFNIRA` logics, for a query over `Int` and `Real` | yes | yes |
| Non-standard logic | `(set-logic ALL)`, which datatypes and strings need | yes | yes |
| Objective optimization | `(maximize …)`/`(minimize …)`, for `%optimize` | yes | **no** — parse error |
| Objective priority | `:opt.priority`, a z3 extension | yes | **no** — answers `unsupported` |

The two columns record what each solver actually answered when probed on the test machine, not
what its documentation claims. The last two rows are the only things cvc5
lacks, and `%optimize` is the only command that needs them: on cvc5 it declines and names
the missing extension. Every other command works on either solver.

Apart from the non-standard `ALL`, every logic a script sets is a standard SMT-LIB 2.6 logic
([the logic list](https://smt-lib.org/logics.shtml)): `QF_UF`, `QF_LIA`, `QF_NIA`, `QF_LRA`,
`QF_NRA`, `AUFLIRA`, `AUFNIRA`, whichever is the narrowest that covers what the query uses. `ALL`
is used only where the standard defines no logic for the feature at all (datatypes, strings),
and the script says so in a comment on the line above `(set-logic ALL)`.

A backend is probed the first time it is used, with one small script per feature the query
needs, and the results are cached for the process. Anything the backend refuses is reported
rather than worked around:

```
sysml> %explain P::C
error: the SMT solver does not support a feature this query needs: mysolver does not support
SMT-LIB 2.6 unsat cores: `:produce-unsat-cores` with `:named` assertions and `(get-unsat-core)`
(unsat-cores), which explaining a conflict needs: it rejected the script: unsupported;
install a solver that supports it or set OPENSYSML_SMT to one
```

This is one of three ways a solver run can end without a verdict. A solver
that crashes, exits or answers unreadably produces a solver *process* error naming the stage at
which it failed, and a solver that answers without deciding produces the verdict `unknown`
together with the reason it gave. In none of these cases is a verdict guessed.

To validate another solver end to end, run the portability harness against it. The harness
reports each feature as `pass`, `refuse` (the backend said it does not support the feature) or
`fail` (the backend rejected a script, which you should report as a bug):

```bash
OPENSYSML_SMT=/path/to/mysolver go test ./internal/core/solve -run TestPortability -v
```

### Verifying the solver is found

`%check` names the solver it used, so the verdict line tells you which one was found:

```
sysml> %check P::C
✗ Constraint C is unsatisfiable (z3, 8ms)
```

The solver is found in this order: `OPENSYSML_SMT` first (an executable name or a path; if it
names something that does not exist, that is an error, not a silent fallback), then `z3` on
`PATH`, then `cvc5`. When both are installed, z3 wins regardless of where they sit in `PATH`.
`OPENSYSML_SMT_TIMEOUT` (default `10s`) limits a single query; when it expires the verdict is
`unknown` rather than an error. See [reference/environment.md](../reference/environment.md).

If no solver is installed anywhere, `%check` and `%explain` say so instead of giving a verdict, and
every other command is unaffected:

```
sysml> %check P::C
error: no SMT solver found: install z3 (`apt install z3`, `brew install z3`) or cvc5, or set OPENSYSML_SMT to a solver executable; looked for [z3 cvc5] on PATH
```

## From source

**Prerequisites:**
- Go 1.25 or later
- Git
- Make (optional but recommended)

**Build:**
```bash
git clone https://github.com/Open-MBEE/OpenSysML.git
cd OpenSysML
make build       # builds bin/sysml, bin/sysml-lsp, and bin/sysml-grpc
# OR
go build -o sysml ./cmd/sysml
go build -o sysml-lsp ./cmd/sysml-lsp
```

**Install (optional):**
```bash
make install     # installs to $GOPATH/bin
# OR
sudo mv bin/sysml bin/sysml-lsp bin/sysml-grpc /usr/local/bin/
```

**Install binaries and manual pages, the way a package does:**
```bash
sudo make install-tree prefix=/usr/local
man sysml
```

`install-tree` honours the usual GNU variables — `DESTDIR`, `prefix`,
`exec_prefix`, `bindir`, `datarootdir`, `mandir`, `man1dir` — so a distribution
package stages it without writing outside its build root:

```bash
make install-tree DESTDIR="$pkgdir" prefix=/usr
```

It installs `sysml`, `sysml-lsp` and `sysml-grpc` into `$(bindir)`, and
`sysml.1`, `sysml-lsp.1` and `sysml-grpc.1` into `$(man1dir)`. The pages are
committed under `man/man1`, so building a package needs no documentation
converter and no network. They are generated from each command's own
description: `make man` rewrites them and `make man-check` (which CI runs)
fails if a committed page is not what the command renders. Each binary can also
write its own page, which is what `make man` calls:

```bash
sysml -man > sysml.1
```

Homebrew installs the pages for the binaries it ships, from the release bundle,
so `man sysml` and `man sysml-lsp` work after
`brew install Open-MBEE/tap/opensysml`. `sysml-grpc` is published raw for the
Python client rather than in the bundle, so its page comes from a source
install.

---

Next: [2. Your first model](02-first-model.md).

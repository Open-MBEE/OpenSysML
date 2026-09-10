# Contributing to OpenSysML

Thank you for your interest in contributing to OpenSysML. This document describes how to set up a
development environment, the standards a change is expected to meet, and how contributions are
reviewed and released.

## Development Setup

### Prerequisites

- Go 1.25 or later
- Git
- Make (recommended for build automation)

### Clone and Build

```bash
git clone https://github.com/Open-MBEE/OpenSysML.git
cd OpenSysML
make build  # builds bin/sysml, bin/sysml-lsp, and bin/sysml-grpc with version info
make test   # runs all tests
make lint   # runs staticcheck and gosec, as CI does

./scripts/download-training-examples.sh   # fetch the OMG corpus the gate needs
```

The OMG training-corpus gate (`internal/core/model/training_examples_test.go`) skips while
`examples/sysml-v2-training/` is absent, so run the download script once before trusting a
local `make test`. CI runs the script itself and sets `OPENSYSML_REQUIRE_TRAINING_CORPUS=1`,
which makes a missing corpus a failure there instead of a skip.

## Development Workflow

For an implementation-focused tour of the core pipeline and practical recipes
for changing syntax, semantics, lowering, and runtime behavior, read
[DEVELOPING.md](DEVELOPING.md). It is repository-only developer documentation
and is not published on the website.

### Building

```bash
# Build all binaries with version info
make build

# Build specific binary
make build-sysml
make build-lsp
make build-grpc

# Install to $GOPATH/bin
make install

# Protobuf schema (api/proto/sysml.proto), all codegen driven by buf
make proto           # regenerate the Go, Java and Python stubs
make proto-buf       # Go and Java stubs only
make python-proto    # Python stubs only (needs grpcio-tools; PYTHON=... picks the interpreter)
make proto-lint      # lint the schema, as CI does
make proto-breaking  # reject wire-breaking changes against develop, as CI does

# Python gRPC bindings
make python-install  # install opensysml package
make python-test     # run Python binding tests

# Clean build artifacts
make clean
```

### Running Tests

```bash
# All tests (using Makefile)
make test

# All tests (direct)
go test ./...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# With race detector
go test -race ./...

# Short tests (faster, no race detector)
make test-short

# Specific package
go test ./internal/core/parser
```

**Parser-specific tests:** When modifying the parser, ensure the four-layer test contract passes:

1. **Conformance gate:** `go test -run TestStdlibConformance ./internal/core/libs`
2. **Golden ASTs:** `go test -run TestGolden ./internal/core/parser`
3. **Negative tests:** `go test -run TestNegative ./internal/core/parser`
4. **Update goldens** (after intentional changes): `go test -run TestGolden -update ./internal/core/parser`

See [docs/internals/architecture.md](docs/internals/architecture.md#parser-test-contract) for full details on the parser testing contract.

### Static Analysis

`make lint` runs the same two checks CI gates on, at the versions pinned in the
Makefile:

- **staticcheck** — must report nothing. Unused code is deleted rather than left
  behind; if a helper is genuinely needed by work in flight, land it with its
  caller.
- **gosec** — must report nothing. Generated protobuf code is excluded
  (`-exclude-generated`). Suppress a finding with `#nosec <rule>` **only** with a
  comment saying why it is safe; do not widen the exclusion list.

### Code Style

- Use `gofmt` (enforced by CI)
- Follow [Effective Go](https://go.dev/doc/effective_go)
- Write tests for new functionality
- Document exported types and functions

### Commit Messages

Follow conventional commits format:

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Test additions/changes
- `refactor`: Code refactoring
- `chore`: Build/tooling changes

**Examples:**
```
feat(parser): add support for state machine transitions
fix(runtime): handle null values in expression evaluation
docs: update Quick Start guide with new commands
test(semantics): add conformance checking test cases
```

## Branches

The repository follows git-flow with two long-lived branches:

- `develop` is the integration branch and the default branch. Every `feature/`, `fix/`, `docs/`,
  `ci/` and `test/` branch is cut from `develop`, and its pull request targets `develop`.
- `main` is the release branch. It receives only `release/x.y.z` pull requests (cut from
  `develop`, with the changelog fragments folded in) and `hotfix/` pull requests. Release tags —
  `v*` and the client package tags — are created on `main`. After a release is tagged, `main`
  is merged back into `develop` (a plain merge, no rebase) so hotfixes and the folded changelog
  flow down. See [docs/project/releasing.md](docs/project/releasing.md).

## Pull Request Process

1. **Fork** the repository
2. **Create a branch** from `develop`: `git checkout -b feat/my-feature`
3. **Make changes** with clear commit messages
4. **Run tests**: `go test ./...`
5. **Push** to your fork
6. **Open a Pull Request** targeting `develop`

### PR Guidelines

- Include tests for new features
- Update documentation as needed
- Ensure CI passes (build + tests)
- Keep PRs focused (one feature/fix per PR)
- Respond to review feedback

## Release Process

**For maintainers:**

### Creating a Release

1. **Update version** (if needed in code)
2. **Merge the `release/x.y.z` pull request** into `main` (see
   [docs/project/releasing.md](docs/project/releasing.md))
3. **Tag the release** on `main`:
   ```bash
   git tag -a v0.1.0 -m "Release v0.1.0: Initial public release"
   git push origin v0.1.0
   ```
4. **CI automatically:**
   - Builds binaries for all platforms
   - Publishes to GitHub Releases
5. **Merge `main` back into `develop`**

### Release Checklist

- [ ] All tests pass
- [ ] Documentation updated
- [ ] Changelog fragments folded in (`python3 scripts/changelog.py release X.Y.Z`)
- [ ] Version tag follows semver (`vX.Y.Z`)
- [ ] Version bump (patch or minor) justified against [§ Versioning](#versioning)
- [ ] Release notes prepared

### Versioning

Versions are `0.MINOR.PATCH` until 1.0 and follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html), whose §4 leaves the meaning of the
segments open while the major version is zero. This project decides the segment by **model
compatibility**, not by the size of the release or the number of features it carries.

A release is a **PATCH** when everything the previous release accepted still behaves the same:

- Every model the previous release accepted is still accepted, with the same diagnostics.
- A model that ran gives the same results. The one exception is a bug fix that turns a wrong
  result into the one the Kernel Semantic Library derives; that is compatible and is listed under
  *Fixed*.
- No CLI flag, REPL command, RPC or wire field is removed or renamed. The protobuf
  wire-compatibility check passes against the previous release's schema:
  `make proto-breaking BUF_BREAKING_REF=origin/main` on the release branch.

New features, new flags, new capabilities and new wire fields are all patch material.

A release is **MINOR** when any of that does not hold:

- A construct the previous release accepted is refused.
- A result the previous release derived correctly changes.
- A flag, command, RPC or wire field is removed or renamed.
- A fixture or output format changes so that existing user artifacts fail — an `.expected.json`
  schema, the trace goldens, the shape of a `-json` report.

**MAJOR** is reserved for 1.0; from 1.0 on, the conventional rule applies — MAJOR for breaking
changes, MINOR for features, PATCH for fixes.

The changelog entry for a release says nothing about which segment was bumped or why; the
decision is recorded through the Release Checklist item above.

## CI/CD

### GitHub Actions

`.github/workflows/pr.yml` is the only CI a pull request runs, and it gates them: gofmt,
`go vet`, `make lint`, the race-enabled test suite, the binaries, the client suites, the
conformance suite over each transport, the protobuf lint and wire-compatibility checks, the
documentation hygiene checks (`make docs-check`, `make man-check` and the census check), the
site build, and in each client's job the release-digest copy it ships and the stubs it commits
(regenerated with the pinned buf and diffed). It downloads the OMG
corpora before the suite and runs each corpus gate as its own step, so those gates are required
rather than skipped. Its `Build and test` job aggregates the rest, so that is the one check
branch protection needs to require.

Its first job, `Changed areas`, runs `scripts/ci-changed-areas.sh` over the pull request's
files and the rest of the jobs are gated on what it reports: a change confined to one client
runs that client's job alone, while a change to the Go sources, the proto, `conformance/` or
the workflows runs everything. A path no area claims turns every area on, so a new directory
is over-tested rather than untested — teach the script about it, and add a case to
`scripts/ci-changed-areas-test.sh`, which the same job runs.

`.github/workflows/devin-review.yml` is not a check. It asks the Devin API for a review of the
pull request as soon as it opens or gains commits, draft or not, so the review runs alongside
`pr.yml` rather than waiting for the pull request to be marked ready. It needs the repository
secret `DEVIN_API_TOKEN`, a Devin service-user API key with the organization's
"Use Review (manual)" permission, and fails visibly when that secret is absent.

### CircleCI

`.circleci/config.yml` runs after a merge and on tags, never on a pull request branch:

**On every push to `main` or `develop`:**
- The same suite, gates, client tests and conformance runs as the pull-request workflow, over
  the merged tree, plus the host binaries
- SonarCloud scan, fed by the Go and client coverage reports

The conformance job stores its JSON report as an artifact and its JUnit XML
(`bin/conformance-report.xml`) as test results, so a failing scenario is named in the Tests tab
rather than only in the log. The README badge tracks this workflow.

**On tags (`v*`, and the client package tags):**
- Run the suite again on the tagged revision
- Build release binaries (all platforms)
- Create GitHub Release and upload binaries
- Publish the client packages

### Required Checks

PRs must pass the GitHub Actions `Build and test` check, which requires:
- [ ] Build succeeds
- [ ] All tests pass
- [ ] No race conditions
- [ ] Code formatted (`gofmt`)
- [ ] Static analysis clean (`make lint`: staticcheck and gosec)
- [ ] OMG corpus gates run (not skipped) and match their expectations
- [ ] Protobuf schema lints and is wire-compatible with the base branch
- [ ] Documentation checks pass (`make docs-check`, `make man-check`) and the site builds

## Project Structure

```
github.com/Open-MBEE/OpenSysML
├── cmd/                    # Binaries (sysml, sysml-lsp, sysml-grpc)
├── internal/core/          # Core implementation
│   ├── source/            # Source file handling
│   ├── lexer/             # Tokenization
│   ├── parser/            # Parsing
│   ├── ast/               # AST nodes
│   ├── symbols/           # Symbol tables
│   ├── resolve/           # Name resolution
│   ├── semantics/         # Type system
│   ├── passes/            # Validation
│   ├── lower/             # AST → execution IR (ActionGraph/StateGraph)
│   ├── runtime/           # Execution
│   ├── model/             # Workspace
│   └── libs/              # Standard library bundling
├── internal/lsp/          # LSP implementation
├── internal/grpc/         # gRPC service implementation
├── internal/repl/         # REPL implementation
├── clients/python/        # Python client bindings (opensysml)
├── clients/rust/          # Rust client (opensysml) and its conformance runner
├── docs/                  # Documentation
│   ├── guide/             # The handbook, in reading order
│   ├── reference/         # CLI, REPL, environment, APIs, RDF mapping
│   ├── internals/         # Architecture, testing, performance, design notes
│   └── project/           # Compliance, roadmap, releasing, measurements
├── testdata/              # Test fixtures
└── .circleci/             # CI configuration
```

## Documentation

Documentation is organized by what a reader wants, not by the feature that landed. Four areas,
mapped in [docs/README.md](docs/README.md) and published as
<https://opensysml.org/>:

- **[docs/guide/](docs/guide/)** — *how do I use it?* A numbered handbook read in order.
- **[docs/reference/](docs/reference/)** — *what does this flag, command, API or triple mean?*
  Look-up material, exhaustive rather than narrative.
- **[docs/internals/](docs/internals/)** — *how is it built?* For someone changing the code.
- **[docs/project/](docs/project/)** — *where does the project stand?* Compliance, roadmap,
  release and testing process, and measured counts.

When a change needs documenting:

- **Extend the chapter that already covers the surface** rather than adding a page per feature —
  a new REPL command belongs in [reference/repl-commands.md](docs/reference/repl-commands.md) and
  the chapter that teaches the workflow, not in a new `MY_FEATURE.md`.
- **Explain in the guide, enumerate in the reference.** Do not repeat a flag table in both; link to it.
- **Measured numbers have one home** (fixture, corpus and conversion counts live in
  [docs/project/](docs/project/)); elsewhere, link to it instead of restating a number that
  will drift. Three release-gate surfaces are the deliberate exception, because
  [docs/project/releasing.md](docs/project/releasing.md) checks the numbers they print:
  `README.md`'s coverage line and status table, and the gate tables in
  [docs/project/roadmap.md](docs/project/roadmap.md) and
  [docs/project/training-examples.md](docs/project/training-examples.md). Recount all four
  together, in one commit.
- **The compliance-row census is counted at build time, never committed.** Adding or changing a
  `✅`/`⚠️`/`❌`/`⛔` row in [docs/project/spec-compliance.md](docs/project/spec-compliance.md) is the
  whole change: no header, `README.md` line or `docs/internals/architecture.md` line restates the
  count, so two branches that both add rows cannot conflict on one. The site build
  (`scripts/mkdocs_census.py`, run by `make docs`) counts the rows into the
  `<!-- doc-counts:begin census -->` block and refuses a `🚧` row, as do `make docs-counts` and
  `go test ./cmd/pilot-diff`. `make docs-counts` still restates the externally refereed oracle
  numbers from the baseline JSONs; run it only when a baseline moved.
- **Changelog entries are fragments, not edits to `CHANGELOG.md`.** A change that a user
  should read about adds one file, `changes/unreleased/<slug>.<section>.md`, holding the list
  item(s) for that section (`added`, `changed`, `fixed`, …); see the README there. Two branches
  then never touch the same changelog lines. `python3 scripts/changelog.py check` validates the
  fragments and runs in CI; the release procedure folds them into `CHANGELOG.md`.
- **Internal work-item labels stay out of what a reader reads.** Waves and slices (`wave 12A`,
  `W8G`), follow-up rows (`F4`), adjudication probes (`P1`) and diagnostic classes (`K5`, `S10`)
  have no public referent, so `CHANGELOG.md`, `README.md`, the guide, the reference, the internals
  pages — and PR bodies and release notes — say what the change did instead. `python3
  scripts/check-doc-ids.py` fails on one and runs in CI. The `docs/project/` conformance records
  are the exception: they cross-reference each other by these labels and each defines them in a
  note at the top. A real keyboard shortcut is not an internal label; spell it `<kbd>F2</kbd>`.
- **An oracle total quoted outside the generated block is a snapshot, and says so.** The totals of
  the differential, Xpect and rejection oracles move with every rule, fixture and pin change, so
  only the `doc-counts` block states the current ones. A page that quotes a figure from the round it
  documents must say it is not the current baseline; `python3 scripts/check-doc-figures.py` fails on
  one that does not, and runs in CI.
- **A new page goes in the `nav:` of [mkdocs.yml](mkdocs.yml)**, in the reading order of its
  area, or it is published but unreachable from the site's navigation. `make docs-install`
  once, then `make docs` builds the site the way CI does and `make docs-serve` previews it.
- **Moving or renaming a page means updating its inbound links.** Run
  `python3 scripts/check-doc-links.py` — it fails on a link to a missing file or heading and
  runs in CI. Where a released `README.md` linked the old path, leave a one-paragraph pointer
  behind there (`docs/ARCHITECTURE.md` and friends are such pointers); a page only ever linked
  from inside `docs/` is moved outright.

## Architecture

See [ARCHITECTURE.md](docs/internals/architecture.md) for detailed design.

**Key principles:**
- **Immutable AST:** Syntax-only, never mutated
- **Side tables:** Semantic info keyed by node/symbol
- **Lazy evaluation:** Compute on-demand, memoize
- **Incremental:** Invalidate only affected parts

## Testing Philosophy

- **Unit tests:** Per-package (`*_test.go`)
- **Integration tests:** Cross-package scenarios
- **Fixtures:** Real SysML v2 models in `testdata/`
- **Golden files:** Expected outputs (where applicable)

## Getting Help

- **GitHub Issues:** three forms, picked when you open one — a **tool bug** (OpenSysML does
  something other than what it documents), a **spec conformance gap** (it disagrees with SysML v2
  or KerML, so name the clause), or an **objection to a ruling** (the behavior is deliberate and
  the report argues that the interpretation is wrong). Check
  [spec-compliance.md](docs/project/spec-compliance.md) and
  [omg-issues.md](docs/project/omg-issues.md) first: a known divergence is usually already a row
  there with the reasoning behind it.
- **Discussions:** questions, ideas, feature requests
- **Pull Requests:** code contributions. The template asks for the specification basis of a
  behavior change and how it was verified.

## Code of Conduct

- Be respectful and inclusive
- Focus on constructive feedback
- Help others learn and grow
- Follow project conventions

## License

By contributing, you agree that your contributions will be licensed under the same license as the project (see [LICENSE](LICENSE)).

---

**Thank you for contributing to OpenSysML.**

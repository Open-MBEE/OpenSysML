# .NET client follow-ups: session instructions

Development record for the .NET client, kept beside [`DOTNET_CLIENT_PLAN.md`](DOTNET_CLIENT_PLAN.md).
It turns suggestions 1–4 of the .NET client review into four self-contained work sessions. Each
session can be handed to an agent (or a person) on its own; read **Common setup** first, then the
one task. Each session is cut as its own branch from `feat/dotnet-client` and merged back into it
when done; a finished session records its commit and implementation notes under its heading.

| # | Session | Depends on | Size | Status |
|---|---------|------------|------|--------|
| 1 | Regenerate the C# protocol code and gate it against drift | — | Small–medium | **Done** (`28733b9de`) |
| 2 | Keep `OPENSYSML_GRPC_TOKEN` out of private services; add first-class token support | — | Medium | **Done** (`d654510ae`) |
| 3 | Bump the client version to 0.11.0 and resync the embedded release digests | — | Small | **Done** (`f0d664ab4`) |
| 4 | Implement the missing service features and adjudicate the failing integration tests | **1** (needs the regenerated wire types) | Large | **Done** (`9bc987de7`); one service bug drafted for upstream |

Sessions 1, 2 and 3 are independent of each other. Session 4 must start from a tree that has
session 1 merged in. Suggested order: 1 → 3 → 2 → 4 (3 is quick and turns two gates green).

> **`AGENTS.md` governs every session.** Read [`AGENTS.md`](../../../AGENTS.md) in full before starting any
> session, and follow it for *how* to implement everything here: correctness over expedience, root
> cause first, never regress, tests as the contract, no dead code, full-feature completeness
> (§8), commit and changelog conventions (§7), and the definition of done (§2). This document says
> *what* to do and where. Where it suggests an approach, `AGENTS.md` decides *how*. If the two ever
> conflict, `AGENTS.md` wins: stop, flag the conflict and resolve it on `AGENTS.md`'s terms rather
> than following this document. Also read the documents `AGENTS.md` points to
> (`CONTRIBUTING.md`, `docs/internals/architecture.md`) and the project skill
> `.agents/skills/testing-dotnet-client/SKILL.md`.

---

## Common setup (every session)

### Branch

All four land on the .NET client feature branch, `feat/dotnet-client`, either directly or
through a short-lived branch cut from it (`fix/dotnet-…` / `feat/dotnet-…`) and merged back. Pull
the latest `feat/dotnet-client` first; if `develop` has moved since, merge it in before starting so
the service you test against is current.

### Toolchain

Run every command from the repository root (`global.json` there pins SDK 10.0.402 and the
Microsoft.Testing.Platform runner). The pinned SDK is installed privately in `~/.dotnet`;
Homebrew's `dotnet` is 10.0.403 and `make dotnet-preflight` rejects it.

```bash
export DOTNET_ROOT=$HOME/.dotnet PATH=$HOME/.dotnet:$PATH
# Interop tests need a real JDK; /usr/bin/java on this machine is the macOS stub.
export JAVA_HOME=/opt/homebrew/opt/openjdk PATH=/opt/homebrew/opt/openjdk/bin:$PATH
make build-grpc                                   # rebuild bin/sysml-grpc after every merge
export OPENSYSML_GRPC_BINARY="$PWD/bin/sysml-grpc" OPENSYSML_REQUIRE_SERVICE=1 OPENSYSML_REQUIRE_INTEROP=1
```

Without the two `REQUIRE` variables, service and interop tests *skip by name* and a green run
proves nothing about them.

### The full gate (run at the end of every session)

```bash
make dotnet-test                  # format, 0-warning build, whole suite, Vehicle sample, gate self-tests
make conformance-dotnet           # shared scenario suite over grpc, connect, connect-json
make conformance-dotnet-check     # scenario-by-scenario against the reference runner
python3 scripts/sync-release-digests.py --check
python3 scripts/changelog.py check
python3 scripts/check-doc-links.py && python3 scripts/check-doc-ids.py
```

`client/python/scripts/check_version.py` needs the `packaging` module and a tag:
`python3 -m venv /tmp/v && /tmp/v/bin/pip install packaging && CIRCLE_TAG=v0.11.0 /tmp/v/bin/python client/python/scripts/check_version.py --dotnet`.

Focused test runs:

```bash
dotnet test --project client/dotnet/test/OpenMBEE.OpenSysML.Tests -c Release \
  --filter 'FullyQualifiedName~ClassName'
```

(`dotnet test <solution>`, the VSTest form, does not work here; use `--project`/`--solution` or the
Make targets.)

### Baseline at the time of writing

On `feat/dotnet-client` at `2fb62f01d` with `develop` merged in, `make dotnet-test` failed 30 of
3,242 tests: 6 only because no JDK was found (they pass with `JAVA_HOME` set as above) and **24
real**, all addressed by these sessions:

| Failing test (×3 = once per wire) | Cause | Session |
|---|---|---|
| `CapabilitiesTests.EveryCapabilityTheServiceDeclaresHasAConstantAndNoConstantIsInvented` | 5 capabilities without constants | 4 |
| `ReleaseDigestsTests.EmbeddedCopyIsTheRepositoryTable`, `…EmbeddedTablePinsEveryAssetOfTheRepositoryTable` | embedded digest table lacks `v0.11.0` | 3 |
| `ServiceQueryDocsTests.EveryViewTheRenderersOwnTestsDeclareIsReadWithItsNodesInOrder` ×3 | `nodes[2] repeats the id "n0"`: compartment rows the stale wire cannot see | 1 then 4 |
| `ServiceQueryDocsTests.QueriesSelectWhatTheServiceEvaluatesInEveryConstraintForm` ×3 | service now refuses a one-member `CompositeConstraint` | 4 |
| `ServiceValueTests.EveryKindARequestCarriesComesBackEqualFromTheService` ×3 | `Echo::same` answers `(7)` where `7` was expected | 4 |
| `ServiceVerificationTests.ACalculationTakesStructuredArgumentsAndFailsInBand` ×3 | same `(5)` shape change | 4 |
| `ServiceVerificationTests.AnAnalysisRunsOnItsSubjectWithItsArgumentsIsExploredAndKeepsWhatAFailedRunComputed` ×3 | same `(12.0)` shape change | 4 |
| `ServiceVerificationTests.ASweepTablesEveryRowKeepsAFailedRowAndDrawsReproducibly` ×3 | `InvalidCastException: SequenceValue → RealValue`, same cause | 4 |
| `ServiceSamplingTests.ARecordedRunIsANewModelHoldingTheRecordsAndTheOriginalIsUntouched` ×3 | same `(6.0)` shape change | 4 |

`make conformance-dotnet-check` also failed: `render_view…: response nodes.1.ports.0.inherited:
only the reference run has it` (sessions 1 and 4).

### House rules that apply to every session (from `AGENTS.md`)

These are reminders, not a summary. **`AGENTS.md` is the authority for how all of this work is
done**; re-read it at the start of each session, and when a decision isn't covered here, decide it
the way `AGENTS.md` says to. In particular:

- **Prime directive:** never choose hacky over correct. Fix root causes upstream, not symptoms.
  No stubs, special cases or bridges to make a test pass (`AGENTS.md` §1, §8, §9).
- **Root-cause first** (§1.2, §6): reproduce, read the exact error and find the layer at fault
  before editing. This matters for session 4's adjudications.
- **Features are built completely** (§8): don't minimise the diff at the expense of completeness,
  and don't silently narrow scope. Anything left out is a stated known limitation, not a hidden gap.
- **Every unsupported path is a typed error with a clear message**, never a silent no-op or wrong
  result (§8).
- **Definition of done** (§2): the build, vet/format and the full suite pass, and you paste the
  results. For this client that means the gate above.
- No hacks, no weakened or deleted tests. A test changes only when it is provably wrong, and the
  commit says why (§1.5).
- `DOTNET_MIN_TESTS` in the `Makefile` is a floor on executed tests: raise it when you add tests,
  never lower it.
- Every test declares `[Trait(Category.Name, Category.Unit|Integration|Interop)]`.
- New public API must be recorded in the project's `PublicAPI.Unshipped.txt` (the analyzer fails the
  build otherwise; the IDE code fix for RS0016 writes the lines, or add them by hand in the same
  format).
- Public members need XML docs (CS1591 is an error). Keep code comments to one or two lines.
- Changelog: the .NET client has not shipped, so extend the matching existing fragment in
  `changes/unreleased/dotnet-client-*.added.md` rather than adding a `fixed` entry for something no
  release had. Never edit `CHANGELOG.md`.
- No internal work-item labels in anything a user reads (docs, messages, commit/PR text), and no
  narration of the session's own process (§7).
- Conventional Commits, e.g. `fix(dotnet): …`, with the `Co-authored-by` trailer (§7).
- Remove interim scaffolding, debug prints and dead code before finishing (§1.6, §8).

---

## Session 1 — Regenerate the C# protocol code and gate it against drift

> Before starting: read [`AGENTS.md`](../../../AGENTS.md) and the **Common setup** section above. `AGENTS.md` governs how everything in this session is implemented, tested, committed and documented; where this section and `AGENTS.md` differ, follow `AGENTS.md`.

### Status: done

`28733b9de` *fix(dotnet): regenerate the C# protocol code and hold it to the schema*, on branch
`fix/dotnet-wire-drift`, fast-forwarded into `feat/dotnet-client` and pushed.

Implementation notes:

- **The diff is the schema's alone.** Regenerating at `7c1e0554d` (when the stubs were last made)
  with the pinned buf and plugins reproduced the committed stubs byte for byte, so plugin output
  had not drifted and `Directory.Packages.props` needed nothing. `Sysml.cs` gained only the listed
  types; `SysmlGrpc.cs` did not change (no RPC was added). No `switch` over a proto enum exists:
  `Internal/QueryWire.cs` casts the public enum, so the new operators wait for session 4.
- **`parity.json`** needed only the 9 new fields and 2 new messages (`VerdictNote`,
  `LibraryReference`) an RPC reaches, all `deferred`. `QueryOutcome` and its children are reachable
  from no response, so the checker does not ask for them; the inventory lists no enum values. The
  file round-trips through `json.dumps(indent=2)` byte for byte, which is how it was edited.
- **CI.** `.github/workflows/pr.yml`'s Linux .NET job already regenerated and diffed the C# stubs;
  only CircleCI's `dotnet-test` job lacked it, and now has it (buf installed to `/usr/local/bin`,
  as the image runs as root). A schema change is a service-area path, which fans out to the .NET
  job in the full tier, as for the other clients' stub checks.
- **Local check.** `WireSchemaTests.TheCommittedStubsAreTheSchema` (Integration) runs
  `buf build api/proto --as-file-descriptor-set -o -#format=binpb` and compares `sysml.proto` with
  `Wire.SysmlReflection.Descriptor.ToProto()` as a declaration-order outline (`WireSchema.Outline`):
  messages, fields with proto3 `optional`, oneofs, reserved ranges and names, enums, RPCs with
  streaming. Options, JSON names and source info are ignored. Unit theories prove each kind of
  change is named and each ignored difference is not. Proven load-bearing twice: an added field
  in `sysml.proto` fails naming it, and the old stubs fail naming `Verdict.notes`.
- **`BufGate`** reads `$BUF` as the Makefile does (whitespace-split command line, so
  `BUF='go run github.com/bufbuild/buf/cmd/buf@v1.57.2'` works), else `buf` on `PATH`;
  `OPENSYSML_REQUIRE_BUF=1` makes absence fail. Both Linux CI jobs set it; the native macOS and
  Windows jobs have no buf and skip by name (documented). `InteropGate`'s `PATH` lookup moved to a
  shared `ToolSearch`. The `Integration` category now means "needs the service binary or buf".
- **Five failures surfaced** that the stale stubs had hidden
  (`Conformance*RenderingTests.TheSamplesSetEveryField…`); they walk the schema, not the inventory,
  and belong to session 4 (recorded there). With them, `make dotnet-test` ran 3,274 tests, 29
  failing: the 24 of the baseline table plus these 5. `DOTNET_MIN_TESTS` is 3274.
- **Pre-existing, not ours:** `go test ./tests/hygiene` fails 16 tests on this branch with or
  without the change, from a dangling `*scan-inputs` alias in `.circleci/config.yml` (and one
  `pr.yml` merge-group line in `dotnet-native`). It also stops YAML parsers that resolve aliases.

### Goal

`client/dotnet/src/OpenMBEE.OpenSysML/Wire/Sysml.cs` and `SysmlGrpc.cs` are exactly what
`api/proto/sysml.proto` generates, CI fails whenever they are not, and the parity inventory can no
longer report "complete" against a stale schema.

### Why

- The C# stubs were last generated on 2026-10-21 (`7c1e0554d`). Since then `api/proto/sysml.proto`
  gained 132 lines: `Verdict.notes`/`VerdictNote`, `ConvertRequest.include_library_stubs`,
  `ConvertResponse.library_references`/`LibraryReference`, `Query.order_by`, four new
  `PrimitiveOperator` values (`LESS_EQUAL`, `GREATER_EQUAL`, `IN`, `INSTANCE_OF`),
  `QueryOutcome`/`QueryFailure`/`QueryFrontier*`/`QueryFinding`,
  `RenderDocumentResponse.asciidoc`, `RenderViewRequest.compartments`,
  `RenderNode.compartment`, `RenderNode.inherited` and `RenderPort.inherited`. None of these exist
  in the C# code (`grep -c Inherited Wire/Sysml.cs` → 0).
- CI verifies the committed Go, Python, TypeScript and Rust stubs are current, but not the C# ones.
- `ParityInventoryTests.TheInventoryClassifiesWhatTheDescriptorDeclares` reads
  `Wire.SysmlReflection.Descriptor`, the descriptor *compiled into the stale stubs*. It is
  self-referential, so the inventory showed "31/31 RPCs, 994/995 fields typed" while the schema had
  moved on.

### Steps

1. **Regenerate.**
   ```bash
   make proto-csharp
   ```
   The Makefile's default `BUF` is `go run github.com/bufbuild/buf/cmd/buf@v1.57.2` (the pin). Do
   **not** pass `BUF=buf` here: the Homebrew `buf` is 1.67.0 and may format output differently from
   what CI will produce. The C# plugins in `api/proto/buf.gen.csharp.yaml` are *remote*
   (`buf.build/protocolbuffers/csharp:v36.2`, `buf.build/grpc/csharp:v1.84.0`), so this needs
   network access to buf.build.
2. **Review the diff** of `Wire/Sysml.cs` and `Wire/SysmlGrpc.cs`. Expect only additions of the
   fields and messages listed above, plus a changed embedded descriptor. If plugin output for
   unchanged messages moved too, the plugin pins and `Directory.Packages.props` (Google.Protobuf
   3.36.2, Grpc.Net.Client 2.84.0) have diverged; stop and reconcile them rather than committing
   churn.
3. **Build** with `make dotnet-build`. New proto fields do not break compilation, but check that no
   `switch` over a proto enum now trips an analyzer (for example over `PrimitiveOperator` in
   `Internal/QueryWire.cs`).
4. **Account for the new schema in the parity inventory.** `make dotnet-test` will now fail
   `ParityInventoryTests` because the inventory does not classify the new fields and messages.
   Update `client/dotnet/test/testdata/parity.json`:
   - Add every new field to its message's `fields` groups with status **`deferred`** (no reason, no
     test; the checker refuses a deferred group that has either). Session 4 promotes them to
     `typed`.
   - Add the new messages (`VerdictNote`, `LibraryReference`, `QueryOutcome`, `QueryFailure`,
     `QueryFrontier`, `QueryFrontierMember`, `QueryFrontierFeature`, `QueryFinding`) where the
     checker requires them: every message an RPC reaches. `QueryOutcome` and its children are an
     error *detail*, not reachable from a response message; follow what the checker says about them.
   - Add the new `PrimitiveOperator` values wherever the inventory lists enum arms, if it does.
   - Leave the `gates` of `Query` and `Convert` for session 4. Record them here only if the checker
     insists, since gates mean the client sends the field, which it cannot do yet.

   Let the test's own failure messages drive these edits; it reports exactly what is unaccounted
   for.
5. **Add the CI drift gate.** In `.circleci/config.yml`, job `dotnet-test`, add two steps after
   checkout, mirroring the Rust job (around the `Verify Rust generated artifacts` step):
   ```yaml
   - run:
       name: Install Buf
       command: |
         curl -fsSL -o /tmp/buf \
           https://github.com/bufbuild/buf/releases/download/v1.57.2/buf-Linux-x86_64
         echo "644e6fa4a62afc2b0d425d3ac9538d2f512947c0f2e82fc95c41a5eab6729414  /tmp/buf" | sha256sum --check --strict
         chmod +x /tmp/buf
         /tmp/buf --version

   # Committed stubs must be what buf generates, or the checked-in client and
   # the schema can drift apart silently.
   - run:
       name: Verify the committed C# stubs are current
       command: |
         make BUF=/tmp/buf proto-csharp
         git add -N client/dotnet/src/OpenMBEE.OpenSysML/Wire
         git diff --exit-code client/dotnet/src/OpenMBEE.OpenSysML/Wire
   ```
   The SDK image runs as root and already installs `curl` in its first step. Put these steps before
   the build so a drift fails fast.
6. **Make the parity inventory check the schema source too.** The CI gate covers CI. To also catch
   drift locally, add an **Integration**-category test (e.g. `WireSchemaTests.TheCommittedStubsAreTheSchema`
   in `client/dotnet/test/OpenMBEE.OpenSysML.Tests/`) that:
   - runs `buf build api/proto --as-file-descriptor-set -o -` (the binary from `$BUF`, else `buf`
     on `PATH`), parses the `FileDescriptorSet`, and compares its `sysml.proto` file with
     `Wire.SysmlReflection.Descriptor.ToProto()` on **messages, fields (name, number, type,
     label, type name, oneof index), enums and values, and service methods**. Ignore `options`:
     managed mode rewrites `csharp_namespace` only in what the C# plugins see.
   - skips by name when no `buf` is found, unless a require variable
     (`OPENSYSML_REQUIRE_BUF=1`, documented beside the others in `client/dotnet/README.md`) is set.
     Set it in the CI job, where step 5 installs buf.
   - on a mismatch, names the first differing element and says to run `make proto-csharp`.

   Prove it is load-bearing: temporarily add a field to `sysml.proto` and see the test fail, then
   revert.

   If running `buf` from a test is unwelcome, the CI step alone is an acceptable minimum. Say so in
   the PR description.
7. **Docs.** In `client/dotnet/README.md` § *Protocol code*, state that CI regenerates the stubs and
   fails on any difference, and document the new test and its variable if you added them. Extend
   `changes/unreleased/dotnet-client-wire.added.md` with one sentence about the drift check.

### Verify

- `make proto-csharp && git diff --exit-code client/dotnet/src/OpenMBEE.OpenSysML/Wire` exits 0.
- `make dotnet-test` is green except the tests sessions 3 and 4 own (see the baseline table). The
  RenderView duplicate-id failure may change once `compartment` exists on the wire; record what you
  see for session 4.
- `make conformance-dotnet` stays green. `conformance-dotnet-check` still reports the missing
  `inherited`; that is session 4's work.
- `DOTNET_MIN_TESTS` raised by the number of tests you added.

### Done when

The C# stubs match the schema, CI fails on drift (and, if added, a local test does too), every new
field and message is `deferred` in `parity.json`, and nothing else regressed.

---

## Session 2 — Keep `OPENSYSML_GRPC_TOKEN` out of private services; add first-class token support

> Before starting: read [`AGENTS.md`](../../../AGENTS.md) and the **Common setup** section above. `AGENTS.md` governs how everything in this session is implemented, tested, committed and documented; where this section and `AGENTS.md` differ, follow `AGENTS.md`.

### Status: done

`d654510ae` *feat(dotnet): send a bearer token and keep it out of private services*, on branch
`feat/dotnet-grpc-token`, fast-forwarded into `feat/dotnet-client` and pushed.

Implementation notes:

- **Two decisions differ from step 2 as written.** (1) *A token with a private service* is sent,
  not refused: Python, Java, Node and Rust all send theirs to the private child (Python's "a
  private child needs none" is a doc note; `open_channel` adds the token either way), and the
  child, started without the variable, ignores `Authorization` since a service with no token
  configured admits every call. Refusing would also break a program that always passes a token
  and lets `$OPENSYSML_SERVICE` choose remote or private. (2) *Token beside an `authorization`
  header*: an explicit `Token` is refused in `Validate()` (an options-only conflict); a token
  that would come only from the variable gives way to the explicit header, which is then the
  credential (`CredentialSource.Header`). Both chosen with the maintainer.
- **The regression test had to be designed, not just written.** Because the client now sends
  `$OPENSYSML_GRPC_TOKEN` to its private child too, the plan's repro passes even without the
  environment fix. `ServiceTokenTests.APrivateServiceStartedWhileTheVariableNamesATokenRequiresNone`
  runs the orphan probe (`serve <binary> <wire> 0`) with the variable set in the probe's
  environment alone (new `ProbeRun.Start` overload), reads the service address the probe now
  reports, and calls it carrying *no* token. Removing `RemoveToken` from `StartInfo` fails it on
  all three wires with the original `Unauthenticated`.
- **Where things live.** `Internal/BearerToken` is the service's `b64token` grammar
  (`internal/frontend/guard/token.go`), shared by option and variable validation.
  `Internal/CallHeaders.Resolve` builds the per-call header list once in `Connection.OpenAsync`
  (after the cancellation check, so an already-cancelled open still reads only
  `$OPENSYSML_SERVICE`) and records the `CredentialSource`; both transports take it instead of
  re-reading `options.Headers`. `Internal/ServiceErrors.FromService` is the one factory for a
  service-sent status; `GrpcTransport.Map` gained a `CredentialSource` parameter.
  `PrivateService.RemoveToken(environment, windows)` is the testable strip.
- **`AuthenticationException`** (public, `: ServiceException`) has `CredentialsSent` and `Remedy`,
  and overrides `Message` to append `\n  fix: <remedy>`; the remedy names where the refused
  credentials came from (option, variable or header) or says none was sent. A non-Connect 401
  (or a gRPC 401 without `grpc-status`) stays a `TransportException` with `Unauthenticated`.
- **Cleartext warning**: logged once at open when credentials of any source go to an `http`
  address that is not loopback (`Uri.IsLoopback`); never for a private service. The message names
  the address only.
- **Tests changed on purpose:** `TransportContractTests.EveryStatusTheServiceSendsIsAServiceExceptionButADeadline`
  and `ConnectionOpenTests.HeaderValuesAreNeverLogged` asserted an exact `ServiceException` for
  `Unauthenticated`; they now expect `AuthenticationException` for that status alone.
- **`ServiceProcess.StartAsync`** (conformance project, shared with the tests) gained an optional
  `token`, given to the service through its environment rather than its command line;
  `TokenServiceFixture` starts one service with a random token for `ServiceTokenTests`.
- **Gate.** `make dotnet-test` ran 3,352 tests (78 new), failing the same 29 as after session 1;
  `DOTNET_MIN_TESTS` is 3352. `make conformance-dotnet` green; `conformance-dotnet-check` reports
  only the known `inherited`. The plan's repro prints the model line.
- **Out of scope, noted:** the conformance runner's own `ServiceProcess` (and the Go reference
  runner) still start their service with the parent environment, so with the variable set the
  runner's service requires it; the client now sends it, so runs still pass.


### Goal

1. A private `sysml-grpc` the .NET client starts never inherits `OPENSYSML_GRPC_TOKEN`, as the
   Python, Java, Node and Rust clients ensure.
2. The client can send a bearer token as an option (`ConnectionOptions.Token`), defaulting to
   `$OPENSYSML_GRPC_TOKEN` for a named service, as every peer does.
3. A service-sent `Unauthenticated` raises a specific `AuthenticationException`.

### Why (reproduced)

`sysml-grpc` now requires a bearer token whenever `-token`, `-token-file` or `OPENSYSML_GRPC_TOKEN`
names one (`cmd/sysml-grpc/main.go`, `access.go`). The .NET client starts its child with the whole
parent environment (`Internal/PrivateService.cs`, `StartInfo`), so:

```bash
OPENSYSML_GRPC_TOKEN=s3cret OPENSYSML_GRPC_BINARY="$PWD/bin/sysml-grpc" \
  dotnet run --project client/dotnet/samples/Vehicle -c Release
# sysml.SysMLService/GetServerInfo was refused by the service with Unauthenticated: …
```

Every private service is unusable for anyone who also talks to a token-protected remote service.

### Reference implementations

- Python `client/python/opensysml/connection.py`: `TOKEN_ENV`; `_PrivateService.start` builds
  `environment = {… if name != TOKEN_ENV}`; `token=` defaults to `$OPENSYSML_GRPC_TOKEN`;
  `_BearerToken` adds `authorization: Bearer <token>`. `errors.py`: `AuthenticationError` mapped from
  `UNAUTHENTICATED`.
- Node `client/node/src/node/service.ts`: `environmentWithoutToken`, which compares names
  **case-insensitively on Windows** because the service reads the variable that way there.
- Java `ConnectionOptions.java` (`TOKEN_ENV`, `token()` falls back to the variable when blank or
  unset), `internal/PrivateService.java` (`builder.environment().remove(TOKEN_ENV)`),
  `internal/ConnectTransport.java` (`Authorization: Bearer …`).
- Rust `client/rust/opensysml/src/connection.rs` (`TOKEN_ENV`, `env_token()` ignores a blank value).
- Contract: `docs/reference/service-transports.md` § Authentication, and
  `docs/reference/environment.md` (the `OPENSYSML_GRPC_TOKEN` row already says "the clients … start a
  private child without it").

### Steps

1. **Strip the variable from the child's environment.** In `Internal/PrivateService.cs`, `StartInfo`
   (with `UseShellExecute = false`, `ProcessStartInfo.Environment` is pre-populated from the parent):
   remove every key equal to `OPENSYSML_GRPC_TOKEN`, using `StringComparer.OrdinalIgnoreCase` on
   Windows and `Ordinal` elsewhere. Iterate over a snapshot of the keys, since on Windows the
   dictionary is already case-insensitive. Add a one-line comment saying why.
2. **Constants and option.** In `ConnectionOptions.cs`:
   - `public const string TokenVariable = "OPENSYSML_GRPC_TOKEN";`, documented like
     `ServiceVariable`.
   - `public string? Token { get; init; }`: "The bearer token every call carries, for a service
     that requires one…". Validate it in `Validate()`: non-blank when set, printable ASCII without
     spaces (RFC 6750 `b64token`; refuse anything that cannot be a header value), with
     `InvalidRequestException`. Never put the value in a message or log.
   - Decide and document the precedence: `Token` option, else `$OPENSYSML_GRPC_TOKEN` (trimmed;
     blank counts as unset, as Java and Rust treat it).
   - **A private service needs no token.** Match Python, which documents "a private child needs
     none": for a private service, don't read the variable, and refuse `Token` with
     `InvalidRequestException` in `ValidatePrivateService` the way `BinaryPath` is refused alongside
     `Address`. Java sends the token to its private child harmlessly; if you prefer that, say so in
     the README. Either way, the child must never *demand* one (step 1).
   - **Conflict with `Headers`:** if `Headers` already holds `authorization` (any case) and a token is
     also in effect, refuse with `InvalidRequestException` ("set one or the other") rather than
     sending two. Add the check where both are known, at resolve time, since the variable is read
     then.
3. **Send it.** Both transports already copy `options.Headers` into every call
   (`GrpcTransport`: `Metadata`; `ConnectTransport`: request headers). Compute the effective header
   list once, where `Connection.OpenAsync` resolves the address and environment, and pass it in
   rather than re-reading options in each transport. The lower-cased name `authorization` and value
   `Bearer <token>`. Keep values out of every log line; grep your diff for `LogDebug`/`Log*` calls
   that could interpolate headers.
4. **Cleartext warning.** The service refuses to serve a token on a non-loopback cleartext port
   unless explicitly told to accept it. On the client side, log a `Warning` (once per connection)
   when a token is sent over `http://` to a non-loopback host, saying the token crosses the network
   in clear. Do not refuse: a TLS-terminating proxy in front is the documented, legitimate case.
5. **`AuthenticationException`.**
   - New public `AuthenticationException : ServiceException` in its own file, constructed for a
     **service-sent** `StatusCode.Unauthenticated` in both transports: `GrpcTransport.Map` (the
     `fromService` branch) and `ConnectTransport` (where `TryReadError` yields a code, around line
     415). Use a small shared factory (e.g. `ServiceException.FromService(code, message, method,
     inner)`) so the two stay in step.
   - The message must say how to fix it: "supply the service's token with `ConnectionOptions.Token`
     or `$OPENSYSML_GRPC_TOKEN`", or "the token sent was refused" when one was sent. The exception
     needs to know whether a token was sent; pass a flag from the transport.
   - A plain HTTP 401 that is not a Connect error (a proxy's page) stays a `TransportException` with
     an inferred `Unauthenticated`, as the README's rule on whose status it is requires. Don't blur
     that.
6. **Tests** (all in `client/dotnet/test/OpenMBEE.OpenSysML.Tests/`):
   - Unit: `StartInfo` drops the variable, case-insensitively on Windows and only the exact name on
     POSIX. Make the comparison a testable internal helper, as Node's `environmentWithoutToken` takes
     the platform as a parameter.
   - Unit: option validation (blank, control characters, conflict with an `authorization` header,
     `Token` alongside a private service).
   - Unit, on the existing `FakeServiceHandler`/fake-transport tests: the token is sent as
     `authorization: Bearer …` over gRPC and Connect; the variable is used when the option is unset;
     a blank variable is ignored.
   - Unit: mapping a service-sent `Unauthenticated` to `AuthenticationException` on both
     transports, and a non-Connect 401 to `TransportException`.
   - **Integration** against the real binary, the regression for the bug: with
     `OPENSYSML_GRPC_TOKEN` set in the test process's environment, `Connection.OpenAsync()` with
     `BinaryPath` set opens and answers `GetServerInfo`. Since setting process environment in a test
     is global, run it in the existing child-process probe style (as `OrphanProbe`/`CacheProbe` do),
     or pass the variable to a probe process. Don't mutate this process's environment from a
     parallel test.
   - **Integration**: start `bin/sysml-grpc -port 0 -report-address -token <random>` yourself (a
     loopback port with a token), then show `Token` succeeds, a missing token raises
     `AuthenticationException`, and a wrong token raises `AuthenticationException` saying it was
     refused. Over all three wires (`ClientWires.All`).
   - Raise `DOTNET_MIN_TESTS`.
7. **Parity and docs.**
   - `client/dotnet/README.md`: § *Connecting to a service* (the `Token` option and variable),
     § *Private services* (started without the variable; needs no token), § *Failures* (a new row:
     "The service wants a bearer token the call did not carry, or carried wrong →
     `AuthenticationException`, a `ServiceException` with `Unauthenticated`").
   - `docs/clients/dotnet.md` (a short "services that require a token" note) and
     `docs/reference/dotnet-api.md` (the option, the constant, the exception).
   - `docs/reference/environment.md`: confirm the .NET client is covered by "the clients read the same
     variable".
   - Extend `changes/unreleased/dotnet-client-connection.added.md` (token) and
     `dotnet-client-private-service.added.md` (child started without the variable).

### Verify

- The repro above prints the model line.
- `make dotnet-test` and `make conformance-dotnet` pass. No token value appears in any log output
  at `Trace`; check with a test logger.
- `ps aux | grep '[s]ysml-grpc -port 0'` prints nothing afterwards.

### Done when

The repro passes, the client sends a token like its peers, `Unauthenticated` from the service is an
`AuthenticationException`, and docs and tests cover all three.

---

## Session 3 — Bump the client version to 0.11.0 and resync the embedded release digests

> Before starting: read [`AGENTS.md`](../../../AGENTS.md) and the **Common setup** section above. `AGENTS.md` governs how everything in this session is implemented, tested, committed and documented; where this section and `AGENTS.md` differ, follow `AGENTS.md`.

### Status: done

`f0d664ab4` *chore(dotnet): release in lockstep at 0.11.0 and pin its digests*, on branch
`fix/dotnet-version-lockstep`, fast-forwarded into `feat/dotnet-client` and pushed.

Implementation notes:

- `sync-release-digests.py` touched only the .NET copy (+12 lines, `v0.11.0`); no other client
  had drifted. `Directory.Build.props` is `0.11.0` (no pre-release, so `check_version.py`'s
  spelling is the plain version). `dotnet restore --force-evaluate` changed exactly the three
  `"OpenMBEE.OpenSysML": "[0.10.1, )"` lines (Tests, Generate.Tests, Conformance.Tests); no
  package moved.
- Remaining `0.10.1` left on purpose: `ReleaseRequestTests` (must name a *published* release),
  the `v0.10.1` tag examples in `ConnectionOptions.cs` and the README (still valid tags), and
  `dotnet-gates-test.py`'s `IDENTITY` strings, which are fixture text checked against a fake
  sample host, not the built version.
- **Gates:** `sync-release-digests.py --check` and `CIRCLE_TAG=v0.11.0 check_version.py --dotnet`
  (prints `0.11.0`) pass; `client/python/tests/test_check_version.py` 98 passed. The six focused
  suites: 229 passed. `make dotnet-test`: 3,352 tests, **27** failing, all session 4's (the two
  `ReleaseDigestsTests` now pass); samples print `0.11.0`; `make conformance-dotnet` green.
- **Manual download check (step 7) needs a different program.** The Vehicle sample deliberately
  does nothing beyond its version line unless `OPENSYSML_SERVICE` or `OPENSYSML_GRPC_BINARY` is set
  (`SampleGate`), so it never exercises the default download. A throwaway console with a
  `ProjectReference` calling `Connection.OpenAsync()`, run with a throwaway `HOME` and those
  variables unset, downloaded `v0.11.0` into the throwaway `~/.opensysml/bin`, recorded digest
  `730a6049…ca9a`, which is the pinned `sysml-grpc-darwin-arm64` entry, and answered
  `sysml-grpc v0.11.0, 72 capabilities`.
- No changelog fragment: the bump is release bookkeeping.


### Goal

Both release-lockstep gates pass, in one commit, because neither is safe alone.

### Why

- `client/dotnet/Directory.Build.props` declares `<Version>0.10.1</Version>`;
  `client/python/opensysml/_version.py` and the Java, Rust and Node manifests are at `0.11.0`.
  `CIRCLE_TAG=v0.11.0 python client/python/scripts/check_version.py --dotnet` fails.
- `client/dotnet/src/OpenMBEE.OpenSysML/release-digests.json` lacks `v0.11.0` (added to the shared
  table by `a3b43414b`). `python3 scripts/sync-release-digests.py --check` fails, and so do
  `ReleaseDigestsTests.EmbeddedCopyIsTheRepositoryTable` and
  `…EmbeddedTablePinsEveryAssetOfTheRepositoryTable`.
- **Coupling:** `ReleaseRequest.BuiltAgainst` is `"v" + <Version>`, the default release a private
  service is downloaded from. Bumping the version without the digests would make the default
  download refuse `v0.11.0` as unpinned. Resyncing the digests without the version leaves
  `check_version.py` red. Do both together.

### Steps

1. `python3 scripts/sync-release-digests.py` (rewrites every client's copy from
   `client/release-digests.json`). `git diff --stat` should touch only the .NET copy. If others move,
   they had drifted too; include them and mention it in the commit.
2. Set `<Version>0.11.0</Version>` in `client/dotnet/Directory.Build.props`. The value is the SemVer
   spelling of the Python version; if the core is a pre-release when you do this, check
   `check_version.py`'s dotnet branch (`dotnet_version`) for the exact spelling it requires.
3. **Regenerate the lock files.** The test projects' `packages.lock.json` record the project
   reference's version (`"OpenMBEE.OpenSysML": "[0.10.1, )"` in
   `test/OpenMBEE.OpenSysML.Tests`, `…Generate.Tests` and `…Conformance.Tests`, and possibly others).
   `make dotnet-build` always restores with `--locked-mode`, so the build fails until they are
   rewritten:
   ```bash
   dotnet restore client/dotnet/OpenSysML.slnx --force-evaluate
   git diff -- '*packages.lock.json'
   ```
   The diff must change only those `[0.10.1, )` → `[0.11.0, )` lines. Any package version moving
   means the restore resolved anew; stop and find out why rather than committing it.
4. Find other hard-coded uses of the old version:
   `grep -rn '0\.10\.1' client/dotnet docs/clients/dotnet.md docs/reference/dotnet-api.md | grep -v '/bin/\|/obj/\|packages.lock.json\|release-digests.json'`.
   At the time of writing these were:
   - `ConnectionOptions.cs` (two XML-doc examples, "a tag, as `v0.10.1`") and `README.md` line ~276:
     examples only, and still valid tags. Updating them to `v0.11.0` is optional.
   - `test/OpenMBEE.OpenSysML.Tests/ReleaseRequestTests.cs`
     (`ThePublishedPlatformsAreTheFiveTheReleasePipelineBuildsAndThatThePinsName` and an
     `[InlineData("v0.10.1", true)]`): they check that a *published* release is pinned for every
     platform. Leave them on a release that exists. Don't switch them to `BuiltAgainst`: the
     version is bumped before a release's digests are pinned, so that would make every bump a
     failing build.
   - `README.md` § *Version and pinned release digests*: update if it names the current version.
5. Run `ReleaseDigestsTests`, `ReleaseRequestTests`, `DownloadResolutionTests`,
   `BinaryDownloaderTests`, `BinaryResolverTests` and `SignedReleaseTests`.
6. Run the documented sample to confirm the printed assembly version:
   `OPENSYSML_GRPC_BINARY="$PWD/bin/sysml-grpc" dotnet run --project client/dotnet/samples/Vehicle -c Release`
   should print `OpenMBEE.OpenSysML 0.11.0 (.NETCoreApp,Version=v10.0)`.
7. **Optional manual check** (reaches GitHub, so not part of any test): with a throwaway `HOME`
   (`HOME=$(mktemp -d)`), and with `OPENSYSML_GRPC_BINARY` and `OPENSYSML_SERVICE` unset, run the
   Vehicle sample. It should download `v0.11.0` into the throwaway `~/.opensysml/bin`, verify it
   against the pin and run. Never point this at your real `~/.opensysml`.
8. Changelog: none needed if the version bump is release bookkeeping; otherwise a one-line extension
   of `dotnet-client-release.added.md`.

### Verify

```bash
python3 scripts/sync-release-digests.py --check
CIRCLE_TAG=v0.11.0 /tmp/v/bin/python client/python/scripts/check_version.py --dotnet   # prints 0.11.0
dotnet test --project client/dotnet/test/OpenMBEE.OpenSysML.Tests -c Release --filter 'FullyQualifiedName~ReleaseDigestsTests|FullyQualifiedName~ReleaseRequestTests|FullyQualifiedName~DownloadResolutionTests|FullyQualifiedName~BinaryResolverTests|FullyQualifiedName~BinaryDownloaderTests|FullyQualifiedName~SignedReleaseTests'
make dotnet-test
```

### Done when

Both gates and the digest tests pass, `make dotnet-build` (locked restore) succeeds, the sample
prints 0.11.0, and the version, digests and lock files change in one commit
(`chore(dotnet): release in lockstep at 0.11.0 and pin its digests`).

---

## Session 4 — Implement the missing service features and adjudicate the failing integration tests

> Before starting: read [`AGENTS.md`](../../../AGENTS.md) and the **Common setup** section above. `AGENTS.md` governs how everything in this session is implemented, tested, committed and documented; where this section and `AGENTS.md` differ, follow `AGENTS.md`.

### Status: done, one service bug left to file

`9bc987de7` *feat(dotnet): order queries, render AsciiDoc and compartments, read verdict notes
and library references*, on branch `feat/dotnet-service-features`, cut from `feat/dotnet-client`.

Implementation notes:

- **4e was decided first: intended, not a regression.** Bisected by building `sysml-grpc` at
  each first-parent `develop` commit and sending `EvaluateCalc(Echo::same, 7)` over raw gRPC
  (the bisect used Python's protobuf stubs directly; the Python *client* reads
  `$OPENSYSML_BINARY`, not `$OPENSYSML_GRPC_BINARY`, and otherwise silently runs the cached
  release). The first answer of `sequence { 7 }` is `ac1340353`, PR #1308
  (`fix/calc-result-effective-multiplicity`): a result or `out` parameter stating no
  multiplicity is `[0..*]` (SysML §7.6.3, §7.19.2), and its changelog tells clients to write
  `[1]`. Python decodes the same one-element list, so it was no client bug. That PR also
  re-pinned the shared conformance scenarios for the shared fixtures (`10-evaluate-calc`,
  `13-run-analysis`, `14-run-sweep`) as sequences, so editing those fixtures to `[1]` would
  have undone it. Decided with the maintainer: `ServiceValueTests`' own model says
  `return r[1] = x` (still a scalar round trip); the four tests over shared fixtures assert
  the sequence the scenarios pin (`Held(…)` in `ServiceVerificationTests`), and the sweep's
  cast is gone.
- **4b.** Names follow Java: `GreaterOrEqual`, `LessOrEqual`, `In`, `InstanceOf` (fixed to
  `@type`, as the service requires). `In` always prints its operands as a list. `orderBy`
  is `Query.OrderBy`; its gate is `query.order_by` in `parity.json` (a dotted path).
  Integration tests pin byte-wise name order, stable ties by `@type`, the inclusive bounds
  of `>=`/`<=` against `<` on `wheels[4]`, `In`, `InstanceOf` and the refusals of an
  undeclared metaclass and an unknown sort property.
- **4c.** `DocumentForm.AsciiDoc` keeps `RenderedDocument`'s one-`Content` design (Java's
  too); an answer in a second form is malformed. The capability constant is
  `RenderDocumentAsciidoc` because `CapabilitiesTests` derives names from values.
  `RenderViewCompartments` mirrors `RenderViewPorts`; no capability guards `compartments`,
  so the README says an older service ignores it. `ViewWire` also refuses a compartment
  row with no parent, or under another row: checked first against all 753 renderings of
  `internal/ir/view/testdata` in the three foldings (412 rows, none violating).
  `ChildrenOf` lists rows with the boxes.
- **Verdict notes need a solver.** The .NET CI image had none, so a service-backed test would
  have been vacuous. `SolverGate` asks the service's own `solve` engine (ListEngines)
  whether it is ready, and skips with its reason unless `OPENSYSML_REQUIRE_SMT` promised
  one. Both Linux .NET jobs now install z3 and set it. Proven load-bearing: with z3 hidden
  from `PATH` the test skips, and fails when the variable is set.
- **4d.** `IncludeLibraryStubs` is gated between `convert_documents` and `convert_compact`,
  where the service checks it; a non-api-json target is the service's to refuse, as for the
  other direction-dependent options. Node is the only peer exposing `libraryReferences`;
  Python and Java do not yet.
- **Out of scope, stated:** the `QueryOutcome` error detail is not decoded (no peer decodes it);
  `docs/reference/dotnet-api.md` lists it under what the client does not do. `parity.json`
  does not list it, being unreachable from any response.
- **Gate.** `make dotnet-test` (service, interop, buf and SMT required): 3,383 tests,
  `DOTNET_MIN_TESTS` 3383, failing only the three below; its later steps (samples, gate
  self-tests, digests) pass when run alone. `make conformance-dotnet` green;
  `make conformance-dotnet-check`: 1,128 results agree, no "only the reference run has it".
  Each `-mutate` run fails. `parity.json`: 1,009 typed, 1 unsupported, none deferred.

### Left failing: a service bug, drafted for upstream

`ServiceQueryDocsTests.EveryViewTheRenderersOwnTestsDeclareIsReadWithItsNodesInOrder` (×3) is
right and stays failing, not skipped. Compartments were not the cause. Text to file on
`Open-MBEE/OpenSysML`:

> **RenderView: the mixed rendering emits one node twice, under two parents**
>
> `RenderView` of `AbstractViews::familyMixed` in `internal/ir/view/testdata/abstract.sysml`
> (`render asMixedDiagram`, exposing `Vehicle::**`, `vehicleFamily::**` and `Models::**`)
> answers `Vehicle::mass` twice with the id `n0`: first as a child of `n1`
> (`AbstractViews::Vehicle`), then as a root, with or without `show = ["all"]`.
> Node ids come from `nodeIDs.take()` and edges and `parent` refer to nodes by id, so a
> repeated id with a different parent leaves a client unable to tell where the node sits; the
> .NET client refuses the answer as malformed. The goldens captured it:
> `abstract-mixed.text.golden` lists `attribute AbstractViews::Vehicle::mass` twice, and
> `abstract-mixed.dot.golden` declares `"n0"` inside `cluster_n1` and again at top level. No
> other view under `internal/ir/view/testdata` repeats an id. Expected: the member is drawn
> once, nested in its owner. Worth stating in the comment on `RenderViewResponse.nodes`, for
> every client, that ids are unique within a rendering.

**Prerequisite:** session 1 merged (the wire types below must exist). Sessions 2 and 3 do not need
to be merged first, but merge them in if they are done, so the final gate is meaningful.

This is the largest session. It is written as five parts (4a–4e), which can be split into separate
sessions if one runs long. Each part follows the same pattern as the existing features: public
option/result type → `Internal/*Wire.cs` encode/decode → capability check before sending →
`parity.json` from `deferred` to `typed` with its test → conformance renderer → docs → tests over all
three wires.

### Recorded by session 1

With the stubs regenerated, `make dotnet-test` (service, interop and buf all required) ran 3,274
tests: the 24 real failures of the baseline table, unchanged, plus **5 that regenerating exposed**.
The stale stubs had hidden them; they are this session's, one per area:

| Failing test | Response fields its samples do not set | Part |
|---|---|---|
| `ConformanceVerificationRenderingTests.TheSamplesSetEveryFieldTheVerificationResponsesReach` | `Verdict.notes`, `VerdictNote.{kind,subject,text}` | 4c |
| `ConformanceGraphDataRenderingTests.TheSamplesSetEveryFieldTheGraphDataResponsesReach` | same (`Verdict` is reached from the graph-data responses too) | 4c |
| `ConformanceSamplingRenderingTests.TheSamplesSetEveryFieldTheRunsSelfCheckAndRecordingResponsesReach` | same | 4c |
| `ConformanceConversionRenderingTests.TheSamplesSetEveryFieldTheConversionAndMigrationResponsesReach` | `ConvertResponse.library_references`, `LibraryReference.{id,qualified_name,type}` | 4d |
| `ConformanceQueryDocsRenderingTests.TheSamplesSetEveryFieldTheQueryDocumentAndViewResponsesReach` | `RenderDocumentResponse.asciidoc`, `RenderNode.compartment`, `RenderNode.inherited`, `RenderPort.inherited` | 4c |

These tests walk the schema, not the inventory, so a `deferred` field still counts. Close them by
setting each field in the `Complete*Wire` samples once the public API carries it (the round-trip tests
beside them then prove the rendering), not by excluding deferred fields.

- `ServiceQueryDocsTests.EveryViewTheRenderersOwnTestsDeclareIsReadWithItsNodesInOrder` is
  unchanged: still `nodes[2] repeats the id "n0"`. `compartment` is now on the wire, but
  `Internal/ViewWire.cs` does not read it yet.
- `make conformance-dotnet` is green. `make conformance-dotnet-check` reports only
  `nodes.1.ports.0.inherited: only the reference run has it`, for the two `render_view` scenarios
  over all three protocols.
- `DOTNET_MIN_TESTS` was 3274 after session 1; it is 3352 after session 2. After session 3 the
  two `ReleaseDigestsTests` pass, so session 4 starts from **27** failures: the 22 real ones of
  the baseline table that are not about digests, plus the 5 above. Session 2 also made a
  service-sent `Unauthenticated` an `AuthenticationException`, which a renderer or adapter that
  switches on exact exception types must allow for.
- `QueryOutcome` and its children are unreachable from any response, so `parity.json` does not list
  them. If 4b decodes the error detail, add whatever the checker then asks for.

### Reference

The peers already implement 4a–4c. Read them before designing the C# shape, and keep names
recognisably the same:

| Capability / field | Python | Java | Node | Rust |
|---|---|---|---|---|
| `query_order_by` / `Query.order_by` | `opensysml/query.py` | `Query.java`, `Model.java` | `src/core/query.ts` | `src/query.rs` |
| new operators `<=` `>=` `in` `instanceOf` | `query.py` (`_OPERATORS`, the checks around lines 200–235) | `Query.java` | `query.ts` | `query.rs` |
| `render_document_asciidoc` | `document.py` / `model.py` | `RenderedDocument`/`Model.java` | `src/core/document.ts` | `src/document.rs` |
| `verdict_notes` / `Verdict.notes` | `verdict.py` | `Verdict.java` | `src/core/verdict.ts` | `src/results.rs` |
| `RenderNode.compartment`/`inherited`, `RenderPort.inherited`, `RenderViewRequest.compartments` | `document.py` (`RenderNode`, `RenderPort`), `connection.py`/`model.py` `render_view(compartments=…)` | `RenderedView.java` | `src/core/render-view.ts` | `src/document.rs` |
| `convert_library_references`, `convert_library_stubs` | — | — | `src/core/conversion.ts` (`libraryReferences`) | — |

Service side: capability constants and their comments are in
`internal/frontend/grpc/service.go`; field semantics are the comments in `api/proto/sysml.proto`.

### 4a. Capability constants

1. Add the five constants to `client/dotnet/src/OpenMBEE.OpenSysML/Capabilities.cs`, each documented
   from its Go comment, in the file's existing order:
   `ConvertLibraryReferences = "convert_library_references"`,
   `ConvertLibraryStubs = "convert_library_stubs"`, `QueryOrderBy = "query_order_by"`,
   `RenderDocumentAsciiDoc = "render_document_asciidoc"`, `VerdictNotes = "verdict_notes"`.
   `CapabilitiesTests.EachConstantIsNamedForItsValue` derives the name from the value, so check
   `RenderDocumentAsciidoc` against `RenderDocumentAsciiDoc`; the test decides.
2. `CapabilitiesTests.EveryCapabilityTheServiceDeclaresHasAConstantAndNoConstantIsInvented` should
   now pass.

### 4b. Query: `orderBy` and the four new operators

1. `QueryConstraint.cs` / `ComparisonOperator.cs`: add `LessOrEqual`, `GreaterOrEqual`, `In`,
   `InstanceOf` with the proto's numbering. Mirror Python's client-side checks: `<`, `<=`, `>`,
   `>=` compare against exactly one value; `=` and `in` against at least one; `instanceOf` requires
   property `@type` and metaclass-name values. Throw `ArgumentException` at construction, as the
   existing factories do. These operators have no capability gate on the service; confirm in
   `service.go` before adding one.
2. **One-member composite:** the standard and now the service require at least two constraints
   (`6000439c7`, "refuse a one-member CompositeConstraint"). Make the `And`/`Or` factories refuse
   fewer than two with `ArgumentException` ("a composite combines at least two constraints; write
   the constraint on its own"), so the mistake fails before sending. Update their XML docs ("at
   least one" → "at least two") and the README.
3. **`orderBy`:** add the ordering to the query API, e.g. `QueryOptions.OrderBy`
   (`IReadOnlyList<string>`, in precedence order, each ascending), or whatever matches how
   `Model.QueryAsync` takes options today. Check `Capabilities.QueryOrderBy` before sending a
   non-empty list (`CapabilityException`, nothing sent). The proto says a service without it would
   silently ignore the field, which is why the check is mandatory. Encode in `Internal/QueryWire.cs`.
4. `parity.json`: `Query.order_by` → `typed` with its test; add the gate to the `Query` RPC row (field
   `order_by`, condition "non-empty"). The parity tests read the service's gate order from its
   source and say if it is wrong.
5. **Adjudicate** `ServiceQueryDocsTests.QueriesSelectWhatTheServiceEvaluatesInEveryConstraintForm`.
   The service change was deliberate, so the test is wrong: replace its one-member composite with a
   two-member one (or a bare constraint) and add a unit test that the factory refuses one member.
   Say why in the commit.
6. Tests: unit tests for each operator's validation and encoding, `orderBy` encoding and the gate;
   integration tests (all three wires) that `orderBy` sorts as the service documents (ties keep
   declaration order) and that each new operator selects what the service evaluates. Reuse the
   fixture `Fixture("query.sysml")`, extending it if needed.

### 4c. RenderDocument AsciiDoc, verdict notes, view compartments and inheritance

1. **AsciiDoc:** add `DocumentForm.AsciiDoc` (doc: "Needs `Capabilities.RenderDocumentAsciiDoc`")
   and `RenderedDocument.AsciiDoc` (string, empty for other forms). Mirror the HTML gate in
   `Model.Document.cs` (around line 91) and decode in `Internal/DocumentWire.cs`.
   `parity.json`: `RenderDocumentResponse.asciidoc` → `typed`; add the gate.
2. **Verdict notes:** a public `VerdictNote` record (`Kind`, `Subject`, `Text`; `Kind` is a
   string whose known values are `"assumed_one"` and `"uncovered"`, documented, not a closed
   enum), and `Verdict.Notes` (`ValueList<VerdictNote>`). Decode in `Internal/VerificationWire.cs`.
   The field is response-only and "reported as the `verdict_notes` capability": an older service
   leaves it empty, so no request gate. Update `Verdict`'s equality and `ToString` if they enumerate
   members. `parity.json`: `Verdict.notes` and the `VerdictNote` fields → `typed`. If the inventory
   has a `population` section for "response fields the service fills only under a capability", add
   `notes` there.
3. **Views:** `ViewNode.Compartment` (string, empty for a box), `ViewNode.Inherited` and
   `ViewPort.Inherited` (bool); `RenderViewOptions.Compartments` (an enum
   `Positioned`/`Most`/`Ports`, default `Positioned`, encoded as `""`/`"most"`/`"ports"` as Python
   does). Decode and encode in `Internal/ViewWire.cs`.
   - **The duplicate-id check:** re-run
     `ServiceQueryDocsTests.EveryViewTheRenderersOwnTestsDeclareIsReadWithItsNodesInOrder` with the
     new wire. Find out from the service (`internal/ir/view`, the RenderView handler) whether ids
     are unique across *all* nodes or only among boxes, given compartment rows. The proto does not
     promise uniqueness. Then:
     - if the service intends unique ids and emits duplicates, that's a service bug: report it with
       the view that reproduces it, and keep the client check;
     - if duplicates are legitimate (a usage drawn both as a box and as a row), change
       `ViewWire.Rendered`'s check to what the contract really is (for example, unique per
       `(id, compartment)`, with `parent` resolving to a box), and **write that rule into the proto
       comment on `RenderViewResponse.nodes`** so the contract is explicit for every client.
     Either way, don't just delete the check.
   - `parity.json`: the four fields → `typed`.
4. **Conformance renderer.** `src/OpenMBEE.OpenSysML.Conformance/Rendering.cs` renders public-API
   results into the shared normalized form; add `inherited` (port and node), `compartment`, verdict
   `notes` and the document's `asciidoc` exactly as the reference runner emits them (look at
   `cmd/` or `conformance/` for the reference shape, or diff `bin/conformance-dotnet-check/` output).
   This is what turns `make conformance-dotnet-check` green. Update the conformance golden
   `test/OpenMBEE.OpenSysML.Conformance.Tests/testdata/golden/report.json` only by regenerating it,
   and review the diff.
5. **Expected skips.** If any scenario in `client/dotnet/test/testdata/expected-skips.json` was
   skipped for want of these capabilities, it will now run, and "a listed skip that ran fails the
   run". Remove it from the list.

### 4d. Conversion: library references and library stubs

1. `ConvertOptions.IncludeLibraryStubs` (bool; api-json only, "restore the ownerless library stubs
   written before 0.11.0"), gated by `Capabilities.ConvertLibraryStubs` like
   `IncludeCheckDiagnostics` is in `Connection.Conversion.cs` (around line 69). Refuse it for a
   non-api-json target if the service would (check the handler).
2. A public `LibraryReference` record (`Id`, `QualifiedName`, `Type`) and
   `Conversion.LibraryReferences` (`ValueList<LibraryReference>`), decoded in
   `Internal/ConversionWire.cs`. Response-only, populated "for successful api-json conversions,
   including compact output"; an older service leaves it empty.
3. Model this on Node's `conversion.ts` (`libraryReferences`). Python and Java don't expose it yet;
   note that in the PR so the gap there is visible.
4. `parity.json`: both fields → `typed`, with the gate on `include_library_stubs`.
5. Tests: unit encode/decode and gate; integration (three wires) converting a model that references
   the standard library to api-json, asserting references are listed and, with stubs on, that the
   output contains them as roots.

### 4e. Adjudicate the `7` → `(7)` result-shape change

Failing: `ServiceValueTests.EveryKindARequestCarriesComesBackEqualFromTheService`,
`ServiceVerificationTests.ACalculationTakesStructuredArgumentsAndFailsInBand`,
`…AnAnalysisRunsOnItsSubjectWithItsArgumentsIsExploredAndKeepsWhatAFailedRunComputed`,
`…ASweepTablesEveryRowKeepsAFailedRowAndDrawsReproducibly` and
`ServiceSamplingTests.ARecordedRunIsANewModelHoldingTheRecordsAndTheOriginalIsUntouched`. Each
expected a scalar and now gets a one-element `SequenceValue` from a calculation result.

This is a decision, not a code change. Do it before touching any of these tests.

1. Reproduce outside .NET. With `bin/sysml` (REPL) or the Python client, evaluate the same model
   (`ServiceValueTests.Model`: `calc def Same { in x; return r = x; }`) against the current build and
   against a build of `develop` from before 2026-10-21. Confirm the service changed, not the client.
2. Find the change: `git log --since=2026-10-20 -- internal/exec/runtime internal/frontend/grpc internal/frontend/protoconv`.
   Candidates include the runtime changes to unbound/optional calc inputs and results. Read the
   change's tests to see whether the new shape was intended.
3. Check the semantics: an untyped `in x;` and `return r = x;` declare no multiplicity, so the
   default applies. Decide whether a single value is now correctly reported as a one-element
   sequence. Check what the Python, Java and Rust suites assert for equivalent calls; if their
   tests still expect a scalar and pass, the .NET client is decoding differently, and that is a
   client bug. Compare `ValueWire.FromWire` with Python's `values.py`.
4. Outcome:
   - **Service regression:** stop. File an issue with the minimal model, both outputs and the
     offending commit. Leave the .NET tests failing (they are right). Don't mark them skipped.
   - **Intended:** update each test's model so it states the multiplicity it means (e.g.
     `in x : ScalarValues::Integer[1]; return r : ScalarValues::Integer[1] = x;`), so the test keeps
     testing a scalar round trip, rather than changing the expected value to a sequence. Say why in
     the commit. The sweep test's `InvalidCastException` must become a typed failure or disappear
     with the model fix. A cast in test code that blows up is not an assertion.
   - **Client decoding bug:** fix `ValueWire` and keep the tests.

### Docs and changelog for session 4

- `client/dotnet/README.md`: § *Querying, documents and views* (orderBy, operators, two-member
  composites, AsciiDoc, compartments, inheritance); § *Verifying…* (verdict notes); § *Converting…*
  (library references/stubs); the status paragraph at the top if it enumerates features.
- `docs/clients/dotnet.md` where the walkthrough touches these; `docs/reference/dotnet-api.md` for
  every new public member.
- If you change the RenderView id contract, update `api/proto/sysml.proto`'s comment and run the
  stub regeneration for every language (`make proto`). Session 1's gate will otherwise fail.
- Extend `changes/unreleased/dotnet-client-query-docs.added.md`,
  `dotnet-client-verification.added.md` and `dotnet-client-conversion.added.md`.

### Verify

```bash
make dotnet-test                  # 0 failures with both REQUIRE variables set and JAVA_HOME set
make conformance-dotnet
make conformance-dotnet-check     # no "only the reference run has it" lines
for m in truncate-lists perturb-reals rewrite-strings; do
  dotnet run --project client/dotnet/src/OpenMBEE.OpenSysML.Conformance -c Release -- \
    -binary bin/sysml-grpc -protocols grpc -mutate $m; echo "$m exit=$?"   # each must be non-zero
done
python3 - <<'EOF'   # nothing left deferred but the deprecated content_hash
import json, collections
d = json.load(open('client/dotnet/test/testdata/parity.json'))
c = collections.Counter(g['status'] for gs in d['fields'].values() for g in gs for _ in g['fields'])
print(c)
EOF
```

### Done when

- No `deferred` field remains in `parity.json` except where a documented reason says otherwise
  (`QueryOutcome` may stay out of scope; no peer decodes it yet. If so, mark it `raw-only` or
  `unsupported` with a reason, not `deferred`, and state it as a known limitation as `AGENTS.md` §8
  requires for any narrowed scope).
- Every capability the service declares has a constant, and the features behind
  `query_order_by`, `render_document_asciidoc` and `verdict_notes` match Python/Java/Node/Rust.
- Every one of the 24 baseline failures is green or explicitly adjudicated as a service regression
  with an issue filed.
- `conformance-dotnet-check` passes, the mutation runs still fail, and `DOTNET_MIN_TESTS` is raised.

---

## Cleanup after any session

- Remove scratch consumers, probe projects and any `dotnet-install.sh` you downloaded.
- `ps aux | grep -E '[s]ysml-grpc -port 0|[F]akeService|[O]rphanProbe'` prints nothing.
- `git status` shows only intended changes.
- Mark the session done in the table at the top and record its commit and implementation notes
  under its heading.

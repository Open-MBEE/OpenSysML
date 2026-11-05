---
name: testing-dotnet-client
description: How to test the OpenMBEE.OpenSysML .NET client (client/dotnet) end to end — provisioning the exact pinned SDK, running the suite with the real sysml-grpc required rather than skipped, the orphan/sibling and load-context probes, cross-client cache-lock interop, the release-trust fixtures, the conformance runner and its mutations, and the documented walkthrough the guide shares with the Vehicle sample.
---

# Testing the .NET client

Read `client/dotnet/README.md` (Building and testing, Conformance) first; every rule it states is a
test somewhere. Run every command from the repository root, since `global.json` there selects the SDK
and the Microsoft.Testing.Platform runner.

## Setup

- **The SDK must be exactly the one `global.json` pins.** `make dotnet-preflight` refuses any other,
  even a newer patch that `rollForward: latestPatch` would accept, and Homebrew upgrades its `dotnet`
  silently. Install the pin privately and put it first:
  `curl -sSfL https://dot.net/v1/dotnet-install.sh -o /tmp/dotnet-install.sh && bash /tmp/dotnet-install.sh --version <pin> --install-dir ~/.dotnet`,
  then `export DOTNET_ROOT=$HOME/.dotnet PATH=$HOME/.dotnet:$PATH` in every shell that runs `make`.
- Build the real service: `make build-grpc`. A stale `bin/sysml-grpc` from before a schema change
  fails parity and conformance in confusing ways; rebuild after every merge.
- Interop needs Python 3, a JDK with `javac` (`JAVA_HOME` or `PATH`) and Cargo. On macOS set
  `JAVA_HOME=$(/usr/libexec/java_home)`.
- `dotnet test <solution>` (the VSTest form) does not work; use `dotnet test --solution ...` or the
  Make targets.

## The suite, with nothing allowed to skip

```bash
export OPENSYSML_GRPC_BINARY="$PWD/bin/sysml-grpc" OPENSYSML_REQUIRE_SERVICE=1 OPENSYSML_REQUIRE_INTEROP=1
export OPENSYSML_REQUIRE_SMT=1   # needs z3 or cvc5 on PATH: the verdict-notes test asks the service a solver question
make dotnet-test        # format, 0-warning build, whole suite, Vehicle sample, gate self-tests, digests
make dotnet-coverage    # same suite under coverlet; Cobertura and JUnit in bin/dotnet/coverage
```

Without the two `REQUIRE` variables, absent prerequisites are skips **by name**, and a run that
skipped them proves nothing about the service, the lifecycle or locking. `DOTNET_MIN_TESTS` in the
Makefile is a floor on executed tests: raise it when tests are added, never lower it to pass. Every
test declares `Category` `Unit`, `Integration` or `Interop`; focus with
`dotnet test --project client/dotnet/test/OpenMBEE.OpenSysML.Tests -c Release --filter 'FullyQualifiedName~ClassName'`.

## Recipes

- **Orphans and siblings.** `OrphanTests` runs `test/OpenMBEE.OpenSysML.OrphanProbe` against the real
  binary: the probe is killed and its service must end while siblings run on. Its leak-control test
  must see a service survive when its input leaks into a sibling; if that test passes vacuously the
  detector is broken. Afterwards `ps aux | grep '[s]ysml-grpc -port 0'` must print nothing.
- **Load contexts.** `LoadContextTests` drives `OpenMBEE.OpenSysML.LoadContextProbe`: three collectible
  copies, each with its own service, each collected after unloading. A copy that is not collected
  usually means a static, timer or thread still roots it.
- **Cache lock interop.** `CacheInteropTests` and `ForeignLockTests` hold `~/.opensysml/bin/sysml-grpc.lock`
  against the Python, Java and Rust holders in `test/interop`, both directions, including a killed
  holder. The Rust holder builds into `bin/dotnet/interop`. Never test against your real
  `~/.opensysml`; the tests use caches of their own.
- **Release trust.** `SignedReleaseTests` serves the Python client's signed-release fixtures
  (`client/python/tests/fixtures/signed_release/`) from the loopback `ReleaseServer`: every unpinned
  one must be refused as `UnpinnedReleaseException` with its signature never fetched.
  `DownloadResolutionTests` and `BinaryDownloaderTests` cover pins, stalls, redirects and size limits.
  No test reaches the network; a real GitHub download is a manual check.
- **Linux `ETXTBSY`.** In a Linux container, three Linux-only tests prove the busy-binary retry; on
  macOS they skip by name. Run Linux in the pinned SDK image with `--init`.
- **Metamodel types.** `make dotnet-metamodel-check` and `go test -C tools ./gen/csmetamodel ./gen/hierarchy`
  guard `src/OpenMBEE.OpenSysML.Metamodel/Generated.cs`; `test/OpenMBEE.OpenSysML.Metamodel.Tests`
  (Unit only, no service) holds it against `internal/translate/rdf/ontology/sysml-metaclasses.ttl`
  and compiles `test/testdata/metamodel/synthetic.golden.cs`, which
  `go test -C tools ./gen/csmetamodel -run TestSyntheticGolden -update` rewrites after a generator
  change. Changing a property in `table.go` must fail both checks; changing an edge in the `.ttl` must
  fail `HierarchyTests`. Restore both by digest afterwards.
- **Metamodel reader.** `MetamodelGraph.ReadJson` is tested by `ReaderInputTests`, `ReaderValueTests`,
  `ReaderExtensionTests` and `MetamodelGraphTests` in the same project, which read the Python client's
  `client/python/tests/fixtures/metamodel/bigger.full.json` in place; `MetamodelExportTests` in the
  library's tests reads a live `api-json` export over all three wires (set `OPENSYSML_REQUIRE_SERVICE=1`,
  or it skips). `make dotnet-test` runs `samples/MetamodelReader` over that fixture through
  `dotnet-gates.py metamodel-sample`. A real export is the load-bearing check: `bin/sysml -convert
  api-json -stdlib` (about 136 MB, 112,031 elements) must read whole; with `CheckRanges` on about
  3,600 derived references are malformed only when read, and 30 values of `@type`s the metamodel lacks
  (`BooleanUsage`, `StructureUsage`, …) are malformed with checks off too. A read that throws on that
  file means a value failure escaped its property. With `KeepExtensions` the same read keeps about
  139,000 members (25,901 elements carry `sysx:sourceText`) for about 34 MB more retained memory. Negative controls worth repeating: make
  `GraphReader` throw on a malformed value (the theory and the stdlib read fail), drop the `@ref`
  branch (`UnresolvedReferencesFailOnlyTheirProperty` fails), and stop normalizing `Supertypes` keys.

## Conformance

```bash
make conformance-dotnet          # every protocol, then strict_conformance and oslc_query withheld
make conformance-dotnet-check    # beside the reference runner, scenario by scenario
dotnet run --project client/dotnet/src/OpenMBEE.OpenSysML.Conformance -c Release -- \
  -binary bin/sysml-grpc -protocols grpc -mutate truncate-lists
```

A `-mutate` run (`perturb-reals`, `truncate-lists`, `rewrite-strings`) must fail; one that passes
means the comparison is vacuous. A skip not in `test/testdata/expected-skips.json` fails the run, and
so does a listed skip that ran.

## The documented walkthrough

`docs/clients/dotnet.md` shows the Vehicle sample's using directives and its
`// <walkthrough>` region verbatim; `DocumentationTests` fails when they differ, and
`make dotnet-test` runs the sample against the real service through `dotnet-gates.py sample`, whose
expected model line must match what the guide prints. When changing the walkthrough, edit
`client/dotnet/samples/Vehicle/Program.cs` and the guide together, then run both. The cross-client tab
in `docs/guide/09-clients.md` is not gated: check it by pasting it into a scratch consumer with a
`ProjectReference` and a `vehicle.sysml` beside it.

## Documentation gates

```bash
python3 scripts/check-doc-links.py && python3 scripts/check-doc-ids.py && python3 scripts/changelog.py check
python3 -m venv /tmp/docs-venv && /tmp/docs-venv/bin/pip install -r docs-requirements.txt
PYTHON=/tmp/docs-venv/bin/python make docs     # strict: a page missing from the nav fails
```

After a self-model change (`examples/self-model/`), run `make self-model`, then the RDF round trip with
every require variable set. Without `OPENSYSML_REQUIRE_SPEC_EXAMPLES=1` and the spec examples
downloaded (`./scripts/download-spec-examples.sh`, which needs `pdftotext`), the gate prints
`GATE NOT RUN` and **passes as a skip**:

```bash
OPENSYSML_REQUIRE_TRAINING_CORPUS=1 OPENSYSML_REQUIRE_PILOT_CORPORA=1 OPENSYSML_REQUIRE_SPEC_EXAMPLES=1 \
  go test -count=1 -v ./tests/corpus -run RoundTrip
```

## Cleanup

Remove scratch consumers and any `dotnet-install.sh`. Confirm no `sysml-grpc`, FakeService, probe or
test host process remains.

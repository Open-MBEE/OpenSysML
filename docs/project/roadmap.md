# OpenSysML — Roadmap

Baseline: `main` @ `30f103bb`, the commit `v0.6.0` is tagged on, verified locally on 2026-09-07
with Go 1.25.0. Nothing is counted ahead of `main`: every status below is what that commit
carries, and the one pull request still open at it (#774, Track D) is stated as open.
Read `AGENTS.md` first; it governs everything below.

> **Labels.** This is an engineering record. The RDF items keep the `D` numbers (`D1`, `D2`,
> `D3.4`, `D7`, `D8`, `D9`, `D10`) that other records, the known-violations inventory and the ontology
> package's README cross-reference; `L` names the library items, `N` the native compilation
> track, `R` the release follow-through, `F` the executor defects the conformance gate carries as
> known failures, `S` the multiple-valid-executions work the executor needs before Track E,
> `E` the behavior-execution semantics the runtime does not yet have, `X` the expression forms it
> parses but does not evaluate, `Q` the runtime query surface, `A` analysis and simulation
> execution, `V` the validation census, `I` the language integrations, and `M` the embedded
> target. Each is stated in full where it is introduced, and a reader who wants only the gap can
> ignore the label.
>
> **Status words.** *Landed* means merged to `main` at the baseline. *Open* names a pull request
> that exists and is not merged; *conflicts* means it no longer merges cleanly onto `main` and
> needs a rebase before review. *In progress* means the work is being implemented and has no
> pull request yet. *Not started* means exactly that. A status is taken from the pull request
> itself, never from a branch name or a commit message.

`v0.6.0` is the newest tag on `Open-MBEE/OpenSysML` (`30f103bb`, 2026-09-07), after `v0.5.1`
(`d7b3eb45`, 2026-09-05) and `v0.5.0` (`0fdeb11e`, 2026-09-04); the tag's CI release job
publishes `sysml`, `sysml-lsp` and `sysml-grpc` for five platforms, the Windows installer and the
Homebrew bundles, and the Python client is on PyPI as `opensysml` 0.4.0. `CHANGELOG.md`'s
**0.6.0** section is the tag's content (#988 folded the fragments; the 0.5.1 and 0.5.0 sections
were folded the same way), and the baseline *is* the tag, so the tracks below state what a
`v0.6.0` binary does. The four fragments under `changes/unreleased/` are CI, build-tooling and
record changes and move no roadmap item. Everything in "Release follow-through" is maintainer- or
account-gated; everything after it is ordinary engineering work.

**What closed between `v0.5.0` (2026-09-04, the previous baseline's tag) and `v0.6.0`** — 109
merged pull requests, in `CHANGELOG.md`'s 0.5.1 and 0.6.0 sections for the detail, listed here
because each retires or narrows a roadmap line.

- *Analysis.* An analysis case runs (#979, closes A1): `-analysis`/`%analysis`, the `RunAnalysis`
  RPC and the Go and Python clients run an `analysis` definition or usage as the calculation it
  is, its body through the action executor, its `objective` and `assert constraint`s as verdicts,
  and an analysis usage's outputs read as features. The follow-ups shipped in the same release: an
  objective binds its requirement's subject by keyword alone, a case's result is readable by its
  qualified name, a recursive step reports once, later objectives keep their position, and an
  actor bound without `:>>` is held to the actor it inherits. An `%optimize` objective states the
  value to improve by redefining the trade-study library's `eval` calculation.
- *Expression and value kinds.* A constructor evaluates in a value position (#981, closes X1); a
  coordinate frame, a measurement scale, a measurement reference and a tensor quantity are runtime
  values, cross the gRPC/Connect API whole and are answered by a named reference's members; arrays,
  vectors and vector quantities cross the API in both directions; a derived `=` value follows the
  features it read and is invalidated with them; a quantity compares with a bare number and with
  zero; `sum` over an empty quantity collection is the declared kind's zero; a parameter redefined
  under a short name alone is bound at run time; a state usage typed directly by `StateAction` is
  exhibited; every transition guard is checked for a Boolean and a state body accepts the
  `if … then` target-transition spellings.
- *Validation.* Six more KerML structural rules, the end-feature, return-parameter and conjugation
  rules, three operator-expression checks, association arity and multiplicity bound types (0.5.1),
  the cross-feature rules, keyword-first relationship end kinds, `bool def` as a behavior
  definition, and the derived-name and short-name rules the reference validators apply; an
  invocation leaving a required parameter unbound is an advisory, `redefinition-type-mismatch` a
  warning, and specialization and binding conformance follow the reference more closely. The census
  now reads 162 of 217 constraints reported.
- *RDF.* A binding connector's ends are connector ends and may be named; anonymous `feature`,
  `event`, `snapshot`, `timeslice` and `assert` declarations, KerML `const`, `portion` and
  `member feature`, nested expressions and guarded successions all survive a graph-only round trip;
  a graph the notation would read back as a different model is refused rather than rewritten. The
  round-trip gate reads 346 of 346 models stable and none refused.
- *Queries, documents and migration.* Queries project `shortName`, `declaredShortName` and
  `documentation`, read quantity-valued and derived attribute values, and documents render query
  rows as prose; `-html-mermaid` and `-html-theme` for the HTML backend; SysML v1 models exported
  from Cameo/MagicDraw migrate to v2 (experimental).
- *Release.* Windows releases ship an installer and the Scoop, winget and MSYS2 manifests are
  maintained as templates (R4); the Linux `amd64` binaries are static and no longer require glibc
  2.34; pull requests run one CI (GitHub Actions) and CircleCI runs on `main` and tags; the
  SonarCloud findings are cleared.

The previous baselines' retirements — native compilation's first phase, the wire contract, L3–L6,
the nested-action frames, D3, Q4, the changelog fragments, the census and its gate, the
enumeration rules (#907, #909) — stay retired and are not repeated.

## Where the repository stands

Full gate green: `gofmt -l .` empty, `go build ./...`, `go vet ./...`, `go test ./...`, and the
corpus gates run locally clean at the baseline, with the pilot material re-fetched at its pin
(`./scripts/download-pilot-corpora.sh`, `./scripts/download-pilot-xpect.sh`). A stale local copy
of the pilot corpora fails `cmd/pilot-diff`, `cmd/pilot-xpect` and the `TestPilotCorpora` gate
with a provenance message naming the drift; that is the gate working, not a regression — re-fetch
before re-recording anything.

| Gate | Count at `main` @ `30f103bb` (`v0.6.0`, 2026-09-07, Go 1.25.0) |
|---|---|
| OMG training corpus | **100/100 clean** — asserted, not ratcheted: no file reports a semantic error |
| OMG pilot corpora (ratchet) | 213 files; 6 report a diagnostic, each adjudicated in [pilot-corpora.md](pilot-corpora.md) and [omg-issues.md](omg-issues.md) |
| Stdlib parser conformance | 98/98 clean — 94 vendored OMG files and 4 non-normative OpenSysML extensions |
| Execution conformance cases | 674 under `TestExecutionConformance`: 671 run and pass, 3 are skipped as the known failures Track F names |
| Golden execution traces | 140 (`TestExecutionTrace`) |
| Runtime robustness cases | 336 first-level subtests of `TestRuntimeRobustness` |
| gRPC conformance fixtures / robustness cases | 15 / 8 |
| Golden AST fixtures | 195 (`TestGolden`: 169 SysML, 26 KerML) |
| Negative parser subtests | 249 first-level subtests of `TestNegative` (338 across the `TestNegative*` functions, 396 across every `*Negative*` parser test) |
| Rejection oracle | 285 self-authored invalid models: 273 both reject by default and 276 when we are asked strictly, 3 the pilot alone by default and none strictly, 9 ours alone (the control-node rules the pilot leaves unimplemented and a non-Boolean succession guard) |
| Validation census | 162 of 217 named constraints reported (156 faithful, 6 approximate), 1 not implemented, 1 deliberate, 53 unknown |
| RDF corpus round trip | 346 of 346 models stable, none refused |

The pilot differential, the Xpect oracle, the scope oracle and the rejection oracle are the
external conformance statement, and their figures are generated into `README.md` by `make
docs-counts` from the committed baselines; they are not repeated here.

The test-suite figures above are counted from `go test -v` at the commit the table names; the
other surfaces `releasing.md` allows to repeat them (`README.md`, `docs/project/spec-compliance.md`,
`docs/project/training-examples.md`) state the same figures, quoting the 671 running conformance
cases without the 3 skipped ones. They are still typed in by hand; folding them into
`cmd/doc-counts` so they are generated like the pilot figures and can no longer drift is a small
open item and is listed under sequencing.

Statement coverage, measured with `go test -cover ./...` at the baseline. It counts only each
package's own tests, which understates a package consumed by others (`internal/core/ast` is
exercised by every parser test; `cmd/sysml-grpc` is gated by a process lifecycle test whose child
process contributes no profile).

| Package | Coverage | Package | Coverage |
|---|---|---|---|
| `internal/core/quickfix` | 100.0% | `internal/core/symbols` | 84.8% |
| `internal/core/conformance` | 100.0% | `internal/core/solve` | 84.3% |
| `internal/core/ast/astcodec` | 99.5% | `internal/core/identity` | 83.5% |
| `internal/core/format` | 97.2% | `internal/core/project` | 81.0% |
| `internal/core/docrender` | 94.4% | `internal/core/queryexec` | 80.9% |
| `internal/core/rdf/ontology` | 92.1% | `internal/core/resolve` | 76.3% |
| `internal/core/libs` | 90.0% | `client/opensysml` | 74.7% |
| `internal/grpc` | 89.8% | `internal/core/query` | 73.2% |
| `internal/core/passes` | 89.8% | `internal/core/lower` | 71.3% |
| `internal/core/parser` | 89.7% | `cmd/sysml-lsp` | 70.0% |
| `internal/repl` | 89.6% | `internal/core/semantics` | 60.9% |
| `internal/core/export` | 89.2% | `internal/interop/flexo` | 39.5% (the live-stack half is gated) |
| `internal/core/migrate` | 89.1% | `internal/core/ast` | 21.8% |
| `internal/lsp` | 88.3% | `cmd/sysml` | 18.3% |
| `internal/core/runtime` | 88.1% | `internal/core/codegen` | 1.9% (its differential runs from `internal/repl`) |
| `internal/core/rdf` | 87.1% | | |
| `internal/core/queryplan` | 85.6% | | |

The corpus gate needs the corpus (`./scripts/download-training-examples.sh`) and never
re-baseline `internal/core/model/testdata/training_examples_expected.txt`: adjudicate each
drifted file and record the verdict in `docs/project/training-examples.md`.

A tag cannot be cut over a corpus regression: `.circleci/config.yml`'s `build-and-test`
downloads the corpora (cached on the download scripts) and runs the suite with
`OPENSYSML_REQUIRE_TRAINING_CORPUS=1` and `OPENSYSML_REQUIRE_PILOT_CORPORA=1`, on `v*` tags as
well as on `main`; the GitHub Actions pull-request workflow does the same before a merge.

---

# Release follow-through

Tagging a core release, publishing the Python client and the Homebrew bump are all proven paths:
`v0.6.0` is tagged and its release job runs the same path `v0.4.2` and `v0.4.3` completed with
their full archive sets, `opensysml-v0.4.0` uploaded the client to PyPI,
and the tap `Open-MBEE/homebrew-tap` bumps itself from its own scheduled workflow on each tag,
rendering the formula from this repository's `scripts/render-homebrew-formula.sh` and template.
The procedure and its post-tag verification are in `docs/project/releasing.md`.

## R1 — the changelog agrees with the tags (done)

`CHANGELOG.md`'s **0.4.3** section holds what `99e02003` shipped (#877), its **0.5.0** section
what `0fdeb11e` shipped (#894), its **0.5.1** section what `d7b3eb45` shipped and its **0.6.0**
section what `30f103bb` shipped (#988), each dated to its tag; **Unreleased** is empty in the file
because since #852 a change adds a fragment under `changes/unreleased/` instead, and
`python3 scripts/changelog.py release X.Y.Z` folds the fragments in at release time
(`releasing.md`). The 0.5.0 fold was the first release cut that way and 0.5.1 and 0.6.0 followed
it. Nothing remains here; the item is kept because the folds are the procedure's proof.

## R2 — the Node, Java and Rust clients are unpublished

Each has its release workflow (`client-node-v*` to npm, `opensysml-java-v*` to Maven Central,
`opensysml-rust-v*` to crates.io) and a worked example the tests run, and none has ever been
tagged. The Java package name already moved to `org.openmbee.opensysml`, the DNS-verified
namespace, so nothing blocks Maven Central but the account. npm and crates.io need a publisher
token in CI; Maven Central needs the Sonatype account and a signing key. These are account gates
like R4, not engineering.

## R3 — Homebrew: install it on a real Mac

Everything about the tap is automated and verified on Linux (install, `brew test`,
`brew audit --strict --online`), and the manual pages (#699, `man/man1/*.1`, generated from
`internal/usage` and drift-gated by `make man-check`) are in the bundles the formula installs. The
one thing never done is running the darwin bottle on macOS: the darwin archives' checksums match
the release manifest and nothing more.

`homebrew/core` — which would drop the tap and the trust step entirely — is gated on
[notability](https://docs.brew.sh/Package-Acceptance-Policy#notability) (75 stars / 30 forks /
30 watchers, or 225 / 90 / 90 self-submitted), so it is not a near-term option.

## R4 — code signing

macOS binaries are not Developer ID signed or notarized, so a browser download trips
Gatekeeper. Root-caused in `docs/project/macos-distribution.md`: it is `com.apple.quarantine`,
not a missing signature — Go's linker already ad-hoc signs darwin/arm64 — so ad-hoc `codesign`
in CI would change nothing. Notarization needs an Apple Developer account, a Developer ID
certificate, an App Store Connect API key in CI and a macOS runner: a purchase, not a task.

Windows Authenticode signing is prepared, pending approval by
[SignPath Foundation](https://signpath.org), which signs open-source Windows binaries for free
from a build it can verify the origin of. The repository side is done: the README carries the
[Code signing policy](../../README.md#code-signing-policy) the terms require, the three Windows
executables embed the `VERSIONINFO` SignPath enforces (`ProductName` `OpenSysML`,
`ProductVersion` from the tag), and `.github/workflows/release-windows.yml` rebuilds them on a
`v*` tag under GitHub Actions — a trusted build system for SignPath, which CircleCI is not —
submits them for signing, and publishes the signed files as `*-signed*` release assets beside
the unsigned ones that the cosign-signed manifest keeps covering. What remains is a
maintainer's application at <https://signpath.org/apply>, the SignPath project and policy
setup, the `SIGNPATH_*` secret and variables in GitHub, and one manual approval per release;
until then the workflow builds and stops. Procedure: `docs/project/releasing.md`, "Windows
Authenticode signing".

Windows packaging is in place on the same workflow: a WiX v5 MSI (`packaging/msi`,
`scripts/build-msi.sh`) installing the three executables to `Program Files\OpenSysML` on `PATH`,
with the Z3 solver as an optional feature pinned by hash, published unsigned as
`opensysml-<x.y.z>-windows-amd64.msi` and, once SignPath is configured, rebuilt from the signed
executables and itself signed as `*-signed.msi` (Z3 stays unsigned by SignPath's terms). Scoop,
winget and MSYS2 manifests that depend on Z3 rather than bundle it are maintained as templates
under `packaging/` with render scripts; what remains there is a maintainer submitting each to
its external repository (and, for winget, confirming the `OpenMBEE.OpenSysML` identifier and a
Z3 package to depend on). Procedure: `docs/project/releasing.md`, "The Windows installer".

## R5 — the VS Code extension is not released

`editors/vscode` builds only as a PR CI artifact: no `.vsix` is attached to a release and there
is no marketplace or Open VSX listing, so a user cannot install it without building it. The
client works against the shipped `sysml-lsp` (`--stdio` is accepted and `shutdown`/`exit` are
honoured). What remains is packaging and publishing: `vsce package` in the release workflow, a
`.vsix` on the release, and (for the marketplace) a publisher account and a PAT in CI — the same
class of account gate as R4.

## Upstream follow-through

Filed and waiting on the other side: the identity-annotation enhancement against SysML 2.0
(`INBOX-2510`, maintainer-approved 2026-09-01) and the `ownedDisjoining` EMF defect against the
pilot (`SysML-v2-Pilot-Implementation#790`). Drafted and waiting on a maintainer to authorise
posting: the three dimensional-analysis errata in the pilot's example corpus and the question
about the `queryx/failing` Xpect fixtures. All four are in [omg-issues.md](omg-issues.md), body
and status; none needs code here until an answer arrives.

---

# Track L — the standard library at run time

Loading is closed: every library file is parsed and indexed on every load path, built once and
frozen, read through a per-model overlay (`libs.Loader`, `libs.SharedBase`, `symbols.NewOverlay`)
rather than copied per model, and the serialized snapshot brings a process up in under 20 ms.
Name resolution is closed too: a qualified name through a `public import` (`ISQ::speed` or any
user façade package) evaluates (#801), an unqualified library call resolves at evaluation exactly
as the checker resolved it (#816), and an overloaded name reaches the declaration the argument
types select (#825, #861). The library review after `v0.4.3` measured *evaluation* against the
Kernel Function Library's 279 declarations in 17 packages and against the domain libraries; of its
findings L3–L6 have landed and L7 — the analysis libraries — is what remains, and it is a Track X
item in library clothing.

## L3 — evaluation does not reach standard-library-inherited features

A feature a user type inherits from the standard library has no value at run time. With `part def
Box :> Item; part b : Box;`, both `b.isSolid` (declared `isSolid = isEmpty(voids)` in `Systems
Library/Items.sysml`) and `b.voids` report `member … not found in instance`; the cause is
`runtime/shape.go` skipping every `libraryDeclared` feature when it lays out an object, so
`Model.Eval` never gets as far as folding the library's value expression. **Landed:**
[PR #830](https://github.com/JPL-Devin/OpenSysML/pull/830) materializes Systems and Domain library
features on objects in tiers (a feature's own value, then inherited value expressions); at this
baseline `%instantiate L::b; %eval in b : isSolid` answers `true` and `voids` `[]`. The solver has
the mirror gap and is not in that PR: its translatable subset does not take library-declared
conditions, and `solve`'s
differential harness still indexes the standard library as ordinary documents (`parseLibraries`)
to reach them. [lossless-library-records.md](lossless-library-records.md) records
what was measured.

## L4 — every Kernel Function Library declaration dispatches by name

At the previous baseline ~135 runtime names were registered explicitly (`runtime/builtin_names.go`,
`library_functions.go`), operators and the core builtins were handled separately, and **12 of the
17 KFL packages had no dispatch gate**: a call to one of their ~38 named functions — every
`ToString`/`ToInteger`/`ToReal`/… conversion in `BaseFunctions`/`ScalarFunctions`, `Rational
floor/round/gcd`, `sum0`, the generic `max`/`min`, the explicit operator-call spellings `'+'(a, b)`
— fell through to `no result expression` naming the wrong thing, instead of either computing or
failing as `library function X is not implemented`. **Landed:**
[PR #821](https://github.com/JPL-Devin/OpenSysML/pull/821) adds the gate and implementations for
every KFL declaration; a call now either computes the library's stated result or is refused by
name (`ToString(42)` is `"42"`, `max(1, 2)` is `2`, `max(1.0, 2.5)` is `2.5` at this baseline).
The named-reducer forms (`->reduce '+'`) are the same mechanism read from a different position and
are X5 below; which overload a name reaches is L6.

## L5 — `QuantityCalculations`, unit canonicalization and domain failures

`QuantityCalculations` (28 declarations) had no runtime registration at the previous baseline, and
a model that imported it lost `->sum()` over quantities (measured; the import brought a `sum`
declaration nothing dispatched into scope ahead of the scalar one). Composed units had no canonical
rendering. **Landed:** [PR #818](https://github.com/JPL-Devin/OpenSysML/pull/818) dispatches the
package and renders composed units canonically — with the import in scope, `(2 [kg], 3 [kg])->sum()`
is `5 [kg]` and `2 [m] * 3 [m]` is `6 [m**2]` at this baseline. What the same probe shows is L6's
gap: with `ScalarFunctions` and `QuantityCalculations` both imported, `ToReal("2.5")` reaches
`QuantityCalculations::ToReal` and is refused for its argument type instead of selecting the
scalar overload. Domain failures (`sqrt` of a negative quantity, a unit mismatch in `+`) must stay
typed errors naming the function and the units, as the scalar library's already do.

## L6 — invocation overloads selected by argument type (done)

The checker and the runtime used to pick a library overload by name and arity, so `max(1, 2)` and
`max(1.0, 2.0)` reached the same declaration and the result type was whatever that declaration
said. **Landed:** [PR #825](https://github.com/JPL-Devin/OpenSysML/pull/825) selects by argument
type with one selection shared between `semantics` and `runtime`, and
[PR #861](https://github.com/JPL-Devin/OpenSysML/pull/861) makes every reader of a call —
expression calls, document queries, `send` — use it; [PR #862](https://github.com/JPL-Devin/OpenSysML/pull/862)
keeps a calc that specializes a library function on its own signature. The `ToReal("2.5")` probe
under L5 reaches the scalar overload. On an ill-formed model the runtime package now agrees with
the checker about a named argument it cannot place (#904, #908): the label is kept as written and
refused as `ErrUnknownParameter` rather than bound by its last segment.

## L7 — the analysis libraries do not run

At this baseline, against `bin/sysml`: `new SampledFunction(samples = (new
SamplePair(0.0, 0.0), new SamplePair(1.0, 2.0)))` constructs (X1 landed) and `Domain(fn)` answers
`[0.0, 1.0]`, but `interpolateLinear(fn, 0.5)` still fails inside the library's own `Linear` —
`operator '-' is not defined for a Real and a sequence` evaluating its `f`, which subtracts
`lowerSample.domainValue`. The generic half of X2 is in (a `Real[0..*]` feature holding one value
takes part in arithmetic); what this library hits is the remaining half, a singleton read *through
a feature chain on an object* (`sp.domainValue`, the library's `KeyValuePair::key` redefined
without a multiplicity), which still arrives as the sequence `[0.0]` and is refused by every
operator. `SampledFunctions::Sample` additionally
needs a function-typed parameter (X6): `Sample(Sq, (1.0, 2.0))` with `Sq` a calc def is refused
`cannot evaluate definition Sq`. `StateSpaceRepresentation` depends on A4; `TradeStudies` loads,
type-checks and is A2. The vector half moved earlier: since #883 a `NumericalVector` and a
`VectorQuantity` are value kinds of their own, `VectorFunctions` computes over them and
`OccurrenceFunctions` evaluates over lifetimes (#884), and the 0.6.0 release added the tensor
quantity, coordinate-frame and measurement-scale values the measurement reference libraries
declare; `TensorValues` proper and the set kind are what is left of X7. Closing this item is
closing the chain-read half of X2 and X6 and then adding a *measured per-library conformance
table* — package → declarations → evaluated → refused by name → wrong — to
`spec-compliance.md`, so the library's status stops being anecdotal. Not started as an item; it
depends on the X items it names.

---

# Track N — native compilation

The interpreter in `internal/core/runtime` is the reference semantics and is fast in absolute terms
(about a microsecond per calc invocation after the September pass), but a compute-bound analysis —
a recursive calc, a long numeric loop — is still three orders of magnitude off native code. The
goal is that a calc, and eventually a whole analysis case, can be compiled ahead of time into a
standalone native program that computes exactly what `sysml -calc` computes, prints it the same
way and fails on the same inputs, or refuses to compile with a typed error naming the construct
outside the subset. Nothing is compiled approximately.

## N1 — scalar and collection calcs to C or Go (landed)

`sysml model.sysml -compile Pkg::Fib -o fib` translates a `calc def` (or a calc usage) into an
executable through C (`cc -O3 -flto`, the default) or Go, with `-source` writing the generated
source alone; the design, the measurements and the phase plan are in
[native-compilation.md](native-compilation.md). The pipeline is `parser → resolve → semantics`
plus `lower.CalcBody` into `codegen.Compiler`, a typed IR (`codegen.Program`), and
`EmitC`/`EmitGo`. Landed in four pull requests: #778 the scalar subset (`Integer`/`Natural`/
`Positive`/`Real`/`Boolean` parameters, literals, checked arithmetic and comparison, the logical
and conditional operators, body-local attributes and assignment, `if`/`while`/`loop … until`,
direct and mutual recursion); #796 statement bodies, redefined parameters, the scalar library
intrinsics and named arguments; #828 homogeneous sequences of the scalar types with any
multiplicity, the shape rules, `for`, the sequence and control libraries and the element budget;
and, on the interpreter side, #781's closure fast path for pure calc bodies, which is the tier
between interpreting and compiling. Everything outside the subset refuses with
`codegen.UnsupportedError` naming the construct; a differential test runs every compiled program
against the interpreter in both backends, and the gate stays green with `cc` absent (the C tests
skip; the Go ones do not).

Measured (Xeon 8559C, GCC 11.4, 2026-09-02): `Fib(25)` 261 ms interpreted, 919 µs as Go, 221 µs
as C; `SumTo(1000000)` 1216 ms / 764 µs / 379 µs; `Collatz(27)` 206 µs / 5.2 µs / 0.98 µs. C beats
Go by 2–5× on every loop or recursion, which is what justified keeping two backends: C is the
default and Go the fallback where no C compiler is installed.

## N2 — the rest of the value phase, and the whole-model phases

[native-compilation.md](native-compilation.md) states the target as a dependency-free executable
or a library with a small C API that runs a whole model, in seven phases: values, instances,
constraints and requirements, documents, actions, state machines, embedding. Phase 1 is half done
(the collection half above). In the order they matter:

1. **Records, enums and record field access** — the remainder of phase 1. `type X is not
   Integer, Real or Boolean` is the refusal today, as is a sequence mixing `Integer` and `Real`
   elements, and the interpreter's new `Array`/`NumericalVector`/`VectorQuantity` kinds (#883)
   are refused by the compiler by the same rule until they have a native layout. Quantities with
   units and structured parameters follow from the record type. This is what stands between "a
   calc compiles" and "an analysis case compiles" (A1).
2. **The step budget.** The interpreter stops a runaway loop at `OPENSYSML_MAX_STEPS`; compiled
   code counts elements (the element budget landed with #828) but not steps, so a `while` that
   never terminates runs forever. Decide whether a compiled program carries an optional iteration
   counter or whether "a compiled program is a program" is the documented contract; N2.4 and M1
   both wait on the answer.
3. **The rest of the library.** What L4 registers in the interpreter needs C and Go equivalents
   with the interpreter's exact results and failure behaviour (domain errors, Integer range) as
   each package is reached; the interpreted/compiled differential is the gate.
4. **A stable C ABI.** One entry point per compiled calc — typed arguments in, a typed result or a
   typed failure out, no allocation the caller does not control — would let the REPL and
   `sysml-grpc` call a compiled calc in place of interpreting it, is what I4 (the C client) links
   against, and is what M4 (the embedded host interface) is a restriction of. Design it once for
   all three; not before N2.2 is decided, since a linked-in calc must honour the host's budget.
5. **Packaging.** The C backend needs GNU C (`__int128`, `__builtin_*_overflow`,
   `setjmp`/`longjmp`); document the toolchain requirement in the install guide, and decide whether
   the release bundles should carry a prebuilt runtime shim.
6. **Phases 2–7** — instances, constraints, documents, actions, state machines, embedding — in the
   record's order. Actions and state machines are token and event semantics, not arithmetic, and
   the interpreter's trace is their contract; their closed IR is Track M's first item (M1), so the
   two tracks meet there rather than duplicating the work.

---

# Track D — model persistence and RDF interchange

Saving and SysML ↔ RDF Turtle conversion landed (`internal/core/rdf`,
`internal/core/export`, `%save`, `sysml -convert`, `-sync-diff`); see
[the RDF mapping](../reference/rdf-mapping.md).

The RDF direction ships **experimental**, because of D1, D2 and D7 below: its vocabulary
may change without a compatibility path, and the one triplestore interop measured — Flexo — still
drops what those items carry. Every surface says so (`export.ExperimentalNotice`), and promoting
it to stable is re-measuring the harness once those land, not a documentation change.

Measured by the per-file ratchet at this baseline (`TestCorpusRoundTrip`,
`internal/core/export/testdata/corpus_roundtrip_expected.txt`), **all 346 models under
`examples/` convert** (the training corpus, the three pilot corpora and this repository's own
demos), every one round-tripping `notation → RDF → notation → RDF` byte-identically; there is no
whitespace-only, graph-diff, unwritable, unparseable or refused verdict left — each file is
pinned, so a movement in any direction has to be adjudicated
([rdf-corpus-roundtrip.md](rdf-corpus-roundtrip.md)). A model the mapping cannot write back is
still refused rather than converted lossily; the corpus simply no longer contains one. The
previous baseline's last refusal class — 40 declarations naming no element of their own (anonymous
`feature`, `event`, `snapshot`, `timeslice` and `assert`) — went in 0.6.0: the graph types the fact
the keyword states (`sysml:portionKind`, `sysml:EventOccurrenceUsage`, `sysml:AssertConstraintUsage`
with `sysml:isNegated`, the named occurrence or constraint as `sysml:references`) and records
`sysx:declaredKeyword`, so the decoder spells the head from the typed facts, and a graph whose
keyword contradicts its typing is refused naming the element. Before that, the 13
result-expression refusals went with #815 and #835 (a body's trailing `a - b` is a
`ResultExpressionMembership` owning the expression itself), the 19 prefix-metadata refusals with
#824 (metadata bodies and prefix metadata are owned `MetadataUsage`s), and the
duplicate-declaration refusals once the parser read the anonymous `connector a to b;` as ends
rather than as a connector named `a`. #827 spells a reference back so it re-resolves to the
element the graph named and #855 links a reference reached through an import or alias. Nothing is
in flight against these numbers.

## The target, stated precisely

The goal is that a graph OpenSysML writes can **stand in for the RDF
`flexo-mms-sysmlv2` produces**: loaded straight into `flexo-mms-layer1-service` as a
branch's model graph, and read back through the SysML v2 API surface as the same
elements, without that service having produced it. Two consequences shape D1–D3, and
both were read from the two services' sources rather than from our own docs:

- **Layer 1 imposes no vocabulary at all.** `routes/gsp/ModelLoad.kt` loads whatever
  triples the request body carries into a load graph, diffs it against staging and
  commits; sanitization (`sanitizeCrudObject`) applies to LDP CRUD objects — orgs,
  repos, branches, policies — not to model triples. So layer 1's requirements are
  transport-level: a Turtle body on `PUT .../branches/{branch}/graph` (or SPARQL update
  on `.../update`), the ETag precondition, an optional `?message=`, and the literal-size
  limit (`maximumLiteralSizeKib`). Named-graph layout, commits, locks and provenance are
  layer 1's own and are not ours to emit.
- **The vocabulary contract belongs to the reader in `flexo-mms-sysmlv2`.**
  `ElementApi.extractModelElementToJson` is what turns triples back into API payloads,
  and it is stricter than `Namespaces.kt` suggested. It keeps `sysml:` and
  `urn:sysmlv2:annotation:json:` predicates and **ignores everything else** (the
  unrecognized-predicate error is commented out), so every `sysx:` triple is dropped the moment
  a graph passes through that service. Whatever a model needs in order to survive has to be
  standard, which is what makes D1 and D2 part of the interop goal rather than refinements
  after it.

What matches today: `sysml:` = `https://www.omg.org/spec/SysML#` and `elmt:` =
`urn:sysmlv2:element:` are identical to `Namespaces.kt`; `rdf:type` plus `sysml:<property>` per
scalar field is the shape the reader expects; our typed literals fall in the datatypes it maps;
every element carries `sysml:elementId` equal to the id its IRI ends in, so listing by id and the
`@id` derivation agree; and ownership is materialized as the abstract syntax states it, so the
roots endpoint sees one root per document. Those were D3.1–D3.3 and D3.5, closed and now measured
rather than claimed — see the next section.

## D3 — make a converted graph readable through Flexo, and prove it

The harness is `internal/interop/flexo`, the `FLEXO_INTEROP` gate `TestFlexoInterop`, documented
in `.agents/skills/flexo-interop`; it brings up the published `openmbee/*` images, `PUT`s our
Turtle to a branch graph, reads every element back through `flexo-mms-sysmlv2` and compares with
what the service's own commit path stores for the same model. It measures the gap instead of
asserting the fix, so every item below shows up as movement in
`internal/interop/flexo/testdata/interop_expected.txt`. Keep it out of `go test ./...`.

What the current recording measures, for the identity-carrying fixture: **49 of 49 elements
listed and 369 of 452 properties delivered** on the graph-load side, against 33 of 33 and 158 of
158 for the same model posted through the service's own commit path; 9 of 49 read as roots, and 9
have no owner in the model; every element is readable directly by id; no subject of the graph is
outside the element namespace. **Every standard property is delivered, the 14 multi-valued ones
included** — `ownedMember`, `ownedMembership`, `ownedRelationship` (3/3 each), `ownedFeature`,
`ownedFeatureMembership` (2/2 each), `specializes` (1/1) — since D3.4 landed. The 83 lost
properties are one thing:

- **10 property keys in `sysx:`** — `sourceText`, `sourceTail`, `sourceLanguage`, `hasBody`,
  `memberIndex`, `argumentIndex`, `declaredKeyword`, `endForm`, `endIndex`, `relatedFeature` —
  dropped unread. That is the D1/D2 residue below, and it is the reason the expression trees and
  end structure the mapping now writes do not survive the hop. (The one multi-valued property
  still lost, `relatedFeature` on 0/1, is among them.)

The commit path delivers 6 of 6 of its own multi-valued properties, because it stores each array
whole as a JSON annotation literal alongside the typed triples; the graph now carries the same
literal, and the two paths deliver every array alike. Two deployed behaviours differ
from the sources: the element listing ignores `pageSize`/`pageAfter` and returns every subject,
and project delete is a soft annotation that leaves the Layer 1 branch behind.

### D3.4 — collection-valued properties need the JSON annotation (done)

The reader **skips** a `sysml:` predicate with more than one object and prefers the
annotation literal at `urn:sysmlv2:annotation:json:<key>`, which it parses as JSON.
Anything multi-valued emitted as bare repeated triples alone is silently dropped on read.
[PR #850](https://github.com/JPL-Devin/OpenSysML/pull/850) closed it, and D3 with it: one
`json:<key>` literal per multi-valued `sysml:` property beside the typed triples (shape taken from
the service's `CommitApi.kt`, cited in `rdf-mapping.md` § Collections), a decoder that accepts
either spelling or both and refuses a graph whose two spellings disagree, and `reposync` keeping
the literal in step when it mints ids. Re-recorded against the live stack, the multi-valued
standard properties went from 0 of 14 to 14 of 14 delivered, and the total from 355/424 to 369/452
(the denominator moved with the source-text properties the mapping added since the previous
recording; the one multi-valued property still lost is `sysx:relatedFeature`, D1/D2 residue).

## D1 — expression trees are standard in shape, non-standard in vocabulary

Every expression-valued position — a feature value, a multiplicity bound, a guard, a filter, a
condition, a send payload — is now a **tree of typed nodes** in the `expr:` namespace
(`rdf-mapping.md` § Expressions): standard metaclasses (`OperatorExpression`,
`FeatureReferenceExpression`, `LiteralRational`, …), `sysml:argument` and `sysml:referent`
linking operands and referents, a deterministic per-position id every node states in
`sysml:elementId`, and a decoder that reads a foreign tree from its structure alone. SPARQL can
see inside a value now; "every part whose mass exceeds 1000" is expressible.

What remains is what the Flexo hop still loses and the metamodel still does not recognise:

- the operator, the operand order and the source text ride in `sysx:` (`sysx:operator`,
  `sysx:argumentIndex`, `sysx:sourceText`), so after the hop a tree keeps its nodes and loses
  their meaning. The metamodel spells the operator `OperatorExpression::operator` and orders
  arguments through `ownedFeatureMembership`s; emit those;
- a node is not a model element — no `qualifiedName`, no ownership, reachable only from the
  position that holds it — where the abstract syntax makes an expression a `Feature` owned through
  a `FeatureMembership`. Writing expressions as owned elements is the same materialization D3.3
  did for ownership, and it is what the ontology gate's `value` → `FeatureValue` findings (D8) are
  waiting on;
- ~~an expression standing as a body member~~ — done: a calc's trailing result expression is a
  `ResultExpressionMembership` owning the expression (#815, #835,
  [rdf-mapping.md § Result expressions](../reference/rdf-mapping.md#result-expressions)), and no
  file in the ratchet is refused for an expression.

## D2 — end bindings are structure, but in `sysx:`

`connect`, `bind`, `flow`, `succession`, `transition`, `accept` and `satisfy` now state their ends
as structure beside the verbatim head — one expression node per end under `sysx:relatedFeature`
with `sysx:endIndex`/`sysx:endRole`, and `sysx:endForm` naming the notation the ends are written
in — so a graph from another tool converts to notation with no text at all, and a succession
carries both its ends including the unnamed member a `then` sequences (`rdf-mapping.md`
§ End-binding heads). The form is stated only when rebuilding it reproduces the head exactly;
heads that state more than their ends (a multiplicity, a `references` clause, an inline payload
declaration, a body) stay text-only and are reported, not guessed, when the text is absent.

What remains: the vocabulary is ours, so the hop drops it (`endForm`, `endIndex`,
`relatedFeature` are three of the eight lost keys). The metamodel's shape is
`Connector::connectorEnd` — end features owned through `EndFeatureMembership`s — with
`sourceFeature`/`targetFeature` over them, the same `Connector_sourceFeature`/`targetFeature`
domain findings the ontology gate records for transitions (D8). Emitting those is an
encoder/decoder change, not a parser one; the ends are already in hand. The ends that are not a
basic name (`drive vehicle`, `1stGear`) convert since #814 by quoting them in the head text; a real
end triple would name the element by IRI and need no quoting, which is the same change.

## D7 — reference-valued properties are emitted as strings, and one metaclass is abstract

The reader turns a resource-valued object into `{"@id": …}` and a literal into a string, so a
property the API defines as a reference has to be an element IRI in the graph. `imports.golden.ttl`
shows both halves of this gap: `sysml:importedNamespace "ISQ"` is a string where the API expects
a reference, and the metaclass is `sysml:Import`, which is abstract in KerML — the API's own
elements are `NamespaceImport` or `MembershipImport`.

The reference-vs-literal half is mechanized against the OWL ontology (D8):
`TestGoldenGraphsMatchOntology` (`internal/core/export`) checks every SysML-namespace triple in
the 54 golden graphs against the metamodel's declared domain and range, finds **412 triples in 79
distinct metaclass/property violations** at this baseline (the count grew with the fixtures the
metadata, result-expression, reference and anonymous-declaration work added, not with new kinds of
disagreement), and
every one is inventoried key-by-key with a reason in
`internal/core/export/testdata/ontology-known-violations.txt`, so any *new* disagreement fails the
build. The object-property-carrying-a-literal group is this item's own bug: `type` on
`AttributeUsage`, `ReferenceUsage` and `PartUsage`, `sourceFeature` on `SuccessionAsUsage` and
`sysx:InitialNode`, `referent` on `FeatureReferenceExpression` where the referent resolves outside
the graph, and `targetFeature` on `FeatureChainExpression`. #827 and #855 narrowed the *decoder*
side — a name written back re-resolves to the element the graph named, through imports and
aliases — but the encoder still writes the name as a literal, which is what this item is. Identity is stable (D3.1), so each is
mechanical: resolve the name and emit the IRI, and fall back to the literal only where the
referent is outside the graph, as feature references already do. The abstract-metaclass half is
not mechanizable from the ontology: `SysML.owl` records no ecore abstractness (see D8), so nothing
in the suite catches `sysml:Import` being abstract, and that audit against the API's own element
list stays manual.

## D8 — an optional second output profile: the Open-MBEE SysML v2 OWL ontology

[`Open-MBEE/sysmlv2-rdf-ontology`](https://github.com/Open-MBEE/sysmlv2-rdf-ontology) renders the
OMG metamodel (version 202407, from `SysML.ecore`) as OML and OWL: `SysML.owl`, 172 classes, 348
object properties, 63 datatype properties, with `rdfs:domain`/`rdfs:range` on each. It uses the
*same* namespace we do and its class IRIs are the plain metaclass names we already emit; the
difference is the properties, each qualified by the metaclass that defines it
(`sysml:Element_declaredName`, `sysml:Element_owner` with range `OwningMembership`), and a
conformant instance graph therefore materializes the abstract syntax's relationship elements
rather than collapsing them.

So this is a **second profile selected by a flag, not a superset**: the property IRIs differ, so
one graph cannot satisfy both conventions, and Flexo's convention stays the default. The encoder
already separates the term layer (`rdf.SysMLTerm`, `internal/core/rdf/vocab.go`) from the
structural decisions (`internal/core/export/convert.go`), so the profile is mostly a term-mapping
layer: property name → defining metaclass.

**Done:** the table and the gate. `internal/core/rdf/ontology` holds the term table generated
from `SysML.owl` by `internal/core/rdf/ontology/gen` from a local checkout (version `202407`,
upstream commit in the generated header): 411 properties spanning only **336 distinct unqualified
names — 59 names are declared by more than one metaclass** (`type`, `value`, `source`, `target`,
…), so the unqualified convention is genuinely lossy in the other direction and a profile encoder
has to pick by the subject's metaclass (`LookupProperty` returns every declaration;
`AmbiguousNames` reports the set). The gate is `TestGoldenGraphsMatchOntology`, whose inventory is
also the profile's work list, sorted into five causes: properties the metamodel declares on a
relationship or membership element that we collapse into the element (`value` → `FeatureValue`,
the multiplicity bounds → `MultiplicityRange`, `isNegated` → `Invariant`, a transition's ends →
`Connector`) — the same collapse D3.3 undid for ownership and D1/D2 will undo for expressions and
ends; names as literals (D7); metaclass names the 202407 rendering does not have (`FlowUsage`,
which it calls `FlowConnectionUsage`, and `TerminateActionUsage`); the metaclasses of our own
`sysx:` namespace; and the properties we write into the SysML namespace that no metaclass
declares, each either a relationship the metamodel reifies as an element (`specializes`,
`subsets`, `redefines`, `references`, `aliasedElement`, `via`) or a notation flag with no
metamodel property (`isAccept`, `isResult`, `isSnapshot`, `isTimeslice`, `isChain`) — arguably
those belong in `sysx:` regardless of this item.

**Open, conflicts:** [PR #774](https://github.com/JPL-Devin/OpenSysML/pull/774) (needs a rebase onto
the current `main` before review) ships the ontology itself as **41 leaf Turtle modules and 6 layer ontologies** under
`ontology/sysmlv2/`, cut along
the package hierarchy of the normative KerML/SysML XMI (`KerML/Root/Elements.ttl`,
`KerML/Kernel/Expressions.ttl`, `SysML/Systems/Requirements.ttl`, …) with a `catalog.tsv` from
term to declaring module and `owl:imports` computed from use. The generator
(`cmd/ontology-modules`, `internal/core/rdf/ontology/modules`) errors rather than dropping data —
every source triple lands in exactly one module and the union is graph-isomorphic to upstream —
and `make ontology-modules-check` in CI keeps the committed output equal to the pinned sources
(`scripts/download-ontology-sources.sh`). It is additive to the term table and the Flexo export;
once merged, the profile can import the module a metaclass lives in rather than the monolith, and
a consumer wanting only, say, the requirements vocabulary has a file to import.

**Not started:** the profile plumbing itself — about one session once the table, the gate and
the modules exist. Conformance beyond that is gated on D1 and D2 rather than on this item: `sysx:`
has no place in the ontology, so an ontology-profile graph is conformant only as far as those
have landed, and the profile's documentation should say so.

## D9 — Flexo as a place models live, not only a place graphs are tested

What exists at the baseline, all landed: `-sync-diff <repo.ttl | endpoint>` computes an
identity-keyed change set between the model and a repository branch, reading the branch through
the SysML v2 API when given an endpoint and never writing; `-sync-apply <endpoint>` writes that
change set as one commit or a batched series through `flexo-mms-sysmlv2`, records the commit in
the sync state, refuses a set it cannot apply whole and leaves what did land committed (#791,
`internal/interop/reposync`, `internal/interop/flexo`); and the harness above loads a whole
converted graph into a Layer 1 branch by `PUT` and measures what the service reads back. Both
directions are refereed against the live stack, and both are element-keyed, which is what identity
bought.

What is missing is the round trip a modeller expects from a repository, and each piece is small
now that D3.4 is in:

1. **Read a branch as notation.** `-sync-diff` reads a branch to compare it; nothing converts a
   branch to `.sysml`. `sysml -convert sysml -from <endpoint or flexo:// URL>` is the decoder D3.4
   completed, applied to the graph the service serves, and `-sync-state` already knows the branch
   and the last commit.
2. **Push a whole graph.** The harness's `PUT .../branches/{branch}/graph` with the ETag
   precondition and `?message=` is the fast path for a first load or a re-baseline, where the
   element-wise commit of `-sync-apply` is the wrong shape. Expose it as the write half of the
   same flag, with the token from `flexo.EnvToken` as today.
3. **What survives the hop.** D2 and D1 decide how much of a pushed model the read path gets
   back; the harness's 369 of 452 is the number to move, and it is re-measured, not asserted,
   after each.

Nothing here is a new subsystem; the order is D9.1 → D9.2, and D9.3 is the RDF track's existing
order applied to this use.

## D10 — write-through from a view-only project to the projects it shows

A project under configuration management that consists entirely of views — `view` usages with
`expose` and `filter` over one or more imported projects, each bound by its own
`@ProjectRef` — owns no element worth editing: everything it shows is some other project's, and
with `@ElementId` on the materialized elements that ownership is explicit
([element-identity-annotations.md](element-identity-annotations.md), nested scopes). Today the
sync tooling stops at the document: `reposync.Diff` produces one `ChangeSet` for one project
scope, `GraphScope` refuses a graph carrying two, and an edit made to an element as it appears in
the view project has nowhere to go but the view project's own branch, which is the wrong owner.
The rendered artefacts (`-render`, `-doc`) stay one-way; the round trip in question is
notation-to-repository by id, and it needs three things:

1. **Per-scope fan-out.** A diff over a multi-scope document splits the local graph by enclosing
   `@ProjectRef`, diffs each part against its own branch (`Options.Base` per scope), and applies
   each as a commit to the owning project under its own `flexo.StaleBranchError` guard. No
   partial success across scopes: a set that is not appliable in one scope refuses the run before
   the first write, as `Appliable` does today within one. `-sync-state` records one last-seen
   commit per scope.
2. **Refusing the edit that would leave the view.** If a write-back would take an element outside
   the `filter` of every view that exposed it — dropping the `#systemRequirement` tag on a
   requirement shown only by `RequirementsView` — the element vanishes from the project the edit
   was made in. That is the classical view-update problem, and the answer is a typed refusal in
   the change set (the same tier as the existing *conflict* verdict), overridable only by an
   explicit confirmation like `ConfirmDeletes`. The resolver's inherited view conditions
   (`Resolver.inheritedViewConditions`) already compute the exposing filters, so the check is a
   re-admission test of the edited element against them.
3. **Variant-tagged edits under conditional configuration.** A view project whose exposure
   depends on a variant selection shows one variant's element at a time; an id says *which*
   element, not *under which selection it was shown*, so a write-back must carry the selection
   that was active and refuse when the target branch's selection differs. Elements with derived
   (name-keyed) identity are excluded from write-through altogether: a rename through a view
   would be a delete plus a create in the owner, which is the failure identity exists to prevent,
   so the path requires `@ElementId` and offers `-sync-mint-ids -sync-annotate` to get there.

The gate is the live-stack harness (`TestFlexoInterop`) with a two-project fixture: a view-only
project over two owned projects, an edit through the view lands as one commit in each owner and
nothing in the view project, the filter-violating edit and the variant mismatch are refused before
any write, and a re-run finds nothing to change. Depends on D9.1 (reading a branch as notation,
which is how the view project materializes what it exposes) and sits after D9.2 in the track
order; independent of D1/D2, since it moves whole elements by id and never inspects their
vocabulary.

---

# Track F — the executor defects the conformance gate carries

`internal/core/runtime/testdata/conformance/known_failures.txt` names three cases that
`TestExecutionConformance` skips rather than runs, each a fixture whose expectation is derived
from the Kernel Semantic Library in [behavior-semantic-oracle.md](behavior-semantic-oracle.md)
("What the executor gets wrong") and not met by `internal/core/runtime/action_executor.go`. They
are the whole of the gate's known-failure list at this baseline, and each is a defect in one
executor step function, not a missing feature. **Not started** as code; the derivations and the
fixtures are in place, so each item is the fix, the skip line removed and the row in
`spec-compliance.md` moved from *known failure* to *faithful*.

## F1 — a node reached over two successions is performed once, after both

Fixture `action_node_with_two_incoming_successions_runs_once`. The library's `HappensBefore`
over both successions fixes one performance of the target after both sources; the executor
performs it once per arriving token (`stepActionExecutionNode` has no synchronization — only
`join` does), so `hits` reads 2 where the derivation fixes 1.

## F2 — a join counts one token per incoming succession

Fixture `action_join_one_token_per_incoming_succession`. A join waits for a token from *each*
incoming succession; `stepJoinNode` fires when as many tokens are parked as there are incoming
successions, whatever edges they arrived over, because a `Token` records no incoming edge. The
fix is the edge on the token and the count per edge.

## F3 — a merge is re-entered on every traversal of a loop

Fixture `action_merge_loop_reenters`. Every arrival traverses a merge; `stepMergeNode` keys
`mergeVisited` on the merge node and the activation of the flow it belongs to, so within one
activation the first traversal closes the merge and a loop's re-entry is dropped. The fix admits
one traversal per arriving token, not one per activation.

---

# Track S — multiple valid executions

The library states a **partial order** between performances and fixes some outcomes; it does not
order two steps no chain of `HappensBefore` connects, and it gives no conflict rule when two such
steps write one feature ([behavior-semantic-oracle.md](behavior-semantic-oracle.md), "What the
library fixes, and what a trace adds"). The executor picks one linearization — tokens are stepped
in descending index order within a step, a fork's branch declared last is stepped first — and
every fixture pins that one: `action_fork_branches_write_one_feature` records `x = 1` as the
executor's scheduling, not as a derived value, and its compliance row is *approximate* for exactly
that reason. So a conformance case cannot today say "1 or 2", a trace cannot say where the
executor chose, and nothing checks that the linearizations it did *not* take would also have met
the derivation. This track makes the choice explicit and checkable. **Not started.**

It is a **prerequisite to Track E**: E1–E7 edit the same executor loops (`stepActionExecutionNode`,
`stepJoinNode`, `stepMergeNode`, the fork and the token list) that S1–S4 instrument and
parameterize, and a termination, interrupt or streaming semantics added on top of an implicit
scheduling order would have to be re-derived once that order becomes a policy. Track F's three
fixes are the same loops again and come first, because a policy over a join that miscounts is a
policy over a defect.

## S1 — admissible outcomes in the conformance schema

The `.expected.json` schema (`internal/core/runtime/testdata/conformance/README.md`) pins one
value per feature. Add a form that states the *set* the derivation admits — for
`action_fork_branches_write_one_feature`, `x ∈ {1, 2}` with `leftRan` and `rightRan` fixed `true`
— so a case distinguishes what the library fixes from what the executor happened to choose. A case
with no such form keeps its meaning; the semantic-oracle fixtures whose derivation says *open* are
rewritten to it, and their compliance rows stop being approximate for the scheduling reason alone.

## S2 — choice points in the execution trace

A `.trace.golden` records one linearization and, in prose in the oracle record, which of its lines
are tool-defined. Make the trace say so itself: at each step where more than one token was
runnable, record the candidates and the one taken, so a golden reviewed against a derivation shows
where the executor chose and a reader can tell a fixed order from a picked one. The golden format
change is one `-update-traces` run whose diff adds lines and changes none.

## S3 — a selectable scheduling policy, the default unchanged

Name the current order (descending token index, last-declared branch first) as the default policy
and add at least one other — declaration order, or a seeded pseudo-random order — selectable on
`sysml`, in the REPL and over the API, so a model's dependence on scheduling can be shown by running
it twice. Every existing golden and trace stays byte-identical under the default; a policy is a
parameter of the executor, not a fork of it. A5 (one clock for actions and states) waits on this,
since a clock is an ordering rule too.

## S4 — bounded exhaustive exploration against the semantic oracle

For a case whose derivation admits several outcomes, run *every* linearization up to a stated
bound on runnable-token choices and check each final state against S1's admissible set and each
trace against the derivation's fixed orderings. This is the proof that the executor's one
scheduling is not hiding a wrong one; it runs as a gate over the oracle corpus, refusing a case
whose choice tree exceeds the bound rather than sampling it.

---

# Track E — behavior execution

The runtime executes actions, state machines, calculations and constraints against the lowered
IR (`internal/core/lower` `ActionGraph`/`StateGraph`, `internal/core/runtime`), and
`docs/project/spec-compliance.md` § "What We Don't (Yet) Support" lists what it does not
execute: interruptible regions, expansion regions, streaming pins, protocol state machines,
operation invocation with positional arguments, and routing a send to a second object of one
usage; a `terminate` inside a body is refused by the runtime with a typed error and appears in no
list. The behavior-execution review after `v0.4.3` found nothing missing beyond those, and this
track records each as work with a stated scope, dependency order and acceptance gate rather than
as a bullet or an error message alone. **Deferred to the release after the current one.** No
conformance fixture or trace golden exercises any of them, each has a typed refusal or a
documented limitation in place of a wrong result, and every item edits the executor loops that
Track F is fixing and Track S is instrumenting and parameterizing in the current release; an E
item written against the implicit scheduling order would be re-derived once that order is a
policy, so E waits for S3. Each item below therefore ends with what would move it forward; until
that happens, the honest status is the "not supported" bullet or the refusal.

Two things about the list's own terms. First, four of the seven items — interruptible regions,
expansion regions, streaming pins, protocol state machines — are UML 2.5.1 concepts that SysML v2
(`formal/2026-03-02`) does not carry as notation: its actions (§7.17) spell termination,
acceptance, loops and flows directly and its states (§7.18) have no protocol variant. Each of
those items therefore starts by naming the SysML v2 spelling it corresponds to, and the target is
that spelling's semantics in the Kernel Semantic Library and the Systems Library, never the UML
feature by name. Second, the proof for every item is the four-layer contract of `AGENTS.md`
§5.2 — a conformance fixture (`.sysml` + `.expected.json` under
`internal/core/runtime/testdata/conformance/`), a trace golden where ordering matters
(`TestExecutionTrace`, `-update-traces`), a robustness case in `robustness_test.go` for the
failure mode, and the row in `spec-compliance.md` moving out of the "not supported" list — and
every item is self-assessed, since the pinned pilot evaluates expressions and executes no action
or state machine ([pilot-execution-referee.md](pilot-execution-referee.md)).

**Landed ahead of E1–E7**, all merged: the one structural finding of the review
— nested action nodes shared the enclosing action's flat feature space, so `p.v` and `q.v`
collided and same-named outputs overwrote each other — is
[PR #823](https://github.com/JPL-Devin/OpenSysML/pull/823), which gives each performance its own
frame with the callee's pins, precedence-ordered binding (flow payload, then `bind`, then the
declared value) and block-flow nodes inside `if`/loop bodies as real nodes. Message delivery
through a binding connector between an assembly's boundary port and a part's port
([PR #839](https://github.com/JPL-Devin/OpenSysML/pull/839)) fixes `accept … via i` never firing
on the inner part. On the debugging surfaces, [PR #843](https://github.com/JPL-Devin/OpenSysML/pull/843)
makes every expression surface read the instantiated object after `-instantiate` (the stale `0`
beside `%features`' `1`, and `%eval in #1` being an unresolved reference),
[PR #805](https://github.com/JPL-Devin/OpenSysML/pull/805) reads a valueless feature as `<unset>`
in `%eval in` and lists behaviors, [PR #808](https://github.com/JPL-Devin/OpenSysML/pull/808) adds
`%send` and [PR #809](https://github.com/JPL-Devin/OpenSysML/pull/809) one object-reference grammar
for every REPL command, reconciled against #810. What the review found *after* #810 was that
`%state <machine>` in its one-argument form attached a fresh, detached performance rather than the
one materialized object that exhibits the machine (after `%instantiate TA::Sys; %state lp; %advance
2.5 [s]`, object `#1` still read `n = 1` where `%state #1` gave `n = 3`). The cause was a
`StateExecutor` built with no `self`, so `entry`/`do` writes landed in the executor's own frame;
[PR #845](https://github.com/JPL-Devin/OpenSysML/pull/845) makes the machine form walk the held
objects and attach to the single exhibitor's running machine, refuses with a typed error naming the
objects (or, before any object exists, the types) when there are zero or several, and leaves a
truly unbound `state def` running detached; re-run at this baseline the same probe reads `n = 4`
on object `#1` after `%advance 2.5 [s]` (entry, then the timed re-entries at 1 and 2). The double
initial-`do` the review first reported is not reproducible on `main` since #810 and is retired.
Since then: `%state <machine>` on a state machine with a typed body attaches the same way (#856);
the state executor refuses the trigger arguments validation refuses (`accept after 5` without a
unit, a conditional `when`) instead of running them (#833, #871); `send` arguments are validated
and `send new Def(args)` constructs the message it sends (#838, #875); and a message through a
binding connector at a boundary port routes in both directions (#839).

## E1 — `terminate` in a body

**Today.** The parser accepts a terminate action usage in every position the grammar allows
(`ast.TerminateStatement`, with the terminated occurrence as `Target` or nil for the containing
action), and lowering carries it losslessly: as a node of the flow (`then terminate;`,
`lower/action_nodes.go`), as a statement of a control node's or a state's `entry`/`do`/`exit`
body (`lower/action_graph.go` `lowerStatement` → `Effect{Kind: EffectTerminate}`;
`lower/state_graph.go` through `BodyStatementMembers`). The runtime then refuses to execute it,
with one message wherever it is reached: `runtime/action_statements.go`
`(*actionStmtHost).effect` and `runtime/state_statements.go` `(*stateStmtHost).effect` return
`<where>: 'terminate' in a body is not executable`; a calculation refuses it as
`ErrCalcSideEffect` (`runtime/calc_statements.go`), and that refusal is correct and stays — a
calculation is pure (`robustness_test.go:calc_terminate_is_rejected`). So an action whose flow
reaches `then terminate;` fails with that error rather than ending. The training corpus's
`19. Terminate Actions/Terminate Actions Example-1.sysml` (`MonitoredActivity`) uses it twice
and does not run at this baseline: it stops earlier, at the nested action node whose members are
`perform`s with no `first` (`no initial node found in action node performCriticalActivity`,
re-measured after #823's nested frames landed — the refusal is the flow rule, not the frame), and
would stop at the first `terminate` once that is in.

Every position is refused. The previous baseline recorded one that was not — a nested action node
written as a statement body (`action a { assign n := 1; terminate; }`) dropped the `terminate`
because `lowerBody`'s statement cases did not include it. That is closed: `BodyStatementMembers`
and `lowerStatement` (`lower/action_graph.go`) take `*ast.TerminateStatement`, the action
executor's `stepStatementNode` recognises it, and the statement reaches the same `'terminate' in a
body is not executable` refusal as every other body. There is no silent no-op left in this item;
what remains is the semantics below.

**Target.** SysML v2 §7.17.10: a terminate action usage "forces the lifetime of the terminated
occurrence to end by the completion of the `TerminateAction`"; when no occurrence is given, the
default is "the immediately containing action of the terminate action usage", and for a nested
action "it is that nested action that is terminated, not any containing actions"; `terminate
this;` in a part's action ends the part; the occurrence may also arrive by a `flow` into the
`terminatedOccurrence` parameter. The library is `Actions::TerminateAction` (`Systems
Library/Actions.sysml`: `in occurrence terminatedOccurrence[1]`, performed by
`terminateOccurrence : destroy`) and the base usage `Actions::terminateActions`, whose
`terminatedOccurrence` defaults to `that as Occurrence`. In a state body the containing action is
the entry, do or exit behavior it is written in — a `StatePerformance`'s `entry`/`do`/`exit` step
(`Kernel Semantic Library/StatePerformances.kerml`) — so it ends that behavior, not the state
machine, unless the machine's exhibiting occurrence is named.

**Work.** Give `lower.Effect` the terminated occurrence as an evaluable target (nil for the
containing performance), and give both executors a notion of *ending a performance that is still
ongoing*: for the action executor, dropping every token the terminated node or action still holds
— including a forked sibling branch still running, as `MonitoredActivity` requires — and
completing it with the outputs it has; for the state executor, ending the running entry/do/exit
behavior at that statement (`runtime/state_executor.go` `startDoAction`/`runDoRound` hold the
running do behaviors) and, for a named occurrence, the object that holds it. A `terminate` naming
an occurrence that is not ongoing, or a value that is no occurrence, is a typed error. Nothing
else in the track depends on anything but this item, and E2 depends on it.

**Proof.** Conformance: the training example above runs to completion, `performCriticalActivity`
ended by its own `terminate` while `monitorCriticalActivity` is still ongoing and `stop` ending
the whole action; `then terminate;` in a flow ends the action with the outputs assigned so far;
`terminate` in a `do` body ends that behavior and the state stays active; a nested action's
`terminate` ends the nested action and its parent continues. A trace golden for
the fork case (which tokens are dropped, in what order). Robustness: terminate of a completed
occurrence, of a non-occurrence, and the calculation refusal unchanged. `spec-compliance.md`: the
Actions map gains a `terminate` row per position (today it has none — the refusal is the only
record), and the pointer to this track under "not supported" drops the `terminate` clause.
**Prioritize when** a corpus model or a user model needs `terminate` to run rather than to be
refused — the training example is the first candidate, once the nested-action frames it also
needs are in.

## E2 — interrupting an ongoing performance ("interruptible regions")

**Today.** SysML v2 has no interruptible-region notation; what UML models with one is spelled in
SysML v2 as an `accept` followed by a `terminate` in a forked branch (§7.17.10's
`MonitoredActivity`: "Terminates `performCriticalActivity` even if `monitorCriticalActivity` is
still ongoing"), or as a transition leaving a state whose `do` is running (§7.18.3: "If the
source state has a do action that is still being performed, that is interrupted."). The action
half is E1 and does not exist yet. The state half runs, with one documented approximation
(`spec-compliance.md` § Known Limitations, *Runtime*): an inline `do` body is one action, so
`runDoRound` advances it as a unit and an outgoing transition interrupts it only between rounds,
never between its statements; the one-action-per-statement `do { … }` form is the interruptible
spelling. There is no refusal here — the approximation is a documented ordering, not an error.

**Target.** §7.18.3's transition semantics, step 1: the source state's do action, "if it is
still being performed, is interrupted" when the transition is triggered — `StatePerformance::do`
is a `step` of the state's performance, and the `StateTransitionPerformance` that leaves it is
keyed on its `accept` step and `transitionLink` (`StatePerformances.kerml`,
`TransitionPerformances.kerml`) — so the do behavior's remaining statements do not run once the
trigger is accepted, whether the body was written as one action or several. For actions, the
target is E1's: the terminate ends the performance whatever else it has in flight.

**Work.** After E1: make an inline `do` body resumable between statements so a round can leave it
mid-body (the statement hosts already run a body statement at a time through `stmtEnv` frames;
what is missing is a do behavior that yields after each statement rather than after the whole
inline body), and drop the pending statements when the state exits. Depends on E1 for the shared
notion of ending an ongoing performance; nothing depends on it.

**Proof.** Conformance and a trace golden for a `do` body of three statements interrupted by a
signal after the first; the existing do-interruption fixtures unchanged; the Known Limitations
bullet removed and the `entry`/`do`/`exit` row in the State Machine map re-stated.
**Prioritize when** a model's result depends on a `do` body being interrupted between two of its
statements — until then the documented spelling (`do { … }` as one action per statement) gives
the same result.

## E3 — concurrent per-element performance ("expansion regions")

**Today.** SysML v2 has no expansion-region notation. Its iterative half is the `for` loop
(§7.17.12, `Actions::ForLoopAction`), which the runtime executes in every body position
(`runtime/action_statements.go`, `runtime/statements.go` `forLoop`), one iteration after the
other, with the step budget bounding it. Its parallel half — the body performed once per element
of a collection, all performances ongoing at once — has no spelling the runtime refuses, because
no spelling for it has been established here: a `for` is sequential by definition
(`ForLoopAction` walks `seq` by an `index`), and a `fork` duplicates control, not a collection.

**Target.** To be settled before any executor work, in a design record under `docs/project/`
like the others: whether SysML v2 §7.17.2's multiplicity on a performed action usage, with a
`flow` delivering the collection to its input, is the standard spelling of concurrent per-element
performance, and what `Performances.kerml` then says about the ordering of those performances.
If the reading is that no such spelling exists, the item closes as "the iterative form is `for`;
the parallel form is not SysML v2" and the bullet is re-worded to say so.

**Work.** The record first; then, if there is a spelling, one performance per element with its
own token and pins, joined when all complete, on top of the concurrency the fork/join executor
already has — and the interaction with E4 (a streaming consumer of the elements) decided with it.
No other item depends on this one.

**Proof.** Conformance for a collection of three performed concurrently with per-element outputs
collected; a trace golden for the interleaving; robustness for an empty collection and a body
that fails on one element. **Prioritize when** someone needs it: a model with a per-element
behavior whose sequential `for` result is wrong or too slow.

## E4 — streaming flows ("streaming pins")

**Today.** A `flow` between two action parameters executes as a *succession* flow: the value
at the source pin is moved to the target pin when the source node completes
(`runtime/action_executor.go` `applyDataFlows`, called from the node-completion paths), so the
target reads it when its own token arrives. SysML v2 §7.16 draws the distinction the runtime does
not: "the input and output parameters are streaming unless designated as succession flows" — a
streaming `flow` "can be ongoing while both the source and target action are being performed",
while a `succession flow` "cannot begin until the source completes". The parser keeps the
distinction (`ast.Usage.IsSuccessionFlow`, used by the control-node succession rule), but
`lower.ObjectFlow` carries no kind and no position refuses anything — both spellings run, both as
the succession reading. That is a wrong
result only for a model whose target reads before its source completes, and no conformance
fixture writes one.

**Target.** `Flows::Flow :> Message, FlowTransfer` for a streaming flow and
`Flows::SuccessionFlow :> Flow, FlowTransferBefore` for the succession form (`Systems
Library/Flows.sysml`, `Kernel Semantic Library/Transfers.kerml`): a streaming flow transfers each
value the source parameter takes while both performances are ongoing; a succession flow transfers
after the source completes. What runs today is the second, applied to both.

**Work.** Carry the kind from the AST (`ast.Usage.IsSuccessionFlow`) through
`lower.ObjectFlow` to the executor; keep the succession behavior for the `succession flow`
spelling; for a plain `flow`, deliver on each write to the source parameter while the target is
ongoing, which needs a node to be readable while it still holds a token — the same notion of an
ongoing performance E1 and E2 introduce. Depends on E1 for that notion; E3's parallel form would
feed it.

**Proof.** Conformance: a producer loop writing three values to an `out` streamed to a consumer
that accumulates them, with the `succession flow` variant of the same model receiving only the
last; a trace golden for the interleaving; robustness for a stream whose source never writes.
`spec-compliance.md`: the Actions map's object-flow rows split by kind and the bullet leaves the
list. **Prioritize when** a model's result differs between the two readings — a consumer that
reads before its producer completes.

## E5 — protocol state machines

**Today.** No SysML v2 notation exists for a protocol state machine (UML 2.5.1 §14.4), so nothing
is parsed, lowered or refused; the bullet in `spec-compliance.md` is the whole record. What SysML
v2 does have is a state machine exhibited by an occurrence (§7.18.4 `exhibit`), which the runtime
runs during materialization of an object of the exhibiting type (the Classifier Behaviors map).

**Target.** None is stated, and none should be invented here: a UML protocol state machine
constrains the order of operation calls on an interface, and the SysML v2 rendering of that
constraint is a design question — an exhibited state machine on a port definition, with an
out-of-order message refused as a typed error, is the obvious candidate — to be settled in a design
record if the need arises.

**Work.** The record; then whatever it concludes. Independent of every other item.

**Proof.** Set by the record. **Prioritize when** a user brings a model that needs the order of
messages on a port checked at run time; until then the bullet stays as it is.

## E6 — operation invocation with positional arguments

**Today.** `Context.InvokeOperation(inst, name, args map[string]Value)`
(`runtime/invoke_operation.go`) runs a member of an object's type with the object as performer,
whichever behavior the member is — an action through `ExecuteActionPerformedBy`, a calc through
the calc invocation with the object as its featuring object, a constraint through condition
evaluation — and `TestInvokeOperationPerformedByTheObject` (`runtime/classifier_behavior_test.go`)
covers all three. The compliance bullet used to name an operation "given as a `calc` or
`constraint`" beside the positional form; that half is closed by the Classifier Behaviors row that
says so, the bullet is re-worded to the positional form with this record, and this item is that
form only. Arguments bind by name and only by name: `operationInputs` takes a map, binds each
`in`/`inout` parameter by its name and refuses a missing one (`ErrUnboundParameter: parameter …
has no argument and no default`) and an unknown one (`ErrUnboundParameter: … is no input
parameter of operation …`). The REPL's `%invoke <object> <op> [<p>=<expr>]` (`repl/meta.go`
`operationArguments`) refuses an argument not written `<parameter>=<expression>`. There is no
positional form on either surface, and no refusal specific to one — the row says so: "no
invocation surface expresses positional operation arguments". An operation call *written in a
model* is an `InvocationExpression` and is bound by the expression machinery, where positional
arguments already work (`runtime/invoke_calc.go` `bindCalcParameter`; for a performed action,
`runtime/invoke_action.go` `bindArguments` binds `inv.args` in parameter order and refuses a
surplus with `action … takes N input parameter(s), got M argument(s)`).

**Target.** KerML 1.0 §8.2.5.8.3 gives an invocation's `ArgumentList` as either a
`PositionalArgumentList` or a `NamedArgumentList`, never a mix, and §8.4.4.9.5 binds a positional
list to the behavior's parameters in declaration order (`feature a redefines F::a = e1; feature b
redefines F::b = e2; …`) — as `bindArguments` and `bindCalcParameter` already do for a call in the
model. The API and the REPL should offer the same two forms, so `%invoke rover1 drive 10 20` binds
`10` and `20` to the first two `in` parameters, a list mixing the two forms is refused, and a
surplus is refused with the same arity error.

**Work.** Small and self-contained: an ordered argument list on `InvokeOperation` (or a second
entry point) sharing `bindArguments`'s rules, and `%invoke` accepting a list of bare expressions
in place of its `<p>=<expr>` pairs. Depends on nothing; nothing depends on it.

**Proof.** `runtime/classifier_behavior_test.go` and `repl/classifier_behavior_test.go` gain the
positional, mixed and surplus cases; `robustness_test.go` the arity failure; the bullet leaves the
list. **Prioritize when** a REPL or API user asks for it — it is the smallest item in the track
and the one most likely to be done on demand.

## E7 — an addressed send to a second object of one usage

**Today.** A `send … to <target>` resolves its target through `runtime/signal.go`
`resolveAddresses` → `featureAddresses` → `addressOwner`: a name that is a feature of the sending
object (or of an object holding it) reaches that feature's value, and otherwise the shortest prefix
naming an occurrence usage that `occursOnce` reaches *the* occurrence this context holds for it
(`ctx.occurrenceOf(sym)`, `runtime/instance.go`). A `via` send follows the connections
(`runtime/routing.go`, `signal.go` `postVia`) to the object at the other end. In both forms the
object reached is the one this context materialized as the usage's occurrence; a second object
instantiated of that same usage is a different object, which a send addressed to the usage does
not reach. There is no refusal: the send reaches the held occurrence, and
`spec-compliance.md` § Known Limitations (*Standard behavioral notation*) documents it.

**Target.** §7.17.7: a `SendAction` has three input parameters — the payload, a *sender*
occurrence (`via`) and a *receiver* occurrence (`to`) — and "the behavior of a `SendAction` is to
transfer the payload from the sender to the receiver"; the receiver is a value the `to` expression
(or a flow or binding into the `receiver` parameter) supplies, and the message is a
`MessageTransfer` between those two occurrences (`Transfers.kerml`). So a send whose receiver
expression yields a particular object reaches that object, whichever usage it was instantiated
from.

**Work.** Address by value rather than by usage: evaluate the `to` expression to an object
reference and post to that object's identity (`objectID`), with the by-usage resolution kept for a
target that is a name. That is only meaningful once a context can hold more than one object of one
usage, which is the "dynamic object creation/destruction" bullet beside this one in the compliance
list and outside this track: an object materialized by `new` or held in a feature the sender
reads. Depends on that object-model item; independent of E1–E6.

**Proof.** Conformance: two objects of one usage, a send addressed to the second, only the second's
accept fires (with the `via` form of the same model); a trace golden; robustness for a target
expression yielding no object. `spec-compliance.md`: the Known Limitations bullet and the "not
supported" bullet both leave. **Prioritize when** the object-model item lands, since without it
there is no second object to address.

---

# Track X — expression forms the evaluator does not reach

Expression evaluation is the most externally refereed part of the runtime: when
`cmd/pilot-exec-diff` was re-run for the review (2026-09-02, at `1f136d27` — a snapshot of that
round, not the current baseline), 55 cases agreed with the pinned pilot, 1 agreed in kind only, 1
disagreement and 2 errors of ours were adjudicated, and 35 cases the pilot cannot evaluate; 70 of
the 71 grammar forms in `KerMLExpressions.xtext` have a corpus witness (only `%` lacks one).
Scalars, Booleans, strings, enumerations, quantities, `Complex` (#777, #788), arrays, vectors and
vector quantities (#883), rationals (#876), occurrence lifetimes (#884), collection bodies and the
named reducers (`->reduce '+'`, `builtinControlReduce`), indexing, qualified names, the lazy
conditionals, `??`, the Kernel Function Library with overloads by argument type, the step and
element budgets, RDF expression trees and the native scalar/collection fast path are done. The
review after `v0.4.3` found two defects, both landed (#794: `x @ T` on a value is `istype`, with
`@@` still the metadata classification; #795: a feature named `chain` resolves as a name where the
lookahead does not establish the `chain` modifier — `step chain …` still does not parse and is a
separate parser item). Since then the 0.6.0 release closed the first two items of this track:
constructors evaluate in a value position (X1, #981) and a singleton sequence takes part in
arithmetic as the scalar (X2). What follows is measured against `runtime/eval.go` and `bin/sysml`
at this baseline: X1 and X2 are stated as landed with what they leave, and X3, X4, X6, X7 and X8
still refuse with a typed error or answer the wrong type; each open item is the evaluation, and
each one ends with what depends on it.

## X1 — constructors: `new Pt(1, 2)` as a value (landed)

`ast.ConstructorExpr` parses with its positional and named arguments (#838, #875), types as the
definition it names with its arguments checked against that type, and since #981 evaluates in a
value position: `runtime/eval.go` dispatches it to `runtime/signal.go` `evalConstructor`, which
creates an object of the definition in the evaluating context with the arguments bound to its
features — positionally in declaration order (`new Pt(1.0, 2.0)`) or by name (`new Pt(y = 2.0,
x = 1.0)`), both reading back `p.x = 1.0`, `p.y = 2.0` — the same path `send new Def(args)` had
used for its payload. What depended on it moved: `new SampledFunction(samples = (new
SamplePair(…), …))` constructs and `Domain` reads it (L7), and an analysis case builds its own
records (A1). What remains is on the compiled side: the interpreted/native differential cannot
cover a constructor until N2.1 gives the compiled side a record type.

## X2 — a singleton sequence is the scalar (landed for a feature's own value)

KerML has no scalar/sequence distinction: a one-element sequence *is* the value, and `[0..*]`
features with one value take part in arithmetic. Both directions hold at this baseline for a
feature read directly: `attribute xs : Real[0..*] = (2.5);` gives `xs + 1.0 = 3.5`, and the
mirror, `attribute ys : Real[0..*] = one;` with `one : Real`, reads `1.0` where a sequence is
expected. The multiplicity checks are not weakened: `attribute two : Real[2..*] = (1.0);` is still
refused by the checker as `1 value(s) bound to a feature with multiplicity lower bound 2` before
anything evaluates. What is *not* coerced is a singleton read through a feature chain on an
object: `sp.domainValue` on a constructed `SamplePair` (a feature redefined without its own
multiplicity, so it inherits `KeyValuePair::key`'s) still arrives as the sequence `[0.0]` and
`1.0 - sp.domainValue` refuses `operator '-' is not defined for a Real and a sequence`. That
remaining half is what keeps `interpolateLinear` from running (L7) and is the one piece of X2 still
open; it is a read-side coercion in the chain evaluation, not a change to what a legal value is.

## X3 — casts: `r as Integer`

Parses (`ast.CastExpr`); at this baseline `r as Integer` on a `Real` is refused at evaluation as
`unsupported operator: 'as': a cast needs the runtime type of a value, which values do not carry
yet`. Semantics are the metamodel's: the result is the value if it conforms to the type,
otherwise the empty sequence — not a conversion (`ToInteger` is L4). The refusal names the
representation gap: a scalar value carries its kind, not the declared type it was read from, so
conformance to `Integer` against `Real` cannot be decided from the value alone. Small once a
value carries its type; it is listed because analysis models from the training corpus use it.

## X4 — `*` as a value, and `.metadata`

`*` in a value position (unbounded, the Infinity of `ScalarValues`) has no runtime value, so
`multiplicity [0..*]` reads fine but `x = *` does not evaluate (`unsupported node type:
*ast.LiteralInfinity` at this baseline); and `elem.metadata` — the metadata
access path the metadata library defines — parses but is not evaluated, while `@@` classification
is. Both are representation items: an infinity kind (or a flag on the numeric kinds) and a metadata
value the evaluator can hand back from the side tables the checker already keeps.

## X5 — `all` (named reducers done)

`all T` (every instance of a type) is the expression form of Q2 and is refused (`'all' needs the
extent of a type, which the runtime does not enumerate`) until there is a runtime population to
enumerate; the item is now Q2's alone. The named-reducer half is done: `->reduce '+'` and the other
named spellings resolve to the library function by name through L4's dispatch
(`runtime/collections.go` `builtinControlReduce`; conformance `calc_library_complex_sum_real_axis`
exercises the Real-axis case).

## X6 — function values

A calc-typed parameter (`in calc calculation { in x; }` in `SampledFunctions::Sample`, invoked in
its body as `calculation(x)`) or a feature typed by a calc cannot be given a value: there is no
function value kind. At this baseline, passing a `calc def Sq` where such a parameter is declared
— `Sample(Sq, (1.0, 2.0))`, or a model's own `Apply(Sq, 3.0)` with `in calc f { in x : Real;
return : Real; }` — is refused at evaluation as `cannot evaluate definition Sq`: the name resolves
to the definition and the evaluator has no value to make of it. Analysis libraries pass behaviors
as data (`SampledFunctions`, `TradeStudies::evaluationFunction`), so A2 and L7 need it. The
representation is a reference to a lowered calc plus the environment it closes over, invoked
through the same path as a calc usage; it is not an arbitrary closure over statements.

## X7 — tensors and set-producing expressions (vectors and arrays done)

Two of the three representation decisions are taken: since #883 a `Collections::Array` is a
`ValArray` with its dimensions and row-major elements, a `NumericalVectorValue` a `ValVector`, and
a `VectorQuantityValue` a `ValVectorQuantity` with a unit per axis, so `VectorValues` and the
measurement-reference libraries type against a value and `VectorFunctions` computes over it rather
than over a stand-in sequence. What is left: `TensorValues` (rank above two, and the shape check for
`m[i][j]`/`m[i, j]` indexing beyond what the array's dimensions give); a set kind, since a
set-producing expression (`->distinct`, the set operators) still yields a sequence in which order
is significant where the library says it is not; the static element type of the new kinds through
collection bodies (X8); their RDF literal form; and their native layout (N2.1 refuses them today).
Still last in the track, but smaller than it was.

## X8 — static element types through collection bodies, and the two harnesses

The checker types a collection operation's result as its input's element type, not what the body
returns, so `xs->collect { in x; x.mass }` is typed as the part not the quantity, and a downstream
error names the wrong type (#903 made the typer terminate on an argument that leads back to its
own call — the Apollo 11 mass rollup — but did not change what it answers). Interpreted and compiled evaluation are held equal by one differential
over the compiled subset; the pilot differential (`cmd/pilot-exec-diff`) still needs its normalization
of numeric spellings and its adjudication file to say why each of the remaining disagreements is
ours or the pilot's; and the RDF expression trees have no round trip of their own — a tree is
written, read and rewritten only as part of a whole model. Three hygiene items that make every X
change measurable; do X8's harness half first when starting the track.

---

# Track Q — queries over the running model

Four query surfaces exist and are landed: the standard API `Query` over a project's elements
(`internal/grpc`, the OSLC query grammar with its diagnostics — #798, #812), the native document
query (`query def` with parameters, planned by `internal/core/queryplan` and run by
`internal/core/queryexec`, from the CLI, the REPL and gRPC), `Evaluate`/`-eval`/`%eval in`, and the
solver's `solve`. All four read the *model*: its elements, its declarations and the expressions
over them. **None reaches the runtime** — the objects `%instantiate` and `-instantiate` create,
their current state, or the trace `-trace` prints — and the documentation does not yet say which
surface answers which question. That is the whole track.

## Q1 — say which query is which

One page distinguishing the four: document queries over elements, the API `Query` over a project,
`Evaluate` over one expression in one scope, and `solve` over constraints; what each returns, what
each cannot see, and where runtime queries (Q2–Q4) will sit. Cheap, and it stops each later item
from re-explaining the boundary. Not started.

## Q2 — a runtime population: `all T`, and predicates over instances

`all Vehicle` (every object typed by `Vehicle` in the session), and the collection operations over
it — `all Vehicle->select { in v; v.mass > 1000 [kg] }` — are the expression form of a runtime
query. The runtime already keeps every object it materialized (`Context.instances`, with identity
across rebuilds — `keepIdentitiesOf`); what is missing is the enumeration as a value, its static
type (`T[0..*]`), and the REPL/CLI/gRPC binding so a document query parameter can be bound to it.
Depends on a stable object representation, which #810/#836/#843 have been settling; goes after them.

## Q3 — state and event queries

"Which state is `#1.lp` in?", "which objects are in `run`?", "what did `#1` accept between
`t = 1 [s]` and `t = 2.5 [s]`?" — the state executor and the trace have the answers
(`StateExecutor.getCurrentState`, the event log `-trace` prints) and no query reads them. Q3 is a runtime
query vocabulary over current state and over the trace as a time-ordered relation, with the same
filter forms as Q2, so the trace stops being something one reads by eye. Depends on Q2's population
and on the trace representation staying stable while A5 (the shared clock) changes it; sequence
Q3 after A5.

## Q4 — document-query parameter defaults evaluate (done)

A `query def` parameter with a default (`in limit : Natural = 10;`) was refused at execution with
`relies on a default not retained in the plan` — the plan recorded `HasDefault` and not the
expression. [PR #849](https://github.com/JPL-Devin/OpenSysML/pull/849) (**landed**) keeps the
compiled default in the immutable plan with its declaring query, evaluates it in the declaring scope
once per execution (not per row), honours inheritance and redefinition (`in redefines threshold
default "5"`), lets an explicit binding override it, refuses an unrepresentable default at planning
and a self-referencing one as a composition cycle; `ErrorDefaultUnavailable` is gone. Verified at
this baseline over `docrender/testdata/defaulted_queries.sysml` through `-run-query` and
`-render-document`. One adjacent rough edge, not a defaults defect: `-run-query "Q threshold=1"`
types the bare `1` as an integer and is refused against a `String` parameter — quote it
(`threshold="1"`).

---

# Track A — analysis and simulation execution

The Systems Library's analysis vocabulary loads and type-checks — `AnalysisCases`, `TradeStudies`,
`StateSpaceRepresentation`, `SampledFunctions`, `VerificationCases` — and the runtime executes
calcs, constraints, requirements, actions, state machines and, since 0.6.0, an *analysis case*
(A1, the keystone the review after `v0.4.3` found missing). Each simulation capability people
expect from "SysML v2 execution" hangs off that keystone, and the rest of the track is what still
does not run on top of it. Every item below is self-assessed (the pilot executes none of this) and
every open one is refused today with a typed error rather than answered wrongly.

## A1 — an analysis case runs (landed)

**Landed** in [PR #979](https://github.com/JPL-Devin/OpenSysML/pull/979), shipped in 0.6.0. An
analysis definition or usage runs as the calculation it is: the `subject` is an `in` parameter
(bound by the usage, by the object the run is asked on, or from the enclosing case; a run with no
subject is refused naming it), the body's `action`/`perform`/nested `analysis` steps are one
`lower.Block` over the action graph they state (`then`, `first`, forks, joins, decisions, merges;
declaration order where none is stated) run by the action executor, `out` and `return` are
evaluated in the case's frame with their units, and the `objective` and every `assert constraint`
are checked afterwards by the requirement engine as satisfied / not satisfied / undecided.
`-analysis Pkg::Case`/`%analysis` beside `-calc` and `-action`, the `RunAnalysis` RPC with
Connect, Go and Python clients, and reads of an analysis usage's outputs as features
(`An::shipCost.total`, `holder.inner.total`, `attribute :>> x = a.result;`) with memoization and
invalidation are in; the follow-ups in the same release bind an objective's requirement subject by
keyword alone, read a case's result by its qualified name, report a recursive step once, keep
later objectives in position and hold an actor bound without `:>>` to the actor it inherits.

What A1 deliberately leaves, each stated where it is owned: `-calc`/`%calc`/`EvaluateCalc` still
refuse an analysis *by kind* and say to run it as one — that is the contract, not a gap; a
verification case body shares the grammar and lowers through the same code but is **not run**, and
its verdict stays what `-requirement`/`-satisfy` compute (A6); the training corpus's `33.
Analysis` fuel-economy cases and the `Analysis Examples` corpus state their step outputs only
through `assert constraint` (`solveForPower.power` has no computation to run), so they end in the
typed `no value for feature` refusal — the solver's territory, not the executor's;
`10c-Fuel Economy Analysis.sysml` passes a bodiless `calc cityScenario` as the value of `in calc
scenario`, which X6 has to admit before its steps can run; and a trade study's iteration over its
alternatives is A2. What the corpora answer today: the pilot's `10d-Dynamics Analysis.sysml` runs
on a supplied subject and inputs (`accelerationProfile = [0.01, 0.01998…]`), and
`10a-Analysis.sysml` binds an untyped `part vehicle` to a `Vehicle` subject and states no mass
values, so it is refused at the binding (typed) and a copy with typed, valued parts answers
`200 [kg]`. Compiling a case (N2.1) was never part of this item.

## A2 — a trade study iterates, evaluates and selects

Two things exist on `main` and neither is this item. `%optimize <case>` (**experimental**,
`docs/reference/repl-commands.md`) asks the SMT solver — `z3` in particular, since
`(minimize …)`/`(maximize …)` is a z3 extension — for the best values an analysis definition or
usage admits: each `objective` typed `MinimizeObjective`/`MaximizeObjective` is improved over the
value its redefinition of the library's `eval` calculation returns, within the conditions the case
requires or assumes, several objectives lexicographically in declaration order, every optimum
verified before it is reported. It is a search over the *constraint* model for an assignment; it
does not run the case. And A1 accepts a `TradeStudy` *as an analysis case*, which it is, and runs
whatever steps its body states. What no surface does is the library's own trade-study semantics: at
this baseline a `TradeStudy` with `studyAlternatives = (e1, e2)`, a `MinimizeObjective` and an
`evaluationFunction` returning each alternative's `cost` loads and type-checks, and `-analysis`
ends in `analysis run failed: no value: output never assigned: output selectedAlternative` — nothing iterates
`studyAlternatives`, invokes `evaluationFunction` on each alternative and binds
`selectedAlternative` to the best under the objective. A2 is exactly that run, and it needs X6
(the function is a value the case can call per alternative) before it can be written; a study
whose alternatives are enumerated by a solver rather than a list stays `%optimize`'s.

## A3 — parameter sweeps, Monte Carlo and result tables

Running one analysis over a range of an input, or over N samples of a distribution, and collecting
the returns as a table is what every analysis pipeline asks for next; today it is a shell loop over
`-calc`. A3 is a `-sweep`/`-samples` surface over A1 with a deterministic seed, a result table with
one row per run (the inputs, the return, the wall time), and the same table from gRPC. It is
orchestration over A1, with no new semantics; a document query over the table (Track Q) follows.

## A4 — continuous time: a state-space runner

`StateSpaceRepresentation` declares the protocol (a state vector, its derivative, an output) and
nothing integrates it: there is no time-stepping runner, no integrator (the RK4 lunar-descent
conformance case is a calc that hand-rolls its stages), and no zero-crossing detection to hand an
event to a state machine. A4 is a fixed-step runner over a state-space definition with at least
Euler and RK4, an event when a guard expression crosses zero, and a trace of `(t, x)` — with the
runner sharing A5's clock so a state machine and a continuous model advance together.

## A5 — one clock for actions and states

`accept after 1 [s]` in a state machine advances against the executor's clock (`%advance`,
`-advance`); a time-triggered action in the ordinary action path has no clock to consult and is
refused (`a time-triggered accept with no clock`, robustness). A5 makes the clock a property of the
runtime context shared by both executors and by A4's runner, so `-advance` moves everything that is
waiting on time in one deterministic order (state machines first or actions first, stated and
tested with a trace golden). This is what "coherent action/state simulation" needs and is a
prerequisite for A4 and Q3; it precedes both in the approved order.

## A6 — verification cases give verdicts from their bodies

A `verification` case's verdict is computed today from `requirement` satisfaction
(`-requirement`, `-satisfy`, `%requirement`, `%satisfy`, the `VerifyRequirement` RPC) and the
solver's checks, not from running the case body, and the library's `PassIf` is recorded as
intentionally non-normative in `spec-compliance.md`. A1 lowers a verification case body through
the analysis path but does not run it; A6 runs it and binds the case's `verdict` to the
`VerificationCases` library's `PassIf`/`VerdictKind` semantics over that run, reported on the same
surfaces beside the satisfaction verdicts. Small now that A1 is in, and first in the track's order.

---

# Track V — validation against the pilot's named constraints

The denominator is the pilot's 217 named `validate*` constraints, read from the pinned jar by the
census [validation-constraints.md](validation-constraints.md) (#822) and re-audited row by row
against the code and the corpus (#900, whose gate now also checks that the function each row cites
exists and that each cited case belongs to its row). With #900 the census read **148 of 217
reported — 137 faithful, 11 approximate — 6 not implemented, 0 deliberate, 0 known failure and 63
unknown**, against 143 / 133 / 10 / 68 at the tag; the adjudication of the unknown KerML rows then
moved it to 156 reported, and the census at this baseline reads **162 of 217 reported — 156
faithful, 6 approximate — 1 not implemented, 1 deliberate, 0 known failure and 53 unknown**, each
remaining unknown row citing why the pilot never reports it.
The oracle at the other end agrees: of 285
self-authored invalid models, the pinned pilot and we both reject 276 (3 of them only in strict
mode, by design), the pilot alone rejects 0, and of the 9 only we reject eight are control-node
succession rules the pinned pilot leaves unimplemented and one is a non-Boolean guard on an action
body's succession that the pilot's check leaves silent once the library types the guard
([pilot-rejection.md](pilot-rejection.md)). Everything the previous baselines listed as open here
has landed:

- **The census and its gate** (#822, #900); **negative cases**, one minimal pilot-refereed invalid
  model per SysML constraint name the corpus owns or a stated reason none can exist (#831), and
  the KerML rejection cases (#817).
- **The rules the sample found missing**: overriding a bound (`=`) feature value (#826, which also
  carries the `=`/`default =` distinction through the AST); control-node succession counts and
  placement (#832); trigger arguments — `after` a duration, `at` an instant, `when` a Boolean —
  in validation and in the state executor (#833, #871); `send` payload, `via` and `to` typing and
  constructor arguments (#838, #875); `enum def F :> E` — an enumeration definition is a
  variation (#811) and an enumerated value is typed only by its enumeration (#907, with the cast
  and body-expression positions in #909).
- **Rows the census audit itself moved**: cross-subsetting (#872), duplicate parameter bindings
  (#875), owning-body membership (#874), initial values and `constant` on variable features (#873),
  metadata typing and `annotatedElement` conformance (#881, #901, #905, #906), association arity
  and the type of a multiplicity bound (#882), requirement-member prefix metadata and short names
  (#868, #880), a chained generalization under the cycle guard (#902).
- **Adjudicated as not a defect**: the inherited-name warning beside a typing error stays (#837);
  the pilot reports both.

**What remains is the census's own tail**, and it is the whole of the track: the 1 row marked *not
implemented* and the 53 marked *unknown* — a row is unknown when neither a pass nor a negative case
can be pointed at for the pilot's name, which is a gap in evidence before it is a gap in checking.
The order is the same as before: take the 1, then work the 53 down by writing the pilot-refereed
negative case first and implementing the rule only where the case shows we accept what the pilot
rejects; each change moves exactly its census row and `make docs-counts` regenerates the summary.
The 6 *approximate* rows (a check under our own wording that covers the pilot's constraint
without matching it) are honest as they stand and become faithful one at a time as their
wording is aligned.

---

# Track I — language integrations

`sysml-grpc` speaks gRPC, Connect and Connect-JSON over one port, with optional TLS, exact-origin
CORS, a health port and stdio; there are clients in Go (the public API), Python (`opensysml` on
PyPI), Node, Java and Rust, each with a worked tour. Verified at this baseline against the rebuilt
binary: plain HTTP/1.1 + JSON reaches every method (`ParseSources` takes `documents`, not
`sources`; `Evaluate` on `2 * 1500` answered `{"result":{"realValue":3000}}`), which means MATLAB,
R, Julia and C can integrate *today* with their HTTP and JSON libraries and no generated code. What
is missing is the contract that makes such a client correct rather than lucky, and a thin package
per language so nobody re-derives it.

## I1 — the wire contract, written down

There was no page a hand-written client can be built from: which field names (proto3
lowerCamelCase), which `Value` arm is which (`intValue` as a JSON *string* for 64-bit,
`realValue`, `boolValue`, `stringValue`, quantity with unit, enumeration by identity not display
name, instance references, `Complex`), how *unset* differs from *absent* and from *no result*, how a
diagnostic and a Connect error arrive, the model-hash lifetime, and how a behavior call and a query
are made. [wire-contract.md](../reference/wire-contract.md) is that page (#848, **landed**): every
example captured from the running service, all eleven `Value` arms, the Connect code table, the
`Instantiate`/`ExecuteState`/`Verify*`/`Query` answer shapes, and R, Julia, MATLAB and C
illustrations marked untested, linked from the transports, clients and API pages. Everything else
in the track reads from it.

## I2 — shared conformance fixtures for handwritten clients

One directory of request/response pairs — every `Value` arm, a diagnostic, a Connect error, a
behavior call, a document query — that the five existing clients and any new one replay, so "the
client decodes the contract" is a test and not a claim. Depends on I1; small.

## I3 — thin R, Julia and MATLAB packages

Each is a few hundred lines over the language's HTTP+JSON: connect, parse, evaluate, query, run a
behavior, decode `Value` by the I1 rules (64-bit integers, quantities, enumerations, unset), and
surface diagnostics as the language's errors. Each ships with the I2 fixtures as its tests and a
page on starting `sysml-grpc` (binary provisioning from the release, the flags, TLS, the health
check). Not started; each is about a session once I1 and I2 exist, and they can go in parallel.
Publishing (CRAN, the Julia registry, File Exchange) is account-gated and goes with R2.

## I4 — a C client, and the C ABI

C is two different things. A **C client** is I3 for C — an HTTP+JSON client over `libcurl` and a
JSON library, and the natural base for anything that embeds by FFI. A **C ABI** is in-process:
calling a compiled calc (N2.4) or, later, an embedded state machine (M4) through a stable header
with no service. The client depends only on I1/I2; the ABI depends on N2.4's design and is the same
artifact as M4's host interface, so design it once and let the three consumers (host tools, the C
client's optional in-process mode, the embedded target) restrict it. The client comes first.

---

# Track M — an embedded, RTOS-compatible target

The question was whether OpenSysML models could run on a microcontroller under an RTOS, and what
"embedded SysML v2" would mean. The review's answer, from the code: not by shrinking the interpreter
— `internal/core/runtime` depends on maps, allocation, `big.Rat`, and the parser and semantic
packages, and `lower.ActionGraph`/`lower.StateGraph` reference AST nodes, symbol scopes and
expression trees, so they are not closed artifacts that can leave the process (TinyGo is therefore
not a route). The route is Track N's discipline applied to behavior: a **closed, serializable
behavior IR**, an **AOT C backend** that emits static tables and no allocation, and a refusal —
typed, naming the construct — for any model the target cannot bound. What the runtime can promise
is *bounded and reproducible* execution; hard real-time guarantees (WCET) are properties of the
target, the compiler and the RTOS configuration, and the documentation must say so rather than
imply them.

## M1 — a closed behavior IR

The lowered graphs, made self-contained: nodes, successions, guards, triggers, effects, states,
transitions, regions, pins and connections with every reference resolved to an index and every
expression carried as N2's typed IR, with no pointer into the AST or the symbol tables — so it can
be serialized, diffed, and handed to a backend. Lowering to it must be lossless over the existing
conformance corpus (the interpreter can run *from* it, which is the proof), and it is the meeting
point with N2.6: the native track's action and state phases start here rather than duplicating it.
This item gates everything else in the track.

## M2 — a state and action C backend over static tables

From M1: state and transition tables, a static Petri-net-style scheduler for the action graph
(token counts per place, no dynamic node creation), fixed-size event queues sized from the model,
static port/connection routing, expression evaluation through N2's C emitter, and no `malloc`
after initialization. A model that cannot be bounded — unbounded multiplicity, recursion the
compiler cannot bound, `all T`, dynamic `new` — is refused by name. Differential against the
interpreter over the state and action conformance corpus, exactly as N1 is refereed.

## M3 — a resource report, and refusal of the unbounded

Every compiled model states its RAM (tables, queues, the state vector), its stack bound per entry
point and its code size, before it is flashed; a budget file the model must fit is a build error
when exceeded. This is what makes the target honest: the report is part of the artifact.

## M4 — the host interface: `init`, `tick(dt)`, `post(event)`, `read(feature)`

A generated model is a library the RTOS task calls: initialise, advance by `dt` (the shared clock of
A5, restricted to a fixed step), post an event into the fixed-size queue, read a feature. This is
the embedded restriction of the C ABI in N2.4/I4 — same header shape, no allocation, no callbacks
into the host except the ones the model declares. Designed with N2.4 and I4, implemented here.

## M5 — Zephyr on QEMU as the proof

One state machine and one action from the conformance corpus, compiled by M2, linked into a Zephyr
application, run under QEMU (`qemu_cortex_m3`) with the trace read back over the serial console and
compared with the interpreter's trace golden. CI runs it in a container with the Zephyr SDK; a
maintainer runs it on hardware. The claim "runs under an RTOS" is made only once this passes.

## M6 — embedded metadata and library definitions

`EmbeddedTarget` metadata (the MCU, the step, the queue depths, the budget), and a small library
package of fixed-width numeric types and bounded collections the compiler recognises, so a model
says what it is for and the checker can refuse a `Real` where the target has no FPU. Declared in
the notation, checked by the existing passes, read by M2/M3. Last, because it depends on knowing
what M2 needs.

---

# Proposed, not started

**The PDF path onto the HTML backend.** The backend
[html-document-backend.md](html-document-backend.md) designs is now implemented: `docrender.HTML`
renders `-doc-form html` straight from the document IR, with the semantic structure, the `sysml-`
classes and `data-` model facts, the default stylesheet in a cascade layer that reader CSS
overrides without specificity fights, `-html-css`, `-html-no-default-css`, `-html-default-css`,
`-html-fragment`, and linked HTML sets sharing one `sysml-document.css`. What remains is the
migration designed alongside it: point the HTML-input PDF engines (`weasyprint`, `prince`) at that
markup and retire `internal/docpdf`'s Markdown re-parse and its own HTML writer, which splits the
print styling out as a shared asset and moves the PDF goldens. Pandoc keeps reading the Markdown,
and `-doc-form markdown` is unaffected. About one session, independent of every track above.

**The pilot as an execution referee.** [pilot-execution-referee.md](pilot-execution-referee.md)
established that the pinned pilot evaluates model-level expressions and nothing else, so
`cmd/pilot-exec-diff` can adjudicate the expression rows of `spec-compliance.md` and no external
implementation adjudicates actions or state machines. Widening that referee means finding one,
not more harness work.

# Suggested sequencing

Two orders, because there are two kinds of item. The **track-local** orders say where to start
inside a track; the **cross-cutting** order says which tracks' first items go first when a session
must choose. The order agreed at the previous baseline had A1 first and X2/X1/X6 second; A1, X1
and X2 landed in 0.6.0, so both orders below are rewritten around what remains. One pull request is
in flight at this baseline — #774 (the ontology modules, in conflict) — and it lands or is closed
before the Track D order is consulted.

## What the next release is taking

These items are in progress as work for the release after 0.6.0 — stated so a reader knows they
are being taken, not abandoned. None had a pull request at this baseline.

- **Track F** — the three executor defects `known_failures.txt` carries (F1 a node reached over two
  successions performs once per token, F2 a join fires on parked tokens, F3 a merge re-enters on
  every traversal), each closed by removing its line from the known-failures file.
- **Track S** — multiple valid executions: S1 admissible outcomes in the conformance schema, S2
  choice points in the trace, S3 a selectable scheduling policy with the default unchanged, S4
  bounded exhaustive exploration against the semantic oracle. In this order; S3 before A5.
- **Track X** — the rest of the track: X3, X4, X6, X7 and X8 (X1 and X2 are done; X2's chain-read
  half goes with X6 since L7 needs both).
- **Track A** — A6 first (small, on A1), then A2 once X6 gives it the function value, then A3,
  then A5 once S3's scheduling policy is in so the clock and the policy are one decision.
- **Track E** is deferred to the release after that one: E1–E7 edit the same executor loops F fixes
  and S instruments, and landing them concurrently would put three tracks on one function.

## Cross-cutting order

1. **F1–F3, then S1–S4** — the executor defects first, since they change what the conformance
   fixtures expect, then the multiple-executions instrumentation over the corrected loops. Both
   precede anything else that touches the action executor.
2. **A6, then X6, then A2** — verification verdicts from a body (A6 is small on A1); function
   values, which the standard analysis library (`SampledFunctions`, `TradeStudies`) and L7 need and
   which A2 waits on for `evaluationFunction`; then trade-study iteration, evaluation and selection.
   X2's remaining chain-read coercion goes with X6 so `interpolateLinear` runs when `Sample` does.
3. **The REPL state attachment** — `%state <machine>` attaching to the exhibiting object (#845,
   **landed**), *after* validating on `main` that #810 closed the original duplicate initial `do`,
   which the review did. Done; kept here so the order reads as agreed.
4. **Q2, then Q1** — runtime query bindings and `all T`, with the page that says which query is
   which. Depends on the object, state and trace representations being stable, which #810, #836
   and #843 (all landed) have settled; Q4 (parameter defaults, #849) landed independently ahead of it.
5. **I2, I3, then I4's client** — the shared fixtures, the thin R, Julia and MATLAB packages, the
   C client, each derived from the wire contract (I1, landed in #848); the C *ABI* half of I4 is
   not here — it is step 9.
6. **D2 and D1, then D9.1 and D9.2** — Flexo: the standard vocabulary for expression trees and
   end structure, then the authenticated push and the branch read (the collection JSON annotations,
   D3.4, landed in #850). Push and read depend on the vocabulary quality, which is why they come
   last in the step; re-record the live-stack harness after D1/D2.
7. **A3, then A5 (after S3), then A4** — sweeps, Monte Carlo and tables as orchestration over A1;
   one clock for actions and states once the scheduling policy exists to order what the clock
   releases; then the continuous-time runner. The clock is prerequisite to coherent action/state
   simulation and to Q3, which follows it.
8. **M1, then M2 (with N2.6)** — the closed behavior IR, then the state and action C backend over
   static tables. Embedded behavior compilation depends on the closed IR; the native track's action
   and state phases start from the same IR rather than a second one. M3 and M5 prove it; M6 last.
9. **The shared C ABI** — N2.4 / I4 / M4 designed once, after N2.2 (the budget) is decided and
   after M1 fixes what an embedded entry point looks like, so a stable native/embedded calling
   contract exists to design against rather than three.
10. **Track E** — after F and S have landed and a release has shipped with them, in the track's
    own order below.

## Track-local orders

- **Release follow-through.** **R1** is done. **R2**–**R5** as the accounts and hardware appear:
  publisher tokens for npm, Maven Central and crates.io, a real Mac for the tap, an Apple Developer
  and an OV/EV certificate to sign with, and a marketplace publisher for the extension. None gates
  the others or anything below. One engineering item sits beside them: recount the hand-written
  test figures in `README.md` and `spec-compliance.md` to match the gate table above, or fold them
  into `cmd/doc-counts` so they are generated and cannot drift again. Small, and independent of
  every track.
- **Track L.** L3–L6 landed (#830, #821, #818, #825/#861). Only L7 is left; it needs X6 and the
  chain-read half of X2, so it is step 2 above.
- **Track N.** N2.1 (records and enums) first, since compiling an analysis case and the
  differential's record and constructor coverage both need it; decide N2.2 (the budget) before
  N2.4; N2.3 tracks L4 package by package; N2.6's actions and states are Track M's M1/M2.
- **Track D.** The RDF ratchet is 346/346 with no refusal left; step 6 above is next; **D7** is
  mechanical now that identity is stable and fits anywhere; rebase and land #774, then **D8**'s
  profile after it, since it only becomes conformant behind D1 and D2; **D10** (write-through from
  a view-only project) after D9.1 and D9.2, which it reads and writes through.
- **Track F.** F1, F2, F3 in that order: F1 and F2 are the token-per-succession model that F3's
  per-traversal merge sits on. Each closes by deleting its `known_failures.txt` line.
- **Track S.** S1 (the schema) before S2 (the trace), since a trace choice point names the
  admissible set; S3 after S2, since a policy is observed through the trace; S4 last, over S3's
  policies.
- **Track E.** Deferred to the release after the one taking F and S. When it is taken, the order
  is **E1** (termination of an ongoing performance, which **E2** and **E4** build on), then **E2**,
  then **E4**; **E6** whenever asked, being a day's work; **E3** and **E5** only after their design
  records; **E7** after the object-model item it depends on.
- **Track X.** X6 first (step 2 above, with X2's chain-read half); X8's harness half
  (normalization, adjudication, the RDF expression round trip) alongside so every later X item is
  measured; X3 and X4 whenever a model needs them; X7 last.
- **Track A.** A6, then A2 (after X6), then A3, then A5 (after S3), then A4 — steps 2 and 7 above.
- **Track V.** Everything queued has landed (#822, #900, #831, #817, the rule pull requests, #811
  reconciled with #907, #909); work the census's 1 *not implemented* and 53 *unknown* rows,
  negative case first, each change moving its row.
- **Track Q, I, M.** Entirely given by the cross-cutting order above.

# opensysml — the public Go API

`github.com/Open-MBEE/OpenSysML` at `client/opensysml` is the Go surface of
OpenSysML: parse SysML v2 models, look up symbols, evaluate expressions,
instantiate parts, run actions and state machines, verify constraints and
requirements, evaluate calculations, query the model, render documents, convert
notations and edit source — from Go code, using the engine already linked into
the calling binary. Every RPC the service answers is a method here.

```sh
go get github.com/Open-MBEE/OpenSysML@latest
```

Nothing else is installed: the SysML standard library is embedded in the module,
and no operation shells out.

```go
client, err := opensysml.New()
if err != nil { ... }
defer client.Close()

model, err := client.ParseFile(ctx, "vehicle.sysml")
mass, err := client.Evaluate(ctx, model, "mass", opensysml.WithSubject("Demo::sedan"))
inst, err := client.Instantiate(ctx, model, "Demo::Vehicle")
```

## The whole surface

| What you want | Call |
| --- | --- |
| Parse one document | `ParseFile`, `ParseSource` |
| Parse a model of several documents | `ParseFiles`, `ParseDocuments` |
| Read the model | `LookupSymbol`, `Diagnostics` |
| Compute with it | `Evaluate`, `Instantiate`, `EvaluateCalc`, `RunAnalysis` |
| Run behavior | `ExecuteAction`, `ExecuteState` |
| Run every linearization of it | `ExploreAction`, `ExploreState`, `ExploreAnalysis` |
| Check it | `VerifyConstraint`, `VerifyRequirement`, `VerifySatisfaction` |
| Search it | `Query`, `QueryOSLC` |
| Report on it | `RunDocumentQuery`, `RenderDocument` |
| Write it out | `Convert`, `ConvertFile`, `ConvertSource` |
| Change its source | `ApplyEdits` |

Execution and verification take the same handles the rest of the API takes:

```go
run, err := client.ExecuteAction(ctx, model, "Demo::addFive",
	map[string]opensysml.Value{"result": opensysml.Int(10)})

verification, err := client.VerifyConstraint(ctx, model, "Demo::massBudget",
	opensysml.Against("Demo::sedan"))
if !verification.Verdict.Holds {
	// verification.Verdict.Condition names what evaluated false
}
```

A verdict of false is an answer about the model, not an error: a verification
fails only when it could not be evaluated at all, and then it is a
`*VerifyError` whose `Reason` classifies the failure. A condition the runtime
could not evaluate for one subject arrives as `Verdict.Undecided()`.

What running a verification case's body answered is a separate answer, reported
beside the satisfaction verdict rather than instead of it, by a service
advertising `CapabilityVerificationVerdicts`. `Verification.Verifications`,
`Analysis.Verifications` and `Satisfaction.Verifications` carry those
`VerificationVerdict`s; because one `VerifySatisfaction` response can cover
several requirements, each `Verdict` in it carries only the cases of its own
`RequirementID`.

A trade study run through `RunAnalysis` reports each application of its
`evaluationFunction` as an `Evaluation` in `Analysis.Evaluations`, in subject
order, the alternative it scored an `InstanceID` that `Analysis.Instance`
resolves; `Analysis.Selected()` is the one `selectOne` picked, and `Tied` marks
another scoring the same. Reported by a service advertising
`CapabilityCaseEvaluations`. A run that fails leaving something to inspect — an
alternative it could not score after scoring others, an objective left
undecided by the failure — is an `*AnalysisError` whose `Partial` keeps the
outputs and evaluations it made, every verdict undecided; a request refused
before the run (`ReasonWrongKind` for a symbol that is no case) or a failure
that left nothing to report is a `*VerifyError` alone:

```go
analysis, err := client.RunAnalysis(ctx, model, "Trade::lightest")
var failed *opensysml.AnalysisError
if errors.As(err, &failed) {
	analysis = failed.Partial // what the run computed before it failed
}
for _, e := range analysis.Evaluations {
	alternative := analysis.Instance(e.Arguments[0].(opensysml.InstanceID))
	// alternative.TypeSymbolID, e.Result or e.Error, e.Selected, e.Tied
}
```

A run resolves its choice points — several tokens steppable at once, several
guards holding, several transitions enabled — under the scheduling policy
`WithSchedule` (or `Schedule`, for an analysis) names: `declared`, `reverse`
(the default) or `seed:<n>`. `ExploreAction`, `ExploreState` and
`ExploreAnalysis` run every linearization instead and answer an `Exploration`:
one `Outcome` per distinct result with the `Linearizations` that reached it and
one run's `Witness`, and whether the search was `Complete` or which `BudgetsHit`
ended it. `WithSchedule("explore:runs=64,depth=8")` sets the budget; a run that
fails under some orders is an `Outcome` with its `Error` set, not a failed call.
The single-run and exploring calls refuse each other's policies with
`CodeInvalidArgument`, so a policy is never quietly answered by the wrong shape.

```go
exploration, err := client.ExploreAction(ctx, model, "Demo::race", nil)
for _, outcome := range exploration.Outcomes {
	fmt.Println(outcome.Outputs["winner"], outcome.Linearizations, outcome.Witness)
}
fmt.Println(exploration.Status()) // complete (6 runs)
```

An action or state machine runs on a simulation clock that starts at 0 and
advances through every `accept after`/`accept at` its tokens and transitions
wait on; `ActionRun.FinalTime` and `StateRun.FinalTime` are where it stood, in
seconds, when the run ended, reported by a service advertising
`CapabilityFinalTime`.

Queries are built from typed conditions rather than a string dialect, so an
unsupported operator is a compile error rather than a refused call:

```go
elements, err := client.Query(ctx, model, opensysml.Query{
	Scope:  []string{"Demo"},
	Select: []string{"name", "qualifiedName"},
	Where: opensysml.All(
		opensysml.Equals("@type", "PartUsage"),
		opensysml.Equals("name", "engine").Not(),
	),
})
```

Edits are typed the same way — `SetValue`, `Rename`, `AddMember`, `Delete` —
and either all apply, answering the edited source, or none do and the refusal
arrives as an `*EditError` naming its kind:

```go
result, err := client.ApplyEdits(ctx, model,
	opensysml.SetValue{Target: "Demo::sedan::mass", Value: "1200.0[SI::kg]"})

var refused *opensysml.EditError
if errors.As(err, &refused) && refused.Failure == opensysml.EditFailureRenameReferenced {
	// refused.Referring names what still refers to it
}
```

## What a model is here

`ParseFile` reads the one path it is given and `ParseSource` the one string, and
neither follows an import into a sibling file: a name declared in another file is
an unresolved reference, reported as a diagnostic on the model rather than as a
failed call.

A model that lives in several files is parsed as one model by `ParseFiles`, or by
`ParseDocuments` for files and in-memory sources together:

```go
model, err := client.ParseFiles(ctx, paths)

model, err := client.ParseDocuments(ctx, []opensysml.Document{
	opensysml.File("lib.sysml"),
	opensysml.Source("top.sysml", generated),
})
```

Each document is parsed on its own and all of them are indexed together, so an
import between them resolves and every symbol of the set is one lookup,
evaluation or instantiation away. Nothing is concatenated: a document keeps its
own name, so a diagnostic locates itself in the file it came from, and
`Model.Roots` holds each document's root namespace in the order given —
`Model.Root` is the first, as it is for a one-document model. A set is cached by
what is in it, so parsing the same documents again answers the same model hash.

Two operations write one document's own notation back out, and they are refused
with `CodeFailedPrecondition` for a model of several rather than applied to one
of them: `Convert` from a model handle, and `ApplyEdits`. Convert a single
document of such a set with `ConvertFile` or `ConvertSource`.

## Concurrency, contexts and lifetime

A `Client` is safe for concurrent use: any number of goroutines may call it, and
each answer belongs to its caller. One `Client` per process is the intended
shape — `New` builds and prewarms a standard-library index, which is what makes
it worth keeping.

Contexts are honoured as the wire honours them. A call whose context is already
done is refused with `CodeCanceled` or `CodeDeadlineExceeded`; a context that
ends while a call is running withholds the answer the same way, because the
engine — like a service answering a caller who stopped listening — still runs
the call it started. A deadline therefore bounds how long a caller waits, not
how long the engine works.

After `Close`, every call is refused with `CodeUnavailable`, and closing twice is
not an error, so a deferred `Close` beside an explicit one is safe.

`ServerInfo.Version` is the version of OpenSysML linked into the binary — the
module version an importing program resolved, `dev` when it was built from a
checkout. It is informational: negotiate on capabilities.

## Why in-process is the default

This is a Go repository: a Go program that imports this module already links
the parser, the semantic engine and the runtime. `New` calls them directly —
no port, no child process, no serialization round trip. It answers through the
same service implementation (`internal/grpc.Service`) the wire transports
serve, so the semantics are the service's semantics: the same content-addressed
parse cache and model hashes, the same capability list, the same in-band
failures, the same runtime budgets (read from the environment, as the service
reads them).

`Dial` addresses a service the caller did not start: a shared, long-lived
`sysml-grpc` run elsewhere, addressed explicitly (`"host:50051"` or
`"https://sysml.example.com"`). It speaks the Connect protocol with protobuf
bodies by default — JSON (`WithJSONBody`) costs an order of magnitude in
encoding time on large responses and is a debugging affordance, per
`docs/internals/design/transport-evaluation.md`. This package never spawns a
service: a Go process that wants the engine in-process uses `New`, which is
strictly better than a private child — the child's entire job would be to serve
the code `New` already calls.

## Errors

A call fails in exactly one of two ways, and the difference is part of the API
because it is part of the wire contract:

| Failure | Type | Test with | Example |
| --- | --- | --- | --- |
| The call is refused | `*StatusError` | `errors.Is(err, opensysml.CodeNotFound)` | an unknown model hash, an unreadable path |
| The answer reports a failure | `*FailureError` | `errors.Is(err, opensysml.ErrFailure)` | an unparsable expression, an unknown symbol |

A `StatusError` renders as `opensysml: NOT_FOUND: model not found: …`, naming
the code canonically. `StatusError.Code` is the canonical gRPC status code,
whichever implementation answered: in process it is the code the handler refused
with; remotely it is the Connect error code, which numbers identically. A transport failure that
never reached the service is `CodeUnavailable`. A panic in the engine does not
cross the boundary: it arrives as `CodeInternal`.

Syntax errors are neither: parsing broken source succeeds, and the errors are
`Model.Diagnostics` — the same shape the LSP and the wire report.

## Ownership

Everything a `Client` returns is a copy the caller owns. In process there is no
serialization boundary to force this, so it is a documented promise instead:
no returned value aliases engine state, and mutating a returned `Model`,
`Symbol`, `Value` or `Instance` changes nothing about the model, the cache or
later answers.

## Capabilities

Negotiate on names, not versions: `ServerInfo.Capabilities` lists what the
answering implementation supports, and `ServerInfo.Has` checks one. A request
that asks for an unavailable capability is refused with `CodeUnimplemented`;
capabilities that describe response population instead omit the fields they
name. Check the list first for an operation-specific error (the `Capability*`
constants name the known ones). Six capabilities are checked for you: a `Complex`
among `ExecuteAction` inputs or `EvaluateCalc`/`RunAnalysis` arguments needs
`complex_values`, an `Array`, `Vector` or `VectorQuantity` needs
`structured_values`, a `MeasurementRef` needs `measurement_refs`, a `Function`
(a calc held as a value, sent back to bind a calc-typed parameter) needs
`function_values`, a `Set` needs `set_values` and a `TensorQuantity` needs
`tensor_values` — each at the top level or nested in a sequence, set or array; a
service without them would read the value as null, so the client refuses with
`CodeUnimplemented` before sending anything. A scheduling policy is checked the
same way: `WithSchedule`/`Schedule` need `schedule`, and the `Explore*` calls
`schedule_explore` besides, since a service without them would run under the
default, or run once, rather than refuse.

A `Set` arrives with its elements in the service's canonical order — Booleans,
then numbers, strings, quantities, enumeration literals and objects, each class
in its own order — so two equal sets arrive alike, and one that lists a member
twice reads as an unsupported `Null` naming it; a `Set` you send may list its
elements in any order, but one listing an element twice, by `Equal`, is refused
with `CodeInvalidArgument` before it is sent rather than read as one element.
`Set.Contains` tests membership and `Equal` compares any two values as the
service does — sets by membership, sequences in order, numbers by value, so
`Int(1)` is `Real(1)` and a `Complex` on the real axis is its real part, exactly
across the whole `Int` range; a `Quantity` by magnitude through its `Term`, so
`1 [m]` is `100 [cm]` (exactly, while the magnitude is an `Int` and the scale a
whole ratio), and one without a `Term` in its unit as written. A
`TensorQuantity` carries its dimensions and one `Quantity` per
component in row-major order, at any rank; one whose dimensions are not all
positive, or whose components do not fill them, is refused with
`CodeInvalidArgument` before it is sent.

## Stability

The module is `v0`, so the Go compatibility promise does not yet formally bind
it (see `docs/project/releasing.md` for how releases are cut). Within `v0`,
this package's exported surface is what OpenSysML commits to keeping
compatible:

- **Stable**: `Client` and its operations, `New`, `Dial`, the option functions,
  the error model (`Code`, `StatusError`, `FailureError`, `VerifyError`,
  `EditError`, `ErrFailure`), and the data types they exchange. Changes will be
  additive.
- **Experimental**: RDF conversion, whose vocabulary may change without a
  compatibility path — a `Conversion` says so in `Experimental` and
  `ExperimentalNotice`.

The `Client` interface is sealed, so adding a method is not a breaking change.

One thing is deliberately absent: generated model-ergonomics types (a Go struct
per SysML definition). Models are read through `Symbol`, `Instance` and
`Value`, which need no code generation step.

No operation here shells out to an SMT solver: verification evaluates conditions
with the runtime that `Evaluate` and `Instantiate` use, so an in-process caller
needs nothing installed. The solver-backed analyses (`%check`, `%solve`,
`%optimize`, `%explain`) belong to the REPL and are not part of the service API.

## Conformance

The language-independent suite in `conformance/` runs through this package —
not through raw stubs — as the `pkg` (in-process) and `pkg-connect` (remote)
protocols of the reference runner:

```sh
make conformance-pkg
# or: go run ./cmd/conformance -protocols pkg,pkg-connect -allow-skips
```

Two scenarios are reported as skips, because they state a request this API's
types cannot express: a parse naming no source, and a query naming no comparison
operator. Every other scenario runs through this package, on both protocols.

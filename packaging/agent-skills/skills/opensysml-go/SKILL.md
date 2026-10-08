---
name: opensysml-go
description: Parse, query, instantiate, execute and verify SysML v2 models from Go with the `opensysml` package (github.com/Open-MBEE/OpenSysML/client/opensysml), which links the OpenSysML engine in-process or dials a shared sysml-grpc service — requiring a clean model, evaluating expressions, reading object features, running actions and calcs, checking requirements, telling refused calls from reported failures. Use when writing Go that reads or runs SysML v2 models.
---

# Using OpenSysML from Go

The Go package links the parser, the semantic engine and the runtime into the calling binary, so
`opensysml.New()` needs no service, port or child process. Reference: `docs/guide/09-clients.md`
(From Go), `client/opensysml/README.md` and `docs/reference/api.md` in the OpenSysML repository.

## Install

```bash
go get github.com/Open-MBEE/OpenSysML@latest
```

```go
import "github.com/Open-MBEE/OpenSysML/client/opensysml"
```

Nothing else is installed: the standard library is embedded and no call shells out.
`opensysml.Dial("host:50051")` is the other constructor, for a `sysml-grpc` someone else runs; the
package never starts a service of its own. Both return the same `opensysml.Client` interface.

## Load, and require a clean model

```go
client, err := opensysml.New()
if err != nil { return err }
defer client.Close()

model, err := client.ParseFile(ctx, "model.sysml")
if err != nil { return err }          // the file could not be read, or the call was refused
if !model.OK() {
	for _, d := range model.Errors() { // Diagnostic: Severity, Message, Code, Span
		fmt.Println(d.Code, d.Message)
	}
	return errors.New("model has errors")
}
```

Parsing broken source succeeds: syntax and resolution errors arrive in `model.Diagnostics`, never as
`err`. Branch on `d.Code` (`"syntax"`, `"unresolved"`, ...), never on message text.
`opensysml.WithStrictConformance()` makes OpenSysML-only notation an error. For a model spread over
several files use `client.ParseFiles(ctx, []string{"base.sysml", "extra.sysml"})`; `ParseFile`
reads one document, so imports across files do not resolve.

## Evaluate, instantiate, run

```go
mass, err := client.Evaluate(ctx, model, "Demo::sedan::mass")       // opensysml.Real(1200)
mass, err = client.Evaluate(ctx, model, "mass", opensysml.WithSubject("Demo::sedan"))

inst, err := client.Instantiate(ctx, model, "Demo::sedan")          // *Instantiation
fv := inst.Root.FeatureValues["mass"]                               // fv.Value, fv.Values or fv.Error
// inst.Instances is every object reachable from Root; inst.Instance(id) resolves an InstanceID

run, err := client.ExecuteAction(ctx, model, "Demo::addFive",
	map[string]opensysml.Value{"x": opensysml.Int(10)})
run.Outputs["result"]                                               // opensysml.Int(15)

calc, err := client.EvaluateCalc(ctx, model, "Demo::Margin",
	opensysml.Real(1200), opensysml.Real(2000))
calc.Result                                                         // opensysml.Real(800)
```

A `Value` is one of the concrete types in `value.go` (`Int`, `BigInt`, `Real`, `Complex`, `Bool`,
`String`, `Quantity`, `InstanceID`, sequences, ...); type-switch on it. Three "no value" results are
not errors and must not be coerced to zero: `opensysml.Undetermined` (the model leaves the value
open), `opensysml.Unset` (an object holds nothing for the feature) and `opensysml.Null` (the model's
`null`).

## Verify

```go
v, err := client.VerifyRequirement(ctx, model, "Demo::Vehicle::lightEnough",
	opensysml.Against("Demo::truck"))
if err != nil { return err }          // could not be evaluated at all: a *VerifyError
switch {
case v.Verdict.Undecided():           // evaluation failed for this subject; see v.Verdict.Error
case !v.Verdict.Holds:                // the model answered false; v.Verdict.Condition says what
}
sat, err := client.VerifySatisfaction(ctx, model, "")  // every `assert satisfy R by x;`; or a scope FQN
```

A false verdict is an answer about the model, not an error. `VerifyConstraint` takes the same
`Against`; `ValidateInstance` checks every assertion about one object and its parts.
`opensysml.WithEngine("check")` picks an engine (`ListEngines` names them).

## Errors

A call fails in exactly one of two ways:

- `*opensysml.StatusError` — the call was refused; match with
  `errors.Is(err, opensysml.CodeNotFound)` (`CodeInvalidArgument`, `CodeUnimplemented`, ...).
- `*opensysml.FailureError` — the call was answered and the answer reports a failure (an
  unparsable expression, an unknown symbol); match with `errors.Is(err, opensysml.ErrFailure)`.
  `*VerifyError` (with a `Reason`), `*EditError` and `*AnalysisError` (with `Partial` results) are
  `FailureError`s too; recover them with `errors.As`.

## Pitfalls

- Reuse one `Client`; it caches parses. Pass a real `context.Context` so long runs can be cancelled.
- Use qualified names (`Package::Def::feature`) wherever a symbol is named.
- `ExecuteAction` refuses an `explore` schedule and `ExploreAction` refuses a one-run schedule, with
  `CodeInvalidArgument`; pick the call that matches the policy.
- `OpenSession` (step-by-step play of a model) works only on a `New` client; a `Dial` client refuses
  it with `CodeUnimplemented`.
- Write models with the `sysml-v2-modeling` skill; `sysml -validate` reports the same diagnostics as
  `model.Diagnostics`.

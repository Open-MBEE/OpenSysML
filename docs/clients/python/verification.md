# Verification and analysis

Verification, calculation and analysis calls use the same runtime as the
`sysml` REPL. They evaluate the model's declarations and return typed results
over the service API.

## Constraints, requirements and satisfaction

Use model methods to check satisfaction assertions, or name a constraint or
requirement directly. A subject asks about an instantiated object rather than
the declaration's default values.

```python
model = opensysml.load("model.sysml", strict=True)

for verdict in model.verify_satisfaction():
    print(verdict)

model.satisfied()
model.verify_satisfaction("Demo::analysisContext")
model.verify_constraint("Demo::Vehicle::massOK", subject="Demo::sedan")
model.verify_requirement("Demo::Vehicle::lightEnough", subject="Demo::sedan")
```

A `Verdict` reports whether its condition holds, the model element and
instance involved, any objects built for the check, diagnostics and an
explanation. A false verdict is an answer, not an exception. A request that
cannot be answered—such as an unknown symbol, an uninstantiable subject or an
exhausted execution budget—is reported separately through `verdict.error`;
`raise_for_error()` turns that failure into `ExecutionError`.

```python
verdict = model.verify_requirement(
    "Demo::Vehicle::lightEnough",
    subject="Demo::sedan",
)
verdict.holds
verdict.condition
verdict.kind
verdict.instance_id
verdict.instances
verdict.diagnostics
verdict.explain()
```

A requirement or constraint whose condition reads `in` parameters takes their
values at verification time: `arguments` binds them positionally in declaration
order, `named_arguments` by name, and the two may be mixed, as `run_analysis`
binds a case's inputs. An unknown name, more positional values than parameters,
a parameter left without a value, default or same-named value on the checked object, or a value not of the parameter's
type is reported through `verdict.error`. Arguments need the
`verification_arguments` capability; only the evaluate question takes them.

```python
verdict = model.verify_requirement(
    "Demo::Under",
    subject="Demo::sedan",
    named_arguments={"limit": 5},
)
model.verify_constraint("Demo::Between", arguments=[3], named_arguments={"high": 4})
```

`WrongKindError` is raised when a valid symbol is not the kind the method
requires; it is an invalid request, not a false verdict. A model with no
satisfaction assertions in a requested scope returns no verdicts, and
`satisfied()` is then vacuously true.

### Questions beyond evaluation

`question="holds"` asks a solver to prove a condition for every possible
assignment of its free features; `"satisfiable"` asks for one assignment.
The default `"evaluate"` runs the expression over the model's current values.

```python
verdict = model.verify_constraint("Demo::lemma", question="holds")
verdict.question
verdict.status
verdict.strength
verdict.witness
verdict.error
```

Statuses include `holds`, `violated`, `undecided`, `satisfiable` and
`unsatisfiable`. An undecided result explains why the service could not
conclude; it is never presented as proof. Solving requires the
`verification_questions` capability, and an unsupported service raises
`MissingCapabilityError`.

`verify_satisfaction` evaluates assertions in one request. Each verdict's
`instance_id` identifies its subject among `verdict.instances`. If a
requirement has verification cases, `verdict.verifications` contains their
individual outcomes without changing the requirement engine's own result.

## Validating an object as a whole

`validate_instance` checks every assertion about one instantiated part and
the objects it holds. It walks the object graph and evaluates applicable
`assert constraint`, requirement and `satisfy` assertions.

```python
validation = model.validate_instance("Fleet::car")

bool(validation)
len(validation)
for verdict in validation:
    print(verdict.instance_path, verdict.holds)

validation.violated
validation.undecided
validation.summary
validation.bounded
validation.raise_for_error()
```

Each result is a `Verdict`. `instance_path` identifies the object under the
root (for example, `wheels[2]`); `instance_id` selects it from
`validation.instances`. `violated` lists decided false answers, while
`undecided` lists assertions the service could not evaluate. `Validation` is
false if an assertion is undecided or the walk reached its depth bound. A
constraint without an `assert` is not swept; check it explicitly with
`verify_constraint`.

An unknown or non-instantiable part raises `ExecutionError` from the call.
The service must advertise `verification` for verification calls and
`validate_instance` for whole-object validation.

## Calculations, analyses and sweeps

`calc` invokes a calculation with positional arguments. A calc usage named
without arguments evaluates its own members and returns its output features.

```python
result = model.calc("Demo::add", arguments=[2.5, 4.0])
result.value
result = model.calc("Demo::c")
result.outputs
```

`run_analysis` executes an analysis case. A usage that binds its subject needs
no extra subject; a definition or a usage without a subject binding takes the
subject to instantiate. Positional or named arguments bind the case's `in`
parameters.

```python
run = model.run_analysis("Demo::CostAnalysis", subject="Demo::barge",
                         named_arguments={"limit": 50.0})
run.outputs
run.verdicts
bool(run)
```

The result carries outputs, verdicts and the subject's instances. Objectives
that could not be decided are marked with an error rather than treated as
passing. An unbound parameter, invalid symbol kind, failed step or recursive
case raises an execution error; an `AnalysisRunError` retains the partial
result of a case that failed partway through.

Trade studies include a `CaseEvaluation` for each alternative, preserving its
arguments, result or error, and whether it was selected or tied. The
`case_evaluations` capability controls whether those details are available.

`run_sweep` evaluates an analysis case or calc over ranges. Each row records
the inputs, outputs, verdicts and elapsed seconds; a failed run is a row of
its own, so later values are still evaluated.

```python
table = model.run_sweep(
    "Demo::CostAnalysis",
    {"limit": (10.0, 50.0, 20.0)},
    subject="Demo::barge",
)
table[0].inputs
table[0].outputs
table.failures

sampled = model.run_sweep(
    "Demo::CostAnalysis",
    {"limit": (10.0, 50.0)},
    subject="Demo::barge",
    samples=8,
    seed=42,
)
sampled.sampled
```

Ranges may specify a step; several ranges run their Cartesian product.
Sampling uses `samples` and `seed`. Invalid ranges, units, parameters or
plans exceeding the service's run budget raise `ExecutionError`.

## Choosing an engine and reading evidence

Verification methods, `calc`, `run_analysis` and `run_sweep` accept
`engine=`: `"auto"` lets the service choose, `"all"` composes every engine
that covers the question, or a registered engine name selects one directly.
An engine that cannot answer the question returns an error result rather than
a false verdict. `model.connection.list_engines()` reports engine names,
authority, supported question kinds, limits, external dependencies and
readiness.

Every `Verdict`, `CalcResult` and `AnalysisResult` carries a `Standing`:
which engine answered, the evidence strength (`observed`, `witnessed`,
`bounded` or `proved`) and the bounds it reached.

```python
verdict = model.verify_constraint(
    "Demo::Vehicle::massOK",
    subject="Demo::sedan",
)
verdict.standing
verdict.engine, verdict.strength, verdict.bounds
verdict.explain()
```

If the service does not advertise `engines`, named-engine selection and
`list_engines()` raise `MissingCapabilityError`, and a result has no reported
standing.

For action and state-machine execution, see [Instances and values](instances.md).

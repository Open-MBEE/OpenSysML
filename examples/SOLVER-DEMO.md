# Solver demo

[`solver-demo.sysml`](solver-demo.sysml) is a rover's power, mass and configuration
budget, written so that each of the experimental solver commands has something to
answer. Constraint solving asks whether conditions *can* hold, which is not what
`%constraint`/`%satisfy` answer — those evaluate what does hold of an object.

Every command below needs an external solver: `z3` or `cvc5` on `PATH`, or
`OPENSYSML_SMT` pointing at one — see
[installing a solver](../docs/guide/01-install.md#installing-a-solver-optional).
`%optimize` needs z3 specifically, since optimization is a z3 extension cvc5 does
not implement.

Load the model and run the commands at the prompt:

```bash
./bin/sysml examples/solver-demo.sysml
```

## `%check` — can it hold?

```
%check SolverDemo::Rover::powerFitsBudget
```

```
✓ Constraint powerFitsBudget is satisfiable (z3, 7ms)
  SolverDemo::Rover::drivePower = 220
  SolverDemo::Rover::sciencePower = 0
```

A requirement states several conditions at once, so one query is about all of
them, and a satisfaction assertion asks the same of the requirement asserted of
an object:

```
%check SolverDemo::PowerBudgetRequirement
%check SolverDemo::rover1::powerIsBudgeted
```

`sat`, `unsat` and `unknown` are kept distinct: an `unknown` is reported as
undecided, never as either verdict.

## `%explain` — which conditions conflict?

`OverbookedBudget` requires a drive power and a science power that together
exceed its own budget, so it cannot hold:

```
%check SolverDemo::OverbookedBudget
%explain SolverDemo::OverbookedBudget
```

```
✗ Requirement OverbookedBudget is unsatisfiable: 3 conditions conflict (z3, 26ms)
  Every condition below is needed: dropping any one leaves the rest satisfiable.
  1. required condition: `rover.drivePower + rover.sciencePower <= 200` — requirement OverbookedBudget, at …:65:13
  2. required condition: `rover.drivePower >= 150` — requirement OverbookedBudget, at …:69:13
  3. required condition: `rover.sciencePower >= 90` — requirement OverbookedBudget, at …:73:13
```

The core is minimal: every condition listed is needed for the conflict.

## `%solve` — what values would satisfy it?

```
%solve SolverDemo::PowerBudgetRequirement
```

```
✓ Requirement PowerBudgetRequirement has values satisfying it (z3, 7ms)
  Synthesised:
    SolverDemo::PowerBudgetRequirement::'rover.drivePower' = 60
    SolverDemo::PowerBudgetRequirement::'rover.sciencePower' = 40
  One witness: a solver may answer with any of the assignments that satisfy it.
```

The assignment is one witness of possibly many; another solver, or another
version of the same one, may answer with different values.

## `%configure` — which variants are permitted?

`roverFamily` has two variation points whose choices interact: a steerable
antenna requires the rugged chassis, which rules out one of the four
combinations.

```
%configure SolverDemo::roverFamily::steerableNeedsRugged all
```

```
✓ Constraint steerableNeedsRugged permits 3 selections of variants, which are all of them (z3, 11ms)
  1.
    SolverDemo::roverFamily::antenna = SolverDemo::roverFamily::antenna::fixed
    SolverDemo::roverFamily::chassis = SolverDemo::roverFamily::chassis::light
  2.
    …
```

A selection can be chosen instead of synthesised, and the rest is filled in
around it:

```
%configure SolverDemo::roverFamily::steerableNeedsRugged antenna=steerable
```

```
✓ the chosen variants are consistent with Constraint steerableNeedsRugged (z3, 9ms)
  Already fixed:
    SolverDemo::roverFamily::antenna = SolverDemo::roverFamily::antenna::steerable  (chosen)
  Synthesised:
    SolverDemo::roverFamily::chassis = SolverDemo::roverFamily::chassis::rugged
```

Enumeration is bounded by `OPENSYSML_SMT_MAX_CONFIGURATIONS` (`all <count>` for a
smaller bound), and the report says whether the selections are all of them or
were cut short.

## `%optimize` — what is best? (z3 only)

Each `objective` of an `analysis def` is improved the way the trade-study
definition typing it says, within the conditions the case requires:

```
%optimize SolverDemo::PowerBudget
```

```
✓ Analysis PowerBudget is optimized (z3, 8ms)
  maximize mostScience = `sciencePower`: 160
  SolverDemo::PowerBudget::drivePower = 60
  SolverDemo::PowerBudget::sciencePower = 160
```

`MassBudget` has a quantity-valued objective, reported with its unit:

```
%optimize SolverDemo::MassBudget
```

Under cvc5 the same `%optimize` is an error rather than a plain satisfiability
check presented as an optimum:

```
OPENSYSML_SMT=$(command -v cvc5) ./bin/sysml examples/solver-demo.sysml
```

```
error: the SMT solver does not implement optimization: cvc5 lacks `(maximize …)`/`(minimize …)`
with `(get-objectives)`, a solver extension: … install z3 or set OPENSYSML_SMT to it
```

## What the model states, and why

| Element | The query it answers |
| --- | --- |
| `Rover` | asserted constraints, each a `%check`/`%solve` target of its own |
| `PowerBudgetRequirement` | several conditions in one query |
| `OverbookedBudget` | conditions that conflict, for `%explain` |
| `rover1` | a satisfaction assertion, asserted of an object |
| `roverFamily` | interacting variation points, for `%configure` |
| `PowerBudget`, `MassBudget` | an objective, and a quantity-valued one |

Every command is documented in
[the REPL command reference](../docs/reference/repl-commands.md), which is
normative where this walkthrough and it differ.

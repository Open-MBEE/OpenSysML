# 13. Query, view and transformation outcomes

QVTS means Query, View, Transformation for SysML v2. OpenSysML's `DocumentQueries` library
provides model queries that return rows and projected properties. Its path and model failures
follow QVTS's distinction between an answer and an unanswerable step; see QVTS-Core
`qvts-core.md` §§3–5 for the specification. The implementation details and current behavior are
tracked in the [query-outcomes design note](../internals/design/query-outcomes.md).

## Three kinds of result

A query can return a **value**, including a valid empty collection. For example, selecting no
elements is a successful result with zero rows; it is not an error.

A **path error** means the query asks for a name, feature or metamodel path the model does not
provide. Its record names the failing step and carries a frontier: where resolution stopped, what
the model offers there, and sometimes the nearest spelling. A misspelled name and a missing
feature are failures, not empty selections.

A **model error** means the model violates a constraint on which the query step depends, such as
an ambiguous qualified name. It identifies the affected element and the constraint the step could
not satisfy.

For a step that visits several elements, one path failure fails the whole query. The record keeps
every failing element; a partial set of rows is not presented as a complete answer. `tolerate =
true` is an explicit alternative for `Project` and `WhereFeature`: `Project` can keep an empty
cell, while `WhereFeature` can drop the element it cannot compare. The current output does not
mark which cells or rows were tolerated; see [Current limits](#current-limits).

## Run a query

Declare a `calc def` specializing `DocumentQueries::Query`, import the library, then name the
query with `-run-query`. The worked examples under
[`examples/qvts`](../../examples/qvts/README.md) use small Lander models and record each command's
output and exit status.

```console
$ bin/sysml examples/qvts/lander.sysml examples/qvts/queries.sysml -run-query QVTSDemo::CabinMass
```

```text
✓ package Lander
✓ package QVTSDemo
✓ Query QVTSDemo::CabinMass returned 1 row
  Columns: qualifiedName, mass
  Row 1: Lander::lander::cabin
    qualifiedName = "Lander::lander::cabin"
    mass = 320.0
```

`-json` writes the same check in machine-readable form. A failed query's outcome appears at
`checks[].outcome`; a successful check currently has no `outcome: value` field. The example below
is copied from the branch-built CLI recording for the missing-feature query:

```console
$ bin/sysml examples/qvts/lander.sysml examples/qvts/queries.sysml -run-query QVTSDemo::MissingMassProject -json
```

```json
{
  "status": "unresolved",
  "exit": 2,
  "checks": [
    {
      "subject": "QVTSDemo::MissingMassProject",
      "status": "unresolved",
      "values": null,
      "lines": [
        "error: query QVTSDemo::MissingMassProject reads feature mass of Lander::lander::cabinEngine, which has no such feature (also absent on Lander::lander::engineBay)",
        "  path error at Feature(mass): expected a feature named mass on the element",
        "    at Lander::lander::cabinEngine; features: []",
        "  path error at Feature(mass): expected a feature named mass on the element",
        "    at Lander::lander::engineBay; features: []"
      ],
      "outcome": {
        "failures": [
          {
            "step": "Feature(mass)",
            "expected": "a feature named mass on the element",
            "frontier": {
              "at": "Lander::lander::cabinEngine",
              "features": [],
              "nearest": null
            }
          },
          {
            "step": "Feature(mass)",
            "expected": "a feature named mass on the element",
            "frontier": {
              "at": "Lander::lander::engineBay",
              "features": [],
              "nearest": null
            }
          }
        ],
        "outcome": "pathError",
        "step": "Feature(mass)"
      },
      "rows": null
    }
  ],
  "diagnostics": null,
  "output": [
    "✓ package Lander",
    "✓ package QVTSDemo"
  ],
  "errors": null
}
```

The CLI uses exit status 2 when the query cannot be evaluated. See the recordings for the full
tables and error messages, including fan-out failures and the JSON model-error case.

## Run the same query in the REPL

The REPL command is `%run-query <qualified-query-name>`. Loading the model files first makes
their query definitions available in the session:

```console
$ printf '%s\n' '%run-query QVTSDemo::CabinMass' | bin/sysml examples/qvts/lander.sysml examples/qvts/queries.sysml
```

```text
loaded 2 files:
  examples/qvts/lander.sysml
  examples/qvts/queries.sysml
✓ package Lander
✓ package QVTSDemo
SysML v2 REPL — %help for commands, Ctrl-D to exit
✓ Query QVTSDemo::CabinMass returned 1 row
  Columns: qualifiedName, mass
  Row 1: Lander::lander::cabin
    qualifiedName = "Lander::lander::cabin"
    mass = 320.0
```

The result is reported as query rows with projected cells. The
[QVTS examples](../../examples/qvts/README.md#1-value-and-empty-selection) include both a nonempty selection and a
successful empty selection.

## Use the service

The gRPC `RunDocumentQuery` RPC returns typed columns and rows for a successful query. If a query
fails with a recorded path or model outcome, the failed RPC status carries a `QueryOutcome` error
detail. It contains the outcome kind and step, the path failures and their frontiers, or the model
findings and any path failures from the same fan-out.

No shipped client—Go (`client/opensysml`), Java, Julia, MATLAB, Node, Python, Rust, or the Jupyter
kernel—decodes this status detail today. Clients can read the status message, but currently do not
expose the structured `QueryOutcome` record as a client-level result. The wire message is defined
in `api/proto/sysml.proto`.

## Current limits

The native query surface has known gaps; the examples record current output rather than changing
query-engine behavior:

| Case | Current behavior |
|:--|:--|
| Unknown metaclass, property, relationship kind or direction | The command exits 2 with a message but no outcome record. |
| Several missing names in one `Named` call | Only the first missing name is reported. |
| `OrderBy(missing = "error")` | Reports “cannot evaluate” without an outcome record. |
| Successful `-json` query check | No `outcome: value` field is emitted. |
| Two values for a `[1]` feature | The current projection returns the first value; QVTS calls for a model error for multiplicity. |
| `tolerate = true` | Empty cells and dropped rows are not marked as tolerated (QVTS-Core §3.3). |

For the distinction between these outcomes and their wire shape, see the
[query-outcomes design note](../internals/design/query-outcomes.md). Return to the
[guide index](README.md).

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
cell, while `WhereFeature` can drop the element it cannot compare — for a feature the element
lacks and for a metamodel property OpenSysML does not derive yet alike. A property the metamodel
declares is never unknown, even when no row read carries it; only a name no metaclass declares and
no row has is. The current output does not
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

## Write the query inline

A query need not be authored in a file first. `-run-inline-query` takes the body of one — the
result expression a `calc def` specializing `DocumentQueries::Query` would hold — and runs it
through the same compiler and executor, so the rows, the JSON, the outcome record on failure
and the exit status are what the named form gives. This is `MassTable` without a file:

```console
$ bin/sysml examples/qvts/lander.sysml -run-inline-query 'Project(WhereType(Descendants(Named("Lander::lander")), "PartUsage", true), ("qualifiedName", "mass"))'
```

```text
✓ package Lander
✓ Inline query returned 3 rows
  Columns: qualifiedName, mass
  Row 1: Lander::lander::cabin
    qualifiedName = "Lander::lander::cabin"
    mass = 320.0
  Row 2: Lander::lander::engine
    qualifiedName = "Lander::lander::engine"
    mass = 145.0
  Row 3: Lander::lander::scienceBay
    qualifiedName = "Lander::lander::scienceBay"
    mass = (none)
```

The expression is planned in the loaded model's context: the `DocumentQueries` library is
imported, the model's own names are reachable (`Descendants(Lander::lander)` as well as
`Named("Lander::lander")`), and the objects `-instantiate` created stand behind `Objects`,
`Verdicts`, `States` and `Events`. Text that does not parse or plan is reported at its line and
column within the expression, with a caret, and is never a diagnostic of the model:

```console
$ bin/sysml examples/qvts/lander.sysml -run-inline-query 'Project(Named("Lander") ("qualifiedName"))'
```

```text
✓ package Lander
sysml: inline query 1:25: expected ')'
Project(Named("Lander") ("qualifiedName"))
                        ^
```

A path or model fault in the inline expression keeps its QVTS outcome record exactly as a named
query's does; it does not collapse to an empty table. `-run-query` and `-run-inline-query` may
repeat and interleave, and run in the order written.

## Run the same query in the REPL

The REPL command is `%run-query <qualified-query-name>`, or `%run-query inline <expression>`
for an expression written in place of a name (a query that is itself named `inline` is run
by its qualified name, or with its `<parameter>=<expression>` bindings, which no query
expression begins with). Loading the model files first makes their query definitions
available in the session:

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

The result is reported as query rows with projected cells. An inline expression is reported as
`Inline query` instead of the query's name, and its faults are positioned within the expression
as on the command line:

```console
$ printf '%s\n' '%run-query inline Project(WhereType(Descendants(Named("Lander::lander")), "PartUsage", true), ("qualifiedName", "mass"))' | bin/sysml examples/qvts/lander.sysml
```

```text
✓ package Lander
SysML v2 REPL — %help for commands, Ctrl-D to exit
✓ Inline query returned 3 rows
  Columns: qualifiedName, mass
  Row 1: Lander::lander::cabin
    qualifiedName = "Lander::lander::cabin"
    mass = 320.0
  Row 2: Lander::lander::engine
    qualifiedName = "Lander::lander::engine"
    mass = 145.0
  Row 3: Lander::lander::scienceBay
    qualifiedName = "Lander::lander::scienceBay"
    mass = (none)
```

The
[QVTS examples](../../examples/qvts/README.md#1-value-and-empty-selection) include both a nonempty selection and a
successful empty selection.

## Use the service

The gRPC `RunDocumentQuery` RPC returns typed columns and rows for a successful query, named in
`query_id` or written in `inline_query` (one or the other; a service advertising the
`inline_document_query` capability accepts the latter, and text that does not parse or plan is
`INVALID_ARGUMENT`, positioned within the expression). The MCP `run_document_query` tool takes
the same choice as `queryId` or `inlineQuery`. If a query
fails with a recorded path or model outcome, the failed RPC status carries a `QueryOutcome` error
detail, for an inline expression as for a named query. It contains the outcome kind and step, the path failures and their frontiers, or the model
findings and any path failures from the same fan-out.

The same engine answers the `Query` RPC: a SysML v2 API & Services `Query` or OSLC Query text
(`-query`, `%query`, `oslc_query`, the MCP `query` tool) is translated into a document-query plan —
`WhereType`, `WhereName`, `WhereFeature`, `WhereText`, `Project`, `OrderBy` over the model's
elements, or `Objects` over the held objects when `rdf:type=elmt:<id>` names an element — and run
by it, so those surfaces fail with the same `QueryOutcome` detail (an unknown property is a path
error naming the nearest spelling) and compare values by type as a `query def` does. See
[OSLC Query text](../reference/oslc-query.md) for the plan each construct becomes.

The Java client decodes this status detail: a failed `runDocumentQuery`, `query` or `queryOslc`
throws a `QueryOutcomeException` (a `ServiceException`) whose `outcome()` is the record above as a
`QueryOutcome` — kind, step, each failure's step, expectation and frontier, each finding's element,
constraint and observation (see [Java API](../reference/java-api.md#document-queries-and-rendering)).
The other shipped clients—Go (`client/opensysml`), Julia, MATLAB, Node, Python, Rust, and the
Jupyter kernel—read the status message but do not yet expose the structured `QueryOutcome` record
as a client-level result. The wire message is defined in `api/proto/sysml.proto`.

## Write the answer as a view

Every query surface can also write what it answered as a model element: one view typed by
`QVTS::QueryResult`, the shape the [QVTS](https://github.com/Open-MBEE/qvts) library gives a
query result. The library ships with OpenSysML (`OpenSysML Libraries/QVTS.sysml`), so the text
parses against any workspace.

```console
$ bin/sysml examples/qvts/lander.sysml examples/qvts/queries.sysml -run-query QVTSDemo::CabinMass -as-view -
```

```sysml
package CabinMassResult {
	private import QVTS::*;

	view cabinMass : QueryResult {
		expose Lander::lander::cabin;
		attribute :>> query = "QVTSDemo::CabinMass";
		attribute :>> source = "examples/qvts/lander.sysml examples/qvts/queries.sysml";
		attribute :>> outcome = Outcome::established;
		attribute :>> provenance = "sha256:110e03c52250fab7b72729f2c70e2cf09f2d1777290be6e30270b523b26e12dd";
	}
}
```

The view exposes each selected element once, by qualified name, in the order the rows came.
Projected cells are not copied into it: a cell is a value of the exposed element, which a
document over the view reads again. The four attributes are

| Attribute | Value |
|:--|:--|
| `query` | A named query's qualified name followed by its bindings; an inline query's expression; the SysML v2 API query or OSLC Query text given to `Query`. |
| `source` | The files the model was read from, as the surface knows them; the model hash over the service. |
| `outcome` | `established` for a value — an empty selection included, which exposes nothing — and `pathError` or `modelError` for a query the model has no answer for. |
| `provenance` | The hash of the model the query read, followed by `; step <step>` naming the step a failed query stopped at. |

A failed query has no selection, so its view exposes the elements the failure is about instead:
each frontier's `at` for a path error, each finding's element for a model error. A failure
about no element — an OSLC property no element type has — exposes nothing. The same model and
the same query write the same bytes.

| Surface | How to ask |
|:--|:--|
| `sysml -run-query`, `-run-inline-query`, `-query` | `-as-view <file>` writes the view to the file beside the usual report; `-as-view -` prints it in place of the rows. A failed query still writes its view and still exits 2. |
| REPL `%run-query`, `%query` | A leading `-as-view` (`%run-query -as-view QVTSDemo::CabinMass`, also before `inline …`) prints the view in place of the rows. |
| gRPC `RunDocumentQuery`, `Query` | `as_view` in the request fills `result_view.text` in the response; a failed query carries a `QueryResultView` status detail beside its `QueryOutcome`. Needs the `query_result_view` capability. |
| MCP `run_document_query`, `query` | `asView` fills `resultView` in the result; `render_model_view` takes `form` to render such a view as a document. |
| Go client | `RunDocumentQueryAsView`, `RunInlineDocumentQueryAsView`, `QueryAsView`, `QueryOSLCAsView`; `StatusError.ResultView` on a failure. |
| Python client | `as_view=True` on `run_document_query` and `query`; `.result_view` on the result and on the error. |
| Java client | `runDocumentQueryAsView`, `runInlineDocumentQueryAsView`, `queryAsView`, `queryOslcAsView`; `QueryOutcomeException.resultView()` on a failure. |
| Magic MDK | *Run query* writes the view beside the project; see [the plugin's README](https://github.com/Open-MBEE/OpenSysML/blob/develop/editors/mdk/README.md). |

A result view is a view like any other, so it renders once its text is part of the model:
`sysml … -render CabinMassResult::cabinMass` draws it as `RenderView` does, and
`-render-document CabinMassResult::cabinMass -doc-form markdown` (or `html`, `asciidoc`, `pdf`;
`form` on the `RenderView` RPC) writes it as a document, the four attributes as the header and
the exposed elements as the rows of one table. The RPC refuses a document form for a view that
is not a `QVTS::QueryResult`. `examples/qvts/expected/views/` records the view and each
rendered form for the queries of this guide, for an OSLC query and its unknown-property path
error, and for the QVTS repository's own `result.sysml`.

## Current limits

The native query surface has known gaps; the examples record current output rather than changing
query-engine behavior:

| Case | Current behavior |
|:--|:--|
| Unknown metaclass, relationship kind or direction | The command exits 2 with a message but no outcome record (an unknown property is a path error naming the nearest spelling). |
| Several missing names in one `Named` call | Only the first missing name is reported. |
| `OrderBy(missing = "error")` | Reports “cannot evaluate” without an outcome record. |
| Successful `-json` query check | No `outcome: value` field is emitted. |
| Two values for a `[1]` feature | The current projection returns the first value; QVTS calls for a model error for multiplicity. |
| `tolerate = true` | Empty cells and dropped rows are not marked as tolerated (QVTS-Core §3.3). |

For the distinction between these outcomes and their wire shape, see the
[query-outcomes design note](../internals/design/query-outcomes.md). Return to the
[guide index](README.md).

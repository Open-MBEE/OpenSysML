# QVTS examples

These examples use a small, self-authored Lander model and the native
`DocumentQueries` library. Run every command from the repository root after
`make build`. Each output block is a recording from the branch-built `bin/sysml`;
the JSON blocks include the complete `-json` response. The separate
`expected/` recordings and `tests/qvts` keep the commands, output, and exit
statuses in sync.

## 1. Value and empty selection

`CabinMass` selects one named element and projects its `mass`. `EmptySelection`
is also a value: no matching `StateUsage` elements means a valid table with
zero rows, not a path error.

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

Exit status: 0

The same query also runs in the REPL after loading both models:

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

Exit status: 0


```console
$ bin/sysml examples/qvts/lander.sysml examples/qvts/queries.sysml -run-query QVTSDemo::EmptySelection
```

```text
✓ package Lander
✓ package QVTSDemo
✓ Query QVTSDemo::EmptySelection returned 0 rows
  Columns: qualifiedName, mass
```

Exit status: 0


## 2. Name path errors

A misspelled qualified name fails at the `Named` step and reports the nearest
member. A path into the deliberately empty nested package has no frontier
members to offer.

```console
$ bin/sysml examples/qvts/lander.sysml examples/qvts/queries.sysml -run-query QVTSDemo::MisspelledName
```

```text
✓ package Lander
✓ package QVTSDemo
sysml: query QVTSDemo::MisspelledName names no element Lander::lander::cabni: Lander::lander has no member cabni — did you mean cabin?
  path error at Named(Lander::lander::cabni): expected exactly one element with that qualified name
    at Lander::lander; members: [cabin (PartUsage), cabinEngine (ConnectionUsage), engine (PartUsage), engineBay (ConnectionUsage), scienceBay (PartUsage)]; nearest: cabin
```

Exit status: 2


```console
$ bin/sysml examples/qvts/lander.sysml examples/qvts/queries.sysml -run-query QVTSDemo::EmptyPackageMember
```

```text
✓ package Lander
✓ package QVTSDemo
sysml: query QVTSDemo::EmptyPackageMember names no element Lander::Empty::nothing: Lander::Empty has no member nothing
  path error at Named(Lander::Empty::nothing): expected exactly one element with that qualified name
    at Lander::Empty; members: []
```

Exit status: 2


## 3. Missing-feature path errors and fan-out

`PartUsage` includes the two connection usages as well as the parts. Neither
connection has a `mass` feature. `Project` fails closed and reports both
missing-feature paths rather than returning a partial table. `WhereFeature`
uses the same source and also fails closed.

```console
$ bin/sysml examples/qvts/lander.sysml examples/qvts/queries.sysml -run-query QVTSDemo::MissingMassProject
```

```text
✓ package Lander
✓ package QVTSDemo
sysml: query QVTSDemo::MissingMassProject reads feature mass of Lander::lander::cabinEngine, which has no such feature (also absent on Lander::lander::engineBay)
  path error at Feature(mass): expected a feature named mass on the element
    at Lander::lander::cabinEngine; features: []
  path error at Feature(mass): expected a feature named mass on the element
    at Lander::lander::engineBay; features: []
```

Exit status: 2


```console
$ bin/sysml examples/qvts/lander.sysml examples/qvts/queries.sysml -run-query QVTSDemo::MissingMassFilter
```

```text
✓ package Lander
✓ package QVTSDemo
sysml: query QVTSDemo::MissingMassFilter reads feature mass of Lander::lander::cabinEngine, which has no such feature (also absent on Lander::lander::engineBay)
  path error at Feature(mass): expected a feature named mass on the element
    at Lander::lander::cabinEngine; features: []
  path error at Feature(mass): expected a feature named mass on the element
    at Lander::lander::engineBay; features: []
```

Exit status: 2


The machine-readable path-error response includes the failing step, all
failures, and the frontier at each missing feature.

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

Exit status: 2


## 4. An unvalued feature is not a missing feature

`scienceBay` declares `mass` through `Payload` but has no value for it. The
projection succeeds and displays `(none)`; compare this with the missing
feature on the connections in section 3.

```console
$ bin/sysml examples/qvts/lander.sysml examples/qvts/queries.sysml -run-query QVTSDemo::UnvaluedMass
```

```text
✓ package Lander
✓ package QVTSDemo
✓ Query QVTSDemo::UnvaluedMass returned 1 row
  Columns: qualifiedName, mass
  Row 1: Lander::lander::scienceBay
    qualifiedName = "Lander::lander::scienceBay"
    mass = (none)
```

Exit status: 0


## 5. Model error and current multiplicity behavior

Two parts named `repeated` make `Named` ambiguous, so the query reports a
`modelError`. The JSON response shows the structured outcome. The second part
has two same-named redefinitions of its `[1]` `mass`; current projection
returns the first value (`3.0`) rather than reporting a multiplicity model
error (QVTS-Core §3.2 describes the expected constraint).

```console
$ bin/sysml examples/qvts/lander.sysml examples/qvts/duplicates.sysml examples/qvts/queries.sysml -run-query QVTSDemo::AmbiguousPart
```

```text
examples/qvts/duplicates.sysml:8:10: warning[name-conflict]: Duplicate of other owned member name
    part repeated : Instrument;
         ^~~~~~~~
examples/qvts/duplicates.sysml:9:10: warning[name-conflict]: Duplicate of other owned member name
    part repeated : Instrument;
         ^~~~~~~~
examples/qvts/duplicates.sysml:12:9: warning[name-conflict]: Duplicate of other owned member name
        attribute :>> mass = 3.0;
        ^~~~~~~~~~~~~~~~~~~~~~~~~
examples/qvts/duplicates.sysml:13:9: warning[name-conflict]: Duplicate of other owned member name
        attribute :>> mass = 4.0;
        ^~~~~~~~~~~~~~~~~~~~~~~~~
✓ package Lander
✓ package Duplicates
✓ package QVTSDemo
sysml: query QVTSDemo::AmbiguousPart names 2 elements Duplicates::repeated: Duplicates::repeated (declared 2 times)
  model error at Named(Duplicates::repeated): Duplicates violates ambiguousName: Duplicates::repeated (declared 2 times)
```

Exit status: 2


```console
$ bin/sysml examples/qvts/lander.sysml examples/qvts/duplicates.sysml examples/qvts/queries.sysml -run-query QVTSDemo::AmbiguousPart -json
```

```json
{
  "status": "unresolved",
  "exit": 2,
  "checks": [
    {
      "subject": "QVTSDemo::AmbiguousPart",
      "status": "unresolved",
      "values": null,
      "lines": [
        "error: query QVTSDemo::AmbiguousPart names 2 elements Duplicates::repeated: Duplicates::repeated (declared 2 times)",
        "  model error at Named(Duplicates::repeated): Duplicates violates ambiguousName: Duplicates::repeated (declared 2 times)"
      ],
      "outcome": {
        "findings": [
          {
            "step": "Named(Duplicates::repeated)",
            "element": "Duplicates",
            "constraint": "ambiguousName",
            "observed": "Duplicates::repeated (declared 2 times)"
          }
        ],
        "outcome": "modelError",
        "pathFailures": [],
        "step": "Named(Duplicates::repeated)"
      },
      "rows": null
    }
  ],
  "diagnostics": [
    {
      "severity": "warning",
      "message": "Duplicate of other owned member name",
      "file": "examples/qvts/duplicates.sysml",
      "line": 8,
      "column": 10,
      "pass": "name-resolution",
      "code": "name-conflict"
    },
    {
      "severity": "warning",
      "message": "Duplicate of other owned member name",
      "file": "examples/qvts/duplicates.sysml",
      "line": 9,
      "column": 10,
      "pass": "name-resolution",
      "code": "name-conflict"
    },
    {
      "severity": "warning",
      "message": "Duplicate of other owned member name",
      "file": "examples/qvts/duplicates.sysml",
      "line": 12,
      "column": 9,
      "pass": "name-resolution",
      "code": "name-conflict"
    },
    {
      "severity": "warning",
      "message": "Duplicate of other owned member name",
      "file": "examples/qvts/duplicates.sysml",
      "line": 13,
      "column": 9,
      "pass": "name-resolution",
      "code": "name-conflict"
    }
  ],
  "output": [
    "examples/qvts/duplicates.sysml:8:10: warning[name-conflict]: Duplicate of other owned member name",
    "    part repeated : Instrument;",
    "         ^~~~~~~~",
    "examples/qvts/duplicates.sysml:9:10: warning[name-conflict]: Duplicate of other owned member name",
    "    part repeated : Instrument;",
    "         ^~~~~~~~",
    "examples/qvts/duplicates.sysml:12:9: warning[name-conflict]: Duplicate of other owned member name",
    "        attribute :\u003e\u003e mass = 3.0;",
    "        ^~~~~~~~~~~~~~~~~~~~~~~~~",
    "examples/qvts/duplicates.sysml:13:9: warning[name-conflict]: Duplicate of other owned member name",
    "        attribute :\u003e\u003e mass = 4.0;",
    "        ^~~~~~~~~~~~~~~~~~~~~~~~~",
    "✓ package Lander",
    "✓ package Duplicates",
    "✓ package QVTSDemo"
  ],
  "errors": null
}
```

Exit status: 2


```console
$ bin/sysml examples/qvts/lander.sysml examples/qvts/duplicates.sysml examples/qvts/queries.sysml -run-query QVTSDemo::DoubleMassValue
```

```text
examples/qvts/duplicates.sysml:8:10: warning[name-conflict]: Duplicate of other owned member name
    part repeated : Instrument;
         ^~~~~~~~
examples/qvts/duplicates.sysml:9:10: warning[name-conflict]: Duplicate of other owned member name
    part repeated : Instrument;
         ^~~~~~~~
examples/qvts/duplicates.sysml:12:9: warning[name-conflict]: Duplicate of other owned member name
        attribute :>> mass = 3.0;
        ^~~~~~~~~~~~~~~~~~~~~~~~~
examples/qvts/duplicates.sysml:13:9: warning[name-conflict]: Duplicate of other owned member name
        attribute :>> mass = 4.0;
        ^~~~~~~~~~~~~~~~~~~~~~~~~
✓ package Lander
✓ package Duplicates
✓ package QVTSDemo
✓ Query QVTSDemo::DoubleMassValue returned 1 row
  Columns: qualifiedName, mass
  Row 1: Duplicates::doubleMass
    qualifiedName = "Duplicates::doubleMass"
    mass = 3.0
```

Exit status: 0


## 6. Tolerate missing features

With `tolerate = true`, `Project` keeps a row and displays `(none)` for each
missing cell. `WhereFeature` silently drops elements whose `mass` is absent.
Current output does not mark either tolerated cells or dropped rows; QVTS-Core
§3.3 describes the expected marking.

```console
$ bin/sysml examples/qvts/lander.sysml examples/qvts/queries.sysml -run-query QVTSDemo::ToleratedMassProject
```

```text
✓ package Lander
✓ package QVTSDemo
✓ Query QVTSDemo::ToleratedMassProject returned 5 rows
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
  Row 4: Lander::lander::cabinEngine
    qualifiedName = "Lander::lander::cabinEngine"
    mass = (none)
  Row 5: Lander::lander::engineBay
    qualifiedName = "Lander::lander::engineBay"
    mass = (none)
```

Exit status: 0


```console
$ bin/sysml examples/qvts/lander.sysml examples/qvts/queries.sysml -run-query QVTSDemo::ToleratedMassFilter
```

```text
✓ package Lander
✓ package QVTSDemo
✓ Query QVTSDemo::ToleratedMassFilter returned 2 rows
  Row 1: Lander::lander::cabin
  Row 2: Lander::lander::engine
```

Exit status: 0


## 7. Views of query results

`MassTable` projects a result table, including the unvalued feature.
`OrderedMasses` uses `OrderBy(missing = "last")`. The JSON form includes the
table structure. The final command attempts a document render of the query
result through an existing `Table` view.

```console
$ bin/sysml examples/qvts/lander.sysml examples/qvts/queries.sysml -run-query QVTSDemo::MassTable
```

```text
✓ package Lander
✓ package QVTSDemo
✓ Query QVTSDemo::MassTable returned 3 rows
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

Exit status: 0


```console
$ bin/sysml examples/qvts/lander.sysml examples/qvts/queries.sysml -run-query QVTSDemo::OrderedMasses
```

```text
✓ package Lander
✓ package QVTSDemo
✓ Query QVTSDemo::OrderedMasses returned 3 rows
  Columns: qualifiedName, mass
  Row 1: Lander::lander::engine
    qualifiedName = "Lander::lander::engine"
    mass = 145.0
  Row 2: Lander::lander::cabin
    qualifiedName = "Lander::lander::cabin"
    mass = 320.0
  Row 3: Lander::lander::scienceBay
    qualifiedName = "Lander::lander::scienceBay"
    mass = (none)
```

Exit status: 0


```console
$ bin/sysml examples/qvts/lander.sysml examples/qvts/queries.sysml -run-query QVTSDemo::MassTable -json
```

```json
{
  "status": "holds",
  "exit": 0,
  "checks": [
    {
      "subject": "QVTSDemo::MassTable",
      "status": "holds",
      "values": [
        {
          "name": "rows",
          "value": "3"
        },
        {
          "name": "columns",
          "value": "qualifiedName, mass"
        }
      ],
      "lines": [
        "✓ Query QVTSDemo::MassTable returned 3 rows",
        "  Columns: qualifiedName, mass",
        "  Row 1: Lander::lander::cabin",
        "    qualifiedName = \"Lander::lander::cabin\"",
        "    mass = 320.0",
        "  Row 2: Lander::lander::engine",
        "    qualifiedName = \"Lander::lander::engine\"",
        "    mass = 145.0",
        "  Row 3: Lander::lander::scienceBay",
        "    qualifiedName = \"Lander::lander::scienceBay\"",
        "    mass = (none)"
      ],
      "rows": null,
      "table": {
        "columns": [
          "qualifiedName",
          "mass"
        ],
        "rows": [
          {
            "element": "Lander::lander::cabin",
            "cells": [
              [
                "\"Lander::lander::cabin\""
              ],
              [
                "320.0"
              ]
            ]
          },
          {
            "element": "Lander::lander::engine",
            "cells": [
              [
                "\"Lander::lander::engine\""
              ],
              [
                "145.0"
              ]
            ]
          },
          {
            "element": "Lander::lander::scienceBay",
            "cells": [
              [
                "\"Lander::lander::scienceBay\""
              ],
              []
            ]
          }
        ]
      }
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

Exit status: 0


```console
$ bin/sysml examples/qvts/lander.sysml examples/qvts/queries.sysml -render-document QVTSDemo::MassReport
```

```text
loaded 2 files:
  examples/qvts/lander.sysml
  examples/qvts/queries.sysml
✓ package Lander
✓ package QVTSDemo
# Lander Mass Report

*Mass values in the lander*

| qualifiedName | mass |
| --- | --- |
| Lander::lander::cabin | 320 |
| Lander::lander::engine | 145 |
| Lander::lander::scienceBay |  |
```

Exit status: 0


## 8. Reflection and comparison

`WhereType(exact = true)` selects exact `PartUsage` elements. The projection
reads metamodel properties; `Differ` compares two projections, while the
misspelled key reports a path error at `Differ` with a nearest suggestion.
The metamodel-table recording below pins only the `PartUsage` entry.

```console
$ bin/sysml examples/qvts/lander.sysml examples/qvts/queries.sysml -run-query QVTSDemo::ExactPartUsages
```

```text
✓ package Lander
✓ package QVTSDemo
✓ Query QVTSDemo::ExactPartUsages returned 3 rows
  Columns: qualifiedName
  Row 1: Lander::lander::cabin
    qualifiedName = "Lander::lander::cabin"
  Row 2: Lander::lander::engine
    qualifiedName = "Lander::lander::engine"
  Row 3: Lander::lander::scienceBay
    qualifiedName = "Lander::lander::scienceBay"
```

Exit status: 0


```console
$ bin/sysml examples/qvts/lander.sysml examples/qvts/queries.sysml -run-query QVTSDemo::MetamodelProperties
```

```text
✓ package Lander
✓ package QVTSDemo
✓ Query QVTSDemo::MetamodelProperties returned 3 rows
  Columns: qualifiedName, elementId, isComposite, declaredName
  Row 1: Lander::lander::cabin
    qualifiedName = "Lander::lander::cabin"
    elementId = "Lander__lander__cabin"
    isComposite = true
    declaredName = "cabin"
  Row 2: Lander::lander::engine
    qualifiedName = "Lander::lander::engine"
    elementId = "Lander__lander__engine"
    isComposite = true
    declaredName = "engine"
  Row 3: Lander::lander::scienceBay
    qualifiedName = "Lander::lander::scienceBay"
    elementId = "Lander__lander__scienceBay"
    isComposite = true
    declaredName = "scienceBay"
```

Exit status: 0


```console
$ bin/sysml examples/qvts/lander.sysml examples/qvts/queries.sysml -run-query QVTSDemo::ChangedMasses
```

```text
✓ package Lander
✓ package QVTSDemo
✓ Query QVTSDemo::ChangedMasses returned 2 rows
  Columns: elementId, left, right
  Row 1: Lander::lander::cabin
    elementId = "Lander__lander__cabin"
    left = 320.0
    right = 321.0
  Row 2: Lander::lander::engine
    elementId = "Lander__lander__engine"
    left = 145.0
    right = 146.0
```

Exit status: 0


```console
$ bin/sysml examples/qvts/lander.sysml examples/qvts/queries.sysml -run-query QVTSDemo::MisspelledDifferKey
```

```text
✓ package Lander
✓ package QVTSDemo
sysml: query QVTSDemo::MisspelledDifferKey operation differ: the left rows project no column elementIdd (columns: elementId, mass)
  path error at Differ(elementIdd, mass): expected a column named elementIdd on the left rows
    at left; nearest: elementId
```

Exit status: 2


```console
$ bin/sysml -metamodel-table
```

```text
    {
      "name": "PartUsage",
      "abstract": false,
      "parents": [
        "ItemUsage"
      ]
    }
```

Exit status: 0


## 9. RDF and SPARQL

The stable Turtle excerpt shows the cabin as a `sysml:PartUsage`, its
`sysml:qualifiedName`, and its `sysml:elementId`. The QVTS steps read the same
model facts that a SPARQL query over this export could read. OpenSysML does not
ship a SPARQL engine.

```console
$ bin/sysml examples/qvts/lander.sysml -convert ttl
```

```text
elmt:Lander__lander__cabin
    a sysml:PartUsage ;
    sysml:qualifiedName "Lander::lander::cabin" ;
    sysml:elementId "Lander__lander__cabin" ;
```

Exit status: 0

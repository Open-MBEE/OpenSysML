# Importing data into a model

`-import` on the command line, `%import` at the prompt, sets feature values
from a CSV, TSV, JSON or JSON Lines file: a spreadsheet export, a simulation's
result table or a script's JSON. Each row names an element and the values it
assigns; the import edits the model's own text, so comments and layout survive.

## A table of values

```sysml
package Vehicle {
    private import ScalarValues::*;
    private import ISQ::*;
    private import SI::*;
    part def Engine { attribute mass : MassValue default = 175 [kg]; }
    part car {
        attribute count : Integer = 3;
        attribute supplier : String;
        part engine : Engine;
    }
}
```

```csv
element,count,supplier,mass [kg]
Vehicle::car,4,"Acme, Inc.",
Vehicle::car::engine,,,180
```

```bash
$ sysml vehicle.sysml -import values.csv -convert sysml -o imported.sysml
✓ package Vehicle
✓ imported 3 values into 2 elements from values.csv
  Vehicle::car::count = 4
  Vehicle::car::supplier = "Acme, Inc."
  Vehicle::car::engine::mass = 180 [kg] (redefines Vehicle::Engine::mass)
wrote imported.sysml (sysml, 376 bytes)
```

- The `element` column names the element by qualified name; every other
  column names one of its features. The name may go through an alias, and
  through features the element inherits: `Vehicle::car1::engine` reaches the
  `engine` that `car1` gets from its definition `Car`, and an import there
  redefines `engine` in `car1` and sets the value inside it.
- A unit follows the column name in brackets, `mass [kg]`, and must measure
  what the feature does: `mass [m]` is refused.
- A feature the element declares gets its value replaced. A feature it
  inherits, as `engine` inherits `mass` from `Engine`, gets a redefinition:
  `attribute :>> mass = 180 [kg];`.
- An empty cell (or a JSON `null`) leaves that value as it was.
- A value is written as the feature's type reads it: a `String` is quoted, an
  `Integer` must be a whole number, a `Boolean` is `true` or `false`, and an
  enumeration value is named by its bare name (`high` for `Grade::high`).

A tab-separated file reads quotes as text: `5" bore` is the string `5" bore`.

A long table, one value per row, works too: columns `element`, `feature`,
`value` and optionally `unit`.

## JSON and JSON Lines

A JSON file is an array of objects, one per element; a JSON Lines file is one
object per line. Top-level fields name features, as columns do. A number or
Boolean may also come as a string (`"180"`), as scripts often write them:

```json
[{"element": "Vehicle::car", "count": 4, "supplier": "Acme, Inc."},
 {"element": "Vehicle::car::engine", "mass [kg]": 180}]
```

## A mapping file

When the data's names are not the model's, `-import-map` (`map <file>` at the
prompt) says which column or JSON Pointer path is which:

```json
{
  "records": "/results",
  "element": {"path": "/id", "prefix": "Vehicle::"},
  "features": {
    "mass":     {"path": "/perf/m", "unitPath": "/perf/unit"},
    "count":    {"column": "n"},
    "supplier": {"column": "vendor", "type": "string"}
  },
  "ignore": ["note"]
}
```

| Key | Meaning |
|-----|---------|
| `format` | `csv`, `tsv`, `json` or `jsonl`, when the file's extension does not say |
| `delimiter` | A one-character CSV delimiter, such as `";"` |
| `records` | The JSON Pointer to the array of records inside a JSON object |
| `element` | The column or path naming the element, and a `prefix` to qualify it |
| `features` | Per feature: its `column` or `path`; a `unit`, `unitColumn` or `unitPath`; a `type` (`string`, `boolean`, `integer`, `real`) to read the value as |
| `ignore` | Columns or fields that are not features |

Columns and fields the mapping does not name still map to features of the
same name. A misspelled key in the mapping file is an error.

`-import` repeats, and the files import in order. An `-import-map` or
`-import-format` belongs to the `-import` before it, so files of different
shapes go in one run:

```bash
sysml rover.sysml \
  -import mass-properties.csv \
  -import battery-test.json -import-map battery-map.json \
  -import motor-telemetry.jsonl \
  -convert sysml -o rover-imported.sysml
```

[`examples/data-import-demo`](../../examples/data-import-demo/README.md) walks
through that run, with the script that writes the files.

## What an import refuses

The whole file is read and checked before anything changes, and the edit is
applied as one: a row naming an element the model lacks, a column naming a
feature the element does not have, a value its type cannot hold, a unit of the
wrong dimension, the same value set twice, or an edit that leaves the model
with new errors refuses the import and leaves the model unchanged. The error
names the file, the line or record, and the column or field.

## Previewing

`-import-dry-run` (`dry-run` at the prompt) reads and checks the file, prints
the values it would set, and changes nothing:

```bash
$ sysml vehicle.sysml -import values.csv -import-dry-run
dry run: would import 3 values into 2 elements from values.csv; the model is unchanged
  ...
```

## At the prompt

```text
sysml> %import values.csv
sysml> %import results.json map results.map.json
sysml> %import values.txt format tsv dry-run
```

`%save` then writes the imported model. See the [CLI reference](../reference/cli.md#importing-data)
and the [REPL commands](../reference/repl-commands.md).

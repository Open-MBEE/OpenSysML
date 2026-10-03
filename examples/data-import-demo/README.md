# Data import demo: a script's results set the model's values

[`rover.sysml`](rover.sysml) is a two-rover fleet whose numbers are not in the
model yet: each `Rover` declares a `mass`, a `serial`, a `health`, a `battery`
with a `capacity` and a `driveMotor` with a `peakPower`, and computes

```sysml
attribute specificEnergy = battery.capacity / mass;
```

from them. The numbers come from outside, as they would from a simulation or
a test rig. [`tools/simulate.py`](tools/simulate.py) stands in for those
tools and writes four files, each a shape scripts commonly write, into
[`results/`](results/):

| File | Shape |
| --- | --- |
| [`mass-properties.csv`](results/mass-properties.csv) | A table, one row per rover, the unit in the header: `mass [kg]` |
| [`battery-test.json`](results/battery-test.json) | A report with the tool's own field names and nested values, read through [`battery-map.json`](battery-map.json) |
| [`motor-telemetry.jsonl`](results/motor-telemetry.jsonl) | JSON Lines, one object per line |
| [`overrides.tsv`](results/overrides.tsv) | A tab-separated long table: `element`, `feature`, `value`, `unit` |

The files are committed, so the demo runs without Python; running the script
rewrites them byte for byte:

```bash
make build-sysml                                 # writes bin/sysml
cd examples/data-import-demo
python3 tools/simulate.py                        # optional: regenerates results/
```

Before the import the model cannot answer the question it is for:

```bash
$ ../../bin/sysml rover.sysml -eval 'RoverDemo::scout.specificEnergy'
sysml: evaluation failed: feature value scout.specificEnergy: no value for feature battery.capacity
```

## Previewing

`-import-dry-run` reads and checks every file, prints what each would set and
changes nothing. Each `-import-map` belongs to the `-import` before it, so the
JSON report gets its mapping and the other files are read as they are:

```bash
$ ../../bin/sysml rover.sysml \
    -import results/mass-properties.csv \
    -import results/battery-test.json -import-map battery-map.json \
    -import results/motor-telemetry.jsonl \
    -import results/overrides.tsv \
    -import-dry-run
dry run: would import 5 values into 2 elements from results/mass-properties.csv; the model is unchanged
  RoverDemo::scout::serial = "S-001" (redefines RoverDemo::Rover::serial)
  RoverDemo::scout::mass = 182.4 [kg] (redefines RoverDemo::Rover::mass)
  RoverDemo::scout::qualified = true (redefines RoverDemo::Rover::qualified)
  RoverDemo::hauler::mass = 311.0 [kg] (redefines RoverDemo::Rover::mass)
  RoverDemo::hauler::qualified = false (redefines RoverDemo::Rover::qualified)
dry run: would import 4 values into 2 elements from results/battery-test.json; the model is unchanged
  RoverDemo::scout::battery (redefines RoverDemo::Rover::battery)
  RoverDemo::scout::battery::capacity = 4104000 [J] (redefines RoverDemo::Battery::capacity)
  RoverDemo::scout::battery::cells = 12 (redefines RoverDemo::Battery::cells)
  RoverDemo::hauler::battery (redefines RoverDemo::Rover::battery)
  RoverDemo::hauler::battery::capacity = 6660000 [J] (redefines RoverDemo::Battery::capacity)
  RoverDemo::hauler::battery::cells = 20 (redefines RoverDemo::Battery::cells)
dry run: would import 2 values into 2 elements from results/motor-telemetry.jsonl; the model is unchanged
  ...
dry run: would import 1 value into 1 element from results/overrides.tsv; the model is unchanged
  RoverDemo::hauler::health = RoverDemo::Health::degraded (redefines RoverDemo::Rover::health)
```

What the preview shows:

- **Units travel with the data.** `mass [kg]` in the CSV header, `"unit": "J"`
  beside the value in the JSON report and `peakPower [W]` as a JSON Lines key
  all become the unit the value is written in.
- **Values are read as the feature's type reads them.** Python's `True` is
  the Boolean `true`; `degraded` is `RoverDemo::Health::degraded`; `S-001`
  is quoted as a `String`.
- **Inherited features are redefined where they belong.** `scout` declares
  nothing; its `mass` comes from `Rover`, so the import redefines it in
  `scout`. `scout::battery` is a part `scout` inherits too, so the import
  redefines `battery` in `scout` and the capacity inside that.
- **An empty cell sets nothing.** `hauler`'s serial is empty in the CSV, so
  the model's `"H-002"` stays.

## Importing

The same flags with `-convert sysml -o` write the imported model;
`rover.sysml` itself is not changed:

```bash
$ ../../bin/sysml rover.sysml \
    -import results/mass-properties.csv \
    -import results/battery-test.json -import-map battery-map.json \
    -import results/motor-telemetry.jsonl \
    -import results/overrides.tsv \
    -convert sysml -o rover-imported.sysml
```

Each rover gains the values, in the source's own layout and comments:

```sysml
    part scout : Rover {
        attribute :>> serial = "S-001";
        attribute :>> mass = 182.4 [kg];
        attribute :>> qualified = true;
        part :>> battery {
            attribute :>> capacity = 4104000 [J];
            attribute :>> cells = 12;
        }
        part :>> driveMotor {
            attribute :>> peakPower = 1450.5 [W];
        }
    }
```

and the model now answers, through an alias as well as by name:

```bash
$ ../../bin/sysml rover-imported.sysml -validate \
    -eval 'RoverDemo::scout.specificEnergy' \
    -eval 'RoverDemo::hauler.health' \
    -eval 'RoverDemo::scout2.driveMotor.peakPower'
✓ rover-imported.sysml: no errors
✓ RoverDemo::scout.specificEnergy
  = 22500.0 [SI::'m²⋅s⁻²']
✓ RoverDemo::hauler.health
  = Health::degraded
✓ RoverDemo::scout2.driveMotor.peakPower
  = 1450.5 [W]
```

(22 500 J/kg, 6.25 Wh/kg.) Import the next simulation's output into
`rover.sysml` the same way; the values are replaced, not appended.

## The mapping file

The battery rig names its records `results`, the element `unit`, and nests
the energy with its unit. [`battery-map.json`](battery-map.json) says which
JSON Pointer is which, prefixes the rig's element names with the package and
drops the rig's `note`:

```json
{
  "records": "/results",
  "element": {"path": "/unit", "prefix": "RoverDemo::"},
  "features": {
    "capacity": {"path": "/energy/value", "unitPath": "/energy/unit"},
    "cells": {"path": "/cells"}
  },
  "ignore": ["note"]
}
```

## At the prompt

`%import` takes the same files, `map <file>` and `dry-run`, and `%save`
writes the result:

```text
$ ../../bin/sysml rover.sysml
sysml> %import results/mass-properties.csv
✓ imported 5 values into 2 elements from results/mass-properties.csv
  ...
sysml> %import results/battery-test.json map battery-map.json
✓ imported 4 values into 2 elements from results/battery-test.json
  ...
sysml> RoverDemo::scout.specificEnergy
...
  = 22500.0 [SI::'m²⋅s⁻²']
sysml> %save rover-imported.sysml
```

## What an import refuses

A file is checked whole before anything changes; one row the model refuses
imports nothing, and the error names the file, line and column:

```bash
$ printf 'element,mass [m]\nRoverDemo::scout,182.4\n' > wrong-unit.csv
$ ../../bin/sysml rover.sysml -import wrong-unit.csv -import-dry-run
error: wrong-unit.csv line 2, column mass [m]: 182.4 [m] is measured in dimension L, but mass is measured in dimension M
  the model was left unchanged

$ printf 'element,health\nRoverDemo::scout,broken\n' > bad-enum.csv
$ ../../bin/sysml rover.sysml -import bad-enum.csv -import-dry-run
error: bad-enum.csv line 2, column health: broken is not a value of RoverDemo::Health
  the model was left unchanged

$ printf 'element,mass [kg]\nRoverDemo::rover3,182.4\n' > missing.csv
$ ../../bin/sysml rover.sysml -import missing.csv -import-dry-run
error: missing.csv line 2: no element named RoverDemo::rover3: RoverDemo has no member named rover3
  the model was left unchanged
```

## The stripped-down binary

Import is part of the `sysml_prod` build, so every command above runs the
same with `bin/sysml-prod` in place of `bin/sysml`:

```bash
make build-prod                                  # writes bin/sysml-prod
```

## What this does not do yet

The import sets values on elements the model already has. Creating elements
from rows, and recording a script's results as an analysis run with
`@AnalysisRecords::RecordedRun` provenance as `-record-run` does, are planned.

See [Importing data into a model](../../docs/manual/importing-data.md) for
every format and mapping key.

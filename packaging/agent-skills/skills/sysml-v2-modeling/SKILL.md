---
name: sysml-v2-modeling
description: Write SysML v2 textual models that OpenSysML accepts and can execute — packages and imports, parts and attributes, calculations, actions with in/out parameters, state machines, constraints and requirements — plus the validate-fix loop and the mistakes that most often produce errors. Use when creating or editing .sysml files.
---

# Writing SysML v2 models for OpenSysML

OpenSysML implements the OMG SysML v2 and KerML textual notation and executes calculations, actions
and state machines. Write a model, run `sysml -validate model.sysml`, fix every error it reports, and
only then evaluate or run anything (see the `opensysml-cli` skill). The handbook in the OpenSysML
repository (`docs/guide/`) has worked examples of every construct below.

## A complete, valid model

```sysml
package Demo {
    private import ScalarValues::*;     // Real, Integer, Boolean, String
    private import SI::*;               // units: m, s, kg, ...

    part def Wheel {
        attribute diameter : Real = 16.0;
    }
    part def Vehicle {
        attribute mass : Real default = 1500.0;     // `default` lets a usage override it
        attribute topSpeed : SpeedValue = 50 [m/s];
        part wheels : Wheel[4];
        constraint massOk { mass < 2000.0 }
    }
    part car : Vehicle;
    part truck : Vehicle { attribute :>> mass = 2500.0; }   // `:>>` redefines

    calc def Margin {
        in reading : Real;
        in threshold : Real;
        return : Real = threshold - reading;
    }

    action def Double {
        in x : Integer default = 21;
        out y : Integer;
        first start;
        then action compute { assign y := x * 2; }
        then done;
    }

    requirement def LightEnough {
        subject v : Vehicle;
        require constraint { v.mass < 2000.0 }
    }
    requirement light : LightEnough;
    part checks {
        assert satisfy light by car;
        assert satisfy light by truck;
    }

    state def Lamp {
        entry; then off;
        state off;
        accept after 10 [s] then on;    // leaves the state declared just before it
        state on;
    }
}
```

With this file saved as `model.sysml`, `sysml -calc "Demo::Margin(20.0, 100.0)" model.sysml` gives
`80.0`, `sysml -action Demo::Double model.sysml` reports `y = 42`, `sysml -satisfy model.sysml`
reports `car` holds and `truck` fails (exit 1), and `sysml -state Demo::Lamp -advance 15 model.sysml`
ends in `on`.

## Rules that matter

- **Imports are explicit and per file.** `Real`, `Integer`, `String` and `Boolean` need
  `private import ScalarValues::*;`; units need `private import SI::*;`. An unresolved name is an
  error, and the diagnostic suggests the import. A root-level import serves only its own file.
- **`=` binds, `default =` defaults.** A value written with `=` is fixed for every feature that
  redefines it, so overriding it later (`in x = 5;` in a subaction, `:>> mass = ...` in a usage) is an
  error. Write `default =` wherever a value must be overridable.
- **Results belong to calculations.** `return` declares a calculation's (or constraint's) result; in
  an action write `out`. An action that specializes a calc or case inherits its result as an output.
- **Actions are token flows.** Start with `first start;`, chain with `then action ...;`, finish with
  `then done;`. Assign with `assign y := expr;`. A performed subaction (`then action d : Double { in
  x = 5; }`, with `x` declared `default`) exposes its outputs to later steps as `d.y`. Branch with
  `then decide; if cond then a; else b;` and give each branch target its own `then done;`.
- **Do not declare a state named `start` or `done`;** every state machine inherits them. Write
  `entry; then <first-state>;` for the initial transition.
- **Sequences are `(1, 2, 3)`,** not `[1, 2, 3]`; brackets hold units (`50 [m/s]`) and
  multiplicities (`Wheel[4]`).
- **Qualify names from outside a package:** `Demo::Vehicle::mass`, `Demo::car.mass`.
- **A feature with no value is not zero.** Evaluated at model level it is `<undetermined>`; on an
  object it is `<unset>`. Give a value or `default` to anything a check reads.
- **Several files form one model** when named together on the command line; each file still imports
  what it uses.
- **Portability:** OpenSysML accepts a few notations of its own (for example the `choice`,
  `junction` and `history` pseudostates) with a warning naming the standard spelling. If another
  SysML v2 tool must read the model, use the standard spelling and check with `sysml -strict -validate`.

## The fix loop

1. `sysml -validate model.sysml`. Each diagnostic is `file:line:col: severity: message` with a caret
   under the location, and most messages say what to write instead.
2. Fix the first error first; later errors are often consequences of it.
3. Repeat until it prints `<file>: no errors` and exits 0. Warnings do not block, but read them.
4. Then run what you need (`-e`, `-calc`, `-action`, `-state`, `-constraint`, `-requirement`,
   `-satisfy`) and check values against what the model should produce.

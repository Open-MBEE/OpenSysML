# `sysml` in fifteen minutes — a live demo script

A single page of runnable material for showing OpenSysML to someone who has never seen it: each
section is a model you can paste, a command you can run, and the output it produces. Sections are
independent, so drop any of them to fit the time you have.

> **Every command and every output block below was run against `bin/sysml` and pasted verbatim**, so
> what you see on stage should match this page character for character — with two known exceptions:
> the `--version` banner in §0 carries your own build, and the attribute order inside a `%tokens` /
> `Results:` block in §7 varies between runs (see the note there).

## 0. Setup (one minute)

```bash
git clone https://github.com/Open-MBEE/OpenSysML.git && cd OpenSysML
make build-sysml          # -> ./bin/sysml
export PATH="$PWD/bin:$PATH"
```

macOS listeners can instead run `brew install Open-MBEE/tap/opensysml`.

Show the audience what is running — the version line carries the commit:

```console
$ sysml --version
sysml <commit>
  Commit:     <commit>
  Build time: <build time>
  Go version: go1.25.0
```

All demo models live in one scratch directory:

```bash
mkdir -p ~/sysml-demo && cd ~/sysml-demo
```

---

## 1. It is a headless evaluator, not only a REPL

`-e` evaluates and exits, so `sysml` drops straight into a shell pipeline or a CI job. Flags come
**before** file arguments.

```console
$ sysml -e "3 * (2 + 4)"
✓ 3 * (2 + 4)
  = 18
```

Repeat `-e` to evaluate several expressions against the same loaded model (used in §3).

---

## 2. The demo model

Paste this once; §3–§5 all use it.

```bash
cat > rover.sysml <<'EOF'
package Rover {
    private import ScalarValues::*;

    part def Wheel {
        attribute diameter : Real = 0.25;
        attribute mass : Real = 1.2;
    }

    part def Battery {
        attribute capacityWh : Real = 450.0;
        attribute charge : Real = 450.0;
    }

    part def Rover {
        part wheels : Wheel[6];
        part battery : Battery {
            :>> capacityWh = 600.0;      // redefine the nested default
        }
        attribute dryMass : Real = 180.0;
    }

    calc rangeKm {
        in charge : Real;
        in wattHoursPerKm : Real;
        charge / wattHoursPerKm
    }

    constraint MassBudget {
        180.0 <= 200.0
    }

    requirement PowerMargin {
        assume constraint { 600.0 > 0.0 }
        require constraint { 600.0 >= 450.0 }
    }
}
EOF
```

Evaluating the model's own `calc` from the command line — a model as a callable artifact:

```console
$ sysml -e "Rover::rangeKm(600, 12)" rover.sysml
✓ package Rover
✓ Rover::rangeKm(600, 12)
  = 50
```

---

## 3. Instantiate the structure and inspect it

The model is *materialized*: multiplicities are expanded into real objects, defaults are propagated,
and a nested redefinition wins over the definition's default (`capacityWh = 600.0`, not `450.0`).

```console
$ sysml rover.sysml
✓ package Rover
SysML v2 REPL — %help for commands, Ctrl-D to exit
sysml> %instantiate Rover::Rover
✓ Created instance of Rover::Rover
  ID: 1
  Use %features Rover::Rover to inspect

sysml> %features Rover::Rover
Instance: Rover::Rover (ID: 1)
Features:
  wheels = [Instance(ID: 2), Instance(ID: 3), Instance(ID: 4), Instance(ID: 5), Instance(ID: 6), Instance(ID: 7)]
    diameter = 0.25
    mass = 1.2
    diameter = 0.25
    mass = 1.2
    diameter = 0.25
    mass = 1.2
    diameter = 0.25
    mass = 1.2
    diameter = 0.25
    mass = 1.2
    diameter = 0.25
    mass = 1.2
  battery = Instance(ID: 8)
    capacityWh = 600.0
    charge = 450.0
  dryMass = 180.0

sysml> %instances
Instances:
  Rover::Rover (ID: 1)
```

`part wheels : Wheel[6]` became six objects with their own feature values — nobody had to write them out.

---

## 4. Analysis: calc, constraint, requirement, satisfy

```console
sysml> %calc Rover::rangeKm 600 12
✓ Rover::rangeKm(600, 12)
  = 50

sysml> %constraint Rover::MassBudget
✓ Constraint Rover::MassBudget passed

sysml> %requirement Rover::PowerMargin
✓ Requirement Rover::PowerMargin satisfied
```

A failure is not a bare `false` — it names the assertion that decided the verdict:

```console
sysml> constraint TooHeavy { 210.0 <= 200.0 }
✓ constraint TooHeavy

sysml> %constraint TooHeavy
✗ Constraint TooHeavy failed
  Assertion evaluated to false: 210.0 <= 200.0
```

### Requirements against real subjects (`%satisfy`)

This is the strongest part of the analysis story: one requirement, two candidate parts, and the
binary decides each `satisfy` assertion in the model.

```bash
cat > landing.sysml <<'EOF'
package Landing {
    part def Lander {
        attribute verticalSpeed;
    }

    requirement def TouchdownRequirement {
        subject lander : Lander;
        attribute maxVerticalSpeed;
        require constraint {
            lander.verticalSpeed <= maxVerticalSpeed
        }
    }

    requirement touchdown : TouchdownRequirement {
        attribute :>> maxVerticalSpeed = 1.5;
    }

    part slowLander : Lander { attribute :>> verticalSpeed = 1.2; }
    part fastLander : Lander { attribute :>> verticalSpeed = 2.4; }

    part analysisContext {
        assert satisfy touchdown by slowLander;
        assert satisfy touchdown by fastLander;
        assert not satisfy touchdown by fastLander;
    }
}
EOF
```

```console
$ sysml landing.sysml
✓ package Landing
SysML v2 REPL — %help for commands, Ctrl-D to exit
sysml> %satisfy
✓ satisfy touchdown by slowLander holds (on Landing::slowLander ID: 1)
✗ satisfy touchdown by fastLander fails (on Landing::fastLander ID: 2)
  Required condition evaluated to false: lander.verticalSpeed <= maxVerticalSpeed
✓ not satisfy touchdown by fastLander holds (on Landing::fastLander ID: 2)
```

The middle line is *meant* to fail — `fastLander` touches down at 2.4 against a 1.5 limit — and the
third line shows the negated assertion holding for the same object.

---

## 5. Units are checked, not decoration

```bash
cat > units.sysml <<'EOF'
package Units {
    private import ISQ::*;
    private import SI::*;

    attribute wheelbase : LengthValue = 1.2 [m];
    attribute clearance : LengthValue = 30.0 [cm];
    attribute driveTime : TimeValue = 90.0 [s];
}
EOF
```

```console
$ sysml units.sysml
✓ package Units
SysML v2 REPL — %help for commands, Ctrl-D to exit
sysml> %eval clearance
✓ clearance
  = 30.0 [cm]

sysml> %eval wheelbase - clearance
✓ wheelbase - clearance
  = 0.9 [m]

sysml> %eval wheelbase > clearance
✓ wheelbase > clearance
  = true

sysml> %eval wheelbase > driveTime
error: evaluation failed: incommensurable units: cannot express s (SI::second) in m (SI::metre)
```

Metres and centimetres are converted before comparison; metres against seconds is refused. The
`ISQ`/`SI` libraries are the bundled OMG standard library — nothing extra to install.

---

## 6. Diagnostics point at the character

```bash
cat > typo.sysml <<'EOF'
package Typo {
    part def Sensor {
        attribute reading = 0.0;
    }
    part def Probe {
        part s : Snesor;
    }
}
EOF
```

```console
$ sysml -e "1" typo.sysml
6:18: error: unresolved reference: Snesor
        part s : Snesor;
                 ^~~~~~
✓ 1
  = 1
```

The caret span covers the whole offending name, and evaluation continues — a broken reference does
not take the session down. `-debug` adds the pass that produced each diagnostic:

```bash
sed 's/reading = 0.0/reading : Real = /' typo.sysml > broken.sysml
```

```console
$ sysml -debug -e "1" broken.sysml
[debug] submission at buffer line 1; 1 diagnostic(s) over the whole buffer
3:36: error: expected an expression [syntax/syntax]
        attribute reading : Real = ;
                                   ^
✓ package Typo
✓ 1
  = 1
```

The `✓ package Typo` line is worth pausing on: the parser recovered and still produced the package,
which is why the REPL keeps going after an error.

Only the *syntax* error is reported even though `Snesor` is still misspelled: validation is tiered,
so name resolution is not run on a file that does not parse.

---

## 7. Behavior, live: stepping an action

This is the moment that usually lands — a SysML action being debugged like code, with breakpoints.

```bash
cat > mission.sysml <<'EOF'
package Mission {
    action drive {
        attribute metersDriven = 0;
        attribute samples = 0;

        first start;
        action rollForward { assign metersDriven := metersDriven + 10; }
        action takeSample  { assign samples := samples + 1; }
        done;

        succession first start then rollForward;
        succession first rollForward then takeSample;
        succession first takeSample then done;
    }

    state rover {
        entry; then idle;
        state idle;
        accept after 5 [SI::s] then driving;
        state driving;
        accept after 10 [SI::s] then charging;
        state charging;
        accept after 20 [SI::s] then idle;
    }
}
EOF
```

```console
$ sysml mission.sysml
✓ package Mission
SysML v2 REPL — %help for commands, Ctrl-D to exit
sysml> %action Mission::drive
✓ Started action executor for "Mission::drive"
  State: Running
  Tokens: 1

Use %step to advance, %tokens to inspect, %continue to run to completion

sysml> %step
✓ Step complete
  State: Running
  Tokens: 1

sysml> %tokens
Active tokens (1):
  Token 1 @ rollForward
    metersDriven = 0
    samples = 0

sysml> %break takeSample
✓ Breakpoint set at node "takeSample"
  %continue runs until a token reaches it

sysml> %continue
⏸ Paused at breakpoint "takeSample"
  State: Suspended
  Tokens: 1

Use %tokens to inspect, %step or %continue to resume

sysml> %tokens
Active tokens (1):
  Token 1 @ takeSample
    metersDriven = 10
    samples = 0

sysml> %continue
✓ Action completed
  Final state: Completed
  Results:
    metersDriven = 10
    samples = 1
```

Point out the token position (`@ rollForward`, then `@ takeSample`) and the data travelling with it:
`metersDriven` is `0` before the assignment ran and `10` at the breakpoint.

The two attributes inside a `%tokens` or `Results:` block may print in either order from run to run —
read the values, not the line order. (`%features` in §3 is stable.)

---

## 8. State machines run on simulated time

Same file. `%advance` moves the clock and drains every event that falls due.

```console
sysml> %state Mission::rover
✓ Started state machine executor for "Mission::rover"
  Current state: idle
  Time: 0.0
  Events: 1

Use %events to see queue, %current for state, %advance <time> to step

sysml> %events
Event queue: 1 events
Use %advance <time> to process next event

sysml> %advance 5
✓ Advanced to 5.0 (1 event(s) processed)
  Current state: driving
  Last event at: 5.0
  Remaining events: 1

sysml> %advance 10
✓ Advanced to 15.0 (1 event(s) processed)
  Current state: charging
  Last event at: 15.0
  Remaining events: 1

sysml> %current
Current state: charging
Time: 15.0
Last event at: 15.0
Execution state: Running
```

No wall-clock waiting: 15 simulated seconds of a rover mission pass instantly, and the machine
re-queues the next timer event as it goes.

---

## 9. `%trace` explains a result instead of asserting it

```console
$ sysml rover.sysml
✓ package Rover
SysML v2 REPL — %help for commands, Ctrl-D to exit
sysml> %trace on
trace: on

sysml> %calc Rover::rangeKm 600 12
[trace] eval literal 600 -> 600
[trace] eval literal 12 -> 12
[trace] enter calc Rover::rangeKm
[trace]   bind charge = 600 [argument]
[trace]   bind wattHoursPerKm = 12 [argument]
[trace]   stmt return
[trace]       eval feature charge -> 600
[trace]       eval feature wattHoursPerKm -> 12
[trace]     eval operator / -> 50
[trace] exit calc Rover::rangeKm -> 50
✓ Rover::rangeKm(600, 12)
  = 50
```

Argument binding, each sub-expression and the operator that produced the answer are all visible.
`sysml -trace model.sysml` turns it on from the command line.

---

## 10. RDF interop: notation ⇄ Turtle, round-trip

For the "does it talk to the rest of the toolchain?" question. Use a structural model:

```bash
cat > structure.sysml <<'EOF'
package Structure {
    private import ScalarValues::*;

    part def Wheel {
        attribute diameter : Real = 0.25;
    }

    part def Rover {
        part wheels : Wheel[6];
        attribute dryMass : Real = 180.0;
    }
}
EOF
```

```console
$ sysml -convert structure.sysml -o structure.ttl
wrote structure.ttl (ttl, 2254 bytes)

$ head -14 structure.ttl
@prefix elmt: <urn:sysmlv2:element:> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix sysx: <urn:opensysml:sysml:> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

elmt:Structure
    a sysml:Package ;
    sysml:qualifiedName "Structure" ;
    sysx:memberIndex "0"^^xsd:integer ;
    sysml:declaredName "Structure" ;
    sysx:hasBody "true"^^xsd:boolean .

<urn:sysmlv2:element:Structure::@0>
```

And back — this direction is a real print of the parsed tree, so it is the canonical notation for the
model that was loaded:

```console
$ sysml -convert structure.ttl -to sysml
package Structure {
    private import ScalarValues::*;
    part def Wheel {
        attribute diameter : Real = 0.25;
    }
    part def Rover {
        part wheels : Wheel[6];
        attribute dryMass : Real = 180.0;
    }
}
```

`%save model.ttl` does the same from inside the REPL; the format always follows the extension.

---

## 11. A runaway is reported, never a hang

Every run carries budgets, so an accidental infinite model fails with an actionable message instead
of freezing the demo.

```console
sysml> %budget
budgets (each bounds one run, not the session):
  evaluation steps     10000000   OPENSYSML_MAX_STEPS
  action steps         1000000    OPENSYSML_MAX_ACTION_STEPS
  state events         1000000    OPENSYSML_MAX_EVENTS
  do action steps      5000000    OPENSYSML_MAX_DO_STEPS
  collection elements  1000000    OPENSYSML_MAX_ELEMENTS

sysml> %eval 1..2000000
error: evaluation failed: collection element limit exceeded (1000000 elements; raise OPENSYSML_MAX_ELEMENTS to allow more)
```

Each budget bounds **one** run — one `%eval`, one action, one state machine — and names the variable
that raises it.

---

## 12. Scripting the whole thing

The REPL reads a script on stdin, which is how you rehearse a demo or wire a model check into CI:

```bash
printf '%%load rover.sysml\n%%instantiate Rover::Rover\n%%features Rover::Rover\n%%constraint Rover::MassBudget\n%%quit\n' | sysml
```

`%save session.sysml` writes the session's model back out (atomically, comments preserved); `%list`
echoes it, `%clear` resets it, and `%verbosity quiet|normal|debug` sets how much is reported.

---

## Presenter notes

- **Flags before files**: `sysml -e "x" model.sysml`, never `sysml model.sysml -e "x"`.
- **A blank line submits.** While typing a multi-line declaration in a brace continuation (`...>`),
  do not leave an empty line in the middle of it.
- **A declaration only drops what it changed.** An object and an in-progress `%action`/`%state`
  session survive a submission that does not touch what they were built from; redeclaring the
  namespace they came from drops them, with a note naming the submission that ended them.
- **`clear` is not a REPL command.** At the `sysml> ` prompt it is parsed as SysML and leaves an
  unresolved session error that is then reported under later commands. `%clear` is the command.
- **`-convert … -to ttl` covers structure and the behavior a body states.** A model with a
  construct the notation could not be rebuilt from — a name declared twice in one body, a
  `perform` with neither a name nor an action to refer to — is rejected (`cannot convert the … at …`) rather
  than silently exported, so use a model that converts for the RDF demo, as in §10.
- **Library types need an import.** `Real` comes from `ScalarValues`, quantities from `ISQ`/`SI`.
- `%help` lists every meta-command if a question goes somewhere unplanned.

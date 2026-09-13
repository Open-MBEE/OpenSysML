# 2. Your first model

In this chapter you declare a part, give it values, instantiate it and inspect the result, first
at the interactive prompt and then from a file. Everything shown is the same notation you would
write in a `.sysml` file; the REPL just shows you the result sooner.

## At the prompt

Launch the interactive REPL:

```bash
$ sysml
SysML v2 REPL — %help for commands, Ctrl-D to exit
sysml> 
```

### Define a Simple Part

Library types such as `Real` are not in scope automatically. Import them exactly as a `.sysml`
file would:

```sysml
sysml> private import ScalarValues::*;
✓ import ScalarValues::*

sysml> part def Wheel {
  ...>     attribute diameter : Real = 16.0;
  ...>     attribute width : Real = 7.5;
  ...> }
✓ part def Wheel
```

Each accepted declaration is echoed back as `✓ <kind> <name>`. An opening brace starts a
continuation prompt (`...>`) that lasts until the matching closing brace. A **blank line ends the
submission**, so don't put one inside a declaration you are typing.

Typing a namespace again **adds to** the one already in the session: submitting
`package P { part def B; }` after `package P { part def A; }` leaves both declared. An empty body
(`package P { }`) clears the namespace. Whenever a submission drops something, a `note:`
line says what: the members no longer declared, the instances that became invalid (their IDs
restart with the new model), and any `%action` or `%state` debugging session that was ended. A
debugging session over a declaration the submission did not touch keeps running.

### Define a Vehicle

```sysml
sysml> part def Vehicle {
  ...>     attribute mass : Real = 1500.0;
  ...>     part wheels : Wheel[4];
  ...> }
✓ part def Vehicle
```

### Instantiate and Inspect

```sysml
sysml> %instantiate Vehicle
✓ Created instance of Vehicle
  ID: 1
  Use %features Vehicle to inspect

sysml> %features Vehicle
Instance: Vehicle (ID: 1)
Features:
  mass = 1500.0
  wheels = [Instance(ID: 2), Instance(ID: 3), Instance(ID: 4), Instance(ID: 5)]
    diameter = 16.0
    width = 7.5
    ownedPorts = []
    performedActions = []
    ownedActions = []
    exhibitedStates = []
    ownedStates = []
    shape = []
    envelopingShapes = []
    boundingShapes = []
    voids = []
    isSolid = true
    subitems = []
    subparts = []
    checkedConstraints = []
    diameter = 16.0
    width = 7.5
…
  ownedPorts = []
  performedActions = []
  ownedActions = []
  exhibitedStates = []
  ownedStates = []
  shape = []
  envelopingShapes = []
  boundingShapes = []
  voids = []
  isSolid = true
  subitems = []
  subparts = []
  checkedConstraints = []

sysml> %instances
Instances:
  Vehicle (ID: 1)
```

After the features the model declares come the ones every part carries from the standard
library — `ownedPorts`, `subparts`, `isSolid` and the rest that `Parts::Part` and
`Items::Item` declare — with their defaults; each wheel lists them too (the `…` above stands
for the three wheels that repeat the first). A listing this size is bounded: `%features
Vehicle depth 0` names the composite features without expanding them, and `%features Vehicle
all` reads the whole tree out ([the REPL command reference](../reference/repl-commands.md)).

### Evaluate Expressions

```sysml
sysml> attribute wheelCount = 4;
✓ attribute wheelCount

sysml> attribute totalDiameter = wheelCount * 16.0;
✓ attribute totalDiameter

sysml> %eval totalDiameter
✓ totalDiameter
  = 64.0
```

---

## From a file

Create a file `my_model.sysml`:

```sysml
package MyModel {
    part def Sensor {
        attribute reading = 0.0;
        attribute threshold = 100.0;
    }

    part def System {
        part sensors : Sensor[3];
    }
}
```

Load the file in the REPL. `%load` submits the file's contents as if you had typed them,
so it reports the same `✓` lines. `%list` prints everything the session currently holds:

```bash
$ sysml
sysml> %load my_model.sysml
✓ package MyModel

sysml> %list
package MyModel {
    part def Sensor {
        attribute reading = 0.0;
        attribute threshold = 100.0;
    }

    part def System {
        part sensors : Sensor[3];
    }
}

sysml> %instantiate MyModel::System
✓ Created instance of MyModel::System
  ID: 1
  Use %features MyModel::System to inspect

sysml> %features MyModel::System
Instance: MyModel::System (ID: 1)
Features:
  sensors = [Instance(ID: 2), Instance(ID: 3), Instance(ID: 4)]
    reading = 0.0
    threshold = 100.0
    ownedPorts = []
…
    checkedConstraints = []
  ownedPorts = []
…
  checkedConstraints = []
```

A composite feature lists the features of each of its objects beneath it, in order — the
model's own first, then the library's, as above.

---

Next: [3. From the command line](03-command-line.md), which runs these same checks without
a prompt. The prompt itself is covered in [4. The REPL](04-repl.md).

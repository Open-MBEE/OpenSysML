# 4. The REPL

Running `sysml` without a model starts an interactive prompt. Each thing you type is a declaration, an
expression or a meta-command beginning with `%`. Declarations accumulate into a session model,
and that is the model every command reports on.

```
$ sysml
SysML v2 REPL — %help for commands, Ctrl-D to exit
sysml> private import ScalarValues::*;
✓ import ScalarValues::*
sysml> package Demo { part def Wheel { attribute diameter : Real = 16.0; } }
✓ package Demo
sysml> %eval Demo::Wheel::diameter
✓ Demo::Wheel::diameter
  = 16.0
```

The import is what makes `Real` resolvable, as described in [chapter 2](02-first-model.md). If a
session uses a library type without importing it, the declaration is rejected with
`unresolved reference: Real`.

## What a submission does

A declaration is parsed and validated when you submit it, and the result comes back immediately:
either `✓` with the declared name, or a set of diagnostics. An unfinished declaration continues
on the next line (`  ...>`) until its braces close.

Two things about the session model are worth understanding before you start a long session:

- **A submission adds to whatever namespace is already in the session.** Entering
  `package Demo { … }` again with an extra member merges into the existing declaration
  (`note: added to the existing package Demo (its other members are kept)`) rather than
  replacing its body. Entering an existing member again does replace that member, and the REPL
  says so. An empty body (`package Demo { }`) empties the namespace.
- **A submission invalidates only what it changed.** Objects created with `%instantiate` survive
  as long as the declarations they were built from are untouched, and an `%action` or
  `%state` debugging session over an unaffected declaration keeps running. A surviving object
  keeps its identity but not its execution state: the behaviors its type exhibits or performs
  restart from their initial states. Everything dropped or restarted is reported on a `note:`
  line. A surviving object is the one object every command reads: what an action wrote on it
  before the submission is what `%features` lists, what `%eval` answers for the feature or for
  a member reached through it, and what a debugger session still running writes to afterwards.
  A submission that *does* change the object's declaration drops it, and every command then
  reports the loss rather than answering from a fresh object.

`%list` shows the session's declarations, `%clear` resets the session, and `%save` writes it out
([chapter 7](07-saving-and-rdf.md)). `%clear` discards every declaration, so nothing it held is
available afterwards: it reports what was discarded, and the next command that would have used
something explains why it is gone.

```
sysml> %clear
session cleared
note: 1 instance was dropped because the session was reset; re-run %instantiate
sysml> %instances
(no instances created; 1 instance was dropped when the session was reset — re-run %instantiate)
```

## Seeing the model

`%print` writes the session back to the prompt as notation, either the whole model or, if you
give a name, a single element and its body:

```
sysml> package Demo {
  ...>   // how heavy it is
  ...>   part def Vehicle {
  ...>     attribute mass = 1500.0;
  ...>   }
  ...> }
✓ package Demo
sysml> %print Demo::Vehicle
// how heavy it is
part def Vehicle {
    attribute mass = 1500.0;
}
```

`%print` uses the same writer `%save` uses for a `.sysml` file, so it preserves comments and
the original text, re-indented the way a save would indent it, and its output can be typed or
loaded back to rebuild the same model. Names are written as in every other command
(`%print 'My Pkg'::Car`). Printing only reads the session: it creates no objects, and an
`%action` or `%state` session continues across it. An empty session, a name nothing
declares, and a name whose element the session has no source for each get a one-line
report. For RDF output, use `%save` with a `.ttl` path
([chapter 7](07-saving-and-rdf.md)); `%print` emits notation only.

## A name that needs quotes

If the notation requires a name to be quoted (it contains a space or punctuation, or it is a
keyword used as a name), pass it to a command exactly as it is written in the model, quotes
included. A quoted segment may appear anywhere in the qualified name:

```
sysml> package 'My Pkg' { part def Car { attribute m = 5.0; } }
sysml> %instantiate 'My Pkg'::Car
sysml> %features 'My Pkg'::Car
sysml> %eval 'My Pkg'::Car::m
```

Names that do not need quoting are written unquoted, as before. Commands report names back
with the same quoting, so a name returned by `%search` can be pasted into the next command
exactly as it appears.

## Loading files

```
sysml> %load examples/rdf-interop-demo.sysml
sysml> %load examples/*.sysml
sysml> %load examples/
```

`%load` accepts files, globs and directories, and submits their contents as if you had
typed them, with one difference: a loaded namespace remembers which file it came from, so entering
it again at the prompt *replaces* it (`note: replaced package …`) rather than adding to it. To change a
loaded file, edit it and load it again. Tab completion completes paths after `%load` and `%save`,
and meta-commands and symbol names everywhere else.

A file the parser cannot read is reported the same way as a typed declaration, with the
diagnostic pointing at the offending line of the file. None of its contents enter the
session, so the next submission is parsed against the model as it stood before the load. In
non-interactive use, a load's diagnostics are errors, so a script that loads a malformed
file fails rather than continuing against an empty session.

Two loaded files that both open `package P` declare two packages of that name, and the load
reports this:

```
sysml> %load a.sysml b.sysml
note: P is opened by more than one loaded file; each opening stays a declaration of its own, so a
member of one is not visible unqualified in the other — qualify it (P::member)
```

Each file keeps its own identity, which is what lets you reload one of them and replace only
its own contribution. If the two openings were merged into a single namespace, an edit to one
file could silently delete the other file's members. The members of both openings are
declared and reachable when qualified (`P::Wheel`, `P::Axle`), but an unqualified reference
from one to the other does not resolve. Entering a package at the prompt is unaffected: it still
merges into the package already in the session.

## Finding what a build offers

`%search` looks for a substring across the declared and library symbols and reports the kind of
each match. `%builtins` lists the library functions the current build can evaluate directly,
which tells you whether an expression will evaluate before you write a model around it, and the
package each must be imported from for its bare name to resolve (`x->size()` needs
`import SequenceFunctions::*;`; `SequenceFunctions::size(x)` resolves anywhere).

```
sysml> %search Vehicle
sysml> %builtins
sysml> %view Demo::summary
```

`%engines` lists the analysis engines the build answers checks with — `run` for one execution,
`explore` for every linearization, `sweep` for a table, `solve` for the SMT solver, one
`tool:<name>` per external tool the manifest directory `OPENSYSML_TOOLS` names, each with the
strongest evidence it can produce and whether its process was found — and `%engine` picks
the one the checks that follow are put to (`%engine solve`, `%engine all` for every engine that
covers the question, `%engine auto` for the default). Every verdict is followed by its
`standing:` line, saying which engine answered and how strong the evidence is
([Analysis engines](../reference/cli.md#analysis-engines)).

`%view <name>` reports the elements a view exposes, the views nested inside it, and whether it
conforms to the viewpoints it satisfies. Conformance checking is read-only and reported in
declaration order. Each `satisfy` gets a verdict of `conforms`, `violated` or `unevaluable`,
followed by a verdict for each concern the viewpoint frames; a concern whose condition fails
names the exposed element it failed for and the reason. A `satisfy` inherited from a
specialized view is marked `(from <view>)`. A concern the viewpoint frames but the view does
not is reported as `violated`. A concern that states no condition, or names one that does not
resolve, is reported as `unevaluable` with the reason, not as a pass. These verdicts, and
the way a nested view's framing is treated, are this implementation's own decisions, because
SysML v2 leaves verification verdict semantics non-normative.

```
sysml> package Demo {
  ...>     private import ScalarValues::*;
  ...>     private import Views::*;
  ...>     part def Wheel {
  ...>         attribute diameter : Real = 16.0;
  ...>     }
  ...>     part def Vehicle {
  ...>         attribute mass : Real;
  ...>         part wheel : Wheel;
  ...>     }
  ...>     part vehicle : Vehicle {
  ...>         attribute :>> mass = 1200.0;
  ...>     }
  ...>     concern def MassBudget {
  ...>         subject s : Vehicle;
  ...>         attribute maxMass : Real = 1000.0;
  ...>         require constraint {
  ...>             s.mass < maxMass
  ...>         }
  ...>     }
  ...>     concern def Modularity {
  ...>         subject s : Vehicle;
  ...>         require constraint {
  ...>             1 < 2
  ...>         }
  ...>     }
  ...>     viewpoint def StructurePerspective {
  ...>         frame concern budget : MassBudget;
  ...>         frame concern modularity : Modularity;
  ...>     }
  ...>     viewpoint structure : StructurePerspective;
  ...>     view def StructureView {
  ...>         satisfy structure;
  ...>         frame concern budget : MassBudget;
  ...>         frame concern modularity : Modularity;
  ...>     }
  ...>     view report : StructureView {
  ...>         expose vehicle;
  ...>         view detail {
  ...>             expose Wheel;
  ...>         }
  ...>     }
  ...>     view summary : StructureView {
  ...>         expose Vehicle;
  ...>         view detail {
  ...>             expose Wheel;
  ...>         }
  ...>     }
  ...>     view parts {
  ...>         expose Vehicle;
  ...>         render asElementTable;
  ...>     }
  ...> }
✓ package Demo

sysml> %view Demo::report
view Demo::report
  exposes
    Demo::vehicle (partUsage)
  nested views
    Demo::report::detail (viewUsage)
  viewpoint conformance
    satisfy structure (from Demo::StructureView): violated
      concern budget: violated
        Demo::vehicle: satisfaction satisfy MassBudget by Demo::vehicle: require condition evaluated to false: s.mass < maxMass
      concern modularity: conforms
```

When a name cannot be found, the session suggests up to three qualified names it might have meant,
nearest scope first: declarations in the session before library symbols, and a package's direct
member before a name nested inside another element.

## Rendering a view

`%render <name>` renders the exposed elements in the form the view's `render` member specifies: a
containment tree with nested views as subtrees, an interconnection diagram of the exposed parts
and the connections between them, a state machine's states and transitions, an action's nodes and
successions, or a table of the exposed elements. A view that specifies no rendering is drawn
as a tree:

```
sysml> %render Demo::summary
Demo::summary - tree rendering (the view states no rendering; a tree is the default)

part def Demo::Vehicle
  attribute mass (Real)
  part wheel (Wheel)
view Demo::summary::detail
  part def Demo::Wheel
    attribute diameter (Real)
```

A view that states `render asElementTable;` is rendered as aligned columns instead, listing the
exposed elements, what they declare, and the views nested inside the rendered view.

`%render <name> mermaid` writes a graph-shaped rendering as a Mermaid diagram, and
`%render <name> markdown` writes a table as a Markdown table. Either can be pasted straight
into a Markdown document or an editor. If you ask for a form the rendering kind does not
support, the REPL tells you which form it does support. State and action renderings read the
lowered graphs the runtime executes, so the picture reflects what actually runs. The rendering
itself is specific to this implementation, because SysML v2 §10.2 specifies the notation rather
than how a tool draws it.

Rendering only reads the model: it creates no objects, and a `%render` issued between two `%step`
commands leaves an action or state debugging session unchanged. A view that exposes nothing
renders as empty and says so. A rendering kind the build does not produce is reported by
name along with the view, not silently substituted, and an element the rendering cannot
represent is reported rather than left out. The same operation outside the prompt is
[`sysml -render`](../reference/cli.md#rendering-a-view).

## Where an expression is evaluated

`%eval <expression>` evaluates in the namespace the session is currently working in, which is the
most recently declared one, so declaring a scratch package later changes that context. To pin the
context, name it explicitly:

```
sysml> %eval in Demo::Vehicle : mass * 2
✓ mass * 2 (in Demo::Vehicle)
  = 3000.0
sysml> %instantiate Demo::Vehicle
✓ Created instance of Demo::Vehicle
  ID: 1
  Use %features Demo::Vehicle to inspect
sysml> %eval in Demo::Vehicle : mass * 2
✓ mass * 2 (on Demo::Vehicle ID: 1)
  = 3000.0
```

The pinned name may be an element, in which case the expression reads that element's namespace,
or an object reference, in which case the expression reads that object's feature values, the same
values an unpinned `%eval` reads after `%instantiate`. A single `:` separates the context from the
expression; the `::` inside a qualified name is not a separator.

## Addressing an object

Every command that takes an object — `%features`, `%invoke`, `%eval in`, and the object `%action`
and `%state` work on — accepts the same three spellings: the name the object was instantiated
under, the id `%instantiate` printed, and either followed by a path into the objects it holds, an
element of a multi-valued feature picked by index counted from 1:

```
sysml> %eval in Demo::Vehicle : mass * 2
sysml> %eval in #1 : mass * 2
sysml> %features Demo::Vehicle.engine
sysml> %features #1.engine
sysml> %features Demo::Vehicle.wheels[2]
```

The id is the object's identity for the rest of the session: a later submission that carries the
object over keeps it, and so does a second `%instantiate` of the same name, which creates a new
object and re-points the name — the first object is still there as `#1`, and `%instances` lists it
as such. A path walks the features that hold objects — parts, ports, connectors, and structured
attributes, the ones `%features` shows as `Instance(ID: n)`; an attribute holding a plain value at
the end of one is not an object, and the error says so. In a path `.` and `::` mean the same
thing, so `Demo::Vehicle::engine` and `Demo::Vehicle.engine` are the same object — with one
difference: what follows a `.` is always a feature of the object before it, whereas `::` may still be
spelling the declared name, which wins when an object was created under it. The forms and
every error a bad reference produces are
listed in the [reference](../reference/repl-commands.md#object-references). <kbd>Tab</kbd>
completes them: `#` offers the ids there are, `car.` the objects `car` holds.

## Command summary

| To find out | Use | Chapter |
|--------|-----|---------|
| what a view shows | `%view`, `%render` | this chapter |
| what an expression is worth | `%eval`, `%eval in … : …` | [5](05-checking.md) |
| what an object holds for each feature | `%instantiate`, `%features`, `%instances` | [5](05-checking.md) |
| whether a check holds | `%constraint`, `%requirement`, `%satisfy`, `%calc` | [5](05-checking.md) |
| what an analysis case computes and whether its objective holds | `%analysis` | [6](06-behavior.md#running-an-analysis-case) |
| what it computes across a range of one parameter | `%sweep`, `%samples` | [6](06-behavior.md#running-an-analysis-case) |
| whether a check *can* hold at all (experimental, needs [z3 or cvc5](01-install.md#installing-a-solver-optional)) | `%check` | [reference](../reference/repl-commands.md) |
| which conditions conflict when it cannot (experimental, needs [z3 or cvc5](01-install.md#installing-a-solver-optional)) | `%explain` | [reference](../reference/repl-commands.md) |
| what values satisfy it, keeping what is already fixed (experimental, needs [z3 or cvc5](01-install.md#installing-a-solver-optional)) | `%solve` | [reference](../reference/repl-commands.md) |
| which variants its conditions permit (experimental, needs [z3 or cvc5](01-install.md#installing-a-solver-optional)) | `%configure` | [reference](../reference/repl-commands.md) |
| which values are best for an analysis case's objectives (experimental, needs [z3](01-install.md#installing-a-solver-optional)) | `%optimize` | [reference](../reference/repl-commands.md) |
| what a behavior does, step by step | `%action`, `%state`, `%step`, `%tokens`, `%advance` | [6](06-behavior.md) |
| whether a result depends on the order the run happened to take, and how to replay another | `%schedule`, `%trace` | [6](06-behavior.md#when-a-model-has-more-than-one-valid-run) |
| which engine answered a check, how strong its evidence is, and which engines the build has | `%engines`, `%engine` | [reference](../reference/cli.md#analysis-engines) |
| where a run stopped and why | `%trace`, `%budget`, `%verbosity` | [10](10-troubleshooting.md) |
| whether what is typed is conforming SysML v2 | `%strict` | [3](03-command-line.md#strict-conformance) |

Every command and its arguments is listed in [reference/repl-commands.md](../reference/repl-commands.md).

## Non-interactive use

`-e` evaluates a single expression against a model and exits, using the same session machinery
without a terminal. See [chapter 3](03-command-line.md#running-from-a-script).

---

Next: [5. Expressions, calculations, constraints and requirements](05-checking.md).

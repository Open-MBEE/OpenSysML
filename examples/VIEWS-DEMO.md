# Views and rendering demo

[`views-demo.sysml`](views-demo.sysml) is a lander and the views that present it,
one view per rendering kind, so that `%view` and `%render` each have something to
show. A rendering is tool-defined output — SysML v2 §10.2 leaves rendering to the
tool — so what comes out is OpenSysML's own notation rather than a standard
interchange form.

```bash
./bin/sysml examples/views-demo.sysml
```

## `%view` — what a view exposes, and whether it conforms

```
%view LanderViews::overview
```

```
view LanderViews::overview
  exposes
    Lander::descender (partUsage)
    Lander::heavyDescender (partUsage)
  nested views
    LanderViews::overview::interfaceSubview (viewUsage)
  viewpoint conformance
    satisfy massPerspective: violated
      concern mass: violated
        Lander::heavyDescender: satisfaction satisfy MassBudget by Lander::heavyDescender:
        require condition evaluated to false: lander.mass <= maxMass
```

The view frames a mass concern and satisfies the viewpoint framing it, so
conformance is checked against each exposed lander: `descender` is within the
budget, `heavyDescender` is over it and named as the violation.

## `%render` — the five kinds

A view states its rendering with a `render` member, or inherits it from the
standard view definition it specializes, or states none and renders as a
containment tree.

| Command | Kind | How the view states it |
| --- | --- | --- |
| `%render LanderViews::overview` | tree | states no rendering, so a tree is the default |
| `%render LanderViews::interfaces` | interconnection | `render asInterconnectionDiagram` |
| `%render LanderViews::partsTable` | table | `render asElementTable` |
| `%render LanderViews::descentStates` | state | `: StateTransitionView` |
| `%render LanderViews::descentFlow` | action | `: ActionFlowView` |

The tree renders the exposed elements and each nested view as a subtree of its
own:

```
LanderViews::overview - tree rendering (the view states no rendering; a tree is the default)

part Lander::descender (Descender)
  attribute mass
part Lander::heavyDescender (Descender)
  attribute mass
view LanderViews::overview::interfaceSubview
  part def Lander::Descender
    …
```

The interconnection rendering shows exposed features as nodes and the
connections and flows between them as edges:

```
LanderViews::interfaces - interconnection rendering (render asInterconnectionDiagram)

part def Lander::Descender
  part tank (Tank)
  part thruster (Thruster)
  …

connections:
  tank -- thruster: supply
  tank => thruster: of Fuel
```

The state rendering comes from the lowered state graph, nested regions and
transitions with their triggers and guards alike. Each body that says where it
starts has a `start`, and the entry transitions out of it are edges in the order
their guards are tried in, carrying the guard like any transition; a state is
`initial` only when its body starts in it whatever the guards say:

```
LanderViews::descentStates - state rendering (view def StateTransitionView)

state def Lander::DescentStates
  start
  state cruise (initial)
  state descent
    start
    state braking (initial)
    state hover
  state landed

transitions:
  start of Lander::DescentStates -> cruise
  start of descent -> braking
  cruise -> descent
  descent -> landed
  braking -> hover: braking_to_hover: accept Signal [altitude < 100.0]
```

The action rendering comes from the lowered action graph, control nodes and
guarded successions alike:

```
LanderViews::descentFlow - action rendering (view def ActionFlowView)

action def Lander::Descend
  initial start
  action deployParachute
  action burn (own flow)
    …
  fork split
  join sync
  decision check

flow:
  split -> deployParachute
  sync -> check
  check -> touchdown: [altitude > 0.0]
  …
```

The table rendering lists the exposed elements as rows, each with its kind, type
and the element declaring it.

## Forms: text, Mermaid and Markdown

The default form is text. `mermaid` is the machine form of a diagram kind and
`markdown` of the table kind; a form the kind is not written in is refused rather
than written as something else.

```
%render LanderViews::descentFlow mermaid
```

```
%% LanderViews::descentFlow — action rendering (view def ActionFlowView)
flowchart TD
  subgraph n0 ["action def Lander::Descend"]
    n1["initial start"]
    …
  end
  n11 -->|"[altitude #gt; 0.0]"| n9
```

```
%render LanderViews::partsTable markdown
```

```
<!-- LanderViews::partsTable — table rendering (render asElementTable) -->
| Element | Kind | Type | Declared in |
| --- | --- | --- | --- |
| Lander::Descender | part def |  |  |
| mass | attribute | Real | Lander::Descender |
| … |
```

Asking for a form the kind is not written in — `%render LanderViews::partsTable
mermaid` — reports that a table rendering is not written as Mermaid and names the
forms it is written in.

## Filtered exposure

`safetyView` exposes the model recursively through an element filter, so it
reaches only what carries the `Safety` metadata:

```
%render LanderViews::safetyView
```

```
LanderViews::safetyView - tree rendering (the view states no rendering; a tree is the default)

part def Lander::Thruster
  attribute thrust (Real)
  port supply (FuelPort)
part def Lander::Parachute
```

## Rendering without the REPL

`sysml -render` writes the same rendering from the command line:

```bash
./bin/sysml examples/views-demo.sysml -render LanderViews::descentStates   # ASCII text at a terminal
./bin/sysml examples/views-demo.sysml -render LanderViews::descentStates > states.mmd   # Mermaid into a file
./bin/sysml examples/views-demo.sysml -render LanderViews::descentStates -render-form mermaid
```

Every command is documented in
[the REPL command reference](../docs/reference/repl-commands.md), which is
normative where this walkthrough and it differ.

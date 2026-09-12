# Diagram layout annotations — design

Status: **the read side and a layout-honoring writer are implemented** — the
`DiagramLayout` library, the semantic side table that resolves a position per view, the
geometry the rendering tree carries, what the Mermaid and text writers make of it, the
LSP fields, the validation pass, and the Graphviz `dot` form that pins nodes and routes
where the model says. Open: the write-back of positions from a graphical editor, and the
OMG proposal. It records the
design agreed for carrying diagram geometry in textual notation and how that geometry
reaches every rendering OpenSysML produces.

## The problem

SysML v2 text carries no diagram positions. The specification treats the graphical
notation as a projection of the model, and the Systems Modeling API defines no place for
an element's position either, so where a diagram draws its boxes is the concern of the
tool that drew it, kept in whatever form that tool keeps. Three consequences follow for
a project whose model lives in text:

1. **Every rendering is auto-laid-out.** OpenSysML renders a view as Mermaid or as text,
   and both leave the placement to the consumer: Mermaid runs its own layout each time
   the artifact is drawn. A person who moved the boxes of a wiring diagram until it read
   well loses that work at the next render.
2. **A diagram edited in a graphical tool does not round-trip through text.** A SysON or
   Cameo diagram has positions; saved as `.sysml` and re-read, it has none. The positions
   live in a sidecar the notation cannot reference, so they are orphaned at the first
   exchange.
3. **Positions cannot be reviewed.** Text is the form engineers version, diff and review;
   a layout kept outside it is not part of the reviewed change, so a reorganized diagram
   is invisible in a pull request.

## The design in one paragraph

Geometry is carried **in-band, as standard user-defined metadata** — notation every
conforming SysML v2 tool parses and preserves — and **per view**: a view's body states
where that view draws each element, an element itself may state a position every view
falls back to, and an element with neither is laid out automatically as today. The
semantic engine resolves the effective position for a (view, element) pair from the
metadata side table, the renderer carries it on the rendering tree as plain values, and
each writer keeps it visible in its own notation rather than dropping it — so a rendering
that cannot honor a position at least does not lose it. Nothing about an unannotated
model changes: its renderings are byte-identical to what they were.

## The metadata library

Three metadata definitions, shipped as a non-normative OpenSysML library extension in the
same tier as `IdentityMetadata` (`internal/core/libs/stdlib/OpenSysML
Libraries/DiagramLayout.sysml`, counted by the stdlib conformance gate with the other
extensions):

```sysml
standard library package DiagramLayout {
    metadata def Layout {
        attribute x : ScalarValues::Real;
        attribute y : ScalarValues::Real;
        attribute width : ScalarValues::Real[0..1];
        attribute height : ScalarValues::Real[0..1];
        attribute collapsed : ScalarValues::Boolean[0..1];
    }

    metadata def Route {
        attribute points : ScalarValues::Real[0..*] ordered nonunique;
    }

    metadata def Canvas {
        attribute unit : ScalarValues::String[0..1];
        attribute width : ScalarValues::Real[0..1];
        attribute height : ScalarValues::Real[0..1];
    }
}
```

Applied:

```sysml
package VehicleLayout {
    private import DiagramLayout::*;
    private import Views::*;

    part def Engine {
        // The position every view that does not place the engine falls back to.
        @Layout { x = 0; y = 0; }
    }
    part def Wheel;
    part def Vehicle {
        part engine : Engine;
        part wheel : Wheel[4];
        connection drive connect engine to wheel;
    }

    view vehicleView {
        expose Vehicle::**;
        render asInterconnectionDiagram;

        @Canvas { unit = "px"; width = 1200; height = 800; }
        metadata Layout about Vehicle::engine { x = 120; y = 80; width = 200; height = 90; }
        metadata Layout about Vehicle::wheel { x = 480; y = 80; collapsed = true; }
        metadata Route about Vehicle::drive { points = (320, 125, 400, 125, 480, 125); }
    }
}
```

Decisions, each with its reason:

- **Pixels, y down, origin at the top left.** The coordinate system every diagram
  format OpenSysML targets or imports shares (SVG, the browser, Mermaid's renderer,
  SysON's Sprotty, Graphviz `plain` output after its own flip). `Canvas.unit` names the
  unit for a consumer that wants another (`"pt"`, `"mm"`); the value `"px"` is the
  default and what OpenSysML writes.
- **`Layout` positions a box; `Route` steers a line.** A `Layout` applies to anything a
  rendering draws as a node; a `Route` applies to the *declaring element* of an edge — a
  connection, an interface, a flow, a transition, a succession — never to its ends. The
  waypoints are one flattened sequence (`x0, y0, x1, y1, …`) because a metadata feature
  holds scalars: a sequence of pairs would need a second attribute definition for the
  pair, which no other tool would read as a point.
- **`Route.points` is `nonunique`.** The specification's default for a feature is
  uniqueness (KerML 7.3.4.4), and OpenSysML's type checker enforces it on a sequence
  written at a unique feature. A vertical segment repeats its x coordinate and a route
  that doubles back repeats a whole point, so a unique `points` would reject the routes
  that most need stating. `ordered nonunique` is what the Kernel library's own sequence
  features declare, and it makes the checker accept repeats without a special case.
- **An extent is a pair, and `0` is an extent.** `Layout.width`/`height` and
  `Canvas.width`/`height` are sized when both are bound, unsized when neither is, and an
  error when only one is; a bound `0` is the size zero, not an omission, so the writers
  and the LSP show it as such.
- **In-band metadata, not a sidecar and not comments.** A sidecar file does not travel
  through another tool; a comment convention has no structure, is reflowed or dropped
  freely, and nothing validates it. Standard metadata survives any conforming tool, is
  parsed into the model, and is checked by the type system like any other value.
- **The view states its own layout.** A position is a property of a diagram, not of the
  element: the engine sits top-left in the wiring diagram and bottom-right in the
  mounting one. `metadata Layout about … { … }` in the view's body states the position
  *this view* uses. An inline `@Layout { … }` in the element's own body is the fallback
  for every view that does not place it — useful for a definition that appears in many
  small diagrams the same way.
- **Metadata forms as the grammar has them.** A prefix `@Layout` before a declaration
  (`@Layout part def Engine;`) annotates the declaration but takes no body, so it cannot
  carry values; the inline fallback is therefore written *inside* the body
  (`part def Engine { @Layout { x = 0; y = 0; } }`), the same way `IdentityMetadata`
  annotations are. A standalone `@Layout { … }` in a package body annotates the package
  itself — a position for the package's node in a containment tree, not for the
  declaration that follows it.
- **Values are constants.** A position is data a tool wrote and a person reads; it is
  not derived from the model at render time. A binding that is not a model-level
  constant of the right kind (a feature reference, `null`, a tuple where a scalar is
  expected, a non-Boolean `collapsed`) is an error naming the attribute, never a silent
  drop.

## Semantics

### Resolution

For a (view, element) pair the **effective layout** is, in order:

1. the `Layout` annotation `about` the element **declared inside the view's body** — its
   declaring scope is the view or a namespace nested in the view;
2. the inline `Layout` annotation in the element's own body (or a standalone
   `metadata Layout about element` stated outside any view);
3. none: the element is laid out by the consumer.

A `Route` resolves the same way for an edge's declaring element. A `Canvas` belongs to a
view alone, stated in its body. Where a view's body states two `about` annotations of one
kind for one element, the first in declaration order applies and the second is a warning.

With no view — the `#tree`, `#interconnection:X` pseudo-views and the LSP's render of a
document — only the element-level fallback applies, since there is no view body to look
in.

The resolution is a lazy, memoized side-table query in `internal/core/semantics/layout.go`
(`Model.LayoutOf`, `Model.RouteOf`, `Model.CanvasOf`, over `Model.LayoutSitesOf`) built
on the metadata side table the element filters and identity annotations already use
(`Model.ElementMetadataOf`, `AnnotationSite.Scope`, `AnnotationSite.About`). A view's body
is found by walking the annotation's declaring scope up to the nearest view; an `about`
annotation stated anywhere else is element-level.

### The rendering tree

`view.Node` carries `Geometry *Geometry` (`X`, `Y`, `Width`, `Height`, `HasSize`,
`Collapsed`), `view.Edge` carries `Route []Point`, and `view.Rendering` carries
`Canvas *Canvas` (`Unit`, `Width`, `Height`, `HasSize`). They are values: the tree stays free of AST
nodes and symbols, and `Rendering.Clone` copies them. Every graph-shaped rendering
populates them — the containment tree, the interconnection diagram, the state machine
and the action flow — through `Renderer.Render` (the view given, so `about` annotations
in its body apply) and `Renderer.RenderExposed` (no view, element fallback only). The
sequence diagram and the table carry none: a lifeline has no box to position and a row
has no geometry.

The state and action renderings draw the lowered `StateGraph` and `ActionGraph`, so their
nodes are lowered declarations; each graph maps a lowered node back to the declaration
it came from (`StateGraph.DeclOf`), and the renderer resolves that declaration to its
symbol through the scopes — never by re-parsing source text. That holds for an unnamed
transition too: `transition first off then on { @Route { … } }` has no name to state a
`Route` `about`, so the annotation lives in its body, and an unnamed transition is an
anonymous member of its state (a `TransitionUsage` is a feature of the state that
declares it, SysML v2 §7.19.2) that the rendering and the validation pass both find
by its declaration. A usage typed by a definition of another document inherits that
definition's states and transitions, so the state machine is lowered through the
name-resolution tier (`lower.NewLibraryStateTypes`), as the runtime lowers it, and the
inherited declarations keep their inline annotations and take the view's. An action
usage's graph is its own successions over the nodes they name, inherited ones included;
the lowering finds those through the scope tree, so a definition in another document is
out of its reach, which the rendering reports as a notice rather than drawing a partial
flow.

### Validation (a constraint-tier pass)

`internal/core/passes/diagram_layout.go`, `DiagramLayoutPass`, source `constraint`:

| Code | Severity | When |
|---|---|---|
| `diagram-layout-unplaced` | warning | A `Layout` on an element the rendering draws no node for, or a `Route` on one it draws no edge for. In a view's body the judge is what that view's rendering actually draws (`Route about Loop::pump` in an interconnection view: a part is a node, not an edge; `Layout about Spare::valve` in a view exposing `Loop` only: nothing is drawn for it); for an element-level annotation, every kind this build produces (`Route` on a `part def`, `Layout` on a dependency). A package is a node of the containment tree, so a `Layout` on one is placed. |
| `diagram-layout-value` | error | `Route.points` of odd length (waypoints are x, y pairs), a `Canvas` binding one of `width` and `height` without the other (an extent is a pair, and `0` is an extent), or a binding that is not a constant of the attribute's kind. |
| `diagram-layout-canvas` | error | A `Canvas` annotating anything that is not a view, or one about a view stated outside that view's body (`metadata Canvas about V { … }` beside `V`, or in another view): it sizes nothing. |
| `diagram-layout-duplicate` | warning | Two `about` annotations of one kind for one element in one view's body; the first stated applies. |

Which elements a rendering draws is asked of the renderer itself: for a view-local
annotation the pass renders the view once and reads what was drawn (`Renderer.DrawnIn`),
for an element-level one it asks what any kind can draw (`Renderer.DrawsAnywhere`), so
the pass and the renderings cannot disagree. A binding that
reads a feature rather than a literal is already an error of the type tier
(`metadata-value-not-evaluable`), so the pass does not repeat it.

## What each writer does

| Form | Positions | Notes |
|---|---|---|
| `mermaid` | Not representable | Written as comments after the header so a round trip through the artifact keeps them: `%% canvas: unit=px w=1200 h=800`, `%% layout: n1 x=120 y=80 w=200 h=90 collapsed`, `%% route: n1->n2 320,125 400,125 480,125`. Node ids are the ones the diagram body uses. |
| `text` | Not representable | `at (120, 80)` after a positioned node, `size 200×90` and `collapsed` when stated; `via (320, 125) (400, 125)` after a routed edge; a `canvas size … in px` line under the title. |
| `dot` | Honored | `pos="x,y!"`, `pin=true`, `width`/`height`/`fixedsize=true` on a placed node, `pos` as a B-spline on a routed edge, `bb` on the root graph and on a sized cluster, and a `// layout: neato -n` or `-n2` header — see [The DOT form](#the-dot-form). |
| `markdown` (table) | n/a | — |
| LSP `opensysml/render` | Structured | Optional `x`, `y`, `width`, `height`, `collapsed` on a node, `route` on an edge, `canvas` on the result — see [the LSP reference](../reference/lsp.md). |

Numbers print in their shortest exact form (`strconv.FormatFloat(v, 'f', -1, 64)`), so
`120` stays `120` and `12.5` stays `12.5`. A model without layout annotations produces
exactly the bytes it produced before; the existing rendering goldens pin that.

### The DOT form

Graphviz is the one target that can draw a node where the model says, so the `dot` form
(`internal/core/view/dot.go`, `dotGeometry`) writes the geometry as Graphviz attributes
rather than as comments. Nothing runs Graphviz — the writer, the tests and the PDF backend
produce and check text — so what follows is what the text asks of a `neato` that reads
it; the choices are verified against the Graphviz documentation of
[the command line](https://graphviz.org/doc/info/command.html), [`pos`](https://graphviz.org/docs/attrs/pos/),
[`splineType`](https://graphviz.org/docs/attr-types/splineType/), [`inputscale`](https://graphviz.org/docs/attrs/inputscale/),
[`bb`](https://graphviz.org/docs/attrs/bb/) and [`pin`](https://graphviz.org/docs/attrs/pin/).

**Coordinates.** The rendering is in pixels from the top-left corner, y downward. Graphviz
reads a `pos` in points (1/72 in) from the bottom-left corner, y upward — `neato -n`
treats input coordinates as points — and a node's `width` and `height` in inches. The
writer converts, in this order:

- *Corner to centre.* `Layout.x`/`y` is the top-left corner of the node's box; Graphviz's
  `pos` is its centre. A node with a size is pinned at `(x + width/2, y + height/2)`. A
  node without one has no box to offset by, so its point is taken as the centre.
- *Y flip.* `y' = H − y`, where `H` is `Canvas.height` when the canvas states an extent,
  otherwise the largest `y + height` over the rendering's placed nodes (a node without a
  size counts its `y`), computed once per rendering so every position of the file flips
  against the same height. With neither, `H` is 0.
- *Pixels to points.* `1 px = 0.75 pt` (the CSS convention, 96 px to the inch). A canvas
  whose `unit` is `"pt"` converts one to one. Any other unit is not converted: the writer
  falls back to px and says so in a `// not represented: canvas unit "mm"; positions are
  converted as px (1 px = 0.75 pt)` header line. A canvas without geometry to convert is
  not noticed.
- *Sizes.* `width` and `height` are the box in inches: `px × 0.75 / 72`, with
  `fixedsize=true` so the label does not grow the box.

A converted number is written to at most four decimals in its shortest form, so a third
of a pixel is `0.25` pt and not sixteen digits, and `-0` never appears.

**Nodes.** A placed node writes `pos="<x>,<y>!"` and `pin=true` after its shape and
label, then `width`, `height` and `fixedsize=true` when it is sized. The `!` and `pin`
are Graphviz's two ways of saying the same thing to `neato`; both are written so the file
reads the same to a person and to either check. A collapsed node (`collapsed = true` on a
node that has children) is written as a leaf: no `subgraph`, its label unchanged, its
children not written; an edge at a hidden child is drawn at the collapsed node, and an
edge between two hidden children of one collapsed node is dropped. A collapsed node
without children is an ordinary leaf.

**Clusters.** Graphviz cannot pin a cluster. A placed node that is drawn as a cluster
pins its invisible anchor node — the node an edge to or from the cluster names — at the
cluster's centre, and when the node is sized the subgraph states its box as
`bb="<llx>,<lly>,<urx>,<ury>"` in points. `neato -n` reads a cluster's `bb` and draws the
cluster from it (its no-op initialization parses the attribute; the `bb` documentation
describes only the output side), so a sized cluster keeps its box. A cluster without a
size states no `bb`, which leaves `neato -n` no box to draw the cluster from; the header
notices it: `// not represented: the box of n1, which has no size (neato -n draws a
cluster from its bb alone)`. A cluster that is not placed at all, but holds placed
members, sets its anchor — unpinned, `pos="<x>,<y>"` — at the centre of its members'
extent, so `neato -n` has the position it requires on every node and draws the cluster
around the members; a cluster with no placed member has no anchor position and counts
as unplaced.

**Edges.** Graphviz's edge `pos` is a `splineType`: a list of `3n + 1` B-spline control
points. Authored `Route.points` are a polyline. The writer emits the spline that draws
that polyline exactly — for each segment `P → Q` the cubic

```text
P,  P + (Q − P)/3,  P + 2(Q − P)/3,  Q
```

sharing `Q` with the next segment, so `n` waypoints become `3(n − 1) + 1` control points
(two waypoints are four points, three are seven). Each point is converted as a node
position is. No `e,x,y`/`s,x,y` endpoint is written — the route is drawn as authored,
with no arrowhead clipping point — and no `lp` (label position): Graphviz places the
label. A route of one waypoint draws no line, is not written, and is noticed. `splines`
is not set; `neato -n2` keeps a supplied `pos` whatever `splines` says.

**Header and graph.** The `// layout:` header names the command the file is written
for:

| Geometry | Header | Why |
|---|---|---|
| none | `// layout: dot` | Byte-identical to a rendering without the library. |
| node positions only | `// layout: neato -n` | `-n` keeps every node's `pos` and routes the edges itself. |
| any edge route | `// layout: neato -n2` | `-n2` keeps node `pos` *and* every edge `pos` supplied, routing only the edges without one. |

`neato -n` requires a `pos` on every node, so a rendering that places some nodes but not
all is noticed (`no position for 1 of 3 nodes (neato -n needs one on every node; neato -s72
keeps the pinned ones and places the rest)`, counting the drawn nodes and anchors without
one): `neato -s72` — no `-n`, input scale 72 so the points are read as points — honors
`pin=true` and lays out the rest. When the canvas states an extent the root graph
carries `bb="0,0,<w>,<h>"` in points, after `rankdir` and `compound`; `size` is not
used, since it is a maximum and not a canvas. The graph attribute list is otherwise
untouched.

**Not representable.** A cluster's own position, when it has no size (noticed, above);
a canvas unit other than `px` or `pt` (noticed, converted as px); a route of one waypoint
(noticed); `Layout.collapsed` on a leaf (nothing to fold). Layout the writer has no say
in — label placement, arrowhead clipping, the layout of unplaced nodes — is Graphviz's.

## Interop

- **Editors.** The LSP fields are what a graphical client (the VS Code diagram panel, a
  SysON-style editor) reads to place nodes where the model says. Writing positions back
  is the `ApplyEdits` `AddMember` of `metadata Layout about …` into the view body —
  planned, not built.
- **RDF / Flexo.** Metadata already maps; `Layout`, `Route` and `Canvas` ride along as
  ordinary metadata usages with no change to the mapping.
- **Other tools.** Any conforming implementation parses and preserves the annotations,
  since user-defined metadata is standard notation. Only a tool that knows the library
  gives them meaning, which is the same situation as `IdentityMetadata`.

## Open items

- **Write-back** from a graphical editor's layout into the view body.
- **Standardization.** A proposal to the SysML v2 taskforce is drafted in
  [omg-issues.md](omg-issues.md) and not filed.
- **Containers.** A `Layout` on a composite node positions its box; where its children
  are drawn relative to it is the consumer's decision. Nested coordinates (relative to
  the parent) were considered and rejected for now: absolute coordinates are what every
  targeted format takes, and relative ones would make moving a parent a rewrite of every
  child.

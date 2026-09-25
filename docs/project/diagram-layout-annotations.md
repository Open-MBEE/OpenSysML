# Diagram layout annotations — design

Status: **implemented, read and write** — the `DiagramLayout` library, the semantic
side table that resolves a position per view, the geometry the rendering tree carries,
what the Mermaid, text and Graphviz DOT writers make of it, the LSP fields, the
validation pass, and the write-back: `internal/check/edit` sets, updates and clears the
annotations source-preservingly, `opensysml/applyModelEdit` exposes that as `setLayout`,
`setRoute` and `setCanvas`, and the VS Code diagram panel writes a `Layout` when a node
is dragged and a `Route` when an edge is. Open: the OMG proposal. This note records the
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

Six metadata definitions, shipped as a non-normative OpenSysML library extension in the
same tier as `IdentityMetadata` (`internal/workspace/libs/stdlib/OpenSysML
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

    metadata def Style {
        attribute fill : ScalarValues::String[0..1];
        attribute line : ScalarValues::String[0..1];
        attribute text : ScalarValues::String[0..1];
        attribute font : ScalarValues::String[0..1];
        attribute fontSize : ScalarValues::Real[0..1];
        attribute bold : ScalarValues::Boolean[0..1];
        attribute italic : ScalarValues::Boolean[0..1];
    }

    metadata def Note {
        attribute text : ScalarValues::String;
        attribute x : ScalarValues::Real;
        attribute y : ScalarValues::Real;
        attribute width : ScalarValues::Real[0..1];
        attribute height : ScalarValues::Real[0..1];
    }

    metadata def Picture {
        attribute location : ScalarValues::String;
        attribute x : ScalarValues::Real;
        attribute y : ScalarValues::Real;
        attribute width : ScalarValues::Real;
        attribute height : ScalarValues::Real;
        attribute alt : ScalarValues::String[0..1];
        attribute above : ScalarValues::Boolean[0..1];
    }
}
```

`Layout`, `Route` and `Canvas` say where things are; `Style`, `Note` and `Picture` say how things
look and what is drawn beside them, the other things a tool's own drawing of a diagram carries that
the model does not. A `Style` applies to whatever a rendering draws as a node or an edge — the colours
as `"#RRGGBB"`, the font by family name, its size in points — each attribute optional, so a
symbol coloured by hand states its fill and nothing else, and the drawing style supplies the
rest. A `Note` is a comment box: its text, the top-left corner of its box and an optional size,
in the units of `Layout`; `metadata Note about X { … }` in a view anchors it to `X` with a dashed
line, a `@Note { … }` on the view itself is free on the drawing surface. A note stated in a
view's body — `about` a member or free on the view — is drawn in that view alone, its corner in
that view's canvas; a note stated outside every view applies in every view. A `Picture` is an
image pasted onto the drawing surface: the file it is read from, relative to the file the view is
written in (an absolute path or a URL as given), the box it fills, an optional alternative text,
and `above` when it lies over the element symbols it overlaps rather than under them, which is
the default — a background picture with the boxes drawn on top. There is no order finer than
that: a picture lies under every element symbol or over every one, never between two. A
`Picture` belongs to a view alone, `@Picture { … }` in its body; a view carries as many as it
shows, in declaration order.
The graph-shaped renderings (interconnection, tree, state and action views) draw it; a table or
sequence rendering has no drawing surface for it, so it keeps its rows and states in its notices
each picture's file and box under `not drawn`.
A `Picture` names a file the way a `DocumentQueries::Image` block's `location` does, and it is
trusted the same way: the model's author states which files of theirs a rendering reads, so a
document rendered from a model lets that model reach whatever the location names, as an
`Image` block always has. What a rendering copies out of such a file is bounded, though: when a
drawn diagram's pictures are inlined into a single-file document, a file is embedded as a data
URI only when its bytes are a recognised image (PNG, JPEG, GIF, BMP, WebP, or one well-formed SVG
document — a single `svg` root in the SVG namespace); any other file stays a reference to its
path, so no text or binary that is not an image is ever copied into a document by naming it. An
SVG picture is shown through an `<image>` element (of the drawn SVG) or an `<img>` element, which
a browser renders as a static image — none of its scripts runs and it loads no external
resource — and the PDF engine runs no script at all.

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
        metadata Style about Vehicle::engine { fill = "#F2DCDB"; line = "#9C0006"; bold = true; }
        metadata Note about Vehicle::engine {
            text = "Sized for the 2.0 L variant."; x = 120; y = 200; width = 160; height = 40;
        }
        @Picture { location = "images/chassis.png"; x = 0; y = 0; width = 1200; height = 800; alt = "the chassis, from above"; }
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

The resolution is a lazy, memoized side-table query in `internal/semantic/semantics/layout.go`
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

`internal/check/passes/diagram_layout.go`, `DiagramLayoutPass`, source `constraint`:

| Code | Severity | When |
|---|---|---|
| `diagram-layout-unplaced` | warning | A `Layout` on an element the rendering draws no node for, or a `Route` on one it draws no edge for. In a view's body the judge is what that view's rendering actually draws (`Route about Loop::pump` in an interconnection view: a part is a node, not an edge; `Layout about Spare::valve` in a view exposing `Loop` only: nothing is drawn for it); for an element-level annotation, every kind this build produces (`Route` on a `part def`, `Layout` on a dependency). A package is a node of the containment tree, so a `Layout` on one is placed. |
| `diagram-layout-value` | error | `Route.points` of odd length (waypoints are x, y pairs), a `Canvas` binding one of `width` and `height` without the other (an extent is a pair, and `0` is an extent), a `Picture` without its `location`, `x` and `y`, or `width` and `height`, with an empty `location` or a `width` or `height` that is not positive, or a binding that is not a constant of the attribute's kind (`null`, a pair where one number is due). A value of another type than the attribute's (`collapsed = 1`, a String among the points) is the type checker's `cannot bind` error, as for any bound value, and one the model cannot evaluate is `metadata-value-not-evaluable`. |
| `diagram-layout-canvas` | error | A `Canvas` annotating anything that is not a view, or one about a view stated outside that view's body (`metadata Canvas about V { … }` beside `V`, or in another view): it sizes nothing. A `Picture` annotating anything that is not a view: it is drawn on no surface. |
| `diagram-layout-duplicate` | warning | Two `about` annotations of one kind for one element in one view's body; the first stated applies. A `Note` or a `Picture` is never a duplicate: every one is drawn. |

Which elements a rendering draws is asked of the renderer itself: for a view-local
annotation the pass renders the view once and reads what was drawn (`Renderer.DrawnIn`),
for an element-level one it asks what any kind can draw (`Renderer.DrawsAnywhere`), so
the pass and the renderings cannot disagree. A binding that
reads a feature rather than a literal is already an error of the type tier
(`metadata-value-not-evaluable`), so the pass does not repeat it.

## What each writer does

| Form | Positions | Style, Note and Picture | Notes |
|---|---|---|---|
| `mermaid` | Not representable | A node's `fill`, `line` and `text` as a `classDef`/`class` pair; its font, an edge's `Style`, every `Note` and every `Picture` (with its file and box) counted in the `%% not represented:` notice | Written as comments after the header so a round trip through the artifact keeps them: `%% canvas: unit=px w=1200 h=800`, `%% layout: n1 x=120 y=80 w=200 h=90 collapsed`, `%% route: n1->n2 320,125 400,125 480,125`. Node ids are the ones the diagram body uses. The nodes drawn are the ones the `dot` form draws: in a rendering that positions some nodes, the placed ones and the edges between them, with the unplaced counted in a `%% not represented:` notice, or every node under `Options.Unplaced = UnplacedStrip`; the `plantuml` form does the same under `' not represented:`. |
| `text` | Not representable | Counted in the notice; nothing coloured is written; each `Picture` listed under `pictures:` with its file, box and alt text | `at (120, 80)` after a positioned node, `size 200×90` and `collapsed` when stated; `via (320, 125) (400, 125)` after a routed edge; a `canvas size … in px` line under the title. |
| `dot` | Honored | Honored whole: `fillcolor`, `color`, `fontcolor`, `fontname`, `fontsize` written after the drawing style's defaults so they win, `<b>`/`<i>` round the label; a `Note` as a `shape=note` node pinned at its box, anchored by a dashed headless edge (see [view rendering forms](view-rendering-forms.md#style)); a `Picture` as a `shape=none` node with `image=` its file, `imagescale=both`, `fixedsize=true` and its box's `width`/`height`, pinned at the box's centre, written before the element nodes (Graphviz draws in order, so it lies under them) or after them when `above`, its `alt` as the `tooltip` | Graphviz's own vocabulary, converted from y-down pixels to y-up points (`inputscale=72`, `dpi=72`; y measured from the canvas's bottom edge, negated with no canvas height): a node pinned at the centre of its box with `pos="x,y!"`, `pin=true`, `width`/`height` in inches — `fixedsize=true` for a stated size, fitted to the label for an unstated one — and `comment="collapsed"`; a cluster's `bb` stated (the stated box, or the one round its positioned members) and its anchor pinned at the centre; a route as a `pos` spline through the waypoints, a route of one waypoint noticed, as is a route `neato` redraws; the canvas echoed as `// canvas:` and held by an invisible point pinned at each corner, so the drawing's bounding box is the canvas. The `// layout:` header names `neato -n2` when every node is placed and any edge routed, `neato -n` when every node is placed and none routed, `neato` when some nodes are, `dot` when none — see [view rendering forms](view-rendering-forms.md#geometry). |
| `plantuml` | Not representable | A node's colours as `#fill;line:line;text:text`; the rest, pictures included, counted in the `' not represented:` notice | The node set and edge set the `dot` form draws, as above |
| `markdown` (table) | n/a | n/a | — |
| LSP `opensysml/render` | Structured | Optional `style` (`fill`, `line`, `text`, `font`, `fontSize`, `bold`, `italic`) on a node or an edge; notes reach the client only through the DOT it asks for | Optional `x`, `y`, `width`, `height`, `collapsed` on a node, `route` on an edge, `canvas` on the result — see [the LSP reference](../reference/lsp.md). |

Numbers print in their shortest exact form (`strconv.FormatFloat(v, 'f', -1, 64)`), so
`120` stays `120` and `12.5` stays `12.5`. A model without layout annotations produces
exactly the bytes it produced before; the existing rendering goldens pin that.

## Interop

- **Editors.** The LSP fields are what a graphical client (the VS Code diagram panel, a
  SysON-style editor) reads to place nodes where the model says. Writing positions back
  is `opensysml/applyModelEdit` with `setLayout`, `setRoute`, `setCanvas` and `setStyle`
  ([the LSP reference](../reference/lsp.md)): `metadata Layout about … { x = …; y = …;
  }` into the view body when the operation names a view, inline into the element's own
  body when it does not, updated in place when one is already stated and removed with
  its line when cleared. Each annotation goes into the workspace document that declares
  what holds it, whichever document the request came from: a view-local `Layout` or
  `Route` and a `Canvas` into the view's document, an inline `Layout` or `Route` into
  the element's, so a view that exposes another file's parts is placed without touching
  that file, and a document drawn directly places what it draws from another file in
  that file. The answer is one `WorkspaceEdit` with a versioned `TextDocumentEdit` per
  document changed, validated together, and one with no edits per document read and left
  as it was, so the client applies nothing once any has moved; a bundled library file,
  or a document the index holds without its source, is never written — the operation
  refuses, naming the file. The VS Code panel draws its own SVG from the geometry, lays
  out what the model does not place, and writes one edit per drag, so the editor's undo
  puts a node back, in every file at once. A model nobody has dragged in keeps exactly
  its bytes.
- **RDF / Flexo.** Metadata already maps; `Layout`, `Route`, `Canvas`, `Style`, `Note` and
  `Picture` ride along as ordinary metadata usages with no change to the mapping.
- **Migration from Cameo.** The SysML v1 migrator writes all six from the diagram symbol
  streams a `.mdzip` carries — each symbol's box, each path's bends, the diagram frame as the
  `Canvas`, a symbol's own `FILL_COLOR`/`PEN_COLOR`/`TEXT_COLOR`/`FONT` as its `Style`, each
  comment or text box as a `Note` anchored where its anchor reaches, and each image pasted onto
  the diagram as a `Picture`, its bytes written to a file under `images/` beside the notation —
  with an MTIP export given by
  `-layout` taking precedence for every element its record places or routes and the stream
  supplying the rest
  ([the mapping](../reference/sysml-v1-migration.md#layout-from-an-mtip-export)).
- **Other tools.** Any conforming implementation parses and preserves the annotations,
  since user-defined metadata is standard notation. Only a tool that knows the library
  gives them meaning, which is the same situation as `IdentityMetadata`.

## Open items

- **Standardization.** A proposal to the SysML v2 taskforce is drafted in
  [omg-issues.md](omg-issues.md) and not filed.
- **Picture bytes.** A `Picture` refers to a file beside the notation rather than holding the
  bytes: a model file stays text, and the image travels with the notation as an ordinary file.
  A tool that moves the notation alone loses the picture, as it loses a linked `DocumentQueries::Image`.
- **Label positions.** A `Route` places a line, not its label; the DOT form sets an edge label
  beside the route's longest segment. Cameo's stream carries no label position either, so
  nothing is lost in migration, but a hand-placed label has no home yet.
- **Containers.** A `Layout` on a composite node positions its box; where its children
  are drawn relative to it is the consumer's decision. Nested coordinates (relative to
  the parent) were considered and rejected for now: absolute coordinates are what every
  targeted format takes, and relative ones would make moving a parent a rewrite of every
  child.

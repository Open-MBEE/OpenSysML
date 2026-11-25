# Notation shape on structured render nodes

## Why a canvas needs this

A structured render is the machine-readable form of a view: a flattened list of nodes and edges
that a tool draws directly. It carries a node's kind, name, type, detail, ports, geometry, style
and source origin, but not how the notation draws that node. A case is a stadium, an actor is a
stick figure, a requirement is a box, and a decision is a diamond — all of that knowledge lived
only in the writers of diagram text, notably the Mermaid writer's `mermaidNodeShape`. A consumer
that wants a canvas rather than a picture therefore had to parse a written form or guess from the
kind string. Neither is reliable: the kind is a notation keyword affected by modifiers, and the
written form is a separate output with its own options.

The new `shape` field answers that question once, on the structured node itself. It is derived
from the element's metaclass (carried into the rendering as the notation kind) and from the
rendering kind, so a case drawn in a case diagram is a stadium while the same definition drawn
in a tree is a box, matching what the written forms already draw.

## Baseline

At `4d67c7a98f` (nightly 2026-11-18, `v0.12.1-2379-g4d67c7a98f`; the latest stable tag was
`v0.12.1`) the wire message `RenderNode` had fields 1 through 17 — `id`, `kind`, `name`,
`name_synthesized`, `type`, `detail`, `text`, `stand_in`, `parent`, `ports`, `origin`, `geometry`,
`style`, `compartment`, `inherited`, `arrow` and `parallel` — and no shape field. The structured
view model (`view.NodeData`) had no shape either, and neither did any client's typed node.
`internal/ir/view/mermaid.go` `mermaidNodeShape` was the only place in the engine that decided a
notation shape. There was no upstream issue or pull request adding one: the adjacent view work
in [PR #2530](https://github.com/Open-MBEE/OpenSysML/pull/2530) and
[PR #2550](https://github.com/Open-MBEE/OpenSysML/pull/2550) drew subject and actor edges but
left the structured node unchanged. Downstream canvas models had the same hole: head, keyword,
detail, stereotype and parent, but no shape.

## What changed

- `internal/ir/view/shape.go` defines `Shape` and the one derivation, `NodeShape(renderingKind,
  node)`. It covers rectangles (definitions), rounded boxes (usages), the case family's stadium,
  notes, diamonds, the control-node circles, bars and crosses, history nodes and borderless item
  boxes. The actor, subject and objective glyphs belong to case and mixed renderings; in any
  other rendering those roles are drawn as their ordinary rounded usage boxes.
- `internal/ir/view/data.go` fills `NodeData.Shape` for every flattened node, including
  compartment rows, once when the structured rendering is made. The immutable AST is untouched
  and no semantic data is recomputed downstream.
- The gRPC response, the JSON-RPC engine response and the language server's `opensysml/render`
  result copy the field. gRPC withholds it without the new capability; the other two transports
  do not negotiate capabilities for response fields.
- The typed clients decode it (Go, Python, Node, Java and Rust), and Julia and MATLAB declare
  the capability. The VS Code canvas draws a stadium as a pill-shaped rectangle and an actor as
  the UML stick figure beside its name. Where a server reports no shape, the canvas falls back
  to its old kind-based symbol.
- The Go conformance wrapper and the WebAssembly test's expected-node decoder copy the field
  alongside the other node fields, so a shape the client decoded is not dropped before comparison.

## Design decisions and alternatives rejected

- **A string vocabulary, not a backend-specific shape.** Values such as `stadium` and `actor`
  name notation, not `rx`, `shape=umlActor` or Mermaid's `(["..."])`. A canvas or another writer
  can map one token to its own drawing primitive; the alternative, carrying a Mermaid or DOT
  fragment, would have exported one writer's syntax as an API and made every client parse it.
- **One derivation on the structured model, not per-writer refactors.** Deriving the shape where
  `NodeData` is built keeps every transport identical by construction. Rewriting the Mermaid,
  DOT, draw.io and PlantUML writers to call the new function would have risked changing their
  byte output — the opposite of the additive contract here — and duplicated the derivation in
  each backend.
- **An additive field plus a capability, not a required field.** Field 19 is a proto3 string,
  so an older client ignores it and an older server leaves it empty. Because a server predating
  the field cannot be distinguished from one that simply omits it, the capability makes the
  distinction explicit and the gRPC service clears the field when a client does not advertise it.
  Field 18 on `RenderNode` is the durable `element_id`; the two additions occupy separate numbers.
- **Rendering kind participates in the derivation.** A case's stadium and the actor, subject and
  objective glyphs belong to a case or mixed rendering; the same roles in a tree or interconnection
  are ordinary rounded usages, as the written forms draw them. Ignoring the rendering kind would
  have made the structured field disagree with the text form for a valid view.
- **Kind-based fallback in the canvas.** A server that does not report a shape still draws as
  before, so the field can roll out without a coordinated client release.

## Public surface

| Surface | Name | Notes |
|---|---|---|
| Protobuf | `RenderNode.shape` (field 19, `string`) | One of `rectangle`, `rounded`, `stadium`, `actor`, `note`, `diamond`, `circle`, `ring`, `cross`, `bar`, `history`, `item`; empty when withheld. `RenderNode.element_id` remains field 18 |
| gRPC capability | `render_node_shape` | Response-populating capability; a service without it leaves `shape` empty on every node |
| Language server | `RenderNode.shape` in `opensysml/render`; experimental capability `openSysmlRenderNodeShape` | Same values as the wire field |
| JSON-RPC engine | `shape` on a `RenderViewResponse` node | protojson-compatible field name, no capability negotiation |
| Go client | `opensysml.RenderNode.Shape`, `opensysml.CapabilityRenderNodeShape` | Decoded from the wire field |
| Python client | `opensysml.RenderNode.shape`, `CAPABILITY_RENDER_NODE_SHAPE` | |
| Node client | `RenderNode.shape`, `CAPABILITY_RENDER_NODE_SHAPE` | Generated TypeScript stub regenerated |
| Java client | `RenderedView.Node.shape()`, `Capabilities.RENDER_NODE_SHAPE` | Generated Java messages come from the schema at build time |
| Rust client | `opensysml::RenderNode.shape`, `CAPABILITY_RENDER_NODE_SHAPE` | Generated prost message and descriptor regenerated |
| Julia and MATLAB clients | `CAPABILITY_RENDER_NODE_SHAPE` | Capability declaration; their typed render records carry the durable `elementId` but no shape field |

No written-form field, file format or golden changes: Mermaid, DOT, draw.io, PlantUML and the
text form are byte-for-byte what they were. The protobuf addition is wire-compatible; regenerated
stubs are committed for Go, Python, Node and Rust.

A maintainer merging this tree with the other wire additions must regenerate the stubs **once**
from the single merged `api/proto/sysml.proto` — with the durable render ids, the document
projection and streaming additions, and the non-keeping parse option — rather than taking either
branch's generated files. Field numbers are assigned in that merged schema: `RenderNode.element_id`
is 18 and `RenderNode.shape` is 19.

## How the acceptance criteria are locked

| Criterion | Test |
|---|---|
| A fixture with a case definition, a requirement definition and an actor | `internal/ir/view/testdata/shape-nodes.sysml` |
| A structured render reports a shape for each of those nodes | `internal/ir/view/shape_test.go` `TestNodeDataCarriesNotationShape`; `internal/frontend/grpc/render_node_shape_test.go` `TestRenderViewCarriesNodeShape`; `internal/frontend/lsp/render_node_shape_test.go` `TestRenderNodesCarryNotationShape` |
| The canvas draws the stadium and actor glyphs from the field | `editors/vscode/src/webview/canvas.test.ts` "drawCanvas draws the notation glyphs the server reports"; `layout.test.ts` pins the shape mapping and its fallback |
| Written-form byte identity is unchanged | `internal/ir/view/render_test.go` `TestGoldenRenderings` runs against the unchanged goldens; `shape_test.go` `TestNodeShapeMatchesWrittenNotation` pins the correspondence between the derived shape and the Mermaid writer's shape for the same nodes |
| Capability gating and client parity | `TestRenderViewWithholdsNodeShapeWithoutCapability`, `TestRenderNodeShapeCapabilityIsTheNewest`, `tests/hygiene/client_capabilities_test.go` `TestClientCapabilityChecklist` and `TestCapabilityDecodesAreRead` |
| Every transport and client decodes the field | `client/opensysml/render_view_internal_test.go`, `client/python/tests/test_document.py`, `client/node/test/render-view-wasm.test.ts`, `client/rust/opensysml/src/document.rs` render-view tests |
| The Go conformance wrapper preserves both additions | scenarios `render_view/a_node_reports_its_notation_shape` and `render_view/nodes_ports_and_edges_carry_durable_element_ids` in `conformance/scenarios/17-render-view.json`, run by `tools/cmd/conformance` over gRPC, Connect, Connect-JSON and the Go package client; the first pins shape, the second pins `element_id` on node, port and edge |

## Verification

The checks recorded for this tree:

- `gofmt -l .` is empty; `go build ./...` and `go vet ./...` pass; the Go and TypeScript
  comment-length gates over the diff print nothing. All stubs are regenerated from one schema in
  which `RenderNode.element_id` is field 18 and `RenderNode.shape` is field 19.
- `make proto-buf`, `make python-proto`, `make proto-ts`, `make proto-rust`, `make proto-lint` and
  `make proto-breaking` pass; the last reports no breaking schema change against the develop
  baseline. `make spec-compliance-check` passes (16 harness tests, 57 sources well-formed).
  `changelog.py check`, `check-doc-ids.py`, `check-doc-figures.py` and `git diff --check` pass.
  `make docs-check` is blocked by a broken link in `docs/project/third-party-notices.md`; its
  other three checks pass when run directly.
- View: `TestNodeDataCarriesNotationShape`, `TestNodeShapeMatchesWrittenNotation`,
  `TestNodeShapeUnknownAndShapeSet` and `TestGoldenRenderings` pass. The case-role derivation is
  gated on case and mixed renderings, and the parity table includes tree and interconnection rows
  for actor, subject and objective. The installed Graphviz warns about a non-ASCII character in
  `TestQuotedNameEscapesDecodeInLabels`, which fails that one test; every other view test and all
  byte-identity goldens pass.
- gRPC: the shape-carried, shape-withheld and capability-order tests pass. A whole-package run has
  no pass: a 20-minute timeout occurred in an edit test with no assertion failure, and the lineage
  test named in a loaded run passes alone in 41 seconds.
- Language server: `TestRenderNodesCarryNotationShape`,
  `TestInitializeAdvertisesTheRenderNodeShapeCapability`, `TestRenderCollapsedKeepsBoundaryPorts`,
  `TestRenderMixedCarriesConnectorPortData` and `TestRenderTakesAPortDisplay` pass. The recorded
  `collapsed-ports.json` carries only added `shape` and `elementId` lines, one per node. The full LSP suite passes
  in 242 seconds.
- WebAssembly: `TestEngineRenderView` passes with the parity helper copying both `Shape` and
  `ElementID` on nodes, ports and edges.
  MCP: `go test ./internal/frontend/mcp/runtimetools` passes with the capability mapped to
  `render_model_view`. Go client: proto-decode and capability-parity tests pass.
- Conformance: both render-view scenarios pass over gRPC, Connect, Connect-JSON and the Go
  package client — one pins shape, the other pins `element_id` on node, port and edge. With the
  wrapper's `Shape` copy removed the shape scenario fails with an empty shape list, so the
  assertion is load-bearing; `go test -C tools ./cmd/conformance` passes. `tests/grpc` passes in
  321 seconds.
- VS Code: typecheck passes and `npm test` reports 473 passed, 0 failed. The Node client's
  `render-view-wasm.test.js` reports 3 passed; the Python client's render-decode test reports 1
  passed. `tests/hygiene` passes in 43 seconds and `client/opensysml` passes in 425 seconds.
- The whole-tree `go test ./...` command has no recorded pass; verification is the sum of these
  package-level runs.

## Known limitations and follow-ups

- The canvas draws `note`, `rounded` and `item` shapes as their existing kind-based boxes; those
  tokens are reported accurately but do not add a folded corner or a new border. Only the stadium
  and actor glyphs are new drawings, which is what a canvas needs for the case, requirement and
  actor forms.
- Requirement nodes report `rectangle` for a definition and `rounded` for a usage, matching the
  written forms; a tool that wants a requirement-specific badge can layer it from `kind`.
- The capability is output-only: there is no request field to ask for a shape or to draw by one.
- Rust and Java client suites have no recorded run: they need a Rust toolchain and a Java runtime
  targeting release 17. Their sources, generated stubs and conformance writer are covered by the
  Go-side hygiene checks; running those suites is a follow-up.

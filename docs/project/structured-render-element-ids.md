# Durable element identity in structured renderings

A rendered view has two identities. The `id` of a node or port is local to one
rendering: a canvas draws `n0`, an edge names `n0.0`, and the next rendering
starts numbering again. The *effective element id* is the durable identity of
the declaration the node was drawn from — the value the `/el/` route resolves
and the `{id}` placeholder writes into a source link. A structured canvas that
lets a user select a box and reveal the declaration needs the second identity
in the data it receives; deriving it again from a name or a source span means
re-implementing the engine's identity rules on every client.

This record describes adding that identity to the structured rendering data at
the view layer and carrying it through every transport and client.

## State at the baseline

At `4d67c7a98f` (the 2026-11-18 nightly) and the stable `v0.12.1` tag
(`b2f0e9b802`), `api/proto/sysml.proto` had:

- `RenderNode`: the render-local `id`, descriptive fields, `ports`, an `origin`
  `Span`, geometry, style and, at the end, `parallel = 17`. No durable id.
- `RenderPort`: `id`, `name`, `type`, `direction`, `inherited`. No durable id and
  no origin.
- `RenderEdge`: `from`, `to`, the port endpoints, label, name, kind, `origin`,
  route and style. No durable id.

The written graph forms were already complete: `internal/ir/view/links.go`
resolves each declaration's source site, whose `ID` is the effective id, and
the link template's `{id}` is consumed by D2 (`link:`), DOT (`URL=`), draw.io,
Mermaid (`click … href`, folded onto its owner box for a composite), PlantUML
and the run SVG/timeline writers. The structured form was the only canvas that
could not address an element by that id. No upstream issue or pull request
proposed the field at either baseline; the capability is implemented directly
in this tree.

## What the change implements

The renderer resolves the declared element's identity once, where it already
has the model and resolver, and stores it on the view model:

- `internal/ir/view/element_ids.go` factors the origin-to-identity lookup out
  of `Renderer.Sites` into `Renderer.identityOf`, and
  `Renderer.resolveElementIDs` fills every node, compartment row, port and edge
  after a rendering is built, memoized by origin. An unnamed connector the
  index names only by its owner falls back to the positional identity the
  element form gives its declaration, so connectors under one owner no longer
  share an id; because `Sites` reads the same lookup, the written `{id}` anchor
  carries that id too.
- `view.Node`, `view.Port`, `view.Edge`, `view.NodeData` and `view.EdgeData`
  carry `ElementID`. The written forms do not read the new field; they keep
  using the same identity through `Sites`. Their bytes move only where an
  unnamed connector's `{id}` anchor now names its declaration instead of the
  shared owner name.
- The gRPC response, the JSON engine's protojson-shaped response and the LSP
  `opensysml/render` result each report the field, and every client that
  exposes the three types decodes it.

## Design decisions and alternatives rejected

- **Add a field, do not redefine `id`.** `id` remains the render-local graph
  identity an edge names. Reusing it for the durable id would break every
  canvas that builds `from`/`to` links from it.
- **Resolve in the view layer, not in each frontend.** All frontends render
  through the same `view.Renderer`, which already holds the resolver and model
  used by `{id}`. Computing identity in gRPC, engine and LSP separately would
  duplicate the rules for annotations, library-fixed ids and anonymous
  members, and would drift. Sharing `identityOf` with `Sites` makes the
  structured field and the written anchor the same value by construction.
- **One post-render walk, not changes at every constructor.** Nodes, ports and
  edges are built in many renderers, including folded compartment rows and
  action pins. A single walk over the finished rendering covers all of them,
  including the clones a compartment fold makes, without touching immutable
  construction sites.
- **No new capability flag or version gate.** The protobuf field is additive;
  an older server simply omits it and a client sees an empty string. request
  fields, RPC names and written-form bytes are unchanged.
- **No origin on `RenderPort`.** The ask is for an addressable id; adding a
  second wire field to every port would enlarge the message without adding
  addressing power. Ports already expose the id and their endpoints' `id`s.
- **No shape field.** Notation shape is a separate surface and is deliberately
  out of scope here.

## Public surface

| Surface | Name | Wire/JSON name |
|---|---|---|
| Protobuf | `RenderNode.element_id = 18`, `RenderPort.element_id = 6`, `RenderEdge.element_id = 11` | `element_id` in protobuf, `elementId` in protojson |
| Go client (`client/opensysml`) | `RenderNode.ElementID`, `RenderPort.ElementID`, `RenderEdge.ElementID` | decoded from `element_id` |
| Go view data (`internal/ir/view`) | `Node.ElementID`, `Port.ElementID`, `Edge.ElementID`, `NodeData.ElementID`, `EdgeData.ElementID` | transport-independent |
| JSON engine (`sysml-engine`, `sysml-wasm`) | `JRenderNode.ElementID`, `JRenderPort.ElementID`, `JRenderEdge.ElementID` | `elementId` |
| LSP `opensysml/render` | `RenderNode.ElementID`, `RenderPort.ElementID`, `RenderEdge.ElementID` | `elementId` |
| Node client | `RenderNode.elementId`, `RenderPort.elementId`, `RenderEdge.elementId` | decoded protojson |
| Python client | `RenderNode.element_id`, `RenderPort.element_id`, `RenderEdge.element_id` | decoded protobuf |
| Rust client | `RenderNode.element_id`, `RenderPort.element_id`, `RenderEdge.element_id` | decoded protobuf |
| Java client | `RenderedView.Node.elementId()`, `RenderedView.Port.elementId()`, `RenderedView.Edge.elementId()` | decoded `getElementId()`; the conformance mock mirrors it |
| MATLAB client | `nodes.elementId`, `ports.elementId`, `edges.elementId` | decoded engine JSON `elementId` |
| Julia client | `RenderNode.element_id`, `RenderPort.element_id`, `RenderEdge.element_id` | decoded engine JSON |
| VS Code protocol | `RenderNode.elementId?`, `RenderPort.elementId?`, `RenderEdge.elementId?` | LSP `elementId` |

Generated stubs were rebuilt from `api/proto/sysml.proto` for Go, Python
(`sysml_pb2.py`, `.pyi`), TypeScript and Rust, together with the Rust
conformance descriptor. The Java client generates its messages at Maven build
time, so it needs no committed stub. Written diagram formats (D2, DOT,
Mermaid, PlantUML, draw.io, text, CSV/TSV, run SVG/timeline) have no new field
or option; their files are byte-identical to the baseline.

## How the acceptance criteria are locked

| Acceptance | Named test |
|---|---|
| A structured render of a fixture with ports and edges | `internal/frontend/grpc/render_view_element_ids_test.go` `TestRenderViewCarriesDurableElementIDs` renders `RenderViewDemo::connections` from `conformance/fixtures/views.sysml` with full ports: three nodes, three ports, one edge |
| Every node, port and edge carries the same effective id the identity route uses | the same test compares each field with `identity.Of` on the declaration at the wire span and, for the inherited ports, on the defining port usages; `internal/ir/view/element_ids_test.go` `TestRenderedElementsCarryTheLinkIdentity` compares every field with `Renderer.Sites(...).ID` and with what `{id}` fills |
| A canvas reveal lands by that id | `TestRenderedElementsCarryTheLinkIdentity` writes the D2 form with `{id}` links and asserts an anchor filled from each node's `elementId`; `internal/frontend/lsp/render_element_ids_test.go` `TestRenderCarriesDurableElementIDs` asserts the field on the LSP JSON the VS Code canvas consumes |
| A Go fixture asserts the field | `client/opensysml/render_view_internal_test.go` `TestRenderedViewFromProtoPreservesFieldsAndPresence` |
| Written-form goldens stay byte-identical | `internal/ir/view/render_test.go` `TestGoldenRenderings` and the existing `*.golden` suites run unchanged; `internal/ir/view/links_test.go` `TestWriteWithNoSitesIsByteIdentical` still holds |
| All clients expose the field | updated decoder fixtures and assertions in `client/node/test/document.test.ts`, `client/python/tests/test_document.py`, `client/rust/opensysml/src/document.rs` and `client/julia/OpenSysML/test/runtests.jl`; the gRPC test also decodes the engine's JSON as `RenderViewResponse` |
| Identity is stable and safe on empty input | `TestRenderedElementIDsAreStableAcrossRenderings` and `TestResolveElementIDsToleratesNilAndUnlocatedOrigins` |

## Verification

Run at this head with `GOMAXPROCS=2` and Go's temporary directory on the home
filesystem:

- `gofmt -l .` — empty output, exit 0.
- `go build -p 2 ./...` — exit 0.
- `go vet -p 2 ./...` — exit 0.
- `go test -p 2 -count=1 ./internal/ir/view -run 'TestRenderedElementsCarryTheLinkIdentity|TestRenderedElementIDsAreStableAcrossRenderings|TestResolveElementIDsToleratesNilAndUnlocatedOrigins|TestGoldenRenderings|TestLinkedDiagramGoldens|TestWriteWithNoSitesIsByteIdentical'` — ok, exit 0.
- `go test -p 2 -count=1 ./internal/frontend/grpc -run 'TestRenderViewCarriesDurableElementIDs|TestRenderViewRendersInterconnectionAndPorts|TestRenderViewCarriesFoldOnlyLayoutUnplaced'` — ok, exit 0.
- `go test -p 2 -count=1 ./internal/frontend/lsp -run 'TestRenderCarriesDurableElementIDs|TestRenderTakesAPortDisplay|TestRenderMixedCarriesConnectorPortData'` — ok, exit 0.
- `go test -p 2 -count=1 ./client/opensysml -run TestRenderedViewFromProtoPreservesFieldsAndPresence` — ok, exit 0.
- `go test -p 2 -count=1 ./tests/hygiene` — ok, exit 0.
- Java: `mvn -o -pl opensysml-client -am -Dtest=RenderedViewTest -Dsurefire.failIfNoSpecifiedTests=false test` with JDK 17 — exit 0; `mvn -o -pl opensysml-conformance -am -DskipTests compile` — exit 0.
- The two comment gates over `git diff 9a71c245cd -U2 -- '*.go'` and `-- '*.proto'`, counting added comment lines past the second, both print nothing.
- `cd client/node && npm run build:tools && node --test build/test/document.test.js` — 13 tests, 13 passed, 0 failed. The TypeScript build reports a pre-existing unresolved `@openmbee/opensysml` import in `test/golden/vehicle_types.ts` until `dist` exists; the compiled test binary runs and passes.
- `cd client/python && PYTHONPATH=. python -m pytest tests/test_document.py::test_render_view_decodes_all_fields_and_optional_messages -q` — 1 passed. Two tests in that file that need a locally installed `sysml-grpc` with the `render_view_form` capability fail against the stale binary in `~/.opensysml/bin`; they are unrelated to this change.
- `make proto-lint` and `make proto-breaking` — both passed.
- `python3 scripts/changelog.py check` and `python3 scripts/check-doc-ids.py` — both passed; `go run -C tools ./cmd/doc-counts` reported the committed counts current.

A whole-tree `go test ./...` run reached `gofmt`, `go build ./...` and
`go vet ./...` cleanly, but its test phase did not complete. No whole-tree
test result is claimed here; the focused suites above are green.

## Known limitations and follow-ups

- An artifact with no resolvable declaration has an empty `elementId`; the
  render-local `id` still addresses it within the rendering. Item boxes take
  the identity of the flow whose line they sit on, and an anonymous flow's item
  box takes its owner's identity, so they are not id-less. A declared `start`
  state likewise carries its own id; an unresolved node such as a `done` the
  model does not declare is the case that comes back empty.
- The field is captured when the rendering is built. Later edits to the model
  do not change an already returned response.
- Clients that talk to an older server see an empty string. No capability
  flag distinguishes "old server" from "element with no identity"; the
  renderer's `origin` remains available where that distinction matters.
- Rust, Julia and MATLAB toolchains are not installed here. Their decoder
  changes and test fixtures were updated alongside the generated stubs, but
  those suites were not executed; their CI jobs run them. Java was run with a
  JDK 17 toolchain outside the default `PATH`; the default JRE has no `javac`
  and cannot run the Maven test.
- No in-tree agent skill describes the structured render node, port or edge
  fields. The service and client skills in the separate `OpenSysML-MDK-Skills`
  repository must be re-verified against these fields and updated in a
  companion pull request; that work is not part of this change and is not
  claimed as done.
- Notation shape on the structured node is a different surface and is not
  implemented here.

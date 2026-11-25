# Indexed connection ends in the lowering and the element form

## The missing capability

SysML v2 has no spelling for selecting one element of a collection-valued feature
at a connector end. OpenSysML admits the extension `connect s.y#(1) to k.u`, where
`s.y` is a three-element port and `#(1)` selects its first element. Parsing,
resolution, type checking, evaluation and formatting of that end already work: the
end carries an `sysx:endElement` expression node beside the standard
`ReferenceSubsetting` of the whole feature, and the decoder writes the `#( … )`
back from that node. Upstream evidence: issue
[#902](https://github.com/Open-MBEE/OpenSysML/issues/902) and pull request
[#965](https://github.com/Open-MBEE/OpenSysML/pull/965).

Two intermediate representations still dropped the selector, and that is what this
change completes:

- The connect lowering kept only the port path. `endName` deliberately reduced
  `s.y#(1)` to `s.y` because routing delivers a message to a *port of an object*,
  never to one element of a port. That is the right routing address, but it left
  no representation of the written end for a consumer of the lowering.
- The analysis model form rendered the end's selector with the expression
  printer. An index whose expression is a body with local declarations has no
  spelling there, so the end came out as the bare port path; a quoted port name
  flattened to the same path as a nested chain of the same segments.

## State at the pinned baseline

At `4d67c7a98f` (nightly of 2026-11-18, based on v0.12.1) and at v0.12.1:

- `internal/ir/lower/connection.go` declared `Connection.Ends []string`; the
  helper `endName` called `FeaturePath(end.AttachedTarget())` and returned the
  port path, dropping `end.AttachedIndex()`.
- The binding lowering already carried an index (`lower.BindingEnd.Index`), so
  bindings selected with their own index while connections did not.
- `internal/translate/export/rdf_out.go` wrote the connector's derived features
  from the `reference` term the end's `ReferenceSubsetting` reached — for an
  indexed end, the whole feature IRI. `sysx:endElement` was written only on the
  connector end, never reached from the connection's derived properties.
- The decoder ignored those derived connector properties when writing notation,
  so the written form was already round-trip stable; the gap was purely in what
  the two intermediate representations exposed.

## What this change implements

- The connect lowering now keeps each end as a `lower.ConnectionEnd` carrying the
  port `Path` that routing addresses, the `Index` AST node of the selector and
  the `Target` node the end attaches to. `TargetAt` and `IndexAt` give the
  written extents of both. Keeping the target node preserves quoting,
  qualification and segment boundaries that `Path` flattens away.
- The connector's derived properties (`sysml:relatedFeature`,
  `sysml:sourceFeature`, `sysml:targetFeature`, `sysml:source`, `sysml:target`)
  keep naming the feature the end's `ReferenceSubsetting` reaches, which is what
  metamodel reflection derives. This holds for a lone-name flow end as well:
  with no `ReferenceSubsetting`, reflection answers no related feature and the
  export writes none. An indexed end carries its selector on the end element the
  ordered `sysml:connectorEnd` links reach, in `sysx:endElement`, so a reader
  follows `connectorEnd[0]` → `sysx:endElement` to find `p#(1)` while a reader of
  the standard vocabulary still sees `p` connected.
- The analysis model form writes each end name and selector from their source
  extents in the end's declaration scope, the way every other expression is
  read. `ConnectionForm.Selectors` records, in end order, the position and the
  index of every indexed end, so the selector survives even where no source
  text is available to spell it. `Ends` carries the written spelling when it
  can be read: `p#({ in i : Integer; i + 1 })` and `'a.b'#(1)` stay distinct from
  `a.b#(1)`.
- Routing, the formatter and the decoder are otherwise unchanged. The decoder
  continues to ignore derived connector properties when writing notation, and
  accepts a graph that names a graph-defined end feature from such a property,
  though this writer does not produce one.

## Design decisions and alternatives

- **Structured end record over a parallel index slice.** A `[]ConnectionEnd`
  with `Path`, `Target` and `Index` keeps one canonical representation per end
  and makes every consumer choose explicitly between the routing address and
  the written end. A parallel `[]ast.Node` next to `Ends` would have been a
  smaller diff but introduces two slices that can drift. The shape mirrors
  `lower.BindingEnd`, which already carries `Index`.
- **Keep `Path` as the routing address.** All runtime routing sites read
  `ConnectionEnd.Path`; none renders the written end. An indexed end still joins
  the port, so delivery semantics are untouched, and a regression test exercises
  that directly.
- **Keep the derived properties on the referenced feature.** Naming the minted
  end feature there instead would have disagreed with `deriveConnectorRelatedFeature`
  and given a lone flow end a reference where reflection answers none. The
  standard properties are derived from the ends' `ReferenceSubsetting` on both
  surfaces; the extension stays in the `sysx:` namespace on the end element.
- **Read the written end from source, not from the expression tree.** A body with
  local declarations has no `ExprPrinter` spelling, and a quoted `'a.b'` is not
  the chain `a.b`. The source extent is what the notation actually wrote; the
  structured selector keeps the index when the extent cannot be read.
- **Additive change to the analysis form.** `graphs:1` keeps its version: `Ends`
  still lists one string per end, and the new `selectors` field adds the index
  at its end position. Existing readers see the written end where it can be
  spelled; readers needing the expression read it structurally with its span.
- **No wire or protobuf change.** The selector travels in the existing RDF and
  API-JSON element form under the `sysx:` extension namespace and in the
  in-process Go lowering and model form; no RPC, client method, capability flag
  or stable public API type was added, so no client needs regenerating.

## Public surface

- Go (internal): `lower.Connection.Ends` is now `[]lower.ConnectionEnd`; each end
  has `Path string`, `Target ast.Node`, `Index ast.Node`, `TargetAt()` and
  `IndexAt()`. This is an internal package; no shipped client depends on it.
- RDF: no new predicate. The existing `sysx:endElement` expression node stays on
  the end element reached from `sysml:connectorEnd`. The connector's derived
  `sysml:relatedFeature`, `sysml:sourceFeature`, `sysml:targetFeature`,
  `sysml:source` and `sysml:target` keep naming the referenced feature, as
  reflection does.
- API JSON (`sysml -convert api-json`): `sourceFeature`, `targetFeature`,
  `relatedFeature`, `source`, `target` and `connectorEnd` answer the same ids as
  `GetElementProperties` for both plain and indexed ends. An indexed end is
  found through `connectorEnd`, whose element states `sysx:sourceText "p#(1)"`
  and `sysx:endElement`; a plain end has no `sysx:endElement`. The element
  array's shape and every existing key are unchanged, so the format needs no
  version bump.
- Analysis model form (`graphs:1`): a connection carries `selectors`, one entry
  per indexed end with its end position and an index `{text, span}` read from
  source. `ends` carries the written spelling, including quoted names and body
  indices, when the source text is available. Routing still uses the paths.
- Notation: byte-identical. No formatter or parser change.

The RDF and API-JSON mappings are documented as experimental in
[the RDF mapping reference](../reference/rdf-mapping.md); this addition stays
inside that status and does not establish a compatibility promise.

## Acceptance locked by named tests

| Criterion | Test |
|---|---|
| A plain and an indexed connection state the same derived `sourceFeature`, agreeing with `GetElementProperties` | `internal/frontend/grpc/element_properties_indexed_end_test.go:TestIndexedEndsAgreeBetweenAPIJSONAndGetElementProperties` |
| Indexed flow ends with and without a `ReferenceSubsetting` agree with reflection, and the index is on `connectorEnd` | the same test |
| An indexed end's index is reachable from the ordered `connectorEnd` links under `sysx:endElement` | the same test, and `tests/export/indexed_connector_ends_test.go:TestIndexedConnectionKeepsStandardEndpointsAndCarriesTheSelector` |
| An undefined `expr:` object on a derived connector property is refused, not silently accepted | `tests/export/indexed_connector_ends_test.go:TestIndexedConnectionRefusesAnUndefinedEndReference` |
| The lowered connection keeps `Path "p"` beside the index and the written target | `internal/ir/lower/connection_test.go:TestLowerIndexedConnectionEndKeepsItsSelector`, `TestLowerIndexedEndKeepsItsWrittenTarget` |
| Routing still addresses the port, not the selected element | `internal/exec/runtime/routing_test.go:TestIndexedEndStillRoutesToItsPort` |
| A body index is written from its source extent, with its span, in the analysis form | `internal/exec/analysis/modelform/graphs_test.go:TestGraphsCarriesIndexedEndSelectorsFromTheirExtent` |
| A quoted indexed port stays distinct from a nested chain of the same segments | `internal/exec/analysis/modelform/graphs_test.go:TestGraphsDistinguishesAQuotedIndexedPortFromANestedOne` |
| Written form stays byte-identical through Turtle and API JSON | `tests/export/indexed_connector_ends_test.go:TestIndexedConnectorEndsCarryTheirIndexAsSysxEndElement`, `TestIndexedConnectorEndsRoundTripThroughAPIJSON`, and both element forms in the parity test |
| The existing end-index contract still holds | `tests/export/indexed_connector_ends_test.go:TestIndexedConnectorEndsCarryTheirIndexAsSysxEndElement`, `TestIndexedConnectorEndsRoundTripThroughAPIJSON` |

## Verification

Gates run on the corrected change:

- `gofmt -l .` — empty output.
- `go build ./...` — clean.
- `go vet ./...` — clean.
- Targeted export, lower, modelform, routing and element-property tests — see the
  commit message and the report for the exact commands and results.

## Known limitations and follow-ups

- The index is read from source text and kept as an expression with a span; a
  graph with no registered source for an indexed end has an empty selector text
  but still records that the end is indexed.
- No new field was added to the connector element itself in RDF/API JSON. If a
  consumer needs the selector without walking to the referenced end, a future
  additive extension predicate could restate it there; it was not needed for
  this capability and would duplicate the canonical expression node.
- The change applies to indexed connector ends, including indexed flow ends.
  Succession and transition ends do not state derived `sourceFeature` values in
  this mapping, and message ends use a separate end representation; neither was
  changed.
- The pilot implementation rejects the `#(` notation entirely, so there is no
  external oracle for the element form of an indexed end. The contract is
  covered by the in-tree tests named above.

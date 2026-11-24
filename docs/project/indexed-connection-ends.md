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
- The derived connector properties in the RDF and API-JSON element form
  (`sysml:relatedFeature`, `sysml:sourceFeature`, `sysml:source`,
  `sysml:targetFeature`, `sysml:target`) pointed straight at the whole feature.
  A consumer reading the connection's `sourceFeature` saw the same value for
  `connect p#(1) to q` as for `connect p to q`, which is exactly the distinction
  a downstream tool needs when it reconstructs a connector from the element form
  rather than from notation.

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
  connector end, never reached from the connection's `sourceFeature`.
- The decoder ignored those derived connector properties when writing notation,
  so the written form was already round-trip stable; the gap was purely in what
  the two intermediate representations exposed.

## What this change implements

- The connect lowering now keeps each end as a `lower.ConnectionEnd` carrying the
  port `Path` that routing addresses and the `Index` AST node of the selector,
  with `Notation()` rendering the end as written. The selector is kept
  losslessly: it is the expression node, not a pre-rendered string.
- For an indexed connector end, the derived connector properties now name the
  end feature that carries `sysx:endElement`, the way a chained end's properties
  name the feature carrying its `FeatureChaining` links. A consumer following
  `sourceFeature` reaches an element stating `sysx:sourceText "p#(1)"` and
  `sysx:endElement` pointing at the index expression, while that end's
  `ReferenceSubsetting` still reaches the whole port `p` for readers of the
  standard vocabulary.
- Routing, the formatter and the decoder are otherwise unchanged. The decoder
  continues to ignore derived connector properties when writing notation, and a
  graph that names a minted end feature from such a property is accepted without
  demanding a `qualifiedName` the minted feature has no reason to carry.

## Design decisions and alternatives

- **Structured end record over a parallel index slice.** A `[]ConnectionEnd`
  with `Path` and `Index` keeps one canonical representation per end and makes
  every consumer choose explicitly between the routing address and the written
  end. A parallel `[]ast.Node` next to `Ends` would have been a smaller diff but
  introduces two slices that can drift. The shape mirrors `lower.BindingEnd`,
  which already carries `Index`.
- **Keep `Path` as the routing address.** All runtime routing sites read
  `ConnectionEnd.Path`; none reads `Notation()`. An indexed end still joins the
  port, so delivery semantics are untouched, and a regression test exercises
  that directly.
- **Name the end feature from the derived properties rather than minting a new
  element.** The end feature already states both the index (`sysx:endElement`)
  and the referenced feature (through its `ReferenceSubsetting`), so a new
  "indexed feature" element would duplicate data that has one canonical home.
  It would also have introduced an element with no notation of its own that the
  element-form reader would have to classify.
- **Keep the `ReferenceSubsetting` to the whole feature.** Standard readers see
  `p` connected, as the extension's contract requires; only the derived
  properties carry the distinction. Writing the index into the
  `ReferenceSubsetting` would have changed the meaning of the standard
  vocabulary, not just added an extension beside it.
- **No wire or protobuf change.** The selector travels in the existing RDF and
  API-JSON element form under the `sysx:` extension namespace and in the
  in-process Go lowering; no RPC, client method, capability flag or stable
  public API type was added, so no client needs regenerating.

## Public surface

- Go (internal): `lower.Connection.Ends` is now `[]lower.ConnectionEnd`; each
  end has `Path string`, `Index ast.Node` and `Notation() string`. This is an
  internal package; no shipped client depends on it.
- RDF: no new predicate. The existing `sysx:endElement` expression node is now
  reachable from a connector's derived `sysml:relatedFeature`,
  `sysml:sourceFeature`, `sysml:targetFeature`, `sysml:source` and `sysml:target`
  properties when the corresponding end is indexed.
- API JSON (`sysml -convert api-json`): for an indexed connection end,
  `sourceFeature` references the end element that carries `sysx:endElement` and
  `sysx:sourceText "p#(1)"`, instead of the port element; `targetFeature`,
  `relatedFeature`, `source` and `target` follow their indexed ends the same
  way. A plain end still references the feature directly. The element array's
  shape and every existing key are unchanged, so the format needs no version bump.
- Analysis model form (`ConnectionForm.Ends`): the recorded end spelling is the
  written one, `p#(1)`, instead of the bare port path `p`. This is the model form
  analysis engines consume; it does not affect message routing.
- Notation: byte-identical. No formatter or parser change.

The RDF and API-JSON mappings are documented as experimental in
[the RDF mapping reference](../reference/rdf-mapping.md); this addition stays
inside that status and does not establish a compatibility promise.

## Acceptance locked by named tests

| Criterion | Test |
|---|---|
| The fixture `part def C { port p[3]; port q; connect p#(1) to q; }` converts with the selector on the end's referenced feature | `tests/export/indexed_connector_ends_test.go:TestIndexedConnectionCarriesTheSelectorOnItsSourceFeature` |
| A plain connection has no selector on its `sourceFeature` (negative control) | the same test, plain-model half |
| An indexed second end is carried on `targetFeature` | `TestIndexedConnectionCarriesTheSelectorOnItsSourceFeature`, both-indexed-ends half |
| An undefined `expr:` object on a derived connector property is refused, not silently accepted | `TestIndexedConnectionCarriesTheSelectorOnItsSourceFeature`, undefined-reference half |
| The lowered connection keeps `Path "p"` beside the index and renders `p#(1)` | `internal/ir/lower/connection_test.go:TestLowerIndexedConnectionEndKeepsItsSelector` |
| Routing still addresses the port, not the selected element | `internal/exec/runtime/routing_test.go:TestIndexedEndStillRoutesToItsPort` |
| Written form stays byte-identical through Turtle and API JSON | `tests/export/indexed_connector_ends_test.go:TestIndexedConnectionCarriesTheSelectorOnItsSourceFeature` (both graph forms read back to `connect p#(1) to q;`), and the pre-existing `TestIndexedConnectorEndsCarryTheirIndexAsSysxEndElement` / `TestIndexedConnectorEndsRoundTripThroughAPIJSON` |
| The existing end-index contract still holds | `tests/export/indexed_connector_ends_test.go:TestIndexedConnectorEndsCarryTheirIndexAsSysxEndElement`, `TestIndexedConnectorEndsRoundTripThroughAPIJSON` |

## Verification

Gates run on this change:

- `gofmt -l .` — empty output.
- `go build ./...` — clean, exit 0.
- `go vet ./...` — clean, exit 0.
- `go test ./tests/export -run 'TestIndexedConnectorEnds|TestIndexedConnection' -count=1` — `ok`; the indexed-end export suite.
- `go test ./internal/ir/lower -run 'TestLowerIndexed|TestPeerPorts|TestLowerVariant' -count=1` — `ok`.
- `go test ./internal/exec/runtime -run 'TestIndexedEndStillRoutesToItsPort|TestUndirectedPortsReceiveInEitherDirection' -count=1` — `ok`.
- `go test ./internal/ir/lower/... ./internal/exec/analysis/modelform/... -count=1` — `ok`.
- `go test ./tests/export -run TestLibraryFilesComeBackFromTheGraphAlone -count=1` — `ok`.
- `go test ./internal/exec/runtime -count=1` — `ok`.
- `python3 scripts/changelog.py check` — clean; `python3 scripts/check-doc-ids.py` — no work-item labels.
- `go test -p 1 ./tests/hygiene -count=1` — `ok`, 37.720s.
- `go test -C tools ./referee/apijson -count=1` — `ok`, 5.870s.
- `go run -C tools ./cmd/pilot-apijson-diff -check` — not runnable: the pilot JSON exporter under `build/pilot-json-exporter/` is absent and must be provisioned with `scripts/download-pilot-json-exporter.sh`.

The training, pilot, specification-example and normative-schema corpora are
download-gated and were not present, so those gates were not exercised.
`TestUnresolvedRequirementConstraintReferenceHasTypedReason` needs the downloaded
training file `examples/sysml-v2-training/32. Requirements/Requirement Groups.sysml`.
The view tests need a Graphviz `dot` that emits no warning, and the gRPC lifecycle
tests use wall-clock deadlines sensitive to concurrent load. A bounded
`go test -p 1 ./internal/translate/export ./tests/export -count=1 -timeout 20m`
run gave `ok` for `tests/export` (855.900s); `internal/translate/export` reported
only the corpus-dependent failure above (105.120s). Whole-tree `go test ./...`
runs exceeding their time budget reported only those categories of failure.

## Known limitations and follow-ups

- The element form distinguishes an indexed end by *where* `sourceFeature`
  points rather than by a new named field. A consumer that already understands
  chained end features follows the reference and reads `sysx:endElement`; one
  that assumes `sourceFeature` is always a declared feature will see an end
  feature for indexed ends. That is the same shape a chained end already has,
  so the two cases are uniformly handled.
- No new field was added to the connection element itself. If a consumer needs
  the selector without walking to the referenced element, a future additive
  extension predicate could restate it there; it was not needed for this
  capability and would duplicate the canonical expression node.
- The change applies to indexed connector ends, including indexed flow ends.
  Succession and transition ends do not state derived `sourceFeature` values in
  this mapping, and message ends use a separate end representation; neither was
  changed.
- The pilot implementation rejects the `#(` notation entirely, so there is no
  external oracle for the element form of an indexed end. The contract is
  covered by the in-tree tests named above.

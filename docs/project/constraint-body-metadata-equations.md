# Verbatim equation mapping annotations in constraint bodies

This record describes the engine-side carrier for equations that a downstream
Modelica generator needs but that this language implementation cannot express as
SysML expressions. It is written for a reader who has only the tree: no session,
conversation or private bookkeeping is needed to follow it.

## The missing capability and who needs it

A steam-generator model carries three equations that a Modelica generator emits
into a generated class body:

- `Medium.temperature(state_a)` — a call into a thermodynamic-state package,
- `inStream(port_a.h_outflow)` — a stream value at a connector,
- `noEvent(if abs(m_flow) > 1.0 then m_flow else 1.0)` — a regularising
  conditional.

The generator consumes a SysML twin and writes Modelica. To generate that class
rather than ship a hand-written one, it needs the equations to be authored in
the model, readable back from the model, and preserved character for character
through the engine's export. A carrier that rewrites `then` into `?`, drops the
package qualification, or loses an escape sequence cannot drive generation or a
deterministic differential against a golden model.

## State at the pinned baseline

At baseline [`4d67c7a98f`](https://github.com/Open-MBEE/OpenSysML/tree/4d67c7a98f)
(described as `v0.12.1-2379-g4d67c7a98f`, a nightly after the `v0.12.1` stable
tag) the generic machinery was already present:

- user-defined metadata definitions and `@`-annotations with String bindings
  parsed, including annotations among a constraint body's members;
- `.metadata#(i).body` evaluated to the bound String;
- the API-JSON writer emitted the binding as its `value` property and the RDF
  writer emitted `sysml:value`, and both read back.

What was absent was a *contract* tying that machinery to this use: no documented
convention, no fixture that locks the three equation forms in the AST, no
conformance case that evaluates the carried values, no export test over the
exact strings (including hostile characters), no negative case, and no record
of the expression-route gap. The direct expression forms did not exist either:
the language spells a conditional `if c ? a else b`, not Modelica's
`if c then a else b`, and neither `inStream`, `noEvent` nor a thermodynamic-state
package call is a builtin. No upstream issue or pull request at that baseline
addressed those forms.

## What this change implements

A documented, tested engine surface for the verbatim route:

- a convention: a user-defined metadata definition with a `body : String`
  feature, attached to the constraint body that states the equations;
- a golden AST fixture locking the three equations as `LiteralString` nodes of
  `@EquationMapping` annotations in a constraint body;
- two execution conformance cases: one deciding the constraint while it carries
  the annotations, one reading the three equation Strings back through
  `.metadata#(i).body`;
- export tests proving the exact strings survive the API-JSON writer, the RDF
  writer and the graph-to-notation reader, including quotes, backslashes,
  ampersands, angle brackets, newlines and non-ASCII;
- a parser negative case for an unterminated equation string and a runtime
  robustness suite for the failure modes (annotations alone are not a verdict,
  names in the carried text are never resolved, hostile escapes are data);
- a skipped test recording the direct expression route as a known limitation;
- a reference-page section and compliance-map rows describing the surface.

No production Go code was added or changed: parse, evaluation and export of a
verbatim String binding were already correct at the baseline, and the change
makes that behavior a named, measured contract rather than an incidental one.
This is stated plainly rather than dressed as a new parser or runtime feature.

## Design decisions and alternatives rejected

- **Verbatim annotation rather than a new expression form.** Giving `inStream`,
  `noEvent` and a medium-state call first-class syntax needs new grammar, AST
  nodes, evaluation and export, plus a reconciliation of the `if … then … else`
  spelling with the language's `?` conditional. That is a separate, larger
  surface. It is recorded as a limitation with a skipped test, not silently
  dropped.
- **No new standard-library definition.** The engine does not own Modelica
  vocabulary; the downstream project owns its mapping library. Adding a library
  file would also change the library digest and force re-recording of unrelated
  differential baselines. The annotation is defined in the model that uses it.
- **No new RDF predicate, API-JSON field, RPC, proto message or capability
  flag.** The existing `sysml:value` / `value` pair is the specification's
  carrier for a feature value and already preserves the string exactly. A
  dedicated marker would split the contract into two readers and require client
  changes with no consumer. No wire or public Go API surface was added, so no
  client needed updating.
- **No magic-name recognition in the runtime.** A helper that recognises a
  particular annotation name would couple the engine to a downstream
  convention and pick the wrong element when names repeat in different scopes.
  Reading `.metadata#(i).body` is the ordinary, already-public route.

## Public surface added

| Surface | Name / shape | Versioning |
|---|---|---|
| Notation | `metadata def M { attribute body : String; }` and `@M { body = "<equation>"; }` among a constraint body's members | Standard SysML textual notation; no new token or grammar production |
| RDF / Turtle | the binding is a `sysml:ReferenceUsage` with a `sysml:value` literal | The RDF mapping is experimental; its vocabulary may change without a compatibility path (`docs/reference/rdf-mapping.md`, Status) |
| API element form | the same binding's `value` property | Same experimental status as the RDF mapping |
| Evaluation | `.metadata#(i).body` evaluates to the exact String | Existing metadata-access expression; no new method or field |

There is no new RPC, gRPC field, protobuf message, capability flag, exported Go
method, AST node type, CLI option or environment variable. The reference-page
section is the normative description of the convention.

## How each acceptance criterion is locked

| Criterion | Named test |
|---|---|
| A model with such a constraint body parses clean | `tests/parser` `TestGolden/constraint_verbatim_equation_mapping.sysml`; `internal/translate/export` `TestVerbatimEquationBodyIsNotAnExpression` |
| Its expression evaluates | conformance `constraint_verbatim_equation_mapping` (the body's condition decides) and `metadata_verbatim_equation_value` (the three Strings read back); `internal/exec/runtime` `TestRuntimeRobustnessVerbatimEquation` |
| API-JSON and RDF export carry the equation verbatim | `internal/translate/export` `TestVerbatimEquationCarriesThroughAPIJSONAndRDF`, `TestVerbatimEquationHostileStringsRoundtrip` |
| Golden AST fixture | `tests/parser/testdata/parse/constraint_verbatim_equation_mapping.sysml` and its `.golden`, driven by `TestGolden` |
| Negative and robustness cases | `tests/parser` `TestNegative/unterminated_verbatim_equation`; `internal/exec/runtime` `TestRuntimeRobustnessVerbatimEquation` |
| Direct expression route known and measured | `internal/translate/export` `TestVerbatimEquationExpressionRouteKnownLimitation` (skipped with reason) |

## Verification

The complete gate was run on the exact head:

- `gofmt -l .` — no output (clean).
- `go build ./...` — succeeded.
- `go vet ./...` — succeeded.
- `go test ./tests/parser -run 'TestGolden/constraint_verbatim_equation_mapping|TestNegative/unterminated_verbatim_equation' -count=1` — `ok`.
- `go test ./internal/translate/export -run 'TestVerbatimEquation' -count=1` — `ok`.
- `go test ./internal/exec/runtime -run 'TestExecutionConformance|TestRuntimeRobustnessVerbatimEquation' -count=1` — `ok`.
- `go run -C tools ./cmd/doc-counts -check` — `doc-counts: already current`.
- `python3 scripts/changelog.py check` — no output (clean).
- `python3 scripts/check-doc-ids.py` — `No internal work-item labels in reader-facing documentation`.

A whole-tree `go test ./...` was attempted. It did not complete cleanly: the
shared host hit a tmpfs disk quota and killed test processes while other
worktrees built concurrently, so several packages reported `disk quota exceeded`
or `signal: killed`. No assertion failure attributable to this change was
observed, and the packages the change touches pass the targeted runs above. The
OMG training, pilot and specification-example corpus gates and the PDF/FMI
converter gates skip while their inputs are absent, as the repository documents;
they do not cover this surface.

## Known limitations and follow-ups

- Writing the Modelica forms directly as constraint-body expressions is not
  supported; the skipped test names the exact gap. The verbatim annotation is
  the shipped surface.
- The engine carries annotation text; it does not validate that a given
  annotation is an equation mapping. A `body` that is absent, or bound to a
  non-String value, is read by the ordinary metadata rules rather than rejected
  by a dedicated diagnostic.
- The RDF and API element forms remain experimental, as the reference page
  states; a consumer should read the `value`/`sysml:value` pair rather than
  depend on an ancillary marker.

## What remains downstream

Everything below belongs to the consuming Modelica generator and is explicitly
out of scope for this repository:

- reading the annotations from the generated twin's element form and emitting
  the steam-generator class body with those equations;
- translating the generated class with `omc`;
- wiring a differential case that compiles generated versus golden and answers
  the same steady-state values within tolerance;
- making the hand-written golden the oracle rather than the shipped source.

This change supplies only the engine-side carrier and its proof: a model with
the equations parses, the values evaluate, and both exports preserve them
verbatim.

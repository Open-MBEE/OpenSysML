# Element identity annotations — design

> **Labels.** This is an engineering record. "Track D" and its sub-items (`D1`, `D3.1`–`D3.5`)
> name entries of [the roadmap](roadmap.md), where each is stated in full; a reader who only
> wants the design can ignore them.

Status: **all six phases implemented** — the library, side table, validation pass, the RDF
writer/reader identity round trip (comments and formatting preserved through the graph's
source text), the sync diff keyed by effective id with the identity-carrying Flexo fixture,
the apply of that diff to a live repository with its measurement against the real stack,
the LSP minting code action, and the OMG issue filed as
[INBOX-2510](https://issues.omg.org/browse/INBOX-2510) (see
[omg-issues.md](omg-issues.md)). It records the design agreed for
carrying stable element identity in textual notation, how that identity permeates the RDF
mapping and repository synchronization, and the plan to submit the notation for
standardization with OMG.

## The problem

The SysML v2 textual notation deliberately carries no element identity: the specification
treats text as a projection of the model, and `elementId` as a property of the abstract
syntax that interchange formats other than notation (the API's JSON, XMI) carry. That
decision has three consequences this project runs into directly:

1. **A round trip through notation loses identity.** A model read from a repository —
   the OMG API, a Flexo MMS branch — arrives with UUID element ids; saved as `.sysml` and
   re-parsed, every element has a fresh identity. Nothing in the text records which
   repository element a declaration *is*.
2. **A rename is a delete plus a create.** OpenSysML derives an element's id from its
   qualified name (`rdf.EncodeElementID`, roadmap item D3.1): readable and deterministic,
   but renaming or moving an element changes its id, so no consumer can see the rename
   as an update to the same element.
3. **Name-derived ids are a convention gamble.** The OMG API's own implementations treat
   element ids as UUIDs. Flexo MMS's `requireValidId` accepts our encoded names today,
   but the roadmap records the open risk that another conforming client rejects a
   non-UUID id outright.

## The design in one paragraph

Identity is carried **in-band, as standard user-defined metadata** — notation every
conforming SysML v2 tool already parses and preserves — and it is **opt-in per element**:
an element annotated with an id *is* that repository element; an element without one
defaults to derived, latest-wins identity exactly as today. A document is bound to a
repository once, at its root namespace, by a project annotation that element-level ids
inherit. The RDF mapping mints IRIs from the annotated id when present and from the
encoded qualified name otherwise, and the reader re-materializes the annotations on the
way back to notation, so identity survives `notation → RDF → notation`. Synchronization
against a repository then diffs by id, which is what turns a rename or a retype into an
update of the same element rather than a delete and a create.

## The metadata library

Two metadata definitions, shipped as a non-normative OpenSysML library extension (the
stdlib already carries two such files; the conformance gate counts them separately):

```sysml
standard library package IdentityMetadata {
    doc /* Binds notation to repository identity. Non-normative OpenSysML
         * extension, proposed for standardization; see the design record. */

    metadata def ElementId {
        doc /* The annotated element is the repository element with this id. */
        attribute id : ScalarValues::String;
    }

    metadata def ProjectRef {
        doc /* Elements below the annotated namespace that carry ElementId
             * resolve against this project and branch. */
        attribute projectId : ScalarValues::String;
        attribute branch : ScalarValues::String[0..1];
        attribute org : ScalarValues::String[0..1];
    }
}
```

Applied:

```sysml
package Vehicles {
    @ProjectRef { projectId = "b3f9c2e8-…"; branch = "main"; }

    part def Vehicle {
        @ElementId { id = "8f3a41d0-…"; }
        attribute mass : ISQ::MassValue;
    }
}
```

Decisions, each with its reason:

- **In-band metadata, not a sidecar file.** A sidecar (`model.sysml.ids`) keeps the text
  clean but does not travel: the moment a model passes through another conforming tool,
  the mapping is orphaned. Standard metadata survives any tool that implements the
  specification, which is the property the whole design exists for. The cost — metadata
  is visible to the language itself — is real and is handled below.
- **Opt-in per element, defaulting to latest.** Only elements whose repository identity
  matters carry an annotation. An unannotated element keeps derived qualified-name
  identity, meaning cross-commit correlation by name — "whatever the latest element of
  this name is". Annotating everything would drown models in noise and turn every sync
  into a merge conflict.
- **Project scope is declared once, at the root.** The OMG API addresses elements as
  `/projects/{p}/commits/{c}/elements/{id}`; an element id alone is not addressable.
  Repeating the project on every element is pure noise, so `ProjectRef` annotates the
  root namespace (or any package below it) and `ElementId` resolves against the nearest
  enclosing `ProjectRef`. A document with `ElementId` annotations but no enclosing
  `ProjectRef` is a validation error, since the ids bind to nothing.
- **Branch, never commit.** A ref names "latest on that branch", matching the default
  semantics; a commit id in source text goes stale on every sync. Which commit a sync
  last saw is working state of the sync tooling, not a property of the model.
- **Org names the project only when the project id cannot.** In the OMG API a
  project is a top-level UUID resource, so `projectId` alone names it, the org —
  like the server URL — stays in the sync tooling's endpoint configuration, and
  moving a project between orgs touches no model file. That is the recommended
  shape, and it is what OpenSysML mints. The optional `org` attribute exists for
  Flexo deployments whose repo ids are not globally unique: there nothing shorter
  than the pair distinguishes two same-named repos, so `org`-plus-`projectId` *is*
  the project for scope equality — unlike `branch`, which selects a version of one
  project rather than naming another. The relocation cost of that choice is
  bounded and honest: an org move rewrites the one root `ProjectRef`, never an
  element — every `ElementId` below it is untouched, so the elements keep their
  identity at the new location and sync there sees updates, not deletes plus
  creates. What no design can offer is relocation-proof identity for an id that
  was never unique without its location; a deployment that wants org moves for
  free uses UUID project ids and omits `org`.
- **String-typed ids, validated separately.** The attribute type is `String` so the
  notation stays plain; a validation pass enforces the id shape rather than the type
  system. Ids are UUIDs when minted by OpenSysML, but the pass accepts anything matching
  `[a-zA-Z0-9_-]+` (Flexo's `requireValidId`), so hand-authored and foreign ids pass.

The same path — library, encoder, side table, validation pass, writer, reader, harness and
the unbuilt sync diff — is modelled in SysML v2 in
[examples/self-model/identity.sysml](../../examples/self-model/identity.sysml), whose views
render it as a diagram.

## Semantics

### Identity resolution

An element's **effective id** is:

1. the `id` of its `ElementId` annotation, if it carries one; otherwise
2. the derived id — `rdf.EncodeElementID` of its qualified name — exactly as today.

Memberships and expression nodes derive their ids from their element's effective id
(`OwningMembershipID` appends `_om`, `ExpressionNodeID` joins the position with `_p`),
so they inherit annotated stability with no further mechanism.

### Validation (a constraint-tier pass)

- Duplicate effective ids **within one project scope**: error on both elements, naming
  each. Identity is the enclosing scope's **project** — `org` where present plus
  `projectId` (or the absent scope, for unbound documents) — plus the effective id,
  so the same id under two different projects is legal; it names two different
  repository elements. The `branch` plays no part in scope equality: a branch
  selects a version of one element, not another identity, so two `ProjectRef`s
  naming one project on different branches are one scope for duplicate validation,
  for mixed-graph refusal, and for the sync diff — where they are instead an error,
  since one document cannot sync one element against two branches at once.
- The check runs over the whole generated id space of the scope, not the element ids
  alone: an annotated id that collides with a **derived** id — another element's encoded
  name, a membership id (`…_om`), an encoded expression-node id — is an error naming
  both, since the generators reserve no suffix and `[a-zA-Z0-9_-]+` admits ids that end
  in `_om` or embed `_p`. Ids OpenSysML mints are UUIDs and cannot collide; the check
  exists for hand-authored and foreign ids, and the round-trip tests must include such
  adversarial ids.
- An `ElementId` whose `id` fails `[a-zA-Z0-9_-]+`: error naming the offending byte.
- `ElementId` without an enclosing `ProjectRef`: error.
- Nested `ProjectRef` scopes are permitted — that is the cross-project story: an element
  tree materialized from another project carries its own `ProjectRef`, mirroring what the
  API represents through project usages.
- An annotation on an element the RDF mapping refuses (see the
  [mapping's limitations](../reference/rdf-mapping.md#limitations)) is legal notation and
  parses; it simply has no RDF effect until the refusal is lifted.

### Minting

OpenSysML mints ids in exactly two places, never during parsing or analysis:

- **On the user's explicit request**, for an element that carries no annotation: the
  CLI mints on first sync to a repository and writes the annotations back as a rewritten
  file behind `-sync-mint-ids`/`-sync-annotate`; the LSP offers the "Annotate … with a
  minted element id" code action (kind `refactor.rewrite`) on the declaration's header
  (a cursor on it, or a selection of it — whole-line selections included), minting a
  UUID v4 and inserting the annotation as a text edit — inline at the head of a body,
  standalone about-form at the end of the file for a bodiless declaration. Under
  no `ProjectRef`, the same edit also binds the root with a placeholder `projectId` for
  the user to fill in; a bare "Bind … to a project" action does that alone.
- **On import from a repository**, elements arrive with ids; the writer emits
  `ElementId` annotations for them, and one `ProjectRef` at the root.

Parsing a file never invents identity: a model that was never near a repository has no
annotations and no UUIDs.

## Permeation through the RDF mapping

One seam on each side:

- **Writer** (`internal/translate/export/rdf_out.go`): `rdf.ElementIRI` takes the effective
  id — annotated UUID when present, encoded qualified name otherwise. `sysml:elementId`
  continues to hold exactly the id the IRI ends in, so the invariant that the triple and
  the IRI cannot disagree is untouched. The `IdentityMetadata` annotations themselves are
  **not** exported as metadata elements: they are consumed into identity, not carried as
  model content, exactly as the mapping already consumes names into IRIs. A `ProjectRef`
  is optionally emitted as `sysx:` provenance triples on the root for traceability;
  Flexo scopes a graph by the org/repo/branch in the request path, so nothing in-graph
  depends on it. That request-path scoping is also what keeps cross-project id reuse
  representable: **one graph carries one project scope** — a conversion that targets a
  repository writes each `ProjectRef` scope as its own graph, destined for its own
  branch — and a single mixed graph (a plain `-convert ttl` of a multi-scope workspace)
  qualifies each element's IRI with its scope's provenance, refusing the conversion if
  two scopes' ids would still land on one IRI, rather than silently merging them. The
  writer also marks an **annotated** id with a `sysx:` triple (`sysx:declaredId true`),
  because explicitness is not recoverable from the value: an annotated id can equal the
  encoding of the element's current qualified name, and dropping the annotation there
  would turn the next rename back into a delete plus a create.
- **Reader** (`internal/translate/export/rdf_in.go`): identity is read from
  `sysml:qualifiedName` today, IRIs being treated as opaque. The reader additionally
  reads `sysml:elementId`; where an element's id is not the encoding of its qualified
  name **or the graph marks it `sysx:declaredId`**, the writer of the notation
  re-materializes the `ElementId` annotation (and a root `ProjectRef` from the
  provenance triples when they are present). That closes the round trip:
  `notation → RDF → notation` preserves annotated identity byte-for-byte, including the
  degenerate case where the annotated id happens to equal the derived one — which the
  planned round-trip tests must cover explicitly.

UUID ids also retire the risk recorded against readable ids: a client that assumes OMG's
UUID convention accepts an annotated element without special cases.

## Updating elements

With stable ids, synchronization against a repository becomes a diff keyed by effective
id rather than by name:

- **rename / move / retype** — same id, changed properties: an update (`PATCH`/change
  payload) to the existing element;
- **new id** (or unannotated new element): a create;
- **id present in the repository, absent from the model**: a delete, behind
  confirmation;
- **conflict** — the annotation names an id the repository's branch no longer has, or
  the repository's element changed since the last-seen commit: surfaced to the user,
  never silently resolved. Which commit a sync last saw is sync-tool state (recorded
  outside the model text), which is what makes "changed since" detectable.

Applying that diff (`sysml -sync-apply <endpoint>`, `reposync.Apply`) posts it as one commit
through the repository's own commit path — each entry addressed by the element's id, an update
carrying the element whole so the repository replaces it under the same id, a delete only
when the set was produced with deletes confirmed. A set that is not appliable (a conflict, an
unconfirmed delete) is refused as a typed error before the first write, and a repository that
rejects a commit leaves the run failed with every change's fate reported; in neither case does
the state move. On success the commit the repository names becomes the last-seen commit, so
the next diff is against it and finds nothing to change; an apply with nothing to change
records the head it compared against, so the baseline exists from the first run. The change
set is computed at one head commit, and the adapter refuses to commit if the branch has
moved since (`flexo.StaleBranchError`) — a check-then-post the SysML v2 API offers no atomic
precondition for, so it narrows the window rather than closing it, but never overwrites an
edit it has seen. A bearer token is not sent over plaintext `http://` to a host other than the
local machine unless `FLEXO_ALLOW_PLAIN_HTTP=1` says so. An apply that mints ids
(`-sync-mint-ids -sync-annotate`) writes the annotated model only after the commit holding
those ids lands, and is refused before any write when a minted element has no name for an
annotation to address — an id the notation cannot keep would be lost to the next parse. Both
sides of a diff are compared under the repository's *representation* — what its commit path
can store of a graph — so what it has no place for is reported as not compared rather than
reapplied on every run.

This subsumes the notation side of what the Flexo interoperability target needs: the
graph OpenSysML writes can stand in for the one `flexo-mms-sysmlv2` produces *for the
same elements across edits*, not merely for one snapshot.

## Standardization plan

The specification's omission of identity from notation is deliberate, so the submission
must argue the interchange case rather than report a gap:

1. **File an OMG issue** against the SysML v2 specification (the textual-notation
   clauses) proposing standard identity annotations — either a normative metadata
   library equivalent to `IdentityMetadata`, or dedicated surface syntax if the
   taskforce prefers it. The argument: notation is the format engineers version, diff
   and review; without in-band identity, every text round trip severs models from the
   repositories the API side of the same specification defines, and implementations are
   already inventing incompatible workarounds.
2. **Ship the prototype first.** Implementation experience is what OMG submissions are
   weighed by. The `IdentityMetadata` library, the validation pass, the RDF permeation
   and the sync diff above are all implementable without any specification change,
   because user-defined metadata is already conforming notation.
3. **Record the measurements.** The Flexo interoperability harness
   (`internal/translate/interop/flexo`) already measures what survives a live round trip; extended
   with an identity-carrying fixture, its committed report becomes the evidence the
   issue cites.
4. Track the filing in [omg-issues.md](omg-issues.md) once posted.

If the taskforce standardizes a different spelling, the migration is mechanical: the
annotations are data, and a rewrite from `IdentityMetadata` to the standard form is a
one-shot tool.

## Risks and alternatives considered

- **Metadata is visible to the language.** `import P::*[@IdentityMetadata::ElementId]`
  filters on it; metadata queries see it. This is judged acceptable: the annotations are
  ordinary model content by design (that is why they travel), the library namespace makes
  filtering *for* them deliberate rather than accidental, and opt-in annotation keeps
  them rare. What must not happen is tooling treating them as invisible — they appear in
  dumps, completions and searches like any metadata.
- **Merge noise.** Annotating every element would make every repository sync touch every
  line. Mitigated by opt-in annotation and by minting only at sync time, behind explicit
  user action.
- **Sidecar mapping file** — rejected above: does not survive foreign tools, which
  defeats the purpose.
- **Encoding identity in the IRI only, no notation change** — the status quo; fails
  requirements 1 and 2 of [the problem](#the-problem).
- **Comment-based conventions** (`/* @id: … */`) — survive nothing: comments carry no
  structure, other tools may reflow or drop them, and nothing validates them.

## Phasing

1. `IdentityMetadata` library file plus parser/stdlib conformance coverage — no
   behavioral change.
2. The validation pass (duplicates, id shape, scoping), with negative tests.
3. Effective-id plumbing in `internal/translate/rdf` and the writer; golden `.ttl` updates.
4. Reader re-materialization; round-trip tests including the stripped-`sysx:sourceText`
   form the RDF round-trip harness uses.
5. Sync diff keyed by effective id; Flexo harness fixture carrying annotated identity;
   re-measure and commit the interop report. Then the apply of that diff to the live
   repository, with the harness measuring update, create, gated delete and refused conflict
   against the real stack.
6. Draft and file the OMG issue, citing the measurements; record it in
   [omg-issues.md](omg-issues.md).

Each phase leaves the full gate green on its own; none blocks a release.

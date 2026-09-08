# The SysML ↔ RDF mapping

This page describes which triples a model becomes, and which constructs the mapping does not
represent. For saving and converting as a task, see [guide chapter 7](../guide/07-saving-and-rdf.md).

## Status: experimental

RDF conversion (`sysml -convert ttl`, `%save model.ttl`, the service's `Convert`
to or from `ttl`, and each in reverse) is **experimental** as of 0.1.0. Saving
and converting notation (`.sysml`, `.kerml`) is stable; this mapping is not. Each
of the following is a deliberate property of the mapping rather than a defect to
report:

- **What is not mapped is refused, not partly converted**, and the refusal names
  the construct. Every one of the 346 models under `examples/` (committed, training and
  pilot corpora) converts to Turtle, and a second
  conversion of the written-back notation reproduces the Turtle byte for byte for
  every one — the notation is written from the [source text](#source-text) the
  graph carries. These figures are the
  per-file ratchet in `internal/core/export/corpus_roundtrip_test.go`, described
  in [rdf-corpus-roundtrip.md](../project/rdf-corpus-roundtrip.md). See
  [Behavior](#behavior) and [Limitations](#limitations).
- **The vocabulary may change without a compatibility path.** A graph written by
  one release may not read back into the next, and no migration is provided.
  Treat a `.ttl` as an interchange artifact you can regenerate, not as the copy
  of record.
- **Interoperability is not yet demonstrated**, and the gap is measured rather
  than argued. The `sysml:` vocabulary and the `elmt:` element base match Flexo
  MMS's `Namespaces.kt`. OpenSysML's ids (the part of an IRI after the final
  `:`, for elements and expression nodes alike) match that service's
  `requireValidId` (`[a-zA-Z0-9_-]+`). Every element carries the
  `sysml:elementId` that paged listing and query select use, and ownership is
  written as the memberships and owner references the roots endpoint filters on.
  A collection-valued property is written twice, as the typed triples and as the
  JSON annotation literal that service reads a collection from
  ([Collections](#collections)). A round trip through a running Flexo MMS stack
  delivers every element of the reference fixture and every one of its standard
  properties, the multi-valued ones included; what it loses is the `sysx:`
  properties, since the reader ignores predicates outside `sysml:` and
  `urn:sysmlv2:annotation:json:`. The measurement lives in
  `internal/interop/flexo`, an opt-in gate described in
  `.agents/skills/flexo-interop`, and its committed report records what changes
  as the remaining work lands.

Every surface reports this status where it is used: the command line writes a
`note:` to stderr, `%save` prints one, and `ConvertResponse` carries `experimental`
and `experimental_notice`, which the Python client raises as an
`ExperimentalFeatureWarning`. The wording is a single constant,
`export.ExperimentalNotice`.

## The RDF mapping

### Namespaces

| Prefix  | IRI                                            | Holds |
|---------|------------------------------------------------|-------|
| `sysml:` | `https://www.omg.org/spec/SysML#`             | Metaclasses and metamodel properties |
| `elmt:`  | `urn:sysmlv2:element:`                        | The elements of the converted model |
| `sysx:`  | `urn:opensysml:sysml:`                        | The few properties the metamodel does not define |
| `expr:`  | `urn:opensysml:expr:`                         | The expressions an element's positions hold, see [Expressions](#expressions) |
| `json:`  | `urn:sysmlv2:annotation:json:`                | One JSON literal per collection-valued property, see [Collections](#collections) |
| `rdf:`, `xsd:` | the standard RDF and XML Schema namespaces | `rdf:type`, literal datatypes |

The `sysml:` vocabulary and the `elmt:` element base match the ones the
[Flexo MMS SysML v2 service](https://github.com/Open-MBEE/flexo-mms-sysmlv2)
writes into its triplestore (`Namespaces.kt`). That service derives an
element's `@id` from the substring after the final `:`, and `requireValidId`
permits only `[a-zA-Z0-9_-]+`. OpenSysML's encoded element ids satisfy both, and
so do the `expr:` node ids. (A node's id used to contain a `.`, which that service
refused to read; the position is now joined with `_p` and encoded instead.)
The `json:` annotation base is that service's `ANNOTATION_JSON` (`Namespaces.kt`),
the one its reader takes a collection from ([Collections](#collections)). One
mismatch remains: the reader ignores predicates outside `sysml:` and
`urn:sysmlv2:annotation:json:`, so `sysx:` triples do not survive that path. See
[Status](#status-experimental).

OpenSysML's own additions live in a separate `sysx:` namespace so a consumer can
tell them apart from the standard vocabulary and ignore them if it wants only
standard SysML.

A graph an earlier release wrote with terms this one no longer reads is refused
rather than read without what those terms said: the pre-rename namespace
`urn:systemica:sysml:`, and the metadata properties `sysx:prefixMetadata` and
`sysml:annotates` that release 0.4.3 wrote for a `#` prefix and an `about`
target (now a `sysml:MetadataUsage` and `sysml:annotatedElement`, see
[What each element carries](#what-each-element-carries)), and the flags
`sysml:isSnapshot` and `sysml:isTimeslice` that release 0.5.1 wrote for a
portion (now `sysml:portionKind`). The error names the term; re-export the
model from its notation source.

### Element IRIs

An element's IRI is its qualified name, encoded as an id, appended to `elmt:`:

```
package Demo { part def Vehicle; }
```

```turtle
elmt:Demo         a sysml:Package ;
    sysml:qualifiedName "Demo" .
elmt:Demo__Vehicle a sysml:PartDefinition ;
    sysml:qualifiedName "Demo::Vehicle" ;
    sysml:elementId "Demo__Vehicle" ;
    sysml:owner elmt:Demo .
```

The encoding (`rdf.EncodeElementID`) works over the UTF-8 bytes of the
qualified name, with `_` as the escape character:

- the `::` separator becomes `__`
- a byte in `[A-Za-z0-9-]` stands for itself
- every other byte — a literal `_` included — becomes `_` plus two lowercase
  hex digits: `A_B::C` → `A_5fB__C`, `A::B_C` → `A__B_5fC`,
  `Importer::@0` → `Importer___400`, `Vehicle Mass` → `Vehicle_20Mass`

The id therefore always matches `[A-Za-z0-9_-]+`, distinct qualified names
never collide, and `rdf.DecodeElementID` reverses the encoding exactly. The
IRI is **deterministic**: converting the same model twice yields the same
IRIs, and re-converting after an edit leaves the untouched elements at the same
addresses. The id is an address, not the copy of record for the name. The
name is carried by `sysml:qualifiedName`, which is where reading a graph back
takes it from.

### Element identity

The encoded qualified name is only the **derived** id, the one an element gets
when nothing declares one. An `@IdentityMetadata::ElementId` annotation
declares the id explicitly, and then the element's IRI and `sysml:elementId`
carry the declared id instead, so a rename keeps the subject:

```
package Demo {
    @IdentityMetadata::ProjectRef { projectId = "proj-1"; }
    part def Vehicle {
        @IdentityMetadata::ElementId { id = "8f3a41d0"; }
    }
}
```

```turtle
elmt:8f3a41d0 a sysml:PartDefinition ;
    sysml:qualifiedName "Demo::Vehicle" ;
    sysml:elementId "8f3a41d0" ;
    sysx:declaredId "true"^^xsd:boolean .
```

The identity annotations are **consumed into identity** rather than exported as
metadata content, exactly as names are consumed into IRIs. `sysx:declaredId`
records that the id came from an annotation. That fact cannot be recovered from
the value itself, since a declared id may happen to equal the encoding of the
current qualified name, and dropping it would turn the next rename into a delete
plus a create. A membership's id derives from its member's effective id
(`8f3a41d0_om`), and an expression node's from its owner's, so both inherit the
id's stability.

A `@IdentityMetadata::ProjectRef` annotation on a scope root is written as
provenance triples on that root: `sysx:projectId`, `sysx:branch`, `sysx:org`.

A document holding **more than one project scope** qualifies each element's IRI
with its scope's provenance (`elmt:<encoded-org>.<encoded-project>:<id>`), so an
id repeated across scopes stays two subjects; two scopes whose elements would
still land on one IRI are refused rather than silently merged.

Reading a graph back keys the subjects on `sysml:elementId`; the qualified
name is a mutable label. A graph without `sysml:elementId` (from before the
property existed, or from another tool) falls back to the encoding of its
qualified name, which is what its IRIs carry. The notation writer re-materializes an
`@ElementId` annotation wherever the graph marks `sysx:declaredId` true **or**
the id differs from the encoding of the qualified name, and one `@ProjectRef`
per root carrying provenance. Subjects are classified by their `rdf:type`,
never by parsing the id — a declared id may legitimately end in `_om` or embed
`_p` without being a membership or expression node. A subject stating several
classes is read as the one that is a subclass of all the others in the SysML
ontology (`sysml:OwningMembership, sysml:ResultExpressionMembership` is a
`ResultExpressionMembership`; `sysml:ActionDefinition, sysml:Function,
sysml:CalculationDefinition` a `CalculationDefinition`, in whichever order the
triples come); a set of classes with no such member is refused, naming the subject.

### What each element carries

- `rdf:type` — the SysML metaclass (`sysml:PartUsage`, `sysml:ActionDefinition`, …).
  Every definition and usage keyword the parser accepts has a metaclass; the
  tables in `internal/core/export/kinds.go` are the source of truth, and the
  reverse direction is derived from them so the two cannot disagree.
- `sysml:declaredName`, `sysml:declaredShortName`, `sysml:qualifiedName` —
  on a requirement's `subject`, `assume constraint` and `require constraint`
  members as on any usage, so `subject <s> x : T;` comes back with its short name
- `sysml:elementId` — the id the element's own IRI ends in, which is what the
  SysML v2 API addresses it by. Every element carries one, including the
  memberships below and the expression nodes of
  [Expressions](#expressions)
- Ownership, described under [Ownership](#ownership): `sysml:owner`, plus
  `sysml:owningMembership` and `sysml:owningRelationship`, or
  `sysml:owningRelatedElement` for an element a relationship owns
- `sysml:owningNamespace` — the containing namespace (absent on a root and on
  an element a relationship owns, whose owner is no namespace), kept alongside
  `sysml:owner` as the compact spelling earlier releases wrote
- `sysml:visibility`, `sysml:direction`
- Feature flags, written only when true, so an absent flag reads as false:
  `isAbstract`, `isVariation`, `isVariant`, `isReference`, `isComposite`, `isDerived`,
  `isOrdered`, `isNonunique`, `isEnd`, `isConstant`, `isIndividual`, `isPortion`,
  `isConjugated`, `isAll`, `isAccept`, `isResult`, and `isEvent` for an `event`
  modifier on a usage whose metaclass is not itself `sysml:EventOccurrenceUsage`.
  A flag the two grammars spell differently is written back in the grammar of
  its root (`sysx:sourceLanguage`): `isConstant` as KerML's `const` or SysML's
  `constant`; `isPortion` as KerML's `portion`, in place of `composite` (a
  portion is composite, so `isPortion` without `isComposite` is refused), and
  on a SysML root as nothing of its own — there it is the fact `snapshot` or
  `timeslice` states (`OccurrenceUsage::portionKind` implies it), so it is
  written back by the portion kind and refused without one, SysML having no
  `portion` prefix and `composite` dropping the fact. The other flags are
  spelled alike in both grammars
- `sysml:portionKind`, `"snapshot"` or `"timeslice"`, for a usage declared as a
  portion (`snapshot :>> start`, `timeslice occurrence t`); the two are the
  metamodel's `OccurrenceUsage::portionKind`, so no flag spells them. Such a
  usage also carries `sysml:isPortion`, the fact its kind implies; a graph
  stating the kind alone reads back by its kind, and gains the flag when
  re-exported
- Declaration-head relationships, as element IRIs where the target resolves
  inside the model — by name resolution, so a name reached through an import,
  an alias or a nested package qualification links to the same element its
  fully qualified spelling does — and as plain literals where it does not: `sysml:type`
  (the `:` clause), `specializes`, `subsets`, `redefines`, `references`,
  `crosses`, `disjointFrom`, `intersects`, `inverseOf`, `unions`, `chains`,
  `includes`, `via`, `subject`, `annotatedElement` for an `about` clause, and
  the namespace or member an import names, `importedNamespace`. A literal
  carries the name itself,
  without the quotes an unrestricted name is written with; a target that is an
  expression rather than a name (a feature chain, say) is carried as the text it
  was written as, typed `sysx:Expression` to tell the two apart. Reading a graph
  back, a literal that is neither — a number, a boolean, a language-tagged
  string, an empty or broken qualified name — is refused rather than written
  into the notation as it stands. A feature
  chain's `sysml:targetFeature` links the member the chain reaches in its
  operand's type, a redefinition the general's feature; written back, each is
  spelled by its own name where that reaches one feature among the operand's
  or the owner's generals, else qualified. A transition or `then` end in a
  state machine links the vertex it names anywhere in the machine, in a nested
  state or a sibling region; a loop's `while` or `until` condition links the
  actions the loop body declares. A body expression's parameter, a `for` loop's
  variable and a trigger's parameter are no elements of the graph: a reference
  to one stays its name, even where it shadows a feature of the same name.
- A KerML relationship written keyword-first as a member of its own
  (`specialization Gen subtype A specializes B;`, `subset f subsets g;`,
  `inverse f of g;`, `featuring of f by T;`, `disjoint A from B;`) is an element
  typed by its metaclass, and its two ends are two properties whose order the
  metamodel fixes: `sysml:Specialization` with `specific` and `general`;
  `sysml:FeatureTyping` with `typedFeature` and `type`; `sysml:Subsetting` with
  `subsettingFeature` and `subsettedFeature`; `sysml:Redefinition` with
  `redefiningFeature` and `redefinedFeature`; `sysml:Conjugation` with
  `conjugatedType` and `originalType`; `sysml:FeatureInverting` with
  `invertingFeature` and `featureInverted`; `sysml:TypeFeaturing` with
  `featureOfType` and `featuringType`; `sysml:Disjoining` with `typeDisjoined`
  and `disjoiningType`. Each end is a link or a literal by the rule above, so a
  feature chain (`disjoint earlier.successors from later.predecessors;`) is
  carried as `sysx:Expression` text. `sysx:declaredKeyword` keeps the keyword
  the member was written with (`subtype` against `subclassifier`) and
  `sysx:declaredPrefix` the `specialization`, `inverting` or `disjoining` that
  introduces its name; the notation is written back from the two ends, so
  swapping them in the graph swaps them in the notation. This is distinct from
  the clause of a declaration (`class C specializes A disjoint from B;`), which
  stays a property of `C` (`sysml:specializes`, `sysml:disjointFrom`).
- `sysml:lowerBound`, `sysml:upperBound` — multiplicity, as expression nodes
  ([Expressions](#expressions))
- `sysml:value` — a feature's value, as an expression node, with
  `sysml:isDefault` and `sysml:isInitial` stating the `default` and `:=` of the
  operator it was written with (so `default = 1` does not come back as the
  binding `= 1`, which a redefinition may not override)
- `sysml:aliasedElement`, `sysml:client`, `sysml:supplier`, `sysml:body`,
  `sysml:language`, `sysml:locale`, `sysml:annotatedElement`
- A metadata annotation — `@Safety;`, `@Safety { level = 2; }`, `metadata m :
  Safety about a, b;` or the prefix `#Safety part def P;` — is a
  `sysml:MetadataUsage` owned by the element it is written in or ahead of,
  through an `OwningMembership` (never a `FeatureMembership`: the annotation is
  not a feature of what it annotates), even when that element is itself a
  relationship such as a `dependency` or a `subject` membership. It carries
  `sysml:type` for its metadata definition, one `sysml:annotatedElement` per
  `about` target, `sysx:hasBody`,
  and `sysx:declaredKeyword` `"@"` or `"#"` for the sigil it was written with
  (`metadata` is the absence of both). The body's members are its owned members
  like any other body's: a value binding (`level = 2;`) is a `sysml:ReferenceUsage`
  carrying `sysml:value`, a redefinition (`:>> level = 3;`) carries
  `sysml:redefines`, a nested feature keeps its own kind, and `sysx:memberIndex`
  orders them. A `#` prefix is an owned member of the declaration it prefixes,
  indexed after the body's members so their indices are the same with or without
  it, and is written back at the head of that declaration rather than in its body,
  in the grammar's position: ahead of the kind keyword and of `assert`/`perform`
  (`#Safety assert not constraint c;`), after `subject`, `actor`, `stakeholder`,
  `objective`, `variant`, `assume`, `require` and `var` (`assume #Safety constraint c;`).
- The cross feature an end declares ahead of its kind keyword —
  `end [0..*] item x : A;`, `end x1 [1] typed by Sub1 item y : B;` — is a
  `sysml:Feature` (in SysML, a `sysml:ReferenceUsage`) owned by the end through
  an `OwningMembership`, indexed after the end's body members and prefix
  annotations, carrying its name, its `sysml:lowerBound`/`sysml:upperBound` and
  its specializations. Its bounds are never the end's:
  `end [0..*] item x : A[1];` states `[0..*]` on the cross feature and `[1]` on
  the end, and the decoder writes each back where it was declared.

The `sysx:` properties:

| Property | Why it exists |
|----------|---------------|
| `sysx:memberIndex` | Declaration order. The notation is sensitive to the order of members; an RDF graph is an unordered set, so the index is what lets a conversion back to notation reproduce the original sequence. |
| `sysx:hasBody` | Distinguishes `part def A;` from `part def A { }`, which are different source and would otherwise convert back identically. Also marks an expression body node, so `{}` rebuilds from structure. |
| `sysx:sourceText`, `sysx:sourceTail` | The element's lines as written, comments and blank lines included, which a conversion back to notation prefers while they still state what the graph states. An element with members carries the lines ahead of them as its text and those after them as its tail. See [Source text](#source-text). |
| `sysx:sourceLanguage` | On each root element, the grammar the file was written in — `sysml` or `kerml` — so the text is read back under the grammar it was written under, and a flag the two grammars spell differently (`const` against `constant`, see the feature flags above) is written in that grammar. Absent for a buffer with no model extension (standard input, a REPL session), which the parser reads as SysML with KerML's `all` prefix. See [Source text](#source-text). |
| `sysx:declaredKeyword` | The kind keyword as written, when it is one of the synonyms several keywords share (`datatype` and `attribute`, `function` and `calc`, KerML's `feature` and `attribute`, `snapshot` and `occurrence`), on a named declaration and on an anonymous one alike (`feature :>> x;`, `snapshot :>> start { … }`). The AST records one kind for all of them, so without this the notation would come back rewritten. Where the graph types the fact the keyword states — `sysml:portionKind` for `snapshot`/`timeslice`, the metaclass `sysml:EventOccurrenceUsage` for `event`, `sysml:AssertConstraintUsage` for `assert` — the typed fact is authoritative and this predicate only chooses between two spellings of it (`snapshot :>> start` against `snapshot occurrence :>> start`; `event m.start` against `event occurrence references m.start`; `assert c` against `assert constraint references c`); a keyword the typing contradicts (`snapshot` with `sysml:portionKind "timeslice"` or none, `event` on a `sysml:PartUsage`, `assert` on a `sysml:ConstraintUsage`) is refused rather than one of the two written. KerML's `feature` has no typed counterpart — an attribute usage is what the AST records for it and `sysx:sourceLanguage` does not decide between the two — so it is carried as this spelling alone. Also the keyword a constraint body's condition is stated with (`assert`, `assume`, or absent for a bare condition, which asserts implicitly), the `constraint` of an `assume`/`require` member that declares a constraint usage (so its `references C` is read as a specialization, where `require C` alone states the constraint the member refers to), and the sigil a metadata annotation was written with: `@` for a member (`@Safety;`), `#` for a prefix ahead of a declaration (`#Safety part def P;`), absent for the `metadata` keyword. |
| `sysx:declaredPrefix` | The keyword qualifying the kind keyword after it — the `assume` of `assume constraint c : C`. It says what the declaration is for, and the AST kind alone does not carry it. The `assert` of `assert constraint c : C` is not written here: that usage is a `sysml:AssertConstraintUsage`, and the metaclass states it; a graph stating both with another prefix is refused. |
| `sysx:endForm` | The notation an end-binding head writes its ends in — `to`, `nary`, `equals`, `firstThen`, `fromTo`, `flowTo`, `satisfy`, `then` — so the head is rebuilt from the graph rather than read back from its text. See [End-binding heads](#end-binding-heads). |
| `sysx:endVerb` | The verb a head writes ahead of its ends when its own keyword is the noun form (`connection c connect a to b`, `connector c from a to b`). Without it the verb would be missing or doubled. |
| `sysx:endName` | The name a connector end declares for itself ahead of the feature it reference-subsets (`connect bead ::> t.bead to …`, `connector a ::> a.x to b;`, `bind e1 ::> a = e2 ::> b;`). The end's node relates the feature; without the name the end would come back as the bare feature. See [End-binding heads](#end-binding-heads). |
| `sysx:endReferencesKeyword` | On a named end, the ReferencesKeyword written between the name and the feature when it is the word `references` (`bind e1 references a = …`). Absent, the end is written with `::>`; a value other than the two spellings is refused. |
| `sysx:sourceMember`, `sysx:targetMember` | The member a succession sequences from or to where the notation names no end (`then b;`, or a `then` beside an unnamed member), or where the name the notation supplies for an end links no element (a `then` after `action redefines walk;` whose `walk` is inherited). The end is the element itself rather than only a name, so a same-named member elsewhere cannot be mistaken for it. |
| `sysx:condition` | The condition a condition member states, as its notation. |
| `sysx:resultExpression` | The expression an expression body (`{ in y : Real; y + x }`) ends in, after its parameters. The bare expression a calculation or case body computes is not an extension: it is the Expression its `sysml:ResultExpressionMembership` owns. See [Result expressions](#result-expressions). |
| `sysx:bodyParameter`, `sysx:bodyMember` | The `in` parameters an expression body declares, each a node carrying its name, type, bounds and value, and the other declarations it makes ahead of its result: a `doc` as a `sysml:Documentation` node, anything else as notation. Both share one `sysx:memberIndex` sequence, the order they were written in. |
| `sysx:declaredId` | The element's id came from an explicit `@IdentityMetadata::ElementId` annotation, see [Element identity](#element-identity). |
| `sysx:projectId`, `sysx:branch`, `sysx:org` | The `@IdentityMetadata::ProjectRef` provenance of a scope root, see [Element identity](#element-identity). |
| `sysx:isKindImplicit` | The declaration wrote no kind keyword (`in x : Real;`), which takes its kind from its owner. Without it the canonical keyword would come back written out, declaring what the author did not. A kind named in a comment in the head (`in /* attribute */ x : Real;`) is trivia, not a keyword the declaration wrote. |
| the behavioral properties | `sysx:guard`, `sysx:expression`, `sysx:payload`, … — the parts of a behavioral node the vocabulary has no predicate for, listed under [Behavior](#behavior). |

Metaclass names with no counterpart in the OMG vocabulary are typed in the
`sysx:` namespace rather than `sysml:`, so a consumer can tell them from the
standard metaclasses: `sysx:Alias`, `sysx:FilterMember`,
`sysx:MultiplicityDeclaration`, `sysx:ConstraintMember`, `sysx:AssumeMember`,
`sysx:RequireMember`, `sysx:BodyMember`, and the
behavioral ones listed under [Behavior](#behavior).

Comments, documentation and textual representations convert as their own
elements (`sysml:Comment`, `sysml:Documentation`, `sysml:TextualRepresentation`)
carrying `sysml:body`.

### Source text

Every element carries the notation it was written as, so a conversion back to
notation can return the file rather than a canonical rendering of it. The text
is the element's **lines**, trivia included: the `//` and `/* */` comments and
blank lines ahead of a member belong to it, and a comment on its last line too.
An element with members carries the lines ahead of its first member as
`sysx:sourceText` and those after its last as `sysx:sourceTail`, since the
members carry their own; a `package P { … }` is written as its head, its
members in `sysx:memberIndex` order, and its `}`. A member written on its
owner's own lines — an accept's payload (`accept sig : Cmd;`), the branches of
an `if` — carries no text of its own: it is part of its owner's text, and an
edit to it rebuilds the owner whole rather than splicing one line. A succession
written as the `then` ahead of its target is likewise part of the target's
text: a succession added to or removed from the graph rebuilds that target.
Expression nodes carry `sysx:sourceText` too, as described under
[Expressions](#expressions).

The text is the notation **as the author wrote it**: the encoder slices the
file's own bytes, never a formatted copy, and the decoder writes them back
untouched, so any file converts to RDF and back **byte for byte** — tabs,
irregular indentation, blank lines inside a head, CRLF line endings, a string
literal or `doc` body spanning lines, all included. Roots written on one line
(`package A; /* note */ package B;`) each carry their slice of it, from their
first token up to the next root's, and what follows the last root (notes, blank
lines, a missing final newline) is that root's tail, since the document itself
has no subject. Tokens are never rewritten by either step, so a synonym (`:>`
for `specializes`, `datatype` for `attribute def`), an unusual member order, or
a reference written relative to another scope all come back as written. Layout
is never what the graph states: where the mapping records how a head was
written (`sysx:endForm`, `sysx:declaredKeyword`, a `then`) it compares the
head's tokens, so a head laid out over several lines, with a comment inside it
or a note after its `;` is recorded like one written on a line; and where the
graph carries a node's text as a structural value (a relationship target, a
trigger), the notes and comments its span runs on over are left out.

**The graph is authoritative.** The text is a rendering of the structural
triples, not a second copy of the model, and the decoder checks it before
trusting it: the candidate notation is converted back to RDF and compared with
the graph being read, source text aside. The candidate is read under the
grammar the roots record as `sysx:sourceLanguage`, since KerML text can read
clean as SysML and mean something else (`binding [1] a = b` names the binding
`a` there); a root recording no language was read as a buffer with no
extension, and its text is read as one again, `all` as a prefix rather than a
name. Roots recording different languages are not read at all, and the graph
is written canonically. `sysx:memberIndex` is set aside too:
the notation lists members in index order whatever the numbers, so a member
removed from the middle of a body leaves those after it standing as written,
their indices no longer running on from zero. Each triple the two disagree on is
charged to the nearest element whose text it falls under — or to the outermost
expression node written from its text — which is then written in canonical
notation instead, and the notation is built and checked again until the two
graphs agree. So a graph edited after it was written (a flag set, a value
changed, a member removed, an id or `ProjectRef` dropped) comes back stating
the edit, with the stale text replaced only where it was stale:

```sysml
// The rover, as modelled.
package Rover {
    /* Definitions come first. */
    part def Wheel :> Part; // a synonym the printer would spell out
    abstract part def Hub;

    part def Vehicle {
        doc /* what a vehicle is for */
        part wheels : Wheel[4]; // four of them
    }
}
```

Here `sysml:isAbstract` was added to `Hub` after the export: its line — and the
note that was written above it — is rebuilt from the graph, and every other
line is kept, the blank line after it included, since that belongs to
`Vehicle`. Rebuilt lines end the way most of the elements' text does, CRLF or
LF; an expression's text lies inside its element's and is not counted again.
Text that no longer parses, or whose disagreement cannot be placed on one
element, demotes every element to canonical notation rather than writing an
invalid or contradictory file; one that lands on notation already rebuilt is
the graph's own (a `declaredName` edited without its `qualifiedName`) and
demotes nothing further. Identity annotations follow the same rule:
text that still carries its `@IdentityMetadata::ElementId` or `ProjectRef` is
kept as written, and text that has lost one is rebuilt with the annotation the
graph states, exactly as a graph without text is written
([Element identity](#element-identity)).

**A graph without source text converts to canonical notation**, unchanged from
before: a graph from another tool, or one with `sysx:sourceText` stripped, is
written from its structural triples alone, with trivia gone and every keyword
spelled canonically. That path is what the round-trip tests exercise — see
[Limitations](#limitations) — and this one adds to it rather than replacing it.

Tests: `verbatim_test.go` (byte-for-byte return, the stripped graph's canonical
notation, an edited flag, an edited expression, an edited string, an edited
accept payload, a removed member, an added and a removed `then`, dropped
identity annotations, text that does not parse) and `export_test.go`
(`TestGoldenConversions` locks both notations for every fixture).

### Ownership

The notation states containment by nesting; the abstract syntax states it as a
membership element between the owner and the member, and that is what the SysML
v2 API's payloads carry. The mapping materializes those memberships.

A namespace owning an ordinary member mints one membership element:

```
package Demo { part def Vehicle { attribute mass; } }
```

```turtle
elmt:Demo
    a sysml:Package ;
    sysml:elementId "Demo" ;
    sysml:ownedMember elmt:Demo__Vehicle ;
    sysml:ownedMembership elmt:Demo__Vehicle_om ;
    sysml:ownedRelationship elmt:Demo__Vehicle_om .

elmt:Demo__Vehicle_om
    a sysml:OwningMembership ;
    sysml:elementId "Demo__Vehicle_om" ;
    sysml:owner elmt:Demo ;
    sysml:memberElement elmt:Demo__Vehicle ;
    sysml:ownedMemberElement elmt:Demo__Vehicle ;
    sysml:ownedRelatedElement elmt:Demo__Vehicle ;
    sysml:owningRelatedElement elmt:Demo ;
    sysml:membershipOwningNamespace elmt:Demo .

elmt:Demo__Vehicle
    a sysml:PartDefinition ;
    sysml:elementId "Demo__Vehicle" ;
    sysml:owner elmt:Demo ;
    sysml:owningRelationship elmt:Demo__Vehicle_om ;
    sysml:owningMembership elmt:Demo__Vehicle_om .
```

- A membership's id is the member's id with `_om` appended, which no element id
  can be: an `_` in an element id starts either `__` for `::` or a hex escape.
  It is minted by `rdf.OwningMembershipID`, so it is deterministic and reverses
  to the member's qualified name.
- A **type owning a feature** — a usage or a state inside a definition — mints a
  `sysml:FeatureMembership` instead, and adds `sysml:ownedMemberFeature` and
  `sysml:owningType` on it and `sysml:ownedFeature` and
  `sysml:ownedFeatureMembership` on the owner. `FeatureMembership` specializes
  `OwningMembership`, so the `_om` id and the properties above still apply.
- A KerML **`member feature`** — `class C { member feature x; }`, the grammar's
  `TypeFeatureMember` — is a feature the type owns through a plain
  `sysml:OwningMembership`, not a `FeatureMembership`: it is a member of the type
  but not one of its features, so none of `ownedFeature`,
  `ownedFeatureMembership`, `ownedMemberFeature` or `owningType` is stated.
  Reading a graph back, a `Feature` a `Type` owns through a plain
  `OwningMembership` is written with the `member` prefix, after its visibility
  (`private member feature x;`), unless the membership is one KerML writes
  another way: a `VariantMembership`, a `ResultExpressionMembership`, a
  metadata annotation, an enumerated value, or the cross feature an end declares
  in its head (described with the metadata annotations above). SysML has no
  `member` keyword, so a SysML-language type
  that owns a feature through a plain `OwningMembership` is an
  `UnsupportedError` naming the feature: writing it as `attribute x;` would
  make it a feature of the type, a different model.
- A **relationship a namespace declares** — an import, a dependency, a state's
  entry membership — is owned directly, with `sysml:owningRelatedElement` on it
  and `sysml:ownedRelationship` on the owner, and no membership between. An
  import also states `sysml:importOwningNamespace` and `sysml:ownedImport`.
- An element a **relationship** owns — the action of an entry membership — states
  the relationship in `sysml:owner` and `sysml:owningMembership`, and the
  relationship states it in `sysml:memberElement`.
- **Visibility belongs to the membership.** `private part wheel;` writes
  `sysml:visibility "private"` on the membership, which is where the metamodel
  declares the property; a relationship that states its own visibility, such as
  an import, keeps it on itself.
- A membership is **not** a declaration of its own: it carries no
  `sysml:qualifiedName`, and reading a graph back traverses it as the ownership
  edge it stands for rather than writing it out. A membership the notation does
  name, such as a `sysml:StateSubactionMembership`, has a qualified name and is
  written back.
- **The compact shape still reads.** `sysml:owningNamespace` is still written,
  and a graph carrying only it — what earlier releases wrote — converts back
  unchanged. A membership that states neither of its ends is reported as
  unsupported naming `sysml:memberElement`, rather than dropping the member; so
  is one whose spellings of an end (`sysml:memberElement`,
  `sysml:ownedMemberElement`, `sysml:ownedMemberFeature`,
  `sysml:ownedResultExpression`, `sysml:ownedRelatedElement`) name different
  elements, one whose end is a literal rather than an element, and a second
  membership owning an element another already owns — rather than keeping one
  edge and dropping the rest. The element's side is held to the same rule: its
  `sysml:owningMembership`/`sysml:owningRelationship` must agree with each other
  and with the membership that claims it, and its `sysml:owner`,
  `sysml:owningNamespace` and `sysml:owningRelatedElement` with the namespace
  that membership puts it under.

Tests: `ownership_graph_test.go` (element ids, roots, membership wiring, the
tree coming back from the memberships with `sysx:sourceText` and
`sysml:owningNamespace` stripped, the compact shape, malformed memberships).

### Collections

A property with several values is stated twice: as one typed `sysml:` triple per
value, which is what RDF states, and as one literal on the same key in the
`json:` namespace holding the whole collection as a JSON array:

```
package Demo { part def A; part def B; part def C specializes A, B; }
```

```turtle
elmt:Demo
    sysml:ownedMember elmt:Demo__A, elmt:Demo__B, elmt:Demo__C ;
    json:ownedMember "[{\"@id\":\"Demo__A\"},{\"@id\":\"Demo__B\"},{\"@id\":\"Demo__C\"}]" .

elmt:Demo__C
    sysml:specializes elmt:Demo__A, elmt:Demo__B ;
    json:specializes "[{\"@id\":\"Demo__A\"},{\"@id\":\"Demo__B\"}]" .
```

The second spelling exists because of how the
[Flexo MMS SysML v2 service](https://github.com/Open-MBEE/flexo-mms-sysmlv2)
reads a graph. `ElementApi.extractModelElementToJson` indexes a subject's
outgoing triples by predicate; a `sysml:` predicate with more than one object is
an array to it, and it **skips** the typed triples and reads the property from
the literal at `urn:sysmlv2:annotation:json:<key>` instead, which must be
exactly one RDF literal, parsed as JSON. Its own commit path (`CommitApi.kt`)
stores a posted array both ways: one JSON annotation literal holding the array,
plus a typed triple per member — an IRI for a `{"@id": …}` member, a typed
literal for a primitive. The mapping writes what that path writes, so a graph
OpenSysML produces reads back through that service with its collections intact;
the live measurement is in `internal/interop/flexo/testdata/interop_expected.txt`.

The JSON shape is the commit path's:

- a **reference** is `{"@id": "<id>"}`, the id being the part of the IRI after
  the final `:`, for elements and expression nodes alike. In a [multi-scope
  document](#element-identity) a reference into another project scope keeps that
  scope's qualifier, `{"@id": "<encoded-org>.<encoded-project>:<id>"}` (an
  empty qualifier, `":<id>"`, naming the unscoped root), exactly as its typed
  triple does; so an id that both scopes carry still names one element, and the
  two spellings compare exactly. A single-scope graph, which is what the service
  holds, never spells a qualifier;
- a **primitive** is a JSON string, boolean or number: `xsd:boolean` as
  `true`/`false`, `xsd:integer`, `xsd:decimal`, `xsd:double` and `xsd:float` as
  numbers, every other literal as a string of its lexical form;
- the array is compact, without HTML escaping, and its **order is the triple
  order** of the graph, which the mapping writes deterministically (declaration
  order for members, source order for a head's targets), so the same model
  yields the same literal.

**Which properties carry it.** Every `sysml:` property a subject states more
than once, and only those; a single-valued property is unchanged. Which
properties that is follows from the mapping rather than from a list: the
ownership collections `ownedMember`, `ownedMembership`, `ownedRelationship`,
`ownedFeature`, `ownedFeatureMembership`, `ownedImport`, and on a relationship
element that itself owns members (an objective, a requirement's `satisfy`) the
`ownedRelatedElement`, `memberElement`, `ownedMemberElement` and
`ownedMemberFeature` it states per member; a head's
relationships when it names several targets — `type`, `specializes`, `subsets`,
`redefines`, `references`, `disjointFrom`, `intersects`, `unions`, `differences`,
`chains`; a dependency's `supplier`; and an expression node's `argument`
([Expressions](#expressions)). A relationship the head states by a name the
model does not resolve is a plain literal in the typed triples and a JSON string
in the annotation, so one collection can mix references and strings. Over the
corpora under `examples/` these are the keys that occur; a model that states
another property twice gets the annotation on that property too.

**Reading a graph back** accepts either spelling or both. A collection stated by
the annotation alone — what that service writes back for a graph it holds —
is materialized as typed triples in the annotation's order before decoding, a
`{"@id": …}` member resolving to the subject with that id in the scope the id
spells — the referring subject's own when it spells none — or, absent one,
standing as an element IRI that dangles as any other unresolved reference does;
a subject outside the element and expression namespaces is never the target,
whatever its local name.
An annotation that names a cross-scope target by its bare id disagrees with the
qualified typed triple and is refused as a conflict rather than retargeted to
the referrer's scope. An id that both an element and an expression node carry
(the two namespaces are disjoint, so an element may declare the id a node
derives) resolves in the referrer's own namespace — an element's members are
elements, an expression node's arguments are nodes — and a referrer in neither
namespace has such an id refused rather than one subject picked.
A string member reads as a plain literal, since the annotation carries no
datatype: a head target written as an expression comes back from the annotation
alone as a name, where the typed triple would have carried `sysx:Expression`.
A collection stated by typed triples alone — a graph an earlier release or
another tool wrote — reads as before. Where both are present they must agree as
multisets, typed triples carrying no order, and the annotation's order is the
one the decoder takes; two spellings that disagree are refused with an
`rdf.CollectionConflictError` naming the subject and the key, rather than one of
them being picked. An annotation that is not one literal, or not a JSON array
of references and primitives, is refused naming the subject and the key; so is
an array that repeats a member, since a graph holds each triple once and could
not give the repetition back.

The sync (`-sync-diff`) compares the typed triples and treats the annotation as
their restatement, reconciling it first; the service's commit path regenerates
it from the array the sync posts. Minting ids into a model rewrites the typed
triples and restates each annotation from them, so the two cannot drift: a
declared id that merely resembles a minted element's derived ids stays as it is.

Code: `rdf.AnnotateCollections` (encoder pass), `rdf.ReconcileCollections`
(decoder pass), `rdf.CollectionJSON`/`rdf.ParseCollectionJSON` (the shape).
Tests: `internal/core/rdf/annotation_test.go`,
`internal/core/export/rdf_collections_test.go`,
`internal/interop/reposync/diff_test.go`.

## Expressions

An expression-valued position — a feature value, a multiplicity bound, a guard,
a filter, a condition, a send payload, a loop's collection — states the
expression as a **tree of typed nodes** in the `expr:` namespace, so a consumer
can query the model's semantics and not only its structure:

```sysml
package P {
    private import ScalarValues::*;
    attribute a : Integer;
    attribute total : Integer = a * 2;
}
```

```turtle
elmt:P__total
    a sysml:AttributeUsage ;
    sysml:value expr:P__total_pvalue .

expr:P__total_pvalue
    a sysml:OperatorExpression ;
    sysx:sourceText "a * 2" ;
    sysml:elementId "P__total_pvalue" ;
    sysx:operator "*" ;
    sysml:argument expr:P__total_pvalue_pa0, expr:P__total_pvalue_pa1 .

expr:P__total_pvalue_pa0
    a sysml:FeatureReferenceExpression ;
    sysx:sourceText "a" ;
    sysml:referent elmt:P__a ;
    sysx:argumentIndex "0"^^xsd:integer .
```

The rules the tree follows:

- **A node's IRI is built from its owner and its position**: `expr:<owner id>_p<slot>`,
  and a nested operand appends its own index (`_pa0`, `_pa1`). Two expressions of
  one element therefore never collide, and the IRIs are deterministic, like element
  IRIs. The `_p` marker and the encoding of the position keep a node's id inside
  `[A-Za-z0-9_-]+`, the alphabet the SysML v2 API's `requireValidId` accepts, and
  it can never be read as an element id or a membership id, because an element id
  never ends in a lone `_`.
- **Every node carries `sysx:sourceText`**, the notation it was written as. The
  tree is *additive*: the text is what a conversion back to notation is written
  from while it still states the tree ([Source text](#source-text)), so
  exactness does not depend on the tree being complete, and
  `TestRoundTripIsLossless` covers the same round trip it did before. A node
  whose tree was edited after export is written back from the tree instead.
- **Operands are ordered** by `sysx:argumentIndex`, because an RDF graph is a set
  and `a - b` is not `b - a`.
- **Metaclasses are the standard ones** where the metamodel names them:
  `LiteralBoolean`, `LiteralInteger`, `LiteralRational`, `LiteralString`,
  `LiteralInfinity`, `NullExpression`, `FeatureReferenceExpression`,
  `FeatureChainExpression`, `OperatorExpression`, `InvocationExpression`,
  `CollectExpression`, `SelectExpression`, `ConstructorExpression`,
  `MetadataAccessExpression`, `Expression` for a body. `sysx:operator`,
  `sysx:argumentIndex` and `sysx:sourceText` are the properties the metamodel
  does not define.
- **A literal's `sysml:value` is a typed literal** whose lexical form is the
  token the notation spells it with: `"2"^^xsd:integer`, `"1.5"^^xsd:decimal`,
  `"1.5E3"^^xsd:double` (an exponent is outside `xsd:decimal`'s lexical space),
  `"true"^^xsd:boolean`, and a string with its escapes resolved. Read back, a
  value is spelled as that token again: a rational with no fractional digits
  (`"3"^^xsd:decimal`) gains them (`3.0`), a boolean is `true` or `false`, a
  string is quoted and escaped; a value no token spells — a signed number, `INF`,
  `NaN` — is reported as unsupported, naming the node, since the notation states
  a sign as an operator applied to a literal.
- **A `LiteralString` carries its value**, the escapes of the notation read: a
  `"say \"hi\""` in the file is `sysml:value "say \"hi\""` in Turtle, and a
  value edited in the graph is written back as the literal that reads to it.
  Control characters are written with Turtle's own escapes (`\b`, `\f`, `\uXXXX`),
  so every triple stays a single line of valid Turtle.
- **A feature reference links to the element** it names (`sysml:referent`) when
  that element is in the graph, and carries its name as a literal when it
  resolves outside it, the same rule the declaration-head relationships follow.
- **A node carries `sysml:elementId`**, the id its own IRI ends in, so it can be
  read and queried by that id like an element. It is still not a model element:
  it has no `sysml:qualifiedName` and no ownership properties, it is reached only
  from the position that holds it, and reading a graph back never turns one into
  a declaration. Writing expressions as elements owned through memberships is
  separate work.
- **A graph from another tool is read from its structure** when it carries no
  `sysx:sourceText`: the supported shapes above are written back as notation, and
  a shape this mapping cannot write (a missing operator, an operand count an
  operator does not take, a literal with no value) is reported as unsupported,
  naming the node, never guessed.
- **Parentheses follow the parser's precedence table.** The tree records no
  parentheses, so the writer places them where the grammar needs them: an operand
  that binds more loosely than the operator around it is parenthesized
  (`size(ae) == (if isEmpty(af) ? 0 else 2) and …`, `(p ?? q) implies r`,
  `(a + b)[1]`, `(x as T).f`, `- (1 + 2) ** 2`, `not (p and q)`), one that binds as
  tightly or tighter is not (`a + b * c`, `if p ? x else - x`, `p hastype T or q`).
  An index encloses a sequence, so a multi-dimensional index is written bare
  (`cube#(2, 1, 2)`, `m[1, 2]`), never `cube#((2, 1, 2))`.
  A conditional, being the loosest form, is parenthesized wherever it is an
  operand or the condition of another conditional; as the operand of
  `**` the left side must bind tighter than exponentiation, so `(a ** b) ** c`
  keeps its parentheses while `a ** b ** c` groups to the right, as the parser
  reads it. Text kept from `sysx:sourceText` is placed the same way, so a
  foreign operand written into a kept expression is parenthesized when needed.
- **An expression body is structure too.** `{ in y : Real; y + x }` is a
  `sysml:Expression` node whose `sysx:bodyParameter`s are nodes of their own —
  each typed `sysml:ReferenceUsage` with `sysml:direction "in"`, its name, `ref`
  flag, `sysml:type`, bounds, `sysml:value` and any body of its own — and whose
  `sysx:resultExpression` is the tree of the expression after them, so a nested
  body (`{ in y : Real; f(x = { in z : Real; z + y }) }`) and an `in expr`
  parameter's body (`in expr keep : Boolean { in v : Real; v > x }`) rebuild from
  the graph alone. The node states `sysx:hasBody`, so an empty body (`{}`) is
  told apart from an expression with no structure at all and comes back as `{}`.
  Documentation opening a body (`{ doc /* … */ in y : Real; y }`)
  is a `sysml:Documentation` node with its `sysml:body`. Any other declaration a
  body makes ahead of its result (`{ in y : Real; private attribute k : Real = 2; y * k }`)
  is a `sysx:BodyMember` carrying its notation; a graph that states one without its
  `sysx:sourceText` is reported, naming the member, as is a parameter with no
  `sysml:declaredName`. Parameters and declarations share one `sysx:memberIndex`
  sequence, so a parameter written after a declaration comes back after it.
- **Older graphs still read.** A position holding a plain literal
  (`sysml:value "1200.0"`), which is what releases before this wrote, is read as
  that notation, and a `sysx:bodyParameter` holding a bare name literal is read
  as that parameter.

Tests: `w6g4_rdf_expr_test.go` (structure, ordering, per-position identity,
legacy literals, foreign trees, unsupported shapes, round-trip exactness),
`result_expression_test.go` (expression bodies, their parameters and members).

### Set and tensor values

The mapping states a model, not an evaluation of it, so a value the runtime
holds as a set (a `Collections::Set`'s `elements`, any unique, unordered
collection) or as a tensor of any rank (`Quantities::TensorQuantityValue`)
has **no literal form** in RDF. What the graph carries is the expression the
feature is written with — the `(3, 1, 2, 2, 3)` valuing `elements`, the
`TensorCalculations::'['(…, cubeRef)` building the tensor, the `cube#(2, 1, 2)`
indexing it — as the typed tree above, and evaluating the model read back gives
the same set or tensor, in the runtime's canonical order. No `xsd` datatype or
`sysx:` vocabulary encodes an evaluated collection or a tensor's shape, and the
RDF conversion never evaluates: a graph that wanted to state a set's members or a
tensor's components would have to state the expression that yields them. The
gRPC service is where evaluated values travel (the `set` and `tensorQuantity`
arms of [the wire contract](wire-contract.md)).

Tests: `set_tensor_rdf_test.go` (exactness with and without the source text,
the expression trees a set-valued and a tensor-valued feature state, the
absence of any evaluated form).

### Result expressions

A calculation, case, analysis or verification body may end in a bare expression,
the result it computes (`calc def Double { in x : Real; x * 2 }`). The abstract
syntax owns that expression through a `ResultExpressionMembership` whose
`ownedResultExpression` redefines `ownedMemberFeature` — the Expression *is* the
member — and so does the graph: the expression is an element of its own, typed
by its expression metaclass, placed by `sysx:memberIndex` like every other
member so a body whose result follows other declarations comes back in the same
order (a graph that states no index, as a standard one does, gets it last, where
the grammar has it), and owned through a membership typed `sysml:ResultExpressionMembership`
that states it as both `sysml:memberElement` and `sysml:ownedResultExpression`:

```turtle
elmt:P__Double___401
    a sysml:OperatorExpression ;
    sysml:qualifiedName "P::Double::@1" ;
    sysx:memberIndex "1"^^xsd:integer ;
    sysml:owningMembership elmt:P__Double___401_om ;
    sysml:operator "*" ;
    sysml:argument expr:P__Double___401_pa0, expr:P__Double___401_pa1 ;
    sysx:sourceText "    x * 2\n" .

elmt:P__Double___401_om
    a sysml:ResultExpressionMembership ;
    sysml:memberElement elmt:P__Double___401 ;
    sysml:ownedMemberFeature elmt:P__Double___401 ;
    sysml:ownedResultExpression elmt:P__Double___401 .
```

The expression has no name, so it is addressed by position, as the shorthand
relationships under [Limitations](#limitations) are. Being an element, its
`sysx:sourceText` is its lines as written, as under [Source text](#source-text),
rather than the bare notation an expression node carries. It is the same tree a
feature value is, so it converts back from the graph with no `sysx:sourceText`
at all, whether it is an operator, a literal, an invocation, a feature chain, a
conditional or an expression body. Any Expression a `ResultExpressionMembership`
owns is written back as its body's result, so a graph another tool wrote with no
`sysx:` term on it reads too. A result whose graph states no expression
structure is reported, naming the expression, rather than written as an empty
line.

Tests: `result_expression_test.go` (the membership, the place among other
members, the round trip with `sysx:sourceText` stripped, the trip from the
membership alone, the refusals) and the `result_expressions` and
`expression_body_members` fixtures under `testdata/convert/`.

## Behavior

An action or state body converts: each node in it has a metaclass and the
properties its notation is rebuilt from, so `notation → RDF → notation` returns
the body byte for byte (`behavior_test.go`). Where the OMG vocabulary names
the node, that name is used; the rest are `sysx:` terms, marked below.

| written | metaclass | carries |
|---|---|---|
| `first x;`, `first x then y { … }` | `sysx:InitialNode` | `sysml:sourceFeature` (the member the body starts at — a reference, not a name it declares), `sysml:targetFeature`, `sysx:guard`, `sysx:hasBody` and the members of its body |
| `done;` | `sysx:FinalNode` | — |
| `action a;`, `action a { x + 1 }` | `sysx:ActionExecutionNode` | `sysml:references` or `sysx:expression` |
| `perform a;` | `sysml:PerformActionUsage` | `sysx:expression` (the action performed) |
| `assign x := 1;` | `sysml:AssignmentActionUsage` | `sysx:target`, `sysml:value`, `sysx:assignmentOperator` when it is not `:=` |
| `send M(x) to p;`, `… via p;` | `sysml:SendActionUsage` | `sysx:payload`, `sysx:receiver`, `sysx:isVia` |
| `terminate;`, `terminate x;` | `sysml:TerminateActionUsage` | `sysx:expression` |
| `accept sig : Signal;`, `accept when c;` | the usage's own metaclass | `sysml:isAccept`, and `sysx:declaredKeyword "accept"` where the optional `action` was not written |
| `fork`, `join`, `merge`, `decide` | `sysml:ForkNode`, `JoinNode`, `MergeNode`, `DecisionNode` | `sysml:declaredName` |
| `succession first a then b;`, `if g then b;`, `else b;` | `sysml:SuccessionAsUsage` | `sysml:sourceFeature`, `sysml:targetFeature`, `sysx:guard`, `sysx:isElse`, `sysx:declaredKeyword` |
| `public succession S first a if g then b;` (a guarded succession, which is a transition) | `sysml:TransitionUsage` | as a transition, with `sysx:declaredKeyword "succession"` for the keyword written; `sysx:transitionSyntax` is derived from where the AST places the source, not from the words ahead of it, so a visibility or a name does not change it. Written back, a named form always writes `first` (`succession S first a …`, `transition T first a …`), since only a nameless `transition` may state a bare source |
| `while c { … }`, `loop { … } until c;` | `sysml:WhileLoopActionUsage` | `sysx:whileCondition`, `sysx:untilCondition` |
| `for x in c { … }` | `sysml:ForLoopActionUsage` | `sysx:loopVariable`, `sysx:collection` |
| `if c { … } else { … }` | `sysml:IfActionUsage` + `sysx:IfBranch` per branch | `sysx:condition`, `sysx:branchKind` |
| `state s { … }`, `state s parallel { … }`, `entry; then s; state s;` | `sysml:StateUsage` | `sysml:declaredName`, `sysx:declaredKeyword`, `sysml:isParallel`, its members |
| `entry`/`do`/`exit`, `entry do { … }` (whatever separates the `do` from the body) | `sysml:StateSubactionMembership` | `sysx:subactionKind`, `sysx:declaredKeyword`, its actions |
| `defer sig, other;` | `sysx:DeferMember` | `sysx:deferredEvent` per event |
| `choice`, `junction`, `fork`, `join`, `entry point`, `exit point`, `shallow`/`deep history` | `sysx:Pseudostate` | `sysx:pseudostateKind`, `sysx:declaredKeyword` |
| `transition [n] [first] s [accept t] [if g] [do e] then t;`, `… then t { … }` | `sysml:TransitionUsage` | `sysml:sourceFeature`, `sysml:targetFeature`, `sysx:trigger`, `sysx:triggerKeyword`, `sysx:guard`, `sysx:transitionSyntax`, its effect and body as members, linked by `sysx:effectMember` and `sysx:bodyMember`, with `sysx:bracedEffect` on every transition written with `do` (true for its braces, so an empty `do { }` survives) and `sysx:hasBody` for a trailing body; a graph with members linked by neither owns an effect alone, `sysx:hasBody` its braces |

A state's members are held in the AST in one bucket per kind (entry, do, exit,
defer, substates); they are written back in the order they were
declared, taken from their source spans, so `do` before `entry` stays that way.

The conditions and expressions these nodes carry are expression trees, like
every other expression-valued position ([Expressions](#expressions)): they
convert back exactly *and* SPARQL can see inside them.

What is still refused, naming the node:

- **A succession that does not name both of its ends.** `then fork;` and
  `then monitorPedal;` written after a preceding member express an order whose
  source end the notation leaves implicit, and the parser records the node the
  statement introduces separately from the edge into it. Reconstructing that
  shape would mean inferring which node an edge belongs to from member position,
  which could silently reattach edges, so it is reported instead. Nine of the
  eighteen remaining refusals under `examples/` are this shape.

## Limitations

These are the constructs the mapping does not fully represent. Each is a
documented limitation, not a silent one: converting an affected element from a
graph that lacks the source text reports an error naming the element rather than
guessing.

**An expression tree is not the metamodel's own expression model.** Feature
values, multiplicity bounds, filter and constraint conditions and guards are
expression trees ([Expressions](#expressions)), which makes them queryable, but
the nodes are not `Feature`s owned through `FeatureMembership`s the way the
abstract syntax models an expression. A conversion back to notation is written
from the text each node was written as where the graph carries it, and from the
tree where it does not. A consumer that wants the metamodel's own shape does not
get it from this mapping; the one membership it does materialize is the
`ResultExpressionMembership` of a [result expression](#result-expressions).

**Lexical comments survive the RDF hop only as source text.** `//` and `/* */`
trivia is attached to no element in the graph's structure; it comes back because
the lines carrying it are the `sysx:sourceText` of the member they precede
([Source text](#source-text)). A graph without that text — from another tool,
or stripped — drops it:

```sysml
// this line comes back with the source text, and is gone without it
package Demo {
    doc /* this is kept either way: doc is a declaration, not trivia */
    comment about Wheel /* kept for the same reason */
    part def Wheel;
}
```

The `comment` and `doc` keywords declare elements, so they convert both ways. An
element whose text is stale — its graph was edited after export — is rebuilt
canonically, and a comment on its lines goes with the text. Save straight to
`.sysml` when the comments must survive an edit; that path writes the source and
keeps everything.

**A reference is written in the spelling that resolves, where it is written, to
the element the graph names.** Every reference an element carries — a
specialization, subsetting, redefinition, reference-subsetting or typing target,
the root and members of a feature chain, an import, a succession, connection or
transition end, the requirement a `satisfy` names — is a link to that element,
not a name. Writing it back, the converter spells the link as the short name when
the resolver reads that name, from the writing scope, as the linked element, and
otherwise as the shortest qualified name it does read that way. So a redefining
attribute that bears its target's name inside a definition whose supertype also
redefines it writes `redefines Packets::'packet data field'`, since the short
name there would reach the inherited redefinition; a `part payload :> payload`
whose target is the package's `payload` writes `subsets Shadowing::payload`,
since `payload` inside the definition would be the subsetting part itself; and a
`: Packet` inside a definition that declares its own `Packet` writes
`: Shadowing::Packet` when the outer one is meant. The scope a spelling is read
in is the one the parser reads it in: a `featured by` or `crosses` target in a
feature's head is read in that feature's own scope first, where its type's
members are visible, so a `member feature` nested in an anonymous
`portion :>> startShot` that is featured by that portion writes `featured by
CC1::startShot`, since the short name in the feature's head would reach the
inherited `Occurrence::startShot` instead. A name shadowed at every
level falls back to the global form (`$::Shadowing::Packet`), and an element
that no spelling reaches from where it is written is reported rather than
written as a different element. What a spelling reaches can depend on how the
references beside it are spelled — an import's short name may read through a
sibling import only while that sibling is written qualified — so the chosen
spellings are checked again in the notation that actually writes them, and
lengthened until every one reads as the graph states. The fixture
`testdata/convert/shadowed_references.sysml` covers the three shadowings, and
`TestRoundTripIsLossless` writes every fixture back from the graph with its
`sysx:sourceText` removed and requires the graph the notation produces to be the
one it came from (`export_test.go:TestWrittenReferencesResolveWhereWritten`,
`TestPacketsRoundTripsStructurally`).

**A head comes back in one spelling.** The graph carries what a head declares,
not how it was spelled, so the notation written back is normalised where the
notation offers a choice and the model does not:

- A relationship written as a symbol or as its keyword (`:>` or `subsets`,
  `:>>` or `redefines`, `::>` or `references`) is the same relationship
  element, so no spelling is recorded and the writer uses one form. This differs
  from `sysx:declaredKeyword`, which is kept where the notation's synonyms name
  *different* declarations (`datatype` and `attribute`).
- The modifiers of a usage are written in the grammar's order (`end #derive r1
  : R;`, `end ref cause : S[*];`), and a multiplicity goes with the typing
  clause it qualifies, or with the name when there is none (`composite
  frontWheel[2] redefines w`). The parser reads the same flags in either order
  and either position (`export_test.go:TestFixturesComeBackFromTheGraphAlone`).
- A `doc` or `comment` body is carried with the line endings it was written
  with, but the notation written back uses the document's own — a body written
  with CRLF comes back with LF. The text is otherwise verbatim.

A second conversion of the notation written back gives the same graph; only
`sysx:sourceText`, which quotes the source verbatim, shows the respelling.

### End-binding heads

**A head that binds ends records the form it writes them in.** A `connect`,
`bind`, `flow`, `succession`, `transition`, `accept` or `satisfy` declaration is
carried as `sysx:sourceText` (the exact text is what a save writes back) and,
beside it, as the structure the head expresses: its ends, and `sysx:endForm`, the
notation those ends are written in. The form is what makes the head
reconstructible without the text, so a graph from another tool converts to
notation as well:

```turtle
elmt:P__Car___402
    a sysml:ConnectionUsage ;
    sysx:sourceText "connect left to right;" ;
    sysx:endForm "to" ;
    sysx:relatedFeature expr:P__Car___402.end0, expr:P__Car___402.end1 .

expr:P__Car___402.end0
    a sysml:FeatureReferenceExpression ;
    sysx:sourceText "left" ;
    sysml:referent elmt:P__Car__left ;
    sysx:endIndex "0"^^xsd:integer .
```

`sysx:relatedFeature` points at one expression node per end, each carrying
`sysx:endIndex` and — for a flow — `sysx:endRole` (`source`, `target`,
`payload`). An end written behind a multiplicity (`connect [1] a to [0..1] b`,
`bind [0..1] a = [0..1] b`) carries its bounds on that node as `sysml:lowerBound`
and `sysml:upperBound` expression nodes, the way a feature carries its own; the
decoder writes them back ahead of the end. A binding's two ends are two such
nodes like a succession's or a connector's — `bind a = b` relates `end0` for `a`
and `end1` for `b` — and neither is the connector's `sysml:value`: a binding
states no value and no `sysml:references` of its own
(`export_test.go:TestBindingEndMultiplicitiesAreStatedAsStructure`,
`binding_connector_ends_test.go`). An end that
declares a name of its own and reference-subsets the feature it attaches to
(`connect bead ::> t.bead to …`, KerML `connector a ::> a.x to b;`,
`bind e1 ::> a = e2 references b;`) relates that
feature — `sysml:referent` or `sysml:targetFeature` is `t.bead`, not `bead` —
and carries the name as `sysx:endName` on the same node, with
`sysx:endReferencesKeyword "references"` where the source spelled the word; the
decoder writes it back as `<name> ::> <feature>` unless that spelling is recorded
(`export_test.go:TestKerMLBinaryConnectorEndsCarryTheRoundTripWithoutSourceText`,
`binding_connector_ends_test.go:TestKerMLBindingConnectorEndsCarryTheRoundTripWithoutSourceText`).
A named end that relates no feature, or one the graph names twice, is refused as
that connector end rather than written (`TestBindingEndsWithoutANotationAreRefused`).
A KerML binary connector without `from` starts with its first end, so
`connector eng to t;` is an anonymous connector whose `end0` is `eng`, and a
named one writes `from` as its `sysx:endVerb`. The forms and what each writes:

| `sysx:endForm` | Notation | Head |
|----------------|----------|------|
| `to` | `<end0> to <end1, …>` | `connect a to b`, `allocate a to b`, `connector c from a to b` |
| `nary` | `(<end0>, <end1>, …)` | `connect (a, b, c)` |
| `equals` | `<end0> = <end1>` | `bind a = b`, `bind e1 ::> a = e2 references b`, `binding [1] of a = b`, `binding of e1 ::> a = e2 ::> b` |
| `firstThen` | `<end0> then <end1>` | `succession first a then b`, `succession [n] first a then b` |
| `fromTo` | `[of <payload>] from <end0> to <end1>` | `flow of P from a to b` |
| `flowTo` | `[of <payload>] <end0> to <end1>` | `flow a to b` |
| `satisfy` | `<requirement>` (the `sysml:subsets` end, written bare) | `satisfy R by v`, `verify R` |
| `then` | the source end is the nearest feature written before it that is not a connector or a transition; a member that is not a feature is read past | `then b;`, `then part b;` |

A head whose own keyword is the noun form writes a verb ahead of its ends, and
that verb is `sysx:endVerb` (`connection c connect a to b`). Where the keyword
is a synonym for the kind (`verify` for a satisfy, `allocate` for an
allocation) it is carried as `sysx:declaredKeyword`, as elsewhere.

An anonymous connector's own multiplicity (`sysml:lowerBound`/`sysml:upperBound`
on the connector, as against on an end node) is its declaration, and is written
ahead of the ends: `succession [n] first a then b`, `binding [1] of a = b`. A
declaration is always followed by the end verb, since `binding [1] a = b` reads
the leading `[1]` as the first end's multiplicity in both notations: a `binding`
or `succession` that declares something but recorded no `sysx:endVerb` is written
with KerML `of`/`first` or SysML `bind`/`first`, which the second hop then records
as its verb. SysML's `bind` shorthand declares nothing, so a `bind` whose graph
states a multiplicity is written `binding [1] bind a = b`.

**The form is only recorded when rebuilding from it reproduces the head's tokens.**
The encoder writes the ends back from `sysx:endForm` and compares them with the
source, whitespace and comments aside, before recording it — a head written over
several lines, or with a note inside it, records its form like any other
(`export_test.go:TestEndFormsSurviveIrregularLayout`) — so a head this mapping
cannot rebuild carries no form and stays readable as text alone. Those are the heads that say
more than their ends: an end that redefines, an inline payload declaration
(`flow of x : P from a to b`), or a satisfy that declares a name of its own
(`satisfy s : R by v`).
Converting such an element from a graph that carries no `sysx:sourceText` is
reported, not guessed. A graph that relates ends but gives no form at all is
reported the same way (`export_test.go:TestEndsWithoutTheirFormAreReported`).

**The body of such a head is mapped like any other body.** `sysx:sourceText`
carries the head's own lines and `sysx:sourceTail` the closing ones, as for any
member with a body (see [Source text](#source-text)), and the members written in the
body (`interface seam connect w.outp to r.inp { attribute coupling : C = C::x; }`)
are elements of their own, owned through `sysml:ownedMember`,
`sysml:ownedFeature` and their membership with a `sysx:memberIndex`, with
`sysx:hasBody` stating that a body was written. The same holds for the body an
action's `first a then b { … }` or `then b { … }` carries. The decoder writes
the body from those members whether or not the graph carries the head's text.

Tests: `export_test.go:TestEndBindingHeadsComeBackFromTheGraphAlone`,
`TestEndBindingBodiesComeBackFromTheGraphAlone` and
`TestBehavioralHeadsComeBackFromTheGraphAlone` strip `sysx:sourceText` from the
graph, write the notation back from the mapping alone, and convert it again.
The second graph must equal the first up to the text triples, which is what
proves the second hop loses nothing. `TestBindingEndsAreStatedAsStructure` covers the ends themselves.

**A succession carries its two ends.** Every succession is one node naming
the members it sequences, whether it was written as its own member
(`succession first a then b;`) or attached to one (`then action b : B;`, which
the parser desugars to the same edge), so the order a model declares survives
the hop:

```turtle
elmt:P__Move___402
    a sysml:SuccessionAsUsage ;
    sysx:endForm "then" ;
    sysml:targetFeature elmt:P__Move__c ;
    sysx:sourceMember elmt:P__Move__a .
```

A `then` that names neither end writes `sysx:sourceMember` and
`sysx:targetMember` instead, pointing at the members it sequences, since the
notation gives them no name. That is what carries a `then` beside a member
the notation leaves unnamed (`then send Show(x) to screen;`, a state's
`entry; then s1;`), the shape the parser used to warn about
(`unnamed-succession-end`) and the encoder used to refuse. Both ends are
positions in one body, so writing them back is exact: the source end is the
member before the succession, and a target that *is* that preceding member is
the declaration the `then` was written ahead of.

The member a `then` sequences from is the one the parser gives it: the nearest
feature before it that is not a connector or a transition. A member that is not
a feature — a `doc`, a `comment`, a `rep`, an `import`, an `alias`, a nested
definition or `package`, a `multiplicity` declaration, a state's `defer` — declares
nothing a succession can run from, so a `then` written after one is read past it. A connector of any kind, named or not
(`connect p to q;`, `interface i connect …`, `allocate`, `bind`, `flow`,
`succession`), and a transition relate other members rather than declaring one,
so those are read past too, while an `attribute`, a `part`, an `action`, a
metadata usage or any other feature is the source. This is the pilot
implementation's rule (`UsageUtil.getPreviousFeature`, which walks back over
every owned member that is not a Feature). Skipping the non-feature members is
also the literal reading of SysML v2 §7.17.4, which describes the source as
"the nearest occurrence lexically previous to the `then`, skipping over any
intervening non-occurrence usages" — a `doc` or an import is not a usage at
all. The connector part of the rule follows the pilot where that text is
underdetermined: read literally it would sequence from a connection (an
occurrence usage) and read past an attribute (not one), the opposite of the
pilot on both counts, and §8.3.13.6 `SuccessionAsUsage` states no constraint for
the implied source (OMG issue SYSML21-171 records the omission). Two parts of
the pilot's rule are not followed: the pilot sequences from a `flow` or
`message` written with no ends (`message m;`), which this implementation reads
past like any other connector, and it resolves an `alias` of a feature to that
feature, where this implementation reads past the alias as §7.17.4 does — both
known gaps. The writer folds a succession back into `then` by the same rule,
shared with the parser as `ast.IsSuccessionSource` (over `ast.UsageKind.IsEdge`
for the connector kinds), so `action a; flow from a.x to b.x; then action b;`,
`action a; connect p to q; then action b;` and `action a; doc /* */ then action
b;` come back as written. The
source end is compared as the name the member answers to, which is what the
parser records: a `first a then b;` sequences from `a`, and a `perform walk;`
or `action redefines walk;` that declares no name of its own answers to `walk`
(KerML 7.3.4.5). A graph describing a position the notation cannot express —
sequencing from an earlier member, or from the connector, documentation or
definition the `then` is read past — is reported rather than written back
somewhere else (`export_test.go:TestUnnamedSuccessionEndComesBackFromTheGraph`,
`TestHalfNamedSuccessionInAGraphIsReported`,
`behavior_test.go:TestThenComesBackPastTheMembersTheParserSkips`,
`TestThenIsRefusedWhenTheGraphSequencesFromAnotherMember`,
`TestThenIsRefusedWhenTheGraphSequencesFromANonFeature`).

Every body that can carry a succession (definition, usage, action, state,
including a parallel state's regions, calculation and requirement) reads these
forms back as the same node, and on the fixtures a second conversion writes the
same Turtle byte for byte (`export_test.go:TestSuccessionRoundTripsInEveryBody`).
That is a statement about the fixtures, not the mapping: over the example corpus
the second hop reproduces the graph for all 303 files that convert, but from
the source text they carry, which the corpus gate does not strip
([rdf-corpus-roundtrip.md](../project/rdf-corpus-roundtrip.md)). An end
whose name needs quotes (`first a then 'drive vehicle';`) is a reference to the
element like any other; the writer quotes the name as the notation requires.

**Conditions convert as their notation.** The members that express
a condition are carried, each as the `sysx:` metaclass named above with its
condition as `sysx:condition`: a constraint body's conditions (`assert`,
`assume`, a bare condition, and the `not` of `assert not …` as
`sysml:isNegated`), a nested `assert constraint [name] { … }`, a requirement's
`assume`/`require` members in all three forms (an expression, the constraint
they name, or a body) together with the declaration of the constraint usage they
own — `sysml:declaredName`, its specializations, `sysml:lowerBound`/`upperBound`
and `sysml:value` with its `default`/`:=` operator (`require #Goal constraint braked [1] = true;`) — and
`subject s : X;` as the `sysml:SubjectMembership` it declares. The `assert` prefixing a named usage
(`assert constraint c : C`) is carried as `sysx:declaredPrefix`. The conditions
themselves are notation, with the limits stated above. An `assume`/`require`
member's `sysx:declaredKeyword`, when present, is `constraint`; any other value
is reported rather than the member written in a form the keyword did not state.
A member is written in one of these forms, so a graph stating an inline
`sysx:condition` together with facts of another form — a `constraint` keyword,
a body, a `sysml:references`, a name, specializations, a multiplicity or a
value — is reported rather than the condition written and the rest dropped.

The nodes in an action or state body are mapped under
[Behavior](#behavior), together with the shapes still refused there.

**A synonym keyword on a declaration with no name of its own is carried like a
named one.** `feature :>> x;`, `composite :>> e = v;`, `snapshot :>> start { … }`,
`timeslice :>> portionOfLife { … }`, `event m.start;`, `event occurrence e;`,
`assert constraint { … }`, `assert c { … }` and `assert not c;` all come back from
the graph alone: the portion, the event and the assertion are typed
(`sysml:portionKind`, `sysml:EventOccurrenceUsage`, `sysml:AssertConstraintUsage`
with `sysml:isNegated`), the occurrence or constraint an `event m.start` or
`assert c` names is its `sysml:references`, and KerML's `feature` is
`sysx:declaredKeyword` — see [What each element carries](#what-each-element-carries)
for how the decoder chooses the spelling. Reading back, the head is spelled from
the typed facts: `snapshot`/`timeslice` from the portion kind, `event` from the
metaclass or `sysml:isEvent`, `assert` from the metaclass with `not` from
`sysml:isNegated`, each as the kind keyword itself where `sysx:declaredKeyword`
says it was written so and as a modifier ahead of `occurrence`/`constraint`
otherwise. What is still refused is a keyword that takes a reference in place
of a name — `perform`, `exhibit`, a state's `entry`/`do`/`exit`, `event`,
`assert` — in a shape the notation cannot state. With neither a
`sysml:declaredName` nor a `sysml:references` the graph has nothing to put in
either of the keyword's two places, `perform a` and `perform action a`. With a
`sysml:declaredName` under `perform`, `exhibit`, `entry`/`do`/`exit` or `event`
the name has no place at all: `event e;` names the `e` it refers to, and the
declaration is spelled `event occurrence e;`, which the graph does not state.
Under `assert` a name is read only where a typing, specialization, `references`
clause or value follows it (`assert safe : Safe;`, which the parser reads as a
declaration), so a named assertion that nothing but a body or a multiplicity
follows — `assert c { … }`, `assert c[1]` — is refused rather than written as a
reference to a different constraint. The parser never produces these shapes
(`perform;` declares a feature named `perform`), so only a graph from another
tool, or one edited by hand, states them; the decoder's refusal names the
keyword and the fact at odds.

**A metadata annotation is carried structurally**, as described under [What
each element carries](#what-each-element-carries): its type, its `about`
targets, the sigil it was written with and its body's members as owned members
with their `sysml:value` expression trees, so `@Safety { level = 2; }` and
`#Safety part def Car;` come back from the graph alone. Four shapes are reported
rather than written, and only a graph from another tool can state them: a
metadata usage whose one `sysml:type` is a subject of another metaclass (a
`sysml:PartDefinition`, say — a literal type names an element the graph does not
define, so it is written as it is), a `#`
prefix carrying a name, an `about` clause or a body (the grammar's
`PrefixMetadataUsage` is the type alone, so the parser never produces one), a
`#` prefix owned by an element whose head has no prefix position (a state's
`entry` action, say), and a `#` prefix on a head kept as `sysx:sourceText`
(`#Safety connect x to y;`) whose text does not write it. `@Safety part def Car;` is not a prefix in the grammar —
`@` introduces a member of its own, so the parser reports the missing `;` or
`{` after `@Safety` — and it is refused at the parser, before conversion.

**A name declared twice in one namespace is refused.** An element's derived id
is the encoding of its qualified name, so `part def A; part def A;` in one
container would merge into a single subject. The duplicate is reported instead.

A shorthand relationship declares no name of its own: the `result` in `bind result = x;`
and the `x` in `first x;` name the end the statement relates. Those elements are
therefore addressed by position (`sysx:memberIndex`) and the name is carried as a
reference. Without that they would collide with the member they name and the model
would be refused as a duplicate.

**Unsupported on the RDF input side**, each reported as an error naming the line or element:

- blank nodes and `[ ... ]` — every element must have a stable IRI
- RDF collections `( ... )` — order is carried by `sysx:memberIndex`
- an element with no `rdf:type`, or a metaclass outside the mapping, or several
  `rdf:type`s none of which is a subclass of all the others
- a reference whose IRI names no subject of the graph and whose id no subject
  carries as `sysml:elementId`; a dangling id is reported as such, never left
  as a silently unresolvable name
- a referenced element with no `sysml:qualifiedName`; the name is read from
  that property, never recovered from the IRI, so a graph with foreign ids
  (UUIDs, say) converts exactly as long as it carries the names
- an element whose `sysml:owningNamespace` is not in the graph
- ownership that forms a cycle, leaving an element no root owns; printing walks
  down from the roots, so this would otherwise write an empty document
- Turtle syntax errors, reported with a line number
- literal shorthands (bare numbers and booleans); literals must be quoted,
  with an `xsd:` datatype where one applies
- a literal whose datatype its property does not take, or with a language tag.
  Every metamodel property the mapping reads as text is a `String`, so a name
  is a plain or `xsd:string` literal; `"3"^^xsd:integer` or `"x"@en` stated as
  one is a different term, not the name `3` or `x`, and is reported naming the
  literal and the subject that states it. The other properties take the
  datatypes the ontology gives them, so a plain string is refused there too:
  `xsd:boolean` for the flags and `sysx:hasBody`; `xsd:integer` or `xsd:int`
  for the `sysx:` indexes and the bounds; and for the `sysml:value` of a
  literal expression, by its class, `xsd:integer` or `xsd:int`
  (`LiteralInteger`), `xsd:decimal`, `owl:real`, `xsd:double` or `xsd:float`
  (`LiteralRational`), `xsd:boolean` (`LiteralBoolean`) or a string
  (`LiteralString`). A `sysx:Expression` literal is taken only where the
  mapping writes notation — a relationship target — never as a name
- a literal whose text is outside its datatype's lexical space
  (`"false"^^xsd:int`, `"yes"^^xsd:boolean`, `"1e3"^^xsd:decimal`): it is no
  term of that datatype, so it is reported rather than read as the text it
  spells, as is an `xsd:int` outside its 32-bit value space. `owl:real`, which
  names no lexical forms of its own, takes a finite `xsd:double`'s
- a `sysx:` index (`sysx:memberIndex`, `sysx:argumentIndex`, `sysx:endIndex`)
  that is negative or too large for the platform's `int`: it is a position the
  writer orders by, and one it cannot hold would otherwise be read as 0 and
  move the member to the front
- a subject stating a single-valued property twice with different objects —
  a body with two `sysx:resultExpression`s, an element with two
  `sysx:memberIndex`es, two `sysx:isNamespaceImport` flags or a
  `sysml:isDefault` stated both true and false: only one could be written, so
  the graph is refused naming both rather than the first being kept. Every
  `sysx:` property is single-valued but the members and parameters of a body,
  `sysx:relatedFeature`, `sysx:deferredEvent` and `sysx:prefixMetadata`; of
  the `sysml:` properties, the boolean `is…` flags and `sysml:portionKind`
  are. A triple stated twice is one triple to the graph, so only differing
  objects are a conflict
- a `sysml:isDefault` or `sysml:isInitial`, whether true or false, on a subject
  with no `sysml:value`: the flags spell the operator a feature value is
  written with (`default =`, `:=`), so without a value there is nothing to
  write them on

A graph that uses none of OpenSysML's `sysx:` properties (one produced by
another tool) converts as far as the mapping allows and errors on the first
element it cannot place, rather than emitting a model with elements missing.

## Where the code lives

| Package | Role |
|---------|------|
| `internal/core/rdf` | Triple/graph model, Turtle writer, Turtle parser |
| `internal/core/export` | `ToRDF` (AST → graph), `ToSysML` (graph → notation), and the `Convert` entry point |
| `internal/core/export/corpus_roundtrip_test.go` | The per-file round-trip ratchet over every model under `examples/`, with its baseline in `testdata/corpus_roundtrip_expected.txt` ([rdf-corpus-roundtrip.md](../project/rdf-corpus-roundtrip.md)) |
| `internal/repl` | `%save` |
| `cmd/sysml` | `-convert`, `-from`, `-o` |

The RDF layer is hand-written against the Turtle grammar rather than pulled in
as a dependency: the subset needed here is small, and the parser rejects what it
does not support instead of accepting it and dropping data.

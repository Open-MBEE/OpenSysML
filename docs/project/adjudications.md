# Adjudications: the divergences we keep, and the rows still open

This is an engineering record. It collects the decisions behind the places where OpenSysML
deliberately differs from the pinned OMG pilot implementation, the rows that are still open against
it, and the reasoning each one rests on. Decisions that later work closed are not kept here: the
behaviour they describe is now the implementation's, and the tests named beside each rule are what
holds it.

**No figure in this record is a current measurement.** The current agreement figures are the
generated block in [README](../../README.md) and [architecture](../internals/architecture.md),
regenerated and gated by `make docs-counts`, and the per-row verdicts are the committed oracle
baselines beside this file.

Every open row carries one of four categories — an unlabelled open row reads as a defect:

| Category | Meaning |
|---|---|
| **our defect** | we are wrong and the pilot is right |
| **unimplemented obligation** | the specification states the rule; we do not implement it yet |
| **pilot limitation** | the pilot's declared expectation does not follow from the specification |
| **adjudicated divergence** | we deliberately differ, with the reading recorded |

A fifth case is *not* a category: the **published material is itself wrong**. Those rows are
declared in the [errata overlay](errata-overlay.md) with a citation and a derivation, and the
overlay never relabels one of the four categories above.

---

## Notation decisions

### `allocate` in usage position requires a connector part

The pinned grammar spells the allocation usage keyword `allocation` (`AllocationKeyword`,
`SysML.xtext:1192`) and gives `allocate` one role — `AllocateKeyword`, which must be followed by a
`ConnectorPart` (`SysML.xtext:1076-1079`, binary or n-ary, the binary form requiring `to`). So
`allocate f to g;` and `allocation al;` are well formed and `allocate al;` has no derivation. We
require the connector part accordingly, and the definition-side spelling is `allocation def` only
(`SysML.xtext:1196-1198`). The RDF export keeps the two spellings distinguishable through
`sysx:declaredKeyword`.

### A bare `import` is an error, and we report it once

`ImportPrefix` makes the visibility indicator mandatory (KerML 8.2.2, SysML v2 7.2.2), deliberately
— the `MemberPrefix` beside it in the same grammar leaves visibility optional:

```xtext
fragment ImportPrefix returns SysML::Import :
	visibility = VisibilityIndicator          // no '?'
	'import' ( isImportAll ?= 'all' )?
;

fragment MemberPrefix returns SysML::Membership :
	( visibility = VisibilityIndicator )?     // optional
;
```

`import Q::*;` is therefore ill-formed and we report it as an error
(`internal/core/passes/import_visibility.go`). `expose` stays exempt: the pinned grammar gives it an
implicit protected visibility (`ExposeVisibilityKind`, `SysML.xtext:2366-2372`).

**Adjudicated divergence, kept.** The pilot emits two diagnostics for the bare form — one
`mismatched input 'import'` plus the recovery cascade at the closing brace, which describes its
parser's state rather than the model. We recover at the import and report the violated rule once, at
the same offset and severity. Reproducing the pair means either naming an internal parser state in a
diagnostic or degrading recovery so a bare import derails the rest of the file. The rows are counted
as disagreements rather than reclassified as wording-only, because the second declared error has no
counterpart of ours at all.

---

## Name resolution and visibility

### A visible-name walk is bounded by member source, not by depth or by name count

`Base.kerml` declares the derivation that makes deep paths deep:

```kerml
abstract classifier Anything {
    feature self: Anything[1] subsets things chains things.that
    feature things: Anything [1..*] nonunique
    feature that : Anything[1]
}
```

`self` and `that` are ordinary members contributed by a general type, so a path through them
continues exactly as far as membership does. KerML 8.2.3.5 makes a qualified name a walk over
memberships, and a namespace's members include those inherited through its generalizations
(8.3.3.1.4) and those imported (8.2.3.4), with no separate budget for derived features. What bounds
the walk is that a namespace may not be re-entered *along the same path*: a circular containment
declares `A.B.B` and stops.

Two earlier readings — a per-*name* occurrence count, and a per-*depth* step budget — are both
underdetermined by the specification, and each fit only part of the corpus, over-enumerating one
fixture while omitting hundreds of names on another. The rule is per **member source**: a path may
continue through a source it has not already traversed on that path, and a feature's declared type is
entered before its implicit base.

`internal/core/model/scope_names.go` carries it: `enter`/`leave` maintain the traversed set for the
current path, `enterFor` exempts the first level, `chainTo`/`chainAvoiding` find a route to a member's
declarer and a detour when the direct route is already on the path, and `expand` falls back to
implicit members only when every supertype of a symbol is already traversed. Locked by
`TestVisibleNamesInheritedFeatureEndsThePath` and
`TestVisibleNamesMutualImportBoundsPathsNotImplicitMembers`, beside the existing re-entry,
implicit-member and determinism tests.

**What is not claimed.** The pilot's truncation of a circular containment is a fact of its corpus
that we match, not a sentence we can cite; and a construct with no scope assertion is not endorsed
by that silence. If a future fixture contradicts the rule, this is the paragraph to reopen.

### A connector end's participant is not one of the connector's own features

```kerml
package test {
    feature x;
    feature y;
    connector c {
        feature f;
        end a ::> x :> f[1];
        end b ::> f[1];       // 'f' must not resolve
        end c ::> y[1];
    }
}
```

`::>` is reference subsetting (KerML 7.4.11), and on a connector end it names the **participant** the
end relates. A connector's ends relate features of the type that features the connector (KerML
8.3.4.5), so the connector's own features are not candidates: `f` is featured by `c`, and only `x` and
`y`, featured by `test`, are. `refFilter.featuredBy` (`internal/core/resolve/target.go`) hides one
scope's members from a reference, and `resolveRelationships` (`document.go`) sets it for a
`RelReferences` target of an `end` whose owner is a connector-kind usage. The narrowing to connector
kinds is load-bearing: an `end` inside a plain `feature` relates nothing and keeps ordinary
visibility. Locked by `TestConnectorEndDoesNotReferenceAFeatureOfTheConnector` and
`TestEndOutsideAConnectorReferencesItsOwnersFeature`.

### An unnamed redefinition of an invisible feature binds no name

A feature with no declared name takes the **effective name** of the feature it redefines (KerML
7.3.4.5), so `feature redefines c` binds a name only if `c` resolves. Where `c` is private in the
redefined scope it does not, and the member binds nothing — indexing the borrowed name anyway masks
the later error a reference through it should draw. `BindsName`
(`internal/core/resolve/unqualified.go`) excludes a member whose effective name comes from a
redefinition that names no visible feature, memoized in `Resolver.effNames` under the existing
`naming` cycle guard and applied through `localBinding` on every local step of the unqualified walk.
Locked by `TestUnnamedRedefinitionOfAnInvisibleFeatureBindsNoName`.

### A recursive import follows publicly re-exported memberships

KerML 8.2.3.5 makes a recursive import take the memberships of the target namespace and,
recursively, of the namespaces it contains — and a namespace's memberships include the ones it
re-exports publicly, which is how `ISQ` publishes `ISQBase`'s names. A recursive branch that walks
the containment tree but consults only each scope's own declarations makes a re-exported name visible
through `ISQ::*` and invisible through `ISQ::**`; both go through the same re-export-aware traversal
(`appendSubtree`), keeping the cycle guard, `importAdmits`, filters, visibility and the body-local
exclusion. Covered by `internal/core/resolve/f67_import_reexport_test.go` and `filter_test.go`.

### Open — a reference's position is resolved without regard to the metaclass it admits

```kerml
classifier non{}
feature aa subsets non;              // pilot: Couldn't resolve reference to Feature 'non'.
feature c : OuterPackage::B.b;       // pilot: … to Feature 'OuterPackage::B'. and … to Feature 'b'.
```

`Subsetting::subsettedFeature` and `Feature::chainingFeature` are typed `Feature`, so a `Classifier`
in either position is a linking failure for the pilot. We resolve the name without regard to the
metaclass the position admits, then reject the kind of what we found — same element and offset,
a metaclass finding rather than a failed lookup. Closing these rows means resolving references
against the metaclass the position requires, which is a cross-cutting change to name resolution.

**Our defect, open.** Owner: the resolver.

---

## Conformance policy we keep

### A redefinition's declared types are checked for direct conformance

```kerml
classifier Try specializes B {
    feature try : A::a1 redefines B::b;      // a1 is nested in A, not a specialization of it
}
```

A redefinition is a subsetting, and the values of a subsetting feature are values of the subsetted
feature (KerML 7.3.4.4–7.3.4.5, 8.3.3.3.8, 8.3.3.3.10). KerML states that relation in terms of the
features' effective co-domains but defines no validator constraint requiring every explicitly named
type of the redefining feature to conform directly to every explicitly named type of the redefined
one: the redefining feature is typed by its own typings *and* the redefined feature's (8.3.3.3.4,
8.3.3.3.6), and the pilot's own `ShapeItems` library relies on that (`item :>> faces : Polygon`
under `faces : StructuredSurface`). The pilot leaves the consequence to model semantics or a
reasoner; we apply the direct pairwise test as an advisory check.

**Adjudicated divergence, kept as a warning.** `redefinition-type-mismatch` reports a redefinition
typed by two unrelated types, because it is almost always a slip, but as a warning, so no model the
specification admits is rejected. The two `Import3` `noErrors` rows in
[pilot-xpect.md](pilot-xpect.md#noerrors--265-of-276-agree) closed with that severity; calling them
wording-only would have been false — the pilot emits nothing there. (The same check once caught the
non-conforming individual redefinition in `Individuals Examples/AnalysisIndividualExample.sysml`,
fixed upstream at `2026-07`.)

### A specialization cycle is reported, and the pilot has no such check

The fixtures that declare silence here declare real cycles: `classifier a specializes b` with
`classifier b specializes a`; `part p1 :> p2; part p2 :> p3; part p3 :> p1;` and `part p4 :> p4;`;
`part def A :> C` with `part def C :> A, B`. KerML 7.3.3.2 and 7.3.4.2 make specialization a strict
partial order, so none of them is satisfiable by any model, and subsetting is a kind of
specialization (7.3.4.4, 8.3.3.3.10) whose transitive closure is irreflexive. The pilot implements no
cycle check of any kind, so its silence records an absent check rather than a reading of the
specification.

**Adjudicated divergence, kept.** Closing these rows means deleting a correct rule.

### Quantity commensurability is analysed, and the pilot does not analyse it

`22/2*25.4 + 110 [mm]` adds a dimensionless value to a length: KerML 8.2.5.8.1–8.2.5.8.2 make `[mm]`
a primary bracket expression, so it binds to `110` rather than to the sum. `1/(2 * Cp) * V^2 +
T_static` adds L^6 to Θ, `V : VolumeValue` making `V^2` L^6 and `Cp : DimensionOneValue` leaving that
dimension unchanged. SysML v2 §9.8.9.1 requires the operands and result of an addition to share a
quantity dimension and top-level quantity type, and says an implementation should warn or error
otherwise. Both published examples violate the rule as written; the pilot implements no
corresponding static check.

**Adjudicated divergence, kept** — and, because the defect is in the published examples rather than
in either implementation, both rows are also declared in the [errata overlay](errata-overlay.md).

### Interface arity is a property of a binary interface

`Interfaces::Interface` declares `ref port :>> participant : Port[2..*] nonunique ordered` — it is
n-ary, and only `BinaryInterface` narrows it to two participants with `source`/`target`. SysML v2
§7.14.1 permits an interface with three or more ends, while §7.14.2 and §8.3.14.2 constrain
`BinaryInterface` alone, so the arity rule applies only to an interface conforming to a binary base
(`passes/constraint.go` `interfaceIsBinary` over `semantics.Model.IsBinaryConnector`). A probe
confirmed the split on the pilot itself: six ends typed by `Interface` draw no arity diagnostic from
it, three ends typed by `BinaryInterface` do.

**Pilot limitation** for the fixture that declares the rule universally.

---

## Metadata and model-level evaluability

Model-level evaluability is a walk over the expression (`semantics/evaluable.go`,
`Model.ModelLevelEvaluable`). Literals, `null`, metadata access, sequences, constructors, invocations
of the Kernel Function Library functions the model evaluates, and reads of features that reach an
evaluable value are evaluable; any other function — including a library one the model cannot
evaluate, such as `RealFunctions::sqrt` — an operation no library function implements (`~3`), a cast
of the instance under evaluation (`(as A).y`) and an unresolved name are not. The evaluable set is
the one KerML gives model-level evaluation (§7.4.9 with the Kernel Function Library, §9.2): equality,
identity, type tests, casts, metadata access, sequence and indexing operations, the arithmetic,
comparison, range and Boolean data functions, and the control functions `.`, `if`, `and`, `or`,
`implies`, `??`, `collect` and `select`.

Around it:

- **One diagnostic per fault, on the membership.** `ElementFilterMembership`'s two constraints
  (KerML 8.2.4) are stated on the membership carrying the condition, so a condition with several
  faulty operands draws each fault once, at the membership, rather than once per operand.
- **`null` and `new T(…)` are model-level evaluable** (KerML 1.0 §7.4.9): `null` is the empty
  sequence, and a constructor is decided by the model when its arguments are — so a condition built
  from a constructor is reported for yielding an instance rather than a truth value.
- **A body is checked wherever it is written** (`passes/w8c_metadata_annotation.go`): the
  body-feature rule and the model-level evaluability of the body's bound values apply to prefix and
  non-prefix annotations alike.
- **What an annotation may annotate** (`semantics/annotated_element.go`): the metaclass of the
  annotated element must conform to every type of the `annotatedElement` feature the metadata type
  restates (KerML 1.0 §8.3.4.9).
- **A conditional `baseType` binding is decided per annotated element**, evaluated against that
  element's metaclass.
- **The diagnostic spans the binding, not the value.** SysML 7.24's `checkMetadataBodyFeature`
  judges the `FeatureValue`, whose notation begins at the binding operator, so the parser records the
  `=` / `:=` / `default =` operator's span on the usage and the metadata diagnostic reads it
  (`internal/core/parser/feature_value_operator_test.go`,
  `internal/core/passes/w8c_metadata_annotation_test.go`).
- **Open — an unresolved invocation target draws one diagnostic, not two.** For
  `y = ScalarFunctions::sqrt(4.0)` in a metadata body the pilot reports both `Must be model-level
  evaluable` and an unresolved reference; we report only the first, because metadata-body expressions
  are not visited by the name-resolution pass. **Our defect, open**; owner: the resolver's visiting,
  not the metadata pass.

---

## Structural rules derived from the normative library

- **An event occurrence in a port body is referential**, not composite, so the referential-nesting
  rule's own exemption applies: `metadata def EventOccurrenceUsage specializes OccurrenceUsage {
  derived attribute isReference : Boolean[1..1] redefines isReference; … }` (`Systems
  Library/SysML.sysml`). The declaring direction still holds — a composite `part b4 : B;` in a port
  body is reported (`passes/w10b_structural.go`, `passes/w10b_structural_test.go`).
- **An index-only binary base still supplies its two ends.** `allocation def Allocation :>
  BinaryConnection` (`Systems Library/Allocations.sysml`) inherits two effective ends, so an
  allocation definition with no declared end relates two elements; `Links::BinaryLink` is index-only
  in `internal/core/libs`, with specialization edges but no parsed body, which is why the count comes
  from semantics (`passes/w10b_related_elements.go`, `semantics/connector.go`). A generic one-end
  concrete connection is still reported.
- **Accessibility of a `satisfy`/`verify` target** is skipped only for a dotted feature-chain target
  (`verify vehicleSpecification.vehicleMassRequirement`) — the notation the message itself prescribes
  — while a `::`-qualified target of another type is reported as `Must be an accessible feature (use
  dot notation for nesting)`, which is the pilot's behaviour on a probe of both forms.
- **Subject position** is a statement about the declaration's parameters in lexical order, so it is
  judged over local declarations, and the semantic member order is consulted only where no local
  parameter can be inspected (`TestW7GLocalResultSuppressesInheritedParameterFallback`).

---

## Parser behaviour that decides what a rule can see

- **A connector end parses one target and stops at the comma.** `ConnectorEnd` admits exactly one
  `OwnedReferenceSubsetting` plus the relationship forms of that production (`::>`, `references`, an
  explicit `:>>`), so in `allocate logical references l, physical references p;` the later ends are
  ends, not relationship targets folded into the first one
  (`internal/core/parser/connector_ends_nary_test.go`).
- **An anonymous enumerated value is not named after its keyword.** `EnumeratedValue` admits a
  keyworded value with no declared name, so the enum body recognizes `enum` followed by `=` or `:=`
  as an anonymous value rather than a value called `enum`; `enum red;` still declares `red`
  (`internal/core/parser/f61_keywordless_members_test.go`).
- **An index operand is a sequence.** Both index forms take a `SequenceExpression`
  (`KerMLExpressions.xtext`, `PrimaryExpression`), so `arr#(1,3)` parses.
- **A feature with a value and no declared type is typed by its value** (KerML 7.4.9), so `b`'s
  members in `feature b = a#(1).b;` are the members of `a#(1).b`. `resolve/document.go` falls back to
  the value's type when a usage declares none, guarded against cycles by `valuesInProgress`, and
  `resolve/target.go` resolves `#(…)` indexing through its operand. Bracket indexing is deliberately
  not routed this way: `[…]` carries quantity and selection meanings that are not "one element of".
- **Prefix metadata is admitted on every member declaration.** KerML 8.2.4's `PrefixMetadataMember`
  allows a prefix on any member, and SysML §7.20's `SubjectMember`, `ActorMember` and
  `StakeholderMember` are member declarations like the rest.
- **A plain-name `exhibit` is a reference, not a declaration.** Only `state` introduces a
  declaration in `ExhibitStateUsageDeclaration`; every other form carries an
  `OwnedReferenceSubsetting`, so `exhibit s;` is an unnamed state usage with a `references`
  relationship (`behavior_exhibit_reference.sysml` locks all four shapes).

### Recovery from a malformed qualified name is unspecified

Six fixtures write a name that cannot parse (`Test3::A:a`, `A..b`, `test..A`) after an earlier line
that also carries an unresolved reference. The pilot's generated parser reports the syntax failure and
keeps linking, so it reports the earlier reference too, and its declared texts are ANTLR internals
(`no viable alternative at input '..'`, `mismatched input`). KerML 8.2 defines the grammar and the
resolution of a qualified name; nothing in KerML or SysML states how an implementation must recover
from one that does not parse, nor how many diagnostics one file must produce.
`parseRelationshipTarget` reports `expected a name after '.'` once and reads the rest of the chain
(`internal/core/parser/chain_double_dot_test.go` pins one diagnostic, at the dots' own offset, with
the following declaration still in the tree).

**Adjudicated divergence, kept.** Emitting an ANTLR alternative trace, or a diagnostic naming an
internal parser state, would move counts without changing what we detect. **Known limitation:** only
the two-dot shape recovers cleanly; `:> a...b` and `:> ..a` still cascade.

### Open — a guard's type is not reported behind a lower-tier failure

`if "test"` as a transition guard draws `transition guard must be Boolean, found String` in
isolation, but where the same file has a name-resolution-tier error the type tier is skipped
(`passes/registry.go`) and the row stays open. Tiering is an architecture invariant, chosen so type
diagnostics are never derived from unresolved names; reporting this row means flattening the tiers or
exempting the rule from them.

**Adjudicated divergence**, owned by [element-scoped tier gating](element-scoped-tier-gating.md).
Nothing in the specification requires a particular *number* of diagnostics for one file, so this is a
divergence in reporting completeness rather than in a rule.

### Open — query path expressions are not parsed, and the pinned pilot rejects them too

```sysml
value v1_i: Integer[0..*] = .*/.*[Integer];
value v2_r: Real[0..*] = .**[Real];
value v_redefining: Integer = ./vehicle_1/cylinders/@redefining;
```

The three fixtures declare silence, and we report `expected a namespace member`. Running the pinned
pilot's own validator on their model bodies reports `no viable alternative at input '/'` and
`For input string: "."`, so **the pinned pilot does not parse these files either** — they live under
its `failing/` tree, and the declared silence is the notation's intent rather than the behaviour of
the pinned release. Nothing in KerML 8.2 or SysML §7 admits `.*`, `.**`, `/`-separated traversal or
`@redefining` as expression syntax at this version.

**Pilot limitation. Owner: upstream** — the question is drafted in [omg-issues.md](omg-issues.md).

---

## How the Xpect oracle reads its own assertions

Two harness rules are decisions rather than measurements, and both live in `cmd/pilot-xpect`:

- **An `at "…"` clause runs to the last quote on the line.** Xpect does not escape quotes inside the
  clause, so reading to the *first* inner quote splits one assertion into a truncated expectation and
  a junk one.
- **A scope assertion anchors at the reference its text names.** The pilot's Xpect method takes the
  offset's `EObject` *and its cross-reference*, so `scopeAnchor` (`scope.go`) prefers the first
  occurrence that carries a reference and falls back to the first whole identifier, while `locate`
  (`compare.go`) still requires a whole identifier for *diagnostic* matching — relaxing that lets an
  assertion match a prefix inside a longer name.

# Diagnostics: codes, lints and checker warnings

Every diagnostic OpenSysML reports — a syntax error, an unresolved name, a typing or
validation-constraint failure, a lint, a document or query plan error — carries a stable,
kebab-case *code* naming the rule that reported it. The code is the `code` field under
`-json`, in the editor (with a `codeDescription` opening this page at the code's section)
and over the wire, and it sits between the severity and the message of every text line:

```text
model.sysml:3:12: error[unresolved]: unresolved reference: Engine
```

One rule has one code however it words its message, so a CI filter, a baseline or a
quick-fix can key on it. `sysml -explain <code>` prints what a code means, how to resolve
it and the clause or validation constraint behind it; bare `-explain` lists every code with
a one-line summary, as the [catalogue](#catalogue) below does. The REPL's `%explain-code` and the MCP tool
`explain_diagnostic` answer the same question. Where a code reports a pilot validation
constraint, its name is the constraint's, so [validation-constraints.md](../project/validation-constraints.md)
and [spec-compliance.md](../project/spec-compliance.md) line up with it.

## Lints

A *lint* is a warning about a model the specification accepts: nothing in SysML v2 or KerML
makes the written model wrong, but it is almost always a slip. Each lint can be switched off
by its code. A lint marked *opt-in* below is off until it is switched on
by its code, since what it reports holds of ordinary models more often than it marks a slip.
A lint is a warning in every mode:
[`-strict`](../guide/03-command-line.md#strict-conformance) promotes notation no SysML v2
production admits and the [duplicate member names](#duplicate-member-names) KerML forbids, and a
lint is neither, so it stays a warning and never changes the exit status or blocks a check.

| Code | Reported on | Tier | Default |
|------|-------------|------|---------|
| `undeclared-signal` | a transition's `when <name>` whose name matches no declaration visible where it is written and no signal the model sends | name resolution | on |
| `port-type-mismatch` | a `connect`, an interface usage or a `flow` joining two ports whose definitions are unrelated | constraint | on |
| `deferred-keeper-unmarked` | an accept of a deferred signal at the root of the do action of a state annotated `MigrationMetadata::DeferredEvent` that is not marked `MigrationMetadata::DeferredKeeper` | name resolution | on |
| `rounded-real-literal` | a decimal literal bound or assigned to a feature typed by `Real` that no binary64 value equals, so the feature holds it rounded | type | opt-in |

## Action-step multiplicity warnings

These constraint-level warnings report valid SysML models whose action-step multiplicities or
succession ordering the executor cannot safely implement. They are emitted by `-validate` with
source `action-step-multiplicity`; they are not lints and cannot be disabled with lint settings.

| Code | Reported when |
|------|---------------|
| `action-step-multiplicity-not-fixed` | An action-step or written succession-end multiplicity does not evaluate to a fixed exact count |
| `action-step-multiplicity-unsupported` | A repeated or zero-count step has an unsupported control-node, guard, pin, binding, connection, external-feature-read, state-behavior, part-level performed-action, or loop/conditional block-flow interaction, or a bound is beyond the 64-bit range (for example, `1180591620717411303424`) |
| `action-step-order-unsatisfiable` | A succession end forces an endpoint count that it does not admit |
| `action-step-order-open` | The declared or defaulted succession ends do not force the endpoint counts under both readings of an unwritten end |

## Duplicate member names

The name-resolution tier reports KerML's distinguishability rule (`validateNamespaceDistinguishibility`)
with code `name-conflict`; they are not lints and cannot be switched off. By default they are
warnings; under [`-strict`](../guide/03-command-line.md#strict-conformance) (the REPL's `%strict`,
the editor's `strictConformance`, the service's `strict_conformance`) the owned, alias, short-name
and inherited wordings are errors, since a namespace that keeps two indistinguishable memberships
is not well-formed. The imported wording stays a warning in every mode: KerML 7.2.5.4 hides the
colliding names rather than keeping them, so the namespace is well-formed and the warning marks
only where OpenSysML's resolution departs from that (below). Two
memberships of one namespace are indistinguishable when one's name or short name is the other's
and their metaclasses are related (a `part def` beside a `part def`, or beside an `item def`
that it specializes; a `part def` beside an `attribute` is distinguishable whatever the names).
For owned and inherited members resolution is not affected: a reference to the name still takes
the first membership, in declaration order. Two *imported* members of one name are different:
KerML (7.2.5.4) hides both from the importing namespace, so the warning marks memberships the
namespace does not have, and an unqualified reference to the name is looked up in the enclosing
namespaces instead — where none has it, the ordinary `unresolved reference` error is reported, with
a hint naming the imports that hide the name: `unresolved reference: Engine — did you mean P::Engine
or Q::Engine? The name is hidden here: import P::* and import Q::* bring distinct elements named
'Engine', so none is a member; qualify the one meant: P::Engine or Q::Engine.` A qualified name
(`P::Engine`) resolves as ever; the importing namespace's own qualified name (`Use::Engine`) does
not reach a hidden membership, and a namespace that re-exports both imports has no such member to
re-export. (The OMG pilot implementation binds the first import's member instead and reports
nothing.) A called name is read the same way: two `calc def pick` that two imports bring are
hidden, so `pick(2)` is unresolved with the hint above, while a set one namespace owns, a type
inherits or one import brings still selects by signature.

| Wording | Reported on |
|---------|-------------|
| `Duplicate of other owned member name 'x'` | the later of two owned members of one namespace |
| `Duplicate of inherited member name 'x' from T` | an owned member of a type whose name an inherited member already has |
| `Duplicate of imported member name 'x': P::x (import P::*), Q::x (import Q::*)` | the import that brings the later of two imported members, naming each member and the import that brought it — these memberships are hidden from the importing namespace; a member a type inherits that an imported one repeats is named `(inherited)`, and that pair is not hidden, the inherited member resolving |

An imported name hidden by an owned member of the same name, one membership reached through two
imports (`import P::*` beside `import Q::*` where `Q` publicly re-imports `P`), an alias or a
membership import of an element beside the membership that owns it, and the standard library's
own members are not reported, and none of these is hidden. An alias under another name (`alias Spare for P::x`) binds only
that name and does not stand in for `P::x`.

The standard library has eight such pairs of its own, which `import ISQ::*` brings in without a
warning: `MagneticDipoleMomentValue`, `MagneticDipoleMomentUnit`,
`CartesianMagneticDipoleMoment3dVector`, `CartesianMagneticDipoleMoment3dCoordinateFrame`,
`magneticDipoleMoment` and `cartesianMagneticDipoleMoment3dVector` are defined in both
`ISQElectromagnetism` (IEC 80000-6) and `ISQAtomicNuclear` (ISO 80000-10) as two different
quantities, and `CartesianDisplacement3dVector` and `cartesianDisplacement3dVector` in both
`ISQSpaceTime` and `ISQCondensedMatter`. Unqualified, each binds the `ISQElectromagnetism` or
`ISQSpaceTime` member; write `ISQAtomicNuclear::MagneticDipoleMomentValue` (or
`ISQCondensedMatter::…`) for the other.

## Switching a lint off or on

| Surface | Off | On (opt-in lints) |
|---------|-----|-------------------|
| CLI | `-disable-lint <code>[,<code>…]`, repeatable; an unknown code is a usage error (exit 2) | `-enable-lint <code>[,<code>…]`, the same way |
| REPL | `%lint <code> off` | `%lint <code> on` |
| Language server | `disabledLints`, a list of codes, in `initializationOptions` or `workspace/didChangeConfiguration` ([LSP extensions](lsp.md#disabled-lints-setting)) | `enabledLints`, the same way |
| Go | `model.WithDisabledLints(codes...)` when the workspace is made, or `(*model.Workspace).SetDisabledLints(codes)` | `model.WithEnabledLints(codes...)` or `(*model.Workspace).SetEnabledLints(codes)` |

`%lint` alone lists every lint, on or off, and switching one reprints the session's
diagnostics. A lint both disabled and enabled is off. Enabling a lint that is on by default
changes nothing. A lint left out is left out of what the workspace reports, not out of the
analysis, so switching it back needs no re-analysis. The gRPC service and the clients report
every lint that is on by default, with its code, and no opt-in lint; a client that does not
want one drops the diagnostics carrying its code.

## `undeclared-signal` in detail

OpenSysML accepts a signal trigger written without `accept` — `transition first a when Ping
then b;`. The spelling is not standard: the pinned pilot grammar admits `when` only as a
change trigger over a Boolean expression inside an `accept` (`SysML.xtext:1483-1485`,
`ChangeTriggerKind`) ([conformance audit](grammar/conformance-audit.md)). The name such a
trigger carries names an event, not a model element: it is left unresolved, and at run time it
matches a signal injected or sent by that name. A misspelled name therefore matches nothing
and the transition never fires, silently.

The lint reports the name when all of the following hold:

- no declaration of that name is visible where the trigger is written, by the ordinary
  scoping rules (KerML §7.2.5, §8.2.3.5);
- no `send` anywhere in the workspace sends a signal by that name, the name the runtime
  gives its message and compares the trigger's final segment with: the definition a
  payload names or constructs (`send Ping to self;`, `send new Ping() to self;`), through
  any alias to the definition it reaches, and the type of a payload feature's value
  (`send reading to self;` with `attribute reading : Real = 1.0;` sends `Real`, not
  `reading`). A written message is the one sent; the body's `payload` parameter is read
  only where the send writes none. A name that resolves to nothing counts as written.
  A send invoking a calculation (`send Ping() to self;` with `calc def Ping`) sends the
  calculation's value, as the runtime does, so it counts the result's type, not `Ping`.
  A literal payload counts its scalar type, as the runtime names it: `send "go" to self;`
  sends `String`, and integer, real and boolean literals send `Integer`, `Real` and `Boolean`.
  A computed payload counts the scalar type its value is known to have (`send 1 + 2 to self;`
  sends `Integer`, `send "a" + "b" to self;` `String`, `send not ready to self;` `Boolean`);
  a number whose kind is not known statically (`send r * 2.0 to self;`) counts both
  `Integer` and `Real`, the two names the runtime gives a number it sends.
  A document held as its interface record counts too: the record keeps the names its body
  sends.

The finding offers the resolver's nearest names in scope as `did you mean …?`:

```text
m.sysml:3:73: warning: `when Pnig` names no declaration visible here and no signal the model sends, so only a signal injected by that name triggers it — did you mean Ping?
```

A signal the model only ever receives from outside (`%send` at the prompt, an injected
event over the wire) is reported, since nothing in the model says it exists; declare it
(`attribute def Ping;`) to say so, or switch the lint off. The trigger's resolution and
behavior are unchanged either way.

## `port-type-mismatch` in detail

Two ports a connector joins are compatible (SysML v2 §7.12.2, §7.12.3) when one of these
holds:

- one's definition specializes the other's, or both specialize a common definition of the
  model (a library definition such as `Ports::Port`, which every port specializes, does not
  count);
- one port's directed features each have a feature of the other with the same name, the
  conjugate direction (`in` against `out`; `inout` against `inout`) and a conforming type —
  a conjugated port (`port p : ~P`) reverses its definition's directions first;
- the connector is typed by an interface definition with two ends, both typed by a port
  definition, directly or through a model end it redefines: that interface decides what the
  ends pair, and its own ends are judged by `port-conjugation`. An interface leaving an end
  untyped decides nothing, and nothing judges a connection definition's or a many-ended
  interface's ends, so their usages' concrete ends are judged.

A port connected is typed by its own declaration, or else by the nearest model port it
redefines (`port :>> p;` keeps the type and conjugation of the `p` it redefines).

Otherwise the lint reports the connector at its first end:

```text
m.sysml:9:13: warning: this connection connects port power : PowerOut to port fuel : FuelIn, whose definitions are unrelated and whose directed features are not conjugate; type one end by the conjugate port (~PowerOut) or by a common definition
```

It covers `connect a.p to b.q;` (and `connection … connect`), interface usages
(`interface connect a.p to b.q;`) and `flow` between two ports, whose syntax is
`ConnectionUsage`, `InterfaceUsage` and `FlowUsage` (`SysML.xtext:1062`, `:1153`, `:1269`).
An end that is not a port, or whose port type does not resolve, is not judged. The
specification states no constraint of this kind, so it is a lint rather than an error.

## `deferred-keeper-unmarked` in detail

A SysML v1 state's deferred signal is migrated to the standard encoding described under
[Deferred signals](sysml-v1-migration.md#deferred-signals): the state is annotated
`@MigrationMetadata::DeferredEvent { ref :>> signal : Sig; }`, and its do action keeps each
occurrence of `Sig` through an accept loop whose accept is written
`#MigrationMetadata::DeferredKeeper action receive accept kept : Sig;`. The runtime knows the
keeping accept by that annotation alone — it is the accept that yields an occurrence to any
other accept of the state able to take it, and keeps what nothing else takes — so an accept of
the deferred signal written without it is an ordinary accept, which consumes the occurrence.
Output migrated before the marker existed wrote the loop's accept bare, and so does a model
written by hand after the pattern.

The lint reports each accept node at the root of the do action of a state annotated
`MigrationMetadata::DeferredEvent` whose payload is typed by the signal the annotation names,
when no accept of that signal there carries `MigrationMetadata::DeferredKeeper`: once one does,
the state has its keeping loop, and a bare accept of the signal beside it is the ordinary
consumer the marker exists to tell apart, which takes an occurrence first. Both annotations
are known by their resolved type, however the model spells them (through an import, an alias
or `$::MigrationMetadata`), and a metadata definition of the model that merely shares the
library name is not one of them.

```text
m.sysml:12:5: warning: accept of deferred signal Ping is not marked #MigrationMetadata::DeferredKeeper, so it is an ordinary accept, not the keeping loop; re-migrate the model or mark it
```

The model still analyses and runs; the accept simply takes each occurrence rather than keeping
it. Re-migrate the model, which writes the marker on every keeping accept, or write
`#MigrationMetadata::DeferredKeeper` on the accept yourself. An accept of the signal nested
below the do action's root, one beside a marked keeper of the signal, or one under a state the
annotation does not name, is an ordinary accept by design and is not reported, nor is an
accept whose signal does not resolve, which name resolution reports.

## `rounded-real-literal` in detail

A decimal literal is the exact Rational it spells (`0.1` is 1/10, KerML 1.0 §8.4.4.9.2), while
a feature typed by `Real` holds an IEEE 754 binary64 value, so a literal written to one is
rounded once to the nearest double. Where that double differs from the literal, a model reading
the feature back gets a value other than the one it wrote, and the lint names the value held:

```text
m.sysml:3:24: warning: 0.1 is rounded to the nearest Real, 0.10000000000000001, since a feature typed by Real holds a binary64 value; type the feature by Rational to keep the literal exact
```

It reports a literal, signed or not, written as a feature's value (`attribute x : Real = 0.1;`,
a parameter's default, a redefinition such as `attribute :>> x = 0.2;` of a `Real` feature) or
written by an `assign`, and each element of a sequence written out (`(0.25, 0.1)` reports
`0.1`). The feature is `Real` by its declared type, or else by the type it inherits. A literal
binary64 holds exactly (`0.25`, `1.5e3`), an Integer literal, a feature typed by `Rational`
and a computed value (`1 / 3`, `0.1 + 0.2`) are not reported, and neither is a literal beyond
the binary64 range, which the run refuses. Comparison with the feature is at Real precision
([exact Rationals](../project/exact-rational-evaluation.md#comparing-a-rational-with-a-binary64-real)),
so `x == 0.1` still holds; the lint is about the value the feature holds and reports.

The lint is opt-in: a decimal written to a `Real` feature is how ordinary models state
measured values (`efficiency = 0.92`), and nearly all of them round. Switch it on with
`-enable-lint rounded-real-literal`, `%lint rounded-real-literal on` or the `enabledLints`
editor setting to find the literals whose rounding matters.

## Catalogue

Every code, by the tier that reports it. A family (`document-plan`, `document-query`) is one
entry whose emitted codes append the rule's name.

<!-- diagnostics-catalogue:begin -->
_Generated from the diagnostic code registry by `make diagnostics-catalogue`; `make diagnostics-check` fails when this section drifts from it. Edit the registry, not this text._

### Syntax

| Code | Severity | Summary |
|------|----------|---------|
| [`actor-membership-owning-type`](#actor-membership-owning-type) | error | `actor` outside a requirement or case body |
| [`defer-notation-removed`](#defer-notation-removed) | error | `defer <event>;` in a state body, a removed extension |
| [`element-filter-membership-owning-type`](#element-filter-membership-owning-type) | error | `filter` outside a package or view body |
| [`enumeration-body-member`](#enumeration-body-member) | error | a member an enumeration definition body does not admit |
| [`expose-owning-namespace`](#expose-owning-namespace) | error | `expose` outside a view body |
| [`framed-concern-membership-owning-type`](#framed-concern-membership-owning-type) | error | `frame` outside a requirement, concern or viewpoint body |
| [`import-visibility`](#import-visibility) | error | an `import` written without `public`, `private` or `protected` |
| [`kerml-notation`](#kerml-notation) | warning | KerML notation in a SysML file |
| [`nonstandard-notation`](#nonstandard-notation) | warning | notation OpenSysML reads that no production of the SysML v2 grammar admits |
| [`objective-membership-owning-type`](#objective-membership-owning-type) | error | `objective` outside a case body |
| [`requirement-constraint-membership-owning-type`](#requirement-constraint-membership-owning-type) | error | `assume` or `require` outside a requirement or case body |
| [`reserved-keyword-name`](#reserved-keyword-name) | error | a declared name spelled as a reserved keyword |
| [`stakeholder-membership-owning-type`](#stakeholder-membership-owning-type) | error | `stakeholder` outside a requirement body |
| [`state-subaction-membership-owning-type`](#state-subaction-membership-owning-type) | error | `entry`, `do` or `exit` outside a state body |
| [`subject-membership-owning-type`](#subject-membership-owning-type) | error | `subject` outside a requirement or case body |
| [`syntax`](#syntax) | error | the text does not parse as KerML or SysML v2 notation |
| [`sysml-notation`](#sysml-notation) | error | SysML notation in a KerML file |
| [`transition-membership-owning-type`](#transition-membership-owning-type) | error | `transition` outside a state body |
| [`undefined-operator`](#undefined-operator) | warning | the unary `~` operator, which the kernel library leaves undefined |
| [`view-rendering-membership-owning-type`](#view-rendering-membership-owning-type) | error | `render` outside a view body |

#### `actor-membership-owning-type`

**error** · Syntax · `actor` outside a requirement or case body

An actor is a parameter of a requirement or a case: `actor` is admitted in the body of a requirement, a case or one of their usages only. Move the member into the requirement or case it belongs to, or write it as an ordinary part usage.

Specification: SysML v2 validateActorMembershipOwningType

#### `defer-notation-removed`

**error** · Syntax · `defer <event>;` in a state body, a removed extension

The state body member `defer <event>;` was an OpenSysML extension that SysML v2 has no notation for, and it was removed. Model a deferred signal explicitly: an ordered buffer (`item deferred : Sig[*] ordered;`), a do action whose accept loop keeps each occurrence while the state holds it, and a transition out of the state that replays the buffer.

Specification: SysML v2 §7.16 (states have no deferral notation)

#### `element-filter-membership-owning-type`

**error** · Syntax · `filter` outside a package or view body

An element filter is a member of a package or a view: `filter` is admitted at the file root, in a package body, or in the body of a view definition or usage only. Move the member into the package or view whose members it selects.

Specification: SysML.xtext ElementFilterMember (a PackageBodyElement, ViewDefinitionBodyItem and ViewBodyItem)

#### `enumeration-body-member`

**error** · Syntax · a member an enumeration definition body does not admit

The body of an enumeration definition holds enumerated values, annotations and imports only; a part, attribute or other usage declared there is not admitted by the grammar. Declare the member in an attribute definition the enumeration specializes, or drop the keyword to make the line an enumerated value.

Specification: SysML v2 §8.2.2.8 EnumerationBody

#### `expose-owning-namespace`

**error** · Syntax · `expose` outside a view body

An expose is an import a view usage owns: `expose` is admitted in the body of a view usage (and, as an OpenSysML extension the notation rules report, a view definition) only. Move the expose into the view that shows the exposed elements, or write an `import` where a plain import is meant.

Specification: SysML v2 validateExposeOwningNamespace

#### `framed-concern-membership-owning-type`

**error** · Syntax · `frame` outside a requirement, concern or viewpoint body

A framed concern is a member of a requirement: `frame` is admitted in the body of a requirement, concern or viewpoint definition or usage only. Move the member into the requirement or viewpoint that frames the concern.

Specification: SysML.xtext FramedConcernMember (a RequirementBodyItem)

#### `import-visibility`

**error** · Syntax · an `import` written without `public`, `private` or `protected`

SysML v2 requires a visibility indicator before `import`; an import without one is KerML notation the SysML grammar does not admit. Write `private import` for an import the namespace uses itself, `public import` to re-export the imported members to whoever imports the namespace.

Specification: SysML v2 §8.2.2.4 ImportPrefix

#### `kerml-notation`

**warning** · Syntax · KerML notation in a SysML file

A `.sysml` file uses a KerML production (`namespace`, `featured by`, `class` …) or a KerML spelling (`typed by` for a typing, which SysML spells `:` or `defined by`) that the SysML v2 grammar does not admit. Move the declaration to a `.kerml` file, or write the SysML form the message names. `-strict` promotes this finding to an error.

Specification: the pinned SysML v2 Xtext grammar

#### `nonstandard-notation`

**warning** · Syntax · notation OpenSysML reads that no production of the SysML v2 grammar admits

The text is an OpenSysML extension: the parser reads it and the model is analyzed, but the pinned SysML v2 and KerML grammars have no production for it, so another tool will reject the file. The message states the standard form to write instead. `-strict` promotes this finding to an error.

Specification: the pinned SysML v2 and KerML Xtext grammars

#### `objective-membership-owning-type`

**error** · Syntax · `objective` outside a case body

An objective is the requirement a case is to meet: `objective` is admitted in the body of a case definition or usage (an analysis, verification or use case included) only. Move the member into the case it belongs to, or write it as an ordinary requirement usage.

Specification: SysML v2 validateObjectiveMembershipOwningType

#### `requirement-constraint-membership-owning-type`

**error** · Syntax · `assume` or `require` outside a requirement or case body

An assumed or required constraint is a member of a requirement: the `assume` and `require` prefixes are admitted in the body of a requirement, a case or one of their usages only. Move the constraint into the requirement it belongs to, or drop the prefix to declare an ordinary constraint usage.

Specification: SysML v2 validateRequirementConstraintMembershipOwningType

#### `reserved-keyword-name`

**error** · Syntax · a declared name spelled as a reserved keyword

An element is named by a word the grammar reserves (`part`, `action`, `end` …), which the ID terminal does not admit, so no reference can name it and the pilot rejects the file. Rename the element, or quote the name as an unrestricted name (`'part'`) where the keyword spelling is what the model needs.

Specification: KerML 1.0 §8.2.2.3 and SysML v2 §8.2.2 (the ID terminal and reserved words)

#### `stakeholder-membership-owning-type`

**error** · Syntax · `stakeholder` outside a requirement body

A stakeholder is a parameter of a requirement: `stakeholder` is admitted in the body of a requirement definition or usage only. Move the member into the requirement it belongs to, or write it as an ordinary part usage.

Specification: SysML v2 validateStakeholderMembershipOwningType

#### `state-subaction-membership-owning-type`

**error** · Syntax · `entry`, `do` or `exit` outside a state body

An entry, do or exit action is a subaction of a state: these keywords are admitted in the body of a state definition or usage only. Move the member into the state it belongs to, or drop the keyword to declare an ordinary action usage.

Specification: SysML v2 validateStateSubactionMembershipOwningType

#### `subject-membership-owning-type`

**error** · Syntax · `subject` outside a requirement or case body

A subject is a parameter of a requirement or a case: `subject` is admitted in the body of a requirement, a case or one of their usages only. Move the member into the requirement or case it belongs to, or write it as an ordinary usage.

Specification: SysML v2 validateSubjectMembershipOwningType

#### `syntax`

**error** · Syntax · the text does not parse as KerML or SysML v2 notation

The parser could not read the text as a production of the grammar: a token is missing, misplaced or unexpected, and the message names what was expected at that point. The parser recovers and reads on, so one slip can be followed by further findings; fix the first one reported and re-check. Every other syntax-tier code names a more specific rule; this one is the general ill-formed-parse finding.

Specification: KerML 1.0 §8.2 and SysML v2 §8.2, the textual notation grammars

#### `sysml-notation`

**error** · Syntax · SysML notation in a KerML file

A `.kerml` file uses a SysML v2 declaration keyword (`part`, `attribute`, `action` …) that the KerML grammar has no production for. Move the declaration to a `.sysml` file, or write the KerML form (`feature`, `classifier`, `behavior`).

Specification: the pinned KerML Xtext grammar

#### `transition-membership-owning-type`

**error** · Syntax · `transition` outside a state body

A transition usage is a member of a state: the `transition` keyword is admitted in the body of a state definition or usage only (a transition between action nodes is an OpenSysML extension the notation rules report separately). Move the transition into the state whose substates it joins, or write a succession (`first a then b;`) between actions.

Specification: SysML v2 §8.2.2.19 TransitionUsageMember (a StateBodyItem)

#### `undefined-operator`

**warning** · Syntax · the unary `~` operator, which the kernel library leaves undefined

The unary `~` maps to `DataFunctions::'~'`, which the Kernel Function Library declares abstract and never defines, so the expression has no value in any conforming evaluator. Write the operation meant explicitly (`not x` for a Boolean, a library function for a bitwise complement).

Specification: KerML 1.0 §8.2.5.8.1 and §9.4.3 DataFunctions

#### `view-rendering-membership-owning-type`

**error** · Syntax · `render` outside a view body

A rendering is a member of a view: `render` is admitted in the body of a view definition or usage only. Move the member into the view it renders, or declare a rendering usage (`rendering r : R;`) elsewhere.

Specification: SysML v2 validateViewRenderingMembershipOwningType

### Name resolution

| Code | Severity | Summary |
|------|----------|---------|
| [`ambiguous`](#ambiguous) | error | a name that resolves to several elements at once |
| [`library-package`](#library-package) | warning | a user package marked `standard library package` |
| [`library-root-name`](#library-root-name) | warning | a root namespace named like a standard library package |
| [`name-conflict`](#name-conflict) | warning | two members of one namespace with the same name |
| [`not-a-vertex`](#not-a-vertex) | error | a transition endpoint that names no state or pseudostate |
| [`unresolved`](#unresolved) | error | a name that resolves to no element visible where it is written |

#### `ambiguous`

**error** · Name resolution · a name that resolves to several elements at once

Two or more elements visible where the name is written answer to it with equal standing, typically through several wildcard imports, so the reference cannot choose. Qualify the name with the namespace meant, or narrow one of the imports.

Specification: KerML 1.0 §8.2.3.5 (name resolution)

#### `library-package`

**warning** · Name resolution · a user package marked `standard library package`

The `standard` prefix marks the normative library packages the specification publishes; a package of the model carrying it is read as library content by every consumer, which hides its findings and changes how tools load it. Write `library package` for a user library, or a plain `package`.

Specification: SysML v2 §7.3.2 and the pilot's validateLibraryPackageNotStandard

#### `library-root-name`

**warning** · Name resolution · a root namespace named like a standard library package

A top-level package of the model has the name of a standard library package (`Base`, `ScalarValues` …), so qualified names starting with it may resolve to the library rather than to the model, depending on which is imported. Rename the package, or qualify the references that are meant for the model's package.

#### `name-conflict`

**warning** · Name resolution · two members of one namespace with the same name

A namespace owns or inherits two members distinguishable by neither name nor short name: a duplicate owned member name, or an owned name that hides an inherited one without redefining it. Rename one member, or redefine the inherited one (`:>>`) where the owned member is meant to replace it. `-strict` reports the KerML-forbidden duplicate owned member names as errors.

Specification: KerML 1.0 §8.3.2.5 validateNamespaceDistinguishability

#### `not-a-vertex`

**error** · Name resolution · a transition endpoint that names no state or pseudostate

The source or target of a transition resolves to an element that is not a state usage or a pseudostate of its state machine (an action, an attribute, a definition). Name a state of the enclosing state definition or usage, or an entry, exit, choice, fork, join or history pseudostate.

Specification: SysML v2 §7.19.2 and validateTransitionUsageSuccession

#### `unresolved`

**error** · Name resolution · a name that resolves to no element visible where it is written

A qualified or simple name names no element the namespace it is written in can see: it is misspelled, declared in a namespace that is not imported, private where it is declared, or not declared at all. The message offers the nearest visible names and, for a declared but invisible one, the import that would make it visible. A member lookup that fails part way (`A::B::c`) reports the segment that did not resolve.

Specification: KerML 1.0 §8.2.3.5 (name resolution)

### Typing

| Code | Severity | Summary |
|------|----------|---------|
| [`binding-connector-type-conformance`](#binding-connector-type-conformance) | error | a value bound to a feature whose type does not admit it |
| [`bound-feature-types`](#bound-feature-types) | warning | a binding connector joining features of unrelated types |
| [`bracket-operator`](#bracket-operator) | warning | `x[i]` written as an index |
| [`cast-conformance`](#cast-conformance) | warning | a cast between unrelated types |
| [`comparison-constant`](#comparison-constant) | warning | a comparison of two values whose types never meet |
| [`condition-boolean`](#condition-boolean) | error | a condition that is not Boolean |
| [`conjugated-port-definition`](#conjugated-port-definition) | error | `~` applied to something other than a port definition, or typing something other than a port |
| [`connection-def-end-not-usage`](#connection-def-end-not-usage) | error | a connection definition end that is no usage |
| [`connector-end-index`](#connector-end-index) | error | a connector end selecting an element outside the feature's multiplicity |
| [`constructor-argument`](#constructor-argument) | error | an argument of `new X(...)` that fits no feature of `X` |
| [`constructor-expression-no-duplicate-feature-redefinition`](#constructor-expression-no-duplicate-feature-redefinition) | error | `new X(...)` binding one feature twice |
| [`event-reference-occurrence`](#event-reference-occurrence) | error | an event occurrence referencing something that is no occurrence |
| [`feature-chain-segment`](#feature-chain-segment) | error | a feature chain stepping through something that is not a feature |
| [`feature-value-count`](#feature-value-count) | error | a feature value holding a number of elements its multiplicity excludes |
| [`filter-not-boolean`](#filter-not-boolean) | error | an element filter whose condition is not Boolean |
| [`filter-not-evaluable`](#filter-not-evaluable) | error | an element filter that is not model-level evaluable |
| [`filter-not-evaluated`](#filter-not-evaluated) | warning | an element filter OpenSysML does not evaluate |
| [`flow-def-end-not-usage`](#flow-def-end-not-usage) | error | a flow definition end that is no usage |
| [`flow-def-too-many-ends`](#flow-def-too-many-ends) | error | a flow definition with more than two ends |
| [`incommensurable-binding`](#incommensurable-binding) | error | a quantity bound to a feature typed by a quantity of another dimension |
| [`incommensurable-operands`](#incommensurable-operands) | warning | an operator combining quantities whose dimensions do not fit |
| [`individual-typing`](#individual-typing) | error | an individual usage typed by none or several individual definitions |
| [`instantiation-not-type`](#instantiation-not-type) | error | `new X(...)` where `X` is not a type |
| [`interface-end-not-port`](#interface-end-not-port) | error | an interface end that is no port |
| [`invocation-ambiguous`](#invocation-ambiguous) | warning | a call whose name several behaviors answer to |
| [`invocation-argument`](#invocation-argument) | error | an argument of a call that binds to no input parameter |
| [`invocation-not-behavior`](#invocation-not-behavior) | error | a call of something that is neither a behavior nor a behavioral feature |
| [`occurrence-usage-type`](#occurrence-usage-type) | error | an occurrence, item or part typed by a non-occurrence definition |
| [`one-type`](#one-type) | error | a usage that admits one type declared with several |
| [`only-one-return-parameter`](#only-one-return-parameter) | error | a calculation or function with more than one return parameter |
| [`operator-operand-type`](#operator-operand-type) | error | an operator applied to operands of a type it is not defined for |
| [`port-owned-usage-composite`](#port-owned-usage-composite) | error | a composite usage owned by a port |
| [`quantity-unit`](#quantity-unit) | warning | a quantity whose unit is no measurement reference |
| [`relationship-end-kind`](#relationship-end-kind) | error | a connector, flow, allocation or succession end that names the wrong kind of element |
| [`relationship-target-kind`](#relationship-target-kind) | error | a specialization, subsetting, redefinition or reference whose target is of the wrong kind |
| [`sequence-index`](#sequence-index) | error | a sequence index that is no Integer, or outside the sequence |
| [`specialization-kind`](#specialization-kind) | error | a KerML classifier specializing one of another family |
| [`specialization-specific-conjugated`](#specialization-specific-conjugated) | error | a conjugated type that specializes |
| [`state-do-action`](#state-do-action) | error | a state with more than one do action |
| [`state-entry-action`](#state-entry-action) | error | a state with more than one entry action |
| [`state-exit-action`](#state-exit-action) | error | a state with more than one exit action |
| [`subsetting-feature`](#subsetting-feature) | error | a subsetting or redefinition whose target is not a feature |
| [`transition-feature-membership-guard-expression`](#transition-feature-membership-guard-expression) | error | a transition guard that is not Boolean |
| [`trigger-after-duration`](#trigger-after-duration) | error | an `after` trigger whose delay is no DurationValue |
| [`trigger-at-time-instant`](#trigger-at-time-instant) | error | an `at` trigger whose time is no TimeInstantValue |
| [`trigger-when-boolean`](#trigger-when-boolean) | error | a `when` trigger whose condition is not Boolean |
| [`unbound-parameter`](#unbound-parameter) | warning | a call leaving a parameter without default unbound |
| [`unique-feature-values`](#unique-feature-values) | error | a repeated element in the value of a unique feature |
| [`usage-reference-kind`](#usage-reference-kind) | error | a reference form (`perform`, `exhibit`, `include`, `satisfy`, `assert`) naming the wrong kind of usage |
| [`usage-typing`](#usage-typing) | error | a usage typed by something other than a definition of its kind |
| [`variant-port-composite`](#variant-port-composite) | error | a variant port that is composite |

#### `binding-connector-type-conformance`

**error** · Typing · a value bound to a feature whose type does not admit it

A feature value (`x : Integer = "a"`), a default and an initial value alike, binds the value to the feature, and a feature's values are instances of its types: here the value's type does not conform to the feature's. Write a value of the feature's type, convert it with a library function, or type the feature by what the value is.

Specification: KerML 1.0 validateBindingConnectorTypeConformance and §8.3.3.3.4 (a feature's values are instances of its types); a `*` value is a Positive (checkLiteralInfinitySpecialization)

#### `bound-feature-types`

**warning** · Typing · a binding connector joining features of unrelated types

`bind a = b` asserts the two features hold the same values, which needs types with common instances; the types written are unrelated, so the binding can hold only when both are empty. Bind features of related types, or type one by the other's type.

Specification: KerML 1.0 validateBindingConnectorTypeConformance

#### `bracket-operator`

**warning** · Typing · `x[i]` written as an index

In KerML `[` is the operator `BaseFunctions::'['`, which the kernel library leaves abstract: `x[i]` indexes nothing, and in SysML it reads a unit (`5 [m]`). Index a sequence with `x#(i)`.

Specification: KerML 1.0 validateOperatorExpressionBracket and §9.4.2 BaseFunctions

#### `cast-conformance`

**warning** · Typing · a cast between unrelated types

`x as T` selects the values of `x` that are instances of `T`; the argument's type and `T` are unrelated, neither specializing the other, so the cast selects no value. Cast to a type related to the argument's, or convert with a library function.

Specification: KerML 1.0 §8.3.4.8 (cast expressions select conforming values)

#### `comparison-constant`

**warning** · Typing · a comparison of two values whose types never meet

`==` or `!=` compares values of types with no common instance (a String with a Natural), so the result is always the same and the condition is likely a slip. Compare values of one type, or convert one side.

#### `condition-boolean`

**error** · Typing · a condition that is not Boolean

The expression stands where a Boolean is required — a constraint or requirement body, `if`, `while`, `until`, a select predicate — and its type is another: a number, a string, a part. Write a comparison or a Boolean feature, or call a predicate.

Specification: KerML 1.0 §8.3.4.7 (a constraint's result is Boolean) and §8.3.4.8 ConditionalExpression

#### `conjugated-port-definition`

**error** · Typing · `~` applied to something other than a port definition, or typing something other than a port

`~P` names the conjugated port definition of the port definition `P`, and only a port usage or a connector end may be typed by it. Apply `~` to a port definition, and type ports alone by the conjugate.

Specification: SysML v2 §7.12.2 ConjugatedPortDefinition

#### `connection-def-end-not-usage`

**error** · Typing · a connection definition end that is no usage

Every end feature of a connection, interface or allocation definition is a usage, the inherited ones included; this definition inherits a KerML end feature from a kernel association such as Links::BinaryLink or Occurrences::HappensLink. Specialize Connections::Connection or BinaryConnection instead, or declare usage ends that redefine the inherited ones.

Specification: SysML v2.1 Beta 2 §8.3.13.3 (connectionEnd : Usage redefines associationEnd); pilot validateConnectionDefinitionEndsAreUsages

#### `connector-end-index`

**error** · Typing · a connector end selecting an element outside the feature's multiplicity

`connect a#(i) to b` selects the i-th element of `a`; the index is no Integer, is 0, or lies outside the multiplicity of the feature it selects from. Index from 1 within the multiplicity, or connect the whole feature.

Specification: SysML v2 §7.13 (connector end indexing)

#### `constructor-argument`

**error** · Typing · an argument of `new X(...)` that fits no feature of `X`

The arguments of a constructor expression bind to the features of the instantiated type, positional ones in order and named ones by name; this one names no feature of the type, is one too many, or has a value the feature's type does not admit. Bind each argument to a feature of the type with a value of its type.

Specification: KerML 1.0 §8.3.4.8 (constructor expressions)

#### `constructor-expression-no-duplicate-feature-redefinition`

**error** · Typing · `new X(...)` binding one feature twice

A constructor expression binds each feature of the instantiated type at most once; a named argument repeats a feature a positional or earlier named argument already bound. Drop the repeated binding.

Specification: KerML 1.0 validateConstructorExpressionNoDuplicateFeatureRedefinition

#### `event-reference-occurrence`

**error** · Typing · an event occurrence referencing something that is no occurrence

`event occurrence e references x` names an occurrence usage; `x` resolves to an attribute, a definition or another element that is no occurrence. Reference an occurrence usage (a part, action, state or occurrence).

Specification: SysML v2 validateEventOccurrenceUsageReferent

#### `feature-chain-segment`

**error** · Typing · a feature chain stepping through something that is not a feature

Every segment of a feature chain (`a.b.c`) names a feature of what precedes it; one segment resolves to a definition, a package or another non-feature. Chain through usages only, and qualify the first segment (`A::a.b`) where a definition's member is meant.

Specification: KerML 1.0 §8.3.3.3 feature chaining

#### `feature-value-count`

**error** · Typing · a feature value holding a number of elements its multiplicity excludes

The value bound to the feature is a sequence of a known length outside the feature's multiplicity: a list bound to a single-valued feature, or one element short of a lower bound. Write a value within the multiplicity, or widen the multiplicity (`[*]`, `[0..1]`).

Specification: KerML 1.0 §8.3.3.3.5 Multiplicity

#### `filter-not-boolean`

**error** · Typing · an element filter whose condition is not Boolean

A `filter` member or an import filter (`[@M]`) is a Boolean expression over each candidate element; this one yields another type. Write a metadata test (`@M`) or a comparison that yields Boolean.

Specification: KerML 1.0 validateElementFilterMembershipIsBoolean

#### `filter-not-evaluable`

**error** · Typing · an element filter that is not model-level evaluable

A filter condition is evaluated over the model itself, so it may use metadata tests, literals, the kernel's Boolean and comparison operators and metadata feature references only; this one calls or reads something evaluation cannot reach. Restrict the condition to model-level evaluable expressions.

Specification: KerML 1.0 validateElementFilterMembershipIsModelLevelEvaluable and §8.3.4.8.7 (model-level evaluability)

#### `filter-not-evaluated`

**warning** · Typing · an element filter OpenSysML does not evaluate

The filter condition is well formed but uses a form OpenSysML's model-level evaluator does not implement yet, so every candidate is kept as if the filter were absent. Rewrite the condition with metadata tests and comparisons the evaluator covers, or expect the unfiltered members.

#### `flow-def-end-not-usage`

**error** · Typing · a flow definition end that is no usage

Every end feature of a flow definition is a usage, the inherited ones included; this definition inherits a KerML end feature from a kernel association such as Transfers::Transfer. Specialize Flows::Flow or Flows::Message instead, or declare usage ends that redefine the inherited ones.

Specification: SysML v2.1 Beta 2 §8.3.16.2 (flowEnd : Usage redefines associationEnd); pilot validateFlowDefinitionEndsAreUsages

#### `flow-def-too-many-ends`

**error** · Typing · a flow definition with more than two ends

A flow connection definition relates a source and a target: it can have at most two ends. Drop the extra ends, or declare a connection definition for an n-ary relationship.

Specification: SysML v2 validateFlowDefinitionConnectionEnds

#### `incommensurable-binding`

**error** · Typing · a quantity bound to a feature typed by a quantity of another dimension

The value is a quantity of one kind (a length, say) and the feature is typed by a quantity value type of another (a mass), so no unit conversion relates them. Write a value of the feature's quantity kind, or type the feature by the value's.

Specification: SysML v2 Quantities and Units Domain Library (QuantityValue, MeasurementReference)

#### `incommensurable-operands`

**warning** · Typing · an operator combining quantities whose dimensions do not fit

`+`, `-` and comparisons take quantities of one kind; the operands are of different kinds, or one is a point on a measurement scale (a date, a temperature in °C) for which the operation has no meaning. Convert one operand, take the difference of two scale points instead of their sum, or check the units written.

Specification: SysML v2 Quantities and Units Domain Library

#### `individual-typing`

**error** · Typing · an individual usage typed by none or several individual definitions

An individual occurrence usage is typed by exactly one individual definition, which names the individual it is a portion or snapshot of. Type the usage by one `individual` definition.

Specification: SysML v2 validateOccurrenceUsageIndividualDefinition

#### `instantiation-not-type`

**error** · Typing · `new X(...)` where `X` is not a type

An instantiation expression names the type whose instance it constructs; `X` resolves to a feature, a package or another non-type. Name a definition or classifier after `new`.

Specification: KerML 1.0 §8.3.4.8 InstantiationExpression (Must have an invoked/instantiated type)

#### `interface-end-not-port`

**error** · Typing · an interface end that is no port

The ends of an interface definition or usage are ports; this end names a part, an attribute or another usage. Connect ports, or write a `connection` where arbitrary usages are joined.

Specification: SysML v2 validateInterfaceDefinitionEnd and validateInterfaceUsageEnd

#### `invocation-ambiguous`

**warning** · Typing · a call whose name several behaviors answer to

The callee's name resolves to more than one calculation or function visible at the call, so which one the arguments bind to depends on the resolution order. Qualify the callee (`Pkg::f(x)`), or rename one of the candidates.

#### `invocation-argument`

**error** · Typing · an argument of a call that binds to no input parameter

A call's positional arguments bind to the callee's `in` parameters in order and its named arguments by name; this argument has no parameter to bind to, names one twice, or names one that is not an input. Match the arguments to the callee's parameters, listed in the message.

Specification: KerML 1.0 §8.3.4.8.8 (invocation argument binding)

#### `invocation-not-behavior`

**error** · Typing · a call of something that is neither a behavior nor a behavioral feature

`f(x)` invokes a function, a calculation, an action or a behavioral feature; `f` resolves to an attribute, a part or a definition that is not a behavior. Call a calculation, function or action, or write a feature reference where no call is meant.

Specification: KerML 1.0 §8.3.4.8 InvocationExpression

#### `occurrence-usage-type`

**error** · Typing · an occurrence, item or part typed by a non-occurrence definition

An occurrence usage (an item, part, action or state included) is typed by occurrence definitions; the type written is an attribute definition, an enumeration or another classifier that is no occurrence. Type the usage by an occurrence, item or part definition.

Specification: SysML v2 validateOccurrenceUsageType

#### `one-type`

**error** · Typing · a usage that admits one type declared with several

A calculation, case, view, viewpoint, rendering, requirement or enumeration usage is typed by exactly one definition, and this one declares more. Keep the one definition meant; where two are needed, declare a definition that specializes both and type the usage by it.

Specification: SysML v2 validateCalculationUsageType, validateCaseUsageType, validateViewUsageType, validateRequirementUsageType, validateEnumerationUsageType …

#### `only-one-return-parameter`

**error** · Typing · a calculation or function with more than one return parameter

A function, expression or calculation has one result parameter; a second `return` declares another. Keep one return, and declare the other results as `out` parameters.

Specification: KerML 1.0 validateFunctionResultParameterMembership and validateExpressionResultParameterMembership

#### `operator-operand-type`

**error** · Typing · an operator applied to operands of a type it is not defined for

`not` takes a Boolean, `and`/`or`/`xor`/`implies` Booleans, `+`, `-`, `*`, `/`, `**` and `%` numbers (or quantities), and `+` also strings; one operand has another type, or the pair has no definition in the kernel function libraries. Write operands of the type the operator takes, or call the library function meant.

Specification: KerML 1.0 §9.4 Kernel Function Library (BaseFunctions, ScalarFunctions, BooleanFunctions …)

#### `port-owned-usage-composite`

**error** · Typing · a composite usage owned by a port

The usages a port definition or port usage owns, other than nested ports, are referential: a port exposes features of the part it belongs to rather than owning them. Write `ref` before the usage, or declare it in the owning part.

Specification: SysML v2 validatePortDefinitionOwnedUsagesNotComposite and validatePortUsageNestedUsagesNotComposite

#### `quantity-unit`

**warning** · Typing · a quantity whose unit is no measurement reference

`5 [u]` makes a quantity whose unit `u` must be a measurement reference (a unit of the ISQ or SI libraries, or a feature typed by `MeasurementUnit`); `u` is something else. Write a unit such as `[m]` or name a feature typed by a measurement reference.

Specification: SysML v2 validateOperatorExpressionQuantity

#### `relationship-end-kind`

**error** · Typing · a connector, flow, allocation or succession end that names the wrong kind of element

An end of a relationship usage (`connect`, `flow`, `allocate`, `succession` …) names an element that is not of the kind the relationship relates: a definition where a usage is required, an attribute where an occurrence is. Name a usage of the required kind, nested with dot notation where it sits inside another.

Specification: SysML v2 §7.13 (connections) and §7.17 (successions) end rules

#### `relationship-target-kind`

**error** · Typing · a specialization, subsetting, redefinition or reference whose target is of the wrong kind

The element named after `:>`, `:>>`, `::>` or `references` is not of a kind the relationship admits for this declaration: a definition specializing a usage, a usage subsetting a definition, a reference to a package. Name an element of the kind the declaration relates to, or change the relationship keyword.

Specification: KerML 1.0 §8.3.3 (specialization relates types to types, subsetting features to features)

#### `sequence-index`

**error** · Typing · a sequence index that is no Integer, or outside the sequence

`x#(i)` selects the i-th element of a sequence, counting from 1; the index written is not an Integer, is 0, or exceeds the sequence's known length. Write an Integer index between 1 and the length.

Specification: KerML 1.0 §8.3.4.8 (index expressions) and §9.4.4 SequenceFunctions

#### `specialization-kind`

**error** · Typing · a KerML classifier specializing one of another family

A class specializes classes, a data type data types, a structure structures, a behavior behaviors, an association associations; the general type written belongs to another family. Specialize a classifier of the same family, or change the declaration's kind.

Specification: KerML 1.0 validateClassSpecialization, validateDataTypeSpecialization, validateStructureSpecialization, validateBehaviorSpecialization, validateAssociationRelatedTypes

#### `specialization-specific-conjugated`

**error** · Typing · a conjugated type that specializes

A conjugated type (`~T`) derives its features from the type it conjugates and cannot itself be the specific type of a specialization. Specialize the original type, or declare a type that conjugates the general one.

Specification: KerML 1.0 validateSpecializationSpecificNotConjugated

#### `state-do-action`

**error** · Typing · a state with more than one do action

A state definition or usage declares at most one `do` action. Merge the do actions into one, with parallel or sequenced steps in its body.

Specification: SysML v2 validateStateDefinitionSubactionKind and validateStateUsageSubactionKind

#### `state-entry-action`

**error** · Typing · a state with more than one entry action

A state definition or usage declares at most one `entry` action. Merge the entry actions into one, sequenced in its body.

Specification: SysML v2 validateStateDefinitionSubactionKind and validateStateUsageSubactionKind

#### `state-exit-action`

**error** · Typing · a state with more than one exit action

A state definition or usage declares at most one `exit` action. Merge the exit actions into one, sequenced in its body.

Specification: SysML v2 validateStateDefinitionSubactionKind and validateStateUsageSubactionKind

#### `subsetting-feature`

**error** · Typing · a subsetting or redefinition whose target is not a feature

Subsetting and redefinition relate features; the target written after `:>` or `:>>` is a classifier, a definition or another non-feature. Name a feature (a usage) as the target, or write a specialization (`:>`) between definitions.

Specification: KerML 1.0 §8.3.3.3 Subsetting and Redefinition

#### `transition-feature-membership-guard-expression`

**error** · Typing · a transition guard that is not Boolean

The `if` guard of a transition or succession must evaluate to a Boolean; this one yields a value of another type, or names a feature no Boolean can come from. Compare the value (`if x > 3`) or name a Boolean feature.

Specification: SysML v2 validateTransitionFeatureMembershipGuardExpression

#### `trigger-after-duration`

**error** · Typing · an `after` trigger whose delay is no DurationValue

`accept after d` waits for a duration; the delay written is a plain number or a quantity of another kind. Write it with a duration unit (`after 5 [s]`) or name a feature typed by `DurationValue`.

Specification: SysML v2 validateTriggerInvocationActionAfterArgument

#### `trigger-at-time-instant`

**error** · Typing · an `at` trigger whose time is no TimeInstantValue

`accept at t` waits for a time instant; the time written is a plain number or a quantity of another kind. Name a feature typed by `TimeInstantValue` or a clock reading such as `TimeOf(this)`.

Specification: SysML v2 validateTriggerInvocationActionAtArgument

#### `trigger-when-boolean`

**error** · Typing · a `when` trigger whose condition is not Boolean

`accept when c` waits for a condition to become true; the expression written is of another type. Compare the value (`when x > 3`) or name a feature typed by Boolean.

Specification: SysML v2 validateTriggerInvocationActionWhenArgument

#### `unbound-parameter`

**warning** · Typing · a call leaving a parameter without default unbound

The callee has an `in` parameter with no default value that the call does not bind, so the call is well formed but cannot be evaluated. Pass the argument, or give the parameter a default.

Specification: KerML 1.0 §8.3.4.8.8

#### `unique-feature-values`

**error** · Typing · a repeated element in the value of a unique feature

A feature is unique unless declared `nonunique`, and the value written repeats one element at two positions, which a unique feature cannot hold. Drop the repetition, or declare the feature `nonunique` where repetition is meant.

Specification: KerML 1.0 §7.3.4.3 (isUnique)

#### `usage-reference-kind`

**error** · Typing · a reference form (`perform`, `exhibit`, `include`, `satisfy`, `assert`) naming the wrong kind of usage

Each reference form admits one kind of referent: `perform` an action, `exhibit` a state, `include` a use case, `satisfy` a requirement, `assert` a constraint; the element named is of another kind. Name a usage of the kind the form requires, or use the form that fits the element.

Specification: SysML v2 validatePerformActionUsageReference, validateExhibitStateUsageReference, validateIncludeUseCaseUsageReference, validateSatisfyRequirementUsageReference, validateAssertConstraintUsageReference

#### `usage-typing`

**error** · Typing · a usage typed by something other than a definition of its kind

A usage is typed by definitions of its own kind: a part by part definitions, an attribute by attribute definitions, an action by action definitions, and so on; the type written is of another kind or is not a definition. Type the usage by a definition of its kind, or change the usage keyword to the kind of the type.

Specification: SysML v2 validateUsageType and the per-kind constraints (validatePartUsageType, validateAttributeUsageType, validateActionUsageType …)

#### `variant-port-composite`

**error** · Typing · a variant port that is composite

A port usage, a variant port included, is referential: it exposes rather than owns. Write `ref port` or drop the composite prefix.

Specification: SysML v2 validatePortUsageIsReference

### Constraints

| Code | Severity | Summary |
|------|----------|---------|
| [`accepter-source-not-state`](#accepter-source-not-state) | error | a transition with a trigger whose source is not a state |
| [`annotation-annotated-element-ownership`](#annotation-annotated-element-ownership) | error | an annotating element whose `about` names its own owner |
| [`assignment-chain-in-calc`](#assignment-chain-in-calc) | error | a calculation assigning a feature of another object |
| [`assignment-chain-step-not-one-object`](#assignment-chain-step-not-one-object) | error | an assignment target stepping through a feature that may hold several objects |
| [`assignment-referent`](#assignment-referent) | error | an assignment whose target is no feature |
| [`assignment-referent-read-only`](#assignment-referent-read-only) | error | an assignment to a constant or derived feature |
| [`assignment-referent-time-varying`](#assignment-referent-time-varying) | error | an assignment to a feature that is not time-varying |
| [`association-end-types`](#association-end-types) | error | an association end with no type or several |
| [`binding-binary`](#binding-binary) | error | a binding connector with more or fewer than two ends |
| [`chaining-feature-not-one`](#chaining-feature-not-one) | error | a feature chained through one feature only |
| [`chaining-features-not-self`](#chaining-features-not-self) | error | a feature chained through itself |
| [`classifier-default-supertype`](#classifier-default-supertype) | error | a classifier that does not specialize its kind's base classifier |
| [`connector-end-referent`](#connector-end-referent) | error | a connector end naming no feature |
| [`connector-ends`](#connector-ends) | error | a connector whose ends do not fit the link it specializes |
| [`connector-related-features`](#connector-related-features) | error | a connector or flow with fewer than two related features |
| [`connector-type-featuring`](#connector-type-featuring) | error | a connector end that is not accessible from the connector's featuring type |
| [`constant-feature-not-variable`](#constant-feature-not-variable) | error | `const` on a feature that is not variable |
| [`control-node-incoming-multiplicity`](#control-node-incoming-multiplicity) | error | a succession into a control node with a target multiplicity other than 1 |
| [`control-node-outgoing-multiplicity`](#control-node-outgoing-multiplicity) | error | a succession out of a control node with a source multiplicity other than 1 |
| [`control-node-owner`](#control-node-owner) | error | a control node outside an action |
| [`cross-feature-specialization`](#cross-feature-specialization) | error | a cross feature that does not specialize the redefined ends' cross features |
| [`cross-feature-type`](#cross-feature-type) | error | a cross feature typed differently from its end feature |
| [`cross-subsetting-at-most-one`](#cross-subsetting-at-most-one) | error | an end feature with more than one cross subsetting |
| [`cross-subsetting-chain`](#cross-subsetting-chain) | error | a cross subsetting that does not chain through an opposite end |
| [`cross-subsetting-crossing-feature`](#cross-subsetting-crossing-feature) | error | a cross subsetting owned by something other than an end feature |
| [`decision-incoming-successions`](#decision-incoming-successions) | error | a decision node with more than one incoming succession |
| [`decision-outgoing-multiplicity`](#decision-outgoing-multiplicity) | error | a succession out of a decision node with a target multiplicity other than 0..1 |
| [`declared-cross-feature`](#declared-cross-feature) | error | an explicitly declared cross feature that is not the end's |
| [`diagram-layout-canvas`](#diagram-layout-canvas) | error | a Canvas or Picture annotation outside the view it belongs to |
| [`diagram-layout-duplicate`](#diagram-layout-duplicate) | warning | a layout annotation stated twice about one element |
| [`diagram-layout-unplaced`](#diagram-layout-unplaced) | warning | a layout annotation on an element no rendering draws |
| [`diagram-layout-value`](#diagram-layout-value) | error | a layout annotation with an ill-formed value |
| [`end-feature-direction`](#end-feature-direction) | error | an end feature with a direction |
| [`end-feature-kind`](#end-feature-kind) | error | an end feature that is derived, abstract, composite or a portion |
| [`end-feature-multiplicity`](#end-feature-multiplicity) | warning | an end feature whose multiplicity is not 1 |
| [`endpoint-not-a-node`](#endpoint-not-a-node) | error | a succession endpoint in an action that names no action node |
| [`endpoint-not-of-machine`](#endpoint-not-of-machine) | error | a transition endpoint naming a vertex of another state machine |
| [`entry-transition-shape`](#entry-transition-shape) | error | an entry transition with a trigger or an effect |
| [`entry-transition-target`](#entry-transition-target) | error | an entry transition targeting something the body cannot start in |
| [`feature-chain-conformance`](#feature-chain-conformance) | error | a chained feature that is no feature of what precedes it |
| [`feature-has-type`](#feature-has-type) | error | a feature with no type |
| [`feature-portion-not-variable`](#feature-portion-not-variable) | error | a portion feature that is variable |
| [`feature-reference-featuring-types`](#feature-reference-featuring-types) | error | a feature referenced where it is not accessible |
| [`feature-reference-referent`](#feature-reference-referent) | error | a feature reference expression naming no feature |
| [`feature-value-overriding`](#feature-value-overriding) | error | a redefinition giving a value to a feature bound with `=` |
| [`flow-end-subsetting`](#flow-end-subsetting) | error | a flow end that does not name the feature the payload flows from or to |
| [`fork-incoming-successions`](#fork-incoming-successions) | error | a fork node with more than one incoming succession |
| [`identity-conflicting-ids`](#identity-conflicting-ids) | error | one element annotated with several different ids |
| [`identity-conflicting-projects`](#identity-conflicting-projects) | error | one element placed in several projects |
| [`identity-duplicate-id`](#identity-duplicate-id) | error | two elements with the same effective element id |
| [`identity-id-shape`](#identity-id-shape) | error | an element id that is empty or holds a character outside [a-zA-Z0-9_-] |
| [`identity-unscoped-id`](#identity-unscoped-id) | error | an element id with no enclosing project to resolve against |
| [`import-top-level-visibility`](#import-top-level-visibility) | error | a top-level import that is not private |
| [`initial-value-not-variable`](#initial-value-not-variable) | error | an initial value (`:=`) on a feature that is not variable |
| [`join-outgoing-successions`](#join-outgoing-successions) | error | a join node with more than one outgoing succession |
| [`merge-incoming-multiplicity`](#merge-incoming-multiplicity) | error | a succession into a merge node with a source multiplicity other than 0..1 |
| [`merge-outgoing-successions`](#merge-outgoing-successions) | error | a merge node with more than one outgoing succession |
| [`metadata-annotated-element`](#metadata-annotated-element) | error | metadata annotating an element its metaclass does not admit |
| [`metadata-body-feature`](#metadata-body-feature) | error | a metadata body member redefining no feature of the metaclass |
| [`metadata-concrete-type`](#metadata-concrete-type) | error | a metadata usage typed by an abstract metaclass |
| [`metadata-metaclass`](#metadata-metaclass) | error | a metadata usage typed by none or several metaclasses |
| [`metadata-owning-type-feature`](#metadata-owning-type-feature) | error | a metadata body feature that is not owned by the metaclass |
| [`metadata-value-not-evaluable`](#metadata-value-not-evaluable) | error | a metadata feature value that is not model-level evaluable |
| [`multiplicity-bound-natural`](#multiplicity-bound-natural) | error | a multiplicity bound that is no Natural |
| [`multiplicity-featuring-type`](#multiplicity-featuring-type) | error | a multiplicity featured by a type other than its feature's |
| [`multiplicity-range`](#multiplicity-range) | error | a multiplicity whose lower bound exceeds its upper bound |
| [`no-outgoing-transition`](#no-outgoing-transition) | error | a routing pseudostate no transition leaves |
| [`no-transition-source`](#no-transition-source) | error | a transition written without a source that no member precedes |
| [`only-one-multiplicity`](#only-one-multiplicity) | error | a type with more than one owned multiplicity |
| [`only-one-objective`](#only-one-objective) | error | a case with more than one objective |
| [`only-one-subject`](#only-one-subject) | error | a requirement or case with more than one subject |
| [`only-one-view-rendering`](#only-one-view-rendering) | error | a view with more than one rendering |
| [`parallel-state-transition`](#parallel-state-transition) | error | a transition or succession between the direct substates of a parallel state |
| [`part-usage-part-definition`](#part-usage-part-definition) | error | a part usage typed by no part definition |
| [`port-conjugation`](#port-conjugation) | warning | an interface joining two ports whose directed features are not conjugate |
| [`portion-owner`](#portion-owner) | error | a portion (snapshot or timeslice) owned by a non-occurrence |
| [`redefinition-direction-conformance`](#redefinition-direction-conformance) | error | a redefining feature whose direction does not fit the redefined one |
| [`redefinition-multiplicity`](#redefinition-multiplicity) | warning | a redefining feature with a smaller lower bound than the redefined one |
| [`redefinition-no-derived-name`](#redefinition-no-derived-name) | warning | an unnamed redefinition given a value in a context that names it differently |
| [`redefinition-no-inherited`](#redefinition-no-inherited) | error | `:>>` naming a feature the owner does not inherit |
| [`redefinition-not-end`](#redefinition-not-end) | error | a non-end feature of an association or connector redefining an end feature |
| [`redefinition-package-level`](#redefinition-package-level) | error | a redefinition of a package-level feature |
| [`redefinition-same-featuring`](#redefinition-same-featuring) | error | a redefinition between features of one featuring type |
| [`redefinition-through-reference`](#redefinition-through-reference) | error | a chain redefinition stepping through a feature that owns no object |
| [`redefinition-type-mismatch`](#redefinition-type-mismatch) | warning | a redefining feature typed by a type unrelated to the redefined feature's |
| [`reference-subsetting-at-most-one`](#reference-subsetting-at-most-one) | error | a feature with more than one reference subsetting |
| [`related-elements`](#related-elements) | error | an association with fewer than two related types |
| [`result-expression-at-most-one`](#result-expression-at-most-one) | error | a function or expression with more than one result expression |
| [`return-parameter-owner`](#return-parameter-owner) | error | `return` outside a function, expression or calculation |
| [`send-payload-missing`](#send-payload-missing) | error | a send as a state subaction or transition effect with no payload |
| [`send-receiver-not-occurrence`](#send-receiver-not-occurrence) | warning | a send whose receiver is no occurrence |
| [`send-sender-not-occurrence`](#send-sender-not-occurrence) | warning | a send whose sender is no occurrence |
| [`send-to-port`](#send-to-port) | warning | `send ... to <port>`, where `via` is meant |
| [`specialization-cycle`](#specialization-cycle) | error | a type that specializes itself, directly or through others |
| [`subject-parameter-position`](#subject-parameter-position) | error | a subject that is not the first parameter |
| [`subsetting-constancy-conformance`](#subsetting-constancy-conformance) | error | a variable feature subsetting or redefining a constant one |
| [`subsetting-featuring-types`](#subsetting-featuring-types) | error | a subsetted feature that is not accessible from the subsetting one |
| [`subsetting-multiplicity`](#subsetting-multiplicity) | warning | a subsetting or redefining feature with a larger upper bound than the subsetted one |
| [`subsetting-uniqueness-conformance`](#subsetting-uniqueness-conformance) | error | a nonunique feature subsetting or redefining a unique one |
| [`succession-order-cycle`](#succession-order-cycle) | error | successions that order performances in a cycle |
| [`succession-orders-nothing`](#succession-orders-nothing) | warning | a succession whose end is no performance of the run |
| [`transition-source-not-vertex`](#transition-source-not-vertex) | error | a transition written without a source whose preceding member is no vertex |
| [`type-conjugators`](#type-conjugators) | error | a type with more than one conjugator |
| [`type-differencing-not-one`](#type-differencing-not-one) | error | `differences` with one type only |
| [`type-differencing-self`](#type-differencing-self) | error | a type differencing itself |
| [`type-intersecting-not-one`](#type-intersecting-not-one) | error | `intersects` with one type only |
| [`type-intersecting-self`](#type-intersecting-self) | error | a type intersecting itself |
| [`type-unioning-not-one`](#type-unioning-not-one) | error | `unions` with one type only |
| [`type-unioning-self`](#type-unioning-self) | error | a type unioning itself |
| [`variable-feature-owner`](#variable-feature-owner) | error | a variable (`var`) feature owned by a non-occurrence type |
| [`variant-kind`](#variant-kind) | error | a variant whose kind is inconsistent with its owning variation |
| [`variant-outside-variation`](#variant-outside-variation) | error | a variant that is not an owned member of a variation |
| [`variation-member-not-variant`](#variation-member-not-variant) | error | a variation owning a usage that is no variant |
| [`variation-specialization`](#variation-specialization) | error | a variation specializing another variation |
| [`verification-owning-type`](#verification-owning-type) | error | `verify` outside the objective of a verification case |
| [`view-satisfy-viewpoint`](#view-satisfy-viewpoint) | error | `satisfy` in a view body naming no viewpoint |

#### `accepter-source-not-state`

**error** · Constraints · a transition with a trigger whose source is not a state

A transition's accepter waits in its source for the trigger, which a state can do and a pseudostate cannot. Move the trigger to a transition out of a state, or drop it from the pseudostate's transition.

Specification: SysML v2 validateTransitionUsageTriggerActions

#### `annotation-annotated-element-ownership`

**error** · Constraints · an annotating element whose `about` names its own owner

An annotation owned by its annotated element must own its annotating element: a comment, documentation or metadata usage whose `about` names the element its body sits in annotates that element twice over, through ownership and through the clause. Drop the `about` clause, or move the annotating element out of the body of what it annotates.

Specification: KerML 1.0 validateAnnotationAnnotatedElementOwnership

#### `assignment-chain-in-calc`

**error** · Constraints · a calculation assigning a feature of another object

A calculation computes a result and has no side effects; an assignment whose target chains into another object (`assign a.x := v`) would write that object. Move the assignment into an action, or compute the value as the calculation's result.

Specification: KerML 1.0 §8.4.4.9 (functions have no side effects)

#### `assignment-chain-step-not-one-object`

**error** · Constraints · an assignment target stepping through a feature that may hold several objects

`assign a.x := v` writes `x` of the one object `a` holds; a feature with multiplicity above 1 in the chain leaves it unclear which object is written. Index the step (`a#(1).x`) or assign within an action performed per element.

Specification: SysML v2 §7.17 AssignmentActionUsage

#### `assignment-referent`

**error** · Constraints · an assignment whose target is no feature

`assign x := v` writes a feature; `x` resolves to a definition, a package or another non-feature. Assign to a usage.

Specification: SysML v2 validateAssignmentActionUsageReferent

#### `assignment-referent-read-only`

**error** · Constraints · an assignment to a constant or derived feature

The assigned feature is `const`, `derived` or bound with `=`, so a run cannot write it. Assign to a variable feature, or declare the feature `var`.

Specification: SysML v2 validateAssignmentActionUsageReferentIsTimeVarying

#### `assignment-referent-time-varying`

**error** · Constraints · an assignment to a feature that is not time-varying

The target of an assignment is a time-varying feature: a `var` feature, or a feature of an occurrence that can change over its lifetime. Declare the feature `var`.

Specification: SysML v2 validateAssignmentActionUsageReferentIsTimeVarying

#### `association-end-types`

**error** · Constraints · an association end with no type or several

Each end of an association is typed by exactly one type, which is what the end relates. Give the end one type.

Specification: KerML 1.0 validateAssociationEndTypes

#### `binding-binary`

**error** · Constraints · a binding connector with more or fewer than two ends

A binding connector asserts two features hold the same values, so it has exactly two ends. Bind two features; chain bindings for more.

Specification: KerML 1.0 validateBindingConnectorIsBinary

#### `chaining-feature-not-one`

**error** · Constraints · a feature chained through one feature only

A feature chain (`chains a.b`) names at least two chaining features; a chain of one is the feature itself. Chain two or more features, or subset the one feature.

Specification: KerML 1.0 validateFeatureChainingFeatureNotOne

#### `chaining-features-not-self`

**error** · Constraints · a feature chained through itself

A feature cannot appear in its own feature chain. Chain through other features.

Specification: KerML 1.0 validateFeatureChainingFeaturesNotSelf

#### `classifier-default-supertype`

**error** · Constraints · a classifier that does not specialize its kind's base classifier

Every classifier specializes, directly or indirectly, the base classifier of its kind (`Base::Anything`, `Occurrences::Occurrence`, `Objects::Object` …); this one declares specializations that leave the base out. Specialize a classifier of the right kind, or drop the specialization that leads elsewhere.

Specification: KerML 1.0 validateClassifierDefaultSupertype

#### `connector-end-referent`

**error** · Constraints · a connector end naming no feature

Each end of a connector references a feature accessible where the connector is written; this end resolves to a definition, a package or another non-feature. Name a usage, nested with dot notation where it sits inside another.

Specification: KerML 1.0 §8.3.4.4 (connector ends reference features)

#### `connector-ends`

**error** · Constraints · a connector whose ends do not fit the link it specializes

A connector that specializes a binary link cannot declare more than two ends, and each of its ends must redefine an end of the link; this one declares too many ends or an end that redefines none. Drop the extra ends or specialize an n-ary link, and redefine the link's ends.

Specification: KerML 1.0 validateConnectorBinarySpecialization and validateAssociationBinarySpecialization

#### `connector-related-features`

**error** · Constraints · a connector or flow with fewer than two related features

A connector relates at least two features; this one names fewer, or names the same feature twice. Name two features to connect.

Specification: KerML 1.0 validateConnectorRelatedFeatures

#### `connector-type-featuring`

**error** · Constraints · a connector end that is not accessible from the connector's featuring type

Every related feature of a connector must be accessible from the type that features the connector: a feature of a sibling, reached without dot notation, is not. Write the end as a chain from a feature of the connector's owner (`a.x`), or move the connector to where both ends are visible.

Specification: KerML 1.0 validateConnectorTypeFeaturing

#### `constant-feature-not-variable`

**error** · Constraints · `const` on a feature that is not variable

`const` fixes a variable feature's value for its lifetime; a feature that is not `var` is already constant. Drop `const`, or declare the feature `var const`.

Specification: KerML 1.0 validateFeatureConstantIsVariable

#### `control-node-incoming-multiplicity`

**error** · Constraints · a succession into a control node with a target multiplicity other than 1

Each succession into a control node reaches exactly one occurrence of it. Drop the multiplicity on the target end, or write `[1]`.

Specification: SysML v2 validateControlNodeIncomingSuccessions

#### `control-node-outgoing-multiplicity`

**error** · Constraints · a succession out of a control node with a source multiplicity other than 1

Each succession out of a control node leaves exactly one occurrence of it. Drop the multiplicity on the source end, or write `[1]`.

Specification: SysML v2 validateControlNodeOutgoingSuccessions

#### `control-node-owner`

**error** · Constraints · a control node outside an action

A fork, join, merge or decision node is a member of an action definition or usage. Declare it in the action whose flow it controls.

Specification: SysML v2 validateControlNodeOwningType

#### `cross-feature-specialization`

**error** · Constraints · a cross feature that does not specialize the redefined ends' cross features

When an end redefines another end, its cross feature specializes the redefined end's cross feature. Subset the redefined cross feature, or drop the redefinition.

Specification: KerML 1.0 validateFeatureCrossFeatureSpecialization

#### `cross-feature-type`

**error** · Constraints · a cross feature typed differently from its end feature

The cross feature of an end has the same type as the end feature it belongs to. Type both alike.

Specification: KerML 1.0 validateFeatureCrossFeatureType

#### `cross-subsetting-at-most-one`

**error** · Constraints · an end feature with more than one cross subsetting

An end feature owns at most one cross subsetting. Keep one `crosses`.

Specification: KerML 1.0 validateFeatureOwnedCrossSubsetting

#### `cross-subsetting-chain`

**error** · Constraints · a cross subsetting that does not chain through an opposite end

The crossed feature is a chain whose first feature is another end of the same type (`crosses other.x`), reaching what that end relates. Chain through an opposite end feature.

Specification: KerML 1.0 validateCrossSubsettingCrossedFeature

#### `cross-subsetting-crossing-feature`

**error** · Constraints · a cross subsetting owned by something other than an end feature

A cross subsetting (`crosses`) is owned by an end feature of a type with two or more ends: it relates that end to the opposite end's feature. Write `crosses` on an end feature of an association or connector.

Specification: KerML 1.0 validateCrossSubsettingCrossingFeature

#### `decision-incoming-successions`

**error** · Constraints · a decision node with more than one incoming succession

A decision chooses one of several outgoing flows for one incoming flow, so exactly one succession enters it. Merge the incoming flows before the decision.

Specification: SysML v2 validateDecisionNodeIncomingSuccessions

#### `decision-outgoing-multiplicity`

**error** · Constraints · a succession out of a decision node with a target multiplicity other than 0..1

A decision takes at most one of its outgoing flows, so each outgoing succession has a target multiplicity of `[0..1]`. Drop the target multiplicity, or write `[0..1]`.

Specification: SysML v2 validateDecisionNodeOutgoingSuccessions

#### `declared-cross-feature`

**error** · Constraints · an explicitly declared cross feature that is not the end's

A feature declared as the cross feature of an end must be the one the end's cross subsetting derives. Let the cross feature be derived, or declare the one `crosses` names.

Specification: KerML 1.0 validateFeatureCrossFeature

#### `diagram-layout-canvas`

**error** · Constraints · a Canvas or Picture annotation outside the view it belongs to

A `Canvas` sizes the view whose body it sits in, and a `Picture` is drawn on the surface of the view it annotates; this one annotates something that is no view, or sizes a view from outside its body. Move the annotation into the view's body.

Specification: OpenSysML diagram layout library

#### `diagram-layout-duplicate`

**warning** · Constraints · a layout annotation stated twice about one element

Two annotations of one kind place the same element on the same view; the first stated applies. Drop the second.

Specification: OpenSysML diagram layout library

#### `diagram-layout-unplaced`

**warning** · Constraints · a layout annotation on an element no rendering draws

A `Position`, `Size` or `Route` annotation places an element on a view, but no rendering of the view draws that element in the role the annotation assumes, so the layout applies to nothing. Annotate an element the rendering draws, or expose it in the view.

Specification: OpenSysML diagram layout library

#### `diagram-layout-value`

**error** · Constraints · a layout annotation with an ill-formed value

The coordinates, size or route points of a layout annotation are not of the form the layout library declares (numbers in the view's units, a route of at least two points). Write the value the message names.

Specification: OpenSysML diagram layout library

#### `end-feature-direction`

**error** · Constraints · an end feature with a direction

An end feature has no direction: `in`, `out` and `inout` are for parameters. Drop the direction.

Specification: KerML 1.0 validateFeatureEndNoDirection

#### `end-feature-kind`

**error** · Constraints · an end feature that is derived, abstract, composite or a portion

An end feature cannot be `derived`, `abstract`, `composite` or `portion`: it relates, it does not own. Drop the modifier.

Specification: KerML 1.0 validateFeatureEndNotDerivedAbstractCompositeOrPortion

#### `end-feature-multiplicity`

**warning** · Constraints · an end feature whose multiplicity is not 1

An end relates exactly one thing per link, so its own multiplicity is 1; the range written says otherwise (the multiplicity of the *other* ends is what `[n]` on an end usually means). Write `[1]` or take the multiplicity from a feature the end subsets or redefines.

Specification: KerML 1.0 validateFeatureEndFeatureMultiplicity

#### `endpoint-not-a-node`

**error** · Constraints · a succession endpoint in an action that names no action node

The source or target of a succession (`first a then b`) in an action body names an action node: an action usage, a control node, an accept or send, or `start`/`done`; this one resolves to an attribute, a definition or another element. Name an action node of the enclosing action.

Specification: SysML v2 §7.17 (successions between action nodes)

#### `endpoint-not-of-machine`

**error** · Constraints · a transition endpoint naming a vertex of another state machine

A transition's source and target are states or pseudostates of the state machine the transition belongs to; this endpoint names a vertex another state definition or usage owns. Name a vertex of the enclosing state, or move the transition.

Specification: UML 2.5.1 §14.2.3.9 (transitions relate vertices of one region), SysML v2 §7.19

#### `entry-transition-shape`

**error** · Constraints · an entry transition with a trigger or an effect

A transition out of the entry action (`entry; then s;`) chooses the starting state by its guard alone: it accepts no trigger and performs no effect. Drop the trigger and effect, or start in a state whose transitions carry them.

Specification: SysML v2 §7.18.3 EntryTransitionMember

#### `entry-transition-target`

**error** · Constraints · an entry transition targeting something the body cannot start in

A transition out of the entry action targets a state of the body; this one targets a pseudostate or an action the state machine cannot start in. Target a substate.

Specification: SysML v2 §7.18.3 EntryTransitionMember

#### `feature-chain-conformance`

**error** · Constraints · a chained feature that is no feature of what precedes it

Each feature of a chain after the first is a feature of the type of the one before it; this segment names something the preceding feature's type does not feature. Chain through members of the preceding feature's type.

Specification: KerML 1.0 validateFeatureChainingFeatureConformance

#### `feature-has-type`

**error** · Constraints · a feature with no type

Every feature has at least one type, declared or implied through what it subsets or redefines; this one has none. Type the feature, or subset a typed one.

Specification: KerML 1.0 validateFeatureHasType

#### `feature-portion-not-variable`

**error** · Constraints · a portion feature that is variable

A portion of an occurrence cannot be `var`: a portion is fixed for the occurrence it is part of. Drop `var` or `portion`.

Specification: KerML 1.0 validateFeaturePortionNotVariable

#### `feature-reference-featuring-types`

**error** · Constraints · a feature referenced where it is not accessible

A feature named in an expression must be accessible from the type the expression is featured by: a feature of a sibling part, or of the outer scope of a nested definition, is not reachable by its bare name. Write a feature chain from a feature of the owner (`sibling.x`), or declare the feature where the expression is.

Specification: KerML 1.0 §8.3.4.8 (Must be an accessible feature)

#### `feature-reference-referent`

**error** · Constraints · a feature reference expression naming no feature

A name used as a value in an expression references a feature; this one resolves to a definition, a package or another non-feature. Name a usage, or instantiate the type (`new T()`) where an instance is meant.

Specification: KerML 1.0 validateFeatureReferenceExpressionReferentIsFeature

#### `feature-value-overriding`

**error** · Constraints · a redefinition giving a value to a feature bound with `=`

A feature bound with `=` is fixed; a feature that redefines it, directly or through further redefinitions, cannot give it another value. Only `default =` values may be overridden. Write `default =` on the redefined feature, or drop the value on the redefining one.

Specification: KerML 1.0 §8.3.4.10.2 validateFeatureValueOverriding

#### `flow-end-subsetting`

**error** · Constraints · a flow end that does not name the feature the payload flows from or to

Each end of a flow names an item or port feature the payload leaves or reaches, with dot notation for a feature of a part (`flow from a.out to b.in`); this end names a part itself, or a feature not reachable from the flow's context. Name the flowing feature with dot notation.

Specification: SysML v2 validateFlowEndSubsetting and validateFlowEndImplicitSubsetting

#### `fork-incoming-successions`

**error** · Constraints · a fork node with more than one incoming succession

A fork splits one flow into several, so exactly one succession enters it. Join the incoming flows first, or add a merge before the fork.

Specification: SysML v2 validateForkNodeIncomingSuccessions

#### `identity-conflicting-ids`

**error** · Constraints · one element annotated with several different ids

Several `ElementId` annotations about one element disagree. Keep one id.

Specification: SysML v2 API and Services (element identity)

#### `identity-conflicting-projects`

**error** · Constraints · one element placed in several projects

Several `ProjectRef` annotations about one element name different projects. Keep one project reference.

Specification: SysML v2 API and Services (projects)

#### `identity-duplicate-id`

**error** · Constraints · two elements with the same effective element id

An `ElementId` annotation gives an element the identity it keeps across interchange; two elements here resolve to one id, so an API or XMI reader cannot tell them apart. Give each element its own id.

Specification: SysML v2 API and Services (element identity)

#### `identity-id-shape`

**error** · Constraints · an element id that is empty or holds a character outside [a-zA-Z0-9_-]

An element id is a non-empty string of letters, digits, `_` and `-`, so it travels through URLs, XMI and the API unchanged. Write an id of that shape.

Specification: SysML v2 API and Services (element identity)

#### `identity-unscoped-id`

**error** · Constraints · an element id with no enclosing project to resolve against

An `ElementId` is scoped by the project its element belongs to; this element lies under no `ProjectRef`. Annotate an enclosing namespace with `ProjectRef`.

Specification: SysML v2 API and Services (projects)

#### `import-top-level-visibility`

**error** · Constraints · a top-level import that is not private

An import owned by the root namespace of a file is private: there is no namespace for it to re-export into. Write `private import`, or move the import into a package.

Specification: KerML 1.0 validateImportTopLevelVisibility

#### `initial-value-not-variable`

**error** · Constraints · an initial value (`:=`) on a feature that is not variable

An initial value is the value a variable feature starts with; a feature that is not `var` has one value for its lifetime. Write `=` for a fixed value, or declare the feature `var`.

Specification: KerML 1.0 validateFeatureValueIsInitial

#### `join-outgoing-successions`

**error** · Constraints · a join node with more than one outgoing succession

A join waits for several flows and continues as one, so exactly one succession leaves it. Fork after the join where several flows are meant.

Specification: SysML v2 validateJoinNodeOutgoingSuccessions

#### `merge-incoming-multiplicity`

**error** · Constraints · a succession into a merge node with a source multiplicity other than 0..1

A merge continues when any one of its incoming flows arrives, so each incoming succession has a source multiplicity of `[0..1]`. Drop the source multiplicity, or write `[0..1]`.

Specification: SysML v2 validateMergeNodeIncomingSuccessions

#### `merge-outgoing-successions`

**error** · Constraints · a merge node with more than one outgoing succession

A merge continues as one flow, so exactly one succession leaves it. Fork after the merge where several flows are meant.

Specification: SysML v2 validateMergeNodeOutgoingSuccessions

#### `metadata-annotated-element`

**error** · Constraints · metadata annotating an element its metaclass does not admit

A metadata definition may restrict, through its `annotatedElement` feature, what kind of element it annotates; this usage is about an element of another kind. Annotate an element the metaclass admits, or widen the definition's `annotatedElement`.

Specification: KerML 1.0 validateMetadataFeatureAnnotatedElement

#### `metadata-body-feature`

**error** · Constraints · a metadata body member redefining no feature of the metaclass

Each feature in the body of a metadata usage redefines a feature of its metadata definition, giving it a value. Redefine (`:>>`) a feature the metaclass declares.

Specification: SysML v2 validateMetadataUsageBodyFeature and KerML 1.0 validateMetadataFeatureBody

#### `metadata-concrete-type`

**error** · Constraints · a metadata usage typed by an abstract metaclass

A metadata usage is typed by a concrete metadata definition: an abstract one has no instances to annotate with. Type it by a concrete specialization.

Specification: KerML 1.0 validateMetadataFeatureMetaclassNotAbstract

#### `metadata-metaclass`

**error** · Constraints · a metadata usage typed by none or several metaclasses

A metadata usage (`@M` or `metadata m : M`) is typed by exactly one metadata definition or metaclass. Type it by one.

Specification: KerML 1.0 validateMetadataFeatureMetaclass

#### `metadata-owning-type-feature`

**error** · Constraints · a metadata body feature that is not owned by the metaclass

A feature in a metadata body redefines a feature the metadata definition itself owns or inherits; this one names a feature of another type. Redefine a feature of the metaclass.

Specification: KerML 1.0 validateMetadataFeatureBody

#### `metadata-value-not-evaluable`

**error** · Constraints · a metadata feature value that is not model-level evaluable

The value of a feature in a metadata body is evaluated over the model itself, so it is a literal, a metadata feature reference or an operator over those; this one calls or reads something evaluation cannot reach. Write a model-level evaluable value.

Specification: KerML 1.0 validateMetadataFeatureBody and §8.3.4.8.7 (model-level evaluability)

#### `multiplicity-bound-natural`

**error** · Constraints · a multiplicity bound that is no Natural

The bounds of a multiplicity range are Natural numbers (or `*` for the upper bound); this one is negative, a Real, or a feature of another type. Write a Natural literal or name a feature typed by Natural.

Specification: KerML 1.0 validateMultiplicityRangeBoundResultTypes

#### `multiplicity-featuring-type`

**error** · Constraints · a multiplicity featured by a type other than its feature's

A multiplicity declared as a separate feature must have the same featuring types as the feature it bounds. Declare the multiplicity beside its feature, or bound the feature inline.

Specification: KerML 1.0 validateFeatureMultiplicityDomain

#### `multiplicity-range`

**error** · Constraints · a multiplicity whose lower bound exceeds its upper bound

`[lo..hi]` requires lo ≤ hi; this range admits no count. Swap or correct the bounds.

Specification: KerML 1.0 §8.3.3.3.5 MultiplicityRange

#### `no-outgoing-transition`

**error** · Constraints · a routing pseudostate no transition leaves

A choice, fork, join or junction pseudostate routes a transition on; one no transition leaves terminates every transition reaching it nowhere. Add an outgoing transition, or remove the pseudostate.

Specification: UML 2.5.1 §15.7.18 (pseudostates), SysML v2 §7.19

#### `no-transition-source`

**error** · Constraints · a transition written without a source that no member precedes

`then <target>;` takes the member declared before it in the body as its source; this one is the first member, so nothing precedes it. Write the source (`transition first s then t;`), or declare the source state before it.

Specification: SysML v2 §7.18.3 TargetTransitionUsage

#### `only-one-multiplicity`

**error** · Constraints · a type with more than one owned multiplicity

A type owns at most one multiplicity. Keep one `[..]`.

Specification: KerML 1.0 validateTypeOwnedMultiplicity

#### `only-one-objective`

**error** · Constraints · a case with more than one objective

A case definition or usage has one objective. Keep one `objective`, merging the requirements into it.

Specification: SysML v2 validateCaseDefinitionOnlyOneObjective and validateCaseUsageOnlyOneObjective

#### `only-one-subject`

**error** · Constraints · a requirement or case with more than one subject

A requirement or case definition or usage has one subject, owned or inherited. Keep one `subject`, redefining an inherited one where its type is to change.

Specification: SysML v2 validateRequirementDefinitionOnlyOneSubject, validateRequirementUsageOnlyOneSubject, validateCaseDefinitionOnlyOneSubject, validateCaseUsageOnlyOneSubject

#### `only-one-view-rendering`

**error** · Constraints · a view with more than one rendering

A view definition or usage has at most one view rendering. Keep one `render`.

Specification: SysML v2 validateViewDefinitionOnlyOneViewRendering and validateViewUsageOnlyOneRendering

#### `parallel-state-transition`

**error** · Constraints · a transition or succession between the direct substates of a parallel state

The direct substates of a `parallel` state run concurrently, so no transition or succession orders them. Drop the transition, or nest the ordered states in one region (a non-parallel substate).

Specification: SysML v2 validateStateDefinitionParallelSubactions and validateStateUsageParallelSubactions

#### `part-usage-part-definition`

**error** · Constraints · a part usage typed by no part definition

A part is typed by at least one part definition, directly or through what it subsets; an item definition alone does not make a part. Type the part by a part definition.

Specification: SysML v2 validatePartUsagePartDefinition

#### `port-conjugation`

**warning** · Constraints · an interface joining two ports whose directed features are not conjugate

The two ends of an interface are typed by ports whose `in` and `out` features point the same way, so what one end sends the other cannot receive. Type one end by the conjugate port (`~P`).

Specification: SysML v2 §7.12.2 (port conjugation)

#### `portion-owner`

**error** · Constraints · a portion (snapshot or timeslice) owned by a non-occurrence

A `snapshot` or `timeslice` is a portion of the occurrence that owns it. Declare it in an occurrence definition or usage.

Specification: SysML v2 validateOccurrenceUsageIsPortion

#### `redefinition-direction-conformance`

**error** · Constraints · a redefining feature whose direction does not fit the redefined one

A redefining feature keeps a direction compatible with the redefined feature's: `in` redefines `in` or `inout`, `out` redefines `out` or `inout`, an undirected feature redefines an undirected one. Match the direction.

Specification: KerML 1.0 validateRedefinitionDirectionConformance

#### `redefinition-multiplicity`

**warning** · Constraints · a redefining feature with a smaller lower bound than the redefined one

A redefinition narrows the redefined feature, so its multiplicity lower bound should not fall below the redefined feature's. Raise the lower bound, or lower the redefined feature's.

Specification: KerML 1.0 validateRedefinitionMultiplicityConformance

#### `redefinition-no-derived-name`

**warning** · Constraints · an unnamed redefinition given a value in a context that names it differently

A member written `:>> x = v` takes the name of what it redefines; here the redefined feature's name differs from what the owner's other members expect, so the value lands on a member the model names otherwise. Name the member, or redefine the feature the name was meant for.

#### `redefinition-no-inherited`

**error** · Constraints · `:>>` naming a feature the owner does not inherit

A redefinition replaces a feature the redefining feature's owner inherits; the feature named is a member of a type the owner does not specialize. Specialize the type that owns the redefined feature, or subset the feature instead.

Specification: KerML 1.0 §8.3.3.3.6 Redefinition

#### `redefinition-not-end`

**error** · Constraints · a non-end feature of an association or connector redefining an end feature

A feature of an association or a connector that redefines an end feature is itself an end feature; a feature of any other type may redefine an end without being one. Write `end` before the redefining feature.

Specification: KerML 1.1 Beta 2 §8.3.3.3.8 validateRedefinitionEndConformance

#### `redefinition-package-level`

**error** · Constraints · a redefinition of a package-level feature

A feature owned directly by a package has no featuring type, so nothing can redefine it: redefinition replaces a feature in the context of a specializing type. Subset the feature instead, or move it into a type.

Specification: KerML 1.0 validateRedefinitionFeaturingTypes

#### `redefinition-same-featuring`

**error** · Constraints · a redefinition between features of one featuring type

A redefining feature and the feature it redefines are featured by different types: redefinition replaces an inherited feature, and two features of one type cannot redefine each other. Subset the feature instead, or redefine from a specializing type.

Specification: KerML 1.0 validateRedefinitionFeaturingTypes

#### `redefinition-through-reference`

**error** · Constraints · a chain redefinition stepping through a feature that owns no object

`:>> a.b` redefines `b` below `a` in this type's context, which needs `a` to own its values; a reference, subject or port usage owns none, so the redefinition has nothing to apply to. Redefine the feature where it is owned, or make the intermediate feature composite.

Specification: KerML 1.0 §8.3.3.3.6 Redefinition

#### `redefinition-type-mismatch`

**warning** · Constraints · a redefining feature typed by a type unrelated to the redefined feature's

The redefining feature is typed by both its declared type and the redefined feature's, so two unrelated types leave it with no possible value — almost always a slip, though no normative constraint forbids it. Type the redefinition by a specialization of the redefined feature's type.

Specification: KerML 1.0 §8.3.3.3.4 (a feature's types are its typings and its subsetted features'); an OpenSysML advisory

#### `reference-subsetting-at-most-one`

**error** · Constraints · a feature with more than one reference subsetting

A feature owns at most one reference subsetting (`::>` or `references`). Keep one.

Specification: KerML 1.0 validateFeatureOwnedReferenceSubsetting

#### `related-elements`

**error** · Constraints · an association with fewer than two related types

An association relates at least two types, one per end. Declare two or more ends.

Specification: KerML 1.0 validateAssociationRelatedTypes

#### `result-expression-at-most-one`

**error** · Constraints · a function or expression with more than one result expression

A function, expression or calculation body ends in at most one result expression, owned or inherited; this one has several. Keep one result, and bind the others to `out` parameters.

Specification: KerML 1.0 validateFunctionResultExpressionMembership and validateExpressionResultExpressionMembership

#### `return-parameter-owner`

**error** · Constraints · `return` outside a function, expression or calculation

Only a function or an expression — a calculation or a constraint in SysML — declares a result parameter; an action, a part or a package has none. Write `out` for an output, or make the owner a calculation.

Specification: KerML 1.0 validateReturnParameterMembershipOwningType

#### `send-payload-missing`

**error** · Constraints · a send as a state subaction or transition effect with no payload

A send written as an entry, do or exit action or as a transition effect must state what it sends (`send Sig() to x`); without a payload there is no signal to deliver. Write the payload expression.

Specification: SysML v2 validateSendActionUsagePayloadArgument

#### `send-receiver-not-occurrence`

**warning** · Constraints · a send whose receiver is no occurrence

`send ... to x` delivers to an occurrence; `x` is typed by an attribute definition or another non-occurrence, which cannot accept a signal. Name a part, action or other occurrence.

Specification: SysML v2 validateSendActionUsageReceiver

#### `send-sender-not-occurrence`

**warning** · Constraints · a send whose sender is no occurrence

`send ... from x` routes the signal through an occurrence of the sender; `x` is typed by a non-occurrence. Name a part, action or other occurrence, or drop the `from` clause.

Specification: SysML v2 §7.17 SendActionUsage

#### `send-to-port`

**warning** · Constraints · `send ... to <port>`, where `via` is meant

`to` names the occurrence that receives a signal; a port is the path it travels, which `via` names. Write `send Sig() via port`.

Specification: SysML v2 validateSendActionUsageReceiver

#### `specialization-cycle`

**error** · Constraints · a type that specializes itself, directly or through others

Specialization is acyclic: following `:>` from this type leads back to it. Break the cycle by dropping one specialization.

Specification: KerML 1.0 §8.3.3.1 (specialization is a partial order)

#### `subject-parameter-position`

**error** · Constraints · a subject that is not the first parameter

The subject of a requirement or case is its first parameter; here another `in` or `out` parameter is declared before it. Declare the subject before the other parameters.

Specification: SysML v2 validateRequirementDefinitionSubjectParameterPosition, validateRequirementUsageSubjectParameterPosition, validateCaseDefinitionSubjectParameterPosition, validateCaseUsageSubjectParameterPosition

#### `subsetting-constancy-conformance`

**error** · Constraints · a variable feature subsetting or redefining a constant one

A subsetting or redefining feature must be constant when the feature it subsets or redefines is constant. Declare the feature `const`, or make the subsetted one variable.

Specification: KerML 1.0 validateSubsettingConstantConformance

#### `subsetting-featuring-types`

**error** · Constraints · a subsetted feature that is not accessible from the subsetting one

A feature subsets or redefines a feature accessible from its featuring type: a feature of a sibling, named without dot notation, is not. Subset through a feature chain (`:> sibling.x`), or move the declaration.

Specification: KerML 1.0 validateSubsettingFeaturingTypes

#### `subsetting-multiplicity`

**warning** · Constraints · a subsetting or redefining feature with a larger upper bound than the subsetted one

The values of a subsetting feature are among those of the subsetted feature, so its multiplicity upper bound should not exceed the subsetted feature's. Lower the bound, or widen the subsetted feature's.

Specification: KerML 1.0 validateSubsettingMultiplicityConformance

#### `subsetting-uniqueness-conformance`

**error** · Constraints · a nonunique feature subsetting or redefining a unique one

A subsetting or redefining feature cannot be `nonunique` when the feature it subsets or redefines is unique: its values are among the other's, which admit no repetition. Drop `nonunique`, or declare the subsetted feature `nonunique`.

Specification: KerML 1.0 validateSubsettingUniquenessConformance

#### `succession-order-cycle`

**error** · Constraints · successions that order performances in a cycle

Following the successions of the body leads from a performance back to itself, so no order satisfies them and a run of the body would wait forever. Drop one succession, or make the loop explicit (`loop`, `while`).

Specification: KerML 1.0 §7.4.6 (successions are a partial order of occurrences)

#### `succession-orders-nothing`

**warning** · Constraints · a succession whose end is no performance of the run

One end of the succession names a requirement, a constraint or another usage whose evaluations are checked as verdicts rather than performed as steps, so the succession orders nothing a run does. Order actions, or drop the succession.

Specification: KerML 1.0 §7.4.6 Succession

#### `transition-source-not-vertex`

**error** · Constraints · a transition written without a source whose preceding member is no vertex

`then <target>;` takes the member declared before it as its source, and that member is an action, an attribute or another non-vertex of the state machine. Write the source explicitly, or declare the transition right after the state it leaves.

Specification: SysML v2 §7.18.3 TargetTransitionUsage

#### `type-conjugators`

**error** · Constraints · a type with more than one conjugator

A type conjugates at most one type. Remove the other `~`.

Specification: KerML 1.0 validateTypeAtMostOneConjugator

#### `type-differencing-not-one`

**error** · Constraints · `differences` with one type only

A type is the difference of two or more types or of none; a differencing of one is the type itself. Difference two or more types, or drop the clause.

Specification: KerML 1.0 validateTypeOwnedDifferencingNotOne

#### `type-differencing-self`

**error** · Constraints · a type differencing itself

A type cannot be one of its own differencing types. Difference other types.

Specification: KerML 1.0 validateTypeDifferencingTypesNotSelf

#### `type-intersecting-not-one`

**error** · Constraints · `intersects` with one type only

A type is the intersection of two or more types or of none; an intersecting of one is the type itself. Intersect two or more types, or drop the clause.

Specification: KerML 1.0 validateTypeOwnedIntersectingNotOne

#### `type-intersecting-self`

**error** · Constraints · a type intersecting itself

A type cannot be one of its own intersecting types. Intersect other types.

Specification: KerML 1.0 validateTypeIntersectingTypesNotSelf

#### `type-unioning-not-one`

**error** · Constraints · `unions` with one type only

A type is the union of two or more types or of none; a unioning of one is the type itself. Union two or more types, or drop the clause.

Specification: KerML 1.0 validateTypeOwnedUnioningNotOne

#### `type-unioning-self`

**error** · Constraints · a type unioning itself

A type cannot be one of its own unioning types. Union other types.

Specification: KerML 1.0 validateTypeUnioningTypesNotSelf

#### `variable-feature-owner`

**error** · Constraints · a variable (`var`) feature owned by a non-occurrence type

A feature that varies over time (`var`) is owned by an occurrence type, whose instances have a lifetime for it to vary over. Drop `var`, or move the feature into an occurrence, item or part definition.

Specification: KerML 1.0 validateFeatureIsVariable

#### `variant-kind`

**error** · Constraints · a variant whose kind is inconsistent with its owning variation

A variant specializes the variation it is a variant of, so the variation definition, or the types of the variation usage, type the variant; here the variant's kind does not take a type of that kind (an action variant of a part variation, an attribute variant of an action variation). Declare the variant with the variation's kind, or move it to a variation of its own kind.

Specification: SysML v2 §7.6.7 and checkUsageVariationDefinitionSpecialization, checkUsageVariationUsageSpecialization

#### `variant-outside-variation`

**error** · Constraints · a variant that is not an owned member of a variation

`variant` marks a member of a variation definition or usage; this one is declared elsewhere, or in an enumeration definition, whose enumerated values are variants already. Move the member into the variation, or drop the keyword.

Specification: SysML v2 validateVariationMembershipOwningNamespace

#### `variation-member-not-variant`

**error** · Constraints · a variation owning a usage that is no variant

Every owned usage of a variation definition or usage is a variant, an objective or an explicit parameter included (an enumeration definition's every member an enumerated value). Write `variant` before the usage, or declare it in a definition the variation specializes.

Specification: SysML v2 validateDefinitionVariationMembership and validateUsageVariationMembership

#### `variation-specialization`

**error** · Constraints · a variation specializing another variation

A variation does not specialize another variation; an enumeration definition, a variation by nature, does not specialize another enumeration. Specialize a non-variation definition, and declare the variants directly.

Specification: SysML v2 validateDefinitionVariationSpecialization and validateUsageVariationSpecialization

#### `verification-owning-type`

**error** · Constraints · `verify` outside the objective of a verification case

A requirement verification belongs in the objective of a verification case definition or usage. Move the `verify` into the objective, or write a `satisfy`.

Specification: SysML v2 validateRequirementVerificationMembershipOwningType

#### `view-satisfy-viewpoint`

**error** · Constraints · `satisfy` in a view body naming no viewpoint

A view satisfies viewpoints, which frame the concerns it is to address; the element named is a requirement or something else. Name a viewpoint usage.

Specification: SysML v2 §7.24 (views satisfy viewpoints)

### Lints

| Code | Severity | Summary |
|------|----------|---------|
| [`deferred-keeper-unmarked`](#deferred-keeper-unmarked) | warning | an accept of a deferred signal not marked as the keeping accept |
| [`port-type-mismatch`](#port-type-mismatch) | warning | a connection, interface or flow between ports of unrelated definitions |
| [`rounded-real-literal`](#rounded-real-literal) | warning | a decimal literal a Real feature holds rounded (opt-in) |
| [`undeclared-signal`](#undeclared-signal) | warning | `when <name>` naming no visible declaration and no sent signal |

#### `deferred-keeper-unmarked`

**warning** · Lints · an accept of a deferred signal not marked as the keeping accept

In a state annotated `MigrationMetadata::DeferredEvent`, the accept at the root of the do action that buffers the deferred signal is expected to be marked `MigrationMetadata::DeferredKeeper`, so the migration tooling and readers can tell the keeper from an ordinary accept. Add the `@DeferredKeeper` annotation. Switch the lint off with `-disable-lint deferred-keeper-unmarked`.

#### `port-type-mismatch`

**warning** · Lints · a connection, interface or flow between ports of unrelated definitions

The two ports joined are typed by port definitions neither of which specializes or conjugates the other, so nothing relates what one sends to what the other receives. Type one end by the conjugate (`~P`) of the other's definition, or relate the definitions. Switch the lint off with `-disable-lint port-type-mismatch`.

#### `rounded-real-literal`

**warning** · Lints · a decimal literal a Real feature holds rounded (opt-in)

A decimal literal bound to a feature typed by `Real` equals no binary64 value, so the feature holds the nearest double rather than the number written; exact comparisons against it may fail. Use a `Rational` feature where exactness matters, or accept the rounding. This lint is off until enabled with `-enable-lint rounded-real-literal`.

#### `undeclared-signal`

**warning** · Lints · `when <name>` naming no visible declaration and no sent signal

A transition's `accept when <name>` or `when <name>` trigger names something no declaration visible where it is written matches and no `send` in the model emits, so only a signal injected from outside by that name ever fires it — almost always a misspelling. Name the signal's attribute or item definition, or declare it. Switch the lint off with `-disable-lint undeclared-signal`.

### Method rules

| Code | Severity | Summary |
|------|----------|---------|
| [`mosa-boundary-not-designated`](#mosa-boundary-not-designated) | warning | a connection between major components that is no modular system interface |
| [`mosa-component-no-data-rights`](#mosa-component-no-data-rights) | warning | a major system component recording no data rights |
| [`mosa-interface-no-control`](#mosa-interface-no-control) | warning | a modular system interface naming no interface control authority |
| [`mosa-interface-no-standard`](#mosa-interface-no-standard) | warning | a modular system interface conforming to no standard |
| [`mosa-interface-not-traced`](#mosa-interface-not-traced) | warning | a modular system interface satisfying no requirement |
| [`mosa-proprietary-no-rationale`](#mosa-proprietary-no-rationale) | warning | an element marked `@Proprietary` with no rationale |
| [`oosem-logical-component-not-allocated`](#oosem-logical-component-not-allocated) | warning | a logical component allocated to nothing |
| [`oosem-requirement-not-derived`](#oosem-requirement-not-derived) | warning | a requirement derived from no higher-level requirement |
| [`oosem-requirement-not-satisfied`](#oosem-requirement-not-satisfied) | warning | a requirement nothing satisfies |
| [`oosem-use-case-subject`](#oosem-use-case-subject) | warning | a use case whose subject is untyped |
| [`self-check-rule`](#self-check-rule) | error | a model's own rule failing for one of its elements |

#### `mosa-boundary-not-designated`

**warning** · Method rules · a connection between major components that is no modular system interface

The connection joins two major system components (or modular systems), a boundary MOSA expects to be designated a modular system interface. Type the connection by a `#modularSystemInterface` definition or mark it so.

Specification: MOSA extension library

#### `mosa-component-no-data-rights`

**warning** · Method rules · a major system component recording no data rights

The model records data rights elsewhere, so MOSA expects a `@DataRights` annotation on this component or its definition stating the rights the program holds in its technical data. Add the annotation.

Specification: MOSA extension library

#### `mosa-interface-no-control`

**warning** · Method rules · a modular system interface naming no interface control authority

The model names an interface control authority for other interfaces, so MOSA expects one on this interface or its definition. Add an `@InterfaceControl` annotation.

Specification: MOSA extension library

#### `mosa-interface-no-standard`

**warning** · Method rules · a modular system interface conforming to no standard

The model declares standards, so MOSA expects every modular system interface to conform to one: a `#conformance` connection naming the interface at a `#conformant` end and a standard at a `#conformsTo` end, or a `@Proprietary` annotation with the rationale. Add the conformance connection or the annotation.

Specification: MOSA (Modular Open Systems Approach) extension library

#### `mosa-interface-not-traced`

**warning** · Method rules · a modular system interface satisfying no requirement

The model declares interface requirements, so MOSA expects each modular system interface, or its definition, to be named by a `satisfy ... by` statement. Add the satisfaction.

Specification: MOSA extension library

#### `mosa-proprietary-no-rationale`

**warning** · Method rules · an element marked `@Proprietary` with no rationale

MOSA expects the justification for each proprietary element to be recorded with it. Give the annotation a `rationale`.

Specification: MOSA extension library

#### `oosem-logical-component-not-allocated`

**warning** · Method rules · a logical component allocated to nothing

The model declares nodes or physical components, so OOSEM expects each logical component, or its type, to be allocated to one. Add an `allocate` from the component to a node or physical component.

Specification: OOSEM extension library

#### `oosem-requirement-not-derived`

**warning** · Method rules · a requirement derived from no higher-level requirement

The model declares requirements at several levels and states derivations, so OOSEM expects this one to derive from one: a `#derivation` connection naming it at a `#derive` end and the original at an `#original` end. Add the derivation.

Specification: OOSEM (Object-Oriented Systems Engineering Method) extension library

#### `oosem-requirement-not-satisfied`

**warning** · Method rules · a requirement nothing satisfies

The model states satisfactions, so OOSEM expects each requirement to be named by a `satisfy` statement. Add the satisfaction, or mark the requirement as not yet allocated.

Specification: OOSEM extension library

#### `oosem-use-case-subject`

**warning** · Method rules · a use case whose subject is untyped

The subject of a use case is the system it serves; OOSEM expects it typed by that system's definition. Type the subject.

Specification: OOSEM extension library

#### `self-check-rule`

**error** · Method rules · a model's own rule failing for one of its elements

A `-self-check-package` runs each constraint of the named package as a predicate over every element of the model whose metaclass its parameter accepts, and this element makes the rule false. Change the element so the rule holds, or amend the rule in the package.

Specification: Model-defined rule; the predicate is a KerML constraint evaluated over the model's metaobjects

### Documents and queries

| Code | Severity | Summary |
|------|----------|---------|
| [`document-plan`](#document-plan) | error | a document definition the document planner cannot compile |
| [`document-query`](#document-query) | error | a document query definition the query planner cannot compile |

#### `document-plan`

**error** · Documents and queries · a document definition the document planner cannot compile

A document definition (one typed by `DocumentDefinitions::Document` or a specialization of it) is compiled into a document plan before it is rendered; the member code names the rule that failed — a content with no title, an attribute that is not one string literal, a nested document, a query the document does not declare, a diagram kind no rendering supports — and the message states where. Correct the member the message names; the plain `document-plan` code reports a failure no rule classifies, with the planner's own message.

Emitted as `document-plan-<rule>`, one code per rule: `document-plan-invalid-context`, `document-plan-library-unavailable`, `document-plan-not-document-definition`, `document-plan-missing-title`, `document-plan-invalid-attribute`, `document-plan-invalid-content`, `document-plan-nested-document`, `document-plan-missing-text`, `document-plan-conflicting-text`, `document-plan-missing-query`, `document-plan-conflicting-query`, `document-plan-unknown-query`, `document-plan-query-planning`, `document-plan-invalid-style`, `document-plan-unknown-parameter`, `document-plan-duplicate-binding`, `document-plan-missing-binding`, `document-plan-unsupported-binding`, `document-plan-binding-type`, `document-plan-binding-multiplicity`, `document-plan-missing-view-source`, `document-plan-invalid-view-source`, `document-plan-unknown-view-source`, `document-plan-missing-diagram-kind`, `document-plan-conflicting-diagram-kind`, `document-plan-unsupported-diagram-kind`, `document-plan-invalid-direction`, `document-plan-unsupported-direction`, `document-plan-invalid-palette`, `document-plan-unsupported-palette`, `document-plan-invalid-ports`, `document-plan-unsupported-ports`, `document-plan-invalid-show`, `document-plan-invalid-overlay`, `document-plan-unsupported-overlay`, `document-plan-conflicting-runs`, `document-plan-ambiguous-run`, `document-plan-missing-run-text`, `document-plan-invalid-run-style`, `document-plan-missing-link-target`, `document-plan-missing-ref-target`, `document-plan-unknown-ref-target`, `document-plan-invalid-ref-target`, `document-plan-ambiguous-ref-target`, `document-plan-unknown-value-target`, `document-plan-invalid-value-target`, `document-plan-unknown-group-column`, `document-plan-invalid-column-widths`, `document-plan-invalid-column-labels`, `document-plan-column-run-without-query`, `document-plan-conflicting-column-runs`, `document-plan-missing-run-column`, `document-plan-conflicting-run-style`, `document-plan-unknown-run-column`, `document-plan-missing-definition-column`, `document-plan-unknown-definition-column`, `document-plan-missing-formula-source`, `document-plan-missing-image-location`, `document-plan-unsupported-markup-language`, `document-plan-unsupported-markup-construct`, `document-plan-misplaced-markup-body`, `document-plan-markup-body-not-paragraph`, `document-plan-markup-tool-missing`, `document-plan-markup-tool-failed`, `document-plan-markup-heading-level`.

Specification: OpenSysML DocumentDefinitions library

#### `document-query`

**error** · Documents and queries · a document query definition the query planner cannot compile

A query definition (one typed by `DocumentQueries::Query` or a specialization of it) is compiled into a query plan; the member code names the rule that failed — a missing result parameter, an argument of the wrong type or count, a composition cycle, a column with no source — and the message states where. Correct the member the message names; the plain `document-query` code reports a failure no rule classifies, with the planner's own message.

Emitted as `document-query-<rule>`, one code per rule: `document-query-invalid-context`, `document-query-library-unavailable`, `document-query-not-query-definition`, `document-query-missing-result-parameter`, `document-query-invalid-parameter`, `document-query-missing-result`, `document-query-conflicting-result`, `document-query-unsupported-result`, `document-query-unknown-invocation`, `document-query-ambiguous-invocation`, `document-query-positional-query-arguments`, `document-query-duplicate-argument`, `document-query-unknown-argument`, `document-query-missing-argument`, `document-query-argument-count`, `document-query-argument-type`, `document-query-argument-multiplicity`, `document-query-composition-cycle`, `document-query-unknown-parameter`, `document-query-unsupported-expression`, `document-query-unsupported-default`, `document-query-default-type`, `document-query-default-multiplicity`, `document-query-invalid-column`, `document-query-column-source`, `document-query-column-name`, `document-query-unknown-column-property`, `document-query-column-operator`, `document-query-column-type`, `document-query-duplicate-column`, `document-query-empty-projection`, `document-query-column-aggregate`, `document-query-collection-function`.

Specification: OpenSysML DocumentQueries library

### Workspace

| Code | Severity | Summary |
|------|----------|---------|
| [`library-declared-twice`](#library-declared-twice) | warning | a library package declared by more than one document of the workspace |

#### `library-declared-twice`

**warning** · Workspace · a library package declared by more than one document of the workspace

Two documents the workspace loads declare the same `library package`: two copies of the standard library under the workspace folder or the library path, a copy of a standard library file beside the bundled one, or a user library vendored twice. Every reference to the package resolves to one copy — the first the index registered, which the message names — and the other is unused, so edits to it change nothing; a filtered view or a metaclass lookup reading the duplicated declarations sees both. Reported once per pair of trees the copies lie in, on the first file of the unused copy; for copies on the library path, the locations are relative to `OPENSYSML_LIBRARY_PATH`, which the message names. Remove the unused copy, or point `OPENSYSML_LIBRARY_PATH` at one tree.

### Runtime

| Code | Severity | Summary |
|------|----------|---------|
| [`action-step-lowering`](#action-step-lowering) | warning | an action body the executor cannot lower |
| [`action-step-multiplicity-not-fixed`](#action-step-multiplicity-not-fixed) | warning | a repeated action step with no exact finite count |
| [`action-step-multiplicity-unsupported`](#action-step-multiplicity-unsupported) | warning | a repeated action step linked in a shape the executor does not run |
| [`action-step-order-open`](#action-step-order-open) | warning | a succession that does not order a step's repetitions |
| [`action-step-order-unsatisfiable`](#action-step-order-unsatisfiable) | warning | succession multiplicities that exclude the step's count |
| [`choice-point`](#choice-point) | warning | a run that chose among several enabled transitions or successions |
| [`guard-unevaluable`](#guard-unevaluable) | warning | a guard the run could not evaluate |
| [`random-draw`](#random-draw) | warning | a run that drew a random value |
| [`runtime.materialize`](#runtimematerialize) | error | a feature value the run could not materialize |
| [`runtime.materialize.bounded`](#runtimematerializebounded) | warning | a check that was bounded rather than completed |
| [`succession-order-violated`](#succession-order-violated) | error | a run that began a performance before one a succession orders first |
| [`tool-divergence`](#tool-divergence) | warning | an external tool whose result diverged from the model's |

#### `action-step-lowering`

**warning** · Runtime · an action body the executor cannot lower

The action body uses a shape the lowering to the executor's graph does not implement, named in the message; the body is checked but cannot be run. Rewrite the body with the forms the message suggests.

#### `action-step-multiplicity-not-fixed`

**warning** · Runtime · a repeated action step with no exact finite count

A step with a multiplicity other than 1 is performed that many times; a bound of `*` or a range (`[1..3]`) fixes no count the executor can run. Write an exact multiplicity (`[3]`), or loop explicitly.

#### `action-step-multiplicity-unsupported`

**warning** · Runtime · a repeated action step linked in a shape the executor does not run

The repeated step is joined by flows or successions the executor cannot unroll per performance (a fork driving it, a flow into one of its pins). Write the repetition as a `loop` or `for` body.

#### `action-step-order-open`

**warning** · Runtime · a succession that does not order a step's repetitions

A succession touching a repeated step says nothing about how its performances follow one another, so the executor runs them in an order the model does not fix. Write the end multiplicities (`first [1] a then [1] b`) that order them, or declare the step `ordered`.

#### `action-step-order-unsatisfiable`

**warning** · Runtime · succession multiplicities that exclude the step's count

The multiplicities written on the successions into and out of a repeated step admit no number of performances that the step's own multiplicity requires. Make the succession and step multiplicities agree.

#### `choice-point`

**warning** · Runtime · a run that chose among several enabled transitions or successions

Several transitions or successions were enabled at once and the model does not say which to take; the run took the one the message names. Add guards that make the choice, or accept the recorded choice.

#### `guard-unevaluable`

**warning** · Runtime · a guard the run could not evaluate

A transition or succession guard reads a feature that holds no value yet or calls what the runtime cannot evaluate, so the run treated it as not taken. Bind the features the guard reads, or simplify the guard.

#### `random-draw`

**warning** · Runtime · a run that drew a random value

The model called a random function; the run records the draw so the result can be reproduced from its seed. Fix the seed for a reproducible run.

#### `runtime.materialize`

**error** · Runtime · a feature value the run could not materialize

A feature read by the run has a value expression the runtime could not evaluate: an unbound parameter, an unsupported function, a value of the wrong type. Bind what the expression reads, or rewrite it with supported forms.

#### `runtime.materialize.bounded`

**warning** · Runtime · a check that was bounded rather than completed

A constraint or requirement check over an unbounded domain was cut off at the run's bound, so what it reports holds of the cases examined only. Raise the bound, or restrict the domain.

#### `succession-order-violated`

**error** · Runtime · a run that began a performance before one a succession orders first

A succession orders one performance before another, and the run began the later one while the earlier had not started: the successions and the behavior's own control flow disagree. Align the successions with the body's flow.

Specification: KerML 1.0 §7.4.6 Succession

#### `tool-divergence`

**warning** · Runtime · an external tool whose result diverged from the model's

A tool invoked by the run returned a value that disagrees with what the model's own calculation gives for the same inputs. Check the tool's binding and units against the calculation.
<!-- diagnostics-catalogue:end -->

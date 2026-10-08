# Diagnostics: lints and checker warnings

A *lint* is a warning about a model the specification accepts: nothing in SysML v2 or KerML
makes the written model wrong, but it is almost always a slip. Each lint has a stable code,
carried as the diagnostic's `code` under `-json`, in the editor and over the wire, and each
can be switched off by that code. A lint marked *opt-in* below is off until it is switched on
by its code, since what it reports holds of ordinary models more often than it marks a slip.
A lint is a warning in every mode:
[`-strict`](../guide/03-command-line.md#strict-conformance) promotes notation no SysML v2
production admits, and a lint is not about notation, so it stays a warning and never changes
the exit status or blocks a check.

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
as warnings with code `name-conflict`; they are not lints and cannot be switched off. Two
memberships of one namespace are indistinguishable when one's name or short name is the other's
and their metaclasses are related (a `part def` beside a `part def`, or beside an `item def`
that it specializes; a `part def` beside an `attribute` is distinguishable whatever the names).
Resolution is not affected: a reference to the name still takes the first membership, in
declaration order and then in import order.

| Wording | Reported on |
|---------|-------------|
| `Duplicate of other owned member name 'x'` | the later of two owned members of one namespace |
| `Duplicate of inherited member name 'x' from T` | an owned member of a type whose name an inherited member already has |
| `Duplicate of imported member name 'x': P::x (import P::*), Q::x (import Q::*)` | the import that brings the later of two imported members, naming each member and the import that brought it |

An imported name hidden by an owned member of the same name, one membership reached through two
imports (`import P::*` beside `import Q::*` where `Q` publicly re-imports `P`), an alias or a
membership import of an element beside the membership that owns it, and the standard library's
own members are not reported.

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

## `undeclared-signal`

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

## `port-type-mismatch`

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

## `deferred-keeper-unmarked`

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

## `rounded-real-literal`

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

# Diagnostics: lints

A *lint* is a warning about a model the specification accepts: nothing in SysML v2 or KerML
makes the written model wrong, but it is almost always a slip. Each lint has a stable code,
carried as the diagnostic's `code` under `-json`, in the editor and over the wire, and each
can be switched off by that code. A lint is a warning in every mode:
[`-strict`](../guide/03-command-line.md#strict-conformance) promotes notation no SysML v2
production admits, and a lint is not about notation, so it stays a warning and never changes
the exit status or blocks a check.

| Code | Reported on | Tier |
|------|-------------|------|
| `undeclared-signal` | a transition's `when <name>` or a state's `defer <name>` whose name matches no declaration visible where it is written and no signal the model sends | name resolution |
| `port-type-mismatch` | a `connect`, an interface usage or a `flow` joining two ports whose definitions are unrelated | constraint |

## Switching a lint off

| Surface | Setting |
|---------|---------|
| CLI | `-disable-lint <code>[,<code>…]`, repeatable; an unknown code is a usage error (exit 2) |
| REPL | `%lint` lists every lint, on or off; `%lint <code> on\|off` switches one and reprints the session's diagnostics |
| Language server | `disabledLints`, a list of codes, in `initializationOptions` or `workspace/didChangeConfiguration` ([LSP extensions](lsp.md#disabled-lints-setting)) |
| Go | `model.WithDisabledLints(codes...)` when the workspace is made, or `(*model.Workspace).SetDisabledLints(codes)` |

A disabled lint is left out of what the workspace reports, not out of the analysis, so
switching it back on needs no re-analysis. The gRPC service and the clients report both lints
with their codes; a client that does not want one drops the diagnostics carrying its code.

## `undeclared-signal`

OpenSysML accepts a signal trigger written without `accept` — `transition first a when Ping
then b;` — and a deferred event, `defer Ping;`. Neither spelling is standard: the pinned
pilot grammar admits `when` only as a change trigger over a Boolean expression inside an
`accept` (`SysML.xtext:1483-1485`, `ChangeTriggerKind`) and has no `defer` literal at all
([conformance audit](grammar/conformance-audit.md)). The name such a trigger carries names an
event, not a model element: it is left unresolved, and at run time it matches a signal
injected or sent by that name. A misspelled name therefore matches nothing and the transition
never fires, silently.

The lint reports the name when all of the following hold:

- no declaration of that name is visible where the trigger is written, by the ordinary
  scoping rules (KerML §7.2.5, §8.2.3.5);
- no `send` anywhere in the workspace sends a signal by that name — neither the name of a
  payload's type (`send new Ping() to self;`) nor the name of a payload feature
  (`send ping to self;`) — the final segment being compared, as the runtime compares it.
  A send invoking a calculation (`send Ping() to self;` with `calc def Ping`) sends the
  calculation's value, as the runtime does, so it counts the result's type, not `Ping`.
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
- the connector is typed by an interface or connection definition with two or more ends,
  every one typed by a port definition, directly or through a model end it redefines: that
  definition decides what the ends pair, and its own ends are judged by `port-conjugation`. A definition leaving an end untyped decides
  nothing, so the concrete ends are judged.

Otherwise the lint reports the connector at its first end:

```text
m.sysml:9:13: warning: this connection connects port power : PowerOut to port fuel : FuelIn, whose definitions are unrelated and whose directed features are not conjugate; type one end by the conjugate port (~PowerOut) or by a common definition
```

It covers `connect a.p to b.q;` (and `connection … connect`), interface usages
(`interface connect a.p to b.q;`) and `flow` between two ports, whose syntax is
`ConnectionUsage`, `InterfaceUsage` and `FlowUsage` (`SysML.xtext:1062`, `:1153`, `:1269`).
An end that is not a port, or whose port type does not resolve, is not judged. The
specification states no constraint of this kind, so it is a lint rather than an error.

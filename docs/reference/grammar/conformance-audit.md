# Grammar Conformance Audit — reserved words and state/action notation

This page is the evidence behind two findings of the
[pilot differential](../../project/pilot-differential.md): that OpenSysML reserved
words the OMG grammars do not, and rejected state-machine notation they allow. It
lists every reserved word and every accepted state-machine construct, checked
against the pinned OMG grammars, and states the resulting policy for each.

## Ground truth

The grammars are the ones at the pin in `scripts/pilot-pin.sh`
(`PILOT_TAG=2026-08`, `Systems-Modeling/SysML-v2-Pilot-Implementation`), read
from a sparse clone rather than vendored:

- `org.omg.kerml.xtext/src/org/omg/kerml/xtext/KerML.xtext` — cited as `KerML.xtext`
- `org.omg.sysml.xtext/src/org/omg/sysml/xtext/SysML.xtext` — cited as `SysML.xtext`
- `org.omg.kerml.expressions.xtext/src/org/omg/kerml/expressions/xtext/KerMLExpressions.xtext` — cited as `KerMLExpressions.xtext`

A word counts as *standard* when it appears as a quoted literal in one of those files,
in the position where OpenSysML accepts it. Line numbers refer to the pinned revision.

## Verdict per word

None of the eleven words below appears as a literal in **any** of the three grammars.
They are not notation OMG defines, so reserving them only stopped models from using
them as names. Each is now an ordinary name, recognized contextually where OpenSysML's
own notation needs it, which is how `point`, `on` and `var` were already treated.

| Word | `KerML.xtext` | `SysML.xtext` | `KerMLExpressions.xtext` | Verdict |
|------|---------------|---------------|--------------------------|---------|
| `choice` | absent | absent | absent | unreserve; notation is an OpenSysML extension (warning) |
| `decision` | absent | absent | absent | unreserve; **an ordinary name only** — the action node spelled `decision` is no longer accepted, write `decide` |
| `deep` | absent | absent | absent | unreserve; notation is an OpenSysML extension (warning) |
| `defer` | absent | absent | absent | unreserve; notation is an OpenSysML extension (warning) |
| `done` | absent | absent | absent | unreserve; **silent** — see "`done` is a library name, not notation" |
| `final` | absent | absent | absent | unreserve; **an ordinary name only** — neither the action node nor the state marker spelled `final` is accepted, write `done` |
| `history` | absent | absent | absent | unreserve; notation is an OpenSysML extension (warning) |
| `initial` | absent | absent | absent | unreserve; **an ordinary name only** — neither the action node nor the state marker spelled `initial` is accepted, write `first <name>` and `entry; then <state>;` |
| `junction` | absent | absent | absent | unreserve; notation is an OpenSysML extension (warning) |
| `region` | absent | absent | absent | unreserve; **an ordinary name only** — the orthogonal-region member spelled `region <name> { … }` is no longer accepted, mark the owning state's body `parallel` |
| `shallow` | absent | absent | absent | unreserve; notation is an OpenSysML extension (warning) |

`done` is the acceptance test that unreserving worked: the bundled normative
library declares features named `done` (`Systems Library/Actions.sysml:50`,
`Items.sysml:34`, `Parts.sysml:29`, `States.sysml:34`, `UseCases.sysml:28`) and
references them (`Flows.sysml:57`, `:69`). OpenSysML used to report an error on every one.

### `done` is a library name, not notation

The OMG corpora write `then done;` (`Systems Library/Actions.sysml:230`;
training examples `17. Control/Fork Join Example.sysml:39`, `Decision
Example.sysml:32`, `Control Structures Example.sysml:27`, `35. Use Cases/Use
Case Usage Example.sysml:35`) and `snapshot junked = done;`
(`27. Occurrences/Time Slice and Snapshot Example.sysml:25`). Those are plain
references to `Actions::Action::done`, a feature of the standard library, not a
keyword. The parser reads `done;` as an anonymous final node and `then done;`
as a succession targeting the `done` library feature. Both stay **silent**:
warning on them would warn on OMG-authored files, which the classification
forbids. In a state body `then done;` names the same library feature and states
that the machine completes; the `final <state>;` marker that once spelled it is
no longer accepted.

## Verdict per construct

Positions checked against `SysML.xtext` `StateBodyItem` (1755-1770),
`StateDefBody` (1744-1746), `StateUsageBody` (1836-1838), `TransitionUsage`
(1851-1880), and the action node productions (1666-1730).

### Standard — silent

| Construct | Citation |
|-----------|----------|
| `entry <action>` state subaction | `SysML.xtext:1772-1778` (`EntryActionMember`, `EntryActionKind : 'entry'`) |
| `do <action>` state subaction | `SysML.xtext:1780-1786` (`DoActionMember`, `DoActionKind : 'do'`) |
| `exit <action>` state subaction | `SysML.xtext:1788-1794` (`ExitActionMember`, `ExitActionKind : 'exit'`) |
| `parallel` before a state body | `SysML.xtext:1745`, `:1837` (`isParallel ?= 'parallel'`) |
| `then <target>;` succession | `SysML.xtext:1705`, `:1711`, `:1724`, `:1799`; `KerML.xtext:894` |
| `first <source>` | `SysML.xtext:1385`, `:1720`, `:1855`; `KerML.xtext:893` |
| `done;` / `then done;` final node | no literal; a reference to `Actions::Action::done`, written by the OMG corpora (above) |
| `fork <name>;`, `join <name>;`, `merge <name>;`, `decide <name>;` | `SysML.xtext:1684`, `:1678`, `:1666`, `:1672`; admitted in a state body by `StateBodyItem` → `BehaviorUsageMember` (`SysML.xtext:1761-1763`) |
| `accept <trigger> [via <port>]` | `SysML.xtext:1447`, `:1894` (`trigger = 'accept'`), `via` at `:1450` |
| `when <expr>` trigger | `SysML.xtext:1483-1485` (`ChangeTriggerKind : 'when'`) |
| `transition [<name>] first <src> … then <tgt>;` | `SysML.xtext:1851-1880` (`TransitionUsage`) |
| `state <name>;` / `state <name> { … }` | `SysML.xtext:1733`, `:1741`, `:1833` |
| `send`, `terminate`, `assign`, `perform`, `if`/`else`, `while`, `loop`, `for` | `SysML.xtext:1500`, `:1643`, `:1540`, `:1412`, `:1600` and the loop node productions |
| `namespace N;` / `namespace N { … }` in a `.kerml` file | `KerML.xtext:119`, `:124-125` (`'namespace' Identification?`), `:128` (`NamespaceBody : ';' \| '{' … '}'`) |

### OpenSysML extension — warning `nonstandard-notation`

No production in the pinned grammars allows these anywhere. They are still parsed
(removing notation users already write is worse than diagnosing it) and are
reported as a warning by default. Under the opt-in
[strict conformance mode](../../guide/03-command-line.md#strict-conformance) the
same findings are errors. Strictness changes only the severity of this table's
diagnostics, so the tree, the spans and the messages are the same in either mode.
Strict mode is what answers the question "is this file conforming SysML v2?"; the
default mode's acceptance of these constructs is intentional and unchanged.

The removed OpenSysML-only spellings `then <source> <target>;`, member-leading
`<source> then <target>;`, `done <name>;`, the orthogonal-region member
`region <name> { … }`, the state markers `initial <state>;` and `final <state>;`
and
`transition [<name>] <src> to <tgt>;` are no longer accepted. Use
`succession first <source> then <target>;`, `done;`, a `parallel` state body with
one state substate per region, `entry; then <state>;`, a transition targeting
`done` and `transition [<name>] first <src> … then <tgt>;` instead.

| Construct | Why it is not standard |
|-----------|------------------------|
| `choice <name>;`, `junction <name>;` | no literal; no pseudostate production of any kind |
| `history <name>;`, `shallow history <name>;`, `deep history <name>;` | same |
| `defer <event> [, <event>]*;` | no `defer` literal; `StatePerformance::deferrable` has the semantics but no notation |

Two further findings are about position rather than spelling: the construct is
standard where a production allows it and an OpenSysML extension everywhere else, so
the warning names the position, not the keyword.

| Construct | Where it is standard | Why it is not standard elsewhere |
|-----------|----------------------|----------------------------------|
| `assume <constraint>;`, `require <constraint>;` | a requirement, concern, viewpoint or objective body | `RequirementConstraintMember` (`SysML.xtext:2039`) is the only production that admits it |
| a one-ended `first <node>;` | an action body | `InitialNodeMember` is reachable from `ActionBodyItem` alone (`:1376`), never from `DefinitionBodyItem` (`:516`); elsewhere a succession names both ends, `first <source> then <target>` |

### Removed extension notation — no longer accepted

An inline condition introduced by a keyword (`assert <expression>;` or
`assume <expression>;` in a constraint body, `assume <expression>;` or
`require <expression>;` in a requirement-style body) was an OpenSysML extension and
is now a parse error: `AssertConstraintUsage` (`SysML.xtext:2007-2013`) and
`RequirementConstraintUsage` (`:2066-2071`) allow a reference subsetting or a
`constraint` declaration after the keyword, never an expression. The standard
spellings, which are unchanged, are a bare condition in a constraint
body (`total <= limit`), `assert [not] <reference>;`,
`assert constraint { … }`, and `assume`/`require constraint { … }` in a
requirement body. A requirement body allows no bare condition
(`RequirementBodyItem`, `:2039-2047`). If you had a negated keyword form, write the
negation as `not (…)` inside the condition to keep the same meaning.

A spelling that is a pure alias of a standard construct is removed rather than
warned, so it is no longer accepted and has no row here. `return <expression>;`
in a calculation body is now a parse error; a computed result is written as
the body's trailing expression (`ResultExpressionMember`, `SysML.xtext:1967`).
`return` itself is unchanged as the result parameter declaration.

`bind <feature> = <expression>;` is removed the same way: a binding relates two
`ConnectorEndMember`s (`BindingConnectorAsUsage`, `SysML.xtext:1020`), so an
expression right end is now a parse error. Write the expression as the
feature's value (`out result : Real = x * 2.0;`), or declare a feature holding
the result and bind to it (`attribute b2 = a + 1;` then `binding bind b = b2;`).
Standard bindings (`bind a = b;`, with qualified, chained and indexed ends)
are unchanged.

The same holds for the two state-machine aliases. `initial <state>;` said where
a machine starts, which `EntryTransitionMember` (`SysML.xtext:1796-1801`) writes
as `entry; then <state>;`, and `transition [<name>] <src> to <tgt>;` gave a
transition's ends, which `TransitionUsage` (`:1851-1880`) writes with `first` and
`then`. `to` is a literal (`:1077`, `:1168`, `:1253`, `:1287`; `KerML.xtext:838`,
`:1009`) in connector, interface, message and flow ends only. Both are parse
errors now, and `initial`, which neither grammar reserves, stays an ordinary
name.

### KerML-only notation in a `.sysml` file — warning `kerml-notation`

`namespace` is a literal in `KerML.xtext` only (`:125`); `SysML.xtext` does not have
it. A `.sysml` file's root is `RootNamespace : PackageBodyElement*`
(`SysML.xtext:38`), which allows package members, element filters, aliases and
imports, but not a `namespace` declaration. Both spellings, `namespace N;` and
`namespace N { … }`, are legal KerML, so the semicolon form is not the problem;
the problem is using the production in a SysML file at all. It is still parsed,
with a warning in `.sysml` and silently in `.kerml`.

### Reserved by SysML only — a name in a `.kerml` file

A word is reserved by the grammar of the file it is written in, so a literal that
appears only in `SysML.xtext` is an ordinary name in KerML. These four are reserved
or not depending on the file kind; they keep their SysML meanings in `.sysml`, where
the literal exists.

| Word | `KerML.xtext` | `KerMLExpressions.xtext` | `SysML.xtext` | Corpus witness |
|------|---------------|--------------------------|---------------|----------------|
| `at` | absent | absent | `:1480` (`TimeTriggerKind`) | `expr at { … }`, `Variable Feature Examples/Enhancements/ExtendedOccurrences.kerml:16` |
| `while` | absent | absent | `:1617` (`WhileLoopActionUsage`) | `expr while { … }`, same file `:25` |
| `merge` | absent | absent | `:1666` (`MergeNode`) | `member step merge : …`, `Enhancements/TimeVaryingSteps.kerml:4`, imported at `:6` |
| `decide` | absent | absent | `:1672` (`DecisionNode`) | `member step decide : …`, same file `:25`, imported at `:27` |

`featured by` is the opposite case: `TypeFeaturingPart` (`KerML.xtext:569-571`)
and `OwnedTypeFeaturing` (`:659`) are KerML productions with no SysML
counterpart, so the clause is parsed everywhere and warned as `kerml-notation`
in a `.sysml` file, the same treatment `namespace` gets.

## Judgment calls

Everything above follows directly from a grammar citation. The decisions below do
not, and are recorded here so a reviewer can disagree with them:

- **`done` silent.** No grammar has the literal. `done` is classified standard
  because the OMG corpora write it (as a library-feature reference) and a warning
  there would be a false positive; `final` appears in no OMG file and is no
  longer notation in any position.
- **An alias of a standard node is removed, not warned.** The action nodes
  spelled `initial`, `final` and `decision` were pure aliases of `first`, `done`
  and `decide`, so they are gone rather than diagnosed; each word stays an
  ordinary name. The state markers `initial <state>;` and `final <state>;` and the
  transition spelling `transition <src> to <tgt>;` are gone for the same reason.
- **Completion is stated, not inferred.** A machine completes when a transition
  reaches `done`, the library feature the OMG corpora already reference in a state
  body. A state with no outgoing transition does not complete on its own, because
  an ancestor or cross-region transition may still leave it.
- **State-body `fork`/`join` silent.** They are action node literals, and
  `StateBodyItem` allows a `BehaviorUsageMember`, so they are read as standard in
  a state body even though the pilot's state examples do not use them there.

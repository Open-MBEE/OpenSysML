# Grammar Reference

**This directory does not contain grammar files.** The parser in `internal/core/parser/` is hand-written recursive descent.

## OMG Grammar Reference

For reference, the official Xtext grammar files from OMG are available at:

**KerML Grammar:**
- https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/blob/master/org.omg.kerml.xtext/src/org/omg/kerml/xtext/KerML.xtext

**SysML v2 Grammar:**
- https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/blob/master/org.omg.sysml.xtext/src/org/omg/sysml/xtext/SysML.xtext

These files are licensed under the Eclipse Public License 2.0 (EPL-2.0), so they are not included in this repository. When you need them, `./scripts/download-pilot-grammars.sh` fetches them at the pinned release into `build/pilot-grammars/`.

[grammar-coverage.md](../../project/grammar-coverage.md) reports which grammar productions the project's test inputs exercise. It measures whether an input for each production is present, not whether the parser code for it ran.

## Conformance Audit

[conformance-audit.md](conformance-audit.md) goes through every keyword and construct OpenSysML accepts and sorts it into one of three groups: standard notation (accepted silently), OpenSysML extensions (accepted with a `nonstandard-notation` warning), and KerML notation used in a `.sysml` file (accepted with a `kerml-notation` warning). Each entry cites the `file:line` in the pinned OMG grammar that justifies it.

## State Machine Notation Beyond the OMG Grammar

The OMG textual notation covers state definitions and usages, `entry`/`do`/`exit`
subactions, and transitions (`SysML.xtext`, `StateDefinition` through `TransitionUsage`,
near the `/* STATES */` section). That is all it says about state machines: there
is no production for any kind of pseudostate, and none for event deferral.
Some of these concepts do have semantics in the bundled KerML semantic
library or the Systems Library, and where they do, that library is the governing reference.
For the rest we cite UML 2.5.1 §14.2.3.4 (Pseudostates), but UML's notation for them is
diagrammatic, so there is no textual syntax to borrow.

OpenSysML therefore defines its own keywords for them, valid only inside a state body.
They are a documented extension, not OMG notation, and using one produces a
`nonstandard-notation` warning:

| Form | Meaning | Semantic reference |
|------|---------|-------------|
| `choice <name>;` | dynamic conditional branch | KerML `ControlPerformances::DecisionPerformance` — selects one of the successions leaving it, `outgoingHBLink: HappensBefore[1]` (notation is an OpenSysML invention) |
| `junction <name>;` | static branch/merge | KerML `DecisionPerformance::outgoingHBLink[1]` / `MergePerformance::incomingHBLink[1]` (notation is an OpenSysML invention) |
| `fork <name>;` | parallel split | UML `fork` pseudostate (a state-body fork has no SysML v2 or KerML counterpart; the action-level one is `Actions::ForkAction`) |
| `join <name>;` | parallel synchronization | UML `join` pseudostate (a state-body join has no SysML v2 or KerML counterpart; the action-level one is `Actions::JoinAction`) |
| `history <name>;` | shallow history (UML `H`) | UML `shallowHistory` pseudostate |
| `shallow history <name>;` | shallow history, spelled out | UML `shallowHistory` pseudostate |
| `deep history <name>;` | deep history (UML `H*`) | UML `deepHistory` pseudostate |
| `entry point <name>;` | entry point | UML `entryPoint` pseudostate |
| `exit point <name>;` | exit point | UML `exitPoint` pseudostate |
| `defer <event> [, <event>]*;` | events the state retains while active | KerML `StatePerformances::StatePerformance::deferrable: Transfer[0..*] subsets acceptable` — "transfers … can be considered for acceptance more than once"; dispatch order is `Occurrences::Occurrence::incomingTransferSort`, defaulting to `earlierFirstIncomingTransferSort` |

The action-level `fork` and `join` control nodes are SysML v2's `Actions::ForkAction`
and `JoinAction`. The library gives them no behavior of their own (a
`ControlAction` has "no inherent behavior"); their effect "results from requiring that the target
[respectively source] multiplicity of all outgoing [incoming] succession
connectors be 1..1".

Notes:

- `fork` and `join` are the exception in the table above. Both are action node
  literals that a state body already admits (`SysML.xtext:1684`, `:1678`, `:1761-1763`), so
  they count as standard and are not warned about.
- None of `choice`, `decision`, `deep`, `defer`, `done`, `final`, `history`,
  `initial`, `junction` or `shallow` is a reserved word. None of them appears as a literal
  in the pinned grammars, so they remain ordinary names and are recognized only in the
  positions where the notation above needs them.
- `point` is **not** reserved either. It is recognized only after `entry`
  or `exit`, and only when a pseudostate name and `;` follow, because models
  routinely declare features named `point`. `entry <action>` keeps its OMG
  meaning.
- `on` and `var` are **not** reserved, for the same reason and on the
  pilot implementation's authority: `on` is not a literal in any of its grammars,
  and `var` appears only in `KerML.xtext`'s `BasicFeaturePrefix` (`isVariable ?= 'var'`).
  So `state on { … }`, `then on;` (the OMG training corpus writes both) and
  `attribute var : Integer;` all declare and name features, while `var` before a
  kind keyword (`var feature x`, `var attribute total : Integer;`) still marks a
  variable feature. `var` without a kind keyword is not supported and is
  reported. See [pilot-differential.md](../../project/pilot-differential.md).
- A deferred event is parsed exactly like a transition trigger, so both a signal
  name (`defer Ping;`) and a call event (`defer setSpeed(value);`) are accepted.
  Time and change events cannot be deferred; lowering reports them.
- `defer` is only meaningful inside a state. One written in the machine's own body is
  reported by `lower.ToStateGraph`.
- A transition without a source part (`accept go then s;`, `if c then s;`, `then s;`,
  `transition if c then s;`) is parsed with no source; the state declared before it in
  the same body is its source (SysML v2 §7.18.3, `TargetTransitionUsage`), derived by
  `ast.ImplicitTransitionSource` for validation and lowering. It is a member of the body
  that declares that state, written after it — the pilot's grammar does not accept it
  inside the state's own body — and one written first in its body or after a member that
  is not a state is reported.
- The same shorthand written right after the body's entry action (`entry; then s;`,
  `entry; if c then s;`, `entry action boot { … } if c then s;`) is an entry transition
  (SysML v2 §7.18.3, `EntryTransitionMember`), naming a state the body starts in. It
  carries a guard at most; `lower.ToStateGraph` reports one with a trigger or an effect,
  or whose target is not a state, as does the constraint tier.
- Unreserved does not mean invisible to editors. `lexer.ContextualWords(kind)`
  lists these words for the two places that want them, the VS Code grammars
  (`keywords-contextual`) and LSP keyword completion, without the lexer
  reserving any of them. `var` is in the `.kerml` list only, and `on` is in
  neither, since it is never syntax. The two lists are checked to be disjoint when
  the grammars are generated, so listing a word cannot make it reserved.

## Validation

Grammar conformance is validated by parsing **OMG's own files**:

1. **Stdlib conformance gate** - all 98 bundled library files (94 OMG standard library files and 4 OpenSysML extensions) must parse with zero diagnostics
   - See: `internal/core/libs/stdlib_conformance_test.go`
   - These files are the **source of truth** for correct parsing

2. **Training examples** - 100 OMG training files (the current result is on the page below)
   - See: `docs/project/training-examples.md`

3. **Golden AST tests** - 33 fixtures with expected AST output
   - See: `internal/core/parser/testdata/parse/`

4. **Negative tests** - 36 test cases for error recovery
   - See: `internal/core/parser/negative_test.go`

## Hand-Written Parser

The parser is hand-written rather than generated because that gives us:
- **Performance** - 10-100x faster than generated parsers
- **Error recovery** - custom `ErrorNode` insertion for fault tolerance
- **Control** - full control over diagnostic messages
- **Incremental parsing** - a path to future LSP support

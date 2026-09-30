# StateMachines library — design

Status: **implemented.** `internal/workspace/libs/stdlib/OpenSysML Libraries/StateMachines.sysml`
is bundled with the other OpenSysML libraries, enters the same conformance and snapshot
gates, and the notation it enables is exercised end to end by the `*_metadata.sysml`
conformance fixtures and by `examples/pseudostates-demo.sysml`.

## The problem

SysML v2 defines no notation for pseudostates — `choice`, `junction`, `history` — or for
event deferral: the grammars admit state usages, transitions and `entry`/`do`/`exit`
subactions and nothing else. OpenSysML long carried keyword spellings of its own for them
(`choice pick;`, `defer Ping;`), which ran but were not conforming notation, so every use
was a `nonstandard-notation` warning and `-strict` an error.

The grammar *does* define the extension mechanism for exactly this case: a usage may carry
a metadata annotation, and a bundled library may declare the metadata definitions. Spelling
a pseudostate as an annotated state usage makes the file ordinary SysML v2.

## Design

`library package StateMachines` declares a `Pseudostate` state-def hierarchy
(`ChoicePseudostate`, `JunctionPseudostate`, `ShallowHistoryPseudostate`,
`DeepHistoryPseudostate`), abstract usages (`choicePseudostates`, `junctionPseudostates`,
`shallowHistoryPseudostates`, `deepHistoryPseudostates`, `deferredEvents`) and one
`SemanticMetadata` definition per concept — `ChoiceMetadata`, `JunctionMetadata`,
`ShallowHistoryMetadata`, `DeepHistoryMetadata`, `DeferredMetadata` — each rebinding
`baseType` to its abstract usage and `annotatedElement` to `SysML::StateUsage`
(`SysML::ReferenceUsage` for `DeferredMetadata`).

A model writes `private import StateMachines::*;` and then, inside a state or
state-machine body:

```sysml
#choice state pick;
#junction state j;
#shallowHistory state h;
#deepHistory state h;
#deferred ref : Ping;
```

Detection in lowering (`internal/ir/lower/state_metadata.go`) is by the resolved
annotation type, not the literal `#choice` text: `semantics.MetadataAnnotationsOf` plus
resolution against the `StateMachines::*Metadata` symbols, so aliases and qualified
spellings (`#StateMachines::choice state pick;`) work and an unresolved annotation is an
ordinary usage whose reference the resolver reports. An annotated state usage synthesizes
the same `*ast.PseudostateNode` the keyword form produces, and a `#deferred` ref appends to
the state's deferred triggers — signal events by a signal type, call events by an action
(definition or usage), carrying no argument bindings — so inheritance, materialization and
the runtime are unchanged.

The keyword spellings keep parsing and lower identically; they are deprecated — the
warning names the replacement and a quick-fix performs it — and the SysML v1 migrator
emits the metadata form (qualified `#StateMachines::<kind>`, so a member named `choice`
cannot shadow the annotation).

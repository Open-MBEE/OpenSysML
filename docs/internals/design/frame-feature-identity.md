# Frame feature identity

## The problem

A performance's values live in frames keyed by simple name: a `bodyCells` for the
body's declared bindings, a `map[string]Value` per block of locals, the data map of
an action node's pins, a state machine's attribute frames. A body writes and reads
by simple name, the REPL and the gRPC surface print values by simple name, and a
snapshot of a frame is a copy of those maps. That storage is correct only while
every name a body can spell denotes at most one feature among the frames in its
reach.

KerML name resolution (KerML 1.0 §8.2.3.5) does not work that way. A name resolves
to one membership: the nearest declaration in the namespace the expression is
written in, then outward through the enclosing namespaces, with inherited and
imported memberships of each (§8.2.3.5.3 local and visible resolution, §8.2.3.5.4
full resolution). Two distinct features in a performance's reach may share a
spelling without either hiding the other for resolution:

- a state definition's attribute and a same-named attribute of a substate, read by
  a transition the definition declares (the substate is not an enclosing namespace
  of the transition, so its attribute is not in reach, however active the state);
- a body attribute and an accept node's payload of the same name (the payload is a
  parameter of the accept node; the body's own declaration is nearer);
- a feature written under its declared name and read under its short name
  (`attribute <sx> x`, KerML 1.0 §7.3.4.5), which names the same feature.

The nested cases — a nested node's attribute, a block-local declaration, a calc
parameter beside a body attribute, an inherited feature redefined by the
specializing definition — were already correct, because each is a frame pushed over
the enclosing ones and the name walk met the nearer frame first.

## The fix

Lowering is the single source of truth for what a body declares, and it knows the
declaration: `lower.Attribute`, `lower.Declare` and `lower.Accept` carry the
`*symbols.Symbol` of the feature they declare (`Symbol`, `Payload`), resolved where
the declaration is lowered (`lower/action_graph.go` `lowerAttributes`,
`lower/calc_body.go`, `lower/state_graph.go`, `lower/state_inheritance.go`,
`lower/action_subflow.go` `acceptPayloadSymbol`). Executors do not re-resolve
declarations.

Each frame keeps, beside its name-keyed values, an index from feature to the name
it binds that feature under (`frame.held`, `actionFrame.declared`, the
`held` stack of `stmtEnv`, the attribute frames of `StateExecutor`), filled by
`Context.holdFeature` — which also indexes every feature the declared one
redefines, by clause or implicitly (a step's parameter redefines its behavior's at
the same position: a nested `action step : Walk { in depth = depth - 1; }` holds
`Walk::depth` too, so a recursion's inner performance answers for its own level), so
a body written against the redefined name reads the redefining feature
(`runtime/feature_identity.go`).

A read of a simple name resolves it first, in the scope the expression was written
in, and reads the innermost frame holding the feature it resolved to
(`EvalContext.lookupHeld`, called from `EvalContext.evalName`). The name walk over
the frames remains, and answers in two cases:

- the name resolves to nothing any frame holds (a calc parameter bound by position,
  a library feature, a feature of the bound instance);
- a nearer frame binds the name without recording the feature it is — a binding
  resolution has no sight of, such as the pins a node adopts from the action an
  invocation expression names (`action scaled = Scale(x = 7)`), which are in reach of
  the body by the runtime's established contract but are not members of the node for
  resolution. `Context.holdResolved` indexes a callee's pin only where the node's
  scope resolves the pin's name to it, so a pin resolution would not have found stays
  unheld and `frame.bindsUnheld` defers to the name walk.

A frame holding the feature answers even when it has no value for it. A performance
records the attributes it declares with no value (`actionFrame.declareUnvalued`, from
the lowered declarations at `newRootFrame` and `seedPerformance`), and `frame.read`
answers a read of one with `NoValueError` until a write binds it, so a nested
`attribute x : Integer;` masks an enclosing `x = 4` instead of reading it. A binding
connector probing its other end treats that error as the end holding nothing yet
(`performances.otherEndHeld`, via `unheldEnd`), as it does an unperformed node's pin.

Simple names are therefore a lookup and display index scoped to the frame declaring
them; identity is the resolved feature.

## A duplicated inherited name

A feature a specialization declares with the simple name of an inherited feature,
without redefining it (`action def Derived :> Base { attribute go : Boolean = false; }`
over `Base`'s `attribute go : Boolean = true;`), hides the inherited one for
resolution in `Derived`'s namespace only (KerML 1.0 §8.2.3.5.3); a guard or
assignment written in `Base` still resolves `go` to `Base::go`. A performance of
`Derived` therefore holds both features. The visible one keeps the simple name;
the hidden one is keyed by its owner-qualified name, `Base::go`
(`lower.ShadowedKey`), which no simple name spells, so the two never share a cell:

- `lower.Attribute.Shadowed` marks it; a state machine keeps it in
  `StateGraph.keptAttributes` and a merged typed node body in `lowerFeatures`,
  dropping it only where a `:>>` clause of the nearer feature redefines it;
- an action performance appends the hidden features of its non-library generals
  (`Context.shadowedInherited`, over `AllSupertypes` and `MembersOf`) to
  `performanceFeatures` and holds them in `addFeatureDirections`, so `lookupHeld`
  finds the cell of whichever feature a name resolved to;
- a succession's guard is evaluated in the scope the succession was declared in
  (`ActionGraph.DeclScope`, from `declaredIn`), in `enabledSuccessions`, the
  decision pass and `probeGuard`, and the SMT encoding translates it there too;
- an assignment written in another scope than the performance's resolves its target
  first (`heldKey`), so an inherited `assign go := false` writes `Base::go`.

The hidden feature is the performance's alone: a performance occurrence's slots
are keyed by name and hold the visible one, so `declaresAttribute` and the
occurrence seeding pass over it. A write to it is checked against its own
declaration (`declarationHeldUnder`: the feature whose key it is, not one it
redefines), and one declaration reached along two generalization paths is held
once. Results, traces and the gRPC feature values list it under its key beside
the visible feature.

## What stays the same

- The maps the REPL, `%trace`, the gRPC feature-value serialization and the trace
  goldens read are the same name-keyed maps; the held index is metadata beside them.
- Snapshots copy the held index by reference: it is fixed when a frame is built
  (`frame.snapshot`, `blockFrame.clone`, `loopFrame.clone`, the paused-engine clone of
  `stmtEnv`), so rollback for `explore` and `check` and replay need no new state.
- Aliases (`frame.aliases`, `actionFrame.aliases`) still canonicalize a redefined
  name to the redefining one for writes and for `node.pin` reads.
- The check's canonical state is spelled from the same values; the reduction
  corpus's `por_alias` pin moved because its short-name reads now observe the
  written value rather than the declaration's default, so the
  interleavings reach more distinct states (`TestCheckReductionIsSound` still holds).

## Invariant

For every frame `f` and feature `s` with `f.held[s] == n`, `f` binds `n` and the
value under `n` is the value of `s`. For every name `n` a frame binds without an
entry in `held` mapping to `n`, the binding is one resolution cannot see, and the
name walk's nearest-frame rule is the contract for it.

## Tests

`conformance/frame_scoping_*` (fifteen cases, with trace goldens for the accept
payload and the transition), `robustness_frame_scoping_test.go`
`TestRuntimeRobustnessFrameScoping`, `robustness_shadowed_inherited_feature_test.go`
`TestRuntimeRobustnessShadowedInheritedFeature`, and the reduction ratchet
`testdata/check/reduction_expected.txt`.

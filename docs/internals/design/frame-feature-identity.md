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
redefines, so a body written against the redefined name reads the redefining
feature (`runtime/feature_identity.go`).

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

Simple names are therefore a lookup and display index scoped to the frame declaring
them; identity is the resolved feature.

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

`conformance/frame_scoping_*` (eight cases, with trace goldens for the accept
payload and the transition), `robustness_frame_scoping_test.go`
`TestRuntimeRobustnessFrameScoping`, and the reduction ratchet
`testdata/check/reduction_expected.txt`.

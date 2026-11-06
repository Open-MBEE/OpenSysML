## Current Implementation Status

Loop and branch bodies that state their own token flow are lowered by
`lower/block_graph.go:lowerStatedBlock` and run as transparent, per-iteration
performances by `runtime/action_statements.go:performBlockFlow` through
`runtime/action_subflow.go:runSubflow`. Their declared attributes are seeded for
each performance, and `terminate;` ends the enclosing action node. An `accept` in a
declaration-order body is a node of the body's flow and parks it (see
[Accept suspension across call chains and block bodies](#accept-suspension-across-call-chains-and-block-bodies)).
The remaining typed refusals are stated flows in calculation bodies and body
succession probability metadata when lowering has no resolver.

### Implemented and Tested

<!-- doc-counts:begin census -->
The map below tracks every rule row on this page. Its census — how many rows are ✅ faithful, 🧩 extension, ⚠️ approximate, ❌ not implemented or ⛔ deliberate divergence, and how many have no external referee — is counted from the rows when the documentation site is built and published at [runtime.opensysml.org](https://runtime.opensysml.org/project/spec-compliance/); no count is typed in or committed here.
<!-- doc-counts:end census -->
Read that as progress, not as a compliance percentage — the denominator is the list of rules *we*
chose to track, so it moves when we add a row, and a specification-derived denominator does not
exist. Only the ✅ rows claim SysML v2 / KerML semantics; a 🧩 row is implemented against UML 2.5.1,
PSSM or fUML, or is an OpenSysML extension, and claims nothing about SysML v2. Every behavioral
row — the Action, State Machine and Classifier Behaviors maps — is self-assessed, whatever its
status, because no external referee executes behavior. What is externally checked is enumerated in [the pilot differential](pilot-differential.md);
what cannot be checked by anything is in
[What Can't Be Claimed for Spec Compliance](#what-cant-be-claimed-for-spec-compliance).

**Calculations (16/16 features):**
- Invocation with typed parameters
- Result evaluation (both the body's trailing expression and a bound return parameter `return : T = <expr>;`)
- Parameter binding (positional + named arguments)
- Parameter defaults (own and inherited)
- Inherited parameters and result through a typed calc usage, including redeclaration
- Nested calc invocation, and invocation from a constraint
- Statement bodies: local declarations, assignment, `if`/`else`, `while`, `loop … until`, `for`, and early `return`
- Purity and termination: a side effect or an outside assignment is rejected, and every loop iteration spends a step of the budget
- Control flow (if/else), including the conditional expression `if c ? a else b` evaluated lazily at runtime
- Unary operators (not, -, +)
- Type coercion (Integer→Real)
- Qualified names (A::B::C)
- Deterministic evaluation trace (parameter binding, sub-expression order, results)
- Error handling (unbound/unknown parameters, arity, missing return, recursion and step budgets)
- Calc usages as multi-output consumers: a usage's `out` features resolve, typecheck and evaluate as features (`attribute z = c.b;`), from one run of the body per usage per object
- Composition of multi-output calcs: a usage nested among a calc def's members binds its inputs from the enclosing evaluation's parameters and locals, one run per usage per object per bound input tuple

**Constraints (7/7 features):**
- Assert evaluation (boolean satisfaction)
- Assume evaluation (a constraint's assumption decides nothing; a requirement's assumptions imply its required conditions)
- Bare expression as invariant
- Negated constraints (assert not)
- Unresolved feature detection
- Conditions of a nested constraint (`assert constraint [name] { <expr> }`)
- Parameters a typed usage binds (`constraint limit : MassLimit { in m = mass; }`)

**Requirements (9/9 features):**
- Require expression evaluation, in a requirement definition body as well as a usage
- Conditions stated through an anonymous nested constraint (`require constraint { <expr> }`)
- The requirement's own attributes, inherited or rebound, in its conditions
- Subject binding evaluation
- Actor binding evaluation  
- Assume expression evaluation, in both spellings: `allTrue(assumptions()) implies allTrue(constraints())`
- Nested requirements
- A required requirement's parameters bound by the reference's body (`require r { in w = unit; }`)
- A violated condition names the condition that failed

**Actions (21/21 features):**
- Initial/final node token placement
- Exact finite multiplicity on action-node usages, including unordered (concurrent-start) subactions, zero, loop and branch bodies, control-node adjacency, guarded successions into a written target end, feature reads, bindings and part-level performs; bounds beyond the 64-bit range are refused with their exact value (for example, `1180591620717411303424`)
- Fork node (1→N parallelism)
- Join node (N→1 synchronization)
- Merge node (N→1 non-blocking)
- Decision node (guarded branching)
- Action execution nodes
- Nested action invocation
- Assignment statements in an action node's body
- Conditional statement (`if <cond> { … } else { … }`), nestable in either direction with a loop
- Pre-condition loop (`while <cond> { … }`) and post-condition loop (`loop { … } until <cond>;`)
- Iteration over a collection (`for <x> in <collection> { … }`, over every collection the expression layer produces; a non-collection input is reported)
- Send statement (⚠️ typed messages addressed to an object's port or receiving node, or routed through a connected port)
- Accept action (⚠️ takes the oldest message of its type, parking its token until one arrives; suspension is bounded by the executor — see the Action map)
- Occurrence messages (`%occurrence`, `Context.ExecuteOccurrence`: an occurrence def's `message`s between the `event occurrence`s of its lifeline usages are realized as transfers in the order the `then` successions admit, a receiving event's `perform` performed on the lifeline object — see the Action map)
- Nested `accept`, `send`, `assign`, `if`/`else`, `while`, `loop`, `for`, and `terminate` action-body items; an empty `else_body` has no branch, while empty then/loop bodies remain explicit blocks (SysML.xtext:1607 ActionBodyParameter, 1442 AcceptNode, 1499 SendNode, 1535 AssignmentNode, 1596 IfNode, 1615 WhileLoopNode, 1624 ForLoopNode, 1641 TerminateNode; formal/2026-03-02). Accepts in nested loop/branch bodies remain a pre-existing runtime limitation.
- Object flow (pin-to-pin data), streaming by default and completion-triggered as a `succession flow`
- Succession edges
- Deadlock detection
- Token-flow tracing (infrastructure ready)
- Step budget enforcement

**State Machines (core: faithful; advanced: partial):**
- Initial state identification and completion on a transition reaching `done`
- State entry/exit actions
- State do behavior (runs while the state is active; concurrently active states interleave)
- Transition firing
- Transition guard evaluation
- Transition effect actions
- AcceptEvent triggers (when signal)
- Sourceless transitions (`accept … then`, `if … then`, `then`): the source is the state declared before them in the same body
- ChangeEvent triggers (when expression)
- TimeEvent triggers (`after` duration, `at` instant)
- Signal discrimination (name matching)
- Unmatched signal dropped
- Signals sent from entry/do/exit/effect actions reaching the machine
- CallEvent triggers (`accept op(arg)` notation, operation and argument matching)
- Completion transitions (nil trigger with guard evaluation)
- Hierarchical substates
- Orthogonal regions (concurrent states)
- Choice pseudostates (dynamic branching)
- Junction pseudostates (static branching)
- Fork pseudostates (one branch per orthogonal region)
- Join pseudostates (arrival-based synchronization)
- Choice/junction reached from inside an orthogonal region
- Nested action invocation in entry/do/exit/effect behaviors
- Run-to-completion semantics
- Event queue management
- Dangling transition detection (a transition names one source and one target vertex of its own machine; a routing pseudostate with no transition out of it reports)
- State visits tracking
- Multi-region event broadcasting
- History pseudostates: shallow and deep restoration (`history` / `shallow history` / `deep history <name>;`, or the `StateMachines` library's `#shallowHistory state` / `#deepHistory state` metadata spellings)
- Deferred signals in the standard buffer / accept-loop / exit-flush encoding, across hierarchy and orthogonal regions

**Expression Evaluation:**
- Binary operators (+, -, *, /, <, >, ==, and, or)
- Exponentiation (`**`, `^`) over Integer and Real operands, folded and evaluated by one implementation
- Unary operators (-, not)
- Literal values (Integer, Real, Boolean, String)
- Feature reference resolution
- Qualified name resolution (A::B::C)
- Type coercion (Integer→Real)
- Unresolved reference error handling
- KerML function library: the numeric functions of `RealFunctions`, `RationalFunctions`, `NumericalFunctions`, `IntegerFunctions`, `NaturalFunctions`, `TrigFunctions`, `VectorFunctions` and `ComplexFunctions`, all of `StringFunctions`, plus the library feature values `TrigFunctions::pi` and `ComplexFunctions::i` (see the Function Library row below)

**Name Resolution:**
- Inherited feature resolution (follows specialization chains)
- Named argument parameter binding
- Redefinition target resolution (:>> featureName)
- Control flow node scope registration

**Test Coverage:** counted from the tree when the documentation site is built and published at [runtime.opensysml.org](https://runtime.opensysml.org/project/spec-compliance/); no figure is typed in or committed here, so a branch adding a test or fixture leaves this page as it is.

- Execution conformance: <!-- doc-counts:begin inventory-conformance -->the execution conformance cases, by prefix<!-- doc-counts:end inventory-conformance --> — the calc cases include the fixed-step RK4 lunar descent whose stages are body-local usages read over a range, the one-binding output case, the library complex/vector/trig cases, and recursion — factorial, fibonacci, a descent over a sequence, a mutually recursive pair and one whose result is its last expression; the action and state cases include a decision and a transition guard reading a calc usage; the new `f62_send_body_payload`, `f62_transition_body_dotted_target`, `f63_control_node_body`, `f63_for_typed_variable`, and `f63_merge_body_runs_on_traversal` fixtures cover node bodies, dotted transition targets, typed `for` variables, and merge traversal, and the `action_merge_loop_reenters`, `action_merge_loop_three_passes`, `action_merge_fork_branch_and_loop` and `action_merge_body_flips_own_guard` fixtures a loop re-entering a merge, a merge fed by a fork and a loop at once, and a merge body write read by the merge's own outgoing guard; the `assign_chain_*` fixtures write through a feature chain at depth two and three, through a port, through an inherited feature, through two features holding one occurrence, from a state's entry, `do` and `exit` behaviors and from a transition effect, with the write read back by a later node and by a guard, and a calculation body's chained target rejected; the `assign_write_*` fixtures write a subtype value and a widening numeric value legally, and refuse a wrong-typed write, a wrong-typed chained write, a collection the target's multiplicity cannot hold and an empty write where one value is required; the `instance_quantity_coherent_units`, `calc_quantity_coherent_result` and `calc_quantity_coherent_mismatch` fixtures report a product, quotient, fractional power, prefixed input and user-declared derived unit in the coherent unit of the declared quantity kind, keep a dimensionless ratio a number and a non-numeric exponent an error, and keep the dimension mismatch of a speed written to an acceleration)
- Runtime robustness: <!-- doc-counts:begin inventory-robustness -->the runtime robustness cases (first-level subtests across the `TestRuntimeRobustness*` functions)<!-- doc-counts:end inventory-robustness -->, among them: a write of a wrong-typed value, of too many values and of none where one is required, each leaving the feature as it was, and such a write from a state entry behavior, through a feature chain, to a calc output, to a body-local, to an output, to a performer feature and to a performance occurrence; a write to a constant or a derived feature, directly, through a chain, through a redefinition and through `SetFeatureValue`, each refused and leaving the value; deadlock, an accept payload read by a node that runs before the accept binds it, a default whose element count does not conform to its feature's multiplicity, a calc output the body never assigns or only a branch that did not run would assign, an output bound both by its declaration and by an assignment or by two assignments, empty entry/do/exit bodies, a do body that never finishes, cyclic specialization, a redefinition naming a missing step, an inherited succession targeting an incompatible replacement, ambiguous inherited redefinitions, an assignment to a qualified target, a chained assignment target whose final segment the object reached does not hold, whose step is not an object, holds several objects or holds no value, whose base the body cannot reach, whose value the target's multiplicity refuses, or that is written in a calculation body, a body-local usage typed by something that is not a calc, a body-local declaration with no execution, a range bound that is not an Integer, a range spending the step budget, a collection spending the element budget, a chain through a part stopping at a calc usage, an index naming no position, a collection operand of the wrong kind, a collection body of the wrong arity, a `select` predicate that is not a condition, a collection operation spending the step budget, a non-terminating loop, a calc usage leaving an input unbound, reading an output it does not declare or one with no value, a usage nested in a calc leaving an input unbound, reading an output it does not declare, an input default naming only itself, a nested usage chain reaching the recursion limit or spending the step budget, outputs valued from each other, a usage typed by something that is not a calc, a usage body spending the step budget, an invocation of a calc that computes several outputs and designates no result, a non-terminating calc loop, a calc body that never returns, a send or a `terminate` inside a calc, an assignment outside a calc body, a non-Boolean calc condition, a body-local declaration that must not leak, a body member that is not executable, accept suspension that can never end, an accept standing as a statement of a loop body that nothing can end, guards, budgets, sourceless accept, fork/join misuse, pseudostate dead ends and cycles, non-numeric time trigger, a time trigger argument of the type validation refuses, misaddressed send, accept of an unsent type, send through an unconnected port, history misuse, non-deferrable deferred trigger, non-terminating do behavior, calc binding/arity/recursion failures, unhandled call, call argument of the wrong type, missing and cyclic `perform` references, a library function outside its domain or with the wrong arity, an extension library function outside its domain, an Integer result beyond the Integer size budget, an Integer beyond int64 used as an index, offset or size, a flow end that names no action node, a flow from a node that produced no value, an action accept waiting on the clock — `after` and `at` fired by advancing it, `at` an instant already past fired at once, a negative `after`, a duration of another dimension — and the clock advanced by zero, by a negative amount, with nothing waiting, past a wait that stays queued and into a machine the event budget stops, a non-Boolean change trigger, a variation with no variant selected, a selection that is not one of a variation's variants, two variants selected at once, a variation read through its declaration, a chain through an unselected variation part, two variation points selecting one variant without an owning object, a `variant` declared outside a variation, a variant under a redefined variation, a deep chain of redefinitions, conflicting redefinitions at several levels, one feature valued under two of its names, a feature both valued and restated in a body, a flow that names no feature to carry, an accepted message carrying no single value to bind, a transition that names no target, a transition endpoint that names nothing, a transition endpoint naming a state of a different machine, a transition endpoint lowered with no name-resolution pass, whose edge is left out, a connector end naming no reachable feature, a connector of several objects with an unreachable end, a connector attached to itself or to one that names it back, a write into a pair of values derived from each other)
- Runtime tests: <!-- doc-counts:begin inventory-runtime-tests -->the runtime test functions (the top-level tests `go test -v ./internal/exec/runtime ./internal/exec/sweep` reports)<!-- doc-counts:end inventory-runtime-tests -->, the conformance, trace and robustness gates above among them
- Golden ASTs: <!-- doc-counts:begin inventory-golden-asts -->the golden AST fixtures (SysML and KerML)<!-- doc-counts:end inventory-golden-asts -->, including the implicitly typed connector forms and the standard behavioral notation — a named flow with `from`, accept trigger expressions, an accept subsetting an event, sends, a succession to a loop with `until`, `then done`, a decision `else` branch, a bodied `exhibit state`, a transition with its trigger on its own line, a qualified namespace-level succession — body-local calc usages and ranges, the three loop forms, pseudostate, timed-trigger, call-trigger, calc default/invocation, calc statement bodies, n-ary connector-end parsing, prefix metadata, keyword-less members, node bodies and dotted transition targets, a chained assignment target, and occurrence typing)
- Golden traces: <!-- doc-counts:begin inventory-traces -->the golden execution traces under the default schedule, by prefix, and the `.trace.golden` files pinning a case under a named policy<!-- doc-counts:end inventory-traces --> — entry/do/exit ordering of inline action bodies and a do body run to its end inside one round, the standard loop `until` with `then done`, a decision's guarded and `else` branches, a named flow carrying a value between action nodes, an accept with a `when` trigger, an accept subsetting an event, a send invocation through a port, a transition accepting through a port, loop and conditional bodies, one calc usage body run feeding several output reads, a usage whose outputs are read either side of an assignment to what its input named, a usage nested in a calc read for two of its outputs, calc statement bodies and their loop iterations, fork/join branch ordering, region entry/exit ordering, do behavior interleaving across orthogonal regions, send/accept, an accept parked until its message arrives, a payload read by a node declared before the accept that binds it, calc and constraint evaluation, library function invocation, the dotted-target transition, control-node and merge-body traces, and the merge loops re-entered on every pass)
- Negative parser tests: <!-- doc-counts:begin inventory-negatives -->the negative parser subtests (first-level subtests of `TestNegative`, across the `TestNegative*` functions, and across every `*Negative*` parser test)<!-- doc-counts:end inventory-negatives -->
- gRPC: <!-- doc-counts:begin inventory-grpc -->the gRPC conformance cases and gRPC robustness cases (first-level subtests across the `TestGRPCRobustness*` functions)<!-- doc-counts:end inventory-grpc --> (`tests/grpc/testdata/conformance/`, `internal/frontend/grpc/robustness*_test.go`)
- Test functions: <!-- doc-counts:begin inventory-tests -->the top-level `Test` functions across the module<!-- doc-counts:end inventory-tests --> (`go test -count=1 ./...` runs them all, with the OMG corpora downloaded, `OPENSYSML_REQUIRE_TRAINING_CORPUS=1 OPENSYSML_REQUIRE_PILOT_CORPORA=1 OPENSYSML_REQUIRE_SMT=1` and z3 installed). The figures on this list are counted from the tree by `tools/cmd/doc-counts` when the site is built, and `go run -C tools ./cmd/doc-counts -check` refuses one typed in; the test and subtest total of a run is not stated, since only a run can state it. A test skips only where it says why: TestHeldImageRoundTrip declines a conformance case that creates no instance, so there is no held image to round-trip. Three skip themselves: TestSubsettingTargetIsTheInheritedFeature and TestRequirementEvaluation_SubjectNotFound against a limitation they record, and TestHelperSolverProcess, which is a solver child process the parent invokes. The others skip for want of something the run did not provide, and each names it: the `weasyprint`, `pandoc` and `prince` subtests of TestRenderWithInstalledEngines and TestRenderInlineRunsWithInstalledEngines and TestRenderDiagramsWithInstalledMermaid want the PDF and Mermaid toolchain, TestExtractionMatchesBaseline and TestUpdateIsIdempotentAcrossDays the pinned pilot validator jar, TestEmitSuite, TestRefereeRowsAreWellFormed, TestSuiteRead and TestSuiteClassification the downloaded PSSM test suite, TestCRealNotationIsLocaleIndependent a non-C locale, TestRenderDocumentsRejectsCaseAliasedTargets a case-insensitive filesystem, and TestFlexoInterop and TestFlexoInteropApply a live Flexo stack.

---

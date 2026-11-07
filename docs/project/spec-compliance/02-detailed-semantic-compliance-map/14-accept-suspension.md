### Accept suspension across call chains and block bodies

What the specification requires, from KerML 1.0 and SysML v2.0 and the Kernel Semantic and
Systems libraries vendored under `internal/workspace/libs/stdlib`:

1. **An accept is a performance that ends only after a transfer reaches it.** An
   `AcceptActionUsage` is typed by `Actions::AcceptAction`, which specializes
   `AcceptMessageAction :> Action, AcceptPerformance` (SysML v2.1 §7.17 Actions; `Systems
   Library/Actions.sysml`). `Transfers::AcceptPerformance` (KerML 1.0 Kernel Semantic Library,
   `Transfers.kerml`) has `feature acceptedTransfer: MessageTransfer[1] subsets
   receiver.incomingTransfersToSelf` and `succession acceptedTransfer then self.endShot`, so the
   performance does not end until a transfer to its receiver exists. Nothing in either library
   bounds when that transfer may arrive or which occurrence sends it: `SendPerformance::sender`
   only defaults to `this`, and the transfer may be posted by any occurrence, by a time trigger
   (`accept after`, `accept at`) or from outside the model.
2. **A performance waits for its subperformances.** `Performances::Performance` declares
   `composite step subperformances: Performance[0..*] subsets enclosedPerformances,
   suboccurrences`, and `enclosedPerformances` subsets `timeEnclosedOccurrences`
   (`Performances.kerml`), so an enclosed performance happens during its encloser. An action's
   nodes and the actions it performs are `Action::subactions :> subperformances`
   (`Actions.sysml`), whose `this` is the owning action's. A caller that performs an action
   whose accept has not ended therefore cannot end either: the whole chain of performances is
   suspended, and resumes when the innermost accept takes its transfer. Synchronous invocation
   is an implementation strategy, not a semantic boundary.
3. **An accept in a loop or branch body is an ordinary subperformance.** `Actions::LoopAction`
   has `in action body[0..*]`, and `ControlPerformances::LoopPerformance` binds
   `whileDecision.thenClause = body` with `succession body then untilDecision`; an
   `IfThenPerformance`'s `thenClause` and an `IfElsePerformance`'s `elseClause` occur only after
   the test (`ControlPerformances.kerml`). A body, or a clause, holding an accept therefore does
   not end, and the next iteration or the statement after the `if` does not start, until the
   accept takes a transfer. No library text exempts these bodies from accepting.

Where the specification is silent, and what the tool decides:

- **Deadlock.** Neither KerML nor SysML define a run, a scheduler, or a deadlock; they only say
  the accept cannot end without a transfer. Reporting `ErrAcceptDeadlock` is tool-defined: a run
  driven to completion reports it only when every remaining token is parked and nothing in the
  run — no runnable token, no wait on the clock, no other object's behavior, no injection
  surface still open — can post a transfer the parked accept takes. A run that can still be
  resumed (`RunToQuiescence`, a stepped `%action` session, an object's performed action) waits
  instead.
- **Order of statements in a block without successions** stays the existing declaration-order
  policy of these bodies (see the block-flow row of the Action table); the accept is one node of
  that order. An accept written directly among a leaf action's members is not such a block: it
  is one of the action's unordered subactions (the unordered-subactions row), so what reads its
  payload is ordered after it only by a succession (`then`).
- **Which accept takes a transfer, and when a woken chain runs relative to its siblings**, are
  unchanged: routing and matching are `Context.messageReaches`/`acceptMatch`, and the order of
  resumption is a scheduling choice the explorer and checker enumerate.
- **Entry, exit and transition-effect behaviors** keep their existing rule — performed whole at
  the instant they are triggered (`StatePerformances.kerml`: `entry then middle`;
  `TransitionPerformances.kerml`: `transitionLinkSource then effect`) — and calculation bodies
  remain unable to accept (a `Function` is an `Evaluation`, not a performance that receives
  transfers); neither is changed here.

UML, fUML and PSSM run-to-completion semantics were consulted as advisory only and decide
nothing above.

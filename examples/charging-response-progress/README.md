# Checking that a charging request gets an answer

This synthetic design accepts one charge request. It may energize under a
recognized lock, or explicitly reject. Both are legitimate outcomes. The
requirement is that **every schedule** reaches one of them after acceptance.

`station.sysml` always resolves. `retry.sysml` also allows a housekeeping
retry that returns to the same pending decision. It can take this branch
forever. The resolving branches and their terminal observations are unchanged.

## Stock check

From the repository root, run this command for each model:

```sh
./bin/sysml -no-record-cache -action ChargingProgress::serve -engine check \
  -check-diverge pending -check-depth 100 -check-states 200 -json \
  examples/charging-response-progress/station.sysml
```

The reference stock run reports exhaustive, no violation, no bound hit, and the
same two terminal outcomes in either model. Only asking about divergence in
`pending` allows the intentionally different final responses without treating
them as defects. The two models have different numbers of reachable states and
moves. The exhaustive label does not assert that every run terminates.

## Progress prototype

```sh
go test -v ./internal/exec/runtime -run 'Test(CheckProgress|RuntimeRobustnessProgress)'
```

The local Go runtime prototype's `CheckProgress` explores the unreduced native
stable-state graph. It checks that an explicit model-authored obligation
predicate leaves the pending region on every schedule. The monitor must be
false before acceptance and must only clear after a legitimate response. It
needs to be a deterministic function of the canonical checkpoint state alone,
not of the path or runtime identities used to reach it.
The checker detects a reachable pending terminal or a repeatable pending cycle,
even when a resolving exit remains enabled. It separates absent activation and
incomplete search from success. An accepted request can be observed pending
without a bug; flagging that state as an invariant violation would be incorrect.

The test re-executes the finite witness prefix and two iterations of the
cycle with the native scheduler and checks that every cycle checkpoint still
holds the obligation and that the canonical state recurs. This is not an
infinite interpreter run, nor a claim about scheduler fairness. Explicit
clock horizons and positive time advancement during search after invocation
startup are unsupported. The model uses one
outstanding request with a model-authored monitor, not a general temporal
language, hardware control or interoperability certification. No new CLI
option is proposed by this prototype.

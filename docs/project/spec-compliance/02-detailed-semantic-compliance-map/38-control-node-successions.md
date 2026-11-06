### Control-Node Successions (SysML v2 §7.17.3 Control Nodes, §8.3.17 `ControlNode`, `DecisionNode`, `ForkNode`, `JoinNode`, `MergeNode`)

The nine validation constraints on the successions of a control node, refereed against the
specification text: the pinned pilot (`2026-09`) implements only `validateControlNodeOwningType`,
and is silent on the other eight (adjudicated as pilot gaps in
[pilot-differential.md](pilot-differential.md), drafted for upstream in
[omg-issues.md](omg-issues.md)). A succession is any `Succession` the action declares or
inherits — a `succession` usage, `first a then b;`, a member-attached `then b;`, and a guarded or
default branch out of a decision (`if g then b;` / `else b;` — a `TransitionUsage` whose owned
`Succession` has the decision as its `sourceFeature`, §8.3.17 `checkTransitionUsageSuccessionSourceSpecialization`);
a `connect`, `bind`, or `flow` is not one. The count and end-multiplicity rules
hold in the abstract syntax even where the notation omits the multiplicities (§7.17.3), so an end
that writes none is taken to carry the required one and only a written multiplicity is judged.
`ControlNodeSuccessionPass` (`passes/behavior/control_node.go`) reports a violation once, at the control
node when the checked action declares it, else at the succession that action adds; a violation
inherited whole is reported at the definition it comes from. A `succession flow` is a Succession
as well as a Flow (`SuccessionFlowUsage`), so it counts too: the parser keeps the prefix
(`ast.Usage.IsSuccessionFlow`), and each end relates the feature its dot notation names ahead of
the payload (`from a.out to f.in` runs from `a` to `f`); an end written without one relates nothing
here and is the flow-end rule's to report; a flow end writes no multiplicity (SysML.xtext
`FlowEnd`), so only the count rules reach it. The runtime carries the kind on from the lowered
`ObjectFlow.Kind`: a `succession flow` moves its value as the source completes, a plain `flow`
streams each write (see the Action map's object-flow rows).

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| `validateControlNodeOwningType` — the `owningType` of a `ControlNode` must be an `ActionDefinition` or `ActionUsage`; a constraint body is a calculation body (SysML.xtext `CalculationBody`) and so admits an action node the rule then rejects | `passes/behavior/control_node.go` `controlNodeChecker.checkOwner` (`control-node-owner`), `semantics/action_succession.go` `ActionDeclaration`; `parser/behavior.go` `parseConstraintBody` routes action nodes through `parseActionMember` | `passes/behavior/control_node_test.go:TestControlNodeOwningType` (a fork in an occurrence definition, a decision in a succession body, a join in a constraint definition, a merge in a constraint usage), `:TestControlNodeInEveryActionBodyIsSilent` (nested actions, loop and branch bodies, a control node's own body, entry actions, calculations, cases, `perform action`); `tests/parser/testdata/parse/constraint_control_node.sysml`; `tools/referee/reject/testdata/negative/semantic/cn05` (both reject) | ✅ Faithful (matched reference run: both report the fork at `4:9` and the decision at `8:13`) |
| `validateControlNodeIncomingSuccessions` — every `Succession` into a `ControlNode` has target multiplicity `1..1` | `passes/behavior/control_node.go` `controlNodeChecker.check` → `checkEndMultiplicity` (`control-node-incoming-multiplicity`), `semantics/multiplicity.go` `Range.HasBounds` | `passes/behavior/control_node_test.go:TestControlNodeEndMultiplicities`, `:TestControlNodeSpecificationExamplesAreSilent` (the §8.4.13.4 examples with every multiplicity written), `:TestControlNodeUnboundedSideIsSilent`; `semantic/cn06` | ✅ Faithful |
| `validateControlNodeOutgoingSuccessions` — every `Succession` out of a `ControlNode` has source multiplicity `1..1` | `passes/behavior/control_node.go` `checkEndMultiplicity` (`control-node-outgoing-multiplicity`) | `passes/behavior/control_node_test.go:TestControlNodeEndMultiplicities`; `semantic/cn07` | ✅ Faithful |
| `validateForkNodeIncomingSuccessions` — a `ForkNode` has at most one incoming `Succession` | `passes/behavior/control_node.go` `controlNodeChecker.checkCount` (`fork-incoming-successions`), `semantics/action_succession.go` `Model.ActionSuccessions` (declared and inherited, redefinitions masking what they replace) | `passes/behavior/control_node_test.go:TestForkWithTwoIncomingSuccessions` (shorthand `then`, `first … then`, `succession s first … then`, and a mix), `:TestControlNodeInheritedSuccessionsCount`, `:TestControlNodeRedefinedSuccessionReplacesInherited`, `:TestControlNodeInheritedViolationReportedOnce`, `:TestControlNodeOtherConnectorsDoNotCount`, `:TestControlNodeSuccessionFlowsCount` (a `succession flow` counts, a `flow` does not), `:TestControlNodeSuccessionFlowEnds`; `tests/parser/testdata/parse/succession_flow.sysml`; `semantic/cn01` | ✅ Faithful |
| `validateJoinNodeOutgoingSuccessions` — a `JoinNode` has at most one outgoing `Succession` | `passes/behavior/control_node.go` `checkCount` (`join-outgoing-successions`) | `passes/behavior/control_node_test.go:TestJoinWithTwoOutgoingSuccessions`, `:TestControlNodeStaticRuleAgreesWithRuntime` (the static rule reports the graph `runtime/action_executor.go` refuses at `initialize()` as `join node … has multiple successors`; the runtime check is kept), `:TestControlNodeSuccessionFlowsCount`; `semantic/cn02` | ✅ Faithful |
| `validateMergeNodeIncomingSuccessions` — every `Succession` into a `MergeNode` has source multiplicity `0..1` | `passes/behavior/control_node.go` `checkEndMultiplicity` (`merge-incoming-multiplicity`) | `passes/behavior/control_node_test.go:TestMergeIncomingSourceMultiplicity`; `semantic/cn08` | ✅ Faithful |
| `validateMergeNodeOutgoingSuccessions` — a `MergeNode` has at most one outgoing `Succession` | `passes/behavior/control_node.go` `checkCount` (`merge-outgoing-successions`) | `passes/behavior/control_node_test.go:TestMergeWithTwoOutgoingSuccessions`, `:TestControlNodeSuccessionFlowsCount`; `semantic/cn03` | ✅ Faithful |
| `validateDecisionNodeIncomingSuccessions` — a `DecisionNode` has at most one incoming `Succession` | `passes/behavior/control_node.go` `checkCount` (`decision-incoming-successions`) | `passes/behavior/control_node_test.go:TestDecisionWithTwoIncomingSuccessions`, `:TestControlNodeSuccessionFlowsCount`; `semantic/cn04` | ✅ Faithful |
| `validateDecisionNodeOutgoingSuccessions` — every `Succession` out of a `DecisionNode` has target multiplicity `0..1` | `passes/behavior/control_node.go` `checkEndMultiplicity` (`decision-outgoing-multiplicity`) | `passes/behavior/control_node_test.go:TestDecisionOutgoingTargetMultiplicity`; `semantic/cn09` | ✅ Faithful |

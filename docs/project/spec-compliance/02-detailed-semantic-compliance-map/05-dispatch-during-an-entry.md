### Dispatch during an entry the model does not run to completion

When a state entry is not run to completion, the specification leaves open
whether a dispatch due at the same instant happens before the unfinished entry
or after it. Both orders are admissible: dispatching first can leave the
entered composite without visiting its later state, while completing the entry
first can visit that state before the dispatch transitions away. The conformance
fixture `state_run_to_completion_false_self_signal` records both outcomes and
its trace goldens record each permitted order. The scoped fixture
`state_run_to_completion_scope_sibling_region` records the two permitted orders when an entry
boundary in one region overlaps a dispatch owned by its sibling; its default twin
`state_run_to_completion_scope_sibling_region_default` proves that the un-redefined machine
retains its single-step behavior.

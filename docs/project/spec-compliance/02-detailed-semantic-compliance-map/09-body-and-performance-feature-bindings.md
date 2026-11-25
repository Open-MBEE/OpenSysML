### Body and Performance Feature Bindings

The dependency-tracked `=` rule for object features also applies to run-owned
action, calculation, statement-block and state-machine features. A source write
invalidates a tracking cell; its next semantic read derives the value again
through the same dependency graph and declaration checks. `:=` initializes a
writable value once, `default` remains a fallback, and assigning a tracking cell
holds the assigned value and ends its tracking. Supplied action pins and call
arguments are writes. At the end of a performance, a tracking cell derives once
more if it has not materialized and then freezes; this end-of-performance value
is an OpenSysML policy because the language does not specify later reads of an
ended occurrence. State exit invalidates dependents of the exiting state's
attribute cells, so a later read observes the existing inactive-state behavior
rather than a cached value from before exit.
On state re-entry, each frozen unwritten `=` attribute restarts for lazy
rederivation; written values remain held.

The cells are journaled with their owning maps for snapshot/restore, carried in
held images (tracking values restore unmaterialized and rederive lazily), and
spelled in explore/check keys with both their held value or unset state and
their tracking, written or frozen status. Constraint-body declarations already
reevaluate through `evalNameGeneral`/`declaredValue`. A body-local calc usage
lowered as `DeclareUsage` remains one-shot: its input pins are captured on the
first read in an activation and do not track later changes to body `=` values.

Implementation: `runtime/body_bindings.go` (`registerBodyBinding`, `deriveBodyCell`,
`writeBodyCell`, `freezeBodyCells`), `runtime/dependents.go` (`deriveWith`),
`runtime/held_image_behavior.go`, `runtime/snapshot.go`, and
`runtime/check_state.go`. Tests: conformance `action_body_feature_binding`,
`action_body_feature_occurrence_mirror`,
`action_nested_body_feature_binding`, `calc_body_feature_binding`,
`action_block_body_feature_bindings`, `state_body_feature_binding_change_guard`,
`state_attribute_body_feature_binding`, `constraint_body_feature_binding`,
`action_body_feature_initial_binding`, `action_body_feature_written`,
`action_body_feature_read_only`, `state_behavior_body_feature_bindings`,
`action_step_multiplicity_shared_writers`, `action_inherited_default_root`,
`action_inherited_default_root_fallback`, and
`action_inherited_supplied_parameter`; trace
`action_body_feature_binding.trace.golden`; robustness
`TestRuntimeRobustnessBodyFeatureBindings`. Status: ✅ Faithful for action,
calculation, statement-block and state-machine cells; ⚠️ body-local calc usages
remain one-shot.

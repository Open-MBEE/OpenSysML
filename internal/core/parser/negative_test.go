package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// TestNegative verifies parser REJECTS malformed input (Phase 2, Task 2.3)
// Guards against silently accepting garbage
func TestNegative(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"unclosed_brace", "part {"},
		// The reference form of `include` subsets an existing use case, so it names
		// one (SysML.xtext:2300 IncludeUseCaseUsage).
		{"include_without_target", "package P { use case def U { include ; } }"},
		// A direction prefixes the feature it applies to (SysML.xtext:554
		// FeatureDirection).
		{"direction_without_feature_action", "package P { action def A { in ; } }"},
		{"direction_without_feature_part", "package P { part def D { out ; } }"},
		{"empty_requirement", "requirement r { require ; }"},
		{"empty_expression", "attribute x = ;"},
		{"numeric_name", "part def 123;"},
		{"missing_semicolon", "part def Engine"},
		{"invalid_keyword_combo", "def usage MyPart;"},
		{"incomplete_connection", "connector c connect a"},
		{"unterminated_string", `part p { doc /* comment `},
		{"double_colon_only", "attribute ::x;"},

		// Behavioral negatives (Phase B1.2)
		{"state_entry_no_keyword", "state s { entry }"},
		{"action_dangling_fork", "action a { fork }"},
		{"transition_then_only", "transition first then"},
		{"named_final_node", "action a { done finish; }"},
		{"named_final_keyword", "action a { final finish; }"},
		{"two_ended_then", "action a { action start; action finish; then start finish; }"},
		{"state_member_then", "state s { state start; state finish; start then finish; }"},
		{"requirement_empty_require", "requirement r { require }"},
		{"calc_empty_return", "calc c { return }"},
		// A specialization after `return` names what the result subsets.
		{"calc_return_subsets_no_target", "calc def C { return :> = 1; }"},
		{"calc_return_named_subsets_no_target", "calc def C { return r :> = 1; }"},
		{"calc_return_short_name_unclosed", "calc def C { return <r result : Real = 1; }"},
		{"calc_return_short_name_empty", "calc def C { return <> = 1; }"},
		// A result declares something: a value or a body alone is no FeatureDeclaration.
		{"calc_return_value_only", "calc def C { in n : Real; return = n; }"},
		{"calc_return_body_only", "calc def C { return { doc /* r */ } }"},
		{"calc_return_multiplicity_unclosed", "calc def C { in xs : Real[*]; return [* = xs; }"},
		// A short name must be closed before the member's declaration tail.
		{"calc_short_name_unclosed", "calc def C { <s x = 1; }"},
		{"calc_short_name_empty", "calc def C { <> x = 1; }"},
		{"part_short_name_unclosed", "part def P { <s :> x = 1; }"},
		{"calc_while_no_condition", "calc def C { while { i = i + 1; } }"},
		{"calc_while_unclosed_body", "calc def C { while i < 2 { i = i + 1; }"},
		{"calc_for_no_variable", "calc def C { for in xs { } }"},
		{"calc_assignment_no_value", "calc def C { i = ; }"},
		{"calc_if_no_body", "calc def C { if i < 2 }"},
		{"constraint_incomplete", "constraint c { assert }"},
		// A parameterised constraint body asserts conditions like any other, so a
		// condition that is missing its expression is an error there too.
		{"constraint_params_assert_no_condition", "constraint c { in x : Real; assert; }"},
		{"constraint_params_assume_no_condition", "constraint c { in x : Real; assume }"},
		{"constraint_params_assert_not_no_condition", "constraint c { in x : Real; assert not; }"},
		// A require member referencing a requirement takes specializations and a
		// body, so neither may be left half written.
		{"require_reference_no_specialization_type", "requirement r { require Q::x : ; }"},
		{"require_reference_unclosed_body", "requirement r { require Q::x { require constraint { x > 0 } }"},
		// Prefix metadata needs a name, and the declaration after it a terminator.
		{"require_prefix_metadata_no_type", "requirement r { require #; }"},
		{"require_prefix_metadata_unterminated", "requirement r { require #goal c }"},
		// Prefix metadata belongs after these keywords, not ahead of them.
		{"subject_prefix_metadata_before_keyword", "requirement r { #goal subject s : T; }"},
		{"subject_prefix_metadata_before_keyword_eof", "requirement r { #goal subject"},
		{"actor_prefix_metadata_before_keyword", "requirement r { #goal actor a; }"},
		{"variant_prefix_metadata_before_keyword", "variation part def V { #goal variant part v; }"},
		{"assume_prefix_metadata_before_keyword", "requirement r { #goal assume constraint a : C; }"},
		{"require_prefix_metadata_before_keyword", "requirement r { #goal require constraint r : C; }"},
		{"require_prefix_metadata_before_keyword_bare", "requirement r { #goal require ; }"},
		// An assertion's prefix metadata comes ahead of `assert`, not after it or
		// its `not` (SysML.xtext AssertConstraintUsage `OccurrenceUsagePrefix 'assert'`).
		{"assert_prefix_metadata_after_keyword", "package P { part def D { assert #B constraint c; } }"},
		{"assert_prefix_metadata_after_not", "package P { part def D { assert not #B constraint c; } }"},
		// A target succession's body closes, and its ends are still required
		// (SysML.xtext:1698 ActionTargetSuccession).
		{"succession_body_unclosed", "package P { action def A { action a; first a; then a { doc /* x */ } }"},
		{"succession_body_no_target", "package P { action def A { action a; first a; then { } } }"},
		// Only an action body item reaches ActionTargetSuccession (SysML.xtext:1698), so a
		// bodied `then` is not a state, definition or namespace member (:1796-1801, :516-524).
		{"entry_succession_body", "package P { state def S { state starting; entry; then starting { doc /* x */ } } }"},
		{"definition_succession_body", "package P { part def D { part a; part b; then b { doc /* x */ } } }"},
		{"namespace_succession_body", "package P { action a; action b; then b { doc /* x */ } }"},
		// A control node in a case body takes a declaration or nothing, and its
		// body closes (SysML.xtext:1676 JoinNode, :1682 ForkNode).
		{"case_body_fork_unclosed", "package P { use case def U { fork f { action a; } }"},
		{"case_body_join_no_terminator", "package P { use case def U { join j } }"},
		// A `for` in a case body states a variable and a sequence
		// (SysML.xtext ForLoopNode).
		{"case_body_for_no_variable", "package P { analysis def A { for in xs { } } }"},
		{"case_body_for_no_sequence", "package P { analysis def A { for i in { } } }"},
		// A metadata usage's `about` names at least one annotated element
		// (SysML.xtext:145-147).
		{"metadata_about_no_target", "package P { metadata def M; @M about ; }"},
		{"metadata_about_trailing_comma", "package P { metadata def M; part a; @M about a, ; }"},
		// Each one is a qualified name, not a feature chain (SysML.xtext AnnotatedElement).
		{"metadata_usage_about_feature_chain", "package P { metadata def M; part a { attribute x; } metadata m : M about a.x; }"},
		{"metadata_usage_about_no_target", "package P { metadata def M; metadata m : M about ; }"},
		// A prefixed dependency still states both of its ends
		// (SysML.xtext:55-58).
		{"prefixed_dependency_no_supplier", "package P { metadata def M; part a; #M dependency d from a to ; }"},
		// A verification's requirement reference is a name, chained or not
		// (SysML.xtext:2119).
		{"verify_chained_no_member", "package P { verification def V { verify a. ; } }"},
		// `not` negates a satisfaction, and nothing else at member level.
		{"not_without_satisfy", "package P { not r1 by p; }"},
		{"not_satisfy_no_subject", "package P { not satisfy r1 by ; }"},
		// A guarded succession needs a guard expression and a target after it.
		{"guarded_succession_no_guard", "action def A { action a; action b; succession S first a if then b; }"},
		{"guarded_succession_no_target", "action def A { action a; succession S first a if x == 0 then ; }"},
		// Orthogonality is spelled `parallel` before a state body, so a bodied
		// `region` member is not a state body item.
		{"region_member", "state def S { region r { state a; } }"},
		{"state_fork_no_name", "state s { fork ; }"},
		{"state_join_no_semicolon", "state s { join sync state t; }"},
		{"call_trigger_unclosed_params", "state s { accept op(a then t; }"},
		{"call_trigger_missing_param_name", "state s { accept op(,) then t; }"},
		{"perform_no_reference", "action a { perform ; }"},
		{"perform_dangling_chain", "action a { perform b.; }"},
		{"allocate_missing_target", "package q { allocate a to ; }"},
		// `allocate` is one keyword with one role, and it must be followed by a
		// ConnectorPart (D1, SysML.xtext:1219-1222).
		{"allocate_without_connector_part", "package q { allocate al; }"},
		{"allocate_with_body_and_no_ends", "package q { allocate al { } }"},
		{"allocate_def", "package q { allocate def D; }"},
		{"message_payload_declaration_no_type", "message m of pay : from a to b;"},
		{"message_payload_declaration_no_target", "message m of pay : T from a;"},
		// `state s { defer ; }` and `state s { history ; }` are no longer malformed:
		// neither word is a grammar literal, so each names a reference usage there
		// (docs/reference/grammar/conformance-audit.md).
		{"defer_no_semicolon", "state s { defer Ping state t; }"},
		{"defer_trailing_comma", "state s { defer Ping, ; }"},
		{"deep_without_history", "state s { deep resume; }"},
		{"shallow_without_history", "state s { shallow resume; }"},
		{"history_no_semicolon", "state s { history resume state t; }"},
		// `entry point;` is not malformed: `point` is not reserved, so it is an
		// entry action referencing a feature named `point`. Dropping the ';' is.
		{"entry_reference_no_semicolon_brace", "state s { entry point }"},
		// There is no entry or exit point pseudostate: `entry point x;` is an entry
		// action referencing `point`, followed by a stray name.
		{"entry_point_pseudostate", "state s { entry point x; }"},
		{"exit_point_pseudostate", "state s { exit point x; }"},
		{"entry_point_pseudostate_in_def", "state def S { entry point into; exit point outOf; }"},
		{"entry_reference_no_semicolon", "state s { entry warmUp state t; }"},
		{"exit_reference_no_semicolon", "state s { exit coolDown state t; }"},
		{"do_reference_dangling_chain", "state s { do warmUp.; }"},

		// `on` and `var` are names, not keywords, so genuine misuse around them
		// is still reported: a state named `on` without its `;`, a transition
		// whose trigger is missing after a source named `on`, and a `var`-marked
		// declaration with no type. A `var` prefix with the kind keyword left out
		// (KerML BasicFeaturePrefix, `var a : Integer;`) is not supported and is
		// reported rather than read as a reference to a feature named `var`.
		{"state_named_on_no_semicolon", "state def S { state on }"},
		{"transition_from_on_no_trigger", "state def S { state on; transition first on accept then off; }"},
		{"var_prefixed_declaration_no_type", "part def D { var attribute x : ; }"},
		{"attribute_named_var_no_type", "part def D { attribute var : ; }"},
		{"var_prefix_without_kind_keyword", "calc def C { var a : Integer; }"},
		{"end_no_feature", "connection def C { end ; }"},
		{"end_unclosed_multiplicity", "connection def C { end [1 part bead : T; }"},
		{"connector_end_no_reference_target", "part p { connection : C connect bead references to rim; }"},
		{"flow_source_without_target", "part def C { item Fuel; part a; flow f of Fuel from a; }"},
		{"nary_connect_unclosed", "part def C { part a; part b; connection conn connect (a, b; }"},
		{"nary_connect_trailing_comma", "part def C { part a; part b; connection conn connect (a, b, ); }"},
		{"nary_connect_empty", "part def C { connection conn connect (); }"},
		{"anonymous_nary_connect_unclosed", "part def C { part a; part b; connect (a, b; }"},
		{"anonymous_nary_connect_empty", "part def C { connect (); }"},

		// Occurrence modifiers (`individual`, `snapshot`) on a usage.
		// `individual ;` is well-formed (SysML.xtext Usage: UsageDeclaration?) and is
		// covered by TestParseUsageOccurrenceModifiers instead.
		{"individual_usage_no_type", "individual testSystem : ;"},
		{"individual_usage_no_body", "individual testSystem : TestSystem"},
		{"snapshot_usage_no_type", "snapshot occurrence takeoff : ;"},
		{"individual_parameter_no_type", "action a { in individual v : ; }"},

		// A keyword names a parameter, so the keywords that state something else
		// there are still read as that: a missing value or type is an error
		// rather than a parameter so named.
		{"keyword_named_parameter_no_type", "action a { in 'type' : ; }"},
		{"parameter_default_no_value", "action a { in x default ; }"},
		{"parameter_redefines_no_target", "action a { in redefines ; }"},

		// A member-attached `then` sequences the members either side of it, so
		// a body with nothing on one side, or a member the notation does not
		// allow one before, declares no order and is rejected rather than
		// parsed with the keyword dropped. A `then` beside a member with no
		// name is legal notation, bound by position
		// (TestSuccessionBindsUnnamedEndsByPosition).
		{"leading_then_has_no_source", "action a { then action b; }"},
		{"trailing_then_has_no_target", "action a { action b; then }"},
		{"then_then", "action a { action b; then then action c; }"},
		{"then_before_definition", "action a { action b; then action def C; }"},
		{"then_before_package", "part def P { part a; then package Inner { } }"},
		{"then_before_attribute", "part def P { part a; then attribute x; }"},
		{"then_before_import", "part def P { part a; then import Other::*; }"},

		// A feature specialization keyword after a short name states a
		// relationship, so a missing target is an error rather than a name.
		{"short_name_redefines_no_target", "part p { attribute <sn> redefines; }"},
		{"short_name_redefines_symbol_no_target", "part p { attribute <sn> :>>; }"},
		{"short_name_defined_by_no_type", "part p { attribute <sn> defined by ; }"},

		// A requirement's subject, assume and require members take an
		// identification (SysML.xtext SubjectUsage, RequirementConstraintUsage →
		// UsageDeclaration), so a malformed short name is an error there too.
		{"subject_empty_short_name", "requirement def R { subject <> x; }"},
		{"subject_unclosed_short_name", "requirement def R { subject <s x; }"},
		{"assume_empty_short_name", "requirement def R { assume constraint <> c; }"},
		{"assume_unclosed_short_name", "requirement def R { assume constraint <c x; }"},
		{"require_empty_short_name", "requirement def R { require constraint <> c; }"},
		{"require_unclosed_short_name", "requirement def R { require constraint <c x; }"},

		// The notation has no definition of a rendering a view names, of a
		// concern a body frames, or of a stakeholder or actor: those keywords own
		// a usage, not a definition (SysML.xtext ViewRenderingUsage,
		// FramedConcernUsage, StakeholderUsage, ActorUsage). `render ;` and
		// `frame ;` are absent deliberately: `frame` and `render` are legal
		// names, so those declare a feature so named rather than being errors.
		{"render_definition", "view def V { render def R; }"},
		{"frame_definition", "viewpoint def V { frame def C; }"},
		{"stakeholder_definition", "stakeholder def Reviewer;"},
		{"actor_definition", "actor def Operator;"},
		{"stakeholder_no_declaration", "viewpoint def V { stakeholder ; }"},
		{"actor_no_declaration", "requirement def R { actor ; }"},
		// A rendering reference takes no value: ViewRenderingUsage has no
		// ValuePart, unlike the performed action reference that shares its shape.
		{"render_reference_value", "view def V { render r = 3; }"},

		// The sequence index and the collection notations: `#` indexes through a
		// parenthesized index and `.?` selects through a body, so each is
		// rejected where the notation it needs is absent rather than parsed as
		// the operand alone.
		{"index_no_paren", "attribute x = xs#3;"},
		{"index_no_index", "attribute x = xs#();"},
		{"index_unclosed", "attribute x = xs#(1;"},
		{"index_bracket_unclosed", "attribute x = 5 [m;"},
		{"index_bracket_empty", "attribute x = 5 [];"},
		{"select_no_body", "attribute x = xs.?;"},
		{"select_expression_body", "attribute x = xs.? x > 1;"},
		{"select_unclosed_body", "attribute x = xs.?{in x; x > 1;"},
		{"collect_unclosed_body", "attribute x = xs.{in x; x * 2;"},
		{"body_param_no_name", "attribute x = xs.{in ; 1};"},
		{"body_param_no_type", "attribute x = xs.{in y : ; 1};"},
		{"receiver_no_operation", "attribute x = xs->;"},
		{"receiver_unclosed_args", "attribute x = xs->union((1, 2);"},

		// A conjugation names the definition it conjugates, only an interface end
		// may omit its declaration, a required requirement binds a feature in its
		// body, and a portion usage declares what it portions.
		{"conjugated_no_type", "part def P { port p : ~; }"},
		{"conjugated_no_type_after_name", "port def P; port p ~;"},
		{"conjugated_end_no_type", "connection def C { end e : ~; }"},
		{"end_outside_connector", "part def P { end ; }"},
		{"end_outside_connector_package", "package p { end ; }"},
		{"end_in_package_nested_in_interface", "interface def I { package P { end ; } }"},
		{"end_in_satisfy_nested_in_interface", "interface def I { satisfy requirement r { end ; } }"},
		{"end_in_binding_nested_in_interface", "interface def I { binding b { end ; } }"},
		{"require_qualified_malformed_body", "analysis def A { objective o { require Q::r { :>> ; } } }"},
		{"require_qualified_trailing_colons", "analysis def A { objective o { require Q::; } }"},
		{"timeslice_no_subject", "package p { timeslice :>> ; }"},
		{"timeslice_usage_no_type", "package p { timeslice item i : ; }"},
		{"timeslice_unterminated", "package p { timeslice item i"},

		// `variation` and `variant` qualify a declaration, so each is rejected
		// where the declaration it qualifies is absent or malformed.
		{"variation_no_declaration", "variation ;"},
		{"variation_attribute_no_name", "part p { variation attribute : ; }"},
		{"variant_unclosed_body", "part p { variation attribute cut { variant attribute cutIdeal { :>> cost = 1.0; } }"},
		{"variant_selection_no_variant_name", "part p { attribute :>> cut = cut::; }"},

		// Behavioral notation: a flow states both of its ends, an accept its
		// payload, a loop the condition its `until` promises and a succession a
		// target, so each is reported where one is missing rather than read as
		// the shorter form it is not.
		{"flow_from_without_to", "action def A { action a; flow x from a; }"},
		{"flow_named_from_no_source", "action def A { flow x from to b; }"},
		{"accept_when_no_condition", "action def A { accept when; }"},
		{"accept_at_no_instant", "action def A { accept at; }"},
		{"accept_no_payload", "action def A { accept; }"},
		{"accept_subsets_no_event", "action def A { action i accept :>; }"},
		{"loop_until_no_condition", "action def A { loop action { } until; }"},
		{"loop_until_no_semicolon", "action def A { action b; then loop action { } until x }"},
		{"then_done_no_semicolon", "action def A { action b; then done }"},
		{"send_via_no_port", "action def A { send Data() via; }"},
		{"send_no_target", "action def A { send Data() to; }"},
		{"decision_else_no_target", "action def A { action m; first m; then decide; else; }"},
		{"transition_trigger_no_target", "state def S { state a; transition first a accept Ping; }"},
		{"transition_two_triggers", "state def S { state a; state b; transition first a accept Ping accept Pong then b; }"},
		{"transition_two_targets", "state def S { state a; state b; transition first a then b then b; }"},
		{"transition_do_without_action", "state def S { state a; state b; transition first a do then b; }"},
		{"exhibit_state_unclosed_body", "part def P { exhibit state modes { state off; }"},
		{"namespace_succession_no_target", "package Q { part p; first p then; }"},
		// Only a transition effect is closed by the transition's next clause, so
		// a statement in an action body still needs its ';'.
		{"body_assignment_no_semicolon", "action def A { attribute x; action b; assign x := 1 then b; }"},
		// An assignment target names a feature or ends in one (SysML.xtext
		// TargetParameter), so a literal, an operator or an invocation is not one.
		{"assignment_target_literal", "action def A { assign 1 := 2; }"},
		{"assignment_target_operator", "action def A { attribute x; assign x + 1 := 2; }"},
		{"assignment_target_invocation", "action def A { calc def f { return : Integer = 1; } assign f() := 2; }"},
		{"assignment_target_index", "action def A { attribute xs : Integer[0..*]; assign xs[1] := 2; }"},
		{"body_send_no_semicolon", "action def A { action b; part self; send Data() to self then b; }"},
		// A transition takes exactly one ';', which its effect statement shares
		// (SysML.xtext TransitionUsage ends with ActionBody); a second one is not
		// an empty member.
		{"transition_effect_perform_two_semicolons", "state def S { state a; state b; transition first a do perform Bump then b ;; }"},
		{"transition_effect_assign_two_semicolons", "state def S { attribute x; state a; state b; transition first a do assign x := 1 then b ;; }"},
		{"transition_effect_no_semicolon", "state def S { attribute x; state a; state b; transition first a do assign x := 1 then b }"},
		{"transition_braced_effect_no_semicolon", "state def S { attribute x; state a; state b; transition first a do { assign x := 1; } then b }"},
		// A binding end names a feature by a qualified name or a chain of them,
		// so neither qualification nor chaining may end in nothing.
		{"binding_end_qualification_no_name", "package P { part c; binding bind R:: = c; }"},
		{"binding_end_chain_trailing_dot", "package P { part a; part c; binding bind a. = c; }"},
		{"binding_end_chain_trailing_dot_qualified", "package P { part c; binding bind R::a. = c; }"},
		{"binding_end_unterminated", "package P { part a; binding bind R::a }"},
		{"binding_end_no_target", "package P { part a; binding bind R::a = ; }"},
		// An expression is not a ConnectorEndMember, so a binding whose right
		// side is one is rejected (see TestBindingEndFailuresAreDistinguishable).
		{"binding_end_expression", "package P { part def D { attribute a; attribute b; bind a = b * 2; } }"},
		// A named binding end references a feature after its `::>` (SysML.xtext
		// ConnectorEnd), and a binding joins exactly two ends with '=', never a
		// succession's `then`.
		{"binding_named_end_no_target", "package P { part def D { attribute a; attribute b; bind e1 ::> = b; } }"},
		{"binding_named_second_end_no_target", "package P { part def D { attribute a; attribute b; bind a = e2 references ; } }"},
		{"binding_named_end_no_eq", "package P { part def D { attribute a; attribute b; bind e1 ::> a b; } }"},
		{"binding_ends_then", "package P { part def D { attribute a; attribute b; attribute c; bind a = b then c; } }"},
		{"binding_named_end_expression", "package P { part def D { attribute a; attribute b; bind e1 ::> a = b + 1; } }"},
		{"binding_declaration_bind_no_ends", "package P { part def D { attribute a; binding x : AB bind ; } }"},
		// A SysML UsageDeclaration puts its multiplicity after the name; a bracket
		// before the name is a connector end's (pilot: `no viable alternative at input 'ab2'`).
		{"binding_multiplicity_before_name", "package P { part def D { attribute a; attribute b; binding [1] ab2 : AB bind a = b; } }"},

		// A connection, interface or flow usage stating its ends where its name
		// would go still states both ends and closes the body it opens.
		{"connect_no_ends", "part def P { connect; }"},
		{"connect_body_no_ends", "part def P { connect { attribute x; } }"},
		{"connect_body_unclosed", "part def P { part a; part b; connect a to b { attribute x; }"},
		{"connect_to_no_target_before_body", "part def P { part a; connect a to { } }"},
		{"interface_ends_no_target", "package P { part a; interface a.p to ; }"},
		{"interface_ends_no_to", "package P { part a; part b; interface a.p b.p; }"},
		{"interface_ends_unclosed_body", "package P { part a; part b; interface a.p to b.p { attribute x; }"},
		{"flow_ends_no_target_before_body", "action def A { action a; flow a.x to { } }"},
		{"flow_ends_unclosed_body", "action def A { action a; action b; flow a.x to b.x { attribute y; }"},
		// An accept node standing as a statement states its payload and, where it
		// names one, the port it accepts through.
		{"accept_statement_no_payload", "action def A { loop { accept; } }"},
		{"accept_statement_no_payload_type", "action def A { loop { accept e : ; } }"},
		{"accept_statement_via_no_port", "action def A { loop { accept e : E via; } }"},
		{"then_accept_no_payload", "action def A { action b; then accept; }"},

		// A word SysML.xtext does not reserve names a usage, so a declaration so
		// named that is malformed is still reported rather than read as the
		// KerML relationship the same word states elsewhere.
		{"part_named_chains_no_type", "part chains : ;"},
		{"part_named_differences_no_terminator", "part differences"},
		{"part_named_disjoint_no_type", "part disjoint : ;"},
		{"part_named_type_no_type", "part type : ;"},
		// The relationship spelling still states the relationship where one
		// belongs, so its missing operand is still an error.
		{"usage_disjoint_from_no_target", "part def T { part a; part b disjoint from ; }"},
		{"usage_inverse_of_no_target", "part def T { part a; part b inverse of ; }"},
		{"usage_chains_no_target", "part def T { part a; part b chains ; }"},
		{"usage_featured_by_no_target", "part def T { part b featured by ; }"},

		// The generalized usage declaration widening keeps its neighbouring
		// malformed spellings reported (F66).
		{"ref_redefines_no_target", "part def T { ref redefines [4]; }"},
		{"ref_redefines_unclosed_multiplicity", "part def T { ref redefines x[4; }"},
		{"verify_redefines_no_target", "requirement def R { verify r :>> ; }"},
		{"variant_use_case_no_type", "use case def U { variant use case uc : ; }"},
		{"assert_not_no_condition", "part def T { assert not ; }"},
		{"assert_not_no_body_end", "part def T { assert not c { }"},

		// `frame` and `render` are SysML keywords, so a framing or rendering
		// with no reference is reported rather than read as a name.
		{"frame_no_concern", "viewpoint def V { frame; }"},
		{"frame_concern_no_declaration", "viewpoint def V { frame concern }"},
		{"render_no_rendering", "view def V { render; }"},
		// A name follows its kind keyword, never precedes it (SysML.xtext
		// DefinitionDeclaration, UsageDeclaration).
		{"name_before_usage_keyword", "part def B { foo attribute bar : A; }"},
		{"name_before_definition_keyword", "package P { x part def Q; }"},

		// A specialization following a parameter's multiplicity still names its
		// target and closes its multiplicity (KerML.xtext:574).
		{"parameter_multiplicity_redefines_no_target", "action def A { in x : Integer[1] redefines ; }"},
		{"parameter_multiplicity_redefines_symbol_no_target", "action def A { in x : Integer[1] :>> ; }"},
		{"parameter_multiplicity_ordered_subsets_no_target", "action def A { in xs : Integer[*] ordered :> ; }"},
		{"parameter_multiplicity_unclosed_before_redefines", "action def A { in x : Integer[1 redefines A::x; }"},
		{"parameter_multiplicity_redefines_no_terminator", "action def A { in x : Integer[1] redefines A::x }"},
		{"out_parameter_multiplicity_redefines_no_target", "action def A { out x : Integer[1] redefines ; }"},
		{"inout_parameter_multiplicity_redefines_no_target", "action def A { inout x : Integer[1] :>> ; }"},
		{"return_multiplicity_redefines_no_target", "calc def C { return : Integer[1] ordered :>> ; }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := source.New(tt.name+".sysml", []byte(tt.input))
			p := New(sf)
			_ = p.ParseFile()

			if len(p.Diagnostics) == 0 {
				t.Errorf("Expected parse errors for malformed input, got none.\nInput: %s", tt.input)
			}
		})
	}
}

func TestRemovedSuccessionFormsProduceDiagnosticsAndErrorNodes(t *testing.T) {
	tests := []struct {
		name          string
		src           string
		forbiddenNode string
	}{
		{"two_ended_then", "action def A { action a; action b; then a b; }", "SuccessionEdge"},
		{"state_member_then", "state def S { state a; state b; a then b; }", "SuccessionEdge"},
		{"named_done", "action def A { done end; }", "FinalNode"},
		{"guarded_two_ended_then", "action def A { action a; action b; then a b if x > 0; }", "ControlFlowEdge"},
		{"malformed_named_done", "action def A { done a b; }", "FinalNode"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("parser panicked: %v", r)
				}
			}()
			p := New(source.New(tt.name+".sysml", []byte(tt.src)))
			root := p.ParseFile()
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(p.Diagnostics) == 0 {
				t.Fatal("expected a diagnostic")
			}
			dump := ast.Dump(root)
			if !strings.Contains(dump, "ErrorNode") {
				t.Fatalf("expected an ErrorNode:\n%s", dump)
			}
			if strings.Contains(dump, tt.forbiddenNode) {
				t.Fatalf("unexpected %s for removed spelling:\n%s", tt.forbiddenNode, dump)
			}
		})
	}
}

// A name written ahead of a kind keyword is no declaration: the stray name is
// reported and skipped without naming anything, and the members after it parse.
func TestNameBeforeKeywordIsNotADeclaration(t *testing.T) {
	tests := []struct {
		name       string
		ext        string
		src        string
		stray      string
		message    string
		wantMember string
	}{
		{"body_usage", ".sysml", "package P { attribute def A; part def B { foo attribute bar : A; attribute ok : A; } }", "foo", msgExpectedBodyMember, `name="ok"`},
		{"body_definition", ".sysml", "package P { part def B { foo part def Q; part def R; } }", "foo", msgExpectedBodyMember, `name="R"`},
		{"body_usage_with_body", ".sysml", "package P { part def B { myConstraint constraint { 1 > 0 } part q; } }", "myConstraint", msgExpectedBodyMember, `name="q"`},
		{"namespace_definition", ".sysml", "package P { x part def Q; part def R; }", "x", "expected a namespace member", `name="R"`},
		{"kerml_body_feature", ".kerml", "package P { class A; class B { foo feature bar : A; feature ok : A; } }", "foo", msgExpectedBodyMember, `name="ok"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("parser panicked: %v", r)
				}
			}()
			sf := source.New(tt.name+tt.ext, []byte(tt.src))
			p := New(sf)
			root := p.ParseFile()
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(p.Diagnostics) != 1 {
				t.Fatalf("want one diagnostic, got %v", p.Diagnostics)
			}
			d := p.Diagnostics[0]
			if d.Message != tt.message {
				t.Errorf("message = %q, want %q", d.Message, tt.message)
			}
			if got := sf.Text(d.Span); got != tt.stray {
				t.Errorf("diagnostic points at %q, want the stray name %q", got, tt.stray)
			}
			dump := ast.Dump(root)
			if !strings.Contains(dump, "ErrorNode") {
				t.Fatalf("expected an ErrorNode:\n%s", dump)
			}
			if strings.Contains(dump, `name="`+tt.stray+`"`) {
				t.Errorf("stray name %q became a declaration:\n%s", tt.stray, dump)
			}
			if !strings.Contains(dump, tt.wantMember) {
				t.Errorf("member after the error was not parsed (want %s):\n%s", tt.wantMember, dump)
			}
		})
	}
}

// An expression binding end is rejected with its own message, so it stays
// distinguishable from an end that is missing altogether.
func TestBindingEndFailuresAreDistinguishable(t *testing.T) {
	const expressionMessage = "a binding end names a feature, not an expression"
	tests := []struct {
		name        string
		src         string
		wantMessage bool
	}{
		{"expression_end", "package P { part def D { attribute a; attribute b; bind a = b * 2; } }", true},
		{"expression_end_named_binding", "package P { part def D { attribute a; attribute b; binding bb bind a = b + 1; } }", true},
		{"expression_end_literal", "package P { part def D { attribute a; bind a = 2; } }", true},
		{"missing_right_end", "package P { part def D { attribute a; attribute b; binding bb bind a = ; } }", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("parser panicked: %v", r)
				}
			}()
			p := New(source.New(tt.name+".sysml", []byte(tt.src)))
			root := p.ParseFile()
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(p.Diagnostics) == 0 {
				t.Fatal("expected a diagnostic")
			}
			if !strings.Contains(ast.Dump(root), "ErrorNode") {
				t.Fatalf("expected an ErrorNode:\n%s", ast.Dump(root))
			}
			var found bool
			for _, d := range p.Diagnostics {
				if strings.Contains(d.Message, expressionMessage) {
					found = true
				}
			}
			if found != tt.wantMessage {
				t.Fatalf("expression-end message present = %v, want %v: %v", found, tt.wantMessage, p.Diagnostics)
			}
		})
	}
}

// An expression assignment target is rejected with its own message, while a
// name or a chain ending in a feature stays accepted (SysML.xtext TargetParameter).
func TestAssignmentTargetMustNameFeature(t *testing.T) {
	const expressionMessage = "an assignment target names a feature or a feature chain, not an expression"
	tests := []struct {
		name     string
		src      string
		wantDiag bool
	}{
		{"name", "action def A { attribute x; assign x := 2; }", false},
		{"qualified", "package P { attribute x; action def A { assign P::x := 2; } }", false},
		{"chain", "action def A { part m { part n { attribute x; } } assign m.n.x := 2; }", false},
		{"indexed_chain", "action def A { part ms[0..*] { attribute x; } assign ms[1].x := 2; }", false},
		{"invocation_chain", "action def A { calc def pick { return : Leaf; } assign pick().x := 2; }", false},
		{"transition_effect", "state def S { attribute x; state a; state b; transition first a do assign x := 1 then b; }", false},
		{"literal", "action def A { assign 1 := 2; }", true},
		{"operator", "action def A { attribute x; assign x + 1 := 2; }", true},
		{"invocation", "action def A { calc def f { return : Integer = 1; } assign f() := 2; }", true},
		{"index", "action def A { attribute xs : Integer[0..*]; assign xs[1] := 2; }", true},
		{"transition_effect_literal", "state def S { state a; state b; transition first a do assign 1 := 1 then b; }", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(source.New(tt.name+".sysml", []byte(tt.src)))
			root := p.ParseFile()
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			if !tt.wantDiag {
				if len(p.Diagnostics) != 0 {
					t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
				}
				if strings.Contains(ast.Dump(root), "ErrorNode") {
					t.Fatalf("unexpected ErrorNode:\n%s", ast.Dump(root))
				}
				return
			}
			if len(p.Diagnostics) != 1 || !strings.Contains(p.Diagnostics[0].Message, expressionMessage) {
				t.Fatalf("want exactly the expression-target diagnostic, got %v", p.Diagnostics)
			}
		})
	}
}

// The standard binding forms stay accepted: unqualified, chained, qualified
// and indexed ends (`bind [0..*] base.edges = [0..*] be;` in the Geometry library).
func TestStandardBindingFormsRemainValid(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"simple", "part def D { attribute a; attribute b; bind a = b; }"},
		{"chain", "part def D { part a { attribute x; } attribute b; bind b = a.x; }"},
		{"named_qualified", "package R { part a; } package P { part c; binding b1 bind R::a = c; }"},
		{"indexed_ends", "part def D { part base { part edges[0..*]; } part be[0..*]; bind [0..*] base.edges = [0..*] be; }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(source.New(tt.name+".sysml", []byte(tt.src)))
			root := p.ParseFile()
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(p.Diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
			}
			if strings.Contains(ast.Dump(root), "ErrorNode") {
				t.Fatalf("unexpected ErrorNode:\n%s", ast.Dump(root))
			}
		})
	}
}

func TestStandardSuccessionFormsRemainValid(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"implicit_target", "action def A { action a; then a; }"},
		{"implicit_done", "action def A { action a; then done; }"},
		{"explicit_succession", "action def A { action a; action b; succession first a then b; }"},
		{"guarded_succession", "action def A { attribute g = true; action a; action b; succession first a if g then b; }"},
		{"ordinary_done_name", "action def A { attribute done : Boolean; }"},
		{"ordinary_done_state_name", "state def S { state done; }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(source.New(tt.name+".sysml", []byte(tt.src)))
			root := p.ParseFile()
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(p.Diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
			}
			if strings.Contains(ast.Dump(root), "ErrorNode") {
				t.Fatalf("unexpected ErrorNode:\n%s", ast.Dump(root))
			}
		})
	}
}

// TestNegativeKerML covers the KerML notation whose malformed variants must
// still be reported: accepting them would be a conformance failure of its own.
func TestNegativeKerML(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		// `featured by` states one target or a list of them, so the keyword pair
		// and every target in the list are required (KerML.xtext:569).
		{"featured_without_by", "package P { class A; feature f featured A; }"},
		{"featured_by_no_target", "package P { feature f featured by ; }"},
		{"featured_by_trailing_comma", "package P { class A; feature f featured by A, ; }"},
		{"featured_by_no_terminator", "package P { class A; feature f featured by A }"},
		{"by_without_featured", "package P { class A; feature f by A; }"},

		// A parenthesized end list holds at least two ends and closes
		// (KerML.xtext:842).
		{"nary_connector_unclosed", "package P { feature a; feature b; connector c (a, b; }"},
		{"nary_connector_trailing_comma", "package P { feature a; feature b; connector c (a, b, ); }"},
		{"nary_connector_empty", "package P { connector c (); }"},
		{"nary_connector_one_end", "package P { feature a; connector c (a); }"},
		{"nary_connector_end_no_target", "package P { feature a; connector c ([1], a); }"},

		// A binary connector states both ends around `to` (KerML.xtext:836).
		{"binary_connector_no_second_end", "package P { feature a; connector a to ; }"},
		{"binary_connector_no_first_end", "package P { feature t; connector to t; }"},
		{"binary_connector_no_to", "package P { feature eng; feature t; connector eng t; }"},
		{"binary_connector_end_multiplicity_no_target", "package P { feature a; feature b; connector [1] to b; }"},
		{"binary_connector_named_end_no_target", "package P { feature a; feature b; connector e ::> to b; }"},
		{"binary_connector_name_without_from", "package P { feature a; feature b; connector c : L a to b; }"},

		// A succession states both ends around `then` (KerML.xtext:891).
		{"succession_declaration_no_then", "package P { behavior B { step a; step b; succession s : L [1] first a b; } }"},
		{"succession_declaration_no_target", "package P { behavior B { step a; succession s : L [1] first a then ; } }"},
		{"succession_declaration_no_ends", "package P { behavior B { succession s : L [1] first then; } }"},

		// An end's crossing multiplicity is followed by its end (KerML.xtext:854), so
		// one followed by nothing or the other end's keyword, or left unclosed, is reported.
		{"binding_end_multiplicity_no_end", "package P { feature a; feature b; binding [1] = b; }"},
		{"binding_end_multiplicity_unclosed", "package P { feature a; feature b; binding [1 a = b; }"},
		{"binding_second_end_multiplicity_no_end", "package P { feature a; feature b; binding [1] a = [1] ; }"},
		{"binding_end_multiplicity_no_terminator", "package P { feature a; feature b; binding [1] a = [1] b }"},
		// A named binding end references a feature after its `::>` or `references`
		// (KerML.xtext ConnectorEnd), and `of` is followed by two ends around '='.
		{"binding_named_end_no_target", "package P { feature a; feature b; binding e1 ::> = b; }"},
		{"binding_of_named_end_no_target", "package P { feature a; feature b; binding of e1 references = b; }"},
		{"binding_of_no_ends", "package P { feature a; feature b; binding x of ; }"},
		{"binding_of_no_second_end", "package P { feature a; feature b; binding of a = ; }"},
		{"binding_ends_then", "package P { feature a; feature b; feature c; binding a = b then c; }"},
		{"binding_named_end_expression", "package P { feature a; feature b; binding of e1 ::> a = b + 1; }"},
		{"succession_end_multiplicity_no_end", "package P { behavior B { step a; step b; succession [1] then b; } }"},
		{"succession_end_multiplicity_unclosed", "package P { behavior B { step a; step b; succession [1 a then b; } }"},
		{"succession_second_end_multiplicity_no_end", "package P { behavior B { step a; step b; succession [1] a then [1] ; } }"},
		{"succession_end_multiplicity_no_terminator", "package P { behavior B { step a; step b; succession [1] a then [1] b } }"},
		{"succession_named_end_no_target", "package P { behavior B { step a; step b; succession [1] e ::> then b; } }"},
		{"succession_global_end_no_name", "package P { behavior B { step a; step b; succession [1] $:: then b; } }"},

		// A named multiplicity states bounds or a subsetting, and the
		// subsetting names one multiplicity (KerML.xtext:754).
		{"multiplicity_subsets_no_target", "package P { multiplicity m subsets ; }"},
		{"multiplicity_subsets_no_terminator", "package P { multiplicity zeroOrMore [0..*]; multiplicity m subsets zeroOrMore }"},
		{"multiplicity_bounds_and_subsets", "package P { multiplicity zeroOrMore [0..*]; multiplicity m [0..*] subsets zeroOrMore; }"},

		// An index states a sequence of at least one expression and closes
		// (KerMLExpressions.xtext PrimaryExpression).
		{"index_sequence_unclosed", "package P { feature a; feature b = a#(1, 2; }"},
		{"index_sequence_empty", "package P { feature a; feature b = a#(); }"},
		{"index_sequence_trailing_comma", "package P { feature a; feature b = a#(1,); }"},
		{"index_bracket_unclosed", "package P { feature a; feature b = a[1, 2; }"},

		// A word the KerML grammar does not reserve names a feature, so a
		// declaration so named that is malformed is still reported.
		{"feature_named_merge_no_type", "package P { feature merge : ; }"},
		{"feature_named_at_no_terminator", "package P { feature at }"},
		{"import_named_while_no_terminator", "package P { public import while }"},

		// A KerML literal states its relationship where a name would go, so it
		// does not name a feature there (the pinned KerML validator answers
		// "no viable alternative at input 'chains'").
		{"feature_named_chains", "package P { class T; feature chains : T; }"},
		// A word KerML.xtext does not reserve names a feature, so a malformed
		// declaration so named is still reported.
		{"feature_named_frame_no_type", "package P { feature frame : ; }"},
		{"feature_named_state_no_terminator", "package P { feature state }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(source.New(tt.name+".kerml", []byte(tt.input)))
			_ = p.ParseFile()

			if len(p.Diagnostics) == 0 {
				t.Errorf("Expected parse errors for malformed input, got none.\nInput: %s", tt.input)
			}
		})
	}
}

// An unterminated comment swallows the rest of the document, so the parser says
// so rather than silently returning a tree that is missing everything after it.
func TestUnterminatedCommentIsReported(t *testing.T) {
	for _, src := range []string{
		"part def A;\n/* oops",
		"part def A;\n//* oops",
		"part def A;\n/*/",
		"part def A;\n/* oops\npart def B;\n",
	} {
		p := New(source.New("t.sysml", []byte(src)))
		p.ParseFile()
		found := false
		for _, d := range p.Diagnostics {
			if strings.Contains(d.Message, "unterminated comment") {
				found = true
			}
		}
		if !found {
			t.Errorf("ParseFile(%q) diagnostics = %v, want an unterminated comment reported", src, p.Diagnostics)
		}
	}
	// A closed comment is not reported.
	p := New(source.New("t.sysml", []byte("part def A;\n/* fine */\n//* also fine */\n")))
	p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Errorf("a closed comment produced %v", p.Diagnostics)
	}
}

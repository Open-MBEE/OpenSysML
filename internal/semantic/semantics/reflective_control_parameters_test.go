package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const controlParameterSource = `package P {
	attribute def Sig;
	port def PD;
	part def T {
		port p : PD;
		attribute x;
		action a {
			action asg assign x := 1;
			action lp for i in (1, 2) { action b1; }
			action w while true { action b2; } until false;
			action lu loop { action b3; } until false;
			action cond if true { action b4; } else { action b5; }
			action once if true { action b6; }
			action snd send new Sig() via p;
			action acc accept m : Sig via p;
			action ter terminate asg;
		}
	}
}`

// controlParameterRows are the parameters of the action at path as "name -> redefined, …" rows.
func controlParameterRows(m *Model, action *symbols.Symbol) []string {
	var rows []string
	for _, parameter := range m.nodeParameters(action) {
		row := m.nodeParameterName(action, parameter) + " ->"
		for _, target := range m.ImpliedRedefinitions(parameter) {
			row += " " + m.QualifiedNameOf(target)
		}
		rows = append(rows, row)
	}
	return rows
}

// An action written as a control form holds the parameters its notation states, each redefining the
// parameter of the form's kind (SysML 8.4.12, Table 33; KerML 8.4.4.6 for the positional ones).
func TestControlActionsHoldTheParametersTheirNotationStates(t *testing.T) {
	m, root := buildModelWithStdlib(t, controlParameterSource)
	for _, tc := range []struct {
		action string
		base   []string
		want   []string
	}{
		{"asg", []string{"Actions::Action::assignments"}, []string{
			"target -> Actions::Action::assignments::target",
			"replacementValues -> Actions::AssignmentAction::replacementValues",
		}},
		{"lp", []string{"Actions::Action::forLoops"}, []string{
			"var -> Actions::ForLoopAction::var",
			"seq -> Actions::ForLoopAction::seq",
			"body -> Actions::ForLoopAction::body",
		}},
		{"w", []string{"Actions::Action::whileLoops"}, []string{
			"whileTest -> Actions::WhileLoopAction::whileTest",
			"body -> Actions::WhileLoopAction::body",
			"untilTest -> Actions::WhileLoopAction::untilTest",
		}},
		{"lu", []string{"Actions::Action::whileLoops"}, []string{
			"whileTest -> Actions::WhileLoopAction::whileTest",
			"body -> Actions::WhileLoopAction::body",
			"untilTest -> Actions::WhileLoopAction::untilTest",
		}},
		{"cond", []string{"Actions::ifThenElseActions", "Actions::Action::ifSubactions"}, []string{
			"ifTest -> Actions::IfThenElseAction::ifTest Actions::IfThenAction::ifTest",
			"thenClause -> Actions::IfThenElseAction::thenClause Actions::IfThenAction::thenClause",
			"elseClause -> Actions::IfThenElseAction::elseClause",
		}},
		{"once", []string{"Actions::Action::ifSubactions"}, []string{
			"ifTest -> Actions::IfThenAction::ifTest",
			"thenClause -> Actions::IfThenAction::thenClause",
		}},
		{"snd", []string{"Actions::Action::sendSubactions"}, []string{
			"payload -> Actions::SendAction::payload",
			"sender -> Transfers::SendPerformance::sender",
		}},
		{"acc", []string{"Actions::Action::acceptSubactions"}, []string{
			"payload -> Actions::AcceptMessageAction::payload",
			"receiver -> Transfers::AcceptPerformance::receiver",
		}},
		{"ter", []string{"Actions::Action::terminateSubactions"}, []string{
			"terminatedOccurrence -> Actions::terminateActions::terminatedOccurrence",
		}},
	} {
		action := nestedSym(t, root, "P::T::a::"+tc.action)
		// Asked first, before anything else has read the action: the answer does not depend on the order of reads.
		if got := controlParameterRows(m, action); !slices.Equal(got, tc.want) {
			t.Errorf("%s parameters = %q, want %q", tc.action, got, tc.want)
		}
		var base []string
		for _, general := range m.ImpliedSpecializations(action) {
			base = append(base, m.QualifiedNameOf(general))
		}
		if !slices.Equal(base, tc.base) {
			t.Errorf("%s implied specializations = %q, want %q", tc.action, base, tc.base)
		}
	}
}

// An assignment's target owns startingAt, which owns accessedFeature, redefining the access
// performance's features and, for accessedFeature, the feature assigned (SysML Table 33).
func TestAssignmentTargetHoldsTheAccessedFeature(t *testing.T) {
	m, root := buildModelWithStdlib(t, controlParameterSource)
	asg := nestedSym(t, root, "P::T::a::asg")
	target := m.nodeParameters(asg)[0]
	startingAt := m.accessFeatures(target)
	if len(startingAt) != 1 {
		t.Fatalf("target owns %d access features, want startingAt", len(startingAt))
	}
	accessed := m.accessFeatures(startingAt[0])
	if len(accessed) != 1 {
		t.Fatalf("startingAt owns %d access features, want accessedFeature", len(accessed))
	}
	names := func(syms []*symbols.Symbol) []string {
		var out []string
		for _, sym := range syms {
			out = append(out, m.QualifiedNameOf(sym))
		}
		return out
	}
	if got, want := names(m.ImpliedRedefinitions(startingAt[0])), []string{assignmentStartingAtFQN}; !slices.Equal(got, want) {
		t.Errorf("startingAt redefines %q, want %q", got, want)
	}
	if got, want := names(m.ImpliedRedefinitions(accessed[0])), []string{assignmentAccessedFeatureFQN, "P::T::x"}; !slices.Equal(got, want) {
		t.Errorf("accessedFeature redefines %q, want %q", got, want)
	}
	for _, feature := range []*symbols.Symbol{target, startingAt[0], accessed[0]} {
		if meta := m.MetaclassOf(feature); meta == nil || meta.Name != "ReferenceUsage" {
			t.Errorf("%s is a %v, want a ReferenceUsage", m.EffectiveNameOf(feature), meta)
		}
		if !slices.Contains(m.ownedElementsOf(m.ownerOf(feature)), feature) {
			t.Errorf("%s is not an owned element of its owner", m.EffectiveNameOf(feature))
		}
	}
}

// What a control action's parameters redefine is what inheritance and derived names read: the
// action does not inherit the parameter of its kind beside its own, and its own takes that name.
func TestControlParametersMaskAndNameAsTheirRedefinitionsSay(t *testing.T) {
	m, root := buildModelWithStdlib(t, controlParameterSource)
	for _, name := range []string{"asg", "lp", "w", "lu", "cond", "once", "snd", "acc", "ter"} {
		action := nestedSym(t, root, "P::T::a::"+name)
		members := m.MembersOf(action)
		for _, parameter := range m.nodeParameters(action) {
			role := m.nodeParameterName(action, parameter)
			for _, target := range m.ImpliedRedefinitions(parameter) {
				if slices.Contains(members, target) {
					t.Errorf("%s inherits %s, which its %s redefines", name, m.QualifiedNameOf(target), role)
				}
			}
			if parameter.Named() || parameter.Kind == symbols.SymbolExpression {
				continue
			}
			if got := m.EffectiveNameOf(parameter); got != role {
				t.Errorf("%s parameter is named %q, want %q", name, got, role)
			}
			if !m.NamedByNodeParameterRole(parameter) {
				t.Errorf("%s::%s is not reported as named by its role", name, role)
			}
		}
	}
	// A loop written without `until` states no untilTest, and inherits the one of its kind.
	plain, plainRoot := buildModelWithStdlib(t, `package Q { action def A { action w while true { } } }`)
	w := nestedSym(t, plainRoot, "Q::A::w")
	until, ok := plain.LookupMember(w, "untilTest")
	if !ok || plain.QualifiedNameOf(until) != "Actions::WhileLoopAction::untilTest" {
		t.Errorf("a loop without until inherits %v, want Actions::WhileLoopAction::untilTest", until)
	}
	if !plain.IsControlKindMember(w, until) {
		t.Error("the inherited untilTest is not reported as a member of the loop's kind")
	}
}

// A parameter is valued by the expression the statement writes, the node lowering reads, and is
// featured by the snapshots of its action, as any variable feature is (KerML 8.4.4.3).
func TestControlParametersAreValuedByTheWrittenExpressions(t *testing.T) {
	m, root := buildModelWithStdlib(t, controlParameterSource)
	for name, valued := range map[string][]string{
		"asg": {"replacementValues"},
		"lp":  {"seq"},
		"snd": {"payload", "sender"},
		"acc": {"receiver"},
		"ter": {"terminatedOccurrence"},
	} {
		action := nestedSym(t, root, "P::T::a::"+name)
		var got []string
		for _, parameter := range m.nodeParameters(action) {
			value := m.ValueExpression(parameter)
			if value == nil {
				continue
			}
			if value.Decl == nil {
				t.Errorf("%s::%s is valued by no written expression", name, m.nodeParameterName(action, parameter))
			}
			got = append(got, m.nodeParameterName(action, parameter))
			featuring := m.ImpliedTypeFeaturings(parameter)
			if len(featuring) != 1 || featuring[0].Type != action || !featuring[0].Snapshots {
				t.Errorf("%s::%s is featured by %+v, want the snapshots of %s", name, got[len(got)-1], featuring, name)
			}
		}
		if !slices.Equal(got, valued) {
			t.Errorf("%s valued parameters = %q, want %q", name, got, valued)
		}
	}
}

// A loop whose body is written as an action usage holds that usage as its body parameter: the one
// symbol the body's scope resolves and lowering runs, not a second element beside it.
func TestLoopBodyParameterIsTheActionWrittenAsTheBody(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
	action a {
		action w while true action step { action inner; }
		action lp for i in (1, 2) action each { action inner; }
	}
}`)
	for _, tc := range []struct{ loop, body, redefined string }{
		{"w", "step", "Actions::WhileLoopAction::body"},
		{"lp", "each", "Actions::ForLoopAction::body"},
	} {
		loop := nestedSym(t, root, "P::a::"+tc.loop)
		declared, found := m.statementScope(loop).LookupLocal(tc.body)
		if !found {
			t.Fatalf("%s: body %s is not a member of the loop's scope", tc.loop, tc.body)
		}
		params := m.nodeParameters(loop)
		if got := params[len(params)-1]; got != declared {
			t.Errorf("%s: body parameter = %s, want the declared %s", tc.loop, m.QualifiedNameOf(got), tc.body)
		}
		var redefined []string
		for _, target := range m.ImpliedRedefinitions(declared) {
			redefined = append(redefined, m.QualifiedNameOf(target))
		}
		if !slices.Equal(redefined, []string{tc.redefined}) {
			t.Errorf("%s: %s redefines %v, want %s", tc.loop, tc.body, redefined, tc.redefined)
		}
		if name, _ := m.EffectiveNamed(declared); name != tc.body {
			t.Errorf("%s: body is named %q, want its declared name %q", tc.loop, name, tc.body)
		}
	}
}

// An object of an action node holds no slot for a parameter its notation states unnamed: the library's
// `payload = acceptedTransfer.payload` would otherwise bind a feature the executor runs from the statement.
func TestUnnamedNodeParametersAreNoShapeFeatures(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
	private import SI::*;
	attribute def Sig;
	part def T {
		port p;
		action a {
			first start;
			then action wait accept after 2 [s];
			then action snd send new Sig() via p;
		}
	}
}`)
	for _, node := range []string{"wait", "snd"} {
		action := nestedSym(t, root, "P::T::a::"+node)
		if len(m.nodeParameters(action)) == 0 {
			t.Fatalf("%s holds no parameter", node)
		}
		for _, feature := range m.ShapeFeatures(action) {
			if m.NamedByNodeParameterRole(feature.Symbol) || m.NamedByNodeParameterRole(feature.Declared) {
				t.Errorf("%s: the shape holds its parameter as %s", node, feature.Name)
			}
		}
	}
}

// The value of a control action's parameter is the node its statement states, the one lowering
// hands the executor (lower.Assign.Value, lower.Loop's collection): no second expression stands beside it.
func TestControlParameterValuesAreTheStatementsOwnNodes(t *testing.T) {
	m, root := buildModelWithStdlib(t, controlParameterSource)
	value := func(action *symbols.Symbol, name string) ast.Node {
		for _, parameter := range m.nodeParameters(action) {
			if m.nodeParameterName(action, parameter) == name {
				if expr := m.ValueExpression(parameter); expr != nil {
					return expr.Decl
				}
			}
		}
		return nil
	}
	asg := nestedSym(t, root, "P::T::a::asg")
	if stmt := controlStatementOf(asg.Decl).(*ast.AssignmentActionNode); value(asg, "replacementValues") != stmt.Value {
		t.Errorf("asg::replacementValues is not valued by the statement's value")
	}
	lp := nestedSym(t, root, "P::T::a::lp")
	if stmt := controlStatementOf(lp.Decl).(*ast.WhileLoopActionNode); value(lp, "seq") != stmt.Collection {
		t.Errorf("lp::seq is not valued by the statement's collection")
	}
}

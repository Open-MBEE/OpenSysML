package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// parseOneMember parses src and returns the single unwrapped top-level member.
func parseOneMember(t *testing.T, src string) ast.Node {
	t.Helper()
	p := New(source.New("<t>", []byte(src)))
	root := p.ParseFile()
	if len(root.Members) != 1 {
		t.Fatalf("%q: expected 1 member, got %d", src, len(root.Members))
	}
	m := root.Members[0]
	if mem, ok := m.(*ast.Membership); ok {
		return mem.Member
	}
	return m
}

func TestParseDefinitionDispatch(t *testing.T) {
	def, ok := parseOneMember(t, "part def Vehicle;").(*ast.Definition)
	if !ok {
		t.Fatalf("expected *ast.Definition")
	}
	if def.Kind != ast.DefPart || def.Ident.Name != "Vehicle" {
		t.Fatalf("got kind=%v name=%q", def.Kind, def.Ident.Name)
	}
	if def.HasBody {
		t.Fatalf("expected no body")
	}
}

func TestParseAttributeDefAndModifiers(t *testing.T) {
	def := parseOneMember(t, "abstract variation attribute def Mass;").(*ast.Definition)
	if def.Kind != ast.DefAttribute || !def.IsAbstract || !def.IsVariation {
		t.Fatalf("got kind=%v abstract=%v variation=%v", def.Kind, def.IsAbstract, def.IsVariation)
	}
}

func TestParseUsageDispatch(t *testing.T) {
	u, ok := parseOneMember(t, "part engine;").(*ast.Usage)
	if !ok {
		t.Fatalf("expected *ast.Usage")
	}
	if u.Kind != ast.UsagePart || u.Ident.Name != "engine" {
		t.Fatalf("got kind=%v name=%q", u.Kind, u.Ident.Name)
	}
}

func TestParseUsageModifiers(t *testing.T) {
	u := parseOneMember(t, "ref in composite derived ordered nonunique part p;").(*ast.Usage)
	if !u.IsReference || u.Direction != ast.DirIn || !u.IsComposite || !u.IsDerived || !u.IsOrdered || !u.IsNonunique {
		t.Fatalf("modifier flags wrong: %+v", u)
	}
	if u.Kind != ast.UsagePart {
		t.Fatalf("got kind=%v", u.Kind)
	}
}

func TestParseAnonymousUsage(t *testing.T) {
	u := parseOneMember(t, "attribute;").(*ast.Usage)
	if u.Kind != ast.UsageAttribute || u.Ident.Name != "" {
		t.Fatalf("got kind=%v name=%q", u.Kind, u.Ident.Name)
	}
}

func relTargets(rels []*ast.Relationship) []string {
	out := make([]string, len(rels))
	for i, r := range rels {
		var parts string
		// Unwrap FeatureReference to get underlying QualifiedName
		target := r.Target
		if ref, ok := target.(*ast.FeatureReference); ok {
			target = ref.Name
		}

		if qn, ok := target.(*ast.QualifiedName); ok {
			for j, seg := range qn.Parts {
				if j > 0 {
					parts += "::"
				}
				parts += seg.Text
			}
		} else {
			parts = "(expr)"
		}
		out[i] = parts
	}
	return out
}

func TestParseDefinitionSpecializes(t *testing.T) {
	def := parseOneMember(t, "part def Car specializes Vehicle, Machine;").(*ast.Definition)
	if len(def.Relationships) != 2 {
		t.Fatalf("expected 2 relationships, got %d", len(def.Relationships))
	}
	for _, r := range def.Relationships {
		if r.Kind != ast.RelSpecializes {
			t.Fatalf("expected RelSpecializes, got %v", r.Kind)
		}
	}
	if got := relTargets(def.Relationships); got[0] != "Vehicle" || got[1] != "Machine" {
		t.Fatalf("targets=%v", got)
	}
}

func TestParseDefinitionSpecializesSymbol(t *testing.T) {
	def := parseOneMember(t, "part def Car :> Vehicle;").(*ast.Definition)
	if len(def.Relationships) != 1 || def.Relationships[0].Kind != ast.RelSpecializes {
		t.Fatalf("rels=%+v", def.Relationships)
	}
}

func TestParseUsageTypingAndSubsets(t *testing.T) {
	u := parseOneMember(t, "part engine : Engine subsets vehicle::parts;").(*ast.Usage)
	if len(u.Relationships) != 2 {
		t.Fatalf("expected 2 relationships, got %d", len(u.Relationships))
	}
	if u.Relationships[0].Kind != ast.RelTyping {
		t.Fatalf("first should be RelTyping, got %v", u.Relationships[0].Kind)
	}
	if u.Relationships[1].Kind != ast.RelSubsets {
		t.Fatalf("second should be RelSubsets, got %v", u.Relationships[1].Kind)
	}
}

func TestParseUsageSubsetsSymbol(t *testing.T) {
	u := parseOneMember(t, "part p :> q;").(*ast.Usage)
	if len(u.Relationships) != 1 || u.Relationships[0].Kind != ast.RelSubsets {
		t.Fatalf("rels=%+v", u.Relationships)
	}
}

func TestParseUsageRedefinesReferencesCrosses(t *testing.T) {
	u := parseOneMember(t, "part p :>> a ::> b => c;").(*ast.Usage)
	if len(u.Relationships) != 3 {
		t.Fatalf("expected 3 relationships, got %d", len(u.Relationships))
	}
	want := []ast.RelationshipKind{ast.RelRedefines, ast.RelReferences, ast.RelCrosses}
	for i, r := range u.Relationships {
		if r.Kind != want[i] {
			t.Fatalf("rel[%d]=%v want %v", i, r.Kind, want[i])
		}
	}
}

func TestParseUsageMultiplicityRange(t *testing.T) {
	u := parseOneMember(t, "part wheels [4];").(*ast.Usage)
	if u.Multiplicity == nil || u.Multiplicity.IsRange {
		t.Fatalf("expected single-bound multiplicity, got %+v", u.Multiplicity)
	}
	if _, ok := u.Multiplicity.Lower.(*ast.LiteralInteger); !ok {
		t.Fatalf("lower should be LiteralInteger, got %T", u.Multiplicity.Lower)
	}
}

func TestParseUsageMultiplicityStarRange(t *testing.T) {
	u := parseOneMember(t, "part parts [0..*];").(*ast.Usage)
	if u.Multiplicity == nil || !u.Multiplicity.IsRange {
		t.Fatalf("expected range multiplicity, got %+v", u.Multiplicity)
	}
	if _, ok := u.Multiplicity.Upper.(*ast.LiteralInfinity); !ok {
		t.Fatalf("upper should be LiteralInfinity, got %T", u.Multiplicity.Upper)
	}
}

func TestParseUsageValue(t *testing.T) {
	u := parseOneMember(t, "attribute mass = 1500;").(*ast.Usage)
	if u.Value == nil {
		t.Fatalf("expected value expression")
	}
	if _, ok := u.Value.(*ast.LiteralInteger); !ok {
		t.Fatalf("value should be LiteralInteger, got %T", u.Value)
	}
}

func TestParseUsageMultiplicityThenValue(t *testing.T) {
	u := parseOneMember(t, "attribute xs [3] = 7;").(*ast.Usage)
	if u.Multiplicity == nil || u.Value == nil {
		t.Fatalf("expected both multiplicity and value, got mult=%v value=%v", u.Multiplicity, u.Value)
	}
}

func TestParseDefinitionNestedBody(t *testing.T) {
	def := parseOneMember(t, "part def Car { part engine; attribute mass; }").(*ast.Definition)
	if !def.HasBody {
		t.Fatalf("expected body")
	}
	if len(def.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(def.Members))
	}
	m0 := def.Members[0].(*ast.Membership).Member.(*ast.Usage)
	if m0.Kind != ast.UsagePart {
		t.Fatalf("member[0] kind=%v", m0.Kind)
	}
	m1 := def.Members[1].(*ast.Membership).Member.(*ast.Usage)
	if m1.Kind != ast.UsageAttribute {
		t.Fatalf("member[1] kind=%v", m1.Kind)
	}
}

func TestParseDefinitionBodyMixedMembers(t *testing.T) {
	def := parseOneMember(t, "part def Car { part wheel; comment /* c */ }").(*ast.Definition)
	if len(def.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(def.Members))
	}
}

func TestParseUsageNestedBody(t *testing.T) {
	u := parseOneMember(t, "part car { part engine; }").(*ast.Usage)
	if !u.HasBody || len(u.Members) != 1 {
		t.Fatalf("expected 1 body member, got hasBody=%v members=%d", u.HasBody, len(u.Members))
	}
}

func TestParseDefinitionBodyVisibility(t *testing.T) {
	def := parseOneMember(t, "part def Car { private part secret; }").(*ast.Definition)
	m := def.Members[0].(*ast.Membership)
	if m.Visibility != ast.VisibilityPrivate {
		t.Fatalf("expected private, got %v", m.Visibility)
	}
}

// TestParseActionUsageIntegration verifies parseActionBodyMixed is invoked for action usages.
func TestParseActionUsageIntegration(t *testing.T) {
	src := `action example {
		first startNode;
		done;
		succession first startNode then endNode;
	}`
	u, ok := parseOneMember(t, src).(*ast.Usage)
	if !ok {
		t.Fatalf("expected *ast.Usage")
	}
	if u.Kind != ast.UsageAction {
		t.Fatalf("expected UsageAction, got %v", u.Kind)
	}
	if !u.HasBody {
		t.Fatalf("expected HasBody=true")
	}
	if len(u.Members) != 3 {
		t.Fatalf("expected 3 action members, got %d", len(u.Members))
	}

	// Verify InitialNode
	init, ok := u.Members[0].(*ast.InitialNode)
	if !ok || init.Name() != "startNode" {
		t.Fatalf("member[0]: expected InitialNode startNode, got %T %+v", u.Members[0], u.Members[0])
	}

	// Verify FinalNode
	if _, ok := u.Members[1].(*ast.FinalNode); !ok {
		t.Fatalf("member[1]: expected FinalNode, got %T", u.Members[1])
	}

	// Verify Succession
	membership, ok := u.Members[2].(*ast.Membership)
	if !ok {
		t.Fatalf("member[2]: expected Membership, got %T", u.Members[2])
	}
	edge, ok := membership.Member.(*ast.Usage)
	if !ok || edge.Kind != ast.UsageSuccession {
		t.Fatalf("member[2]: expected succession Usage, got %T", membership.Member)
	}
	// QualifiedName.Parts[0].Text holds the identifier
	if len(edge.ConnectorEnds) != 2 {
		t.Fatalf("succession has %d ends, want 2", len(edge.ConnectorEnds))
	}
	source, ok := edge.ConnectorEnds[0].Target.(*ast.QualifiedName)
	if !ok || len(source.Parts) == 0 || source.Parts[0].Text != "startNode" {
		t.Fatalf("succession source: expected startNode, got %+v", edge.ConnectorEnds[0].Target)
	}
	target, ok := edge.ConnectorEnds[1].Target.(*ast.QualifiedName)
	if !ok || len(target.Parts) == 0 || target.Parts[0].Text != "endNode" {
		t.Fatalf("succession target: expected endNode, got %+v", edge.ConnectorEnds[1].Target)
	}
}

func TestParseEndModifier(t *testing.T) {
	// Anonymous usage with end modifier: end source: Anything;
	u := parseOneMember(t, "end source: Anything;").(*ast.Usage)
	if !u.IsEnd {
		t.Fatalf("expected IsEnd = true, got %v", u.IsEnd)
	}
	if u.Ident.Name != "source" {
		t.Fatalf("expected name 'source', got %q", u.Ident.Name)
	}
	if u.Kind != ast.UsageAttribute {
		t.Fatalf("expected UsageAttribute, got %v", u.Kind)
	}

	// Explicit kind with end modifier: end part x;
	u2 := parseOneMember(t, "end part x;").(*ast.Usage)
	if !u2.IsEnd {
		t.Fatalf("expected IsEnd = true, got %v", u2.IsEnd)
	}
	if u2.Ident.Name != "x" {
		t.Fatalf("expected name 'x', got %q", u2.Ident.Name)
	}
}

func TestParseEnumLiterals(t *testing.T) {
	src := `enum def LevelEnum {
    low = 0.25;
    medium = 0.50;
    high = 0.75;
}`
	p := New(source.New("test.sysml", []byte(src)))
	f := p.ParseFile()

	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Logf("diagnostic: %s", d.Message)
		}
		t.Fatalf("parse failed with %d diagnostics", len(p.Diagnostics))
	}

	// Check enum def
	if len(f.Members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(f.Members))
	}

	mem, ok := f.Members[0].(*ast.Membership)
	if !ok {
		t.Fatalf("expected Membership, got %T", f.Members[0])
	}

	def, ok := mem.Member.(*ast.Definition)
	if !ok {
		t.Fatalf("expected Definition, got %T", mem.Member)
	}

	if def.Kind != ast.DefEnumeration {
		t.Fatalf("expected DefEnumeration, got %v", def.Kind)
	}

	// Check enum literals
	if len(def.Members) != 3 {
		t.Fatalf("expected 3 body members, got %d", len(def.Members))
	}

	t.Logf("enum body: %#v", def.Members)
}

func TestParseEnumLiteralsWithTyping(t *testing.T) {
	src := `enum def LevelEnum :> Level {
    low = 0.25;
    medium = 0.50;
    high = 0.75;
}`
	p := New(source.New("test.sysml", []byte(src)))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Logf("diagnostic: %s", d.Message)
		}
		t.Fatalf("parse failed with %d diagnostics", len(p.Diagnostics))
	}

	t.Logf("parsed cleanly")
}

// BodyAdmitsMember answers, for a declaration's body, what the parser would
// accept in it; MemberOwner names the body kind a confined member wants.
func TestBodyAdmitsMember(t *testing.T) {
	decl := func(src string) ast.Node { return parseOneMember(t, src) }
	for _, tc := range []struct {
		owner ast.Node
		kw    string
		want  bool
	}{
		{decl("requirement def R;"), "subject", true},
		{decl("requirement r;"), "actor", true},
		{decl("verification def V;"), "objective", true},
		{decl("verification def V;"), "stakeholder", false},
		{decl("concern def C;"), "stakeholder", true},
		{decl("requirement def R;"), "objective", false},
		{decl("part def P;"), "subject", false},
		{decl("part def P;"), "part", true},
		{decl("package K;"), "part", true},
		{decl("package K;"), "actor", false},
		{nil, "part", true},
		{nil, "subject", false},
	} {
		if got := BodyAdmitsMember(tc.owner, tc.kw); got != tc.want {
			t.Errorf("BodyAdmitsMember(%T, %q) = %v, want %v", tc.owner, tc.kw, got, tc.want)
		}
	}
	for kw, want := range map[string]string{"subject": "requirement or case", "objective": "case", "part": ""} {
		if got := MemberOwner(kw); got != want {
			t.Errorf("MemberOwner(%q) = %q, want %q", kw, got, want)
		}
	}
}

func TestBodyKindsForAuthoring(t *testing.T) {
	for _, tc := range []struct {
		src             string
		calculation     bool
		requirementBody bool
	}{
		{"calc def C;", true, false},
		{"calc c;", true, false},
		{"case def C;", true, false},
		{"verification def V;", true, false},
		{"constraint c;", true, false},
		{"constraint def K;", true, false},
		{"requirement def R;", false, true},
		{"requirement r;", false, true},
		{"concern def C;", false, true},
		{"part def P;", false, false},
		{"action def A;", false, false},
	} {
		owner := parseOneMember(t, tc.src)
		if got := BodyIsCalculation(owner); got != tc.calculation {
			t.Errorf("BodyIsCalculation(%q) = %t, want %t", tc.src, got, tc.calculation)
		}
		if got := BodyIsRequirement(owner); got != tc.requirementBody {
			t.Errorf("BodyIsRequirement(%q) = %t, want %t", tc.src, got, tc.requirementBody)
		}
	}
}

func TestSubstateUsesStateBodyContext(t *testing.T) {
	owner := parseOneMember(t, "state def S { state toasting; }").(*ast.Definition)
	substate := ast.DeclMembers(owner)[0]
	if membership, ok := substate.(*ast.Membership); ok {
		substate = membership.Member
	}
	if _, ok := substate.(*ast.SubstateMember); !ok {
		t.Fatalf("state body member = %T, want *ast.SubstateMember", substate)
	}
	if got := declarationBodyContext(substate); got != usageBodyContext(ast.UsageState) {
		t.Errorf("substate body context = %v, want state usage context %v",
			got, usageBodyContext(ast.UsageState))
	}
	if !BodyAdmitsBehaviorUsage(substate) {
		t.Error("substate body does not admit behavior usages")
	}
	if BodyIsCalculation(substate) {
		t.Error("substate body is classified as a calculation")
	}
	if !BodyAdmitsMember(substate, "entry") || !BodyAdmitsMember(substate, "transition") {
		t.Error("substate body does not admit state members")
	}
}

func TestBodyAdmitsBehaviorUsage(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want bool
	}{
		{"package P;", true},
		{"part def P;", true},
		{"enum def E;", false},
		{"metadata M;", false},
	} {
		owner := parseOneMember(t, tc.src)
		if got := BodyAdmitsBehaviorUsage(owner); got != tc.want {
			t.Errorf("BodyAdmitsBehaviorUsage(%q) = %t, want %t", tc.src, got, tc.want)
		}
	}
}

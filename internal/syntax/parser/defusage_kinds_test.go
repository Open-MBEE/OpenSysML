package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// parseOneMemberWithDiags parses src and returns the single unwrapped top-level
// member together with the parser diagnostics.
func parseOneMemberWithDiags(t *testing.T, src string) (ast.Node, []Diagnostic) {
	t.Helper()
	p := New(source.New("<t>", []byte(src)))
	root := p.ParseFile()
	if len(root.Members) != 1 {
		t.Fatalf("%q: expected 1 member, got %d", src, len(root.Members))
	}
	m := root.Members[0]
	if mem, ok := m.(*ast.Membership); ok {
		return mem.Member, p.Diagnostics
	}
	return m, p.Diagnostics
}

func TestParseTierADefDispatch(t *testing.T) {
	cases := map[string]ast.DefinitionKind{
		"item def X;":       ast.DefItem,
		"occurrence def X;": ast.DefOccurrence,
		"individual def X;": ast.DefIndividual,
		"metadata def X;":   ast.DefMetadata,
		"enum def X;":       ast.DefEnumeration,
		"view def X;":       ast.DefView,
		"viewpoint def X;":  ast.DefViewpoint,
		"rendering def X;":  ast.DefRendering,
		"concern def X;":    ast.DefConcern,
	}
	for src, want := range cases {
		def, ok := parseOneMember(t, src).(*ast.Definition)
		if !ok {
			t.Fatalf("%q: expected *ast.Definition", src)
		}
		if def.Kind != want {
			t.Fatalf("%q: kind = %v, want %v", src, def.Kind, want)
		}
	}
}

func TestParseTierAUsageDispatch(t *testing.T) {
	cases := map[string]ast.UsageKind{
		"item x;":       ast.UsageItem,
		"occurrence x;": ast.UsageOccurrence,
		"enum x;":       ast.UsageEnumeration,
		"view x;":       ast.UsageView,
		"concern x;":    ast.UsageConcern,
	}
	for src, want := range cases {
		u, ok := parseOneMember(t, src).(*ast.Usage)
		if !ok {
			t.Fatalf("%q: expected *ast.Usage", src)
		}
		if u.Kind != want {
			t.Fatalf("%q: kind = %v, want %v", src, u.Kind, want)
		}
	}
}

func TestParseTierCDispatchWithGenericBody(t *testing.T) {
	cases := map[string]ast.DefinitionKind{
		"action def A { part p; }":        ast.DefAction,
		"state def S { part p; }":         ast.DefState,
		"calc def C { part p; }":          ast.DefCalc,
		"constraint def K { part p; }":    ast.DefConstraint,
		"requirement def R { part p; }":   ast.DefRequirement,
		"case def C2 { part p; }":         ast.DefCase,
		"analysis def AC { part p; }":     ast.DefAnalysisCase,
		"verification def VC { part p; }": ast.DefVerificationCase,
	}
	for src, want := range cases {
		def, ok := parseOneMember(t, src).(*ast.Definition)
		if !ok {
			t.Fatalf("%q: expected *ast.Definition", src)
		}
		if def.Kind != want {
			t.Fatalf("%q: kind = %v, want %v", src, def.Kind, want)
		}
		if !def.HasBody || len(def.Members) != 1 {
			t.Fatalf("%q: expected generic body with 1 member, got hasBody=%v members=%d", src, def.HasBody, len(def.Members))
		}
	}
}

func TestParseUseCaseTwoWord(t *testing.T) {
	def, ok := parseOneMember(t, "use case def Login;").(*ast.Definition)
	if !ok || def.Kind != ast.DefUseCase || def.Ident.Name != "Login" {
		t.Fatalf("use case def: got %+v", def)
	}
	u, ok := parseOneMember(t, "use case checkout;").(*ast.Usage)
	if !ok || u.Kind != ast.UsageUseCase || u.Ident.Name != "checkout" {
		t.Fatalf("use case usage: got %+v", u)
	}
}

func TestParseConnectionBinaryEnds(t *testing.T) {
	u := parseOneMember(t, "connection c connect a to b;").(*ast.Usage)
	if u.Kind != ast.UsageConnection {
		t.Fatalf("kind = %v", u.Kind)
	}
	if len(u.ConnectorEnds) != 2 {
		t.Fatalf("expected 2 ends, got %d", len(u.ConnectorEnds))
	}
	// After stdlib coverage work: parseConnectorEnd uses parseRelationshipTarget, producing QualifiedName
	qn0, ok0 := u.ConnectorEnds[0].Target.(*ast.QualifiedName)
	qn1, ok1 := u.ConnectorEnds[1].Target.(*ast.QualifiedName)
	if !ok0 || !ok1 || qn0.Parts[0].Text != "a" || qn1.Parts[0].Text != "b" {
		t.Fatalf("ends: %T / %T", u.ConnectorEnds[0].Target, u.ConnectorEnds[1].Target)
	}
}

func TestParseConnectionNaryEnds(t *testing.T) {
	u := parseOneMember(t, "connection c connect (a, b, c);").(*ast.Usage)
	if len(u.ConnectorEnds) != 3 {
		t.Fatalf("expected 3 ends, got %d", len(u.ConnectorEnds))
	}
}

func TestParseAllocationEnds(t *testing.T) {
	u := parseOneMember(t, "allocation al allocate f to g;").(*ast.Usage)
	if u.Kind != ast.UsageAllocation || len(u.ConnectorEnds) != 2 {
		t.Fatalf("allocation ends wrong: kind=%v ends=%d", u.Kind, len(u.ConnectorEnds))
	}
}

// `allocation al;` names an allocation usage and `allocate f to g;` states its
// ends without naming it; both are legal (D1, SysML.xtext:1219-1222).
func TestParseAllocationSpellings(t *testing.T) {
	named := parseOneMember(t, "allocation al;").(*ast.Usage)
	if named.Kind != ast.UsageAllocation || named.Ident.Name != "al" || len(named.ConnectorEnds) != 0 {
		t.Fatalf("named allocation wrong: kind=%v name=%q ends=%d", named.Kind, named.Ident.Name, len(named.ConnectorEnds))
	}
	ends := parseOneMember(t, "allocate f to g;").(*ast.Usage)
	if ends.Kind != ast.UsageAllocation || len(ends.ConnectorEnds) != 2 {
		t.Fatalf("allocate ends wrong: kind=%v ends=%d", ends.Kind, len(ends.ConnectorEnds))
	}
}

func TestParseInterfaceEnds(t *testing.T) {
	u := parseOneMember(t, "interface i connect p to q;").(*ast.Usage)
	if u.Kind != ast.UsageInterface || len(u.ConnectorEnds) != 2 {
		t.Fatalf("interface ends wrong: kind=%v ends=%d", u.Kind, len(u.ConnectorEnds))
	}
}

func TestParseFlowFromTo(t *testing.T) {
	u := parseOneMember(t, "flow f from a to b;").(*ast.Usage)
	if u.FlowEnds == nil || u.FlowEnds.From == nil || u.FlowEnds.To == nil {
		t.Fatalf("flow ends wrong: %+v", u.FlowEnds)
	}
	if u.FlowEnds.Payload != nil {
		t.Fatalf("expected no payload")
	}
}

func TestParseFlowWithPayload(t *testing.T) {
	u := parseOneMember(t, "flow f of Fuel from a to b;").(*ast.Usage)
	if u.FlowEnds == nil || u.FlowEnds.Payload == nil {
		t.Fatalf("expected payload, got %+v", u.FlowEnds)
	}
	qn, ok := u.FlowEnds.Payload.(*ast.QualifiedName)
	if !ok {
		t.Fatalf("payload expected QualifiedName, got %T", u.FlowEnds.Payload)
	}
	if qn.Parts[0].Text != "Fuel" {
		t.Fatalf("payload = %v", qn)
	}
}

func TestParseFlowShorthand(t *testing.T) {
	u := parseOneMember(t, "flow a to b;").(*ast.Usage)
	if u.FlowEnds == nil || u.FlowEnds.From == nil || u.FlowEnds.To == nil {
		t.Fatalf("shorthand flow ends wrong: %+v", u.FlowEnds)
	}
}

func TestParsePortConjugation(t *testing.T) {
	u := parseOneMember(t, "port p : ~ PortDef;").(*ast.Usage)
	if u.Kind != ast.UsagePort {
		t.Fatalf("expected port usage, got kind=%v", u.Kind)
	}
	if len(u.Relationships) != 1 || u.Relationships[0].Kind != ast.RelTyping {
		t.Fatalf("expected typing relationship, got %+v", u.Relationships)
	}
	if !u.Relationships[0].Conjugated {
		t.Fatal("expected the typing relationship to be conjugated")
	}
}

func TestParseMalformedConnectorEndRecovers(t *testing.T) {
	node, diags := parseOneMemberWithDiags(t, "connection c connect a to ;")
	u, ok := node.(*ast.Usage)
	if !ok {
		t.Fatalf("expected *ast.Usage even on malformed end, got %T", node)
	}
	// After stdlib coverage work: parseConnectorEnd returns nil for invalid end
	// Parser keeps only valid ends
	if len(u.ConnectorEnds) != 1 {
		t.Fatalf("expected 1 end (only valid end kept), got %d", len(u.ConnectorEnds))
	}
	if len(diags) == 0 {
		t.Fatalf("expected a parse diagnostic for the missing end")
	}
}

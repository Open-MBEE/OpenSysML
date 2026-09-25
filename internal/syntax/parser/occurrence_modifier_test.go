package parser

import (
	"fmt"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// parseUsages parses src and returns the usages declared directly in the given
// namespace member, flattening memberships.
func parseSingleUsage(t *testing.T, input string) *ast.Usage {
	t.Helper()
	sf := source.New("occurrence_modifier.sysml", []byte(input))
	p := New(sf)
	file := p.ParseFile()
	for _, d := range p.Diagnostics {
		t.Errorf("parse error: %s", d.Message)
	}
	if len(file.Members) != 1 {
		t.Fatalf("expected one namespace member, got %d", len(file.Members))
	}
	m, ok := file.Members[0].(*ast.Membership)
	if !ok {
		t.Fatalf("expected a membership, got %T", file.Members[0])
	}
	u, ok := m.Member.(*ast.Usage)
	if !ok {
		t.Fatalf("expected a usage, got %T", m.Member)
	}
	return u
}

// parseSingleBodyUsage parses src, which must declare a single definition whose
// body declares a single usage, and returns that usage.
func parseSingleBodyUsage(t *testing.T, input string) *ast.Usage {
	t.Helper()
	sf := source.New("occurrence_modifier_body.sysml", []byte(input))
	p := New(sf)
	file := p.ParseFile()
	for _, d := range p.Diagnostics {
		t.Errorf("parse error: %s", d.Message)
	}
	def, ok := file.Members[0].(*ast.Membership).Member.(*ast.Definition)
	if !ok {
		t.Fatalf("expected a definition, got %T", file.Members[0].(*ast.Membership).Member)
	}
	if len(def.Members) != 1 {
		t.Fatalf("expected one body member, got %d", len(def.Members))
	}
	u, ok := def.Members[0].(*ast.Membership).Member.(*ast.Usage)
	if !ok {
		t.Fatalf("expected a usage, got %T", def.Members[0].(*ast.Membership).Member)
	}
	return u
}

// The `individual` modifier and the `snapshot`/`timeslice` portion prefixes are
// syntax, so the parser stores them on the usage node: SysML v2 §8.3.9.11 gives
// OccurrenceUsage an `isIndividual` attribute and a `portionKind`.
func TestParseUsageOccurrenceModifiers(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantKind       ast.UsageKind
		wantIndividual bool
		wantPortion    ast.PortionKind
	}{
		// With no kind keyword after it, `individual` declares an occurrence usage
		// itself (SysML.xtext IndividualUsage returns SysML::OccurrenceUsage).
		{"individual", "individual testSystem : TestSystem;", ast.UsageIndividual, true, ast.PortionNone},
		// The declaration is optional, so the keyword alone declares one too
		// (SysML.xtext Usage: UsageDeclaration? UsageCompletion).
		{"individual_empty", "individual ;", ast.UsageIndividual, true, ast.PortionNone},
		// The modifier is orthogonal to the kind keyword, which still declares the kind.
		{"individual_part", "individual part testVehicle : TestSystem;", ast.UsagePart, true, ast.PortionNone},
		{"individual_occurrence", "individual occurrence testFlight : Flight;", ast.UsageOccurrence, true, ast.PortionNone},
		{"individual_item", "individual item testCargo : Cargo;", ast.UsageItem, true, ast.PortionNone},
		{"individual_anonymous", "individual : TestSystem;", ast.UsageIndividual, true, ast.PortionNone},
		{"snapshot", "snapshot atLiftoff : Flight;", ast.UsageOccurrence, false, ast.PortionSnapshot},
		{"snapshot_occurrence", "snapshot occurrence takeoff : Flight;", ast.UsageOccurrence, false, ast.PortionSnapshot},
		{"snapshot_part", "snapshot part vehicleAtTakeoff : Vehicle;", ast.UsagePart, false, ast.PortionSnapshot},
		{"individual_snapshot", "individual snapshot testAtLiftoff : Flight;", ast.UsageOccurrence, true, ast.PortionSnapshot},
		{"timeslice", "timeslice ascent : Flight;", ast.UsageOccurrence, false, ast.PortionTimeslice},
		{"timeslice_item", "timeslice item cargoInFlight : Cargo;", ast.UsageItem, false, ast.PortionTimeslice},
		{"timeslice_part", "timeslice part vehicleInFlight : Vehicle;", ast.UsagePart, false, ast.PortionTimeslice},
		{"individual_timeslice", "individual timeslice testAscent : Flight;", ast.UsageOccurrence, true, ast.PortionTimeslice},
		{"plain_usage", "occurrence takeoff : Flight;", ast.UsageOccurrence, false, ast.PortionNone},
		{"plain_part", "part vehicle : Vehicle;", ast.UsagePart, false, ast.PortionNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := parseSingleUsage(t, tt.input)
			if u.Kind != tt.wantKind {
				t.Errorf("kind = %v, want %v", u.Kind, tt.wantKind)
			}
			if u.IsIndividual != tt.wantIndividual {
				t.Errorf("IsIndividual = %t, want %t", u.IsIndividual, tt.wantIndividual)
			}
			if u.Portion != tt.wantPortion {
				t.Errorf("Portion = %q, want %q", u.Portion.Keyword(), tt.wantPortion.Keyword())
			}
		})
	}
}

// A modified usage in a definition body keeps its modifier: the body is the
// form these declarations actually take in a model, and the enclosing
// declaration parser must not consume `individual` as a bare kind prefix.
func TestParseUsageOccurrenceModifiersInBody(t *testing.T) {
	tests := []struct {
		name           string
		member         string
		wantKind       ast.UsageKind
		wantIndividual bool
		wantPortion    ast.PortionKind
	}{
		{"individual", "individual testSystem : TestSystem;", ast.UsageIndividual, true, ast.PortionNone},
		{"individual_part", "individual part p;", ast.UsagePart, true, ast.PortionNone},
		{"individual_occurrence", "individual occurrence o;", ast.UsageOccurrence, true, ast.PortionNone},
		{"individual_snapshot", "individual snapshot s;", ast.UsageOccurrence, true, ast.PortionSnapshot},
		{"snapshot_part", "snapshot part sp;", ast.UsagePart, false, ast.PortionSnapshot},
		{"timeslice", "timeslice ts;", ast.UsageOccurrence, false, ast.PortionTimeslice},
		{"timeslice_item", "timeslice item ti;", ast.UsageItem, false, ast.PortionTimeslice},
		{"plain_part", "part p;", ast.UsagePart, false, ast.PortionNone},
		// A feature modifier ahead of the occurrence modifier takes the member
		// down the anonymous-feature path, which must keep the modifier too.
		{"ref_individual", "ref individual v : V;", ast.UsageIndividual, true, ast.PortionNone},
		{"ref_individual_redefines", "ref individual redefines v : V;", ast.UsageIndividual, true, ast.PortionNone},
		{"ref_individual_shorthand", "ref individual :>> v : V;", ast.UsageIndividual, true, ast.PortionNone},
		{"ref_timeslice", "ref timeslice ts : T;", ast.UsageOccurrence, false, ast.PortionTimeslice},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := parseSingleBodyUsage(t, "part def C { "+tt.member+" }")
			if u.Kind != tt.wantKind {
				t.Errorf("kind = %v, want %v", u.Kind, tt.wantKind)
			}
			if u.IsIndividual != tt.wantIndividual {
				t.Errorf("IsIndividual = %t, want %t", u.IsIndividual, tt.wantIndividual)
			}
			if u.Portion != tt.wantPortion {
				t.Errorf("Portion = %q, want %q", u.Portion.Keyword(), tt.wantPortion.Keyword())
			}
		})
	}
}

// `all` is KerML.xtext's `isSufficient` prefix; SysML.xtext admits the word
// after `import` alone, so the pinned validator rejects `snapshot all s : T;`.
func TestParseBarePortionAllIsKerMLOnly(t *testing.T) {
	for _, input := range []string{"snapshot all s : Flight;", "timeslice all t : Flight;"} {
		p := New(source.New("occurrence_modifier.sysml", []byte(input)))
		p.ParseFile()
		if len(p.Diagnostics) == 0 {
			t.Errorf("%s: parsed clean, want a diagnostic", input)
		}
	}
}

// A kind keyword after a portion prefix is the portioned usage's kind, whatever
// follows it: `timeslice item : Cargo;` is an unnamed item usage, not an
// occurrence named `item`. Both portion keywords must read it the same way.
func TestParseAnonymousPortionOfTypedKind(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  ast.PortionKind
	}{
		{"snapshot item : Cargo;", ast.PortionSnapshot},
		{"timeslice item : Cargo;", ast.PortionTimeslice},
		{"snapshot item;", ast.PortionSnapshot},
		{"timeslice item;", ast.PortionTimeslice},
	} {
		u := parseSingleUsage(t, tt.input)
		if u.Kind != ast.UsageItem {
			t.Errorf("%s: kind = %v, want item", tt.input, u.Kind)
		}
		if u.Ident.Name != "" {
			t.Errorf("%s: name = %q, want no name", tt.input, u.Ident.Name)
		}
		if u.Portion != tt.want {
			t.Errorf("%s: portion = %q, want %q", tt.input, u.Portion.Keyword(), tt.want.Keyword())
		}
	}
}

// Either portion keyword prefixes an occurrence usage identically, whatever
// spelling of the declaration follows it.
func TestParsePortionKeywordsAgree(t *testing.T) {
	for _, form := range []string{"%s 'launch event';", "%s 'launch event' : Flight;", "%s : Flight;", "%s;"} {
		for kw, want := range map[string]ast.PortionKind{"snapshot": ast.PortionSnapshot, "timeslice": ast.PortionTimeslice} {
			input := fmt.Sprintf(form, kw)
			u := parseSingleUsage(t, input)
			if u.Kind != ast.UsageOccurrence {
				t.Errorf("%s: kind = %v, want occurrence", input, u.Kind)
			}
			if u.Portion != want {
				t.Errorf("%s: portion = %q, want %q", input, u.Portion.Keyword(), want.Keyword())
			}
		}
	}
}

// A directed parameter takes the same modifiers through the behavior parser's
// own parameter path, which must store them too.
func TestParseDirectionParameterOccurrenceModifiers(t *testing.T) {
	input := `action def Collect {
		in individual subjectVehicle : TestVehicle;
		in snapshot vehicleState : Flight;
		in timeslice vehicleAscent : Flight;
		out item payload : Cargo;
	}`
	sf := source.New("occurrence_modifier_params.sysml", []byte(input))
	p := New(sf)
	file := p.ParseFile()
	for _, d := range p.Diagnostics {
		t.Errorf("parse error: %s", d.Message)
	}
	def := file.Members[0].(*ast.Membership).Member.(*ast.Definition)
	want := []struct {
		name           string
		wantIndividual bool
		wantPortion    ast.PortionKind
	}{
		{"subjectVehicle", true, ast.PortionNone},
		{"vehicleState", false, ast.PortionSnapshot},
		{"vehicleAscent", false, ast.PortionTimeslice},
		{"payload", false, ast.PortionNone},
	}
	if len(def.Members) != len(want) {
		t.Fatalf("expected %d parameters, got %d", len(want), len(def.Members))
	}
	for i, w := range want {
		u := def.Members[i].(*ast.Membership).Member.(*ast.Usage)
		if u.Ident.Name != w.name {
			t.Fatalf("parameter %d is %q, want %q", i, u.Ident.Name, w.name)
		}
		if u.IsIndividual != w.wantIndividual {
			t.Errorf("%s: IsIndividual = %t, want %t", w.name, u.IsIndividual, w.wantIndividual)
		}
		if u.Portion != w.wantPortion {
			t.Errorf("%s: Portion = %q, want %q", w.name, u.Portion.Keyword(), w.wantPortion.Keyword())
		}
	}
}

package symbolfacts

import (
	"slices"
	"testing"
)

// annotatedModel annotates in each form: a prefix, a body usage, and `about`
// usages written in the enclosing package and in another one.
const annotatedModel = `
package Demo {
    metadata def FailureMode { attribute severity : ScalarValues::Integer; }
    #FailureMode part def Prefixed;
    part def Bodied { @FailureMode { severity = 3; } }
    part def Pump { part motor; }
    metadata pump_fm : FailureMode about Pump { severity = 2; }
    metadata motor_fm : FailureMode about Pump::motor;
}
package Elsewhere {
    metadata remote_fm : Demo::FailureMode about Demo::Pump;
}
`

func infoOf(t *testing.T, id string) *Info {
	t.Helper()
	idx, sc := modelWithLibrary(t, annotatedModel)
	syms := LookupNamed(idx, id)
	if len(syms) != 1 {
		t.Fatalf("%s resolves to %d symbols", id, len(syms))
	}
	return Of(syms[0], sc)
}

// A symbol names every metadata usage annotating it by the id GetSymbol
// answers, while an `about` usage stays out of its children and members.
func TestAnnotatingMetadataIdsNameEveryForm(t *testing.T) {
	for id, want := range map[string][]string{
		"Demo::Prefixed":    {"Demo::Prefixed::@0"},
		"Demo::Bodied":      {"Demo::Bodied::@0"},
		"Demo::Pump":        {"Demo::pump_fm", "Elsewhere::remote_fm"},
		"Demo::Pump::motor": {"Demo::motor_fm"},
		"Demo::pump_fm":     nil,
		"Demo":              nil,
	} {
		if got := infoOf(t, id).AnnotatingMetadataIds; !slices.Equal(got, want) {
			t.Errorf("%s: annotating metadata = %v, want %v", id, got, want)
		}
	}
	pump := infoOf(t, "Demo::Pump")
	if want := []string{"Demo::Pump::motor"}; !slices.Equal(pump.ChildIds, want) {
		t.Errorf("Demo::Pump: child ids = %v, want %v", pump.ChildIds, want)
	}
	for _, member := range pump.Members {
		if member.Kind == "metadataUsage" {
			t.Errorf("Demo::Pump lists %s as a member", member.Id)
		}
	}
}

// The usage a symbol names is an ordinary symbol: typed by the metadata
// definition, carrying the value the annotation binds.
func TestAnnotatingMetadataUsageCarriesTypeAndValue(t *testing.T) {
	usage := infoOf(t, infoOf(t, "Demo::Pump").AnnotatingMetadataIds[0])
	if usage.Kind != "metadataUsage" {
		t.Errorf("kind = %q, want metadataUsage", usage.Kind)
	}
	if usage.TypeInfo == nil || usage.TypeInfo.ResolvedId != "Demo::FailureMode" {
		t.Errorf("type = %+v, want Demo::FailureMode", usage.TypeInfo)
	}
	var severity *Attribute
	for _, attr := range usage.Attributes {
		if attr.Name == "severity" {
			severity = attr
		}
	}
	if severity == nil || severity.Value == nil || severity.Value.Const == nil || severity.Value.Const.Int != 2 {
		t.Errorf("severity = %+v, want 2", severity)
	}
}

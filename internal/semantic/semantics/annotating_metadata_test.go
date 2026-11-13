package semantics

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// annotatingNames reports the names of the metadata usages annotating sym,
// "@" standing for an unnamed one.
func annotatingNames(m *Model, sym *symbols.Symbol) []string {
	var out []string
	for _, usage := range m.AnnotatingMetadataOf(sym) {
		name := usage.Name
		if name == "" {
			name = "@"
		}
		out = append(out, name)
	}
	return out
}

// Every form of annotation names its metadata usage: the `#M` prefix and the
// body usages the element owns, then the `about` usages written elsewhere.
func TestAnnotatingMetadataCoversPrefixBodyAndAbout(t *testing.T) {
	m, _, user := buildOverlayModel(t, "metadata def Safety;", `
		metadata def Audit { attribute level : ScalarValues::Integer; }
		metadata before : Audit about pump { level = 1; }
		#Safety part pump {
			@Audit { level = 2; }
			metadata named : Audit { level = 3; }
			part motor;
		}
		metadata after : Audit about pump, pump::motor;
		part plain;
	`)

	pump := sym(t, user, "pump")
	got := annotatingNames(m, pump)
	if want := []string{"@", "@", "named", "before", "after"}; !slices.Equal(got, want) {
		t.Errorf("annotating metadata of pump = %v, want %v", got, want)
	}
	for _, usage := range m.AnnotatingMetadataOf(pump) {
		if usage.Kind != symbols.SymbolMetadataUsage {
			t.Errorf("%q is a %v, want a metadata usage", usage.Name, usage.Kind)
		}
	}
	if got := annotatingNames(m, sym(t, pump.Scope, "motor")); !slices.Equal(got, []string{"after"}) {
		t.Errorf("annotating metadata of pump::motor = %v, want [after]", got)
	}
	if got := annotatingNames(m, sym(t, user, "plain")); len(got) != 0 {
		t.Errorf("annotating metadata of plain = %v, want none", got)
	}
	// An `about` usage annotates what it names, not itself nor its namespace.
	if got := annotatingNames(m, sym(t, user, "after")); len(got) != 0 {
		t.Errorf("annotating metadata of the usage `after` = %v, want none", got)
	}
}

// An `about` usage in a frozen library document and one in the workspace both
// name themselves on the element they annotate, the library's first.
func TestAnnotatingMetadataAcrossLibraryAndWorkspace(t *testing.T) {
	m, lib, user := buildOverlayModel(t, `
		metadata def Safety;
		part def Belt { @Safety; }
		metadata fromLib : Safety about Belt;
	`, "metadata fromUser : Safety about Belt;")

	belt := sym(t, lib, "Belt")
	if got, want := annotatingNames(m, belt), []string{"@", "fromLib", "fromUser"}; !slices.Equal(got, want) {
		t.Errorf("annotating metadata of Belt = %v, want %v", got, want)
	}
	usages := m.AnnotatingMetadataOf(belt)
	if len(usages) == 3 && usages[2] != sym(t, user, "fromUser") {
		t.Errorf("third usage is %v, want the workspace's fromUser", usages[2])
	}
}

// A usage naming the same element twice in its `about` list is listed once, and
// one annotating an element it owns is still found on that element.
func TestAnnotatingMetadataIsDeduplicatedAndFollowsOwnedTargets(t *testing.T) {
	m, _, user := buildOverlayModel(t, "metadata def Safety;", `
		part pump;
		metadata twice : Safety about pump, pump;
		metadata holder : Safety about holder::inner { part inner; }
	`)
	if got := annotatingNames(m, sym(t, user, "pump")); !slices.Equal(got, []string{"twice"}) {
		t.Errorf("annotating metadata of pump = %v, want [twice]", got)
	}
	holder := sym(t, user, "holder")
	if got := annotatingNames(m, sym(t, holder.Scope, "inner")); !slices.Equal(got, []string{"holder"}) {
		t.Errorf("annotating metadata of holder::inner = %v, want [holder]", got)
	}
}

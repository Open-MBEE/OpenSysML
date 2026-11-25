package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A type's members carry its anonymous documentation, and a specialization's
// same-named documentation masks the general's as any inherited name is masked.
func TestMembersIncludeDocumentationUnderNameMasking(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def Base { doc Note /* base */ doc /* anonymous */ }
		part def Child :> Base { doc Note /* child */ }
	}`)
	pkg, _ := root.LookupLocal("P")
	child := member(t, m, pkg, "Child")
	members, ok := m.ReflectiveElements(child, "member")
	if !ok {
		t.Fatal("member of Child is unsupported")
	}
	var notes, anonymous int
	for _, s := range members {
		if s.Kind != symbols.SymbolDocumentation {
			continue
		}
		switch {
		case s.Name == "Note":
			notes++
			if s.Owner() != child {
				t.Errorf("Note comes from %s, want Child's own", s.Owner().Name)
			}
		case s.Name == "":
			anonymous++
		}
	}
	if notes != 1 || anonymous != 1 {
		t.Fatalf("Child has %d Note and %d anonymous documentation members, want 1 and 1", notes, anonymous)
	}
}

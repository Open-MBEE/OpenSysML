package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

func TestRedefinedUsageInheritsTypingConstraints(t *testing.T) {
	src := `
		package P {
			part def B;
			part def FA {
				port p1 : B;
			}
			part f : FA {
				port pp redefines p1;
			}
		}
	`
	var got []diag.Diagnostic
	for _, d := range typeDiags(t, src) {
		if d.Message == "A port must be typed by port definitions." {
			got = append(got, d)
		}
	}
	if len(got) != 2 {
		t.Fatalf("expected diagnostics for the declared and inherited port typings, got %v", got)
	}
	if got[0].Span.Offset == got[1].Span.Offset {
		t.Fatalf("expected inherited typing diagnostic at a distinct declaration, got %v", got)
	}
}

func TestReferenceRedefinitionsDoNotInheritUsageKindTyping(t *testing.T) {
	src := `
		package P {
			part def B;
			part def FA {
				port p1 : B;
			}
			part f : FA {
				ref :>> p1;
				ref q redefines p1;
			}
		}
	`
	var got []diag.Diagnostic
	for _, d := range typeDiags(t, src) {
		if d.Message == "A port must be typed by port definitions." {
			got = append(got, d)
		}
	}
	if len(got) != 1 || got[0].Span.Offset != 51 {
		t.Fatalf("expected only the directly declared invalid port typing, got %v", got)
	}
}

func TestRedefinitionTypingStopsAtNearestDeclaredType(t *testing.T) {
	src := `
		package P {
			rendering def Rendering :> Part;
			rendering def Table :> Rendering;
			rendering base : Table;
			rendering derived :> base;
		}
	`
	for _, d := range w9cLibraryDiags(t, src, false) {
		if d.Message == "A rendering must be typed by one rendering definition." {
			t.Fatalf("unexpected transitive-supertype typing diagnostic at %v", d.Span)
		}
	}
}

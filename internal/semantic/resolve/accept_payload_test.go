package resolve

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// An accept node's payload is a feature of the action body it is declared in, so
// a sibling node reads it by simple name.
func TestAcceptPayloadVisibleToSiblingNode(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		package P {
			item def Warning { attribute level; }
			action A {
				attribute lvl;
				action wait accept msg : Warning;
				action handle { assign lvl := msg.level; }
			}
		}
	`)
	assertNoUnresolved(t, r)
}

// Two accepts in one body bind different payloads, each visible under its own
// name.
func TestAcceptPayloadTwoAcceptsInOneBody(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		package P {
			item def Warning { attribute level; }
			item def Reset { attribute code; }
			action A {
				attribute lvl;
				action wait accept msg : Warning;
				action rearm accept ack : Reset;
				action handle { assign lvl := msg.level; }
				action clear { assign lvl := ack.code; }
			}
		}
	`)
	assertNoUnresolved(t, r)
}

// A nested if or loop body resolves through the action body, so the payload is
// visible there too.
func TestAcceptPayloadVisibleInNestedBody(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		package P {
			item def Warning { attribute level; }
			action A {
				attribute lvl;
				action wait accept msg : Warning;
				action handle {
					if msg.level > 1 { assign lvl := msg.level; }
					while msg.level > 0 { assign lvl := msg.level; }
				}
			}
		}
	`)
	assertNoUnresolved(t, r)
}

// A payload read textually before its accept resolves: scoping is not ordered by
// declaration position (KerML 8.2.3.5).
func TestAcceptPayloadVisibleBeforeDeclaration(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		package P {
			item def Warning { attribute level; }
			action A {
				attribute lvl;
				action handle { assign lvl := msg.level; }
				action wait accept msg : Warning;
			}
		}
	`)
	assertNoUnresolved(t, r)
}

// The nearer declaration wins (KerML 8.2.3.5.3): the payload shadows an outer
// feature of the same name.
func TestAcceptPayloadShadowsOuterFeature(t *testing.T) {
	const src = `
		package P {
			item def Warning { attribute level; }
			attribute msg : Warning;
			action A {
				attribute lvl;
				action wait accept msg : Warning;
				action handle { assign lvl := msg.level; }
			}
		}
	`
	r := resolveDoc(t, "d.sysml", src)
	assertNoUnresolved(t, r)

	body := scopeNamed(t, r, "d.sysml", "A")
	wait := scopeNamed(t, r, "d.sysml", "wait")
	payload, ok := wait.LookupLocal("msg")
	if !ok {
		t.Fatal("the accept node declares no payload named msg")
	}
	sym, ok := r.LookupName(body, "msg")
	if !ok {
		t.Fatal("msg does not resolve in the action body")
	}
	if sym.Decl != payload.Decl {
		t.Errorf("msg resolved to %v, want the accept payload declaration", sym.Decl)
	}
}

// A name nothing declares is still reported, payloads or not.
func TestAcceptPayloadUnresolvedStillReported(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		package P {
			item def Warning { attribute level; }
			action A {
				attribute lvl;
				action wait accept msg : Warning;
				action handle { assign lvl := nope.level; }
			}
		}
	`)
	var found bool
	for _, d := range r.Diagnostics {
		if strings.Contains(d.Message, "nope") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an unresolved diagnostic for nope, got %v", r.Diagnostics)
	}
}

// A payload does not escape the body that accepts it: an outer scope does not
// see it.
func TestAcceptPayloadDoesNotEscapeBody(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		package P {
			item def Warning { attribute level; }
			action A {
				action wait accept msg : Warning;
			}
			action B { attribute lvl = msg.level; }
		}
	`)
	if len(r.Diagnostics) == 0 {
		t.Fatal("expected msg to be unresolved outside the accepting body")
	}
}

// Only a behavior body shares a feature space: a part declaring an action node
// does not read that node's payload.
func TestAcceptPayloadNotSharedByAPartBody(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		package P {
			item def Warning;
			part p {
				action wait accept msg : Warning;
				attribute seen = msg;
			}
		}
	`)
	if len(r.Diagnostics) == 0 {
		t.Fatal("expected msg to be unresolved in the part body")
	}
}

// A payload with no declared name of its own binds nothing at execution, so it
// must not mask the feature whose name it borrows (KerML 7.3.4.5).
func TestAcceptPayloadWithoutADeclaredNameDoesNotMaskIt(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		package P {
			item def Shutdown;
			attribute shutDown : Shutdown;
			action A {
				attribute taken : Shutdown;
				action interrupt accept ::> shutDown;
				action handle { assign taken := shutDown; }
			}
		}
	`)
	assertNoUnresolved(t, r)

	pkg := scopeNamed(t, r, "d.sysml", "P")
	body := scopeNamed(t, r, "d.sysml", "A")
	outer, ok := pkg.LookupLocal("shutDown")
	if !ok {
		t.Fatal("the package declares no attribute named shutDown")
	}
	sym, ok := r.LookupName(body, "shutDown")
	if !ok {
		t.Fatal("shutDown does not resolve in the action body")
	}
	if sym.Decl != outer.Decl {
		t.Errorf("shutDown resolved to %v, want the body's own attribute", sym.Decl)
	}
}

// scopeNamed returns the scope owned by the declaration called name in doc.
func scopeNamed(t *testing.T, r *Resolver, doc, name string) *symbols.Scope {
	t.Helper()
	var find func(*symbols.Scope) *symbols.Scope
	find = func(s *symbols.Scope) *symbols.Scope {
		if s == nil {
			return nil
		}
		if owner := s.Owner(); owner != nil && owner.Name == name {
			return s
		}
		for _, child := range s.Children() {
			if found := find(child); found != nil {
				return found
			}
		}
		return nil
	}
	scope := find(r.Index().DocumentRoot(doc))
	if scope == nil {
		t.Fatalf("no scope owned by %s", name)
	}
	return scope
}

func assertNoUnresolved(t *testing.T, r *Resolver) {
	t.Helper()
	for _, d := range r.Diagnostics {
		t.Errorf("diagnostic: %s", d.Message)
	}
}

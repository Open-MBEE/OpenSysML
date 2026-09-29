package resolve

import (
	"strings"
	"testing"
)

func TestAcceptTriggerReferences(t *testing.T) {
	tests := []struct {
		name           string
		decls          string
		transition     string
		action         string
		wantTransition string
		wantAction     string
	}{
		{
			name:       "bare type",
			decls:      "item def Start;",
			transition: "accept Start",
			action:     "accept Start",
		},
		{
			name:           "bare unresolved type suggests declaration",
			decls:          "item def Start;",
			transition:     "accept Strat",
			action:         "accept Strat",
			wantTransition: "unresolved reference: Strat — did you mean Start?",
			wantAction:     "unresolved reference: Strat — did you mean Start?",
		},
		{
			name:       "qualified type",
			decls:      "package Outer { item def Start; }",
			transition: "accept Outer::Start",
			action:     "accept Outer::Start",
		},
		{
			name:           "qualified unresolved type",
			decls:          "package Outer { item def Start; }",
			transition:     "accept Outer::Strat",
			action:         "accept Outer::Strat",
			wantTransition: "unresolved reference: Outer::Strat",
			wantAction:     "unresolved reference: Outer::Strat",
		},
		{
			name:           "resolved type and unresolved target",
			decls:          "item def Go;",
			transition:     "accept Go then missing_state",
			action:         "accept Go",
			wantTransition: "missing_state",
		},
		{
			name:       "named payload type",
			decls:      "item def Start;",
			transition: "accept s : Start",
			action:     "accept s : Start",
		},
		{
			name:           "named payload unresolved type",
			decls:          "item def Start;",
			transition:     "accept s : Strat",
			action:         "accept s : Strat",
			wantTransition: "did you mean Start?",
			wantAction:     "did you mean Start?",
		},
		{
			name:       "subsetting payload",
			decls:      "item def Start; item sig : Start;",
			transition: "accept s :> sig",
			action:     "accept s :> sig",
		},
		{
			name:           "subsetting payload unresolved target",
			decls:          "item def Start; item sig : Start;",
			transition:     "accept s :> sgi",
			action:         "accept s :> sgi",
			wantTransition: "did you mean sig?",
			wantAction:     "did you mean sig?",
		},
		{
			name:       "via port",
			decls:      "item def Start; port def Wire; port p : Wire;",
			transition: "accept Start via p",
		},
		{
			name:           "via unresolved port",
			decls:          "item def Start; port def Wire; port p : Wire;",
			transition:     "accept Start via q",
			wantTransition: "unresolved reference: q",
		},
		{
			name:       "after declared attribute",
			decls:      "attribute d;",
			transition: "accept after d",
			action:     "accept after d",
		},
		{
			name:           "after unresolved attribute",
			transition:     "accept after d",
			action:         "accept after d",
			wantTransition: "unresolved reference: d",
			wantAction:     "unresolved reference: d",
		},
		{
			name:       "at declared attribute",
			decls:      "attribute t;",
			transition: "accept at t",
			action:     "accept at t",
		},
		{
			name:           "at unresolved attribute",
			transition:     "accept at t",
			action:         "accept at t",
			wantTransition: "unresolved reference: t",
			wantAction:     "unresolved reference: t",
		},
		{
			name:       "change with declared attribute",
			decls:      "attribute cond;",
			transition: "accept when cond",
			action:     "accept when cond",
		},
		{
			name:           "change with unresolved attribute",
			transition:     "accept when cond",
			action:         "accept when cond",
			wantTransition: "unresolved reference: cond",
			wantAction:     "unresolved reference: cond",
		},
		{
			name:       "when name remains injected signal",
			transition: "when Name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.transition != "" {
				src := "package P { " + tc.decls + `
					state def S {
						state a;
						state b;
						transition first a ` + tc.transition + func() string {
					if strings.Contains(tc.transition, " then ") {
						return ";\n"
					}
					return " then b;\n"
				}() + `
					}
				}`
				assertAcceptReferenceResult(t, src, tc.wantTransition)
			}
			if tc.action != "" {
				src := "package P { " + tc.decls + `
					action a { ` + tc.action + `; }
				}`
				assertAcceptReferenceResult(t, src, tc.wantAction)
			}
		})
	}
}

func assertAcceptReferenceResult(t *testing.T, src, want string) {
	t.Helper()
	r := resolveDoc(t, "accept-trigger.sysml", src)
	if want == "" {
		if len(r.Diagnostics) != 0 {
			t.Fatalf("got %d diagnostics, want none: %v", len(r.Diagnostics), r.Diagnostics)
		}
		return
	}
	if len(r.Diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want exactly one containing %q: %v", len(r.Diagnostics), want, r.Diagnostics)
	}
	if !strings.Contains(r.Diagnostics[0].Message, want) {
		t.Errorf("diagnostic = %q, want it to contain %q", r.Diagnostics[0].Message, want)
	}
}

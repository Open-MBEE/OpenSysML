package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes/behavior"
)

// successionBodyForms are the two spellings of an action target succession
// that end in a body of their own rather than a ';'.
var successionBodyForms = map[string]string{
	"then target":              "action def A { action prep; action starting; first prep; then starting { %s } }",
	"first source then target": "action def A { action prep; action starting; first prep then starting { %s } }",
}

// A transition guard nested in a succession body is type-checked like one
// written anywhere else.
func TestTransitionGuardInSuccessionBody(t *testing.T) {
	for name, form := range successionBodyForms {
		t.Run(name, func(t *testing.T) {
			src := "package P { " + strings.Replace(form, "%s",
				`attribute n : ScalarValues::Integer = 1; state def SM { state a; state b; transition first a if n then b; }`, 1) + " }"
			diags := diagsIn(t, "a.sysml", src, "type")
			if len(diags) != 1 || diags[0].Message != "transition guard must be Boolean, found Integer" {
				t.Fatalf("got %v, want the nested transition guard diagnostic", diags)
			}
			silent := "package P { " + strings.Replace(form, "%s",
				`attribute ok : ScalarValues::Boolean = true; state def SM { state a; state b; transition first a if ok then b; }`, 1) + " }"
			if diags := diagsIn(t, "a.sysml", silent, "type"); len(diags) != 0 {
				t.Fatalf("got %v, want no diagnostics for a Boolean guard", diags)
			}
		})
	}
}

// A succession endpoint nested in a succession body is checked like one
// written anywhere else.
func TestActionEndpointInSuccessionBody(t *testing.T) {
	for name, form := range successionBodyForms {
		t.Run(name, func(t *testing.T) {
			src := "package P { " + strings.Replace(form, "%s",
				`action def Inner { attribute flag = 0; action leaf; succession first leaf then flag; }`, 1) + " }"
			ctx, root := nameresCtx(t, "a.sysml", src)
			got := behavior.ActionEndpointPass{}.Run(ctx, "a.sysml", root)
			if len(got) != 1 || got[0].Code != behavior.CodeEndpointNotANode {
				t.Fatalf("expected one nested action endpoint diagnostic, got %+v", got)
			}
			silent := "package P { " + strings.Replace(form, "%s",
				`action def Inner { action leaf; action next; succession first leaf then next; }`, 1) + " }"
			ctx, root = nameresCtx(t, "a.sysml", silent)
			if got := (behavior.ActionEndpointPass{}).Run(ctx, "a.sysml", root); len(got) != 0 {
				t.Fatalf("got %+v, want no diagnostics for node endpoints", got)
			}
		})
	}
}

// A succession body is a UsageBody, so a one-ended `first` written directly in
// it is an extension, while one in an action definition nested there is not.
func TestOneEndedFirstInSuccessionBodyIsAnExtension(t *testing.T) {
	for name, form := range successionBodyForms {
		t.Run(name, func(t *testing.T) {
			wantNotation(t, "a.sysml", strings.Replace(form, "%s", "action orphan; first orphan;", 1),
				CodeNonstandardNotation, "one-ended `first <node>;` outside an action body")
			wantSilent(t, "a.sysml", strings.Replace(form, "%s", "action def Inner { action leaf; first leaf; }", 1))
		})
	}
}

// A one-ended `first` ends in a RelationshipBody, which admits annotations
// only, so a one-ended `first` nested in it is an extension as well.
func TestOneEndedFirstInAnInitialNodeBodyIsAnExtension(t *testing.T) {
	wantNotation(t, "a.sysml", "action def A { action a; action b; first a { first b; } }",
		CodeNonstandardNotation, "one-ended `first <node>;` outside an action body")
	wantSilent(t, "a.sysml", "action def A { action a; first a { doc /* starts here */ } }")
}

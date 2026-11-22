package passes

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes/behavior"
	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
)

// chainEndSpellings are the notations that state a succession end in an action
// body, each with %s where the end under test is written.
var chainEndSpellings = map[string]string{
	"first then target":  `first a then %s;`,
	"first then source":  `first %s then a;`,
	"succession target":  `succession s first a then %s;`,
	"succession source":  `succession s first %s then a;`,
	"positional then":    `first a; then %s;`,
	"guarded then":       `first a; then decide; if true then %s; else a;`,
	"else":               `first a; then decide; if false then a; else %s;`,
	"transition target":  `transition first a then %s;`,
	"guarded succession": `first a if true then %s;`,
}

// A succession end chaining through an object (SysML v2 §7.17.4) is judged the
// same in every spelling, and the tier reports exactly what is no occurrence.
func TestActionEndpointPassChainEndsAgreeWithLowering(t *testing.T) {
	ends := map[string]struct {
		end      string
		reported bool
		lowerErr error
		lowers   bool
	}{
		"action through a part":           {end: "v.close", lowers: true},
		"action through two parts":        {end: "fleet.lead.close", lowers: true},
		"event occurrence through a part": {end: "v.opened", lowerErr: lower.ErrSuccessionEndNotPerformed},
		"part through a part":             {end: "fleet.lead", lowerErr: lower.ErrSuccessionEndNotPerformed},
		"state through a part":            {end: "v.locked", lowerErr: lower.ErrSuccessionEndNotPerformed},
		"attribute through a part":        {end: "v.n", reported: true},
		"enumeration through a part":      {end: "v.gear", reported: true},
	}
	for spelling, form := range chainEndSpellings {
		for name, c := range ends {
			t.Run(spelling+"/"+name, func(t *testing.T) {
				src := `package P {
					enum def Gear { park; drive; }
					part def Vehicle {
						action close;
						event occurrence opened;
						state locked;
						attribute n = 0;
						enum gear : Gear;
					}
					part def Fleet { part lead : Vehicle; }
					action def A {
						part v : Vehicle;
						part fleet : Fleet;
						action a;
						` + strings.ReplaceAll(form, "%s", c.end) + `
					}
				}`
				ctx, root := nameresCtx(t, "a.sysml", src)
				diags := behavior.ActionEndpointPass{}.Run(ctx, "a.sysml", root)
				pkg, _ := ctx.Index.DocumentRoot("a.sysml").LookupLocal("P")
				def, ok := pkg.Scope.LookupLocal("A")
				if !ok {
					t.Fatal("action def A not found")
				}
				_, err := lower.ToActionGraph(def.Decl, def.Scope)
				switch {
				case c.lowers:
					if err != nil || len(diags) != 0 {
						t.Fatalf("lowering error %v, diagnostics %+v; want the end performed", err, diags)
					}
				case c.lowerErr != nil:
					if !errors.Is(err, c.lowerErr) || len(diags) != 0 {
						t.Fatalf("lowering error %v, diagnostics %+v; want %v and silence", err, diags, c.lowerErr)
					}
				default:
					if err == nil || len(diags) != 1 || diags[0].Code != behavior.CodeEndpointNotANode {
						t.Fatalf("lowering error %v, diagnostics %+v; want one endpoint diagnostic", err, diags)
					}
					if covered := src[diags[0].Span.Offset : diags[0].Span.Offset+diags[0].Span.Len]; strings.TrimSpace(covered) != c.end {
						t.Fatalf("the diagnostic covers %q, want %q", covered, c.end)
					}
				}
			})
		}
	}
}

// A chained end ending in `done` or `start` names a feature of the object it
// chains through, never the flow's own markers.
func TestActionEndpointPassChainedMarkerNameIsNoMarker(t *testing.T) {
	src := `package P {
		part def Vehicle { attribute done = 0; }
		action def A {
			part v : Vehicle;
			action a;
			first a then v.done;
		}
	}`
	ctx, root := nameresCtx(t, "a.sysml", src)
	diags := behavior.ActionEndpointPass{}.Run(ctx, "a.sysml", root)
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "v.done") {
		t.Fatalf("got %+v, want the chained end reported", diags)
	}
}

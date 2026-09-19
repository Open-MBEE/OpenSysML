package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A condition reading a subject by its short name sees the object the subject
// is bound to, whether the subject also declares a name or only the short one.
func TestSubjectBindingIsReadByShortName(t *testing.T) {
	for _, tc := range []struct{ label, decl, holds, broken string }{
		{"both names", "subject <t> truck : Truck;", "subject <t> truck = loadedTruck;", "subject <t> truck = heavyTruck;"},
		{"short name only", "subject <t> : Truck;", "subject <t> = loadedTruck;", "subject <t> = heavyTruck;"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			file := parseAndBuild(t, `
				package test {
					part def Truck { attribute payload : Real; }
					part loadedTruck : Truck { attribute redefines payload = 5.0; }
					part heavyTruck : Truck { attribute redefines payload = 15.0; }
					requirement def PayloadReq {
						`+tc.decl+`
						require constraint { t.payload <= 10.0 }
					}
					requirement payloadHolds : PayloadReq { `+tc.holds+` }
					requirement payloadBroken : PayloadReq { `+tc.broken+` }
				}
			`)
			if file == nil {
				t.Fatal("parse failed")
			}
			idx, _, ctx := buildRuntime(t, "<test>", file)
			rootScope := idx.DocumentRoot("<test>")

			holds, err := ctx.EvaluateRequirement(findSymbolByName(rootScope, "payloadHolds", ast.DefRequirement), rootScope)
			if err != nil || !holds {
				t.Fatalf("payloadHolds = %v, %v; want true", holds, err)
			}
			holds, err = ctx.EvaluateRequirement(findSymbolByName(rootScope, "payloadBroken", ast.DefRequirement), rootScope)
			if holds || !errors.Is(err, ErrViolated) {
				t.Fatalf("payloadBroken = %v, %v; want ErrViolated", holds, err)
			}

			def := findSymbolByName(rootScope, "PayloadReq", ast.DefRequirement)
			holds, err = ctx.EvaluateRequirement(def, rootScope)
			if !errors.Is(err, ErrUnboundSubject) {
				t.Fatalf("PayloadReq = %v, %v; want ErrUnboundSubject", holds, err)
			}
			if !strings.Contains(err.Error(), "t") {
				t.Errorf("error %q does not name the subject", err)
			}
		})
	}
}

// The object a satisfaction assertion supplies binds a subject read by its
// short name, over the value the requirement binds itself.
func TestSatisfactionSubjectIsReadByShortName(t *testing.T) {
	for _, tc := range []struct{ label, decl, bound string }{
		{"both names", "subject <l> lander : Lander;", "subject <l> lander = slowLander;"},
		{"short name only", "subject <l> : Lander;", "subject <l> = slowLander;"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			src := `
				package Landing {
					part def Lander { attribute verticalSpeed; }
					part slowLander : Lander { attribute :>> verticalSpeed = 1.2; }
					part fastLander : Lander { attribute :>> verticalSpeed = 2.4; }
					requirement def TouchdownRequirement {
						` + tc.decl + `
						attribute maxVerticalSpeed = 1.5;
						require constraint { l.verticalSpeed <= maxVerticalSpeed }
					}
					requirement touchdown : TouchdownRequirement { ` + tc.bound + ` }
					part analysisContext {
						assert satisfy touchdown by fastLander;
						assert satisfy touchdown by slowLander;
					}
				}
			`
			ctx, a := satisfactionOf(t, src, "satisfy touchdown by fastLander")
			if satisfied, err := ctx.EvaluateSatisfaction(a); !errors.Is(err, ErrViolated) {
				t.Fatalf("by fastLander = %v, %v; want ErrViolated: `by` supplies the subject", satisfied, err)
			}
			ctx, a = satisfactionOf(t, src, "satisfy touchdown by slowLander")
			if satisfied, err := ctx.EvaluateSatisfaction(a); err != nil || !satisfied {
				t.Fatalf("by slowLander = %v, %v; want true", satisfied, err)
			}
		})
	}
}

package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A subject or actor a derived requirement redeclares is one feature with the
// one it redefines, so its value is read under every name of that feature: the
// inherited name and short name as well as the redefining declaration's own.
func TestRedefiningSubjectBindsTheRedefinedNames(t *testing.T) {
	for _, tc := range []struct{ label, base, condition, typ, holds, broken string }{
		{"explicit redefinition, inherited name",
			"subject <t> truck : Truck;", "truck.payload <= 10.0", "PayloadReq",
			"subject renamed :>> truck = loadedTruck;", "subject renamed :>> truck = heavyTruck;"},
		{"explicit redefinition, inherited short name",
			"subject <t> truck : Truck;", "t.payload <= 10.0", "PayloadReq",
			"subject renamed :>> truck = loadedTruck;", "subject renamed :>> truck = heavyTruck;"},
		{"explicit redefinition by short name, new short name",
			"subject <t> truck : Truck;", "t.payload <= 10.0 and truck.payload <= 10.0 and r.payload <= 10.0", "PayloadReq",
			"subject <r> renamed :>> t = loadedTruck;", "subject <r> renamed :>> t = heavyTruck;"},
		{"short-name-only redefinition",
			"subject truck : Truck;", "truck.payload <= 10.0 and r.payload <= 10.0", "PayloadReq",
			"subject <r> :>> truck = loadedTruck;", "subject <r> :>> truck = heavyTruck;"},
		{"anonymous redefinition",
			"subject truck : Truck;", "truck.payload <= 10.0", "PayloadReq",
			"subject :>> truck = loadedTruck;", "subject :>> truck = heavyTruck;"},
		{"implicit role redefinition",
			"subject <t> truck : Truck;", "truck.payload <= 10.0 and t.payload <= 10.0", "PayloadReq",
			"subject renamed = loadedTruck;", "subject renamed = heavyTruck;"},
		{"actor redefinition",
			"subject truck : Truck; actor <d> driver : Truck;", "d.payload <= 10.0 and driver.payload <= 10.0", "PayloadReq",
			"subject truck = loadedTruck; actor op :>> driver = loadedTruck;",
			"subject truck = loadedTruck; actor op :>> driver = heavyTruck;"},
		{"redefinition of a redefinition",
			"subject <t> truck : Truck;", "t.payload <= 10.0 and truck.payload <= 10.0 and mid.payload <= 10.0", "MidReq",
			"subject last :>> mid = loadedTruck;", "subject last :>> mid = heavyTruck;"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			file := parseAndBuild(t, `
				package test {
					part def Truck { attribute payload : Real; }
					part loadedTruck : Truck { attribute redefines payload = 5.0; }
					part heavyTruck : Truck { attribute redefines payload = 15.0; }
					requirement def PayloadReq {
						`+tc.base+`
						require constraint { `+tc.condition+` }
					}
					requirement def MidReq :> PayloadReq { subject mid :>> truck; }
					requirement payloadHolds : `+tc.typ+` { `+tc.holds+` }
					requirement payloadBroken : `+tc.typ+` { `+tc.broken+` }
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
		})
	}
}

// A redefining subject left unbound is reported as the unbound subject under
// whichever of its names the condition read, not as a feature without a value.
func TestUnboundRedefiningSubjectIsReportedAsUnbound(t *testing.T) {
	file := parseAndBuild(t, `
		package test {
			part def Truck { attribute payload : Real; }
			requirement def PayloadReq {
				subject <t> truck : Truck;
				require constraint { t.payload <= 10.0 }
			}
			requirement payloadOpen : PayloadReq { subject renamed :>> truck; }
		}
	`)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	holds, err := ctx.EvaluateRequirement(findSymbolByName(rootScope, "payloadOpen", ast.DefRequirement), rootScope)
	if !errors.Is(err, ErrUnboundSubject) {
		t.Fatalf("payloadOpen = %v, %v; want ErrUnboundSubject", holds, err)
	}
	if !strings.Contains(err.Error(), "t subject is unbound") {
		t.Errorf("error %q does not name the subject as read", err)
	}
}

// A redefining subject without a value of its own reads the value the
// redefined subject binds, under its own name too.
func TestRedefiningSubjectInheritsTheRedefinedBinding(t *testing.T) {
	file := parseAndBuild(t, `
		package test {
			part def Truck { attribute payload : Real; }
			part loadedTruck : Truck { attribute redefines payload = 5.0; }
			requirement def PayloadReq {
				subject truck : Truck = loadedTruck;
			}
			requirement def RenamedReq :> PayloadReq {
				subject <r> renamed :>> truck;
				require constraint { renamed.payload <= 10.0 and r.payload <= 10.0 }
			}
			requirement payloadHolds : RenamedReq;
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
}

// The object a satisfaction assertion supplies binds a redefining subject under
// every name of the feature it redefines, over the value the requirement binds.
func TestSatisfactionBindsRedefiningSubjectUnderRedefinedNames(t *testing.T) {
	src := `
		package Landing {
			part def Lander { attribute verticalSpeed; }
			part slowLander : Lander { attribute :>> verticalSpeed = 1.2; }
			part fastLander : Lander { attribute :>> verticalSpeed = 2.4; }
			requirement def TouchdownRequirement {
				subject <l> lander : Lander;
				attribute maxVerticalSpeed = 1.5;
				require constraint { l.verticalSpeed <= maxVerticalSpeed and lander.verticalSpeed <= maxVerticalSpeed }
			}
			requirement touchdown : TouchdownRequirement {
				subject <v> vehicle :>> lander = slowLander;
				require constraint { v.verticalSpeed <= maxVerticalSpeed }
			}
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
}

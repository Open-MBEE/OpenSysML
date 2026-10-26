package main

import "testing"

// subrequirementModel nests requirement usages in a requirement definition: light
// and the typed engine check take the container's subject, pow declares its own.
const subrequirementModel = `
package Fleet {
	private import ScalarValues::*;
	part def Eng { attribute power : Real; }
	part def Veh { attribute mass : Real; part eng : Eng; }
	requirement def PowerReq { subject e : Eng; require constraint { e.power > 100.0 } }
	requirement def R {
		subject v : Veh;
		requirement light { require constraint { v.mass < 10.0 } }
		requirement pow : PowerReq { subject e = v.eng; }
	}
	part heavy : Veh { attribute :>> mass = 20.0; part :>> eng { attribute :>> power = 200.0; } }
	part feather : Veh { attribute :>> mass = 5.0; part :>> eng { attribute :>> power = 200.0; } }
	requirement rHeavy : R { subject v = heavy; }
	requirement rFeather : R { subject v = feather; }
}
`

// TestRequirementCountsSubrequirements: -requirement reports a requirement whose subrequirement fails as failed, naming the
// failing condition, rather than as a vacuous pass (SysML v2 §7.21.2: subrequirements are required constraints).
func TestRequirementCountsSubrequirements(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, subrequirementModel, "-requirement=Fleet::rHeavy"), 1,
		"✗ Requirement Fleet::rHeavy failed",
		"v.mass < 10.0")

	wantReport(t, check(t, binary, subrequirementModel, "-requirement=Fleet::rFeather"), 0,
		"✓ Requirement Fleet::rFeather satisfied")
}

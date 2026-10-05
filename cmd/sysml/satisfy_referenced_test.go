package main

import "testing"

// freightModel asserts a requirement whose `require r { in w = unit; }` members
// bind the subject of each referenced requirement, which computes the attribute
// its condition reads from that subject.
const freightModel = `package Freight {
    private import ISQ::*;
    private import SI::*;
    part def Wagon {
        attribute tare : MassValue;
        attribute cargo : MassValue;
    }
    part wagon1 : Wagon {
        attribute :>> tare = 800 [kg];
        attribute :>> cargo = 150 [kg];
        satisfy wagonSpec by wagon1;
    }
    requirement wagonSpec {
        subject unit : Wagon;
        require ladenLimit { in w = unit; }
        require emptyLimit { in w = unit; }
    }
    requirement def MassCap {
        attribute actual : MassValue;
        attribute cap : MassValue;
        require constraint { actual <= cap }
    }
    requirement def WagonMassCap :> MassCap {
        subject w : Wagon;
        attribute :>> actual = w.tare + w.cargo;
        assume constraint { w.cargo > 0 [kg] }
    }
    requirement ladenLimit : WagonMassCap {
        attribute :>> cap = 1000 [kg];
    }
    requirement emptyLimit : WagonMassCap {
        attribute :>> cap = 900 [kg];
    }
}`

// TestSatisfyDecidesReferencedRequirementsFromTheSubject checks that -satisfy
// reaches a verdict through the subject bindings the required references state,
// while -requirement on a referenced requirement alone is undecided for want of a subject.
func TestSatisfyDecidesReferencedRequirementsFromTheSubject(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, freightModel, "-satisfy"), 1,
		"✗ satisfy wagonSpec by wagon1 fails",
		"Required condition evaluated to false: actual <= cap")

	wantReport(t, check(t, binary, freightModel, "-requirement", "Freight::ladenLimit"), 2,
		"? Requirement Freight::ladenLimit could not be evaluated", "w subject is unbound")
}

package main

import (
	"strings"
	"testing"
)

// vacuousSatisfyFiles satisfy a viewpoint whose definition frames a concern and
// requires no constraint.
var vacuousSatisfyFiles = map[string]string{
	"reading.sysml": `package Reading {
    part def Mission;
    part mission : Mission;
    concern def Traceability {
        subject mission : Mission;
    }
    concern traceability : Traceability;
    viewpoint def Overview {
        frame traceability;
    }
    viewpoint reading : Overview;
    view tree {
        expose mission;
        satisfy reading;
    }
}
`,
}

// TestSatisfyOfAViewpointWithoutConstraintHolds checks that -satisfy reports a
// viewpoint requiring nothing as held, says why, and exits 0.
func TestSatisfyOfAViewpointWithoutConstraintHolds(t *testing.T) {
	binary := buildCLI(t)
	out := checkFiles(t, binary, vacuousSatisfyFiles, "-satisfy")
	wantReport(t, out, 0,
		"✓ satisfy reading holds (on Reading::tree ID: ",
		"standing: holds (observed: 1 run under reverse; vacuous: viewpoint reading states no required constraint, so it holds vacuously)")
	if strings.Contains(out.output(), "could not be evaluated") {
		t.Errorf("a vacuous viewpoint was reported unevaluable:\n%s", out.output())
	}
}

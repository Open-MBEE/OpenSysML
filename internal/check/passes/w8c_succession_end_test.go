package passes

import (
	"strings"
	"testing"
)

// A succession's ends are connector ends: each must name a feature featured
// within the action the succession belongs to (KerML 1.1 8.3.4.5.3
// checkConnectorTypeFeaturing). A nested action, named or the unnamed body of
// a loop or branch, does not feature its enclosing definition's features or
// the enclosing action's nodes, so an end naming one is reported at the first
// feature of the path — both ends, as the pinned pilot reports them.
func TestW8CSuccessionEndsReachOnlyTheirActionsFeatures(t *testing.T) {
	cases := map[string]struct {
		src  string
		want []string
	}{
		"loop action relates the use case subject": {`package P {
	part def DroneSystem { action reportStatus; }
	use case def Monitor {
		subject droneSystem : DroneSystem { perform action ucReport ::> reportStatus; }
		action cycle loop {
			first start then droneSystem.ucReport;
			first droneSystem.ucReport then done;
		}
	}
}`, []string{"6:21", "7:10"}},
		"loop action relates the part of its part def": {`package P {
	part def DroneSystem { action reportStatus; }
	part def Holder {
		part s : DroneSystem;
		action inner loop { first start then s.reportStatus; }
	}
}`, []string{"5:40"}},
		"while body relates the enclosing action's nodes": {`package P {
	action def A {
		action a;
		action b;
		while true {
			first start then a;
			first a then b;
		}
	}
}`, []string{"6:21", "7:10", "7:17"}},
		"if branch relates the enclosing action's nodes": {`package P {
	action def A {
		action a;
		action b;
		if true { first a then b; }
	}
}`, []string{"5:19", "5:26"}},
		"succession usage in a loop body": {`package P {
	action def A {
		action a;
		action b;
		while true { succession first a then b; }
	}
}`, []string{"5:33", "5:40"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var got []string
			for _, d := range constraintDiags(t, tc.src) {
				if d.Code == "feature-reference-featuring-types" {
					got = append(got, locationOf(tc.src, d.Span.Offset))
				}
			}
			if strings.Join(got, " ") != strings.Join(tc.want, " ") {
				t.Errorf("reported at %v, want %v", got, tc.want)
			}
		})
	}
}

// An end that names what the succession's own action features — a node of the
// same body, the subject or part of the definition the succession is written
// in, start and done, a node declared in the same loop body — is clean at
// every tier, so no lower tier hides the check.
func TestW8CSuccessionEndsInTheirOwnBodyStayClean(t *testing.T) {
	cases := map[string]string{
		"use case body relates its subject's performed actions": `package P {
	part def DroneSystem { action poll; action report; }
	use case def Monitor {
		subject droneSystem : DroneSystem {
			perform action ucPoll ::> poll;
			perform action ucReport ::> report;
		}
		first start then droneSystem.ucPoll;
		first droneSystem.ucPoll then droneSystem.ucReport;
		succession first droneSystem.ucReport then done;
	}
}`,
		"use case body relates its part's action": `package P {
	part def DroneSystem { action reportStatus; }
	use case def Holder { part s : DroneSystem; first start then s.reportStatus; }
}`,
		"loop body relates its own nodes": `package P {
	action def A {
		while true {
			action a;
			action b;
			first start then a;
			first a then b;
			then done;
		}
	}
}`,
		"nested action relates its own nodes and a fork": `package P {
	action def A {
		action nested {
			action a;
			action b;
			fork f;
			first start then f;
			first f then a;
			first f then b;
		}
	}
}`,
		"action def relates nodes through a dotted path of its own": `package P {
	action def Step { action go; }
	action def A {
		action s1 : Step;
		action s2 : Step;
		first s1.go then s2.go;
	}
}`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if diags := analyzeAll(t, "clean.sysml", src); len(diags) != 0 {
				t.Errorf("diagnostics: %v", diags)
			}
		})
	}
}

// locationOf spells offset as line:column in src, 1-based.
func locationOf(src string, offset int) string {
	line, col := 1, 1
	for i := 0; i < offset && i < len(src); i++ {
		if src[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return strings.Join([]string{itoa(line), itoa(col)}, ":")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

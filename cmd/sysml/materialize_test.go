package main

import (
	"errors"
	"strings"
	"testing"
)

// TestCheckReportsMaterializationDiagnostics checks the exit-status rule for the
// diagnostics only materializing an object produces: creating it is part of the
// run, so a default whose value count does not conform to its feature's
// multiplicity is reported and the run exits 2 — never `no errors` and 0.
func TestCheckReportsMaterializationDiagnostics(t *testing.T) {
	binary := buildCLI(t)

	cases := []struct {
		name   string
		model  string
		object string
		status int
		want   []string
		// once is a diagnostic the report must carry exactly one of.
		once string
	}{
		{
			name: "a conforming default is materialized and the model reported clean",
			model: `package M {
    private import ScalarValues::Real;
    part def Sub { attribute volume : Real = 2.0; }
    part def Craft {
        part tank : Sub;
        attribute volumes : Real[1] = tank.volume;
    }
    part craft : Craft;
}
`,
			object: "M::craft",
			status: 0,
			want:   []string{"✓ Created instance of M::craft", "no errors"},
		},
		{
			name: "a quantity attribute nothing values holds no value and is reported clean",
			model: `package M {
    private import ISQ::*;
    part def Engine { attribute mass :> ISQ::mass; }
    part def Craft { part engine : Engine; }
    part craft : Craft;
}
`,
			object: "M::craft",
			status: 0,
			want:   []string{"✓ Created instance of M::craft", "no errors"},
		},
		{
			name: "a default of fewer values than the declared lower bound is reported",
			model: `package M {
    private import ScalarValues::Real;
    part def Sub { attribute volume : Real = 2.0; }
    part def Craft {
        part tank : Sub;
        attribute volumes : Real[3] = tank.volume;
    }
    part craft : Craft;
}
`,
			object: "M::craft",
			status: 2,
			want:   []string{"feature value craft.volumes", "1 value(s) bound to a feature with multiplicity lower bound 3"},
		},
		{
			name: "a multi-valued default on a feature declaring no multiplicity is held to 1..1",
			model: `package M {
    private import ScalarValues::Real;
    part def Craft { attribute volumes : Real = (1.0, 2.0); }
    part craft : Craft;
}
`,
			object: "M::craft",
			status: 2,
			want:   []string{"feature value craft.volumes", "2 value(s) bound to a feature with multiplicity upper bound 1"},
		},
		{
			name: "a redefined feature and the feature it redefines read one feature value, reported once",
			model: `package M {
    private import ScalarValues::Real;
    part def Base { attribute mass : Real; }
    part def Craft :> Base { attribute grossMass :>> mass = (1.0, 2.0); }
    part craft : Craft;
}
`,
			object: "M::craft",
			status: 2,
			want:   []string{"2 value(s) bound to a feature with multiplicity upper bound 1"},
			once:   "multiplicity violation",
		},
		{
			name: "a wide model bounds the check rather than materializing without end",
			model: `package M {
    private import ScalarValues::Real;
    part def Leaf { attribute v : Real = 1.0; }
    part def L3 { part leaves : Leaf[100]; }
    part def L2 { part inner : L3[100]; }
    part def L1 { part inner : L2[100]; }
    part def Craft { part inner : L1[100]; }
    part craft : Craft;
}
`,
			object: "M::craft",
			status: 0,
			want:   []string{"materialization is bounded", "no errors in the feature values checked"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := check(t, binary, tc.model, "-instantiate", tc.object, "-validate")
			wantReport(t, got, tc.status, tc.want...)
			if tc.status != 0 && strings.Contains(got.output(), "no errors") {
				t.Errorf("an invalid model was reported as having no errors:\n%s", got.output())
			}
			// A diagnostic about the model belongs on stderr, where the other
			// findings of a run that decided nothing are reported.
			if tc.status != 0 && strings.Contains(got.stdout, "multiplicity") {
				t.Errorf("a diagnostic was reported on stdout:\n%s", got.stdout)
			}
			if tc.once != "" {
				if n := strings.Count(got.output(), tc.once); n != 1 {
					t.Errorf("%q reported %d times, want once:\n%s", tc.once, n, got.output())
				}
			}
		})
	}
}

// A violation under nesting the walk does not descend into is unfound, so the
// run must report what it did not check rather than that there were no errors.
func TestCheckDoesNotReportCleanWhenNestingWasElided(t *testing.T) {
	binary := buildCLI(t)

	deep := `package M {
    private import ScalarValues::Real;
    part def L9 { attribute volumes : Real = (1.0, 2.0); }
    part def L8 { part c : L9; }
    part def L7 { part c : L8; }
    part def L6 { part c : L7; }
    part def L5 { part c : L6; }
    part def L4 { part c : L5; }
    part def L3 { part c : L4; }
    part def L2 { part c : L3; }
    part def L1 { part c : L2; }
    part def Craft { part c : L1; }
    part craft : Craft;
}
`
	recursive := `package M {
    private import ScalarValues::Real;
    part def Node { part child : Node; }
    part def Craft { part root : Node; }
    part craft : Craft;
}
`
	for name, model := range map[string]string{"deeper than the walk descends": deep, "a part holding its own kind": recursive} {
		t.Run(name, func(t *testing.T) {
			got := check(t, binary, model, "-instantiate", "M::craft", "-validate")
			if got.status != 0 {
				t.Errorf("exit status = %d, want 0: an elision is no model error\n%s", got.status, got.output())
			}
			if !strings.Contains(got.output(), "materialization is bounded") {
				t.Errorf("the run did not report what it left unchecked:\n%s", got.output())
			}
			for _, line := range strings.Split(got.output(), "\n") {
				if strings.HasSuffix(line, "no errors") {
					t.Errorf("a partly checked model was reported clean: %q\n%s", line, got.output())
				}
			}
		})
	}
}

// TestCheckReportsMaterializationDiagnosticsAsJSON checks that -json carries the
// same finding as data, so a script reads why the run was undecided rather than
// matching printed output.
func TestCheckReportsMaterializationDiagnosticsAsJSON(t *testing.T) {
	binary := buildCLI(t)

	const model = `package M {
    private import ScalarValues::Real;
    part def Craft { attribute volumes : Real = (1.0, 2.0); }
    part craft : Craft;
}
`
	got := check(t, binary, model, "-instantiate", "M::craft", "-validate", "-json")
	if got.status != 2 {
		t.Errorf("exit status = %d, want 2\n%s", got.status, got.output())
	}
	for _, want := range []string{`"status": "unresolved"`, `"code": "runtime.materialize"`,
		"2 value(s) bound to a feature with multiplicity upper bound 1"} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("the JSON report is missing %q:\n%s", want, got.stdout)
		}
	}
}

// unmaterializableModel binds two values to a feature declaring no multiplicity,
// so reading the feature value finds a default that does not conform to 1..1.
const unmaterializableModel = `package Demo { attribute def X { attribute bad : ScalarValues::Real = (1.0, 2.0); }
               part def R { attribute b : X; } }
`

// TestPipedSessionExitsOnAMaterializationFailure checks the exit-status rule for a
// session driven from a pipe: a command that reported a feature value it could not
// materialize answered nothing about it, so the run exits 2 with the diagnostic
// rendered, while a conforming model still exits 0.
func TestPipedSessionExitsOnAMaterializationFailure(t *testing.T) {
	binary := buildCLI(t)

	cases := []struct {
		name   string
		stdin  string
		model  string
		status int
		want   []string
	}{{
		name:   "a feature value listing that could not materialize leaves the run undecided",
		stdin:  "%instantiate Demo::R\n%features Demo::R\n",
		model:  unmaterializableModel,
		status: exitUnevaluable,
		want:   []string{"bad: <error:", "multiplicity violation: 2 value(s) bound to a feature with multiplicity upper bound 1"},
	}, {
		name:   "an evaluation of the same feature value leaves the run undecided",
		stdin:  "%instantiate Demo::R\n%eval bad\n",
		model:  unmaterializableModel,
		status: exitUnevaluable,
		want:   []string{"error: evaluation failed", "multiplicity violation"},
	}, {
		name:   "an evaluation pinned to the object holding it leaves the run undecided",
		stdin:  "%instantiate Demo::R\n%eval in Demo::R::b : bad\n",
		model:  unmaterializableModel,
		status: exitUnevaluable,
		want:   []string{"error:", "multiplicity violation"},
	}, {
		name:   "a name that is no feature value of the object decides nothing",
		stdin:  "%instantiate Demo::R\n%eval nosuch\n",
		model:  unmaterializableModel,
		status: exitHolds,
		want:   []string{"error:"},
	}, {
		name:   "quitting after the failure was reported does not report success",
		stdin:  "%instantiate Demo::R\n%features Demo::R\n%quit\n",
		model:  unmaterializableModel,
		status: exitUnevaluable,
		want:   []string{"bad: <error:"},
	}, {
		name:   "a model whose features materialize exits on what analysis found",
		stdin:  "%instantiate Rover::pack\n%features Rover::pack\n%quit\n",
		model:  checkModel,
		status: exitHolds,
		want:   []string{"capacity = 100"},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runPiped(t, binary, tc.stdin, tc.model)
			if got.status != tc.status {
				t.Errorf("exit status = %d, want %d\n%s", got.status, tc.status, got.output())
			}
			for _, want := range tc.want {
				if !strings.Contains(got.output(), want) {
					t.Errorf("the session did not report %q:\n%s", want, got.output())
				}
			}
		})
	}
}

// TestSessionStatusAtATerminal checks that the prompt is unaffected: at a terminal
// the session is where an unusable model gets fixed, so what a command could not
// materialize is reported at the prompt without deciding the run.
func TestSessionStatusAtATerminal(t *testing.T) {
	failure := []error{errors.New("feature value X.bad: multiplicity violation")}

	cases := []struct {
		name     string
		loaded   int
		terminal bool
		failures []error
		want     int
	}{
		{"a failure at the prompt decides nothing", exitHolds, true, failure, exitHolds},
		{"a model that did not analyse decides nothing at the prompt", exitUnevaluable, true, nil, exitHolds},
		{"a failure over a pipe leaves the run undecided", exitHolds, false, failure, exitUnevaluable},
		{"a pipe over a model that analysed reports success", exitHolds, false, nil, exitHolds},
		{"a pipe keeps what the load found", exitUnevaluable, false, nil, exitUnevaluable},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sessionStatus(tc.loaded, tc.terminal, tc.failures); got != tc.want {
				t.Errorf("sessionStatus = %d, want %d", got, tc.want)
			}
		})
	}
}

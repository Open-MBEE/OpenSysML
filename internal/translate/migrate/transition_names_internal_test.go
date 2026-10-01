package migrate

import (
	"strings"
	"testing"
)

// machineDiagram shows every vertex and transition of machineMembers.
func machineDiagram(shown ...string) string {
	return diagram("_d", "Overview", "_sm", "SysML State Machine Diagram", append([]string{"_idle", "_run", "_t_go", "_sm_init"}, shown...)...)
}

// TestTransitionNameAvoidsMembers covers a synthesized transition name that a
// member of the state def already bears: the transition is numbered past it, as
// is each further transition a multi-trigger transition is written as.
func TestTransitionNameAvoidsMembers(t *testing.T) {
	for _, tc := range []struct {
		name, members, diagram string
		want, wantNot          []string
	}{
		{"an operation named like the transition",
			strings.Replace(machineMembers, `<region xmi:type="uml:Region" xmi:id="_r">`,
				`<ownedOperation xmi:type="uml:Operation" xmi:id="_op" name="Idle accept Go then Run"/>
      <region xmi:type="uml:Region" xmi:id="_r">`, 1),
			machineDiagram(),
			[]string{"abstract action def 'Idle accept Go then Run';", "transition 'Idle accept Go then Run2' first Idle accept Go then Run;"},
			[]string{"transition 'Idle accept Go then Run' first"}},
		{"an operation named like the further transition of a multi-trigger transition",
			strings.NewReplacer(`<region xmi:type="uml:Region" xmi:id="_r">`,
				`<ownedOperation xmi:type="uml:Operation" xmi:id="_op" name="Idle accept Stop then Run"/>
      <region xmi:type="uml:Region" xmi:id="_r">`,
				`<trigger xmi:type="uml:Trigger" xmi:id="_tr_go" event="_ev_go"/>`,
				`<trigger xmi:type="uml:Trigger" xmi:id="_tr_go" event="_ev_go"/>
          <trigger xmi:type="uml:Trigger" xmi:id="_tr_stop" event="_ev_stop"/>`,
				`<packagedElement xmi:type="uml:SignalEvent" xmi:id="_ev_go" signal="_go"/>`,
				`<packagedElement xmi:type="uml:SignalEvent" xmi:id="_ev_go" signal="_go"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_stop" name="Stop"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_ev_stop" signal="_stop"/>`).Replace(machineMembers),
			machineDiagram(),
			[]string{"abstract action def 'Idle accept Stop then Run';",
				"transition 'Idle accept Go then Run' first Idle accept Go then Run;",
				"transition 'Idle accept Stop then Run2' first Idle accept Stop then Run;"},
			[]string{"transition 'Idle accept Stop then Run' first"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Migrate("names.xmi", []byte(diagramModel(tc.members, tc.diagram)))
			if err != nil {
				t.Fatal(err)
			}
			got := string(r.Notation)
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("notation lacks %q:\n%s", w, got)
				}
			}
			for _, w := range tc.wantNot {
				if strings.Contains(got, w) {
					t.Errorf("notation has %q:\n%s", w, got)
				}
			}
			for _, e := range r.Report.Entries {
				if e.ID == "_t_go" && e.Verdict == Unmapped {
					t.Errorf("transition is unmapped (%s)", e.Note)
				}
			}
		})
	}
}

// TestCommentAboutTransition covers a comment of the state machine annotating a
// transition, written before the transition is: the comment names the
// transition's member, given or synthesized, and is mapped.
func TestCommentAboutTransition(t *testing.T) {
	members := strings.Replace(machineMembers, `<region xmi:type="uml:Region" xmi:id="_r">`,
		`<ownedComment xmi:type="uml:Comment" xmi:id="_c_go" annotatedElement="_t_go" body="goes"/>
      <ownedComment xmi:type="uml:Comment" xmi:id="_c_halt" annotatedElement="_t_halt" body="stops"/>
      <region xmi:type="uml:Region" xmi:id="_r">
        <transition xmi:type="uml:Transition" xmi:id="_t_halt" name="halt" source="_run" target="_idle"/>`, 1)
	r, err := Migrate("comments.xmi", []byte(diagramModel(members, machineDiagram())))
	if err != nil {
		t.Fatal(err)
	}
	got := string(r.Notation)
	for _, w := range []string{
		"    comment about 'Idle accept Go then Run' /* goes */\n",
		"    comment about halt /* stops */\n",
		"transition 'Idle accept Go then Run' first Idle accept Go then Run;",
		"transition halt first Run then Idle;",
	} {
		if !strings.Contains(got, w) {
			t.Errorf("notation lacks %q:\n%s", w, got)
		}
	}
	if i, j := strings.Index(got, "comment about halt"), strings.Index(got, "transition halt"); i > j {
		t.Errorf("the comment is written after the transition:\n%s", got)
	}
	for _, id := range []string{"_c_go", "_c_halt"} {
		var found bool
		for _, e := range r.Report.Entries {
			if e.ID != id {
				continue
			}
			found = true
			if e.Verdict != Mapped || e.Note != "" {
				t.Errorf("%s is %s (%s), want mapped", id, e.Verdict, e.Note)
			}
		}
		if !found {
			t.Errorf("%s is not reported", id)
		}
	}
}

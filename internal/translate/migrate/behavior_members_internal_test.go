package migrate

import (
	"fmt"
	"strings"
	"testing"
)

// rule serializes a constraint id, named name, with an opaque specification.
func rule(id, name string) string {
	return `<ownedRule xmi:type="uml:Constraint" xmi:id="` + id + `" name="` + name + `">
	  <specification xmi:type="uml:OpaqueExpression" xmi:id="` + id + `s"><language>OCL</language><body>true</body></specification>
	</ownedRule>`
}

// TestBehaviorMembersUnwritten covers the members a behavior owns that its v2 body has no
// place for: they are reported unmapped, once, and a diagram showing them does not expose them.
func TestBehaviorMembersUnwritten(t *testing.T) {
	for _, tc := range []struct {
		name, members string
		unwritten     []string // ids reported unmapped as members the body has no place for
		written       []string // ids the body writes
		unexposed     int      // written ids the view still cannot expose
		wantNotation  []string
	}{
		{"an opaque behavior written as a calc def has no place for a constraint or attribute",
			`<packagedElement xmi:type="uml:OpaqueBehavior" xmi:id="_b" name="Halt">
			   <language>JavaScript</language><body>1;</body>` + rule("_r", "keep") + `
			   <ownedAttribute xmi:type="uml:Property" xmi:id="_p" name="p"/>
			 </packagedElement>`,
			[]string{"_r", "_p"}, nil, 0,
			[]string{"calc def Halt {\n    view Rules {\n        expose Halt;\n        render Views::asTextualNotation;\n    }\n    /* not migrated: Constraint 'keep' — owned by a OpaqueBehavior, whose v2 body is its parameters and code, not a place for a Constraint */"}},
		{"an opaque behavior written as an action def has no place for a constraint",
			`<packagedElement xmi:type="uml:OpaqueBehavior" xmi:id="_b" name="Halt">
			   <language>JavaScript</language><body>x = 1; y = 2;</body>` + rule("_r", "keep") + `
			 </packagedElement>`,
			[]string{"_r"}, nil, 0,
			[]string{"action def Halt {\n    view Rules : StandardViewDefinitions::ActionFlowView {\n        expose Halt;\n        render Views::asInterconnectionDiagram;\n    }\n    /* not migrated: Constraint 'keep' — owned by a OpaqueBehavior, whose v2 body is its parameters and code, not a place for a Constraint */"}},
		{"a function behavior has no place for a constraint",
			`<packagedElement xmi:type="uml:FunctionBehavior" xmi:id="_b" name="Square">
			   <language>JavaScript</language><body>1;</body>` + rule("_r", "keep") + `
			 </packagedElement>`,
			[]string{"_r"}, nil, 0,
			[]string{"calc def Square {\n    view Rules {\n        expose Square;\n        render Views::asTextualNotation;\n    }\n    /* not migrated: Constraint 'keep'"}},
		{"an opaque behavior that is an operation's method has no place for a constraint in the operation's body",
			`<packagedElement xmi:type="uml:Class" xmi:id="_valve" name="Valve">
			   <ownedOperation xmi:type="uml:Operation" xmi:id="_shut" name="Shut" method="_b"/>
			   <ownedBehavior xmi:type="uml:OpaqueBehavior" xmi:id="_b" name="Shutting" specification="_shut">
			     <language>JavaScript</language><body>x = 1; y = 2;</body>` + rule("_r", "keep") + `
			   </ownedBehavior>
			 </packagedElement>`,
			[]string{"_r"}, nil, 0,
			[]string{"action def Shut {\n        view Rules : StandardViewDefinitions::ActionFlowView {\n            expose Shut;\n            render Views::asInterconnectionDiagram;\n        }\n        /* not migrated: Constraint 'keep' — owned by a OpaqueBehavior"}},
		{"an interaction written as a scenario has no place for an attribute",
			`<packagedElement xmi:type="uml:Signal" xmi:id="_go" name="Go"/>
			 <packagedElement xmi:type="uml:Class" xmi:id="_rig" name="Rig">
			   <ownedAttribute xmi:type="uml:Property" xmi:id="_rCtrl" name="ctrl" type="_pump" aggregation="composite"/>
			   <ownedBehavior xmi:type="uml:Interaction" xmi:id="_b" name="Spin">
			     <ownedAttribute xmi:type="uml:Property" xmi:id="_p" name="p"/>
			     <lifeline xmi:type="uml:Lifeline" xmi:id="_lc" name="c" represents="_rCtrl"/>
			     <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_s" covered="_lc" message="_msg"/>
			     <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_rcv" covered="_lc" message="_msg"/>
			     <message xmi:type="uml:Message" xmi:id="_msg" name="go" messageSort="asynchSignal" signature="_go" sendEvent="_s" receiveEvent="_rcv"/>
			   </ownedBehavior>
			 </packagedElement>`,
			[]string{"_p"}, nil, 0,
			[]string{"action def Spin {\n        view Rules : StandardViewDefinitions::ActionFlowView {\n            expose Spin;\n            render Views::asInterconnectionDiagram;\n        }\n        /* not migrated: Property 'p' — owned by a Interaction, whose v2 body is its parameters and scenario steps, not a place for a Property */\n        action go send new Go() to this.ctrl;"}},
		{"an activity writes its constraint and attribute, and has no place for a port",
			`<packagedElement xmi:type="uml:Activity" xmi:id="_b" name="Run">` + rule("_r", "keep") + `
			   <ownedAttribute xmi:type="uml:Property" xmi:id="_p" name="p"/>
			   <ownedPort xmi:type="uml:Port" xmi:id="_q" name="q"/>
			 </packagedElement>`,
			[]string{"_q"}, []string{"_r", "_p"}, 1,
			[]string{"action def Run {\n    view Rules : StandardViewDefinitions::ActionFlowView {\n        expose Run;\n        expose p;\n        render Views::asInterconnectionDiagram;\n    }\n    /* not migrated: Port 'q' — owned by a Activity, whose v2 body is its parameters and flow, not a place for a Port */\n    ref p;\n    constraint keep { true }\n}"}},
		{"a state machine writes its constraint and attribute, and has no place for a port",
			`<packagedElement xmi:type="uml:StateMachine" xmi:id="_b" name="Modes">` + rule("_r", "keep") + `
			   <ownedAttribute xmi:type="uml:Property" xmi:id="_p" name="p"/>
			   <ownedPort xmi:type="uml:Port" xmi:id="_q" name="q"/>
			 </packagedElement>`,
			[]string{"_q"}, []string{"_r", "_p"}, 1,
			[]string{"state def Modes {\n    view Rules {\n        expose Modes;\n        expose p;\n        render Views::asTextualNotation;\n    }\n    /* not migrated: Port 'q' — owned by a StateMachine, whose v2 body is its parameters and states, not a place for a Port */\n    constraint keep { true }\n    ref p;"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			shown := append(append([]string{"_b"}, tc.unwritten...), tc.written...)
			r, err := Migrate("members.xmi", []byte(diagramModel(tc.members,
				diagram("_d", "Rules", "_b", "SysML Activity Diagram", shown...))))
			if err != nil {
				t.Fatal(err)
			}
			got := string(r.Notation)
			for _, w := range tc.wantNotation {
				if !strings.Contains(got, w) {
					t.Errorf("notation lacks %q:\n%s", w, got)
				}
			}
			entries := map[string][]Entry{}
			for _, e := range r.Report.Entries {
				entries[e.ID] = append(entries[e.ID], e)
			}
			for _, id := range tc.unwritten {
				es := entries[id]
				if len(es) != 1 || es[0].Verdict != Unmapped || !strings.Contains(es[0].Note, "not a place for a") {
					t.Errorf("entries for %s = %+v, want one unmapped as a member the body has no place for", id, es)
				}
				if strings.Contains(got, "expose "+es[0].Name+";") {
					t.Errorf("the diagram exposes the unwritten %s:\n%s", id, got)
				}
			}
			for _, id := range tc.written {
				es := entries[id]
				if len(es) != 1 || es[0].Verdict == Unmapped {
					t.Errorf("entries for %s = %+v, want one written", id, es)
				}
			}
			dropped := ""
			if n := len(tc.unwritten) + tc.unexposed; n > 0 {
				dropped = fmt.Sprintf("%d of %d shown elements are not written and not exposed", n, len(shown))
			}
			for _, e := range entries["_d"] {
				if !strings.Contains(e.Note, dropped) {
					t.Errorf("diagram note %q lacks %q", e.Note, dropped)
				}
			}
		})
	}
}

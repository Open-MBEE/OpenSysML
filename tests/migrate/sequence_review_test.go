package migrate_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// A combined fragment whose operator is not alt/opt/loop/par/seq/strict (critical, neg,
// break, assert, ignore, consider) is still a sequence: its operand's messages are written
// in occurrence order under the operator comment, and the fragment is approximated,
// not the whole interaction refused.
func TestCombinedFragmentOperatorsBeyondTheActionSubset(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/rig_interactions.xmi")
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range []string{"critical", "neg", "break", "assert", "ignore", "consider"} {
		t.Run(op, func(t *testing.T) {
			changed := bytes.Replace(data, []byte(`interactionOperator="opt"`), []byte(`interactionOperator="`+op+`"`), 1)
			if bytes.Equal(changed, data) {
				t.Fatal("the fixture no longer has the opt fragment to retype")
			}
			r, err := migrate.Migrate("rig_interactions.xmi", changed)
			if err != nil {
				t.Fatalf("Migrate: %v", err)
			}
			wantLine(t, r.Notation, "/* "+op+" [ctrl.got > 10.0] */")
			wantLine(t, r.Notation, "message 'new' from c.newSent to m.newReceived;")
			wantNote(t, r, "_opt", migrate.Approximated, "the "+op+" fragment has no form in v2 sequence notation")
			wantNote(t, r, "_spinup", migrate.Mapped, "written as an occurrence def")
			for _, d := range errors(t, "rig_interactions.sysml", r.Notation) {
				t.Errorf("%v", d)
			}
		})
	}
}

// A «TestCase» interaction's parameters are declared once, by the verification
// def's parameter list, which the sequence's bindings then name.
func TestTestCaseInteractionDeclaresItsParametersOnce(t *testing.T) {
	withParam := strings.Replace(testBench, `<ownedBehavior xmi:type="uml:Interaction" xmi:id="_tc" name="Bench Test">`,
		`<ownedBehavior xmi:type="uml:Interaction" xmi:id="_tc" name="Bench Test">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_tcRpm" name="rpm" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
          <defaultValue xmi:type="uml:LiteralReal" xmi:id="_tcRpm0" value="12.0"/>
        </ownedParameter>`, 1)
	if withParam == testBench {
		t.Fatal("the bench fixture no longer has the test case to give a parameter")
	}
	r := migrateDocument(t, withParam, `
  <sysml:Block xmi:id="_s1" base_Class="_motor"/>
  <sysml:Block xmi:id="_s2" base_Class="_ctrl"/>
  <sysml:Block xmi:id="_s3" base_Class="_bench"/>
  <sysml:Block xmi:id="_s4" base_Class="_idle"/>
  <sysml:BindingConnector xmi:id="_s5" base_Connector="_bind"/>
  <sysml:TestCase xmi:id="_s6" base_Behavior="_tc"/>
  <sysml:TestCase xmi:id="_s7" base_Behavior="_idling"/>`)
	def := between(t, string(r.Notation), "verification def 'Bench Test'", "\n    }\n")
	if n := strings.Count(def, "in rpm :"); n != 1 {
		t.Errorf("rpm is declared %d times in the verification def, want once:\n%s", n, def)
	}
	wantLine(t, r.Notation, "in rpm : ScalarValues::Real[1] default = 12.0;")
	wantLine(t, r.Notation, "verification def 'Bench Test'")
	if bytes.Contains(r.Notation, []byte("attribute rpm")) {
		t.Errorf("the sequence declares its own attribute beside the verification parameter:\n%s", r.Notation)
	}
	for _, d := range errors(t, "bench.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
}

// between returns the text of s from the first from to the next to after it.
func between(t *testing.T, s, from, to string) string {
	t.Helper()
	i := strings.Index(s, from)
	if i < 0 {
		t.Fatalf("%q not found in:\n%s", from, s)
	}
	j := strings.Index(s[i:], to)
	if j < 0 {
		t.Fatalf("%q not found after %q in:\n%s", to, from, s)
	}
	return s[i : i+j]
}

// An interaction parameter's default value is kept on the occurrence def's attribute,
// and one outside the translated subset is kept as a comment and reported.
func TestInteractionParameterDefaultsAreKept(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/interaction_use.xmi")
	if err != nil {
		t.Fatal(err)
	}
	param := []byte(`<ownedParameter xmi:type="uml:Parameter" xmi:id="_suRpm" name="rpm" direction="in">`)
	if !bytes.Contains(data, param) {
		t.Fatal("the fixture no longer has the parameter to give a default")
	}
	t.Run("literal", func(t *testing.T) {
		changed := bytes.Replace(data, param, append(append([]byte{}, param...), []byte(`<defaultValue xmi:type="uml:LiteralReal" xmi:id="_suRpm0" value="12.0"/>`)...), 1)
		r, err := migrate.Migrate("interaction_use.xmi", changed)
		if err != nil {
			t.Fatalf("Migrate: %v", err)
		}
		wantLine(t, r.Notation, "attribute rpm : ScalarValues::Real default = 12.0;")
		wantNote(t, r, "_suRpm", migrate.Approximated, "written as the attribute rpm of the occurrence def")
		for _, d := range errors(t, "interaction_use.sysml", r.Notation) {
			t.Errorf("%v", d)
		}
	})
	t.Run("reads the context", func(t *testing.T) {
		changed := bytes.Replace(data, param, append(append([]byte{}, param...), []byte(`<defaultValue xmi:type="uml:OpaqueExpression" xmi:id="_suRpm0"><body>limit</body></defaultValue>`)...), 1)
		r, err := migrate.Migrate("interaction_use.xmi", changed)
		if err != nil {
			t.Fatalf("Migrate: %v", err)
		}
		wantLine(t, r.Notation, "attribute rpm : ScalarValues::Real;")
		wantLine(t, r.Notation, "/* default value not migrated: limit — reads limit of the interaction's context, which its occurrence def has no access to */")
		wantNote(t, r, "_suRpm", migrate.Approximated, "reads limit of the interaction's context, which its occurrence def has no access to")
		for _, d := range errors(t, "interaction_use.sysml", r.Notation) {
			t.Errorf("%v", d)
		}
	})
	t.Run("untranslatable", func(t *testing.T) {
		changed := bytes.Replace(data, param, append(append([]byte{}, param...), []byte(`<defaultValue xmi:type="uml:OpaqueExpression" xmi:id="_suRpm0"><language>JavaScript</language><body>a + "!"</body></defaultValue>`)...), 1)
		r, err := migrate.Migrate("interaction_use.xmi", changed)
		if err != nil {
			t.Fatalf("Migrate: %v", err)
		}
		wantLine(t, r.Notation, "attribute rpm : ScalarValues::Real;")
		wantLine(t, r.Notation, `/* default value not migrated: {JavaScript} a + "!" — `)
		wantNote(t, r, "_suRpm", migrate.Approximated, "default value not migrated:")
		for _, d := range errors(t, "interaction_use.sysml", r.Notation) {
			t.Errorf("%v", d)
		}
	})
}

// A «TestCase» interaction's sequence diagram is still a sequence view of its
// verification def, with the lifeline heads placed and the message lines routed; owned
// by a use case, which is written as a usage, the def's subject subsets it.
func TestTestCaseSequenceDiagramIsASequenceView(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/usecase_sequence.xmi")
	if err != nil {
		t.Fatal(err)
	}
	changed := bytes.Replace(data, []byte("</xmi:XMI>"), []byte(`  <sysml:TestCase xmi:id="_sTC" base_Behavior="_seq"/>
</xmi:XMI>`), 1)
	if bytes.Equal(changed, data) {
		t.Fatal("the fixture no longer ends as expected")
	}
	r, err := migrate.MigrateOptions("usecase_sequence.xmi", streamArchiveOf(t, "usecase_sequence", changed), migrate.Options{})
	if err != nil {
		t.Fatalf("MigrateOptions: %v", err)
	}
	wantLine(t, r.Notation, "verification def 'Acquire Images' {")
	wantLine(t, r.Notation, "subject context :> Model::'Acquire Images';")
	wantLine(t, r.Notation, "view 'Acquire Images' : StandardViewDefinitions::SequenceView {")
	wantLine(t, r.Notation, "metadata DiagramLayout::Layout about operator { x = 20; y = 20; width = 120; height = 50; }")
	wantLine(t, r.Notation, "metadata DiagramLayout::Route about move { points = (80, 120, 260, 120); }")
	if l := r.Report.Layout; l == nil || l.PlacementsWritten != 5 || l.RoutesWritten != 4 {
		t.Errorf("layout report = %+v, want 5 lifeline heads placed and 4 message lines routed", l)
	}
	for _, d := range errors(t, "usecase_sequence.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
}

// An interaction use whose argument cannot be written leaves the parameter unbound,
// and says so: the use is approximated, not mapped.
func TestInteractionUseWithAnUnwritableArgumentIsApproximated(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/interaction_use.xmi")
	if err != nil {
		t.Fatal(err)
	}
	changed := bytes.Replace(data, []byte(`<argument xmi:type="uml:LiteralReal" xmi:id="_oUseRpm" value="5.0"/>`),
		[]byte(`<argument xmi:type="uml:OpaqueExpression" xmi:id="_oUseRpm"><language>JavaScript</language><body>a + "!"</body></argument>`), 1)
	if bytes.Equal(changed, data) {
		t.Fatal("the fixture no longer has the argument to spoil")
	}
	r, err := migrate.Migrate("interaction_use.xmi", changed)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	wantNote(t, r, "_oUse", migrate.Approximated, "is not written, which stays unbound")
	wantLine(t, r.Notation, "occurrence 'spin Up' : 'Spin Up';")
	for _, d := range errors(t, "interaction_use.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
}

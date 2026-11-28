package export_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// An action a `perform`, `entry`, `do` or `exit` introduces is a PerformActionUsage, so one written with a
// statement owns the statement's node: the statement's metaclass is still an element of the graph.
func TestPerformedActionNodeOwnsItsStatement(t *testing.T) {
	statements := []struct{ name, text, metaclass string }{
		{"assignment", "assign p.w := 1;", "AssignmentActionUsage"},
		{"conditional", "if p.w > 0 {\n            action x;\n        }", "IfActionUsage"},
		{"while loop", "while p.w > 0 {\n            action x;\n        }", "WhileLoopActionUsage"},
		{"send", "send new Sig() to p;", "SendActionUsage"},
	}
	for _, keyword := range []string{"perform", "entry", "do", "exit"} {
		for _, head := range []struct{ name, text string }{{"name-only", "action n"}, {"typed", "action n : Act"}} {
			for _, statement := range statements {
				t.Run(keyword+" "+head.name+" "+statement.name, func(t *testing.T) {
					owner := "action def A"
					if keyword != "perform" {
						owner = "state def A"
					}
					src := "package P {\n    item def Sig;\n    part def PD { attribute w : ScalarValues::Integer; }\n    part p : PD;\n" +
						"    action def Act;\n    " + owner + " {\n        " + keyword + " " + head.text + " " + statement.text + "\n    }\n}\n"
					turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
					if err != nil {
						t.Fatalf("to turtle: %v", err)
					}
					assertPerformedStatement(t, turtle, statement.metaclass)
					back, err := convert.Convert("m.ttl", withoutTriples(t, turtle, "sysx:sourceText"), convert.FormatTurtle, convert.FormatSysML)
					if err != nil {
						t.Fatalf("back to notation: %v", err)
					}
					again, err := convert.Convert("m.sysml", back, convert.FormatSysML, convert.FormatTurtle)
					if err != nil {
						t.Fatalf("the notation written back does not convert: %v\n%s", err, back)
					}
					assertPerformedStatement(t, again, statement.metaclass)
					second, err := convert.Convert("m.ttl", withoutTriples(t, again, "sysx:sourceText"), convert.FormatTurtle, convert.FormatSysML)
					if err != nil {
						t.Fatalf("second pass: %v", err)
					}
					if string(second) != string(back) {
						t.Errorf("the notation is not a fixed point:\n%s\nthen\n%s", back, second)
					}
				})
			}
		}
	}
}

// assertPerformedStatement fails unless the graph states P::A::n as a PerformActionUsage and one element of
// metaclass, which n owns.
func assertPerformedStatement(t *testing.T, turtle []byte, metaclass string) {
	t.Helper()
	wrapper := performedNode.FindSubmatch(turtle)
	if wrapper == nil {
		t.Fatalf("the graph does not state n as a PerformActionUsage:\n%s", turtle)
	}
	var statements []string
	for _, subject := range strings.Split(string(turtle), "\n\n") {
		if strings.Contains(subject, "\n    a sysml:"+metaclass+" ;") {
			statements = append(statements, subject)
		}
	}
	if len(statements) != 1 {
		t.Fatalf("the graph states %d %s, want 1:\n%s", len(statements), metaclass, turtle)
	}
	if owner := "\n    sysml:owner " + string(wrapper[1]) + " ;"; !strings.Contains(statements[0], owner) {
		t.Errorf("n does not own the %s:\n%s", metaclass, statements[0])
	}
}

// A state's subaction is addressed under its membership (`P__A___400__n`).
var performedNode = regexp.MustCompile(`\n(elmt:P__A__(?:\w+__)?n)\n    a sysml:PerformActionUsage ;`)

var statedMetaclass = regexp.MustCompile(`\n    a sysml:(\w+) ;`)

// The runtime conformance models that perform a statement node convert back from a graph without
// `sysx:sourceText` to notation stating the same elements, at a fixed point.
func TestPerformedStatementConformanceModelsConvertBack(t *testing.T) {
	for _, name := range []string{
		"succession_join_predecessors", "succession_predecessors_fired", "state_portions",
		"action_terminate_this_ends_part", "action_terminate_owner_before_lazy_part", "clock_do_action_while_token_waits",
	} {
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join("..", "..", "internal", "exec", "runtime", "testdata", "conformance", name+".sysml"))
			if err != nil {
				t.Fatal(err)
			}
			turtle, err := convert.Convert(name+".sysml", src, convert.FormatSysML, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			back, err := convert.Convert(name+".ttl", withoutTriples(t, turtle, "sysx:sourceText"), convert.FormatTurtle, convert.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			again, err := convert.Convert(name+".sysml", back, convert.FormatSysML, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("the notation written back does not convert: %v\n%s", err, back)
			}
			want, got := statedMetaclass.FindAllStringSubmatch(string(turtle), -1), statedMetaclass.FindAllStringSubmatch(string(again), -1)
			count := func(stated [][]string) map[string]int {
				out := map[string]int{}
				for _, m := range stated {
					out[m[1]]++
				}
				return out
			}
			wantBy, gotBy := count(want), count(got)
			if wantBy["AssignmentActionUsage"] == 0 {
				t.Errorf("the graph states no AssignmentActionUsage:\n%s", turtle)
			}
			for metaclass, n := range wantBy {
				if gotBy[metaclass] != n {
					t.Errorf("%s: %d before, %d after the round trip", metaclass, n, gotBy[metaclass])
				}
			}
			second, err := convert.Convert(name+".ttl", withoutTriples(t, again, "sysx:sourceText"), convert.FormatTurtle, convert.FormatSysML)
			if err != nil {
				t.Fatalf("second pass: %v", err)
			}
			if string(second) != string(back) {
				t.Errorf("the notation is not a fixed point:\n%s\nthen\n%s", back, second)
			}
		})
	}
}

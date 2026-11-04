package passes

import (
	"strings"
	"testing"
)

// The five typings of issue #1885: each usage kind takes the metaclass its
// derived definition property names, whatever the keyword of the definition.
func TestUsageTypingFollowsTheDefinitionPropertyMetaclass(t *testing.T) {
	for _, src := range []string{
		"package P { action def AD; part def Q { state s : AD; } }",
		"package P { constraint def CD; part def Q { calc c : CD; } }",
		"package P { concern def C; viewpoint def V; part def Q { requirement r1 : C; requirement r2 : V; } }",
		"package P { constraint def CD; requirement def RD; part def Q { action a : CD; action b : RD; } }",
	} {
		if diags := typeDiags(t, src); len(diags) != 0 {
			t.Errorf("%s: expected no type diagnostics, got %v", src, diags)
		}
	}
	for _, tt := range []struct{ src, want string }{
		{"package P { part def PD; part def Q { in attribute a : PD; } }", "An attribute must be typed by attribute definitions."},
		{"package P { part def PD; part def Q { out attribute a : PD; } }", "An attribute must be typed by attribute definitions."},
		{"package P { part def PD; part def Q { attribute a : PD; } }", "An attribute must be typed by attribute definitions."},
		{"package P { part def PD; part def Q { state s : PD; } }", "A state must be typed by state definitions."},
		{"package P { action def AD; part def Q { calc c : AD; } }", "A calculation must be typed by one calculation definition."},
		{"package P { constraint def CD; part def Q { requirement r : CD; } }", "A requirement must be typed by one requirement definition."},
	} {
		diags := typeDiags(t, tt.src)
		if len(diags) != 1 || diags[0].Message != tt.want {
			t.Errorf("%s: want %q, got %v", tt.src, tt.want, diags)
		}
	}
}

// Every usage kind against the definition kinds its metaclass admits and
// rejects (SysML v2 §8.3, the `/…Definition` property of each usage metaclass).
func TestUsageTypingKindTable(t *testing.T) {
	cases := []struct {
		usage            string
		accepts, rejects []string
	}{
		{"attribute", []string{"attribute def", "enum def"}, []string{"part def", "item def", "occurrence def"}},
		{"enum", []string{"enum def"}, []string{"attribute def"}},
		{"occurrence", []string{"occurrence def", "part def", "port def", "action def"}, []string{"attribute def", "enum def"}},
		{"item", []string{"item def", "part def", "occurrence def", "action def", "metadata def"}, []string{"attribute def"}},
		{"part", []string{"part def", "item def", "occurrence def", "connection def", "view def"}, []string{"attribute def"}},
		{"port", []string{"port def"}, []string{"part def", "item def"}},
		{"connection", []string{"connection def", "interface def", "allocation def", "flow def"}, []string{"part def", "action def"}},
		{"interface", []string{"interface def"}, []string{"connection def", "allocation def"}},
		{"allocation", []string{"allocation def"}, []string{"connection def", "interface def"}},
		{"flow", []string{"flow def"}, []string{"connection def", "action def"}},
		{"action", []string{"action def", "state def", "calc def", "flow def", "use case def", "constraint def", "requirement def"}, []string{"part def", "port def"}},
		{"state", []string{"state def", "action def", "calc def", "constraint def"}, []string{"part def", "attribute def"}},
		{"calc", []string{"calc def", "case def", "analysis def", "constraint def", "requirement def", "viewpoint def"}, []string{"action def", "state def", "part def"}},
		{"constraint", []string{"constraint def", "requirement def", "concern def", "viewpoint def"}, []string{"calc def", "part def"}},
		{"requirement", []string{"requirement def", "concern def", "viewpoint def"}, []string{"constraint def", "part def"}},
		{"concern", []string{"concern def"}, []string{"requirement def", "viewpoint def"}},
		{"viewpoint", []string{"viewpoint def"}, []string{"requirement def", "concern def"}},
		{"view", []string{"view def"}, []string{"part def", "rendering def"}},
		{"rendering", []string{"rendering def"}, []string{"part def", "view def"}},
		{"case", []string{"case def", "analysis def", "verification def", "use case def"}, []string{"calc def", "part def"}},
		{"analysis", []string{"analysis def"}, []string{"case def", "verification def"}},
		{"verification", []string{"verification def", "case def"}, []string{"calc def", "part def"}},
		{"use case", []string{"use case def"}, []string{"case def", "analysis def"}},
		{"metadata", []string{"metadata def"}, []string{"item def", "part def"}},
	}
	for _, c := range cases {
		for _, def := range c.accepts {
			src := "package T { " + def + " D; " + c.usage + " u : D; }"
			if diags := except(typeDiags(t, src), "individual-typing"); len(diags) != 0 {
				t.Errorf("%s: expected no type diagnostics, got %v", src, diags)
			}
		}
		for _, def := range c.rejects {
			src := "package T { " + def + " D; " + c.usage + " u : D; }"
			diags := typeDiags(t, src)
			if len(diags) == 0 {
				t.Errorf("%s: expected a typing diagnostic", src)
				continue
			}
			if !strings.Contains(diags[0].Message, "typed by") && !strings.Contains(diags[0].Message, "kind mismatch") {
				t.Errorf("%s: unexpected diagnostic %v", src, diags)
			}
		}
	}
}

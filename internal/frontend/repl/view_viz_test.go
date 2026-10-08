package repl

import (
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

const vizModel = `package P {
  part def Wheel;
  part def Car { part fl : Wheel; part fr : Wheel; connect fl to fr; }
  part car : Car { part a : Wheel; part b : Wheel; connect a to b; }
  part lone : Wheel;
  state def Lamp { state off; state on; transition off then on; }
  state lamp : Lamp;
  action def Start { action a; action b; first a then b; }
  action start : Start;
  use case def Drive;
  part 'text';
}
`

func vizSession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	res := s.Submit(vizModel)
	for _, d := range res.Diagnostics {
		if d.Severity == diag.SeverityError {
			t.Fatalf("model did not load: %v", res.Diagnostics)
		}
	}
	return s
}

func TestVizChoosesTheKindTheElementsCallFor(t *testing.T) {
	s := vizSession(t)
	cases := []struct {
		line string
		kind string
	}{
		{"%viz P::Lamp", "state rendering"},
		{"%viz P::lamp", "state rendering"},
		{"%viz P::Start", "action rendering"},
		{"%viz P::start", "action rendering"},
		{"%viz P::car", "interconnection rendering"},
		{"%viz P::Car", "tree rendering"},
		{"%viz P::lone", "tree rendering"},
		{"%viz P", "tree rendering"},
		{"%viz P::Drive", "case rendering"},
		{"%viz P::Lamp P::lamp", "state rendering"},
		{"%viz P::Lamp P::Start", "mixed rendering"},
		{"%viz --view=DEFAULT P::Lamp", "state rendering"},
	}
	for _, c := range cases {
		got := run(t, s, c.line)
		if !strings.HasPrefix(got, c.kind) {
			t.Errorf("%s drew %q, want the %s", c.line, strings.SplitN(got, "\n", 2)[0], c.kind)
		}
		if !strings.Contains(got, "the elements call for") {
			t.Errorf("%s did not say the kind was chosen from the elements:\n%s", c.line, got)
		}
	}
}

func TestVizDrawsSeveralNamesInOneRendering(t *testing.T) {
	s := vizSession(t)
	got := run(t, s, "%viz P::Lamp P::Start")
	wants(t, got, "rendering P::Lamp, P::Start directly", "state def P::Lamp", "action def P::Start", "off -> on", "a -> b")
}

func TestVizTakesTheViewInAnyCase(t *testing.T) {
	s := vizSession(t)
	cases := map[string]string{
		"%viz --view STATE P::Lamp":          "state rendering",
		"%viz --view=state P::Lamp":          "state rendering",
		"%viz --view Tree P::Lamp":           "tree rendering",
		"%viz P::Lamp --view=Mixed":          "mixed rendering",
		"%viz --view INTERCONNECTION P::car": "interconnection rendering",
		"%viz --view ACTION P::Start":        "action rendering",
		"%viz --view SEQUENCE P::Car":        "sequence rendering",
		"%viz --view CASE P::Drive":          "case rendering",
	}
	for line, want := range cases {
		got := run(t, s, line)
		if !strings.HasPrefix(got, want) || strings.Contains(got, "call for") {
			t.Errorf("%s drew %q, want the %s as asked", line, strings.SplitN(got, "\n", 2)[0], want)
		}
	}
}

func TestVizTakesAFormWhereverItStands(t *testing.T) {
	s := vizSession(t)
	for _, line := range []string{"%viz mermaid P::Lamp", "%viz P::Lamp mermaid", "%viz --view STATE mermaid --style LR P::Lamp"} {
		wants(t, run(t, s, line), "stateDiagram-v2")
	}
	wants(t, run(t, s, "%viz plantuml P::car"), "@startuml")
	wants(t, run(t, s, "%viz --style PUMLCODE P::car"), "@startuml")
	wants(t, run(t, s, "%viz --style pumlcode plantuml P::car"), "@startuml")
	// A quoted word is a name, not a form.
	wants(t, run(t, s, "%viz 'text'"), "tree rendering", "part P::text")
	if got := run(t, s, "%viz text P::Car"); !strings.HasPrefix(got, "tree rendering") {
		t.Errorf("the text form drew %q", strings.SplitN(got, "\n", 2)[0])
	}
}

func TestVizMapsTheStylesItDraws(t *testing.T) {
	s := vizSession(t)
	wants(t, run(t, s, "%viz --style LR mermaid P::Lamp"), "direction LR")
	wants(t, run(t, s, "%viz --style=lr mermaid P::Car"), "flowchart LR")
	wants(t, run(t, s, "%viz --style BT mermaid P::Car"), "flowchart BT")
	wants(t, run(t, s, "%viz --style cameo dot P::Car"), "Arial")
	wants(t, run(t, s, "%viz --style okabe-ito --style full mermaid P::car"), "flowchart")
	if got := run(t, s, "%viz --style DEFAULT mermaid P::Car"); !strings.Contains(got, "flowchart") || strings.Contains(got, "not represented") {
		t.Errorf("the pilot's DEFAULT style is the pilot drawing style, not a note:\n%s", got)
	}
}

func TestVizNotesThePilotStylesItDoesNotDraw(t *testing.T) {
	s := vizSession(t)
	got := run(t, s, "%viz --style ortholine --style LR --style ShowInherited --style ORTHOLINE P::Car")
	wants(t, got, "tree rendering", "not represented:", "style ORTHOLINE (orthogonal line style) is not drawn", "style SHOWINHERITED (show inherited members) is not drawn")
	if strings.Count(got, "ORTHOLINE") != 1 {
		t.Errorf("a style given twice is noted once:\n%s", got)
	}
	wants(t, run(t, s, "%viz --style polyline mermaid P::Car"), "%% not represented: style POLYLINE (polyline style) is not drawn", "flowchart")
	wants(t, run(t, s, "%viz --style compmost dot P::Car"), "// not represented: style COMPMOST")
}

func TestVizMisuseShowsUsage(t *testing.T) {
	s := vizSession(t)
	cases := map[string]string{
		"%viz":                                        "name at least one element",
		"%viz --view STATE":                           "name at least one element",
		"%viz --view":                                 "--view takes a view name",
		"%viz --style":                                "--style takes a style name",
		"%viz --view nope P::Car":                     `unknown view "nope"; the views are DEFAULT, TREE, INTERCONNECTION, STATE, ACTION, SEQUENCE, MIXED, CASE`,
		"%viz --view=TABLE P::Car":                    `unknown view "TABLE"`,
		"%viz --view TREE --view TREE P::Car":         "--view is given twice",
		"%viz --style nosuch P::Car":                  `unknown style "nosuch"; the styles are TB, LR, RL, BT, pilot, cameo, `,
		"%viz --style LR --style TB P::Car":           "style LR and style TB both set the direction",
		"%viz --style pilot --style cameo P::Car":     "both set the drawing style",
		"%viz --style minimal --style full P::Car":    "both set the port display",
		"%viz --style viridis --style cividis P::Car": "both set the palette",
		"%viz --bogus P::Car":                         `unknown option "--bogus"; the options are --view and --style`,
	}
	for line, want := range cases {
		out, _, err := s.RunMeta(line)
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != 2 || !strings.HasPrefix(out[0], errPrefix) || !strings.Contains(out[0], want) || out[1] != vizUsage {
			t.Errorf("%q = %q, want %q and the usage line", line, out, want)
		}
		_, err = s.Viz(MetaArgs(line)[1:], view.FormText)
		var usage *UsageError
		if !asUsageError(err, &usage) || !slices.Equal(usage.Lines, out) || usage.Error() != strings.TrimPrefix(out[0], errPrefix) {
			t.Errorf("Viz(%q) = %v, want a *UsageError holding %q", line, err, out)
		}
	}
	for _, line := range []string{"%viz --help", "%viz -h", "%viz P::Car --help"} {
		if out, _, _ := s.RunMeta(line); !slices.Equal(out, []string{vizUsage}) {
			t.Errorf("%q = %q, want the usage line", line, out)
		}
	}
}

func TestVizNamesWhatItCannotResolve(t *testing.T) {
	s := vizSession(t)
	got := run(t, s, "%viz P::Car P::Nope")
	if !strings.HasPrefix(got, errPrefix) || !strings.Contains(got, "P::Nope") {
		t.Errorf("an unresolved name was not reported by name: %q", got)
	}
	if _, err := s.Viz([]string{"Nope"}, view.FormText); err == nil || !strings.Contains(err.Error(), "Nope") {
		t.Errorf("Viz(Nope) = %v, want an error naming it", err)
	}
	var usage *UsageError
	if _, err := s.Viz([]string{"Nope"}, view.FormText); asUsageError(err, &usage) {
		t.Error("an unresolved name is no usage problem")
	}
}

func TestVizFallsBackToTheFormsTheKindSupports(t *testing.T) {
	s := vizSession(t)
	rendered, err := s.Viz([]string{"P::Lamp"}, view.FormMermaid, view.FormDot)
	if err != nil {
		t.Fatal(err)
	}
	if len(rendered) != 2 || rendered[0].Form != view.FormMermaid || rendered[1].Form != view.FormDot {
		t.Fatalf("Viz fell back to %+v, want mermaid then dot", rendered)
	}
	wants(t, strings.Join(rendered[0].Lines, "\n"), "stateDiagram-v2")
	wants(t, strings.Join(rendered[1].Lines, "\n"), "digraph")
	// A sequence rendering has no DOT form, so DOT is left out.
	rendered, err = s.Viz([]string{"--view", "SEQUENCE", "P::Car"}, view.FormMermaid, view.FormDot)
	if err != nil {
		t.Fatal(err)
	}
	if len(rendered) != 1 || rendered[0].Form != view.FormMermaid {
		t.Errorf("a sequence rendering fell back to %+v, want mermaid alone", rendered)
	}
	// A form asked for is the only one written.
	rendered, err = s.Viz([]string{"dot", "P::Lamp"}, view.FormMermaid)
	if err != nil {
		t.Fatal(err)
	}
	if len(rendered) != 1 || rendered[0].Form != view.FormDot {
		t.Errorf("an asked-for form was answered with %+v", rendered)
	}
}

func TestVizIsInHelpAndCompletion(t *testing.T) {
	if !strings.Contains(strings.Join(helpText(), "\n"), "%viz [--view=<VIEW>] [--style=<STYLE>...] [form] <name> [<name>...]") {
		t.Error("the viz command is not in help with the pilot's grammar")
	}
	if !slices.Contains(metaCommands(), "%viz") || !KnownMeta("%viz") {
		t.Error("the viz command is not in the command table")
	}
	s := vizSession(t)
	cases := []struct {
		head      string
		want      []string
		forbidden []string
	}{
		{"%vi", []string{"%viz"}, nil},
		{"%viz --", []string{"--view=", "--style="}, nil},
		{"%viz --v", []string{"--view="}, []string{"--style="}},
		{"%viz --view=", []string{"--view=DEFAULT", "--view=TREE", "--view=STATE", "--view=CASE"}, nil},
		{"%viz --view=st", []string{"--view=STATE"}, []string{"--view=TREE"}},
		{"%viz --view ", []string{"DEFAULT", "TREE", "INTERCONNECTION", "STATE", "ACTION", "SEQUENCE", "MIXED", "CASE"}, []string{"P::Car", "mermaid"}},
		{"%viz --view tr", []string{"TREE"}, []string{"STATE"}},
		{"%viz --style ", []string{"LR", "TB", "pilot", "cameo", "okabe-ito", "minimal", "ORTHOLINE", "PUMLCODE"}, []string{"P::Car"}},
		{"%viz --style=or", []string{"--style=ORTHOLINE"}, nil},
		{"%viz --style LR --style ca", []string{"cameo"}, []string{"P::Car"}},
		{"%viz P::L", []string{"P::Lamp"}, []string{"mermaid"}},
		{"%viz P::l", []string{"P::lamp", "P::lone"}, []string{"mermaid"}},
		{"%viz --view STATE P::L", []string{"P::Lamp"}, nil},
		{"%viz me", []string{"mermaid"}, nil},
		{"%viz ", []string{"text", "mermaid", "dot", "--view=", "--style="}, nil},
		{"%viz mermaid P::L", []string{"P::Lamp"}, nil},
		{"%viz mermaid ", []string{"Car", "Lamp"}, []string{"markdown", "--view="}},
		{"%viz mermaid P::Car ", []string{"Car", "Lamp"}, []string{"markdown"}},
		{"%viz P::Car me", []string{"mermaid"}, nil},
	}
	for _, c := range cases {
		got := s.Complete(c.head, len(c.head))
		for _, want := range c.want {
			if !slices.Contains(got.Candidates, want) {
				t.Errorf("completing %q offered %v, want %s", c.head, got.Candidates, want)
			}
		}
		for _, forbidden := range c.forbidden {
			if slices.Contains(got.Candidates, forbidden) {
				t.Errorf("completing %q offered %s", c.head, forbidden)
			}
		}
	}
}

func asUsageError(err error, usage **UsageError) bool {
	u, ok := err.(*UsageError)
	if ok {
		*usage = u
	}
	return ok
}

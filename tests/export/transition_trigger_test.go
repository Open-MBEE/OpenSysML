package export_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// triggerModel writes every trigger form a transition takes (SysML.xtext
// TransitionUsage: TriggerActionMember; AcceptParameterPart, TriggerExpression),
// plus the OpenSysML `when <name>` that names an injected signal.
const triggerModel = `package T {
    attribute def Sig;
    attribute def Cmd;
    state def S {
        in port inPort;
        entry; then s1;
        state s1;
        state s2;
        transition t1 first s1 accept Sig then s2;
        transition t2 first s2 accept c : Cmd via inPort then s1;
        transition t3 first s1 accept after 5[SI::second] then s2;
        transition t4 first s2 accept at SI::second then s1;
        transition t5 first s1 accept when true then s2;
        transition t6 first s2 accept Sig via inPort then s1;
        transition t7 first s1 when 1 > 2 then s2;
        transition t8 first s2 when Sig then s1;
        transition t9 first s1 accept p : Cmd via inPort if p != null then s2;
    }
}
`

func triggerGraph(t *testing.T) (string, *rdf.Graph) {
	t.Helper()
	turtle, err := convert.Convert("t.sysml", []byte(triggerModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatal(err)
	}
	return string(turtle), graph
}

func element(name string) rdf.Term { return rdf.IRI(rdf.Element + name) }

// Each transition trigger is its TransitionUsage::triggerAction, an
// AcceptActionUsage owned through a TransitionFeatureMembership of kind trigger
// (formal/2026-03-02 § 8.3.17.9), whose payloadParameter carries the typing and
// whose receiverArgument is the `via` port, as the pinned pilot builds it.
func TestTransitionTriggerIsMetamodelStructure(t *testing.T) {
	_, g := triggerGraph(t)
	accepter := func(name string) rdf.Term {
		t.Helper()
		action, ok := g.Object(element("T__S__"+name), rdf.SysML+"triggerAction")
		if !ok {
			t.Fatalf("%s states no sysml:triggerAction", name)
		}
		if got := g.Type(action); got != rdf.SysML+"AcceptActionUsage" {
			t.Fatalf("%s's trigger action is a %s", name, got)
		}
		membership, ok := g.Object(action, rdf.SysML+"owningMembership")
		if !ok || g.Type(membership) != rdf.SysML+"TransitionFeatureMembership" {
			t.Fatalf("%s's trigger action is not owned by a TransitionFeatureMembership", name)
		}
		if kind, _ := g.Lexical(membership, rdf.SysML+"kind"); kind != "trigger" {
			t.Fatalf("%s's trigger membership has kind %q", name, kind)
		}
		if feature, _ := g.Object(membership, rdf.SysML+"transitionFeature"); feature != action {
			t.Fatalf("%s's trigger membership names %v as its transition feature", name, feature)
		}
		return action
	}
	payload := func(action rdf.Term) rdf.Term {
		t.Helper()
		p, ok := g.Object(action, rdf.SysML+"payloadParameter")
		if !ok {
			t.Fatalf("%s states no payloadParameter", action.Value)
		}
		return p
	}
	for _, name := range []string{"t1", "t2", "t3", "t4", "t5", "t6", "t7", "t9"} {
		action := accepter(name)
		for _, legacy := range []string{"trigger", "via"} {
			ns := rdf.OpenSysML
			if legacy == "via" {
				ns = rdf.SysML
			}
			if g.HasProperty(element("T__S__"+name), ns+legacy) {
				t.Errorf("%s still states its trigger as %s", name, legacy)
			}
		}
		if !g.HasProperty(payload(action), rdf.SysML+"isAccept") {
			t.Errorf("%s's payload parameter is not flagged isAccept", name)
		}
	}
	if typ, _ := g.Object(payload(accepter("t1")), rdf.SysML+"type"); typ != element("T__Sig") {
		t.Errorf("t1's payload is typed %v, want T::Sig", typ)
	}
	c := payload(accepter("t2"))
	if name, _ := g.Lexical(c, rdf.SysML+"declaredName"); name != "c" {
		t.Errorf("t2's payload is named %q, want c", name)
	}
	typed := false
	for _, rel := range g.Objects(c, rdf.SysML+"ownedRelationship") {
		if g.Type(rel) == rdf.SysML+"FeatureTyping" {
			if typ, _ := g.Object(rel, rdf.SysML+"type"); typ == element("T__Cmd") {
				typed = true
			}
		}
	}
	if !typed {
		t.Error("t2's payload owns no FeatureTyping to T::Cmd")
	}
	for _, name := range []string{"t2", "t6", "t9"} {
		receiver, ok := g.Object(accepter(name), rdf.SysML+"receiverArgument")
		if !ok {
			t.Fatalf("%s states no receiverArgument", name)
		}
		if got := g.Type(receiver); got != rdf.SysML+"FeatureReferenceExpression" {
			t.Errorf("%s's receiver argument is a %s", name, got)
		}
		if referent, _ := g.Object(receiver, rdf.SysML+"referent"); referent != element("T__S__inPort") {
			t.Errorf("%s's receiver argument refers to %v, want T::S::inPort", name, referent)
		}
	}
	for name, kind := range map[string]string{"t3": "after", "t4": "at", "t5": "when", "t7": "when"} {
		action := accepter(name)
		argument, ok := g.Object(action, rdf.SysML+"payloadArgument")
		if !ok {
			t.Fatalf("%s states no payloadArgument", name)
		}
		if value, _ := g.Object(payload(action), rdf.SysML+"value"); value != argument {
			t.Errorf("%s's payload argument is not its payload's value", name)
		}
		if got := g.Type(argument); got != rdf.SysML+"TriggerInvocationExpression" {
			t.Errorf("%s's payload argument is a %s", name, got)
		}
		if got, _ := g.Lexical(argument, rdf.SysML+"kind"); got != kind {
			t.Errorf("%s's trigger invocation has kind %q, want %q", name, got, kind)
		}
		if len(g.Objects(argument, rdf.SysML+"argument")) != 1 {
			t.Errorf("%s's trigger invocation should have one argument", name)
		}
	}
	// `when Sig` names an injected signal, which has no metamodel form.
	t8 := element("T__S__t8")
	if g.HasProperty(t8, rdf.SysML+"triggerAction") {
		t.Error("the injected-signal `when Sig` should not be written as a trigger action")
	}
	if trigger, _ := g.Lexical(t8, rdf.OpenSysML+"trigger"); trigger != "Sig" {
		t.Errorf("t8 sysx:trigger = %q, want Sig", trigger)
	}
	if keyword, _ := g.Lexical(t8, rdf.OpenSysML+"triggerKeyword"); keyword != "when" {
		t.Errorf("t8 sysx:triggerKeyword = %q, want when", keyword)
	}
}

// The trigger structure alone, source text stripped, writes each transition back.
func TestTransitionTriggerReadsFromStructureAlone(t *testing.T) {
	turtle, _ := triggerGraph(t)
	back := backFromTheGraphAlone(t, turtle)
	for _, line := range strings.Split(triggerModel, "\n") {
		if strings.Contains(line, "transition ") && !strings.Contains(back, strings.TrimSpace(line)) {
			t.Errorf("the graph alone did not write %q\n--- notation ---\n%s", strings.TrimSpace(line), back)
		}
	}
}

// Negative controls: without the predicates that carry them, the trigger's
// receiver and the trigger action itself are lost.
func TestTransitionTriggerStructureIsLoadBearing(t *testing.T) {
	turtle, _ := triggerGraph(t)
	stripped := withoutSourceText(t, []byte(turtle))

	// The receiver parameter still holds the port, so its missing link is
	// refused rather than the `via` silently dropped.
	noReceiver := withoutTriples(t, stripped, "sysml:receiverArgument")
	back, err := convert.Convert("m.ttl", noReceiver, convert.FormatTurtle, convert.FormatSysML)
	if err == nil || !strings.Contains(err.Error(), "no sysml:receiverArgument names it") {
		t.Errorf("a receiver parameter without sysml:receiverArgument should be refused, got %v:\n%s", err, back)
	}

	noKind := withoutTriples(t, stripped, "sysml:kind")
	back, err = convert.Convert("m.ttl", noKind, convert.FormatTurtle, convert.FormatSysML)
	if err == nil && strings.Contains(string(back), "accept Sig then s2") {
		t.Errorf("the trigger came back without its TransitionFeatureMembership kind:\n%s", back)
	}
}

// A sysx:trigger stated beside the structure must agree with it.
func TestTransitionTriggerStructureAndLegacyTextAgree(t *testing.T) {
	turtle, _ := triggerGraph(t)
	stripped := string(withoutSourceText(t, []byte(turtle)))
	marker := "    sysml:triggerAction elmt:T__S__t2___40trigger ;"
	if !strings.Contains(stripped, marker) {
		t.Fatalf("the graph no longer spells t2's head as expected:\n%s", stripped)
	}
	with := func(trigger string) ([]byte, error) {
		graph := strings.Replace(stripped, marker, "    sysx:trigger \""+trigger+"\" ;\n"+marker, 1)
		return convert.Convert("m.ttl", []byte(graph), convert.FormatTurtle, convert.FormatSysML)
	}
	back, err := with("c : Cmd")
	if err != nil {
		t.Fatalf("an agreeing sysx:trigger was refused: %v", err)
	}
	if !strings.Contains(string(back), "transition t2 first s2 accept c : Cmd via inPort then s1;") {
		t.Errorf("an agreeing sysx:trigger changed t2:\n%s", back)
	}
	if _, err := with("d : Cmd"); err == nil || !strings.Contains(err.Error(), "its sysx:trigger states") {
		t.Errorf("a disagreeing sysx:trigger should be refused, got %v", err)
	}
}

// Links the structure states twice must agree with each other, or the graph
// is refused rather than one of them dropped.
func TestTransitionTriggerLinksAgree(t *testing.T) {
	turtle, _ := triggerGraph(t)
	stripped := string(withoutSourceText(t, []byte(turtle)))
	refused := func(name, from, to, want string) {
		t.Helper()
		if !strings.Contains(stripped, from) {
			t.Fatalf("%s: the graph no longer states %q", name, from)
		}
		graph := strings.Replace(stripped, from, to, 1)
		back, err := convert.Convert("m.ttl", []byte(graph), convert.FormatTurtle, convert.FormatSysML)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want a refusal mentioning %q, got %v:\n%s", name, want, err, back)
		}
	}
	refused("triggerAction",
		"    sysml:triggerAction elmt:T__S__t1___40trigger ;",
		"    sysml:triggerAction elmt:T__S__t6___40trigger ;",
		"while its trigger membership owns")
	refused("payloadArgument",
		"    sysml:payloadArgument expr:T__S__t3___40trigger___400_pvalue .",
		"    sysml:payloadArgument expr:T__S__t4___40trigger___400_pvalue .",
		"is not the value of its payload parameter")
	refused("receiverArgument",
		"    sysml:receiverArgument expr:T__S__t2___40trigger___401_pvalue",
		"    sysml:receiverArgument expr:T__S__t6___40trigger___401_pvalue",
		"is not the value of its receiver parameter")
}

// A comment between `accept` and the payload does not hide the trigger form.
func TestCommentedTransitionTriggerIsStructure(t *testing.T) {
	model := `package T {
    attribute def Sig;
    state def S {
        entry; then s1;
        state s1;
        state s2;
        transition t1 first s1 accept /* note */ Sig then s2;
        transition t2 first s2 accept // note
            after 5[SI::second] then s1;
    }
}
`
	turtle, err := convert.Convert("t.sysml", []byte(model), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	g, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"T__S__t1", "T__S__t2"} {
		if !g.HasProperty(element(name), rdf.SysML+"triggerAction") {
			t.Errorf("%s: a commented trigger should still be a trigger action", name)
		}
		if g.HasProperty(element(name), rdf.OpenSysML+"triggerKeyword") {
			t.Errorf("%s: a comment was taken for the trigger keyword", name)
		}
	}
	back := backFromTheGraphAlone(t, string(turtle))
	for _, want := range []string{
		"transition t1 first s1 accept Sig then s2;",
		"transition t2 first s2 accept after 5[SI::second] then s1;",
	} {
		if !strings.Contains(back, want) {
			t.Errorf("the graph alone did not write %q\n--- notation ---\n%s", want, back)
		}
	}
}

// An earlier release wrote a transition's trigger as sysx:trigger text with
// sysx:triggerKeyword and sysml:via; that graph still reads back, source text
// stripped, and writes today's structure again.
func TestLegacyTransitionTriggerGraphStillReads(t *testing.T) {
	legacy, err := os.ReadFile(filepath.Join("testdata", "legacy_transition_triggers.ttl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(legacy), "sysx:trigger ") || strings.Contains(string(legacy), "sysml:triggerAction") {
		t.Fatal("the legacy fixture is written in today's shape")
	}
	back, err := convert.Convert("legacy.ttl", withoutSourceText(t, legacy), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("legacy graph refused: %v", err)
	}
	for _, want := range []string{
		"transition off_to_start first off accept sig : Signal then starting;",
		"transition active_to_off first active accept sig : Signal via lane if monitor do action stop : Warm then off;",
	} {
		if !strings.Contains(string(back), want) {
			t.Errorf("the legacy graph did not write %q\n--- notation ---\n%s", want, back)
		}
	}
	again, err := convert.Convert("m.sysml", back, convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle again: %v", err)
	}
	if strings.Contains(string(again), "sysx:trigger ") || !strings.Contains(string(again), "sysml:receiverArgument") {
		t.Errorf("the legacy triggers should be written as structure again:\n%s", again)
	}
}

// The API element form carries the same trigger structure, and reads back
// from it with the source text dropped.
func TestTransitionTriggerAPIJSON(t *testing.T) {
	_, g := triggerGraph(t)
	document, err := export.WriteAPIJSON(g)
	if err != nil {
		t.Fatalf("WriteAPIJSON: %v", err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatal(err)
	}
	transition := apiJSONElement(t, elements, "T__S__t2")
	if !strings.Contains(strings.Join(strings.Fields(string(transition["triggerAction"])), ""), `{"@id":"T__S__t2___40trigger"}`) {
		t.Errorf("t2 triggerAction = %s", transition["triggerAction"])
	}
	membership := apiJSONElement(t, elements, "T__S__t2___40trigger_om")
	if string(membership["@type"]) != `"TransitionFeatureMembership"` || string(membership["kind"]) != `"trigger"` {
		t.Errorf("t2's trigger membership = %s %s", membership["@type"], membership["kind"])
	}
	accepter := apiJSONElement(t, elements, "T__S__t2___40trigger")
	for _, key := range []string{"payloadParameter", "receiverArgument"} {
		if _, ok := accepter[key]; !ok {
			t.Errorf("t2's AcceptActionUsage has no %s", key)
		}
	}
	for i := range elements {
		delete(elements[i], "sysx:sourceText")
		delete(elements[i], "sysx:sourceTail")
	}
	stripped, err := json.Marshal(elements)
	if err != nil {
		t.Fatal(err)
	}
	back, err := convert.Convert("m.json", stripped, convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back from the element form alone: %v", err)
	}
	for _, line := range strings.Split(triggerModel, "\n") {
		if strings.Contains(line, "transition ") && !strings.Contains(string(back), strings.TrimSpace(line)) {
			t.Errorf("the element form alone did not write %q\n--- notation ---\n%s", strings.TrimSpace(line), back)
		}
	}
}

package semantics

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// successionsOf are the SuccessionAsUsages owner owns, in order, through any membership.
func successionsOf(t *testing.T, model *Model, owner *symbols.Symbol) []*symbols.Symbol {
	t.Helper()
	var out []*symbols.Symbol
	for _, rel := range model.OwnedRelationships(owner) {
		member := rel.Implicit.Member
		if member == nil {
			continue
		}
		if mc := model.MetaclassOf(member); mc != nil && mc.Name == "SuccessionAsUsage" {
			out = append(out, member)
		}
	}
	return out
}

// describeFeature names a feature, or its metaclass in angle brackets where it is nameless.
func describeFeature(model *Model, sym *symbols.Symbol) string {
	if sym == nil {
		return "<nil>"
	}
	if sym.Name != "" {
		return sym.Name
	}
	if mc := model.MetaclassOf(sym); mc != nil {
		return "<" + mc.Name + ">"
	}
	return "<?>"
}

func reflectedFeatures(model *Model, sym *symbols.Symbol, property string) string {
	elements, ok := model.ReflectiveElements(sym, property)
	if !ok {
		return "unanswered"
	}
	var out []string
	for _, element := range elements {
		out = append(out, describeFeature(model, element))
	}
	return strings.Join(out, " ")
}

// A bare `assign` or `terminate` between two `then`s is the action usage its owner owns
// (SysML.xtext AssignmentNode, TerminateNode), so the successions around it relate it.
func TestThenSuccessionRelatesStatementActions(t *testing.T) {
	const src = `package P {
		action def Settle {
			out t : Real;
			first start;
			then perform action prepare;
			then assign t := 1.0;
			then terminate;
		}
	}`
	want := []string{
		"start prepare",
		"prepare <AssignmentActionUsage>",
		"<AssignmentActionUsage> <TerminateActionUsage>",
	}
	for f, fixture := range reflectiveFixtures(t, "then-statements.sysml", source.KindSysML, src) {
		model := fixture.model
		successions := successionsOf(t, model, nestedSym(t, fixture.root, "P::Settle"))
		if len(successions) != len(want) {
			t.Fatalf("fixture %d: %d successions, want %d", f, len(successions), len(want))
		}
		for i, succession := range successions {
			if got := reflectedFeatures(model, succession, "relatedFeature"); got != want[i] {
				t.Errorf("fixture %d succession %d relatedFeature = %q, want %q", f, i, got, want[i])
			}
			ends := strings.Fields(want[i])
			if got := reflectedFeatures(model, succession, "sourceFeature"); got != ends[0] {
				t.Errorf("fixture %d succession %d sourceFeature = %q, want %q", f, i, got, ends[0])
			}
			if got := reflectedFeatures(model, succession, "targetFeature"); got != ends[1] {
				t.Errorf("fixture %d succession %d targetFeature = %q, want %q", f, i, got, ends[1])
			}
		}
	}
}

// An action body's `first a then b;` is a SuccessionAsUsage the action owns (SysML.xtext ActionBodyItem),
// whose two ConnectorEndMembers each own the ReferenceSubsetting of the feature written.
func TestTwoEndedFirstThenIsAnOwnedSuccession(t *testing.T) {
	const src = `package P {
		action def Two {
			action a; action b; action c;
			first a then b;
			succession s first b then c;
			first start then a;
			first c then [0..1] done;
		}
	}`
	want := []string{"a b", "b c", "start a", "c done"}
	fixture := reflectiveFixtures(t, "two-ended.sysml", source.KindSysML, src)[0]
	model := fixture.model
	successions := successionsOf(t, model, nestedSym(t, fixture.root, "P::Two"))
	if len(successions) != len(want) {
		t.Fatalf("%d successions, want %d", len(successions), len(want))
	}
	for i, succession := range successions {
		if got := reflectedFeatures(model, succession, "relatedFeature"); got != want[i] {
			t.Errorf("succession %d relatedFeature = %q, want %q", i, got, want[i])
		}
		ends, _ := model.ReflectiveElements(succession, "connectorEnd")
		if len(ends) != 2 {
			t.Errorf("succession %d owns %d connector ends, want 2", i, len(ends))
			continue
		}
		var referenced, memberships []string
		for _, end := range ends {
			subsettings, _ := model.ReflectiveElements(end, "ownedReferenceSubsetting")
			for _, subsetting := range subsettings {
				referenced = append(referenced, reflectedFeatures(model, subsetting, "referencedFeature"))
			}
			if membership := model.OwningMembership(end); membership != nil {
				memberships = append(memberships, relMetaName(model, membership))
			}
		}
		if got := strings.Join(referenced, " "); got != want[i] {
			t.Errorf("succession %d ends reference %q, want %q", i, got, want[i])
		}
		if got := strings.Join(memberships, " "); got != "EndFeatureMembership EndFeatureMembership" {
			t.Errorf("succession %d end memberships = %q", i, got)
		}
	}
}

// relatedFeature is what a connector's ends reference (KerML 1.1 §8.3.4.5.3 deriveConnectorRelatedFeature):
// for every succession form of an action or state body it is read from the two ends the succession owns.
func TestSuccessionRelatedFeaturesAreItsEndsReferences(t *testing.T) {
	const src = `package P {
		item def Sig;
		action def A {
			action a;
			then action b;
			then send Sig() to a;
			then accept Sig;
			then fork f;
			then a;
			then b;
			first a then b;
			action;
			then done;
		}
		state def S {
			entry;
			then idle;
			state idle;
			then running;
			state running;
			transition first running then idle;
			transition t2 first idle accept Sig then running;
		}
	}`
	wantRelated := map[string][]string{
		"P::A": {"a b", "b <SendActionUsage>", "<SendActionUsage> <AcceptActionUsage>", "<AcceptActionUsage> f", "f a", "f b", "a b", "<ActionUsage> done"},
		"P::S": {"<ActionUsage> idle"},
	}
	fixture := reflectiveFixtures(t, "ends-derive.sysml", source.KindSysML, src)[0]
	model := fixture.model
	check := func(label string, succession *symbols.Symbol, want string) {
		t.Helper()
		ends, _ := model.ReflectiveElements(succession, "connectorEnd")
		if len(ends) != 2 {
			t.Errorf("%s owns %d connector ends, want 2", label, len(ends))
			return
		}
		var fromEnds []string
		for _, end := range ends {
			if end.OwnerScope == nil || end.OwnerScope.Owner() != succession {
				t.Errorf("%s: an end is not owned by the succession", label)
			}
			fromEnds = append(fromEnds, describeFeature(model, model.EndReferencedFeature(end)))
		}
		if got := strings.Join(fromEnds, " "); got != want {
			t.Errorf("%s ends reference %q, want %q", label, got, want)
		}
		if got := reflectedFeatures(model, succession, "relatedFeature"); got != want {
			t.Errorf("%s relatedFeature = %q, want %q", label, got, want)
		}
	}
	for owner, want := range wantRelated {
		successions := successionsOf(t, model, nestedSym(t, fixture.root, owner))
		if len(successions) != len(want) {
			t.Fatalf("%s: %d successions, want %d", owner, len(successions), len(want))
		}
		for i, succession := range successions {
			check(owner+" succession "+string(rune('0'+i)), succession, want[i])
		}
	}
	// Each transition owns its succession (SysML.xtext TransitionSuccessionMember), whose source the transition names.
	var transitions []string
	for _, rel := range model.OwnedRelationships(nestedSym(t, fixture.root, "P::S")) {
		member := rel.Implicit.Member
		if member == nil || model.MetaclassOf(member) == nil || model.MetaclassOf(member).Name != "TransitionUsage" {
			continue
		}
		successions := successionsOf(t, model, member)
		if len(successions) != 1 {
			t.Errorf("transition %d owns %d successions, want 1", len(transitions), len(successions))
			continue
		}
		want := []string{"idle running", "running idle", "idle running"}[len(transitions)]
		check("transition succession", successions[0], want)
		transitions = append(transitions, want)
	}
	if len(transitions) != 3 {
		t.Errorf("%d transitions, want 3", len(transitions))
	}
}

// A bare `if`, `while` or `for` statement has no element standing for it yet, so a succession reaching
// one leaves relatedFeature unanswered rather than one feature short.
func TestSuccessionAtAControlStatementIsUnanswered(t *testing.T) {
	const src = `package P {
		action def L {
			attribute x : ScalarValues::Integer;
			action a; action c;
			first a;
			then while x < 3 { assign x := x + 1; }
			then if x > 1 { action q; }
			then for i in (1, 2) { action r; }
			then c;
		}
	}`
	fixture := reflectiveFixtures(t, "control-statements.sysml", source.KindSysML, src)[0]
	successions := successionsOf(t, fixture.model, nestedSym(t, fixture.root, "P::L"))
	if len(successions) != 4 {
		t.Fatalf("%d successions, want 4", len(successions))
	}
	for i, succession := range successions {
		if got := reflectedFeatures(fixture.model, succession, "relatedFeature"); got != "unanswered" {
			t.Errorf("succession %d relatedFeature = %q, want it unanswered", i, got)
		}
	}
}

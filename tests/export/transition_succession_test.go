package export_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// A `transition t first a then b;` owns its succession as a SuccessionAsUsage
// through an OwningMembership (SysML.xtext TransitionSuccessionMember): the
// succession's first end is the empty source end, its second end a connector
// end whose ReferenceSubsetting names the target. The transition still states
// sysml:source/sysml:target itself, and the owned succession is implied — it is
// never written back as a member.
const transitionSuccessionModel = `package T {
    attribute g : Boolean;
    state def S {
        state a;
        state b;
        state c;
        transition t first a then b;
        transition u first b if g then c;
        succession v first a if g then c;
    }
}
`

func TestTransitionOwnsItsSuccession(t *testing.T) {
	turtle, back := graphOnlyRoundTrip(t, "t.sysml", []byte(transitionSuccessionModel))
	graph := string(turtle)
	for _, want := range []string{
		"sysml:succession elmt:T__S__t_succession ;",
		"sysml:source elmt:T__S__a ;",
		"sysml:target elmt:T__S__b ;",
		"elmt:T__S__t_succession\n    a sysml:SuccessionAsUsage ;",
		"elmt:T__S__t_succession_om\n    a sysml:OwningMembership ;",
		"expr:T__S__t_succession_pend0\n    a sysml:ReferenceUsage ;",
		"expr:T__S__t_succession_pend1\n    a sysml:ReferenceUsage ;",
		"expr:T__S__t_succession_pend0_om\n    a sysml:EndFeatureMembership ;",
		"expr:T__S__t_succession_pend1_om\n    a sysml:EndFeatureMembership ;",
		"expr:T__S__t_succession_pend1_prs\n    a sysml:ReferenceSubsetting ;",
		"sysml:isEnd \"true\"^^xsd:boolean ;",
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("the graph should state %q:\n%s", want, graph)
		}
	}
	// Every transition owns its succession: the guarded one and the `succession`
	// member alike.
	for _, want := range []string{
		"sysml:succession elmt:T__S__u_succession ;",
		"sysml:succession elmt:T__S__v_succession ;",
		"elmt:T__S__v_succession\n    a sysml:SuccessionAsUsage ;",
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("the graph should state %q:\n%s", want, graph)
		}
	}
	notation := string(back)
	for _, want := range []string{
		"transition t first a then b;\n",
		"transition u first b if g then c;\n",
		"succession v first a if g then c;\n",
	} {
		if !strings.Contains(notation, want) {
			t.Errorf("the notation should contain %q:\n%s", want, notation)
		}
	}
	// The implied succession is not a member: no `succession` line other than
	// the `succession v` member may appear, and nothing spells its ends.
	if strings.Count(notation, "succession") != 1 {
		t.Errorf("the owned succession came back as a member:\n%s", notation)
	}
}

// The grammar's member order puts the TransitionSuccessionMember after the
// guard and effect memberships and before the body's (SysML.xtext
// TransitionUsage): the ownedRelationship list states it in that order, after
// the FeatureChainMember naming the source and the EmptyParameterMember.
func TestTransitionSuccessionFollowsEffectAndPrecedesBody(t *testing.T) {
	src := `package T {
    attribute g : Boolean;
    state def S {
        state a;
        state b;
        state c;
        transition t first a if g do send a to b then c {
            state inner;
        }
    }
}
`
	turtle, back := graphOnlyRoundTrip(t, "t.sysml", []byte(src))
	graph := string(turtle)
	if !strings.Contains(graph,
		"sysml:ownedRelationship expr:T__S__t_psourcemember, expr:T__S__t_plinkparam_om, expr:T__S__t_pguard_om, elmt:T__S__t___400_om, elmt:T__S__t_succession_om, elmt:T__S__t__inner_om ;") {
		t.Errorf("the memberships should order source, parameter, guard, effect, succession, body:\n%s", graph)
	}
	notation := string(back)
	for _, want := range []string{"transition t first a if g do send a to b then c {\n", "state inner;\n"} {
		if !strings.Contains(notation, want) {
			t.Errorf("the notation should contain %q:\n%s", want, notation)
		}
	}
}

// A succession whose target disagrees with the transition's sysml:target is no
// restatement of the head: the conversion refuses it rather than dropping one.
func TestTransitionSuccessionDisagreementIsRefused(t *testing.T) {
	turtle, err := convert.Convert("t.sysml", []byte(transitionSuccessionModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph := string(turtle)
	block := "expr:T__S__t_succession_pend1_prs\n"
	start := strings.Index(graph, block)
	if start < 0 {
		t.Fatalf("the graph should state %s:\n%s", block, graph)
	}
	end := strings.Index(graph[start:], "\n\n")
	if end < 0 {
		t.Fatalf("unterminated block:\n%s", graph[start:])
	}
	mutated := graph[:start] +
		strings.ReplaceAll(graph[start:start+end], "elmt:T__S__b", "elmt:T__S__c") +
		graph[start+end:]
	if _, err := convert.Convert("m.ttl", []byte(mutated), convert.FormatTurtle, convert.FormatSysML); err == nil {
		t.Errorf("a succession targeting c under a transition targeting b should be refused")
	}
}

// A named transition owns, ahead of its trigger, the FeatureChainMember naming
// its source and an EmptyParameterMember, and a second EmptyParameterMember
// ahead of a trigger it owns (SysML-textual-bnf TransitionUsage, :1281-1290); a
// chained source or target is an owned feature chain (FeatureChainMember
// :1111-1116, OwnedReferenceSubsetting :463-466), not the feature it reaches.
func TestTransitionOwnsItsSourceParameterAndChainedEnds(t *testing.T) {
	src := "package P {\n    item def Sig;\n    state def S {\n        entry;\n        then a;\n        state a;\n        state b {\n            entry;\n            then c;\n            state c;\n        }\n" +
		"        transition t first a then b.c;\n        transition u first a accept Sig then b;\n        transition v first b.c then a;\n    }\n}\n"
	if diagnostics := diagnosticMessages("m.sysml", []byte(src)); len(diagnostics) > 0 {
		t.Fatalf("the model should analyse clean:\n%s", strings.Join(diagnostics, "\n"))
	}
	if _, back := graphOnlyRoundTrip(t, "m.sysml", []byte(src)); string(back) != src {
		t.Fatalf("the notation changed\n--- want ---\n%s\n--- got ---\n%s", src, back)
	}

	doc, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(doc, &elements); err != nil {
		t.Fatal(err)
	}
	byID := map[string]map[string]any{}
	for _, e := range elements {
		byID[e["@id"].(string)] = e
	}
	ref := func(v any) string {
		id, _ := v.(map[string]any)["@id"].(string)
		return id
	}
	refs := func(v any) []string {
		var out []string
		list, _ := v.([]any)
		for _, r := range list {
			out = append(out, ref(r))
		}
		return out
	}
	// chain is the qualified names a chain Feature chains, or nil.
	chain := func(id string) []string { return refs(byID[id]["chainingFeature"]) }
	transitions := map[string]map[string]any{}
	for _, e := range elements {
		if e["@type"] == "TransitionUsage" {
			transitions[e["declaredName"].(string)] = e
		}
	}
	for _, tc := range []struct {
		name        string
		head        []string // metaclasses of the owned relationships ahead of the succession
		source      []string // the source member: one element, or a chain
		target      []string // the target end's referenced feature: one element, or a chain
		chainSource bool
		chainTarget bool
	}{
		{"t", []string{"Membership", "ParameterMembership"}, []string{"P__S__a"}, []string{"P__S__b", "P__S__b__c"}, false, true},
		{"u", []string{"Membership", "ParameterMembership", "ParameterMembership", "TransitionFeatureMembership"}, []string{"P__S__a"}, []string{"P__S__b"}, false, false},
		{"v", []string{"OwningMembership", "ParameterMembership"}, []string{"P__S__b", "P__S__b__c"}, []string{"P__S__a"}, true, false},
	} {
		transition := transitions[tc.name]
		if transition == nil {
			t.Fatalf("no transition %s in the API JSON", tc.name)
		}
		owned := refs(transition["ownedRelationship"])
		if len(owned) < len(tc.head)+1 {
			t.Fatalf("%s: ownedRelationship %v, want %v ahead of the succession", tc.name, owned, tc.head)
		}
		for i, metaclass := range tc.head {
			if got := byID[owned[i]]["@type"]; got != metaclass {
				t.Errorf("%s: owned relationship %d is a %v, want %s", tc.name, i, got, metaclass)
			}
		}
		source := byID[owned[0]]
		if tc.chainSource {
			if got := chain(refs(source["ownedRelatedElement"])[0]); strings.Join(got, ",") != strings.Join(tc.source, ",") {
				t.Errorf("%s: source chain %v, want %v", tc.name, got, tc.source)
			}
		} else if got := ref(source["memberElement"]); got != tc.source[0] {
			t.Errorf("%s: source member %s, want %s", tc.name, got, tc.source[0])
		}
		for _, membership := range owned[1:] {
			if byID[membership]["@type"] != "ParameterMembership" {
				continue
			}
			parameter := byID[refs(byID[membership]["ownedRelatedElement"])[0]]
			if parameter["@type"] != "ReferenceUsage" || parameter["declaredName"] != nil {
				t.Errorf("%s: parameter %v, want an EmptyUsage", tc.name, parameter)
			}
		}
		succession := byID[ref(transition["succession"])]
		ends := refs(succession["connectorEnd"])
		subsetting := byID[ref(byID[ends[1]]["ownedReferenceSubsetting"])]
		referenced := ref(subsetting["referencedFeature"])
		if tc.chainTarget {
			if got := chain(referenced); strings.Join(got, ",") != strings.Join(tc.target, ",") {
				t.Errorf("%s: target chain %v, want %v", tc.name, got, tc.target)
			}
		} else if referenced != tc.target[0] {
			t.Errorf("%s: target %s, want %s", tc.name, referenced, tc.target[0])
		}
	}
	if _, err := convert.Convert("m.json", doc, convert.FormatAPIJSON, convert.FormatSysML); err != nil {
		t.Errorf("the API JSON did not read back: %v", err)
	}

	// A source member naming another state than the transition's source is
	// refused: the notation writes the source once.
	turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph := string(withoutTriples(t, turtle, "sysx:sourceText"))
	const member = "expr:P__S__t_psourcemember\n"
	at := strings.Index(graph, member)
	if at < 0 {
		t.Fatalf("no source member %s in the graph:\n%s", member, graph)
	}
	block := graph[at:]
	if end := strings.Index(block, "\n\n"); end >= 0 {
		block = block[:end]
	}
	edited := strings.Replace(graph, block, strings.Replace(block, "sysml:memberElement elmt:P__S__a", "sysml:memberElement elmt:P__S__b", 1), 1)
	if edited == graph {
		t.Fatalf("the source member was not edited:\n%s", block)
	}
	_, err = convert.Convert("m.ttl", []byte(edited), convert.FormatTurtle, convert.FormatSysML)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) || !strings.Contains(err.Error(), "source") {
		t.Errorf("expected the disagreeing source member to be refused, got %v", err)
	}
}

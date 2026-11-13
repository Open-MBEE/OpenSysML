package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const sendArgumentsModel = `package S {
    item def Alarm;
    part def Monitor {
        port alarmPort;
        part display;
        action supervise {
            action notifyVia send new Alarm() via alarmPort;
            action notifyTo send new Alarm() to display;
            action both send new Alarm() via alarmPort to display;
            action noPayload send via alarmPort;
            send new Alarm() to display;
        }
        state sm {
            entry send new Alarm() to display;
            state idle;
            transition first idle do send new Alarm() via alarmPort then idle;
        }
    }
    action def Supervise {
        action nested send 1 to Monitor::display;
    }
}
`

// sendMembers are the send actions scope declares, in source order.
func sendMembers(scope *symbols.Scope) []*symbols.Symbol {
	var out []*symbols.Symbol
	scope.ForEachMember(func(member *symbols.Symbol) bool {
		if member.Kind == symbols.SymbolSendActionUsage {
			out = append(out, member)
		}
		return true
	})
	return out
}

// TestSendActionReflectsItsArguments: a send action owns its payload, sender and receiver
// parameters in argument order, valued by its expressions, which are its payloadArgument,
// senderArgument and receiverArgument (SysML v2.1 §8.3.17.15).
func TestSendActionReflectsItsArguments(t *testing.T) {
	m, root := buildModelWithStdlib(t, sendArgumentsModel)
	monitor := nested(t, sym(t, root, "S").Scope, "Monitor")
	port := nested(t, monitor.Scope, "alarmPort")
	display := nested(t, monitor.Scope, "display")
	supervise := nested(t, monitor.Scope, "supervise")

	argument := func(send *symbols.Symbol, feature string) *symbols.Symbol {
		t.Helper()
		got, ok := m.ReflectiveElements(send, feature)
		if !ok {
			t.Fatalf("%s %s is not derived", send.Name, feature)
		}
		if len(got) == 0 {
			return nil
		}
		if len(got) != 1 {
			t.Fatalf("%s %s = %v, want at most one", send.Name, feature, got)
		}
		return got[0]
	}
	referent := func(expr *symbols.Symbol) *symbols.Symbol {
		t.Helper()
		if expr == nil {
			return nil
		}
		got, _ := m.ReflectiveElements(expr, "referent")
		if len(got) != 1 {
			return nil
		}
		return got[0]
	}
	cases := []struct {
		send              *symbols.Symbol
		params            int
		payload           string
		sender, receiver  *symbols.Symbol
		emptySenderBefore bool
	}{
		{send: nested(t, supervise.Scope, "notifyVia"), params: 2, payload: "ConstructorExpression", sender: port},
		{send: nested(t, supervise.Scope, "notifyTo"), params: 3, payload: "ConstructorExpression", receiver: display},
		{send: nested(t, supervise.Scope, "both"), params: 3, payload: "ConstructorExpression", sender: port, receiver: display},
		{send: nested(t, supervise.Scope, "noPayload"), params: 2, sender: port},
		{send: sendMembers(supervise.Scope)[4], params: 3, payload: "ConstructorExpression", receiver: display},
		{send: nested(t, sym(t, root, "S").Scope, "Supervise", "nested"), params: 3, payload: "LiteralInteger", receiver: display},
	}
	for _, tc := range cases {
		send := tc.send
		if got := m.MetaclassOf(send).Name; got != "SendActionUsage" {
			t.Fatalf("%s metaclass = %s, want SendActionUsage", send.Name, got)
		}
		if !m.reflectiveMetaclassConforms(send, "ActionUsage") {
			t.Errorf("%s does not conform to ActionUsage", send.Name)
		}
		for _, feature := range []string{"ownedFeature", "ownedMember", "parameter", "input", "directedFeature"} {
			got, _ := m.ReflectiveElements(send, feature)
			if len(got) != tc.params {
				t.Fatalf("%s %s = %v, want %d parameters", send.Name, feature, got, tc.params)
			}
			for i, param := range got {
				if name := m.MetaclassOf(param).Name; name != "ReferenceUsage" {
					t.Errorf("%s %s[%d] metaclass = %s, want ReferenceUsage", send.Name, feature, i, name)
				}
				if dir, ok := m.ReflectiveFeatureValue(param, "direction"); !ok || dir.RefFQN != "KerML::Core::FeatureDirectionKind::in" {
					t.Errorf("%s %s[%d] direction = %v, want in", send.Name, feature, i, dir)
				}
			}
		}
		params, _ := m.ReflectiveElements(send, "parameter")
		memberships := m.OwnedMemberships(send)
		if len(memberships) < tc.params {
			t.Fatalf("%s ownedMembership = %v, want the %d parameter memberships first", send.Name, memberships, tc.params)
		}
		for i := 0; i < tc.params; i++ {
			rel := memberships[i].Implicit
			if rel == nil || rel.Member != params[i] || m.MetaclassOf(memberships[i]).Name != "ParameterMembership" {
				t.Errorf("%s ownedMembership[%d] = %v, want the ParameterMembership of parameter %d", send.Name, i, memberships[i], i)
			}
		}
		payload := argument(send, "payloadArgument")
		if tc.payload == "" {
			if payload != nil {
				t.Errorf("%s payloadArgument = %v, want none", send.Name, payload)
			}
			if m.ValueExpression(params[0]) != nil {
				t.Errorf("%s payload parameter has a value, want an empty parameter", send.Name)
			}
		} else {
			if payload == nil || m.MetaclassOf(payload).Name != tc.payload {
				t.Fatalf("%s payloadArgument = %v, want a %s", send.Name, payload, tc.payload)
			}
			if m.ValueExpression(params[0]) != payload {
				t.Errorf("%s payload parameter value is not the payloadArgument", send.Name)
			}
			if owned := m.OwnedMemberships(params[0]); len(owned) != 1 || m.MetaclassOf(owned[0]).Name != "FeatureValue" {
				t.Errorf("%s payload parameter ownedMembership = %v, want one FeatureValue", send.Name, owned)
			}
		}
		sender := argument(send, "senderArgument")
		if got := referent(sender); got != tc.sender {
			t.Errorf("%s senderArgument referent = %v, want %v", send.Name, got, tc.sender)
		}
		if tc.sender == nil && tc.params == 3 && m.ValueExpression(params[1]) != nil {
			t.Errorf("%s sender parameter has a value, want the empty parameter of a `to`", send.Name)
		}
		if tc.sender != nil && m.ValueExpression(params[1]) != sender {
			t.Errorf("%s sender parameter value is not the senderArgument", send.Name)
		}
		receiver := argument(send, "receiverArgument")
		if got := referent(receiver); got != tc.receiver {
			t.Errorf("%s receiverArgument referent = %v, want %v", send.Name, got, tc.receiver)
		}
		if tc.receiver != nil && m.ValueExpression(params[2]) != receiver {
			t.Errorf("%s receiver parameter value is not the receiverArgument", send.Name)
		}
		if len(symbols.KeyOf(params[0]).String()) == 0 {
			t.Errorf("%s parameter has no identity", send.Name)
		}
		keys := map[string]bool{}
		for _, param := range params {
			key := symbols.KeyOf(param).String()
			if keys[key] {
				t.Errorf("%s parameters share the identity %s", send.Name, key)
			}
			keys[key] = true
		}
	}
}

// TestStateActionsAndEffectsAreSendActions: a state's entry send and a transition's
// `do send` effect classify as send actions with their arguments.
func TestStateActionsAndEffectsAreSendActions(t *testing.T) {
	m, root := buildModelWithStdlib(t, sendArgumentsModel)
	monitor := nested(t, sym(t, root, "S").Scope, "Monitor")
	sm := nested(t, monitor.Scope, "sm")
	entries := sendMembers(sm.Scope)
	if len(entries) != 1 {
		t.Fatalf("state sm declares %d send members, want the entry send", len(entries))
	}
	if got, _ := m.ReflectiveElements(entries[0], "receiverArgument"); len(got) != 1 {
		t.Errorf("entry send receiverArgument = %v, want the display reference", got)
	}
	var transition *symbols.Symbol
	sm.Scope.ForEachMember(func(member *symbols.Symbol) bool {
		if member.DeclaresTransition() {
			transition = member
		}
		return true
	})
	if transition == nil {
		t.Fatal("no transition in sm")
	}
	effects, _ := m.ReflectiveElements(transition, "effectAction")
	if len(effects) != 1 || m.MetaclassOf(effects[0]).Name != "SendActionUsage" {
		t.Fatalf("transition effectAction = %v, want one SendActionUsage", effects)
	}
	if got, _ := m.ReflectiveElements(effects[0], "senderArgument"); len(got) != 1 {
		t.Errorf("effect send senderArgument = %v, want the alarmPort reference", got)
	}
	if params, _ := m.ReflectiveElements(effects[0], "parameter"); len(params) != 2 {
		t.Errorf("effect send parameter = %v, want payload and sender", params)
	}
}

// TestSendBodyParametersFollowItsArguments: a parameter a send's body declares follows
// the payload, sender and receiver among its parameters, inputs and directed features
// (SysML.xtext SendNode: the arguments, then the body).
func TestSendBodyParametersFollowItsArguments(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package S {
    item def Alarm;
    part def Monitor {
        part display;
        action supervise {
            action delayed send new Alarm() to display { in delay : Integer = 50; }
        }
    }
}
`)
	send := nested(t, sym(t, root, "S").Scope, "Monitor", "supervise", "delayed")
	delay := nested(t, send.Scope, "delay")
	if got := m.MetaclassOf(send).Name; got != "SendActionUsage" {
		t.Fatalf("delayed metaclass = %s, want SendActionUsage", got)
	}
	for _, feature := range []string{"parameter", "input", "directedFeature"} {
		got, _ := m.ReflectiveElements(send, feature)
		if len(got) != 4 || got[3] != delay {
			t.Fatalf("delayed %s = %v, want the payload, sender and receiver parameters then delay", feature, got)
		}
		for i := 0; i < 3; i++ {
			if name := m.MetaclassOf(got[i]).Name; name != "ReferenceUsage" {
				t.Errorf("delayed %s[%d] metaclass = %s, want ReferenceUsage", feature, i, name)
			}
		}
	}
	if got, _ := m.ReflectiveElements(send, "receiverArgument"); len(got) != 1 {
		t.Errorf("delayed receiverArgument = %v, want the display reference", got)
	}
}

package symbolfacts

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const sendModel = `
item def Alarm;
part def Monitor {
	port alarmPort;
	part display;
	action supervise {
		action notifyVia send new Alarm(3) via alarmPort;
		action notifyTo send new Alarm(5) to display;
		action both send new Alarm() via alarmPort to display;
		action noPayload send via alarmPort;
		action plain { first start; then done; }
	}
}
`

// TestSendActionFactsStateItsArguments: a send action's facts report the kind
// sendActionUsage and its payload, sender and receiver as written, each only
// when the send writes it (SysML v2 §8.3.17.15).
func TestSendActionFactsStateItsArguments(t *testing.T) {
	idx, sc := modelWithLibrary(t, sendModel)
	sc.Semantics.SetSourceText(source.TextOf(map[string]*source.SourceFile{
		"model.sysml": source.New("model.sysml", []byte(sendModel)),
	}, nil))
	cases := map[string]map[string]string{
		"Monitor::supervise::notifyVia": {"payload": "new Alarm(3)", "sender": "alarmPort"},
		"Monitor::supervise::notifyTo":  {"payload": "new Alarm(5)", "receiver": "display"},
		"Monitor::supervise::both":      {"payload": "new Alarm()", "sender": "alarmPort", "receiver": "display"},
		"Monitor::supervise::noPayload": {"sender": "alarmPort"},
	}
	for fqn, want := range cases {
		syms := idx.LookupQualified(fqn)
		if len(syms) != 1 {
			t.Fatalf("%s resolves to %v, want one symbol", fqn, syms)
		}
		info := Of(syms[0], sc)
		if info.Kind != "sendActionUsage" {
			t.Errorf("%s kind = %q, want sendActionUsage", fqn, info.Kind)
		}
		for _, key := range []string{"payload", "sender", "receiver"} {
			if got, ok := info.Metadata[key]; got != want[key] || ok != (want[key] != "") {
				t.Errorf("%s metadata[%s] = %q (present %t), want %q", fqn, key, got, ok, want[key])
			}
		}
	}
	plain := Of(idx.LookupQualified("Monitor::supervise::plain")[0], sc)
	if plain.Kind != "actionUsage" {
		t.Errorf("plain action kind = %q, want actionUsage", plain.Kind)
	}
	for _, key := range []string{"payload", "sender", "receiver"} {
		if got, ok := plain.Metadata[key]; ok {
			t.Errorf("plain action metadata[%s] = %q, want it absent", key, got)
		}
	}
}

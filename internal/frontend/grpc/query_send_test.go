package grpc

import (
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// A send action queries as a SendActionUsage whose payloadArgument, senderArgument
// and receiverArgument are its arguments as written, each present only when the
// send writes it (SysML v2 §8.3.17.15).
func TestQuerySendActionArguments(t *testing.T) {
	model := `package Demo {
		item def Alarm;
		part def Monitor {
			port alarmPort;
			part display;
			action supervise {
				action notifyVia send new Alarm(3) via alarmPort;
				action notifyTo send new Alarm(5) to display;
				action both send new Alarm() via alarmPort to display;
				send via alarmPort;
				action plain { first start; then done; }
			}
		}
	}`
	_, _, resp := runQueryOnSource(t, model, &pb.Query{
		Where:  primitive(QueryPropType, opEqual, false, "SendActionUsage"),
		Select: []string{QueryPropPayloadArgument, QueryPropSenderArgument, QueryPropReceiverArgument},
	})
	want := map[string]map[string]string{
		"Demo::Monitor::supervise::notifyVia": {QueryPropPayloadArgument: "new Alarm(3)", QueryPropSenderArgument: "alarmPort"},
		"Demo::Monitor::supervise::notifyTo":  {QueryPropPayloadArgument: "new Alarm(5)", QueryPropReceiverArgument: "display"},
		"Demo::Monitor::supervise::both":      {QueryPropPayloadArgument: "new Alarm()", QueryPropSenderArgument: "alarmPort", QueryPropReceiverArgument: "display"},
		"Demo::Monitor::supervise::@3":        {QueryPropSenderArgument: "alarmPort"},
	}
	if len(resp.Elements) != len(want) {
		t.Fatalf("SendActionUsage elements = %d, want %d: %v", len(resp.Elements), len(want), resp.Elements)
	}
	for _, el := range resp.Elements {
		expect, ok := want[el.Id]
		if !ok {
			t.Errorf("unexpected element %s", el.Id)
			continue
		}
		if el.Type != "SendActionUsage" {
			t.Errorf("%s @type = %q, want SendActionUsage", el.Id, el.Type)
		}
		for _, property := range []string{QueryPropPayloadArgument, QueryPropSenderArgument, QueryPropReceiverArgument} {
			got, present := el.Properties[property]
			if got != expect[property] || present != (expect[property] != "") {
				t.Errorf("%s %s = %q (present %t), want %q", el.Id, property, got, present, expect[property])
			}
		}
	}
}

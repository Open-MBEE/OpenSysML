package migrate_test

import (
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

func TestActionsWithNoV2FormKeepTheirPinsAndPlaceInTheFlow(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/no_v2_actions.xmi")
	if err != nil {
		t.Fatal(err)
	}
	r, err := migrate.Migrate("no_v2_actions.xmi", data)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	for _, line := range []string{
		"action acceptCall {",
		"out returnInformation : Payload",
		"out result : Payload[1];",
		"action reply {",
		"in returnInformation : Payload",
		"in replyValue : Payload",
		"action unmarshall {",
		"in object : Payload",
		"action broadcast {",
		"in argument : Payload",
		"first start then acceptCall;",
		"first acceptCall then reply;",
		"first reply then unmarshall;",
		"first unmarshall then broadcast;",
		"first broadcast then final;",
		"flow acceptCall.returnInformation to reply.returnInformation;",
	} {
		wantLine(t, r.Notation, line)
	}

	for _, action := range []struct {
		id, name, kind string
	}{
		{"_acceptCall", "acceptCall", "AcceptCallAction"},
		{"_reply", "reply", "ReplyAction"},
		{"_unmarshall", "unmarshall", "UnmarshallAction"},
		{"_broadcast", "broadcast", "BroadcastSignalAction"},
	} {
		wantLine(t, r.Notation, "not migrated: "+action.kind)
		wantNote(t, r, action.id, migrate.Unmapped, "no v2 form for a UML "+action.kind)
		if !strings.Contains(string(r.Notation), "action "+action.name+" {") {
			t.Errorf("%s is not an action usage:\n%s", action.kind, r.Notation)
		}
	}

	if diags := errors(t, "no_v2_actions.sysml", r.Notation); len(diags) != 0 {
		t.Errorf("migrated notation has validation errors: %v\n%s", diags, r.Notation)
	}
}

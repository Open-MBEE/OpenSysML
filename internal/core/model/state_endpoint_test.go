package model

import (
	"strings"
	"testing"
)

func TestInheritedStateActionEndpointsAreVertices(t *testing.T) {
	const uri = "file:///state-endpoints.sysml"
	ws := NewWorkspace()
	ws.Open(uri, []byte(`package P {
		attribute def Exit;
		state def S {
			state S1;
			accept Exit then done;
			entry;
			then S1;
			transition aTransition first start accept Exit then done;
			transition toStart first S1 then start;
			transition fromDone first done then S1;
		}
	}`), 1)
	defer ws.Close(uri)

	for _, diagnostic := range ws.Diagnostics(uri) {
		if strings.Contains(diagnostic.Message, "transition endpoint") {
			t.Errorf("inherited state endpoint rejected: %s", diagnostic.Message)
		}
	}
}

package model_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// A record keeps the features each end of a flow names, so an interface read
// from its record still pairs its ends' port features by the end each flow
// starts at, as the loaded document does.
func TestInterfaceRecordKeepsFlowPairing(t *testing.T) {
	docs := map[string][]byte{
		"defs.sysml": []byte(`package Defs {
	port def P { out item sent; in item received; }
	interface def Same { end x : P; end y : P; flow x.sent to x.received; }
	interface def Half { end x : P; end y : P; flow x.sent to y.received; }
	interface def Cross { end x : P; end y : P; flow x.sent to y.received; flow y.sent to x.received; }
	interface def Sub :> Cross { end :>> x; end :>> y; }
}`),
		"uses.sysml": []byte(`package Uses {
	private import Defs::*;
	part def H { port p : P; }
	part def Asm {
		part h1 : H;
		part h2 : H;
		interface same : Same connect h1.p to h2.p;
		interface half : Half connect h1.p to h2.p;
		interface cross : Cross connect h1.p to h2.p;
		interface sub : Sub connect h1.p to h2.p;
	}
}`),
	}
	ws := model.NewWorkspace()
	for name, content := range docs {
		ws.Open(name, content, 1)
	}
	for _, name := range []string{"defs.sysml", "uses.sysml"} {
		var warned []string
		for _, d := range ws.Diagnostics(name) {
			if d.Code == "port-conjugation" {
				warned = append(warned, d.Message)
			}
		}
		if len(warned) != 2 {
			t.Fatalf("%s: port-conjugation diagnostics = %v, want one for Same and one for Half", name, warned)
		}
	}
	if recorded := recordDifferential(t, docs); recorded != 2 {
		t.Fatalf("recorded %d documents, want both", recorded)
	}
}

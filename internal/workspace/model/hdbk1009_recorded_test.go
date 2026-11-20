package model

import (
	"strings"
	"testing"
)

// Links held as an interface record still answer the HDBK-1009A rules: the
// record keeps the dependencies, refinements and verifications its body states.
func TestWorkspaceHDBK1009ReadsRecordedRelationships(t *testing.T) {
	reqs := []byte(`package Reqs {
		private import HDBK1009::*;
		#stakeholderExpectationStatement requirement ses;
		#missionObjective requirement o;
		#systemRequirement requirement sys;
		#measureOfEffectiveness requirement moe;
		#verificationRequirement requirement ver { :>> verifyMethod = VerificationMethodKind::test; }
	}`)
	links := []byte(`package Links {
		private import HDBK1009::*;
		private import Reqs::*;
		dependency from o to ses;
		#derivation connection { end #original ::> o; end #derive ::> sys; }
		#refinement dependency from moe to o;
		#vvCase verification c { objective { verify sys; } }
		#refinement dependency from c to ver;
	}`)
	hdbk := func(ws *Workspace) map[string]int {
		out := map[string]int{}
		for code, n := range codesOf(ws.Diagnostics("reqs.sysml")) {
			if strings.HasPrefix(code, "hdbk1009-") {
				out[code] = n
			}
		}
		return out
	}

	alone := NewWorkspace()
	alone.Open("reqs.sysml", reqs, 1)
	if got := hdbk(alone); got["hdbk1009-measure-not-refining"] != 1 || got["hdbk1009-expectation-not-traced"] != 1 {
		t.Fatalf("reqs.sysml alone: %v, want the untraced objective and the unrefining measure", got)
	}

	loaded := NewWorkspace()
	loaded.OpenAll([]Input{{Name: "reqs.sysml", Content: reqs, Version: 1}, {Name: "links.sysml", Content: links, Version: 1}})
	loaded.DiagnosticsAll([]string{"reqs.sysml", "links.sysml"})
	rec, err := loaded.InterfaceRecord("links.sysml")
	if err != nil {
		t.Fatal(err)
	}
	g := rec.Scope.Gathered
	if g == nil || len(g.Dependencies) != 3 || len(g.Refinements) != 2 || len(g.Verifications) != 1 {
		t.Fatalf("the record keeps %+v, want 3 dependencies, 2 refinements and 1 verification", g)
	}

	ws := NewWorkspace()
	ws.Open("reqs.sysml", reqs, 1)
	if err := ws.OpenRecorded(rec, links); err != nil {
		t.Fatal(err)
	}
	if got := hdbk(ws); len(got) != 0 {
		t.Fatalf("reqs.sysml beside the recorded links: %v, want no finding", got)
	}
	if !ws.Recorded("links.sysml") {
		t.Fatal("links.sysml was hydrated: the fixture does not exercise recorded relationships")
	}
}

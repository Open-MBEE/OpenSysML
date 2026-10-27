package grpc

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/solve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// kermlUnstatedSource reads a KerML feature that declares no multiplicity,
// which the default `0..*` governs (KerML 1.1 §7.4.12), in an invariant.
const kermlUnstatedSource = `package Tanks {
	private import ScalarValues::Real;
	class Tank {
		feature level : Real;
		inv nonNegative { level >= 0.0 }
	}
}`

const assumedOneText = "Assuming Tanks::Tank::level holds one value: it declares no multiplicity, so the default [0..*] governs it"

// mustKerMLModel parses the KerML model inline and returns its hash.
func mustKerMLModel(t *testing.T, srv *Service) string {
	t.Helper()
	resp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:   &pb.ParseFileRequest_Content{Content: kermlUnstatedSource},
		Language: "kerml",
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(resp.Diagnostics) > 0 {
		t.Fatalf("fixture has diagnostics: %v", resp.Diagnostics)
	}
	return resp.ModelHash
}

// A solver's verdict carries the assumption the REPL prints, as one note naming
// the feature; the evaluator's verdict about the same constraint carries none.
func TestVerifyQuestionsCarryTheOneValueAssumption(t *testing.T) {
	requireSolver(t)
	srv := mustNewService(t, 10)
	hash := mustKerMLModel(t, srv)

	resp := verifyQuestion(t, srv, hash, "Tanks::Tank::nonNegative", "holds", "", "")
	if resp.Verdict.Status != statusViolated {
		t.Fatalf("status = %q, want violated: %q", resp.Verdict.Status, resp.Verdict.Error)
	}
	notes := resp.Verdict.Notes
	if len(notes) != 1 {
		t.Fatalf("notes = %v, want the one-value assumption alone", notes)
	}
	if notes[0].Kind != solve.NoteAssumedOne || notes[0].Subject != "Tanks::Tank::level" || notes[0].Text != assumedOneText {
		t.Errorf("note = %v, want kind %q about Tanks::Tank::level reading %q", notes[0], solve.NoteAssumedOne, assumedOneText)
	}

	evaluated, err := srv.VerifyConstraint(context.Background(), &pb.VerifyConstraintRequest{ModelHash: hash, SymbolId: "Tanks::Tank::nonNegative"})
	if err != nil {
		t.Fatalf("VerifyConstraint(evaluate): %v", err)
	}
	if len(evaluated.Verdict.Notes) != 0 {
		t.Errorf("an evaluation carries notes %v, want none", evaluated.Verdict.Notes)
	}
}

// The notes are the query's, in the order the REPL prints them: what the query
// leaves uncovered first, then what it assumes holds one, each worded once.
func TestVerdictNotesFollowTheQueryOrder(t *testing.T) {
	one := semantics.Bound{Value: 1, Known: true}
	q := &solve.Query{
		Uncovered:  []solve.Refused{{Path: "gear", Reason: "it may hold more than one object"}},
		AssumedOne: []solve.AssumedOne{{Name: "test::Craft::spare", Multiplicity: semantics.Range{Lower: semantics.Bound{Known: true}, Upper: semantics.Bound{Infinite: true, Known: true}}, Object: true}},
	}
	notes := verdictNotes([]*solve.Query{q, {AssumedOne: []solve.AssumedOne{{Name: "test::C::a", Multiplicity: semantics.Range{Lower: one, Upper: one}}}}})
	want := []*pb.VerdictNote{
		{Kind: solve.NoteUncovered, Subject: "gear", Text: "Not in the query: what gear asserts about its own objects, as it may hold more than one object; instantiate and validate an object to check it"},
		{Kind: solve.NoteAssumedOne, Subject: "test::Craft::spare", Text: "Assuming test::Craft::spare holds one object: it declares no multiplicity, so the default [0..*] governs it"},
		{Kind: solve.NoteAssumedOne, Subject: "test::C::a", Text: "Assuming test::C::a holds one value: it declares no multiplicity, so the default [1] governs it"},
	}
	if len(notes) != len(want) {
		t.Fatalf("notes = %v, want %v", notes, want)
	}
	for i, n := range notes {
		if n.Kind != want[i].Kind || n.Subject != want[i].Subject || n.Text != want[i].Text {
			t.Errorf("note %d = %v, want %v", i, n, want[i])
		}
	}
	if verdictNotes(nil) != nil {
		t.Errorf("no queries carry notes %v", verdictNotes(nil))
	}
}

package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

// TestGRPCRobustnessVerdictNotes: the notes field survives the shapes a mixed
// deployment produces — an old client decoding a new verdict, the JSON
// transport, a refused translation — without a fault or a changed answer.
func TestGRPCRobustnessVerdictNotes(t *testing.T) {
	requireSolver(t)
	srv := mustNewService(t, 10)
	hash := mustKerMLModel(t, srv)
	resp := verifyQuestion(t, srv, hash, "Tanks::Tank::nonNegative", "holds", "", "")
	if len(resp.Verdict.Notes) != 1 {
		t.Fatalf("notes = %v, want one", resp.Verdict.Notes)
	}

	t.Run("an old client reads a new verdict without its notes", func(t *testing.T) {
		old := dynamicpb.NewMessage(verdictWithoutNotes(t))
		wire, err := proto.Marshal(resp.Verdict)
		if err != nil {
			t.Fatal(err)
		}
		if err := (proto.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(wire, old); err != nil {
			t.Fatalf("an old client cannot decode the verdict: %v", err)
		}
		fields := old.Descriptor().Fields()
		if got := old.Get(fields.ByName("status")).String(); got != resp.Verdict.Status {
			t.Errorf("status read by an old client = %q, want %q", got, resp.Verdict.Status)
		}
		if got := len(old.GetUnknown()); got != 0 {
			t.Errorf("%d bytes of unknown fields kept, want the notes discarded", got)
		}
	})

	t.Run("JSON spells the notes in camelCase with every part", func(t *testing.T) {
		data, err := protojson.Marshal(resp.Verdict)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{`"notes":[{`, `"kind":"assumed_one"`, `"subject":"Tanks::Tank::level"`, `"text":"Assuming Tanks::Tank::level holds one value`} {
			if !strings.Contains(string(data), want) {
				t.Errorf("JSON lacks %s:\n%s", want, data)
			}
		}
		var back pb.Verdict
		if err := protojson.Unmarshal(data, &back); err != nil {
			t.Fatal(err)
		}
		if !proto.Equal(&back, resp.Verdict) {
			t.Errorf("JSON round trip = %v, want %v", &back, resp.Verdict)
		}
	})

	t.Run("a refused translation carries no notes", func(t *testing.T) {
		hash := mustVerifyModel(t, srv, `package P {
	private import ScalarValues::Real;
	part def Tank { attribute level : Real; assert constraint sq { level ** 2.0 > 0.0 } }
}`, "verdict-notes-refused")
		resp, err := srv.VerifyConstraint(context.Background(), &pb.VerifyConstraintRequest{ModelHash: hash, SymbolId: "P::Tank::sq", Question: "holds"})
		if err != nil {
			t.Fatalf("VerifyConstraint: %v", err)
		}
		if resp.Verdict.Status != statusUndecided || resp.Verdict.Error == "" {
			t.Fatalf("status = %q error = %q, want an undecided refusal", resp.Verdict.Status, resp.Verdict.Error)
		}
		if len(resp.Verdict.Notes) != 0 {
			t.Errorf("a refused translation carries notes %v", resp.Verdict.Notes)
		}
	})
}

// verdictWithoutNotes is the Verdict descriptor as a client predating the notes
// field compiled it: the same file with the field and its message removed.
func verdictWithoutNotes(t *testing.T) protoreflect.MessageDescriptor {
	t.Helper()
	file := protodesc.ToFileDescriptorProto(pb.File_sysml_proto)
	messages := file.MessageType[:0]
	for _, m := range file.MessageType {
		switch m.GetName() {
		case "VerdictNote":
			continue
		case "Verdict":
			fields := m.Field[:0]
			for _, f := range m.Field {
				if f.GetName() != "notes" {
					fields = append(fields, f)
				}
			}
			m.Field = fields
		}
		messages = append(messages, m)
	}
	file.MessageType = messages
	old, err := protodesc.NewFile(file, protoregistry.GlobalFiles)
	if err != nil {
		t.Fatalf("old descriptor: %v", err)
	}
	return old.Messages().ByName("Verdict")
}

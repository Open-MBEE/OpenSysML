package grpc

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// rangeModel spaces its references apart from what follows them, so a range
// that ran on to the next token would end past the name.
const rangeModel = `package P {
    part def A ;
    part def A ;
    part x : Strat ;
    part def D :> Missing  {
    }
    part q : A  :>  Nope ;
}
`

// typingModel draws a finding on a whole declaration whose terminator sits
// on the next line.
const typingModel = `package P {
    attribute def V ;
    part q : V
        ;
}
`

type wantRange struct {
	message                              string
	startLine, startCol, endLine, endCol int32
}

// A diagnostic's range covers its subject's text and ends at its last
// character, never after the whitespace following it. Each start is the
// line:column the pinned pilot validator reports for the same finding.
func TestDiagnosticRangesEndAtTheirSubject(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  []wantRange
	}{
		{"references and names", rangeModel, []wantRange{
			{"Duplicate of other owned member name", 2, 14, 2, 15},
			{"Duplicate of other owned member name", 3, 14, 3, 15},
			{"unresolved reference: Strat", 4, 14, 4, 19},
			{"unresolved reference: Missing", 5, 19, 5, 26},
			{"unresolved reference: Nope", 7, 21, 7, 25},
		}},
		{"declaration", typingModel, []wantRange{
			{"An occurrence, item or part must be typed by occurrence definitions.", 3, 5, 4, 10},
		}},
	}
	srv := mustNewService(t, 10)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
				Source: &pb.ParseFileRequest_Content{Content: tt.model},
			})
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			for _, w := range tt.want {
				if !hasRange(resp.Diagnostics, w) {
					t.Errorf("no diagnostic %q at %d:%d-%d:%d among %v", w.message,
						w.startLine, w.startCol, w.endLine, w.endCol, resp.Diagnostics)
				}
			}
		})
	}
}

func hasRange(diags []*pb.Diagnostic, w wantRange) bool {
	for _, d := range diags {
		sp := d.Span
		if sp == nil || len(d.Message) < len(w.message) || d.Message[:len(w.message)] != w.message {
			continue
		}
		if sp.StartLine == w.startLine && sp.StartCol == w.startCol && sp.EndLine == w.endLine && sp.EndCol == w.endCol {
			return true
		}
	}
	return false
}

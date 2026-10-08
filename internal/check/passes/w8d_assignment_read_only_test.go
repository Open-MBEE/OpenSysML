package passes

import (
	"strings"
	"testing"
)

func TestAssignmentToReadOnlyFeatureIsReported(t *testing.T) {
	src := `package Test {
	private import ScalarValues::*;
	part def Base {
		constant attribute k : Integer = 1;
		attribute raw : Integer default 2;
		derived attribute scaled : Integer = raw * 10;
	}
	part def Sub :> Base {
		attribute :>> k = 1;
	}
	part def Holder {
		constant attribute limit : Integer = 5;
		derived attribute twice : Integer = limit * 2;
		attribute count : Integer default 0;
		part base : Base;
		part sub : Sub;
		action def Write {
			assign limit := 7;
			assign twice := 7;
			assign count := 7;
			assign base.k := 2;
			assign base.scaled := 2;
			assign base.raw := 2;
			assign sub.k := 2;
		}
	}
}`
	type want struct {
		at, text string
	}
	wants := []want{
		{"limit := 7", "Assignment target limit is constant: its value does not change over the lifetime of its featuring occurrence."},
		{"twice := 7", "Assignment target twice is derived: its values are determined by the model, not written."},
		{"k := 2;\n\t\t\tassign base.scaled", "Assignment target base.k is constant"},
		{"scaled := 2", "Assignment target base.scaled is derived"},
		{"k := 2;\n\t\t}", "Assignment target sub.k is constant by Base::k, which it redefines or subsets"},
	}
	for _, warm := range []bool{false, true} {
		got := assignmentDiags(t, src, warm, "assignment-referent-read-only")
		if len(got) != len(wants) {
			t.Fatalf("warm=%v: got %v, want %d read-only diagnostics", warm, got, len(wants))
		}
		for i, w := range wants {
			if !strings.HasPrefix(got[i].Message, w.text) {
				t.Errorf("warm=%v diagnostic %d: message %q, want prefix %q", warm, i, got[i].Message, w.text)
			}
			if off := strings.Index(src, w.at); got[i].Span.Offset != off {
				t.Errorf("warm=%v diagnostic %d: offset %d, want %d (%q)", warm, i, got[i].Span.Offset, off, w.at)
			}
		}
		if others := assignmentReferentFindings(t, src, warm); len(others) != 0 {
			t.Errorf("warm=%v: a read-only target is also reported as not time varying: %v", warm, others)
		}
	}
}

// An end feature that may vary in time is implicitly constant (SysML 2.0
// §8.4.2.2), so it is refused as a read-only target like a declared constant —
// a declared end, and the ends a library connection or allocation types on.
func TestAssignmentToImplicitlyConstantEndIsReported(t *testing.T) {
	src := `package Test {
	part def Thing;
	part def Holder {
		end part e : Thing;
		part spare : Thing;
		action def Repoint { assign e := spare; }
	}
	part def Sys {
		part q1 : Thing;
		part q2 : Thing;
		connection c : Connections::BinaryConnection connect q1 to q2;
		allocation al : Allocations::Allocation allocate q1 to q2;
		action def Rewire {
			assign c.source := q2;
			assign c.target := q1;
			assign al.source := q2;
			assign al.target := q1;
		}
	}
}`
	type want struct {
		at, text string
	}
	wants := []want{
		{"e := spare", "Assignment target e is constant as an end feature"},
		{"source := q2;\n\t\t\tassign c.target", "Assignment target c.source is constant"},
		{"target := q1;\n\t\t\tassign al.source", "Assignment target c.target is constant"},
		{"source := q2;\n\t\t\tassign al.target", "Assignment target al.source is constant"},
		{"target := q1;\n\t\t}\n\t}\n}", "Assignment target al.target is constant"},
	}
	for _, warm := range []bool{false, true} {
		got := assignmentDiags(t, src, warm, "assignment-referent-read-only")
		if len(got) != len(wants) {
			t.Fatalf("warm=%v: got %v, want %d read-only diagnostics", warm, got, len(wants))
		}
		for i, w := range wants {
			if !strings.HasPrefix(got[i].Message, w.text) {
				t.Errorf("warm=%v diagnostic %d: message %q, want prefix %q", warm, i, got[i].Message, w.text)
			}
			if off := strings.Index(src, w.at); got[i].Span.Offset != off {
				t.Errorf("warm=%v diagnostic %d: offset %d, want %d (%q)", warm, i, got[i].Span.Offset, off, w.at)
			}
		}
		if others := assignmentReferentFindings(t, src, warm); len(others) != 0 {
			t.Errorf("warm=%v: a read-only target is also reported as not time varying: %v", warm, others)
		}
	}
}

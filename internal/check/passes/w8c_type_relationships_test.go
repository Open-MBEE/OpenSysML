package passes

import "testing"

// w8cMessages returns the messages of the diagnostics the constraint tier
// reports for src, parsed as KerML.
func w8cMessages(t *testing.T, src string) []string {
	t.Helper()
	var out []string
	for _, d := range constraintDiagsKerML(t, src) {
		out = append(out, d.Message)
	}
	return out
}

func w8cCount(msgs []string, want string) int {
	n := 0
	for _, m := range msgs {
		if m == want {
			n++
		}
	}
	return n
}

func TestW8CTypeRelationshipsNotOne(t *testing.T) {
	src := `package P {
	classifier A;
	classifier B;
	classifier C;
	classifier X unions A intersects B differences C;
}`
	msgs := w8cMessages(t, src)
	for _, want := range []string{msgOnlyOneUnioning, msgOnlyOneIntersecting, msgOnlyOneDifferencing} {
		if w8cCount(msgs, want) != 1 {
			t.Errorf("want one %q, got %v", want, msgs)
		}
	}
}

func TestW8CTypeRelationshipsNotSelf(t *testing.T) {
	src := `package P {
	classifier A;
	classifier B;
	classifier C;
	classifier Y unions A, Y intersects B, Y differences C, Y;
}`
	msgs := w8cMessages(t, src)
	for _, want := range []string{msgUnioningSelf, msgIntersectingSelf, msgDifferencingSelf} {
		if w8cCount(msgs, want) != 1 {
			t.Errorf("want one %q, got %v", want, msgs)
		}
	}
	// Two operands each: the not-one rule must stay silent.
	for _, unwanted := range []string{msgOnlyOneUnioning, msgOnlyOneIntersecting, msgOnlyOneDifferencing} {
		if w8cCount(msgs, unwanted) != 0 {
			t.Errorf("unexpected %q in %v", unwanted, msgs)
		}
	}
}

func TestW8CTypeRelationshipsLegal(t *testing.T) {
	src := `package P {
	classifier A;
	classifier B;
	classifier C;
	classifier D;
	classifier X unions A, B intersects C, D;
	feature f {
		feature g;
	}
	feature h chains f.g;
}`
	if msgs := w8cMessages(t, src); len(msgs) != 0 {
		t.Errorf("expected no constraint diagnostics, got %v", msgs)
	}
}

func TestW8CChainingFeatureNotOne(t *testing.T) {
	src := `package P {
	feature f;
	feature g chains f;
}`
	msgs := w8cMessages(t, src)
	if w8cCount(msgs, msgOnlyOneChaining) != 1 {
		t.Errorf("want one %q, got %v", msgOnlyOneChaining, msgs)
	}
}

func TestW8CChainingFeatureNotSelf(t *testing.T) {
	src := `package P {
	feature f {
		feature h chains f.h;
	}
}`
	msgs := w8cMessages(t, src)
	if w8cCount(msgs, msgChainingFeaturesSelf) != 1 {
		t.Errorf("want one %q, got %v", msgChainingFeaturesSelf, msgs)
	}
}

func TestW8CChainingSpanIsTheNamedFeature(t *testing.T) {
	src := "package P {\n\tfeature f;\n\tfeature g chains f;\n}"
	diags := constraintDiagsKerML(t, src)
	var found bool
	for _, d := range diags {
		if d.Message != msgOnlyOneChaining {
			continue
		}
		found = true
		got := src[d.Span.Offset:d.Span.End()]
		if got != "f" {
			t.Errorf("span covers %q, want %q", got, "f")
		}
	}
	if !found {
		t.Fatalf("rule did not fire: %v", diags)
	}
}

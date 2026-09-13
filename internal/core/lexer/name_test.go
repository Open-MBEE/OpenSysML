package lexer

import (
	"slices"
	"testing"
)

func TestNameTextQuotesWhatIsNoBasicName(t *testing.T) {
	for _, tc := range []struct {
		name string
		want string
	}{
		{"vehicle", "vehicle"},
		{"Vehicle_2", "Vehicle_2"},
		{"My Vehicle", "'My Vehicle'"},
		{"2wheels", "'2wheels'"},
		{"", "''"},
		// A name spelling a keyword is no basic name, so it keeps its quotes.
		{"frame", "'frame'"},
		{"state", "'state'"},
		{"render", "'render'"},
		// An escape the notation wrote stays as it is, rather than being
		// escaped a second time.
		{`it\'s`, `'it\'s'`},
	} {
		if got := NameText(tc.name); got != tc.want {
			t.Errorf("NameText(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestQualifiedNameTextQuotesEachSegmentOnItsOwn(t *testing.T) {
	for _, tc := range []struct {
		fqn  string
		want string
	}{
		{"", ""},
		{"Demo::Vehicle", "Demo::Vehicle"},
		{"Demo::My Vehicle", "Demo::'My Vehicle'"},
		{"state::frame::ok", "'state'::'frame'::ok"},
	} {
		if got := QualifiedNameText(tc.fqn); got != tc.want {
			t.Errorf("QualifiedNameText(%q) = %q, want %q", tc.fqn, got, tc.want)
		}
	}
}

func TestQualifiedNameOfQuotesEachNameOnItsOwn(t *testing.T) {
	for _, tc := range []struct {
		names []string
		want  string
	}{
		{nil, ""},
		{[]string{"x::y"}, "'x::y'"},
		{[]string{"x", "y"}, "x::y"},
		{[]string{"P", "x::y", "it\\'s"}, "P::'x::y'::'it\\'s'"},
	} {
		if got := QualifiedNameOf(tc.names); got != tc.want {
			t.Errorf("QualifiedNameOf(%q) = %q, want %q", tc.names, got, tc.want)
		}
	}
}

func TestQualifiedNameSegmentsReadsTheNotationBack(t *testing.T) {
	for _, tc := range []struct {
		text string
		want []string
	}{
		{"x", []string{"x"}},
		{"x::y", []string{"x", "y"}},
		{"'x::y'", []string{"x::y"}},
		{"P::'x::y'::'it\\'s'", []string{"P", "x::y", "it\\'s"}},
		{"'a b'::c", []string{"a b", "c"}},
	} {
		got, ok := QualifiedNameSegments(tc.text)
		if !ok || !slices.Equal(got, tc.want) {
			t.Errorf("QualifiedNameSegments(%q) = %q, %v, want %q", tc.text, got, ok, tc.want)
		}
		if back := QualifiedNameOf(got); back != tc.text {
			t.Errorf("QualifiedNameOf(QualifiedNameSegments(%q)) = %q", tc.text, back)
		}
	}
	for _, bad := range []string{"", "x::", "::y", "'x", "''", "a'b", "'x'y", "x::'y"} {
		if got, ok := QualifiedNameSegments(bad); ok {
			t.Errorf("QualifiedNameSegments(%q) = %q, want a refusal", bad, got)
		}
	}
}

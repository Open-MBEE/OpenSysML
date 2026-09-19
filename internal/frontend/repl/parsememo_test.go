package repl

import (
	"fmt"
	"testing"
)

// A repeated invocation is answered from the parse the first one made; an
// invocation with other arguments parses those, and both keep evaluating.
func TestCalcReusesParsedArgumentsAcrossInvocations(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%calc add 5 -3"), "✓ add(5, -3)", "= 2")
	first := s.argMemo.entries["5 -3"]
	if first.err != nil || len(first.exprs) != 2 {
		t.Fatalf("parsed %q = %v, %v; want two expressions", "5 -3", first.exprs, first.err)
	}
	wants(t, run(t, s, "%calc add 5 -3"), "✓ add(5, -3)", "= 2")
	if got := s.argMemo.entries["5 -3"]; got.exprs[0].expr != first.exprs[0].expr || got.exprs[1].expr != first.exprs[1].expr {
		t.Error("a repeated invocation parsed its arguments again")
	}
	if len(s.argMemo.entries) != 1 {
		t.Errorf("memo holds %d argument lists after one distinct invocation, want 1", len(s.argMemo.entries))
	}

	wants(t, run(t, s, "%calc add (2 + 3) -3"), "✓ add((2 + 3), -3)", "= 2")
	wants(t, run(t, s, "%calc add 5, -3"), "✓ add(5, -3)", "= 2")
	wants(t, run(t, s, "%calc add 10 20"), "✓ add(10, 20)", "= 30")
	if len(s.argMemo.entries) != 4 {
		t.Errorf("memo holds %d argument lists after four distinct invocations, want 4", len(s.argMemo.entries))
	}
	// The exact text keys the memo: a spelling that differs only in spacing is
	// its own entry, and both evaluate to what they say.
	wants(t, run(t, s, "%calc add 10  20"), "✓ add(10, 20)", "= 30")
	if len(s.argMemo.entries) != 5 {
		t.Errorf("memo holds %d argument lists, want 5: the exact text keys it", len(s.argMemo.entries))
	}
}

// The parse is reused but the evaluation is not: an argument naming a feature
// reads the value the current documents give it, including after a submission
// replaces the declaration it names.
func TestCalcEvaluatesReusedArgumentsAgainstCurrentState(t *testing.T) {
	s := NewSession()
	s.Submit(`package P {
		attribute k = 2;
		calc def dbl { in x; return : Integer = x * 2; }
	}`)
	wants(t, run(t, s, "%calc dbl k"), "✓ dbl(k)", "= 4")
	wants(t, run(t, s, "%calc dbl k + 1"), "✓ dbl(k + 1)", "= 6")
	before := s.argMemo.entries["k"]

	s.Submit(`package P {
		attribute k = 5;
		calc def dbl { in x; return : Integer = x * 2; }
	}`)
	wants(t, run(t, s, "%calc dbl k"), "✓ dbl(k)", "= 10")
	wants(t, run(t, s, "%calc dbl k + 1"), "✓ dbl(k + 1)", "= 12")
	if after := s.argMemo.entries["k"]; after.exprs[0].expr != before.exprs[0].expr {
		t.Error("a submission made the same argument text parse again")
	}

	// A submission may also change what the same text means: a name that was
	// an Integer now names a Real.
	s.Submit(`package P {
		attribute k : Real = 2.5;
		calc def dbl { in x; return : Real = x * 2; }
	}`)
	wants(t, run(t, s, "%calc dbl k"), "✓ dbl(k)", "= 5.0")

	// After a reset the text is evaluated against the new session, where the
	// name is gone until a submission declares it again.
	s.Clear()
	s.Submit(`package P {
		calc def dbl { in x; return : Integer = x * 2; }
	}`)
	wants(t, run(t, s, "%calc dbl k"), `evaluation of argument "k" failed`)
	s.Submit(`package P {
		attribute k = 7;
		calc def dbl { in x; return : Integer = x * 2; }
	}`)
	wants(t, run(t, s, "%calc dbl k"), "✓ dbl(k)", "= 14")
}

// A malformed argument list is reported the same way every time it is typed,
// and the report is made before anything is evaluated.
func TestCalcMalformedArgumentsReportedOnEveryInvocation(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	cases := []struct{ line, want string }{
		{"%calc add (5 3", `failed to parse argument "(5 3"`},
		{"%calc add a=1 2", "named arguments are not supported here; pass arguments positionally"},
		{"%calc add 5 - 3", `parameter "y" has no argument`},
	}
	for _, tc := range cases {
		first := run(t, s, tc.line)
		wants(t, first, tc.want)
		if again := run(t, s, tc.line); again != first {
			t.Errorf("%s reported differently the second time:\n%s\nfirst:\n%s", tc.line, again, first)
		}
	}
	// The verdict path reports the same error the prompt does.
	v := s.RunCalc("add(5 3")
	if v.Status != VerdictUnresolved {
		t.Fatalf("RunCalc(add(5 3) status = %v, want unresolved", v.Status)
	}
	wants(t, joinLines(v.Lines), `failed to parse argument "(5 3"`)
	if v2 := s.RunCalc("add(5 3"); joinLines(v2.Lines) != joinLines(v.Lines) {
		t.Errorf("RunCalc reported differently the second time:\n%s\nfirst:\n%s", joinLines(v2.Lines), joinLines(v.Lines))
	}
}

// The name a run command is given is read from the notation once per spelling;
// what it resolves to still follows the documents.
func TestLookupReusesParsedNameAcrossDocuments(t *testing.T) {
	s := NewSession()
	s.Submit("package 'My Pkg' { part def A; }")
	if _, fqn, err := s.lookupSymbol("'My Pkg'::A"); err != nil || fqn != "My Pkg::A" {
		t.Fatalf("lookup = %q, %v; want My Pkg::A", fqn, err)
	}
	if got, ok := s.nameMemo.entries["'My Pkg'::A"]; !ok || got.name != "My Pkg::A" || !got.ok {
		t.Fatalf("memo of %q = %+v, %v", "'My Pkg'::A", got, ok)
	}
	s.Clear()
	s.Submit("package 'My Pkg' { part def B; }")
	if _, fqn, err := s.lookupSymbol("'My Pkg'::A"); err == nil {
		t.Errorf("A still resolves as %q after a reset dropped it", fqn)
	}
	if _, fqn, err := s.lookupSymbol("'My Pkg'::B"); err != nil || fqn != "My Pkg::B" {
		t.Errorf("lookup B = %q, %v; want My Pkg::B", fqn, err)
	}
}

// The memo is bounded: past the limit it starts afresh rather than growing with
// every distinct argument list a long session types.
func TestParseMemoIsBounded(t *testing.T) {
	var m parseMemo[int]
	parses := 0
	parse := func(text string) int { parses++; return len(text) }
	for i := 0; i < parseMemoLimit; i++ {
		m.get(fmt.Sprintf("arg%d", i), parse)
	}
	if parses != parseMemoLimit || len(m.entries) != parseMemoLimit {
		t.Fatalf("after %d distinct texts: %d parses, %d entries", parseMemoLimit, parses, len(m.entries))
	}
	m.get("arg0", parse)
	if parses != parseMemoLimit {
		t.Errorf("a text already held was parsed again")
	}
	if got := m.get("one more", parse); got != len("one more") || parses != parseMemoLimit+1 {
		t.Errorf("get past the limit = %d, %d parses", got, parses)
	}
	if len(m.entries) != 1 {
		t.Errorf("memo holds %d entries past the limit, want 1", len(m.entries))
	}
}

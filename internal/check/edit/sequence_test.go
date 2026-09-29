package edit

import (
	"strings"
	"testing"
)

const sequenceTestModel = "package P {\n" +
	"    action def Heat;\n" +
	"    action def A {\n" +
	"        action a : Heat;\n" +
	"        action b {\n" +
	"            in x : ScalarValues::Real;\n" +
	"        }\n" +
	"    }\n" +
	"}\n"

func TestAddSequenceForms(t *testing.T) {
	tests := []struct {
		name string
		op   Operation
		want string
	}{
		{"first", AddFirst("P::A", "start"), "first start;"},
		{"then ref", AddThen("P::A", "done"), "then done;"},
		{"then member node", AddThen("P::A", "b"), "then b;"},
		{"then action", AddThenMember("P::A", "action", "g", "P::Heat"), "then action g : P::Heat;"},
		{"then action untyped", AddThenMember("P::A", "action", "g", ""), "then action g;"},
		{"then perform action", AddThenMember("P::A", "perform action", "g", "P::Heat"), "then perform action g : P::Heat;"},
		{"then state", AddThenMember("P::A", "state", "s", ""), "then state s;"},
		{"then merge", AddThenMember("P::A", "merge", "", ""), "then merge;"},
		{"then decide", AddThenMember("P::A", "decide", "", ""), "then decide;"},
		{"then join", AddThenMember("P::A", "join", "", ""), "then join;"},
		{"then fork", AddThenMember("P::A", "fork", "", ""), "then fork;"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := loadContent(t, "sequence.sysml", sequenceTestModel)
			requireClean(t, model)
			result := applyOne(t, model, test.op)
			if !strings.Contains(string(result.Content), "        "+test.want+"\n") {
				t.Fatalf("%q not appended:\n%s", test.want, result.Content)
			}
			requireClean(t, loadContent(t, "sequence.sysml", string(result.Content)))
		})
	}
}

func TestAddActionBodyStatements(t *testing.T) {
	plain := func(op Operation) Operation {
		op.SequenceKeyword = ""
		return op
	}
	tests := []struct {
		name string
		op   Operation
		want string
	}{
		{"accept", AddAccept("A", "payload", "ScalarValues::Integer", ""), "then accept payload : ScalarValues::Integer;"},
		{"plain accept", plain(AddAccept("A", "Integer", "", "")), "accept Integer;"},
		{"send", AddSend("A", "1", "self", "p"), "then send 1 via p to self;"},
		{"plain send", plain(AddSend("A", "1", "", "")), "send 1;"},
		{"assign", AddAssign("A", "x", "x + 1"), "then assign x := x + 1;"},
		{"plain assign", plain(AddAssign("A", "x", "x + 1")), "assign x := x + 1;"},
		{"if", AddIf("A", "x == 0", []Operation{
			{Kind: OpAddSequence, MemberKind: "assign", SequenceTarget: "x", SequenceValue: "1"},
		}, []Operation{
			{Kind: OpAddSequence, MemberKind: "assign", SequenceTarget: "x", SequenceValue: "2"},
		}), "then if x == 0 {\n        assign x := 1;\n    } else {\n        assign x := 2;\n    }"},
		{"plain if", plain(AddIf("A", "x == 0", nil, nil)), "if x == 0 { }"},
		{"while", AddWhile("A", "x < 2", []Operation{
			{Kind: OpAddSequence, MemberKind: "assign", SequenceTarget: "x", SequenceValue: "x + 1"},
		}, "x == 2"), "then while x < 2 {\n        assign x := x + 1;\n    } until x == 2;"},
		{"plain while", plain(AddWhile("A", "x < 2", nil, "")), "while x < 2 { }"},
		{"loop", AddLoop("A", []Operation{
			{Kind: OpAddSequence, MemberKind: "assign", SequenceTarget: "x", SequenceValue: "x + 1"},
		}, "x == 2"), "then loop {\n        assign x := x + 1;\n    } until x == 2;"},
		{"plain loop", plain(AddLoop("A", nil, "")), "loop { }"},
		{"for", AddFor("A", "i", "ScalarValues::Integer", "1..2", []Operation{
			{Kind: OpAddSequence, MemberKind: "assign", SequenceTarget: "x", SequenceValue: "x + i"},
		}), "then for i : ScalarValues::Integer in 1..2 {\n        assign x := x + i;\n    }"},
		{"plain for", plain(AddFor("A", "i", "", "1..2", nil)), "for i in 1..2 { }"},
		{"terminate", AddTerminate("A", "x"), "then terminate x;"},
		{"plain terminate", plain(AddTerminate("A", "")), "terminate;"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := loadContent(t, "action-body.sysml", "private import ScalarValues::*;\nport def PortDef;\naction def A {\n    action a;\n    port p : PortDef;\n    attribute x : ScalarValues::Integer = 0;\n}\n")
			requireClean(t, model)
			result := applyOne(t, model, test.op)
			if !strings.Contains(string(result.Content), test.want) {
				t.Fatalf("want %q in edited content:\n%s", test.want, result.Content)
			}
			requireClean(t, loadContent(t, "action-body.sysml", string(result.Content)))
		})
	}
}

func TestAddActionBodyStatementRefusals(t *testing.T) {
	base := loadContent(t, "action-body-refusals.sysml",
		"action def A { action a; attribute x : ScalarValues::Integer = 0; }\n")
	set := func(op Operation, update func(*Operation)) Operation {
		update(&op)
		return op
	}
	tests := []struct {
		name string
		m    Model
		op   Operation
		want Failure
	}{
		{"missing accept payload", base, AddAccept("A", "", "", ""), FailureIllegalKind},
		{"invalid accept type", base, set(AddAccept("A", "payload", "T", ""), func(op *Operation) {
			op.Type = "not a type"
		}), FailureInvalidName},
		{"invalid accept via expression", base, set(AddAccept("A", "payload", "", ""), func(op *Operation) {
			op.SequenceVia = "1 +"
		}), FailureInvalidValue},
		{"accept has unsupported target", base, set(AddAccept("A", "payload", "", ""), func(op *Operation) {
			op.SequenceTarget = "x"
		}), FailureIllegalKind},
		{"missing send payload", base, AddSend("A", "", "", ""), FailureIllegalKind},
		{"invalid send payload", base, AddSend("A", "1 +", "", ""), FailureInvalidValue},
		{"invalid send receiver", base, AddSend("A", "1", "1 +", ""), FailureInvalidValue},
		{"invalid send via", base, AddSend("A", "1", "", "1 +"), FailureInvalidValue},
		{"send has unsupported parameter", base, set(AddSend("A", "1", "", ""), func(op *Operation) {
			op.SequenceParameter = "payload"
		}), FailureIllegalKind},
		{"missing assignment fields", base, AddAssign("A", "", ""), FailureIllegalKind},
		{"invalid assignment target", base, AddAssign("A", "x +", "1"), FailureInvalidName},
		{"invalid assignment value", base, AddAssign("A", "x", "1 +"), FailureInvalidValue},
		{"assignment has unsupported via", base, set(AddAssign("A", "x", "1"), func(op *Operation) {
			op.SequenceVia = "port"
		}), FailureIllegalKind},
		{"missing if condition", base, AddIf("A", "", nil, nil), FailureIllegalKind},
		{"invalid if condition", base, AddIf("A", "1 +", nil, nil), FailureInvalidValue},
		{"if has unsupported type", base, set(AddIf("A", "true", nil, nil), func(op *Operation) {
			op.Type = "T"
		}), FailureIllegalKind},
		{"missing while condition", base, AddWhile("A", "", nil, ""), FailureIllegalKind},
		{"invalid while condition", base, AddWhile("A", "1 +", nil, ""), FailureInvalidValue},
		{"invalid while until", base, AddWhile("A", "true", nil, "1 +"), FailureInvalidValue},
		{"while has unsupported else body", base, set(AddWhile("A", "true", nil, ""), func(op *Operation) {
			op.SequenceElse = []Operation{{Kind: OpAddSequence, MemberKind: "assign", SequenceTarget: "x", SequenceValue: "1"}}
		}), FailureIllegalKind},
		{"loop has unsupported condition", base, set(AddLoop("A", nil, ""), func(op *Operation) {
			op.SequenceCondition = "true"
		}), FailureIllegalKind},
		{"invalid loop until", base, AddLoop("A", nil, "1 +"), FailureInvalidValue},
		{"loop has unsupported value", base, set(AddLoop("A", nil, ""), func(op *Operation) {
			op.SequenceValue = "1"
		}), FailureIllegalKind},
		{"missing for variable", base, AddFor("A", "", "", "1..2", nil), FailureIllegalKind},
		{"missing for collection", base, AddFor("A", "i", "", "", nil), FailureIllegalKind},
		{"invalid for variable", base, AddFor("A", "2i", "", "1..2", nil), FailureInvalidName},
		{"invalid for type", base, set(AddFor("A", "i", "T", "1..2", nil), func(op *Operation) {
			op.Type = "not a type"
		}), FailureInvalidName},
		{"invalid for collection", base, AddFor("A", "i", "", "1 +", nil), FailureInvalidValue},
		{"for has unsupported via", base, set(AddFor("A", "i", "", "1..2", nil), func(op *Operation) {
			op.SequenceVia = "port"
		}), FailureIllegalKind},
		{"invalid terminate occurrence", base, AddTerminate("A", "1 +"), FailureInvalidValue},
		{"terminate has unsupported body", base, set(AddTerminate("A", ""), func(op *Operation) {
			op.SequenceBody = []Operation{{Kind: OpAddSequence, MemberKind: "assign", SequenceTarget: "x", SequenceValue: "1"}}
		}), FailureIllegalKind},
		{"missing guarded succession guard", base, AddGuardedThen("A", "", "done"), FailureIllegalKind},
		{"invalid guarded succession guard", base, AddGuardedThen("A", "1 +", "done"), FailureInvalidValue},
		{"unknown guarded succession target", base, AddGuardedThen("A", "true", "missing"), FailureUnknownTarget},
		{"missing default succession target", base, AddElse("A", ""), FailureIllegalKind},
		{"unknown default succession target", base, AddElse("A", "missing"), FailureUnknownTarget},
		{"source multiplicity without then", base, set(plainSequenceStatement(AddAssign("A", "x", "1")), func(op *Operation) {
			op.Multiplicity = "[1]"
		}), FailureIllegalKind},
		{"nested body owner", base, AddIf("A", "true", []Operation{
			set(AddAssign("", "x", "1"), func(op *Operation) { op.Owner = "A" }),
		}, nil), FailureIllegalKind},
		{"nested body after", base, AddIf("A", "true", []Operation{
			set(AddAssign("", "x", "1"), func(op *Operation) { op.After = "a" }),
		}, nil), FailureIllegalKind},
		{"nested then without source", base, AddIf("A", "true", []Operation{
			AddThen("", "done"),
		}, nil), FailureIllegalKind},
		{"unknown after member", base, set(AddAssign("A", "x", "1"), func(op *Operation) {
			op.After = "missing"
		}), FailureUnknownTarget},
		{"action-body item on non-action owner", loadContent(t, "action-body-owner.sysml", "part def P;\n"), AddAssign("P", "x", "1"), FailureIllegalKind},
		{"unknown action owner", base, AddAssign("Missing", "x", "1"), FailureOwnerUnknown},
		{"then without source", loadContent(t, "action-body-empty.sysml", "action def Empty;\n"), AddIf("Empty", "true", nil, nil), FailureIllegalKind},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			addFailure(t, test.m, test.op, test.want)
		})
	}
}

func TestSourceMultiplicityRefusals(t *testing.T) {
	base := loadContent(t, "action-body-multiplicity-refusals.sysml",
		"action def A { action a; }\n")
	for _, test := range []struct {
		name, value string
	}{
		{"missing close after opener", "["},
		{"missing close after bound", "[1"},
		{"trailing text", "[1] x"},
		{"not bracketed", "1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			op := AddThen("A", "done")
			op.Multiplicity = test.value
			err := addFailure(t, base, op, FailureInvalidValue)
			if len(err.Diagnostics) == 0 {
				t.Fatal("parse refusal has no diagnostics")
			}
		})
	}
}

func plainSequenceStatement(op Operation) Operation {
	op.SequenceKeyword = ""
	return op
}

func TestAddGuardedAndDefaultSuccessions(t *testing.T) {
	tests := []struct {
		name string
		op   Operation
		want string
	}{
		{"guarded", AddGuardedThen("A", "ready", "done"), "if ready then done;"},
		{"default", AddElse("A", "done"), "else done;"},
		{"then source multiplicity", func() Operation {
			op := AddThen("A", "done")
			op.Multiplicity = "[0..1]"
			return op
		}(), "[0..1] then done;"},
		{"member source multiplicity", func() Operation {
			op := AddThenMember("A", "action", "next", "")
			op.Multiplicity = "[1]"
			return op
		}(), "then [1] action next [1];"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := loadContent(t, "action-succession.sysml", "action def A {\n    action a;\n}\n")
			requireClean(t, model)
			result := applyOne(t, model, test.op)
			if !strings.Contains(string(result.Content), test.want) {
				t.Fatalf("want %q in edited content:\n%s", test.want, result.Content)
			}
			requireClean(t, loadContent(t, "action-succession.sysml", string(result.Content)))
		})
	}
}

func TestAddSequenceAfter(t *testing.T) {
	t.Run("after a plain member", func(t *testing.T) {
		model := loadContent(t, "sequence-after.sysml", sequenceTestModel)
		requireClean(t, model)
		op := AddThen("P::A", "done")
		op.After = "a"
		result := applyOne(t, model, op)
		want := "        action a : Heat;\n        then done;\n        action b {\n"
		if !strings.Contains(string(result.Content), want) {
			t.Fatalf("then not inserted after a:\n%s", result.Content)
		}
		requireClean(t, loadContent(t, "sequence-after.sysml", string(result.Content)))
	})
	t.Run("after a member with a body", func(t *testing.T) {
		model := loadContent(t, "sequence-after-body.sysml", sequenceTestModel)
		requireClean(t, model)
		op := AddThen("P::A", "done")
		op.After = "b"
		result := applyOne(t, model, op)
		want := "            in x : ScalarValues::Real;\n        }\n        then done;\n    }\n"
		if !strings.Contains(string(result.Content), want) {
			t.Fatalf("then not inserted after b's body:\n%s", result.Content)
		}
		requireClean(t, loadContent(t, "sequence-after-body.sysml", string(result.Content)))
	})
	t.Run("before a same-line sibling", func(t *testing.T) {
		model := loadContent(t, "sequence-inline.sysml",
			"action def A { action a; action b; }\n")
		requireClean(t, model)
		op := AddThen("A", "done")
		op.After = "a"
		result := applyOne(t, model, op)
		const want = "action def A { action a; then done; action b; }\n"
		if string(result.Content) != want {
			t.Fatalf("content = %q, want %q", result.Content, want)
		}
		requireClean(t, loadContent(t, "sequence-inline.sysml", string(result.Content)))
	})
	t.Run("before a same-line closing brace", func(t *testing.T) {
		model := loadContent(t, "sequence-inline-brace.sysml",
			"action def A { action a; }\n")
		requireClean(t, model)
		op := AddThen("A", "done")
		op.After = "a"
		result := applyOne(t, model, op)
		const want = "action def A { action a; then done; }\n"
		if string(result.Content) != want {
			t.Fatalf("content = %q, want %q", result.Content, want)
		}
		requireClean(t, loadContent(t, "sequence-inline-brace.sysml", string(result.Content)))
	})
	t.Run("after a member with a trailing comment", func(t *testing.T) {
		model := loadContent(t, "sequence-comment.sysml",
			"action def A {\n    action a; // c\n    action b;\n}\n")
		requireClean(t, model)
		op := AddThen("A", "done")
		op.After = "a"
		result := applyOne(t, model, op)
		const want = "action def A {\n    action a; // c\n    then done;\n    action b;\n}\n"
		if string(result.Content) != want {
			t.Fatalf("content = %q, want %q", result.Content, want)
		}
		requireClean(t, loadContent(t, "sequence-comment.sysml", string(result.Content)))
	})
	t.Run("chain insertion", func(t *testing.T) {
		model := loadContent(t, "sequence-chain.sysml",
			"action def A {\n    action a;\n    action b;\n    then c;\n    action c;\n}\n")
		requireClean(t, model)
		op := AddThenMember("A", "action", "x", "")
		op.After = "b"
		result := applyOne(t, model, op)
		want := "    action b;\n    then action x;\n    then c;\n"
		if !strings.Contains(string(result.Content), want) {
			t.Fatalf("then not chained after b:\n%s", result.Content)
		}
		requireClean(t, loadContent(t, "sequence-chain.sysml", string(result.Content)))
	})
}

func TestAddSequenceRefusals(t *testing.T) {
	tests := []struct {
		name string
		m    Model
		op   Operation
		want Failure
	}{
		{
			name: "non-SysML source",
			m:    loadContent(t, "sequence.kerml", "package P;\n"),
			op:   AddFirst("P", "start"),
			want: FailureIllegalKind,
		},
		{
			name: "unknown owner",
			m:    loadContent(t, "sequence-owner.sysml", sequenceTestModel),
			op:   AddFirst("P::Missing", "start"),
			want: FailureOwnerUnknown,
		},
		{
			name: "part def owner",
			m:    loadContent(t, "sequence-owner.sysml", "part def P;\n"),
			op:   AddFirst("P", "start"),
			want: FailureIllegalKind,
		},
		{
			name: "state def owner",
			m:    loadContent(t, "sequence-owner.sysml", "state def S;\n"),
			op:   AddFirst("S", "start"),
			want: FailureIllegalKind,
		},
		{
			name: "calc def owner",
			m:    loadContent(t, "sequence-owner.sysml", "calc def C { in x : ScalarValues::Real; x }\n"),
			op:   AddFirst("C", "start"),
			want: FailureIllegalKind,
		},
		{
			name: "bad keyword",
			m:    loadContent(t, "sequence-kw.sysml", sequenceTestModel),
			op:   Operation{Kind: OpAddSequence, Owner: "P::A", SequenceKeyword: "next", SequenceRef: "a"},
			want: FailureIllegalKind,
		},
		{
			name: "first with a member",
			m:    loadContent(t, "sequence-kw.sysml", sequenceTestModel),
			op:   Operation{Kind: OpAddSequence, Owner: "P::A", SequenceKeyword: "first", MemberKind: "action", MemberName: "g"},
			want: FailureIllegalKind,
		},
		{
			name: "first without a ref",
			m:    loadContent(t, "sequence-kw.sysml", sequenceTestModel),
			op:   Operation{Kind: OpAddSequence, Owner: "P::A", SequenceKeyword: "first"},
			want: FailureIllegalKind,
		},
		{
			name: "then with both forms",
			m:    loadContent(t, "sequence-kw.sysml", sequenceTestModel),
			op:   Operation{Kind: OpAddSequence, Owner: "P::A", SequenceKeyword: "then", SequenceRef: "a", MemberKind: "action", MemberName: "g"},
			want: FailureIllegalKind,
		},
		{
			name: "then ref with a member name",
			m:    loadContent(t, "sequence-kw.sysml", sequenceTestModel),
			op:   Operation{Kind: OpAddSequence, Owner: "P::A", SequenceKeyword: "then", SequenceRef: "a", MemberName: "g"},
			want: FailureIllegalKind,
		},
		{
			name: "then ref with a type",
			m:    loadContent(t, "sequence-kw.sysml", sequenceTestModel),
			op:   Operation{Kind: OpAddSequence, Owner: "P::A", SequenceKeyword: "then", SequenceRef: "a", Type: "P::Heat"},
			want: FailureIllegalKind,
		},
		{
			name: "then with neither form",
			m:    loadContent(t, "sequence-kw.sysml", sequenceTestModel),
			op:   Operation{Kind: OpAddSequence, Owner: "P::A", SequenceKeyword: "then"},
			want: FailureIllegalKind,
		},
		{
			name: "ref not a feature reference",
			m:    loadContent(t, "sequence-ref.sysml", sequenceTestModel),
			op:   AddThen("P::A", "a b"),
			want: FailureInvalidName,
		},
		{
			name: "ref resolves to nothing",
			m:    loadContent(t, "sequence-ref.sysml", sequenceTestModel),
			op:   AddThen("P::A", "missing"),
			want: FailureUnknownTarget,
		},
		{
			name: "then with no preceding source",
			m:    loadContent(t, "sequence-empty.sysml", "action def A;\n"),
			op:   AddThen("A", "done"),
			want: FailureIllegalKind,
		},
		{
			name: "then after a parameter only",
			m:    loadContent(t, "sequence-param.sysml", "action def A { in x : ScalarValues::Real; }\n"),
			op:   AddThen("A", "done"),
			want: FailureIllegalKind,
		},
		{
			name: "bad member kind",
			m:    loadContent(t, "sequence-kind.sysml", sequenceTestModel),
			op:   AddThenMember("P::A", "part", "g", ""),
			want: FailureIllegalKind,
		},
		{
			name: "type on a control node",
			m:    loadContent(t, "sequence-kind.sysml", sequenceTestModel),
			op:   AddThenMember("P::A", "merge", "", "P::Heat"),
			want: FailureIllegalKind,
		},
		{
			name: "invalid member name",
			m:    loadContent(t, "sequence-name.sysml", sequenceTestModel),
			op:   AddThenMember("P::A", "action", "not a name", ""),
			want: FailureInvalidName,
		},
		{
			name: "member name taken",
			m:    loadContent(t, "sequence-name.sysml", sequenceTestModel),
			op:   AddThenMember("P::A", "action", "a", ""),
			want: FailureMemberNameTaken,
		},
		{
			name: "after names nothing",
			m:    loadContent(t, "sequence-after.sysml", sequenceTestModel),
			op: func() Operation {
				op := AddThen("P::A", "done")
				op.After = "missing"
				return op
			}(),
			want: FailureUnknownTarget,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			addFailure(t, test.m, test.op, test.want)
		})
	}
}

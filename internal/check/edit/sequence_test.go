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
	t.Run("before the next member's comment", func(t *testing.T) {
		model := loadContent(t, "sequence-comment-before.sysml",
			"action def A {\n    action a;\n    // explain b\n    action b;\n}\n")
		requireClean(t, model)
		op := AddThenMember("A", "action", "x", "")
		op.After = "a"
		result := applyOne(t, model, op)
		const want = "action def A {\n    action a;\n    then action x;\n    // explain b\n    action b;\n}\n"
		if string(result.Content) != want {
			t.Fatalf("content = %q, want %q", result.Content, want)
		}
		requireClean(t, loadContent(t, "sequence-comment-before.sysml", string(result.Content)))
	})
	t.Run("block comment that does not close on the line", func(t *testing.T) {
		model := loadContent(t, "sequence-block-open.sysml",
			"action def A {\n    action a; /* explanation\n continued */\n    action b;\n}\n")
		requireClean(t, model)
		op := AddThenMember("A", "action", "x", "")
		op.After = "a"
		result := applyOne(t, model, op)
		const want = "action def A {\n    action a; then action x; /* explanation\n continued */\n    action b;\n}\n"
		if string(result.Content) != want {
			t.Fatalf("content = %q, want %q", result.Content, want)
		}
		parsed := loadContent(t, "sequence-block-open.sysml", string(result.Content))
		requireClean(t, parsed)
		if syms := parsed.Index.LookupQualified("A::x"); len(syms) != 1 {
			t.Fatalf("A::x = %d symbols, want the then member to parse", len(syms))
		}
	})
	t.Run("block comment followed by code on the line", func(t *testing.T) {
		model := loadContent(t, "sequence-block-code.sysml",
			"action def A { action a; /* note */ action b; }\n")
		requireClean(t, model)
		op := AddThenMember("A", "action", "x", "")
		op.After = "a"
		result := applyOne(t, model, op)
		const want = "action def A { action a; then action x; /* note */ action b; }\n"
		if string(result.Content) != want {
			t.Fatalf("content = %q, want %q", result.Content, want)
		}
		parsed := loadContent(t, "sequence-block-code.sysml", string(result.Content))
		requireClean(t, parsed)
		if syms := parsed.Index.LookupQualified("A::x"); len(syms) != 1 {
			t.Fatalf("A::x = %d symbols, want the then member to parse", len(syms))
		}
	})
	t.Run("block comment closing on the anchor's line", func(t *testing.T) {
		model := loadContent(t, "sequence-block-close.sysml",
			"action def A {\n    action a; /* note */\n    action b;\n}\n")
		requireClean(t, model)
		op := AddThenMember("A", "action", "x", "")
		op.After = "a"
		result := applyOne(t, model, op)
		const want = "action def A {\n    action a; /* note */\n    then action x;\n    action b;\n}\n"
		if string(result.Content) != want {
			t.Fatalf("content = %q, want %q", result.Content, want)
		}
		parsed := loadContent(t, "sequence-block-close.sysml", string(result.Content))
		requireClean(t, parsed)
		if syms := parsed.Index.LookupQualified("A::x"); len(syms) != 1 {
			t.Fatalf("A::x = %d symbols, want the then member to parse", len(syms))
		}
	})
	t.Run("before a blank line", func(t *testing.T) {
		model := loadContent(t, "sequence-blank.sysml",
			"action def A {\n    action a;\n\n    action b;\n}\n")
		requireClean(t, model)
		op := AddThen("A", "done")
		op.After = "a"
		result := applyOne(t, model, op)
		const want = "action def A {\n    action a;\n    then done;\n\n    action b;\n}\n"
		if string(result.Content) != want {
			t.Fatalf("content = %q, want %q", result.Content, want)
		}
		requireClean(t, loadContent(t, "sequence-blank.sysml", string(result.Content)))
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

func TestAddSequenceGlobalReference(t *testing.T) {
	model := loadContent(t, "sequence-global.sysml",
		"package P {\n    action def A {\n        action a;\n    }\n}\n")
	requireClean(t, model)
	// `first $::x;` is not grammatical — the grammar admits a plain node name
	// after `first` — so only `then` shares this path.
	result := applyOne(t, model, AddThen("P::A", "$::P::A::a"))
	const want = "        then $::P::A::a;\n"
	if !strings.Contains(string(result.Content), want) {
		t.Fatalf("%q not written:\n%s", want, result.Content)
	}
	requireClean(t, loadContent(t, "sequence-global.sysml", string(result.Content)))
}

func TestAddSequenceGlobalReferenceAcrossDocuments(t *testing.T) {
	model := loadWorkspace(t, "a.sysml",
		"package P {\n    action def A {\n        action a;\n    }\n}\n",
		map[string]string{"q.sysml": "package Q {\n    action def B {\n        action q;\n    }\n}\n"})
	requireClean(t, model)
	scope := model.Index.LookupQualified("P::A")[0].Scope
	// A `$::` name resolves through the index, not the edited document alone:
	// a sibling document's member and a library member resolve, an unknown
	// name does not.
	if !model.sequenceNodeVisible(scope, "$::Q::B::q") {
		t.Fatal("$::Q::B::q did not resolve through the index")
	}
	if !model.sequenceNodeVisible(scope, "$::Actions::Action::done") {
		t.Fatal("$::Actions::Action::done did not resolve through the library")
	}
	if model.sequenceNodeVisible(scope, "$::Q::B::nope") {
		t.Fatal("$::Q::B::nope resolved; want it unresolvable")
	}
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
			name: "ref resolves to nothing",
			m:    loadContent(t, "sequence-ref.sysml", sequenceTestModel),
			op:   AddThen("P::A", "$::P::A::nope"),
			want: FailureUnknownTarget,
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

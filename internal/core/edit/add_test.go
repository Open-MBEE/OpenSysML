package edit

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func addFailure(t *testing.T, m Model, op Operation, want Failure) *Error {
	t.Helper()
	res, err := Apply(m, []Operation{op})
	if res != nil {
		t.Fatalf("refused operation returned content:\n%s", res.Content)
	}
	e := editError(t, err)
	if e.Failure != want {
		t.Fatalf("failure = %s (%s), want %s", e.Failure, e.Message, want)
	}
	return e
}

func TestAddMemberWritesOptionalNotation(t *testing.T) {
	m := loadContent(t, "add.sysml", "package P {\n    part def Base;\n}\n")
	res, err := Apply(m, []Operation{
		{
			Kind:         OpAddMember,
			Owner:        "P",
			MemberKind:   "attribute",
			MemberName:   "mass",
			Type:         "ISQ::MassValue",
			Multiplicity: "[1]",
			Value:        "2.0[SI::kg]",
		},
		{
			Kind:        OpAddMember,
			Owner:       "P",
			MemberKind:  "part def",
			MemberName:  "Car",
			Specializes: []string{"Base"},
		},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got := string(res.Content)
	for _, want := range []string{
		"attribute mass : ISQ::MassValue [1] = 2.0[SI::kg];",
		"part def Car specializes Base;",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("content does not contain %q:\n%s", want, got)
		}
	}
}

func TestAddMemberOptionalFieldCombinations(t *testing.T) {
	fields := []struct {
		name, typ, multiplicity, value string
	}{
		{"none", "", "", ""},
		{"type", "ISQ::MassValue", "", ""},
		{"multiplicity", "", "[1]", ""},
		{"value", "", "", "2.0"},
		{"type-and-multiplicity", "ISQ::MassValue", "[1]", ""},
		{"type-and-value", "ISQ::MassValue", "", "2.0"},
		{"multiplicity-and-value", "", "[1]", "2.0"},
		{"all", "ISQ::MassValue", "[1]", "2.0"},
	}
	for _, tc := range fields {
		t.Run(tc.name, func(t *testing.T) {
			m := loadContent(t, "combination.sysml", "package P;\n")
			op := AddMember("", "attribute", "a")
			op.Type, op.Multiplicity, op.Value = tc.typ, tc.multiplicity, tc.value
			if op.Type != "" && op.Value != "" {
				op.Value = "2.0[SI::kg]"
			}
			res, err := Apply(m, []Operation{op})
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if !strings.Contains(string(res.Content), "attribute a") {
				t.Fatalf("member was not emitted:\n%s", res.Content)
			}
		})
	}
	for _, tc := range []struct {
		name string
		refs []string
	}{
		{"one-specialization", []string{"Base"}},
		{"several-specializations", []string{"Base", "Other"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := loadContent(t, "specializes.sysml",
				"part def Base;\npart def Other;\n")
			op := AddMember("", "part def", "Child")
			op.Specializes = tc.refs
			res, err := Apply(m, []Operation{op})
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if !strings.Contains(string(res.Content),
				"part def Child specializes "+strings.Join(tc.refs, ", ")) {
				t.Fatalf("specialization was not emitted:\n%s", res.Content)
			}
		})
	}
}

func TestAddMemberValueValidation(t *testing.T) {
	m := loadContent(t, "add.sysml", "package P {\n    attribute seed = 2;\n}\n")
	if _, err := Apply(m, []Operation{{
		Kind: OpAddMember, Owner: "P", MemberKind: "attribute",
		MemberName: "copy", Value: "seed",
	}}); err != nil {
		t.Fatalf("a value resolving in the owner was refused: %v", err)
	}
	if e := addFailure(t, m, Operation{
		Kind: OpAddMember, Owner: "P", MemberKind: "attribute",
		MemberName: "broken", Value: "1 +",
	}, FailureInvalidValue); !strings.Contains(e.Message, "does not parse") {
		t.Fatalf("invalid-value message = %q", e.Message)
	}
	e := addFailure(t, m, Operation{
		Kind: OpAddMember, Owner: "P", MemberKind: "attribute",
		MemberName: "missing", Value: "notInScope",
	}, FailureResultInvalid)
	if !strings.Contains(e.Message, "notInScope") {
		t.Fatalf("result-invalid message = %q", e.Message)
	}
}

func TestAddMemberBatchIsAtomicWhenLaterOperationRefuses(t *testing.T) {
	src := "package P;\n"
	m := loadContent(t, "atomic.sysml", src)
	_, err := Apply(m, []Operation{
		AddMember("", "part def", "Added"),
		AddMember("", "class", "Illegal"),
	})
	if err == nil {
		t.Fatal("batch unexpectedly succeeded")
	}
	if got := string(m.Source.Bytes()); got != src {
		t.Fatalf("source changed after refusal: %q", got)
	}
}

func TestAddMemberRefusals(t *testing.T) {
	tests := []struct {
		name string
		m    Model
		op   Operation
		want Failure
		msg  string
	}{
		{
			name: "unknown owner",
			m:    loadContent(t, "add.sysml", "package P;\n"),
			op:   AddMember("Missing", "part def", "X"),
			want: FailureOwnerUnknown,
		},
		{
			name: "owner is not namespace",
			m:    loadContent(t, "add.sysml", "part def X;\nalias Y for X;\n"),
			op:   AddMember("Y", "part", "y"),
			want: FailureOwnerNotNamespace,
		},
		{
			name: "illegal SysML kind",
			m:    loadContent(t, "add.sysml", "package P;\n"),
			op:   AddMember("", "class", "C"),
			want: FailureIllegalKind,
			msg:  "sysml",
		},
		{
			name: "illegal KerML kind",
			m:    loadContent(t, "add.kerml", "package P;\n"),
			op:   AddMember("", "part", "p"),
			want: FailureIllegalKind,
			msg:  "kerml",
		},
		{
			name: "duplicate member",
			m:    loadContent(t, "add.sysml", "package P { part def X; }\n"),
			op:   AddMember("P", "part def", "X"),
			want: FailureMemberNameTaken,
		},
		{
			name: "invalid name",
			m:    loadContent(t, "add.sysml", "package P;\n"),
			op:   AddMember("", "part def", "2X"),
			want: FailureInvalidName,
		},
		{
			name: "typing definition",
			m:    loadContent(t, "add.sysml", "package P;\n"),
			op: func() Operation {
				op := AddMember("", "part def", "X")
				op.Type = "P"
				return op
			}(),
			want: FailureIllegalKind,
		},
		{
			name: "specializes usage",
			m:    loadContent(t, "add.sysml", "package P { part def Base; }\n"),
			op: func() Operation {
				op := AddMember("P", "part", "x")
				op.Specializes = []string{"Base"}
				return op
			}(),
			want: FailureIllegalKind,
		},
		{
			name: "unresolved type",
			m:    loadContent(t, "add.sysml", "package P;\n"),
			op: func() Operation {
				op := AddMember("", "part", "x")
				op.Type = "Missing"
				return op
			}(),
			want: FailureResultInvalid,
			msg:  "Missing",
		},
		{
			name: "unresolved specialization",
			m:    loadContent(t, "add.sysml", "package P;\n"),
			op: func() Operation {
				op := AddMember("", "part def", "X")
				op.Specializes = []string{"Missing"}
				return op
			}(),
			want: FailureResultInvalid,
			msg:  "Missing",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := addFailure(t, tc.m, tc.op, tc.want)
			if tc.msg != "" && !strings.Contains(strings.ToLower(e.Message), strings.ToLower(tc.msg)) {
				t.Fatalf("message %q does not mention %q", e.Message, tc.msg)
			}
		})
	}
}

func TestAddMemberDuplicateRootNamesRefuse(t *testing.T) {
	m := loadContent(t, "add.sysml", "package P;\n")
	_, err := Apply(m, []Operation{
		AddMember("", "part def", "Vehicle"),
		AddMember("", "part def", "Vehicle"),
	})
	e := editError(t, err)
	if e.Failure != FailureMemberNameTaken {
		t.Fatalf("failure = %s, want member-name-taken: %s", e.Failure, e.Message)
	}
	if !strings.Contains(e.Message, "Vehicle") {
		t.Fatalf("message %q does not mention duplicate name", e.Message)
	}
}

// memberKindTypes pairs each typed usage kind with a definition kind it may be
// typed by. SysML usages are typed by their own `def`.
var memberKindTypes = map[string]string{
	"feature": "class", "step": "behavior", "expr": "function", "bool": "predicate",
}

// TestEveryMemberKindWrites drives each registered kind through the parser and
// the analyzer: a definition or control node by name alone, a usage typed by
// its definition. Every offered kind must come out clean.
func TestEveryMemberKindWrites(t *testing.T) {
	for name, lang := range map[string]source.Kind{"kinds.sysml": source.KindSysML, "kinds.kerml": source.KindKerML} {
		kinds := MemberKinds(lang)
		if len(kinds) < 13 {
			t.Fatalf("%s offers only %v", name, kinds)
		}
		for _, kind := range kinds {
			t.Run(name+"/"+kind, func(t *testing.T) {
				src := "package P {\n    action def A {\n        action a0;\n    }\n}\n"
				if lang == source.KindKerML {
					src = "package P {\n    behavior A {\n        step s0;\n    }\n}\n"
				}
				op := AddMember("P", kind, "added")
				switch {
				case memberKinds[kind].typed:
					typeKind := memberKindTypes[kind]
					if typeKind == "" {
						typeKind = kind + " def"
					}
					src = strings.Replace(src, "package P {\n", "package P {\n    "+typeKind+" T;\n", 1)
					op.Type = "T"
				case !memberKinds[kind].definition && kind != "package":
					op.Owner = "P::A"
				}
				m := loadContent(t, name, src)
				requireClean(t, m)
				res, err := Apply(m, []Operation{op})
				if err != nil {
					t.Fatalf("Apply: %v", err)
				}
				got := string(res.Content)
				want := kind + " added"
				if op.Type != "" {
					want += " : T"
				}
				if !strings.Contains(got, want+";") {
					t.Fatalf("content lacks %q:\n%s", want, got)
				}
				requireClean(t, loadContent(t, name, got))
			})
		}
	}
}

// A connector definition declared by name alone has no ends, which the
// analyzer requires, so the kind is not offered and a request for it is
// refused as illegal rather than written and then refused as invalid.
func TestConnectorDefinitionsAreNotMemberKinds(t *testing.T) {
	for name, kinds := range map[string][]string{
		"kinds.sysml": {"connection def", "interface def", "flow def"},
		"kinds.kerml": {"assoc", "interaction"},
	} {
		m := loadContent(t, name, "package P {\n}\n")
		lang := m.Source.Kind()
		for _, kind := range kinds {
			for _, offered := range MemberKinds(lang) {
				if offered == kind {
					t.Fatalf("%s offers %q", name, kind)
				}
			}
			_, err := Apply(m, []Operation{AddMember("P", kind, "added")})
			if e := editError(t, err); e.Failure != FailureIllegalKind {
				t.Fatalf("%s %q: failure = %s (%s), want %s", name, kind, e.Failure, e.Message, FailureIllegalKind)
			}
		}
	}
}

func TestAddMemberKerMLNotation(t *testing.T) {
	m := loadContent(t, "add.kerml", "package P {\n\tclass Base;\n}\n")
	res, err := Apply(m, []Operation{
		{Kind: OpAddMember, Owner: "P", MemberKind: "class", MemberName: "Child", Specializes: []string{"Base"}},
		{Kind: OpAddMember, Owner: "P", MemberKind: "feature", MemberName: "f", Type: "Child"},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got := string(res.Content)
	for _, want := range []string{"class Child specializes Base;", "feature f : Child;"} {
		if !strings.Contains(got, want) {
			t.Fatalf("content does not contain %q:\n%s", want, got)
		}
	}
}

func TestAddMemberIndentationAndRootEOF(t *testing.T) {
	intoP := AddMember("P", "part def", "Added")
	atRoot := AddMember("", "part def", "Added")
	tests := []struct {
		name, src, want string
		op              Operation
	}{
		{
			name: "four spaces and indented closing brace",
			src:  "package P {\n    part def Existing;\n    }\n",
			want: "package P {\n    part def Existing;\n    part def Added;\n    }\n",
			op:   intoP,
		},
		{
			name: "tabs and indented closing brace",
			src:  "package P {\n\tpart def Existing;\n\t}\n",
			want: "package P {\n\tpart def Existing;\n\tpart def Added;\n\t}\n",
			op:   intoP,
		},
		{
			name: "tabs and existing body",
			src:  "package P {\n\tpart def Existing;\n}\n",
			want: "package P {\n\tpart def Existing;\n\tpart def Added;\n}\n",
			op:   intoP,
		},
		{
			name: "two spaces",
			src:  "package P {\n  part def Existing;\n}\n",
			want: "package P {\n  part def Existing;\n  part def Added;\n}\n",
			op:   intoP,
		},
		{
			name: "empty body",
			src:  "package P {\n}\n",
			want: "package P {\n    part def Added;\n}\n",
			op:   intoP,
		},
		{
			name: "bodyless owner",
			src:  "part def P;\n",
			want: "part def P {\n    part x;\n}\n",
			op:   AddMember("P", "part", "x"),
		},
		{
			name: "bodyless typed usage owner",
			src:  "part def Tank;\npart def Car {\n    part tank : Tank;\n}\n",
			want: "part def Tank;\npart def Car {\n    part tank : Tank {\n        port p;\n    }\n}\n",
			op:   AddMember("Car::tank", "port", "p"),
		},
		{
			name: "bodyless valued usage owner",
			src:  "part def Car {\n    attribute mass = 1200;\n}\n",
			want: "part def Car {\n    attribute mass = 1200 {\n        attribute unit;\n    }\n}\n",
			op:   AddMember("Car::mass", "attribute", "unit"),
		},
		{
			name: "root with newline",
			src:  "package P;\n",
			want: "package P;\npart def Added;\n",
			op:   atRoot,
		},
		{
			name: "root without newline",
			src:  "package P;",
			want: "package P;\npart def Added;\n",
			op:   atRoot,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := loadContent(t, "add.sysml", tc.src)
			res, err := Apply(m, []Operation{tc.op})
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if got := string(res.Content); got != tc.want {
				t.Fatalf("content = %q, want %q", got, tc.want)
			}
		})
	}
}

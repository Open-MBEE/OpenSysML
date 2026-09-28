package edit

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const verificationAuthoringModel = `package ToasterDemo {
    private import ISQ::*;
    private import SI::*;
    private import VerificationCases::*;
    part def Toaster { attribute cycleTime : ISQ::DurationValue; }
    requirement def TimelyToast {
        subject toaster : Toaster;
        require constraint { toaster.cycleTime <= 180.0 [SI::s] }
    }
    requirement timely : TimelyToast;
    verification def TimelyToastTest {
        subject toaster : Toaster;
    }
}
`

func TestAddVerifyCreatesObjectiveAndWritesExpectedModel(t *testing.T) {
	m := loadContent(t, "verification.sysml", verificationAuthoringModel)
	requireClean(t, m)
	res, err := Apply(m, []Operation{
		AddVerify("ToasterDemo::TimelyToastTest", "timely"),
		AddMetadata("ToasterDemo::TimelyToastTest", "VerificationMethod", "", nil,
			[]MetadataValue{{Feature: "kind", Value: "VerificationMethodKind::test"}}, false),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := strings.Replace(verificationAuthoringModel,
		"        subject toaster : Toaster;\n    }\n",
		"        subject toaster : Toaster;\n        objective {\n            verify timely;\n        }\n        metadata VerificationMethod { kind = VerificationMethodKind::test; }\n    }\n", 1)
	if got := string(res.Content); got != want {
		t.Fatalf("content mismatch:\n%s\nwant:\n%s", got, want)
	}
	requireClean(t, loadContent(t, "verification.sysml", string(res.Content)))
}

func TestAddVerifyUsesNamedObjectiveAndCreatesItsBody(t *testing.T) {
	const src = `package P {
    private import VerificationCases::*;
    requirement def R;
    requirement r : R;
    verification def V {
        objective check;
    }
}
`
	m := loadContent(t, "named-objective.sysml", src)
	requireClean(t, m)
	res, err := Apply(m, []Operation{AddVerify("P::V::check", "P::r")})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got, want := string(res.Content), strings.Replace(src, "objective check;", "objective check {\n            verify P::r;\n        }", 1); got != want {
		t.Fatalf("content mismatch:\n%s\nwant:\n%s", got, want)
	}
	requireClean(t, loadContent(t, "named-objective.sysml", string(res.Content)))
}

func TestAddVerifyUsesExistingObjectiveAndVerificationUsage(t *testing.T) {
	const src = `package P {
    private import VerificationCases::*;
    requirement def R;
    requirement r : R;
    verification def V {
        objective {
        }
    }
    verification v : VerificationCase;
}
`
	m := loadContent(t, "existing-objective.sysml", src)
	requireClean(t, m)
	res, err := Apply(m, []Operation{AddVerify("P::V", "P::r")})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !strings.Contains(string(res.Content), "objective {\n            verify P::r;\n        }") {
		t.Fatalf("existing objective was not edited:\n%s", res.Content)
	}
	m = loadContent(t, "verification-usage.sysml", src)
	res, err = Apply(m, []Operation{AddVerify("P::v", "P::r")})
	if err != nil {
		t.Fatalf("Apply to verification usage: %v", err)
	}
	if !strings.Contains(string(res.Content),
		"verification v : VerificationCase {\n        objective {\n            verify P::r;\n        }") {
		t.Fatalf("verification usage did not gain an objective:\n%s", res.Content)
	}
	requireClean(t, loadContent(t, "verification-usage.sysml", string(res.Content)))
}

func TestAddVerifyExpandsAnAnonymousObjective(t *testing.T) {
	const src = `package P {
    private import VerificationCases::*;
    requirement def R;
    requirement r : R;
    verification def V {
        objective;
    }
}
`
	m := loadContent(t, "bodyless-objective.sysml", src)
	requireClean(t, m)
	res, err := Apply(m, []Operation{AddVerify("P::V", "P::r")})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := strings.Replace(src, "        objective;", "        objective {\n            verify P::r;\n        }", 1)
	if got := string(res.Content); got != want {
		t.Fatalf("content mismatch:\n%s\nwant:\n%s", got, want)
	}
	requireClean(t, loadContent(t, "bodyless-objective.sysml", string(res.Content)))
}

func TestAddVerifyRefusals(t *testing.T) {
	tests := []struct {
		name string
		src  string
		op   Operation
		want Failure
	}{
		{
			name: "KerML", src: "package P;\n", op: AddVerify("", "R"),
			want: FailureIllegalKind,
		},
		{
			name: "non verification owner",
			src:  "package P { part def X; }\n",
			op:   AddVerify("P::X", "R"), want: FailureIllegalKind,
		},
		{
			name: "analysis objective",
			src: `package P {
    analysis def A { objective { } }
}
`,
			op: AddVerify("P::A", "R"), want: FailureIllegalKind,
		},
		{
			name: "malformed requirement",
			src:  "package P { verification def V; }\n",
			op:   AddVerify("P::V", "R bad"), want: FailureInvalidName,
		},
		{
			name: "unresolved requirement",
			src: `package P {
    private import VerificationCases::*;
    verification def V;
}
`,
			op: AddVerify("P::V", "missing"), want: FailureResultInvalid,
		},
		{
			name: "part target",
			src: `package P {
    private import VerificationCases::*;
    part def T;
    part t : T;
    verification def V;
}
`,
			op: AddVerify("P::V", "P::t"), want: FailureResultInvalid,
		},
		{
			name: "requirement definition target",
			src: `package P {
    private import VerificationCases::*;
    requirement def R;
    verification def V;
}
`,
			op: AddVerify("P::V", "P::R"), want: FailureResultInvalid,
		},
		{
			name: "multiple direct objectives",
			src: `package P {
    private import VerificationCases::*;
    verification def V { objective first { } objective second { } }
}
`,
			op: AddVerify("P::V", "P::r"), want: FailureIllegalKind,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := loadContent(t, "verify-refusal.sysml", tc.src)
			addFailure(t, m, tc.op, tc.want)
		})
	}
}

func TestAddVerifyRefusesInheritedUserObjective(t *testing.T) {
	const src = `package P {
    private import VerificationCases::*;
    requirement def R;
    requirement r : R;
    verification def Base { objective inherited { verify r; } }
    verification def Child specializes Base;
}
`
	m := loadContent(t, "verify-inherited.sysml", src)
	addFailure(t, m, AddVerify("P::Child", "P::r"), FailureIllegalKind)
}

func TestAddAnonymousObjectiveAndRejectSecondObjectiveByAnalysis(t *testing.T) {
	const src = `package P {
    private import VerificationCases::*;
    verification def V;
}
`
	m := loadContent(t, "anonymous-objective.sysml", src)
	res, err := Apply(m, []Operation{AddMember("P::V", "objective", "")})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !strings.Contains(string(res.Content), "objective;") {
		t.Fatalf("anonymous objective was not emitted:\n%s", res.Content)
	}
	addFailure(t, loadContent(t, "second-objective.sysml",
		"package P { private import VerificationCases::*; verification def V { objective { } } }\n"),
		AddMember("P::V", "objective", ""), FailureResultInvalid)
	typed := AddMember("P::V", "objective", "")
	typed.Type = "ObjectiveType"
	m = loadContent(t, "typed-anonymous-objective.sysml",
		"package P { private import VerificationCases::*; objective def ObjectiveType; verification def V; }\n")
	res, err = Apply(m, []Operation{typed})
	if err != nil {
		t.Fatalf("Apply typed anonymous objective: %v", err)
	}
	if !strings.Contains(string(res.Content), "objective : ObjectiveType;") {
		t.Fatalf("typed anonymous objective was not emitted:\n%s", res.Content)
	}
}

func TestAddMetadataWritesLongAndShorthandForms(t *testing.T) {
	const src = `package P {
    private import VerificationCases::*;
    part def Toaster;
    verification def V;
}
`
	m := loadContent(t, "metadata.sysml", src)
	res, err := Apply(m, []Operation{
		AddMetadata("P::V", "VerificationMethod", "vm", []string{"P::Toaster", "P::V"},
			[]MetadataValue{{Feature: "kind", Value: "VerificationMethodKind::test"}}, false),
		AddMetadata("P::V", "VerificationMethod", "", nil,
			[]MetadataValue{{Feature: "kind", Value: "VerificationMethodKind::test"}}, true),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got := string(res.Content)
	for _, want := range []string{
		"metadata vm : VerificationMethod about P::Toaster, P::V { kind = VerificationMethodKind::test; }",
		"@VerificationMethod { kind = VerificationMethodKind::test; }",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("content lacks %q:\n%s", want, got)
		}
	}
	requireClean(t, loadContent(t, "metadata.sysml", got))
}

func TestAddMetadataWritesWithoutValuesAndAtRoot(t *testing.T) {
	m := loadContent(t, "metadata-root.sysml", "metadata def Safety;\n")
	res, err := Apply(m, []Operation{AddMetadata("", "Safety", "", nil, nil, false)})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got, want := string(res.Content), "metadata def Safety;\nmetadata Safety;\n"; got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
	requireClean(t, loadContent(t, "metadata-root.sysml", string(res.Content)))
}

func TestAddMetadataToPackage(t *testing.T) {
	const src = "metadata def Safety;\npackage P;\n"
	m := loadContent(t, "metadata-package.sysml", src)
	res, err := Apply(m, []Operation{AddMetadata("P", "Safety", "", nil, nil, false)})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := "metadata def Safety;\npackage P {\n    metadata Safety;\n}\n"
	if got := string(res.Content); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
	requireClean(t, loadContent(t, "metadata-package.sysml", string(res.Content)))
}

func TestAddMetadataWorksInKerML(t *testing.T) {
	const src = `metaclass M { feature f : ScalarValues::Integer; }
class C;
`
	m := loadContent(t, "metadata.kerml", src)
	res, err := Apply(m, []Operation{AddMetadata("C", "M", "", nil,
		[]MetadataValue{{Feature: "f", Value: "1"}}, true)})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !strings.Contains(string(res.Content), "@M { f = 1; }") {
		t.Fatalf("KerML metadata was not emitted:\n%s", res.Content)
	}
	requireClean(t, loadContent(t, "metadata.kerml", string(res.Content)))
}

func TestAddMetadataRefusals(t *testing.T) {
	const metadataModel = `package P {
    private import VerificationCases::*;
    metadata def Safety;
    part def T;
    verification def V;
}
`
	tests := []struct {
		name string
		op   Operation
		want Failure
	}{
		{"empty type", AddMetadata("P::V", "", "", nil, nil, false), FailureInvalidName},
		{"malformed type", AddMetadata("P::V", "bad ref", "", nil, nil, false), FailureInvalidName},
		{"malformed name", AddMetadata("P::V", "Safety", "bad name", nil, nil, false), FailureInvalidName},
		{"name taken", AddMetadata("P::V", "Safety", "existing", nil, nil, false), FailureMemberNameTaken},
		{"malformed about", AddMetadata("P::V", "Safety", "", []string{"bad ref"}, nil, false), FailureInvalidName},
		{"malformed feature", AddMetadata("P::V", "Safety", "", nil, []MetadataValue{{Feature: "bad ref", Value: "1"}}, false), FailureInvalidName},
		{"empty value", AddMetadata("P::V", "Safety", "", nil, []MetadataValue{{Feature: "f", Value: ""}}, false), FailureInvalidValue},
		{"invalid expression", AddMetadata("P::V", "Safety", "", nil, []MetadataValue{{Feature: "f", Value: "1 +"}}, false), FailureInvalidValue},
		{"duplicate feature", AddMetadata("P::V", "Safety", "", nil, []MetadataValue{{Feature: "f", Value: "1"}, {Feature: "f", Value: "2"}}, false), FailureInvalidValue},
		{"unresolved feature", AddMetadata("P::V", "Safety", "", nil, []MetadataValue{{Feature: "missing", Value: "1"}}, false), FailureResultInvalid},
		{"non metadata type", AddMetadata("P::V", "P::T", "", nil, nil, false), FailureResultInvalid},
		{"unknown type", AddMetadata("P::V", "Missing", "", nil, nil, false), FailureResultInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := metadataModel
			if tc.name == "name taken" {
				src = strings.Replace(src, "verification def V;", "verification def V { metadata existing : Safety; }", 1)
			}
			addFailure(t, loadContent(t, "metadata-refusal.sysml", src), tc.op, tc.want)
		})
	}
}

func TestAddMetadataTypeRefusesFeaturePathAndPrefixReturn(t *testing.T) {
	m := loadContent(t, "metadata-type-path.sysml", "metadata def M;\npart def P;\n")
	addFailure(t, m, AddMetadata("", "P.M", "", nil, nil, false), FailureInvalidName)
	ret := AddMember("C", "return", "x")
	ret.MetadataPrefixes = []string{"M"}
	addFailure(t, loadContent(t, "prefix-return.sysml", "calc def C;\nmetadata def M;\n"), ret, FailureIllegalKind)
	prefix := AddMember("", "part def", "X")
	prefix.MetadataPrefixes = []string{"bad ref"}
	addFailure(t, loadContent(t, "prefix-invalid.sysml", "metadata def M;\n"), prefix, FailureInvalidName)
}

func TestAddMemberMetadataPrefixPlacementForEveryMemberKind(t *testing.T) {
	for _, language := range []source.Kind{source.KindSysML, source.KindKerML} {
		for _, kindName := range MemberKinds(language) {
			t.Run(language.String()+"/"+kindName, func(t *testing.T) {
				op := AddMember("", kindName, "x")
				op.MetadataPrefixes = []string{"Safety"}
				if kindName == "return" {
					addFailure(t, loadContent(t, "prefix-return.sysml",
						"metadata def Safety;\ncalc def C;\n"), op, FailureIllegalKind)
					return
				}
				if kindName == "ref" {
					op.Type = "Q"
				}
				text := writeMember(op, memberKinds[kindName])
				extension := kindName == "ref" || kindName == "individual" ||
					kindName == "individual def" || kindName == "subject" ||
					kindName == "actor" || kindName == "stakeholder" ||
					kindName == "objective"
				var want string
				if extension {
					if kindName == "individual def" {
						want = "individual #Safety def x;"
					} else if kindName == "ref" {
						want = "ref #Safety x : Q;"
					} else {
						want = kindName + " #Safety x;"
					}
				} else {
					want = "#Safety " + kindName + " x;"
				}
				if text != want {
					t.Fatalf("writeMember = %q, want %q", text, want)
				}
				var content string
				switch parser.MemberOwner(kindName) {
				case "requirement or case", "requirement":
					content = "requirement def R { " + text + " }\n"
				case "case":
					content = "verification def V { " + text + " }\n"
				default:
					switch kindName {
					case "decide", "fork", "join", "merge", "perform", "perform action":
						content = "action def A { " + text + " }\n"
					default:
						content = "package P { " + text + " }\n"
					}
				}
				if language == source.KindSysML {
					content = "metadata def Safety;\n" + content
				}
				sf := source.NewWithKind("prefix."+language.String(), []byte(content), language)
				p := parser.New(sf)
				root := p.ParseFile()
				if len(p.Diagnostics) != 0 {
					t.Fatalf("prefixed member does not reparse: %v\n%s", p.Diagnostics, content)
				}
				if got := strings.Count(ast.Dump(root), `(PrefixMetadata type="Safety"`); got != 1 {
					t.Fatalf("AST records %d prefixes on the parsed declaration, want one:\n%s",
						got, ast.Dump(root))
				}
			})
		}
	}
}

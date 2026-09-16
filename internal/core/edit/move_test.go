package edit

import (
	"strings"
	"testing"
)

// moveOne applies a single move to src and checks the result parses and
// analyzes clean; it returns the new source.
func moveOne(t *testing.T, name, src, target, owner string) string {
	t.Helper()
	m := loadContent(t, name, src)
	requireClean(t, m)
	res := applyOne(t, m, Move(target, owner))
	got := string(res.Content)
	requireClean(t, loadContent(t, name, got))
	return got
}

func TestMoveIntoBodyCarriesBodyAndComments(t *testing.T) {
	src := "package P {\n" +
		"    part def A;\n" +
		"\n" +
		"    // the moved part\n" +
		"    part def B {\n" +
		"        attribute x : ScalarValues::Integer = 1;\n" +
		"    }\n" +
		"\n" +
		"    part def C {\n" +
		"        part def D;\n" +
		"    }\n" +
		"}\n"
	got := moveOne(t, "move.sysml", src, "P::B", "P::C")
	want := "package P {\n" +
		"    part def A;\n" +
		"\n" +
		"    part def C {\n" +
		"        part def D;\n" +
		"        // the moved part\n" +
		"        part def B {\n" +
		"            attribute x : ScalarValues::Integer = 1;\n" +
		"        }\n" +
		"    }\n" +
		"}\n"
	if got != want {
		t.Fatalf("moved source:\n%s\nwant:\n%s", got, want)
	}
}

func TestMoveOpensBodyOfBodylessOwner(t *testing.T) {
	src := "package P {\n" +
		"    part def A;\n" +
		"    part def B;\n" +
		"}\n"
	got := moveOne(t, "move.sysml", src, "P::B", "P::A")
	want := "package P {\n" +
		"    part def A {\n" +
		"        part def B;\n" +
		"    }\n" +
		"}\n"
	if got != want {
		t.Fatalf("moved source:\n%s\nwant:\n%s", got, want)
	}
}

func TestMoveToRootAndOutOfNesting(t *testing.T) {
	src := "package P {\n" +
		"    part def A {\n" +
		"        part def B {\n" +
		"            part def C;\n" +
		"        }\n" +
		"    }\n" +
		"}\n"
	got := moveOne(t, "move.sysml", src, "P::A::B", "")
	want := "package P {\n" +
		"    part def A {\n" +
		"    }\n" +
		"}\n" +
		"part def B {\n" +
		"    part def C;\n" +
		"}\n"
	if got != want {
		t.Fatalf("moved source:\n%s\nwant:\n%s", got, want)
	}
}

func TestMoveRespellsQualifiedReferences(t *testing.T) {
	src := "package P {\n" +
		"    package Lib {\n" +
		"        part def Base;\n" +
		"    }\n" +
		"    package Other;\n" +
		"    part def Use {\n" +
		"        part b : Lib::Base;\n" +
		"        part c : P::Lib::Base;\n" +
		"    }\n" +
		"}\n"
	got := moveOne(t, "move.sysml", src, "P::Lib::Base", "P::Other")
	want := "package P {\n" +
		"    package Lib {\n" +
		"    }\n" +
		"    package Other {\n" +
		"        part def Base;\n" +
		"    }\n" +
		"    part def Use {\n" +
		"        part b : Other::Base;\n" +
		"        part c : P::Other::Base;\n" +
		"    }\n" +
		"}\n"
	if got != want {
		t.Fatalf("moved source:\n%s\nwant:\n%s", got, want)
	}
}

func TestMoveQualifiesLocalReferencesThatStopResolving(t *testing.T) {
	src := "package P {\n" +
		"    part def Base;\n" +
		"    part def Holder;\n" +
		"    part b : Base;\n" +
		"}\n"
	got := moveOne(t, "move.sysml", src, "P::Base", "P::Holder")
	want := "package P {\n" +
		"    part def Holder {\n" +
		"        part def Base;\n" +
		"    }\n" +
		"    part b : Holder::Base;\n" +
		"}\n"
	if got != want {
		t.Fatalf("moved source:\n%s\nwant:\n%s", got, want)
	}
}

func TestMoveKeepsReferencesStillResolving(t *testing.T) {
	src := "package P {\n" +
		"    part def Base;\n" +
		"    part def Holder {\n" +
		"        part def Inner;\n" +
		"        part b : Base;\n" +
		"    }\n" +
		"    part def Sibling {\n" +
		"        part h : Holder;\n" +
		"    }\n" +
		"}\n"
	got := moveOne(t, "move.sysml", src, "P::Sibling", "P::Holder")
	want := "package P {\n" +
		"    part def Base;\n" +
		"    part def Holder {\n" +
		"        part def Inner;\n" +
		"        part b : Base;\n" +
		"        part def Sibling {\n" +
		"            part h : Holder;\n" +
		"        }\n" +
		"    }\n" +
		"}\n"
	if got != want {
		t.Fatalf("moved source:\n%s\nwant:\n%s", got, want)
	}
}

func TestMoveRewritesReferencesInsideTheTarget(t *testing.T) {
	src := "package P {\n" +
		"    package Lib {\n" +
		"        part def Base;\n" +
		"        part def Use {\n" +
		"            part b : Base;\n" +
		"        }\n" +
		"    }\n" +
		"    package Other;\n" +
		"}\n"
	got := moveOne(t, "move.sysml", src, "P::Lib::Use", "P::Other")
	want := "package P {\n" +
		"    package Lib {\n" +
		"        part def Base;\n" +
		"    }\n" +
		"    package Other {\n" +
		"        part def Use {\n" +
		"            part b : Lib::Base;\n" +
		"        }\n" +
		"    }\n" +
		"}\n"
	if got != want {
		t.Fatalf("moved source:\n%s\nwant:\n%s", got, want)
	}
}

func TestMoveRemovesImportMadeRedundant(t *testing.T) {
	src := "package P {\n" +
		"    package Lib {\n" +
		"        part def Base;\n" +
		"    }\n" +
		"    package Other {\n" +
		"        import Lib::Base;\n" +
		"        part b : Base;\n" +
		"    }\n" +
		"}\n"
	got := moveOne(t, "move.sysml", src, "P::Lib::Base", "P::Other")
	want := "package P {\n" +
		"    package Lib {\n" +
		"    }\n" +
		"    package Other {\n" +
		"        part b : Base;\n" +
		"        part def Base;\n" +
		"    }\n" +
		"}\n"
	if got != want {
		t.Fatalf("moved source:\n%s\nwant:\n%s", got, want)
	}
}

func TestMoveReadsNameBetweenCarriedImportsItRespells(t *testing.T) {
	// Both carried imports grow by the package's name; the name between them is
	// read where it lands, not where the first import's growth would push it.
	src := "package Spacecraft_Models {\n" +
		"    package Lib {\n" +
		"        part def A;\n" +
		"        part def B;\n" +
		"    }\n" +
		"    part def Holder {\n" +
		"        import Lib::A;\n" +
		"        part a : A;\n" +
		"        import Lib::B;\n" +
		"        part b : B;\n" +
		"    }\n" +
		"}\n"
	got := moveOne(t, "move.sysml", src, "Spacecraft_Models::Holder", "")
	want := "package Spacecraft_Models {\n" +
		"    package Lib {\n" +
		"        part def A;\n" +
		"        part def B;\n" +
		"    }\n" +
		"}\n" +
		"part def Holder {\n" +
		"    import Spacecraft_Models::Lib::A;\n" +
		"    part a : A;\n" +
		"    import Spacecraft_Models::Lib::B;\n" +
		"    part b : B;\n" +
		"}\n"
	if got != want {
		t.Fatalf("moved source:\n%s\nwant:\n%s", got, want)
	}
}

func TestMoveRespellsImportOfTheTarget(t *testing.T) {
	src := "package P {\n" +
		"    package Lib {\n" +
		"        part def Base;\n" +
		"    }\n" +
		"    package Other;\n" +
		"    package Client {\n" +
		"        import Lib::Base;\n" +
		"        part b : Base;\n" +
		"    }\n" +
		"}\n"
	got := moveOne(t, "move.sysml", src, "P::Lib::Base", "P::Other")
	want := "package P {\n" +
		"    package Lib {\n" +
		"    }\n" +
		"    package Other {\n" +
		"        part def Base;\n" +
		"    }\n" +
		"    package Client {\n" +
		"        import Other::Base;\n" +
		"        part b : Base;\n" +
		"    }\n" +
		"}\n"
	if got != want {
		t.Fatalf("moved source:\n%s\nwant:\n%s", got, want)
	}
}

// The $:: spelling is the one route left when every suffix of the element's
// qualified name is captured at the reference's new place.
func TestMoveQualifiesFromTheGlobalNamespaceWhenCaptured(t *testing.T) {
	src := "package P {\n" +
		"    part def Base;\n" +
		"    part def Holder {\n" +
		"        private part def Base;\n" +
		"        private part def P {\n" +
		"            part def Base;\n" +
		"        }\n" +
		"    }\n" +
		"    part b : Base;\n" +
		"}\n"
	got := moveOne(t, "move.sysml", src, "P::b", "P::Holder")
	if !strings.Contains(got, "part b : $::P::Base;") {
		t.Fatalf("moved source:\n%s\nwant b typed by $::P::Base", got)
	}
}

func TestMoveRefusals(t *testing.T) {
	src := "package P {\n" +
		"    part def A {\n" +
		"        part def Inner;\n" +
		"    }\n" +
		"    part def B {\n" +
		"        part def Inner;\n" +
		"    }\n" +
		"    alias Alias for A;\n" +
		"    requirement def R {\n" +
		"        subject s : A;\n" +
		"    }\n" +
		"    state def Run {\n" +
		"        entry action boot;\n" +
		"    }\n" +
		"}\n"
	cases := []struct {
		name string
		op   Operation
		want Failure
	}{
		{"unknown target", Move("P::Missing", "P::A"), FailureUnknownTarget},
		{"unknown owner", Move("P::A", "P::Missing"), FailureOwnerUnknown},
		{"owner not a namespace", Move("P::A", "P::Alias"), FailureOwnerNotNamespace},
		{"owner is the target", Move("P::A", "P::A"), FailureOwnerInsideTarget},
		{"owner inside the target", Move("P::A", "P::A::Inner"), FailureOwnerInsideTarget},
		{"name clash", Move("P::A::Inner", "P::B"), FailureMemberNameTaken},
		{"kind not admitted", Move("P::R::s", "P::A"), FailureIllegalKind},
		{"prefixed kind not admitted", Move("P::Run::boot", "P::A"), FailureIllegalKind},
	}
	m := loadContent(t, "move.sysml", src)
	requireClean(t, m)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			addFailure(t, m, tc.op, tc.want)
		})
	}
}

func TestMoveRefusesWhenAnotherDocumentRefers(t *testing.T) {
	src := "package P {\n" +
		"    part def Base;\n" +
		"    part def Holder;\n" +
		"}\n"
	m := loadWorkspace(t, "move.sysml", src, map[string]string{
		"other.sysml": "package Q {\n    part b : P::Base;\n}\n",
	})
	e := addFailure(t, m, Move("P::Base", "P::Holder"), FailureReferencedElsewhere)
	if !strings.Contains(strings.Join(e.Referring, ","), "other.sysml") {
		t.Fatalf("referrers = %v, want other.sysml", e.Referring)
	}
}

// A local reference the new owner would capture is qualified past the capture.
func TestMoveQualifiesCapturedReference(t *testing.T) {
	src := "package P {\n" +
		"    part def Base;\n" +
		"    part def Holder {\n" +
		"        private part def Base;\n" +
		"    }\n" +
		"    part b : Base;\n" +
		"}\n"
	got := moveOne(t, "move.sysml", src, "P::b", "P::Holder")
	want := "package P {\n" +
		"    part def Base;\n" +
		"    part def Holder {\n" +
		"        private part def Base;\n" +
		"        part b : P::Base;\n" +
		"    }\n" +
		"}\n"
	if got != want {
		t.Fatalf("moved source:\n%s\nwant:\n%s", got, want)
	}
}

// A reference through a feature chain has no qualified-name spelling, so a
// move that breaks it is refused rather than left dangling.
func TestMoveRefusesReferenceItCannotRespell(t *testing.T) {
	src := "package P {\n" +
		"    part def Base;\n" +
		"    part def H {\n" +
		"        part b : Base;\n" +
		"    }\n" +
		"    part def Other;\n" +
		"    part h : H;\n" +
		"    part c : Base = h.b;\n" +
		"}\n"
	m := loadContent(t, "move.sysml", src)
	requireClean(t, m)
	e := addFailure(t, m, Move("P::H::b", "P::Other"), FailureMoveReferenced)
	if !strings.Contains(e.Message, "P::c") {
		t.Fatalf("message = %q, want the reference named", e.Message)
	}
	if len(e.Referring) != 1 || e.Referring[0] != "P::c" {
		t.Fatalf("referring = %v, want [P::c]", e.Referring)
	}
	if len(e.Referrers) != 1 || e.Referrers[0] != (Referrer{Name: "P::c", Document: "move.sysml"}) {
		t.Fatalf("referrers = %+v, want P::c in move.sysml", e.Referrers)
	}
}

func TestMoveInBatchIsSequentialAndAllOrNothing(t *testing.T) {
	src := "package P {\n" +
		"    part def Base;\n" +
		"    part def Holder;\n" +
		"}\n"
	m := loadContent(t, "move.sysml", src)
	requireClean(t, m)
	ops := []Operation{
		AddMember("P::Holder", "part def", "Inner"),
		Move("P::Base", "P::Holder::Inner"),
		Rename("P::Holder::Inner::Base", "Core"),
		Delete("P::Holder", false),
	}
	if !needsSequential(ops) {
		t.Fatal("a batch with a move is applied sequentially")
	}
	res, err := Apply(m, ops)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := "package P {\n}\n"
	if got := string(res.Content); got != want {
		t.Fatalf("batch result:\n%s\nwant:\n%s", got, want)
	}

	res, err = Apply(m, []Operation{
		Move("P::Base", "P::Holder"),
		Move("P::Holder::Base", "P::Missing"),
	})
	if res != nil {
		t.Fatalf("refused batch returned content:\n%s", res.Content)
	}
	e := editError(t, err)
	if e.Failure != FailureOwnerUnknown || e.OperationIndex != 1 {
		t.Fatalf("failure = %s at %d, want %s at 1", e.Failure, e.OperationIndex, FailureOwnerUnknown)
	}
	if string(m.Source.Bytes()) != src {
		t.Fatal("a refused batch changed the source")
	}
}

// A tab-indented, commented declaration keeps its comment and takes the new
// owner's indentation; references to what it declares are respelled with it.
func TestMoveSpacecraftPartIntoUsage(t *testing.T) {
	m := load(t, "spacecraft.sysml")
	requireClean(t, m)
	res := applyOne(t, m, Move("Demo::SC::avionics", "Demo::sc"))
	got := string(res.Content)
	want := "\tpart sc : SC {\n" +
		"\t\tattribute redefines unitMass = 1200.0[SI::kg];\n" +
		"\t\tpart avionics {\n" +
		"\t\t\tpart board {\n" +
		"\t\t\t\tattribute count : ScalarValues::Integer = 2;\n" +
		"\t\t\t}\n" +
		"\t\t}\n" +
		"\t}\n"
	if !strings.Contains(got, want) {
		t.Fatalf("moved source:\n%s\nwant it to contain:\n%s", got, want)
	}
	if strings.Contains(got, "\t\tpart avionics {\n\t\t\tpart board {\n\t\t\t\tattribute count : ScalarValues::Integer = 2;\n\t\t\t}\n\t\t}\n\t}\n\n\tpart sc") {
		t.Fatalf("target left in SC:\n%s", got)
	}
	if !strings.Contains(got, "\t\tattribute total : ISQ::MassValue = unitMass;\n\t}\n\n\tpart sc : SC {") {
		t.Fatalf("SC's body did not close where avionics was:\n%s", got)
	}
	requireClean(t, loadContent(t, "spacecraft.sysml", got))
}

func TestMoveRespellsReferencesToMembersOfTheTargetAndQuotedNames(t *testing.T) {
	src := "package P {\n" +
		"    package Lib {\n" +
		"        part def 'Base Part' {\n" +
		"            part def Inner;\n" +
		"        }\n" +
		"    }\n" +
		"    package 'New Home';\n" +
		"    part def Use {\n" +
		"        part i : Lib::'Base Part'::Inner;\n" +
		"    }\n" +
		"}\n"
	got := moveOne(t, "move.sysml", src, "P::Lib::'Base Part'", "P::'New Home'")
	want := "package P {\n" +
		"    package Lib {\n" +
		"    }\n" +
		"    package 'New Home' {\n" +
		"        part def 'Base Part' {\n" +
		"            part def Inner;\n" +
		"        }\n" +
		"    }\n" +
		"    part def Use {\n" +
		"        part i : 'New Home'::'Base Part'::Inner;\n" +
		"    }\n" +
		"}\n"
	if got != want {
		t.Fatalf("moved source:\n%s\nwant:\n%s", got, want)
	}
}

func TestMoveRootDeclarationIntoPackage(t *testing.T) {
	src := "part def Widget;\n" +
		"package P {\n" +
		"    part b : Widget;\n" +
		"}\n" +
		"part c : Widget;\n"
	got := moveOne(t, "move.sysml", src, "Widget", "P")
	want := "package P {\n" +
		"    part b : Widget;\n" +
		"    part def Widget;\n" +
		"}\n" +
		"part c : P::Widget;\n"
	if got != want {
		t.Fatalf("moved source:\n%s\nwant:\n%s", got, want)
	}
}

func TestMoveWithinTheSameOwnerReorders(t *testing.T) {
	src := "package P {\n" +
		"    part def A;\n" +
		"    part def B;\n" +
		"}\n"
	got := moveOne(t, "move.sysml", src, "P::A", "P")
	want := "package P {\n" +
		"    part def B;\n" +
		"    part def A;\n" +
		"}\n"
	if got != want {
		t.Fatalf("moved source:\n%s\nwant:\n%s", got, want)
	}
}

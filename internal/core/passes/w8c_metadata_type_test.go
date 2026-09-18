package passes

import (
	"sort"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

func TestW8CMetadataAbstractType(t *testing.T) {
	src := `package P {
	abstract metaclass A { feature :>> annotatedElement : KerML::Classifier; }
	classifier B {
		@A;
	}
}`
	msgs := w8cLibraryMessagesIn(t, "meta-abstract.kerml", src)
	if w8cCount(msgs, msgMetadataConcreteType) != 1 {
		t.Errorf("want one %q, got %v", msgMetadataConcreteType, msgs)
	}
}

// The rule judges each annotation on its own, so an unresolved name elsewhere in
// the file does not hide it, and an unresolved metaclass draws no cascade.
func TestW8CMetadataAbstractTypeIsElementScoped(t *testing.T) {
	src := `package P {
	abstract metaclass A { feature :>> annotatedElement : KerML::Classifier; }
	classifier B {
		@A;
		feature :>> nowhere;
	}
}`
	msgs := w8cLibraryMessagesIn(t, "meta-abstract-unresolved.kerml", src)
	if w8cCount(msgs, msgMetadataConcreteType) != 1 {
		t.Errorf("want one %q despite the unresolved feature, got %v", msgMetadataConcreteType, msgs)
	}

	unresolved := `package P {
	classifier B {
		@Nowhere;
	}
}`
	msgs = w8cLibraryMessagesIn(t, "meta-unresolved-metaclass.kerml", unresolved)
	if w8cCount(msgs, msgMetadataConcreteType) != 0 {
		t.Errorf("unresolved metaclass must not cascade: %v", msgs)
	}
}

// p24: the metaclass is a library one, so the rule can only judge it while the
// library is parsed on every load path rather than reduced to facts.
func TestW8CMetadataAbstractLibraryMetaclass(t *testing.T) {
	src := `package P {
	item p {
		@Metaobjects::Metaobject;
	}
}`
	msgs := w8cLibraryMessagesIn(t, "meta-abstract-library.sysml", src)
	if w8cCount(msgs, msgMetadataConcreteType) != 1 {
		t.Errorf("want one %q for the abstract library metaclass, got %v", msgMetadataConcreteType, msgs)
	}
}

// A concrete library metaclass stays legal, so the rule reads abstractness rather
// than rejecting library metaclasses.
func TestW8CMetadataConcreteLibraryMetaclass(t *testing.T) {
	src := `package P {
	item p {
		@KerML::Comment;
	}
}`
	msgs := w8cLibraryMessagesIn(t, "meta-concrete-library.sysml", src)
	if w8cCount(msgs, msgMetadataConcreteType) != 0 {
		t.Errorf("unexpected %q for a concrete library metaclass: %v", msgMetadataConcreteType, msgs)
	}
}

// A metadata feature is typed by exactly one metaclass (validateMetadataFeatureMetadata):
// a concrete class, structure or datatype is not one, in either spelling of the annotation.
func TestW8CMetadataTypeMustBeAMetaclass(t *testing.T) {
	src := `package P {
	class M;
	struct S;
	datatype D;
	metaclass MC;
	class C;
	@M about C;
	#S class X;
	metadata m : D about C;
	metadata ok : MC about C;
	#MC class Y;
}`
	diags := w8cLibraryDiagnostics(t, "meta-not-metaclass.kerml", src)
	if lines := linesOf(src, only(diags, "metadata-metaclass")); !equalInts(lines, []int{7, 8, 9}) {
		t.Errorf("metadata-metaclass at lines %v, want [7 8 9]: %v", lines, diags)
	}
	for _, d := range only(diags, "metadata-metaclass") {
		if d.Message != msgMetadataMetaclass {
			t.Errorf("message %q, want %q", d.Message, msgMetadataMetaclass)
		}
	}
	if len(only(diags, "metadata-concrete-type")) != 0 {
		t.Errorf("a concrete non-metaclass is not reported as abstract: %v", diags)
	}
}

// The SysML spelling (validateMetadataUsageType): a metadata usage is typed by one
// metadata definition, and each faulty typing is reported exactly once whichever rule names it.
func TestW8CMetadataUsageTypeMustBeOneMetadataDefinition(t *testing.T) {
	src := `package P {
	part def PD;
	metadata def MD;
	metadata def MD2;
	part p { @PD; }
	#PD part q;
	metadata m : PD about p;
	metadata two : MD, MD2 about p;
	metadata ok : MD about p;
	#MD part r;
	part s { @MD2; }
}`
	diags := w8cLibraryDiagnostics(t, "meta-usage-type.sysml", src)
	var typed []diag.Diagnostic
	for _, d := range diags {
		if d.Message == oneTypeUsageMessages[ast.UsageMetadata] {
			typed = append(typed, d)
		}
	}
	if lines := linesOf(src, typed); !equalInts(lines, []int{5, 6, 7, 8}) {
		t.Errorf("metadata typing reported at lines %v, want [5 6 7 8]: %v", lines, diags)
	}
}

// The rule reads the type through an alias.
func TestW8CMetadataTypeThroughAlias(t *testing.T) {
	src := `package P {
	class M;
	metaclass MC;
	alias AM for M;
	alias AMC for MC;
	class C;
	@AM about C;
	@AMC about C;
}`
	diags := w8cLibraryDiagnostics(t, "meta-alias.kerml", src)
	if lines := linesOf(src, only(diags, "metadata-metaclass")); !equalInts(lines, []int{7}) {
		t.Errorf("metadata-metaclass at lines %v, want [7]: %v", lines, diags)
	}
}

// The annotated element owns its annotations, prefix or member (KerML 8.2.4.2), so
// a same-named type nested in its body shadows the outer one, as in the pilot.
func TestW8CMetadataTypeIsReadInTheAnnotatedElement(t *testing.T) {
	src := `package P {
	metaclass M;
	class N;
	#M class A { class M; }
	#N class B { metaclass N; }
	class C { class M; @M; }
	class D { metaclass N; @N; }
}`
	diags := w8cLibraryDiagnostics(t, "meta-annotation-scope.kerml", src)
	if lines := linesOf(src, only(diags, "metadata-metaclass")); !equalInts(lines, []int{4, 6}) {
		t.Errorf("metadata-metaclass at lines %v, want [4 6]: %v", lines, diags)
	}

	sysml := `package P {
	metadata def M;
	part def N;
	#M part def A { part def M; }
	#N part def B { metadata def N; }
	part def C { part def M; @M; }
	part def D { metadata def N; @N; }
}`
	diags = w8cLibraryDiagnostics(t, "meta-annotation-scope.sysml", sysml)
	var typed []diag.Diagnostic
	for _, d := range diags {
		if d.Message == oneTypeUsageMessages[ast.UsageMetadata] {
			typed = append(typed, d)
		}
	}
	if lines := linesOf(sysml, typed); !equalInts(lines, []int{4, 6}) {
		t.Errorf("metadata typing reported at lines %v, want [4 6]: %v", lines, diags)
	}
}

// linesOf returns the sorted 1-based lines of the diagnostics.
func linesOf(src string, diags []diag.Diagnostic) []int {
	var lines []int
	for _, d := range diags {
		lines = append(lines, w8dLine(src, d.Span))
	}
	sort.Ints(lines)
	return lines
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// A prefix on a subject, assume or require member names a metadata type like a
// prefix on any usage, so an abstract one is reported on each of the three.
func TestW8CMetadataAbstractTypeOnRequirementMembers(t *testing.T) {
	src := `package P {
	abstract metadata def A;
	constraint def C;
	requirement def R {
		subject #A s;
		assume #A constraint a : C;
		require #A constraint r : C;
		assume #A constraint { true }
		require #A constraint { true }
	}
}`
	msgs := w8cLibraryMessagesIn(t, "meta-abstract-requirement-members.sysml", src)
	if w8cCount(msgs, msgMetadataConcreteType) != 5 {
		t.Errorf("want five %q, got %v", msgMetadataConcreteType, msgs)
	}
}

func TestW8CMetadataConcreteTypeLegal(t *testing.T) {
	src := `package P {
	metaclass A { feature :>> annotatedElement : KerML::Classifier; }
	classifier B {
		@A;
	}
}`
	msgs := w8cLibraryMessagesIn(t, "meta-concrete.kerml", src)
	if w8cCount(msgs, msgMetadataConcreteType) != 0 {
		t.Errorf("unexpected %q: %v", msgMetadataConcreteType, msgs)
	}
}

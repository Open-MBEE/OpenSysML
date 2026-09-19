package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/identity"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// identityDiagsAcross analyses two workspace documents with the default
// registry and returns each document's diagnostics.
func identityDiagsAcross(t *testing.T, srcA, srcB string) ([]diag.Diagnostic, []diag.Diagnostic) {
	t.Helper()
	rootA := parser.New(source.New("<a>", []byte(srcA))).ParseFile()
	rootB := parser.New(source.New("<b>", []byte(srcB))).ParseFile()
	idx := newTestIndex()
	idx.AddDocument("<a>", rootA)
	idx.AddDocument("<b>", rootB)
	return Analyze("<a>", rootA, nil, idx), Analyze("<b>", rootB, nil, idx)
}

func TestIdentityDuplicateIdsAcrossDocumentsOfOneProject(t *testing.T) {
	srcA := `package PA {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def A {
		@IdentityMetadata::ElementId { id = "same-id"; }
	}
}
`
	srcB := `package PB {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def B {
		@IdentityMetadata::ElementId { id = "same-id"; }
	}
}
`
	diagsA, diagsB := identityDiagsAcross(t, srcA, srcB)
	for _, diags := range [][]diag.Diagnostic{only(diagsA, "identity-duplicate-id"), only(diagsB, "identity-duplicate-id")} {
		if len(diags) != 1 {
			t.Fatalf("got %d duplicate-id diagnostics in one document, want 1: %v", len(diags), diags)
		}
		if !strings.Contains(diags[0].Message, "PA::A") || !strings.Contains(diags[0].Message, "PB::B") {
			t.Fatalf("diagnostic must name both elements: %q", diags[0].Message)
		}
	}
}

func TestIdentityDuplicateIdsAcrossDocumentsOfDifferentProjectsAreLegal(t *testing.T) {
	srcA := `package PA {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def A {
		@IdentityMetadata::ElementId { id = "same-id"; }
	}
}
`
	srcB := `package PB {
	@IdentityMetadata::ProjectRef { projectId = "proj-2"; }
	part def B {
		@IdentityMetadata::ElementId { id = "same-id"; }
	}
}
`
	diagsA, diagsB := identityDiagsAcross(t, srcA, srcB)
	if n := len(only(diagsA, "identity-duplicate-id")) + len(only(diagsB, "identity-duplicate-id")); n != 0 {
		t.Fatalf("got %d duplicate-id diagnostics, want none across different projects", n)
	}
}

func TestIdentityEmptyIdFailsTheShapeCheck(t *testing.T) {
	src := `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def A {
		@IdentityMetadata::ElementId { id = ""; }
	}
}
`
	diags := only(w8dDiags(t, src), "identity-id-shape")
	if len(diags) != 1 {
		t.Fatalf("got %d shape diagnostics, want 1: %v", len(diags), diags)
	}
	if !strings.Contains(diags[0].Message, "empty") {
		t.Fatalf("diagnostic must call the id empty: %q", diags[0].Message)
	}
}

func TestIdentityDuplicateDeclaredIdsInOneScope(t *testing.T) {
	src := `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def A {
		@IdentityMetadata::ElementId { id = "same-id"; }
	}
	part def B {
		@IdentityMetadata::ElementId { id = "same-id"; }
	}
}
`
	diags := only(w8dDiags(t, src), "identity-duplicate-id")
	if len(diags) != 2 {
		t.Fatalf("got %d duplicate-id diagnostics, want 2: %v", len(diags), diags)
	}
	for _, d := range diags {
		if !strings.Contains(d.Message, "P::A") || !strings.Contains(d.Message, "P::B") {
			t.Fatalf("diagnostic must name both elements: %q", d.Message)
		}
	}
}

func TestIdentitySameIdUnderDifferentProjectsIsLegal(t *testing.T) {
	src := `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def A {
		@IdentityMetadata::ElementId { id = "same-id"; }
	}
	package Q {
		@IdentityMetadata::ProjectRef { projectId = "proj-2"; }
		part def B {
			@IdentityMetadata::ElementId { id = "same-id"; }
		}
	}
}
`
	w8dWantLines(t, src, "identity-duplicate-id")
}

func TestIdentitySameProjectOnTwoBranchesIsOneScope(t *testing.T) {
	src := `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "main"; }
	part def A {
		@IdentityMetadata::ElementId { id = "same-id"; }
	}
	package Q {
		@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "dev"; }
		part def B {
			@IdentityMetadata::ElementId { id = "same-id"; }
		}
	}
}
`
	w8dWantLines(t, src, "identity-duplicate-id", 4, 9)
}

func TestIdentityDeclaredIdCollidingWithADerivedIdErrorsOnBoth(t *testing.T) {
	src := `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def X;
	part def Y {
		@IdentityMetadata::ElementId { id = "P__X"; }
	}
}
`
	w8dWantLines(t, src, "identity-duplicate-id", 3, 5)
}

func TestIdentityDeclaredIdEndingInOmCollidesWithAMembershipId(t *testing.T) {
	src := `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def X;
	part def Y {
		@IdentityMetadata::ElementId { id = "P__X_om"; }
	}
}
`
	diags := only(w8dDiags(t, src), "identity-duplicate-id")
	if len(diags) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %v", len(diags), diags)
	}
	for _, d := range diags {
		if !strings.Contains(d.Message, "owning-membership") ||
			!strings.Contains(d.Message, "P::X") || !strings.Contains(d.Message, "P::Y") {
			t.Fatalf("diagnostic must name both elements and the membership space: %q", d.Message)
		}
	}
}

func TestIdentityDeclaredIdEmbeddingPCollidesWithAnExpressionNodeId(t *testing.T) {
	src := `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def X;
	part def Y {
		@IdentityMetadata::ElementId { id = "P__X_p3"; }
	}
}
`
	diags := only(w8dDiags(t, src), "identity-duplicate-id")
	if len(diags) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %v", len(diags), diags)
	}
	for _, d := range diags {
		if !strings.Contains(d.Message, "expression-node") ||
			!strings.Contains(d.Message, "P::X") || !strings.Contains(d.Message, "P::Y") {
			t.Fatalf("diagnostic must name both elements and the expression space: %q", d.Message)
		}
	}
}

func TestIdentityIdsWithUndecodableSuffixesAfterPAreLegal(t *testing.T) {
	for _, id := range []string{"P__X_p", "P__X_p_"} {
		src := `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def X;
	part def Y {
		@IdentityMetadata::ElementId { id = "` + id + `"; }
	}
}
`
		w8dWantLines(t, src, "identity-duplicate-id")
	}
}

func TestIdentityIdShapeErrorNamesTheOffendingByte(t *testing.T) {
	src := `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def A {
		@IdentityMetadata::ElementId { id = "bad id"; }
	}
}
`
	diags := only(w8dDiags(t, src), "identity-id-shape")
	if len(diags) != 1 {
		t.Fatalf("got %d shape diagnostics, want 1: %v", len(diags), diags)
	}
	if !strings.Contains(diags[0].Message, "0x20") || !strings.Contains(diags[0].Message, "offset 3") {
		t.Fatalf("diagnostic must name the offending byte and offset: %q", diags[0].Message)
	}
}

func TestIdentityElementIdWithoutEnclosingProjectRef(t *testing.T) {
	src := `package P {
	part def A {
		@IdentityMetadata::ElementId { id = "some-id"; }
	}
}
`
	w8dWantLines(t, src, "identity-unscoped-id", 3)
}

func TestIdentityNestedProjectRefScopesArePermitted(t *testing.T) {
	src := `package Outer {
	@IdentityMetadata::ProjectRef { projectId = "outer-proj"; }
	part def A {
		@IdentityMetadata::ElementId { id = "id-a"; }
	}
	package Inner {
		@IdentityMetadata::ProjectRef { projectId = "inner-proj"; }
		part def B {
			@IdentityMetadata::ElementId { id = "id-b"; }
		}
	}
}
`
	for _, code := range []string{"identity-duplicate-id", "identity-id-shape", "identity-unscoped-id"} {
		w8dWantLines(t, src, code)
	}
}

func TestIdentityLegalAnnotationsStaySilent(t *testing.T) {
	src := `package Vehicles {
	@IdentityMetadata::ProjectRef { projectId = "b3f9c2e8"; branch = "main"; }
	part def Vehicle {
		@IdentityMetadata::ElementId { id = "8f3a41d0-1c22-4d9e-9c3b-000000000001"; }
	}
	part def Wheel;
}
`
	for _, d := range w8dDiags(t, src) {
		if strings.HasPrefix(d.Code, "identity-") {
			t.Fatalf("unexpected identity diagnostic: %+v", d)
		}
	}
}

func TestIdentityConflictingInlineAndAboutIdsErrorOnBoth(t *testing.T) {
	src := `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def X {
		@IdentityMetadata::ElementId { id = "inline-id"; }
	}
	metadata xid : IdentityMetadata::ElementId about X {
		id = "about-id";
	}
}
`
	diags := only(w8dDiags(t, src), "identity-conflicting-ids")
	if len(diags) != 2 {
		t.Fatalf("got %d conflicting-ids diagnostics, want one per annotation: %v", len(diags), diags)
	}
	for _, d := range diags {
		if !strings.Contains(d.Message, `"about-id"`) || !strings.Contains(d.Message, `"inline-id"`) {
			t.Fatalf("diagnostic must name both ids: %q", d.Message)
		}
	}
	w8dWantLines(t, src, "identity-conflicting-ids", 4, 6)
}

func TestIdentityAgreeingInlineAndAboutIdsStaySilent(t *testing.T) {
	src := `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def X {
		@IdentityMetadata::ElementId { id = "one-id"; }
	}
	metadata xid : IdentityMetadata::ElementId about X {
		id = "one-id";
	}
}
`
	if diags := w8dDiags(t, src); len(only(diags, "identity-conflicting-ids"))+len(only(diags, "identity-duplicate-id")) != 0 {
		t.Fatalf("agreeing annotations must stay silent: %v", diags)
	}
}

func TestIdentityAboutFormDuplicateIdsError(t *testing.T) {
	src := `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def A;
	part def B {
		@IdentityMetadata::ElementId { id = "same-id"; }
	}
	metadata aid : IdentityMetadata::ElementId about A {
		id = "same-id";
	}
}
`
	diags := only(w8dDiags(t, src), "identity-duplicate-id")
	if len(diags) != 2 {
		t.Fatalf("got %d duplicate-id diagnostics, want 2: %v", len(diags), diags)
	}
	for _, d := range diags {
		if !strings.Contains(d.Message, "P::A") || !strings.Contains(d.Message, "P::B") {
			t.Fatalf("diagnostic must name both elements: %q", d.Message)
		}
	}
}

func TestIdentityAboutFormElementIdWithoutProjectRefErrors(t *testing.T) {
	src := `package P {
	part def X;
	metadata xid : IdentityMetadata::ElementId about X {
		id = "some-id";
	}
}
`
	w8dWantLines(t, src, "identity-unscoped-id", 3)
}

func TestIdentityAboutFormBadIdShapeErrorsAtTheUsage(t *testing.T) {
	src := `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def X;
	metadata xid : IdentityMetadata::ElementId about X {
		id = "bad id";
	}
}
`
	diags := only(w8dDiags(t, src), "identity-id-shape")
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "0x20") {
		t.Fatalf("got %v, want one shape error naming byte 0x20", diags)
	}
	w8dWantLines(t, src, "identity-id-shape", 4)
}

func TestIdentityUnnamedAboutFormBadIdShapeErrorsAtTheUsage(t *testing.T) {
	src := `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def X;
	metadata : IdentityMetadata::ElementId about X {
		id = "bad id";
	}
}
`
	diags := only(w8dDiags(t, src), "identity-id-shape")
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "0x20") {
		t.Fatalf("got %v, want one shape error naming byte 0x20", diags)
	}
	w8dWantLines(t, src, "identity-id-shape", 4)
}

func TestIdentityCrossDocumentAboutAnnotationErrorsInItsOwnDocument(t *testing.T) {
	srcA := `package PA {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def X;
}
`
	srcB := `package PB {
	metadata xid : IdentityMetadata::ElementId about PA::X {
		id = "bad id";
	}
}
`
	diagsA, diagsB := identityDiagsAcross(t, srcA, srcB)
	if len(only(diagsA, "identity-id-shape")) != 0 {
		t.Fatalf("the annotated element's document must stay clean, got %v", only(diagsA, "identity-id-shape"))
	}
	diags := only(diagsB, "identity-id-shape")
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "PA::X") {
		t.Fatalf("got %v, want one shape error in the annotating document naming PA::X", diags)
	}
	if line := w8dLine(srcB, diags[0].Span); line != 2 {
		t.Fatalf("got line %d, want the annotating usage's line 2", line)
	}
}

func TestIdentityCrossDocumentConflictErrorsInEachDeclaringDocument(t *testing.T) {
	srcA := `package PA {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def X {
		@IdentityMetadata::ElementId { id = "inline-id"; }
	}
}
`
	srcB := `package PB {
	metadata xid : IdentityMetadata::ElementId about PA::X {
		id = "about-id";
	}
}
`
	diagsA, diagsB := identityDiagsAcross(t, srcA, srcB)
	confA, confB := only(diagsA, "identity-conflicting-ids"), only(diagsB, "identity-conflicting-ids")
	if len(confA) != 1 || len(confB) != 1 {
		t.Fatalf("got %d and %d conflict diagnostics, want one per declaring document: %v %v",
			len(confA), len(confB), confA, confB)
	}
	if line := w8dLine(srcA, confA[0].Span); line != 4 {
		t.Fatalf("got line %d in the inline document, want 4", line)
	}
	if line := w8dLine(srcB, confB[0].Span); line != 2 {
		t.Fatalf("got line %d in the annotating document, want 2", line)
	}
}

func TestIdentityAboutFormProjectRefScopesTheElements(t *testing.T) {
	src := `package PA {
	part def A {
		@IdentityMetadata::ElementId { id = "same-id"; }
	}
}
package PB {
	part def B {
		@IdentityMetadata::ElementId { id = "same-id"; }
	}
}
package Meta {
	metadata pa : IdentityMetadata::ProjectRef about PA { projectId = "proj-1"; }
	metadata pb : IdentityMetadata::ProjectRef about PB { projectId = "proj-2"; }
}
`
	diags := w8dDiags(t, src)
	if len(only(diags, "identity-duplicate-id"))+len(only(diags, "identity-unscoped-id")) != 0 {
		t.Fatalf("distinct about-form projects must keep the ids legal: %v", diags)
	}
}

func TestIdentityConflictingProjectRefsErrorAtEachAnnotation(t *testing.T) {
	src := `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def X;
	metadata pref : IdentityMetadata::ProjectRef about P { projectId = "proj-2"; }
}
`
	diags := only(w8dDiags(t, src), "identity-conflicting-projects")
	if len(diags) != 2 {
		t.Fatalf("got %d conflicting-project diagnostics, want one per annotation: %v", len(diags), diags)
	}
	for _, d := range diags {
		if !strings.Contains(d.Message, `"proj-1"`) || !strings.Contains(d.Message, `"proj-2"`) {
			t.Fatalf("diagnostic must name both projects: %q", d.Message)
		}
	}
	w8dWantLines(t, src, "identity-conflicting-projects", 2, 4)
}

func TestIdentityAgreeingProjectRefsShareOneScope(t *testing.T) {
	src := `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; org = "org-a"; branch = "main"; }
	part def X;
	metadata pref : IdentityMetadata::ProjectRef about P { projectId = "proj-1"; org = "org-a"; branch = "dev"; }
}
`
	if diags := only(w8dDiags(t, src), "identity-conflicting-projects"); len(diags) != 0 {
		t.Fatalf("agreeing projects must not conflict (branch is a version selector): %v", diags)
	}
}

func TestIdentityAboutFormProjectRefsConflictingWithEachOther(t *testing.T) {
	src := `package P {
	part def X;
}
package Meta {
	metadata pa : IdentityMetadata::ProjectRef about P { projectId = "proj-1"; }
	metadata pb : IdentityMetadata::ProjectRef about P { projectId = "proj-2"; }
}
`
	diags := only(w8dDiags(t, src), "identity-conflicting-projects")
	if len(diags) != 2 {
		t.Fatalf("got %d conflicting-project diagnostics, want one per annotation: %v", len(diags), diags)
	}
	w8dWantLines(t, src, "identity-conflicting-projects", 5, 6)
}

func TestIdentityCrossDocumentProjectConflictErrorsInEachDeclaringDocument(t *testing.T) {
	srcA := `package PA {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def X;
}
`
	srcB := `package PB {
	metadata pref : IdentityMetadata::ProjectRef about PA { projectId = "proj-2"; }
}
`
	diagsA, diagsB := identityDiagsAcross(t, srcA, srcB)
	confA, confB := only(diagsA, "identity-conflicting-projects"), only(diagsB, "identity-conflicting-projects")
	if len(confA) != 1 || len(confB) != 1 {
		t.Fatalf("got %d and %d project-conflict diagnostics, want one per declaring document: %v %v",
			len(confA), len(confB), confA, confB)
	}
	if line := w8dLine(srcA, confA[0].Span); line != 2 {
		t.Fatalf("got line %d in the inline document, want 2", line)
	}
	if line := w8dLine(srcB, confB[0].Span); line != 2 {
		t.Fatalf("got line %d in the annotating document, want 2", line)
	}
}

func TestIdentityAboutFormOnLibraryElementValidatesInTheAnnotatingDocument(t *testing.T) {
	src := `package Meta {
	metadata sid : IdentityMetadata::ElementId about ScalarValues::Boolean {
		id = "bad id";
	}
}
`
	diags := only(w8dDiags(t, src), "identity-id-shape")
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "ScalarValues::Boolean") {
		t.Fatalf("got %v, want one shape error naming the library element", diags)
	}
	w8dWantLines(t, src, "identity-id-shape", 2)
	w8dWantLines(t, src, "identity-unscoped-id", 2)
}

// libraryCollisionSrc declares id for a user element in the project ScalarValues is bound to.
func libraryCollisionSrc(id string) string {
	return `package Meta {
	metadata pref : IdentityMetadata::ProjectRef about ScalarValues { projectId = "proj-1"; }
}
package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def A {
		@IdentityMetadata::ElementId { id = "` + id + `"; }
	}
}
`
}

func TestIdentityCollisionWithLibraryDescendantUnderAboutFormProjectRef(t *testing.T) {
	src := libraryCollisionSrc(identity.ElementID(identity.KerML, "ScalarValues::Boolean"))
	diags := only(w8dDiags(t, src), "identity-duplicate-id")
	if len(diags) != 1 {
		t.Fatalf("got %d duplicate-id diagnostics, want 1 at the workspace annotation: %v", len(diags), diags)
	}
	if !strings.Contains(diags[0].Message, "P::A") || !strings.Contains(diags[0].Message, "ScalarValues::Boolean") {
		t.Fatalf("diagnostic must name both elements: %q", diags[0].Message)
	}
	w8dWantLines(t, src, "identity-duplicate-id", 7)
}

// The encoded name is no library element's id any more, so declaring it collides with nothing.
func TestIdentityEncodedLibraryNameIsFreeUnderAboutFormProjectRef(t *testing.T) {
	src := libraryCollisionSrc(rdf.EncodeElementID("ScalarValues::Boolean"))
	if diags := only(w8dDiags(t, src), "identity-duplicate-id"); len(diags) != 0 {
		t.Fatalf("got duplicate-id diagnostics %v, want none", diags)
	}
}

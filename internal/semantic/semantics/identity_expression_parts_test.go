package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A satisfy subject, its `by` value expression and the expression's result share one span,
// so their identities must still differ, as must the FeatureValue's and the result's membership.
func TestExpressionPartsSharingASpanAreDistinctElements(t *testing.T) {
	m, root := buildModelWithStdlibNamedKind(t, "t.sysml", source.KindSysML, `package P {
	part def Sys;
	requirement def R;
	requirement massRequirement : R;
	part telescope : Sys;
	part observatory { satisfy massRequirement by telescope; }
}`)
	pkg, _ := root.LookupLocal("P")
	observatory, _ := pkg.Scope.LookupLocal("observatory")
	var subject *symbols.Symbol
	for _, member := range observatory.Scope.AllMembers() {
		for _, membership := range m.OwnedMemberships(member) {
			if m.implicitMetaclassName(membership) == "SubjectMembership" {
				subject = membership.Implicit.Member
			}
		}
	}
	if subject == nil {
		t.Fatal("no subject parameter under the satisfy usage")
	}
	featureValues := m.OwnedMemberships(subject)
	if len(featureValues) != 1 || m.implicitMetaclassName(featureValues[0]) != "FeatureValue" {
		t.Fatalf("the subject owns %v, want its FeatureValue", featureValues)
	}
	value := featureValues[0].Implicit.Member
	result := m.ExpressionResult(value)
	if value == nil || result == nil {
		t.Fatalf("value %v, result %v", value, result)
	}
	var resultMembership *symbols.Symbol
	for _, membership := range m.OwnedMemberships(value) {
		if m.implicitMetaclassName(membership) == "ReturnParameterMembership" {
			resultMembership = membership
		}
	}
	if resultMembership == nil {
		t.Fatal("the value owns no ReturnParameterMembership")
	}
	if subject.DeclSpan != value.DeclSpan || value.DeclSpan != result.DeclSpan {
		t.Fatalf("spans differ (%v, %v, %v); the fixture no longer exercises a shared span", subject.DeclSpan, value.DeclSpan, result.DeclSpan)
	}
	elements := map[string]*symbols.Symbol{"subject": subject, "value": value, "result": result,
		"featureValue": featureValues[0], "resultMembership": resultMembership}
	for a, sa := range elements {
		for b, sb := range elements {
			if a != b && symbols.SameElement(sa, sb) {
				t.Errorf("%s and %s read as one element: %v", a, b, symbols.KeyOf(sa))
			}
		}
	}
}

package passes

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// metadataDiags analyzes src as KerML against the standard library and returns
// the findings about metadata annotations, with the source text each covers.
func metadataDiags(t *testing.T, src string) []metadataFinding {
	t.Helper()
	return metadataDiagsNamed(t, "<t>.kerml", src)
}

// A body nested in a metadata body redefines the members of the feature the
// enclosing declaration redefines, through its type, at every depth and in
// every spelling; the redefinitions are neither duplicates nor unknown members.
func TestNestedMetadataBodiesRedefineTheNestedFeatureMembers(t *testing.T) {
	kerml := `package P {
	class U { feature d : ScalarValues::Integer; }
	class T { feature b : ScalarValues::Integer; feature c : U; }
	metaclass M { feature a : T; }
	feature f : ScalarValues::Integer;
	class C {
		@M { a { b = 1; c { d = 2; } } }
	}
	@M about C { :>> a { b = 1; c { d = 2; } } }
	metadata m : M about C { a { :>> b = 1; c { :>> d = 2; } } }
	%s
}`
	toSysML := strings.NewReplacer(
		"class U", "attribute def U",
		"class T", "attribute def T",
		"class C", "part def C",
		"metaclass", "metadata def",
		"feature", "attribute",
	)
	for _, tc := range []struct{ member, want string }{
		{"", ""},
		{"@M about C { a { c { e = 3; } } }", "metadata-owning-type-feature on e = 3;"},
		{"@M about C { :>> a { c { :>> f = 3; } } }", "metadata-owning-type-feature on :>> f = 3;"},
		{"metadata n : M about C { a { c { e = 3; } } }", "metadata-body-feature on e = 3;"},
		{"metadata n : M about C { :>> a { c { :>> f = 3; } } }", "metadata-body-feature on :>> f = 3;"},
	} {
		src := fmt.Sprintf(kerml, tc.member)
		for name, src := range map[string]string{"nested.kerml": src, "nested.sysml": toSysML.Replace(src)} {
			var got []string
			for _, f := range metadataDiagsNamed(t, name, src) {
				got = append(got, fmt.Sprintf("%s on %s", f.Code, strings.TrimSpace(f.Text)))
			}
			var want []string
			if tc.want != "" {
				want = []string{tc.want}
			}
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("%s with %q: findings %v, want %v", name, tc.member, got, want)
			}
		}
	}
}

// metadataDiagsNamed is metadataDiags over a document of the given name, whose
// extension decides the language it is analyzed as.
func metadataDiagsNamed(t *testing.T, name, src string) []metadataFinding {
	t.Helper()

	idx := newTestIndex()
	root := parser.New(source.New(name, []byte(src))).ParseFile()
	idx.AddDocument(name, root)
	idx.ExpandWildcardImports()

	var out []metadataFinding
	for _, d := range Analyze(name, root, nil, idx) {
		out = append(out, metadataFinding{
			Code: d.Code,
			Text: src[d.Span.Offset:d.Span.End()],
			Msg:  d.Message,
		})
	}
	return out
}

type metadataFinding struct {
	Code string
	Text string
	Msg  string
}

// findingsWithCode returns the findings carrying one code.
func findingsWithCode(found []metadataFinding, code string) []metadataFinding {
	var out []metadataFinding
	for _, f := range found {
		if f.Code == code {
			out = append(out, f)
		}
	}
	return out
}

const metadataEvaluableSrc = `package P {
	metadata def A {
		feature x;
		feature y;
	}
	feature f { in p; }
	feature a {
		@A {
			x = ~3;
			y = 1 + 2;
		}
	}
}`

// A metadata feature is a model-level element, so a value it binds must be one
// the model alone decides (KerML §7.4.9).
func TestMetadataBodyValueMustBeModelLevelEvaluable(t *testing.T) {
	found := findingsWithCode(metadataDiags(t, metadataEvaluableSrc), "metadata-value-not-evaluable")
	if len(found) != 1 {
		t.Fatalf("want one inevaluable value, got %v", found)
	}
	if found[0].Text != "= ~3" {
		t.Errorf("reported %q, want the binding and value `= ~3`", found[0].Text)
	}
	if found[0].Msg != msgFilterNotEvaluable {
		t.Errorf("message %q, want %q", found[0].Msg, msgFilterNotEvaluable)
	}
}

// An extent `all T` is decided by a run, never by the model (KerML §8.2.5.8.1
// Table 5), so a metadata value cannot state one, whatever type it names.
func TestMetadataBodyValueRejectsAnExtent(t *testing.T) {
	src := `package P {
		metadata def A { feature x; feature y; }
		datatype Color { feature red : Color; }
		feature a {
			@A {
				x = all Color;
				y = (all A) == null;
			}
		}
	}`
	found := findingsWithCode(metadataDiags(t, src), "metadata-value-not-evaluable")
	if len(found) != 2 {
		t.Fatalf("want both extent values reported, got %v", found)
	}
	if found[0].Text != "= all Color" || found[1].Text != "= (all A) == null" {
		t.Errorf("reported %q and %q, want the two extent bindings", found[0].Text, found[1].Text)
	}
}

func TestNestedMetadataBodyValueUsesBindingSpan(t *testing.T) {
	src := `package P {
		metadata def A {
			feature x { feature y; feature z; }
		}
		feature f { in p; }
		feature a {
			@A {
				x {
					y = ~3;
					z = 1 + 2;
				}
			}
		}
	}`
	found := findingsWithCode(metadataDiags(t, src), "metadata-value-not-evaluable")
	if len(found) != 1 || found[0].Text != "= ~3" {
		t.Fatalf("want one nested finding on `= ~3`, got %v", found)
	}
}

// A declaration in an annotation body restates a feature of the metadata type;
// one that restates nothing is reported where it is written.
func TestMetadataBodyFeatureMustRestateAnOwningTypeFeature(t *testing.T) {
	src := `package P {
	metadata def A { feature x; }
	feature a {
		@A {
			x = 1;
			bad;
		}
	}
}`
	found := findingsWithCode(metadataDiags(t, src), "metadata-owning-type-feature")
	if len(found) != 1 || strings.TrimSpace(found[0].Text) != "bad;" {
		t.Fatalf("want one finding on `bad;`, got %v", found)
	}
}

// The metaclass of an annotated element conforms to every type of the
// annotatedElement feature of the metadata type (KerML §8.3.4.9).
func TestMetadataAnnotatedElementMustConform(t *testing.T) {
	src := `package P {
	metaclass Only {
		:>> annotatedElement : KerML::Structure;
	}
	#Only struct Ok;
	#Only class Bad;
}`
	found := findingsWithCode(metadataDiags(t, src), "metadata-annotated-element")
	if len(found) != 1 {
		t.Fatalf("want one annotated-element finding, got %v", found)
	}
	if want := msgCannotAnnotate + "Class"; found[0].Msg != want {
		t.Errorf("message %q, want %q", found[0].Msg, want)
	}
}

// The annotatedElement feature is read through inheritance, redefinition and subsetting,
// in every spelling; a private redefinition is not inherited but its sibling subsets it.
func TestMetadataAnnotatedElementReadsEffectiveFeatures(t *testing.T) {
	src := `package P {
	metaclass M { :>> annotatedElement : KerML::Class; }
	metaclass Inherits :> M;
	metaclass Narrows :> M { :>> annotatedElement : KerML::Structure; }
	metaclass Either { :>> annotatedElement : KerML::Class; feature alt :> annotatedElement : KerML::Package; }
	metaclass Any;
	metaclass Hidden { private feature :>> annotatedElement : KerML::Class; feature alt :> annotatedElement : KerML::Package; }
	class C { feature f; }
	struct S;
	package Q;
	@M about C, C::f;
	metadata mm : Inherits about C::f;
	#Narrows class NC;
	#Narrows struct NS;
	@Either about Q, C, C::f;
	@Any about Q, C, C::f;
	@Hidden about Q, C, C::f;
}`
	var got []string
	for _, f := range findingsWithCode(metadataDiags(t, src), "metadata-annotated-element") {
		got = append(got, strings.TrimSpace(f.Text)+" => "+f.Msg)
	}
	want := []string{
		"@M about C, C::f; => Cannot annotate Feature",
		"metadata mm : Inherits about C::f; => Cannot annotate Feature",
		"#Narrows => Cannot annotate Class",
		"@Either about Q, C, C::f; => Cannot annotate Package",
		"@Either about Q, C, C::f; => Cannot annotate Feature",
		"@Hidden about Q, C, C::f; => Cannot annotate Package",
		"@Hidden about Q, C, C::f; => Cannot annotate Class",
		"@Hidden about Q, C, C::f; => Cannot annotate Feature",
	}
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("findings\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// A feature merely named annotatedElement, specializing nothing, is a duplicate
// member rather than an alternative: it neither restricts nor admits anything.
// Matches the pinned pilot validators on both spellings.
func TestMetadataAnnotatedElementIsReadByIdentityNotByName(t *testing.T) {
	kerml := `package P {
	metaclass Named { feature annotatedElement : KerML::Class; }
	metaclass Sub :> Named;
	metaclass Alt { feature annotatedElement : KerML::Class; feature other :> Metaobjects::Metaobject::annotatedElement : KerML::Package; }
	class C { feature f; }
	package Q;
	@Named about Q, C;
	@Sub about Q, C;
	@Alt about Q, C, C::f;
}`
	var got []string
	for _, f := range findingsWithCode(metadataDiags(t, kerml), "metadata-annotated-element") {
		got = append(got, strings.TrimSpace(f.Text)+" => "+f.Msg)
	}
	want := []string{
		"@Alt about Q, C, C::f; => Cannot annotate Class",
		"@Alt about Q, C, C::f; => Cannot annotate Feature",
	}
	sort.Strings(got)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("KerML findings\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	sysml := `package P {
	metadata def Named { ref annotatedElement : SysML::PartDefinition; }
	#Named part def PD;
	#Named part p;
	#Named attribute def AD;
}`
	if found := only(w8cLibraryDiagnostics(t, "meta-named.sysml", sysml), "metadata-annotated-element"); len(found) != 0 {
		t.Errorf("SysML: want no annotated-element finding, got %v", found)
	}
}

// A model declaring the library feature's qualified name does not hide the
// library feature: the restriction is still read from the bundled declaration.
func TestMetadataAnnotatedElementSurvivesAShadowingDeclaration(t *testing.T) {
	kerml := `package P {
	metaclass M { :>> annotatedElement : KerML::Class; }
	class C { feature f; }
	@M about C, C::f;
}
package Metaobjects { metaclass Metaobject { feature annotatedElement; } }`
	var got []string
	for _, f := range findingsWithCode(metadataDiags(t, kerml), "metadata-annotated-element") {
		got = append(got, strings.TrimSpace(f.Text)+" => "+f.Msg)
	}
	want := []string{"@M about C, C::f; => Cannot annotate Feature"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("findings\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// The SysML spelling reads the same feature, redefined to a SysML metaclass.
func TestMetadataAnnotatedElementInSysML(t *testing.T) {
	src := `package P {
	metadata def M { :>> annotatedElement : SysML::PartUsage; }
	part def PD;
	part p : PD;
	attribute a;
	#M part q : PD;
	@M about p;
	metadata m : M about p, a;
	#M attribute b;
}`
	var got []string
	for _, d := range only(w8cLibraryDiagnostics(t, "meta-annotated.sysml", src), "metadata-annotated-element") {
		got = append(got, strings.TrimSpace(src[d.Span.Offset:d.Span.End()])+" => "+d.Message)
	}
	want := []string{
		"metadata m : M about p, a; => Cannot annotate AttributeUsage",
		"#M => Cannot annotate AttributeUsage",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("findings\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// An explicit `:>>` in a body, nested or not, names a feature of the metadata
// type or of a type it specializes.
func TestMetadataBodyRedefinitionMustNameAnOwningTypeFeature(t *testing.T) {
	src := `package P {
	feature g;
	metaclass Base { feature inherited; }
	metaclass M :> Base {
		feature x;
		feature u { feature v; }
	}
	class C { feature own; }
	@M about C { :>> g; }
	@M about C { :>> C::own; }
	@M about C { :>> x; :>> inherited; u { :>> v; } }
	@M about C { u { :>> g; } }
}`
	var got []string
	for _, f := range findingsWithCode(metadataDiags(t, src), "metadata-owning-type-feature") {
		got = append(got, strings.TrimSpace(f.Text))
	}
	want := []string{":>> g;", ":>> C::own;", ":>> g;"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("findings %q, want %q", got, want)
	}
}

// A subject, assume or require member annotated by a prefix is checked against
// the annotatedElement of the metadata definition the way a part is: a usage-
// only definition accepts all four, a definition-only one rejects all four.
func TestMetadataAnnotatedElementOnRequirementMembers(t *testing.T) {
	const body = `package P {
	metadata def OnUsage { :>> annotatedElement : SysML::Usage; }
	metadata def OnDef { :>> annotatedElement : SysML::Definition; }
	part def Vehicle;
	constraint def C;
	requirement def R {
		subject #%[1]s v : Vehicle;
		assume #%[1]s constraint a : C;
		require #%[1]s constraint r : C;
		#%[1]s part p : Vehicle;
		assume #%[1]s constraint { true }
		require #%[1]s constraint : C { true }
	}
}`
	ok := metadataDiagsNamed(t, "<t>.sysml", fmt.Sprintf(body, "OnUsage"))
	if found := findingsWithCode(ok, "metadata-annotated-element"); len(found) != 0 {
		t.Errorf("usage-only metadata on usages: unexpected findings %v", found)
	}

	bad := findingsWithCode(metadataDiagsNamed(t, "<t>.sysml", fmt.Sprintf(body, "OnDef")), "metadata-annotated-element")
	want := []string{
		msgCannotAnnotate + "PartUsage",
		msgCannotAnnotate + "ConstraintUsage",
		msgCannotAnnotate + "ConstraintUsage",
		msgCannotAnnotate + "PartUsage",
		msgCannotAnnotate + "ConstraintUsage",
		msgCannotAnnotate + "ConstraintUsage",
	}
	if len(bad) != len(want) {
		t.Fatalf("definition-only metadata on usages: findings %v, want %d", bad, len(want))
	}
	for i, f := range bad {
		if f.Msg != want[i] || strings.TrimSpace(f.Text) != "#OnDef" {
			t.Errorf("finding %d = %q on %q, want %q on #OnDef", i, f.Msg, f.Text, want[i])
		}
	}
}

// A metadata value calling an overloaded name is judged by the overload its
// arguments select — the checker's and runtime's choice — not the first found.
func TestMetadataBodyValueSelectsTheOverloadItCalls(t *testing.T) {
	const body = `package P {
	private import ScalarValues::*;
	private import %s::*;
	private import %s::*;
	package Q {
		private import ScalarValues::*;
		function 'if' { in test : Boolean; in t : String; in f : String; return : String; }
	}
	metadata def A { feature x; feature y; }
	feature a {
		@A {
			x = 'if'(true, 1, 2);
			y = 'if'(true, "a", "b");
		}
	}
}`
	for _, imports := range [][2]string{{"ControlFunctions", "Q"}, {"Q", "ControlFunctions"}} {
		src := fmt.Sprintf(body, imports[0], imports[1])
		found := findingsWithCode(metadataDiags(t, src), "metadata-value-not-evaluable")
		if len(found) != 1 || found[0].Text != `= 'if'(true, "a", "b")` {
			t.Errorf("imports %v: findings %v, want only the call selecting Q::'if'", imports, found)
		}
	}
}

// A body value is judged in the body's own scope, where the metadata type's
// members shadow what the annotated element sees: the call and the read below
// name A's own function and feature, not the imported one and the unfoldable one.
func TestMetadataBodyValueIsJudgedInTheBodyScope(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	private import ControlFunctions::*;
	feature k = ~3;
	metadata def A {
		feature x;
		feature y;
		feature k;
		function 'if' { in test : Boolean; in t : Integer; in f : Integer; return : Integer; }
	}
	feature a {
		@A {
			x = 'if'(true, 1, 2);
			y = k;
		}
	}
}`
	found := findingsWithCode(metadataDiags(t, src), "metadata-value-not-evaluable")
	want := []string{"= 'if'(true, 1, 2)"}
	if len(found) != len(want) {
		t.Fatalf("findings %v, want %v", found, want)
	}
	for i, f := range found {
		if f.Text != want[i] {
			t.Errorf("finding %d is %q, want %q", i, f.Text, want[i])
		}
	}
}

// An annotation written in a document's root namespace annotates that namespace
// and is judged like a package member's: the pilot draws exactly these findings.
func TestMetadataAnnotationsInTheRootNamespace(t *testing.T) {
	kerml := `metaclass M { :>> annotatedElement : KerML::Class; }
metaclass P { :>> annotatedElement : KerML::Namespace; }
metaclass W { feature k : ScalarValues::Integer; }
class N;
abstract metaclass Q;
class C;
feature g : ScalarValues::Integer;
@N;
@Q;
@M;
@P;
@M about C;
@P about C;
@W { :>> k = ~3; }
@W { :>> g = 1; }
@W about C { :>> g = 1; }
@W { :>> k = 1; }
metadata m : M;
metadata p : P;
`
	sysml := strings.NewReplacer(
		"metaclass M", "metadata def M",
		"metaclass P", "metadata def P",
		"metaclass W { feature k", "metadata def W { attribute k",
		"class N", "part def N",
		"abstract metaclass Q", "abstract metadata def Q",
		"class C", "part def C",
		"feature g", "attribute g",
	).Replace(kerml)
	want := map[string][]string{
		"metadata-concrete-type":       {"@Q;"},
		"metadata-annotated-element":   {"@M;", "metadata m : M;"},
		"metadata-value-not-evaluable": {"= ~3"},
		"metadata-owning-type-feature": {":>> g = 1;", ":>> g = 1;"},
	}
	for name, src := range map[string]string{"root.kerml": kerml, "root.sysml": sysml} {
		found := metadataDiagsNamed(t, name, src)
		typed := findingsWithCode(found, "metadata-metaclass")
		if name == "root.sysml" {
			typed = nil
			for _, f := range found {
				if f.Msg == oneTypeUsageMessages[ast.UsageMetadata] {
					typed = append(typed, f)
				}
			}
		}
		if len(typed) != 1 || strings.TrimSpace(typed[0].Text) != "@N;" {
			t.Errorf("%s: metadata typing findings = %v, want one on @N;", name, typed)
		}
		for code, texts := range want {
			var got []string
			for _, f := range findingsWithCode(found, code) {
				got = append(got, strings.TrimSpace(f.Text))
			}
			if fmt.Sprint(got) != fmt.Sprint(texts) {
				t.Errorf("%s: %s on %v, want %v", name, code, got, texts)
			}
		}
	}
}

// An annotation body may carry annotations of its own metadata feature or of its
// body features, at any depth; each is judged once, in the body's scope.
func TestMetadataAnnotationsNestedInAnnotationBodies(t *testing.T) {
	kerml := `package P {
	metaclass OnlyMeta { :>> annotatedElement : KerML::MetadataFeature; }
	metaclass OnlyClass { :>> annotatedElement : KerML::Class; }
	metaclass OnlyFeature { :>> annotatedElement : KerML::Feature; }
	class T { feature b : ScalarValues::Integer; }
	metaclass W { feature k : ScalarValues::Integer; feature a : T; }
	class N;
	class C;
	feature g : ScalarValues::Integer;
	@W about C {
		@OnlyMeta;
		@OnlyClass;
		@N;
		metadata OnlyClass;
		@W { :>> k = ~3; :>> g = 1; @OnlyMeta; @OnlyClass about C; @OnlyFeature; }
		@n : W { @OnlyMeta; @OnlyFeature; }
		a { @OnlyFeature; @OnlyClass; b { @OnlyFeature; @OnlyMeta; } }
	}
	metadata m : W about C {
		@OnlyMeta;
		@OnlyClass;
		a { @OnlyFeature; @OnlyClass; @OnlyMeta; }
	}
}`
	sysml := strings.NewReplacer(
		"metaclass", "metadata def",
		"KerML::Class", "SysML::PartDefinition",
		"KerML::Feature", "SysML::Usage",
		"class T { feature b", "attribute def T { attribute b",
		"feature k", "attribute k",
		"feature a", "attribute a",
		"class N", "part def N",
		"class C", "part def C",
		"feature g", "attribute g",
	).Replace(kerml)
	want := []string{
		"metadata-annotated-element on @OnlyClass;",
		"metadata-annotated-element on @OnlyClass;",
		"metadata-annotated-element on @OnlyClass;",
		"metadata-annotated-element on @OnlyClass;",
		"metadata-annotated-element on @OnlyMeta;",
		"metadata-annotated-element on @OnlyMeta;",
		"metadata-annotated-element on metadata OnlyClass;",
		"metadata-owning-type-feature on :>> g = 1;",
		"metadata-value-not-evaluable on = ~3",
		"typing on @N;",
	}
	for name, src := range map[string]string{"nested.kerml": kerml, "nested.sysml": sysml} {
		var got []string
		for _, f := range metadataDiagsNamed(t, name, src) {
			code := f.Code
			if code == "metadata-metaclass" || f.Msg == oneTypeUsageMessages[ast.UsageMetadata] {
				code = "typing"
			}
			got = append(got, fmt.Sprintf("%s on %s", code, strings.TrimSpace(f.Text)))
		}
		sort.Strings(got)
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%s: findings\n%s\nwant\n%s", name, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}
}

// A metadata usage nested in an annotation body has its own body checked like
// one written at the top level, in either spelling of the enclosing annotation.
// `:>> i` and `i = 1` both redefine `i`, so they are duplicate owned members too;
// the one named only by its redefinition is reported at the whole member.
func TestMetadataUsagesNestedInAnnotationBodies(t *testing.T) {
	kerml := `package P {
	metaclass Outer;
	metaclass Inner { feature i : ScalarValues::Integer; }
	class C;
	feature g : ScalarValues::Integer;
	@Outer about C {
		metadata m : Inner { :>> i = ~3; zz; }
		@Inner { metadata n : Inner { :>> g = 1; } }
	}
	metadata o : Outer about C {
		@Inner { metadata q : Inner { :>> i = ~4; i = 1; } }
	}
}`
	sysml := strings.NewReplacer(
		"metaclass", "metadata def",
		"feature i", "attribute i",
		"class C", "part def C",
		"feature g", "attribute g",
	).Replace(kerml)
	want := []string{
		"metadata-body-feature on :>> g = 1;",
		"metadata-body-feature on zz;",
		"metadata-value-not-evaluable on = ~3",
		"metadata-value-not-evaluable on = ~4",
		"name-conflict on :>> i = ~4;",
		"name-conflict on i",
	}
	for name, src := range map[string]string{"nested.kerml": kerml, "nested.sysml": sysml} {
		var got []string
		for _, f := range metadataDiagsNamed(t, name, src) {
			got = append(got, fmt.Sprintf("%s on %s", f.Code, strings.TrimSpace(f.Text)))
		}
		sort.Strings(got)
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%s: findings\n%s\nwant\n%s", name, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}
}

// A body with no fault draws nothing: a value the model folds and a feature that
// restates one of the metadata type are both legal.
func TestMetadataBodyWithoutFaultsIsSilent(t *testing.T) {
	src := `package P {
	metadata def A { feature x; }
	feature a {
		@A { x = 1 + 2; }
	}
}`
	for _, f := range metadataDiags(t, src) {
		switch f.Code {
		case "metadata-value-not-evaluable", "metadata-owning-type-feature", "metadata-annotated-element":
			t.Errorf("unexpected finding %v", f)
		}
	}
}

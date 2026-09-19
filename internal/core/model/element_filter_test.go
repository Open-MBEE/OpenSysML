package model

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// filterWorkspace is a two-document workspace exercising both element-filter
// forms over the same model: a namespace whose `filter` member restricts what it
// re-exports, imports carrying their own filter clause, and a view exposing a
// filtered subtree. The second document references the filtered namespaces from
// outside, which is where the difference between "surfaced" and "hidden" shows.
const filterModelSource = `package Vehicles {
	private import ScalarValues::Boolean;

	metadata def Safety {
		attribute isMandatory : Boolean;
	}
	metadata def CrashSafety :> Safety;

	part vehicle {
		part seatBelt { @Safety{isMandatory = true;} }
		part airBag { @CrashSafety{isMandatory = false;} }
		part mirror { @Safety; }
		part keylessEntry;
	}
}

package SafetyFeatures {
	public import Vehicles::vehicle::*;
	filter @Vehicles::Safety;
}

package FilteredImport {
	public import Vehicles::vehicle::*[@Vehicles::Safety];
}

package MandatorySafety {
	public import Vehicles::vehicle::*[@Vehicles::Safety and Vehicles::Safety::isMandatory == true];
}

package OnwardSafety {
	public import SafetyFeatures::*;
}

package FurtherOnward {
	public import OnwardSafety::*;
}

package Nested {
	package SafetyInner {
		public import Vehicles::vehicle::*;
		filter @Vehicles::Safety;
	}
}

package Assemblies {
	#Vehicles::Safety package Restraints { part strap; }
	package Audio { part radio; }
}

package FilteredPackages {
	public import Assemblies::*[@Vehicles::Safety];
}

package UnqualifiedFilter {
	public import Vehicles::*;
	public import Vehicles::vehicle::*;
	filter @Safety;
}`

const filterClientSource = `package Client {
	private import SafetyFeatures::*;

	alias a for seatBelt;
	alias b for airBag;
}`

// openFilterWorkspace opens the filter model plus one client document and
// returns the client's diagnostic messages.
func openFilterWorkspace(t *testing.T, client string) []string {
	t.Helper()
	ws := NewWorkspace()
	ws.Open("file:///vehicles.sysml", []byte(filterModelSource), 1)
	ws.Open("file:///client.sysml", []byte(client), 1)
	var msgs []string
	for _, d := range ws.Diagnostics("file:///client.sysml") {
		msgs = append(msgs, d.Message)
	}
	return msgs
}

// A namespace's filters and its imports may be written in different documents;
// the filter restricts what the import brings in either way, so an unqualified
// lookup inside the importing declaration is gated like a lookup from outside.
func TestNamespaceFilterFromAnotherDocumentGatesItsImports(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("file:///vehicles.sysml", []byte(filterModelSource), 1)
	ws.Open("file:///filter.sysml", []byte("package Split { filter @Vehicles::Safety; }"), 1)
	ws.Open("file:///imports.sysml", []byte(`package Split {
		public import Vehicles::vehicle::*;
		alias a for seatBelt;
		alias c for keylessEntry;
	}`), 1)

	var msgs []string
	for _, d := range ws.Diagnostics("file:///imports.sysml") {
		msgs = append(msgs, d.Message)
	}
	if len(msgs) != 1 || !strings.Contains(msgs[0], "keylessEntry") {
		t.Fatalf("the other document's filter must hide keylessEntry and only it, got %v", msgs)
	}
}

// Completion enumerates a namespace's members through the index; a filter's
// verdict has to reach that enumeration too, or a name is offered that resolution
// then rejects.
func TestFilteredMembersAreNotOfferedForCompletion(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("file:///vehicles.sysml", []byte(filterModelSource), 1)
	ws.Open("file:///client.sysml", []byte(`package Client { }`), 1)
	doc := ws.Document("file:///client.sysml")
	if doc == nil || doc.Scope == nil {
		t.Fatal("the client document has no scope")
	}

	for _, ns := range []string{"SafetyFeatures", "FilteredImport"} {
		var names []string
		for _, sym := range ws.MembersOnPath(doc.Scope, []string{ns}) {
			names = append(names, sym.Name)
		}
		if slices.ContainsFunc(names, func(n string) bool { return strings.HasSuffix(n, "keylessEntry") }) {
			t.Errorf("%s filters keylessEntry out, so completion must not offer it, got %v", ns, names)
		}
		if !slices.ContainsFunc(names, func(n string) bool { return strings.HasSuffix(n, "seatBelt") }) {
			t.Errorf("%s admits seatBelt, which completion must offer, got %v", ns, names)
		}
	}
}

// A package is annotated by its prefix metadata like any other element, so a
// filter classifying by that metadata surfaces the tagged package and no other.
func TestFilteredImportClassifiesATaggedPackage(t *testing.T) {
	client := `package Client {
		private import FilteredPackages::*;
		alias a for Restraints::strap;
	}`
	if got := openFilterWorkspace(t, client); len(got) != 0 {
		t.Fatalf("a #Safety package must pass a filter selecting @Safety, got %v", got)
	}

	client = `package Client {
		private import FilteredPackages::*;
		alias a for Audio::radio;
	}`
	got := openFilterWorkspace(t, client)
	if len(got) != 1 || !strings.Contains(got[0], "Audio") {
		t.Fatalf("an untagged package must not pass the filter, got %v", got)
	}
}

func TestNamespaceFilterSurfacesAnnotatedMembers(t *testing.T) {
	if got := openFilterWorkspace(t, filterClientSource); len(got) != 0 {
		t.Fatalf("annotated members should stay visible through a filtered namespace, got %v", got)
	}
}

func TestNamespaceFilterHidesUnannotatedMember(t *testing.T) {
	client := `package Client {
		private import SafetyFeatures::*;
		alias c for keylessEntry;
	}`
	got := openFilterWorkspace(t, client)
	if len(got) != 1 || !strings.Contains(got[0], "keylessEntry") {
		t.Fatalf("an unannotated member must not be visible through a filtered namespace, got %v", got)
	}
}

func TestFilteredImportHidesUnannotatedMember(t *testing.T) {
	client := `package Client {
		private import FilteredImport::*;
		alias c for keylessEntry;
	}`
	got := openFilterWorkspace(t, client)
	if len(got) != 1 || !strings.Contains(got[0], "keylessEntry") {
		t.Fatalf("a filtered import must not bring in an unannotated element, got %v", got)
	}
}

// The reflective metadata types of the standard library classify an element by
// what it is, which is the form the OMG corpus filters views with.
func TestFilterOnReflectiveMetadataType(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("file:///reflective.sysml", []byte(`package Reflective {
		part def Wheel;
		part vehicle {
			part seatBelt;
		}
	}

	package ReflectiveUsages {
		public import Reflective::vehicle::*;
		filter @SysML::PartUsage;
	}

	package ReflectiveAll {
		public import Reflective::*[@SysML::PartUsage];
	}`), 1)
	for _, tc := range []struct {
		client  string
		wantErr string
	}{
		{"package C1 { private import ReflectiveUsages::*; alias a for seatBelt; }", ""},
		{"package C2 { private import ReflectiveAll::*; alias a for vehicle; }", ""},
		{"package C3 { private import ReflectiveAll::*; alias a for Wheel; }", "Wheel"},
	} {
		ws.Open("file:///client.sysml", []byte(tc.client), 1)
		var msgs []string
		for _, d := range ws.Diagnostics("file:///client.sysml") {
			msgs = append(msgs, d.Message)
		}
		switch {
		case tc.wantErr == "" && len(msgs) != 0:
			t.Fatalf("%s: unexpected diagnostics %v", tc.client, msgs)
		case tc.wantErr != "" && (len(msgs) != 1 || !strings.Contains(msgs[0], tc.wantErr)):
			t.Fatalf("%s: diagnostics %v, want one mentioning %q", tc.client, msgs, tc.wantErr)
		}
	}
}

func TestFilteredImportComparesAnnotationFeature(t *testing.T) {
	client := `package Client {
		private import MandatorySafety::*;
		alias a for seatBelt;
	}`
	if got := openFilterWorkspace(t, client); len(got) != 0 {
		t.Fatalf("the mandatory-safety element should be visible, got %v", got)
	}

	client = `package Client {
		private import MandatorySafety::*;
		alias b for airBag;
	}`
	got := openFilterWorkspace(t, client)
	if len(got) != 1 || !strings.Contains(got[0], "airBag") {
		t.Fatalf("a non-mandatory element must not pass the feature comparison, got %v", got)
	}

	// The guard decides an unannotated element without reading a feature it has
	// no annotation to read from, so it is filtered out rather than kept.
	client = `package Client {
		private import MandatorySafety::*;
		alias c for keylessEntry;
	}`
	got = openFilterWorkspace(t, client)
	if len(got) != 1 || !strings.Contains(got[0], "keylessEntry") {
		t.Fatalf("an unannotated element must not pass a guarded filter, got %v", got)
	}
}

// A feature the annotation binds nothing to has no value, so the comparison
// reading it does not hold and the element is not surfaced — while a condition
// only testing the annotation still surfaces it.
func TestFilteredImportRejectsAnUnboundAnnotationFeature(t *testing.T) {
	client := `package Client {
		private import FilteredImport::*;
		alias a for mirror;
	}`
	if got := openFilterWorkspace(t, client); len(got) != 0 {
		t.Fatalf("an annotated element should pass a filter testing only the annotation, got %v", got)
	}

	client = `package Client {
		private import MandatorySafety::*;
		alias a for mirror;
	}`
	got := openFilterWorkspace(t, client)
	if len(got) != 1 || !strings.Contains(got[0], "mirror") {
		t.Fatalf("an element binding no value to the compared feature must not pass, got %v", got)
	}
}

// A filtered element is gone from every route to it, not merely absent from the
// unqualified one: naming it under the filtering namespace does not resolve
// either.
func TestFilteredElementIsUnresolvableQualified(t *testing.T) {
	client := `package Client {
		alias c for SafetyFeatures::keylessEntry;
		alias d for FilteredImport::keylessEntry;
	}`
	got := openFilterWorkspace(t, client)
	if len(got) != 2 {
		t.Fatalf("a filtered element must not resolve under a qualified name either, got %v", got)
	}
	for _, msg := range got {
		if !strings.Contains(msg, "keylessEntry") {
			t.Fatalf("diagnostics %v should both be about keylessEntry", got)
		}
	}

	// The elements the filters admit do resolve under the same qualified route,
	// so the restriction is the filter's verdict and not the route itself.
	client = `package Client {
		alias a for SafetyFeatures::seatBelt;
		alias b for FilteredImport::airBag;
	}`
	if got := openFilterWorkspace(t, client); len(got) != 0 {
		t.Fatalf("admitted elements should resolve qualified, got %v", got)
	}
}

// A filter keeps holding when a further namespace imports the filtering one
// onward: what the filtered namespace re-exports is what any chain of imports of
// it carries, however many hops long.
func TestNamespaceFilterHoldsThroughChainedImports(t *testing.T) {
	for _, route := range []string{"OnwardSafety", "FurtherOnward"} {
		client := `package Client {
			private import ` + route + `::*;
			alias c for keylessEntry;
			alias d for ` + route + `::keylessEntry;
		}`
		got := openFilterWorkspace(t, client)
		if len(got) != 2 {
			t.Fatalf("%s: a filtered element must stay hidden through an onward import, got %v", route, got)
		}

		client = `package Client {
			private import ` + route + `::*;
			alias a for seatBelt;
			alias b for ` + route + `::seatBelt;
		}`
		if got := openFilterWorkspace(t, client); len(got) != 0 {
			t.Fatalf("%s: an admitted element should still resolve onward, got %v", route, got)
		}
	}
}

// A qualified name reaches a filtering namespace at any depth, not only as the
// second segment of the name.
func TestFilterAppliesToDeeplyQualifiedRoute(t *testing.T) {
	client := `package Client {
		alias c for Nested::SafetyInner::keylessEntry;
	}`
	got := openFilterWorkspace(t, client)
	if len(got) != 1 || !strings.Contains(got[0], "keylessEntry") {
		t.Fatalf("a filtered element must not resolve through a nested qualified route, got %v", got)
	}

	client = `package Client {
		alias a for Nested::SafetyInner::seatBelt;
	}`
	if got := openFilterWorkspace(t, client); len(got) != 0 {
		t.Fatalf("an admitted element should resolve through a nested qualified route, got %v", got)
	}
}

// The metadata type a condition names is resolved from the namespace the filter
// is written in, so an imported type may be named unqualified.
func TestFilterNamesMetadataTypeThroughAnImport(t *testing.T) {
	client := `package Client {
		private import UnqualifiedFilter::*;
		alias c for keylessEntry;
	}`
	got := openFilterWorkspace(t, client)
	if len(got) != 1 || !strings.Contains(got[0], "keylessEntry") {
		t.Fatalf("a filter naming an imported metadata type must apply, got %v", got)
	}

	client = `package Client {
		private import UnqualifiedFilter::*;
		alias a for seatBelt;
	}`
	if got := openFilterWorkspace(t, client); len(got) != 0 {
		t.Fatalf("the annotated element should pass a filter naming an imported type, got %v", got)
	}

	// The condition's own names are not subject to the condition: a namespace's
	// filter restricts what it re-exports, not what resolves in its body.
	ws := NewWorkspace()
	ws.Open("file:///vehicles.sysml", []byte(filterModelSource), 1)
	for _, d := range ws.Diagnostics("file:///vehicles.sysml") {
		if strings.Contains(d.Message, "filter condition") {
			t.Fatalf("a filter naming an imported type should be evaluable, got %q", d.Message)
		}
	}
}

// A view's `expose` is an import, and its filter restricts what the view
// surfaces: the recursive form walks the subtree and keeps the matching subset.
func TestFilteredExposeSurfacesSubset(t *testing.T) {
	client := `package Client {
		view safetyView {
			expose Vehicles::vehicle::**[@Vehicles::Safety];
			alias a for seatBelt;
			alias b for airBag;
		}
	}`
	if got := openFilterWorkspace(t, client); len(got) != 0 {
		t.Fatalf("a filtered expose should surface the matching elements, got %v", got)
	}

	client = `package Client {
		view safetyView {
			expose Vehicles::vehicle::**[@Vehicles::Safety];
			alias c for keylessEntry;
		}
	}`
	got := openFilterWorkspace(t, client)
	if len(got) != 1 || !strings.Contains(got[0], "keylessEntry") {
		t.Fatalf("a filtered expose must not surface a non-matching element, got %v", got)
	}

	// Unfiltered, the same expose surfaces the whole subtree, which is what makes
	// the filtered case a strict subset.
	client = `package Client {
		view allView {
			expose Vehicles::vehicle::**;
			alias c for keylessEntry;
		}
	}`
	if got := openFilterWorkspace(t, client); len(got) != 0 {
		t.Fatalf("an unfiltered expose should surface the whole subtree, got %v", got)
	}
}

// A `filter` member of a definition or usage body restricts what the imports
// declared beside it bring into that body — the form the corpus writes in a view
// definition to narrow what its views expose (SysML v2 7.4.4).
func TestFilterMemberOfADefinitionBodyRestrictsItsImports(t *testing.T) {
	src := `package Meta { metadata def Safety; }
	package Vehicles {
		part vehicle {
			#Meta::Safety part seatBelt;
			part keylessEntry;
		}
	}
	package Views {
		view safetyView {
			expose Vehicles::vehicle::*;
			filter @Meta::Safety;
			alias a for seatBelt;
		}
		view rejectingView {
			expose Vehicles::vehicle::*;
			filter @Meta::Safety;
			alias b for keylessEntry;
		}
	}`
	ws := NewWorkspace()
	ws.Open("file:///v.sysml", []byte(src), 1)
	var msgs []string
	for _, d := range ws.Diagnostics("file:///v.sysml") {
		msgs = append(msgs, d.Message)
	}
	if len(msgs) != 1 || !strings.Contains(msgs[0], "keylessEntry") {
		t.Fatalf("a view's filter should surface only the annotated element, got %v", msgs)
	}
}

// The metadata type a condition names resolves through the namespace's own
// imports, which the condition does not filter.
func TestFilterConditionResolvesThroughTheImportsItFilters(t *testing.T) {
	src := `package Meta { metadata def Safety; }
	package Vehicles {
		part vehicle {
			#Meta::Safety part seatBelt;
			part keylessEntry;
		}
	}
	package Facade {
		private import Meta::*;
		public import Vehicles::vehicle::*;
		filter @Safety;
		alias a for seatBelt;
	}`
	ws := NewWorkspace()
	ws.Open("file:///f.sysml", []byte(src), 1)
	for _, d := range ws.Diagnostics("file:///f.sysml") {
		t.Fatalf("a condition naming an imported metadata type should hold: %s", d.Message)
	}
}

// Evaluating a filter reaches library declarations, whose own unresolved
// references belong to the library, not to the document whose filter reached
// them — so a cold stdlib cache reports exactly what a warm one does.
func TestFilterEvaluationReportsOnlyThisDocument(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	src := `package Meta {
		private import ScalarValues::Boolean;
		metadata def Safety { attribute isMandatory : Boolean; }
	}

	package Vehicles {
		part vehicle {
			#Meta::Safety part seatBelt;
			part keylessEntry;
		}
	}

	package Views {
		view safetyView {
			expose Vehicles::vehicle::**[@Meta::Safety];
			alias c for keylessEntry;
		}
	}`
	var first []string
	for run := 1; run <= 2; run++ { // run 1 parses the stdlib, run 2 restores it
		ws := NewWorkspace()
		ws.Open("file:///a.sysml", []byte(src), 1)
		var msgs []string
		for _, d := range ws.Diagnostics("file:///a.sysml") {
			if int(d.Span.Offset) > len(src) {
				t.Fatalf("run %d: diagnostic %q spans offset %d, past the end of the document", run, d.Message, d.Span.Offset)
			}
			msgs = append(msgs, d.Message)
		}
		if run == 1 {
			first = msgs
			continue
		}
		if !slices.Equal(msgs, first) {
			t.Fatalf("a warm stdlib cache reports %v, a cold one %v", msgs, first)
		}
	}
}

// A library's filters decide the same way whether its records were parsed or
// restored from the symbol cache, so a workspace's second run over an unchanged
// library resolves exactly the names its first one did.
func TestFilteredLibraryIsCacheStateIndependent(t *testing.T) {
	libDir := t.TempDir()
	lib := `package Meta {
	metadata def Safety;
}

package Vehicles {
	part vehicle {
		part seatBelt { @Meta::Safety; }
		part keylessEntry;
	}
}

package SafeReexport {
	public import Vehicles::vehicle::*;
	filter @Meta::Safety;
}

package Nested {
	package SafeInner {
		public import Vehicles::vehicle::*;
		filter @Meta::Safety;
	}
}
`
	if err := os.WriteFile(filepath.Join(libDir, "lib.sysml"), []byte(lib), 0o600); err != nil {
		t.Fatalf("write library: %v", err)
	}
	t.Setenv(libs.LibraryPathEnvVar, libDir)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	client := `package C {
		private import SafeReexport::*;
		alias a for keylessEntry;
		alias b for SafeReexport::keylessEntry;
		alias c for SafeReexport::seatBelt;
		alias d for Nested::SafeInner::keylessEntry;
		alias e for Nested::SafeInner::seatBelt;
	}`
	var first []string
	for run := 1; run <= 3; run++ { // run 1 parses the library, later runs restore it
		ws := NewWorkspace()
		ws.Open("file:///c.sysml", []byte(client), 1)
		var msgs []string
		for _, d := range ws.Diagnostics("file:///c.sysml") {
			msgs = append(msgs, d.Message)
		}
		if run == 1 {
			if len(msgs) != 3 {
				t.Fatalf("run 1: want the three routes to keylessEntry filtered out, got %v", msgs)
			}
			for _, msg := range msgs {
				if !strings.Contains(msg, "keylessEntry") {
					t.Fatalf("run 1: diagnostics %v should all be about keylessEntry", msgs)
				}
			}
			first = msgs
			continue
		}
		if !slices.Equal(msgs, first) {
			t.Fatalf("run %d over a cache-restored library reports %v, but %v when parsed", run, msgs, first)
		}
	}
}

func TestAFileLevelFilteredImportHidesWhatItRejects(t *testing.T) {
	lib := `package Lib { metadata def Safety; part def Radio; #Safety part def Belt; }`
	src := `private import Lib::*[@Lib::Safety];
alias x for Radio;
alias y for Belt;`
	ws := NewWorkspace()
	ws.Open("file:///lib.sysml", []byte(lib), 1)
	ws.Open("file:///a.sysml", []byte(src), 1)
	var msgs []string
	for _, d := range ws.Diagnostics("file:///a.sysml") {
		msgs = append(msgs, d.Message)
	}
	if len(msgs) != 1 || !strings.Contains(msgs[0], "Radio") {
		t.Fatalf("an import filter at a document's root should hide only Radio, got %v", msgs)
	}
}

func TestARootFilterRestrictsOnlyItsOwnDocument(t *testing.T) {
	lib := `package Lib { metadata def Safety; part def Radio; #Safety part def Belt; }`
	filtered := `private import Lib::*;
filter @Lib::Safety;
alias inA for Belt;`
	other := `private import Lib::*;
alias inB for Radio;`
	ws := NewWorkspace()
	ws.Open("file:///lib.sysml", []byte(lib), 1)
	ws.Open("file:///a.sysml", []byte(filtered), 1)
	ws.Open("file:///b.sysml", []byte(other), 1)
	for _, doc := range []string{"file:///a.sysml", "file:///b.sysml"} {
		for _, d := range ws.Diagnostics(doc) {
			t.Fatalf("%s: one document's root filter should not restrict another's imports: %s", doc, d.Message)
		}
	}
	unresolvable := `private import Lib::*;
filter @Lib::Safety;
alias inA for Radio;`
	ws.Open("file:///a.sysml", []byte(unresolvable), 2)
	var msgs []string
	for _, d := range ws.Diagnostics("file:///a.sysml") {
		msgs = append(msgs, d.Message)
	}
	if len(msgs) != 1 || !strings.Contains(msgs[0], "Radio") {
		t.Fatalf("a root filter should still restrict its own document's imports, got %v", msgs)
	}
}

// A condition may name a metadata type the import it gates is what brings in, so
// the condition's own names have to be resolved without applying it to itself.
func TestAnImportFilterMayNameAMetadataTypeThatImportSurfaces(t *testing.T) {
	lib := `package Lib { metadata def Safety; part def Radio; #Safety part def Belt; }`
	for name, src := range map[string]string{
		"in a package": `package P { private import Lib::*[@Safety]; alias x for Radio; alias y for Belt; }`,
		"at the root": `private import Lib::*[@Safety];
alias x for Radio;
alias y for Belt;`,
	} {
		t.Run(name, func(t *testing.T) {
			ws := NewWorkspace()
			ws.Open("file:///lib.sysml", []byte(lib), 1)
			ws.Open("file:///a.sysml", []byte(src), 1)
			var msgs []string
			for _, d := range ws.Diagnostics("file:///a.sysml") {
				msgs = append(msgs, d.Message)
			}
			if len(msgs) != 1 || !strings.Contains(msgs[0], "unresolved reference: Radio") {
				t.Fatalf("the filter should resolve Safety through its own import and hide only Radio, got %v", msgs)
			}
		})
	}
}

func TestAnUnresolvedNameInAnImportFilterIsReported(t *testing.T) {
	lib := `package Lib { metadata def Safety; part def Radio; }`
	src := `package P { private import Lib::*[@NoSuchMeta]; alias x for Radio; }`
	ws := NewWorkspace()
	ws.Open("file:///lib.sysml", []byte(lib), 1)
	ws.Open("file:///a.sysml", []byte(src), 1)
	var msgs []string
	for _, d := range ws.Diagnostics("file:///a.sysml") {
		msgs = append(msgs, d.Message)
	}
	if len(msgs) != 1 || !strings.Contains(msgs[0], "unresolved reference: NoSuchMeta") {
		t.Fatalf("a typo in an import's filter clause should be an unresolved reference, got %v", msgs)
	}
}

// An operator whose result is Boolean whatever its operands are draws no
// evaluability fault from an operand a lower tier could not resolve.
func TestAnUnresolvedOperandOfABooleanFilterYieldsOnlyTheUnresolvedReference(t *testing.T) {
	for _, cond := range []string{
		"@Safe and Undefined", "Undefined1 and Undefined2", "not Undefined",
		"Undefined == 1", "Undefined < 1", "Undefined != Undefined2",
		"Undefined istype Safe", "Undefined hastype Safe",
	} {
		src := "package P { metadata def Safe; filter " + cond + "; }"
		ws := NewWorkspace()
		ws.Open("file:///a.sysml", []byte(src), 1)
		for _, d := range ws.Diagnostics("file:///a.sysml") {
			if !strings.Contains(d.Message, "unresolved reference") {
				t.Errorf("filter %s: an unresolved operand is the whole fault, also got %q", cond, d.Message)
			}
		}
	}
}

// A classification test written without its type parses to an operator with no
// type reference: the syntax error is the whole fault, as the pilot has it.
func TestAClassificationFilterMissingItsTypeIsDiagnosedNotFatal(t *testing.T) {
	for _, cond := range []string{"x istype ", "x hastype ", "@", "@@"} {
		src := "package P { filter " + cond + "; }"
		ws := NewWorkspace()
		ws.Open("file:///a.sysml", []byte(src), 1)
		diags := ws.Diagnostics("file:///a.sysml")
		if len(diags) == 0 {
			t.Errorf("filter %s: malformed condition reported nothing", cond)
		}
		for _, d := range diags {
			if strings.Contains(d.Message, "model-level evaluable") || strings.Contains(d.Message, "Boolean result") {
				t.Errorf("filter %s: a missing type is a syntax fault, also got %q", cond, d.Message)
			}
		}
	}
}

// TestEditorResolutionMatchesDiagnosticsForARootImport locks the editor read
// path (go-to-definition, hover, rename) to the same verdict the diagnostics
// give: a document builds its own scope tree, whose document the index does not
// know by identity, and it needs a semantic model to judge a filter at all.
func TestEditorResolutionMatchesDiagnosticsForARootImport(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("file:///vehicles.sysml", []byte(filterModelSource), 1)
	ws.Open("file:///app.sysml", []byte("import Vehicles::vehicle::*[@Vehicles::Safety];\npart a :> seatBelt;"), 1)
	doc := ws.Document("file:///app.sysml")
	if doc == nil || doc.Scope == nil {
		t.Fatal("the document has no scope")
	}
	for name, want := range map[string]bool{"seatBelt": true, "keylessEntry": false} {
		qn := &ast.QualifiedName{Parts: []ast.NameSegment{{Text: name}}}
		if _, ok := ws.ResolveQualifiedInDoc("file:///app.sysml", doc.Scope, qn); ok != want {
			t.Errorf("the root import resolves %s in the editor = %v, want %v", name, ok, want)
		}
	}
}

// A condition reading a feature through its metadata type (`Safety::isMandatory`)
// has to recognise that type whether the library declaring it was parsed or
// restored: a restored symbol has no owning scope, only an indexed name.
func TestAFeatureComparisonIsCacheStateIndependent(t *testing.T) {
	libDir := t.TempDir()
	lib := `package Meta {
	private import ScalarValues::Boolean;
	metadata def Safety { attribute isMandatory : Boolean; }
}

package Vehicles {
	part vehicle {
		part seatBelt { @Meta::Safety{isMandatory = true;} }
		part mirror { @Meta::Safety{isMandatory = false;} }
	}
}
`
	if err := os.WriteFile(filepath.Join(libDir, "lib.sysml"), []byte(lib), 0o600); err != nil {
		t.Fatalf("write library: %v", err)
	}
	t.Setenv(libs.LibraryPathEnvVar, libDir)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	client := `package C {
		private import Vehicles::vehicle::*[@Meta::Safety and Meta::Safety::isMandatory == true];
		alias a for seatBelt;
		alias b for mirror;
	}`
	var first []string
	for run := 1; run <= 3; run++ { // run 1 parses the library, later runs restore it
		ws := NewWorkspace()
		ws.Open("file:///c.sysml", []byte(client), 1)
		var msgs []string
		for _, d := range ws.Diagnostics("file:///c.sysml") {
			msgs = append(msgs, d.Message)
		}
		if run == 1 {
			if len(msgs) != 1 || !strings.Contains(msgs[0], "mirror") {
				t.Fatalf("run 1: the comparison should hide mirror and only it, got %v", msgs)
			}
			first = msgs
			continue
		}
		if !slices.Equal(msgs, first) {
			t.Fatalf("run %d over a cache-restored library reports %v, but %v when parsed", run, msgs, first)
		}
	}
}

// Completion enumerates a document's root-level names, which a filtered import
// and another document's import both narrow: it must offer only what resolution
// then admits.
func TestCompletionOffersOnlyAdmittedRootNames(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("file:///lib.sysml", []byte(`package Lib {
	metadata def Safety;
	part seatBelt { @Safety; }
	part radio;
}`), 1)
	ws.Open("file:///a.sysml", []byte("private import Lib::*[@Lib::Safety];\npart x;"), 1)
	ws.Open("file:///b.sysml", []byte("part y;"), 1)

	for doc, want := range map[string]map[string]bool{
		"file:///a.sysml": {"seatBelt": true, "radio": false},
		"file:///b.sysml": {"seatBelt": false, "radio": false},
	} {
		offered := make(map[string]bool)
		for _, sym := range ws.TopLevelSymbols(doc) {
			offered[sym.Name] = true
		}
		for name, ok := range want {
			if offered[name] != ok {
				t.Errorf("%s offers %s = %v, want %v", doc, name, offered[name], ok)
			}
		}
	}
}

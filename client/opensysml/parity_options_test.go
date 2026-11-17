package opensysml_test

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

const librarySecondSource = "package P { private import SI::*; attribute time = 20 [s]; }\n"

func apiJSONElements(t *testing.T, content string) []map[string]any {
	t.Helper()
	var elements []map[string]any
	if err := json.Unmarshal([]byte(content), &elements); err != nil {
		t.Fatalf("API JSON content does not parse: %v\n%s", err, content)
	}
	return elements
}

// A conversion carries owned structure, and WithInheritedMembers adds the inherited lists.
func TestConvertWritesInheritedMembersOnRequest(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()
	const source = "package P { part def A { attribute x; } part def B :> A; }"
	inherits := func(content string) bool {
		for _, element := range apiJSONElements(t, content) {
			if _, ok := element["inheritedFeature"]; ok {
				return true
			}
		}
		return false
	}
	owned, err := client.ConvertSource(ctx, source, opensysml.FormatAPIJSON, opensysml.WithFromFormat(opensysml.FormatSysML))
	if err != nil {
		t.Fatalf("ConvertSource: %v", err)
	}
	complete, err := client.ConvertSource(ctx, source, opensysml.FormatAPIJSON,
		opensysml.WithFromFormat(opensysml.FormatSysML), opensysml.WithInheritedMembers())
	if err != nil {
		t.Fatalf("ConvertSource with inherited members: %v", err)
	}
	if inherits(owned.Content) || !inherits(complete.Content) {
		t.Errorf("inheritedFeature written: %v by default, %v with WithInheritedMembers", inherits(owned.Content), inherits(complete.Content))
	}
}

func TestConvertNamesLibraryReferencesAndRestoresStubsOnRequest(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()
	plain, err := client.ConvertSource(ctx, librarySecondSource, opensysml.FormatAPIJSON, opensysml.WithFromFormat(opensysml.FormatSysML))
	if err != nil {
		t.Fatalf("ConvertSource: %v", err)
	}
	index := slices.IndexFunc(plain.LibraryReferences, func(ref opensysml.LibraryReference) bool {
		return ref.QualifiedName == "SI::second"
	})
	if index < 0 {
		t.Fatalf("library references %+v name no SI::second", plain.LibraryReferences)
	}
	second := plain.LibraryReferences[index]
	if second.ID != "0c539f7c-20a1-520d-bae0-f3f855517aea" || second.Type != "AttributeUsage" {
		t.Errorf("SI::second = %+v; want its normative id and AttributeUsage", second)
	}
	for _, element := range apiJSONElements(t, plain.Content) {
		if element["isLibraryElement"] == true {
			t.Errorf("a library stub %v is written without WithLibraryStubs", element["@id"])
		}
	}

	legacy, err := client.ConvertSource(ctx, librarySecondSource, opensysml.FormatAPIJSON,
		opensysml.WithFromFormat(opensysml.FormatSysML), opensysml.WithLibraryStubs())
	if err != nil {
		t.Fatalf("ConvertSource with library stubs: %v", err)
	}
	stubs := apiJSONElements(t, legacy.Content)
	stub := slices.IndexFunc(stubs, func(element map[string]any) bool { return element["@id"] == second.ID })
	if stub < 0 {
		t.Fatalf("WithLibraryStubs wrote no stub for %s", second.ID)
	}
	if stubs[stub]["qualifiedName"] != "SI::second" || stubs[stub]["isLibraryElement"] != true {
		t.Errorf("stub = %v; want SI::second marked as a library element", stubs[stub])
	}
	if !slices.Equal(legacy.LibraryReferences, plain.LibraryReferences) {
		t.Errorf("library references with stubs = %+v, want the same as without %+v", legacy.LibraryReferences, plain.LibraryReferences)
	}

	notation, err := client.ConvertSource(ctx, librarySecondSource, opensysml.FormatSysML, opensysml.WithFromFormat(opensysml.FormatSysML))
	if err != nil {
		t.Fatalf("ConvertSource to notation: %v", err)
	}
	if notation.LibraryReferences != nil {
		t.Errorf("a notation conversion reports library references %+v", notation.LibraryReferences)
	}
}

func TestConvertSpellsDerivedIDsInTheRequestedForm(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()
	uuid := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	source := "package Demo { part def Car; }\n"
	idsOf := func(form opensysml.IDForm) []string {
		t.Helper()
		opts := []opensysml.ConvertOption{opensysml.WithFromFormat(opensysml.FormatSysML)}
		if form != "" {
			opts = append(opts, opensysml.WithIDForm(form))
		}
		conversion, err := client.ConvertSource(ctx, source, opensysml.FormatAPIJSON, opts...)
		if err != nil {
			t.Fatalf("ConvertSource(%q): %v", form, err)
		}
		var ids []string
		for _, element := range apiJSONElements(t, conversion.Content) {
			if element["qualifiedName"] == "Demo::Car" {
				ids = append(ids, element["@id"].(string))
			}
		}
		if len(ids) != 1 {
			t.Fatalf("ConvertSource(%q): Demo::Car appears %d times", form, len(ids))
		}
		return ids
	}
	asUUID, qualified, defaulted := idsOf(opensysml.IDFormUUID), idsOf(opensysml.IDFormQualified), idsOf("")
	if !uuid.MatchString(asUUID[0]) {
		t.Errorf("IDFormUUID spelled Demo::Car as %q, want a uuid", asUUID[0])
	}
	if qualified[0] == asUUID[0] {
		t.Errorf("IDFormQualified spelled Demo::Car as %q, the same as IDFormUUID", qualified[0])
	}
	if defaulted[0] != qualified[0] {
		t.Errorf("the default id form spelled Demo::Car as %q, want the qualified form %q", defaulted[0], qualified[0])
	}

	_, err := client.ConvertSource(ctx, source, opensysml.FormatSysML,
		opensysml.WithFromFormat(opensysml.FormatSysML), opensysml.WithIDForm(opensysml.IDFormUUID))
	var status *opensysml.StatusError
	if !errors.As(err, &status) || status.Code != opensysml.CodeInvalidArgument {
		t.Errorf("WithIDForm on a notation target: %v, want CodeInvalidArgument", err)
	}
}

const (
	baseLibrarySource = "package Lib { part def Wheel; }\n"
	baseTopSource     = "package Top { private import Lib::*; part w : Wheel; }\n"
)

func TestParseDocumentsRelatesAModelToItsBase(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()
	documents := func(top string) []opensysml.Document {
		return []opensysml.Document{
			opensysml.Source("lib.sysml", baseLibrarySource),
			opensysml.Source("top.sysml", top),
		}
	}
	// The service relates two models document by document once it has seen
	// their set more than once; the first parse of a set is answered fresh.
	if _, err := client.ParseDocuments(ctx, documents(baseTopSource+"// first\n")); err != nil {
		t.Fatalf("ParseDocuments: %v", err)
	}
	base, err := client.ParseDocuments(ctx, documents(baseTopSource))
	if err != nil {
		t.Fatalf("ParseDocuments: %v", err)
	}
	if base.Affected != nil {
		t.Errorf("a parse without a base reports affected documents %q", base.Affected)
	}
	edited, err := client.ParseDocuments(ctx, documents(strings.Replace(baseTopSource, "part w", "part wheel", 1)),
		opensysml.WithBaseModel(base))
	if err != nil {
		t.Fatalf("ParseDocuments with a base: %v", err)
	}
	if !slices.Contains(edited.Affected, "top.sysml") {
		t.Errorf("affected = %q, want the edited top.sysml", edited.Affected)
	}
	if slices.Contains(edited.Affected, "lib.sysml") {
		t.Errorf("affected = %q; lib.sysml did not change", edited.Affected)
	}
	if edited.Hash == base.Hash {
		t.Error("an edited document set kept the base model's hash")
	}

	var status *opensysml.StatusError
	_, err = client.ParseSource(ctx, baseTopSource, opensysml.WithBaseModel(base))
	if !errors.As(err, &status) || status.Code != opensysml.CodeInvalidArgument {
		t.Errorf("ParseSource with a base: %v, want CodeInvalidArgument", err)
	}
	_, err = client.ParseFile(ctx, "top.sysml", opensysml.WithBaseModel(base))
	if !errors.As(err, &status) || status.Code != opensysml.CodeInvalidArgument {
		t.Errorf("ParseFile with a base: %v, want CodeInvalidArgument", err)
	}
	same, err := client.ParseDocuments(ctx, documents(baseTopSource), opensysml.WithBaseModel(nil))
	if err != nil {
		t.Fatalf("ParseDocuments with a nil base: %v", err)
	}
	if same.Affected != nil {
		t.Errorf("a nil base reports affected documents %q", same.Affected)
	}
}

func TestImportDataNamesTheImportForm(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, importCompareVehicles)
	ctx := context.Background()
	imports := []opensysml.DataImport{{Name: "fleet.tsv", Content: "element\tcount\nV::vehicle\t7\n"}}
	result, err := client.ImportData(ctx, model, imports, opensysml.ImportAs(opensysml.ImportAsValues), opensysml.ImportDryRun())
	if err != nil {
		t.Fatalf("ImportData as values: %v", err)
	}
	if len(result.Files) != 1 || len(result.Files[0].Assignments) != 1 {
		t.Errorf("import as values = %+v; want the one assignment", result)
	}
	_, err = client.ImportData(ctx, model, imports, opensysml.ImportAs("rows"))
	var status *opensysml.StatusError
	if !errors.As(err, &status) || status.Code != opensysml.CodeInvalidArgument || !strings.Contains(status.Message, "rows") {
		t.Errorf("an unsupported import form: %v, want CodeInvalidArgument naming rows", err)
	}
}

// WithDocuments writes only the named documents of a model, keeping the references into the others.
func TestConvertWritesOnlyTheDocumentsNamed(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()
	model, err := client.ParseDocuments(ctx, []opensysml.Document{
		opensysml.Source("lib.sysml", "package Lib { part def Engine; }"),
		opensysml.Source("app.sysml", "package App { private import Lib::*; part engine : Engine; }"),
	})
	if err != nil {
		t.Fatalf("ParseDocuments: %v", err)
	}
	ids := func(opts ...opensysml.ConvertOption) []string {
		t.Helper()
		out, err := client.Convert(ctx, model, opensysml.FormatAPIJSON, opts...)
		if err != nil {
			t.Fatalf("Convert: %v", err)
		}
		var written []string
		for _, element := range apiJSONElements(t, out.Content) {
			id, _ := element["@id"].(string)
			written = append(written, id)
			if typed, _ := element["type"].(map[string]any); element["@type"] == "FeatureTyping" && typed["@id"] == "Lib__Engine" {
				written = append(written, "typed by Lib__Engine")
			}
		}
		return written
	}
	whole, app, lib := ids(), ids(opensysml.WithDocuments("app.sysml")), ids(opensysml.WithDocuments("lib.sysml"))
	if !slices.Contains(whole, "App__engine") || !slices.Contains(whole, "Lib__Engine") {
		t.Errorf("the whole model writes %q, want both documents", whole)
	}
	if !slices.Contains(app, "App__engine") || slices.Contains(app, "Lib__Engine") {
		t.Errorf("app.sysml alone writes %q, want App::engine and not Lib::Engine", app)
	}
	if !slices.Contains(app, "typed by Lib__Engine") {
		t.Errorf("app.sysml alone writes %q; App::engine lost its typing by Lib::Engine", app)
	}
	if !slices.Contains(lib, "Lib__Engine") || slices.Contains(lib, "App__engine") {
		t.Errorf("lib.sysml alone writes %q, want Lib::Engine and not App::engine", lib)
	}
	if len(app)-1+len(lib) != len(whole)-1 {
		t.Errorf("the documents write %d and %d elements apart, %d together", len(app)-1, len(lib), len(whole)-1)
	}
	if every := ids(opensysml.WithDocuments()); !slices.Equal(every, whole) {
		t.Errorf("WithDocuments() writes %q, want every document: %q", every, whole)
	}

	var status *opensysml.StatusError
	_, err = client.Convert(ctx, model, opensysml.FormatAPIJSON, opensysml.WithDocuments("absent.sysml"))
	if !errors.As(err, &status) || status.Code != opensysml.CodeInvalidArgument || !strings.Contains(status.Message, "absent.sysml") {
		t.Errorf("a document the model does not hold: %v, want CodeInvalidArgument naming it", err)
	}
	_, err = client.ConvertSource(ctx, "package P;", opensysml.FormatAPIJSON,
		opensysml.WithFromFormat(opensysml.FormatSysML), opensysml.WithDocuments("p.sysml"))
	if !errors.As(err, &status) || status.Code != opensysml.CodeInvalidArgument {
		t.Errorf("WithDocuments on inline content: %v, want CodeInvalidArgument", err)
	}
}

// WithCompact writes the compact document, and WithOmitDerived trims it to all but the kept derived properties.
func TestConvertWritesTheCompactDocumentOnRequest(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()
	const source = "package P { part def A { attribute x; } part def B :> A; }"
	from := opensysml.WithFromFormat(opensysml.FormatSysML)
	type compact struct {
		Format   string           `json:"format"`
		Elements []map[string]any `json:"elements"`
	}
	convert := func(opts ...opensysml.ConvertOption) (compact, int) {
		t.Helper()
		out, err := client.ConvertSource(ctx, source, opensysml.FormatAPIJSON, append([]opensysml.ConvertOption{from}, opts...)...)
		if err != nil {
			t.Fatalf("ConvertSource: %v", err)
		}
		var document compact
		if err := json.Unmarshal([]byte(out.Content), &document); err != nil {
			t.Fatalf("the compact content does not parse: %v\n%s", err, out.Content)
		}
		return document, len(out.Content)
	}
	writes := func(document compact, key string) bool {
		return slices.ContainsFunc(document.Elements, func(element map[string]any) bool {
			_, ok := element[key]
			return ok
		})
	}
	array, err := client.ConvertSource(ctx, source, opensysml.FormatAPIJSON, from)
	if err != nil {
		t.Fatalf("ConvertSource: %v", err)
	}
	full, fullSize := convert(opensysml.WithCompact())
	if full.Format != "api-json-compact/1" || len(full.Elements) != len(apiJSONElements(t, array.Content)) {
		t.Errorf("WithCompact wrote format %q with %d elements, want api-json-compact/1 with the element array's %d",
			full.Format, len(full.Elements), len(apiJSONElements(t, array.Content)))
	}
	if !writes(full, "qualifiedName") || !writes(full, "owner") {
		t.Error("the compact document without WithOmitDerived lacks a derived property")
	}
	trimmed, trimmedSize := convert(opensysml.WithCompact(), opensysml.WithOmitDerived())
	if writes(trimmed, "qualifiedName") || writes(trimmed, "owner") || !writes(trimmed, "declaredName") {
		t.Error("WithOmitDerived left a derived property or dropped an owned one")
	}
	kept, keptSize := convert(opensysml.WithCompact(), opensysml.WithOmitDerived("qualifiedName"))
	if !writes(kept, "qualifiedName") || writes(kept, "owner") {
		t.Error(`WithOmitDerived("qualifiedName") did not keep exactly the named property`)
	}
	if !(trimmedSize < keptSize && keptSize < fullSize && fullSize < len(array.Content)) {
		t.Errorf("sizes: element array %d, compact %d, keeping one %d, omitting all %d; want each smaller than the last",
			len(array.Content), fullSize, keptSize, trimmedSize)
	}

	var status *opensysml.StatusError
	for name, opts := range map[string][]opensysml.ConvertOption{
		"WithOmitDerived without WithCompact": {from, opensysml.WithOmitDerived()},
		"a kept name that is not derived":     {from, opensysml.WithCompact(), opensysml.WithOmitDerived("declaredName")},
	} {
		if _, err := client.ConvertSource(ctx, source, opensysml.FormatAPIJSON, opts...); !errors.As(err, &status) || status.Code != opensysml.CodeInvalidArgument {
			t.Errorf("%s: %v, want CodeInvalidArgument", name, err)
		}
	}
	if _, err := client.ConvertSource(ctx, source, opensysml.FormatTTL, from, opensysml.WithCompact()); !errors.As(err, &status) || status.Code != opensysml.CodeInvalidArgument {
		t.Errorf("WithCompact to Turtle: %v, want CodeInvalidArgument", err)
	}
}

// WithCheckDiagnostics reports the semantic check beside the conversion, each finding with its severity and code.
func TestConvertReportsTheCheckDiagnosticsOnRequest(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()
	const broken = "package Broken { part def Boat { part motor : Missing; } }"
	from := opensysml.WithFromFormat(opensysml.FormatSysML)
	unchecked, err := client.ConvertSource(ctx, broken, opensysml.FormatAPIJSON, from)
	if err != nil {
		t.Fatalf("ConvertSource: %v", err)
	}
	if len(unchecked.Diagnostics) != 0 {
		t.Errorf("a conversion that was not asked to check reports %+v", unchecked.Diagnostics)
	}
	checked, err := client.ConvertSource(ctx, broken, opensysml.FormatAPIJSON, from, opensysml.WithCheckDiagnostics())
	if err != nil {
		t.Fatalf("ConvertSource with check diagnostics: %v", err)
	}
	if checked.Content != unchecked.Content {
		t.Error("asking for the check's diagnostics changed the converted content")
	}
	index := slices.IndexFunc(checked.Diagnostics, func(d opensysml.Diagnostic) bool { return strings.Contains(d.Message, "Missing") })
	if index < 0 {
		t.Fatalf("diagnostics = %+v, want the unresolved Missing", checked.Diagnostics)
	}
	if found := checked.Diagnostics[index]; found.Severity != opensysml.SeverityError || found.Code == "" || found.Span == nil {
		t.Errorf("the unresolved name is reported as %+v, want an error with its code and span", found)
	}

	// A parsed model reports the same findings its parse did.
	model, err := client.ParseDocuments(ctx, []opensysml.Document{opensysml.Source("broken.sysml", broken)})
	if err != nil {
		t.Fatalf("ParseDocuments: %v", err)
	}
	cached, err := client.Convert(ctx, model, opensysml.FormatAPIJSON, opensysml.WithCheckDiagnostics())
	if err != nil {
		t.Fatalf("Convert with check diagnostics: %v", err)
	}
	if !slices.ContainsFunc(cached.Diagnostics, func(d opensysml.Diagnostic) bool {
		return d.Severity == opensysml.SeverityError && strings.Contains(d.Message, "Missing")
	}) {
		t.Errorf("a parsed model's diagnostics = %+v, want the unresolved Missing as an error", cached.Diagnostics)
	}

	var status *opensysml.StatusError
	array, err := client.ConvertSource(ctx, "package P;", opensysml.FormatAPIJSON, from)
	if err != nil {
		t.Fatalf("ConvertSource: %v", err)
	}
	_, err = client.ConvertSource(ctx, array.Content, opensysml.FormatSysML,
		opensysml.WithFromFormat(opensysml.FormatAPIJSON), opensysml.WithCheckDiagnostics())
	if !errors.As(err, &status) || status.Code != opensysml.CodeInvalidArgument {
		t.Errorf("WithCheckDiagnostics on an API JSON source: %v, want CodeInvalidArgument", err)
	}
}

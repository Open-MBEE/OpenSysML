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

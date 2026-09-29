package grpc

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const (
	sourcesLibrary = "package Lib {\n\tpart def Engine {\n\t\tattribute power = 150;\n\t}\n}\n"
	sourcesTop     = "package Top {\n\tprivate import Lib::*;\n\tpart def Car {\n\t\tpart motor : Engine;\n\t}\n}\n"
)

// inlineDocuments names inline documents by the names given, in order.
func inlineDocuments(named ...string) []*pb.SourceDocument {
	documents := make([]*pb.SourceDocument, 0, len(named)/2)
	for i := 0; i+1 < len(named); i += 2 {
		documents = append(documents, &pb.SourceDocument{
			Source: &pb.SourceDocument_Content{Content: named[i+1]},
			Name:   named[i],
		})
	}
	return documents
}

func TestParseSourcesResolvesBetweenDocuments(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()

	resp, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
		Documents: inlineDocuments("lib.sysml", sourcesLibrary, "top.sysml", sourcesTop),
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	if len(resp.Diagnostics) != 0 {
		t.Errorf("diagnostics from documents that resolve each other: %v", resp.Diagnostics)
	}
	if len(resp.Roots) != 2 {
		t.Fatalf("Roots = %d, want 2", len(resp.Roots))
	}

	cached, ok := srv.cache.Get(resp.ModelHash)
	if !ok {
		t.Fatal("the model was not cached")
	}
	if len(cached.Documents) != 2 {
		t.Errorf("cached documents = %d, want 2", len(cached.Documents))
	}
	// Each document keeps its own name, which is what its diagnostics and the
	// index's document roots are keyed by.
	for i, name := range []string{"lib.sysml", "top.sysml"} {
		if got := cached.Documents[i].Source.Name(); got != name {
			t.Errorf("document %d named %q, want %q", i, got, name)
		}
	}
	if len(cached.DocumentRoots()) != 2 {
		t.Errorf("document roots = %d, want 2", len(cached.DocumentRoots()))
	}
	if syms := lookupNamed(cached.Index, "Lib::Engine"); len(syms) == 0 {
		t.Error("the library document's symbol is not in the model's index")
	}
}

func TestParseSourcesLocatesEachDiagnosticInItsOwnDocument(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()

	resp, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
		Documents: inlineDocuments(
			"clean.sysml", sourcesLibrary,
			"broken.sysml", "package Broken {\n\tpart def Wheel {\n\t\tpart hub : Missing;\n\t}\n}\n",
		),
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	if len(resp.Diagnostics) == 0 {
		t.Fatal("an undeclared type produced no diagnostic")
	}
	for _, diag := range resp.Diagnostics {
		if diag.Span != nil && diag.Span.File != "broken.sysml" {
			t.Errorf("diagnostic located in %q, want broken.sysml: %s", diag.Span.File, diag.Message)
		}
	}
}

func TestParseSourcesRefusesADocumentSetItCannotParse(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()

	for name, req := range map[string]*pb.ParseSourcesRequest{
		"no documents": {},
		"duplicate names": {
			Documents: inlineDocuments("same.sysml", sourcesLibrary, "same.sysml", sourcesTop),
		},
		"no source": {Documents: []*pb.SourceDocument{{Name: "empty.sysml"}}},
	} {
		_, err := srv.ParseSources(context.Background(), req)
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: err = %v, want INVALID_ARGUMENT", name, err)
		}
	}
}

func TestParseSourcesIsCapabilityGated(t *testing.T) {
	srv := mustNewServiceWithout(t, CapabilityParseSources)
	defer srv.Close()

	_, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
		Documents: inlineDocuments("lib.sysml", sourcesLibrary),
	})
	if connect.CodeOf(err) != connect.CodeUnimplemented {
		t.Errorf("err = %v, want UNIMPLEMENTED", err)
	}
}

// Convert writes the documents of a multi-document model as one graph, so a
// notation target is still refused; ApplyEdits edits the model as a whole, so
// the same model is answered, and an empty request is refused as it is for one
// document: it names no edit.
func TestOneDocumentOperationsRefuseAModelOfSeveral(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()

	resp, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
		Documents: inlineDocuments("lib.sysml", sourcesLibrary, "top.sysml", sourcesTop),
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}

	_, convertErr := srv.Convert(context.Background(), &pb.ConvertRequest{
		Source:   &pb.ConvertRequest_ModelHash{ModelHash: resp.ModelHash},
		ToFormat: "sysml",
	})
	if connect.CodeOf(convertErr) != connect.CodeFailedPrecondition ||
		!strings.Contains(convertErr.Error(), "one document") {
		t.Errorf("Convert err = %v, want a FAILED_PRECONDITION naming the one-document limit", convertErr)
	}

	_, legacyErr := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{ModelHash: resp.ModelHash})
	if connect.CodeOf(legacyErr) != connect.CodeFailedPrecondition ||
		!strings.Contains(legacyErr.Error(), "accept_documents") {
		t.Errorf("ApplyEdits err = %v, want a FAILED_PRECONDITION naming accept_documents", legacyErr)
	}

	edited, editErr := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{ModelHash: resp.ModelHash, AcceptDocuments: true})
	if editErr != nil {
		t.Fatalf("ApplyEdits err = %v, want the empty request refused in the response", editErr)
	}
	if edited.Failure != pb.EditFailure_EDIT_FAILURE_NO_OPERATIONS {
		t.Errorf("ApplyEdits failure = %s (%s), want NO_OPERATIONS", edited.Failure, edited.Error)
	}
	if edited.Content != "" || len(edited.Documents) != 0 {
		t.Errorf("a refusal returned notation: content=%q documents=%v", edited.Content, edited.Documents)
	}
}

func TestAReexportBetweenDocumentsIsIndexed(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()

	// A name re-exported by one document is registered under the re-exporting
	// namespace only once every document of the model is in the index.
	resp, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
		Documents: inlineDocuments(
			"lib.sysml", "package EngineLib {\n\tpart def Engine;\n}\n",
			"facade.sysml", "package EngineFacade {\n\tpublic import EngineLib::*;\n}\n",
			"top.sysml", "package Top {\n\tprivate import EngineFacade::*;\n\tpart def Car {\n\t\tpart motor : Engine;\n\t}\n}\n",
		),
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	if len(resp.Diagnostics) != 0 {
		t.Errorf("diagnostics from a chained import: %v", resp.Diagnostics)
	}

	cached, ok := srv.cache.Get(resp.ModelHash)
	if !ok {
		t.Fatal("the model was not cached")
	}
	if syms := lookupNamed(cached.Index, "EngineFacade::Engine"); len(syms) == 0 {
		t.Error("the name EngineFacade re-exports is not in the model's index")
	}
}

// A cached model of several documents converts to a graph as one model:
// the documents' elements resolve against each other, so the cross-document
// reference links to the element it declares in the library document.
func TestConvertConvertsEveryDocumentOfAModel(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()

	resp, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
		Documents: inlineDocuments("lib.sysml", sourcesLibrary, "top.sysml", sourcesTop),
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}

	for _, format := range []string{"turtle", "api-json"} {
		converted, err := srv.Convert(context.Background(), &pb.ConvertRequest{
			Source:   &pb.ConvertRequest_ModelHash{ModelHash: resp.ModelHash},
			ToFormat: format,
		})
		if err != nil {
			t.Fatalf("Convert to %s: %v", format, err)
		}
		if converted.Error != "" {
			t.Fatalf("Convert to %s: %s", format, converted.Error)
		}
		if !strings.Contains(converted.Content, "Lib__Engine") {
			t.Errorf("the %s output lost the library document's element:\n%s", format, converted.Content)
		}
		if !strings.Contains(converted.Content, "sourceDocument") {
			t.Errorf("the %s output carries no document provenance:\n%s", format, converted.Content)
		}
	}
}

func TestTwoDocumentSetsDoNotShareAModel(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()

	// The two sets differ only in where a boundary falls, which a key that ran
	// its fields together would spell the same way.
	hashOf := func(documents []*pb.SourceDocument) string {
		resp, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{Documents: documents})
		if err != nil {
			t.Fatalf("ParseSources: %v", err)
		}
		return resp.ModelHash
	}
	one := hashOf(inlineDocuments("a.sysml", "package A;\x00b.sysml\x00package B;"))
	two := hashOf(inlineDocuments("a.sysml", "package A;", "b.sysml", "package B;"))
	if one == two {
		t.Error("two document sets share one model hash")
	}
}

// An inline KerML document converts by its declared language, not the language
// its name suggests: the model's SourceDocument carries the language the
// client parsed it as, and Convert parses each document back in that kind.
func TestConvertParsesAnInlineKerMLDocumentAsKerML(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()

	resp, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
		Documents: []*pb.SourceDocument{
			{
				Source:   &pb.SourceDocument_Content{Content: "classifier C;\n"},
				Name:     "<content-0>",
				Language: "kerml",
			},
			{
				Source: &pb.SourceDocument_Content{Content: sourcesTop},
				Name:   "<content-1>",
			},
		},
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}

	converted, err := srv.Convert(context.Background(), &pb.ConvertRequest{
		Source:   &pb.ConvertRequest_ModelHash{ModelHash: resp.ModelHash},
		ToFormat: "turtle",
	})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if converted.Error != "" {
		t.Fatalf("Convert: %s", converted.Error)
	}
	if !strings.Contains(converted.Content, "sysx:declaredKeyword \"classifier\"") {
		t.Errorf("the KerML document's classifier did not convert:\n%s", converted.Content)
	}
}

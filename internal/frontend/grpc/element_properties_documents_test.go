package grpc

import (
	"context"
	"slices"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// parseDocumentsForElementProperties parses several documents as one model.
func parseDocumentsForElementProperties(t *testing.T, documents ...*pb.SourceDocument) *elementPropertiesReader {
	t.Helper()
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{Documents: documents})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	return &elementPropertiesReader{t: t, srv: srv, hash: parsed.ModelHash}
}

func excludedNames(resp *pb.GetElementPropertiesResponse) []string {
	var out []string
	for _, excluded := range resp.ExcludedDocuments {
		out = append(out, excluded.Document)
	}
	return out
}

func TestElementPropertiesExcludesOnlyTheDocumentWithASyntaxError(t *testing.T) {
	r := parseDocumentsForElementProperties(t,
		&pb.SourceDocument{Name: "a.sysml", Source: &pb.SourceDocument_Content{Content: "package A { part def Unit; }"}},
		&pb.SourceDocument{Name: "b.sysml", Source: &pb.SourceDocument_Content{Content: "package B { import A::*; part u : Unit; }"}},
		&pb.SourceDocument{Name: "c.sysml", Source: &pb.SourceDocument_Content{Content: "package C { part def Broken { ; }"}},
	)
	answered := r.read("B::u", "type")
	if answered.Error != "" || len(answered.Properties) != 1 {
		t.Fatalf("an element of a readable document answers %+v", answered)
	}
	if got := excludedNames(answered); len(got) != 1 || got[0] != "c.sysml" {
		t.Errorf("excluded documents = %v, want [c.sysml]", got)
	}
	if !strings.Contains(answered.ExcludedDocuments[0].Error, "syntax error") {
		t.Errorf("c.sysml is excluded for %q, want its syntax error", answered.ExcludedDocuments[0].Error)
	}
	if other := r.read("A::Unit", "name"); other.Error != "" || len(other.Properties) != 1 {
		t.Errorf("an element of another readable document answers %+v", other)
	}
	refused := r.read("C", "name")
	if !strings.Contains(refused.Error, "c.sysml") || !strings.Contains(refused.Error, "syntax error") || len(refused.Properties) != 0 {
		t.Errorf("an element of the excluded document answers %+v, want its document's syntax error", refused)
	}
	if got := excludedNames(refused); len(got) != 1 || got[0] != "c.sysml" {
		t.Errorf("the refusal lists excluded documents %v, want [c.sysml]", got)
	}
}

func TestElementPropertiesExcludesTheLaterDocumentDeclaringATopLevelNameAgain(t *testing.T) {
	r := parseDocumentsForElementProperties(t,
		&pb.SourceDocument{Name: "a.sysml", Source: &pb.SourceDocument_Content{Content: "package A { part def Unit; }"}},
		&pb.SourceDocument{Name: "b.sysml", Source: &pb.SourceDocument_Content{Content: "package B { part def Other; }"}},
		&pb.SourceDocument{Name: "d.sysml", Source: &pb.SourceDocument_Content{Content: "package A { part def Twice; }"}},
	)
	first := r.read("A::Unit", "name")
	if first.Error != "" || len(first.Properties) != 1 {
		t.Fatalf("the first declaration's element answers %+v", first)
	}
	if got := excludedNames(first); len(got) != 1 || got[0] != "d.sysml" {
		t.Fatalf("excluded documents = %v, want [d.sysml]", got)
	}
	why := first.ExcludedDocuments[0].Error
	if !strings.Contains(why, "A at d.sysml:1:1") || !strings.Contains(why, "A at a.sysml:1:1") {
		t.Errorf("d.sysml is excluded for %q, want both declarations named", why)
	}
	if other := r.read("B::Other", "name"); other.Error != "" || len(other.Properties) != 1 {
		t.Errorf("an element of an uninvolved document answers %+v", other)
	}
	refused := r.read("A::Twice", "name")
	if refused.Error != why || len(refused.Properties) != 0 {
		t.Errorf("an element of the later declaration answers %+v, want its document's error", refused)
	}
}

func TestElementPropertiesExcludesNothingForAValidationError(t *testing.T) {
	r := parseDocumentsForElementProperties(t,
		&pb.SourceDocument{Name: "a.sysml", Source: &pb.SourceDocument_Content{Content: "package A { part def Unit; }"}},
		&pb.SourceDocument{Name: "b.sysml", Source: &pb.SourceDocument_Content{Content: "package B { part broken : Missing; }"}},
	)
	for _, element := range []string{"A::Unit", "B::broken"} {
		answered := r.read(element, "name")
		if answered.Error != "" || len(answered.Properties) != 1 || len(answered.ExcludedDocuments) != 0 {
			t.Errorf("%s answers %+v, want an answer with no excluded documents", element, answered)
		}
	}
}

func TestElementPropertiesReadsAPackageAnUnreadableDocumentAlsoDeclares(t *testing.T) {
	r := parseDocumentsForElementProperties(t,
		&pb.SourceDocument{Name: "broken.sysml", Source: &pb.SourceDocument_Content{Content: "package A { part def Unit; part def }"}},
		&pb.SourceDocument{Name: "good.sysml", Source: &pb.SourceDocument_Content{Content: "package A { part def Whole; }"}},
	)
	whole := r.read("A::Whole", "name")
	if whole.Error != "" || len(whole.Properties) != 1 {
		t.Fatalf("the readable declaration's element answers %+v", whole)
	}
	if got := excludedNames(whole); len(got) != 1 || got[0] != "broken.sysml" {
		t.Errorf("excluded documents = %v, want [broken.sysml] only", got)
	}
}

func TestElementPropertiesKeepsDocumentsWhoseShortNameOnlyIsShared(t *testing.T) {
	r := parseDocumentsForElementProperties(t,
		&pb.SourceDocument{Name: "alpha.sysml", Source: &pb.SourceDocument_Content{Content: "package <A> Alpha { part def Unit; }"}},
		&pb.SourceDocument{Name: "a.sysml", Source: &pb.SourceDocument_Content{Content: "package A { part def Other; }"}},
	)
	for _, id := range []string{"Alpha::Unit", "A::Other"} {
		resp := r.read(id, "name")
		if resp.Error != "" || len(resp.Properties) != 1 || len(resp.ExcludedDocuments) != 0 {
			t.Errorf("%s answers %+v, want an answer with no excluded document", id, resp)
		}
	}
}

// TestElementPropertiesAnswerBesideAnExcludedDocument: an element of a readable document answers
// whatever a second document does (a syntax error, a top-level name declared again, a validation error).
func TestElementPropertiesAnswerBesideAnExcludedDocument(t *testing.T) {
	cases := []struct {
		name, document, content string
		excluded                []string
	}{
		{"a syntax error", "c.sysml", "package C { part y }", []string{"c.sysml"}},
		{"a top-level name declared again", "d.sysml", "package A { part z; }", []string{"d.sysml"}},
		{"a validation error", "v.sysml", "package V { part y : Missing; }", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := parseDocumentsForElementProperties(t,
				&pb.SourceDocument{Name: "a.sysml", Source: &pb.SourceDocument_Content{Content: "package A { part x; }"}},
				&pb.SourceDocument{Name: tc.document, Source: &pb.SourceDocument_Content{Content: tc.content}},
			)
			answered := r.read("A::x", "name")
			if answered.Error != "" || len(answered.Properties) != 1 {
				t.Fatalf("A::x answers %+v, want its name", answered)
			}
			if got := answered.Properties[0].Values; len(got) != 1 || got[0].GetStringValue() != "x" {
				t.Errorf("A::x name = %v, want x", got)
			}
			if got := excludedNames(answered); !slices.Equal(got, tc.excluded) {
				t.Errorf("excluded documents = %v, want %v", got, tc.excluded)
			}
		})
	}
}

func TestElementPropertiesNameAnExcludedDocumentsElement(t *testing.T) {
	r := parseDocumentsForElementProperties(t,
		&pb.SourceDocument{Name: "u.sysml", Source: &pb.SourceDocument_Content{Content: "package U { private import C::*; part u : CD; }"}},
		&pb.SourceDocument{Name: "c.sysml", Source: &pb.SourceDocument_Content{Content: "package C { part def CD; part y }"}},
	)
	typed := r.read("U::u", "type")
	if typed.Error != "" || len(typed.Properties) != 1 || len(typed.Properties[0].Values) != 1 {
		t.Fatalf("U::u answers %+v, want its one type", typed)
	}
	named := typed.Properties[0].Values[0].GetElement()
	if named.GetId() == "" {
		t.Fatalf("U::u type = %+v, want an element reference", typed.Properties[0].Values[0])
	}
	followed := r.read(named.Id, "name")
	if !strings.Contains(followed.Error, "c.sysml") || !strings.Contains(followed.Error, "syntax error") {
		t.Errorf("following %s answers %+v, want c.sysml's syntax error", named.Id, followed)
	}
}

// An id the element form cannot mint refuses the model whole, as Convert does: no
// element answers, one of a document the parser could not read whole included.
func TestElementPropertiesRefuseAModelWhoseIdsCannotBeMinted(t *testing.T) {
	r := parseDocumentsForElementProperties(t,
		&pb.SourceDocument{Name: "c.sysml", Source: &pb.SourceDocument_Content{Content: "package C { part def Broken; part def X { ; }"}},
		&pb.SourceDocument{Name: "d.sysml", Source: &pb.SourceDocument_Content{Content: "package D {\n\tprivate import IdentityMetadata::*;\n\tpart def A { @ElementId { id = \"bad id\"; } }\n}"}},
	)
	for _, id := range []string{"C::Broken", "D::A", "D"} {
		answered := r.read(id, "name")
		if !strings.Contains(answered.Error, `its id "bad id"`) || len(answered.Properties) != 0 {
			t.Errorf("%s answers %+v, want the model's refusal", id, answered)
		}
	}
}

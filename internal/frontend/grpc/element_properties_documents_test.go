package grpc

import (
	"context"
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

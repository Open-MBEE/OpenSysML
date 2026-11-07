package grpc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// TestElementPropertiesAnswersEveryElementOfTheCorpora loads the pilot corpora and
// the training examples as one model and reads every element: one of an excluded
// document answers that document's error, every other one is answered.
func TestElementPropertiesAnswersEveryElementOfTheCorpora(t *testing.T) {
	var documents []*pb.SourceDocument
	for _, root := range []string{"../../../examples/pilot-corpora", "../../../examples/sysml-v2-training"} {
		if _, err := os.Stat(root); err != nil {
			t.Skipf("%s is absent", root)
		}
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if ext := filepath.Ext(path); !entry.IsDir() && (ext == ".sysml" || ext == ".kerml") {
				absolute, err := filepath.Abs(path)
				if err != nil {
					return err
				}
				documents = append(documents, &pb.SourceDocument{Source: &pb.SourceDocument_FilePath{FilePath: absolute}})
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(documents) < 100 {
		t.Fatalf("%d corpus documents found, want the whole corpora", len(documents))
	}
	srv := mustNewService(t, 10)
	defer srv.Close()
	parsed, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{Documents: documents})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	cached, ok := srv.cache.Get(parsed.ModelHash)
	if !ok {
		t.Fatal("the parsed model is not cached")
	}
	reader := cached.elementReader()
	t.Logf("%d documents, %d excluded", len(documents), len(reader.exclusions))
	answered, refused := 0, 0
	cached.Index.Registered(func(fqn string, syms []*symbols.Symbol) {
		for _, sym := range syms {
			if sym.Name == "" || sym.Implicit != nil || sym.Expr != nil || cached.Index.Library(sym) || cached.Index.GetFQN(sym) != fqn {
				continue
			}
			if _, inModel := reader.excluded[sym.DocName]; !inModel && sym.DocName == "" {
				continue
			}
			resp, err := srv.GetElementProperties(context.Background(), &pb.GetElementPropertiesRequest{
				ModelHash: parsed.ModelHash, ElementId: fqn, Properties: []string{"name"},
			})
			if err != nil {
				t.Fatalf("GetElementProperties(%s): %v", fqn, err)
			}
			served, _ := reader.lookup(fqn)
			if excluded, isExcluded := reader.excluded[served.DocName]; isExcluded {
				refused++
				if resp.Error != excluded.Error {
					t.Errorf("%s of excluded %s answers %q, want its document's error", fqn, served.DocName, resp.Error)
				}
				continue
			}
			if resp.Error != "" && !strings.HasPrefix(resp.Error, "element not found") {
				t.Errorf("%s (%s) is refused: %s", fqn, served.DocName, resp.Error)
			}
			answered++
		}
	})
	t.Logf("%d elements answered, %d refused with their document's error", answered, refused)
	if answered == 0 {
		t.Fatal("no element was answered")
	}
}

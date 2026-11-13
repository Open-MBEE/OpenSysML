package grpc

import (
	"context"
	"sync"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// Concurrent GetSymbol requests share one cached model, whose semantic model
// memoizes the member walk a positionally named id takes: the lookups must serialize on it.
func TestGetSymbolConcurrentLookupsShareTheModel(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, _ := parseContent(t, srv, `package P {
		private import ScalarValues::*;
		calc def Calc { in x : Real; return r : Real; }
		constraint def Cons { in x : Real; x > 0 }
		verification def Ver { subject s; }
	}`)
	ids := []string{
		"P::Calc", "P::Calc::r", "P::Cons", "P::Ver",
		"Performances::BooleanEvaluation::result", "Performances::Evaluation::result",
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		for _, id := range ids {
			wg.Add(1)
			go func() {
				defer wg.Done()
				resp, err := srv.GetSymbol(context.Background(), &pb.GetSymbolRequest{ModelHash: parsed.ModelHash, SymbolId: id})
				if err != nil || resp.GetSymbol() == nil {
					t.Errorf("GetSymbol(%s): %v", id, err)
				}
			}()
		}
	}
	wg.Wait()
}

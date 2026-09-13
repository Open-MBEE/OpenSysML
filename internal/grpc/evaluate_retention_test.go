package grpc

import (
	"context"
	"fmt"
	"sync"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// Each request evaluates on a resolver and semantic model of its own over the cached
// index, so what a request resolves or parses is the request's, and requests on one
// model run beside each other. Under -race, this is the service's isolation test.
func TestEvaluateRunsEachRequestOnAWorkerOfItsOwn(t *testing.T) {
	const model = `
package Demo {
  calc def Twice { in x : ScalarValues::Real; return : ScalarValues::Real = x * 2.0; }
  part def Vehicle {
    attribute mass = 1500.0;
    part engine { attribute power = 100.0; }
  }
  part sedan : Vehicle {
    attribute :>> mass = 1200.0;
  }
}
`
	srv := mustNewService(t, 10)
	parseResp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: model},
		ContentHash: "evaluate-retention",
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	cached, ok := srv.cache.Get(parseResp.ModelHash)
	if !ok {
		t.Fatal("parsed model not cached")
	}

	evaluate := func(expr string) (*pb.EvaluateResponse, error) {
		return srv.Evaluate(context.Background(), &pb.EvaluateRequest{
			ModelHash: parseResp.ModelHash, Expression: expr, SubjectSymbolId: "Demo::sedan",
		})
	}
	var wg sync.WaitGroup
	errs := make(chan string, 128)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			expr := fmt.Sprintf("Twice(mass) + engine.power + Demo::sedan::mass + %d.0", i)
			resp, err := evaluate(expr)
			switch {
			case err != nil:
				errs <- fmt.Sprintf("Evaluate(%q): %v", expr, err)
			case resp.GetError() != "":
				errs <- fmt.Sprintf("Evaluate(%q): %s", expr, resp.GetError())
			case resp.GetResult().GetRealValue() != 2400+100+1200+float64(i):
				errs <- fmt.Sprintf("Evaluate(%q) = %v", expr, resp.GetResult())
			}
			// A misspelling is the request's: resolving it must not disturb the others.
			if resp, err := evaluate(fmt.Sprintf("nosuch%d(1.0) + masss%d", i, i)); err != nil || resp.GetError() == "" {
				errs <- fmt.Sprintf("expression %d: unresolved names evaluated: %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}

	// Nothing a request resolved is the cached model's: a worker built after them
	// starts with no resolutions and no selections.
	worker, err := cached.Semantics()
	if err != nil {
		t.Fatalf("Semantics: %v", err)
	}
	resolver, sem := worker.Resolver(), worker.Semantics()
	if got, gotSel := resolver.MemoSize(), sem.MemoSize(); got != 0 || gotSel != 0 {
		t.Fatalf("a new worker starts with %d resolutions and %d selections, want none", got, gotSel)
	}
	otherModel, err := cached.Semantics()
	if err != nil {
		t.Fatalf("Semantics: %v", err)
	}
	if other := otherModel.Resolver(); other == resolver || other.Index() != resolver.Index() || resolver.Index() != cached.Index {
		t.Fatal("two workers over one model: want distinct resolvers over the one cached index")
	}
}

// A worker handed on from request to request keeps what it resolved of the model and nothing
// of the requests' expressions: unique expressions, resolved or not, do not grow its memo.
func TestWorkerHandedOnDoesNotRetainRequestExpressions(t *testing.T) {
	const model = `
package Demo {
  calc def Twice { in x : ScalarValues::Real; return : ScalarValues::Real = x * 2.0; }
  part def Vehicle {
    attribute mass = 1500.0;
    part engine { attribute power = 100.0; }
  }
  part sedan : Vehicle {
    attribute :>> mass = 1200.0;
  }
}
`
	srv := mustNewService(t, 10)
	parseResp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: model},
		ContentHash: "evaluate-worker-retention",
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	cached, ok := srv.cache.Get(parseResp.ModelHash)
	if !ok {
		t.Fatal("parsed model not cached")
	}
	evaluate := func(expr string) string {
		resp, err := srv.Evaluate(context.Background(), &pb.EvaluateRequest{
			ModelHash: parseResp.ModelHash, Expression: expr, SubjectSymbolId: "Demo::sedan",
		})
		if err != nil {
			t.Fatalf("Evaluate(%q): %v", expr, err)
		}
		return resp.GetError()
	}
	// Sequential requests hold the one worker in turn, so its memo is what they all left.
	memoSize := func() (int, int, int) {
		w, release := cached.worker()
		defer release()
		if got := len(cached.idle); got != 0 {
			t.Fatalf("%d workers idle while the requests' worker is held, want the one worker handed on", got)
		}
		return w.Model.Resolver().MemoSize(), w.Model.Semantics().MemoSize(), len(w.Model.Resolver().Diagnostics)
	}
	if msg := evaluate("Twice(mass) + engine.power + Demo::sedan::mass + 1.0"); msg != "" {
		t.Fatalf("Evaluate: %s", msg)
	}
	before, beforeSel, beforeDiags := memoSize()
	for i := 0; i < 50; i++ {
		if msg := evaluate(fmt.Sprintf("Twice(mass) + engine.power + Demo::sedan::mass + %d.0", i)); msg != "" {
			t.Fatalf("Evaluate %d: %s", i, msg)
		}
		if evaluate(fmt.Sprintf("nosuch%d(1.0) + masss%d", i, i)) == "" {
			t.Fatalf("expression %d: unresolved names evaluated", i)
		}
	}
	if got, gotSel, gotDiags := memoSize(); got != before || gotSel != beforeSel || gotDiags != beforeDiags {
		t.Fatalf("the worker handed on grew from %d+%d resolutions and selections with %d diagnostics to %d+%d with %d over 100 unique expressions",
			before, beforeSel, beforeDiags, got, gotSel, gotDiags)
	}
}

// Each request selects the overloads its expressions invoke on the semantic model of the
// worker it holds, every request selects alike, and a selection the model's own declarations
// made stays on the worker for the next request while the request's expression leaves nothing.
func TestEvaluateSelectsInvocationsOnEachWorker(t *testing.T) {
	const model = `
package Demo {
  calc def Twice { in x : ScalarValues::Real; return : ScalarValues::Real = x * 2.0; }
  part def Vehicle {
    attribute mass = 1500.0;
    attribute doubled = Twice(mass);
  }
  part sedan : Vehicle;
}
`
	srv := mustNewService(t, 10)
	parseResp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: model},
		ContentHash: "evaluate-selection-retention",
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	cached, ok := srv.cache.Get(parseResp.ModelHash)
	if !ok {
		t.Fatal("parsed model not cached")
	}
	evaluate := func() float64 {
		resp, err := srv.Evaluate(context.Background(), &pb.EvaluateRequest{
			ModelHash: parseResp.ModelHash, Expression: "doubled", SubjectSymbolId: "Demo::sedan",
		})
		if err != nil || resp.GetError() != "" {
			t.Fatalf("Evaluate: %v %s", err, resp.GetError())
		}
		return resp.GetResult().GetRealValue()
	}
	if got := evaluate(); got != 3000 {
		t.Fatalf("doubled = %v, want 3000", got)
	}
	if got := evaluate(); got != 3000 {
		t.Fatalf("doubled on a second request = %v, want 3000", got)
	}

	// The selection lives on the worker that made it, which the requests held in turn and
	// gave back warm: the next request finds Twice(mass) already selected.
	worker, err := cached.Semantics()
	if err != nil {
		t.Fatalf("Semantics: %v", err)
	}
	if got := worker.Semantics().MemoSize(); got != 0 {
		t.Fatalf("a new worker starts with %d selections, want none", got)
	}
	rt, release := srv.newRuntime(cached)
	defer release()
	warm := rt.Semantics().MemoSize()
	if warm == 0 {
		t.Fatal("the requests' worker was not handed on: it holds no selection")
	}
	doubled := cached.Index.LookupQualified("Demo::Vehicle::doubled")
	if len(doubled) != 1 {
		t.Fatalf("Demo::Vehicle::doubled resolved to %d symbols, want 1", len(doubled))
	}
	if _, err := rt.EvalDeclaredValue(doubled[0]); err != nil {
		t.Fatalf("doubled on the worker: %v", err)
	}
	if got := rt.Semantics().MemoSize(); got != warm {
		t.Fatalf("evaluating doubled on the warm worker selected %d more invocations, want its selection kept", got-warm)
	}
}

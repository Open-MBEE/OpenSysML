// Package perfbench holds the benchmarks behind docs/project/performance-profile-2026-09.md.
// Run with: go test ./internal/perfbench -run '^$' -bench . -benchmem
package perfbench

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/api/proto/protoconnect"
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/grpc"
	"github.com/Open-MBEE/OpenSysML/internal/repl"
)

// ---------------------------------------------------------------- area 5: lexer/parser

func BenchmarkLex(b *testing.B) {
	for _, w := range workloads {
		b.Run(w.name, func(b *testing.B) {
			src := w.src(b)
			b.SetBytes(int64(len(src)))
			b.ReportAllocs()
			var toks int
			for i := 0; i < b.N; i++ {
				sf := source.New("m.sysml", src)
				lx := lexer.New(sf)
				toks = 0
				for {
					t := lx.Next()
					toks++
					if t.Kind == lexer.EOF {
						break
					}
				}
			}
			b.ReportMetric(float64(toks), "tokens")
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(toks), "ns/token")
		})
	}
}

func BenchmarkParse(b *testing.B) {
	for _, w := range workloads {
		b.Run(w.name, func(b *testing.B) {
			src := w.src(b)
			b.SetBytes(int64(len(src)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				sf := source.New("m.sysml", src)
				p := parser.New(sf)
				root := p.ParseFile()
				if len(p.Diagnostics) > 0 {
					b.Fatalf("diags: %v", p.Diagnostics[0])
				}
				_ = root
			}
		})
	}
}

// ---------------------------------------------------------------- area 1: indexing / resolution

func parseRoot(tb testing.TB, name string, src []byte) *ast.RootNamespace {
	p := parser.New(source.New(name, src))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		tb.Fatalf("diags: %v", p.Diagnostics[0])
	}
	return root
}

// AddDocument + ExpandWildcardImports on a fresh stdlib overlay (no parse).
func BenchmarkIndexAddExpand(b *testing.B) {
	for _, w := range workloads {
		b.Run(w.name, func(b *testing.B) {
			root := parseRoot(b, "m.sysml", w.src(b))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				idx := libs.NewModelIndex()
				idx.AddDocument("m.sysml", root)
				idx.ExpandWildcardImports()
			}
		})
	}
}

func BenchmarkIndexAddOnly(b *testing.B) {
	for _, w := range workloads {
		b.Run(w.name, func(b *testing.B) {
			root := parseRoot(b, "m.sysml", w.src(b))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				idx := libs.NewModelIndex()
				idx.AddDocument("m.sysml", root)
			}
		})
	}
}

// Full analysis (all passes) over an indexed model.
func BenchmarkAnalyze(b *testing.B) {
	for _, w := range workloads {
		b.Run(w.name, func(b *testing.B) {
			root := parseRoot(b, "m.sysml", w.src(b))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				idx := libs.NewModelIndex()
				idx.AddDocument("m.sysml", root)
				idx.ExpandWildcardImports()
				b.StopTimer()
				b.StartTimer()
				diags := passes.AnalyzeWithOptions("m.sysml", source.KindSysML, root, nil, idx, passes.Options{})
				for _, d := range diags {
					if d.Severity == passes.SeverityError {
						b.Fatalf("error: %s", d.Message)
					}
				}
			}
		})
	}
}

// Workspace edit path: Update one open doc (reparse + reindex + expand), then Diagnostics.
func BenchmarkWorkspaceEdit(b *testing.B) {
	for _, w := range workloads {
		b.Run(w.name, func(b *testing.B) {
			src := w.src(b)
			ws := model.NewWorkspace()
			ws.Open("m.sysml", src, 1)
			_ = ws.Diagnostics("m.sysml")
			edited := append([]byte{}, src...)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// flip a comment-safe byte: append a trailing comment line
				e := append(edited[:len(edited):len(edited)], []byte(fmt.Sprintf("\n// edit %d\n", i))...)
				ws.Update("m.sysml", e, i+2)
			}
		})
		b.Run(w.name+"/reindex+diagnostics", func(b *testing.B) {
			src := w.src(b)
			ws := model.NewWorkspace()
			ws.Open("m.sysml", src, 1)
			_ = ws.Diagnostics("m.sysml")
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				e := append(src[:len(src):len(src)], []byte(fmt.Sprintf("\n// edit %d\n", i))...)
				ws.Update("m.sysml", e, i+2)
				_ = ws.Diagnostics("m.sysml")
			}
		})
	}
}

// LSP-like: large model on disk (closed), small open doc edited repeatedly.
func BenchmarkWorkspaceEditSmallDocBesideLarge(b *testing.B) {
	src := syntheticSource(b)
	ws := model.NewWorkspace()
	ws.Open("big.sysml", src, 1)
	_ = ws.Diagnostics("big.sysml")
	small := "package Small { private import Perf::*; part x : Comp1 { attribute :>> mass = 3.0; } }"
	ws.Open("small.sysml", []byte(small), 1)
	_ = ws.Diagnostics("small.sysml")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ws.Update("small.sysml", []byte(small+fmt.Sprintf("\n// %d\n", i)), i+2)
		_ = ws.Diagnostics("small.sysml")
	}
}

func BenchmarkFQNOf(b *testing.B) {
	src := syntheticSource(b)
	root := parseRoot(b, "m.sysml", src)
	idx := libs.NewModelIndex()
	idx.AddDocument("m.sysml", root)
	idx.ExpandWildcardImports()
	var syms []*symbols.Symbol
	collect(idx.DocumentRoot("m.sysml"), &syms)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, s := range syms {
			_ = symbols.FQNOf(s)
		}
	}
	b.ReportMetric(float64(len(syms)), "syms")
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(len(syms)), "ns/sym")
}

func BenchmarkLookupQualified(b *testing.B) {
	src := syntheticSource(b)
	root := parseRoot(b, "m.sysml", src)
	idx := libs.NewModelIndex()
	idx.AddDocument("m.sysml", root)
	idx.ExpandWildcardImports()
	var syms []*symbols.Symbol
	collect(idx.DocumentRoot("m.sysml"), &syms)
	fqns := make([]string, 0, len(syms))
	for _, s := range syms {
		if s.Name != "" {
			fqns = append(fqns, symbols.FQNOf(s))
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, f := range fqns {
			if len(idx.LookupQualified(f)) == 0 {
				b.Fatalf("missing %s", f)
			}
		}
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(len(fqns)), "ns/lookup")
}

func collect(sc *symbols.Scope, out *[]*symbols.Symbol) {
	if sc == nil {
		return
	}
	sc.ForEachMember(func(s *symbols.Symbol) bool {
		*out = append(*out, s)
		collect(s.Scope, out)
		return true
	})
}

// Tier-1 flattening: effective features of every part def / usage, fresh context each iter.
func BenchmarkFeaturesOf(b *testing.B) {
	src := syntheticSource(b)
	root := parseRoot(b, "m.sysml", src)
	idx := libs.NewModelIndex()
	idx.AddDocument("m.sysml", root)
	idx.ExpandWildcardImports()
	var syms []*symbols.Symbol
	collect(idx.DocumentRoot("m.sysml"), &syms)
	var types []*symbols.Symbol
	for _, s := range syms {
		switch s.Kind {
		case symbols.SymbolPartDef, symbols.SymbolPartUsage, symbols.SymbolStateDef, symbols.SymbolActionDef, symbols.SymbolRequirementDef:
			types = append(types, s)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := resolve.New(idx)
		m := semantics.NewModel(r)
		ctx := runtime.NewContext(runtime.NewModel(m, r), 100000)
		for _, t := range types {
			_ = ctx.FeaturesOf(t)
		}
	}
	b.ReportMetric(float64(len(types)), "types")
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(len(types)), "ns/type")
}

// ---------------------------------------------------------------- REPL path (area 1 + 4)

func loadedSession(tb testing.TB, path string) *repl.Session {
	s := repl.NewSession()
	if _, err := s.LoadFilesSummary([]string{path}); err != nil {
		tb.Fatal(err)
	}
	if s.HasErrors() {
		tb.Fatalf("errors: %v", s.DiagnosticLines()[:3])
	}
	return s
}

func BenchmarkREPLLoadFile(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		loadedSession(b, syntheticPath(b))
	}
}

// One small snippet submitted into a session holding the big model.
func BenchmarkREPLSubmitSnippet(b *testing.B) {
	s := loadedSession(b, syntheticPath(b))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := s.Submit(fmt.Sprintf("package Snip%d { private import Perf::*; part p : Comp1 { attribute :>> mass = %d.0; } }", i, i+1))
		if len(res.Diagnostics) > 0 {
			b.Fatalf("diag: %v", res.Diagnostics[0].Message)
		}
	}
}

func BenchmarkREPLEvalExpr(b *testing.B) {
	s := loadedSession(b, syntheticPath(b))
	if _, err := s.EvalExpr("Perf::Calc1(2.0, 3.0)"); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.EvalExpr("Perf::Calc1(2.0, 3.0)"); err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------- area 2: action/state

type rt struct {
	idx  *symbols.Index
	res  *resolve.Resolver
	sem  *semantics.Model
	ctx  *runtime.Context
	root *symbols.Scope
}

func newRT(tb testing.TB, src []byte) *rt {
	root := parseRoot(tb, "m.sysml", src)
	idx := libs.NewModelIndex()
	idx.AddDocument("m.sysml", root)
	idx.ExpandWildcardImports()
	r := resolve.New(idx)
	m := semantics.NewModel(r)
	ctx := runtime.NewContext(runtime.NewModel(m, r), 10_000_000)
	return &rt{idx: idx, res: r, sem: m, ctx: ctx, root: idx.DocumentRoot("m.sysml")}
}

func (r *rt) sym(tb testing.TB, fqn string) *symbols.Symbol {
	syms := r.idx.LookupQualified(fqn)
	if len(syms) == 0 {
		tb.Fatalf("no symbol %s", fqn)
	}
	return syms[0]
}

func BenchmarkLowerActionGraph(b *testing.B) {
	r := newRT(b, syntheticSource(b))
	act := r.sym(b, "Perf::proc1")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := lower.ToActionGraph(act.Decl, act.Scope); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLowerStateGraph(b *testing.B) {
	r := newRT(b, syntheticSource(b))
	sm := r.sym(b, "Perf::SM1")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := lower.ToStateGraph(sm.Decl, sm.Scope); err != nil {
			b.Fatal(err)
		}
	}
}

// Whole action run (lower + init + run) with step count so ns/step is derivable.
func BenchmarkExecuteAction(b *testing.B) {
	r := newRT(b, syntheticSource(b))
	act := r.sym(b, "Perf::proc1")
	var steps int
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		exec, err := r.ctx.CreateActionExecutor(act)
		if err != nil {
			b.Fatal(err)
		}
		steps = 0
		for exec.State() != runtime.StateCompleted {
			if err := exec.Step(); err != nil {
				b.Fatal(err)
			}
			steps++
		}
	}
	b.ReportMetric(float64(steps), "steps")
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(steps), "ns/step")
}

// Same action executed on the same context repeatedly (cache warm) vs fresh context per run.
func BenchmarkExecuteActionFreshContext(b *testing.B) {
	r := newRT(b, syntheticSource(b))
	act := r.sym(b, "Perf::proc1")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := runtime.NewContext(runtime.NewModel(r.sem, r.res), 10_000_000)
		if _, err := ctx.ExecuteAction(act); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkExecuteState(b *testing.B) {
	r := newRT(b, syntheticSource(b))
	sm := r.sym(b, "Perf::SM1")
	var events int
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		exec, err := r.ctx.CreateStateExecutor(sm)
		if err != nil {
			b.Fatal(err)
		}
		events = 0
		for exec.State() == runtime.StateRunning && exec.HasPendingWork() {
			if err := exec.ProcessNextEvent(); err != nil {
				b.Fatal(err)
			}
			events++
		}
		if exec.State() != runtime.StateCompleted {
			b.Fatalf("state %v after %d events", exec.State(), events)
		}
	}
	b.ReportMetric(float64(events), "events")
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(events), "ns/event")
}

// ---------------------------------------------------------------- area 3: batch constraints

func BenchmarkBatchConstraints(b *testing.B) {
	r := newRT(b, syntheticSource(b))
	n := 1000
	insts := make([]*runtime.Instance, 0, n)
	cons := make([][2]*symbols.Symbol, 0, n)
	for i := 0; i < n; i++ {
		inst, err := r.ctx.Instantiate(r.sym(b, fmt.Sprintf("Perf::inst%d", i)))
		if err != nil {
			b.Fatal(err)
		}
		insts = append(insts, inst)
		cons = append(cons, [2]*symbols.Symbol{
			r.sym(b, fmt.Sprintf("Perf::Comp%d::massOK", i)),
			r.sym(b, fmt.Sprintf("Perf::Comp%d::powerOK", i)),
		})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j, inst := range insts {
			for _, c := range cons[j] {
				res, err := r.ctx.CheckConstraintOn(c, c.OwnerScope, inst)
				if err != nil && !strings.Contains(err.Error(), "evaluated to false") {
					b.Fatalf("constraint %s: %v", c.Name, err)
				}
				_ = res
			}
		}
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(2*n), "ns/check")
}

// Same constraint over many instances of one type (repeated subexpression across carriers).
func BenchmarkSameConstraintManyInstances(b *testing.B) {
	r := newRT(b, syntheticSource(b))
	c := r.sym(b, "Perf::Comp1::massOK")
	inst, err := r.ctx.Instantiate(r.sym(b, "Perf::inst1"))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := r.ctx.CheckConstraintOn(c, c.OwnerScope, inst)
		if err != nil || !res.Holds {
			b.Fatalf("%v %v", err, res.Holds)
		}
	}
}

func BenchmarkBatchSatisfy(b *testing.B) {
	r := newRT(b, syntheticSource(b))
	assertions := r.ctx.SatisfyAssertionsIn(r.sym(b, "Perf").Scope)
	if len(assertions) != 1000 {
		b.Fatalf("assertions: %d", len(assertions))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := runtime.NewContext(runtime.NewModel(r.sem, r.res), 10_000_000)
		for _, a := range assertions {
			if _, err := ctx.EvaluateSatisfaction(a); err != nil && !strings.Contains(err.Error(), "evaluated to false") {
				b.Fatalf("satisfy: %v", err)
			}
		}
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(len(assertions)), "ns/assertion")
}

func BenchmarkInstantiate(b *testing.B) {
	r := newRT(b, syntheticSource(b))
	syms := make([]*symbols.Symbol, 0, 1000)
	for i := 0; i < 1000; i++ {
		syms = append(syms, r.sym(b, fmt.Sprintf("Perf::inst%d", i)))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := runtime.NewContext(runtime.NewModel(r.sem, r.res), 10_000_000)
		for _, s := range syms {
			if _, err := ctx.Instantiate(s); err != nil {
				b.Fatal(err)
			}
		}
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/1000, "ns/inst")
}

// ---------------------------------------------------------------- area 4: gRPC / Connect

func BenchmarkGRPCParseFileCached(b *testing.B) {
	svc, err := sysmlgrpc.NewService(16, "bench")
	if err != nil {
		b.Fatal(err)
	}
	src := string(vehicle(b))
	req := &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: src}}
	if _, err := svc.ParseFile(context.Background(), req); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := svc.ParseFile(context.Background(), req); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGRPCParseFileUncached(b *testing.B) {
	svc, err := sysmlgrpc.NewService(4, "bench")
	if err != nil {
		b.Fatal(err)
	}
	src := string(vehicle(b))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: src + fmt.Sprintf("\n// %d\n", i)}}
		if _, err := svc.ParseFile(context.Background(), req); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGRPCEvaluate(b *testing.B) {
	svc, err := sysmlgrpc.NewService(16, "bench")
	if err != nil {
		b.Fatal(err)
	}
	src := string(syntheticSource(b))
	pr, err := svc.ParseFile(context.Background(), &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: src}})
	if err != nil {
		b.Fatal(err)
	}
	req := &pb.EvaluateRequest{ModelHash: pr.ModelHash, Expression: "Perf::Calc1(2.0, 3.0)"}
	res, err := svc.Evaluate(context.Background(), req)
	if err != nil || res.Error != "" {
		b.Fatalf("%v %s", err, res.GetError())
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := svc.Evaluate(context.Background(), req); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGRPCVerifyConstraint(b *testing.B) {
	svc, err := sysmlgrpc.NewService(16, "bench")
	if err != nil {
		b.Fatal(err)
	}
	src := string(syntheticSource(b))
	pr, err := svc.ParseFile(context.Background(), &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: src}})
	if err != nil {
		b.Fatal(err)
	}
	req := &pb.VerifyConstraintRequest{ModelHash: pr.ModelHash, SymbolId: "Perf::Comp1::massOK", SubjectSymbolId: "Perf::inst1"}
	res, err := svc.VerifyConstraint(context.Background(), req)
	if err != nil {
		b.Fatal(err)
	}
	if res.Error != "" {
		b.Fatalf("%s", res.Error)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := svc.VerifyConstraint(context.Background(), req); err != nil {
			b.Fatal(err)
		}
	}
}

// Over HTTP (Connect protocol) to include transport + protobuf marshal cost.
func BenchmarkConnectEvaluateHTTP(b *testing.B) {
	svc, err := sysmlgrpc.NewService(16, "bench")
	if err != nil {
		b.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle(protoconnect.NewSysMLServiceHandler(sysmlgrpc.NewConnectAdapter(svc)))
	server := httptest.NewServer(mux)
	defer server.Close()
	client := protoconnect.NewSysMLServiceClient(http.DefaultClient, server.URL)
	src := string(vehicle(b))
	pr, err := client.ParseFile(context.Background(), connect.NewRequest(&pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: src}}))
	if err != nil {
		b.Fatal(err)
	}
	req := &pb.EvaluateRequest{ModelHash: pr.Msg.ModelHash, Expression: "1 + 2"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := client.Evaluate(context.Background(), connect.NewRequest(req)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkConnectParseFileHTTPCached(b *testing.B) {
	svc, err := sysmlgrpc.NewService(16, "bench")
	if err != nil {
		b.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle(protoconnect.NewSysMLServiceHandler(sysmlgrpc.NewConnectAdapter(svc)))
	server := httptest.NewServer(mux)
	defer server.Close()
	client := protoconnect.NewSysMLServiceClient(http.DefaultClient, server.URL)
	src := string(vehicle(b))
	req := &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: src}}
	if _, err := client.ParseFile(context.Background(), connect.NewRequest(req)); err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := client.ParseFile(context.Background(), connect.NewRequest(req)); err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------- per-step scaling on a tiny model

func tinyModel(loop, count int) []byte {
	return []byte(fmt.Sprintf(`package T {
    private import ScalarValues::*;
    action proc {
        attribute total : Integer = 0;
        first start;
        action iterate {
            for i in 1..%d {
                assign total := total + i;
            }
        }
        done;
        succession first start then iterate;
        succession first iterate then done;
    }
    action chainAct {
        attribute total : Integer = 0;
        first start;
        %s
        done;
        succession first start then a0;
        %s
        succession first a%d then done;
    }
    state def SM {
        attribute count : Integer = 0;
        entry; then s0;
        state s0;
        accept after 1 [SI::s] if count < %d then s1;
        accept after 1 [SI::s] if count >= %d then done;
        state s1 {
            entry assign count := count + 1;
        }
        accept after 1 [SI::s] then s0;
    }
}
`, loop, chainActions(loop), chainSuccessions(loop), loop-1, count, count))
}

func chainActions(n int) string {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, "action a%d { assign total := total + %d; }\n        ", i, i)
	}
	return sb.String()
}

func chainSuccessions(n int) string {
	var sb strings.Builder
	for i := 0; i+1 < n; i++ {
		fmt.Fprintf(&sb, "succession first a%d then a%d;\n        ", i, i+1)
	}
	return sb.String()
}

func BenchmarkActionLoop(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("for%d", n), func(b *testing.B) {
			r := newRT(b, tinyModel(n, 10))
			act := r.sym(b, "T::proc")
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				out, err := r.ctx.ExecuteAction(act)
				if err != nil {
					b.Fatal(err)
				}
				_ = out
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(n), "ns/iter")
		})
	}
}

// Chain of n nested action usages joined by successions: one token move per action.
func BenchmarkActionChain(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("chain%d", n), func(b *testing.B) {
			r := newRT(b, tinyModel(n, 10))
			act := r.sym(b, "T::chainAct")
			var steps int
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				exec, err := r.ctx.CreateActionExecutor(act)
				if err != nil {
					b.Fatal(err)
				}
				steps = 0
				for exec.State() != runtime.StateCompleted {
					if err := exec.Step(); err != nil {
						b.Fatal(err)
					}
					steps++
				}
			}
			b.ReportMetric(float64(steps), "steps")
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(steps), "ns/step")
		})
	}
}

func BenchmarkStateLoop(b *testing.B) {
	for _, n := range []int{50, 500, 5000} {
		b.Run(fmt.Sprintf("count%d", n), func(b *testing.B) {
			r := newRT(b, tinyModel(10, n))
			sm := r.sym(b, "T::SM")
			var events int
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				exec, err := r.ctx.CreateStateExecutor(sm)
				if err != nil {
					b.Fatal(err)
				}
				events = 0
				for exec.State() == runtime.StateRunning && exec.HasPendingWork() {
					if err := exec.ProcessNextEvent(); err != nil {
						b.Fatal(err)
					}
					events++
				}
				if exec.State() != runtime.StateCompleted {
					b.Fatalf("state %v after %d events", exec.State(), events)
				}
			}
			b.ReportMetric(float64(events), "events")
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(events), "ns/event")
		})
	}
}

func BenchmarkLowerChain(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("chain%d", n), func(b *testing.B) {
			r := newRT(b, tinyModel(n, 10))
			act := r.sym(b, "T::chainAct")
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := lower.ToActionGraph(act.Decl, act.Scope); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

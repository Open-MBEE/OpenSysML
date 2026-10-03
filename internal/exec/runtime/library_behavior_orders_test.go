package runtime

import (
	goruntime "runtime"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

const libraryBehaviorOrderFile = "behavior-orders.sysml"

const libraryBehaviorOrderModel = `package test {
	part def Bot {
		perform action m {
			first start;
			then done;
		}
		perform action g {
			first start;
			then done;
		}
	}
	part b : Bot;
	first b.g then b.m;
}`

func TestLibraryBehaviorOrdersShippedSet(t *testing.T) {
	byRoot := libraryBehaviorOrders(libs.SharedBase())
	var orders []lower.BehaviorOrder
	for _, rootOrders := range byRoot {
		orders = append(orders, rootOrders...)
	}
	if len(orders) != 1 {
		t.Fatalf("shipped library has %d behavior orders, want exactly one: %+v", len(orders), orders)
	}
	order := orders[0]
	if order.Name != "causalOrdering" {
		t.Errorf("order name = %q, want causalOrdering", order.Name)
	}
	if order.File != "Domain Libraries/Cause and Effect/CausationConnections.sysml" {
		t.Errorf("order file = %q", order.File)
	}
	if order.Featuring == nil {
		t.Error("order has no featuring type")
	}
	if len(order.Earlier.Path) != 2 {
		t.Errorf("earlier path length = %d, want 2", len(order.Earlier.Path))
	}
	if len(order.Later.Path) != 1 {
		t.Errorf("later path length = %d, want 1", len(order.Later.Path))
	}
	if order.Refusal == nil {
		t.Fatal("order has no refusal")
	}
	if order.Refusal.Code != "succession-orders-nothing" {
		t.Errorf("refusal code = %q, want succession-orders-nothing", order.Refusal.Code)
	}
}

func TestLibraryBehaviorOrdersSnapshotBaseMemoized(t *testing.T) {
	base := libs.SharedBase()
	before := libraryOrderWalks.Load()
	libraryBehaviorOrders(base)
	afterFirst := libraryOrderWalks.Load()
	if walks := afterFirst - before; walks > 1 {
		t.Fatalf("first lookup added %d library order walks, want at most one", walks)
	}

	libraryBehaviorOrders(base)
	if got := libraryOrderWalks.Load(); got != afterFirst {
		t.Fatalf("second lookup added %d library order walks, want zero", got-afterFirst)
	}
}

func TestLibraryBehaviorOrdersSharedAcrossModels(t *testing.T) {
	base, _ := libs.FrozenLibrary()
	if !base.Frozen() {
		t.Fatal("FrozenLibrary returned an unfrozen index")
	}
	withRegisteredScopes := libraryBehaviorOrderContext(t, base, true)
	fromIndexDocuments := libraryBehaviorOrderContext(t, base, false)

	before := libraryOrderWalks.Load()
	contexts := []*Context{withRegisteredScopes, fromIndexDocuments}
	for _, ctx := range contexts {
		ctx.behaviorOrders()
	}
	if got := libraryOrderWalks.Load() - before; got != 1 {
		t.Fatalf("library order walks increased by %d, want exactly one", got)
	}

	for _, ctx := range contexts {
		got := ctx.behaviorOrders()
		want := lower.BehaviorOrders(ctx.model.semantics, libraryBehaviorOrderRoots(ctx)...)
		assertLibraryBehaviorOrdersEqual(t, got, want)

		hasUserOrder := false
		for _, order := range got {
			if order.File == libraryBehaviorOrderFile && order.Refusal == nil {
				hasUserOrder = true
				break
			}
		}
		if !hasUserOrder {
			t.Fatalf("orders contain no non-refused user order: %+v", got)
		}
	}
}

func TestLibraryBehaviorOrdersReleasedWithIndex(t *testing.T) {
	done := make(chan struct{})
	cacheLibraryBehaviorOrdersForCleanup(t, done)
	for range 50 {
		goruntime.GC()
		select {
		case <-done:
			return
		case <-time.After(200 * time.Millisecond):
		}
	}
	t.Fatal("frozen library index remained reachable through the behavior order cache")
}

func BenchmarkBehaviorOrdersLibraryModel(b *testing.B) {
	base := libs.SharedBase()
	src, file := parseLibraryBehaviorOrderDocument(b)
	libraryBehaviorOrders(base)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		idx := symbols.NewOverlay(base)
		idx.AddDocument(src.Name(), file)
		idx.ExpandWildcardImports()
		resolver := resolve.New(idx)
		semanticModel := semantics.NewModel(resolver)
		resolver.SetModel(semanticModel)
		ctx := NewContext(typedModel(semanticModel, resolver), 10000)
		ctx.behaviorOrders()
	}
}

func libraryBehaviorOrderContext(t *testing.T, base *symbols.Index, registerScopes bool) *Context {
	t.Helper()
	idx := symbols.NewOverlay(base)
	src, file := parseLibraryBehaviorOrderDocument(t)
	idx.AddDocument(src.Name(), file)
	idx.ExpandWildcardImports()

	resolver := resolve.New(idx)
	semanticModel := semantics.NewModel(resolver)
	resolver.SetModel(semanticModel)
	model := typedModel(semanticModel, resolver)
	if registerScopes {
		for _, name := range idx.Documents() {
			model.RegisterScope(idx.DocumentRoot(name))
		}
	}
	return NewContext(model, 10000)
}

func cacheLibraryBehaviorOrdersForCleanup(t *testing.T, done chan struct{}) {
	t.Helper()
	base, _ := libs.FrozenLibrary()
	ctx := libraryBehaviorOrderContext(t, base, false)
	ctx.behaviorOrders()
	goruntime.AddCleanup(base, func(ch chan struct{}) { close(ch) }, done)
}

func parseLibraryBehaviorOrderDocument(t testing.TB) (*source.SourceFile, *ast.RootNamespace) {
	t.Helper()
	src := source.New(libraryBehaviorOrderFile, []byte(libraryBehaviorOrderModel))
	p := parser.New(src)
	file := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	return src, file
}

func libraryBehaviorOrderRoots(ctx *Context) []*symbols.Scope {
	roots := append([]*symbols.Scope(nil), ctx.model.scopes...)
	if len(roots) == 0 && ctx.model.resolver != nil && ctx.model.resolver.Index() != nil {
		index := ctx.model.resolver.Index()
		for _, document := range index.Documents() {
			roots = append(roots, index.DocumentRoot(document))
		}
	}
	return roots
}

func assertLibraryBehaviorOrdersEqual(t *testing.T, got, want []lower.BehaviorOrder) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Fatalf("order slice nilness differs: got nil=%t, want nil=%t", got == nil, want == nil)
	}
	if len(got) != len(want) {
		t.Fatalf("order count = %d, want %d", len(got), len(want))
	}
	for i, actual := range got {
		expected := want[i]
		if actual.Decl != expected.Decl {
			t.Errorf("order %d declaration differs", i)
		}
		if actual.Name != expected.Name {
			t.Errorf("order %d name = %q, want %q", i, actual.Name, expected.Name)
		}
		if actual.File != expected.File {
			t.Errorf("order %d file = %q, want %q", i, actual.File, expected.File)
		}
		if actual.Span != expected.Span {
			t.Errorf("order %d span = %+v, want %+v", i, actual.Span, expected.Span)
		}
		if actual.Featuring != expected.Featuring {
			t.Errorf("order %d featuring symbol differs", i)
		}
		assertLibraryBehaviorOrderPath(t, i, "earlier", actual.Earlier.Path, expected.Earlier.Path)
		assertLibraryBehaviorOrderPath(t, i, "later", actual.Later.Path, expected.Later.Path)
		if actual.Earlier.Multiplicity != expected.Earlier.Multiplicity {
			t.Errorf("order %d earlier multiplicity differs", i)
		}
		if actual.Earlier.Span != expected.Earlier.Span {
			t.Errorf("order %d earlier span = %+v, want %+v", i, actual.Earlier.Span, expected.Earlier.Span)
		}
		if actual.Later.Multiplicity != expected.Later.Multiplicity {
			t.Errorf("order %d later multiplicity differs", i)
		}
		if actual.Later.Span != expected.Later.Span {
			t.Errorf("order %d later span = %+v, want %+v", i, actual.Later.Span, expected.Later.Span)
		}
		if (actual.Refusal == nil) != (expected.Refusal == nil) {
			t.Errorf("order %d refusal nilness differs", i)
		} else if actual.Refusal != nil {
			if actual.Refusal.Code != expected.Refusal.Code {
				t.Errorf("order %d refusal code = %q, want %q", i, actual.Refusal.Code, expected.Refusal.Code)
			}
			if actual.Refusal.Reason != expected.Refusal.Reason {
				t.Errorf("order %d refusal reason = %q, want %q", i, actual.Refusal.Reason, expected.Refusal.Reason)
			}
			if actual.Refusal.Span != expected.Refusal.Span {
				t.Errorf("order %d refusal span = %+v, want %+v", i, actual.Refusal.Span, expected.Refusal.Span)
			}
		}
	}
}

func assertLibraryBehaviorOrderPath(t *testing.T, order int, end string, got, want []*symbols.Symbol) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Errorf("order %d %s path nilness differs", order, end)
		return
	}
	if len(got) != len(want) {
		t.Errorf("order %d %s path length = %d, want %d", order, end, len(got), len(want))
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("order %d %s path symbol %d differs", order, end, i)
		}
	}
}

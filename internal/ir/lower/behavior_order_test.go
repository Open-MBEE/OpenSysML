package lower

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestBehaviorOrdersCollectsOutsideBodySuccessions(t *testing.T) {
	text := `package P {
		part def Robot { perform action move; perform action grip; first grip then move; }
		part r : Robot;
		first r::move then r::grip;
		part part1 { action action1; }
		requirement requirement1;
		first part1::action1 then requirement1;
		part b { action g; action m; }
		first b.g then b.m;
		part p1 { action a; }
		part p2 { action b; }
		first p1::a then p2::b;
	}`
	parserInstance := parser.New(source.New("orders.sysml", []byte(text)))
	file := parserInstance.ParseFile()
	if len(parserInstance.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %+v", parserInstance.Diagnostics)
	}
	index := symbols.NewIndex()
	index.AddDocument("orders.sysml", file)
	resolver := resolve.New(index)
	model := semantics.NewModel(resolver)
	resolver.SetModel(model)
	root := index.DocumentRoot("orders.sysml")

	orders := BehaviorOrders(model, root)
	if len(orders) != 5 {
		t.Fatalf("BehaviorOrders returned %d orders, want 5", len(orders))
	}
	byName := make(map[string]BehaviorOrder)
	for _, order := range orders {
		byName[behaviorOrderTestEnds(order)] = order
	}
	for _, want := range []string{
		"grip then move",
		"move then grip",
		"action1 then requirement1",
		"b.g then b.m",
		"a then b",
	} {
		if _, ok := byName[want]; !ok {
			t.Errorf("missing order %q; got %v", want, behaviorOrderTestKeys(byName))
		}
	}
	if refusal := byName["action1 then requirement1"].Refusal; refusal == nil ||
		refusal.Code != "succession-orders-nothing" ||
		!strings.Contains(refusal.Reason, "requirement1") ||
		!strings.Contains(refusal.Reason, "verdicts") {
		t.Errorf("requirement order refusal = %+v, want precise succession-orders-nothing refusal", refusal)
	}
	if refusal := byName["a then b"].Refusal; refusal == nil ||
		refusal.Code != "connector-type-featuring" {
		t.Errorf("unrelated features refusal = %+v, want connector-type-featuring", refusal)
	}
	if got := byName["b.g then b.m"].Earlier.Path; len(got) != 2 ||
		got[0].Name != "b" || got[1].Name != "g" {
		t.Errorf("b.g path = %v, want [b g]", behaviorOrderTestPath(got))
	}
	if got := byName["move then grip"].Featuring; got == nil || got.Name != "Robot" {
		t.Errorf("qualified succession featuring type = %v, want Robot", got)
	}
}

func behaviorOrderTestEnds(order BehaviorOrder) string {
	return strings.Join([]string{
		behaviorOrderTestPath(order.Earlier.Path),
		behaviorOrderTestPath(order.Later.Path),
	}, " then ")
}

func behaviorOrderTestPath(path []*symbols.Symbol) string {
	names := make([]string, 0, len(path))
	for _, sym := range path {
		names = append(names, sym.Name)
	}
	return strings.Join(names, ".")
}

func behaviorOrderTestKeys(orders map[string]BehaviorOrder) []string {
	keys := make([]string, 0, len(orders))
	for key := range orders {
		keys = append(keys, key)
	}
	return keys
}

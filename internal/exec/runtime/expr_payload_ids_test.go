package runtime

import (
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// stated fails unless id is tree in table.
func stated(t *testing.T, what string, table *lower.ExprTable, id program.ExprID, tree ast.Node) {
	t.Helper()
	if tree == nil {
		t.Fatalf("%s states no expression", what)
	}
	if entry := table.Entry(id); id == 0 || entry == nil || entry.Tree != tree {
		t.Fatalf("%s: id %d is not its expression in the table", what, id)
	}
}

const payloadFlowSrc = `
	action def outer {
		attribute limit : Integer = 3;
		out total : Integer;
		first start;
		then action count {
			assign total := limit + 1;
		}
		then decide d;
		if total > limit then over;
		action over { assign total := total * 2; }
		then done;
	}
`

// TestFlowPayloadsCarryIDsOfTheirOwnTable pins that what a flow states is behind ids in
// the table of that flow, a nested flow's in its own, and that a run reads them.
func TestFlowPayloadsCarryIDsOfTheirOwnTable(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, payloadFlowSrc))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	graph, err := ctx.model.actionGraph(sym)
	if err != nil {
		t.Fatal(err)
	}
	if graph.Exprs == nil {
		t.Fatal("the flow has no table")
	}
	stated(t, "attribute limit", graph.Exprs, graph.Attributes[0].ValueID, graph.Attributes[0].Value)
	guards, assigns := 0, 0
	var visit func(g *lower.ActionGraph)
	visit = func(g *lower.ActionGraph) {
		for _, node := range g.Nodes {
			for _, edge := range g.Edges[node] {
				if edge.Guard != nil {
					guards++
					stated(t, "guard", g.Exprs, edge.GuardID, edge.Guard)
				}
			}
			for _, stmt := range g.Bodies[node] {
				if assign, ok := stmt.(lower.Assign); ok {
					assigns++
					stated(t, "assignment", g.Exprs, assign.ValueID, assign.Value)
				}
			}
			if sub := g.Subflows[node]; sub != nil && sub.Graph != nil {
				if sub.Graph.Exprs == nil {
					t.Fatalf("nested flow has no table")
				}
				visit(sub.Graph)
			}
		}
	}
	visit(graph)
	if guards == 0 || assigns != 2 {
		t.Fatalf("found %d guards and %d assignments", guards, assigns)
	}

	out, err := ctx.ExecuteAction(sym)
	if err != nil || FormatValue(out["total"]) != "[8]" {
		t.Fatalf("total = %s, %v; want 8", FormatValue(out["total"]), err)
	}
}

// TestFlowPayloadWithoutAnIDIsRefused pins that a guard whose id is lost is an error of
// the run: the tree still beside it is not what the run falls back to.
func TestFlowPayloadWithoutAnIDIsRefused(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, payloadFlowSrc))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	graph, err := ctx.model.actionGraph(sym)
	if err != nil {
		t.Fatal(err)
	}
	lost := 0
	for _, node := range graph.Nodes {
		edges := graph.Edges[node]
		for i := range edges {
			if edges[i].Guard != nil {
				edges[i].GuardID = 0
				lost++
			}
		}
	}
	if lost == 0 {
		t.Fatal("the flow has no guard")
	}
	if _, err := ctx.ExecuteAction(sym); err == nil || !strings.Contains(err.Error(), "expression 0 is not in the table of its flow") {
		t.Fatalf("run with a guard without an id: %v", err)
	}
}

// TestPayloadIsReadInTheScopeOfTheRun pins that an id interned in one scope, read by a
// context of another, is evaluated as the tree reads there, and an id the table lacks is refused.
func TestPayloadIsReadInTheScopeOfTheRun(t *testing.T) {
	ctx, idx := contextForSource(t, loweredExprSrc)
	node := loweredValue(t, idx, "P::A::y")
	a, b := lookupOne(t, idx, "P::A").Scope, lookupOne(t, idx, "P::B").Scope
	table := lower.NewExprTable(ctx.model.resolver)
	id := table.Intern(node, a)
	if again := table.Intern(node, a); again != id || id == 0 {
		t.Fatalf("interned twice as %d then %d", id, again)
	}
	if other := table.Intern(node, b); other == id {
		t.Fatalf("one id %d for two scopes", id)
	}
	if val, err := newEvalContext(ctx, a).reading(table).evalPayload(id); err != nil || val.Const.Int != 2 {
		t.Fatalf("in A = %v, %v; want 2", val, err)
	}
	if val, err := newEvalContext(ctx, b).reading(table).evalPayload(id); err != nil || val.Const.Int != 11 {
		t.Fatalf("in B = %v, %v; want 11", val, err)
	}
	for _, missing := range []program.ExprID{0, 10000} {
		if _, err := newEvalContext(ctx, a).reading(table).evalPayload(missing); err == nil {
			t.Fatalf("id %d was evaluated", missing)
		}
	}
	if _, err := newEvalContext(ctx, a).reading(nil).evalPayload(id); err == nil {
		t.Fatal("an id was evaluated without a table")
	}
}

// TestRuntimePayloadsCarryIDsOfTheModelsTable pins the four payloads the runtime builds:
// a condition, a feature's default, an objective's value and a satisfy subject chain.
func TestRuntimePayloadsCarryIDsOfTheModelsTable(t *testing.T) {
	ctx, scope := analysisFixture(t, `
		package test {
			private import ScalarValues::*;
			private import TradeStudies::*;
			part def Tank {
				attribute level : Integer = 4;
			}
			requirement def Filled { subject tank : Tank; require constraint { tank.level > 2 } }
			part def Plant {
				part main : Tank;
				part site { part spare : Tank; }
				assert satisfy Filled by site.spare;
			}
			analysis def Trade {
				attribute cost : Integer;
				objective cheapest : MinimizeObjective {
					subject :>> selectedAlternative;
					in calc :>> eval { cost }
				}
			}
		}
	`)
	table := ctx.model.exprs
	tank, _ := scope.LookupLocal("Tank")

	filled, _ := scope.LookupLocal("Filled")
	conds := ctx.ConditionsOf(filled, filled.OwnerScope)
	if len(conds) != 1 {
		t.Fatalf("Filled has %d conditions, want 1", len(conds))
	}
	stated(t, "condition", table, conds[0].ExprID, conds[0].Expr)

	var defaults []string
	for _, feat := range ctx.FeaturesOf(tank) {
		if feat.DefaultValue != nil {
			defaults = append(defaults, feat.Name)
			stated(t, "default of "+feat.Name, table, feat.DefaultValueID, feat.DefaultValue)
		} else if feat.DefaultValueID != 0 {
			t.Fatalf("%s has id %d and no default", feat.Name, feat.DefaultValueID)
		}
	}
	if !slices.Contains(defaults, "level") {
		t.Fatalf("Tank has defaults %v, want one of level", defaults)
	}

	objs := objectivesOfCase(t, ctx, scope, "Trade")
	if len(objs) != 1 {
		t.Fatalf("Trade has %d objectives, want 1", len(objs))
	}
	stated(t, "objective", table, objs[0].ValueID, objs[0].Value)

	plant, _ := scope.LookupLocal("Plant")
	assertions := ctx.SatisfyAssertionsIn(plant.Scope)
	if len(assertions) != 1 || assertions[0].SubjectChain == nil {
		t.Fatalf("Plant has %d satisfy assertions with a subject chain", len(assertions))
	}
	stated(t, "satisfy subject", table, assertions[0].SubjectChainID, assertions[0].SubjectChain)

	inst, err := ctx.Instantiate(tank)
	if err != nil {
		t.Fatalf("instantiate Tank: %v", err)
	}
	if level := inst.FeatureValues["level"]; level == nil || level.Value.Const.Int != 4 {
		t.Fatalf("level = %v, want its default 4", level)
	}
	if _, err := ctx.Instantiate(plant); err != nil {
		t.Fatalf("instantiate Plant: %v", err)
	}
	if ok, err := ctx.EvaluateSatisfaction(assertions[0]); err != nil || !ok {
		t.Fatalf("satisfaction = %v, %v; want it to hold", ok, err)
	}
}

// TestMachinePayloadsCarryIDsOfItsTable pins that a machine's transition guards and
// change conditions, and an accept's change condition in a flow, are ids of their table.
func TestMachinePayloadsCarryIDsOfItsTable(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `
		state def Machine {
			attribute level : Integer = 0;
			entry; then idle;
			state idle;
			state busy;
			transition first idle accept when level > 1 if level < 9 then busy;
		}
	`)
	graph := exec.graph
	if graph.Exprs == nil {
		t.Fatal("the machine has no table")
	}
	stated(t, "attribute level", graph.Exprs, graph.Attributes[0].ValueID, graph.Attributes[0].Value)
	found := 0
	for _, transitions := range graph.Transitions {
		for _, trans := range transitions {
			change, ok := trans.Trigger.(*ast.ChangeEvent)
			if !ok {
				continue
			}
			found++
			stated(t, "change condition", graph.Exprs, trans.TriggerExprID, change.Condition)
			stated(t, "guard", graph.Exprs, trans.GuardID, trans.Guard)
		}
	}
	if found != 1 {
		t.Fatalf("found %d change transitions, want 1", found)
	}

	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		action def outer {
			attribute level : Integer = 2;
			first start;
			then action wait accept when level > 1;
			then done;
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	flow, err := ctx.model.actionGraph(sym)
	if err != nil {
		t.Fatal(err)
	}
	accepts := 0
	for _, accept := range flow.Accepts {
		accepts++
		stated(t, "accept condition", flow.Exprs, accept.TriggerExprID, accept.Trigger.(*ast.ChangeEvent).Condition)
	}
	if accepts != 1 {
		t.Fatalf("found %d accepts, want 1", accepts)
	}
	if _, err := ctx.ExecuteAction(sym); err != nil {
		t.Fatalf("run: %v", err)
	}
}

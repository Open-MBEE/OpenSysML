package main

import (
	"fmt"
	"log"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════╗")
	fmt.Println("║       SysML v2 Execution Runtime Demo                 ║")
	fmt.Println("║  Tier 1-3: Expression Eval + Instance Materialization ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Println()

	demo1_ExpressionEval()
	demo2_PartInstantiation()
	demo3_NestedParts()
}

func demo1_ExpressionEval() {
	fmt.Println("┌─ Demo 1: Expression Evaluation ────────────────────────┐")
	fmt.Println("│ Model:                                                 │")
	fmt.Println("│   attribute sum = 10 + 5 * 2;         (precedence)     │")
	fmt.Println("│   attribute area = 3.14 * 5.0 * 5.0;  (real math)      │")
	fmt.Println("│   attribute comparison = 10 > 5;      (comparison)     │")
	fmt.Println("└────────────────────────────────────────────────────────┘")

	code := `
		attribute sum = 10 + 5 * 2;
		attribute area = 3.14 * 5.0 * 5.0;
		attribute comparison = 10 > 5;
	`

	model, root := parseModel(code)
	ctx := runtime.NewContext(model, runtime.DefaultMaxSteps)

	// Evaluate sum
	sumSym, _ := root.LookupLocal("sum")
	sumUsage := sumSym.Decl.(*ast.Usage)
	sumVal, err := ctx.Eval(sumUsage.Value)
	if err != nil {
		fmt.Printf("❌ sum evaluation failed: %v\n\n", err)
		return
	}
	fmt.Printf("✓ sum = %d\n", sumVal.Const.Int)

	// Evaluate area
	areaSym, _ := root.LookupLocal("area")
	areaUsage := areaSym.Decl.(*ast.Usage)
	areaVal, err := ctx.Eval(areaUsage.Value)
	if err != nil {
		fmt.Printf("❌ area evaluation failed: %v\n\n", err)
		return
	}
	fmt.Printf("✓ area = %.2f\n", areaVal.Const.Real)

	// Evaluate comparison
	compSym, _ := root.LookupLocal("comparison")
	compUsage := compSym.Decl.(*ast.Usage)
	compVal, err := ctx.Eval(compUsage.Value)
	if err != nil {
		fmt.Printf("❌ comparison evaluation failed: %v\n\n", err)
		return
	}
	fmt.Printf("✓ comparison = %v\n", compVal.Const.Bool)
	fmt.Println()
}

func demo2_PartInstantiation() {
	fmt.Println("┌─ Demo 2: Part Instantiation ───────────────────────────┐")
	fmt.Println("│ Model:                                                 │")
	fmt.Println("│   part def Wheel {                                     │")
	fmt.Println("│     attribute diameter = 0.5;                          │")
	fmt.Println("│     attribute pressure = 32.0;                         │")
	fmt.Println("│   }                                                    │")
	fmt.Println("└────────────────────────────────────────────────────────┘")

	code := `
		part def Wheel {
			attribute diameter = 0.5;
			attribute pressure = 32.0;
		}
	`

	model, root := parseModel(code)
	ctx := runtime.NewContext(model, runtime.DefaultMaxSteps)

	// Instantiate Wheel
	wheelSym, _ := root.LookupLocal("Wheel")
	inst, err := ctx.Instantiate(wheelSym)
	if err != nil {
		fmt.Printf("❌ Instantiation failed: %v\n\n", err)
		return
	}

	fmt.Printf("✓ Created instance of Wheel (ID: %d)\n", inst.ID)

	// Access diameter
	diamFeatureValue, err := inst.GetFeatureValue(ctx, "diameter")
	if err != nil {
		fmt.Printf("❌ diameter access failed: %v\n", err)
	} else {
		fmt.Printf("  → diameter = %.1f meters\n", diamFeatureValue.Value.Const.Real)
	}

	// Access pressure
	pressFeatureValue, err := inst.GetFeatureValue(ctx, "pressure")
	if err != nil {
		fmt.Printf("❌ pressure access failed: %v\n", err)
	} else {
		fmt.Printf("  → pressure = %.1f PSI\n", pressFeatureValue.Value.Const.Real)
	}

	fmt.Println()
}

func demo3_NestedParts() {
	fmt.Println("┌─ Demo 3: Nested Parts (Composite Structure) ──────────┐")
	fmt.Println("│ Model:                                                 │")
	fmt.Println("│   part def Engine {                                    │")
	fmt.Println("│     attribute power = 300.0;                           │")
	fmt.Println("│   }                                                    │")
	fmt.Println("│   part def Vehicle {                                   │")
	fmt.Println("│     attribute wheels = 4;                              │")
	fmt.Println("│     part engine: Engine;                               │")
	fmt.Println("│   }                                                    │")
	fmt.Println("└────────────────────────────────────────────────────────┘")

	code := `
		part def Engine {
			attribute power = 300.0;
		}
		
		part def Vehicle {
			attribute wheels = 4;
			part engine: Engine;
		}
	`

	model, root := parseModel(code)
	ctx := runtime.NewContext(model, runtime.DefaultMaxSteps)

	// Instantiate Vehicle
	vehicleSym, _ := root.LookupLocal("Vehicle")
	inst, err := ctx.Instantiate(vehicleSym)
	if err != nil {
		fmt.Printf("❌ Instantiation failed: %v\n\n", err)
		return
	}

	fmt.Printf("✓ Created instance of Vehicle (ID: %d)\n", inst.ID)

	// Access wheels attribute
	wheelsFeatureValue, err := inst.GetFeatureValue(ctx, "wheels")
	if err != nil {
		fmt.Printf("❌ wheels access failed: %v\n", err)
	} else {
		fmt.Printf("  → wheels = %d\n", wheelsFeatureValue.Value.Const.Int)
	}

	// Access nested engine part
	engineFeatureValue, err := inst.GetFeatureValue(ctx, "engine")
	if err != nil {
		fmt.Printf("❌ engine access failed: %v\n", err)
	} else if engineFeatureValue.Value.Kind == runtime.ValInstance {
		fmt.Printf("  → engine = Instance(ID: %d) [nested part]\n", engineFeatureValue.Value.Instance)
		fmt.Printf("     (Nested instance fully materialized via lazy evaluation)\n")
	}

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("All demos completed successfully!")
	fmt.Println("Runtime capabilities demonstrated:")
	fmt.Println("  ✓ Expression evaluation (arithmetic, real, comparison)")
	fmt.Println("  ✓ Part instantiation with default values")
	fmt.Println("  ✓ Nested composite structures")
	fmt.Println("  ✓ Lazy feature value materialization")
}

// Helper: parse model and build semantic index
func parseModel(code string) (*runtime.Model, *symbols.Scope) {
	src := source.New("demo.sysml", []byte(code))
	p := parser.New(src)
	root := p.ParseFile()

	if len(p.Diagnostics) > 0 {
		log.Println("Parse errors:")
		for _, d := range p.Diagnostics {
			log.Printf("  %s", d.Message)
		}
		log.Fatal("Parse failed")
	}

	idx := symbols.NewIndex()
	idx.AddDocument("demo.sysml", root)
	rootScope := idx.DocumentRoot("demo.sysml")

	if rootScope == nil {
		log.Fatal("Root scope is nil")
	}

	resolver := resolve.New(idx)
	model := runtime.NewModel(semantics.NewModel(resolver), resolver)

	return model, rootScope
}

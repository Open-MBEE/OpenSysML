package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func TestCalcInvocation_SimpleCalc(t *testing.T) {
	// Use existing testdata/simple_calc.sysml
	// It contains:
	//   calc add { in x: Integer; in y: Integer; x + y }
	//   part def Result { attribute sum: Integer = add(3, 5); }

	path := filepath.Join("testdata", "simple_calc.sysml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", path, err)
	}

	// Parse
	file := parser.New(source.New(path, data)).ParseFile()

	// Build symbol index
	idx := symbols.NewIndex()
	idx.AddDocument(path, file)

	// Create resolver and semantic model
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)

	// Create runtime context
	ctx := NewContext(NewModel(model, resolver), 10000)

	// Find the default value expression from Result.sum
	// Navigate: root -> test package -> Result part def -> sum attribute -> default expr
	rootScope := idx.DocumentRoot(path)
	testPkg := rootScope.Children()[0] // "test" package scope
	resultSym, ok := testPkg.LookupLocal("Result")
	if !ok {
		t.Fatal("Result part def not found")
	}

	// Find sum attribute in Result's members
	resultDef, ok := resultSym.Decl.(*ast.Definition)
	if !ok {
		t.Fatal("Result is not a Definition")
	}

	var invExpr ast.Node
	for _, member := range resultDef.Members {
		// Unwrap Membership
		node := member
		if membership, ok := member.(*ast.Membership); ok {
			node = membership.Member
		}

		// Find usage named "sum" with default value
		if usage, ok := node.(*ast.Usage); ok && usage.Ident.Name == "sum" {
			invExpr = usage.Value
			break
		}
	}

	if invExpr == nil {
		t.Fatal("sum default value not found")
	}

	// Evaluate add(3, 5) with scope context from test package
	result, err := ctx.EvalWithScope(invExpr, testPkg)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	// Check result: add(3, 5) should return 8
	if result.Kind != ValConst {
		t.Fatalf("Expected ValConst, got %v", result.Kind)
	}
	if result.Const.Kind != semantics.ValInt {
		t.Fatalf("Expected ValInt, got %v", result.Const.Kind)
	}
	if result.Const.Int != 8 {
		t.Fatalf("Expected 8, got %d", result.Const.Int)
	}

	t.Logf("✓ add(3, 5) = %d", result.Const.Int)
}

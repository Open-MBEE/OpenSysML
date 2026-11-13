package view

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

func visitQuerySymbols(scope *symbols.Scope, visit func(*symbols.Symbol)) {
	if scope == nil {
		return
	}
	for _, list := range [][]*symbols.Symbol{scope.Members(), scope.AnonymousMembers()} {
		for _, sym := range list {
			visit(sym)
			visitQuerySymbols(sym.Scope, visit)
		}
	}
}

func TestDrawnQueryMatchesRender(t *testing.T) {
	files, err := filepath.Glob("testdata/*.sysml")
	if err != nil {
		t.Fatal(err)
	}
	var checked int
	for _, path := range files {
		file := filepath.Base(path)
		r, idx := loadFixture(t, file)
		visitQuerySymbols(idx.DocumentRoot(file), func(sym *symbols.Symbol) {
			if !semantics.IsView(sym) {
				return
			}
			_, _, err := r.KindOf(sym)
			if err != nil {
				return
			}
			t.Run(file+"/"+idx.GetFQN(sym), func(t *testing.T) {
				query, err := r.DrawnIn(sym)
				if err != nil {
					t.Fatal(err)
				}
				full, err := r.Render(sym)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(full.Drawn(), query) {
					t.Errorf("drawn query differs from full render:\nfull: %#v\nquery: %#v", full.Drawn(), query)
				}
				checked++
			})
		})
	}
	if checked < 50 {
		t.Fatalf("only checked %d views", checked)
	}
}

func sharedQueryModel(depth, width int) string {
	var src strings.Builder
	src.WriteString("package P { private import Views::*; private import DiagramLayout::*; part def D0 { part leaf; }\n")
	for level := 1; level <= depth; level++ {
		fmt.Fprintf(&src, "part def D%d {\n", level)
		for branch := range width {
			fmt.Fprintf(&src, "part p%d : D%d;\n", branch, level-1)
		}
		src.WriteString("}\n")
	}
	fmt.Fprintf(&src, "part root : D%d; view v { expose root; render asTreeDiagram; metadata Layout about root { x = 0; y = 0; } } }", depth)
	return src.String()
}

func TestTreeDrawnQuerySharesDeepDefinitions(t *testing.T) {
	r, idx := loadSources(t, []string{"shared.sysml"}, [][]byte{[]byte(sharedQueryModel(12, 8))})
	drawn, err := r.DrawnIn(lookup(t, idx, "P::v"))
	if err != nil {
		t.Fatal(err)
	}
	if !drawn.Node(lookup(t, idx, "P::D0::leaf")) {
		t.Fatal("deep inherited leaf is not drawn")
	}
	for level := 1; level <= 12; level++ {
		for branch := range 8 {
			if !drawn.Node(lookup(t, idx, fmt.Sprintf("P::D%d::p%d", level, branch))) {
				t.Fatalf("missing D%d::p%d", level, branch)
			}
		}
	}
}

func TestTreeDrawnQueryKeepsRecursionContexts(t *testing.T) {
	const src = `package P {
	private import Views::*;
	private import DiagramLayout::*;
	part def A { part b : B; part leafA; }
	part def B { part a : A; part leafB; }
	part def C { part a : A; part b : B; }
	part def D { part c : C; }
	part x : D; part y : C; part z : D;
	view v { expose x; expose y; expose z; expose A; expose B;
		metadata Layout about x::c { x = 0; y = 0; collapsed = true; }
		render asTreeDiagram;
	}
}`
	r, idx := loadSources(t, []string{"cycles.sysml"}, [][]byte{[]byte(src)})
	for _, depth := range []int{0, 1, 2, 3, 4} {
		r.SetDepth(depth)
		full, err := r.Render(lookup(t, idx, "P::v"))
		if err != nil {
			t.Fatal(err)
		}
		query, err := r.DrawnIn(lookup(t, idx, "P::v"))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(full.Drawn(), query) {
			t.Errorf("drawn sets differ at depth %d", depth)
		}
	}
}

func TestTreeDrawnQueryMatchesSharedGraphs(t *testing.T) {
	random := rand.New(rand.NewPCG(1, 2))
	for sample := range 64 {
		var src strings.Builder
		src.WriteString("package P { private import Views::*; private import DiagramLayout::*;\n")
		for def := range 4 {
			fmt.Fprintf(&src, "part def D%d { doc /* a definition */ part leaf;\n", def)
			for member := range 3 {
				fmt.Fprintf(&src, "part p%d : D%d;\n", member, random.IntN(4))
			}
			src.WriteString("}\n")
		}
		for root := range 3 {
			fmt.Fprintf(&src, "part r%d : D%d;\n", root, random.IntN(4))
		}
		src.WriteString("view v { expose r0; expose r1; expose r2;\n")
		for def := range 4 {
			if random.IntN(2) == 0 {
				fmt.Fprintf(&src, "expose D%d;\n", def)
			}
		}
		src.WriteString("metadata Layout about D0::p0 { x = 0; y = 0; collapsed = true; } render asTreeDiagram; } }")
		r, idx := loadSources(t, []string{"graph.sysml"}, [][]byte{[]byte(src.String())})
		r.SetDepth(1 + sample%5)
		full, err := r.Render(lookup(t, idx, "P::v"))
		if err != nil {
			t.Fatal(err)
		}
		query, err := r.DrawnIn(lookup(t, idx, "P::v"))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(full.Drawn(), query) {
			t.Fatalf("drawn sets differ for graph %d:\n%s", sample, src.String())
		}
	}
}

func TestTreeDrawnQueryKeepsMultipleTypeBarriers(t *testing.T) {
	r, idx := loadSources(t, []string{"multiple.sysml"}, [][]byte{[]byte(`
	package P {
		private import Views::*;
		part def A;
		part def B { part leaf; }
		part def H { part mixed : A, B { part c : B; } }
		part def Wrap { part h : H; }
		part plain : Wrap;
		part a : A { part wrap : Wrap; }
		view v { expose plain; expose a; render asTreeDiagram; }
	}`)})
	full, err := r.Render(lookup(t, idx, "P::v"))
	if err != nil {
		t.Fatal(err)
	}
	query, err := r.DrawnIn(lookup(t, idx, "P::v"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(full.Drawn(), query) {
		t.Fatal("drawn query differs when a recursion barrier suppresses marking another declared type")
	}
}

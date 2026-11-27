package resolve

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Environment variables of TestWildcardImportDifferential: the first names a file
// to write every observation to, the second such a file written by another revision.
const (
	importDifferentialOut  = "OPENSYSML_IMPORT_DIFFERENTIAL_OUT"
	importDifferentialBase = "OPENSYSML_IMPORT_DIFFERENTIAL_BASE"
)

const importDifferentialLibs = `
	package Q { package X { part def B; package Y { part def C; } private package Z { part def D; } } }
	package R { package X { part def E; package Y { part def F; } } }
	package Named { public import Q::X; }
	package Chained { public import Named::X; }
	package Mixed { private import Q::X; public import R::X; }
	package Both { private import Q::X; public import Q::X; }
	package Kept { private import Q::*; }
	package Wild { public import Q::*; }
	package WildNamed { public import Q::*; public import R::X; }
	package Aliased { alias X for Q::X; }
	package KeptWild { private import Q::*; public import R::*; }
	package Ring1 { public import Ring2::*; public import Q::X; }
	package Ring2 { public import Ring1::*; }
	package U { part def H; }
`

// importDifferentialModels enumerates, in a fixed order, every model the
// differential reads: N's `public import <target>::*` below each combination of
// what K and N import or alias, with and without an X owned further out, and
// with and without a second import in N, of a namespace nothing else names.
func importDifferentialModels(yield func(key, src string)) {
	var stmts []string
	for _, vis := range []string{"private", "public"} {
		for _, target := range []string{"Q", "Q::X", "R::X", "Named::X", "Chained::X", "Mixed::X", "Aliased::X", "Wild::X",
			"Q::*", "R::*", "Named::*", "Chained::*", "Mixed::*", "Kept::*", "Wild::*", "WildNamed::*", "Aliased::*", "KeptWild::*", "Ring2::*"} {
			stmts = append(stmts, vis+" import "+target+";")
		}
		stmts = append(stmts, vis+" alias X for Q::X;", vis+" alias X for R::X;")
	}
	inK := []string{""}
	for i, a := range stmts {
		inK = append(inK, a)
		for _, b := range stmts[i+1:] {
			if strings.HasPrefix(a, "private") && strings.HasPrefix(b, "private") {
				inK = append(inK, a+" "+b)
			}
		}
	}
	inN := []string{"", "private import R::X;", "private import R::*;", "private import Q::*;", "private alias X for R::X;"}
	targets := []string{"X", "X::Y", "X::Z", "Named::X", "Chained::X::Y", "Mixed::X", "Both::X", "Aliased::X::Y", "Wild::X::Y", "KeptWild::X", "Named::X::Z", "Ring2::X"}
	for _, second := range []string{"", " public import U::*;"} {
		for _, own := range []string{"package X { part def A; package Y { part def G; } }", ""} {
			for _, k := range inK {
				for _, n := range inN {
					for _, target := range targets {
						yield(fmt.Sprintf("second=%t|own=%t|K{%s}|N{%s}|%s", second != "", own != "", k, n, target),
							importDifferentialLibs+"package P { "+own+" package K { "+k+" package N { "+n+" public import "+target+"::*;"+second+" } package S { } } }")
					}
				}
			}
		}
	}
}

// What N exposes through `public import <target>::*`, by name lookup and through
// the index, is what name lookup finds under the import's target, over every
// enumerated combination of imports and aliases. The models that still disagree
// are the known gap of a private and a public wildcard import of one name.
//
// With OPENSYSML_IMPORT_DIFFERENTIAL_OUT the observations are written to that
// file; with OPENSYSML_IMPORT_DIFFERENTIAL_BASE, a file another revision wrote,
// no name that revision reads as expected may be read otherwise here.
func TestWildcardImportDifferential(t *testing.T) {
	var out *bufio.Writer
	if path := os.Getenv(importDifferentialOut); path != "" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		out = bufio.NewWriter(f)
		defer func() {
			if err := out.Flush(); err != nil {
				t.Error(err)
			}
			if err := f.Close(); err != nil {
				t.Error(err)
			}
		}()
	}
	base := map[string]string{}
	if path := os.Getenv(importDifferentialBase); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for line := range strings.Lines(string(data)) {
			if name, read, ok := strings.Cut(strings.TrimSuffix(line, "\n"), "\t"); ok {
				base[name] = read
			}
		}
	}

	models, regressed := 0, 0
	wrong := map[string]bool{}
	importDifferentialModels(func(key, src string) {
		models++
		p := parser.New(source.New("d.sysml", []byte(src)))
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("%s: parse diagnostics: %v", key, p.Diagnostics)
		}
		idx := symbols.NewIndexFromDoc("d.sysml", root)
		idx.ExpandWildcardImports()
		r := New(idx)
		r.ResolveDocument("d.sysml", root)
		outside, inN := scopeNamed(t, r, "d.sysml", "S"), scopeNamed(t, r, "d.sysml", "N")
		target := key[strings.LastIndex(key, "|")+1:]
		targetSym, targetOK := r.ResolveQualified(inN, qn(false, strings.Split(target, "::")...))
		for _, member := range []string{"A", "B", "C", "D", "E", "F", "G", "H", "Y", "Z"} {
			want := ""
			if member == "H" {
				if strings.HasPrefix(key, "second=true") {
					want = "U::H"
				}
			} else if targetOK && targetSym != nil {
				path := append(strings.Split(symbols.FQNOf(targetSym), "::"), member)
				if sym, ok := r.ResolveQualified(outside, qn(true, path...)); ok && sym != nil {
					want = symbols.FQNOf(sym)
				}
			}
			var reads []string
			if sym, ok := r.ResolveQualified(outside, qn(false, "N", member)); ok && sym != nil {
				reads = append(reads, symbols.FQNOf(sym))
			} else {
				reads = append(reads, "")
			}
			agrees := reads[0] == want
			held := idx.LookupQualified("P::K::N::" + member)
			// The index holds what name lookup reads.
			if want != "" && len(held) == 0 {
				agrees = false
			}
			for _, sym := range held {
				reads = append(reads, symbols.FQNOf(sym))
				agrees = agrees && symbols.FQNOf(sym) == want
			}
			name, read := key+"|"+member, strings.Join(reads, ",")
			if out != nil {
				fmt.Fprintf(out, "%s\t%s\n", name, read)
			}
			if agrees {
				continue
			}
			wrong[key] = true
			// The other revision read the name as expected here: lookup alone, or lookup and the index.
			// Where the index here only lacks the name, the base revision's index held it.
			lacks := reads[0] == want && len(held) == 0
			if was, ok := base[name]; ok && (was == want+","+want || (was == want && !lacks)) {
				regressed++
				t.Errorf("%s reads %q, the base revision %q", name, read, was)
			}
		}
	})

	const wantModels, knownGap = 60720, 16
	if models != wantModels {
		t.Errorf("enumerated %d models, want %d", models, wantModels)
	}
	var unexpected []string
	for key := range wrong {
		if strings.Contains(key, "|N{}|") && strings.Contains(key, "KeptWild") {
			continue
		}
		unexpected = append(unexpected, key)
	}
	sort.Strings(unexpected)
	for _, key := range unexpected {
		t.Errorf("%s: N does not expose what name lookup finds under the import's target", key)
	}
	if len(wrong)-len(unexpected) != knownGap {
		t.Errorf("%d models in the private-and-public wildcard gap, want %d", len(wrong)-len(unexpected), knownGap)
	}
	if len(base) != 0 {
		t.Logf("%d models; %d disagree here; %d names the base revision read as expected are read otherwise", models, len(wrong), regressed)
	}
}

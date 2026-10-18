package runtime

import (
	"strings"
	"testing"
)

const sweepPrefixPrelude = `package Probe {
	private import ScalarValues::*;
	part def P2 { attribute v : Integer = 7; }
	item def I1;
	port def Pt { attribute pv : Integer; }
	action def A1;
	state def S1;
	requirement def R1;
	constraint def C1 { true }
	occurrence def O1;
	enum def E1 { a; b; }
	calc def K1 { return : Integer = 4; }
	connection def Cn1 { end e1 : P2; end e2 : P2; }
	attribute def At1;
`

// kind → declaration tail after "x" (type and any clause), value form for a binding, and whether value-typed
type sweepPrefixKind struct {
	kw, typ, tail, value string
}

var sweepPrefixKinds = []sweepPrefixKind{
	{"attribute", "Integer", "", "1"},
	{"attribute", "At1", "", ""},
	{"enum", "E1", "", "E1::a"},
	{"calc", "K1", "", ""},
	{"part", "P2", "", "sub1"},
	{"item", "I1", "", "it1"},
	{"port", "Pt", "", "pt1"},
	{"action", "A1", "", "act1"},
	{"state", "S1", "", "st1"},
	{"requirement", "R1", "", "rq1"},
	{"constraint", "C1", "", "cq1"},
	{"occurrence", "O1", "", "oc1"},
	{"connection", "Cn1", " connect sub1 to sub2", ""},
}

const sweepPrefixSiblings = `
		part sub1 : P2; part sub2 : P2; item it1 : I1; port pt1 : Pt; action act1 : A1; state st1 : S1;
		requirement rq1 : R1; constraint cq1 : C1; occurrence oc1 : O1;`

type sweepPrefix struct {
	name string
	// decl builds the usage declaration(s) inside part def D for kind k; "" skips the cell
	decl func(k sweepPrefixKind) string
	// extra adds declarations after `part d : D;` (another usage or def)
	extra string
	reads []string
}

func valOr(k sweepPrefixKind, op string) string {
	if k.value == "" {
		return ""
	}
	return " " + op + " " + k.value
}

var sweepPrefixes = []sweepPrefix{
	{"(none)", func(k sweepPrefixKind) string { return k.kw + " x : " + k.typ + k.tail + ";" }, "", []string{"Probe::d.x"}},
	{"in", func(k sweepPrefixKind) string { return "in " + k.kw + " x : " + k.typ + k.tail + ";" }, "", []string{"Probe::d.x"}},
	{"out", func(k sweepPrefixKind) string { return "out " + k.kw + " x : " + k.typ + k.tail + ";" }, "", []string{"Probe::d.x"}},
	{"inout", func(k sweepPrefixKind) string { return "inout " + k.kw + " x : " + k.typ + k.tail + ";" }, "", []string{"Probe::d.x"}},
	{"ref (unbound)", func(k sweepPrefixKind) string { return "ref " + k.kw + " x : " + k.typ + k.tail + ";" }, "", []string{"Probe::d.x"}},
	{"ref (bound =)", func(k sweepPrefixKind) string {
		if k.value == "" {
			return ""
		}
		return "ref " + k.kw + " x : " + k.typ + k.tail + " = " + k.value + ";"
	}, "", []string{"Probe::d.x", "Probe::d.x == Probe::d.sub1"}},
	{"abstract [0..*]", func(k sweepPrefixKind) string { return "abstract " + k.kw + " x : " + k.typ + "[0..*]" + k.tail + ";" }, "", []string{"Probe::d.x"}},
	{"abstract [0..*] + subsetter", func(k sweepPrefixKind) string {
		return "abstract " + k.kw + " x : " + k.typ + "[0..*]" + k.tail + "; " + k.kw + " y : " + k.typ + k.tail + " :> x;"
	}, "", []string{"Probe::d.x", "Probe::d.y"}},
	{"variation", func(k sweepPrefixKind) string {
		return "variation " + k.kw + " x : " + k.typ + k.tail + " { variant " + k.kw + " va : " + k.typ + k.tail + "; variant " + k.kw + " vb : " + k.typ + k.tail + "; }"
	}, "", []string{"Probe::d.x", "Probe::d.x.va"}},
	{"constant", func(k sweepPrefixKind) string {
		return "constant " + k.kw + " x : " + k.typ + k.tail + valOr(k, "=") + ";"
	}, "", []string{"Probe::d.x"}},
	{"readonly", func(k sweepPrefixKind) string {
		return "readonly " + k.kw + " x : " + k.typ + k.tail + valOr(k, "=") + ";"
	}, "", []string{"Probe::d.x"}},
	{"derived", func(k sweepPrefixKind) string {
		return "derived " + k.kw + " x : " + k.typ + k.tail + valOr(k, "=") + ";"
	}, "", []string{"Probe::d.x"}},
	{"end", func(k sweepPrefixKind) string { return "end " + k.kw + " x : " + k.typ + k.tail + ";" }, "", []string{"Probe::d.x"}},
	{"portion", func(k sweepPrefixKind) string { return "portion " + k.kw + " x : " + k.typ + k.tail + ";" }, "", []string{"Probe::d.x"}},
	{"individual", func(k sweepPrefixKind) string { return "individual " + k.kw + " x : " + k.typ + k.tail + ";" }, "", []string{"Probe::d.x"}},
	{"[2..4]", func(k sweepPrefixKind) string { return k.kw + " x : " + k.typ + "[2..4]" + k.tail + ";" }, "", []string{"Probe::d.x"}},
	{"[*] = (3,1,3) (unique default)", func(k sweepPrefixKind) string {
		if k.typ != "Integer" {
			return ""
		}
		return k.kw + " x : " + k.typ + "[*] = (3, 1, 3);"
	}, "", []string{"Probe::d.x"}},
	{"[*] ordered nonunique = (3,1,3)", func(k sweepPrefixKind) string {
		if k.typ != "Integer" {
			return ""
		}
		return k.kw + " x : " + k.typ + "[*] ordered nonunique = (3, 1, 3);"
	}, "", []string{"Probe::d.x"}},
	{"[*] ordered = (3,1,3)", func(k sweepPrefixKind) string {
		if k.typ != "Integer" {
			return ""
		}
		return k.kw + " x : " + k.typ + "[*] ordered = (3, 1, 3);"
	}, "", []string{"Probe::d.x"}},
	{"[*] nonunique = (3,1,3)", func(k sweepPrefixKind) string {
		if k.typ != "Integer" {
			return ""
		}
		return k.kw + " x : " + k.typ + "[*] nonunique = (3, 1, 3);"
	}, "", []string{"Probe::d.x"}},
	{"[2] = (1,2,3) (too many)", func(k sweepPrefixKind) string {
		if k.typ != "Integer" {
			return ""
		}
		return k.kw + " x : " + k.typ + "[2] = (1, 2, 3);"
	}, "", []string{"Probe::d.x"}},
	{"default", func(k sweepPrefixKind) string {
		if k.value == "" {
			return ""
		}
		return k.kw + " x : " + k.typ + k.tail + " default " + k.value + ";"
	}, "", []string{"Probe::d.x"}},
	{"default + :>> override", func(k sweepPrefixKind) string {
		if k.value == "" {
			return ""
		}
		return k.kw + " x : " + k.typ + k.tail + " default " + k.value + ";"
	}, "part d2 : D { :>> x = SUB2; }", []string{"Probe::d.x", "Probe::d2.x", "Probe::d2.x == Probe::d2.sub2"}},
	{":= initial", func(k sweepPrefixKind) string {
		if k.value == "" {
			return ""
		}
		return k.kw + " x : " + k.typ + k.tail + " := " + k.value + ";"
	}, "", []string{"Probe::d.x"}},
	{":>> redefinition masks", func(k sweepPrefixKind) string {
		if k.value == "" {
			return ""
		}
		return k.kw + " x : " + k.typ + k.tail + " = " + k.value + ";"
	}, "part def D2 :> D { " + "KW :>> x = SUB2; } part d2 : D2;", []string{"Probe::d.x", "Probe::d2.x", "Probe::d2.x == Probe::d2.sub2"}},
	{":> subsetting", func(k sweepPrefixKind) string {
		return k.kw + " x : " + k.typ + "[0..*]" + k.tail + "; " + k.kw + " y : " + k.typ + k.tail + " :> x;"
	}, "", []string{"Probe::d.x", "Probe::d.y"}},
	{"::> reference subsetting", func(k sweepPrefixKind) string {
		return k.kw + " x : " + k.typ + k.tail + "; ref " + k.kw + " y ::> x;"
	}, "", []string{"Probe::d.x", "Probe::d.y", "Probe::d.y == Probe::d.x"}},
}

// sweepPrefixUsageKinds builds one model per grammar prefix and usage kind and reads the
// usage it declares, one line per cell.
func sweepPrefixUsageKinds(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, pf := range sweepPrefixes {
		for _, k := range sweepPrefixKinds {
			decl := pf.decl(k)
			if decl == "" {
				continue
			}
			extra := strings.ReplaceAll(strings.ReplaceAll(pf.extra, "SUB2", sweepPrefixSibling(k)), "KW", k.kw)
			model := sweepPrefixPrelude + "\tpart def D {" + sweepPrefixSiblings + "\n\t\t" + decl + "\n\t}\n\tpart d : D;\n\t" + extra + "\n}\n"
			out = append(out, "prefix\t"+pf.name+"\t"+k.kw+" "+k.typ+"\t"+decl+"\t"+sweepGuarded(func() string {
				ctx, idx := libraryShapeContext(t, model)
				root := idx.DocumentRoot("<test>")
				pkgSym, _ := root.LookupLocal("Probe")
				var outs []string
				for _, r := range pf.reads {
					v, e := sweepEval(ctx, pkgSym.Scope, r)
					outs = append(outs, r+" => "+sweepOutcome(ctx, v, e))
				}
				return strings.Join(outs, " ;; ")
			}))
		}
	}
	return out
}

// sweepPrefixSibling names the sibling usage of k's kind a probe binds or subsets.
func sweepPrefixSibling(k sweepPrefixKind) string {
	switch k.typ {
	case "Integer":
		return "2"
	case "E1":
		return "E1::b"
	case "I1":
		return "it1"
	case "Pt":
		return "pt1"
	case "A1":
		return "act1"
	case "S1":
		return "st1"
	case "R1":
		return "rq1"
	case "C1":
		return "cq1"
	case "O1":
		return "oc1"
	}
	return "sub2"
}

package runtime

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

const sweepSystemsModel = `package Probe {
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;
	part def P1 {
		attribute a : Real = 2.5;
		attribute n : Integer;
		part sub : P2[2];
		part opt : P2[0..1];
		item cargo : I1;
		port pt : Pt;
		action act : A1;
		state st : S1;
		ref part other : P2;
		ref item otherItem : I1;
		ref part others : P2[0..*];
		requirement rq : R1 { subject s :>> s = sub#(1); }
		constraint cq : C1 { in x :>> x = 1.0; in y :>> y = 2.0; }
		calc kq : K1 { in x :>> x = 3.0; }
		case caq : Ca1;
		analysis anq : An1;
		verification vq : V1;
		use case uq : U1;
		view vwq : Vw1;
		viewpoint vpq : Vp1;
		concern conq : Con1;
		rendering rdq : Rd1;
		connection cnq : Cn1 connect sub#(1) to sub#(2);
		interface ifq : If1 connect pt to pt;
		allocation alq : Al1 allocate sub#(1) to sub#(2);
		flow flq : Fl1 from pt.outflow to pt.inflow;
		occurrence oq : O1;
		enum eq : E1 = E1::a;
		attribute atq : At1;
		attribute mq : MassValue = 2.0 [kg];
		attribute vq3 : VectorQuantityValue;
		snapshot snap;
		timeslice slice;
		event occurrence ev;
		perform action pa : A2;
		exhibit state es : S1;
		satisfy requirement sr : R1 by sub;
		satisfy requirement srOne : R1 by p2;
		assert constraint ac : C1 { in x :>> x = 1.0; in y :>> y = 2.0; }
		include use case iu : U1;
		attribute :>> mass = 3.0 [kg];
	}
	part def P2;
	item def I1 { attribute w : Real = 1.0; }
	port def Pt { attribute x : Integer; in item inflow : I1; out item outflow : I1; }
	action def A1 { in i : Integer; out o : Integer; action nested : A2; }
	action def A2;
	state def S1 { entry; then on1; state on1; }
	requirement def R1 { subject s : P2; attribute lim : Real = 5.0; actor who : P2; stakeholder sh : P2; require constraint { true } assume constraint { true } }
	constraint def C1 { in x : Real; in y : Real; x < y }
	calc def K1 { in x : Real; return : Real = x * 2.0; }
	case def Ca1 { subject s : P1; objective { true } }
	analysis def An1 { subject s : P1; objective { true } }
	verification def V1 { subject s : P1; objective { verify r1; } requirement r1 : R1; }
	use case def U1 { subject s : P1; actor who : P2; }
	view def Vw1 { expose Probe::P1; }
	viewpoint def Vp1 { frame concern con1 : Con1; }
	concern def Con1 { stakeholder sh : P2; }
	rendering def Rd1;
	connection def Cn1 { end e1 : P2; end e2 : P2; }
	interface def If1 { end p1 : Pt; end p2 : Pt; }
	allocation def Al1 { end a1 : P2; end a2 : P2; }
	flow def Fl1 { in item inflow : I1; }
	occurrence def O1;
	enum def E1 { a; b; }
	metadata def M1 { attribute tag : String; }
	attribute def At1 { attribute v : Real; }
	part p1 : P1;
	part p2 : P2;
	item i1 : I1;
	port pt : Pt;
	action act : A1;
	state st : S1;
	requirement r1 : R1;
	constraint c1 : C1;
	calc k1 : K1;
	case ca1 : Ca1;
	analysis an1 : An1;
	verification v1 : V1;
	use case u1 : U1;
	view vw1 : Vw1;
	viewpoint vp1 : Vp1;
	concern con1 : Con1;
	rendering rd1 : Rd1;
	connection cn1 : Cn1 connect p2 to p2;
	interface if1 : If1 connect p1.pt to p1.pt;
	allocation al1 : Al1 allocate p2 to p2;
	flow fl1 : Fl1 from p1.pt.outflow to p1.pt.inflow;
	occurrence o1 : O1;
	enum e1 : E1 = E1::a;
	attribute at1 : At1;
	attribute mass1 : MassValue = 2.0 [kg];
	attribute len1 : LengthValue = 2.0 [m];
	attribute q : ScalarQuantityValue;
	item box : ShapeItems::Box { :>> length = 1.0 [m]; :>> width = 2.0 [m]; :>> height = 3.0 [m]; }
	item sph : ShapeItems::Sphere { :>> radius = 1.0 [m]; }
	part clk : Time::Clock;
	item spat : SpatialItems::SpatialItem;
	part sys : CauseAndEffect::Cause;
	requirement der : RequirementDerivation::DerivedRequirement;
	metadata risk : RiskMetadata::RiskLevel = RiskMetadata::RiskLevel::high;
	part dsub : CauseAndEffect::Effect;
	part psi : ParametersOfInterestMetadata::MeasureOfEffectiveness;
}`

func sweepLibraryTier(sym *symbols.Symbol) string {
	d := sym.DocName
	switch {
	case strings.Contains(d, "Kernel Semantic Library"):
		return "kernel"
	case strings.Contains(d, "Kernel Data Type"):
		return "kerneldata"
	case strings.Contains(d, "Kernel Function"):
		return "kernelfn"
	case strings.Contains(d, "Systems Library"):
		return "systems"
	case strings.Contains(d, "Domain Libraries"):
		return "domain"
	case d == "<test>":
		return "model"
	}
	return "other:" + d
}

func sweepOutcome(ctx *Context, val Value, err error) string {
	if err != nil {
		if errors.Is(err, ErrNoSuchFeature) {
			return "ERR_NOSUCHFEATURE " + err.Error()
		}
		return "error " + err.Error()
	}
	switch val.Kind {
	case ValInstance:
		if inst, ok := ctx.instances[val.Instance]; ok {
			return "object:" + symbols.FQNOf(inst.Type)
		}
		return "object"
	case ValSequence, ValSet:
		elements := elementsOf(val)
		if len(elements) == 0 {
			return "empty"
		}
		// Objects are numbered by first appearance in the row: identity is kept, allocation order is not.
		members := make([]string, len(elements))
		ordinal := map[int64]int{}
		for i, element := range elements {
			members[i] = sweepOutcome(ctx, element, nil)
			if element.Kind == ValInstance {
				if _, seen := ordinal[element.Instance]; !seen {
					ordinal[element.Instance] = len(ordinal) + 1
				}
				members[i] = fmt.Sprintf("%s#%d", members[i], ordinal[element.Instance])
			}
		}
		return fmt.Sprintf("collection(%d) [%s]", len(elements), strings.Join(members, ", "))
	case ValNull:
		return "null"
	case ValUndetermined:
		return "undetermined " + FormatValue(val)
	}
	return fmt.Sprintf("%s %s", val.Kind, FormatValue(val))
}

// sweepSystemsLibraryReads reads every public Systems and Domain library feature, and every
// model feature, of each probe object, one line per read.
func sweepSystemsLibraryReads(t *testing.T) []string {
	t.Helper()
	ctx, idx := libraryShapeContext(t, sweepSystemsModel)
	root := idx.DocumentRoot("<test>")
	var out []string
	var paths []string
	for _, p := range []string{"p1", "p2", "i1", "pt", "act", "st", "r1", "c1", "k1", "ca1", "an1", "v1", "u1", "vw1", "vp1", "con1", "rd1", "cn1", "if1", "al1", "fl1", "o1", "e1", "at1", "mass1", "len1", "q", "box", "sph", "clk", "spat", "sys", "der", "risk", "dsub", "psi"} {
		paths = append(paths, "Probe::"+p)
	}
	for _, p := range []string{"sub", "opt", "cargo", "pt", "act", "st", "other", "otherItem", "others", "rq", "cq", "kq", "caq", "anq", "vq", "uq", "vwq", "vpq", "conq", "rdq", "cnq", "ifq", "alq", "flq", "oq", "eq", "atq", "mq", "vq3", "snap", "slice", "ev", "pa", "es", "sr", "srOne", "ac", "iu", "mass"} {
		paths = append(paths, "Probe::p1."+p)
	}
	for _, d := range []string{"P1", "P2", "I1", "Pt", "A1", "S1", "R1", "C1", "K1", "Ca1", "An1", "V1", "U1", "Vw1", "Vp1", "Con1", "Rd1", "Cn1", "If1", "Al1", "Fl1", "O1", "E1", "M1", "At1"} {
		paths = append(paths, "new Probe::"+d+"()")
	}
	for _, path := range paths {
		val, err := evalIn(t, ctx, root, path)
		out = append(out, fmt.Sprintf("object\t%s\t%s", path, sweepOutcome(ctx, val, err)))
		if err != nil || val.Kind != ValInstance {
			continue
		}
		inst := ctx.instances[val.Instance]
		var members []*symbols.Symbol
		for _, m := range ctx.model.semantics.MembersOf(inst.Type) {
			if m.Name == "" || !m.IsFeature() || m.Visibility == ast.VisibilityPrivate || m.Visibility == ast.VisibilityProtected {
				continue
			}
			members = append(members, m)
		}
		sort.SliceStable(members, func(i, j int) bool { return members[i].Name < members[j].Name })
		for _, m := range members {
			tier := sweepLibraryTier(m)
			if tier == "kernel" {
				continue
			}
			frame := ""
			if ctx.model.semantics.FrameFeature(m) {
				frame = "FRAME"
			}
			v, e := evalIn(t, ctx, root, path+"."+m.Name)
			out = append(out, fmt.Sprintf("read\t%s.%s\t%s%s\t%s\t%s", path, m.Name, tier, frame, ctx.QualifiedName(m), sweepOutcome(ctx, v, e)))
		}
	}
	return out
}

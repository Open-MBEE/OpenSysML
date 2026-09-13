package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// contextOverLibrary indexes libSrc as library content and src as the model, and
// returns a runtime context over both.
func contextOverLibrary(t *testing.T, libSrc, src string) (*Context, *symbols.Index) {
	t.Helper()
	idx := symbols.NewIndex()
	idx.AddDocument("frame.sysml", parser.New(source.New("frame.sysml", []byte(libSrc))).ParseFile())
	idx.MarkLibrary("frame.sysml")
	idx.AddDocument("<test>", parser.New(source.New("<test>", []byte(src))).ParseFile())
	resolver := resolve.New(idx)
	return NewContext(NewModel(semantics.NewModel(resolver), resolver), 10000), idx
}

// featureNames is the names of a shape's effective features, in order.
func featureNames(features []EffectiveFeature) []string {
	out := make([]string, 0, len(features))
	for _, f := range features {
		out = append(out, f.Name)
	}
	return out
}

// A library supertype's features frame what a part is, and materializing them
// would give the object feature values the model never asked for.
func TestShapeLeavesOutLibraryDeclaredFeatures(t *testing.T) {
	ctx, idx := contextOverLibrary(t, `package Frame {
	part def Framed {
		attribute frameKind;
	}
}`, `package M {
	part def Chassis :> Frame::Framed {
		attribute weight;
	}
	part def Car :> Chassis {
		attribute mass;
	}
}`)

	got := featureNames(ctx.buildFeatures(lookupOne(t, idx, "M::Car")))
	want := map[string]bool{"mass": true, "weight": true}
	for _, name := range got {
		if !want[name] {
			t.Fatalf("Car features = %v; a library-declared feature is not the model's", got)
		}
		delete(want, name)
	}
	if len(want) != 0 {
		t.Fatalf("Car features = %v, missing %v: only library provenance withholds a feature", got, want)
	}
}

// chainMembers is what conditions, objectives and parameters are read from, so a
// library link contributing members would make them the model's.
func TestChainMembersLeavesOutLibraryDeclaredMembers(t *testing.T) {
	ctx, idx := contextOverLibrary(t, `package Frame {
	constraint def Framed {
		assume constraint { true }
	}
}`, `package M {
	constraint def Limit :> Frame::Framed {
		require constraint { 1 < 2 }
	}
}`)

	sym := lookupOne(t, idx, "M::Limit")
	if n := len(ctx.chainMembers(sym, sym.OwnerScope)); n != 1 {
		t.Fatalf("chainMembers(M::Limit) returned %d members, want the model's one", n)
	}
}

// A calc specializing a library calc takes its parameters from what this runtime
// implements, so the library link contributes none.
func TestCalcChainLeavesOutLibraryDeclaredLinks(t *testing.T) {
	ctx, idx := contextOverLibrary(t, `package Frame {
	calc def Framed {
		in x;
		return : Real;
	}
}`, `package M {
	calc def Double :> Frame::Framed {
		in y : Real;
		return : Real = y * 2;
	}
}`)

	chain := ctx.calcChain(lookupOne(t, idx, "M::Double"))
	if len(chain) != 1 || symbols.FQNOf(chain[0]) != "M::Double" {
		t.Fatalf("calcChain(M::Double) = %v, want the model's calc alone", chain)
	}
}

// An anonymous connector declared by a library supertype joins ends of the
// metamodel frame, not of the object the model materialized.
func TestAnonymousConnectorsLeaveOutLibraryDeclaredOnes(t *testing.T) {
	ctx, idx := contextOverLibrary(t, `package Frame {
	part def End;
	part def Framed {
		part a : End;
		part b : End;
		connect a to b;
	}
}`, `package M {
	part def Bus :> Frame::Framed {
		part p : Frame::End;
		part q : Frame::End;
		connect p to q;
	}
}`)

	if conns := ctx.anonymousConnectors(lookupOne(t, idx, "M::Bus")); len(conns) != 1 {
		t.Fatalf("anonymousConnectors(M::Bus) returned %d connectors, want the model's one", len(conns))
	}
}

package symbolfacts

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// conjugatedPortModel types one port by a port definition and another by its
// conjugate, so the facts of the two can be told apart.
const conjugatedPortModel = `package ConjugatedPorts {
	port def CmdPort {
		out attribute cmd : ScalarValues::Integer;
	}
	part def Sender {
		port cmdOut : CmdPort;
	}
	part def Receiver {
		port cmdIn : ~CmdPort;
	}
}
`

func lookupOne(t *testing.T, idx *symbols.Index, fqn string) *symbols.Symbol {
	t.Helper()
	syms := idx.LookupQualified(fqn)
	if len(syms) != 1 {
		t.Fatalf("LookupQualified(%q) = %d symbols, want 1", fqn, len(syms))
	}
	return syms[0]
}

// A conjugated typing (SysML v2 8.3.12.3) keeps its `~` in the declared text and
// resolves to the ConjugatedPortDefinition the written definition implies.
func TestConjugatedTypingFactsNameTheConjugate(t *testing.T) {
	idx, sc := modelWithLibrary(t, conjugatedPortModel)
	conjugate := lookupOne(t, idx, "ConjugatedPorts::CmdPort::~CmdPort")
	if !conjugate.IsConjugatedPortDefinition() {
		t.Fatal("ConjugatedPorts::CmdPort::~CmdPort is not the implied ConjugatedPortDefinition")
	}
	original := lookupOne(t, idx, "ConjugatedPorts::CmdPort")
	cmdIn := lookupOne(t, idx, "ConjugatedPorts::Receiver::cmdIn")
	cmdOut := lookupOne(t, idx, "ConjugatedPorts::Sender::cmdOut")

	in, out := sc.typeInfoOf(cmdIn), sc.typeInfoOf(cmdOut)
	if out.Declared != "CmdPort" || out.ResolvedId != idx.GetFQN(original) {
		t.Errorf("cmdOut type = %+v, want CmdPort resolved to %s", out, idx.GetFQN(original))
	}
	if in.Declared != "~CmdPort" {
		t.Errorf("cmdIn declared type = %q, want ~CmdPort", in.Declared)
	}
	if in.ResolvedId != idx.GetFQN(conjugate) || in.ResolvedKind != out.ResolvedKind {
		t.Errorf("cmdIn resolved type = %s (%s), want %s (%s)", in.ResolvedId, in.ResolvedKind, idx.GetFQN(conjugate), out.ResolvedKind)
	}

	specs := sc.specializationsOf(cmdIn)
	if len(specs) != 1 || specs[0].Kind != "typing" || specs[0].Declared != "~CmdPort" || specs[0].TargetId != idx.GetFQN(conjugate) {
		t.Errorf("cmdIn specializations = %+v, want one typing of ~CmdPort targeting %s", specs, idx.GetFQN(conjugate))
	}

	meta := map[string]string{}
	extractMetadata(cmdIn, meta)
	if meta["type"] != "~CmdPort" {
		t.Errorf("cmdIn metadata type = %q, want ~CmdPort", meta["type"])
	}
	meta = map[string]string{}
	extractMetadata(cmdOut, meta)
	if meta["type"] != "CmdPort" {
		t.Errorf("cmdOut metadata type = %q, want CmdPort", meta["type"])
	}
}

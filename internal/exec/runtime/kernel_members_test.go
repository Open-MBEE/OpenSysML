package runtime

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

var updateKernelMembers = flag.Bool("update-kernel-members", false, "rewrite the Kernel member read golden")

// kernelProbeModel holds one object of each kind the Kernel Semantic Library gives
// its frame to: a part with nested part, item, port, action and state usages, and an
// action definition with its own usage.
const kernelProbeModel = `package Probe {
	part def P1 {
		part pa; part pb; item ia; port pt;
		attribute x : ScalarValues::Integer = 1;
		action act; state st;
	}
	part p1 : P1;
	action def A { action a1; action a2; attribute y : ScalarValues::Integer = 2; }
	action a : A;
}`

// kernelTierMembers lists the public members of typ the shape leaves out as the
// Kernel Semantic Library's frame, as the semantics resolve them, in a stable order.
func kernelTierMembers(ctx *Context, typ *symbols.Symbol) []*symbols.Symbol {
	var members []*symbols.Symbol
	for _, m := range ctx.model.semantics.MembersOf(typ) {
		if m.Name == "" || !m.IsFeature() || !ctx.kernelFrameMember(m) {
			continue
		}
		if m.Visibility == ast.VisibilityPrivate || m.Visibility == ast.VisibilityProtected {
			continue
		}
		members = append(members, m)
	}
	sort.SliceStable(members, func(i, j int) bool { return members[i].Name < members[j].Name })
	return members
}

// kernelReadOutcome names what a member read answered, without object identities,
// so the golden is stable across runs: the receiver, its owner, another object, a
// collection with its size, null, an undetermined value, or an error.
func kernelReadOutcome(ctx *Context, receiver *Instance, val Value, err error) string {
	if err != nil {
		if errors.Is(err, ErrNoSuchFeature) {
			return "ERROR no such feature: " + err.Error()
		}
		return "error: " + err.Error()
	}
	switch val.Kind {
	case ValInstance:
		switch {
		case val.Instance == receiver.ID:
			return "object (the receiver)"
		case receiver.owner != nil && val.Instance == receiver.owner.ID:
			return "object (the owner)"
		}
		if inst, ok := ctx.instances[val.Instance]; ok {
			return "object of " + symbols.FQNOf(inst.Type)
		}
		return "object"
	case ValSequence, ValSet:
		n := len(elementsOf(val))
		if n == 0 {
			return "empty"
		}
		return fmt.Sprintf("collection of %d", n)
	case ValNull:
		return "null"
	case ValUndetermined:
		return "undetermined " + FormatValue(val)
	}
	return fmt.Sprintf("%s %s", val.Kind, FormatValue(val))
}

// TestKernelLibraryMembersReadable reads every public Kernel Semantic Library
// member of a part, item, port, action and state object, from outside through a
// feature chain and from inside through `self.<m>`, and pins each answer in a
// golden: no read may fail as a missing feature or panic, and the two paths agree.
func TestKernelLibraryMembersReadable(t *testing.T) {
	ctx, idx := libraryShapeContext(t, kernelProbeModel)
	root := idx.DocumentRoot("<test>")

	p1 := mustEvalIn(t, ctx, root, "Probe::p1")
	if p1.Kind != ValInstance {
		t.Fatalf("Probe::p1 = %s, want an object", FormatValue(p1))
	}
	instantiated := func(fqn string) *Instance {
		inst, err := ctx.Instantiate(oneSymbol(t, idx, fqn))
		if err != nil {
			t.Fatalf("instantiate %s: %v", fqn, err)
		}
		return inst
	}

	var golden bytes.Buffer
	read := func(label string, inst *Instance, outside func(name string) (Value, error)) {
		fmt.Fprintf(&golden, "# %s (type %s)\n", label, symbols.FQNOf(inst.Type))
		for _, m := range kernelTierMembers(ctx, inst.Type) {
			val, err := outside(m.Name)
			out := kernelReadOutcome(ctx, inst, val, err)
			if errors.Is(err, ErrNoSuchFeature) {
				t.Errorf("%s.%s: %v", label, m.Name, err)
			}
			inVal, inErr := evalOn(t, ctx, inst, "self."+m.Name)
			if in := kernelReadOutcome(ctx, inst, inVal, inErr); in != out {
				t.Errorf("%s.%s: outside read %q, self.%s inside the body %q", label, m.Name, out, m.Name, in)
			}
			fmt.Fprintf(&golden, "%s: %s\n", m.Name, out)
		}
		golden.WriteString("\n")
	}
	// Structural usages are read from outside through a chain from the part; an
	// action or state object is one the runtime materializes, read as `self`.
	for _, path := range []string{"Probe::p1", "Probe::p1.pa", "Probe::p1.ia", "Probe::p1.pt"} {
		val, err := evalIn(t, ctx, root, path)
		if err != nil || val.Kind != ValInstance {
			t.Fatalf("%s = %s, %v; want an object", path, FormatValue(val), err)
		}
		read(path, ctx.instances[val.Instance], func(name string) (Value, error) { return evalIn(t, ctx, root, path+"."+name) })
	}
	for _, fqn := range []string{"Probe::P1::act", "Probe::P1::st", "Probe::A"} {
		inst := instantiated(fqn)
		read("object of "+fqn, inst, func(name string) (Value, error) { return evalOn(t, ctx, inst, "self."+name) })
	}
	// The usage `Probe::a`, which no performance has made an occurrence of, is
	// read through the same rule as an occurrence of its definition.
	actionInst := instantiated("Probe::A")
	fmt.Fprintf(&golden, "# Probe::a (unperformed action usage)\n")
	for _, m := range kernelTierMembers(ctx, oneSymbol(t, idx, "Probe::A")) {
		val, err := evalIn(t, ctx, root, "Probe::a."+m.Name)
		if errors.Is(err, ErrNoSuchFeature) {
			t.Errorf("Probe::a.%s: %v", m.Name, err)
		}
		fmt.Fprintf(&golden, "%s: %s\n", m.Name, kernelReadOutcome(ctx, actionInst, val, err))
	}

	// A name the type does not declare is refused as a missing feature, naming the type.
	_, err := evalIn(t, ctx, root, "Probe::p1.ownedParts")
	if !errors.Is(err, ErrNoSuchFeature) {
		t.Errorf("Probe::p1.ownedParts: %v, want ErrNoSuchFeature", err)
	}
	fmt.Fprintf(&golden, "\n# nonexistent member\nProbe::p1.ownedParts: %s\n", kernelReadOutcome(ctx, ctx.instances[p1.Instance], Value{}, err))

	path := filepath.Join("testdata", "kernel_member_reads.golden")
	if *updateKernelMembers {
		if err := os.WriteFile(path, golden.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v (run with -update-kernel-members to record it)", err)
	}
	if !bytes.Equal(want, golden.Bytes()) {
		t.Errorf("Kernel member reads differ from %s; review and rerun with -update-kernel-members.\n--- got ---\n%s", path, golden.String())
	}
}

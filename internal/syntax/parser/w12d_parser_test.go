package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestW12DEndCrossFeature locks the end's inline cross feature onto its own node
// rather than onto the end's identification (KerML.xtext
// OwnedCrossFeatureMember).
func TestW12DEndCrossFeature(t *testing.T) {
	src := "package P { assoc A { end x1 [0..1] feature x : C1 crosses y.b; end feature y : C2; } }"
	root, diags := parseKerML(t, "w12d_cross.kerml", src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	assoc := memberAt(t, root, 0, 0).(*ast.Usage)
	end := unwrapMember(t, assoc.Members[0]).(*ast.Usage)
	if end.Ident.Name != "x" || end.Ident.ShortName != "" {
		t.Fatalf("end identification = %+v, want name x and no short name", end.Ident)
	}
	if end.CrossFeature == nil || end.CrossFeature.Ident.Name != "x1" {
		t.Fatalf("cross feature = %+v, want x1", end.CrossFeature)
	}
	if end.CrossFeature.Multiplicity == nil {
		t.Error("cross feature multiplicity not recorded")
	}
	// The bounds are the cross feature's alone; the end declares no own
	// multiplicity (semantics.UsageMultiplicityOf falls back to the cross feature's).
	if end.Multiplicity != nil {
		t.Error("cross feature multiplicity copied onto the end")
	}
}

// TestW12DRequirementPrefixMetadata locks the metadata a subject, actor or
// stakeholder takes after its keyword (SysML.xtext SubjectUsage, ActorUsage,
// StakeholderUsage).
func TestW12DRequirementPrefixMetadata(t *testing.T) {
	src := "package P { metadata def B; requirement def R { subject #B s; actor #B a; stakeholder #B st; } }"
	p := New(source.New("w12d_meta.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
	}
	req := memberAt(t, root, 0, 1).(*ast.Definition)
	subject, ok := req.Members[0].(*ast.SubjectMember)
	if !ok {
		t.Fatalf("first member = %T, want *ast.SubjectMember", req.Members[0])
	}
	if len(subject.Prefixes) != 1 {
		t.Fatalf("subject prefixes = %d, want 1", len(subject.Prefixes))
	}
	for _, i := range []int{1, 2} {
		u, ok := req.Members[i].(*ast.Usage)
		if !ok {
			t.Fatalf("member %d = %T, want *ast.Usage", i, req.Members[i])
		}
		if len(u.Prefixes) != 1 {
			t.Errorf("%s prefixes = %d, want 1", u.Ident.Name, len(u.Prefixes))
		}
	}
}

// TestW12DParallelState locks the `parallel` of a state body onto the AST, since
// the substates' orthogonality is what the transition rule reads.
func TestW12DParallelState(t *testing.T) {
	src := "package P { state def S parallel { state s1; state s2; } part p { state s parallel { state a; } } }"
	p := New(source.New("w12d_parallel.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
	}
	if def := memberAt(t, root, 0, 0).(*ast.Definition); !def.IsParallel {
		t.Error("state def parallel not recorded")
	}
	part := memberAt(t, root, 0, 1).(*ast.Usage)
	if state := unwrapMember(t, part.Members[0]).(*ast.Usage); !state.IsParallel {
		t.Error("state usage parallel not recorded")
	}
}

// TestNegativeW12D covers the recovery paths of the forms above: each is
// malformed, so each must produce a diagnostic without panicking.
func TestNegativeW12D(t *testing.T) {
	tests := []struct {
		name string
		src  string
		file string
	}{
		{"cross_feature_without_end_type", "package P { assoc A { end x1 [0..1] feature x : ; } }", "w12d_neg1.kerml"},
		{"cross_feature_unterminated_multiplicity", "package P { assoc A { end x1 [0..1 feature x : C1; } }", "w12d_neg2.kerml"},
		{"subject_metadata_without_type", "package P { requirement def R { subject #; } }", "w12d_neg3.sysml"},
		{"actor_metadata_without_type", "package P { requirement def R { actor #; } }", "w12d_neg4.sysml"},
		{"parallel_state_without_body", "package P { state def S parallel ; }", "w12d_neg5.sysml"},
		{"multiplicity_member_without_subset", "package P { classifier C { multiplicity subsets ; } }", "w12d_neg6.kerml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("parser panicked: %v", r)
				}
			}()
			p := New(source.New(tt.file, []byte(tt.src)))
			root := p.ParseFile()
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(p.Diagnostics) == 0 {
				t.Fatal("expected a diagnostic")
			}
		})
	}
}

// unwrapMember returns the declaration a body member wraps.
func unwrapMember(t *testing.T, n ast.Node) ast.Node {
	t.Helper()
	if mem, ok := n.(*ast.Membership); ok {
		return mem.Member
	}
	return n
}

// memberAt walks a path of member indices from the root, unwrapping memberships.
func memberAt(t *testing.T, root *ast.RootNamespace, path ...int) ast.Node {
	t.Helper()
	members := root.Members
	var node ast.Node
	for _, i := range path {
		if i >= len(members) {
			t.Fatalf("member %d of %d not present", i, len(members))
		}
		node = unwrapMember(t, members[i])
		switch n := node.(type) {
		case *ast.Package:
			members = n.Members
		case *ast.Definition:
			members = n.Members
		case *ast.Usage:
			members = n.Members
		default:
			members = nil
		}
	}
	return node
}

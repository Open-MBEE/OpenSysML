package opensysml_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

const unnamedMembersSource = `package U {
	part def Unit { out item o; in item i; }
	metadata def M;
	part def Holder {
		part x : Unit;
		part y : Unit;
		part : Unit;
		attribute : ScalarValues::Real;
		bind x.o = y.o;
		connect x to y;
		flow from x.o to y.i;
		@M;
	}
	part holder : Holder;
}`

var unnamedMemberKinds = []string{
	"partUsage", "attributeUsage", "bindingConnectorAsUsage", "connectionUsage", "flowUsage", "metadataUsage",
}

// The unnamed members of a definition are its children under positional ids, and the
// inherited members of a usage typed by it; each id looks the member up again.
func TestLookupSymbolListsUnnamedMembersOwnAndInherited(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()
	model, err := client.ParseSource(ctx, unnamedMembersSource)
	if err != nil {
		t.Fatalf("ParseSource: %v", err)
	}
	def, err := client.LookupSymbol(ctx, model, "U::Holder")
	if err != nil {
		t.Fatalf("LookupSymbol: %v", err)
	}
	var owned, kinds []string
	for _, id := range def.ChildIDs {
		if !strings.Contains(id, "::@") {
			continue
		}
		child, err := client.LookupSymbol(ctx, model, id)
		if err != nil {
			t.Fatalf("LookupSymbol(%s): %v", id, err)
		}
		if child.ID != id || child.Name != "" {
			t.Errorf("LookupSymbol(%s) = %q named %q, want the same id and no name", id, child.ID, child.Name)
		}
		owned = append(owned, id)
		kinds = append(kinds, child.Kind)
	}
	if !slices.Equal(kinds, unnamedMemberKinds) {
		t.Errorf("unnamed children kinds = %v, want %v", kinds, unnamedMemberKinds)
	}
	usage, err := client.LookupSymbol(ctx, model, "U::holder")
	if err != nil {
		t.Fatalf("LookupSymbol: %v", err)
	}
	if len(usage.ChildIDs) != 0 {
		t.Errorf("U::holder ChildIDs = %v, want none: it owns no member", usage.ChildIDs)
	}
	var inherited []string
	for _, member := range usage.Members {
		if member.Name == "" && member.Inherited && member.InheritedFrom == "U::Holder" {
			inherited = append(inherited, member.ID)
		}
	}
	if !slices.Equal(inherited, owned) {
		t.Errorf("U::holder unnamed inherited members = %v, want %v", inherited, owned)
	}
}

// The unnamed features the library declares are inherited members under positional
// ids of the library type once library members are asked for.
func TestLookupSymbolListsUnnamedLibraryMembers(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()
	model, err := client.ParseSource(ctx, unnamedMembersSource)
	if err != nil {
		t.Fatalf("ParseSource: %v", err)
	}
	holder, err := client.LookupSymbol(ctx, model, "U::Holder", opensysml.WithLibraryMembers())
	if err != nil {
		t.Fatalf("LookupSymbol: %v", err)
	}
	bindings := 0
	for _, member := range holder.Members {
		if member.Name != "" || !member.Library {
			continue
		}
		if !member.Inherited || !strings.HasPrefix(member.ID, member.InheritedFrom+"::@") {
			t.Errorf("unnamed library member %+v, want a positional id of the type it is inherited from", member)
		}
		resolved, err := client.LookupSymbol(ctx, model, member.ID)
		if err != nil {
			t.Fatalf("LookupSymbol(%s): %v", member.ID, err)
		}
		if resolved.ID != member.ID || resolved.Kind != member.Kind {
			t.Errorf("LookupSymbol(%s) = %s %s, want kind %s", member.ID, resolved.ID, resolved.Kind, member.Kind)
		}
		if member.Kind == "bindingConnectorAsUsage" && member.InheritedFrom == "Occurrences::Occurrence" {
			bindings++
		}
	}
	if bindings != 3 {
		t.Errorf("unnamed bindings inherited from Occurrences::Occurrence = %d, want 3", bindings)
	}
}

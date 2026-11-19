package sessiontools

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

// Sessions are keyed by owner and name: two owners' sessions of one name are
// two sessions, each owner lists and closes only its own, and Keys sees all.
func TestManagerKeysSessionsByOwner(t *testing.T) {
	m := mustManager(t, t.TempDir(), func() (*repl.Session, error) { return repl.NewSession(), nil })
	t.Cleanup(m.CloseAll)
	const ann, bob = "oidc:ann", "oidc:bob"
	for _, owner := range []string{ann, bob} {
		if _, err := m.Open(owner, "work"); err != nil {
			t.Fatalf("%s opening work: %v", owner, err)
		}
	}
	if _, err := m.Open(ann, "work"); err == nil {
		t.Error("ann opened work twice")
	}
	annWork, _ := m.Get(ann, "work")
	bobWork, _ := m.Get(bob, "work")
	annDefault, _ := m.Get(ann, "")
	bobDefault, _ := m.Get(bob, "")
	if annWork == bobWork || annDefault == bobDefault {
		t.Error("two owners share a session")
	}
	if got := m.Names(ann); !slices.Equal(got, []string{"default", "work"}) {
		t.Errorf("ann's names = %v", got)
	}
	want := []Key{{ann, "default"}, {ann, "work"}, {bob, "default"}, {bob, "work"}}
	if got := m.Keys(); !slices.Equal(got, want) {
		t.Errorf("Keys = %v, want %v", got, want)
	}
	if err := m.Close(bob, "work"); err != nil {
		t.Fatalf("bob closing work: %v", err)
	}
	if err := m.Use(context.Background(), ann, "work", func(*repl.Session) error { return nil }); err != nil {
		t.Errorf("ann's work after bob closed his: %v", err)
	}
	if err := m.Close(bob, "work"); !errors.Is(err, ErrNoSession) {
		t.Errorf("bob closing work again: %v, want ErrNoSession", err)
	}
	if _, err := m.Get(bob, "work"); !errors.Is(err, ErrNoSession) {
		t.Errorf("bob's work after closing: %v, want ErrNoSession", err)
	}
	m.CloseAll()
	if got := m.Keys(); len(got) != 0 {
		t.Errorf("after CloseAll, Keys = %v", got)
	}
}

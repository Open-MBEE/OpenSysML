package resolve

import (
	"slices"
	"sync"
	"testing"
)

func TestCapturesNoteEachDocumentOnceHoweverOftenItRepeats(t *testing.T) {
	r := &Resolver{}
	var inner *readCapture
	outer := r.capturing(func() {
		r.captureDep("a.sysml")
		r.captureDep("a.sysml")
		inner = r.capturing(func() {
			r.captureDep("b.sysml")
			r.captureDep("a.sysml")
			r.captureDep("a.sysml")
			r.captureDep("b.sysml")
		})
		r.captureDep("c.sysml")
		r.captureDep("c.sysml")
		r.captureDep("")
		r.captureDep("a.sysml")
	})
	if want := []string{"a.sysml", "c.sysml"}; !slices.Equal(outer.deps, want) {
		t.Errorf("outer capture deps %v, want %v", outer.deps, want)
	}
	if want := []string{"a.sysml", "b.sysml"}; !slices.Equal(inner.deps, want) {
		t.Errorf("inner capture deps %v, want %v", inner.deps, want)
	}
	if len(outer.children) != 1 || outer.children[0] != inner {
		t.Errorf("outer capture has children %v, want the inner one", outer.children)
	}
	// Replayed, the outer capture makes the reader depend on every document
	// either capture noted.
	w := newTrackedIndex(t, map[string]string{"x.sysml": "package X;"})
	w.r.InDocument("x.sysml", func() { outer.replay(w.r) })
	for _, doc := range []string{"a.sysml", "b.sysml", "c.sysml"} {
		if deps := w.r.Dependents(doc); !slices.Contains(deps, "x.sysml") {
			t.Errorf("%s's dependents are %v after the replay, want x.sysml", doc, deps)
		}
	}
}

// Captures are replayed from parallel batch workers: a parent and the child it
// shares with another computation replay at once without a race.
func TestCapturesReplayInParallelWithoutRacing(t *testing.T) {
	r := &Resolver{}
	var child *readCapture
	parent := r.capturing(func() {
		r.captureDep("a.sysml")
		child = r.capturing(func() {
			r.captureDep("b.sysml")
			r.capturing(func() { r.captureDep("c.sysml") })
		})
	})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		c := parent
		if i%2 == 1 {
			c = child
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := newTrackedIndex(t, map[string]string{"x.sysml": "package X;"})
			w.r.InDocument("x.sysml", func() { c.replay(w.r) })
			if deps := w.r.Dependents("c.sysml"); !slices.Contains(deps, "x.sysml") {
				t.Errorf("c.sysml's dependents are %v after the replay, want x.sysml", deps)
			}
		}()
	}
	wg.Wait()
}

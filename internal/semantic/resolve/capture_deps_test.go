package resolve

import (
	"slices"
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
	})
	if want := []string{"a.sysml", "b.sysml", "c.sysml"}; !slices.Equal(outer.deps, want) {
		t.Errorf("outer capture deps %v, want %v", outer.deps, want)
	}
	if want := []string{"a.sysml", "b.sysml"}; !slices.Equal(inner.deps, want) {
		t.Errorf("inner capture deps %v, want %v", inner.deps, want)
	}
}

package symbols

import "testing"

func TestDerived(t *testing.T) {
	t.Run("frozen index caches per key", func(t *testing.T) {
		idx := NewIndex()
		idx.Freeze()

		builds := 0
		build := func() any {
			builds++
			return builds
		}
		first := idx.Derived("first", build)
		viaView := idx.Recording(nil).Derived("first", build)
		again := idx.Derived("first", build)
		other := idx.Derived("other", build)

		if first != viaView {
			t.Errorf("recording view value = %v, want %v", viaView, first)
		}
		if first != again {
			t.Errorf("same-key values differ: %v and %v", first, again)
		}
		if first != 1 || other != 2 {
			t.Errorf("derived values = %v and %v, want 1 and 2", first, other)
		}
		if builds != 2 {
			t.Errorf("build ran %d times, want once per key", builds)
		}
	})

	t.Run("writable index rebuilds every call", func(t *testing.T) {
		idx := NewIndex()
		builds := 0
		build := func() any {
			builds++
			return builds
		}

		first := idx.Derived("same", build)
		second := idx.Derived("same", build)
		if first != 1 || second != 2 {
			t.Errorf("derived values = %v and %v, want 1 and 2", first, second)
		}
		if builds != 2 {
			t.Errorf("build ran %d times, want every call", builds)
		}
	})
}

package pack

// Arena hands out slices of T carved from larger chunks, so that decoding many
// small slices costs a few allocations rather than one each. A slice's
// capacity is its length, so appending to one never overwrites its neighbour.
type Arena[T any] struct {
	chunk []T
}

const arenaChunk = 2048

// Take returns a zeroed slice of n elements, or nil for n == 0.
func (a *Arena[T]) Take(n int) []T {
	switch {
	case n == 0:
		return nil
	case n > arenaChunk/4:
		return make([]T, n)
	case n > len(a.chunk):
		a.chunk = make([]T, arenaChunk)
	}
	out := a.chunk[:n:n]
	a.chunk = a.chunk[n:]
	return out
}

package lsp

// hunk is a run of lines a diff rewrites: old[oldStart:oldEnd] becomes
// new[newStart:newEnd]. A pure insertion has oldStart == oldEnd, a pure
// deletion newStart == newEnd.
type hunk struct {
	oldStart, oldEnd int
	newStart, newEnd int
}

// diffLines returns the minimal edit script turning line ids a into b, in order.
// Linear-space Myers: O(D) passes for D differing lines, O(len(a)+len(b)) memory.
func diffLines(a, b []int) []hunk {
	d := &differ{}
	d.diff(a, b, 0, 0)
	return d.hunks
}

type differ struct {
	hunks   []hunk
	forward []int
	reverse []int
}

// add records a hunk, merging it into the previous one when they abut: the
// search emits a deletion and the insertion beside it as separate steps.
func (d *differ) add(h hunk) {
	if n := len(d.hunks); n > 0 {
		prev := &d.hunks[n-1]
		if prev.oldEnd == h.oldStart && prev.newEnd == h.newStart {
			prev.oldEnd, prev.newEnd = h.oldEnd, h.newEnd
			return
		}
	}
	d.hunks = append(d.hunks, h)
}

// diff records the hunks between e and f (lines i and j of old and new): strip
// the shared ends, find the middle snake of a shortest path, recurse on each half.
func (d *differ) diff(e, f []int, i, j int) {
	prefix := 0
	for prefix < len(e) && prefix < len(f) && e[prefix] == f[prefix] {
		prefix++
	}
	e, f, i, j = e[prefix:], f[prefix:], i+prefix, j+prefix
	suffix := 0
	for suffix < len(e) && suffix < len(f) && e[len(e)-1-suffix] == f[len(f)-1-suffix] {
		suffix++
	}
	e, f = e[:len(e)-suffix], f[:len(f)-suffix]

	n, m := len(e), len(f)
	switch {
	case n == 0 && m == 0:
		return
	case n == 0:
		d.add(hunk{oldStart: i, oldEnd: i, newStart: j, newEnd: j + m})
		return
	case m == 0:
		d.add(hunk{oldStart: i, oldEnd: i + n, newStart: j, newEnd: j})
		return
	}
	total := n + m
	delta := n - m
	// Diagonal k lives at index k+off, so k-1 and k+1 stay in bounds for
	// every k in [-m, n].
	forward, reverse := d.arrays(total + 3)
	off := m + 1
	for h := 0; h <= (total+1)/2; h++ {
		for pass := 0; pass < 2; pass++ {
			// Forward walks from the top-left, reverse from the bottom-right;
			// each reads the other's furthest reach to see where they meet.
			own, other := forward, reverse
			odd := 1
			if pass == 1 {
				own, other = reverse, forward
				odd = 0
			}
			for k := -(h - 2*max(0, h-m)); k <= h-2*max(0, h-n); k += 2 {
				var x int
				if k == -h || (k != h && own[k-1+off] < own[k+1+off]) {
					x = own[k+1+off]
				} else {
					x = own[k-1+off] + 1
				}
				y := x - k
				snakeX, snakeY := x, y
				if pass == 0 {
					for x < n && y < m && e[x] == f[y] {
						x++
						y++
					}
				} else {
					for x < n && y < m && e[n-1-x] == f[m-1-y] {
						x++
						y++
					}
				}
				own[k+off] = x
				z := delta - k
				if total%2 == odd && z >= -(h-odd) && z <= h-odd && x+other[z+off] >= n {
					// Paths overlap on diagonal k: this snake lies on a shortest path.
					var steps, headX, headY, tailX, tailY int
					if odd == 1 {
						steps, headX, headY, tailX, tailY = 2*h-1, snakeX, snakeY, x, y
					} else {
						steps, headX, headY, tailX, tailY = 2*h, n-x, m-y, n-snakeX, m-snakeY
					}
					switch {
					case steps > 1 || (headX != tailX && headY != tailY):
						d.diff(e[:headX], f[:headY], i, j)
						d.diff(e[tailX:], f[tailY:], i+tailX, j+tailY)
					case m > n:
						// One step and no snake: e is a prefix of f.
						d.diff(nil, f[n:], i+n, j+n)
					case m < n:
						d.diff(e[m:], nil, i+m, j+m)
					}
					return
				}
			}
		}
	}
}

// arrays returns two zeroed furthest-reach arrays for one search, reusing the
// last allocation when it is large enough.
func (d *differ) arrays(size int) (forward, reverse []int) {
	if cap(d.forward) < size {
		d.forward = make([]int, size)
		d.reverse = make([]int, size)
	}
	forward, reverse = d.forward[:size], d.reverse[:size]
	clear(forward)
	clear(reverse)
	return forward, reverse
}

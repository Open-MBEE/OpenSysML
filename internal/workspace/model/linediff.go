package model

import (
	"fmt"
	"strings"
)

// Hunk is a run of lines a diff rewrites: old[OldStart:OldEnd] becomes
// new[NewStart:NewEnd]. A pure insertion has OldStart == OldEnd, a pure
// deletion NewStart == NewEnd.
type Hunk struct {
	OldStart, OldEnd int
	NewStart, NewEnd int
}

// DiffLines returns the minimal edit script turning line ids a into b, in order.
// Linear-space Myers: O(D) passes for D differing lines, O(len(a)+len(b)) memory.
func DiffLines(a, b []int) []Hunk {
	d := &differ{}
	d.diff(a, b, 0, 0)
	return d.hunks
}

type differ struct {
	hunks   []Hunk
	forward []int
	reverse []int
}

// add records a Hunk, merging it into the previous one when they abut: the
// search emits a deletion and the insertion beside it as separate steps.
func (d *differ) add(h Hunk) {
	if n := len(d.hunks); n > 0 {
		prev := &d.hunks[n-1]
		if prev.OldEnd == h.OldStart && prev.NewEnd == h.NewStart {
			prev.OldEnd, prev.NewEnd = h.OldEnd, h.NewEnd
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
		d.add(Hunk{OldStart: i, OldEnd: i, NewStart: j, NewEnd: j + m})
		return
	case m == 0:
		d.add(Hunk{OldStart: i, OldEnd: i + n, NewStart: j, NewEnd: j})
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

// UnifiedDiff renders what turns old into updated as a unified diff of name,
// three lines of context around each Hunk; empty when nothing changed.
func UnifiedDiff(name string, old, updated []byte) string {
	oldLines, newLines := splitLines(string(old)), splitLines(string(updated))
	ids := map[string]int{}
	intern := func(lines []string) []int {
		out := make([]int, len(lines))
		for i, line := range lines {
			id, ok := ids[line]
			if !ok {
				id = len(ids)
				ids[line] = id
			}
			out[i] = id
		}
		return out
	}
	hunks := DiffLines(intern(oldLines), intern(newLines))
	if len(hunks) == 0 {
		return ""
	}
	const context = 3
	var b strings.Builder
	fmt.Fprintf(&b, "--- a/%s\n+++ b/%s\n", name, name)
	for i := 0; i < len(hunks); {
		start := hunks[i]
		oldFrom, newFrom := max(start.OldStart-context, 0), max(start.NewStart-context, 0)
		j := i
		for j+1 < len(hunks) && hunks[j+1].OldStart-hunks[j].OldEnd <= 2*context {
			j++
		}
		end := hunks[j]
		oldTo, newTo := min(end.OldEnd+context, len(oldLines)), min(end.NewEnd+context, len(newLines))
		fmt.Fprintf(&b, "@@ -%s +%s @@\n", hunkRange(oldFrom, oldTo), hunkRange(newFrom, newTo))
		o, n := oldFrom, newFrom
		for k := i; k <= j; k++ {
			for ; o < hunks[k].OldStart; o, n = o+1, n+1 {
				b.WriteString(" " + oldLines[o])
			}
			for ; o < hunks[k].OldEnd; o++ {
				b.WriteString("-" + oldLines[o])
			}
			for ; n < hunks[k].NewEnd; n++ {
				b.WriteString("+" + newLines[n])
			}
		}
		for ; o < oldTo; o, n = o+1, n+1 {
			b.WriteString(" " + oldLines[o])
		}
		i = j + 1
	}
	return b.String()
}

// hunkRange spells a unified-diff range, 1-based, as "start,count".
func hunkRange(from, to int) string {
	if from == to {
		return fmt.Sprintf("%d,0", from)
	}
	if to-from == 1 {
		return fmt.Sprint(from + 1)
	}
	return fmt.Sprintf("%d,%d", from+1, to-from)
}

// splitLines cuts text into lines that keep their terminator; a last line
// without one is marked as the diff marks it.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.SplitAfter(text, "\n")
	if last := lines[len(lines)-1]; last == "" {
		lines = lines[:len(lines)-1]
	} else {
		lines[len(lines)-1] = last + "\n\\ No newline at end of file\n"
	}
	return lines
}

package lower

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
)

// orderOf builds a StatementOrder over n statements from dependent pairs and fixed positions.
func orderOf(n int, dependent [][2]int, fixed ...int) *StatementOrder {
	o := &StatementOrder{fixed: make([]bool, n), dependent: make([][]bool, n), shared: make([][]bool, n)}
	for i := range n {
		o.dependent[i], o.shared[i] = make([]bool, n), make([]bool, n)
	}
	for _, p := range dependent {
		o.dependent[p[0]][p[1]], o.dependent[p[1]][p[0]] = true, true
	}
	for _, f := range fixed {
		o.fixed[f] = true
	}
	return o
}

// orders lists every order the schedule reaches by picking each time among Next.
func orders(o *StatementOrder) [][]int {
	var out [][]int
	var walk func(done, blocked []bool, prefix []int)
	walk = func(done, blocked []bool, prefix []int) {
		next := o.Next(done, blocked, false)
		if len(next) == 0 {
			out = append(out, prefix)
			return
		}
		for _, s := range next {
			d, b := slices.Clone(done), slices.Clone(blocked)
			o.Ran(s, d, b, false)
			walk(d, b, append(slices.Clone(prefix), s))
		}
	}
	walk(make([]bool, o.Len()), make([]bool, o.Len()), nil)
	return out
}

// class spells the equivalence class of an order: for each dependent pair, which
// ran first, with every pair split by a fixed statement in declaration order.
func class(o *StatementOrder, order []int) string {
	pos := make([]int, len(order))
	for k, s := range order {
		pos[s] = k
	}
	key := ""
	for i := range order {
		for j := i + 1; j < len(order); j++ {
			if o.dependent[i][j] || o.fixed[i] || o.fixed[j] {
				key += fmt.Sprint(pos[i] < pos[j])
			}
		}
	}
	return key
}

// admitted lists every order keeping each fixed statement in its place relative to all others.
func admitted(o *StatementOrder) [][]int {
	var out [][]int
	var perm func(prefix []int, left []int)
	perm = func(prefix []int, left []int) {
		if len(left) == 0 {
			out = append(out, prefix)
			return
		}
		for k, s := range left {
			rest := slices.Concat(left[:k:k], left[k+1:])
			ok := true
			for _, r := range rest {
				if (o.fixed[s] || o.fixed[r]) && r < s {
					ok = false
				}
			}
			if ok {
				perm(append(slices.Clone(prefix), s), rest)
			}
		}
	}
	all := make([]int, o.Len())
	for i := range all {
		all[i] = i
	}
	perm(nil, all)
	return out
}

func TestStatementOrderReachesEachClassOnce(t *testing.T) {
	cases := []struct {
		name      string
		order     *StatementOrder
		wantPaths int
	}{
		{"independent", orderOf(3, nil), 1},
		{"one dependent pair", orderOf(2, [][2]int{{0, 1}}), 2},
		{"dependent pair beside an independent one", orderOf(3, [][2]int{{0, 2}}), 2},
		{"chain", orderOf(3, [][2]int{{0, 1}, {1, 2}}), 4},
		{"all dependent", orderOf(3, [][2]int{{0, 1}, {0, 2}, {1, 2}}), 6},
		{"fixed splits runs", orderOf(4, [][2]int{{0, 1}, {2, 3}}, 2), 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := orders(c.order)
			if len(got) != c.wantPaths {
				t.Errorf("reached %d orders %v, want %d", len(got), got, c.wantPaths)
			}
			checkClasses(t, c.order, got)
		})
	}
	rng := rand.New(rand.NewPCG(1, 2))
	for trial := range 300 {
		n := 2 + rng.IntN(5)
		var pairs [][2]int
		for i := range n {
			for j := i + 1; j < n; j++ {
				if rng.IntN(3) == 0 {
					pairs = append(pairs, [2]int{i, j})
				}
			}
		}
		var fixed []int
		if rng.IntN(4) == 0 {
			fixed = append(fixed, rng.IntN(n))
		}
		o := orderOf(n, pairs, fixed...)
		t.Run(fmt.Sprintf("random %d", trial), func(t *testing.T) { checkClasses(t, o, orders(o)) })
	}
}

// checkClasses fails unless the orders reached are admitted, one per class, and cover every admitted class.
func checkClasses(t *testing.T, o *StatementOrder, got [][]int) {
	t.Helper()
	want := map[string]bool{}
	for _, order := range admitted(o) {
		want[class(o, order)] = true
	}
	seen := map[string]bool{}
	for _, order := range got {
		key := class(o, order)
		if !want[key] {
			t.Errorf("reached %v, which keeps no fixed statement's place", order)
		}
		if seen[key] {
			t.Errorf("reached %v, whose class another order reached", order)
		}
		seen[key] = true
	}
	if len(seen) != len(want) {
		t.Errorf("reached %d classes, want %d", len(seen), len(want))
	}
}

func TestStatementOrderReorders(t *testing.T) {
	if orderOf(3, nil).Reorders(false) {
		t.Error("independent statements reorder")
	}
	if !orderOf(2, [][2]int{{0, 1}}).Reorders(false) {
		t.Error("a dependent pair does not reorder")
	}
	if orderOf(3, [][2]int{{0, 2}}, 1).Reorders(false) {
		t.Error("a pair a fixed statement splits reorders")
	}
}

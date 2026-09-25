package graphcmp

import (
	"strings"
	"sync"
	"testing"
)

type node struct {
	name     string
	next     *node
	kids     []*node
	byKid    map[*node]string
	mu       sync.Mutex
	cache    int
	optional []int
}

func ring(name string) *node {
	n := &node{name: name}
	n.next = n
	return n
}

func TestEqualAcceptsIsomorphicGraphs(t *testing.T) {
	build := func() *node {
		shared := &node{name: "shared"}
		root := ring("root")
		root.kids = []*node{shared, shared}
		root.byKid = map[*node]string{shared: "s", root: "r"}
		return root
	}
	a, b := build(), build()
	a.cache, b.cache = 1, 2
	a.mu.Lock()
	if err := Equal(a, b, SkipFields("node.cache")); err != nil {
		t.Fatal(err)
	}
}

func TestEqualReportsDifferences(t *testing.T) {
	cases := []struct {
		name string
		a, b *node
		want string
	}{
		{
			name: "value",
			a:    &node{name: "x"},
			b:    &node{name: "y"},
			want: `"x" vs "y"`,
		},
		{
			name: "nil vs empty",
			a:    &node{optional: []int{}},
			b:    &node{},
			want: "nil slice false vs true",
		},
		{
			name: "sharing lost",
			a: func() *node {
				s := &node{}
				return &node{kids: []*node{s, s}}
			}(),
			b:    &node{kids: []*node{{}, {}}},
			want: "one *graphcmp.node object on the left is two",
		},
		{
			name: "sharing gained",
			a:    &node{kids: []*node{{}, {}}},
			b: func() *node {
				s := &node{}
				return &node{kids: []*node{s, s}}
			}(),
			want: "two *graphcmp.node objects on the left are one",
		},
		{
			name: "pointer key",
			a: func() *node {
				k := &node{}
				return &node{kids: []*node{k}, byKid: map[*node]string{k: "v"}}
			}(),
			b: func() *node {
				k := &node{}
				return &node{kids: []*node{k}, byKid: map[*node]string{{}: "v"}}
			}(),
			want: "key missing",
		},
		{
			name: "cycle shape",
			a:    ring("a"),
			b:    &node{name: "a", next: &node{name: "a"}},
			want: "one *graphcmp.node object on the left is two",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Equal(tc.a, tc.b)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Equal = %v, want an error containing %q", err, tc.want)
			}
		})
	}
}

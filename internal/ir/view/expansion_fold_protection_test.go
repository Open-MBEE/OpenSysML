package view

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestRepeatedExpansionFoldKeepsProtectedMembers(t *testing.T) {
	for _, form := range []string{"asTreeDiagram", "asInterconnectionDiagram"} {
		t.Run(form, func(t *testing.T) {
			src := fmt.Sprintf(`package P {
			private import Views::*;
			private import DiagramLayout::*;
			part def Leaf { part omitted; part placed; part endpoint; part replaced; }
			part def Outer {
				part repeated : Leaf { part own; part :>> replaced; }
			}
			part def Carrier { part outer : Outer; }
			part a : Carrier;
			part b : Carrier;
			part c : Carrier;
			connection link connect a.outer.repeated.endpoint to b.outer.repeated.endpoint;
			view v {
				expose a; expose b; expose c; expose link;
				render %s;
				metadata Layout about Leaf::placed { x = 10; y = 20; width = 40; height = 30; }
			}
		}`, form)
			r, idx := loadSources(t, []string{"protected.sysml"}, [][]byte{[]byte(src)})
			r.expansionFoldThreshold = 1
			v := lookup(t, idx, "P::v")
			got, err := r.Render(v)
			if err != nil {
				t.Fatal(err)
			}
			query, err := r.DrawnIn(v)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Drawn(), query) {
				t.Fatal("fold changed drawn set")
			}
			if got.foldedOccurrences != 2 {
				t.Fatalf("fold count=%d; want 2", got.foldedOccurrences)
			}
			wantNotice := "Repeated definition expansions folded: 2 later occurrences of 1 definitions. Each definition's features are shown at an earlier occurrence expanded at least as deeply; later occurrences retain their own and redefined features."
			if got.ExpansionFoldNotice != wantNotice {
				t.Fatalf("wrong notice: %v", got.ExpansionFoldNotice)
			}
			var repeated []*Node
			for _, node := range everyNode(got.Roots) {
				if node.Name == "repeated" {
					repeated = append(repeated, node)
				}
			}
			if len(repeated) != 3 {
				t.Fatalf("got %d occurrences; want 3", len(repeated))
			}
			for i, node := range repeated {
				children := map[string]*Node{}
				for _, child := range node.Children {
					children[child.Name] = child
				}
				for _, name := range []string{"own", "replaced", "placed"} {
					if children[name] == nil {
						t.Fatalf("occurrence %d lost %s", i, name)
					}
				}
				if form == "asInterconnectionDiagram" && children["endpoint"] == nil {
					t.Fatal("fold lost edge endpoint")
				}
				if children["placed"].Geometry == nil {
					t.Fatal("positioned member lost its Layout")
				}
				if (children["omitted"] == nil) != (i > 0) {
					t.Fatalf("unexpected inherited expansion at occurrence %d", i)
				}
			}
			if form == "asInterconnectionDiagram" {
				if len(got.Edges) != 1 {
					t.Fatalf("edges=%v; want original connection", got.Edges)
				}
				for i, endpoint := range []string{got.Edges[0].From, got.Edges[0].To} {
					found := false
					for _, child := range repeated[i].Children {
						if child.Name == "endpoint" && child.ID == endpoint {
							found = true
						}
					}
					if !found {
						t.Fatal("edge no longer ends at its occurrence's endpoint")
					}
				}
			}
			for _, node := range repeated {
				if strings.Contains(node.Detail, "repeated expansion folded") && node.Detail != "features of P::Leaf shown at P::a::outer::repeated; repeated expansion folded" {
					t.Fatalf("wrong representative: %q", node.Detail)
				}
			}
		})
	}
}

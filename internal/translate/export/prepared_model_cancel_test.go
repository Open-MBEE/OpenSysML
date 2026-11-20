package export

import (
	"bytes"
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// doneAfter is done from its live-th look on.
type doneAfter struct {
	context.Context
	live, looks atomic.Int64
}

func (c *doneAfter) Err() error {
	if c.looks.Add(1) > c.live.Load() {
		return context.Canceled
	}
	return nil
}

func TestPreparedModelStopsAtEveryCheck(t *testing.T) {
	var documents []ModelDocument
	for _, input := range [][2]string{
		{"a.sysml", "package A { part def Vehicle { attribute mass : ScalarValues::Real; } }"},
		{"b.sysml", "package B { private import A::*; part car : Vehicle; }"},
	} {
		file := source.New(input[0], []byte(input[1]))
		p := parser.New(file)
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatal(p.Diagnostics)
		}
		documents = append(documents, ModelDocument{File: file, Root: root})
	}
	undisturbed, err := ModelToRDFWith(documents, IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	want, err := WriteAPIJSON(undisturbed)
	if err != nil {
		t.Fatal(err)
	}
	prepared := PrepareModel(documents)
	for live := int64(0); ; live++ {
		ctx := &doneAfter{Context: context.Background()}
		ctx.live.Store(live)
		graph, err := prepared.ToRDF(ctx, nil, IDQualifiedName)
		if ctx.looks.Load() <= live {
			if err != nil {
				t.Fatal(err)
			}
			// One look before each document and one in the evaluation of the merged graph, at least.
			if live < int64(len(documents))+1 {
				t.Errorf("the conversion looked at its context %d times, want at least %d", live, len(documents)+1)
			}
			got, err := WriteAPIJSON(graph)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Error("the graph after the cancelled conversions differs from an undisturbed one")
			}
			return
		}
		if !errors.Is(err, context.Canceled) || graph != nil {
			t.Fatalf("context done at look %d: graph %v, error %v", live+1, graph != nil, err)
		}
	}
}

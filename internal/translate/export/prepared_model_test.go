package export

import (
	"bytes"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestPreparedModelMatchesFreshExports(t *testing.T) {
	var docs []ModelDocument
	for _, input := range [][2]string{{"library.sysml", "package Lib { part def Motor { attribute speed : ScalarValues::Integer; } }"}, {"app.sysml", "package App { private import Lib::*; part motor : Motor; }"}} {
		file := source.New(input[0], []byte(input[1]))
		parser := parser.New(file)
		root := parser.ParseFile()
		if len(parser.Diagnostics) != 0 {
			t.Fatal(parser.Diagnostics)
		}
		docs = append(docs, ModelDocument{File: file, Root: root})
	}
	prepared := PrepareModel(docs)
	if _, err := prepared.ToRDF([]string{"missing.sysml"}, IDQualifiedName); err == nil {
		t.Fatal("missing document accepted")
	}
	type sample struct {
		selection []string
		form      IDForm
		want      []byte
	}
	var samples []sample
	for _, form := range []IDForm{IDQualifiedName, IDUUID} {
		for _, selection := range [][]string{nil, {"library.sysml"}, {"app.sysml"}} {
			selected := append([]ModelDocument(nil), docs...)
			for i := range selected {
				selected[i].Referenced = len(selection) > 0 && selected[i].File.Name() != selection[0]
			}
			graph, err := ModelToRDFWith(selected, form)
			if err != nil {
				t.Fatal(err)
			}
			want, err := WriteAPIJSON(graph)
			if err != nil {
				t.Fatal(err)
			}
			samples = append(samples, sample{selection, form, want})
		}
	}
	var wg sync.WaitGroup
	for range 2 {
		for _, sample := range samples {
			wg.Go(func() {
				graph, err := prepared.ToRDF(sample.selection, sample.form)
				if err != nil {
					t.Error(err)
					return
				}
				got, err := WriteAPIJSON(graph)
				if err != nil {
					t.Error(err)
					return
				}
				if !bytes.Equal(got, sample.want) {
					t.Errorf("changed output for %v/%v", sample.selection, sample.form)
				}
			})
		}
	}
	wg.Wait()
}

func TestPreparedModelRecoversAfterRefusedExport(t *testing.T) {
	texts := [][2]string{
		{"written.sysml", "package W { part def C; }"},
		{"duplicate.sysml", `package R { @IdentityMetadata::ProjectRef { projectId = "project"; } part def A { @IdentityMetadata::ElementId { id = "shared"; } } part def B { @IdentityMetadata::ElementId { id = "shared"; } } }`},
	}
	var docs []ModelDocument
	for _, input := range texts {
		file := source.New(input[0], []byte(input[1]))
		parser := parser.New(file)
		root := parser.ParseFile()
		if len(parser.Diagnostics) > 0 {
			t.Fatal(parser.Diagnostics)
		}
		docs = append(docs, ModelDocument{File: file, Root: root})
	}
	prepared := PrepareModel(docs)
	for range 2 {
		if _, err := prepared.ToRDF(nil, IDQualifiedName); err == nil {
			t.Fatal("duplicate IDs accepted")
		}
		graph, err := prepared.ToRDF([]string{"written.sysml"}, IDQualifiedName)
		if err != nil {
			t.Fatal(err)
		}
		docs[1].Referenced = true
		fresh, err := ModelToRDFWith(docs, IDQualifiedName)
		if err != nil {
			t.Fatal(err)
		}
		got, err := WriteAPIJSON(graph)
		if err != nil {
			t.Fatal(err)
		}
		want, err := WriteAPIJSON(fresh)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatal("failed request contaminated following export")
		}
	}
}

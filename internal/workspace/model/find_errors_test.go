package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindDoneStartErrors(t *testing.T) {
	trainingDir := filepath.Join("..", "..", "..", "examples", "sysml-v2-training")

	// Skip if training examples not downloaded
	if _, err := os.Stat(trainingDir); os.IsNotExist(err) {
		t.Skip("Training examples not downloaded (run ./scripts/download-training-examples.sh)")
	}

	ws := NewWorkspace()
	var files []string

	err := filepath.Walk(trainingDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".sysml") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Checking %d files", len(files))

	doneErrors := []string{}
	startErrors := []string{}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		relPath, _ := filepath.Rel(trainingDir, file)
		ws.Open(relPath, data, 1)
		diags := ws.Diagnostics(relPath)

		for _, d := range diags {
			if d.Message == "unresolved reference: done" {
				doneErrors = append(doneErrors, relPath)
				t.Logf("DONE error in: %s", relPath)
			}
			if d.Message == "unresolved reference: start" {
				startErrors = append(startErrors, relPath)
				t.Logf("START error in: %s", relPath)
			}
		}
	}

	t.Logf("\nFiles with 'done' errors: %d", len(doneErrors))
	for _, f := range doneErrors {
		t.Logf("  %s", f)
	}

	t.Logf("\nFiles with 'start' errors: %d", len(startErrors))
	for _, f := range startErrors {
		t.Logf("  %s", f)
	}

	// `start` and `done` are features of Items::Item, inherited by every item and
	// part definition, so no corpus file may report them unresolved.
	for _, f := range append(doneErrors, startErrors...) {
		t.Errorf("unexpected start/done error in: %s", f)
	}
}

package notebook

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(name string) string {
	return filepath.Join("..", "..", "..", "tests", "testdata", "notebooks", name)
}

func mustRead(t *testing.T, name string) *Notebook {
	t.Helper()
	nb, err := Read(fixture(name))
	if err != nil {
		t.Fatalf("Read(%s): %v", name, err)
	}
	return nb
}

func TestReadKeepsCodeCellsInOrderAndDropsTheRest(t *testing.T) {
	nb := mustRead(t, "plain.ipynb")
	if nb.Language != "sysml" {
		t.Errorf("Language = %q, want sysml", nb.Language)
	}
	if len(nb.Cells) != 2 {
		t.Fatalf("got %d cells, want the 2 code cells: %+v", len(nb.Cells), nb.Cells)
	}
	if nb.Cells[0].Index != 1 || nb.Cells[1].Index != 2 {
		t.Errorf("cells are numbered %d, %d; want 1, 2", nb.Cells[0].Index, nb.Cells[1].Index)
	}
	// A source written as a list of lines reads as the one text they join to.
	if want := "package Wheels {\n  private import ScalarValues::*;\n  part def Wheel { attribute diameter : Real = 16.0; }\n}\n"; nb.Cells[0].Source != want {
		t.Errorf("cell 1 source = %q, want %q", nb.Cells[0].Source, want)
	}
	if !strings.HasPrefix(nb.Cells[1].Source, "package Cars {") {
		t.Errorf("cell 2 source = %q, want the Cars package", nb.Cells[1].Source)
	}
}

func TestReadKeepsTheCellTags(t *testing.T) {
	nb := mustRead(t, "tagged.ipynb")
	if len(nb.Cells) != 4 {
		t.Fatalf("got %d cells, want 4", len(nb.Cells))
	}
	if !nb.Cells[0].Tagged("model") || nb.Cells[0].Tagged(SkipTag) {
		t.Errorf("cell 1 tags = %v, want model only", nb.Cells[0].Tags)
	}
	if !nb.Cells[1].Tagged(SkipTag) {
		t.Errorf("cell 2 tags = %v, want %s", nb.Cells[1].Tags, SkipTag)
	}
	if !nb.Cells[2].Tagged("model") || !nb.Cells[2].Tagged("extra") {
		t.Errorf("cell 3 tags = %v, want model and extra", nb.Cells[2].Tags)
	}
	if len(nb.Cells[3].Tags) != 0 {
		t.Errorf("cell 4 tags = %v, want none", nb.Cells[3].Tags)
	}
}

func TestANotebookRecordingNoLanguageLoads(t *testing.T) {
	nb := mustRead(t, "nolanguage.ipynb")
	if nb.Language != "" || len(nb.Cells) != 1 {
		t.Errorf("notebook = %+v, want no language and one cell", nb)
	}
}

func TestAPythonNotebookIsRefused(t *testing.T) {
	_, err := Read(fixture("python.ipynb"))
	var lang *LanguageError
	if !errors.As(err, &lang) {
		t.Fatalf("err = %v, want a *LanguageError", err)
	}
	if lang.Language != "python" || !strings.Contains(err.Error(), "a python notebook") {
		t.Errorf("err = %v, want it to name the python language", err)
	}
}

func TestAnNBFormat3NotebookIsRefused(t *testing.T) {
	_, err := Read(fixture("nbformat3.ipynb"))
	var format *FormatError
	if !errors.As(err, &format) {
		t.Fatalf("err = %v, want a *FormatError", err)
	}
	if !strings.Contains(err.Error(), "nbformat 3") || !strings.Contains(err.Error(), "nbformat 4") {
		t.Errorf("err = %v, want it to name both format versions", err)
	}
}

func TestAMalformedNotebookIsRefused(t *testing.T) {
	_, err := Read(fixture("malformed.ipynb"))
	var format *FormatError
	if !errors.As(err, &format) {
		t.Fatalf("err = %v, want a *FormatError", err)
	}
	if !strings.Contains(err.Error(), "malformed.ipynb") || !strings.Contains(err.Error(), "not a notebook") {
		t.Errorf("err = %v, want it to name the file and say it is not a notebook", err)
	}
}

func TestParseRefusesJSONThatIsNoNotebook(t *testing.T) {
	for _, tt := range []struct{ name, json, want string }{
		{"no nbformat", `{"cells": []}`, "no nbformat version"},
		{"no cells", `{"nbformat": 4}`, "no cells"},
		{"cells not a list", `{"nbformat": 4, "cells": 3}`, "not a notebook"},
		{"source of the wrong type", `{"nbformat": 4, "cells": [{"cell_type": "code", "source": 3}]}`, "a cell's source must be a string or a list of strings"},
		{"text after the notebook", `{"nbformat": 4, "cells": [{"cell_type": "code", "source": "package P {}"}]} trailing`, "text follows the notebook object"},
		{"a second notebook after the first", `{"nbformat": 4, "cells": []} {"nbformat": 4, "cells": []}`, "text follows the notebook object"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse("x.ipynb", []byte(tt.json))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Parse(%s) = %v, want an error mentioning %q", tt.json, err, tt.want)
			}
		})
	}
}

func TestAMissingNotebookIsAReadError(t *testing.T) {
	_, err := Read(fixture("nowhere.ipynb"))
	var format *FormatError
	if err == nil || errors.As(err, &format) {
		t.Fatalf("err = %v, want the read error itself", err)
	}
}

func TestIsNotebook(t *testing.T) {
	for path, want := range map[string]bool{"a.ipynb": true, "dir/B.IPYNB": true, "a.sysml": false, "a.ipynb.bak": false, "ipynb": false} {
		if got := IsNotebook(path); got != want {
			t.Errorf("IsNotebook(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestSelectionByPosition(t *testing.T) {
	nb := mustRead(t, "tagged.ipynb")
	sel, err := ParseSelection("1,3-4")
	if err != nil {
		t.Fatal(err)
	}
	if sel.String() != "1,3-4" {
		t.Errorf("String() = %q", sel.String())
	}
	cells, err := sel.Apply(nb)
	if err != nil {
		t.Fatal(err)
	}
	if len(cells) != 3 || cells[0].Index != 1 || cells[1].Index != 3 || cells[2].Index != 4 {
		t.Errorf("selected %+v, want cells 1, 3 and 4", cells)
	}
}

func TestSelectionByPositionIsInNotebookOrderWithoutRepeats(t *testing.T) {
	nb := mustRead(t, "tagged.ipynb")
	sel, err := ParseSelection("3,1-2,2")
	if err != nil {
		t.Fatal(err)
	}
	cells, err := sel.Apply(nb)
	if err != nil {
		t.Fatal(err)
	}
	if len(cells) != 3 || cells[0].Index != 1 || cells[1].Index != 2 || cells[2].Index != 3 {
		t.Errorf("selected %+v, want cells 1, 2 and 3 once each", cells)
	}
}

func TestSelectionOutOfRangeNamesTheCellCount(t *testing.T) {
	nb := mustRead(t, "tagged.ipynb")
	sel, err := ParseSelection("2,6")
	if err != nil {
		t.Fatal(err)
	}
	_, err = sel.Apply(nb)
	if err == nil || !strings.Contains(err.Error(), "4 code cells") || !strings.Contains(err.Error(), "tagged.ipynb") {
		t.Errorf("err = %v, want the notebook and its 4 code cells named", err)
	}
	one := &Notebook{Name: "one.ipynb", Cells: []Cell{{Index: 1}}}
	if _, err := sel.Apply(one); err == nil || !strings.Contains(err.Error(), "has 1 code cell") {
		t.Errorf("err = %v, want the single cell counted", err)
	}
}

func TestSelectionByTag(t *testing.T) {
	nb := mustRead(t, "tagged.ipynb")
	sel, err := ParseSelection("tag:model")
	if err != nil {
		t.Fatal(err)
	}
	cells, err := sel.Apply(nb)
	if err != nil {
		t.Fatal(err)
	}
	if len(cells) != 2 || cells[0].Index != 1 || cells[1].Index != 3 {
		t.Errorf("selected %+v, want cells 1 and 3", cells)
	}
	missing, _ := ParseSelection("tag:nowhere")
	if _, err := missing.Apply(nb); err == nil || !strings.Contains(err.Error(), "no code cell is tagged nowhere") {
		t.Errorf("err = %v, want the missing tag named", err)
	}
}

func TestTheZeroSelectionPicksEveryCell(t *testing.T) {
	nb := mustRead(t, "tagged.ipynb")
	cells, err := Selection{}.Apply(nb)
	if err != nil || len(cells) != 4 {
		t.Errorf("Apply = %v, %v; want all 4 cells", cells, err)
	}
	if !(Selection{}).IsZero() {
		t.Error("the zero Selection is not IsZero")
	}
}

func TestParseSelectionRefusesWhatIsNotOne(t *testing.T) {
	for _, text := range []string{"", "0", "a", "3-1", "1,,2", "tag:", "1-", "-2"} {
		if _, err := ParseSelection(text); err == nil {
			t.Errorf("ParseSelection(%q) accepted", text)
		}
	}
}

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	modeledit "github.com/Open-MBEE/OpenSysML/internal/check/edit"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// runImportDiagram reads a draw.io file the drawio form wrote and the user
// edited, and writes what moved back into the model as DiagramLayout metadata.
func runImportDiagram(files []string) error {
	if len(files) == 0 {
		return errors.New("no model to import into; name the files the view is declared in, as `sysml model.sysml -import-diagram view.drawio`")
	}
	data, err := os.ReadFile(importDiagramPath)
	if err != nil {
		return err
	}
	file, err := view.ReadDrawio(data, importView)
	if err != nil {
		return fmt.Errorf("%s: %w", importDiagramPath, err)
	}
	sess, err := loadArtifactModel(files, "nothing was imported")
	if err != nil {
		return err
	}
	rendering, err := sess.OverlaidViewRendering(file.View, "", 0)
	if err != nil {
		return err
	}
	sites, err := sess.ViewSites()
	if err != nil {
		return err
	}
	imp, err := rendering.ImportDrawio(file, view.Options{Links: view.Links{Sites: sites}})
	if err != nil {
		return fmt.Errorf("%s: %w", importDiagramPath, err)
	}
	reportDrawioImport(imp)
	if len(imp.Edits) == 0 {
		fmt.Fprintf(os.Stderr, "%s: nothing differs from the rendering of %s; nothing to import\n", importDiagramPath, file.View)
		return nil
	}
	ops := modeledit.DrawioOperations(imp, false)
	doc := drawioImportDocument(imp, files)
	result, err := sess.ApplyModelEdit(doc, ops)
	if err != nil {
		var refusal *modeledit.Error
		if errors.As(err, &refusal) {
			return fmt.Errorf("%s: %s is not imported: %s", importDiagramPath, imp.Edits[refusal.OperationIndex].Label, refusal.Message)
		}
		return err
	}
	for _, edited := range result.Documents {
		if dryRun {
			fmt.Fprint(os.Stdout, model.UnifiedDiff(edited.Name, edited.Original, edited.Content))
			continue
		}
		if err := writeEdited(edited.Name, edited.Content); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "%s: written\n", edited.Name)
	}
	if dryRun {
		fmt.Fprintf(os.Stderr, "dry run: %d edits planned, nothing written\n", len(imp.Edits))
	}
	return nil
}

// reportDrawioImport lists the edits an import plans and what it leaves alone.
func reportDrawioImport(imp *view.DrawioImport) {
	for _, e := range imp.Edits {
		fmt.Fprintf(os.Stdout, "%s about %s\n", e.Kind, drawioEditTarget(e))
	}
	for _, name := range imp.Unmatched {
		fmt.Fprintf(os.Stderr, "%s: %s is not in the rendering; its cell is not imported\n", importDiagramPath, name)
	}
	for _, cell := range imp.Unknown {
		fmt.Fprintf(os.Stderr, "%s: cell %s names no element; it is not imported\n", importDiagramPath, cell.ID)
	}
	for _, name := range imp.Deleted {
		fmt.Fprintf(os.Stderr, "%s: %s was deleted from the file; the element stays in the model\n", importDiagramPath, name)
	}
	for _, notice := range imp.Notices {
		fmt.Fprintf(os.Stderr, "%s: %s\n", importDiagramPath, notice)
	}
}

func drawioEditTarget(e view.DrawioEdit) string {
	switch {
	case e.Kind == view.DrawioEditCanvas:
		return "the page"
	case e.Edge != nil:
		return e.Edge.String()
	case e.FQN != "":
		return e.FQN
	}
	return strings.ReplaceAll(e.Label, "\n", " ")
}

// drawioImportDocument is the session document the edits are applied through:
// the one declaring the first edited element, else the first file named.
func drawioImportDocument(imp *view.DrawioImport, files []string) string {
	for _, e := range imp.Edits {
		if e.Origin.Located() {
			return e.Origin.Doc
		}
	}
	return files[0]
}

// writeEdited rewrites a model file in place, keeping its permissions.
func writeEdited(name string, content []byte) error {
	mode := fs.FileMode(0o644)
	if info, err := os.Stat(name); err == nil {
		mode = info.Mode().Perm()
	}
	return os.WriteFile(name, content, mode)
}

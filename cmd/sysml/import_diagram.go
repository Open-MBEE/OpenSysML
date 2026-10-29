package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
			if i := refusal.OperationIndex; i >= 0 && i < len(imp.Edits) {
				return fmt.Errorf("%s: %s is not imported: %s", importDiagramPath, drawioEditTarget(imp.Edits[i]), refusal.Message)
			}
			return fmt.Errorf("%s: not imported: %s", importDiagramPath, refusal.Message)
		}
		return err
	}
	if dryRun {
		for _, edited := range result.Documents {
			fmt.Fprint(os.Stdout, model.UnifiedDiff(edited.Name, edited.Original, edited.Content))
		}
		fmt.Fprintf(os.Stderr, "dry run: %d edits planned, nothing written\n", len(imp.Edits))
		return nil
	}
	if err := commitEditedDocuments(result.Documents); err != nil {
		return err
	}
	for _, edited := range result.Documents {
		fmt.Fprintf(os.Stderr, "%s: written\n", edited.Name)
	}
	return nil
}

// commitEditedDocuments rewrites the model files as a set: each is staged
// beside its file and swapped in; a failure puts every file back as it was.
func commitEditedDocuments(documents []model.DocumentEdit) (err error) {
	type swap struct {
		target, backup string
		movedAside     bool
	}
	var done []swap
	defer func() {
		if err == nil {
			return
		}
		for i := len(done) - 1; i >= 0; i-- {
			restoreBackup(done[i].target, done[i].backup, done[i].movedAside)
		}
	}()
	staged := make([]string, 0, len(documents))
	defer func() {
		for _, name := range staged {
			_ = os.Remove(name)
		}
	}()
	for _, edited := range documents {
		info, statErr := os.Stat(edited.Name)
		if statErr != nil {
			return statErr
		}
		name, stageErr := stageEdited(edited, info.Mode().Perm())
		if name != "" {
			staged = append(staged, name)
		}
		if stageErr != nil {
			return stageErr
		}
		backup, movedAside, backupErr := backUp(edited.Name)
		if backupErr != nil {
			return fmt.Errorf("write %s: %w", edited.Name, backupErr)
		}
		done = append(done, swap{edited.Name, backup, movedAside})
		if renameErr := os.Rename(name, edited.Name); renameErr != nil {
			return fmt.Errorf("write %s: %w", edited.Name, renameErr)
		}
	}
	for _, s := range done {
		_ = os.Remove(s.backup)
	}
	done = nil
	return nil
}

// stageEdited writes the edited document to a temporary file beside its own,
// with the same permissions, and names it.
func stageEdited(edited model.DocumentEdit, perm fs.FileMode) (string, error) {
	file, err := os.CreateTemp(filepath.Dir(edited.Name), ".sysml-staged-*")
	if err != nil {
		return "", fmt.Errorf("stage %s: %w", edited.Name, err)
	}
	name := file.Name()
	if _, err := file.Write(edited.Content); err != nil {
		_ = file.Close()
		return name, fmt.Errorf("stage %s: %w", edited.Name, err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return name, fmt.Errorf("stage %s: %w", edited.Name, err)
	}
	if err := file.Close(); err != nil {
		return name, fmt.Errorf("stage %s: %w", edited.Name, err)
	}
	if err := os.Chmod(name, perm); err != nil {
		return name, fmt.Errorf("stage %s: %w", edited.Name, err)
	}
	return name, nil
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

package edit

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

// DrawioOperations turns the edits an import of a draw.io file found into the
// operations writing them: each annotation stated in the body of the imported
// view — inline on the element with inline set — and, whatever the scope, inline
// on an element no qualified name reaches, as a view's body cannot name it. A
// moved note or picture rewrites the annotation that drew it.
func DrawioOperations(imp *view.DrawioImport, inline bool) []Operation {
	ops := make([]Operation, 0, len(imp.Edits))
	for _, e := range imp.Edits {
		viewName := imp.View
		if inline || e.FQN == "" {
			viewName = ""
		}
		var op Operation
		switch e.Kind {
		case view.DrawioEditLayout:
			op = drawioTargeted(e, SetLayout(e.FQN, viewName, e.Layout), SetLayoutAt(e.Origin.Span, viewName, e.Layout))
		case view.DrawioEditRoute:
			op = drawioTargeted(e, SetRoute(e.FQN, viewName, e.Route), SetRouteAt(e.Origin.Span, viewName, e.Route))
		case view.DrawioEditStyle:
			op = drawioTargeted(e, SetStyle(e.FQN, viewName, e.Style), SetStyleAt(e.Origin.Span, viewName, e.Style))
		case view.DrawioEditCanvas:
			op = SetCanvas(imp.View, e.Canvas)
		case view.DrawioEditNote:
			op = SetNoteAt(e.Origin.Span, e.Note).DeclaredIn(e.Origin.Doc)
		case view.DrawioEditPicture:
			op = SetPictureAt(e.Origin.Span, e.Picture).DeclaredIn(e.Origin.Doc)
		case view.DrawioEditRename:
			op = Rename(e.FQN, e.NewName)
		default:
			continue
		}
		ops = append(ops, op)
	}
	return ops
}

// drawioTargeted is the operation naming the element when the import knows its
// qualified name, else the one declaring it where the rendering found it.
func drawioTargeted(e view.DrawioEdit, named, declared Operation) Operation {
	if e.FQN != "" {
		return named
	}
	return declared.DeclaredIn(e.Origin.Doc)
}

// ImportDrawio applies an import's edits through apply, the editor of the
// document the view is declared in. A rename the editor refuses — the name is
// taken, a reference would be captured — is reported in the import's notices
// and the other edits are applied again, so one refused rename holds none of
// them back; any other refusal is the caller's. The import's edits are left
// as what was applied.
func ImportDrawio[R any](imp *view.DrawioImport, inline bool, apply func([]Operation) (R, error)) (R, error) {
	for {
		var none R
		if len(imp.Edits) == 0 {
			return none, nil
		}
		result, err := apply(DrawioOperations(imp, inline))
		var refusal *Error
		if !errors.As(err, &refusal) {
			return result, err
		}
		i := refusal.OperationIndex
		if i < 0 || i >= len(imp.Edits) || imp.Edits[i].Kind != view.DrawioEditRename {
			return result, err
		}
		e := imp.Edits[i]
		imp.Notices = append(imp.Notices, fmt.Sprintf("%s: not renamed to %q: %s", e.FQN, e.NewName, refusal.Message))
		imp.Edits = append(imp.Edits[:i:i], imp.Edits[i+1:]...)
	}
}

package edit

import (
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

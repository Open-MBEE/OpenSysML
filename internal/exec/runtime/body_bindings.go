package runtime

import (
	"fmt"
	"maps"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// bodyBindingCheckError identifies a value rejected by its body's declaration.
type bodyBindingCheckError struct {
	err error
}

// Error returns the underlying declaration-check error.
func (e bodyBindingCheckError) Error() string { return e.err.Error() }

// Unwrap exposes the declaration-check error to errors.Is and errors.As.
func (e bodyBindingCheckError) Unwrap() error { return e.err }

// bodyCells holds one runtime store and its dependency-graph cells.
type bodyCells struct {
	vars    map[string]Value
	cells   map[string]*bodyCell
	order   []string
	context func(*symbols.Scope) *EvalContext
}

// bodyCell pairs a stored value with its dependency-graph cell.
type bodyCell struct {
	owner   *bodyCells
	name    string
	fv      FeatureValue
	binding *bodyBinding
}

// bodyBinding records the expression and declaration context a cell tracks.
type bodyBinding struct {
	value       ast.Node
	scope       *symbols.Scope
	masked      string
	check       func(*Value) error
	context     func(*symbols.Scope) *EvalContext
	onDerived   func(*Value) error
	visible     []map[string]bool
	limitFrames bool
	frozen      bool
}

// occurrenceHasRedefinedDefault reports whether an occurrence holds an overriding declaration's default.
func occurrenceHasRedefinedDefault(fv *FeatureValue, attr lower.Attribute, fallback *symbols.Scope) bool {
	if fv == nil || fv.Feature == nil || fv.Feature.DefaultDecl == nil {
		return false
	}
	scope := attr.Scope
	if scope == nil {
		scope = fallback
	}
	declared := memberSymbol(scope, attr.Node)
	return declared != nil && fv.Feature.DefaultDecl != declared
}

// bodyCellState is the observable tracking state of one body cell.
type bodyCellState struct {
	cell         *bodyCell
	written      bool
	frozen       bool
	materialized bool
	tracking     bool
}

// bodyCellsCapture preserves a store's maps and binding metadata for restoration.
type bodyCellsCapture struct {
	cells    *bodyCells
	vars     mapState[string, Value]
	members  map[string]*bodyCell
	bindings map[*bodyCell]*bodyBinding
	order    []string
}

// captureBodyCells snapshots the maps and binding metadata without deriving values.
func captureBodyCells(cells *bodyCells) bodyCellsCapture {
	capture := bodyCellsCapture{cells: cells}
	if cells == nil {
		return capture
	}
	capture.vars = captureMap(cells.vars)
	capture.members = maps.Clone(cells.cells)
	capture.order = append([]string(nil), cells.order...)
	capture.bindings = make(map[*bodyCell]*bodyBinding, len(cells.cells))
	for _, cell := range cells.cells {
		if cell.binding == nil {
			continue
		}
		binding := *cell.binding
		binding.visible = cloneBodyBindingVisibility(binding.visible)
		capture.bindings[cell] = &binding
	}
	return capture
}

// restore reinstates the maps and bindings held by the capture.
func (c bodyCellsCapture) restore() {
	if c.cells == nil {
		return
	}
	c.cells.vars = c.vars.restore()
	if c.cells.cells == nil {
		c.cells.cells = make(map[string]*bodyCell, len(c.members))
	}
	clear(c.cells.cells)
	c.cells.order = append(c.cells.order[:0], c.order...)
	for name, cell := range c.members {
		cell.owner, cell.name = c.cells, name
		if saved := c.bindings[cell]; saved != nil {
			binding := *saved
			binding.visible = cloneBodyBindingVisibility(saved.visible)
			cell.binding = &binding
		} else {
			cell.binding = nil
		}
		c.cells.cells[name] = cell
	}
}

// newBodyCells wraps a value store with a lazy dependency-cell map.
func newBodyCells(vars map[string]Value, context func(*symbols.Scope) *EvalContext) *bodyCells {
	return &bodyCells{vars: vars, cells: make(map[string]*bodyCell), context: context}
}

// cell returns the named cell, creating a source cell for an existing value.
func (cells *bodyCells) cell(name string) *bodyCell {
	if cells == nil {
		return nil
	}
	if cell := cells.cells[name]; cell != nil {
		return cell
	}
	cell := &bodyCell{owner: cells, name: name}
	cell.fv.body = cell
	if value, ok := cells.vars[name]; ok {
		cell.fv.Value = value
		cell.fv.Materialized = true
	}
	cells.cells[name] = cell
	cells.order = append(cells.order, name)
	return cell
}

// existingCell returns the named cell without creating one.
func (cells *bodyCells) existingCell(name string) *bodyCell {
	if cells == nil {
		return nil
	}
	return cells.cells[name]
}

// registerBodyBinding adds an expression-backed cell using the store's context.
func (ctx *Context) registerBodyBinding(
	cells *bodyCells,
	name string,
	value ast.Node,
	scope *symbols.Scope,
	check func(*Value) error,
	onDerived func(*Value) error,
) *bodyCell {
	return ctx.registerBodyBindingInContext(cells, name, value, scope, check, nil, onDerived)
}

// registerBodyBindingInContext adds an expression-backed cell with a custom context.
func (ctx *Context) registerBodyBindingInContext(
	cells *bodyCells,
	name string,
	value ast.Node,
	scope *symbols.Scope,
	check func(*Value) error,
	context func(*symbols.Scope) *EvalContext,
	onDerived func(*Value) error,
) *bodyCell {
	cell := cells.cell(name)
	cell.binding = &bodyBinding{
		value: value, scope: scope, check: check, context: context, onDerived: onDerived,
	}
	return cell
}

// deriveBodyCell evaluates an unmaterialized binding and records its dependencies.
func (ctx *Context) deriveBodyCell(cells *bodyCells, name string, cell *bodyCell) (Value, error) {
	if cell == nil || cell.binding == nil || cell.binding.frozen || cell.fv.Written {
		value, ok := cells.vars[name]
		if !ok {
			return Value{}, &NoValueError{Feature: name}
		}
		return value, nil
	}
	if cell.fv.Materialized {
		return cells.vars[name], nil
	}
	if ctx.derivingValue(&cell.fv) {
		return Value{}, fmt.Errorf("%w: %s", ErrCyclicFeatureValue, name)
	}
	value, err := ctx.deriveWith(&cell.fv, func() (Value, error) {
		if cells.context == nil {
			return Value{}, fmt.Errorf("body binding %s has no evaluation context", name)
		}
		ec := cells.context(cell.binding.scope)
		if cell.binding.context != nil {
			ec = cell.binding.context(cell.binding.scope)
		}
		if ec == nil {
			return Value{}, fmt.Errorf("body binding %s has no evaluation context", name)
		}
		if cell.binding.limitFrames {
			frames := append([]frame(nil), ec.frames...)
			if len(frames) > len(cell.binding.visible) {
				frames = frames[:len(cell.binding.visible)]
			}
			for i := range frames {
				if i < len(cell.binding.visible) {
					frames[i].visible = cell.binding.visible[i]
				} else {
					frames[i].visible = map[string]bool{}
				}
			}
			contextCopy := *ec
			contextCopy.frames = frames
			ec = &contextCopy
		}
		value, err := ec.Eval(cell.binding.value)
		if err != nil {
			return Value{}, err
		}
		if cell.binding.check != nil {
			if err := cell.binding.check(&value); err != nil {
				return Value{}, bodyBindingCheckError{err: err}
			}
		}
		if cell.binding.onDerived != nil {
			if err := cell.binding.onDerived(&value); err != nil {
				return Value{}, err
			}
		}
		ctx.noteProbeWrite(&cell.fv)
		cell.fv.Value, cell.fv.Values, cell.fv.Materialized = value, Value{}, true
		cell.fv.intrinsic = false
		cells.vars[name] = value
		return value, nil
	})
	return value, err
}

// seedBodyBindingFromFeatureValue reuses a matching occurrence value and its read edges.
func (ctx *Context) seedBodyBindingFromFeatureValue(cells *bodyCells, name string, source *FeatureValue) (Value, bool, error) {
	cell := cells.existingCell(name)
	if cell == nil || cell.binding == nil || source == nil || !source.Materialized || source.Written || source.body != nil {
		return Value{}, false, nil
	}
	value := source.HeldValue()
	if value.Kind == ValInvalid {
		return Value{}, false, nil
	}
	if cell.binding.check != nil {
		if err := cell.binding.check(&value); err != nil {
			return Value{}, false, bodyBindingCheckError{err: err}
		}
	}
	reads := append([]*FeatureValue(nil), source.reads...)
	ctx.forgetReads(&cell.fv)
	ctx.noteProbeWrite(&cell.fv)
	for _, read := range reads {
		ctx.listRead(read, &cell.fv)
	}
	cell.fv.Value, cell.fv.Values, cell.fv.Materialized = value, Value{}, true
	cell.fv.Written, cell.fv.BindingDerived, cell.fv.intrinsic = false, false, false
	cells.vars[name] = value
	if cell.binding.onDerived != nil {
		if err := cell.binding.onDerived(&value); err != nil {
			return Value{}, false, err
		}
		cell.fv.Value = value
		cells.vars[name] = value
	}
	ctx.noteRead(nil, &cell.fv)
	return value, true, nil
}

// cloneBodyBindingVisibility copies the lexical visibility captured by a binding.
func cloneBodyBindingVisibility(visible []map[string]bool) []map[string]bool {
	if visible == nil {
		return nil
	}
	cloned := make([]map[string]bool, len(visible))
	for i, frame := range visible {
		cloned[i] = maps.Clone(frame)
	}
	return cloned
}

// readBodyCell returns the current value and records it as a dependency when needed.
func (ctx *Context) readBodyCell(cells *bodyCells, name string) (Value, bool, error) {
	if cells == nil {
		return Value{}, false, nil
	}
	cell := cells.cells[name]
	if cell == nil && len(ctx.deriving) != 0 {
		if _, ok := cells.vars[name]; ok {
			cell = cells.cell(name)
		}
	}
	if cell == nil {
		value, ok := cells.vars[name]
		return value, ok, nil
	}
	if cell.binding != nil && !cell.binding.frozen && !cell.fv.Written && !cell.fv.Materialized {
		value, err := ctx.deriveBodyCell(cells, name, cell)
		if err != nil {
			return Value{}, false, err
		}
		ctx.noteRead(nil, &cell.fv)
		return value, true, nil
	}
	value, ok := cells.vars[name]
	if ok {
		ctx.noteRead(nil, &cell.fv)
	}
	return value, ok, nil
}

// writeBodyCell stores a value and ends tracking when the cell is a binding.
func (ctx *Context) writeBodyCell(cells *bodyCells, name string, value Value) {
	if cells == nil {
		return
	}
	cell := cells.cells[name]
	if cell == nil {
		cells.vars[name] = value
		return
	}
	held := ctx.beforeWrite(&cell.fv)
	if !held.taken {
		ctx.noteProbeWrite(&cell.fv)
	}
	cells.vars[name] = value
	cell.fv.Value, cell.fv.Values, cell.fv.Materialized = value, Value{}, true
	cell.fv.intrinsic = false
	if cell.binding != nil {
		cell.fv.Written = true
		ctx.forgetReads(&cell.fv)
	}
	ctx.afterWrite(&cell.fv, held)
}

// writeBodyValue stores a semantic value through its dependency cell when present.
func (ctx *Context) writeBodyValue(cells *bodyCells, vars map[string]Value, name string, value Value) {
	if cells == nil {
		vars[name] = value
		return
	}
	ctx.writeBodyCell(cells, name, value)
}

// mirrorBodyCell exposes a body's tracked value through the matching occurrence feature.
func (ctx *Context) mirrorBodyCell(inst *Instance, name string, cell *bodyCell, value Value) (Value, error) {
	if inst == nil {
		return value, nil
	}
	fv, ok := inst.FeatureValues[name]
	if !ok {
		return value, nil
	}
	ctx.noteProbeWrite(fv)
	fv.body = cell
	deriving := ctx.derivingValue(fv)
	if !deriving {
		if err := inst.BindFeatureValue(ctx, name, value); err != nil {
			return value, err
		}
	}
	fv.Written = false
	ctx.listRead(&cell.fv, fv)
	if deriving {
		return value, nil
	}
	return fv.HeldValue(), nil
}

// clearBodyValue clears a stored value and invalidates its dependents.
func (ctx *Context) clearBodyValue(cells *bodyCells, vars map[string]Value, name string) {
	if cells == nil || cells.cells[name] == nil {
		delete(vars, name)
		return
	}
	cell := cells.cells[name]
	held := ctx.beforeWrite(&cell.fv)
	if !held.taken {
		ctx.noteProbeWrite(&cell.fv)
	}
	delete(vars, name)
	cell.fv.Value, cell.fv.Values, cell.fv.Materialized = Value{}, Value{}, false
	cell.fv.intrinsic = false
	if cell.binding != nil {
		cell.fv.Written = true
		ctx.forgetReads(&cell.fv)
	}
	ctx.afterWrite(&cell.fv, held)
}

// bodyCellStateOf reports the stored and tracking state of a cell.
func bodyCellStateOf(cells *bodyCells, name string) bodyCellState {
	if cells == nil || cells.cells[name] == nil {
		return bodyCellState{}
	}
	cell := cells.cells[name]
	state := bodyCellState{cell: cell, materialized: cell.fv.Materialized, written: cell.fv.Written}
	if cell.binding != nil {
		state.frozen = cell.binding.frozen
		state.tracking = !state.written && !state.frozen
	}
	return state
}

// restoreBodyCellState reinstates a captured cell state and its dependency edges.
func (ctx *Context) restoreBodyCellState(cells *bodyCells, name string, state bodyCellState) {
	if state.cell == nil {
		return
	}
	cell := state.cell
	cell.fv.Written = state.written
	if cell.binding != nil {
		cell.binding.frozen = state.frozen
	}
	if state.tracking {
		ctx.forgetReads(&cell.fv)
		delete(cells.vars, name)
		cell.fv.Value, cell.fv.Values, cell.fv.Materialized = Value{}, Value{}, false
		cell.fv.intrinsic = false
		ctx.invalidateDependents(&cell.fv)
		return
	}
	cell.fv.Materialized = state.materialized
	if state.materialized {
		cell.fv.Value = cells.vars[name]
	} else {
		cell.fv.Value = Value{}
	}
}

// readBodyValue reads a value from its dependency-aware store.
func (ctx *Context) readBodyValue(cells *bodyCells, vars map[string]Value, name string) (Value, bool, error) {
	if cells == nil {
		value, ok := vars[name]
		return value, ok, nil
	}
	return ctx.readBodyCell(cells, name)
}

// deriveBodyCells settles the still-tracking cells of a store.
func (ctx *Context) deriveBodyCells(cells *bodyCells) error {
	if cells == nil {
		return nil
	}
	for _, name := range cells.order {
		cell := cells.cells[name]
		if cell.binding == nil || cell.binding.frozen || cell.fv.Written || cell.fv.Materialized {
			continue
		}
		if _, err := ctx.deriveBodyCell(cells, name, cell); err != nil {
			return err
		}
	}
	return nil
}

// freezeBodyCells derives and freezes every still-tracking cell in a store.
func (ctx *Context) freezeBodyCells(cells *bodyCells) error {
	if err := ctx.deriveBodyCells(cells); err != nil {
		return err
	}
	if cells == nil {
		return nil
	}
	for _, name := range cells.order {
		cell := cells.cells[name]
		if cell.binding == nil || cell.binding.frozen {
			continue
		}
		ctx.noteProbeWrite(&cell.fv)
		cell.binding.frozen = true
		ctx.forgetReads(&cell.fv)
	}
	return nil
}

// restartBodyCells makes frozen, unwritten bindings eligible to derive again.
func (ctx *Context) restartBodyCells(cells *bodyCells) {
	if cells == nil {
		return
	}
	for _, name := range cells.order {
		cell := cells.cells[name]
		if cell.binding == nil || !cell.binding.frozen || cell.fv.Written {
			continue
		}
		ctx.invalidateDependents(&cell.fv)
		ctx.noteProbeWrite(&cell.fv)
		ctx.forgetReads(&cell.fv)
		delete(cells.vars, name)
		cell.fv.Value, cell.fv.Values, cell.fv.Materialized = Value{}, Value{}, false
		cell.fv.BindingDerived, cell.fv.intrinsic = false, false
		cell.binding.frozen = false
	}
}

// forgetBodyCells removes the dependencies of every binding in a store.
func (ctx *Context) forgetBodyCells(cells *bodyCells) {
	if cells == nil {
		return
	}
	for _, name := range cells.order {
		cell := cells.cells[name]
		if cell.binding != nil {
			ctx.forgetReads(&cell.fv)
			cell.binding.frozen = true
		}
	}
}

// exitBodyCells invalidates values owned by an exiting state.
func (ctx *Context) exitBodyCells(cells *bodyCells) {
	if cells == nil {
		return
	}
	for _, name := range cells.order {
		cell := cells.cells[name]
		if cell.binding == nil {
			continue
		}
		ctx.invalidateDependents(&cell.fv)
		ctx.noteProbeWrite(&cell.fv)
		cell.binding.frozen = true
		ctx.forgetReads(&cell.fv)
	}
}

// resetBodyCells removes the stored cells and their dependency edges.
func (ctx *Context) resetBodyCells(cells *bodyCells) {
	if cells == nil {
		return
	}
	ctx.forgetBodyCells(cells)
	cells.cells = make(map[string]*bodyCell)
	cells.order = nil
	for name := range cells.vars {
		delete(cells.vars, name)
	}
}

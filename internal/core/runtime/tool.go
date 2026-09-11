package runtime

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// The standard library's AnalysisTooling metadata a tool-computed action carries.
const (
	fqnToolExecution  = "AnalysisTooling::ToolExecution"
	fqnToolVariable   = "AnalysisTooling::ToolVariable"
	fqnSIPackage      = "SI"
	toolExecutionURI  = "uri"
	toolExecutionTool = "toolName"
	toolVariableName  = "name"
)

// ToolCall is one performance of an action annotated ToolExecution, as the external tool
// sees it: the tool and URI the metadata names, the inputs read from the run keyed by
// their ToolVariable names, and the outputs the tool is to answer.
type ToolCall struct {
	// Action is the annotated action definition or usage performed.
	Action   *symbols.Symbol
	ToolName string
	URI      string
	Inputs   []ToolInput
	Outputs  []ToolOutput

	exec *ActionExecutor
}

// ToolInput is one `in` or `inout` parameter's value, under its ToolVariable name.
type ToolInput struct {
	Variable  string
	Parameter string
	Value     ToolValue
}

// ToolOutput is one `out` or `inout` parameter the tool answers, under its ToolVariable name.
type ToolOutput struct {
	Variable  string
	Parameter string
	// Declared is the parameter's declaration, whose unit the tool's answer is converted to.
	Declared *symbols.Symbol
}

// ToolValue is one value as the tool protocol carries it: a number or truth in Value, or a
// string in Text when Value is invalid, and for a quantity the unit expression it is measured in.
type ToolValue struct {
	Value semantics.Value
	Text  string
	Unit  string
}

// ToolAnswer is what one invocation established: the values bound to the call's outputs by
// parameter name, and whether an earlier invocation with equal inputs answered differently.
type ToolAnswer struct {
	Outputs  map[string]Value
	Diverged bool
}

// ToolRunner runs the tool a ToolExecution names for one performance of the action,
// binding the tool's outputs through ToolCall.Bind. A context with no runner attached
// refuses every tool-computed action as not registered.
type ToolRunner interface {
	RunTool(call *ToolCall) (ToolAnswer, error)
}

// ErrToolNotRegistered is the typed error for a ToolExecution naming a tool no manifest entry registers.
var ErrToolNotRegistered = errors.New("tool is not registered")

// ToolNotRegisteredError reports the tool a performance named and nothing answers to.
type ToolNotRegisteredError struct {
	Tool string
}

// Error is the refusal, naming the environment variable that registers tools.
func (e *ToolNotRegisteredError) Error() string {
	return fmt.Sprintf("tool '%s' is not registered; set OPENSYSML_TOOLS", e.Tool)
}

// Is matches ErrToolNotRegistered.
func (e *ToolNotRegisteredError) Is(target error) bool { return target == ErrToolNotRegistered }

// ErrTool is the typed error every failure of a tool invocation unwraps to.
var ErrTool = errors.New("tool failed")

// ToolErrorKind is what failed in one tool invocation.
type ToolErrorKind int

const (
	// ToolProcessFailed is a process that could not be started or exited non-zero.
	ToolProcessFailed ToolErrorKind = iota
	// ToolMalformed is a reply that is not one JSON object of the protocol, or a value in it
	// the receiving parameter cannot read.
	ToolMalformed
	// ToolMissingOutput is an `out` parameter the reply gave no value for.
	ToolMissingOutput
	// ToolUnknownOutput is a reply value no ToolVariable of the action receives.
	ToolUnknownOutput
	// ToolTimeout is a process that outlived OPENSYSML_TOOL_TIMEOUT.
	ToolTimeout
	// ToolRefused is a reply carrying the tool's own error message.
	ToolRefused
	// ToolUnsentInput is an input the protocol carries no value of.
	ToolUnsentInput
	// ToolAmbiguousVariable is a ToolVariable name two parameters of the action carry.
	ToolAmbiguousVariable
)

// String names the kind as the error spells it.
func (k ToolErrorKind) String() string {
	switch k {
	case ToolProcessFailed:
		return "process failed"
	case ToolMalformed:
		return "malformed output"
	case ToolMissingOutput:
		return "missing output"
	case ToolUnknownOutput:
		return "unknown output"
	case ToolTimeout:
		return "timeout"
	case ToolRefused:
		return "tool error"
	case ToolUnsentInput:
		return "input not carried"
	case ToolAmbiguousVariable:
		return "ambiguous variable"
	}
	return "unknown failure"
}

// ToolError reports one tool invocation's failure: the tool, what failed, and the detail.
type ToolError struct {
	Tool   string
	Kind   ToolErrorKind
	Detail string
}

// Error names the tool, the kind and the detail.
func (e *ToolError) Error() string {
	return fmt.Sprintf("tool '%s': %s: %s", e.Tool, e.Kind, e.Detail)
}

// Is matches ErrTool.
func (e *ToolError) Is(target error) bool { return target == ErrTool }

// ToolDivergenceCode is the diagnostic code of a ToolDivergence note.
const ToolDivergenceCode = "tool-divergence"

// ToolDivergence is a tool answering two invocations with equal inputs differently: the
// outcome table over it is not reproducible, and the run says so without changing.
type ToolDivergence struct {
	Tool   string
	Action string
	File   string
	Span   source.Span
}

// Describe renders the divergence for a diagnostic.
func (d ToolDivergence) Describe() string {
	return fmt.Sprintf("tool '%s' answered differently for equal inputs at %s", d.Tool, d.Action)
}

// String is the trace line the divergence is recorded as.
func (d ToolDivergence) String() string {
	return "tool divergence: " + d.Describe()
}

// Location is the annotated action's declaration.
func (d ToolDivergence) Location() (string, source.Span) {
	return d.File, d.Span
}

// Diagnostic is the divergence as a finding about the run, a warning since the
// results resting on the tool are not reproducible.
func (d ToolDivergence) Diagnostic() passes.Diagnostic {
	return passes.Diagnostic{
		Severity: passes.SeverityWarning,
		Span:     d.Span,
		Message:  d.Describe(),
		Code:     ToolDivergenceCode,
		Source:   "runtime",
	}
}

// SetToolRunner attaches the runner tool-computed actions of this context's runs invoke.
func (ctx *Context) SetToolRunner(runner ToolRunner) {
	ctx.tools = runner
}

// ToolRunner is the runner attached, nil when none is.
func (ctx *Context) ToolRunner() ToolRunner {
	return ctx.tools
}

// toolExecution is the ToolExecution an action carries, as its performances read it:
// the tool and URI its bindings state, and the action or supertype annotated.
type toolExecution struct {
	tool, uri string
	on        *symbols.Symbol
}

// toolExecutionOf reads the ToolExecution annotating an action, or the definition it is
// typed by. Nil for an action carrying none; an annotation is one whatever its toolName.
func (ctx *Context) toolExecutionOf(action *symbols.Symbol) (*toolExecution, error) {
	if ctx.model == nil || action == nil {
		return nil, nil
	}
	if held, ok := ctx.model.toolExecutions[action]; ok {
		return held, nil
	}
	inst, typ, on, err := ctx.annotationObject(action, fqnToolExecution)
	if err != nil {
		return nil, err
	}
	var held *toolExecution
	if inst != nil {
		held = &toolExecution{on: on}
		if held.tool, err = ctx.metadataString(inst, typ, toolExecutionTool); err != nil {
			return nil, err
		}
		if held.uri, err = ctx.metadataString(inst, typ, toolExecutionURI); err != nil {
			return nil, err
		}
	}
	ctx.model.toolExecutions[action] = held
	return held, nil
}

// toolVariableOf reads the ToolVariable annotating a parameter, or one it redefines: the
// name the tool calls it. False for a parameter carrying none.
func (ctx *Context) toolVariableOf(param *symbols.Symbol) (string, bool, error) {
	inst, typ, _, err := ctx.annotationObject(param, fqnToolVariable)
	if err != nil || inst == nil {
		return "", false, err
	}
	name, err := ctx.metadataString(inst, typ, toolVariableName)
	if err != nil {
		return "", false, err
	}
	return name, true, nil
}

// annotationObject is the object of the first annotation of the library metadata type fqn
// on an element, else on its supertypes nearest first, with the annotation's type and the
// element carrying it; nil when none does.
func (ctx *Context) annotationObject(element *symbols.Symbol, fqn string) (*Instance, *symbols.Symbol, *symbols.Symbol, error) {
	elements := append([]*symbols.Symbol{element}, ctx.model.semantics.AllSupertypes(element)...)
	for _, sym := range elements {
		for i, annotation := range ctx.model.semantics.ElementMetadataOf(sym) {
			if !ctx.metadataIs(annotation.Type, fqn) {
				continue
			}
			inst, err := ctx.metadataObject(sym, i, annotation)
			if err != nil {
				return nil, nil, nil, err
			}
			return inst, annotation.Type, sym, nil
		}
	}
	return nil, nil, nil, nil
}

// metadataIs reports whether a metadata type is, or specializes, the library type named.
func (ctx *Context) metadataIs(typ *symbols.Symbol, fqn string) bool {
	if typ == nil {
		return false
	}
	if symbols.FQNOf(typ) == fqn {
		return true
	}
	for _, super := range ctx.model.semantics.AllSupertypes(typ) {
		if symbols.FQNOf(super) == fqn {
			return true
		}
	}
	return false
}

// metadataObject is the object one annotation of an element denotes, as `.metadata` reads it.
func (ctx *Context) metadataObject(element *symbols.Symbol, index int, annotation semantics.ElementMetadata) (*Instance, error) {
	val, err := NewEvalContextIn(ctx, element.OwnerScope, nil).metadataInstance(metadataAnnotation{element: element, index: index}, annotation)
	if err != nil {
		return nil, err
	}
	id, ok := val.Object()
	if !ok {
		return nil, fmt.Errorf("%w: metadata %s denotes no object", ErrTypeMismatch, ctx.qualifiedSymbolName(annotation.Type))
	}
	inst, live := ctx.instances[id]
	if !live || inst == nil {
		return nil, fmt.Errorf("%w: metadata %s denotes no object", ErrTypeMismatch, ctx.qualifiedSymbolName(annotation.Type))
	}
	return inst, nil
}

// metadataString reads the string a metadata object's feature holds; one holding no string
// is a type mismatch, since the tool protocol has nothing else to pass through.
func (ctx *Context) metadataString(inst *Instance, typ *symbols.Symbol, feature string) (string, error) {
	fv, err := inst.GetFeatureValue(ctx, feature)
	if err != nil {
		return "", fmt.Errorf("metadata %s: %s: %w", ctx.qualifiedSymbolName(typ), feature, err)
	}
	held := fv.Value
	if held.Kind != ValString {
		return "", fmt.Errorf("%w: metadata %s: %s holds %s, not a String",
			ErrTypeMismatch, ctx.qualifiedSymbolName(typ), feature, describeValue(held))
	}
	return held.Str(), nil
}

// performByTool performs an action a ToolExecution annotates: its inputs are bound as any
// performance's are, the tool the metadata names is invoked once with them, and its outputs
// stand as the action's; the action's own flow is never run.
func (e *ActionExecutor) performByTool(execution *toolExecution) error {
	defer e.ctx.beginExecutorRun(&e.driven)()
	tool := execution.tool
	// An annotation naming no tool names none registered; the body never stands in.
	if tool == "" {
		return &ToolNotRegisteredError{Tool: tool}
	}
	if err := e.checkResultParameters(); err != nil {
		return err
	}
	e.ctx.beginPerformanceLife(e.occurrence, e.ctx.newActivation())
	defer e.ctx.endPerformanceLife(e.occurrence)
	if err := e.bindInputs(); err != nil {
		return err
	}
	call, err := e.toolCall(execution)
	if err != nil {
		return err
	}
	if e.ctx.tools == nil {
		return &ToolNotRegisteredError{Tool: tool}
	}
	answer, err := e.ctx.tools.RunTool(call)
	if err != nil {
		return err
	}
	if err := e.setFrameFeatures(e.root, answer.Outputs); err != nil {
		return err
	}
	if answer.Diverged {
		e.ctx.note(ToolDivergence{
			Tool:   tool,
			Action: e.ctx.qualifiedSymbolName(execution.on),
			File:   execution.on.DocName,
			Span:   execution.on.DeclSpan,
		})
	}
	e.state = StateCompleted
	return nil
}

// toolCall is the performance as the tool sees it: of the action performed, every `in`/`inout`
// parameter carrying a ToolVariable is an input, every `out`/`inout` one an output. An unbound
// input is ErrUnboundParameter unless optional, which the call omits; a ToolVariable name two
// parameters carry is a ToolError, since the protocol keys by it. The declaration the tool
// binds (e.action) names and types each parameter, and the performance holds it under that name.
func (e *ActionExecutor) toolCall(execution *toolExecution) (*ToolCall, error) {
	tool := execution.tool
	call := &ToolCall{Action: execution.on, ToolName: tool, URI: execution.uri, exec: e}
	namedBy := make(map[string]string)
	for _, param := range e.ctx.model.semantics.BehaviorParametersOf(e.action) {
		if param.Symbol == nil || param.Symbol.Name == "" {
			continue
		}
		variable, named, err := e.ctx.toolVariableOf(param.Symbol)
		if err != nil {
			return nil, err
		}
		if !named {
			continue
		}
		if other, taken := namedBy[variable]; taken {
			return nil, &ToolError{Tool: tool, Kind: ToolAmbiguousVariable,
				Detail: fmt.Sprintf("%s names both %s and %s of %s", variable, other, param.Symbol.Name, symbolText(e.performed))}
		}
		namedBy[variable] = param.Symbol.Name
		name := param.Symbol.Name
		reads := param.Direction == ast.DirIn || param.Direction == ast.DirInOut
		writes := param.Direction == ast.DirOut || param.Direction == ast.DirInOut
		if reads {
			held, bound := e.root.data[e.root.key(name)]
			if !bound && !e.ctx.model.semantics.OptionalParameter(param.Symbol) {
				return nil, fmt.Errorf("%w: action %s: input parameter %s is bound by no argument",
					ErrUnboundParameter, symbolText(e.performed), name)
			}
			if bound {
				sent, err := toolInput(tool, param.Symbol, held)
				if err != nil {
					return nil, err
				}
				call.Inputs = append(call.Inputs, ToolInput{Variable: variable, Parameter: name, Value: sent})
			}
		}
		if writes {
			call.Outputs = append(call.Outputs, ToolOutput{Variable: variable, Parameter: name, Declared: param.Symbol})
		}
	}
	sort.Slice(call.Inputs, func(i, j int) bool { return call.Inputs[i].Variable < call.Inputs[j].Variable })
	sort.Slice(call.Outputs, func(i, j int) bool { return call.Outputs[i].Variable < call.Outputs[j].Variable })
	return call, nil
}

// toolPerformance is the declaration a tool binds for a performance of performed, of callee:
// performed where it or a supertype carries the ToolExecution, else callee where it does; nil without one.
func (ctx *Context) toolPerformance(performed, callee *symbols.Symbol) (*symbols.Symbol, *toolExecution, error) {
	tool, err := ctx.toolExecutionOf(performed)
	if err != nil {
		return nil, nil, err
	}
	if tool != nil {
		return performed, tool, nil
	}
	if performed != callee {
		if tool, err = ctx.toolExecutionOf(callee); err != nil {
			return nil, nil, err
		}
		if tool != nil {
			return callee, tool, nil
		}
	}
	return nil, nil, nil
}

// performanceBody is the action a performance of performed, of callee, holds the features
// of: the body callee states, or under a tool, which runs no body, the declaration it binds.
func (ctx *Context) performanceBody(performed, callee *symbols.Symbol) (*symbols.Symbol, *toolExecution, error) {
	held, tool, err := ctx.toolPerformance(performed, callee)
	if err != nil {
		return nil, nil, err
	}
	if tool != nil {
		return held, tool, nil
	}
	return ctx.actionBodySymbol(callee), nil, nil
}

// performanceInterface is the declaration whose parameters a performance of performed, of
// callee, takes and binds arguments by: the tool's under a tool, else callee.
func (ctx *Context) performanceInterface(performed, callee *symbols.Symbol) (*symbols.Symbol, error) {
	held, tool, err := ctx.toolPerformance(performed, callee)
	if err != nil {
		return nil, err
	}
	if tool != nil {
		return held, nil
	}
	return callee, nil
}

// performanceParameters are the parameters of performanceInterface(performed, callee).
func (ctx *Context) performanceParameters(performed, callee *symbols.Symbol) ([]actionParameter, error) {
	held, err := ctx.performanceInterface(performed, callee)
	if err != nil {
		return nil, err
	}
	return ctx.actionParametersOf(held), nil
}

// toolInput is one parameter's value as the protocol carries it: a number, truth or string
// as is, a quantity as the run holds it, its unit spelt by short names (`km/h`).
func toolInput(tool string, param *symbols.Symbol, held Value) (ToolValue, error) {
	switch held.Kind {
	case ValConst:
		if held.Const.Kind == semantics.ValInvalid || held.Const.Kind == semantics.ValInfinity {
			break
		}
		return ToolValue{Value: held.Const}, nil
	case ValString:
		return ToolValue{Text: held.Str()}, nil
	case ValQuantity:
		q := held.Quantity()
		return ToolValue{Value: q.Num, Unit: q.Unit.Product.ShortSpelling().String()}, nil
	}
	return ToolValue{}, &ToolError{Tool: tool, Kind: ToolUnsentInput,
		Detail: fmt.Sprintf("%s holds %s, which the protocol does not carry", param.Name, describeValue(held))}
}

// Bind reads the tool's outputs, keyed by ToolVariable name, as the values of the call's
// output parameters: each quantity converted to its parameter's declared unit. An output
// missing, unknown, or not readable as its parameter's value is a ToolError.
func (c *ToolCall) Bind(outputs map[string]ToolValue) (map[string]Value, error) {
	byVariable := make(map[string]ToolOutput, len(c.Outputs))
	for _, out := range c.Outputs {
		byVariable[out.Variable] = out
	}
	var unknown []string
	for variable := range outputs {
		if _, ok := byVariable[variable]; !ok {
			unknown = append(unknown, variable)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, &ToolError{Tool: c.ToolName, Kind: ToolUnknownOutput,
			Detail: fmt.Sprintf("%s: no ToolVariable of %s receives it", strings.Join(unknown, ", "), symbolText(c.Action))}
	}
	bound := make(map[string]Value, len(c.Outputs))
	for _, out := range c.Outputs {
		answered, ok := outputs[out.Variable]
		if !ok {
			return nil, &ToolError{Tool: c.ToolName, Kind: ToolMissingOutput,
				Detail: fmt.Sprintf("%s (%s of %s) was not answered", out.Variable, out.Parameter, symbolText(c.Action))}
		}
		value, err := c.exec.toolOutput(c.ToolName, out, answered)
		if err != nil {
			return nil, err
		}
		bound[out.Parameter] = value
	}
	return bound, nil
}

// toolOutput reads one answered value as the parameter's: a string or bare number as is,
// a quantity converted to the coherent unit of the parameter's declared quantity kind,
// spelt as the declared type prefers. A unit is refused unless the parameter is a quantity,
// and a value the parameter's declaration cannot hold is malformed.
func (e *ActionExecutor) toolOutput(tool string, out ToolOutput, answered ToolValue) (Value, error) {
	malformed := func(format string, args ...any) error {
		return &ToolError{Tool: tool, Kind: ToolMalformed,
			Detail: out.Variable + ": " + fmt.Sprintf(format, args...)}
	}
	value, err := e.toolOutputValue(malformed, out, answered)
	if err != nil {
		return Value{}, err
	}
	mult, _ := e.ctx.extractMultiplicity(out.Declared)
	target := &writeTarget{name: out.Parameter, typ: e.ctx.extractType(out.Declared), mult: mult}
	if err := e.ctx.checkWrite(e.ctx.protocolScope(e.root.scope), out.Parameter, target, &value); err != nil {
		return Value{}, malformed("%v", err)
	}
	return value, nil
}

// protocolScope is the scope a tool's literals are typed in: the scalar library's, since a
// JSON number, boolean or string is its Integer, Real, Boolean or String whatever the model imports.
func (ctx *Context) protocolScope(fallback *symbols.Scope) *symbols.Scope {
	if pkg := ctx.librarySymbol(scalarValuesPackageFQN); pkg != nil && pkg.Scope != nil {
		return pkg.Scope
	}
	return fallback
}

// toolOutputValue converts one answered value to the run's, by its unit and the
// parameter's declared quantity kind; malformed builds the refusal of one that cannot be.
func (e *ActionExecutor) toolOutputValue(malformed func(string, ...any) error, out ToolOutput, answered ToolValue) (Value, error) {
	if answered.Value.Kind == semantics.ValInvalid {
		if answered.Unit != "" {
			return Value{}, malformed("text %q is measured in %s", answered.Text, answered.Unit)
		}
		return NewStringValue(answered.Text), nil
	}
	if answered.Unit == "" {
		return Value{Kind: ValConst, Const: answered.Value}, nil
	}
	if !answered.Value.IsNumeric() {
		return Value{}, malformed("a truth is measured in %s", answered.Unit)
	}
	unit, err := e.toolUnit(answered.Unit)
	if err != nil {
		return Value{}, malformed("%v", err)
	}
	q := Quantity{Num: answered.Value, Unit: unit}
	dim, ok := e.ctx.model.semantics.DimensionOfFeature(out.Declared)
	if !ok {
		if !e.ctx.quantityTyped(out.Declared) {
			return Value{}, malformed("%s is not a quantity to be measured in %s", out.Parameter, answered.Unit)
		}
		return quantityResult(q, nil)
	}
	coherent, ok := e.ctx.model.semantics.CoherentUnitFor(dim, out.Declared)
	if !ok {
		return NewQuantityValue(&q), nil
	}
	converted, err := semantics.ConvertQuantity(q, coherent)
	if err != nil {
		return Value{}, malformed("%s does not measure %s: %v", answered.Unit, out.Parameter, err)
	}
	return quantityResult(converted, nil)
}

// quantityTyped reports a feature one of whose types is a scalar quantity value type, so it
// holds a measured number; ScalarQuantityValue itself counts, fixing no dimension.
func (ctx *Context) quantityTyped(feature *symbols.Symbol) bool {
	scalar := ctx.librarySymbol(scalarQuantityTypeFQN)
	if scalar == nil {
		return false
	}
	for _, typ := range ctx.model.semantics.FeatureTypes(feature) {
		if ctx.model.semantics.Conforms(typ, scalar) {
			return true
		}
	}
	return false
}

// toolUnitKey names a unit spelling read in one scope.
type toolUnitKey struct {
	scope *symbols.Scope
	text  string
}

// toolUnit reads a unit the protocol spells, in the action's scope, else in the library's
// SI package so a tool's `m/s**2` reads whatever the model imports; the reading is
// memoized per scope since resolution memoizes per parsed name.
func (e *ActionExecutor) toolUnit(text string) (semantics.Unit, error) {
	key := toolUnitKey{scope: e.root.scope, text: text}
	if unit, ok := e.ctx.model.toolUnits[key]; ok {
		return unit, nil
	}
	expr, ok := parseToolUnit(text)
	if !ok {
		return semantics.Unit{}, fmt.Errorf("%q is not a unit expression", text)
	}
	unit, err := e.ctx.model.semantics.UnitOfExpr(e.root.scope, expr)
	if errors.Is(err, semantics.ErrNotAUnit) {
		if si := e.ctx.librarySymbol(fqnSIPackage); si != nil && si.Scope != nil {
			expr, _ = parseToolUnit(text)
			if inSI, siErr := e.ctx.model.semantics.UnitOfExpr(si.Scope, expr); siErr == nil {
				unit, err = inSI, nil
			}
		}
	}
	if err != nil {
		return semantics.Unit{}, fmt.Errorf("%q is not a unit: %w", text, err)
	}
	e.ctx.model.toolUnits[key] = unit
	return unit, nil
}

// parseToolUnit parses text as exactly one expression; false for anything else.
func parseToolUnit(text string) (ast.Node, bool) {
	p := parser.New(source.New("<tool>", []byte(text)))
	expr := p.ParseExpression()
	return expr, expr != nil && len(p.Diagnostics) == 0 && p.Offset() == len(text)
}

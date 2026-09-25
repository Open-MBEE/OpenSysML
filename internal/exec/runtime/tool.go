package runtime

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
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

// ToolCall is one performance or calculation of an action or calc annotated
// ToolExecution, as the external tool sees it: the tool and URI the metadata names,
// the inputs read from the run keyed by their ToolVariable names, and the outputs
// the tool is to answer.
type ToolCall struct {
	// Action is the annotated action or calc the tool computes.
	Action   *symbols.Symbol
	ToolName string
	URI      string
	Inputs   []ToolInput
	Outputs  []ToolOutput

	ctx   *Context
	scope *symbols.Scope
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

// Context is the context the call's performance runs in, nil for a call built outside one.
func (c *ToolCall) Context() *Context {
	return c.ctx
}

// ToolValue is one value as the tool protocol carries it: a number or truth in Value, or a
// string in Text when Value is invalid, and for a quantity the unit expression it is measured in.
// A non-nil Items makes the value a sequence: each item is a scalar (its own Value/Text, Unit
// empty — the sequence's Unit is shared by every item); an empty sequence is a non-nil empty
// slice. Readers always allocate Items non-nil.
type ToolValue struct {
	Value semantics.Value
	Text  string
	Unit  string
	Items []ToolValue
}

// ToolAnswer is what one invocation established: the values bound to the call's outputs by
// parameter name, and whether an earlier invocation with equal inputs answered differently.
type ToolAnswer struct {
	Outputs  map[string]Value
	Diverged bool
}

// ToolRunner runs the tool a ToolExecution names for one performance or calculation,
// binding the tool's outputs through ToolCall.Bind. A context with no runner attached
// refuses every tool-computed action or calc as not registered.
type ToolRunner interface {
	RunTool(call *ToolCall) (ToolAnswer, error)
}

// ErrToolNotRegistered is the typed error for a ToolExecution naming a tool no manifest entry registers.
var ErrToolNotRegistered = errors.New("tool is not registered")

// ErrToolDryRun is the error a dry run stops the performance with at its first
// tool call; it passes through unchanged.
var ErrToolDryRun = errors.New("tool dry run")

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

// ToolDivergence is a tool answering two performances or calculations with equal inputs
// differently: the outcome table over it is not reproducible, and the run says so without changing.
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

// Location is the annotated action's or calc's declaration.
func (d ToolDivergence) Location() (string, source.Span) {
	return d.File, d.Span
}

// Diagnostic is the divergence as a finding about the run, a warning since the
// results resting on the tool are not reproducible.
func (d ToolDivergence) Diagnostic() diag.Diagnostic {
	return diag.Diagnostic{
		Severity: diag.SeverityWarning,
		Span:     d.Span,
		Message:  d.Describe(),
		Code:     ToolDivergenceCode,
		Source:   "runtime",
	}
}

// SetToolRunner attaches the runner tool-computed actions and calcs of this
// context's runs invoke.
func (ctx *Context) SetToolRunner(runner ToolRunner) {
	ctx.tools = runner
}

// ToolRunner is the runner attached, nil when none is.
func (ctx *Context) ToolRunner() ToolRunner {
	return ctx.tools
}

// toolExecution is the ToolExecution an action or calc carries, as its performances
// read it: the tool and URI its bindings state, and the element or supertype annotated.
type toolExecution struct {
	tool, uri string
	on        *symbols.Symbol
}

// toolExecutionOf reads the ToolExecution annotating an action or calc, or the
// definition it is typed by. Nil for one carrying none; an annotation is one
// whatever its toolName.
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
	return e.ctx.endedWhole(&e.driven)
}

// toolCall is the performance as the tool sees it: of the action performed, every `in`/`inout`
// parameter carrying a ToolVariable is an input, every `out`/`inout` one an output. An unbound
// input is ErrUnboundParameter unless optional, which the call omits; a ToolVariable name two
// parameters carry is a ToolError, since the protocol keys by it. The declaration the tool
// binds (e.action) names and types each parameter, and the performance holds it under that name.
func (e *ActionExecutor) toolCall(execution *toolExecution) (*ToolCall, error) {
	tool := execution.tool
	call := &ToolCall{Action: execution.on, ToolName: tool, URI: execution.uri, ctx: e.ctx, scope: e.root.scope}
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
	if v, ok := ToolValueOf(held); ok {
		return v, nil
	}
	return ToolValue{}, &ToolError{Tool: tool, Kind: ToolUnsentInput,
		Detail: fmt.Sprintf("%s holds %s, which the protocol does not carry", param.Name, describeValue(held))}
}

// ToolValueOf is a value as the tool protocol carries it; false for one it does not carry.
func ToolValueOf(held Value) (ToolValue, bool) {
	switch held.Kind {
	case ValConst:
		if held.Const.Kind == semantics.ValInvalid || held.Const.Kind == semantics.ValInfinity {
			break
		}
		return ToolValue{Value: held.Const}, true
	case ValString:
		return ToolValue{Text: held.Str()}, true
	case ValQuantity:
		q := held.Quantity()
		return ToolValue{Value: q.Num, Unit: q.Unit.Product.ShortSpelling().String()}, true
	}
	return ToolValue{}, false
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
		value, err := c.ctx.toolOutput(c.scope, c.ToolName, out, answered)
		if err != nil {
			return nil, err
		}
		bound[out.Parameter] = value
	}
	return bound, nil
}

// toolOutput reads one answered value as the parameter's, the unit spellings read in
// scope: a string or bare number as is,
// a quantity converted to the coherent unit of the parameter's declared quantity kind,
// spelt as the declared type prefers; a sequence answer binds each item likewise under the
// shared unit. A unit is refused unless the parameter is a quantity, a sequence answered to
// a single-valued parameter or a scalar to a multi-valued one is malformed, and a value the
// parameter's declaration cannot hold — count included — is malformed.
func (ctx *Context) toolOutput(scope *symbols.Scope, tool string, out ToolOutput, answered ToolValue) (Value, error) {
	malformed := func(format string, args ...any) error {
		return &ToolError{Tool: tool, Kind: ToolMalformed,
			Detail: out.Variable + ": " + fmt.Sprintf(format, args...)}
	}
	mult, _ := ctx.extractMultiplicity(out.Declared)
	var value Value
	if answered.Items != nil {
		if mult.AtMostOne() {
			return Value{}, malformed("%d values answered but %s holds at most one value (multiplicity %s)", len(answered.Items), out.Parameter, mult.Text())
		}
		if len(answered.Items) == 0 && answered.Unit != "" {
			// An empty sequence has no element to carry the unit: it is
			// measured in the parameter's coherent unit itself.
			from, to, err := ctx.toolMeasuredUnits(scope, malformed, out, answered.Unit)
			if err != nil {
				return Value{}, err
			}
			// The unit is the only thing to check: a zero magnitude meets the
			// same commensurability refusal an answered element would.
			if _, err := semantics.ConvertQuantity(Quantity{Num: semantics.Value{Kind: semantics.ValReal}, Unit: from}, to); err != nil {
				return Value{}, malformed("%s does not measure %s: %v", answered.Unit, out.Parameter, err)
			}
			value = NewEmptySequenceOf(to)
		} else {
			elements := make([]Value, 0, len(answered.Items))
			for i, item := range answered.Items {
				item.Unit = answered.Unit
				// Items are named by their zero-based index, as JSON pointer indices are.
				itemErr := func(format string, args ...any) error {
					return malformed("element %d: "+format, append([]any{i}, args...)...)
				}
				element, err := ctx.toolOutputValue(scope, itemErr, out, item)
				if err != nil {
					return Value{}, err
				}
				elements = append(elements, element)
			}
			var err error
			value, err = ctx.newSequence(elements)
			if err != nil {
				return Value{}, err
			}
		}
	} else {
		if !mult.AtMostOne() {
			return Value{}, malformed("one value answered but %s holds a sequence (multiplicity %s)", out.Parameter, mult.Text())
		}
		var err error
		value, err = ctx.toolOutputValue(scope, malformed, out, answered)
		if err != nil {
			return Value{}, err
		}
	}
	target := &writeTarget{name: out.Parameter, typ: ctx.extractType(out.Declared), mult: mult}
	if err := ctx.checkWrite(ctx.protocolScope(scope), out.Parameter, target, &value); err != nil {
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
func (ctx *Context) toolOutputValue(scope *symbols.Scope, malformed func(string, ...any) error, out ToolOutput, answered ToolValue) (Value, error) {
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
	from, to, err := ctx.toolMeasuredUnits(scope, malformed, out, answered.Unit)
	if err != nil {
		return Value{}, err
	}
	converted, err := semantics.ConvertQuantity(Quantity{Num: answered.Value, Unit: from}, to)
	if err != nil {
		return Value{}, malformed("%s does not measure %s: %v", answered.Unit, out.Parameter, err)
	}
	return quantityResult(converted, nil)
}

// toolMeasuredUnits reads a unit the answer spells and resolves the unit its
// parameter measures in: from is the unit as spelled, to the coherent unit of
// the declared parameter's dimension, or the unit as spelled when the parameter
// declares none but a quantity anyway.
func (ctx *Context) toolMeasuredUnits(scope *symbols.Scope, malformed func(string, ...any) error, out ToolOutput, text string) (from semantics.Unit, to semantics.Unit, err error) {
	unit, err := ctx.UnitOf(scope, text)
	switch {
	case errors.Is(err, ErrNoExpressionParser):
		return semantics.Unit{}, semantics.Unit{}, err
	case err != nil:
		return semantics.Unit{}, semantics.Unit{}, malformed("%v", err)
	}
	dim, ok := ctx.model.semantics.DimensionOfFeature(out.Declared)
	if !ok {
		if !ctx.quantityTyped(out.Declared) {
			return semantics.Unit{}, semantics.Unit{}, malformed("%s is not a quantity to be measured in %s", out.Parameter, text)
		}
		return unit, unit, nil
	}
	if coherent, ok := ctx.model.semantics.CoherentUnitFor(dim, out.Declared); ok {
		return unit, coherent, nil
	}
	return unit, unit, nil
}

// quantityTyped reports a feature one of whose types is a scalar quantity value type, so it
// holds a measured number; ScalarQuantityValue itself counts, fixing no dimension.
func (ctx *Context) quantityTyped(feature *symbols.Symbol) bool {
	scalar := ctx.librarySymbol(scalarQuantityTypeFQN)
	if scalar == nil {
		return false
	}
	for _, typ := range ctx.model.semantics.FeatureTypes(feature) {
		if ctx.modelConforms(typ, scalar) {
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

// UnitOf reads a unit spelled as expression text in scope, else in the library's SI
// package, which a nil scope reads alone; the reading is memoized per scope.
func (ctx *Context) UnitOf(scope *symbols.Scope, text string) (semantics.Unit, error) {
	key := toolUnitKey{scope: scope, text: text}
	if unit, ok := ctx.model.toolUnits[key]; ok {
		return unit, nil
	}
	expr, ok, err := ctx.model.parseOneExpression("<tool>", text)
	if err != nil {
		return semantics.Unit{}, err
	}
	if !ok {
		return semantics.Unit{}, fmt.Errorf("%q is not a unit expression", text)
	}
	var unit semantics.Unit
	err = fmt.Errorf("%w: no scope reads %s", semantics.ErrNotAUnit, text)
	if scope != nil {
		unit, err = ctx.model.semantics.UnitOfExpr(scope, expr)
	}
	if errors.Is(err, semantics.ErrNotAUnit) {
		if si := ctx.librarySymbol(fqnSIPackage); si != nil && si.Scope != nil {
			expr, _, _ = ctx.model.parseOneExpression("<tool>", text)
			if inSI, siErr := ctx.model.semantics.UnitOfExpr(si.Scope, expr); siErr == nil {
				unit, err = inSI, nil
			}
		}
	}
	if err != nil {
		return semantics.Unit{}, fmt.Errorf("%q is not a unit: %w", text, err)
	}
	ctx.model.toolUnits[key] = unit
	return unit, nil
}

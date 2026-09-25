package runtime

import (
	"fmt"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A calc annotated ToolExecution is computed by the tool the metadata names, exactly as an
// annotated action is performed by it: the calc's `in`/`inout` parameters carrying a
// ToolVariable are sent, its `out`/`inout` ones and its result parameter are bound from
// the reply, and the body — when the calc states one — is never evaluated.

// calcToolCall is the calculation as the tool sees it: of the calc the annotation holds
// for (the definition or usage annotated), every `in`/`inout` parameter carrying a
// ToolVariable is an input whose value held answers, every `out`/`inout` one an output,
// and the result parameter is always an output — keyed by its ToolVariable name, else its
// declared name, else `result`. The same refusals as a performance's call apply: an
// unbound non-optional input is ErrUnboundParameter, a ToolVariable name two parameters
// carry is a ToolError. The name the result is bound under is returned with the call.
func (ctx *Context) calcToolCall(shape *calcShape, scope *symbols.Scope, held func(param string) (Value, bool)) (*ToolCall, string, error) {
	execution := shape.Tool
	tool := execution.tool
	call := &ToolCall{Action: execution.on, ToolName: tool, URI: execution.uri, ctx: ctx, scope: scope}
	namedBy := make(map[string]string)
	var result *symbols.Symbol
	for _, param := range ctx.model.semantics.BehaviorParametersOf(shape.Sym) {
		if param.Symbol == nil {
			continue
		}
		if param.IsResult {
			result = param.Symbol
			continue
		}
		if param.Symbol.Name == "" {
			continue
		}
		variable, named, err := ctx.toolVariableOf(param.Symbol)
		if err != nil {
			return nil, "", err
		}
		if !named {
			continue
		}
		if other, taken := namedBy[variable]; taken {
			return nil, "", &ToolError{Tool: tool, Kind: ToolAmbiguousVariable,
				Detail: fmt.Sprintf("%s names both %s and %s of %s", variable, other, param.Symbol.Name, shape.Label)}
		}
		namedBy[variable] = param.Symbol.Name
		name := param.Symbol.Name
		reads := param.Direction == ast.DirIn || param.Direction == ast.DirInOut
		writes := param.Direction == ast.DirOut || param.Direction == ast.DirInOut
		if reads {
			value, bound := held(name)
			if !bound && !ctx.model.semantics.OptionalParameter(param.Symbol) {
				return nil, "", fmt.Errorf("%w: %s: input parameter %s is bound by no argument",
					ErrUnboundParameter, shape.Label, name)
			}
			if bound {
				sent, err := toolInput(tool, param.Symbol, value)
				if err != nil {
					return nil, "", err
				}
				call.Inputs = append(call.Inputs, ToolInput{Variable: variable, Parameter: name, Value: sent})
			}
		}
		if writes {
			call.Outputs = append(call.Outputs, ToolOutput{Variable: variable, Parameter: name, Declared: param.Symbol})
		}
	}
	resultKey := resultOutputName
	if result != nil {
		variable := resultOutputName
		if result.Name != "" {
			resultKey, variable = result.Name, result.Name
		}
		if named, has, err := ctx.toolVariableOf(result); err != nil {
			return nil, "", err
		} else if has {
			variable = named
		}
		if other, taken := namedBy[variable]; taken {
			return nil, "", &ToolError{Tool: tool, Kind: ToolAmbiguousVariable,
				Detail: fmt.Sprintf("%s names both %s and %s of %s", variable, other, resultKey, shape.Label)}
		}
		call.Outputs = append(call.Outputs, ToolOutput{Variable: variable, Parameter: resultKey, Declared: result})
	}
	sort.Slice(call.Inputs, func(i, j int) bool { return call.Inputs[i].Variable < call.Inputs[j].Variable })
	sort.Slice(call.Outputs, func(i, j int) bool { return call.Outputs[i].Variable < call.Outputs[j].Variable })
	return call, resultKey, nil
}

// computeCalcByTool computes a tool-annotated calc: the tool shape.Tool names is invoked
// once with the inputs held answers, its outputs stand as the calc's outputs, and the
// result parameter's value is the calculation's result. The failures of a performance's
// tool apply unchanged: an annotation naming no tool, or a context with no runner, is
// not registered and the body never stands in.
func (ctx *Context) computeCalcByTool(shape *calcShape, scope *symbols.Scope, held func(param string) (Value, bool)) (Value, map[string]Value, error) {
	tool := shape.Tool.tool
	if tool == "" {
		return Value{}, nil, &ToolNotRegisteredError{Tool: tool}
	}
	call, resultKey, err := ctx.calcToolCall(shape, scope, held)
	if err != nil {
		return Value{}, nil, err
	}
	if ctx.tools == nil {
		return Value{}, nil, &ToolNotRegisteredError{Tool: tool}
	}
	answer, err := ctx.tools.RunTool(call)
	if err != nil {
		return Value{}, nil, err
	}
	if answer.Diverged {
		ctx.note(ToolDivergence{
			Tool:   tool,
			Action: ctx.qualifiedSymbolName(shape.Tool.on),
			File:   shape.Tool.on.DocName,
			Span:   shape.Tool.on.DeclSpan,
		})
	}
	return answer.Outputs[resultKey], answer.Outputs, nil
}

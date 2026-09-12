package runtime

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// builtinFunc is the implementation of a built-in: a function over runtime
// values that the evaluation context calls with its arguments in parameter order.
type builtinFunc = func(*EvalContext, []Value) (Value, error)

// declaredParam is one input parameter of a built-in or library function, as
// the library declares it.
type declaredParam struct {
	name     string
	optional bool // its multiplicity admits no value, so an unbound argument is null
	deferred bool // declared `expr`: the argument binds unevaluated for the function to evaluate
}

func param(name string) declaredParam         { return declaredParam{name: name} }
func optionalParam(name string) declaredParam { return declaredParam{name: name, optional: true} }
func exprParam(name string) declaredParam {
	return declaredParam{name: name, optional: true, deferred: true}
}

// builtinSignatures lists each built-in's parameters in the library's declared
// order, which is what an argument written by name binds against. The
// dispatch gate (library_functions_test.go) holds them to the declarations.
var builtinSignatures = map[string][]declaredParam{
	"SequenceFunctions::#":            {optionalParam("seq"), param("index")},
	"SequenceFunctions::size":         {optionalParam("seq")},
	"SequenceFunctions::isEmpty":      {optionalParam("seq")},
	"SequenceFunctions::notEmpty":     {optionalParam("seq")},
	"SequenceFunctions::includes":     {optionalParam("seq1"), optionalParam("seq2")},
	"SequenceFunctions::includesOnly": {optionalParam("seq1"), optionalParam("seq2")},
	"SequenceFunctions::excludes":     {optionalParam("seq1"), optionalParam("seq2")},
	"SequenceFunctions::equals":       {optionalParam("x"), optionalParam("y")},
	"SequenceFunctions::same":         {optionalParam("x"), optionalParam("y")},
	"SequenceFunctions::union":        {optionalParam("seq1"), optionalParam("seq2")},
	"SequenceFunctions::intersection": {optionalParam("seq1"), optionalParam("seq2")},
	"SequenceFunctions::including":    {optionalParam("seq"), optionalParam("values")},
	"SequenceFunctions::includingAt":  {optionalParam("seq"), optionalParam("values"), param("index")},
	"SequenceFunctions::excluding":    {optionalParam("seq"), optionalParam("values")},
	"SequenceFunctions::subsequence":  {optionalParam("seq"), param("startIndex"), optionalParam("endIndex")},
	"SequenceFunctions::excludingAt":  {optionalParam("seq"), param("startIndex"), optionalParam("endIndex")},
	"SequenceFunctions::head":         {optionalParam("seq")},
	"SequenceFunctions::tail":         {optionalParam("seq")},
	"SequenceFunctions::last":         {optionalParam("seq")},

	"CollectionFunctions::#":           {param("col"), param("index")},
	"CollectionFunctions::array#":      {param("arr"), param("indexes")},
	"CollectionFunctions::==":          {optionalParam("col1"), optionalParam("col2")},
	"CollectionFunctions::size":        {param("col")},
	"CollectionFunctions::isEmpty":     {param("col")},
	"CollectionFunctions::notEmpty":    {param("col")},
	"CollectionFunctions::contains":    {param("col"), optionalParam("values")},
	"CollectionFunctions::containsAll": {param("col1"), param("col2")},
	"CollectionFunctions::head":        {param("col")},
	"CollectionFunctions::tail":        {param("col")},
	"CollectionFunctions::last":        {param("col")},

	"BaseFunctions::#": {optionalParam("seq"), param("index")},
	"BaseFunctions::,": {optionalParam("seq1"), optionalParam("seq2")},

	"DataFunctions::..":    {param("lower"), param("upper")},
	"ScalarFunctions::..":  {param("lower"), param("upper")},
	"IntegerFunctions::..": {param("lower"), param("upper")},

	"ControlFunctions::if":        {param("test"), exprParam("thenValue"), exprParam("elseValue")},
	"ControlFunctions::??":        {optionalParam("firstValue"), exprParam("secondValue")},
	"ControlFunctions::and":       {param("firstValue"), exprParam("secondValue")},
	"ControlFunctions::or":        {param("firstValue"), exprParam("secondValue")},
	"ControlFunctions::implies":   {param("firstValue"), exprParam("secondValue")},
	"ControlFunctions::select":    {optionalParam("collection"), exprParam("selector")},
	"ControlFunctions::selectOne": {optionalParam("collection"), exprParam("selector1")},
	"ControlFunctions::reject":    {optionalParam("collection"), exprParam("rejector")},
	"ControlFunctions::collect":   {optionalParam("collection"), exprParam("mapper")},
	"ControlFunctions::forAll":    {optionalParam("collection"), exprParam("test")},
	"ControlFunctions::exists":    {optionalParam("collection"), exprParam("test")},
	"ControlFunctions::allTrue":   {optionalParam("collection")},
	"ControlFunctions::anyTrue":   {optionalParam("collection")},
	"ControlFunctions::reduce":    {optionalParam("collection"), exprParam("reducer")},
	"ControlFunctions::minimize":  {param("collection"), exprParam("fn")},
	"ControlFunctions::maximize":  {param("collection"), exprParam("fn")},

	"NumericalFunctions::sum":      {optionalParam("collection")},
	"NumericalFunctions::product":  {optionalParam("collection")},
	"NumericalFunctions::sum0":     {optionalParam("collection"), param("zero")},
	"NumericalFunctions::product1": {optionalParam("collection"), param("one")},
	"IntegerFunctions::sum":        {optionalParam("collection")},
	"IntegerFunctions::product":    {optionalParam("collection")},
	"RationalFunctions::sum":       {optionalParam("collection")},
	"RationalFunctions::product":   {optionalParam("collection")},
	"RealFunctions::sum":           {optionalParam("collection")},
	"RealFunctions::product":       {optionalParam("collection")},
}

// invokeBuiltin binds the arguments of a call to the built-in name to its
// declared parameters and applies fn to them.
func (ec *EvalContext) invokeBuiltin(name string, fn builtinFunc, exprs []ast.Node, named []ast.NamedArg, names []string, unbound []error) (Value, error) {
	return ec.invokeBuiltinWith(name, fn, exprs, named, names, unbound, func(params []declaredParam, param, at int) (Value, error) {
		if at < len(exprs) {
			return ec.evalArgument(params, param, exprs[at])
		}
		return ec.evalArgument(params, param, named[at-len(exprs)].Value)
	})
}

// argumentMaterializer yields the value bound to parameter param from the argument
// written at index at, the positional arguments counted before the named ones.
type argumentMaterializer func(params []declaredParam, param, at int) (Value, error)

// invokeBuiltinWith is invokeBuiltin with each argument materialized by materialize.
func (ec *EvalContext) invokeBuiltinWith(name string, fn builtinFunc, exprs []ast.Node, named []ast.NamedArg, names []string, unbound []error, materialize argumentMaterializer) (Value, error) {
	outer := ec.entered
	ec.entered = ec.ctx.activations
	defer func() { ec.entered = outer }()
	return tracedBuiltin(ec.trace, name,
		func() ([]Value, error) { return bindBuiltinArgs(name, len(exprs), names, unbound, materialize) },
		func(args []Value) (Value, error) { return applyBuiltinArgs(ec, name, fn, args) },
	)
}

// applyBuiltinArgs applies a built-in to its bound arguments. One the model leaves
// open leaves the result open, unless the built-in decides such arguments itself.
func applyBuiltinArgs(ec *EvalContext, name string, fn builtinFunc, args []Value) (Value, error) {
	if val, open := ec.ctx.undeterminedInvocation(name, args); open {
		return val, nil
	}
	return fn(ec, args)
}

// tracedBuiltin binds and applies a built-in within the calc enter/bind/exit
// events of any other function; a binding failure closes the level it opened.
func tracedBuiltin(tr *TraceRecorder, name string, bind func() ([]Value, error), apply func([]Value) (Value, error)) (Value, error) {
	if tr == nil {
		args, err := bind()
		if err != nil {
			return Value{}, err
		}
		return apply(args)
	}
	tr.RecordCalcEnter(name)
	args, err := bind()
	if err != nil {
		tr.RecordCalcExitError(name, err)
		return Value{}, err
	}
	for i, p := range builtinSignatures[name] {
		tr.RecordCalcBind(p.name, args[i], "argument")
	}
	result, err := apply(args)
	if err != nil {
		tr.RecordCalcExitError(name, err)
		return Value{}, err
	}
	tr.RecordCalcExit(name, result)
	return result, nil
}

// bindBuiltinArgs materializes a call's arguments into one value per declared
// parameter. Positional arguments bind in sequence; named ones bind the
// parameter of their name. A parameter left unbound is null when its
// multiplicity admits no value and reported otherwise. An `expr` parameter's
// argument is bound unevaluated either way.
func bindBuiltinArgs(name string, positional int, names []string, unbound []error, materialize argumentMaterializer) ([]Value, error) {
	params := builtinSignatures[name]
	for _, argName := range names {
		if argName == "" {
			return nil, fmt.Errorf("unnamed argument in invocation of %s", writtenName(name))
		}
	}
	return bindBuiltin(name, positional, names, unbound, func(param, arg int) (Value, error) {
		if len(names) > 0 {
			return materialize(params, param, positional+arg)
		}
		return materialize(params, param, arg)
	})
}

// bindBuiltinValues binds arguments a direct invocation already evaluated. An
// `expr` parameter takes a body or expression value to apply when selected,
// or the operand's value itself.
func bindBuiltinValues(name string, args calcArgs) ([]Value, error) {
	names := make([]string, 0, len(args.named))
	for argName := range args.named {
		names = append(names, argName)
	}
	slices.Sort(names)
	return bindBuiltin(name, len(args.positional), names, nil, func(_, arg int) (Value, error) {
		if len(names) > 0 {
			return args.named[names[arg]], nil
		}
		return args.positional[arg], nil
	})
}

// bindBuiltin assigns a call's positional arguments, or its named ones, to the
// built-in's declared parameters, materializing each through bind(param, arg);
// a named argument the model could not place (unbound[j]) is reported in its turn.
func bindBuiltin(name string, positional int, named []string, unbound []error, bind func(param, arg int) (Value, error)) ([]Value, error) {
	params := builtinSignatures[name]
	written := writtenName(name)
	if positional > 0 && len(named) > 0 {
		return nil, fmt.Errorf("%w: function %s takes either positional or named arguments", ErrCalcArity, written)
	}
	if positional > len(params) {
		return nil, fmt.Errorf("%w: function %s takes %d argument(s), got %d",
			ErrCalcArity, written, len(params), positional)
	}
	args := make([]Value, len(params))
	bound := make([]bool, len(params))
	for i := 0; i < positional; i++ {
		val, err := bind(i, i)
		if err != nil {
			return nil, err
		}
		args[i], bound[i] = val, true
	}
	for j, argName := range named {
		if unbound != nil && unbound[j] != nil {
			return nil, unbound[j]
		}
		i := slices.IndexFunc(params, func(p declaredParam) bool { return p.name == argName })
		if i < 0 {
			return nil, fmt.Errorf("%w: function %s has no input parameter %q (expected %s)",
				ErrUnknownParameter, written, argName, builtinParameterList(params))
		}
		if bound[i] {
			return nil, fmt.Errorf("%w: function %s binds parameter %q twice", ErrCalcArity, written, argName)
		}
		val, err := bind(i, j)
		if err != nil {
			return nil, err
		}
		args[i], bound[i] = val, true
	}
	return fillUnbound(written, params, args, bound)
}

// fillUnbound binds null to every optional parameter no argument was written
// for and reports the first required one.
func fillUnbound(written string, params []declaredParam, args []Value, bound []bool) ([]Value, error) {
	for i, p := range params {
		if bound[i] {
			continue
		}
		if !p.optional {
			return nil, fmt.Errorf("%w: function %s is called without an argument for parameter %q",
				ErrCalcArity, written, p.name)
		}
		args[i] = nullValue()
	}
	return args, nil
}

// evalArgument evaluates the argument bound to parameter i, or keeps it as
// written for an `expr` parameter. A body literal is a function already, so it
// is evaluated (to itself) like any other argument, which the trace records.
func (ec *EvalContext) evalArgument(params []declaredParam, i int, arg ast.Node) (Value, error) {
	if _, body := arg.(*ast.BodyExpr); !body && i < len(params) && params[i].deferred {
		return NewExprValue(arg, ec.closure()), nil
	}
	return ec.Eval(arg)
}

// builtinParameterList renders the declared parameter names for an error.
func builtinParameterList(params []declaredParam) string {
	names := make([]string, len(params))
	for i, p := range params {
		names[i] = p.name
	}
	return strings.Join(names, ", ")
}

package codegen

import (
	"fmt"
	"math"
	"strings"
)

// LibOp is one library function operation over scalars, chosen from the function
// the model calls and the types of its operands (runtime/library_functions.go).
type LibOp int

const (
	LibSqrt LibOp = iota
	LibAbsReal
	LibAbsInt
	LibFloor
	LibRound
	LibMaxReal
	LibMinReal
	LibMaxInt
	LibMinInt
	LibMaxNatural
	LibMinNatural
	LibIsZero
	LibIsUnit
	LibSin
	LibCos
	LibTan
	LibCot
	LibArcsin
	LibArccos
	LibArctan
	LibDeg
	LibRad
	LibExp
	LibLn
	LibLog
	LibAtan2
)

// libSpec is how one operation types and spells itself in each target. Operand
// placeholders are %s, in operand order.
type libSpec struct {
	name     string
	operands []Type
	result   Type
	c, g     string
}

var libSpecs = map[LibOp]libSpec{
	LibSqrt:       {"sqrt", []Type{TypeReal}, TypeReal, "sysml_lib_real(sqrt(%s))", "sysmlLibReal(math.Sqrt(%s))"},
	LibAbsReal:    {"abs", []Type{TypeReal}, TypeReal, "fabs(%s)", "math.Abs(%s)"},
	LibAbsInt:     {"abs", []Type{TypeInt}, TypeInt, "sysml_iabs(%s)", "sysmlIAbs(%s)"},
	LibFloor:      {"floor", []Type{TypeReal}, TypeInt, "sysml_lib_int(floor(%s))", "sysmlLibInt(math.Floor(%s))"},
	LibRound:      {"round", []Type{TypeReal}, TypeInt, "sysml_lib_int(round(%s))", "sysmlLibInt(math.Round(%s))"},
	LibMaxReal:    {"max", []Type{TypeReal, TypeReal}, TypeReal, "sysml_rmax(%s, %s)", "math.Max(%s, %s)"},
	LibMinReal:    {"min", []Type{TypeReal, TypeReal}, TypeReal, "sysml_rmin(%s, %s)", "math.Min(%s, %s)"},
	LibMaxInt:     {"max", []Type{TypeInt, TypeInt}, TypeInt, "sysml_imax(%s, %s)", "sysmlIMax(%s, %s)"},
	LibMinInt:     {"min", []Type{TypeInt, TypeInt}, TypeInt, "sysml_imin(%s, %s)", "sysmlIMin(%s, %s)"},
	LibMaxNatural: {"max", []Type{TypeInt, TypeInt}, TypeInt, "sysml_natural_max(%s, %s)", "sysmlIMax(sysmlNaturalArg(%s), sysmlNaturalArg(%s))"},
	LibMinNatural: {"min", []Type{TypeInt, TypeInt}, TypeInt, "sysml_natural_min(%s, %s)", "sysmlIMin(sysmlNaturalArg(%s), sysmlNaturalArg(%s))"},
	LibIsZero:     {"isZero", []Type{TypeReal}, TypeBool, "((%s) == 0)", "((%s) == 0)"},
	LibIsUnit:     {"isUnit", []Type{TypeReal}, TypeBool, "((%s) == 1)", "((%s) == 1)"},
	LibSin:        {"sin", []Type{TypeReal}, TypeReal, "sysml_lib_real(sin(%s))", "sysmlLibReal(math.Sin(%s))"},
	LibCos:        {"cos", []Type{TypeReal}, TypeReal, "sysml_lib_real(cos(%s))", "sysmlLibReal(math.Cos(%s))"},
	LibTan:        {"tan", []Type{TypeReal}, TypeReal, "sysml_tan(%s)", "sysmlTan(%s)"},
	LibCot:        {"cot", []Type{TypeReal}, TypeReal, "sysml_cot(%s)", "sysmlCot(%s)"},
	LibArcsin:     {"arcsin", []Type{TypeReal}, TypeReal, "sysml_lib_real(asin(%s))", "sysmlLibReal(math.Asin(%s))"},
	LibArccos:     {"arccos", []Type{TypeReal}, TypeReal, "sysml_lib_real(acos(%s))", "sysmlLibReal(math.Acos(%s))"},
	LibArctan:     {"arctan", []Type{TypeReal}, TypeReal, "sysml_lib_real(atan(%s))", "sysmlLibReal(math.Atan(%s))"},
	LibDeg:        {"deg", []Type{TypeReal}, TypeReal, "sysml_lib_real((%s) * 180 / SYSML_PI)", "sysmlLibReal((%s) * 180 / math.Pi)"},
	LibRad:        {"rad", []Type{TypeReal}, TypeReal, "sysml_lib_real((%s) * SYSML_PI / 180)", "sysmlLibReal((%s) * math.Pi / 180)"},
	LibExp:        {"exp", []Type{TypeReal}, TypeReal, "sysml_lib_real(exp(%s))", "sysmlLibReal(math.Exp(%s))"},
	LibLn:         {"ln", []Type{TypeReal}, TypeReal, "sysml_ln(%s)", "sysmlLn(%s)"},
	LibLog:        {"log", []Type{TypeReal, TypeReal}, TypeReal, "sysml_log(%s, %s)", "sysmlLog(%s, %s)"},
	LibAtan2:      {"atan2", []Type{TypeReal, TypeReal}, TypeReal, "sysml_atan2(%s, %s)", "sysmlAtan2(%s, %s)"},
}

func (op LibOp) String() string { return libSpecs[op].name }

// Operands is the types the operation's operands are coerced to.
func (op LibOp) Operands() []Type { return libSpecs[op].operands }

// Result is the operation's result type.
func (op LibOp) Result() Type { return libSpecs[op].result }

func (op LibOp) cExpr(args []string) string  { return spell(libSpecs[op].c, args) }
func (op LibOp) goExpr(args []string) string { return spell(libSpecs[op].g, args) }

func spell(template string, args []string) string {
	parts := make([]any, len(args))
	for i, a := range args {
		parts[i] = a
	}
	return fmt.Sprintf(template, parts...)
}

// libOpFor is the operation a call of the library function fqn denotes over
// operands of the given types, or why no compiled operation does. Kind-preserving
// functions pick the Integer form for Integer operands, as the interpreter does.
func libOpFor(fqn string, args []Type) (LibOp, string) {
	for _, t := range args {
		if t != TypeInt && t != TypeReal {
			return 0, fmt.Sprintf("%s requires numeric arguments", fqn)
		}
	}
	ints := true
	for _, t := range args {
		ints = ints && t == TypeInt
	}
	pkg, name, _ := strings.Cut(fqn, "::")
	kindPreserving := pkg == "NumericalFunctions" || pkg == "RationalFunctions"
	switch {
	case fqn == "RealFunctions::sqrt":
		return LibSqrt, ""
	case fqn == "RealFunctions::abs":
		return LibAbsReal, ""
	case fqn == "RealFunctions::floor":
		return LibFloor, ""
	case fqn == "RealFunctions::round":
		return LibRound, ""
	case fqn == "RealFunctions::max":
		return LibMaxReal, ""
	case fqn == "RealFunctions::min":
		return LibMinReal, ""
	case kindPreserving && name == "abs":
		if ints {
			return LibAbsInt, ""
		}
		return LibAbsReal, ""
	case kindPreserving && name == "max":
		if ints {
			return LibMaxInt, ""
		}
		return LibMaxReal, ""
	case kindPreserving && name == "min":
		if ints {
			return LibMinInt, ""
		}
		return LibMinReal, ""
	case fqn == "NumericalFunctions::isZero":
		return LibIsZero, ""
	case fqn == "NumericalFunctions::isUnit":
		return LibIsUnit, ""
	case pkg == "IntegerFunctions" || pkg == "NaturalFunctions":
		if !ints {
			return 0, fmt.Sprintf("%s requires Integer arguments, and a Real one never conforms", fqn)
		}
		switch fqn {
		case "IntegerFunctions::abs":
			return LibAbsInt, ""
		case "IntegerFunctions::max":
			return LibMaxInt, ""
		case "IntegerFunctions::min":
			return LibMinInt, ""
		case "NaturalFunctions::max":
			return LibMaxNatural, ""
		case "NaturalFunctions::min":
			return LibMinNatural, ""
		}
	case pkg == "TrigFunctions":
		if op, ok := map[string]LibOp{"sin": LibSin, "cos": LibCos, "tan": LibTan, "cot": LibCot,
			"arcsin": LibArcsin, "arccos": LibArccos, "arctan": LibArctan, "deg": LibDeg, "rad": LibRad}[name]; ok {
			return op, ""
		}
	case pkg == "OpenSysMLMathFunctions":
		if op, ok := map[string]LibOp{"exp": LibExp, "ln": LibLn, "log": LibLog, "atan2": LibAtan2}[name]; ok {
			return op, ""
		}
	}
	return 0, fmt.Sprintf("library function %s is not compiled", fqn)
}

// libFeatureValue is the value of a library feature the compiled subset knows.
func libFeatureValue(fqn string) (Expr, bool) {
	if fqn == "TrigFunctions::pi" {
		return RealLit{Value: math.Pi}, true
	}
	return nil, false
}

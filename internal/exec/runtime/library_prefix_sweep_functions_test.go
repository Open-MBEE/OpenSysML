package runtime

import (
	"fmt"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const sweepFunctionsModel = `package Probe {
	private import ScalarValues::*;
	private import Collections::*;
	part def P1 { part sub : P2[2]; }
	part def P2;
	part p1 : P1;
	part p2 : P2;
	attribute os : OrderedSet { :>> elements = (3, 1, 2); }
	attribute lst : List { :>> elements = (1, 2, 3); }
	attribute arr : Array { :>> elements = (1, 2, 3, 4); :>> dimensions = (2, 2); }
	attribute xs : Integer[*] = (1, 2, 3);
	attribute empty : Integer[*] = ();
	enum def E { a; b; }
	metadata def M;
	#M part def Tagged;
}
package ProbeImports {
	private import ScalarValues::*;
	private import Collections::*;
	private import BaseFunctions::*;
	private import DataFunctions::*;
	private import ScalarFunctions::*;
	private import NumericalFunctions::*;
	private import ControlFunctions::*;
	private import SequenceFunctions::*;
	private import CollectionFunctions::*;
	private import IntegerFunctions::*;
	private import RealFunctions::*;
	private import StringFunctions::*;
	private import BooleanFunctions::*;
	private import TrigFunctions::*;
	private import Probe::*;
}`

type sweepFunctionProbe struct{ fn, expr, want string }

var sweepFunctionProbes = []sweepFunctionProbe{
	// BaseFunctions
	{"BaseFunctions::'=='", "1 == 1", "true"}, {"BaseFunctions::'=='", "BaseFunctions::'=='(1, 2)", "false"},
	{"BaseFunctions::'!='", "1 != 2", "true"}, {"BaseFunctions::'!='", "BaseFunctions::'!='(1, 1)", "false"},
	{"BaseFunctions::'==='", "Probe::p1 === Probe::p1", "true"}, {"BaseFunctions::'==='", "BaseFunctions::'==='(1, 1)", "true"},
	{"BaseFunctions::'!=='", "Probe::p1 !== Probe::p2", "true"}, {"BaseFunctions::'!=='", "BaseFunctions::'!=='(1, 1)", "false"},
	{"BaseFunctions::ToString", "BaseFunctions::ToString(42)", "\"42\""},
	{"BaseFunctions::'['", "2.0 [SI::m]", "2.0 [SI::m]"}, {"BaseFunctions::'['", "BaseFunctions::'['(2.0, SI::m)", "2.0 [m]"},
	{"BaseFunctions::'#'", "(10, 20, 30)#(2)", "20"}, {"BaseFunctions::'#'", "BaseFunctions::'#'((10, 20, 30), 2)", "20"},
	{"BaseFunctions::','", "(1, 2)", "[1, 2]"}, {"BaseFunctions::','", "BaseFunctions::','(1, 2)", "[1, 2]"},
	{"BaseFunctions::'all'", "size(all Probe::E)", "2"}, {"BaseFunctions::'all'", "size(BaseFunctions::'all'(Probe::E))", "2"},
	{"BaseFunctions::'istype'", "1 istype Integer", "true"}, {"BaseFunctions::'istype'", "1 istype String", "false"}, {"BaseFunctions::'istype'", "BaseFunctions::'istype'(1, Integer)", "true"},
	{"BaseFunctions::'hastype'", "1 hastype Integer", "true"}, {"BaseFunctions::'hastype'", "1 hastype Real", "false"}, {"BaseFunctions::'hastype'", "BaseFunctions::'hastype'(1, Integer)", "true"},
	{"BaseFunctions::'@'", "1 @ Integer", "true"}, {"BaseFunctions::'@'", "BaseFunctions::'@'(1, Integer)", "true"},
	{"BaseFunctions::'@@'", "Probe::Tagged @@ Probe::M", "true"}, {"BaseFunctions::'@@'", "Probe::P1 @@ Probe::M", "false"},
	{"BaseFunctions::'as'", "1 as Real", "1"}, {"BaseFunctions::'as'", "BaseFunctions::'as'(1, Real)", "1"}, {"BaseFunctions::'as'", "(1, \"a\", 2) as Integer", "[1, 2]"},
	{"BaseFunctions::'meta'", "Probe::P1 meta SysML::PartDefinition", "[meta(Probe::P1 : SysML::Systems::PartDefinition)]"}, {"BaseFunctions::'meta'", "BaseFunctions::'meta'(Probe::P1, SysML::PartDefinition)", "metaobject"},
	// BooleanFunctions
	{"BooleanFunctions::'not'", "not true", "false"}, {"BooleanFunctions::'not'", "BooleanFunctions::'not'(false)", "true"},
	{"BooleanFunctions::'xor'", "true xor false", "true"}, {"BooleanFunctions::'xor'", "BooleanFunctions::'xor'(true, true)", "false"},
	{"BooleanFunctions::'|'", "true | false", "true"}, {"BooleanFunctions::'|'", "BooleanFunctions::'|'(false, false)", "false"},
	{"BooleanFunctions::'&'", "true & false", "false"}, {"BooleanFunctions::'&'", "BooleanFunctions::'&'(true, true)", "true"},
	{"BooleanFunctions::'=='", "true == true", "true"}, {"BooleanFunctions::'=='", "BooleanFunctions::'=='(true, false)", "false"},
	{"BooleanFunctions::ToString", "BooleanFunctions::ToString(true)", "\"true\""},
	{"BooleanFunctions::ToBoolean", "BooleanFunctions::ToBoolean(\"true\")", "true"}, {"BooleanFunctions::ToBoolean", "BooleanFunctions::ToBoolean(\"false\")", "false"},
	// CollectionFunctions
	{"CollectionFunctions::'=='", "CollectionFunctions::'=='(Probe::lst, Probe::lst)", "true"}, {"CollectionFunctions::'=='", "Probe::lst == Probe::os", "false"},
	{"CollectionFunctions::size", "CollectionFunctions::size(Probe::lst)", "3"},
	{"CollectionFunctions::isEmpty", "CollectionFunctions::isEmpty(Probe::lst)", "false"},
	{"CollectionFunctions::notEmpty", "CollectionFunctions::notEmpty(Probe::lst)", "true"},
	{"CollectionFunctions::contains", "CollectionFunctions::contains(Probe::lst, 2)", "true"}, {"CollectionFunctions::contains", "CollectionFunctions::contains(Probe::lst, 9)", "false"},
	{"CollectionFunctions::containsAll", "CollectionFunctions::containsAll(Probe::lst, Probe::os)", "true"},
	{"CollectionFunctions::head", "CollectionFunctions::head(Probe::os)", "3"},
	{"CollectionFunctions::tail", "CollectionFunctions::tail(Probe::os)", "[1, 2]"},
	{"CollectionFunctions::last", "CollectionFunctions::last(Probe::os)", "2"},
	{"CollectionFunctions::'#'", "CollectionFunctions::'#'(Probe::os, 1)", "3"}, {"CollectionFunctions::'#'", "Probe::os#(1)", "3"},
	{"CollectionFunctions::'array#'", "CollectionFunctions::'array#'(Probe::arr, (2, 1))", "3"}, {"CollectionFunctions::'array#'", "Probe::arr#(2, 1)", "3"},
	// ComplexFunctions
	{"ComplexFunctions::rect", "ComplexFunctions::rect(1.0, 2.0)", "1.0 + 2.0i"},
	{"ComplexFunctions::polar", "ComplexFunctions::re(ComplexFunctions::polar(2.0, 0.0))", "2.0"},
	{"ComplexFunctions::re", "ComplexFunctions::re(ComplexFunctions::rect(1.0, 2.0))", "1.0"},
	{"ComplexFunctions::im", "ComplexFunctions::im(ComplexFunctions::rect(1.0, 2.0))", "2.0"},
	{"ComplexFunctions::isZero", "ComplexFunctions::isZero(ComplexFunctions::rect(0.0, 0.0))", "true"},
	{"ComplexFunctions::isUnit", "ComplexFunctions::isUnit(ComplexFunctions::rect(1.0, 0.0))", "true"},
	{"ComplexFunctions::abs", "ComplexFunctions::abs(ComplexFunctions::rect(3.0, 4.0))", "5.0"},
	{"ComplexFunctions::arg", "ComplexFunctions::arg(ComplexFunctions::rect(1.0, 0.0))", "0.0"},
	{"ComplexFunctions::'+'", "ComplexFunctions::'+'(ComplexFunctions::rect(1.0, 2.0), ComplexFunctions::rect(1.0, 1.0))", "2.0 + 3.0i"}, {"ComplexFunctions::'+'", "ComplexFunctions::rect(1.0, 2.0) + ComplexFunctions::rect(1.0, 1.0)", "2.0 + 3.0i"},
	{"ComplexFunctions::'-'", "ComplexFunctions::rect(1.0, 2.0) - ComplexFunctions::rect(1.0, 1.0)", "0.0 + 1.0i"},
	{"ComplexFunctions::'*'", "ComplexFunctions::rect(0.0, 1.0) * ComplexFunctions::rect(0.0, 1.0)", "-1.0 + 0.0i"},
	{"ComplexFunctions::'/'", "ComplexFunctions::rect(2.0, 0.0) / ComplexFunctions::rect(1.0, 0.0)", "2.0 + 0.0i"},
	{"ComplexFunctions::'**'", "ComplexFunctions::rect(0.0, 1.0) ** 2", "-1.0 + 0.0i"},
	{"ComplexFunctions::'^'", "ComplexFunctions::rect(0.0, 1.0) ^ 2", "-1.0 + 0.0i"},
	{"ComplexFunctions::'=='", "ComplexFunctions::rect(1.0, 2.0) == ComplexFunctions::rect(1.0, 2.0)", "true"},
	{"ComplexFunctions::ToString", "ComplexFunctions::ToString(ComplexFunctions::rect(1.0, 2.0))", "string"},
	{"ComplexFunctions::ToComplex", "ComplexFunctions::ToComplex(\"1.0 + 2.0i\")", "1.0 + 2.0i"},
	{"ComplexFunctions::sum", "ComplexFunctions::sum((ComplexFunctions::rect(1.0, 0.0), ComplexFunctions::rect(0.0, 1.0)))", "1.0 + 1.0i"},
	{"ComplexFunctions::product", "ComplexFunctions::product((ComplexFunctions::rect(2.0, 0.0), ComplexFunctions::rect(0.0, 1.0)))", "0.0 + 2.0i"},
	// ControlFunctions
	{"ControlFunctions::'.'", "Probe::p1.sub", "collection(2)"}, {"ControlFunctions::'.'", "(Probe::p1, Probe::p1).sub", "collection(4)"},
	{"ControlFunctions::'if'", "if true ? 1 else 2", "1"}, {"ControlFunctions::'if'", "if false ? 1 else 2", "2"}, {"ControlFunctions::'if'", "ControlFunctions::'if'(true, 1, 2)", "1"}, {"ControlFunctions::'if'", "if true ? 1 else 1/0", "1"},
	{"ControlFunctions::'??'", "null ?? 3", "3"}, {"ControlFunctions::'??'", "Probe::empty ?? 3", "3"}, {"ControlFunctions::'??'", "2 ?? 3", "2"}, {"ControlFunctions::'??'", "ControlFunctions::'??'((), 5)", "5"}, {"ControlFunctions::'??'", "1 ?? 1/0", "1"},
	{"ControlFunctions::'and'", "true and false", "false"}, {"ControlFunctions::'and'", "false and (1/0 == 0)", "false"}, {"ControlFunctions::'and'", "ControlFunctions::'and'(true, true)", "true"},
	{"ControlFunctions::'or'", "true or (1/0 == 0)", "true"}, {"ControlFunctions::'or'", "false or false", "false"}, {"ControlFunctions::'or'", "ControlFunctions::'or'(false, true)", "true"},
	{"ControlFunctions::'implies'", "false implies (1/0 == 0)", "true"}, {"ControlFunctions::'implies'", "true implies false", "false"}, {"ControlFunctions::'implies'", "ControlFunctions::'implies'(true, true)", "true"},
	{"ControlFunctions::collect", "(1, 2, 3)->collect{in x; x * 2}", "[2, 4, 6]"}, {"ControlFunctions::collect", "ControlFunctions::collect((1, 2, 3), {in x; x * 2})", "[2, 4, 6]"},
	{"ControlFunctions::select", "(1, 2, 3)->select{in x; x > 1}", "[2, 3]"}, {"ControlFunctions::select", "ControlFunctions::select((1, 2, 3), {in x; x > 1})", "[2, 3]"},
	{"ControlFunctions::selectOne", "(1, 2, 3)->selectOne{in x; x > 1}", "2"}, {"ControlFunctions::selectOne", "ControlFunctions::selectOne((1, 2, 3), {in x; x > 1})", "2"},
	{"ControlFunctions::reject", "(1, 2, 3)->reject{in x; x > 1}", "[1]"}, {"ControlFunctions::reject", "ControlFunctions::reject((1, 2, 3), {in x; x > 1})", "[1]"},
	{"ControlFunctions::reduce", "(1, 2, 3)->reduce{in a; in b; a + b}", "6"}, {"ControlFunctions::reduce", "(1, 2, 3)->reduce '+'", "6"}, {"ControlFunctions::reduce", "ControlFunctions::reduce((1, 2, 3), {in a; in b; a + b})", "6"},
	{"ControlFunctions::forAll", "(1, 2, 3)->forAll{in x; x > 0}", "true"}, {"ControlFunctions::forAll", "ControlFunctions::forAll((1, 2, 3), {in x; x > 1})", "false"},
	{"ControlFunctions::exists", "(1, 2, 3)->exists{in x; x > 2}", "true"}, {"ControlFunctions::exists", "ControlFunctions::exists((1, 2, 3), {in x; x > 3})", "false"},
	{"ControlFunctions::allTrue", "ControlFunctions::allTrue((true, true))", "true"}, {"ControlFunctions::allTrue", "(true, false)->allTrue()", "false"},
	{"ControlFunctions::anyTrue", "ControlFunctions::anyTrue((false, true))", "true"}, {"ControlFunctions::anyTrue", "(false, false)->anyTrue()", "false"},
	{"ControlFunctions::minimize", "(3, 1, 2)->minimize{in x; x}", "1"}, {"ControlFunctions::minimize", "ControlFunctions::minimize((3, 1, 2), {in x; -x})", "-3"},
	{"ControlFunctions::maximize", "(3, 1, 2)->maximize{in x; x}", "3"}, {"ControlFunctions::maximize", "ControlFunctions::maximize((3, 1, 2), {in x; -x})", "-1"},
	// DataFunctions (concrete only)
	{"DataFunctions::'==='", "DataFunctions::'==='(1, 1)", "true"}, {"DataFunctions::'==='", "1 === 2", "false"},
	// IntegerFunctions
	{"IntegerFunctions::abs", "IntegerFunctions::abs(-3)", "3"}, {"IntegerFunctions::abs", "abs(-3)", "3"},
	{"IntegerFunctions::'+'", "1 + 2", "3"}, {"IntegerFunctions::'+'", "IntegerFunctions::'+'(1, 2)", "3"}, {"IntegerFunctions::'+'", "IntegerFunctions::'+'(1)", "1"},
	{"IntegerFunctions::'-'", "5 - 2", "3"}, {"IntegerFunctions::'-'", "IntegerFunctions::'-'(5)", "-5"}, {"IntegerFunctions::'-'", "-5", "-5"},
	{"IntegerFunctions::'*'", "2 * 3", "6"}, {"IntegerFunctions::'*'", "IntegerFunctions::'*'(2, 3)", "6"},
	{"IntegerFunctions::'/'", "7 / 2", "3.5"}, {"IntegerFunctions::'/'", "IntegerFunctions::'/'(6, 2)", "3.0"},
	{"IntegerFunctions::'**'", "2 ** 3", "8"}, {"IntegerFunctions::'**'", "IntegerFunctions::'**'(2, 3)", "8"},
	{"IntegerFunctions::'^'", "2 ^ 3", "8"}, {"IntegerFunctions::'^'", "IntegerFunctions::'^'(2, 3)", "8"},
	{"IntegerFunctions::'%'", "7 % 2", "1"}, {"IntegerFunctions::'%'", "IntegerFunctions::'%'(-7, 2)", "-1"},
	{"IntegerFunctions::'<'", "1 < 2", "true"}, {"IntegerFunctions::'<'", "IntegerFunctions::'<'(2, 1)", "false"},
	{"IntegerFunctions::'>'", "2 > 1", "true"}, {"IntegerFunctions::'>'", "IntegerFunctions::'>'(1, 2)", "false"},
	{"IntegerFunctions::'<='", "2 <= 2", "true"}, {"IntegerFunctions::'<='", "IntegerFunctions::'<='(3, 2)", "false"},
	{"IntegerFunctions::'>='", "2 >= 2", "true"}, {"IntegerFunctions::'>='", "IntegerFunctions::'>='(1, 2)", "false"},
	{"IntegerFunctions::max", "IntegerFunctions::max(1, 2)", "2"}, {"IntegerFunctions::max", "max(1, 2)", "2"},
	{"IntegerFunctions::min", "IntegerFunctions::min(1, 2)", "1"}, {"IntegerFunctions::min", "min(1, 2)", "1"},
	{"IntegerFunctions::'=='", "IntegerFunctions::'=='(1, 1)", "true"},
	{"IntegerFunctions::'..'", "1..3", "[1, 2, 3]"}, {"IntegerFunctions::'..'", "IntegerFunctions::'..'(1, 3)", "[1, 2, 3]"}, {"IntegerFunctions::'..'", "3..1", "[]"},
	{"IntegerFunctions::ToString", "IntegerFunctions::ToString(7)", "\"7\""},
	{"IntegerFunctions::ToNatural", "IntegerFunctions::ToNatural(7)", "7"},
	{"IntegerFunctions::ToInteger", "IntegerFunctions::ToInteger(\"7\")", "7"},
	{"IntegerFunctions::sum", "IntegerFunctions::sum((1, 2, 3))", "6"}, {"IntegerFunctions::sum", "sum((1, 2, 3))", "6"}, {"IntegerFunctions::sum", "IntegerFunctions::sum(())", "0"},
	{"IntegerFunctions::product", "IntegerFunctions::product((1, 2, 3))", "6"}, {"IntegerFunctions::product", "product((2, 3))", "6"}, {"IntegerFunctions::product", "IntegerFunctions::product(())", "1"},
	// NaturalFunctions
	{"NaturalFunctions::'+'", "NaturalFunctions::'+'(1, 2)", "3"}, {"NaturalFunctions::'*'", "NaturalFunctions::'*'(2, 3)", "6"},
	{"NaturalFunctions::'/'", "NaturalFunctions::'/'(7, 2)", "3"}, {"NaturalFunctions::'%'", "NaturalFunctions::'%'(7, 2)", "1"},
	{"NaturalFunctions::'<'", "NaturalFunctions::'<'(1, 2)", "true"}, {"NaturalFunctions::'>'", "NaturalFunctions::'>'(1, 2)", "false"},
	{"NaturalFunctions::'<='", "NaturalFunctions::'<='(2, 2)", "true"}, {"NaturalFunctions::'>='", "NaturalFunctions::'>='(1, 2)", "false"},
	{"NaturalFunctions::max", "NaturalFunctions::max(1, 2)", "2"}, {"NaturalFunctions::min", "NaturalFunctions::min(1, 2)", "1"},
	{"NaturalFunctions::'=='", "NaturalFunctions::'=='(1, 1)", "true"}, {"NaturalFunctions::ToString", "NaturalFunctions::ToString(7)", "\"7\""},
	{"NaturalFunctions::ToNatural", "NaturalFunctions::ToNatural(\"7\")", "7"},
	// NumericalFunctions concrete
	{"NumericalFunctions::sum0", "NumericalFunctions::sum0((1, 2), 0)", "3"}, {"NumericalFunctions::sum0", "NumericalFunctions::sum0((), 0)", "0"},
	{"NumericalFunctions::product1", "NumericalFunctions::product1((2, 3), 1)", "6"}, {"NumericalFunctions::product1", "NumericalFunctions::product1((), 1)", "1"},
	// OccurrenceFunctions
	{"OccurrenceFunctions::'==='", "OccurrenceFunctions::'==='(Probe::p1, Probe::p1)", "true"}, {"OccurrenceFunctions::'==='", "OccurrenceFunctions::'==='(Probe::p1, Probe::p2)", "false"},
	{"OccurrenceFunctions::isDuring", "OccurrenceFunctions::isDuring(Probe::p1)", "true"},
	{"OccurrenceFunctions::create", "OccurrenceFunctions::create(Probe::p1)", "object"}, {"OccurrenceFunctions::create", "OccurrenceFunctions::create(new Probe::P2())", "object"},
	{"OccurrenceFunctions::destroy", "OccurrenceFunctions::destroy(new Probe::P2())", "object"},
	{"OccurrenceFunctions::addNew", "size(OccurrenceFunctions::addNew(Probe::p1.sub, new Probe::P2()))", "3"},
	{"OccurrenceFunctions::addNewAt", "size(OccurrenceFunctions::addNewAt(Probe::p1.sub, new Probe::P2(), 1))", "3"},
	// RationalFunctions
	{"RationalFunctions::rat", "RationalFunctions::rat(1, 2)", "0.5"}, {"RationalFunctions::numer", "RationalFunctions::numer(RationalFunctions::rat(2, 4))", "1"}, {"RationalFunctions::denom", "RationalFunctions::denom(RationalFunctions::rat(2, 4))", "2"},
	{"RationalFunctions::abs", "RationalFunctions::abs(RationalFunctions::rat(-1, 2))", "0.5"},
	{"RationalFunctions::'+'", "RationalFunctions::rat(1, 2) + RationalFunctions::rat(1, 2)", "1.0"}, {"RationalFunctions::'+'", "RationalFunctions::'+'(RationalFunctions::rat(1, 3), RationalFunctions::rat(1, 3))", "2/3"},
	{"RationalFunctions::'-'", "RationalFunctions::rat(1, 2) - RationalFunctions::rat(1, 4)", "0.25"},
	{"RationalFunctions::'*'", "RationalFunctions::rat(1, 2) * RationalFunctions::rat(1, 2)", "0.25"},
	{"RationalFunctions::'/'", "RationalFunctions::rat(1, 2) / RationalFunctions::rat(1, 4)", "2.0"},
	{"RationalFunctions::'**'", "RationalFunctions::rat(1, 2) ** 2", "0.25"}, {"RationalFunctions::'^'", "RationalFunctions::rat(1, 2) ^ 2", "0.25"},
	{"RationalFunctions::'<'", "RationalFunctions::rat(1, 2) < RationalFunctions::rat(3, 4)", "true"}, {"RationalFunctions::'>'", "RationalFunctions::rat(1, 2) > RationalFunctions::rat(3, 4)", "false"},
	{"RationalFunctions::'<='", "RationalFunctions::rat(1, 2) <= RationalFunctions::rat(1, 2)", "true"}, {"RationalFunctions::'>='", "RationalFunctions::rat(1, 2) >= RationalFunctions::rat(3, 4)", "false"},
	{"RationalFunctions::max", "RationalFunctions::max(RationalFunctions::rat(1, 2), RationalFunctions::rat(3, 4))", "0.75"}, {"RationalFunctions::min", "RationalFunctions::min(RationalFunctions::rat(1, 2), RationalFunctions::rat(3, 4))", "0.5"},
	{"RationalFunctions::'=='", "RationalFunctions::rat(1, 2) == RationalFunctions::rat(2, 4)", "true"},
	{"RationalFunctions::gcd", "RationalFunctions::gcd(12, 18)", "6"},
	{"RationalFunctions::floor", "RationalFunctions::floor(RationalFunctions::rat(7, 2))", "3"}, {"RationalFunctions::round", "RationalFunctions::round(RationalFunctions::rat(7, 2))", "4"},
	{"RationalFunctions::ToString", "RationalFunctions::ToString(RationalFunctions::rat(1, 2))", "\"0.5\""}, {"RationalFunctions::ToInteger", "RationalFunctions::ToInteger(RationalFunctions::rat(4, 2))", "2"},
	{"RationalFunctions::ToRational", "RationalFunctions::ToRational(\"1/2\")", "0.5"},
	{"RationalFunctions::sum", "RationalFunctions::sum((RationalFunctions::rat(1, 2), RationalFunctions::rat(1, 2)))", "1.0"}, {"RationalFunctions::product", "RationalFunctions::product((RationalFunctions::rat(1, 2), RationalFunctions::rat(1, 2)))", "0.25"},
	// RealFunctions
	{"RealFunctions::re", "RealFunctions::re(2.5)", "2.5"}, {"RealFunctions::im", "RealFunctions::im(2.5)", "0.0"},
	{"RealFunctions::abs", "RealFunctions::abs(-2.5)", "2.5"}, {"RealFunctions::arg", "RealFunctions::arg(-1.0)", "0.0"}, {"RealFunctions::arg", "RealFunctions::arg(1.0)", "0.0"},
	{"RealFunctions::'+'", "1.5 + 1.0", "2.5"}, {"RealFunctions::'-'", "1.5 - 1.0", "0.5"}, {"RealFunctions::'*'", "1.5 * 2.0", "3.0"}, {"RealFunctions::'/'", "3.0 / 2.0", "1.5"},
	{"RealFunctions::'**'", "2.0 ** 3", "8.0"}, {"RealFunctions::'^'", "2.0 ^ 3", "8.0"}, {"RealFunctions::'**'", "RealFunctions::'**'(2.0, 0.5)", "1.4142135623730951"},
	{"RealFunctions::'<'", "1.0 < 2.0", "true"}, {"RealFunctions::'>'", "1.0 > 2.0", "false"}, {"RealFunctions::'<='", "2.0 <= 2.0", "true"}, {"RealFunctions::'>='", "1.0 >= 2.0", "false"},
	{"RealFunctions::max", "RealFunctions::max(1.0, 2.0)", "2.0"}, {"RealFunctions::min", "RealFunctions::min(1.0, 2.0)", "1.0"},
	{"RealFunctions::'=='", "1.0 == 1.0", "true"},
	{"RealFunctions::sqrt", "RealFunctions::sqrt(4.0)", "2.0"}, {"RealFunctions::sqrt", "sqrt(4.0)", "2.0"},
	{"RealFunctions::floor", "RealFunctions::floor(2.7)", "2"}, {"RealFunctions::round", "RealFunctions::round(2.5)", "3"}, {"RealFunctions::round", "RealFunctions::round(-2.5)", "-3"},
	{"RealFunctions::ToString", "RealFunctions::ToString(2.5)", "\"2.5\""}, {"RealFunctions::ToInteger", "RealFunctions::ToInteger(2.7)", "2"}, {"RealFunctions::ToRational", "RealFunctions::ToRational(0.5)", "0.5"},
	{"RealFunctions::ToReal", "RealFunctions::ToReal(\"2.5\")", "2.5"},
	{"RealFunctions::sum", "RealFunctions::sum((1.0, 2.5))", "3.5"}, {"RealFunctions::sum", "RealFunctions::sum(())", "0.0"}, {"RealFunctions::product", "RealFunctions::product((2.0, 2.5))", "5.0"}, {"RealFunctions::product", "RealFunctions::product(())", "1.0"},
	// SequenceFunctions
	{"SequenceFunctions::'#'", "SequenceFunctions::'#'((10, 20), 2)", "20"},
	{"SequenceFunctions::equals", "SequenceFunctions::equals((1, 2), (1, 2))", "true"}, {"SequenceFunctions::equals", "(1, 2)->equals((2, 1))", "false"},
	{"SequenceFunctions::same", "SequenceFunctions::same((Probe::p1, Probe::p2), (Probe::p1, Probe::p2))", "true"}, {"SequenceFunctions::same", "SequenceFunctions::same((1, 2), (1, 2))", "true"},
	{"SequenceFunctions::size", "SequenceFunctions::size((1, 2, 3))", "3"}, {"SequenceFunctions::size", "size((1, 2, 3))", "3"}, {"SequenceFunctions::size", "(1, 2, 3)->size()", "3"}, {"SequenceFunctions::size", "size(())", "0"},
	{"SequenceFunctions::isEmpty", "SequenceFunctions::isEmpty(())", "true"}, {"SequenceFunctions::isEmpty", "isEmpty((1))", "false"}, {"SequenceFunctions::isEmpty", "()->isEmpty()", "true"},
	{"SequenceFunctions::notEmpty", "SequenceFunctions::notEmpty((1))", "true"}, {"SequenceFunctions::notEmpty", "notEmpty(())", "false"},
	{"SequenceFunctions::includes", "SequenceFunctions::includes((1, 2), 2)", "true"}, {"SequenceFunctions::includes", "(1, 2)->includes(3)", "false"}, {"SequenceFunctions::includes", "includes((1, 2), (1, 2))", "true"},
	{"SequenceFunctions::includesOnly", "SequenceFunctions::includesOnly((1, 1), 1)", "true"}, {"SequenceFunctions::includesOnly", "SequenceFunctions::includesOnly((1, 2), 1)", "false"},
	{"SequenceFunctions::excludes", "SequenceFunctions::excludes((1, 2), 3)", "true"}, {"SequenceFunctions::excludes", "(1, 2)->excludes(2)", "false"},
	{"SequenceFunctions::union", "SequenceFunctions::union((1, 2), (2, 3))", "[1, 2, 2, 3]"}, {"SequenceFunctions::union", "(1, 2)->union((3))", "[1, 2, 3]"},
	{"SequenceFunctions::intersection", "SequenceFunctions::intersection((1, 2), (2, 3))", "[2]"}, {"SequenceFunctions::intersection", "(1, 2, 2)->intersection((2))", "[2, 2]"},
	{"SequenceFunctions::including", "SequenceFunctions::including((1, 2), 3)", "[1, 2, 3]"}, {"SequenceFunctions::including", "(1, 2)->including(3)", "[1, 2, 3]"},
	{"SequenceFunctions::includingAt", "SequenceFunctions::includingAt((1, 3), 2, 2)", "[1, 2, 3]"}, {"SequenceFunctions::includingAt", "(1, 3)->includingAt(2, 2)", "[1, 2, 3]"},
	{"SequenceFunctions::excluding", "SequenceFunctions::excluding((1, 2, 3), 2)", "[1, 3]"}, {"SequenceFunctions::excluding", "(1, 2, 2, 3)->excluding(2)", "[1, 3]"},
	{"SequenceFunctions::excludingAt", "SequenceFunctions::excludingAt((1, 2, 3), 2)", "[1, 3]"}, {"SequenceFunctions::excludingAt", "(1, 2, 3)->excludingAt(2)", "[1, 3]"},
	{"SequenceFunctions::subsequence", "SequenceFunctions::subsequence((1, 2, 3, 4), 2, 3)", "[2, 3]"}, {"SequenceFunctions::subsequence", "(1, 2, 3, 4)->subsequence(2, 3)", "[2, 3]"},
	{"SequenceFunctions::head", "SequenceFunctions::head((1, 2, 3))", "1"}, {"SequenceFunctions::head", "(1, 2, 3)->head()", "1"}, {"SequenceFunctions::head", "head(())", "null"},
	{"SequenceFunctions::tail", "SequenceFunctions::tail((1, 2, 3))", "[2, 3]"}, {"SequenceFunctions::tail", "(1, 2, 3)->tail()", "[2, 3]"}, {"SequenceFunctions::tail", "tail((1))", "[]"},
	{"SequenceFunctions::last", "SequenceFunctions::last((1, 2, 3))", "3"}, {"SequenceFunctions::last", "(1, 2, 3)->last()", "3"}, {"SequenceFunctions::last", "last(())", "null"},
	// StringFunctions
	{"StringFunctions::'+'", "\"a\" + \"b\"", "\"ab\""}, {"StringFunctions::'+'", "StringFunctions::'+'(\"a\", \"b\")", "\"ab\""},
	{"StringFunctions::Length", "StringFunctions::Length(\"abc\")", "3"}, {"StringFunctions::Length", "Length(\"\")", "0"},
	{"StringFunctions::Substring", "StringFunctions::Substring(\"abcd\", 2, 3)", "\"bc\""}, {"StringFunctions::Substring", "Substring(\"abcd\", 1, 1)", "\"a\""},
	{"StringFunctions::'<'", "\"a\" < \"b\"", "true"}, {"StringFunctions::'>'", "\"a\" > \"b\"", "false"}, {"StringFunctions::'<='", "\"a\" <= \"a\"", "true"}, {"StringFunctions::'>='", "\"a\" >= \"b\"", "false"},
	{"StringFunctions::'=='", "\"a\" == \"a\"", "true"}, {"StringFunctions::ToString", "StringFunctions::ToString(\"a\")", "\"a\""},
	// TrigFunctions
	{"TrigFunctions::deg", "TrigFunctions::deg(TrigFunctions::pi)", "180.0"}, {"TrigFunctions::rad", "TrigFunctions::rad(180.0)", "3.141592653589793"},
	{"TrigFunctions::sin", "TrigFunctions::sin(0.0)", "0.0"}, {"TrigFunctions::sin", "TrigFunctions::sin(90.0)", "0.893996663600558"},
	{"TrigFunctions::cos", "TrigFunctions::cos(0.0)", "1.0"}, {"TrigFunctions::cos", "TrigFunctions::cos(180.0)", "-0.5984600690578581"},
	{"TrigFunctions::tan", "TrigFunctions::tan(0.0)", "0.0"}, {"TrigFunctions::tan", "TrigFunctions::tan(45.0)", "1.6197751905438613"},
	{"TrigFunctions::cot", "TrigFunctions::cot(45.0)", "0.6173696237835552"},
	{"TrigFunctions::arcsin", "TrigFunctions::arcsin(1.0)", "1.5707963267948966"}, {"TrigFunctions::arccos", "TrigFunctions::arccos(1.0)", "0.0"}, {"TrigFunctions::arctan", "TrigFunctions::arctan(1.0)", "0.7853981633974483"},
	// VectorFunctions
	{"VectorFunctions::VectorOf", "VectorFunctions::VectorOf((1, 2, 3)).elements", "[1, 2, 3]"},
	{"VectorFunctions::CartesianVectorOf", "VectorFunctions::CartesianVectorOf((1.0, 2.0)).elements", "[1.0, 2.0]"},
	{"VectorFunctions::CartesianThreeVectorOf", "VectorFunctions::CartesianThreeVectorOf((1.0, 2.0, 3.0)).elements", "[1.0, 2.0, 3.0]"},
	{"VectorFunctions::isCartesianZeroVector", "VectorFunctions::isCartesianZeroVector(VectorFunctions::CartesianVectorOf((0.0, 0.0)))", "true"},
	{"VectorFunctions::'cartesian+'", "VectorFunctions::'cartesian+'(VectorFunctions::CartesianVectorOf((1.0, 2.0)), VectorFunctions::CartesianVectorOf((1.0, 1.0))).elements", "[2.0, 3.0]"},
	{"VectorFunctions::'cartesian-'", "VectorFunctions::'cartesian-'(VectorFunctions::CartesianVectorOf((1.0, 2.0)), VectorFunctions::CartesianVectorOf((1.0, 1.0))).elements", "[0.0, 1.0]"},
	{"VectorFunctions::cartesianScalarVectorMult", "VectorFunctions::cartesianScalarVectorMult(2.0, VectorFunctions::CartesianVectorOf((1.0, 2.0))).elements", "[2.0, 4.0]"},
	{"VectorFunctions::cartesianVectorScalarMult", "VectorFunctions::cartesianVectorScalarMult(VectorFunctions::CartesianVectorOf((1.0, 2.0)), 2.0).elements", "[2.0, 4.0]"},
	{"VectorFunctions::cartesianInner", "VectorFunctions::cartesianInner(VectorFunctions::CartesianVectorOf((1.0, 2.0)), VectorFunctions::CartesianVectorOf((3.0, 4.0)))", "11.0"},
	{"VectorFunctions::cartesianNorm", "VectorFunctions::cartesianNorm(VectorFunctions::CartesianVectorOf((3.0, 4.0)))", "5.0"},
	{"VectorFunctions::cartesianAngle", "VectorFunctions::cartesianAngle(VectorFunctions::CartesianVectorOf((1.0, 0.0)), VectorFunctions::CartesianVectorOf((0.0, 1.0)))", "1.5707963267948966"},
	{"VectorFunctions::sum", "VectorFunctions::sum((VectorFunctions::CartesianThreeVectorOf((1.0, 0.0, 0.0)), VectorFunctions::CartesianThreeVectorOf((0.0, 1.0, 0.0)))).elements", "[1.0, 1.0, 0.0]"},
	{"VectorFunctions::'+'", "VectorFunctions::CartesianVectorOf((1.0, 2.0)) + VectorFunctions::CartesianVectorOf((1.0, 1.0))", "⟨2.0, 3.0⟩"},
	{"VectorFunctions::scalarVectorMult", "VectorFunctions::CartesianVectorOf((1.0, 2.0)) * 2.0", "⟨2.0, 4.0⟩"},
}

// sweepKernelFunctionCalls evaluates every Kernel Function Library probe in the probe
// package, and again with the library imported where the bare name did not resolve.
func sweepKernelFunctionCalls(t *testing.T) []string {
	t.Helper()
	ctx, idx := libraryShapeContext(t, sweepFunctionsModel)
	root := idx.DocumentRoot("<test>")
	pkgSym, _ := root.LookupLocal("Probe")
	impSym, _ := root.LookupLocal("ProbeImports")
	var out []string
	for _, p := range sweepFunctionProbes {
		out = append(out, "call\t"+p.fn+"\t"+p.expr+"\t"+sweepGuarded(func() string {
			val, err := sweepEval(ctx, pkgSym.Scope, p.expr)
			o := sweepOutcome(ctx, val, err)
			if strings.Contains(o, "unresolved reference") {
				val, err = sweepEval(ctx, impSym.Scope, p.expr)
				o += " || with imports: " + sweepOutcome(ctx, val, err)
			}
			return o + sweepVerdict(p.want, val, err)
		}))
	}
	return out
}

// sweepGuarded runs one probe, recording a panic as its outcome rather than failing the sweep.
func sweepGuarded(probe func() string) (outcome string) {
	defer func() {
		if r := recover(); r != nil {
			outcome = fmt.Sprintf("PANIC %v", r)
		}
	}()
	return probe()
}

func sweepEval(ctx *Context, scope *symbols.Scope, src string) (Value, error) {
	p := parser.New(source.New("<expr>", []byte(src)))
	expr := p.ParseExpression()
	if expr == nil || len(p.Diagnostics) > 0 {
		return Value{}, fmt.Errorf("PARSE %v", p.Diagnostics)
	}
	return ctx.EvalWithScope(expr, scope)
}

// sweepVerdict compares a probe's value with the value the library text calls for (`object`
// and `collection(n)` name a kind): `ok`, `WRONG` with both values, or `UNEVALUATED` on an error.
func sweepVerdict(want string, val Value, err error) string {
	switch {
	case want == "":
		return ""
	case err != nil:
		return "\tUNEVALUATED want " + want
	case want == "object" && val.Kind == ValInstance,
		want == fmt.Sprintf("collection(%d)", len(elementsOf(val))) && (val.Kind == ValSequence || val.Kind == ValSet),
		FormatValue(val) == want:
		return "\tok"
	}
	got := FormatValue(val)
	if val.Kind == ValInstance {
		got = "object"
	}
	return "\tWRONG want " + want + ", got " + got
}

// sweepFunctionPackages are the Kernel Function Library packages the probes cover.
var sweepFunctionPackages = []string{
	"BaseFunctions", "BooleanFunctions", "CollectionFunctions", "ComplexFunctions", "ControlFunctions",
	"DataFunctions", "IntegerFunctions", "NaturalFunctions", "NumericalFunctions", "OccurrenceFunctions",
	"RationalFunctions", "RealFunctions", "ScalarFunctions", "SequenceFunctions", "StringFunctions",
	"TrigFunctions", "VectorFunctions",
}

// TestKernelFunctionLibraryProbesEveryDeclaration fails a concrete Kernel Function Library
// function with no probe, and a probe naming no function: a new function cannot evade the sweep.
func TestKernelFunctionLibraryProbesEveryDeclaration(t *testing.T) {
	ctx := newLibraryContext(t, libs.NewModelIndex())
	// A probe quotes an operator name (`BaseFunctions::'=='`); the index names it bare.
	probed := map[string]int{}
	for _, p := range sweepFunctionProbes {
		probed[strings.ReplaceAll(p.fn, "'", "")]++
	}
	for _, pkg := range sweepFunctionPackages {
		for _, decl := range callableDeclarations(t, ctx, pkg) {
			sym, function := sweepFunctionDeclaration(ctx, decl)
			if !function {
				continue
			}
			// An abstract function (`abstract function '=='`) is called through the operator
			// a concrete library function binds, so it needs no probe of its own.
			if probed[decl] == 0 && !symbols.IsAbstract(sym) {
				t.Errorf("%s has no probe", decl)
			}
			delete(probed, decl)
		}
	}
	for decl := range probed {
		t.Errorf("%s is probed and is not a function the Kernel Function Library declares", decl)
	}
}

// sweepFunctionDeclaration is the function or predicate qualified names directly in its package;
// a behavior (`SequenceFunctions::add`) or a nested invariant is not called from an expression.
func sweepFunctionDeclaration(ctx *Context, qualified string) (*symbols.Symbol, bool) {
	if strings.Count(qualified, "::") != 1 {
		return nil, false
	}
	matches := ctx.model.resolver.Index().LookupQualified(qualified)
	if len(matches) != 1 {
		return nil, false
	}
	switch decl := matches[0].Decl.(type) {
	case *ast.Definition:
		if decl.Kind == ast.DefCalc || decl.Kind == ast.DefPredicate {
			return matches[0], true
		}
	case *ast.Usage:
		if decl.Direction == ast.DirNone && (decl.Kind == ast.UsageCalc || decl.Kind == ast.UsagePredicate) {
			return matches[0], true
		}
	}
	return nil, false
}

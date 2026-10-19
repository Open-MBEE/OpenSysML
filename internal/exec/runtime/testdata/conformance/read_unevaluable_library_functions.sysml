// The Kernel Function Library declarations the runtime does not evaluate as
// function calls, each given a real receiver, and what the library text says of
// each. An abstract function whose type parameter is a type reference
// (BaseFunctions `istype`, `hastype`, `@`, `as`, `meta`, `all`; KerML 8.3.4.8
// ClassificationExpression, ExtentExpression, MetadataAccessExpression) and the
// abstract `'['` evaluate in their operator notation, and the call form is
// refused with that reason before its arguments are read. ComplexFunctions
// `ToString`/`ToComplex` declare no body and no Complex string notation.
// NaturalFunctions `'/'` returns `Natural[1]`, so a quotient that is no Natural
// is a domain error while an exact one evaluates. OccurrenceFunctions `create`
// constrains the occurrence's start to happen during the call, so a fresh
// occurrence is created and returned.
package test {
	private import ScalarValues::*;

	part def P1 { part pa; }
	part p1 : P1;
	enum def E { a; b; }
	occurrence def O;

	calc probe { out one : Integer = 1; }

	attribute bracketOperator = 2.0 [SI::m];
	attribute bracketCall = BaseFunctions::'['(2.0, SI::m);
	attribute istypeOperator : Boolean = 1 istype Integer;
	attribute istypeCall = BaseFunctions::'istype'(1, Integer);
	attribute hastypeOperator : Boolean = 1 hastype Integer;
	attribute hastypeCall = BaseFunctions::'hastype'(1, Integer);
	attribute atOperator : Boolean = 1 @ Integer;
	attribute atCall = BaseFunctions::'@'(1, Integer);
	attribute asOperator = 1 as Real;
	attribute asCall = BaseFunctions::'as'(1);
	attribute allOperator : Natural = SequenceFunctions::size(all E);
	attribute allCall = SequenceFunctions::size(BaseFunctions::'all'(E));
	attribute metaCall = BaseFunctions::'meta'(P1);
	attribute complexToString = ComplexFunctions::ToString(ComplexFunctions::rect(1.0, 2.0));
	attribute complexFromString = ComplexFunctions::ToComplex("1.0 + 2.0i");
	attribute naturalQuotientExact : Natural = NaturalFunctions::'/'(6, 2);
	attribute naturalQuotientInexact = NaturalFunctions::'/'(7, 2);
	attribute createdFresh = OccurrenceFunctions::create(new O());
}

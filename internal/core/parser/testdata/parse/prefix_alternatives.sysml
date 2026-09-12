// Each prefix alternative alone (SysML.xtext BasicDefinitionPrefix, RefPrefix,
// FeatureDirection) and a feature or arithmetic multiplicity bound, an extension of
// KerML.xtext MultiplicityExpressionMember that reads a leading '-' as a syntax error.
package PrefixAlternatives {
	abstract part def A;
	variation part def V { variant part a : A; }
	variation part v : V;
	part def R { ref part r : A; }
	port def Q { in item i; out item o; inout item io; }
	attribute n : ScalarValues::Integer;
	part few [n] : A;
	part more [0..n+1] : A;
}

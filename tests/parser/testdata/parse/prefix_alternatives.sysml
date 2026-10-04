// Each prefix alternative alone (SysML.xtext BasicDefinitionPrefix, RefPrefix,
// FeatureDirection), plus feature-reference multiplicity bounds.
package PrefixAlternatives {
	abstract part def A;
	variation part def V { variant part a : A; }
	variation part v : V;
	part def R { ref part r : A; }
	port def Q { in item i; out item o; inout item io; }
	attribute n : ScalarValues::Integer;
	part few [n] : A;
	part more [0..n] : A;
}

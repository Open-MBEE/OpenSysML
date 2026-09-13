// Each KerML feature prefix alternative alone: 'composite' or 'portion', one
// direction, and the KerML spelling 'const' (KerML.xtext BasicFeaturePrefix).
package PrefixAlternatives {
	class A {
		composite feature c;
		portion feature p;
		in feature i;
		inout feature io;
		const feature k;
	}
}

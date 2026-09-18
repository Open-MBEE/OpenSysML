// A leading multiplicity on an end-only binding or succession is the first
// end's crossing multiplicity (KerML.xtext:854 ConnectorEnd); with `of` or
// `first` it is the connector's own (KerML.xtext:875, :891 alternative 1).
package ConnectorEndMultiplicity {
	class A { feature a; feature b; }
	behavior B { step s1; step s2; }
	class Forms {
		feature x : A;
		feature y : A;
		feature p : B;

		binding [1] x.a = [1] x.b;
		binding [0..1] x.a = y.b;
		binding x.a = [*] y.b;
		binding [1] of x.a = x.b;
		binding bb [1] of x.a = x.b;

		succession [1] p.s1 then [*] p.s2;
		succession [1] p.s1 then p.s2;
		succession p.s1 then [0..1] p.s2;
		succession [1] first p.s1 then p.s2;
		succession sn [1] first p.s1 then p.s2;

		// A first end may be a global name or a named end (KerML.xtext:854).
		succession [1] $::ConnectorEndMultiplicity::Forms::p.s1 then p.s2;
		succession [1] e1 ::> p.s1 then p.s2;
		succession [0..1] e2 references p.s1 then [*] e3 ::> p.s2;
		succession e4 ::> p.s1 then p.s2;
	}
}

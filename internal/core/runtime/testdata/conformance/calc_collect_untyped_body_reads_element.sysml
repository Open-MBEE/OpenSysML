// A body's parameter is bound to each element of the collection the body is
// applied to (KerML 1.1 §8.3.4.8), so a parameter declaring no type reads the
// element's features exactly as one declared of the element type does, in the
// receiver, plain and named-argument notations, in a reducer's two parameters
// and in a body nested in another's.
package test {
	private import ScalarValues::*;
	private import ControlFunctions::*;

	part def Component { attribute mass : Real; }
	part def Assembly { part parts : Component[*]; }

	part light : Component { attribute :>> mass = 1.5; }
	part medium : Component { attribute :>> mass = 2.5; }
	part heavy : Component { attribute :>> mass = 4.0; }
	part front : Assembly { part :>> parts = (light, heavy); }
	part rear : Assembly { part :>> parts = (medium); }

	calc def Masses {
		attribute cs : Component[*] = (light, medium, heavy);
		attribute assemblies : Assembly[*] = (front, rear);

		out typedMasses : Real[*] = cs.{in c : Component; c.mass};
		out masses : Real[*] = cs.{in c; c.mass};
		out collected : Real[*] = cs->collect {in c; c.mass};
		out plain : Real[*] = collect(cs, {in c; c.mass});
		out named : Real[*] = collect(mapper = {in c; c.mass}, collection = cs);
		out heavyMasses : Real[*] = cs.?{in c; c.mass > 2.0}.{in c; c.mass};
		out lightMasses : Real[*] = (cs->reject {in c; c.mass > 2.0}).{in c; c.mass};
		out heaviestMass : Real[*] = (cs->selectOne {in c; c.mass > 3.0}).{in c; c.mass};
		out allWeigh : Boolean = cs->forAll {in c; c.mass > 1.0};
		out anyHeavy : Boolean = cs->exists {in c; c.mass > 3.0};
		out total : Real = masses->reduce {in acc; in m; acc + m};
		out heaviest : Real[*] = (cs->reduce {in a; in b; if a.mass > b.mass ? a else b}).{in c; c.mass};
		out lightest : Real = masses->minimize {in m; m};
		out heaviestOfAll : Real = masses->maximize {in m; m};
		out nested : Real[*] = assemblies.{in a; a.parts.{in p; p.mass}};
	}

	calc m : Masses;
}

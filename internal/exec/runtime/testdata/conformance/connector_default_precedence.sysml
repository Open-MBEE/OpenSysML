// A `default =` in a connector end's body yields to a value an outer binding states for the
// connected port: a default applies only "if no other explicit values are given for the
// feature" (KerML 1.1 §7.4.11, §8.3.4.10.2), so `bind inner.k.dst.voltage = supply.src.voltage`
// in the owner gives the port 28 whether the inner default evaluates or fails.
package test {
	private import ScalarValues::*;

	port def Pw { attribute voltage : Integer; }
	part def Src { port src : Pw { attribute :>> voltage = 28; } }
	part def Snk { port dst : ~Pw; }
	interface def EndDefaultFails { end a : Pw; end b : ~Pw { attribute zero : Integer = 0; attribute :>> voltage default = 1 / zero; } }
	interface def EndDefaultOk { end a : Pw; end b : ~Pw { attribute :>> voltage default = 12; } }
	part def Inner { port inp : Pw; part k : Snk; interface i : EndDefaultFails connect inp to k.dst; }
	part def InnerOk { port inp : Pw; part k : Snk; interface i : EndDefaultOk connect inp to k.dst; }
	part def Outer {
		part supply : Src;
		part inner : Inner;
		part innerOk : InnerOk;
		bind inner.k.dst.voltage = supply.src.voltage;
		bind innerOk.k.dst.voltage = supply.src.voltage;
		assert constraint outerOverridesFailingDefault { inner.k.dst.voltage == 28 }
		assert constraint outerOverridesDefault { innerOk.k.dst.voltage == 28 }
	}
	part outer : Outer;
}

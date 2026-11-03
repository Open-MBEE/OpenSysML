// A binding connector an interface or connection owns — in the definition's
// body, the usage's body, or a supertype of the definition — binds the features
// its ends attach: `bind a.voltage = b.voltage` inside `interface def L { end a :
// Pw; end b : ~Pw; }` makes `k.dst.voltage` hold `s.src.voltage` once `interface
// i : L connect s.src to k.dst` relates them, exactly as `bind s.src.voltage =
// k.dst.voltage` written by the containing part does (KerML 1.1 §8.4.4.6.2, the
// related features of a binding connector have the same values; §8.4.4.6, a
// connector's related features are features of its featuring type; SysML v2
// §7.13, an interface or connection end is the connected usage). The value flows
// either way across the binding — from the conjugated side too — and through a
// chain of two interfaces.
package test {
	private import ScalarValues::*;

	port def Pw { attribute voltage : Integer; }

	interface def L { end a : Pw; end b : ~Pw; bind a.voltage = b.voltage; }
	interface def Plain { end a : Pw; end b : ~Pw; }
	interface def Derived :> L;
	connection def Wire { end a : Pw; end b : ~Pw; bind a.voltage = b.voltage; }
	interface def RenamedEnds :> L { end input :>> a; end output :>> b; }
	interface def PositionalEnds :> L { end input : Pw; end output : ~Pw; }

	part def Src { port src : Pw { attribute :>> voltage = 28; } }
	part def Snk { port dst : ~Pw; }
	part def Bare { port src : Pw; }
	part def Relay { port inlet : ~Pw; port outlet : Pw; bind inlet.voltage = outlet.voltage; }

	part def DirectBinding {
		part s : Src;
		part k : Snk;
		interface i : Plain connect s.src to k.dst;
		bind s.src.voltage = k.dst.voltage;
	}
	part def DefinitionBinding {
		part s : Src;
		part k : Snk;
		interface i : L connect s.src to k.dst;
	}
	part def UsageBodyBinding {
		part s : Src;
		part k : Snk;
		interface i : Plain connect s.src to k.dst { bind a.voltage = b.voltage; }
	}
	part def ConnectionDefinitionBinding {
		part s : Src;
		part k : Snk;
		connection w : Wire connect s.src to k.dst;
	}
	part def InheritedBinding {
		part s : Src;
		part k : Snk;
		interface i : Derived connect s.src to k.dst;
	}
	part def RenamedEndsBinding {
		part s : Src;
		part k : Snk;
		interface i : RenamedEnds connect s.src to k.dst;
	}
	part def PositionalEndsBinding {
		part s : Src;
		part k : Snk;
		interface i : PositionalEnds connect s.src to k.dst;
	}
	part def ConjugatedSideBinding {
		part s : Bare;
		part k : Snk { port :>> dst { attribute :>> voltage = 12; } }
		interface i : L connect s.src to k.dst;
	}
	part def ChainedBindings {
		part s : Src;
		part r : Relay;
		part k : Snk;
		interface upstream : L connect s.src to r.inlet;
		interface downstream : L connect r.outlet to k.dst;
	}

	part system {
		part direct : DirectBinding;
		part definition : DefinitionBinding;
		part renamed : RenamedEndsBinding;
		part positional : PositionalEndsBinding;
		part usageBody : UsageBodyBinding;
		part connectionDefinition : ConnectionDefinitionBinding;
		part inherited : InheritedBinding;
		part conjugatedSide : ConjugatedSideBinding;
		part chained : ChainedBindings;

		assert constraint directBindingHolds { direct.k.dst.voltage == 28 }
		assert constraint definitionBindingHolds { definition.k.dst.voltage == 28 }
		assert constraint usageBodyBindingHolds { usageBody.k.dst.voltage == 28 }
		assert constraint connectionDefinitionBindingHolds { connectionDefinition.k.dst.voltage == 28 }
		assert constraint inheritedBindingHolds { inherited.k.dst.voltage == 28 }
		assert constraint renamedEndsBindingHolds { renamed.k.dst.voltage == 28 }
		assert constraint positionalEndsBindingHolds { positional.k.dst.voltage == 28 }
		assert constraint conjugatedSideBindingHolds { conjugatedSide.s.src.voltage == 12 }
		assert constraint chainedBindingsHold { chained.k.dst.voltage == 28 and chained.r.inlet.voltage == 28 and chained.r.outlet.voltage == 28 }
	}
}

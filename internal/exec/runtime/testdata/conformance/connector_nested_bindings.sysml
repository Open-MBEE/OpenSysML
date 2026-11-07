// A connector's bindings reach the connected features through the connector's
// own features, through connectors nested in a connection definition, and
// through the feature values its end bodies state. `bind a.voltage = m.inV;
// bind m.outV = b.voltage` inside an interface definition routes the value
// through the part `m` the connector holds (KerML 1.1 §8.4.4.6.2, the related
// features of each binding have the same values, so transitively the ends
// agree), an anonymous interface's feature read on the connector object its
// declaration materializes, an index (`readings#(index)`) read there too; an
// interface a connection definition owns over its own ends
// (`interface w : W connect a.p to b.q`) is re-rooted again when the connection
// attaches (KerML 1.1 §8.4.4.6.1, an end references the connected feature, so a
// path through an end is a path through that feature; SysML v2 §7.15.1, an
// allocation definition is a connection definition); and `attribute :>> voltage
// = 12` in an end's body is an implicit binding of the connected port's voltage
// (SysML v2 §7.13.4), a `default =` one yielding to a value the port has of its own.
package test {
	private import ScalarValues::*;

	port def Pw { attribute voltage : Integer; }
	part def Mid { attribute inV : Integer; attribute outV : Integer = inV; }

	interface def ThroughPart { end a : Pw; end b : ~Pw; part m : Mid; bind a.voltage = m.inV; bind m.outV = b.voltage; }
	interface def ThroughShared { end a : Pw; end b : ~Pw; attribute shared : Integer; bind a.voltage = shared; bind shared = b.voltage; }
	interface def ThroughChain { end a : Pw; end b : ~Pw; attribute s1 : Integer; attribute s2 : Integer; bind a.voltage = s1; bind s1 = s2; bind s2 = b.voltage; }
	interface def EndValue { end a : Pw; end b : ~Pw { attribute :>> voltage = 12; } }
	interface def EndDefault { end a : Pw; end b : ~Pw { attribute :>> voltage default = 12; } }
	interface def Plain { end a : Pw; end b : ~Pw; }
	interface def Picks { end a : Pw; end b : ~Pw; attribute readings : Integer[2] = (5, 28); attribute index : Integer = 2; bind b.voltage = readings#(index); }

	part def Src { port src : Pw { attribute :>> voltage = 28; } }
	part def Snk { port dst : ~Pw; }
	part def Valued { port dst : ~Pw { attribute :>> voltage = 5; } }

	part def A { port p : Pw { attribute :>> voltage = 28; } }
	part def B { port q : ~Pw; }
	connection def Wired { end a : A; end b : B; interface w : ThroughShared connect a.p to b.q; }
	part def Rack { part s : A; part k : B; }
	allocation def Racked { end src : Rack; end dst : Rack; connection c : Wired connect src.s to dst.k; }

	part def ThroughPartBinding {
		part s : Src;
		part k : Snk;
		interface i : ThroughPart connect s.src to k.dst;
		assert constraint throughPartBindingHolds { k.dst.voltage == 28 and i.m.inV == 28 }
	}
	part def ThroughSharedBinding {
		part k : Snk;
		part s : Src;
		interface i : ThroughShared connect s.src to k.dst;
		assert constraint throughSharedBindingHolds { k.dst.voltage == 28 and i.shared == 28 }
	}
	part def AnonymousSharedBinding {
		part s : Src;
		part k : Snk;
		interface : ThroughShared connect s.src to k.dst;
		assert constraint anonymousSharedHolds { k.dst.voltage == 28 }
	}
	part def AnonymousIndexedBinding {
		part s : Src;
		part k : Snk;
		attribute index : Integer = 1;
		interface : Picks connect s.src to k.dst;
		assert constraint anonymousIndexHolds { k.dst.voltage == 28 }
	}
	part def ThroughChainBinding {
		part s : Src;
		part k : Snk;
		interface i : ThroughChain connect s.src to k.dst;
		assert constraint throughChainBindingHolds { k.dst.voltage == 28 and i.s2 == 28 }
	}
	part def NestedConnector {
		part s : A;
		part k : B;
		connection c : Wired connect s to k;
		assert constraint nestedConnectorBindingHolds { k.q.voltage == 28 and c.w.shared == 28 }
	}
	part def TwoLevelNesting {
		part r1 : Rack;
		part r2 : Rack;
		allocation d : Racked allocate r1 to r2;
		assert constraint twoLevelNestingHolds { r2.k.q.voltage == 28 and d.c.w.shared == 28 }
	}
	part def EndBodyValue {
		part s : Src;
		part k : Snk;
		interface i : EndValue connect s.src to k.dst;
		assert constraint endBodyValueHolds { k.dst.voltage == 12 }
	}
	part def EndBodyDefaultApplies {
		part s : Src;
		part k : Snk;
		interface i : EndDefault connect s.src to k.dst;
		assert constraint endBodyDefaultApplies { k.dst.voltage == 12 }
	}
	part def EndBodyDefaultYields {
		part s : Src;
		part k : Valued;
		interface i : EndDefault connect s.src to k.dst;
		assert constraint endBodyDefaultYields { k.dst.voltage == 5 }
	}
	part def UsageEndBodyValue {
		part s : Src;
		part k : Snk;
		interface i : Plain { end :>> a ::> s.src; end :>> b ::> k.dst { attribute :>> voltage = 12; } }
		assert constraint usageEndBodyValueHolds { k.dst.voltage == 12 }
	}

	part def System {
		part throughPart : ThroughPartBinding;
		part throughShared : ThroughSharedBinding;
		part anonymousShared : AnonymousSharedBinding;
		part anonymousIndexed : AnonymousIndexedBinding;
		part throughChain : ThroughChainBinding;
		part nested : NestedConnector;
		part twoLevel : TwoLevelNesting;
		part endValue : EndBodyValue;
		part endDefaultApplies : EndBodyDefaultApplies;
		part endDefaultYields : EndBodyDefaultYields;
		part usageEndValue : UsageEndBodyValue;
		assert constraint throughPartBindingHolds { throughPart.k.dst.voltage == 28 }
		assert constraint throughSharedBindingHolds { throughShared.k.dst.voltage == 28 }
		assert constraint anonymousSharedHolds { anonymousShared.k.dst.voltage == 28 }
		assert constraint anonymousIndexHolds { anonymousIndexed.k.dst.voltage == 28 }
		assert constraint throughChainBindingHolds { throughChain.k.dst.voltage == 28 }
		assert constraint nestedConnectorBindingHolds { nested.k.q.voltage == 28 }
		assert constraint twoLevelNestingHolds { twoLevel.r2.k.q.voltage == 28 }
		assert constraint endBodyValueHolds { endValue.k.dst.voltage == 12 }
		assert constraint endBodyDefaultApplies { endDefaultApplies.k.dst.voltage == 12 }
		assert constraint endBodyDefaultYields { endDefaultYields.k.dst.voltage == 5 }
		assert constraint usageEndBodyValueHolds { usageEndValue.k.dst.voltage == 12 }
	}
	part system : System;
}

// A connection usage carries a body after its ends (SysML.xtext ConnectionUsage),
// and its ends may be feature chains of any depth.
package Interconnection {
	port def P;
	part def A {
		port p : P;
	}
	part def Assembly {
		part inner : A;
	}
	part def System {
		ref r : A;
		part x : A;
		part assembly : Assembly;

		connect x.p to r.p {
			doc /* the ports the connection joins */
		}
		connect x.p to assembly.inner.p;
		connect [1] x.p to [1] r.p;
		connection named connect x.p to r.p {
			doc /* a named connection states its ends after `connect` */
		}
		connect (x.p, r.p, assembly.inner.p) {
			doc /* an n-ary connection */
		}
	}
}

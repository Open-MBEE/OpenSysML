// An implicitly constant end (SysML 2.0 §8.4.2.2) is refused only to a
// behavior's write: binding one at creation is how it gets its value, so the
// ends of a typed connection still attach to the parts they connect — a
// library connection or allocation, or a user connection definition's ends.
package test {
	part def Thing;

	connection def Link {
		end part src : Thing;
		end part dst : Thing;
	}

	part def Sys {
		part a : Thing;
		part b : Thing;
		connection link : Connections::BinaryConnection connect a to b;
		allocation al : Allocations::Allocation allocate a to b;
		connection l2 : Link connect src ::> a to dst ::> b;
	}

	part sys : Sys;
}

// This document opens a Meta::Safety of its own, a distinct Type with the same
// qualified name; a filter naming it does not select what the other annotates.
package Meta { metadata def Safety; }
package ClientB {
	private import LibA::*[@Meta::Safety];
	part a :> belt;
}

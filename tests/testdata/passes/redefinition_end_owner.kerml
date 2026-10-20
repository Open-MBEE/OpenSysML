// A non-end feature may redefine an inherited end feature unless its owning type
// is an Association or a Connector (KerML 1.1 §8.3.3.3.8
// validateRedefinitionEndConformance). Here the owner is a feature typed by an
// association structure, so `end` is not required; the pinned validate-kerml is
// silent.
package RedefinitionEndOwner {
	class T;
	assoc struct AS { end feature e1 : T; end feature e2 : T; }
	feature k : AS { feature zz :>> e1; }
}

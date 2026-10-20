// A non-end usage may redefine an inherited end feature unless its owning type
// is an Association or a Connector (KerML 1.1 §8.3.3.3.8
// validateRedefinitionEndConformance). Here the owners are a part definition and
// a part usage typed by a connection definition, so `end` is not required; the
// pinned validate-sysml is silent on both. `constant` keeps the redefinitions
// constant-conformant, as the ends are.
package RedefinitionEndOwner {
	part def A;
	connection def Conn { end one : A; end two : A; }
	part k : Conn { constant ref kk :>> one; }
	part def D { end part e : A; }
	part def E :> D { constant part :>> e; }
}

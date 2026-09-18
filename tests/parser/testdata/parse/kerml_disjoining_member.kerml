// A Disjoining written keyword-first (KerML.xtext:426) is a member of its own
// whose two ends are ordered: the disjoined type, then the disjoining type.
package DisjoiningMember {
	classifier A;
	classifier B;
	class Occ {
		feature successors : Occ;
		feature predecessors : Occ;
	}
	feature earlier : Occ;
	feature later : Occ;

	// The bare form, and the same with its own identification.
	disjoint A from B;
	disjoining D disjoint A from B;
	disjoining <d2> disjoint B from A;

	// A relationship body and a visibility prefix.
	private disjoining Documented disjoint A from B {
		doc /* A and B share no instance. */
	}

	// Feature-chain ends, as the Kernel Semantic Library writes them.
	disjoint earlier.successors from later.predecessors;

	// A global name and a word KerML does not reserve begin an end too.
	feature part : Occ;
	disjoint $::DisjoiningMember::A from B;
	disjoining part disjoint part from B;
	disjoining E disjoint $::DisjoiningMember::part.successors from later;

	// The declaration clause stays a clause of the declaration it follows.
	class C specializes A disjoint from B;
}

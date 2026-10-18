// One rule decides the count of a `[0..*]` or `[0..1]` collection nothing values,
// whether the Systems Library or the Kernel Semantic Library declares it: read at
// model level, on a package-level usage, the multiplicity fixes no count and the
// read is undetermined (KerML §7.3.4.1, §7.4.12); read on an instantiated object
// the collection holds what was recorded, so it is empty, or null for `[0..1]`.
// An abstract collection is exactly its subsetters' values on both tiers.
package test {
	part def P1 {
		part pa; part pb; item ia;
	}
	part p1 : P1;

	calc probe { out one : ScalarValues::Integer = 1; }

	attribute voidsModel = p1.voids;
	attribute incomingModel = p1.incomingTransfers;
	attribute predecessorsModel = p1.predecessors;
	attribute suboccurrencesModel = p1.suboccurrences;
	attribute middleModel = p1.middleTimeSlice;
	attribute subpartsModel = p1.subparts;
	attribute nestedIncomingModel = p1.pa.incomingTransfers;
	attribute voidsObject = new P1().voids;
	attribute incomingObject = new P1().incomingTransfers;
	attribute predecessorsObject = new P1().predecessors;
	attribute suboccurrencesObject = new P1().suboccurrences;
	attribute middleObject = new P1().middleTimeSlice;
	attribute subpartsObject = new P1().subparts;
}

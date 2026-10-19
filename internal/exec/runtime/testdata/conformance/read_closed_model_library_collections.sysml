// A library collection a package-level usage values with nothing holds what the
// model declares, read at model level exactly as on an instantiated object
// (KerML 1.0 §7.3.4.4: a subsetting feature's values are the subsetted feature's;
// §7.3.4: a lower bound of 0 is met by no value). The Systems and Kernel tiers
// answer alike, abstract (`performedActions`, `subparts`) or not (`voids`,
// `suboccurrences`, `predecessors`, `middleTimeSlice`). A usage the model itself
// declares with an open multiplicity keeps the model-level rule: no definite count.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::size;

	part def P1 {
		part pa;
		part pb;
		attribute v : Integer = size(self.suboccurrences);
	}
	part p1 : P1;

	part def R {
		part loose : Part[0..2];
		part gear : Part[1..*];
	}
	part rack : R;

	part def H { item si : SpatialItems::SpatialItem; }
	part h : H;

	part def P3 {
		action def A;
		perform action act : A;
	}
	part p3 : P3;

	calc probe { out one : Integer = 1; }

	attribute voidsModel = p1.voids;
	attribute isSolidModel : Boolean = p1.isSolid;
	attribute subOccurrencesModel = p1.suboccurrences;
	attribute vModel : Integer = p1.v;
	attribute subpartsModel = p1.subparts;
	attribute predecessorsModel = p1.predecessors;
	attribute middleTimeSliceModel = p1.middleTimeSlice;
	attribute performedActionsModel = p3.performedActions;
	attribute looseModel = rack.loose;
	attribute gearModel = rack.gear;
	attribute subSpatialItemsModel = h.si.subSpatialItems;

	attribute voidsObject = new P1().voids;
	attribute subOccurrencesObject = new P1().suboccurrences;
	attribute vObject : Integer = new P1().v;
	attribute performedActionsObject = new P3().performedActions;
}

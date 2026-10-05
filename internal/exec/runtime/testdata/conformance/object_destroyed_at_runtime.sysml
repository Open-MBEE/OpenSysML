// Destroying an object created at runtime ends its life wherever it is referred to:
// `destroy` through one reference ends the object the group still holds, it leaves
// the extent of its definition while the group keeps the stale reference, its
// exhibited state machine ends with it, nothing happens during it any more, and a
// feature read through the stale reference is refused as a read of a destroyed occurrence.
package test {
	private import ScalarValues::*;
	private import OccurrenceFunctions::*;
	private import SequenceFunctions::*;

	part def Car {
		attribute n : Integer;
		exhibit state running { entry; then idle; state idle; }
	}

	part def Fleet {
		part cars : Car[0..*];
		ref part spare : Car[0..1];
		attribute extentBefore : Integer;
		attribute extentAfter : Integer;
		attribute heldAfter : Integer;
		attribute spareAlive : Boolean;
		attribute firstAlive : Boolean;
		attribute sameObject : Boolean;
		attribute readAfter : Integer = cars#(2).n;

		perform action cycle {
			first start;
			then action make assign cars := addNew(cars, new Car(1));
			then assign cars := addNew(cars, new Car(2));
			then assign spare := cars#(2);
			then assign extentBefore := size(all Car);
			then action scrap assign sameObject := destroy(spare) === cars#(2);
			then assign extentAfter := size(all Car);
			then assign heldAfter := size(cars);
			then assign spareAlive := isDuring(spare);
			then assign firstAlive := isDuring(cars#(1));
			then done;
		}
	}
}

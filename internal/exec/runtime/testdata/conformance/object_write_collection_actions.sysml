package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	part def Box {
		attribute values : Integer[0..*] ordered nonunique;
	}

	part def Fleet {
		part box : Box;

		perform action collectionWrites {
			first start;
			then action supply {
				out result : Box = box;
			}
			then action write {
				in ref object : Box;
				assign object.values := SequenceFunctions::including(object.values, 1);
				assign object.values := SequenceFunctions::including(object.values, 3);
				assign object.values := SequenceFunctions::including(object.values, 4);
				assign object.values := SequenceFunctions::includingAt(object.values, 2, 2);
				assign object.values := SequenceFunctions::excluding(object.values, 1);
				assign object.values := SequenceFunctions::excludingAt(object.values, 2, 2);
			}
			flow supply.result to write.object;
			then done;
		}
	}
}

// The forms a migrated variable and its actions write: a private attribute,
// assigns over SequenceFunctions, a clear, and a read binding its result pin.
package test {
	private import ScalarValues::*;

	action def MigratedVariables {
		private attribute v : Integer[0..*];

		first start;

		action add {
			in value : Integer = 7;
			assign v := SequenceFunctions::including(v, value);
		}
		action addAt {
			in value : Integer = 9;
			in insertAt : Integer = 2;
			assign v := SequenceFunctions::includingAt(v, value, insertAt);
		}
		action remove {
			in value : Integer = 9;
			assign v := SequenceFunctions::excluding(v, value);
		}
		action removeAt {
			in removeAt : Integer = 1;
			assign v := SequenceFunctions::excludingAt(v, removeAt);
		}
		action clear {
			assign v := null;
		}
		action add2 {
			in value : Integer = 5;
			assign v := SequenceFunctions::including(v, value);
		}
		action read {
			out result : Integer[0..*] = v;
		}
		action final terminate;

		first start then add;
		first add then addAt;
		first addAt then remove;
		first remove then removeAt;
		first removeAt then clear;
		first clear then add2;
		first add2 then read;
		first read then final;
	}
}

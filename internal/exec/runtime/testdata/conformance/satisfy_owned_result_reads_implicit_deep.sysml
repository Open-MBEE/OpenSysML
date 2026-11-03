// A satisfy usage declared without `by` in a part nested several parts deep takes that
// part as its subject (SysML v2 §7.21.4): the subject read and the `result` read answer
// about the same object.
package test {
	private import ScalarValues::*;

	part def Lander {
		attribute verticalSpeed : Real;
	}

	requirement def Touchdown {
		subject lander : Lander;
		require constraint { lander.verticalSpeed <= 1.5 }
	}

	part def Fleet {
		part wing {
			part craft : Lander {
				attribute :>> verticalSpeed = 0.7;
				satisfy requirement touchdown : Touchdown;
			}
		}
		attribute verdict : Boolean = wing.craft.touchdown.result;
		attribute subjectSpeed : Real = wing.craft.touchdown.lander.verticalSpeed;
	}
}

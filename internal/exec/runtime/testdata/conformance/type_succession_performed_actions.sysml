package test {
	part def Robot {
		attribute n : ScalarValues::Integer := 0;

		perform action move {
			first start;
			then action move1 {
				assign n := n + 1;
			}
			then done;
		}

		perform action grip {
			first start;
			then action grip1 {
				assign n := n * 10;
			}
			then done;
		}

		first grip then move;
	}

	part r1 : Robot;
	part r2 : Robot;
	part def Holder {
		part r1 : Robot;
		part r2 : Robot;
	}
}

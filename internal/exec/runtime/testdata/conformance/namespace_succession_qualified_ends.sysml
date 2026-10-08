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
	}

	part r : Robot;
	part s : Robot;
	first r::grip then r::move;
	part def Holder {
		part r : Robot;
		part s : Robot;
	}
}

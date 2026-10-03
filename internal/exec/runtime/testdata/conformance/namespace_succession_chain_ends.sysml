package test {
	part def Bot {
		attribute n : ScalarValues::Integer := 0;

		perform action m {
			first start;
			then action m1 {
				assign n := n + 1;
			}
			then done;
		}

		perform action g {
			first start;
			then action g1 {
				assign n := n * 10;
			}
			then done;
		}
	}

	part b : Bot;
	first b.g then b.m;
}

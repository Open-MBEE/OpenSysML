package Sequenced {
	part part1 {
		attribute n : ScalarValues::Integer := 0;
		perform action action1 {
			first start;
			then action increment {
				assign n := n + 1;
			}
			then done;
		}
	}

	requirement requirement1;
	first part1::action1 then requirement1;
}

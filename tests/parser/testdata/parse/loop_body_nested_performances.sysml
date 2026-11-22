package LoopBodyNestedPerformances {
	action def Run {
		attribute n : Integer = 0;
		attribute t : Integer = 0;
		first start;
		then loop action body {
			first start;
			then action inc assign n := n + 1;
			then done;
		} until n >= 3;
		then while n < 5 {
			assign n := n + 1;
			for i in (1, 2) {
				action add assign t := t + i;
			}
		}
		then loop action {
			action bump assign n := n + 1;
		} until n >= 7;
		then done;
	}
	analysis def Counted {
		subject s : Integer = 0;
		attribute n : Integer = 0;
		loop action body {
			action inc assign n := n + 1;
		} until n >= 3;
		return r : Integer = n;
	}
}

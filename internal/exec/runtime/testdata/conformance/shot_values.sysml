package test {
	private import ScalarValues::*;
	part def Counter {
		perform action count {
			attribute n : Integer = 0;
			first start;
			then action bump assign n := 7;
			then done;
		}
		attribute atEnd : Integer = count.done.n;
		attribute atStart : Integer = count.start.n;
		attribute ran : Integer = count.n;
	}
}

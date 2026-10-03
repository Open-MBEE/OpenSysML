package test {
	private import ScalarValues::*;

	action blocks {
		attribute total : Integer = 0;
		first start;
		action driver {
			loop {
				accept repeated : Integer;
			} until total > 0;

			if total == 0 {
				accept thenValue : Integer;
			} else {
				accept elseValue : Integer;
			}

			loop {
				action step {
					accept nested : Integer;
					assign total := total + nested;
				}
			} until total > 1;
		}
		done;
		succession first start then driver;
		succession first driver then done;
	}
}

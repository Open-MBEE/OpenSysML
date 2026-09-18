package F63 {
	action def DecisionTest {
		attribute x = 1;
		attribute total = 0;

		decide 'test x';
		if x == 1 then A1;
		if x > 1 then A2;
		else A3;

		action A1;
		action A2;
		action A3;

		then fork F {
			attribute local = 1;
		}
		then join J { assign total := total + 1; }
		then merge M { assign total := total + 1; }

		action accumulate {
			for n : ScalarValues::Integer in (1, 2, 3) {
				assign total := total + n;
			}
		}
	}
}

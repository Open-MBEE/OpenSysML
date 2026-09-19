package test {
	// A perform statement runs the action it reference-subsets (SysML 7.17.6),
	// exchanging values with it through parameters: `result` is 12 only if
	// performing `doIt` executed the body of `increment`.
	action outer {
		attribute base : Integer = 7;
		attribute result : Integer = 0;

		action increment {
			in base : Integer;
			out result : Integer;

			first begin;

			action bump {
				assign result := base + 5;
			}

			done;

			succession first begin then bump;
			succession first bump then done;
		}

		first start;

		perform action doIt references increment;

		done;

		succession first start then doIt;
		succession first doIt then done;
	}
}

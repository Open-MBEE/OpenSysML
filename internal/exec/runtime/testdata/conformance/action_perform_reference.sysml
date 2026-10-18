package test {
	// A perform statement runs the action it reference-subsets (SysML 7.17.6) as a
	// performance of its own: `increment`'s `in base`, which nothing binds, reads as
	// unset inside its body — never as the performer's same-named `base` — so the
	// body's `base + 5` fails with the operator's type error.
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

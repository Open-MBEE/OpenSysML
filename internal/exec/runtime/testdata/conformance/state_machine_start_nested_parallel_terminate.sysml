package test {
	private import ScalarValues::*;

	state Machine {
		attribute marker : Integer = 0;

		entry; then nested;
		state work parallel {
			state terminating {
				entry action boot { }
				transition boot then route;
				junction route;
				transition first route then stop;
				action stop terminate;
			}
			state left {
				entry; then nested;
				state nested {
					entry { assign marker := 1; }
				}
			}
		}
	}
}

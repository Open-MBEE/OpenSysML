// A value written with `=` over other features is derived again once one of them
// is assigned: read before the assignment in an entry action, it answers from the
// old value; read after it, from the new one — transitively, through a derived
// value the assigned feature reaches by way of another.
package test {
	private import ScalarValues::*;

	part def Box {
		attribute a : Integer default 3;
		attribute d : Integer = a * 2;
		attribute dd : Integer = d + 1;
		attribute before : Integer = 0;
		attribute after : Integer = 0;

		exhibit state run {
			entry; then assigning;

			state assigning {
				entry action set {
					assign before := dd;
					assign a := 9;
					assign after := dd;
				}
			}
		}
	}
}

// A `perform action` usage naming nothing — no reference subsetting, no typing
// — performs itself (SysML v2 §8.3.16, eventOccurrence), and its `in` member
// binds the performance's parameter from the object's attribute.
package test {
	part def Host {
		attribute level : Integer = 3;

		perform action boost {
			in amount : Integer = level;
		}
	}
}

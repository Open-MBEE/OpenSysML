// The operations inside a calc body's `while`: the loop walks the sequence by
// index, and its condition is a collection operation of its own.
package test {
	private import SequenceFunctions::*;
	calc runningTotal {
		attribute xs : Integer[*] = (5, 7, 9);
		attribute i : Integer = 1;
		attribute total : Integer = 0;
		while i <= xs->size() {
			assign total := total + xs#(i);
			assign i := i + 1;
		}
		return : Integer = total + xs.?{in x; x > 5}->size();
	}
}

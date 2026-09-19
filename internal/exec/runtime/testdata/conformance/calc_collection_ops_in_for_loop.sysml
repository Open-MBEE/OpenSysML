// The operations work where models use them: inside a calc body's `for`, over
// the sequence a collect computed, indexing it per iteration.
package test {
	calc weightedTotal {
		attribute xs : Integer[*] = (1, 2, 3);
		attribute weights : Integer[*] = xs.{in x; x * 10};
		attribute total : Integer = 0;
		for i in (1, 2, 3) {
			assign total := total + weights#(i) + xs#(i);
		}
		return : Integer = total;
	}
}

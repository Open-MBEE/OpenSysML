// The index counts from 1, so 0 names no position: `Positive[1]` excludes it.
package test {
	calc pick {
		attribute xs : Integer[*] = (10, 20, 30);
		return : Integer = xs#(0);
	}
}

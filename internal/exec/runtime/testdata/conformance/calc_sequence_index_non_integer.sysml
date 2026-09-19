// An index that is not a whole number is reported rather than truncated: a
// Real names no position of a sequence.
package test {
	calc pick {
		attribute xs : Integer[*] = (10, 20, 30);
		return : Integer = xs#(1.5);
	}
}

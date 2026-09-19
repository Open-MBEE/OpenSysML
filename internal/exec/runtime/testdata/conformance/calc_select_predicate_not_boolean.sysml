// A selector the library declares `Boolean[1]` that answers something else is
// reported: reading a non-Boolean as false would silently drop every element
// and answer a filter the model never wrote.
package test {
	calc badFilter {
		attribute xs : Integer[*] = (1, 2, 3);
		return : Integer = SequenceFunctions::size(xs.?{in x; x * 2});
	}
}

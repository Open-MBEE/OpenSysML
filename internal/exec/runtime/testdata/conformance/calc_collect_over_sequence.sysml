// `collect` maps a body over a collection, binding each element to the
// parameter the body declares — whatever that parameter is called. The mapped
// sequence is a value like any other, so it is indexed and aggregated here.
package test {
	calc scaled {
		in factor : Integer;
		attribute xs : Integer[*] = (1, 2, 3);
		attribute doubled : Integer[*] = xs.{in element; element * factor};
		return : Integer = doubled#(1) * 100 + doubled#(2) * 10 + doubled#(3);
	}
}

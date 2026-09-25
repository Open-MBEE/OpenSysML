// A body called with one element must declare one parameter: a body declaring
// two would leave a parameter unbound, which is reported rather than evaluated
// with a missing value.
package test {
	calc badArity {
		attribute xs : Integer[*] = (1, 2, 3);
		return : Integer = SequenceFunctions::size(ControlFunctions::collect(xs, {in x; in y; x + y}));
	}
}

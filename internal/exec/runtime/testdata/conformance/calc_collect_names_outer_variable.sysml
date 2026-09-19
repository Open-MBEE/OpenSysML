// A body sees the names of the scope it was written in as well as its own
// parameter, so a mapper may close over a feature of the calc — the outer
// `offset` here — and a nested body's parameter shadows the enclosing one only
// for its own result.
package test {
	calc offsetProducts {
		in offset : Integer;
		attribute xs : Integer[*] = (1, 2);
		attribute ys : Integer[*] = (10, 20);
		attribute sums : Integer[*] = xs.{in x; (ys.{in y; y + offset}#(1)) + x};
		return : Integer = sums#(1) + sums#(2);
	}
}

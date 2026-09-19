// An `out` feature an `assign` statement of the calc's body writes is bound for
// that activation and is read through a calc usage, whether the statements are
// spelled at the top of the body or nested in a while/if. A body computing its
// outputs with a loop and a conditional binds them the same way.
package test {
	private import ScalarValues::*;

	calc def TwoStmt {
		in n : Integer;
		out a : Integer;
		out b : Integer;
		assign a := n + 1;
		assign b := n + 2;
	}

	// The outputs are written from inside a nested statement body.
	calc def TwoAssign {
		in n : Integer;
		out a : Integer;
		out b : Integer;
		if true {
			assign a := n + 1;
			assign b := n + 2;
		}
	}

	calc def LoopBranch {
		in n : Integer;
		out total : Integer;
		out size : String;
		attribute s : Integer = 0;
		attribute i : Integer = 1;
		while i <= n {
			assign s := s + i;
			assign i := i + 1;
		}
		assign total := s;
		if s > 10 {
			assign size := "big";
		} else {
			assign size := "small";
		}
	}

	// An output is accumulated by the statements that write it, the last of them to
	// run being its binding for the activation.
	calc def Accum {
		in n : Integer;
		out total : Integer;
		attribute i : Integer = 1;
		assign total := 0;
		while i <= n {
			assign total := total + i;
			assign i := i + 1;
		}
	}

	// An inout is bound by the invocation and rebound by the body.
	calc def Bump {
		inout p : Integer;
		out q : Integer;
		assign p := p + 1;
		assign q := p * 10;
	}

	// A body that returns a value of its own leaves an output it assigned alone: the
	// return is the invocation's value, the assignment the output's.
	calc def AssignAndReturn {
		in n : Integer;
		out a : Integer;
		assign a := n + 1;
		n * 100
	}

	part h {
		calc cs : TwoStmt { in n = 5; }
		calc ca : TwoAssign { in n = 5; }
		calc cl : LoopBranch { in n = 5; }
		calc cb : Bump { in p = 5; }
		calc cn : Accum { in n = 5; }
		calc cr : AssignAndReturn { in n = 5; }

		attribute sa : Integer = cs.a;
		attribute sb : Integer = cs.b;
		attribute aa : Integer = ca.a;
		attribute ab : Integer = ca.b;
		attribute total : Integer = cl.total;
		attribute size : String = cl.size;
		attribute pv : Integer = cb.p;
		attribute qv : Integer = cb.q;
		attribute acc : Integer = cn.total;
		attribute ra : Integer = cr.a;
		attribute rv : Integer = AssignAndReturn(5);
	}
}

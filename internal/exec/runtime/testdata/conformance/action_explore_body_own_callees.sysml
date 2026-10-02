package test {
	private import ScalarValues::*;

	action def Own {
		in x : Integer;
		attribute a : Integer := 0;
		attribute b : Integer := 0;
		first start then s1;
		action s1 { assign a := x; }
		then s2;
		action s2 { assign b := a + 1; }
		then done;
	}

	action def OwnCallees {
		attribute sum : Integer = 0;
		// Oracle (docs/project/behavior-semantic-oracle.md): each branch performs Own in an
		// executor of its own, touching only attributes and inputs its performance holds; one outcome.
		first start;
		then fork f;
			then p;
			then q;
		action p : Own { in x = 1; }
		action q : Own { in x = 2; }
		succession p then j;
		succession q then j;
		join j;
		then action total assign sum := p.b + q.b;
		then done;
	}
}

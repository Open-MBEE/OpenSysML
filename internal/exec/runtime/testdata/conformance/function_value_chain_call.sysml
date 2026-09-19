package test {
	private import ScalarValues::*;
	part def Holder {
		attribute k : Real = 2.0;
		calc scale { in x : Real; return : Real = x * k; }
	}
	part holder : Holder;
	calc def UseChain { return : Real = holder.scale(3.0) + holder.scale(x = 4.0); }
}

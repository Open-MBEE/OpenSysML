package test {
	private import ScalarValues::*;

	part def Holder {
		attribute k : Real = 2.0;
		calc scaled { in x : Real = 5.0; return : Real = x * k; }
		calc squared { return : Real = k * k; }
	}
	part def Rig {
		part holder : Holder;
		part other : Holder { attribute :>> k = 3.0; }
	}
	part rig : Rig;

	// A calc every input of which a default supplies, or that has no input, is
	// still applied when written in call position: `holder.scaled()` is 10.0, not
	// the bare read `holder.scaled`.
	calc def UseDefaults {
		return : Real = rig.holder.scaled() + rig.holder.scaled(4.0) + rig.holder.scaled(x = 1.0)
			+ rig.holder.squared() + rig.other.squared() + rig.other.scaled();
	}
	// 10 + 8 + 2 + 4 + 9 + 15
	part def Reads {
		attribute bare : Real = rig.holder.scaled;
		attribute called : Real = rig.holder.scaled();
		attribute total : Real = UseDefaults();
	}
}

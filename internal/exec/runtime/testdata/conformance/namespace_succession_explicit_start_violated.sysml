package test {
	private import SI::*;

	part def Holder {
		perform action earlier {
			first start;
			then action wait accept after 10 [s];
			then done;
		}
		action later;
		first earlier then later;
	}

	action def Kick {
		action make { out result : Holder = new Holder(); }
		action kick { in target : Holder; perform target.later.start; }
		flow make.result to kick.target;
		first start then make;
		first make then kick;
		first kick then done;
	}
}

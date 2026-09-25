package F62 {
	attribute def Sig {
		x;
	}
	part p;
	action act;

	state def S {
		entry; then S1;

		state S1;
			accept s : Sig
			do action D
			then S2;

		state S2 {
			state S3;
		}

		transition
			first S1
			accept s : Sig
			do action D
			then S2.S3;

		transition first S2.S3 then S1 {
			assign x := 1;
		}

		attribute x = 0;
	}

	action def A {
		port def Pt;
		part driver {
			port p1 : Pt;
		}

		action turnOn send ignitionCmd via driver.p1 {
			in ignitionCmd : Sig;
		}
		action trigger accept ignitionCmd : Sig
			via driver.p1;

		first start then turnOn;
	}
}

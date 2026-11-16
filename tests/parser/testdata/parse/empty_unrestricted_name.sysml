package EmptyName {
	part def Pt {
		part x;
	}
	part def Holder {
		part <''> shortOnly : Pt;
	}
	part def Both {
		part <''> '';
	}
	part '' : Pt;
	part '   ' : Pt;
	part : Pt;
	part r :> '';
	part s :> EmptyName::''.x;
	package 'q q' {
		package '' {
			part def Q;
		}
	}
	part u : 'q q'::''::Q;
	alias Al for 'q q'::'';
}

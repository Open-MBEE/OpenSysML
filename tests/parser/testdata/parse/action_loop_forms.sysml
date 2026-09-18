package LoopForms {
	action def Descend {
		attribute altitude : Real;
		attribute steps : Real[*];
		while altitude > 0.0 {
			assign altitude := altitude - 1.0;
		}
		loop {
			assign altitude := altitude - 1.0;
		} until altitude < 0.0;
		for s in steps {
			assign altitude := altitude - s;
		}
	}
}

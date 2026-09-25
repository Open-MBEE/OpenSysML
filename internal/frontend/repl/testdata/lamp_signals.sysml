// A lamp driven by signals, so %send has accepts to trigger: a bare signal, a
// signal carrying a level its guard reads, a part exhibiting the machine and one
// exhibiting none.
package Lamps {
	private import ScalarValues::*;

	attribute def go;
	attribute def Dim {
		attribute level : Integer;
	}
	attribute def Reset;
	attribute def Batch {
		attribute levels : Integer[2..*];
	}
	attribute def Label {
		attribute text : String;
	}

	state def Lamp {
		attribute brightness : Integer = 0;
		attribute label : String = "";
		entry; then off;
		state off;
		transition off_on first off accept go then on;
		state on;
		transition on_dim first on
			accept d : Dim
			if d.level > 0
			do assign brightness := d.level
			then dimmed;
		state dimmed;
		transition dim_label first dimmed
			accept l : Label
			do assign label := l.text
			then labeled;
		state labeled;
	}

	// A machine no part exhibits, so %state of the definition alone runs it detached.
	state def Loose {
		entry; then off;
		state off;
		transition off_on first off accept go then on;
		state on;
	}

	part def Bulb {
		exhibit state lamp : Lamp;
	}

	part def Plain {
		attribute watts : Integer = 60;
	}

	part bulb : Bulb;
	part plain : Plain;
}

// A plain-name exhibit names an existing state; only `exhibit state` declares one.
package ExhibitReference {
	state def Modes {
		state on;
	}
	part def Vehicle {
		exhibit state modes : Modes;
		exhibit modes;
		exhibit modes.on;
		perform doIt;
	}
}

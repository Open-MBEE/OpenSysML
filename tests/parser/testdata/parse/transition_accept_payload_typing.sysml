package P {
	item def Start;
	item sig : Start;
	port p;
	attribute d;
	attribute t;
	attribute cond;
	package Outer {
		item def Start;
	}
	state def S {
		state a;
		state b;
		transition first a accept Start then b;
		transition first a accept Outer::Start then b;
		transition first a accept s : Start then b;
		transition first a accept s :> sig then b;
		transition first a accept Start via p then b;
		transition first a accept after d then b;
		transition first a accept at t then b;
		transition first a accept when cond then b;
		transition first a accept op(x) then b;
	}
}

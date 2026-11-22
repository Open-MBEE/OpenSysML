package P {
	part def Vehicle {
		attribute n : ScalarValues::Integer := 0;
		action close { assign n := n + 1; }
	}
	action def Base {
		part v : Vehicle;
		perform action c ::> v.close;
	}
	// A step redefining the performer without restating the reference still
	// performs `v.close`, and is the step the chain names.
	action def Mid :> Base {
		action c2 :>> c;
	}
	action def Derived :> Mid {
		out result : ScalarValues::Integer;
		action read { assign result := v.n; }
		first v.close then read;
	}
}

package P {
	enum def Level {
		low = 0.25;
		high;
		open { doc /* a value's body holds members, not values */ }
	}
	part def Q {
		w;
		v = 1;
		u { doc /* a body */ }
	}
}

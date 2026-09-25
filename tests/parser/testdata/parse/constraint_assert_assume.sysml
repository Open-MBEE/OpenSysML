constraint def SystemConstraints {
	in attribute pressure : Real;
	in attribute temp : Real;
	
	// assert constraint
	assert constraint safetyLimit {
		pressure < 100
	}
	
	// assume constraint
	assume constraint operatingConditions {
		temp > 0 and temp < 200
	}
	
	// bare expression invariant
	pressure * temp < 10000;
}

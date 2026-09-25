calc def Distance {
	in attribute v0 : Real;
	in attribute a : Real;
	in attribute dt : Real;
	return : Real = v0 * dt + 0.5 * a * dt * dt;
}

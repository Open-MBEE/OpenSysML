package P {
	metadata def M;
	part def X;
	metadata M about X;
	metadata M;
	metadata M about X {
		id = "a";
	}
	metadata m : M about X;
	@M {}
	metadata $::P::M about X;
	metadata $::P::M about X {
		id = "b";
	}
}

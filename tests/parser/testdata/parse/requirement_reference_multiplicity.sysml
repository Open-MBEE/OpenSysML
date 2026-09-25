package ReferenceMultiplicity {
	constraint c1;
	constraint c2;
	constraint c3;
	concern c4;

	requirement def R {
		assume c1 [0..*];
		require c2 [0..*];
		require c3 [0..*] :> c2;
		frame c4[0..*];
	}
}

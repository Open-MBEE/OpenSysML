package Constraints {
	part def A specializes B;
	part def B specializes A;
	part def C {
		part cap [0..3];
		part few subsets cap [0..10];
	}
	part def D {
		part lo [5..2];
	}
}

package test {
	part def A {
		attribute q = 1;
	}
	part def Sys {
		part p1 : A;
		part p2 : A;
		binding bind p1 = p2;
	}
}

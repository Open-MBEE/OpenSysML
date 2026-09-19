package test {
	part def Child {
		attribute b = 9;
	}
	part def Sys {
		attribute x;
		part child : Child;
		attribute mirror = child.b;
		binding bind child.b = x;
	}
}

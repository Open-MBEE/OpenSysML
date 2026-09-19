package test {
	part def Child {
		attribute b;
	}
	part def Sys {
		attribute x = 9;
		part child : Child;
		attribute mirror = child.b;
		binding bind child.b = x;
	}
}

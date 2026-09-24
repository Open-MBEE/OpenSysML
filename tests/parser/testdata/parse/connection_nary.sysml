part def Assembly {
	part a;
	part b;
	part c;
	part d;

	connection binary connect a to b;
	connection ternary connect (a, b, c);
	connection quaternary connect (a, b, c, d);

	connect a to b;
	connect (a, b, c);
	connect (a, b, c, d);
}

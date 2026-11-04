// `chains` takes one feature chain — a single chaining feature or two or more
// joined by `.` — and `inverse of` one feature or feature chain; `featured by`
// lists its targets (KerML 1.1 8.2.4.3.1 ChainingPart, InvertingPart, TypeFeaturingPart).
package P {
	class T { feature b; feature c { feature d; } }
	class U;
	feature a : T;
	feature x chains a.b;
	feature deep chains a.c.d;
	feature one chains a;
	feature parents : T inverse of a;
	feature children : T inverse of a.c;
	feature both : T chains a.b inverse of x featured by T, U;
}

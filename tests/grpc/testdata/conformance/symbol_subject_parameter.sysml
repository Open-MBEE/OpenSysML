// A subject is an `in` reference usage typed as written, in a requirement body
// as in a case body, and the return parameter a case inherits from the library
// is listed among its members as a declared `return` would be.
package P {
	item def Thing;
	requirement def Req { subject s : Thing; }
	verification def Ver { subject s : Thing; }
}

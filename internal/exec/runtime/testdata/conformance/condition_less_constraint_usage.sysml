// A constraint usage typed by a definition declaring no condition inherits no body
// either, so nothing binds its result: no value, not a verdict (KerML 1.1 §7.4.8.2).
package test {
	constraint def Empty;
	constraint empty : Empty;
}

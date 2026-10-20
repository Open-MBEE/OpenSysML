// Census: validateExposeVisibility
// Case: violating
// Ours: clean
// Pilot: e: no viable alternative at input 'private'
package P { part def D; } view def VD; view v : VD { private expose P::*; }

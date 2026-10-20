// Census: validateExposeVisibility
// Case: violating
// Ours: clean
// Pilot: e: no viable alternative at input 'public'
package P { part def D; } view def VD; view v : VD { public expose P::*; }

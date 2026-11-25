// Census: validateExposeVisibility
// Case: violating
// Ours: e[expose-visibility] expose with a visibility indicator: an expose is always protected, so SysML v2 admits no public, private or protected before 'expose'
// Pilot: e: no viable alternative at input 'public'
package P { part def D; } view def VD; view v : VD { public expose P::*; }

// Census: validateExposeVisibility
// Expect: error: expose with a visibility indicator: an expose is always protected, so SysML v2 admits no public, private or protected before 'expose'
package P { part def D; } view def VD; view v : VD { public expose P::*; }

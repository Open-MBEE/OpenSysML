// Census: validateConstructorExpressionOwnedFeatures
// Case: conforming
// Ours: clean
// Pilot: clean
package P { class A { feature x; } feature f = new A(x = 1); }

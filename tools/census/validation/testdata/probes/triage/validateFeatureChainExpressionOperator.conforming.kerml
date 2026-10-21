// Census: validateFeatureChainExpressionOperator
// Case: conforming
// Ours: clean
// Pilot: clean
package P { class A { feature x; } feature a : A; feature r = a.x; }

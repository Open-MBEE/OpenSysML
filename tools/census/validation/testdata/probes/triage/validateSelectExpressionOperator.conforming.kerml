// Census: validateSelectExpressionOperator
// Case: conforming
// Ours: clean
// Pilot: clean
package P { class A; feature a : A; feature r = a.?{in y; true}; }

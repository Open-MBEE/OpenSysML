// Census: validateFlowPayloadFeature
// Case: conforming
// Ours: clean
// Pilot: clean
package P { class A { feature x; } class I; feature a : A; feature b : A; flow of i : I from a.x to b.x; }

// Census: validateFlowEndNestedFeature
// Case: conforming
// Ours: clean
// Pilot: clean
package P { class A { feature x; } feature a : A; feature b : A; flow from a.x to b.x; }

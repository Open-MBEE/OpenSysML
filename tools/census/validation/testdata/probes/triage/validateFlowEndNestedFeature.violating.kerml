// Census: validateFlowEndNestedFeature
// Case: violating
// Ours: e[flow-end-subsetting] a flow end must name the feature the payload flows from or to using dot | e[flow-end-subsetting] a flow end must name the feature the payload flows from or to using dot
// Pilot: e: Must have at least two related elements | e: Cannot identify flow end (use dot notation) | e: Cannot identify flow end (use dot notation)
package P { class A { feature x; } feature a : A; feature b : A; flow from a to b; }

// Census: validateDecisionNodeOutgoingSuccessions
// Expect: error: succession out of decide d has target multiplicity [1]
package P { action def A { action a; decide d; action b; action c; first a then d; succession first d then [1] b; succession first d then [0..1] c; } }

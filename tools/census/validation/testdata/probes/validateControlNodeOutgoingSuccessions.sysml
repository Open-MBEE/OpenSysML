// Census: validateControlNodeOutgoingSuccessions
// Expect: error: succession out of join j has source multiplicity [0..1]
package P { action def A { action a; action b; join j; action c; first a then j; first b then j; succession first [0..1] j then c; } }

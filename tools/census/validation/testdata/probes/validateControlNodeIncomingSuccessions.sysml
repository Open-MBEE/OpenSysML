// Census: validateControlNodeIncomingSuccessions
// Expect: error: succession into fork f has target multiplicity [0..1]
package P { action def A { action a; fork f; action b; action c; succession first a then [0..1] f; first f then b; first f then c; } }

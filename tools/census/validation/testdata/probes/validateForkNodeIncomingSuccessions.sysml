// Census: validateForkNodeIncomingSuccessions
// Expect: error: fork f has 2 incoming successions
package P { action def A { action a; action b; fork f; action c; action d; first a then f; first b then f; first f then c; first f then d; } }

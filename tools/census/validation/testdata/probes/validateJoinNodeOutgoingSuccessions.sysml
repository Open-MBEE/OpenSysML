// Census: validateJoinNodeOutgoingSuccessions
// Expect: error: join j has 2 outgoing successions
package P { action def A { action a; action b; join j; action c; action d; first a then j; first b then j; first j then c; first j then d; } }

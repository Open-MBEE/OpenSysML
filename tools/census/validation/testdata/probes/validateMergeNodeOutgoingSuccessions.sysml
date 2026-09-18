// Census: validateMergeNodeOutgoingSuccessions
// Expect: error: merge m has 2 outgoing successions
package P { action def A { action a; action b; merge m; action c; action d; first a then m; first b then m; first m then c; first m then d; } }

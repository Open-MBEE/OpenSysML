// Census: validateMergeNodeIncomingSuccessions
// Expect: error: succession into merge m has source multiplicity [1]
package P { action def A { action a; action b; merge m; action c; succession first [1] a then m; first b then m; first m then c; } }

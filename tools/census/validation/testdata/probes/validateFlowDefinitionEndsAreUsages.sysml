// Census: validateFlowDefinitionEndsAreUsages
// Expect: error: A flow definition end must be a usage.
package P { private import Transfers::Transfer; abstract flow def F :> Transfer; }

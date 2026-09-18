// Census: validateFlowDefinitionConnectionEnds
// Expect: error: A flow connection definition can have at most two ends.
package P { part def D; flow def F { end a : D; end b : D; end c : D; } }

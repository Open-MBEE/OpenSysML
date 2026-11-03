// Census: validateFlowDefinitionFlowEnds
// Case: violating
// Ours: e[flow-def-too-many-ends] A flow definition can have at most two ends.
// Pilot: e: A flow definition can have at most two ends.
package P { part def D; flow def F { end a : D; end b : D; end c : D; } }

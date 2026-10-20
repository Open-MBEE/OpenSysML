// Census: validateElementFilterMembershipConditionIsModelLevelEvaluable
// Case: violating
// Ours: clean
// Pilot: e: Couldn't resolve reference to Type 'Boolean'. | e: An attribute must be typed by attribute definitions. | e: Must have a Boolean result
package P { private import ScalarValues::*; attribute a : Boolean; filter a; }

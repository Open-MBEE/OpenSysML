// Census: validateEventOccurrenceUsageReference
// Case: violating
// Ours: w[name-conflict] Duplicate of inherited member name 'self' from DataValue, Occurrence | e[occurrence-usage-type] An occurrence, item or part must be typed by occurrence definitions. | e[event-reference-occurrence] Must reference an occurrence.
// Pilot: e: Must reference an occurrence. | e: An occurrence, item or part must be typed by occurrence definitions.
package P { part def D { attribute a : ScalarValues::Real; event a; } }

// Census: validateOccurrenceUsageIndividualDefinition
// Expect: error: At most one individual definition is allowed.
package P { individual part def I1; individual part def I2; individual part i : I1, I2; }

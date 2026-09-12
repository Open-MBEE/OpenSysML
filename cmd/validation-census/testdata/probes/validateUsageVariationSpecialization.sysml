// Census: validateUsageVariationSpecialization
// Expect: error: A variation must not specialize another variation.
package P { part def D; variation part v1 : D; variation part v2 :> v1; }

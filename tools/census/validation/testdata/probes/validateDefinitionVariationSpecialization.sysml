// Census: validateDefinitionVariationSpecialization
// Expect: error: A variation must not specialize another variation.
package P { variation part def V1; variation part def V2 :> V1; }

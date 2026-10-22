// Census: checkUsageVariationDefinitionSpecialization
// Expect: error: An action must be typed by action definitions. variant action `a` specializes its owning variation part def `V`.
package P { part def D; variation part def V :> D { variant action a; } }

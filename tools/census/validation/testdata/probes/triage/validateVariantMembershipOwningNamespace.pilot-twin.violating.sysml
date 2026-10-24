// Census: validateVariantMembershipOwningNamespace
// Case: violating
// Ours: e[variant-outside-variation] A variant must be an owned member of a variation.
// Pilot: e: A variant must be an owned member of a variation.
package P { part def D { variant part a; } }

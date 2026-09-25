// Census: validateAssertConstraintUsageReference
// Expect: error: assert target must be a constraint usage
package P { part def D { part x : D; assert x; } }

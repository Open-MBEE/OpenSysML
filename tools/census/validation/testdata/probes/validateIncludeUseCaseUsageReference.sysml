// Census: validateIncludeUseCaseUsageReference
// Expect: error: Must reference a use case.
package P { part def D; use case def U { part d : D; include d; } }

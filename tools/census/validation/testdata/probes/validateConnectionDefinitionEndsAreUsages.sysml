// Census: validateConnectionDefinitionEndsAreUsages
// Expect: error: A connection definition end must be a usage.
package P { private import Links::BinaryLink; abstract connection def C :> BinaryLink; }
